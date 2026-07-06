---
name: syslog-backup-rsyslog
description: "Tencent Cloud rsyslog remote log backup config, test results, and Windows UDP sending gotchas"
metadata: 
  node_type: memory
  type: reference
  originSessionId: 2fbb5f58-8a7a-431f-abdf-ad78cd9821ec
---

## Server Configuration

- **Server**: Tencent Cloud `132.232.231.41`
- **Config file**: `/etc/rsyslog.d/50-remote-syslog.conf`
- **Active config**: plain TCP + UDP on port 8514 (no TLS)
- **Backup config**: `/etc/rsyslog.d/50-remote-syslog.conf.bak` (identical non-TLS version)

### Listeners
```
module(load="imudp")
input(type="imudp" port="8514" name="udp-input")
module(load="imtcp")
input(type="imtcp" port="8514" name="tcp-input")
```

### Storage
- By source IP: `/var/log/tcjtj/<source-ip>/<programname>.log`
- All remote: `/var/log/tcjtj/all-remote.log`
- Firewall: `8514/tcp` + `8514/udp` both open

## TCP: Always Works

Can use: bash `/dev/tcp`, `nc`, PowerShell, Python, Go — all fine.

## UDP: Windows Gotchas (2026-05-13 verified)

### Works
- **PowerShell `UdpClient`** — reliable, proper Windows socket API
- **Python with `time.sleep(0.1)` before `close()`** — works
- **Python without `close()`** (let process exit flush) — works

### Does NOT work on this Windows setup
- **bash `/dev/udp`** — silently fails, no packets leave the machine
- **BusyBox `nc -u`** — `-u` flag not supported
- **Python `sendto()` + immediate `close()`** — Windows drops UDP buffer on close, packet never sent

### Root Cause
Windows socket teardown: calling `close()` immediately after `sendto()` discards unflushed UDP buffers. TCP has graceful shutdown (SYN/FIN handshake) so it's immune. PowerShell `UdpClient.Close()` internally flushes; Python `socket.close()` does not.

### Reliable Python one-liner
```bash
# Option 1: no explicit close — process exit flushes
python3 -c "import socket; s=socket.socket(socket.AF_INET, socket.SOCK_DGRAM); s.sendto(b'<134>tag: msg', ('132.232.231.41', 8514))"

# Option 2: explicit delay before close
python3 -c "import socket,time; s=socket.socket(socket.AF_INET, socket.SOCK_DGRAM); s.sendto(b'<134>tag: msg', ('132.232.231.41', 8514)); time.sleep(0.1); s.close()"
```

### Syslog format requirement
Use RFC 3164: `<PRI>Timestamp Hostname Tag: Message`
Missing timestamp or malformed format may cause rsyslog to parse `%programname%` as empty → file named `.log`.

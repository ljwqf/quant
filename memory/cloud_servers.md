---
name: cloud servers deployment
description: OKX quant project is deployed on two cloud servers with different access patterns and constraints
type: reference
originSessionId: a2aa21b0-966c-4e8e-819a-10242a8c9bf0
---
## Cloud Servers

### RackNerd Server (海外)
- **IP**: 23.95.165.223
- **SSH port**: 10087
- **Access**: `ssh -p 10087 -i ~/.ssh/racknerd_key root@23.95.165.223`
- **SSH key**: `~/.ssh/racknerd_key` (on Claude Code user machine: `C:\Users\ljw\.ssh\racknerd_key`)
- **SOCKS5 proxy**: running on this server at `23.95.165.223:1080` (dante-server)
- **Notes**: OKX API accessible directly (overseas network)
- **Known issue**: Web UI was slow loading due to CDN (chart.js), fixed by localizing chart.min.js
- **Also serves as**: SOCKS5 proxy jump point for Tencent Cloud server

### Tencent Cloud Server (国内 - 132.232.231.41)
- **IP**: 132.232.231.41
- **SSH port**: 22
- **Access**: `ssh -i ~/.ssh/racknerd_key -p 22 root@132.232.231.41`
- **SSH key**: Same as RackNerd (`~/.ssh/racknerd_key`)
- **Notes**: Cannot access OKX API directly (China mainland restriction)
- **Solution**: Uses SOCKS5 proxy via RackNerd — config has `proxy_url: "socks5://23.95.165.223:1080"` and `proxy_skip_verify: true`
- **Project path**: `/root/quant/`
- **Binary**: `/root/quant/trader`
- **Config**: `/root/quant/configs/config.yaml`
- **Service**: runs trader as a foreground process (no systemd service yet)

## Deployment Workflow

### How to build and deploy to Tencent Cloud

1. **Build on local Windows machine** (cross-compile for Linux):
   ```bash
   CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o trader-linux-amd64 ./cmd/trader
   ```

2. **Upload binary to server**:
   ```bash
   scp -i ~/.ssh/racknerd_key -P 22 trader-linux-amd64 root@132.232.231.41:/root/quant/trader
   ```

3. **On the server, restart the trader**:
   ```bash
   ssh -i ~/.ssh/racknerd_key -p 22 root@132.232.231.41
   cd /root/quant
   pkill trader          # kill old process
   nohup ./trader > trader.log 2>&1 &   # start new process
   tail -f trader.log    # monitor logs
   ```

4. **Verify**:
   - Check WebSocket connects: look for "WebSocket 连接成功"
   - Check API works: look for account info logged
   - Web UI: `http://132.232.231.41:8765`

### Important notes about the SOCKS5 proxy
- The SOCKS5 proxy runs on RackNerd (23.95.165.223:1080)
- Tencent Cloud trader config references it: `proxy_url: "socks5://23.95.165.223:1080"`
- WebSocket connect has a 2-second delay when proxy is configured (SOCKS5 initialization time)
- If proxy is down, OKX API calls from Tencent Cloud will fail

## Deployment
- Docker multi-stage build (implemented but not actively used)
- Binary runs on port 8765
- Config: `configs/config.yaml` (in .gitignore)

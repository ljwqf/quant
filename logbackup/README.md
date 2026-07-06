# 日志备份服务配置指南

## 服务概览

| 服务 | 端口 | 用途 | 认证方式 |
|---|---|---|---|
| rsyslog + TLS | 8514 | 交换机/防火墙 syslog | 单向 TLS（服务器证书） |
| Go log-server | 8600 | 自定义/应用日志 | 双向 TLS（mTLS） |

服务器地址: `132.232.231.41`

---

## 方案一：交换机/防火墙 → rsyslog (8514)

### Cisco IOS 交换机/路由器

```cisco
! 配置 syslog 服务器（TCP + TLS 需设备支持）
logging host 132.232.231.41 transport tcp port 8514
logging trap informational
logging origin-id hostname
logging source-interface GigabitEthernet0/0

! 如果设备支持 TLS syslog（部分高端型号）
logging host 132.232.231.41 transport tcp port 8514 tls
```

### Huawei 交换机

```huawei
info-center loghost 132.232.231.41 port 8514 channel 0
info-center loghost source interface GigabitEthernet0/0/1
```

### Fortinet 防火墙

```fortios
config log syslogd setting
    set status enable
    set server "132.232.231.41"
    set port 8514
    set mode reliable
end
```

### H3C 交换机

```h3c
info-center loghost 132.232.231.41 port 8514
info-center loghost source interface GigabitEthernet1/0/1
```

> **注意：** 如果设备不支持 TLS syslog，可以先用普通 TCP/UDP 测试连通性，
> 然后升级到支持 TLS 的固件版本。rsyslog 配置当前设置为 `StreamDriver.Mode="1"`（强制 TLS），
> 如需兼容不支持 TLS 的设备，可改为 `"2"`（允许非 TLS）。

---

## 方案二：自定义应用 → Go log-server (8600)

使用 `log-client` 二进制发送日志：

```bash
# 单个文件
./log-client -ca-cert ca.crt -client-cert client.crt -client-key client.key \
  -server 132.232.231.41:8600 /path/to/logfile.log

# 批量发送
./log-client -ca-cert ca.crt -client-cert client.crt -client-key client.key \
  -server 132.232.231.41:8600 /var/log/app1.log /var/log/app2.log

# 自动生成测试日志并发送
./log-client -ca-cert ca.crt -client-cert client.crt -client-key client.key \
  -server 132.232.231.41:8600 -gen-test 3
```

### 为新设备生成客户端证书

```bash
# 编辑 gen-certs.sh，修改客户端 CN 为设备名称
# 然后运行
bash gen-certs.sh

# 将生成的 certs/client.crt 和 certs/client.key 复制到对应设备
```

---

## 服务器端管理

### 查看 rsyslog 状态

```bash
systemctl status rsyslog
cat /var/log/remote/all-remote.log      # 所有远程日志
ls /var/log/remote/                      # 按 IP 分类的目录
```

### 查看 Go log-server 状态

```bash
ps aux | grep log-server
cat /root/logbackup/server.log
ls -la /root/logbackup/logs_received/    # 接收到的日志文件
cat /root/logbackup/stats.json           # 统计信息
```

### 重启服务

```bash
# rsyslog
systemctl restart rsyslog

# Go log-server
pkill log-server
cd /root/logbackup
nohup ./bin/log-server -addr :8600 -ca-cert certs/ca.crt \
  -server-cert certs/server.crt -server-key certs/server.key \
  -log-dir ./logs_received > server.log 2>&1 &
```

---

## 故障排查

### 设备无法连接

```bash
# 检查端口监听
ss -tlnp | grep -E '(8514|8600)'

# 检查防火墙
iptables -L -n | head -20

# 检查 rsyslog 日志
journalctl -u rsyslog -n 50 --no-pager
```

### 日志未写入

```bash
# rsyslog 配置语法检查
rsyslogd -N1

# 查看 rsyslog 调试日志
# 在 /etc/rsyslog.conf 添加: $DebugLevel 2
```

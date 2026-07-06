# rsyslog + TLS 安装配置完整指南

> 适用于 OpenCloudOS/CentOS/RHEL 9.x，其他发行版命令略有差异

---

## 1. 安装 rsyslog-gnutls 模块

```bash
# OpenCloudOS / CentOS / RHEL
yum install -y rsyslog-gnutls

# Ubuntu / Debian
apt install -y rsyslog-gnutls
```

验证模块安装：
```bash
ls /usr/lib64/rsyslog/lmnsd_gtls.so    # RHEL 系
ls /usr/lib/rsyslog/lmnsd_gtls.so       # Debian 系
```

---

## 2. 生成 TLS 证书

### 方式 A：自签名 CA + 证书（推荐内网使用）

创建脚本 `gen-certs.sh`：

```bash
#!/bin/bash
CERT_DIR="./certs"
mkdir -p "$CERT_DIR"

# CA
openssl req -new -x509 -days 3650 -newkey rsa:4096 -nodes \
  -keyout "$CERT_DIR/ca.key" -out "$CERT_DIR/ca.crt" \
  -subj "/C=CN/ST=Beijing/O=LogBackup/CN=LogBackup CA"

# 服务端证书（需替换 IP 为你的服务器 IP）
cat > "$CERT_DIR/server.cnf" <<'EOF'
[req]
prompt = no
default_md = sha256
req_extensions = v3_req
distinguished_name = dn

[dn]
C = CN
O = LogBackup
CN = log-server

[v3_req]
subjectAltName = @alt_names

[alt_names]
IP.1 = 127.0.0.1
IP.2 = YOUR_SERVER_IP
EOF

openssl req -new -newkey rsa:2048 -nodes \
  -keyout "$CERT_DIR/server.key" -out "$CERT_DIR/server.csr" \
  -config "$CERT_DIR/server.cnf"

openssl x509 -req -days 3650 \
  -in "$CERT_DIR/server.csr" \
  -CA "$CERT_DIR/ca.crt" -CAkey "$CERT_DIR/ca.key" -CAcreateserial \
  -out "$CERT_DIR/server.crt" \
  -extfile "$CERT_DIR/server.cnf" -extensions v3_req

rm -f "$CERT_DIR"/*.csr "$CERT_DIR"/*.cnf "$CERT_DIR"/*.srl
```

### 方式 B：使用已有证书

直接将证书放到服务器：

```bash
mkdir -p /etc/rsyslog-certs
chmod 600 /etc/rsyslog-certs/server.key
chmod 644 /etc/rsyslog-certs/server.crt
chmod 644 /etc/rsyslog-certs/ca.crt
```

---

## 3. rsyslog 配置

创建 `/etc/rsyslog.d/50-tls-syslog.conf`：

```conf
# ============================================================
# rsyslog TLS 接收端配置
# ============================================================

# 1. 加载 TLS 驱动模块
module(load="imtcp"
    StreamDriver.Name="gtls"
    StreamDriver.Mode="1"
    StreamDriver.AuthMode="anon"
)

# 2. 全局 TLS 参数
global(
    DefaultNetstreamDriver="gtls"
    DefaultNetstreamDriverCAFile="/etc/rsyslog-certs/ca.crt"
    DefaultNetstreamDriverCertFile="/etc/rsyslog-certs/server.crt"
    DefaultNetstreamDriverKeyFile="/etc/rsyslog-certs/server.key"
)

# 3. 输入配置 — 监听 8514 端口（仅 TLS）
input(type="imtcp"
    port="8514"
    name="tls-input"
    # 如需双向 TLS 认证（要求客户端证书），取消注释：
    # StreamDriver.AuthMode="x509/name"
)

# 4. 存储模板
# 按来源 IP 分目录存储
template(name="RemoteByIP" type="string"
    string="/var/log/remote/%fromhost-ip%/%programname%.log"
)

# 统一存储（所有远程日志汇总）
template(name="RemoteAll" type="string"
    string="/var/log/remote/all-remote.log"
)

# 5. 路由规则
# 仅处理从 tls-input 收到的日志
:inputname, isequal, "tls-input" ?RemoteByIP
:inputname, isequal, "tls-input" ?RemoteAll
```

创建日志目录：
```bash
mkdir -p /var/log/remote
chmod 755 /var/log/remote
```

---

## 4. 验证配置并重启

```bash
# 检查配置语法
rsyslogd -N1

# 重启服务
systemctl restart rsyslog

# 查看状态
systemctl status rsyslog

# 确认端口监听
ss -tlnp | grep 8514
```

---

## 5. 测试

### 从服务器本地测试

```bash
# 发送测试日志
logger -n 127.0.0.1 -P 8514 -t testapp "rsyslog TLS test OK"

# 查看接收结果
tail -f /var/log/remote/all-remote.log
ls -la /var/log/remote/127.0.0.1/
```

### 从远程设备测试（交换机/防火墙）

**Cisco IOS：**
```cisco
logging host 132.232.231.41 transport tcp port 8514
logging trap informational
```

**Huawei：**
```huawei
info-center loghost 132.232.231.41 port 8514 channel 0
```

**Fortinet：**
```fortios
config log syslogd setting
    set status enable
    set server "132.232.231.41"
    set port 8514
    set mode reliable
end
```

---

## 6. 故障排查

### 端口不通

```bash
# 确认监听
ss -tlnp | grep 8514

# 检查防火墙
iptables -L -n | grep 8514

# 检查 SELinux（如启用）
sestatus
getsebool -a | grep syslog
```

### TLS 握手失败

```bash
# 开启 rsyslog 调试模式
# 在 /etc/rsyslog.conf 顶部添加：
$DebugLevel 2
$DebugLogFile /var/log/rsyslog-debug.log

systemctl restart rsyslog
tail -f /var/log/rsyslog-debug.log
```

### 常见错误

| 错误 | 原因 | 解决 |
|---|---|---|
| `module "lmnsd_gtls" not found` | 未安装 rsyslog-gnutls | `yum install rsyslog-gnutls` |
| `cert/key file not readable` | 文件权限错误 | `chmod 644 *.crt && chmod 600 *.key` |
| `no shared cipher` | 加密套件不匹配 | 检查设备支持的 TLS 版本 |
| `certificate verify failed` | 设备不信任 CA | 在设备上导入 ca.crt |

---

## 7. 配置参数说明

### StreamDriver.Mode

| 值 | 含义 |
|---|---|
| `0` | 不使用 TLS |
| `1` | 仅 TLS（强制） |
| `2` | TLS 优先，允许非 TLS |

### StreamDriver.AuthMode

| 值 | 含义 |
|---|---|
| `anon` | 不要求客户端证书（单向 TLS） |
| `x509/name` | 要求客户端证书 + 验证 CN |
| `x509/fingerprint` | 要求客户端证书 + 验证指纹 |

---

## 8. 完整文件清单

```
/etc/rsyslog.d/50-tls-syslog.conf   # rsyslog TLS 配置
/etc/rsyslog-certs/ca.crt           # CA 证书
/etc/rsyslog-certs/server.crt       # 服务端证书
/etc/rsyslog-certs/server.key       # 服务端私钥
/var/log/remote/                    # 日志存储目录
/var/log/remote/all-remote.log      # 汇总日志
/var/log/remote/<IP>/               # 按 IP 分类
```

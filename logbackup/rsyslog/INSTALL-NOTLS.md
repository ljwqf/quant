# rsyslog 安装配置指南（非 TLS）

> 适用于 OpenCloudOS/CentOS/RHEL 9.x，其他发行版命令略有差异

---

## 1. 安装 rsyslog

大多数 Linux 发行版已预装 rsyslog。如未安装：

```bash
# OpenCloudOS / CentOS / RHEL
yum install -y rsyslog

# Ubuntu / Debian
apt install -y rsyslog
```

验证安装：
```bash
rsyslogd -v
```

---

## 2. 创建日志存储目录

```bash
mkdir -p /var/log/remote
chmod 755 /var/log/remote
```

---

## 3. rsyslog 配置

创建 `/etc/rsyslog.d/50-remote-syslog.conf`：

```conf
# ============================================================
# rsyslog 远程日志接收配置（非 TLS）
# ============================================================

# -----------------------------------------------------------
# 选项 A：UDP 接收（推荐，传统 syslog 默认协议）
# -----------------------------------------------------------
module(load="imudp")
input(type="imudp" port="514" name="udp-input")

# -----------------------------------------------------------
# 选项 B：TCP 接收（可靠传输，但不加密）
# 如需使用 TCP，取消注释下面两行，并注释上面的 UDP 配置
# -----------------------------------------------------------
# module(load="imtcp")
# input(type="imtcp" port="514" name="tcp-input")

# -----------------------------------------------------------
# 存储模板
# -----------------------------------------------------------

# 按来源 IP 分目录存储
template(name="RemoteByIP" type="string"
    string="/var/log/remote/%fromhost-ip%/%programname%.log"
)

# 统一存储（所有远程日志汇总）
template(name="RemoteAll" type="string"
    string="/var/log/remote/all-remote.log"
)

# -----------------------------------------------------------
# 路由规则
# -----------------------------------------------------------

# UDP 模式：仅处理从 udp-input 收到的日志
:inputname, isequal, "udp-input" ?RemoteByIP
:inputname, isequal, "udp-input" ?RemoteAll

# TCP 模式（如使用 TCP，取消注释下面两行）
# :inputname, isequal, "tcp-input" ?RemoteByIP
# :inputname, isequal, "tcp-input" ?RemoteAll
```

### UDP vs TCP 对比

| 特性 | UDP（选项 A） | TCP（选项 B） |
|---|---|---|
| 可靠性 | 可能丢包（无确认） | 可靠传输（有确认） |
| 性能 | 更高 | 稍低 |
| 适用场景 | 大量日志、内网 | 重要日志、跨网络 |
| 交换机默认 | 大多数设备默认支持 | 需显式配置 |

---

## 4. 禁用 TLS 配置（如存在）

如果之前配置过 TLS，需要确保 TLS 配置不生效：

```bash
# 检查是否存在 TLS 配置
ls /etc/rsyslog.d/50-tls-syslog.conf

# 如果存在，重命名禁用
mv /etc/rsyslog.d/50-tls-syslog.conf /etc/rsyslog.d/50-tls-syslog.conf.disabled
```

---

## 5. 验证配置并重启

```bash
# 检查配置语法
rsyslogd -N1

# 重启服务
systemctl restart rsyslog

# 查看状态
systemctl status rsyslog

# 确认端口监听
# UDP 模式：
ss -ulnp | grep 514
# TCP 模式：
ss -tlnp | grep 514
```

---

## 6. 测试

### 从服务器本地测试

```bash
# UDP 测试
logger -n 127.0.0.1 -P 514 -t testapp "rsyslog UDP test OK"

# TCP 测试
logger -n 127.0.0.1 -P 514 -t testapp -T "rsyslog TCP test OK"

# 查看接收结果
tail -f /var/log/remote/all-remote.log
ls -la /var/log/remote/127.0.0.1/
```

### 从远程设备测试

**Cisco IOS：**
```cisco
! UDP 模式
logging host 132.232.231.41 transport udp port 514
! 或 TCP 模式
! logging host 132.232.231.41 transport tcp port 514
logging trap informational
```

**Huawei：**
```huawei
# UDP 模式
info-center loghost 132.232.231.41 port 514 channel 0
```

**H3C SecPath F1080：**
```h3c
# 进入系统视图
system-view

# 配置日志源接口（可选，建议使用管理口或 Loopback 地址作为源）
info-center loghost source <interface-name>

# 配置 syslog 服务器地址和端口（UDP 模式，默认就是 UDP）
info-center loghost 云服务器IP port 514 facility local6

# 配置发送日志级别（informational 及以上）
info-center source default channel loghost log level informational

# 可选：指定哪些模块发送日志到 syslog 服务器
# 例如：只发送防火墙会话日志
# info-center source SESSION channel loghost log level informational

# 可选：启用 NAT 日志输出（F1080 防火墙场景常用）
# info-center source NAT channel loghost log level informational

# 可选：开启安全日志
# info-center source SECURITY channel loghost log state on log level informational

# 保存配置
save
```

验证配置：
```h3c
# 查看日志主机配置
display info-center loghost

# 查看信息中心状态
display info-center

# 查看当前日志缓冲区，确认有日志产生
display logbuffer
```

> **注意**：
> - H3C 设备默认使用 **UDP** 协议发送 syslog
> - `facility local6` 用于在 rsyslog 端区分来源，可按需改为 `local0` ~ `local7`
> - 如果防火墙有安全策略，需要放行到 rsyslog 服务器的 UDP 514 出方向流量：
>   ```h3c
>   # 创建对象组或 ACL 放行 syslog
>   security-policy ip
>    rule 10 permit source <firewall-zone> destination <trust-zone> udp destination-port 514
>   ```

**Fortinet：**
```fortios
config log syslogd setting
    set status enable
    set server "132.232.231.41"
    set port 514
    set mode udp
end
```

**Linux 客户端（远程）：**
```bash
# 编辑 /etc/rsyslog.conf 或 /etc/rsyslog.d/99-remote.conf
# 添加以下行（发送到服务器）
*.* @132.232.231.41:514       # @ 表示 UDP
# 或
*.* @@132.232.231.41:514     # @@ 表示 TCP

# 重启 rsyslog
systemctl restart rsyslog
```

---

## 7. 故障排查

### 端口不通

```bash
# 确认监听
ss -ulnp | grep 514    # UDP
ss -tlnp | grep 514    # TCP

# 检查防火墙
iptables -L -n | grep 514
firewall-cmd --list-ports    # firewalld

# 开放端口（如需要）
firewall-cmd --permanent --add-port=514/udp
firewall-cmd --permanent --add-port=514/tcp
firewall-cmd --reload
```

### 收不到日志

```bash
# 开启 rsyslog 调试模式
# 在 /etc/rsyslog.conf 顶部添加：
$DebugLevel 2
$DebugLogFile /var/log/rsyslog-debug.log

systemctl restart rsyslog
tail -f /var/log/rsyslog-debug.log
```

### 常见问题

| 错误 | 原因 | 解决 |
|---|---|---|
| `module "imudp" not found` | rsyslog 未安装 UDP 模块 | `yum install rsyslog`（通常内置） |
| `Permission denied` | SELinux 或文件权限 | `setsebool -P syslogd_full_access 1` |
| 端口被占用 | 其他服务占用 514 | 更换端口或停止占用服务 |
| 日志未写入 | 路径不存在或权限不足 | `mkdir -p /var/log/remote && chmod 755 /var/log/remote` |

---

## 8. 配置参数说明

### UDP vs TCP 符号

在 rsyslog 配置中：
- `@` 表示 UDP（单 @）
- `@@` 表示 TCP（双 @）

### 日志级别

| 级别 | 说明 |
|---|---|
| `emerg` | 系统不可用 |
| `alert` | 需要立即行动 |
| `crit` | 严重 |
| `err` | 错误 |
| `warning` | 警告 |
| `notice` | 重要 |
| `info` | 信息 |
| `debug` | 调试 |

示例：只接收 info 及以上级别
```conf
*.info;inputname, isequal, "udp-input" ?RemoteByIP
```

---

## 9. 文件清单

```
/etc/rsyslog.d/50-remote-syslog.conf  # rsyslog 远程日志配置
/var/log/remote/                      # 日志存储目录
/var/log/remote/all-remote.log        # 汇总日志
/var/log/remote/<IP>/                 # 按 IP 分类
```

---

## 10. 日志轮转（可选）

防止日志无限增长，创建 `/etc/logrotate.d/remote-syslog`：

```conf
/var/log/remote/*.log
/var/log/remote/*/*.log {
    daily
    rotate 30
    compress
    delaycompress
    missingok
    notifempty
    create 0644 root root
    sharedscripts
    postrotate
        systemctl restart rsyslog
    endscript
}
```

# OKX 量化交易系统 - 用户手册

> 版本: 1.4.0 | 最后更新: 2026-04-17

---

## 目录

1. [快速开始](#1-快速开始)
2. [环境配置](#2-环境配置)
3. [运行模式](#3-运行模式)
4. [配置说明](#4-配置说明)
5. [策略模块](#5-策略模块)
6. [风控系统](#6-风控系统)
7. [API 接口](#7-api-接口)
8. [LLM 智能分析](#8-llm-智能分析)
9. [监控与告警](#9-监控与告警)
10. [安全功能](#10-安全功能)
11. [部署指南](#11-部署指南)
12. [常见问题](#12-常见问题)

---

## 1. 快速开始

### 1.1 系统要求

- Go 1.21+
- Git
- 操作系统: Windows / Linux / macOS

### 1.2 安装

```bash
# 克隆仓库
git clone https://github.com/ljwqf/quant.git
cd quant

# 安装依赖
make deps

# 编译
make build
```

### 1.3 快速启动

#### 方式一：使用 Go 直接运行（推荐）

```bash
# 1. 克隆仓库
git clone https://github.com/ljwqf/quant.git
cd quant

# 2. 编译项目
go build ./...

# 3. 启动模拟盘（推荐先用模拟盘测试）
go run cmd/trader/main.go --config configs/config.sim.yaml
```

#### 方式二：使用 Makefile

```bash
# 1. 设置环境变量
cp scripts/setup-env.sh scripts/setup-env.local.sh
# 编辑 setup-env.local.sh，填入真实 API 密钥
source scripts/setup-env.local.sh

# 2. 检查环境配置
make check-env

# 3. 启动模拟盘（推荐先用模拟盘测试）
make sim
```

---

## 2. 环境配置

### 2.1 API 密钥配置

系统支持两种方式配置 API 密钥：

#### 方式一：环境变量（推荐）

```bash
# OKX 交易所凭证
export OKX_API_KEY="your-api-key"
export OKX_SECRET_KEY="your-secret-key"
export OKX_PASSPHRASE="your-passphrase"

# CryptoQuant API（可选，用于 SmartFilter）
export CRYPTOQUANT_API_KEY="your-cryptoquant-key"

# LLM 提供商（可选）
export OPENAI_API_KEY="your-openai-key"
export CLAUDE_API_KEY="your-claude-key"
export QWEN_API_KEY="your-qwen-key"

# 通知系统（可选）
export TELEGRAM_BOT_TOKEN="your-telegram-bot-token"
export TELEGRAM_CHAT_ID="your-telegram-chat-id"
export DISCORD_WEBHOOK_URL="your-discord-webhook-url"
export EMAIL_PASSWORD="your-email-password"
```

#### 方式二：使用配置脚本

```bash
# 复制模板
cp scripts/setup-env.sh scripts/setup-env.local.sh

# 编辑并填入密钥
nano scripts/setup-env.local.sh

# 加载环境变量
source scripts/setup-env.local.sh
```

### 2.2 获取 API 密钥

#### OKX API 密钥

1. 登录 [OKX 官网](https://www.okx.com)
2. 进入 API 管理页面
3. 创建新 API Key
4. 权限设置建议：
   - 模拟盘：只读 + 交易
   - 实盘：只读 + 交易（谨慎授权）

#### CryptoQuant API 密钥

1. 登录 [CryptoQuant](https://cryptoquant.com)
2. 进入 API 设置页面
3. 生成 API Key

### 2.3 环境检查

```bash
make check-env
```

输出示例：
```
=== 环境变量 ===
OKX_API_KEY: 已设置
OKX_SECRET_KEY: 已设置
OKX_PASSPHRASE: 已设置
CRYPTOQUANT_API_KEY: 未设置

=== QUANT_ENV ===
QUANT_ENV: simulation

=== 配置文件 ===
configs/config.sim.yaml: 存在
configs/config.prod.yaml: 存在
```

---

## 3. 运行模式

### 3.1 模式说明

| 模式 | 配置文件 | 资金 | 用途 |
|------|----------|------|------|
| 模拟盘 | `config.sim.yaml` | 虚拟资金 | 策略测试、功能验证 |
| 实盘 | `config.prod.yaml` | 真实资金 | 正式交易 |

### 3.2 切换方式

#### 方式一：Makefile 命令（推荐）

```bash
# 模拟盘
make sim

# 实盘（需要确认）
make prod
```

#### 方式二：命令行参数

```bash
# 模拟盘
./bin/okx-trader -env simulation
./bin/okx-trader -env sim

# 实盘
./bin/okx-trader -env production
./bin/okx-trader -env prod
```

#### 方式三：环境变量

```bash
export QUANT_ENV=simulation
./bin/okx-trader
```

#### 方式四：直接指定配置

```bash
./bin/okx-trader -config configs/config.sim.yaml
```

### 3.3 优先级

```
-config > -env > QUANT_ENV > 默认配置
```

### 3.4 启动脚本

```bash
# 模拟盘脚本
bash scripts/start-sim.sh

# 实盘脚本（带安全确认）
bash scripts/start-prod.sh
```

---

## 4. 配置说明

### 4.1 配置文件结构

```
configs/
├── config.yaml          # 默认配置
├── config.sim.yaml      # 模拟盘配置
├── config.prod.yaml     # 实盘配置
└── config.yaml.example  # 配置模板
```

### 4.2 主要配置项

#### 基本配置

```yaml
basic:
  app_name: "okx-quant"
  env: "simulation"      # simulation / production
  log_level: "debug"     # debug / info / warn / error
  log_file: "./logs/okx-quant.log"
```

#### 通知系统配置

```yaml
notifications:
  enabled: true
  default_level: "info"    # debug / info / warning / error / critical
  
  console:
    enabled: true
    level: "info"
  
  telegram:
    enabled: false
    bot_token: "${TELEGRAM_BOT_TOKEN}"
    chat_id: "${TELEGRAM_CHAT_ID}"
    level: "info"
  
  discord:
    enabled: false
    webhook_url: "${DISCORD_WEBHOOK_URL}"
    level: "info"
  
  email:
    enabled: false
    smtp_host: "smtp.example.com"
    smtp_port: 587
    from: "trader@example.com"
    to: "user@example.com"
    username: "trader@example.com"
    password: "${EMAIL_PASSWORD}"
    level: "warning"
```

通知级别说明：
- `debug` - 调试信息（详细日志）
- `info` - 一般信息（策略信号、订单执行）
- `warning` - 警告信息（异常情况）
- `error` - 错误信息（交易失败）
- `critical` - 严重信息（系统故障）

#### 交易所配置

```yaml
exchange:
  okx:
    api_key: "${OKX_API_KEY}"      # 环境变量
    secret_key: "${OKX_SECRET_KEY}"
    passphrase: "${OKX_PASSPHRASE}"
    simulated: true                 # true=模拟盘, false=实盘
```

#### 风控配置

```yaml
risk:
  enable: true
  max_position_size: 10000    # 最大持仓价值 (USDT)
  max_daily_loss: 1000        # 日亏损上限 (USDT)
  max_drawdown: 0.1           # 最大回撤 (10%)
  stop_loss_percent: 0.05     # 止损比例 (5%)
  take_profit_percent: 0.1    # 止盈比例 (10%)
  max_trades_per_day: 100     # 日交易次数上限
```

#### 策略配置

```yaml
strategy:
  enable: true                    # 策略总开关
  default_symbol: "BTC-USDT"      # 默认交易对
  default_bar_interval: "1m"      # 默认K线周期
```

### 4.3 模拟盘 vs 实盘配置差异

| 配置项 | 模拟盘 | 实盘 |
|--------|--------|------|
| `simulated` | true | false |
| `max_position_size` | 10000 | 5000 |
| `max_daily_loss` | 1000 | 500 |
| `max_drawdown` | 0.1 | 0.05 |
| `stop_loss_percent` | 0.05 | 0.02 |
| `max_trades_per_day` | 100 | 50 |

---

## 5. 策略模块

### 5.1 策略列表

| 策略名称 | 类型 | 风险等级 | 描述 |
|----------|------|----------|------|
| DeltaNeutralFunding-Pro | 套利 | 低 | 资金费率套利 |
| NeedleStrategy | 短线 | 中 | MACD背离插针策略 |
| TrendFollowingStrategy | 趋势 | 中 | EMA+ADX趋势跟踪 |
| MeanReversionStrategy | 震荡 | 中 | RSI+布林带均值回归 |
| VolatilityBreakoutStrategy | 突破 | 高 | ATR波动率突破 |
| MMPEngine-Pro | 做市 | 高 | 做市商策略 |
| LiquidityHuntEngine | 流动性 | 高 | 流动性狩猎 |
| BetaArbitrageEngine | 套利 | 中 | Beta套利 |

### 5.2 策略开关

```yaml
strategy:
  enable: true  # false = 禁用所有策略
```

禁用时：
- 策略不会生成交易信号
- 不会订阅行情数据
- 风控和手动交易功能正常

### 5.3 策略参数调优

```yaml
strategy:
  trend_following:
    ema_short_period: 12
    ema_long_period: 26
    adx_threshold: 25.0
    stop_loss_percent: 0.05

  mean_reversion:
    rsi_period: 14
    rsi_overbought: 70.0
    rsi_oversold: 30.0
    bb_std_dev: 2.0
```

---

## 6. 风控系统

### 6.1 风控检查项

系统在每次交易前执行以下检查：

| 检查项 | 说明 | 错误码 |
|--------|------|--------|
| 日亏损检查 | 日亏损 < max_daily_loss | ErrDailyLossExceeded |
| 交易次数检查 | 日交易 < max_trades_per_day | ErrTradeLimitExceeded |
| 持仓限制 | 总持仓 < max_position_size | ErrPositionLimitExceeded |
| 流动性检查 | 订单簿深度充足 | ErrLiquidityInsufficient |
| 滑点检查 | 预估滑点 < max_slippage | ErrPriceDeviationTooHigh |
| 时间熔断 | 非结算时段交易 | ErrMarketClosed |

### 6.2 流动性检查

系统会在交易前：
1. 获取订单簿深度数据
2. 计算预估成交价格
3. 评估滑点是否在可接受范围
4. 流动性不足或滑点过高时拒绝交易

配置：
```yaml
execution:
  smart_routing:
    order_book_depth: 20           # 订单簿深度
    max_estimated_slippage: 0.0025 # 最大滑点 (0.25%)
```

### 6.3 熔断机制

```yaml
execution:
  rebalance:
    circuit_auto_reset: true   # 自动重置熔断
    circuit_cooldown: 15m      # 熔断冷却时间
```

---

## 7. API 接口

### 7.1 服务端口

默认端口: `8765`

```yaml
server:
  enable: true
  port: 8765
  host: "0.0.0.0"        # 0.0.0.0 允许外部访问，127.0.0.1 仅本地
```

### 7.2 Dashboard 访问

系统启动后，可以通过浏览器访问 Dashboard：

```
http://<服务器IP>:8765
```

Dashboard 功能：
- 系统状态概览
- 实时行情数据
- 策略信号查看
- 持仓和订单管理
- 条件单和限时单管理
- 移动止损设置
- 技术指标可视化
- LLM 智能分析
- 系统配置管理

### 7.3 WebSocket 连接

```
ws://<服务器IP>:8765/ws?token=your-token
```

或使用 `Authorization` 头（编程方式）：

```javascript
const ws = new WebSocket('ws://<服务器IP>:8765/ws', {
  headers: { 'Authorization': 'Bearer your-token' }
});
```

### 7.4 主要接口

#### 系统与状态

| 接口 | 方法 | 认证 | 说明 |
|------|------|------|------|
| `/health` | GET | 否 | 健康检查 |
| `/ready` | GET | 否 | 就绪检查 |
| `/api/status` | GET | 是 | 系统状态 |
| `/api/strategies` | GET | 是 | 策略列表 |
| `/api/positions` | GET | 是 | 持仓信息 |
| `/api/orders` | GET | 是 | 订单列表 |
| `/api/signals` | GET | 是 | 信号列表 |
| `/api/account` | GET | 是 | 账户信息 |
| `/api/config` | GET | 是 | 查看配置 |
| `/api/config` | PUT/POST | 是 | 保存配置 |
| `/metrics` | GET | 是 | Prometheus 指标 |

#### 策略控制

| 接口 | 方法 | 认证 | 说明 |
|------|------|------|------|
| `/api/strategy/start/<name>` | POST | 是 | 启动策略 |
| `/api/strategy/stop/<name>` | POST | 是 | 停止策略 |
| `/api/strategy/params/<name>` | GET | 是 | 查看策略参数 |
| `/api/strategy/params/<name>` | PUT/POST | 是 | 更新策略参数 |

#### 再平衡管理

| 接口 | 方法 | 认证 | 说明 |
|------|------|------|------|
| `/api/rebalance/circuit` | GET | 是 | 查看熔断状态 |
| `/api/rebalance/events` | GET | 是 | 再平衡事件历史 |
| `/api/rebalance/circuit/reset` | POST | 是 | 重置熔断 |

#### 手动交易

| 接口 | 方法 | 认证 | 说明 |
|------|------|------|------|
| `/api/manual/order` | POST | 是 | 手动下单 |
| `/api/manual/order/<id>` | DELETE | 是 | 撤单 |
| `/api/manual/orders` | GET | 是 | 订单列表 |
| `/api/manual/position/close` | POST | 是 | 手动平仓 |
| `/api/manual/position/tp-sl` | POST | 是 | 设置止盈止损 |
| `/api/manual/position/leverage` | POST | 是 | 设置杠杆 |
| `/api/manual/position/trailing-stop` | POST | 是 | 设置移动止损 |

#### 条件单与限时单

| 接口 | 方法 | 认证 | 说明 |
|------|------|------|------|
| `/api/manual/conditional-order` | POST | 是 | 创建条件单 |
| `/api/manual/conditional-order/<id>` | DELETE | 是 | 取消条件单 |
| `/api/manual/conditional-orders` | GET | 是 | 条件单列表 |
| `/api/manual/timed-order` | POST | 是 | 创建限时单 |
| `/api/manual/timed-order/<id>` | DELETE | 是 | 取消限时单 |
| `/api/manual/timed-orders` | GET | 是 | 限时单列表 |

#### 市场数据

| 接口 | 方法 | 认证 | 说明 |
|------|------|------|------|
| `/api/market/ticker` | GET | 是 | 实时行情 |
| `/api/market/bars` | GET | 是 | K线数据 |
| `/api/market/orderbook` | GET | 是 | 订单簿深度 |

#### LLM 智能分析

| 接口 | 方法 | 认证 | 说明 |
|------|------|------|------|
| `/api/llm/analyze/trade` | POST | 是 | 交易分析 |
| `/api/llm/analyze/positions` | GET | 是 | 持仓分析 |
| `/api/llm/analyze/market` | POST | 是 | 市场分析 |
| `/api/llm/analyze/orders` | POST | 是 | 订单分析 |
| `/api/llm/history` | GET | 是 | 分析历史 |

#### 数据服务

| 接口 | 方法 | 认证 | 说明 |
|------|------|------|------|
| `/api/data/news` | GET | 是 | 新闻数据 |
| `/api/data/events` | GET | 是 | 经济事件 |
| `/api/data/collect` | POST | 是 | 立即采集数据 |
| `/api/data/history` | GET | 是 | 历史K线 |
| `/api/data/ticks` | GET | 是 | 行情快照 |

#### 回测与指标

| 接口 | 方法 | 认证 | 说明 |
|------|------|------|------|
| `/api/backtest/start` | POST | 是 | 启动回测 |
| `/api/backtest/strategies` | GET | 是 | 回测策略列表 |
| `/api/backtest/results/<id>` | GET | 是 | 回测结果 |
| `/api/backtest/report/<id>` | GET | 是 | 回测报告 |
| `/api/indicators/calculate` | POST | 是 | 计算技术指标 |
| `/api/indicators/list` | GET | 是 | 指标列表 |

#### 提醒与告警

| 接口 | 方法 | 认证 | 说明 |
|------|------|------|------|
| `/api/alerts` | GET | 是 | 告警列表 |
| `/api/alerts/send` | POST | 是 | 发送告警 |

### 7.5 API 认证

#### 配置 API Token

```yaml
server:
  api_token: "your-secret-token"     # 设置后所有 /api/* 请求需认证
  force_token: true                  # 强制认证，忽略本地IP信任
  trusted_proxies: ["10.0.0.0/8"]   # 可信代理IP段
```

#### 认证方式

所有 `/api/*` 读接口（GET）和写接口（POST/PUT/DELETE）均支持以下认证方式：

**方式一：X-API-Token 请求头（推荐）**
```bash
curl -H "X-API-Token: your-token" http://<server>:8765/api/status
```

**方式二：Authorization Bearer 头**
```bash
curl -H "Authorization: Bearer your-token" http://<server>:8765/api/status
```

**方式三：WebSocket URL 参数**
```
ws://<server>:8765/ws?token=your-token
```

#### 本地信任机制

未配置 `api_token` 时，仅允许本地请求（127.0.0.1, ::1, localhost）访问。
外部 IP 请求会被拒绝。配置 `api_token` 后，所有请求均需携带令牌。

### 7.6 速率限制

所有写接口（POST/PUT/DELETE）受速率限制保护：
- **限制**：每个 IP 每分钟最多 60 次请求
- **响应码**：超出限制返回 `429 Too Many Requests`
- **说明**：持有有效 Token 的请求不受速率限制（优先通过 Token 认证绕过）

### 7.7 CORS 与安全头

系统自动处理以下安全头，支持跨域浏览器访问：

| 响应头 | 值 | 说明 |
|--------|-----|------|
| `Access-Control-Allow-Origin` | 请求的 Origin | 跨域浏览器访问 |
| `Access-Control-Allow-Methods` | `GET, POST, PUT, DELETE, OPTIONS` | 允许的 HTTP 方法 |
| `Access-Control-Allow-Headers` | `Content-Type, X-API-Token, Authorization` | 允许的请求头 |
| `X-Content-Type-Options` | `nosniff` | 防止 MIME 嗅探 |
| `X-Frame-Options` | `DENY` | 防止点击劫持 |
| `Cache-Control` | `no-store, no-cache, must-revalidate` | 禁止缓存敏感数据 |

---

## 8. LLM 智能分析

### 8.1 功能概述

系统集成了 OpenAI、Claude、Qwen 三种 LLM 提供商，支持以下分析类型：

- **交易分析**：入场前分析，评估交易信号的质量
- **持仓分析**：定期评估当前持仓，给出加仓/减仓/平仓建议
- **市场分析**：整体市场情绪和趋势判断
- **订单分析**：历史订单表现分析

### 8.2 配置

```yaml
llm:
  enable: true
  provider: "qwen"       # openai / claude / qwen
  model: "qwen-plus"     # 可选，不填则使用默认模型
  api_key: "${LLM_API_KEY}"
  base_url: ""           # 可选，自定义 API 地址
  timeout: "30s"         # 请求超时
  max_retries: 3         # 重试次数
  cache_duration: "5m"   # 响应缓存时间
```

### 8.3 使用

通过 Web 仪表盘的"AI 智能分析"面板操作，或通过 API 调用：

```bash
# 交易分析
curl -X POST http://localhost:8765/api/llm/analyze/trade \
  -H "X-API-Token: your-token" \
  -H "Content-Type: application/json" \
  -d '{"symbol":"BTC-USDT","side":"buy"}'

# 持仓分析
curl -X POST http://localhost:8765/api/llm/analyze/positions \
  -H "X-API-Token: your-token" \
  -H "Content-Type: application/json" \
  -d '{"symbol":"BTC-USDT"}'
```

---

## 9. 监控与告警

### 9.1 监控指标

- 账户余额变化
- 持仓盈亏
- 策略表现
- 订单执行状态
- 系统资源使用
- 资金费率
- Prometheus 指标（`/metrics` 端点）

### 9.2 告警配置

```yaml
alert:
  enable: true
  channels:
    - "web"
    - "webhook"
    - "dingtalk"   # 钉钉
    - "wecom"      # 企业微信
  price_change_threshold: 0.05
  check_interval: 1m

monitoring:
  alert_threshold:
    max_drawdown: 0.15
    max_loss: 1500
    position_limit: 15000
```

### 9.3 通知渠道

支持以下通知渠道：

| 渠道 | 配置项 | 说明 |
|------|--------|------|
| Web | 内置 | Web 仪表盘内通知 |
| Webhook | `webhook_url` | 自定义 HTTP 回调 |
| 钉钉 | `dingtalk_webhook` | 钉钉机器人 Markdown 消息 |
| 企业微信 | `wecom_webhook` | 企业微信机器人 Markdown 消息 |

---

### 10.4 外部访问与安全

### 10.1 外部服务器部署

将系统部署到云服务器时，需要配置以下内容以允许外部访问并确保安全。

#### 服务器配置

```yaml
server:
  enable: true
  port: 8765
  host: "0.0.0.0"              # 监听所有网卡，允许外部 IP 访问
  api_token: "your-secret-token"  # 必须设置，保护 API
  force_token: true             # 强制认证，禁用本地信任回退
  trusted_proxies: []           # 如有反向代理，填入代理 IP 段
```

#### 启用 HTTPS（可选）

```yaml
server:
  tls_enable: true
  tls_cert_file: "/path/to/cert.pem"
  tls_key_file: "/path/to/key.pem"
```

启用后服务自动使用 `ListenAndServeTLS`，浏览器访问 `https://<服务器IP>:8765`。

### 10.2 访问控制清单

部署到外部服务器前，请确认以下配置：

| 项目 | 检查项 | 建议值 |
|------|--------|--------|
| 绑定地址 | `host` | `0.0.0.0` |
| API Token | `api_token` | 强随机字符串（>32字符） |
| 强制认证 | `force_token` | `true` |
| 防火墙 | 服务器防火墙 | 仅开放 8765 端口 |
| HTTPS | `tls_enable` | 生产环境建议启用 |

### 10.3 防火墙配置

**Linux (ufw)**
```bash
# 仅允许特定 IP 访问
sudo ufw allow from 192.168.1.0/24 to any port 8765

# 或开放给所有 IP（需配合 API Token 使用）
sudo ufw allow 8765
```

**Linux (firewalld)**
```bash
firewall-cmd --permanent --add-port=8765/tcp
firewall-cmd --reload
```

### 10.4 反向代理配置（Nginx 示例）

如果使用 Nginx 作为反向代理并添加 HTTPS：

```nginx
server {
    listen 443 ssl;
    server_name your-domain.com;

    ssl_certificate /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;

    location / {
        proxy_pass http://127.0.0.1:8765;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;

        # WebSocket 支持
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
    }
}
```

配置后需在量化系统中添加代理 IP 到信任列表：
```yaml
server:
  trusted_proxies: ["127.0.0.1/32"]
```

### 10.5 WebSocket 外部访问

外部客户端连接 WebSocket 时，必须携带有效 Token：

```javascript
// 浏览器端连接示例
const ws = new WebSocket('wss://your-domain.com/ws?token=your-token');

ws.onopen = () => {
    // 订阅行情数据
    ws.send(JSON.stringify({
        action: 'subscribe',
        events: ['ticker', 'positions', 'orders']
    }));
};

ws.onmessage = (event) => {
    const data = JSON.parse(event.data);
    console.log(data.type, data.data);
};
```

### 10.6 安全建议

1. **API Token 安全**
   - 使用强随机字符串（建议 32+ 字符）
   - 不要在配置文件中明文存储，使用环境变量 `${API_TOKEN}`
   - 定期更换 Token

2. **HTTPS 优先**
   - 生产环境建议启用 TLS，防止中间人攻击
   - 可使用 Let's Encrypt 免费证书

3. **最小权限原则**
   - OKX API 密钥仅授予"交易"权限，不授予"提币"
   - 使用模拟盘充分测试后再切换实盘

4. **防火墙限制**
   - 如果只从固定 IP 访问，在防火墙层面限制来源 IP
   - 不要将管理端口暴露到公网

5. **监控异常访问**
   - 查看日志中的 "API认证失败" 告警
   - 关注速率限制触发的 429 响应

## 10. 安全功能

### 10.1 审计日志

系统内置结构化审计日志，记录以下 14 种敏感操作：

| 事件类型 | 说明 |
|----------|------|
| `order.create` / `order.cancel` | 下单/撤单 |
| `config.update` | 配置变更 |
| `strategy.start` / `strategy.stop` / `strategy.params` | 策略启停/参数修改 |
| `leverage.change` | 杠杆调整 |
| `tpsl.change` | 止盈止损设置 |
| `trailing.stop` | 移动止损 |
| `timedorder.create` / `timedorder.cancel` | 限时单创建/取消 |
| `condorder.create` / `condorder.cancel` | 条件单创建/取消 |
| `rebalance.reset` | 再平衡重置 |

每条记录包含：时间戳、调用方身份（api_token/local/anonymous）、HTTP 方法、路径、远程地址、详情、状态。
同时输出结构化字段和 JSON 格式，便于日志聚合工具查询。

### 10.2 IP 白名单

在 `server.ip_whitelist` 中配置允许的 IP 地址段，支持 CIDR 格式：

```yaml
server:
  ip_whitelist:
    - "192.168.1.0/24"
    - "10.0.0.1"
```

- 不配置时：允许所有请求
- 配置后：仅白名单内 IP 可访问，其他返回 403
- 支持 `X-Forwarded-For` / `X-Real-IP` 头提取真实 IP（反向代理场景）
- 运行时可通过 `Update()` 方法动态刷新

### 10.3 HTTPS/TLS

```yaml
server:
  tls_enable: true
  tls_cert_file: "/path/to/cert.pem"
  tls_key_file: "/path/to/key.pem"
```

启用后服务自动使用 `ListenAndServeTLS` 启动。

---

## 11. 部署指南

### 11.1 Docker 部署

```bash
docker build -t okx-quant .
docker run -d --name quant \
  -p 8765:8765 \
  -e OKX_API_KEY=xxx \
  -e OKX_SECRET_KEY=xxx \
  -e OKX_PASSPHRASE=xxx \
  okx-quant
```

Dockerfile 特性：多阶段构建、CGO_ENABLED=0 静态链接、非 root 用户（appuser）、健康检查（/health 端点）、ldflags 版本注入。

**云服务器部署注意**：`docker-compose.yml` 中需添加 `- ./web:/app/web` 卷挂载，使 `web/` 目录下的静态文件（HTML/JS）可通过宿主机直接更新，无需重新构建镜像。

Chart.js 已本地化至 `web/static/js/chart.min.js`，不再依赖外部 CDN，确保国内网络环境下页面加载速度。

### 11.2 Kubernetes 部署

```bash
kubectl apply -f deployments/k8s/
```

包含：namespace、configmap、secret、deployment、service、pvc。
详见 `deployments/k8s/README.md`。

### 11.3 CI/CD

推送代码到 GitHub 后自动触发：
1. **lint** — golangci-lint + go vet + gofmt
2. **test** — -race 检测 + coverage 统计
3. **build** — 多平台交叉编译（linux/darwin/windows × amd64/arm64）
4. **security** — Trivy 漏洞扫描

详见 `.github/workflows/ci.yml`。

---

## 12. 常见问题

### 11.1 启动失败

**问题**: `加载配置失败`

**解决**:
1. 检查配置文件是否存在
2. 检查配置文件格式是否正确
3. 确保环境变量已设置

```bash
make check-env
```

### 11.2 API 连接失败

**问题**: `连接交易所失败`

**解决**:
1. 检查网络连接
2. 确认 API 密钥正确
3. 检查 IP 白名单设置
4. 确认 API 权限配置

### 11.3 策略不执行

**问题**: 策略已启用但不产生交易

**解决**:
1. 检查 `strategy.enable` 是否为 true
2. 检查风控限制是否过严
3. 检查行情数据是否正常
4. 查看日志了解具体原因

### 11.4 流动性检查失败

**问题**: `ErrLiquidityInsufficient`

**解决**:
1. 降低单笔交易数量
2. 提高滑点容忍度
3. 选择流动性更好的交易对

```yaml
execution:
  smart_routing:
    max_estimated_slippage: 0.005  # 提高到 0.5%
```

### 11.5 如何切换交易对

修改配置文件：
```yaml
strategy:
  default_symbol: "ETH-USDT"  # 改为 ETH
```

或使用命令行参数（需代码支持）。

### 11.6 外部浏览器无法访问 Dashboard

**问题**: 部署到服务器后，浏览器打不开 Dashboard

**解决**:
1. 确认 `host` 配置为 `0.0.0.0` 而非 `127.0.0.1`
2. 确认服务器防火墙已开放 8765 端口
3. **检查云服务器安全组** — RackNerd 等云服务商可能有外部防火墙，需在控制面板中放行 8765 端口
4. 测试连通性: `curl http://<服务器IP>:8765/health`
5. 如端口被云服务商封锁，可考虑将端口改为 80/443（通常默认放行）

### 11.7 API 请求返回 401 未授权

**问题**: 调用 API 接口返回 `{"error": "未授权：缺少认证令牌"}`

**解决**:
1. 未配置 `api_token` 时，仅本地请求可访问。外部请求需设置 `api_token`
2. 请求时携带认证头: `-H "X-API-Token: your-token"`
3. 或在 WebSocket URL 中添加 `?token=your-token` 参数
4. 配置 `force_token: true` 后可强制所有请求认证

### 11.8 API 请求返回 429 请求过多

**问题**: 频繁调用写接口返回 429 错误

**解决**:
1. 降低请求频率，每 IP 每分钟限制 60 次写操作
2. 持有有效 Token 的请求不受速率限制，确保携带正确的 `X-API-Token`
3. 在客户端实现重试机制，遇到 429 时等待后重试

---

## 附录

### A. 完整命令列表

```bash
make help
```

| 命令 | 说明 |
|------|------|
| `make build` | 编译程序 |
| `make run` | 运行程序（默认配置）|
| `make sim` | 模拟盘运行 |
| `make prod` | 实盘运行 |
| `make test` | 运行测试 |
| `make test-cover` | 覆盖率测试 |
| `make test-race` | 竞态检测 |
| `make check-env` | 环境检查 |
| `make clean` | 清理编译产物 |

### B. 文件结构

```
quant/
├── cmd/trader/           # 主程序
├── configs/              # 配置文件
│   ├── config.yaml
│   ├── config.sim.yaml
│   └── config.prod.yaml
├── internal/             # 内部模块
│   ├── api/              # API 服务
│   ├── config/           # 配置加载
│   ├── exchange/         # 交易所接口
│   ├── execution/        # 执行引擎
│   ├── risk/             # 风控引擎
│   └── strategy/         # 策略模块
├── pkg/                  # 公共包
├── scripts/              # 脚本
│   ├── setup-env.sh
│   ├── start-sim.sh
│   └── start-prod.sh
├── web/                  # Web 界面
├── Makefile
└── README.md
```

### C. 安全建议

1. **永远不要提交真实 API 密钥到 Git**
2. 使用环境变量管理敏感信息
3. 模拟盘测试充分后再切换实盘
4. 设置合理的风控参数
5. 定期检查账户和系统状态

---

## 联系与支持

- 问题反馈: [GitHub Issues](https://github.com/ljwqf/quant/issues)
- 文档更新: 查看 `docs/` 目录
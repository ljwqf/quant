# OKX 量化交易系统 - 用户手册

> 版本: 1.0.0 | 最后更新: 2026-03-27

---

## 目录

1. [快速开始](#1-快速开始)
2. [环境配置](#2-环境配置)
3. [运行模式](#3-运行模式)
4. [配置说明](#4-配置说明)
5. [策略模块](#5-策略模块)
6. [风控系统](#6-风控系统)
7. [API 接口](#7-api-接口)
8. [监控与告警](#8-监控与告警)
9. [常见问题](#9-常见问题)

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
  host: "0.0.0.0"
```

### 7.2 主要接口

| 接口 | 方法 | 说明 |
|------|------|------|
| `/health` | GET | 健康检查 |
| `/ready` | GET | 就绪检查 |
| `/api/status` | GET | 系统状态 |
| `/api/strategies` | GET | 策略列表 |
| `/api/positions` | GET | 持仓信息 |
| `/api/orders` | GET | 订单列表 |
| `/api/balance` | GET | 账户余额 |

### 7.3 API 认证

```yaml
server:
  api_token: "${API_TOKEN}"  # 可选，设置后需要认证
```

请求头：
```
X-API-Token: your-token
```

---

## 8. 监控与告警

### 8.1 监控指标

- 账户余额变化
- 持仓盈亏
- 策略表现
- 订单执行状态
- 系统资源使用

### 8.2 告警配置

```yaml
alert:
  enable: true
  channels:
    - "web"
    - "webhook"
  price_change_threshold: 0.05  # 价格变动告警阈值
  check_interval: 1m            # 检查间隔

monitoring:
  alert_threshold:
    max_drawdown: 0.15    # 回撤告警阈值
    max_loss: 1500        # 亏损告警阈值
    position_limit: 15000 # 持仓告警阈值
```

### 8.3 Webhook 告警

```yaml
monitoring:
  alert:
    webhook_url: "${ALERT_WEBHOOK_URL}"
```

---

## 9. 常见问题

### 9.1 启动失败

**问题**: `加载配置失败`

**解决**:
1. 检查配置文件是否存在
2. 检查配置文件格式是否正确
3. 确保环境变量已设置

```bash
make check-env
```

### 9.2 API 连接失败

**问题**: `连接交易所失败`

**解决**:
1. 检查网络连接
2. 确认 API 密钥正确
3. 检查 IP 白名单设置
4. 确认 API 权限配置

### 9.3 策略不执行

**问题**: 策略已启用但不产生交易

**解决**:
1. 检查 `strategy.enable` 是否为 true
2. 检查风控限制是否过严
3. 检查行情数据是否正常
4. 查看日志了解具体原因

### 9.4 流动性检查失败

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

### 9.5 如何切换交易对

修改配置文件：
```yaml
strategy:
  default_symbol: "ETH-USDT"  # 改为 ETH
```

或使用命令行参数（需代码支持）。

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
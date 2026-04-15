# OKX量化交易系统

> **版本:** 1.3.0 | **更新日期:** 2026-04-16 | **状态:** ✅ P0/P1 全部完成，代码级验证收敛

这是一个基于Go语言开发的OKX量化交易系统，支持自动交易、风险管理、回测和监控功能。

## 功能特性

- **自动交易**：基于OKX v5 API的自动交易，支持市价单、限价单、条件单、限时单
- **多策略引擎**：8种交易策略（DeltaNeutral、TrendFollowing、MeanReversion、VolatilityBreakout、Needle、MMP、BayesianAllocator、BetaArbitrage）
- **SmartFilter**：基于链上数据（CryptoQuant/Santiment）的市场状态分析和策略过滤
- **LLM智能分析**：集成OpenAI/Claude/Qwen，支持交易分析、持仓分析、市场分析、订单分析
- **风控管理**：止损止盈、仓位限制、每日亏损限制、最大回撤控制
- **回测系统**：基于历史数据测试交易策略，生成绩效报告
- **实时监控**：实时行情、资金费率监控、系统资源监控
- **多渠道通知**：Webhook、钉钉、企业微信
- **Prometheus指标**：内置 `/metrics` 端点，可对接 Grafana
- **条件单/移动止损**：价格条件、时间条件、移动止损
- **手动交易**：Web 仪表盘支持手动下单、限时单、条件单、持仓管理
- **WebSocket推送**：实时行情、订单状态、条件单状态变更推送
- **真实数据源**：CryptoCompare 新闻、TradingEconomics 经济日历（API失败自动降级）
- **安全加固**：审计日志（14种事件类型）、IP白名单中间件、HTTPS/TLS支持
- **CI/CD**：GitHub Actions 流水线（lint + test + build + Trivy安全扫描）

## 项目结构

```
quant/
├── cmd/
│   └── trader/           # 主程序入口
│       ├── main.go       # 启动流程
│       └── *_test.go     # 启动测试
├── configs/              # 配置文件（sim/prod）
├── data/                 # 运行时数据目录
├── deployments/          # 部署配置
│   └── k8s/              # Kubernetes 部署 YAML
├── docs/                 # 文档
│   ├── CHANGELOG.md      # 变更记录
│   ├── ENHANCEMENT_PLAN.md  # 增强计划
│   ├── PRODUCTION_ROADMAP.md # 生产路线图
│   └── USER_MANUAL.md    # 用户手册
├── internal/
│   ├── api/              # HTTP API 服务器 + WebSocket
│   │   ├── audit.go      # 审计日志（14种事件类型）
│   │   └── ip_whitelist.go # IP白名单中间件
│   ├── backtest/         # 回测引擎
│   ├── config/           # 配置管理
│   ├── dataservice/      # 数据采集（新闻/经济日历）
│   ├── exchange/         # 交易所接口抽象
│   │   └── okx/          # OKX API（REST + WebSocket）
│   ├── execution/        # 订单执行 + 对账服务
│   ├── indicator/        # 技术指标计算
│   ├── llmanalysis/      # LLM 智能分析
│   ├── manualtrading/    # 手动交易（条件单/限时单）
│   ├── monitoring/       # 系统监控 + Prometheus + 资金费率
│   ├── notifications/    # 多渠道通知（Webhook/钉钉/企微）
│   ├── risk/             # 风控管理
│   ├── storage/          # 数据库 + 仓储层
│   └── strategy/         # 交易策略（8种）
├── pkg/
│   ├── errors/           # 错误处理
│   ├── logger/           # 日志系统（zap）
│   ├── persistence/      # 持久化抽象
│   └── types/            # 核心类型定义
├── web/                  # Web 仪表盘
│   ├── index.html
│   └── static/
│       ├── css/style.css
│       └── js/app.js
├── .github/workflows/
│   └── ci.yml            # CI/CD 流水线
├── Dockerfile            # 多阶段 Docker 构建
├── .dockerignore         # Docker 排除配置
├── go.mod
└── README.md
```

## 安装与运行

### 环境要求

- Go 1.21+
- Docker（可选，用于容器化部署）

### 本地运行

1. 克隆项目

```bash
git clone https://github.com/yourusername/okx-quant.git
cd okx-quant
```

2. 安装依赖

```bash
go mod download
```

3. 配置API密钥

推荐使用环境变量，不在配置文件中写入明文密钥：

```powershell
$env:OKX_API_KEY="your_api_key"
$env:OKX_SECRET_KEY="your_secret_key"
$env:OKX_PASSPHRASE="your_passphrase"
```

配置文件建议：
- `configs/config.sim.yaml`（模拟盘）
- `configs/config.prod.yaml`（实盘）

4. 运行应用

```bash
go run cmd/trader/main.go -env simulation
```

5. 访问 Web 仪表盘

打开浏览器访问 `http://localhost:8765`

### 构建

```bash
go build -o trader ./cmd/trader
```

### 运行测试

```bash
go test ./... -v
```

### 容器化部署

1. 构建镜像

```bash
docker build -t okx-quant .
```

2. 运行容器

```bash
docker-compose up -d
```

3. 查看日志

```bash
docker-compose logs -f
```

### Kubernetes 部署

```bash
kubectl apply -f deployments/k8s/
```

详见 `deployments/k8s/README.md`。

## 项目状态（2026-04-16）

| 维度 | 状态 |
|------|------|
| P0 项 | 10/10 (100%) ✅ |
| P1 项 | 37/37 (100%) ✅ |
| 构建 | `go build` / `go vet` 通过，零警告 |
| 测试 | 20 个测试包全部通过 |
| 安全扫描 | 无密钥泄露、无 SQL/命令注入 |
| 资源管理 | Ticker/HTTP Body/Mutex 全部正确释放 |
| 恢复机制 | WS 重连、REST 重试、数据源降级均就绪 |

详见：
- `spec/checklist.md` — 上线前检查清单
- `docs/ENHANCEMENT_PLAN.md` — 增强实施计划
- `docs/CHANGELOG.md` — 变更记录

## 配置说明

配置文件位于 `configs/config.yaml`，主要包含以下部分：

- **basic**：应用名称、环境、日志级别
- **exchange**：OKX API 密钥和 URL
- **risk**：最大仓位、止损止盈、每日亏损限制
- **backtest**：初始余额、数据目录
- **monitoring**：检查间隔、告警阈值
- **strategy**：策略配置（8种策略）
- **server**：HTTP 服务器端口
- **notifications**：通知渠道配置（Webhook/钉钉/企微）
- **llm**：LLM 提供商和 API 配置

### 环境变量配置

- **OKX API 配置**：
  - `OKX_API_KEY` / `OKX_SECRET_KEY` / `OKX_PASSPHRASE`

- **SmartFilter 配置**：
  - `SMART_FILTER_SOURCE`：数据源（file, http, env, auto）
  - `SMART_FILTER_FILE_PATH`：文件路径
  - `SMART_FILTER_REFRESH_ENABLED`：自动刷新（默认：true）
  - `SMART_FILTER_REFRESH_INTERVAL`：刷新间隔（默认：5m）
  - `SMART_FILTER_NETFLOW` / `SMART_FILTER_SOPR` / `SMART_FILTER_MVRV`

- **CryptoQuant 配置**：
  - `CRYPTOQUANT_API_KEY`：API 密钥

- **Santiment 配置**（可选）：
  - `SANTIMENT_API_KEY`：API 密钥

- **CryptoCompare 新闻**（自动降级，无需额外配置）

- **TradingEconomics 经济日历**：
  - `TRADING_ECONOMICS_API_KEY`：API 密钥（可选，无 key 时使用内置事件）

## 回测系统使用

1. 准备历史数据

将历史K线数据保存到 `data/` 目录，格式为CSV文件，包含以下字段：
- timestamp：时间戳（毫秒）
- open：开盘价
- high：最高价
- low：最低价
- close：收盘价
- volume：成交量
- interval：时间周期

2. 实现策略

创建一个实现 `backtest.Strategy` 接口的策略文件：

```go
package strategies

import (
	"github.com/ljwqf/quant/pkg/types"
)

type MyStrategy struct {
	// 策略参数
}

func (s *MyStrategy) Name() string {
	return "MyStrategy"
}

func (s *MyStrategy) Init(params map[string]interface{}) error {
	return nil
}

func (s *MyStrategy) OnBar(bar *types.Bar) (*types.Signal, error) {
	// 策略逻辑
	return nil, nil
}

func (s *MyStrategy) GetParameters() map[string]interface{} {
	return nil
}
```

3. 运行回测

```go
strategy := &strategies.MyStrategy{}
engine := backtest.NewEngine(strategy, 10000)

if err := engine.LoadDataFromFile("data/btc_usdt_1h.csv", "BTC/USDT"); err != nil {
	log.Fatal(err)
}

if err := engine.Run(); err != nil {
	log.Fatal(err)
}

result := engine.GetResult()
fmt.Printf("回测结果: %+v\n", result)
```

## 监控系统

监控系统实时监控以下指标：

- 账户余额和权益
- 持仓状态
- 交易历史
- 风险指标
- 系统健康状态
- 资金费率
- Prometheus 指标（`/metrics` 端点）

告警渠道：
- 控制台
- Webhook
- 钉钉
- 企业微信

## 风险管理

风险管理模块检查以下风险：

- 仓位限制：控制单个持仓的最大大小
- 止损止盈：自动设置止损和止盈订单
- 每日亏损限制：控制每日最大亏损
- 最大回撤：控制最大资金回撤
- 交易频率：控制每日最大交易次数
- 订单对账：定期同步本地与交易所订单状态

## SmartFilter 使用指南

SmartFilter 是一个基于链上数据的市场状态分析和策略过滤模块。

### 数据源配置

1. **CryptoQuant**（推荐）：
   - 免费计划：50 req/day
   - 配置 `CRYPTOQUANT_API_KEY` 环境变量
   - 系统自动获取，无需运行额外脚本

2. **Santiment**（可选）：
   - 配置 `SANTIMENT_API_KEY` 环境变量

3. **文件数据源**：手动创建 JSON 文件

4. **环境变量**：直接设置 `SMART_FILTER_NETFLOW`、`SMART_FILTER_SOPR`、`SMART_FILTER_MVRV`

### 自动数据刷新

SmartFilter 默认每 5 分钟自动刷新数据，可通过 `SMART_FILTER_REFRESH_INTERVAL` 调整。

## 注意事项

1. **API密钥安全**：不要将API密钥提交到版本控制系统
2. **策略测试**：实盘交易前，通过回测系统充分测试
3. **风险控制**：合理设置风险管理参数
4. **系统监控**：定期检查系统运行状态和告警
5. **API限制**：注意 CryptoQuant 免费计划的限制（50 req/day）
6. **数据缓存**：系统内置缓存机制，遵守 API 调用限制

## 许可证

MIT License

# OKX量化交易系统

> **版本:** 2.0-dev | **分支:** `arch-v2-liquidity` | **状态:** V2 五层架构重构进行中

基于 Go 语言开发的 OKX 量化交易系统。V1 已完成全部核心功能并通过生产可行性验证（CRITICAL + HIGH 问题全部修复），当前正在推进 **V2 五层架构重构**——设计哲学从"看涨跌"转向"只看流动性墙的坍缩与真空的填充"。

> **V2 架构文档**: [docs/V2_ARCHITECTURE_PLAN.md](docs/V2_ARCHITECTURE_PLAN.md) | **V1 用户手册**: [docs/USER_MANUAL.md](docs/USER_MANUAL.md) | **变更记录**: [docs/CHANGELOG.md](docs/CHANGELOG.md)

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
│   └── trader/           # 主程序入口（V1）
│       ├── main.go       # 启动流程、组件组装
│       └── subscriptions.go # 行情订阅 → 策略回调链
├── configs/              # 配置文件
│   ├── config.yaml       # 默认配置
│   ├── config.sim.yaml   # 模拟盘
│   └── config.prod.yaml  # 实盘
├── deployments/          # 部署配置
│   ├── k8s/              # Kubernetes 部署 YAML
│   └── private/          # 双服务器部署记录（.gitignore）
├── docs/                 # 文档
│   ├── V2_ARCHITECTURE_PLAN.md  # V2 五层架构重构计划（30 步安全切片）
│   ├── CHANGELOG.md      # 变更记录（含历史归档）
│   └── USER_MANUAL.md    # V1 用户手册
├── internal/             # 内部包（V1）
│   ├── api/              # HTTP API + WebSocket（3441 行）
│   ├── exchange/okx/     # OKX REST + WebSocket 客户端
│   ├── execution/        # 订单执行引擎（3112 行）
│   ├── risk/             # 风控引擎
│   ├── strategy/         # 9 种交易策略
│   ├── config/           # 配置加载（viper）
│   ├── monitoring/       # Prometheus + 资金费率 + 实时 PnL
│   ├── notifications/    # 钉钉/企微/Telegram/Discord/Email
│   ├── storage/          # SQLite + 仓储层
│   ├── llmanalysis/      # LLM 智能分析
│   ├── manualtrading/    # 手动交易
│   ├── dataservice/      # 新闻/经济数据采集
│   ├── backtest/         # 回测引擎
│   ├── indicator/        # 技术指标
│   └── alertservice/     # 提醒服务
├── internal/v2/          # V2 五层架构（重构中）
│   ├── ingestion/        # 第一层：数据与感知
│   ├── computation/      # 第二层：计算与衍生
│   ├── decision/         # 第三层：决策与大脑
│   ├── execution/        # 第四层：风控与执行
│   ├── monitor/          # 第五层：监控与日志
│   ├── events/           # channel 事件类型定义
│   └── adapter/          # V1→V2 适配器（防腐层）
├── pkg/                  # 公共包
│   ├── types/            # 核心类型（Order, Signal, Tick, Bar...）
│   ├── logger/           # zap 日志
│   └── errors/           # 错误处理
├── web/                  # Web 仪表盘
│   ├── index.html        # V1 面板
│   ├── config.html       # 配置页面
│   └── static/
│       ├── css/style.css
│       └── js/
│           ├── app.js    # V1 前端逻辑（3139 行）
│           └── chart.min.js
├── Dockerfile            # 多阶段 Docker 构建
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

> **注意**: 云服务器部署时，需在 `docker-compose.yml` 中添加 `- ./web:/app/web` 卷挂载，
> 确保静态文件更新无需重新构建镜像。同时确认云服务商安全组已放行 8765 端口。

3. 查看日志

```bash
docker-compose logs -f
```

### Kubernetes 部署

```bash
kubectl apply -f deployments/k8s/
```

详见 `deployments/k8s/README.md`。

## 项目状态（2026-04-26）

### V1 状态（已完成）

| 维度 | 状态 |
|------|------|
| CRITICAL 问题 | 3/3 全部修复 ✅ |
| HIGH 问题 | 4/4 全部修复 ✅ |
| P0 检查项 | 10/10 (100%) ✅ |
| P1 检查项 | 37/37 (100%) ✅ |
| 构建 | `go build` / `go vet` 通过 |
| 部署 | 腾讯云 + RackNerd 双服务器运行中 |

### V2 状态（进行中）

| Phase | 状态 | 说明 |
|-------|------|------|
| Phase 0 基础设施 | 待开始 | V2 目录、接口、适配器 |
| Phase 1 数据感知层 | 待开始 | TickStream + OrderBookBuilder |
| Phase 2 计算衍生层 | 待开始 | 流动性墙 + 耗散速率 |
| Phase 3 决策大脑层 | 待开始 | 状态机 + 模糊评分 |
| Phase 4 风控执行层 | 待开始 | ProfitPool + 行为止损 |
| Phase 5 监控日志层 | 待开始 | KillAll + 快照存证 |
| Phase 6 Web 面板 | 待开始 | V2 独立面板 |
| Phase 7 部署切换 | 待开始 | 双轨运行 → V2 独立 |

详见 `docs/V2_ARCHITECTURE_PLAN.md`

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

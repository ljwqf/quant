# V2 架构实施评估与路线图

## 目标

本文档记录当前 OKX 量化交易项目的现状评估、主要风险、V2 五层架构实施路线和验收标准，用于指导后续从 V1 向 V2 平滑演进。

本次评估不生产业务代码，重点是基于当前仓库事实形成可执行方案。

## 总体结论

当前项目不是从零开始，而是一个 V1 功能较完整、正在向 V2 五层架构演进的 OKX 量化交易系统。V1 已具备交易执行、策略、回测、Web 面板、监控、审计、通知、风控等核心能力，但代码集中度较高，V2 目录尚未实际落地。

后续最稳妥的路线不是直接拆 V1，而是旁路建设 `internal/v2/`，通过防腐适配器复用 V1 能力，逐层验证后灰度切换。

## 当前状态

- 项目语言和框架：Go，模块名为 `github.com/ljwqf/quant`。
- 主入口：`cmd/trader/main.go`，当前仍是 V1 组装中心。
- 核心业务模块集中在 `internal/`。
- 通用类型在 `pkg/types/`，监控能力在 `pkg/monitor/`。
- V2 文档已存在：`docs/V2_ARCHITECTURE_PLAN.md`。
- 当前未发现 `internal/v2/**/*.go`，说明 V2 设计已形成，工程落地尚未开始或尚未提交。

## 核心模块

- `internal/exchange/`：交易所接口与 OKX 实现。
- `internal/execution/`：订单执行、风控、订单状态等。
- `internal/strategy/`：策略接口和策略引擎。
- `internal/api/`：API 与 WebSocket 服务。
- `internal/backtest/`：回测系统。
- `internal/dataservice/`：外部数据源。
- `internal/storage/`：存储层。
- `internal/config/`：配置加载。

## 可复用资产

- `internal/exchange/exchange.go` 已有较完整交易所抽象，可作为 V2 ingestion 和 execution 的防腐基础。
- `internal/exchange/okx/` 可继续复用 OKX REST/WebSocket 能力。
- `pkg/types/types.go` 已有 `Order`、`Tick`、`OrderBook`、`Signal`、`Position`、`Account` 等基础类型。
- `internal/backtest/` 可作为 V2 因子与决策逻辑的验证基础。
- `internal/storage/` 可复用历史数据、订单、交易记录等存储能力。
- `pkg/monitor/`、Prometheus、Grafana、审计、通知能力可继续用于 V2。
- CI 已包含 lint、vet、race test、coverage、安全扫描和多平台构建，基础工程能力较好。

## 主要问题

- `cmd/trader/main.go` 职责过重，是典型 God function，组件初始化、配置、依赖装配、生命周期和业务编排耦合在一起。
- `internal/execution/execution.go` 职责过宽，执行、风控、订单状态、重试、熔断等逻辑集中，后续扩展流动性交易会变得脆弱。
- `internal/api/server.go` 体量较大，API/Web 层直接持有较多业务对象，接口层与核心交易逻辑耦合偏高。
- `web/static/js/app.js` 单文件规模大，前端可维护性弱。
- V2 架构文档已经清晰，但代码骨架尚未存在，缺少可运行的最小闭环。
- Go 版本配置不一致：`go.mod` 声明 `go 1.25.0`，CI 使用 `1.21`，存在构建不一致风险。
- Docker 配置端口不一致：`Dockerfile` 暴露 `8765`，`docker-compose.yml` 映射 `8080:8080`，需要统一。
- README 提到 `configs/config.yaml`，但当前主要看到 `configs/config.yaml.example`、`configs/config.prod.yaml`、`configs/config.sim.yaml`，本地启动路径需要进一步规范。

## 推荐实施路线

### Phase 0：V2 最小骨架

- 新增 `internal/v2/` 目录结构。
- 定义统一事件模型，例如市场事件、订单簿事件、因子事件、决策事件、执行事件、风险事件。
- 定义生命周期接口，例如 `Start(ctx)`、`Stop(ctx)`、`Health()`。
- 定义 channel 边界，保证层间只通过事件流通信。
- 建立 V1 到 V2 的防腐适配器，不直接让 V2 依赖 V1 复杂对象。
- 添加 feature flag，例如 `v2.enabled`、`v2.shadow_mode`、`v2.paper_only`。

### Phase 1：第一层 ingestion

- 接入 OKX ticker、bar、order book、trade 数据。
- 将 V1 exchange WebSocket 能力包装成 V2 事件源。
- 实现连接状态、重连、延迟、数据丢包统计。
- 先只读数据，不下单。
- 验收重点是数据稳定性和事件结构正确性。

### Phase 2：第二层 computation

- 实现流动性墙、订单簿不平衡、价差、成交密度、波动率、盘口真空等因子。
- 将原始行情事件转为因子事件。
- 保持纯计算，不触碰下单、不触碰账户状态。
- 加入单元测试和回放测试。

### Phase 3：第三层 decision

- 实现基于因子的决策模块。
- 输出标准化交易意图，而不是直接下单。
- 保留策略权重、置信度、冷却时间、去抖动机制。
- 接入回测或历史回放验证。
- 先以 shadow mode 运行，只记录信号，不执行。

### Phase 4：第四层 execution

- 将交易意图转换为订单计划。
- 接入 V1 exchange 下单能力，但通过适配器隔离。
- 将风控前置，包括最大仓位、最大亏损、最大订单频率、最大滑点、KillAll。
- 首先只允许模拟盘。
- 所有执行路径必须可审计。

### Phase 5：第五层 monitor

- 为 V2 增加独立指标：事件吞吐、channel 积压、数据延迟、因子计算耗时、决策次数、拒单次数、风控拦截次数、KillAll 状态。
- 将 V2 健康状态暴露给现有 API 或新增 `/api/v2/status`。
- 建立故障降级策略。

### Phase 6：Web/API V2 面板

- 不建议直接重写现有 Web。
- 先新增 V2 状态接口和简洁页面。
- 面板优先展示 ingestion 状态、因子状态、决策流、风控状态和订单执行状态。
- 后续再模块化拆分前端。

### Phase 7：回测与模拟盘验证

- 使用历史 order book 或 ticker/bar 回放验证 V2 因子和决策。
- shadow mode 对比 V1 策略输出。
- paper trading 至少运行 24h，再扩展到 72h。
- 验证 WebSocket 断线、OKX API 错误、订单部分成交、订单取消失败、行情延迟、channel 堵塞和风控触发等异常场景。

### Phase 8：灰度切换

- V2 初期只允许只读和模拟盘。
- 之后允许小资金、低频、单交易对灰度。
- 保留 V1 回退开关。
- 实盘前要求 KillAll、审计、指标、告警全部通过验收。

## 优先级建议

- P0：统一 Go 版本、Docker 端口、配置文件路径，避免环境不一致。
- P0：建立 `internal/v2/` 骨架和事件协议。
- P0：建立 V2 防腐适配器，不让 V2 直接耦合 V1 大对象。
- P1：完成 ingestion 最小可运行链路。
- P1：完成 computation 因子层和测试。
- P1：完成 shadow mode 决策链路。
- P2：接入模拟盘 execution。
- P2：完善 V2 API 和监控。
- P3：前端模块化和实盘灰度。

## 验收标准

```bash
# Build all packages
go build ./...

# Run vet checks
go vet ./...

# Run all tests
go test ./...

# Run V2 race tests
go test -race ./internal/v2/...
```

- V2 shadow mode 可连续运行 24 小时无 panic、无 goroutine 泄露、无明显 channel 堵塞。
- V2 paper mode 可连续运行 72 小时，风控、订单状态、审计记录完整。
- KillAll 可以在任意阶段中断 V2 执行链路。
- V1 和 V2 可以通过配置切换或并行运行。
- V2 任一层故障不会直接导致实盘错误下单。

## 最大风险

- 最大技术风险是直接拆 V1，导致交易执行链路回归。
- 最大架构风险是 V2 只复制 V1 目录，而没有真正建立事件边界。
- 最大交易风险是过早接入实盘 execution。
- 最大工程风险是缺少回放数据和模拟盘长期验证。
- 最大维护风险是 Web/API 继续与交易核心互相引用。

## 推荐下一步

先做 Phase 0，不碰实盘逻辑，不改 V1 核心执行路径。具体是建立 V2 目录、事件类型、层间接口、生命周期、配置开关和防腐适配器，然后跑通一个“OKX 行情输入到 V2 ingestion，再到日志/指标输出”的最小闭环。

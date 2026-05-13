# OKX 量化交易系统 V2 — 五层架构重构计划

## Context

当前系统是一个紧耦合的单体架构，核心问题：

| 文件 | 行数 | 问题 |
|------|------|------|
| `cmd/trader/main.go` | 1433 | God function，所有组件初始化、回调链组装、信号分发全在一个函数 |
| `internal/execution/execution.go` | 3112 | 执行引擎职责过重：下单、仓位管理、止盈、信号去重、再平衡熔断 |
| `internal/api/server.go` | 3441 | Web/API 层直接持有业务引用，34+ 个 handler 硬编码 |
| `web/static/js/app.js` | 3139 | 前端单文件，无模块化 |

**依赖链**（数据流向）：
```
OKX WS → exchange.SubscribeTicker/Bar/OrderBook
  → subscriptions.go → strategyEngine.OnTick/OnBar/OnOrderBook
    → executeSignal() → executionEngine.Execute()
      → riskEngine.CheckRisk() → exchange.PlaceOrder()
        → apiServer.UpdateStrategyStatus/AddSignal/AddOrder
          → WebSocket → 前端 app.js
```

**耦合点**：
- `main.go` 中 9 个策略通过闭包回调与 `executionEngine`、`riskEngine` 直接绑定
- `execution.Engine` 内部持有 `exchange.Exchange`、`risk.Engine`、`strategy.Engine` 三重引用
- `api.Server` 通过 `ActionHandlers` 闭包 + `Set*()` 方法注入全部业务依赖
- 策略通过 `SetSignalCallback(func)` 回调信号，与执行层紧耦合

**设计哲学**：不看涨跌，只看**流动性墙的坍缩与真空的填充**。

---

## 架构总览

```
[第五层] 监控与日志层 (pkg/monitor/)    ← 状态回传 channel
  ↑
[第四层] 风控与执行层 (pkg/execution/)   ← 交易指令 channel
  ↑
[第三层] 决策与大脑层 (pkg/decision/)    ← 因子数据 channel
  ↑
[第二层] 计算与衍生层 (pkg/computation/) ← 原始 Tick channel
  ↑
[第一层] 数据与感知层 (pkg/ingestion/)
```

**核心约束**：层与层之间通过 channel 通信，**禁止跨层直接调用**。

---

## 安全重构原则

### 反依赖失序三原则

1. **防腐层优先**：任何破坏性改动前，先在旧接口内部转发到新逻辑，确保外部调用不报错
2. **双轨并行**：新旧实现通过 feature flag 切换，旧实现保留至新实现验证通过
3. **逐层隔离**：每层独立可测试，通过 mock channel 验证，不依赖下游层就绪

### 验收铁律

每个步骤必须满足：
- `go build ./...` 通过
- `go vet ./...` 通过
- 已有测试不回归
- 新代码有对应测试
- 模拟盘可运行（如有功能变更）

---

## 各层详细设计

### 第一层：数据与感知层 (`internal/v2/ingestion/`)

**职责**：将 OKX WebSocket 原始数据翻译为标准化事件流。

| 组件 | 职责 | 输出 channel |
|------|------|-------------|
| `TickStream` | 连接 OKX `trades` channel，区分 Taker Buy/Sell | `chan TickEvent` |
| `OrderBookBuilder` | 连接 OKX `books` channel，增量更新 + checksum 校验 | `chan OrderBookEvent` |
| `KlineStream` | 连接 OKX `candle` channel，多周期 K 线 | `chan KlineEvent` |
| `ConnectionManager` | 断线重连 + 序列对齐 + context 生命周期 | 内部管理 |

**复用**：`internal/exchange/okx/ws_client.go` 的重连逻辑、心跳、订阅管理。

### 第二层：计算与衍生层 (`internal/v2/computation/`)

**职责**：将原始数据翻译为流动性力学因子。

| 组件 | 输入 | 输出 |
|------|------|------|
| `LiquidityWallDetector` | OrderBookEvent | `chan LiquidityWallEvent`（挂单高墙位置、厚度） |
| `VacuumZoneDetector` | OrderBookEvent | `chan VacuumZoneEvent`（流动性真空区） |
| `DissipationRate` | TickEvent + LiquidityWallEvent | `chan DissipationEvent`（吸收 vs 坍缩） |
| `MicroImbalance` | TickEvent | `chan ImbalanceEvent`（2s vs 10s 买盘失衡） |

### 第三层：决策与大脑层 (`internal/v2/decision/`)

**职责**：模糊评分器替代布尔逻辑，宏观约束微观。

| 组件 | 输入 | 输出 |
|------|------|------|
| `MacroStateMachine` | KlineEvent | `chan MacroState`（Trend_Up/Trend_Down/Momentum_Exhausted） |
| `FuzzyScorer` | DissipationEvent + ImbalanceEvent | `chan ScoreEvent`（多因子加权评分） |
| `ReflexiveFlipper` | MacroState + ScoreEvent | `chan SignalEvent`（反身性翻转信号） |

### 第四层：风控与执行层 (`internal/v2/execution/`)

**职责**：锁死人类弱点，利润子弹池 + 结构性止损。

| 组件 | 职责 |
|------|------|
| `ProfitPool` | BaseCapital 永不触碰，ProfitPool 用于加仓 |
| `BehavioralStopLoss` | 基于"流动性墙被击穿"等行为止损，非固定百分比 |
| `MakerFirstExecutor` | 限价单排队吃返佣，必要时拆单 |

**复用**：`internal/exchange/okx/order.go` 的下单 API。

### 第五层：监控与日志层 (`internal/v2/monitor/`)

**职责**：让人类看到机器在干什么。

| 组件 | 职责 |
|------|------|
| `MissedSignalLogger` | 价格大波动但无信号时 dump 所有因子 |
| `SnapshotArchiver` | 开平仓快照：orderBook 结构、失衡度 |
| `KillAllAPI` | HTTP 端点，一键清仓 |

### Web 面板 (`web/`)

**改造策略**：保留现有 UI 框架，替换数据源和图表。

| 模块 | 改动 |
|------|------|
| 流动性墙可视化 | 新增：实时显示挂单高墙和真空区 |
| 因子评分曲线 | 新增：ScoreEvent 实时折线图 |
| 策略面板 | 改造：从旧 9 策略改为单一流动性策略 |
| 信号记录 + 快照 | 新增：点击信号可查看开仓时的 orderBook 快照 |
| KillAll 按钮 | 新增：紧急熔断 |

### 代理与部署

| 组件 | 改动 |
|------|------|
| SOCKS5 代理 | V2 优先部署海外（RackNerd），不依赖代理 |
| Docker | 更新 Dockerfile，新增 V2 二进制 |
| systemd | 新增 service 文件，替代 nohup |
| K8s | 更新 deployment.yaml |

---

## 依赖关系分析

```
                    ┌─────────────┐
                    │  pkg/types  │  ← 全局共享，不可修改接口
                    └──────┬──────┘
                           │
        ┌──────────────────┼──────────────────┐
        │                  │                  │
  ┌─────▼─────┐    ┌──────▼──────┐    ┌──────▼──────┐
  │ ingestion  │    │ computation │    │   decision   │
  │  (第一层)  │───▶│  (第二层)   │───▶│   (第三层)   │
  └─────┬─────┘    └──────┬──────┘    └──────┬──────┘
        │                  │                  │
        │           ┌──────▼──────┐           │
        │           │  execution  │◀──────────┘
        │           │  (第四层)   │
        │           └──────┬──────┘
        │                  │
        │           ┌──────▼──────┐
        └──────────▶│   monitor   │
                    │  (第五层)   │
                    └─────────────┘
```

**破坏性依赖矩阵**（改动 A 会导致 B 报错）：

| 改动 | 直接破坏 | 间接破坏 |
|------|---------|---------|
| 修改 `types.Signal` 结构体 | strategy/*.go, execution.go, server.go | main.go, app.js |
| 修改 `strategy.Strategy` 接口 | 9 个策略实现, strategy.Engine | main.go, subscriptions.go |
| 修改 `exchange.Exchange` 接口 | okx/client.go, execution.go | risk.go, monitoring.go |
| 修改 `execution.Engine` API | main.go, server.go | app.js |
| 修改 `api.Server` 路由 | app.js（前端调用） | 无 |
| 修改 `config.Config` 结构体 | config.yaml*, main.go | 所有使用配置的组件 |
| 修改 WebSocket 消息格式 | app.js（前端解析） | 无 |

---

## 安全执行切片计划

### Phase 0: 基础设施准备（无破坏性改动）

#### 步骤 0.1：创建 V2 目录结构 + 事件类型定义

**具体动作**：
- 创建 `internal/v2/` 目录结构：`ingestion/`, `computation/`, `decision/`, `execution/`, `monitor/`
- 创建 `internal/v2/events/` 包，定义所有 channel 事件类型（`TickEvent`, `OrderBookEvent`, `SignalEvent` 等）
- 创建 `internal/v2/lifecycle/` 包，定义 context 生命周期管理器

**目的**：建立 V2 骨架，零侵入现有代码

**影响面**：仅新增文件，不影响任何现有代码

**验收标准**：
- `go build ./...` 通过
- `go vet ./...` 通过
- `go test ./internal/v2/...` 通过（即使只有空测试）

---

#### 步骤 0.2：定义 V2 核心接口（在 `internal/v2/` 内）

**依赖前提**：步骤 0.1 验收通过

**具体动作**：
- 定义 `v2/ingestion.DataProvider` 接口（替代直接依赖 `exchange.Exchange`）
- 定义 `v2/decision.SignalGenerator` 接口（替代 `strategy.Strategy`）
- 定义 `v2/execution.OrderExecutor` 接口（替代 `execution.Engine` 的下单方法）
- 定义 `v2/monitor.StatusReporter` 接口（替代 `api.Server` 的状态更新方法）

**目的**：V2 各层依赖抽象接口，不依赖 V1 实现

**影响面**：仅新增文件，不影响任何现有代码

**验收标准**：
- `go build ./...` 通过
- 接口定义清晰，每个接口 1-3 个方法

---

#### 步骤 0.3：为 V1 核心接口编写适配器（防腐层）

**依赖前提**：步骤 0.2 验收通过

**具体动作**：
- 创建 `internal/v2/adapter/exchange_adapter.go`：实现 `v2/ingestion.DataProvider`，内部调用 V1 `exchange.Exchange`
- 创建 `internal/v2/adapter/strategy_adapter.go`：实现 `v2/decision.SignalGenerator`，内部调用 V1 `strategy.Strategy`
- 创建 `internal/v2/adapter/execution_adapter.go`：实现 `v2/execution.OrderExecutor`，内部调用 V1 `execution.Engine`

**目的**：V2 代码可以通过适配器调用 V1 实现，实现双轨并行

**影响面**：仅新增文件，不影响任何现有代码

**验收标准**：
- `go build ./...` 通过
- 适配器有单元测试，验证 V1→V2 桥接正确

---

### Phase 1: 第一层（数据与感知层）

#### 步骤 1.1：实现 TickStream（OKX trades channel）

**依赖前提**：步骤 0.3 验收通过

**具体动作**：
- 实现 `v2/ingestion.TickStream`，连接 OKX `trades` WebSocket
- 区分 Taker Buy / Taker Sell
- 通过 `chan events.TickEvent` 输出
- 使用 context 管理生命周期

**目的**：V2 第一层最小可运行组件

**影响面**：仅新增文件

**验收标准**：
- 单元测试：mock WebSocket 输入，验证 TickEvent 输出正确
- 集成测试：连接 OKX 测试网，验证实时数据接收

---

#### 步骤 1.2：实现 OrderBookBuilder（OKX books channel）

**依赖前提**：步骤 1.1 验收通过

**具体动作**：
- 实现 `v2/ingestion.OrderBookBuilder`，连接 OKX `books` WebSocket
- 增量更新 + checksum 校验
- 使用 `sync.Pool` 优化内存分配
- 每 50ms 通过 `chan events.OrderBookEvent` 输出

**目的**：完成第一层数据采集

**影响面**：仅新增文件

**验收标准**：
- 单元测试：模拟增量更新，验证 checksum 校验
- 集成测试：连接 OKX 测试网，验证 OrderBook 完整性

---

#### 步骤 1.3：实现 ConnectionManager（断线重连 + 序列对齐）

**依赖前提**：步骤 1.2 验收通过

**具体动作**：
- 实现 `v2/ingestion.ConnectionManager`
- 管理 TickStream 和 OrderBookBuilder 的生命周期
- 断线重连后校验序列连续性
- 重连时 buffer 数据，杜绝"未来函数"

**目的**：第一层具备生产级稳定性

**影响面**：仅新增文件

**验收标准**：
- 单元测试：模拟断线，验证重连 + 序列对齐
- 压力测试：频繁断线重连不丢数据

---

#### 步骤 1.4：第一层集成验证 + 可观测输出

**依赖前提**：步骤 1.3 验收通过

**具体动作**：
- 创建 `cmd/v2-ingestion-test/main.go`：启动第一层，打印接收到的 Tick 和 OrderBook
- 添加 Prometheus 指标：tick_count、orderbook_update_count、reconnect_count
- 添加日志：每 10s 打印一次数据接收统计

**目的**：第一层可独立运行和验证

**影响面**：仅新增文件

**验收标准**：
- `go run cmd/v2-ingestion-test/main.go` 能实时打印 OKX 数据
- Prometheus 指标可查询

---

### Phase 2: 第二层（计算与衍生层）

#### 步骤 2.1：实现 LiquidityWallDetector

**依赖前提**：步骤 1.4 验收通过

**具体动作**：
- 实现 `v2/computation.LiquidityWallDetector`
- 从 `chan events.OrderBookEvent` 读取
- 扫描上下 N 档，找出显著大于平均挂单量的价位
- 识别整数关口、巨量限价单
- 通过 `chan events.LiquidityWallEvent` 输出

**目的**：核心因子之一 — 流动性墙识别

**影响面**：仅新增文件

**验收标准**：
- 单元测试：构造 OrderBook 数据，验证墙检测算法
- 输出包含：墙位置、墙厚度、与当前价距离

---

#### 步骤 2.2：实现 VacuumZoneDetector

**依赖前提**：步骤 2.1 验收通过

**具体动作**：
- 实现 `v2/computation.VacuumZoneDetector`
- 计算挂单密度，找出低流动性区间
- 通过 `chan events.VacuumZoneEvent` 输出

**目的**：核心因子之二 — 流动性真空区识别

**影响面**：仅新增文件

**验收标准**：
- 单元测试：构造稀疏 OrderBook，验证真空区检测

---

#### 步骤 2.3：实现 DissipationRate + MicroImbalance

**依赖前提**：步骤 2.2 验收通过

**具体动作**：
- 实现 `v2/computation.DissipationRate`：主动成交量 / 墙的挂单厚度
- 实现 `v2/computation.MicroImbalance`：过去 2s 主动买盘 vs 过去 10s 平均值
- 两者都从 `chan events.TickEvent` 读取

**目的**：完成第二层全部因子计算

**影响面**：仅新增文件

**验收标准**：
- 单元测试：构造 Tick 序列，验证因子计算正确性
- 因子值在合理范围内（0-100 或 -1 到 1）

---

#### 步骤 2.4：第二层集成验证

**依赖前提**：步骤 2.3 验收通过

**具体动作**：
- 扩展 `cmd/v2-ingestion-test/` 或新建 `cmd/v2-pipeline-test/`
- 连接第一层输出到第二层输入
- 打印实时因子数据

**目的**：第一层 + 第二层 pipeline 可运行

**影响面**：仅新增文件

**验收标准**：
- 实时输出：流动性墙位置、耗散速率、微观失衡度
- 延迟 < 10ms（从 Tick 到因子输出）

---

### Phase 3: 第三层（决策与大脑层）

#### 步骤 3.1：实现 MacroStateMachine

**依赖前提**：步骤 2.4 验收通过

**具体动作**：
- 实现 `v2/decision.MacroStateMachine`
- 独立协程跑 1H/4H 级别 K 线
- 状态：Trend_Up, Trend_Down, Momentum_Exhausted
- 通过 `chan events.MacroState` 输出

**目的**：宏观约束微观的基础

**影响面**：仅新增文件

**验收标准**：
- 单元测试：构造 K 线序列，验证状态转换逻辑
- 状态转换有日志记录

---

#### 步骤 3.2：实现 FuzzyScorer

**依赖前提**：步骤 3.1 验收通过

**具体动作**：
- 实现 `v2/decision.FuzzyScorer`
- Score = 流动性墙变化权重*40 + 耗散速率权重*30 + 失衡度权重*30
- 阈值触发信号（Score >= 75 → 开仓信号）
- 通过 `chan events.ScoreEvent` 输出

**目的**：模糊评分器替代布尔逻辑

**影响面**：仅新增文件

**验收标准**：
- 单元测试：构造因子输入，验证评分计算
- 阈值可配置

---

#### 步骤 3.3：实现 ReflexiveFlipper

**依赖前提**：步骤 3.2 验收通过

**具体动作**：
- 实现 `v2/decision.ReflexiveFlipper`
- Trend_Down + 买盘激增 → 做空信号
- Trend_Up + 卖盘激增 → 做多信号
- 通过 `chan events.SignalEvent` 输出

**目的**：反身性翻转逻辑

**影响面**：仅新增文件

**验收标准**：
- 单元测试：构造宏观状态 + 评分输入，验证翻转逻辑
- 信号包含：方向、置信度、触发因子

---

#### 步骤 3.4：第三层集成验证

**依赖前提**：步骤 3.3 验收通过

**具体动作**：
- 连接第二层输出到第三层输入
- 打印实时信号（如果有）

**目的**：前三层 pipeline 可运行

**影响面**：仅新增文件

**验收标准**：
- 无信号时打印 "等待信号"
- 有信号时打印完整信号信息

---

### Phase 4: 第四层（风控与执行层）

#### 步骤 4.1：实现 ProfitPool

**依赖前提**：步骤 3.4 验收通过

**具体动作**：
- 实现 `v2/execution.ProfitPool`
- BaseCapital 永不触碰
- ProfitPool 用于加仓，亏损只扣利润
- 利润池归零后只能使用基础仓位

**目的**：资金管理核心

**影响面**：仅新增文件

**验收标准**：
- 单元测试：模拟盈亏序列，验证资金分配正确
- BaseCapital 永不减少

---

#### 步骤 4.2：实现 BehavioralStopLoss

**依赖前提**：步骤 4.1 验收通过

**具体动作**：
- 实现 `v2/execution.BehavioralStopLoss`
- 开空后价格击穿上方流动性墙 → 平仓
- 主动买盘指数级爆发 → 平仓
- 不使用固定百分比止损

**目的**：结构性行为止损

**影响面**：仅新增文件

**验收标准**：
- 单元测试：构造墙击穿场景，验证止损触发
- 止损原因记录到日志

---

#### 步骤 4.3：实现 MakerFirstExecutor

**依赖前提**：步骤 4.2 验收通过

**具体动作**：
- 实现 `v2/execution.MakerFirstExecutor`
- 优先挂限价单吃返佣
- 必要时拆单算法，避免自吃流动性
- 通过适配器调用 V1 `exchange.PlaceOrder`

**目的**：执行层落地

**影响面**：仅新增文件

**验收标准**：
- 单元测试：验证限价单优先逻辑
- 集成测试：通过适配器在模拟盘下单

---

#### 步骤 4.4：第四层集成验证 + 模拟盘测试

**依赖前提**：步骤 4.3 验收通过

**具体动作**：
- 连接第三层输出到第四层输入
- 在模拟盘运行完整 pipeline
- 验证：Tick → 因子 → 评分 → 信号 → 下单 全链路

**目的**：前四层 pipeline 可在模拟盘运行

**影响面**：仅新增文件

**验收标准**：
- 模拟盘成功下单
- ProfitPool 资金计算正确
- BehavioralStopLoss 能触发平仓

---

### Phase 5: 第五层（监控与日志层）

#### 步骤 5.1：实现 MissedSignalLogger + SnapshotArchiver

**依赖前提**：步骤 4.4 验收通过

**具体动作**：
- 实现 `v2/monitor.MissedSignalLogger`：价格大波动但无信号时 dump 所有因子
- 实现 `v2/monitor.SnapshotArchiver`：开平仓时保存 orderBook 结构、失衡度

**目的**：信号质量可观测

**影响面**：仅新增文件

**验收标准**：
- missed_signals.log 有输出
- 快照文件可读取

---

#### 步骤 5.2：实现 KillAllAPI

**依赖前提**：步骤 5.1 验收通过

**具体动作**：
- 实现 `v2/monitor.KillAllAPI`：HTTP 端点
- 接收 KillAll 指令后：
  1. 停止所有策略信号生成
  2. 平掉所有仓位
  3. 取消所有挂单
  4. 推送通知到所有渠道

**目的**：人工干预熔断

**影响面**：仅新增文件

**验收标准**：
- HTTP POST `/api/v2/kill-all` 触发清仓
- 模拟盘验证：所有仓位被平掉

---

#### 步骤 5.3：第五层集成验证

**依赖前提**：步骤 5.2 验收通过

**具体动作**：
- 完整五层 pipeline 集成测试
- `go test -race ./internal/v2/...` 全部通过
- 模拟盘运行 24 小时

**目的**：V2 核心 pipeline 完整可运行

**影响面**：仅新增文件

**验收标准**：
- 无 race condition
- 模拟盘 24 小时无 panic
- missed_signals.log 有合理输出

---

### Phase 6: Web 面板适配

#### 步骤 6.1：新增 V2 API 路由（防腐层）

**依赖前提**：步骤 5.3 验收通过

**具体动作**：
- 在 `internal/api/server.go` 的 `setupRoutes()` 中新增 `/api/v2/` 前缀路由
- V2 路由读取 V2 pipeline 的状态数据
- V1 路由保持不变

**目的**：V2 数据通过新 API 暴露，V1 API 不受影响

**影响面**：`internal/api/server.go` 新增路由（不修改现有路由）

**验收标准**：
- `/api/v2/status` 返回 V2 pipeline 状态
- `/api/status` 仍返回 V1 状态
- 两个 API 互不干扰

---

#### 步骤 6.2：前端新增 V2 面板页面

**依赖前提**：步骤 6.1 验收通过

**具体动作**：
- 新增 `web/v2.html`：V2 专用面板
- 新增 `web/static/js/v2-app.js`：V2 专用 JS
- 新增 `web/static/css/v2-style.css`：V2 专用样式
- 在 `index.html` 顶部导航添加"V2 面板"链接

**目的**：V2 面板独立于 V1 面板

**影响面**：`web/index.html` 新增一个链接（不修改现有功能）

**验收标准**：
- 访问 `/v2.html` 显示 V2 面板
- 访问 `/` 仍显示 V1 面板
- 两个面板互不干扰

---

#### 步骤 6.3：V2 面板功能实现

**依赖前提**：步骤 6.2 验收通过

**具体动作**：
- 流动性墙实时可视化（D3.js 或 Canvas）
- 因子评分实时折线图
- 信号触发记录 + 快照查看
- KillAll 紧急按钮
- ProfitPool 资金状态

**目的**：V2 面板完整可用

**影响面**：仅新增文件

**验收标准**：
- 流动性墙可视化与 OrderBook 数据一致
- 因子曲线实时更新
- KillAll 按钮触发清仓

---

### Phase 7: 部署与切换

#### 步骤 7.1：Docker + systemd 配置

**依赖前提**：步骤 5.3 验收通过

**具体动作**：
- 更新 `Dockerfile`：构建 V2 二进制
- 新增 `deployments/systemd/okx-quant-v2.service`
- 更新 `deployments/k8s/deployment.yaml`：添加 V2 container

**目的**：V2 可独立部署

**影响面**：新增部署配置

**验收标准**：
- Docker 镜像构建成功
- systemd service 启动正常

---

#### 步骤 7.2：双轨运行（V1 + V2 并行）

**依赖前提**：步骤 7.1 验收通过

**具体动作**：
- 在 `main.go` 中添加 `--mode` 参数：`v1`（默认）、`v2`、`dual`
- `dual` 模式：V1 和 V2 同时运行，V2 信号仅记录不下单
- 对比 V1 和 V2 的信号质量

**目的**：安全验证 V2 策略逻辑

**影响面**：`cmd/trader/main.go` 新增模式判断（V1 逻辑不变）

**验收标准**：
- `--mode=v1` 行为与当前完全一致
- `--mode=v2` 运行 V2 pipeline
- `--mode=dual` 两者并行，V2 仅记录

---

#### 步骤 7.3：V2 独立运行 + V1 下线

**依赖前提**：步骤 7.2 运行 1-2 周无问题

**具体动作**：
- 将 `--mode` 默认值改为 `v2`
- 删除 V1 策略代码（`internal/strategy/` 中的旧策略）
- 删除 V1 执行引擎中的旧逻辑
- 清理 `main.go` 中的旧初始化代码

**目的**：完成迁移，V1 退役

**影响面**：删除旧代码

**验收标准**：
- `go build ./...` 通过
- 模拟盘运行正常
- 所有 V1 代码已清理

---

### Phase 8: 代理与网络优化

#### 步骤 8.1：海外服务器直连优化

**依赖前提**：Phase 7 完成

**具体动作**：
- 优化 `ConnectionManager`：优先直连 OKX，备用代理
- 实现智能路由：海外服务器直连，国内服务器走代理
- 代理健康检查 + 自动切换

**目的**：消除 SOCKS5 代理不稳定问题

**影响面**：`internal/v2/ingestion/` 内部优化

**验收标准**：
- 海外服务器直连延迟 < 50ms
- 代理断开时自动切换

---

#### 步骤 8.2：双服务器部署

**依赖前提**：步骤 8.1 验收通过

**具体动作**：
- RackNerd（海外）：主运行实例，直连 OKX
- 腾讯云（国内）：备用实例，通过 RackNerd 代理
- 配置同步机制

**目的**：高可用部署

**影响面**：部署配置

**验收标准**：
- 两台服务器都能运行
- 主服务器故障时可切换到备用

---

## 从旧项目吸取的经验教训

| 旧项目坑 | V2 应对 |
|----------|---------|
| main.go 1429 行 God function | 每层独立初始化，main.go 仅组装 channel 链 |
| execution.go 3112 行职责过重 | 拆分为 ProfitPool、BehavioralStop、MakerExecutor |
| 9 个策略仅 2 个产生信号 | 只保留 1 个核心策略（流动性墙坍缩） |
| 策略通过回调紧耦合 | channel 通信，层间无直接引用 |
| SOCKS5 代理不稳定 | 优先海外直连，代理仅作备用 |
| nohup 前台运行 | systemd service + Docker |
| SmartFilter 数据源缺失 | 不使用 SmartFilter（不符合流动性哲学） |
| MeanReversion 被去重拦截 | V2 无信号去重逻辑（channel 天然有序） |
| WS 断连导致策略收不到数据 | ConnectionManager 内置重连 + 数据 buffer |

---

## 旧项目可复用组件

| 组件 | 复用方式 | 说明 |
|------|---------|------|
| `exchange/okx/ws_client.go` | **部分复用** | 重连逻辑、心跳、订阅管理 |
| `exchange/okx/rest_client.go` | **复用** | 下单 API、账户查询 |
| `exchange/okx/order.go` | **复用** | PlaceOrder、CancelOrder |
| `config/config.go` | **复用** | 配置加载，扩展 V2 配置段 |
| `pkg/logger/` | **复用** | 直接使用 |
| `pkg/types/` | **扩展复用** | 保留现有类型，新增 V2 事件类型 |
| `storage/` | **复用** | SQLite 快照存储 |
| `notifications/` | **复用** | 钉钉/企微/Telegram 通知 |
| `web/` | **改造复用** | 保留 UI 框架，替换数据源 |
| `indicator/` | **不复用** | V2 用流动性因子替代技术指标 |
| `strategy/` | **不复用** | V2 只用流动性墙坍缩策略 |
| `llmanalysis/` | **不复用** | 违背"纯数学、无机器学习"原则 |
| `backtest/` | **改造复用** | 适配 V2 因子输入 |

---

## 风险与缓解

| 风险 | 等级 | 缓解 |
|------|------|------|
| 五层 channel 通信延迟过高 | 高 | 无缓冲 channel + select，延迟 < 10ms |
| OKX trades channel 数据量过大 | 中 | sync.Pool + 环形缓冲区 |
| 重构周期过长 | 中 | 按 Phase 逐步交付，每 Phase 有可运行程序 |
| 新策略信号过少 | 中 | 先放宽评分阈值，模拟盘期间逐步收紧 |
| V1→V2 切换期间业务中断 | 高 | 双轨并行 + feature flag，V1 保留至 V2 稳定 |
| Web 面板改造影响现有功能 | 中 | V2 面板独立页面，不修改 V1 页面 |
| 部署环境差异 | 中 | Docker 标准化，双服务器验证 |

---

## 验证方案

### 单元测试
- 每层独立测试，使用 mock channel 输入/输出
- `go test -race ./internal/v2/...` 必须通过
- 覆盖率 >= 80%

### 集成测试
- 启动完整五层 pipeline（连接 OKX 测试环境）
- 验证：Tick → OrderBook → 因子 → 评分 → 信号 全链路通畅
- 验证：KillAll 端点能立即终止所有协程

### 模拟盘验证
- 在 OKX 模拟盘运行 1-2 周
- 检查 missed_signals.log，评估信号遗漏率
- 检查快照存证，验证开平仓逻辑
- 对比 V1 和 V2 的信号质量

### 性能验证
- Tick 处理延迟 < 10ms
- OrderBook 更新延迟 < 50ms
- 信号生成延迟 < 100ms
- 内存占用 < 500MB

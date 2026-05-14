# 流动性策略实施文档

## 目标

本文档将对话导出中的交易思想整理为可工程化、可回测、可状态机化的 V2 策略实施规格。当前阶段只定义策略、因子、状态机、风控和验证路径，不生产业务代码。

核心交易哲学：不预测涨跌，只响应流动性墙、订单流失衡、结构证伪与风险边界。

## 架构对齐

策略落地应与 `docs/V2_ARCHITECTURE_PLAN.md` 的五层架构保持一致。

| 层级 | 策略职责 | 关键产物 |
|------|----------|----------|
| 第一层 ingestion | 接入逐笔成交、L2 OrderBook、K 线和连接状态 | `TickEvent`、`OrderBookEvent`、`KlineEvent` |
| 第二层 computation | 将原始行情转换为流动性与订单流因子 | 流动性墙、真空区、Delta、OBI、耗散速率、价差状态 |
| 第三层 decision | 用宏观状态机和微观评分器输出交易意图 | 策略状态、评分、开平仓候选、禁用原因 |
| 第四层 execution | 执行结构性止损、利润子弹池、滑点控制 | 订单计划、仓位、止损线、风控拦截 |
| 第五层 monitor | 记录因子快照、错失行情和执行审计 | `missed_signals.log`、信号快照、复盘事件 |

## 数据需求

### 必需数据

| 数据 | 用途 | 最低要求 |
|------|------|----------|
| 逐笔成交 | 区分主动买入、主动卖出和成交密度 | 毫秒级时间戳、价格、数量、方向 |
| L2 OrderBook | 识别流动性墙、真空区、补单和撤单 | 至少前 20 档，支持增量重建和 checksum |
| 多周期 K 线 | 判断宏观趋势和动能衰减 | 1m、5m、1H、4H |
| 价差 | 过滤猎杀阶段和滑点恶化 | bid/ask 实时价差 |
| 账户与仓位 | 控制风险暴露和利润池 | 可用余额、持仓、未实现盈亏 |

### 可选数据

| 数据 | 用途 |
|------|------|
| 资金费率 | Delta 中性资金费率套利 |
| Open Interest | 辅助判断杠杆拥挤和清算风险 |
| 大额成交标记 | 辅助识别猎杀和机构吸收 |
| 链上数据 | 作为低频宏观过滤器 |

## 因子定义

### 流动性墙

流动性墙是关键价位附近 N 档挂单量显著高于邻近区间的区域。

```text
WallSize(price_range) = sum(orderbook.quantity within price_range)
WallDetected = WallSize >= rolling_mean(WallSize) + k * rolling_std(WallSize)
```

用于识别前高、前低、整数关口、密集成交区附近的潜在吸收区。

### 流动性真空

流动性真空是某个价格方向上挂单厚度显著不足的区域。

```text
VacuumScore = 1 - depth_near_path / normal_depth
```

当墙坍缩且真空区出现时，价格容易沿阻力最小路径快速移动。

### 订单簿不平衡 OBI

```text
OBI = (BidDepth_N - AskDepth_N) / (BidDepth_N + AskDepth_N)
```

OBI 用于衡量 N 档内买卖挂单的静态不平衡。OBI 只能描述盘口形态，需要结合主动成交和补单变化使用。

### Delta

```text
Delta = AggressiveBuyVolume - AggressiveSellVolume
```

滚动 Delta 用于识别主动买卖压力。持续偏离中性区间，并超过同波动率历史均值约 2 倍时，可视为失衡显现候选。

### WallAbsorbRate

```text
WallAbsorbRate = AggressiveVolumeAtWall / max(WallSizeDecrease, epsilon)
```

该因子衡量主动单进入后，墙是被真实吃掉、被快速补上，还是主动单被墙吸收。

### WallDelta

```text
delta_wall = current_wall - previous_wall
WallCollapse = delta_wall < -collapse_threshold
```

当 `delta_wall` 快速为负，说明原有墙撤单或被吃穿，可能产生动态阻力坍缩。

### 主动消耗与被动补充背离

```text
aggressive_buy_volume = sum(market_buy_trades_hit_wall)
passive_add_volume = sum(limit_sell_orders_added_to_wall)
BuyWallPressure = aggressive_buy_volume / max(passive_add_volume, epsilon)
```

当主动消耗明显大于被动补充时，墙可能被突破；当主动消耗强但墙厚度维持或增加时，可能是吸收陷阱。

### 价差稳定性

```text
SpreadStable = Spread <= normal_spread * 1.5 for 3s-5s
```

价差稳定用于过滤猎杀阶段。猎杀过程中的价差扩张、流动性断层和成交冲击会显著放大滑点。

## 通用状态机

策略应通过状态机表达，避免一次性布尔条件触发。

| 状态 | 含义 | 是否允许开仓 |
|------|------|--------------|
| `Idle` | 无有效结构 | 否 |
| `Sweeping_Hunting` | 插针、扫止损、猎杀正在发生 | 否 |
| `Consolidation_Setting` | 猎杀结束后盘口重新稳定 | 否 |
| `Imbalance_Confirmed` | 方向性失衡显现 | 是 |
| `Position_Open` | 持仓中 | 按风控管理 |
| `Invalidated` | 结构证伪或风控触发 | 否 |
| `Cooldown` | 冷却期 | 否 |

### 状态转换

```text
Idle -> Sweeping_Hunting
Condition: price breaks recent high/low with spread expansion and aggressive volume spike

Sweeping_Hunting -> Consolidation_Setting
Condition: aggressive flow decays, price exits vacuum zone, spread stays stable for 3s-5s

Consolidation_Setting -> Imbalance_Confirmed
Condition: rolling Delta deviates, passive walls rebuild asymmetrically, rejection test succeeds

Imbalance_Confirmed -> Position_Open
Condition: score >= open_threshold and risk budget is available

Position_Open -> Invalidated
Condition: structural stop, hard stop, spread deterioration, time stop, or kill switch

Invalidated -> Cooldown
Condition: position is closed and event snapshot is archived

Cooldown -> Idle
Condition: cooldown duration expires and market structure resets
```

状态应持久化，至少记录交易对、时间戳、状态、关键价位、核心因子快照和切换原因。

## 策略卡片

### LQT_Down01：上方流动性墙陷阱做空

| 项目 | 说明 |
|------|------|
| 假说 | 宏观下跌趋势中，价格反弹到上方流动性墙，主动买盘强但无法推穿墙体，墙表现为吸收，反转做空具备正期望 |
| 市场状态 | `Trend_Down && Momentum_Decay` |
| 触发区域 | 前高、整数关口或密集成交区上方 N 档流动性墙 |
| 关键因子 | OBI 偏买、WallAbsorbRate 低、主动买盘衰减、价差恢复、上方墙未坍缩 |
| 入场 | 价格进入墙附近，评分达到阈值，出现微观拒绝测试 |
| 止损 | 价格突破墙上边界并维持 N 秒，且主动买持续进入 |
| 止盈 | 下方真空被填充、Delta 转中性、卖压衰减或到达结构目标位 |
| 仓位 | 默认使用 `BaseCapital * 0.5%-1%` 风险，高评分只允许动用 `ProfitPool` 加仓 |

评分建议：

```text
Score = w1 * TrendDownScore
      + w2 * MomentumDecayScore
      + w3 * WallAbsorptionScore
      + w4 * BuyExhaustionScore
      + w5 * SpreadStabilityScore
      - w6 * BreakoutStrengthScore
```

### LQT_REV01：流动性猎杀后反转

| 项目 | 说明 |
|------|------|
| 假说 | 价格刺破前高或前低后触发止损和追单，若主动单枯竭且价格收回关键位，猎杀后的反转具备交易价值 |
| 禁止动作 | 猎杀阶段追单 |
| 等待条件 | `Sweeping_Hunting -> Consolidation_Setting` |
| 确认条件 | Delta 极值回落、价差恢复、价格回到关键位内侧、反向主动单接管 |
| 入场 | `Imbalance_Confirmed` 后顺反转方向开仓 |
| 止损 | 插针极值外侧加结构缓冲，或再次进入真空并维持 N 秒 |
| 退出 | 到达前一流动性池、订单流衰减、时间止盈或硬风险触发 |

该策略适合作为双插针后的主状态机策略。

### LQT_MOM01：动态阻力坍缩顺势

| 项目 | 说明 |
|------|------|
| 假说 | 支撑或阻力墙突然撤单或被吃穿后，价格会沿流动性真空快速移动 |
| 触发 | `delta_wall < -collapse_threshold` 且同方向真空区扩大 |
| 确认 | 主动成交与墙坍缩方向一致，补单速度低于消耗速度 |
| 入场 | 回踩坍缩墙附近未恢复厚度，或突破后短暂整理继续失衡 |
| 止损 | 墙体重新建立、Delta 反向、价差恶化 |
| 适用 | 快速行情、突破延续、清算后单边延伸 |

### LQT_DIV01：主动消耗与被动补充背离

| 项目 | 说明 |
|------|------|
| 假说 | 主动单强度和被动补单速度的背离，能提前识别墙被吃穿或吸收陷阱 |
| 突破版本 | 主动消耗持续大于被动补充，墙厚度下降 |
| 反转版本 | 主动消耗强但墙厚度维持或增加，主动方被吸收 |
| 入场 | 与宏观状态和价差稳定共同确认 |
| 止损 | 背离关系消失，墙体行为反转 |

该策略更适合作为 LQT_Down01 和 LQT_MOM01 的底层确认模块。

### LQT_STAIR_SHORT01：阶梯式反弹做空

| 项目 | 说明 |
|------|------|
| 假说 | 大周期下跌中，小周期反弹末端出现买盘枯竭和卖单攻击，适合分层做空 |
| 宏观过滤 | 4H/1H 下跌趋势，反弹未破结构高点 |
| 微观条件 | 买盘越来越少、上方墙吸收、反向卖单接管 |
| 入场 | 第一笔轻仓，确认后按利润池或风险预算阶梯加仓 |
| 禁用 | 下跌伴随卖单越来越少、缺乏反弹结构、进入流动性真空失重状态 |
| 止损 | 最近反弹结构高点、上方墙被有效突破、硬风险上限 |

### LQT_NEUTRAL01：资金费率 Delta 中性

| 项目 | 说明 |
|------|------|
| 假说 | 使用现货和合约反向仓保持 Delta 近似 0，以资金费率作为主要收益来源 |
| 数据 | 资金费率、现货价格、合约价格、借贷或保证金成本 |
| 入场 | 资金费率显著高于成本和滑点，且可稳定对冲 |
| 退出 | 资金费率回落、对冲成本上升、价差异常或风险预算不足 |
| 风控 | 保证金率、强平距离、交易所风险、对冲偏离 |

该策略与订单流方向策略应使用独立风险预算。

## 缠论降维过滤器

缠论概念只作为工程化过滤器使用。

| 缠论概念 | 工程化解释 | 落地方式 |
|----------|------------|----------|
| 中枢 | 流动性反复交换的价格区间 | 成交密集区、墙反复重建区 |
| 背驰 | 推动力边际递减 | Delta、成交密度、WallAbsorbRate 衰减 |
| 走势闭环 | 状态机阶段变化 | `Sweeping -> Setting -> Imbalance` |
| 级别 | 多周期上下文 | 4H/1H 宏观，1m/5m 微观 |

该过滤器用于减少逆大周期交易和低质量信号。

## 风控规则

### 结构性止损

止损优先使用策略假说证伪线。

| 场景 | 止损依据 |
|------|----------|
| 做空墙吸收 | 墙上边界有效突破并维持 N 秒 |
| 猎杀反转 | 插针极值外侧加结构缓冲 |
| 坍缩顺势 | 坍缩墙重新建立或价格回到墙内侧 |
| 阶梯做空 | 最近反弹结构高点被有效突破 |

### 硬风险上限

结构性止损之外，必须保留硬风险上限。

| 风险项 | 建议 |
|--------|------|
| 单笔最大亏损 | `BaseCapital * 0.5%-1%` |
| 单日最大亏损 | 达到阈值后进入只读观察 |
| 单周最大亏损 | 触发后禁用实盘策略并复盘 |
| 单策略最大亏损 | 禁用对应策略，不影响其他策略观察 |
| 最大滑点 | 超过阈值取消或降级为观察 |
| 最大连续亏损 | 进入冷却并降低交易频率 |

### 利润子弹池

```text
BaseCapital = initial_principal
ProfitPool = realized_profit_after_withdrawal_rules
BaseRisk = BaseCapital * risk_percent
ExtraRisk = min(ProfitPool * profit_risk_percent, signal_quality_budget)
```

原则：基础仓只由本金风险预算决定，额外加仓只来自已实现利润池。利润池只能用于高评分、低频、结构完整的信号。

### 禁用条件

| 条件 | 动作 |
|------|------|
| 价差持续扩大 | 禁止开仓 |
| 数据延迟或 OrderBook 序列错乱 | 禁止开仓 |
| 猎杀阶段仍在进行 | 禁止开仓 |
| 趋势方向与策略假说冲突 | 降权或禁用 |
| 同交易对短时间连续止损 | 进入冷却 |
| 人工干预未记录原因 | 禁止后续自动加仓 |

## 回测与影子模式

### 回测要求

| 项目 | 要求 |
|------|------|
| 数据粒度 | 至少逐笔成交 + L2 OrderBook 回放 |
| 成本模型 | 手续费、滑点、Maker/Taker、撤单失败 |
| 事件回放 | 按时间顺序重建状态机 |
| 指标 | 胜率、盈亏比、最大回撤、交易频率、错失行情数 |
| 分桶 | 按波动率、交易时段、趋势状态、资金费率分组 |

### 影子模式

影子模式只记录信号，不下单。

每个候选信号应记录：

```text
timestamp
symbol
strategy_id
state_before
state_after
macro_state
factor_snapshot
score
open_threshold
reject_reason
expected_entry
structural_stop
post_5m_return
post_15m_return
post_1h_return
```

### 错失行情记录器

当价格发生大幅移动但系统无信号时，应记录完整因子快照。

```text
missed_signal_condition = abs(return_N) >= large_move_threshold and no_signal_emitted
```

记录目的：判断是策略刻意过滤，还是因子缺失、阈值过严或状态机滞后。

## 实施优先级

### P0：只读因子与日志

- 接入逐笔成交和 L2 OrderBook 回放能力。
- 计算流动性墙、真空区、OBI、Delta、WallDelta、价差稳定性。
- 建立因子快照和错失行情日志。
- 只跑历史回放和 shadow mode。

### P1：状态机与评分器

- 实现通用状态机。
- 实现 LQT_Down01、LQT_REV01、LQT_MOM01 的评分模型。
- 输出标准化交易意图，不接实盘下单。
- 建立状态持久化和复盘页面数据源。

### P2：模拟盘执行

- 接入结构性止损和硬风险上限。
- 接入利润子弹池模型。
- 在 paper mode 连续运行 72 小时。
- 验证断线、延迟、滑点、撤单失败和 KillAll。

### P3：小资金灰度

- 单交易对、低频、低风险预算启动。
- 初期只启用评分最高的一到两个策略。
- 每日复盘 shadow 信号、实盘信号和错失行情。
- 达到风控触发条件后回到 shadow mode。

## 验收标准

| 阶段 | 验收标准 |
|------|----------|
| 因子层 | 历史回放中因子无明显跳变，OrderBook 重建无序列错误 |
| 状态机 | 每次状态转换都有原因、时间戳和因子快照 |
| 策略层 | 每个信号能解释假说、入场、止损和退出条件 |
| 回测层 | 结果包含成本、滑点、最大回撤和分桶表现 |
| 影子模式 | 连续 24 小时无 panic、无明显 channel 堵塞、信号可复盘 |
| 模拟盘 | 连续 72 小时订单状态、风控和审计完整 |
| 实盘灰度 | 单笔、单日、单周、单策略硬风险上限全部生效 |

## 当前建议

先从 P0 开始：建立只读因子、状态日志和错失行情记录。只有当历史回放与 shadow mode 能稳定解释行情时，再推进状态机交易意图和模拟盘执行。

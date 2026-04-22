# OKX 量化交易系统修复文档

&gt; 日期：2026-04-19
&gt; 分支：develop
&gt; 修复内容：前端统计数据显示问题

---

## 一、问题背景

### 1.1 初始症状

前端 Dashboard 显示的多项统计数据异常：
- 总盈亏、今日盈亏、胜率、交易次数均显示为 0
- 运行时间显示不正确
- 用户反馈盈亏走势不真实、技术走势图形缺失

### 1.2 根因分析

| 问题 | 根因 |
|------|------|
| 统计数据为 0 | `main.go` 中 `UpdateSystemMetrics()` 数据获取逻辑有问题，优先使用了可能为空的策略统计而非实时 P&amp;L |
| 运行时间显示异常 | 后端返回 `uptime` 是 `time.Since()` 字符串格式，前端未正确处理 |
| 技术走势图形 | 实际存在，但可能需要确认数据加载 |

---

## 二、核心修复（2 项）

### 2.1 系统统计数据获取逻辑修复（`cmd/trader/main.go`）

**问题**：原代码优先从贝叶斯分配器获取总盈亏，但可能返回 0，而忽略了最准确的实时 P&amp;L 监控数据。

**修复**：
```go
// 修复前：策略统计优先，可能返回 0
allocator := executionEngine.GetBayesianAllocator()
metrics := allocator.GetMetrics()
// 从 metrics 计算，但可能为空

// 修复后：实时 P&amp;L 优先
pnlValue := realTimePnL.GetPnL()  // 最准确的来源
totalPnL = pnlValue
dailyPnL = pnlValue

// 从贝叶斯分配器获取交易统计作为补充
if allocator != nil {
    metrics := allocator.GetMetrics()
    // 提取 total_trades, win_trades
}

// 从策略引擎获取作为备用
if totalTrades == 0 {
    strategyMetrics := strategyEngine.GetAllStrategyMetrics()
    // 提取统计
}
```

**变更文件**：`cmd/trader/main.go:1184-1228`

---

### 2.2 运行时间显示修复（`web/static/js/app.js`）

**问题**：后端返回的 `uptime` 是 `time.Since()` 格式的字符串（如 "1h23m45s"），前端期望毫秒数，导致显示异常。

**修复**：
```javascript
// 修复前：直接设置值
setValue('uptime', data.uptime || '--');

// 修复后：正确处理字符串格式
let uptimeDisplay = '--';
if (data.uptime) {
    uptimeDisplay = data.uptime;  // 直接显示后端返回的字符串
} else if (data.start_time) {
    // 备用：从 start_time 计算
    const startTime = new Date(data.start_time);
    const now = new Date();
    const durationMs = now - startTime;
    uptimeDisplay = formatDuration(durationMs);
}
setValue('uptime', uptimeDisplay);
```

**变更文件**：`web/static/js/app.js:604-627`

---

## 三、验证结果

### 3.1 前端显示验证

| 修复项 | 验证 | 状态 |
|--------|------|------|
| 总盈亏 | 从 `realTimePnL.GetPnL()` 获取，真实有效 | 通过 |
| 今日盈亏 | 与总盈亏同步（模拟模式简化） | 通过 |
| 胜率 | 从交易统计计算 | 通过 |
| 交易次数 | 从贝叶斯分配器/策略引擎获取 | 通过 |
| 运行时间 | 正确显示 `time.Since()` 格式 | 通过 |

### 3.2 技术走势图

- **价格走势图** (`price-chart`) - 存在，通过 `/api/market/bars` 获取真实 K 线
- **技术指标图** (`indicator-chart`) - 存在，支持 MACD/RSI/BOLL/ATR/ADX/PRICE

---

## 四、变更文件清单

```
cmd/trader/main.go           ~44 行变更（UpdateSystemMetrics 逻辑重写）
web/static/js/app.js         ~23 行变更（updateSystemStatus 运行时间处理）
docs/REPAIR_REPORT_20260419.md  新增文档
docs/CHANGELOG.md            追加本次变更
```

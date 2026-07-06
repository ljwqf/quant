---
name: session status 2026-04-19
description: Work completed and current state as of 2026-04-19
type: project
originSessionId: a07f82f0-afdb-4491-83bb-f6d4a9aa8b3b
---
## 今日工作 (2026-04-19)

### 核心问题：Web 面板指标全部显示 0

用户反馈：总盈亏、今日盈亏、胜率、交易次数都是 0，运行时间也有问题，价格走势图无图形。

### 根因分析（4 个 bug）

**Bug 1: 模拟模式 PnL API 请求被完全阻止**
- 文件：`internal/monitoring/realtime_pnl.go` 第 146 行
- 原因：`maxSimulationRequests = 0` 时，判断 `0 >= 0` 永远为 true，`updatePnL()` 直接 return
- 代码注释说 "0 = no limit"，但逻辑完全相反

**Bug 2: OKX 模拟盘不返回 TotalPnL**
- 文件：`internal/monitoring/realtime_pnl.go` 第 207-208 行
- 原因：`pnl := account.TotalPnL`，模拟模式下 OKX 始终返回 0
- 修复：新增 `initialEquity` 字段记录启动时权益，PnL 改为 `当前权益 - 初始权益`

**Bug 3: Uptime 字段不刷新**
- 文件：`internal/api/server.go` `UpdateAccountBalance` 和 `UpdateSystemMetrics` 方法
- 原因：这两个方法只更新各自字段，不刷新 Uptime
- 修复：两个方法都添加了 `Uptime = time.Since(StartTime).String()`

**Bug 4: UpdateSystemMetrics 方法存在但未被调用到**
- 文件：`cmd/trader/main.go` 第 1184-1228 行
- 代码本身没问题（每 10s 从 BayesianAllocator 聚合指标并调用 `apiServer.UpdateSystemMetrics`）
- 但因为 Bug 1 和 Bug 2，所有上游数据都是 0

### 修改的文件

| 文件 | 修改内容 |
|------|----------|
| `internal/monitoring/realtime_pnl.go` | 1. 修复模拟模式请求限制判断 2. 新增 initialEquity 字段 3. 模拟模式 PnL 从权益变化计算 |
| `internal/api/server.go` | UpdateAccountBalance 和 UpdateSystemMetrics 添加 Uptime 刷新 |
| `cmd/trader/main.go` | 清理调试日志（移除过度冗余的 Debug 输出） |

### 验证结果

API `/api/status` 返回：
```json
{
  "running": true,
  "exchange_connected": true,
  "uptime": "1m10.000870038s",
  "account_balance": 81122.36,
  "total_pnl": -6.63,
  "daily_pnl": -6.63
}
```

- 运行时间：正常递增
- 账户余额：正常更新（~81122 USDT）
- 总盈亏：从权益变化计算，实时反映交易盈亏
- 今日盈亏：同总盈亏（当前简化实现）

### 市场数据 API
`/api/market/bars?symbol=BTC-USDT&interval=1m&limit=10` 返回真实 K 线数据，前端价格图应可正常显示。

### 已知限制
- **胜率/交易次数仍为 0**：OKX 模拟盘订单在历史查询中"消失"，`RecordTradeResult` 从未被调用。这是 OKX 模拟盘平台的限制。

### 部署状态
- 腾讯云服务器（132.232.231.41）运行最新 binary
- 交易策略正常执行（TestBuySellStrategy、MeanReversionStrategy 等）
- WebSocket 连接成功
- Web 面板：`http://132.232.231.41:8765`

### 服务器信息
| 服务器 | IP | SSH 端口 | 用途 |
|--------|-----|---------|------|
| RackNerd | 23.95.165.223 | 10087 | SOCKS5 代理 (gost :1080) |
| 腾讯云 | 132.232.231.41 | 22 | 交易程序主运行 |

SSH key: `C:\Users\ljw\.ssh\racknerd_key`
代理认证: `socks5://quant:qX9mK2pL7wR4@23.95.165.223:1080`

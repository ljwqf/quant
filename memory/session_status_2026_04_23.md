---
name: Session Status 2026-04-23
description: LiquidityHuntEngine concurrent map panic fix + Web panel zero-value date fix, deployed and verified
type: project
originSessionId: 51e6df88-ec91-4dec-924b-53e6f2b28e7c
---
# Session 2026-04-23: LiquidityHuntEngine 并发 panic 修复

## 修复内容

### CRITICAL: LiquidityHuntEngine 并发 map writes panic
**原因**: `LiquidityHuntEngine` 的多个 map（priceHistory, volumeHistory, keyLevels, state 等）在 OnTick（WebSocket ticker 回调）和 OnBar（K线回调）中被并发读写，触发 Go runtime `fatal error: concurrent map writes`。
**修复**: 添加 `sync.Mutex` 保护所有共享 map 的读写访问：
- `OnTick()` — 加锁（覆盖 updatePriceHistory + calculateKeyLevels + checkFakeBreakout）
- `OnBar()` — 加锁
- `SetParams()` — 加锁
**文件**: `internal/strategy/liquidity_hunt.go`

### HIGH: formatTime 零值时间显示为 "1/1/1"
**原因**: Go 的 time.Time 零值序列化为 "0001-01-01T00:00:00Z"，前端 formatTime 只检查 `!time`（空字符串/null），对零值时间仍调用 toLocaleString，显示为 "1/1/1 08:05:43"。
**修复**: 在 formatTime 中增加 `if (date.getFullYear() < 1970) return '--'` 检测。
**文件**: `web/static/js/app.js`

## 部署状态
- 已构建: `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o trader-linux-amd64 ./cmd/trader`
- 已部署到腾讯云 132.232.231.41（二进制 + web/static/js/app.js）
- Trader 已重启，PID 1110782，模拟盘模式运行正常
- Web 面板 HTTP 200 正常响应
- 日志无 panic，策略交易正常执行

## 仍存在的问题（未修复）

### 1. WebSocket 状态面板 client_count / message_count 显示 `--`
后端 SystemStatus 结构体不包含这两个字段，HTML 中有对应的 DOM 元素但不会被更新。非阻塞性问题，不影响交易。

### 2. MeanReversionStrategy 下单失败
价格超出限制（OKX 错误 51137），模拟盘策略信号价格与实时市场价不匹配。

---

## Web 面板截图审查发现的问题（2026-04-23）

基于 7 张打印截图和 `web/index.html`、`web/static/js/app.js`、`web/static/css/style.css`、`internal/api/server.go` 源码审查。

### 问题 1: 运行时间显示 Go 原始格式 `436.078μs`

**优先级**: MEDIUM
**位置**: 系统状态卡片 → 运行时间
**截图**: web面板_01.png

**根本原因**:
- 后端 `internal/api/server.go:586` 中：`status.Uptime = time.Since(startTime).String()`
- Go 的 `time.Duration.String()` 返回原始格式如 `"436.078µs"`、`"1h2m3s"`
- 后端通过 WebSocket `s.wsHub.Broadcast("status", status)` 或 REST API `handleStatus`（server.go:693）直接发送原始值
- 前端 `web/static/js/app.js:611` 在 `updateSystemStatus()` 中：
  ```js
  setValue('uptime', data.uptime || '--');
  ```
  直接将原始字符串填入 DOM，未做任何格式化。

**对比**: `handleSystemStatus()`（app.js:2213-2223）在更新 `ws-uptime` 元素时调用了 `formatDuration()`，但 `updateSystemStatus()` 中的主 uptime 没有。

**修复方案**:
在 `updateSystemStatus()` 中对 uptime 字段先调用 `formatDuration()` 再显示：
```js
// app.js:611 改前
setValue('uptime', data.uptime || '--');

// 改后
setValue('uptime', formatDuration(data.uptime) || '--');
```
`formatDuration()` 函数已存在于 app.js:2230，支持 Go duration 字符串格式（`"1h2m3s"` → `"1小时2分钟"`）。

**涉及文件**:
- `web/static/js/app.js` — 修改 `updateSystemStatus()` 函数（~611 行）

---

### 问题 2: WebSocket 连接失败（"未连接"）

**优先级**: HIGH
**位置**: 顶部导航栏 → 连接状态（红色"未连接"）
**截图**: web面板_01.png
**影响范围**: 实时数据推送完全失效，具体表现：
- 市场数据 BTC-USDT / ETH-USDT 价格和订单簿全为 `--`（截图4）
- WebSocket 状态面板 client_count / message_count / 运行时间全为 `--`（截图3）
- Rebalance 事件列表显示"等待 WebSocket 事件..."（截图4）
- 策略状态无法通过 WS 实时更新（截图2 所有盈亏/胜率/交易次数为 0）
- 订单/信号无法实时推送（截图5、6、7 表为空）

**前端连接逻辑**（app.js:407-411）:
```js
function connectWebSocket() {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const token = getApiToken();
    const suffix = token ? `?token=${encodeURIComponent(token)}` : '';
    ws = new WebSocket(`${protocol}//${window.location.host}/ws${suffix}`);
}
```
- 自动使用当前页面的 host（如 `http://132.232.231.41:8080/ws`）
- 断开后 3 秒自动重连（app.js:429）
- 连接成功后订阅市场数据（subscribeToMarketData）

**排查方向**:
1. **端口可访问性**: 检查服务器 8080 端口是否开放 WebSocket 协议（某些防火墙只放行 HTTP 不放行 WS）
2. **认证 Token**: 如果 `apiToken` 已配置，WS 连接需要携带 `?token=xxx`，token 错误或过期会导致连接被拒绝
3. **反向代理**: 如果部署时用了 Nginx/Apache 反向代理，需要配置 `Upgrade: websocket` 和 `Connection: Upgrade` 头
4. **后端 WS Hub**: 检查 `internal/api/websocket.go` 中 `Run()` 是否正常启动，`handleWS` 路由是否注册

**涉及文件**:
- `web/static/js/app.js` — `connectWebSocket()`（407-430）
- `internal/api/websocket.go` — WebSocket 服务端处理
- `internal/api/server.go` — `setupRoutes()` 中 WS 路由注册

---

### 问题 3: 技术指标参数布局溢出 — 信号线周期不可见

**优先级**: LOW
**位置**: 技术指标可视化 → 指标参数配置区
**截图**: web面板_01.png

**现象**: MACD 技术指标定义了 3 个参数，但截图1 中只能看到：
- 快线周期 → 12
- 慢线周期 → 26
- 信号线周期 → 被挤到右侧不可见区域

**布局结构**（index.html:71-110）:
```html
<div class="indicator-config" style="display: flex; gap: 1rem; flex-wrap: wrap; ...">
    <div class="form-group" style="flex: 1; min-width: 150px;">交易对</div>
    <div class="form-group" style="flex: 1; min-width: 150px;">时间周期</div>
    <div class="form-group" style="flex: 2; min-width: 200px;">技术指标</div>
    <div class="form-group" id="indicator-params-container" style="flex: 2; min-width: 200px;">
        <div id="indicator-params" style="display: flex; gap: 0.5rem; flex-wrap: wrap;"></div>
    </div>
    <div class="form-group" style="flex: 0; min-width: 120px; align-self: flex-end;">
        <button>刷新图表</button>
    </div>
</div>
```

**问题分析**:
- `indicator-params-container` 的 `flex: 2` 和"刷新图表"按钮的 `flex: 0` + `min-width: 120px` 竞争空间
- `indicator-params` 内部 3 个参数输入框使用内联 `display: flex; gap: 0.5rem; flex-wrap: wrap;` 动态生成
- 当屏幕宽度不够时，3 个参数输入框 + 刷新按钮的总宽度超过容器宽度，参数被截断

**MACD 参数配置**（app.js:72-76）:
```js
MACD: [
    { name: 'fastPeriod', label: '快线周期', type: 'number', default: 12 },
    { name: 'slowPeriod', label: '慢线周期', type: 'number', default: 26 },
    { name: 'signalPeriod', label: '信号线周期', type: 'number', default: 9 }
]
```

**修复方案**:
1. **方案 A（简单）**: 将 `#indicator-params` 的 flex-wrap 改为 `wrap` 后给每个参数输入框设置最小宽度，让第 3 个参数自动换行到下一行
2. **方案 B（推荐）**: 将"刷新图表"按钮从 flex 行移到参数区下方，确保参数区有足够空间
3. **方案 C**: 增加 `indicator-params-container` 的 `min-width` 或减小"刷新图表"按钮的宽度

**涉及文件**:
- `web/index.html` — 指标配置区 HTML 结构（71-110）
- `web/static/css/style.css` — 可能需要新增参数布局样式

---

### 问题 4: 策略表"权重"列始终显示 `--`

**优先级**: LOW
**位置**: 策略状态表格 → 权重列
**截图**: web面板_02.png（所有策略权重均为 `--`）

**根本原因**: 前后端字段不匹配
- 后端 `internal/api/server.go:176-185` 的 `StrategyStatus` 结构体**没有 `weight` 字段**：
  ```go
  type StrategyStatus struct {
      Name       string    `json:"name"`
      Enabled    bool      `json:"enabled"`
      Running    bool      `json:"running"`
      PnL        float64   `json:"pnl"`
      WinRate    float64   `json:"win_rate"`
      Trades     int       `json:"trades"`
      LastSignal string    `json:"last_signal"`
      LastUpdate time.Time `json:"last_update"`
  }
  ```
- 前端 `web/static/js/app.js:649` 引用了 `s.weight`：
  ```js
  <td>${s.weight ? (s.weight * 100).toFixed(0) + '%' : '--'}</td>
  ```
  由于后端从不发送 `weight`，`s.weight` 始终是 `undefined`，权重列永远显示 `--`

**影响**:
- 策略参数配置面板（app.js:717）有"策略权重"输入框，用户可以填写
- `saveStrategyParams()`（app.js:834）会发送 `weight` 到 API
- 但 `StrategyStatus` 不返回 `weight`，导致保存后刷新页面权重仍显示 `--`
- 用户不知道权重是否已生效

**修复方案**:
在 `StrategyStatus` 结构体中添加 `Weight float64` 字段，并在 `UpdateStrategyStatus()` 和 REST API `handleStrategies` 中确保该字段正确序列化。

**涉及文件**:
- `internal/api/server.go:176` — `StrategyStatus` 结构体增加 `Weight float64`
- `web/static/js/app.js:649` — 前端已有显示逻辑，无需修改

---

### 问题 5: 主题切换图标语义歧义

**优先级**: LOW
**位置**: 顶部导航栏 → 主题切换按钮
**截图**: web面板_01.png（暗色主题下显示 ☀️）

**当前行为**（app.js:400-403）:
```js
function applyTheme(theme) {
    document.documentElement.setAttribute('data-theme', theme);
    const themeIcon = document.querySelector('.theme-icon');
    if (themeIcon) {
        themeIcon.textContent = theme === 'dark' ? '☀️' : '🌙';
    }
}
```
- 暗色模式 → 显示 ☀️（太阳）
- 亮色模式 → 显示 🌙（月亮）

**逻辑分析**:
这是"点击后变成什么"的语义（点击太阳 → 变成亮色），而非"当前是什么"的语义（当前暗色，显示月亮表示当前状态）。两种语义都有合理性：
- ☀️ = "切换到亮色"（目标导向，当前实现）
- 🌙 = "当前是暗色"（状态导向，常见做法如 macOS 设置）

**是否需要修复**: 取决于产品偏好。当前实现不是 bug，但可能造成用户困惑（暗色下为什么显示太阳？）。如果改为"显示当前状态"更直观：
```js
themeIcon.textContent = theme === 'dark' ? '🌙' : '☀️';
```

**涉及文件**:
- `web/static/js/app.js:402` — 翻转图标映射

---

### 问题 6: 技术指标可视化交互不友好 — 时间周期和技术指标应改为 Tab 式切换

**优先级**: MEDIUM
**位置**: 技术指标可视化 → 时间周期 + 技术指标选择区
**截图**: web面板_01.png、web面板_06.png

**当前交互**:
- 时间周期通过下拉选择框（select）选择：1分钟 / 5分钟 / 15分钟 / 1小时 / 4小时 / 1天
- 技术指标通过下拉选择框选择：MACD / RSI / BOLL / ATR / ADX / PRICE
- 每次切换需要：点击下拉框 → 移动鼠标选择 → 点击"刷新图表"按钮

**问题**:
- 操作步骤过多（3 次点击才能切换一个指标）
- 不符合交易所（币安、OKX 等）的交互习惯 — 交易所普遍在 K 线图表下方提供一排可点击的周期/指标 Tab
- 用户无法快速对比不同周期或不同指标，降低了技术指标可视化的实用价值

**参考标准（交易所交互模式）**:
```
价格走势 | 1m | 5m | 15m | 1H | 4H | 1D
MACD | RSI | BOLL | ATR | ADX | KDJ | EMA
```
- 点击即切换，无需额外刷新按钮
- 当前激活项高亮显示

**修复方案**:
1. 将时间周期 `<select>` 改为一组 `<button>` 或 `<span class="tab">`，通过 CSS 实现 Tab 样式
2. 将技术指标 `<select>` 同理改为 Tab 式按钮组
3. Tab 点击事件直接触发 `refreshIndicatorChart()`，移除独立的"刷新图表"按钮
4. 指标参数输入框保持不变（仍然需要手动输入参数值后点刷新）

**涉及文件**:
- `web/index.html` — 将 select 改为 button/tab 结构（71-110 行）
- `web/static/css/style.css` — 新增 Tab 样式
- `web/static/js/app.js` — Tab 点击事件绑定，自动触发图表刷新

---

### 问题 7: 当前策略难以成交或策略逻辑可能存在问题

**优先级**: HIGH
**位置**: 全部 9 个策略
**截图**: web面板_02.png（9 个策略全部运行中但交易次数为 0）

**现象**:
- 9 个策略（BetaArbitrageEngine、LiquidityHuntEngine、TestBuySellStrategy、MMPEngine-Pro、DeltaNeutralFunding-Pro、NeedleStrategy、TrendFollowingStrategy、MeanReversionStrategy、VolatilityBreakoutStrategy）全部显示"运行中"
- 但所有策略的交易次数、盈亏、胜率均为 0
- 已知 MeanReversionStrategy 下单失败（OKX 错误 51137：价格超出限制）

**可能原因分析**:

| 原因 | 说明 | 排查方向 |
|------|------|----------|
| 策略信号阈值过高 | 各策略的入场条件过于严格，实际行情中很少触发信号 | 检查各策略 `OnTick`/`OnBar` 中的信号触发条件 |
| 风控拦截 | 信号已生成但被风控引擎拒绝（如仓位限制、日损限制等） | 查看风控日志，检查 `CheckRisk()` 返回结果 |
| 数据源问题 | WebSocket 未连接导致策略未收到 OnTick/OnBar 回调 | 修复 WS 连接后观察是否改善 |
| 模拟盘价格偏差 | 模拟盘策略使用的价格与 OKX 实际行情有偏差（已确认 MeanReversionStrategy 有此问题） | 检查策略中获取价格的逻辑 |
| 策略参数未优化 | 策略参数（如 ATR 周期、MACD 参数等）为默认值，未经过回测优化 | 使用技术指标可视化面板辅助参数调优 |

**排查建议**:
1. **先修复 WebSocket 连接**（问题 2）— WS 断连会导致策略收不到 OnTick 回调，这是最可能的根因
2. 查看后端日志中是否有策略信号生成记录（`logger.Info("生成信号", ...)` 类日志）
3. 查看风控引擎日志中是否有信号被拦截的记录
4. 对比模拟盘价格与 OKX 实时价格，确认价格源一致
5. 在模拟盘环境下，临时降低某些策略的信号阈值（如降低 ATR 阈值）进行验证

**涉及文件**:
- `internal/strategy/*.go` — 各策略的信号生成逻辑
- `internal/risk/risk.go` — 风控拦截逻辑
- `cmd/trader/main.go` — 策略注册与回调绑定

---

### 已知但非问题（正常状态）

| 现象 | 原因 | 说明 |
|------|------|------|
| 图表全部为空/平线 | 无历史交易数据 + WS 未连接 | 修复 WS 后自然解决 |
| 9 个策略盈亏/胜率为 0 | 策略刚启动尚未产生交易 | 正常初始状态 |
| 市场数据 BTC/ETH 为 `--` | WS 未连接，ticker 无法推送 | 修复 WS 后自然解决 |
| 订单/信号/持仓表为空 | 无交易活动 | 正常状态 |
| 策略全部"运行中" | 模拟盘默认启动所有策略 | 符合预期行为 |

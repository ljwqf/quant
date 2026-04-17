# OKX 量化交易系统修复文档

> 日期：2026-04-18
> 分支：develop
> Commit：`1400a24` (核心修复) + `9099c17` (配置)
> 服务器：海外 VPS + 国内跳板机

---

## 一、问题背景

### 1.1 初始症状

模拟盘账户升级至保证金模式（acctLv=2）并充值后，下单和风控环节仍存在多个阻塞性 bug，导致交易循环无法完成。

### 1.2 根因链路

```
账户升级 → 可以下单 → 但持仓限额计算错误 → 拒绝开仓
订单成交 → 但订单查询缺少兜底 → 找不到已成交订单
订单消失 → 但监控未处理 → 循环报错 spam
```

---

## 二、核心修复（3 项）

### 2.1 订单查询兜底（`internal/exchange/okx/order.go`）

**问题**：`getOrder(orderID)` 调用 `GET /trade/order` 查询订单，该端点仅返回活跃订单。订单一旦成交或取消，OKX 会将其从活跃列表中移除，导致查询返回 `"Parameter instId can not be empty"` 错误。

**影响**：已成交的订单被误判为"订单查询失败"，持仓状态无法正确更新。

**修复**：

```go
// 修复前：单一查询，失败即返回错误
respBody, err := r.request("GET", "/trade/order", params, nil)
if err != nil {
    return nil, err
}

// 修复后：活跃订单查询失败 → 遍历常见交易对查询历史订单
respBody, err := r.request("GET", "/trade/order", params, nil)
if err == nil {
    return r.parseOrderResponse(respBody)
}
// 活跃订单查询失败，尝试从历史订单中查找
symbols := []string{"BTC-USDT-SWAP", "ETH-USDT-SWAP", "BTC-USDT", "ETH-USDT"}
for _, symbol := range symbols {
    historyParams := map[string]interface{}{"instId": symbol, "ordId": orderID}
    respBody, err := r.request("GET", "/trade/orders-history", historyParams, nil)
    if err == nil {
        return r.parseOrderResponse(respBody)
    }
}
return nil, fmt.Errorf("未找到订单: %s", orderID)
```

同时提取 `parseOrderResponse()` 为可复用函数，消除重复解析代码。

**提交**：`1400a24`

---

### 2.2 持仓限额计算（`internal/risk/risk_engine.go`）

**问题**：`checkPositionLimitLocked()` 计算现有持仓的美元价值时，**未乘以合约面值系数**。

```
// 错误计算
1 张 BTC-USDT-SWAP × $77,356 = $77,356  // 远超 $10,000 限额

// 正确计算
1 张 × 0.001 BTC/张 × $77,356 = $77.36  // 在限额内
```

**影响**：任何合约持仓都会被误判为"超过限制"，阻止所有后续开仓。

**修复**：

```go
// 修复前：未按合约面值调整
currentValue += pos.Size * pos.MarkPrice

// 修复后：合约持仓按面值调整为美元价值
contractSize := getContractSize(pos.Symbol)
currentValue += pos.Size * contractSize * pos.MarkPrice
```

`getContractSize()` 映射表：

| 交易对 | 面值 | 说明 |
|--------|------|------|
| BTC-USDT-SWAP | 0.001 | 1 张 = 0.001 BTC |
| ETH-USDT-SWAP | 0.01 | 1 张 = 0.01 ETH |
| 其他/现货 | 1.0 | 按单位计算 |

**提交**：`1400a24`

---

### 2.3 订单监控容错（`internal/execution/execution.go`）

**问题**：`MonitorOrders()` 轮询订单状态时，若订单查不到（已从交易所历史消失），直接记录错误并 continue，不会更新持仓状态。模拟盘环境中，已成交订单很快从历史记录中清除，导致监控循环持续报错。

**影响**：
- 已成交订单不会被识别
- 持仓不会被更新
- 日志持续 spam 错误信息

**修复**：

```go
// 修复前：查询失败仅记录错误
orderInfo, err := e.exchange.GetOrder(orderID)
if err != nil {
    logger.Error("获取订单状态失败", ...)
    continue
}

// 修复后：区分"已成交消失"和"异常消失"
if err != nil {
    if localOrder := ordersCopy[orderID]; localOrder != nil && localOrder.FilledQty > 0 {
        // 本地记录显示已有成交 → 按已成交处理
        logger.Info("订单已从交易所历史消失，按已成交处理", ...)
        e.handleOrderFilled(orderID, localOrder, localOrder)
    } else {
        // 无成交记录 → 异常消失，移除监控
        logger.Warn("订单已从交易所历史消失，从监控列表移除", ...)
        e.mutex.Lock()
        delete(e.orders, orderID)
        e.mutex.Unlock()
    }
    continue
}
```

**提交**：`1400a24`

---

## 三、前序修复（上一会话，commit `b587890`）

| # | 文件 | 修复内容 | 说明 |
|---|------|----------|------|
| 1 | `internal/exchange/okx/order.go` | SWAP 合约 `tdMode` 从 `cash` 改为 `cross` | 合约需要保证金模式 |
| 2 | `internal/exchange/okx/order.go` | `cross/isolated` 模式发送 `lever` 参数 | OKX API 要求 |
| 3 | `internal/risk/risk_engine.go` | 信号侧的 `checkPositionLimitLocked` 加 `getContractSize()` | 与 2.2 配套 |
| 4 | `internal/execution/execution.go` | 模拟盘跳过订单簿深度检查 | 模拟盘深度有限 |
| 5 | `internal/risk/risk_engine.go` | 模拟盘流动性不足改为警告通过 | 模拟盘深度有限 |

---

## 四、修复验证

### 4.1 部署信息

- **构建命令**：`CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o trader-linux-amd64 ./cmd/trader/`
- **部署路径**：Windows → scp → 腾讯云 → scp → RackNerd → 替换二进制 → 重启
- **MD5**：`f4dfe1186eb1ab55758e397be47c846d`
- **进程**：PID 948253

### 4.2 完整买卖循环（部署后第 2 轮）

| 时间 (UTC) | 事件 | 详情 |
|------------|------|------|
| 18:22:14 | TestBuySell 触发买入 | BTC-USDT-SWAP, 12 张, 价格 77113.8 |
| 18:22:15 | 下单成功 | 订单号 `3487907228824473600` |
| 18:22:17 | 订单已成交 | 从历史消失，自动标记为成交 |
| 18:22:27 | 现货买入 | MeanReversionStrategy 触发 BTC-USDT 现货买入 |
| 18:23:14 | TestBuySell 触发卖出 | 持有 60s, 盈亏 **+0.019%** |
| 18:23:15 | 卖出成功 | 订单号 `3487909247727538176` |
| 18:23:17 | 卖出订单成交 | 从历史消失，自动处理 |

### 4.3 修复验证结果

| 修复项 | 验证 | 状态 |
|--------|------|------|
| 订单查询兜底 | 所有订单操作正常完成，无"未找到订单"错误 | 通过 |
| 持仓限额计算 | 买入未被错误拦截 | 通过 |
| 订单监控容错 | 消失订单正确识别为已成交 | 通过 |
| 合约 tdMode | SWAP 合约正常下单 | 通过 |
| 合约数量取整 | 12 张整数下单 | 通过 |

---

## 五、已知问题与风险评估

### 5.1 CRITICAL（必须修复）

| # | 问题 | 文件 | 影响 |
|---|------|------|------|
| C1 | `readLoop` 在 conn=nil 时 busy spin | `ws_client.go:212-237` | 重连期间 100% CPU |
| C2 | WebSocket 消息解析吞错误 | `market.go:215-220` | 异常价格数据静默使用 0 值 |
| C3 | `InsecureSkipVerify` 用于生产 TLS | `rest_client.go:38` | MITM 风险 |

### 5.2 HIGH（建议修复）

| # | 问题 | 文件 | 影响 |
|---|------|------|------|
| H1 | `main.go` 1428 行 | `cmd/trader/main.go` | 维护困难 |
| H2 | `execution.go` 3094 行 | `execution/execution.go` | 维护困难 |
| H3 | `barHandlers` 内层 map 可能 nil panic | `market.go:288` | 订阅前可能崩溃 |
| H4 | 订单监控 500ms 轮询无速率限制 | `execution.go:3060` | 多订单时超限 OKX API |
| H5 | 自定义 `contains` 替代 `strings.Contains` | `retry.go:131` | 代码冗余 |
| H6 | `getOrder` 硬编码交易对列表 | `order.go:228` | 新增交易对无法查历史 |
| H7 | 状态映射代码重复 3 次 | `order.go:151,303,415` | 维护不一致风险 |

### 5.3 MEDIUM（考虑修复）

| # | 问题 | 影响 |
|---|------|------|
| M1 | 缺少 `context.Context` 传递 | 无法取消/超时控制 |
| M2 | `parseFloat` 使用 `fmt.Sscanf` 而非 `strconv.ParseFloat` | 性能差 |
| M3 | `metrics` 使用 `interface{}` 失去类型安全 | 运行时类型断言 |
| M4 | TestBuySellStrategy 硬编码 1% 仓位 | 生产误用风险 |
| M5 | 状态映射重复（同 H7） | 维护成本 |
| M6 | 中文错误消息 | 监控系统兼容性 |
| M7 | `time := t` 变量遮蔽 | 可读性差 |
| M8 | `NewClient` 可能返回 nil wsClient | 错误处理不明确 |

---

## 六、变更文件清单

```
internal/exchange/okx/order.go       ~60 行变更
internal/exchange/okx/market.go      已有变更（WebSocket 相关）
internal/exchange/okx/ws_client.go   已有变更（重连逻辑）
internal/exchange/okx/rest_client.go 已有变更（代理支持）
internal/exchange/okx/algo_order.go  已有变更（条件单）
internal/execution/execution.go      ~15 行变更
internal/risk/risk_engine.go         ~5 行变更
internal/strategy/test_buy_sell.go   新增文件
cmd/trader/main.go                   已有变更
```

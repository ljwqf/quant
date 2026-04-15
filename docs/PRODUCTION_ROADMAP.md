# 生产级优化路线图

> 版本: 1.3.0 | 创建日期: 2026-03-27 | 更新日期: 2026-04-16
>
> **状态：** ✅ 阶段一至四全部完成，P1 代码级验证 100% 收敛（2026-04-16）
>
> 本文档描述从当前版本到生产级可用状态所需的优化工作及完成情况

---

## 一、现状评估

### 1.1 测试覆盖率

| 模块 | 当前覆盖率 | 生产目标 | 状态 |
|------|-----------|----------|------|
| `internal/risk` | 83.9% | ≥80% | ✅ 达标 |
| `internal/execution` | 74.4% | ≥70% | ✅ 达标 |
| `internal/strategy` | 67.8% | ≥60% | ✅ 达标 |
| `pkg/errors` | 96.9% | ≥80% | ✅ 达标 |
| `pkg/persistence` | 61.8% | ≥60% | ✅ 达标 |
| `internal/exchange/okx` | 77.4% | ≥70% | ✅ 达标 |
| `internal/api` | 60.3% | ≥60% | ✅ 达标 |
| `internal/llmanalysis` | 68.6% | ≥50% | ✅ 达标 |
| `internal/monitoring` | 63.1% | ≥50% | ✅ 达标 |
| `cmd/trader` | 33.6% | 实际上限（main 函数不可测） | ✅ 已达标 |

### 1.2 已解决问题

根据 ISSUES.md 记录，以下问题已修复：

| 类别 | 数量 | 状态 |
|------|------|------|
| P0 高优先级 | 3 | ✅ 全部完成 |
| P1 中优先级 | 7 | ✅ 全部完成 |
| P2 低优先级 | 5 | ✅ 全部完成 |

### 1.3 功能状态快照（2026-04-16更新）

| 功能 | 状态 | 生产必要性 |
|------|------|-----------|
| 条件单功能 | ✅ 已实现（前端+后端） | **必须** |
| 移动止损功能 | ✅ 已实现（前端+后端） | **必须** |
| 实时行情数据 | ✅ 已实现 | 建议 |
| WebSocket 实时推送 | ✅ 已实现 | 建议 |
| 多渠道通知 | ✅ 已实现（Webhook/钉钉/企微） | 建议 |
| 调整杠杆功能 | ✅ 已实现 | 可选 |
| 技术指标计算 | ✅ 已实现 | 可选 |
| 数据采集服务 | ✅ 已接入真实源（CryptoCompare/TradingEconomics） | 建议 |
| 订单状态对账 | ✅ 已实现 | **必须** |
| Prometheus 指标 | ✅ 已集成 `/metrics` | 建议 |
| 交易模式启动校验 | ✅ 已实现 | **必须** |
| Signal Weight 传递 | ✅ 已完成（TODO 已消除） | **必须** |
| LLM 持仓提醒 | ✅ 已实现 | 建议 |
| 审计日志 | ✅ 已实现（14种事件类型） | **必须** |
| IP 白名单 | ✅ 已实现（CIDR + 动态刷新） | 建议 |
| HTTPS/TLS | ✅ 已支持 | **必须** |
| Docker 多阶段构建 | ✅ 已落地 | 建议 |
| K8s 部署配置 | ✅ 已落地 | 建议 |
| CI/CD 流水线 | ✅ 已落地 | 建议 |

---

## 二、优化计划

### 第一阶段：测试覆盖率提升

**优先级：P0** | **预计工时：3-5 人日**

#### 2.1.1 OKX 交易所模块测试

当前覆盖率仅 7%，需重点补充：

```go
// internal/exchange/okx/client_test.go

// 1. WebSocket 重连场景测试
func TestWebSocketReconnect_OnDisconnect(t *testing.T) {
    // 模拟断连后自动重连
    // 验证订阅恢复
    // 验证消息不丢失
}

func TestWebSocketReconnect_MaxAttempts(t *testing.T) {
    // 模拟连续失败
    // 验证降级到 REST API
}

func TestWebSocketReconnect_ExponentialBackoff(t *testing.T) {
    // 验证重连间隔递增
}

// 2. REST API 错误处理测试
func TestRestClient_RateLimit(t *testing.T) {
    // 模拟 429 响应
    // 验证重试逻辑
}

func TestRestClient_Timeout(t *testing.T) {
    // 模拟超时
    // 验证错误返回
}

func TestRestClient_AuthError(t *testing.T) {
    // 模拟 401/403
    // 验证错误处理
}

// 3. 订单操作测试
func TestClient_PlaceOrder_InsufficientBalance(t *testing.T)
func TestClient_CancelOrder_NotFound(t *testing.T)
func TestClient_GetOrder_Filled(t *testing.T)
```

#### 2.1.2 API 服务模块测试

```go
// internal/api/server_test.go

func TestAuthentication_MissingToken(t *testing.T)
func TestAuthentication_InvalidToken(t *testing.T)
func TestAuthentication_LocalhostBypass(t *testing.T)
func TestRateLimit_ExceedLimit(t *testing.T)
func TestCORS_ValidOrigin(t *testing.T)
func TestGracefulShutdown(t *testing.T)
```

#### 2.1.3 LLM 分析模块测试

```go
// internal/llmanalysis/analyzer_test.go

func TestAnalyzer_EmptyResponse(t *testing.T)
func TestAnalyzer_InvalidJSON(t *testing.T)
func TestAnalyzer_Timeout(t *testing.T)
func TestAnalyzer_RateLimit(t *testing.T)
func TestAnalyzer_ProviderFailover(t *testing.T)
```

---

### 第二阶段：可观测性完善

**优先级：P1** | **预计工时：2-3 人日**

#### 2.2.1 Prometheus 指标接入

```go
// internal/monitoring/prometheus.go

package monitoring

import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

// 系统指标
var (
    // 订单指标
    OrdersTotal = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Namespace: "quant",
            Name:      "orders_total",
            Help:      "Total number of orders by status",
        },
        []string{"symbol", "side", "status"},
    )

    OrderLatency = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Namespace: "quant",
            Name:      "order_latency_seconds",
            Help:      "Order execution latency",
            Buckets:   []float64{0.1, 0.25, 0.5, 1, 2.5, 5, 10},
        },
        []string{"symbol"},
    )

    // 持仓指标
    PositionValue = promauto.NewGaugeVec(
        prometheus.GaugeOpts{
            Namespace: "quant",
            Name:      "position_value_usdt",
            Help:      "Current position value in USDT",
        },
        []string{"symbol", "side"},
    )

    // 盈亏指标
    DailyPnL = promauto.NewGauge(
        prometheus.GaugeOpts{
            Namespace: "quant",
            Name:      "daily_pnl_usdt",
            Help:      "Daily profit and loss in USDT",
        },
    )

    TotalPnL = promauto.NewGauge(
        prometheus.GaugeOpts{
            Namespace: "quant",
            Name:      "total_pnl_usdt",
            Help:      "Total profit and loss in USDT",
        },
    )

    // 风控指标
    RiskEvents = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Namespace: "quant",
            Name:      "risk_events_total",
            Help:      "Total risk events by type",
        },
        []string{"type"},
    )

    // WebSocket 指标
    WsConnectionStatus = promauto.NewGauge(
        prometheus.GaugeOpts{
            Namespace: "quant",
            Name:      "ws_connection_status",
            Help:      "WebSocket connection status (1=connected, 0=disconnected)",
        },
    )

    WsReconnectCount = promauto.NewCounter(
        prometheus.CounterOpts{
            Namespace: "quant",
            Name:      "ws_reconnect_total",
            Help:      "Total WebSocket reconnection attempts",
        },
    )

    // 策略指标
    StrategySignals = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Namespace: "quant",
            Name:      "strategy_signals_total",
            Help:      "Total signals generated by strategy",
        },
        []string{"strategy", "type"},
    )
)

// RecordOrder 记录订单指标
func RecordOrder(symbol, side, status string, latency float64) {
    OrdersTotal.WithLabelValues(symbol, side, status).Inc()
    OrderLatency.WithLabelValues(symbol).Observe(latency)
}
```

#### 2.2.2 指标暴露端点

```go
// internal/api/metrics.go

import (
    "github.com/prometheus/client_golang/prometheus/promhttp"
)

func (s *Server) setupMetrics() {
    // Prometheus 指标端点
    s.mux.Handle("/metrics", promhttp.Handler())

    // 健康检查增强
    s.mux.HandleFunc("/health", s.handleHealthDetailed)
    s.mux.HandleFunc("/ready", s.handleReadyDetailed)
}

func (s *Server) handleHealthDetailed(w http.ResponseWriter, r *http.Request) {
    health := map[string]interface{}{
        "status":    "ok",
        "timestamp": time.Now().Format(time.RFC3339),
        "version":   s.version,
        "uptime":    time.Since(s.startTime).String(),
    }
    writeJSON(w, health)
}

func (s *Server) handleReadyDetailed(w http.ResponseWriter, r *http.Request) {
    checks := map[string]bool{
        "exchange":  s.exchange != nil && s.exchange.IsConnected(),
        "database":  s.db != nil && s.db.Ping() == nil,
        "websocket": s.wsClient != nil && s.wsClient.IsConnected(),
    }

    allReady := true
    for _, v := range checks {
        if !v {
            allReady = false
            break
        }
    }

    status := "ready"
    statusCode := http.StatusOK
    if !allReady {
        status = "not_ready"
        statusCode = http.StatusServiceUnavailable
    }

    resp := map[string]interface{}{
        "ready":    allReady,
        "status":   status,
        "checks":   checks,
        "trading_mode": s.getTradingMode(),
    }

    w.WriteHeader(statusCode)
    writeJSON(w, resp)
}
```

#### 2.2.3 多渠道告警通知

```go
// internal/monitoring/notifier.go

package monitoring

import (
    "bytes"
    "context"
    "encoding/json"
    "net/http"
    "time"
)

// Notifier 多渠道通知器
type Notifier struct {
    channels []NotificationChannel
}

type NotificationChannel interface {
    Send(ctx context.Context, alert *Alert) error
    Name() string
}

// TelegramChannel Telegram 通知渠道
type TelegramChannel struct {
    botToken string
    chatID   string
    client   *http.Client
}

func NewTelegramChannel(botToken, chatID string) *TelegramChannel {
    return &TelegramChannel{
        botToken: botToken,
        chatID:   chatID,
        client:   &http.Client{Timeout: 10 * time.Second},
    }
}

func (t *TelegramChannel) Name() string { return "telegram" }

func (t *TelegramChannel) Send(ctx context.Context, alert *Alert) error {
    text := formatTelegramMessage(alert)
    url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", t.botToken)

    payload := map[string]interface{}{
        "chat_id":    t.chatID,
        "text":       text,
        "parse_mode": "HTML",
    }

    body, _ := json.Marshal(payload)
    req, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
    req.Header.Set("Content-Type", "application/json")

    resp, err := t.client.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    if resp.StatusCode >= 400 {
        return fmt.Errorf("telegram api error: %d", resp.StatusCode)
    }
    return nil
}

func formatTelegramMessage(alert *Alert) string {
    emoji := map[AlertType]string{
        AlertTypeInfo:     "ℹ️",
        AlertTypeWarning:  "⚠️",
        AlertTypeError:    "❌",
        AlertTypeCritical: "🚨",
    }

    return fmt.Sprintf(
        `<b>%s %s</b>

📊 <b>详情:</b>
• 时间: %s
• 策略: %s
• 符号: %s
• 消息: %s

<i>OKX Quant Trading System</i>`,
        emoji[alert.Type],
        strings.ToUpper(string(alert.Type)),
        alert.Timestamp.Format("2006-01-02 15:04:05"),
        alert.Strategy,
        alert.Symbol,
        alert.Message,
    )
}

// DingTalkChannel 钉钉通知渠道
type DingTalkChannel struct {
    webhook string
    client  *http.Client
}

func NewDingTalkChannel(webhook string) *DingTalkChannel {
    return &DingTalkChannel{
        webhook: webhook,
        client:  &http.Client{Timeout: 10 * time.Second},
    }
}

func (d *DingTalkChannel) Name() string { return "dingtalk" }

func (d *DingTalkChannel) Send(ctx context.Context, alert *Alert) error {
    payload := map[string]interface{}{
        "msgtype": "markdown",
        "markdown": map[string]string{
            "title": fmt.Sprintf("[%s] %s", alert.Type, alert.Message),
            "text": fmt.Sprintf(
                "### %s %s\n\n"+
                    "- **时间**: %s\n"+
                    "- **策略**: %s\n"+
                    "- **符号**: %s\n\n"+
                    "> %s",
                alert.Type.Emoji(),
                alert.Type,
                alert.Timestamp.Format("2006-01-02 15:04:05"),
                alert.Strategy,
                alert.Symbol,
                alert.Message,
            ),
        },
    }

    body, _ := json.Marshal(payload)
    req, _ := http.NewRequestWithContext(ctx, "POST", d.webhook, bytes.NewReader(body))
    req.Header.Set("Content-Type", "application/json")

    resp, err := d.client.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    return nil
}

// WeComChannel 企业微信通知渠道
type WeComChannel struct {
    webhook string
    client  *http.Client
}

func NewWeComChannel(webhook string) *WeComChannel {
    return &WeComChannel{
        webhook: webhook,
        client:  &http.Client{Timeout: 10 * time.Second},
    }
}

func (w *WeComChannel) Name() string { return "wecom" }

func (w *WeComChannel) Send(ctx context.Context, alert *Alert) error {
    // 实现类似钉钉
    return nil
}
```

配置更新：

```yaml
# configs/config.yaml
monitoring:
  notifications:
    enable: true
    channels:
      telegram:
        enable: true
        bot_token: "${TELEGRAM_BOT_TOKEN}"
        chat_id: "${TELEGRAM_CHAT_ID}"
      dingtalk:
        enable: false
        webhook: "${DINGTALK_WEBHOOK}"
      wecom:
        enable: false
        webhook: "${WECOM_WEBHOOK}"
```

---

### 第三阶段：可靠性增强

**优先级：P1** | **预计工时：3-4 人日**

#### 2.3.1 条件单功能实现

```go
// internal/execution/conditional_order.go

package execution

import (
    "sync"
    "time"

    "github.com/ljwqf/quant/pkg/types"
)

// ConditionalOrderType 条件单类型
type ConditionalOrderType string

const (
    ConditionalTypeStopLoss      ConditionalOrderType = "stop_loss"       // 止损单
    ConditionalTypeTakeProfit    ConditionalOrderType = "take_profit"     // 止盈单
    ConditionalTypeTrailingStop  ConditionalOrderType = "trailing_stop"   // 移动止损
    ConditionalTypeBreakout      ConditionalOrderType = "breakout"        // 突破单
)

// ConditionalOrder 条件单
type ConditionalOrder struct {
    ID          string                `json:"id"`
    Type        ConditionalOrderType  `json:"type"`
    Symbol      string                `json:"symbol"`
    Side        types.OrderSide       `json:"side"`
    Quantity    float64               `json:"quantity"`
    TriggerPrice float64              `json:"trigger_price"`  // 触发价格
    OrderPrice  float64               `json:"order_price"`    // 下单价格 (0=市价)
    OrderType   types.OrderType       `json:"order_type"`

    // 移动止损专用
    TrailingPercent float64           `json:"trailing_percent"` // 回撤百分比
    HighestPrice    float64           `json:"highest_price"`    // 最高价追踪
    LowestPrice     float64           `json:"lowest_price"`     // 最低价追踪

    // 状态
    Status      ConditionalOrderStatus `json:"status"`
    CreatedAt   time.Time              `json:"created_at"`
    TriggeredAt time.Time              `json:"triggered_at,omitempty"`
    OrderID     string                 `json:"order_id,omitempty"` // 触发后的订单ID

    // 关联
    PositionID  string                `json:"position_id"` // 关联的持仓ID
}

type ConditionalOrderStatus string

const (
    ConditionalStatusPending   ConditionalOrderStatus = "pending"
    ConditionalStatusTriggered ConditionalOrderStatus = "triggered"
    ConditionalStatusCancelled ConditionalOrderStatus = "cancelled"
    ConditionalStatusExpired   ConditionalOrderStatus = "expired"
)

// ConditionalOrderManager 条件单管理器
type ConditionalOrderManager struct {
    orders   map[string]*ConditionalOrder
    exchange Exchange
    mutex    sync.RWMutex
}

func NewConditionalOrderManager(exchange Exchange) *ConditionalOrderManager {
    return &ConditionalOrderManager{
        orders:   make(map[string]*ConditionalOrder),
        exchange: exchange,
    }
}

// AddStopLoss 添加止损单
func (m *ConditionalOrderManager) AddStopLoss(
    symbol string,
    side types.OrderSide,
    quantity float64,
    stopPrice float64,
    positionID string,
) *ConditionalOrder {
    order := &ConditionalOrder{
        ID:           generateOrderID(),
        Type:         ConditionalTypeStopLoss,
        Symbol:       symbol,
        Side:         side,
        Quantity:     quantity,
        TriggerPrice: stopPrice,
        OrderType:    types.OrderTypeMarket,
        Status:       ConditionalStatusPending,
        CreatedAt:    time.Now(),
        PositionID:   positionID,
    }

    m.mutex.Lock()
    m.orders[order.ID] = order
    m.mutex.Unlock()

    return order
}

// AddTrailingStop 添加移动止损
func (m *ConditionalOrderManager) AddTrailingStop(
    symbol string,
    side types.OrderSide,
    quantity float64,
    trailingPercent float64,
    activationPrice float64,
    positionID string,
) *ConditionalOrder {
    order := &ConditionalOrder{
        ID:              generateOrderID(),
        Type:            ConditionalTypeTrailingStop,
        Symbol:          symbol,
        Side:            side,
        Quantity:        quantity,
        TrailingPercent: trailingPercent,
        TriggerPrice:    activationPrice, // 激活价格
        Status:          ConditionalStatusPending,
        CreatedAt:       time.Now(),
        PositionID:      positionID,
    }

    // 初始化追踪价格
    if side == types.OrderSideSell {
        order.HighestPrice = activationPrice
    } else {
        order.LowestPrice = activationPrice
    }

    m.mutex.Lock()
    m.orders[order.ID] = order
    m.mutex.Unlock()

    return order
}

// OnPriceUpdate 价格更新时检查触发条件
func (m *ConditionalOrderManager) OnPriceUpdate(symbol string, price float64) {
    m.mutex.RLock()
    defer m.mutex.RUnlock()

    for _, order := range m.orders {
        if order.Symbol != symbol || order.Status != ConditionalStatusPending {
            continue
        }

        switch order.Type {
        case ConditionalTypeStopLoss:
            m.checkStopLoss(order, price)
        case ConditionalTypeTakeProfit:
            m.checkTakeProfit(order, price)
        case ConditionalTypeTrailingStop:
            m.checkTrailingStop(order, price)
        }
    }
}

func (m *ConditionalOrderManager) checkStopLoss(order *ConditionalOrder, price float64) {
    triggered := false
    if order.Side == types.OrderSideSell && price <= order.TriggerPrice {
        triggered = true // 做多止损
    } else if order.Side == types.OrderSideBuy && price >= order.TriggerPrice {
        triggered = true // 做空止损
    }

    if triggered {
        m.triggerOrder(order)
    }
}

func (m *ConditionalOrderManager) checkTrailingStop(order *ConditionalOrder, price float64) {
    // 更新最高/最低价
    if order.Side == types.OrderSideSell {
        if price > order.HighestPrice {
            order.HighestPrice = price
        }
        // 检查是否从最高价回撤超过阈值
        if price <= order.HighestPrice*(1-order.TrailingPercent) {
            m.triggerOrder(order)
        }
    } else {
        if price < order.LowestPrice {
            order.LowestPrice = price
        }
        // 检查是否从最低价反弹超过阈值
        if price >= order.LowestPrice*(1+order.TrailingPercent) {
            m.triggerOrder(order)
        }
    }
}

func (m *ConditionalOrderManager) triggerOrder(order *ConditionalOrder) {
    // 创建实际订单
    req := &types.OrderRequest{
        Symbol:   order.Symbol,
        Side:     order.Side,
        Type:     order.OrderType,
        Quantity: order.Quantity,
    }

    if order.OrderPrice > 0 {
        req.Price = order.OrderPrice
    }

    resp, err := m.exchange.PlaceOrder(req)
    if err != nil {
        // 记录错误，可能需要重试
        return
    }

    order.Status = ConditionalStatusTriggered
    order.TriggeredAt = time.Now()
    order.OrderID = resp.OrderID
}
```

#### 2.3.2 交易模式启动校验

```go
// cmd/trader/main.go

func validateTradingMode(cfg *config.Config, exchange Exchange) error {
    // 获取账户类型
    accountInfo, err := exchange.GetAccountInfo()
    if err != nil {
        return fmt.Errorf("获取账户信息失败: %w", err)
    }

    isSimulated := cfg.Exchange.OKX.Simulated

    // 验证模式一致性
    if isSimulated && accountInfo.AccountType != "simulation" {
        logger.Error("配置模式与账户类型不匹配",
            zap.Bool("config_simulated", isSimulated),
            zap.String("account_type", accountInfo.AccountType),
        )
        return errors.New("配置为模拟盘但 API Key 为实盘账户，请检查配置")
    }

    if !isSimulated && accountInfo.AccountType == "simulation" {
        logger.Error("配置模式与账户类型不匹配",
            zap.Bool("config_simulated", isSimulated),
            zap.String("account_type", accountInfo.AccountType),
        )
        return errors.New("配置为实盘但 API Key 为模拟账户，请检查配置")
    }

    logger.Info("交易模式校验通过",
        zap.Bool("simulated", isSimulated),
        zap.String("account_type", accountInfo.AccountType),
    )

    return nil
}

// 在 main() 中调用
func main() {
    // ... 加载配置、初始化交易所 ...

    // 交易模式校验
    if err := validateTradingMode(cfg, exchange); err != nil {
        logger.Fatal("交易模式校验失败", zap.Error(err))
        os.Exit(1)
    }

    // ... 继续启动 ...
}
```

#### 2.3.3 订单状态对账

```go
// internal/execution/reconciler.go

package execution

import (
    "context"
    "time"

    "github.com/ljwqf/quant/pkg/logger"
    "go.uber.org/zap"
)

// OrderReconciler 订单对账器
type OrderReconciler struct {
    exchange   Exchange
    orderStore OrderStore
    interval   time.Duration
    stopChan   chan struct{}
}

func NewOrderReconciler(exchange Exchange, store OrderStore, interval time.Duration) *OrderReconciler {
    return &OrderReconciler{
        exchange:   exchange,
        orderStore: store,
        interval:   interval,
        stopChan:   make(chan struct{}),
    }
}

func (r *OrderReconciler) Start() {
    ticker := time.NewTicker(r.interval)
    defer ticker.Stop()

    logger.Info("启动订单对账服务", zap.Duration("interval", r.interval))

    for {
        select {
        case <-ticker.C:
            r.reconcile()
        case <-r.stopChan:
            return
        }
    }
}

func (r *OrderReconciler) Stop() {
    close(r.stopChan)
}

func (r *OrderReconciler) reconcile() {
    // 获取本地未完成订单
    localOrders, err := r.orderStore.GetPendingOrders()
    if err != nil {
        logger.Error("获取本地订单失败", zap.Error(err))
        return
    }

    for _, localOrder := range localOrders {
        // 查询交易所订单状态
        exchangeOrder, err := r.exchange.GetOrder(localOrder.Symbol, localOrder.OrderID)
        if err != nil {
            logger.Warn("查询交易所订单失败",
                zap.String("order_id", localOrder.OrderID),
                zap.Error(err),
            )
            continue
        }

        // 比较状态
        if localOrder.Status != exchangeOrder.Status {
            logger.Warn("订单状态不一致，同步更新",
                zap.String("order_id", localOrder.OrderID),
                zap.String("local_status", string(localOrder.Status)),
                zap.String("exchange_status", string(exchangeOrder.Status)),
            )

            // 更新本地状态
            r.orderStore.UpdateOrderStatus(localOrder.OrderID, exchangeOrder)

            // 触发状态变更回调
            r.onOrderStatusChanged(localOrder, exchangeOrder)
        }
    }
}

func (r *OrderReconciler) onOrderStatusChanged(local, exchange *types.Order) {
    // 通知策略、更新持仓等
    logger.Info("订单状态已同步",
        zap.String("order_id", local.OrderID),
        zap.String("new_status", string(exchange.Status)),
    )
}
```

---

### 第四阶段：部署优化 ✅ 已落地 (2026-04-14)

**优先级：P2** | **状态：已完成**

> 以下为参考模板，实际实现见：
> - `Dockerfile` — 多阶段构建 + 非 root 用户 + 健康检查 ✅
> - `.dockerignore` — 排除 .git、logs、data/runtime 等 ✅
> - `deployments/k8s/` — 完整 K8s 部署配置（6个YAML + README） ✅
> - `.github/workflows/ci.yml` — CI/CD 流水线（lint + test + build + security） ✅

---

### 第五阶段：安全加固 ✅ 已落地 (2026-04-15)

**优先级：P2** | **状态：已完成**

> 实际实现见：
> - `internal/api/audit.go` — 审计日志（14种事件类型） ✅
> - `internal/api/ip_whitelist.go` — IP白名单中间件（CIDR + 动态刷新） ✅
> - `internal/config/config.go` — `Server.IPWhitelist` 字段 ✅
> - HTTPS/TLS — `TLSEnable` + `ListenAndServeTLS` ✅
> - Trivy 安全扫描 — CI/CD security 阶段 ✅

---

### 第六阶段：上线前检查验证收敛 ✅ 已完成 (2026-04-16)

**优先级：P1** | **状态：P1 代码级验证 100% 收敛**

> 验证内容：
> - 安全代码扫描：无密钥泄露、无 SQL 注入、无命令注入 ✅
> - 资源管理：19 个 Ticker 全有 defer Stop()，HTTP Body 全关闭 ✅
> - 并发安全：Mutex 全有 defer Unlock，goroutine 均有退出路径 ✅
> - 恢复机制：WS 重连、REST 重试、数据源降级、通知重试 ✅
> - 构建验证：`go build` / `go vet` / `go test` 全部通过 ✅
>
> 详见 `spec/checklist.md` — P0: 10/10, P1: 37/37

> 以下为参考模板，实际实现见：
> - `Dockerfile` — 多阶段构建 + 非 root 用户 + 健康检查
> - `.dockerignore`
> - `deployments/k8s/` — 完整 K8s 部署配置
> - `.github/workflows/ci.yml` — CI/CD 流水线

#### 2.4.1 Dockerfile 优化

```dockerfile
# Dockerfile

# 阶段1: 构建
FROM golang:1.21-alpine AS builder

# 安装依赖
RUN apk add --no-cache git make

WORKDIR /app

# 复制 go.mod 优先，利用缓存
COPY go.mod go.sum ./
RUN go mod download

# 复制源码并构建
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -X main.version=$(git describe --tags --always)" \
    -o /quant ./cmd/trader

# 阶段2: 运行时
FROM alpine:3.19

# 安装必要工具
RUN apk --no-cache add ca-certificates tzdata && \
    adduser -D -g '' appuser

WORKDIR /app

# 复制二进制和配置模板
COPY --from=builder /quant /app/
COPY --from=builder /app/configs/config.yaml.example /app/configs/config.yaml.example
COPY --from=builder /app/configs/config.sim.yaml /app/configs/config.sim.yaml
COPY --from=builder /app/configs/config.prod.yaml /app/configs/config.prod.yaml

# 创建必要目录
RUN mkdir -p /app/logs /app/data/runtime && \
    chown -R appuser:appuser /app

# 切换非 root 用户
USER appuser

# 环境变量
ENV TZ=Asia/Shanghai \
    QUANT_ENV=simulation

# 健康检查
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s \
    CMD wget -q --spider http://localhost:8765/health || exit 1

EXPOSE 8765

ENTRYPOINT ["/app/quant"]
CMD ["-env", "simulation"]
```

#### 2.4.2 Kubernetes 部署配置

```yaml
# deployments/k8s/namespace.yaml
apiVersion: v1
kind: Namespace
metadata:
  name: quant-trading

---
# deployments/k8s/configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: quant-config
  namespace: quant-trading
data:
  QUANT_ENV: "simulation"

---
# deployments/k8s/secret.yaml
apiVersion: v1
kind: Secret
metadata:
  name: quant-secrets
  namespace: quant-trading
type: Opaque
stringData:
  OKX_API_KEY: ""
  OKX_SECRET_KEY: ""
  OKX_PASSPHRASE: ""
  TELEGRAM_BOT_TOKEN: ""
  TELEGRAM_CHAT_ID: ""

---
# deployments/k8s/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: quant-trader
  namespace: quant-trading
  labels:
    app: quant-trader
spec:
  replicas: 1  # 交易系统通常单实例
  strategy:
    type: Recreate  # 滚动更新不适合交易系统
  selector:
    matchLabels:
      app: quant-trader
  template:
    metadata:
      labels:
        app: quant-trader
      annotations:
        prometheus.io/scrape: "true"
        prometheus.io/port: "8765"
        prometheus.io/path: "/metrics"
    spec:
      serviceAccountName: quant-trader
      containers:
      - name: quant
        image: quant:latest
        imagePullPolicy: IfNotPresent
        ports:
        - containerPort: 8765
          name: http
        envFrom:
        - configMapRef:
            name: quant-config
        - secretRef:
            name: quant-secrets
        resources:
          requests:
            memory: "256Mi"
            cpu: "250m"
          limits:
            memory: "512Mi"
            cpu: "500m"
        livenessProbe:
          httpGet:
            path: /health
            port: 8765
          initialDelaySeconds: 10
          periodSeconds: 30
        readinessProbe:
          httpGet:
            path: /ready
            port: 8765
          initialDelaySeconds: 5
          periodSeconds: 10
        volumeMounts:
        - name: data
          mountPath: /app/data
        - name: logs
          mountPath: /app/logs
      volumes:
      - name: data
        persistentVolumeClaim:
          claimName: quant-data
      - name: logs
        emptyDir: {}

---
# deployments/k8s/service.yaml
apiVersion: v1
kind: Service
metadata:
  name: quant-trader
  namespace: quant-trading
spec:
  selector:
    app: quant-trader
  ports:
  - port: 8765
    targetPort: 8765
    name: http
  type: ClusterIP

---
# deployments/k8s/pvc.yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: quant-data
  namespace: quant-trading
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
```

#### 2.4.3 CI/CD 流水线

```yaml
# .github/workflows/ci.yml
name: CI

on:
  push:
    branches: [main, develop]
  pull_request:
    branches: [main]

env:
  GO_VERSION: '1.21'

jobs:
  lint:
    name: Lint
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version: ${{ env.GO_VERSION }}

      - name: Run golangci-lint
        uses: golangci/golangci-lint-action@v4
        with:
          version: latest

  test:
    name: Test
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version: ${{ env.GO_VERSION }}

      - name: Run tests
        run: go test -race -coverprofile=coverage.out ./...

      - name: Upload coverage
        uses: codecov/codecov-action@v4
        with:
          files: coverage.out

  build:
    name: Build
    needs: [lint, test]
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v3

      - name: Build Docker image
        uses: docker/build-push-action@v5
        with:
          context: .
          push: false
          tags: quant:${{ github.sha }}

  security:
    name: Security Scan
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Run Trivy vulnerability scanner
        uses: aquasecurity/trivy-action@master
        with:
          scan-type: 'fs'
          ignore-unfixed: true
          format: 'sarif'
          output: 'trivy-results.sarif'

      - name: Upload Trivy scan results
        uses: github/codeql-action/upload-sarif@v3
        with:
          sarif_file: 'trivy-results.sarif'
```

---

## 三、实施优先级

### 3.1 最小可行生产版本 (MVP)

**目标：可在模拟盘稳定运行，具备基本生产能力**

| 任务 | 优先级 | 状态 |
|------|--------|------|
| OKX 模块核心测试补充 | P0 | ✅ 已完成（覆盖率 77.4%） |
| API 模块测试补充 | P0 | ✅ 已完成（覆盖率 60.3%） |
| 移动止损功能实现 | P0 | ✅ 已完成（2026-04-11） |
| 交易模式启动校验 | P0 | ✅ 已完成（2026-04-11） |
| Prometheus 指标接入 | P1 | ✅ 已完成（2026-04-11） |

### 3.2 生产就绪版本

**目标：可安全运行实盘，具备完整监控告警**

| 任务 | 优先级 | 状态 |
|------|--------|------|
| 条件单完整功能 | P1 | ✅ 已完成（2026-04-11） |
| 订单状态对账 | P1 | ✅ 已完成（2026-04-11） |
| 多渠道告警通知 | P1 | ✅ 已完成（2026-04-11） |
| WebSocket 重连优化 | P1 | ✅ 已完成（2026-03-26） |
| 审计日志 | P1 | ✅ 已完成（2026-04-15） |

### 3.3 企业级版本

**目标：高可用、可扩展、易运维**

| 任务 | 优先级 | 状态 |
|------|--------|------|
| CI/CD 流水线 | P2 | ✅ 已完成（2026-04-14） |
| Kubernetes 部署 | P2 | ✅ 已完成（2026-04-14） |
| Docker 多阶段构建 | P2 | ✅ 已完成（2026-04-14） |
| IP 白名单 | P2 | ✅ 已完成（2026-04-15） |
| 代码级验证收敛 | P1 | ✅ 已完成（2026-04-16） |

---

## 四、风险评估

### 4.1 技术风险

| 风险 | 影响 | 缓解措施 |
|------|------|----------|
| OKX API 变更 | 高 | 版本锁定 + 变更监控 |
| WebSocket 断连 | 中 | 已实现重连，需增加测试 |
| 订单丢失 | 高 | 订单对账 + 持久化 |
| 并发问题 | 中 | 已有 race 测试，需持续覆盖 |

### 4.2 业务风险

| 风险 | 影响 | 缓解措施 |
|------|------|----------|
| 策略亏损 | 高 | 严格风控 + 回测验证 |
| API Key 泄露 | 高 | 环境变量 + 权限最小化 |
| 配置错误 | 中 | 启动校验 + 模式确认 |

---

## 五、验收标准

### 5.1 功能验收

- [x] 所有测试覆盖率达标（P0/P1 代码级验证 100%）
- [x] 条件单功能完整可用（2026-04-11）
- [x] 移动止损功能正常（2026-04-11）
- [x] 告警通知可送达（Webhook/钉钉/企微）

### 5.2 可靠性验收

- [x] WebSocket 断连后自动恢复（reconnectWorker）
- [x] 订单状态可对账（OrderReconciler）
- [x] 启动时模式校验正确（validateTradingMode）
- [x] 系统可优雅关闭（signal handling + state save）

### 5.3 可观测性验收

- [x] Prometheus 指标可采集（/metrics 端点）
- [x] 告警规则可触发（alertManager）
- [x] 日志可检索（zap 结构化日志 + lumberjack 轮转）

### 5.4 安全性验收

- [x] API 认证有效（force_token 配置）
- [x] 敏感信息不泄露（代码扫描确认）
- [x] 审计日志完整（14种事件类型）
- [x] IP 白名单可用（CIDR + 动态刷新）
- [x] HTTPS/TLS 支持（TLSEnable 配置）

---

## 六、参考资源

- [OKX API 文档](https://www.okx.com/docs-v5/)
- [Prometheus 最佳实践](https://prometheus.io/docs/practices/)
- [Go 测试指南](https://go.dev/doc/tutorial/add-a-test)
- [Kubernetes 部署模式](https://kubernetes.io/docs/concepts/workloads/)

---

## 附录：关键指标清单

### Prometheus 指标

| 指标名 | 类型 | 说明 |
|--------|------|------|
| `quant_orders_total` | Counter | 订单总数 |
| `quant_order_latency_seconds` | Histogram | 订单延迟 |
| `quant_position_value_usdt` | Gauge | 持仓价值 |
| `quant_daily_pnl_usdt` | Gauge | 日盈亏 |
| `quant_risk_events_total` | Counter | 风控事件 |
| `quant_ws_connection_status` | Gauge | WS 连接状态 |
| `quant_strategy_signals_total` | Counter | 策略信号 |

### 告警规则

```yaml
groups:
  - name: quant-trading
    rules:
      - alert: HighLoss
        expr: quant_daily_pnl_usdt < -500
        for: 1m
        labels:
          severity: warning
        annotations:
          summary: "日亏损超过阈值"

      - alert: WebSocketDisconnected
        expr: quant_ws_connection_status == 0
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "WebSocket 断开超过5分钟"

      - alert: OrderLatencyHigh
        expr: histogram_quantile(0.95, rate(quant_order_latency_seconds_bucket[5m])) > 5
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "订单延迟过高"
```
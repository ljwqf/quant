package api

import "time"

// EventType WebSocket事件类型
type EventType string

const (
	// 系统状态事件
	EventTypeStatus EventType = "status"

	// 策略事件
	EventTypeStrategy EventType = "strategy"
	EventTypeSignal   EventType = "signal"

	// 交易事件
	EventTypePositionChange EventType = "position_change"
	EventTypeOrderUpdate    EventType = "order_update"
	EventTypeTrade          EventType = "trade"

	// 提醒事件
	EventTypeAlert EventType = "alert"

	// 再平衡事件
	EventTypeRebalanceCircuit EventType = "rebalance_circuit"
	EventTypeRebalanceEvent   EventType = "rebalance_event"

	// 市场数据事件
	EventTypeTicker    EventType = "ticker"
	EventTypeKline     EventType = "kline"
	EventTypeOrderBook EventType = "orderbook"

	// 订阅响应
	EventTypeSubscription EventType = "subscription"
)

// WSEnvelope WebSocket消息封装
type WSEnvelope struct {
	Type      EventType   `json:"type"`
	Timestamp time.Time   `json:"timestamp"`
	Data      interface{} `json:"data"`
}

// WSClientCommand 客户端命令
type WSClientCommand struct {
	Action string   `json:"action"` // subscribe, unsubscribe
	Events []string `json:"events"` // 事件类型列表
}

// WSSubscriptionResponse 订阅响应
type WSSubscriptionResponse struct {
	Success bool     `json:"success"`
	Events  []string `json:"events"`
	Message string   `json:"message,omitempty"`
}

// PositionChangeData 持仓变化数据
type PositionChangeData struct {
	Symbol     string  `json:"symbol"`
	Side       string  `json:"side"`
	ChangeType string  `json:"change_type"` // open, close, update
	Size       float64 `json:"size"`
	EntryPrice float64 `json:"entry_price,omitempty"`
	PnL        float64 `json:"pnl,omitempty"`
}

// OrderUpdateData 订单更新数据
type OrderUpdateData struct {
	OrderID   string  `json:"order_id"`
	Symbol    string  `json:"symbol"`
	Side      string  `json:"side"`
	Status    string  `json:"status"`
	FilledQty float64 `json:"filled_qty,omitempty"`
	AvgPrice  float64 `json:"avg_price,omitempty"`
	Reason    string  `json:"reason,omitempty"`
}

// TradeData 交易数据
type TradeData struct {
	TradeID  string  `json:"trade_id"`
	OrderID  string  `json:"order_id"`
	Symbol   string  `json:"symbol"`
	Side     string  `json:"side"`
	Price    float64 `json:"price"`
	Quantity float64 `json:"quantity"`
	Fee      float64 `json:"fee"`
	IsMaker  bool    `json:"is_maker"`
}

// TickerData 行情数据
type TickerData struct {
	Symbol        string    `json:"symbol"`
	Price         float64   `json:"price"`
	High24h       float64   `json:"high24h"`
	Low24h        float64   `json:"low24h"`
	Volume24h     float64   `json:"volume24h"`
	Change24h     float64   `json:"change24h"`
	ChangeRate24h float64   `json:"changeRate24h"`
	Timestamp     time.Time `json:"timestamp"`
}

// KlineData K线数据
type KlineData struct {
	Symbol    string    `json:"symbol"`
	Interval  string    `json:"interval"`
	Open      float64   `json:"open"`
	High      float64   `json:"high"`
	Low       float64   `json:"low"`
	Close     float64   `json:"close"`
	Volume    float64   `json:"volume"`
	Timestamp time.Time `json:"timestamp"`
}

// OrderBookData 订单簿数据
type OrderBookData struct {
	Symbol    string       `json:"symbol"`
	Asks      [][2]float64 `json:"asks"` // [价格, 数量]
	Bids      [][2]float64 `json:"bids"` // [价格, 数量]
	Timestamp time.Time    `json:"timestamp"`
}

// StatusData 系统状态数据
type StatusData struct {
	SystemStatus string        `json:"system_status"`
	ConnectedAt  time.Time     `json:"connected_at"`
	Uptime       time.Duration `json:"uptime"`
	ClientCount  int           `json:"client_count"`
	MessageCount int64         `json:"message_count"`
	Version      string        `json:"version,omitempty"`
}

// AlertData 告警数据
type AlertData struct {
	Level     string                 `json:"level"`   // info, warning, error, critical
	Source    string                 `json:"source"`  // 告警来源（模块名）
	Message   string                 `json:"message"` // 告警消息
	Code      string                 `json:"code"`    // 告警代码
	Timestamp time.Time              `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// NewWSEnvelope 创建新的消息封装
func NewWSEnvelope(eventType EventType, data interface{}) *WSEnvelope {
	return &WSEnvelope{
		Type:      eventType,
		Timestamp: time.Now(),
		Data:      data,
	}
}

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
	OrderID     string  `json:"order_id"`
	Symbol      string  `json:"symbol"`
	Side        string  `json:"side"`
	Status      string  `json:"status"`
	FilledQty   float64 `json:"filled_qty,omitempty"`
	AvgPrice    float64 `json:"avg_price,omitempty"`
	Reason      string  `json:"reason,omitempty"`
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

// NewWSEnvelope 创建新的消息封装
func NewWSEnvelope(eventType EventType, data interface{}) *WSEnvelope {
	return &WSEnvelope{
		Type:      eventType,
		Timestamp: time.Now(),
		Data:      data,
	}
}
package types

import (
	"time"
)

// OrderType 订单类型
type OrderType string

const (
	OrderTypeLimit  OrderType = "limit"  // 限价单
	OrderTypeMarket OrderType = "market" // 市价单
)

// OrderSide 订单方向
type OrderSide string

const (
	OrderSideBuy  OrderSide = "buy"  // 买入
	OrderSideSell OrderSide = "sell" // 卖出
)

// OrderStatus 订单状态
type OrderStatus string

const (
	OrderStatusPending   OrderStatus = "pending"   // 待成交
	OrderStatusFilled    OrderStatus = "filled"    // 完全成交
	OrderStatusPartially OrderStatus = "partially" // 部分成交
	OrderStatusCancelled OrderStatus = "cancelled" // 已取消
	OrderStatusFailed    OrderStatus = "failed"    // 失败
)

// Order 订单
type Order struct {
	ID           string                 `json:"id"`
	Symbol       string                 `json:"symbol"`
	Side         OrderSide              `json:"side"`
	Type         OrderType              `json:"type"`
	Quantity     float64                `json:"quantity"`
	Price        float64                `json:"price,omitempty"` // 限价单必填
	AveragePrice float64                `json:"average_price,omitempty"`
	FilledQty    float64                `json:"filled_qty,omitempty"`
	Status       OrderStatus            `json:"status"`
	Leverage     int                    `json:"leverage"`
	Timestamp    time.Time              `json:"timestamp"`
	ClientID     string                 `json:"client_id,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// OrderResult 订单结果
type OrderResult struct {
	OrderID      string      `json:"order_id"`
	ClientID     string      `json:"client_id,omitempty"`
	Symbol       string      `json:"symbol"`
	Side         OrderSide   `json:"side"`
	Type         OrderType   `json:"type"`
	Quantity     float64     `json:"quantity"`
	Price        float64     `json:"price,omitempty"`
	Status       OrderStatus `json:"status"`
	Timestamp    time.Time   `json:"timestamp"`
	Error        string      `json:"error,omitempty"`
}

// Trade 交易记录
type Trade struct {
	ID          string    `json:"id"`
	OrderID     string    `json:"order_id"`
	Symbol      string    `json:"symbol"`
	Side        OrderSide `json:"side"`
	Price       float64   `json:"price"`
	Quantity    float64   `json:"quantity"`
	Fee         float64   `json:"fee"`
	Timestamp   time.Time `json:"timestamp"`
	IsMaker     bool      `json:"is_maker"`
	OrderType   OrderType `json:"order_type"`
}

// Position 仓位
type Position struct {
	Symbol         string    `json:"symbol"`
	Side           OrderSide `json:"side"`
	Size           float64   `json:"size"`
	EntryPrice     float64   `json:"entry_price"`
	MarkPrice      float64   `json:"mark_price"`
	UnrealizedPnL  float64   `json:"unrealized_pnl"`
	Leverage       int       `json:"leverage"`
	LiquidationPrice float64 `json:"liquidation_price"`
	Timestamp      time.Time `json:"timestamp"`
}

// Tick 行情快照
type Tick struct {
	Symbol     string    `json:"symbol"`
	Price      float64   `json:"price"`
	Size       float64   `json:"size"`
	Side       OrderSide `json:"side,omitempty"`
	Timestamp  time.Time `json:"timestamp"`
	Volume24h  float64   `json:"volume_24h,omitempty"`
	Open24h    float64   `json:"open_24h,omitempty"`
	High24h    float64   `json:"high_24h,omitempty"`
	Low24h     float64   `json:"low_24h,omitempty"`
}

// Bar K线数据
type Bar struct {
	Symbol    string    `json:"symbol"`
	Open      float64   `json:"open"`
	High      float64   `json:"high"`
	Low       float64   `json:"low"`
	Close     float64   `json:"close"`
	Volume    float64   `json:"volume"`
	Timestamp time.Time `json:"timestamp"`
	Interval  string    `json:"interval"` // 时间周期，如 "1m", "5m", "1h"
}

// OrderBookLevel 订单簿层级
type OrderBookLevel struct {
	Price  float64 `json:"price"`
	Size   float64 `json:"size"`
}

// OrderBook 订单簿
type OrderBook struct {
	Symbol    string           `json:"symbol"`
	Asks      []OrderBookLevel `json:"asks"` // 卖单
	Bids      []OrderBookLevel `json:"bids"` // 买单
	Timestamp time.Time        `json:"timestamp"`
	Checksum  int64            `json:"checksum,omitempty"`
}

// Balance 余额
type Balance struct {
	Currency   string  `json:"currency"`
	Total      float64 `json:"total"`
	Available  float64 `json:"available"`
	Frozen     float64 `json:"frozen"`
	Timestamp  time.Time `json:"timestamp"`
}

// Account 账户信息
type Account struct {
	TotalEquity     float64            `json:"total_equity"`
	TotalMargin     float64            `json:"total_margin"`
	TotalAvailable  float64            `json:"total_available"`
	TotalPnL        float64            `json:"total_pnl"`
	TotalUnrealizedPnL float64         `json:"total_unrealized_pnl"`
	TotalRealizedPnL   float64         `json:"total_realized_pnl"`
	Balance          map[string]Balance `json:"balance"`
	Positions        []*Position        `json:"positions"`
	Timestamp        time.Time         `json:"timestamp"`
}

// SignalType 信号类型
type SignalType string

const (
	SignalTypeBuy      SignalType = "buy"      // 买入信号
	SignalTypeSell     SignalType = "sell"     // 卖出信号
	SignalTypeHold     SignalType = "hold"     // 持有信号
	SignalTypeExit     SignalType = "exit"     // 退出信号
)

// Signal 交易信号
type Signal struct {
	Strategy   string      `json:"strategy"`
	Symbol     string      `json:"symbol"`
	Type       SignalType  `json:"type"`
	Price      float64     `json:"price,omitempty"`
	Quantity   float64     `json:"quantity,omitempty"`
	Confidence float64     `json:"confidence,omitempty"` // 信号置信度 (0-1)
	Weight     float64     `json:"weight,omitempty"`     // 策略权重 (0-1)
	Timestamp  time.Time   `json:"timestamp"`
	Metadata   interface{} `json:"metadata,omitempty"` // 额外信息
}

// StrategyParams 策略参数
type StrategyParams map[string]interface{}

// TimeFrame 时间周期
type TimeFrame string

const (
	TimeFrame1m  TimeFrame = "1m"  // 1分钟
	TimeFrame5m  TimeFrame = "5m"  // 5分钟
	TimeFrame15m TimeFrame = "15m" // 15分钟
	TimeFrame1h  TimeFrame = "1h"  // 1小时
	TimeFrame4h  TimeFrame = "4h"  // 4小时
	TimeFrame1d  TimeFrame = "1d"  // 1天
)

// Error 错误类型
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Response API响应
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Config 通用配置
type Config map[string]interface{}

// AlgoOrderType 算法单类型
type AlgoOrderType string

const (
	AlgoOrderConditional AlgoOrderType = "conditional" // 条件单（止盈止损）
)

// AlgoOrder 算法单（条件单/止盈止损）
type AlgoOrder struct {
	AlgoID      string        `json:"algo_id"`
	Symbol      string        `json:"symbol"`
	Side        OrderSide     `json:"side"`
	OrdType     AlgoOrderType `json:"ord_type"`
	Size        float64       `json:"size"`
	SlTriggerPx float64       `json:"sl_trigger_px"` // 止损触发价
	SlOrderPx   float64       `json:"sl_order_px"`   // 止损委托价（-1=市价）
	TpTriggerPx float64       `json:"tp_trigger_px"` // 止盈触发价
	TpOrderPx   float64       `json:"tp_order_px"`   // 止盈委托价（-1=市价）
	CloseFraction float64     `json:"close_fraction"` // 平仓比例（0-1）
	ClientID    string        `json:"client_id"`
	TdMode      string        `json:"td_mode"` // 保证金模式
	State       string        `json:"state"`   // 状态
}

// AlgoOrderResult 算法单下单结果
type AlgoOrderResult struct {
	AlgoID    string    `json:"algo_id"`
	Symbol    string    `json:"symbol"`
	ClientID  string    `json:"client_id"`
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
}

// FundingRate 资金费率数据
type FundingRate struct {
	InstId           string    `json:"instId"`
	FundingRate      float64   `json:"fundingRate"`
	NextFundingRate  float64   `json:"nextFundingRate"`
	NextSettlementTime time.Time `json:"nextFundingTime"`
	Timestamp        time.Time `json:"timestamp"`
}

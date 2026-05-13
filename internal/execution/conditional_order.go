package execution

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/ljwqf/quant/internal/exchange"
	"github.com/ljwqf/quant/pkg/logger"
	"github.com/ljwqf/quant/pkg/types"
	"go.uber.org/zap"
)

// ConditionalOrderType 条件单类型
type ConditionalOrderType string

const (
	ConditionalTypeStopLoss     ConditionalOrderType = "stop_loss"      // 止损单
	ConditionalTypeTakeProfit   ConditionalOrderType = "take_profit"    // 止盈单
	ConditionalTypeTrailingStop ConditionalOrderType = "trailing_stop"  // 移动止损
	ConditionalTypeBreakout     ConditionalOrderType = "breakout"       // 突破单
)

// ConditionalOrderStatus 条件单状态
type ConditionalOrderStatus string

const (
	ConditionalStatusPending   ConditionalOrderStatus = "pending"   // 待触发
	ConditionalStatusTriggered ConditionalOrderStatus = "triggered" // 已触发
	ConditionalStatusCancelled ConditionalOrderStatus = "cancelled" // 已取消
	ConditionalStatusExpired   ConditionalOrderStatus = "expired"   // 已过期
)

// ConditionalOrder 条件单
type ConditionalOrder struct {
	ID             string                 `json:"id"`
	Type           ConditionalOrderType   `json:"type"`
	Symbol         string                 `json:"symbol"`
	Side           types.OrderSide        `json:"side"`
	Quantity       float64                `json:"quantity"`
	TriggerPrice   float64                `json:"trigger_price"`   // 触发价格
	OrderPrice     float64                `json:"order_price"`     // 下单价格 (0=市价)
	OrderType      types.OrderType        `json:"order_type"`      // 触发后的订单类型
	Status         ConditionalOrderStatus `json:"status"`
	CreatedAt      time.Time              `json:"created_at"`
	TriggeredAt    time.Time              `json:"triggered_at,omitempty"`
	TriggeredOrderID string               `json:"triggered_order_id,omitempty"` // 触发后的订单ID
	PositionID     string                 `json:"position_id,omitempty"`        // 关联的持仓ID
	Strategy       string                 `json:"strategy,omitempty"`           // 关联的策略
	ExpiresAt      *time.Time             `json:"expires_at,omitempty"`         // 过期时间

	// 移动止损专用字段
	TrailingPercent float64 `json:"trailing_percent,omitempty"` // 回撤百分比
	HighestPrice    float64 `json:"highest_price,omitempty"`    // 最高价追踪（做多移动止损）
	LowestPrice     float64 `json:"lowest_price,omitempty"`     // 最低价追踪（做空移动止损）
	ActivationPrice float64 `json:"activation_price,omitempty"` // 激活价格
	Activated       bool    `json:"activated,omitempty"`        // 是否已激活

	// 元数据
	Metadata map[string]interface{} `json:"metadata,omitempty"`

	// 触发回调
	onTrigger func(*ConditionalOrder, *types.OrderResult)
}

// ConditionalOrderManager 条件单管理器
type ConditionalOrderManager struct {
	orders    map[string]*ConditionalOrder
	ordersBySymbol map[string]map[string]*ConditionalOrder // 按符号索引
	exchange  exchange.Exchange
	mu        sync.RWMutex
	nowFunc   func() time.Time // 可注入的时间函数
}

// NewConditionalOrderManager 创建条件单管理器
func NewConditionalOrderManager(ex exchange.Exchange) *ConditionalOrderManager {
	return &ConditionalOrderManager{
		orders:         make(map[string]*ConditionalOrder),
		ordersBySymbol: make(map[string]map[string]*ConditionalOrder),
		exchange:       ex,
		nowFunc:        time.Now,
	}
}

// SetNowFunc 设置时间函数（用于测试）
func (m *ConditionalOrderManager) SetNowFunc(fn func() time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nowFunc = fn
}

// generateOrderID 生成订单ID
func generateConditionalOrderID() string {
	return fmt.Sprintf("cond_%s", uuid.New().String()[:8])
}

// AddStopLoss 添加止损单
func (m *ConditionalOrderManager) AddStopLoss(
	symbol string,
	side types.OrderSide,
	quantity float64,
	stopPrice float64,
	opts ...ConditionalOrderOption,
) (*ConditionalOrder, error) {
	if quantity <= 0 {
		return nil, fmt.Errorf("quantity must be positive")
	}
	if stopPrice <= 0 {
		return nil, fmt.Errorf("stop price must be positive")
	}

	order := &ConditionalOrder{
		ID:           generateConditionalOrderID(),
		Type:         ConditionalTypeStopLoss,
		Symbol:       symbol,
		Side:         side,
		Quantity:     quantity,
		TriggerPrice: stopPrice,
		OrderType:    types.OrderTypeMarket,
		Status:       ConditionalStatusPending,
		CreatedAt:    m.nowFunc(),
	}

	// 应用可选配置
	for _, opt := range opts {
		opt(order)
	}

	m.mu.Lock()
	m.addOrderLocked(order)
	m.mu.Unlock()

	logger.Info("止损单已创建",
		zap.String("order_id", order.ID),
		zap.String("symbol", symbol),
		zap.String("side", string(side)),
		zap.Float64("stop_price", stopPrice),
		zap.Float64("quantity", quantity),
	)

	return order, nil
}

// AddTakeProfit 添加止盈单
func (m *ConditionalOrderManager) AddTakeProfit(
	symbol string,
	side types.OrderSide,
	quantity float64,
	takeProfitPrice float64,
	opts ...ConditionalOrderOption,
) (*ConditionalOrder, error) {
	if quantity <= 0 {
		return nil, fmt.Errorf("quantity must be positive")
	}
	if takeProfitPrice <= 0 {
		return nil, fmt.Errorf("take profit price must be positive")
	}

	order := &ConditionalOrder{
		ID:           generateConditionalOrderID(),
		Type:         ConditionalTypeTakeProfit,
		Symbol:       symbol,
		Side:         side,
		Quantity:     quantity,
		TriggerPrice: takeProfitPrice,
		OrderType:    types.OrderTypeMarket,
		Status:       ConditionalStatusPending,
		CreatedAt:    m.nowFunc(),
	}

	for _, opt := range opts {
		opt(order)
	}

	m.mu.Lock()
	m.addOrderLocked(order)
	m.mu.Unlock()

	logger.Info("止盈单已创建",
		zap.String("order_id", order.ID),
		zap.String("symbol", symbol),
		zap.String("side", string(side)),
		zap.Float64("take_profit_price", takeProfitPrice),
		zap.Float64("quantity", quantity),
	)

	return order, nil
}

// AddTrailingStop 添加移动止损
func (m *ConditionalOrderManager) AddTrailingStop(
	symbol string,
	side types.OrderSide,
	quantity float64,
	trailingPercent float64,
	activationPrice float64,
	opts ...ConditionalOrderOption,
) (*ConditionalOrder, error) {
	if quantity <= 0 {
		return nil, fmt.Errorf("quantity must be positive")
	}
	if trailingPercent <= 0 || trailingPercent >= 1 {
		return nil, fmt.Errorf("trailing percent must be between 0 and 1")
	}

	order := &ConditionalOrder{
		ID:              generateConditionalOrderID(),
		Type:            ConditionalTypeTrailingStop,
		Symbol:          symbol,
		Side:            side,
		Quantity:        quantity,
		TrailingPercent: trailingPercent,
		ActivationPrice: activationPrice,
		TriggerPrice:    activationPrice, // 初始触发价格为激活价格
		OrderType:       types.OrderTypeMarket,
		Status:          ConditionalStatusPending,
		CreatedAt:       m.nowFunc(),
	}

	// 初始化追踪价格
	if side == types.OrderSideSell {
		// 做多移动止损：追踪最高价
		order.HighestPrice = activationPrice
	} else {
		// 做空移动止损：追踪最低价
		order.LowestPrice = activationPrice
	}

	for _, opt := range opts {
		opt(order)
	}

	m.mu.Lock()
	m.addOrderLocked(order)
	m.mu.Unlock()

	logger.Info("移动止损已创建",
		zap.String("order_id", order.ID),
		zap.String("symbol", symbol),
		zap.String("side", string(side)),
		zap.Float64("trailing_percent", trailingPercent*100),
		zap.Float64("activation_price", activationPrice),
		zap.Float64("quantity", quantity),
	)

	return order, nil
}

// AddBreakout 添加突破单
func (m *ConditionalOrderManager) AddBreakout(
	symbol string,
	side types.OrderSide,
	quantity float64,
	breakoutPrice float64,
	orderPrice float64,
	opts ...ConditionalOrderOption,
) (*ConditionalOrder, error) {
	if quantity <= 0 {
		return nil, fmt.Errorf("quantity must be positive")
	}
	if breakoutPrice <= 0 {
		return nil, fmt.Errorf("breakout price must be positive")
	}

	order := &ConditionalOrder{
		ID:           generateConditionalOrderID(),
		Type:         ConditionalTypeBreakout,
		Symbol:       symbol,
		Side:         side,
		Quantity:     quantity,
		TriggerPrice: breakoutPrice,
		OrderPrice:   orderPrice,
		OrderType:    types.OrderTypeLimit,
		Status:       ConditionalStatusPending,
		CreatedAt:    m.nowFunc(),
	}

	for _, opt := range opts {
		opt(order)
	}

	m.mu.Lock()
	m.addOrderLocked(order)
	m.mu.Unlock()

	logger.Info("突破单已创建",
		zap.String("order_id", order.ID),
		zap.String("symbol", symbol),
		zap.String("side", string(side)),
		zap.Float64("breakout_price", breakoutPrice),
		zap.Float64("order_price", orderPrice),
		zap.Float64("quantity", quantity),
	)

	return order, nil
}

// addOrderLocked 添加订单（需要已持有锁）
func (m *ConditionalOrderManager) addOrderLocked(order *ConditionalOrder) {
	m.orders[order.ID] = order

	if _, ok := m.ordersBySymbol[order.Symbol]; !ok {
		m.ordersBySymbol[order.Symbol] = make(map[string]*ConditionalOrder)
	}
	m.ordersBySymbol[order.Symbol][order.ID] = order
}

// Cancel 取消条件单
func (m *ConditionalOrderManager) Cancel(orderID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	order, ok := m.orders[orderID]
	if !ok {
		return fmt.Errorf("order not found: %s", orderID)
	}

	if order.Status != ConditionalStatusPending {
		return fmt.Errorf("cannot cancel order with status: %s", order.Status)
	}

	order.Status = ConditionalStatusCancelled
	logger.Info("条件单已取消", zap.String("order_id", orderID))

	return nil
}

// CancelByPosition 取消指定持仓的所有条件单
func (m *ConditionalOrderManager) CancelByPosition(positionID string) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	cancelled := 0
	for _, order := range m.orders {
		if order.PositionID == positionID && order.Status == ConditionalStatusPending {
			order.Status = ConditionalStatusCancelled
			cancelled++
		}
	}

	logger.Info("按持仓取消条件单",
		zap.String("position_id", positionID),
		zap.Int("cancelled", cancelled),
	)

	return cancelled
}

// GetOrder 获取条件单
func (m *ConditionalOrderManager) GetOrder(orderID string) (*ConditionalOrder, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	order, ok := m.orders[orderID]
	return order, ok
}

// GetPendingOrders 获取所有待触发订单
func (m *ConditionalOrderManager) GetPendingOrders() []*ConditionalOrder {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*ConditionalOrder, 0)
	for _, order := range m.orders {
		if order.Status == ConditionalStatusPending {
			result = append(result, order)
		}
	}
	return result
}

// GetOrdersBySymbol 获取指定符号的所有待触发订单
func (m *ConditionalOrderManager) GetOrdersBySymbol(symbol string) []*ConditionalOrder {
	m.mu.RLock()
	defer m.mu.RUnlock()

	orders, ok := m.ordersBySymbol[symbol]
	if !ok {
		return nil
	}

	result := make([]*ConditionalOrder, 0, len(orders))
	for _, order := range orders {
		if order.Status == ConditionalStatusPending {
			result = append(result, order)
		}
	}
	return result
}

// OnPriceUpdate 价格更新时检查触发条件
func (m *ConditionalOrderManager) OnPriceUpdate(symbol string, price float64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	orders, ok := m.ordersBySymbol[symbol]
	if !ok {
		return
	}

	now := m.nowFunc()

	for _, order := range orders {
		if order.Status != ConditionalStatusPending {
			continue
		}

		// 检查过期
		if order.ExpiresAt != nil && now.After(*order.ExpiresAt) {
			order.Status = ConditionalStatusExpired
			logger.Info("条件单已过期", zap.String("order_id", order.ID))
			continue
		}

		// 根据类型检查触发条件
		switch order.Type {
		case ConditionalTypeStopLoss:
			m.checkStopLossLocked(order, price)
		case ConditionalTypeTakeProfit:
			m.checkTakeProfitLocked(order, price)
		case ConditionalTypeTrailingStop:
			m.checkTrailingStopLocked(order, price)
		case ConditionalTypeBreakout:
			m.checkBreakoutLocked(order, price)
		}
	}
}

// checkStopLossLocked 检查止损触发（需要已持有锁）
func (m *ConditionalOrderManager) checkStopLossLocked(order *ConditionalOrder, price float64) {
	triggered := false

	// 做多止损：价格跌破止损价
	if order.Side == types.OrderSideSell && price <= order.TriggerPrice {
		triggered = true
	}
	// 做空止损：价格涨破止损价
	if order.Side == types.OrderSideBuy && price >= order.TriggerPrice {
		triggered = true
	}

	if triggered {
		m.triggerOrderLocked(order)
	}
}

// checkTakeProfitLocked 检查止盈触发（需要已持有锁）
func (m *ConditionalOrderManager) checkTakeProfitLocked(order *ConditionalOrder, price float64) {
	triggered := false

	// 做多止盈：价格涨破止盈价
	if order.Side == types.OrderSideSell && price >= order.TriggerPrice {
		triggered = true
	}
	// 做空止盈：价格跌破止盈价
	if order.Side == types.OrderSideBuy && price <= order.TriggerPrice {
		triggered = true
	}

	if triggered {
		m.triggerOrderLocked(order)
	}
}

// checkTrailingStopLocked 检查移动止损触发（需要已持有锁）
func (m *ConditionalOrderManager) checkTrailingStopLocked(order *ConditionalOrder, price float64) {
	// 检查是否需要激活
	if !order.Activated {
		if order.Side == types.OrderSideSell && price >= order.ActivationPrice {
			// 做多移动止损：价格涨过激活价时激活
			order.Activated = true
			order.HighestPrice = price
			logger.Info("移动止损已激活",
				zap.String("order_id", order.ID),
				zap.Float64("activation_price", order.ActivationPrice),
				zap.Float64("current_price", price),
			)
		} else if order.Side == types.OrderSideBuy && price <= order.ActivationPrice {
			// 做空移动止损：价格跌破激活价时激活
			order.Activated = true
			order.LowestPrice = price
			logger.Info("移动止损已激活",
				zap.String("order_id", order.ID),
				zap.Float64("activation_price", order.ActivationPrice),
				zap.Float64("current_price", price),
			)
		} else {
			// 未激活且不满足激活条件
			return
		}
	}

	// 已激活，更新追踪价格并检查触发
	if order.Side == types.OrderSideSell {
		// 做多移动止损：追踪最高价
		if price > order.HighestPrice {
			order.HighestPrice = price
			logger.Debug("移动止损更新最高价",
				zap.String("order_id", order.ID),
				zap.Float64("new_highest", price),
			)
		}
		// 检查是否从最高价回撤超过阈值
		triggerPrice := order.HighestPrice * (1 - order.TrailingPercent)
		if price <= triggerPrice {
			logger.Info("移动止损触发",
				zap.String("order_id", order.ID),
				zap.Float64("highest", order.HighestPrice),
				zap.Float64("trigger_price", triggerPrice),
				zap.Float64("current_price", price),
			)
			m.triggerOrderLocked(order)
		}
	} else {
		// 做空移动止损：追踪最低价
		if price < order.LowestPrice {
			order.LowestPrice = price
			logger.Debug("移动止损更新最低价",
				zap.String("order_id", order.ID),
				zap.Float64("new_lowest", price),
			)
		}
		// 检查是否从最低价反弹超过阈值
		triggerPrice := order.LowestPrice * (1 + order.TrailingPercent)
		if price >= triggerPrice {
			logger.Info("移动止损触发",
				zap.String("order_id", order.ID),
				zap.Float64("lowest", order.LowestPrice),
				zap.Float64("trigger_price", triggerPrice),
				zap.Float64("current_price", price),
			)
			m.triggerOrderLocked(order)
		}
	}
}

// checkBreakoutLocked 检查突破单触发（需要已持有锁）
func (m *ConditionalOrderManager) checkBreakoutLocked(order *ConditionalOrder, price float64) {
	triggered := false

	// 向上突破：价格涨破突破价
	if order.Side == types.OrderSideBuy && price >= order.TriggerPrice {
		triggered = true
	}
	// 向下突破：价格跌破突破价
	if order.Side == types.OrderSideSell && price <= order.TriggerPrice {
		triggered = true
	}

	if triggered {
		m.triggerOrderLocked(order)
	}
}

// triggerOrderLocked 触发订单（需要已持有锁）
func (m *ConditionalOrderManager) triggerOrderLocked(order *ConditionalOrder) {
	if m.exchange == nil {
		logger.Error("无法触发条件单：交易所未设置", zap.String("order_id", order.ID))
		return
	}

	// 创建实际订单
	req := &types.Order{
		Symbol:   order.Symbol,
		Side:     order.Side,
		Type:     order.OrderType,
		Quantity: order.Quantity,
	}

	if order.OrderPrice > 0 {
		req.Price = order.OrderPrice
	}

	// 下单
	result, err := m.exchange.PlaceOrder(req)
	if err != nil {
		logger.Error("条件单触发失败",
			zap.String("order_id", order.ID),
			zap.Error(err),
		)
		return
	}

	// 更新状态
	order.Status = ConditionalStatusTriggered
	order.TriggeredAt = m.nowFunc()
	order.TriggeredOrderID = result.OrderID

	logger.Info("条件单已触发",
		zap.String("conditional_order_id", order.ID),
		zap.String("triggered_order_id", result.OrderID),
		zap.String("symbol", order.Symbol),
		zap.String("side", string(order.Side)),
		zap.Float64("quantity", order.Quantity),
	)

	// 触发回调
	if order.onTrigger != nil {
		order.onTrigger(order, result)
	}
}

// SetTriggerCallback 设置触发回调
func (o *ConditionalOrder) SetTriggerCallback(fn func(*ConditionalOrder, *types.OrderResult)) {
	o.onTrigger = fn
}

// ConditionalOrderOption 条件单可选配置
type ConditionalOrderOption func(*ConditionalOrder)

// WithPositionID 设置关联持仓ID
func WithPositionID(positionID string) ConditionalOrderOption {
	return func(o *ConditionalOrder) {
		o.PositionID = positionID
	}
}

// WithStrategy 设置关联策略
func WithStrategy(strategy string) ConditionalOrderOption {
	return func(o *ConditionalOrder) {
		o.Strategy = strategy
	}
}

// WithExpiresAt 设置过期时间
func WithExpiresAt(expiresAt time.Time) ConditionalOrderOption {
	return func(o *ConditionalOrder) {
		o.ExpiresAt = &expiresAt
	}
}

// WithOrderPrice 设置下单价格
func WithOrderPrice(price float64) ConditionalOrderOption {
	return func(o *ConditionalOrder) {
		o.OrderPrice = price
		o.OrderType = types.OrderTypeLimit
	}
}

// WithMetadata 设置元数据
func WithMetadata(metadata map[string]interface{}) ConditionalOrderOption {
	return func(o *ConditionalOrder) {
		o.Metadata = metadata
	}
}

// Stats 统计信息
type ConditionalOrderStats struct {
	Total       int `json:"total"`
	Pending     int `json:"pending"`
	Triggered   int `json:"triggered"`
	Cancelled   int `json:"cancelled"`
	Expired     int `json:"expired"`
	StopLoss    int `json:"stop_loss"`
	TakeProfit  int `json:"take_profit"`
	TrailingStop int `json:"trailing_stop"`
	Breakout    int `json:"breakout"`
}

// GetStats 获取统计信息
func (m *ConditionalOrderManager) GetStats() ConditionalOrderStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := ConditionalOrderStats{}
	for _, order := range m.orders {
		stats.Total++
		switch order.Status {
		case ConditionalStatusPending:
			stats.Pending++
		case ConditionalStatusTriggered:
			stats.Triggered++
		case ConditionalStatusCancelled:
			stats.Cancelled++
		case ConditionalStatusExpired:
			stats.Expired++
		}
		switch order.Type {
		case ConditionalTypeStopLoss:
			stats.StopLoss++
		case ConditionalTypeTakeProfit:
			stats.TakeProfit++
		case ConditionalTypeTrailingStop:
			stats.TrailingStop++
		case ConditionalTypeBreakout:
			stats.Breakout++
		}
	}
	return stats
}
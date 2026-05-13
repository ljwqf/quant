package manualtrading

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"

	"github.com/ljwqf/quant/internal/config"
	"github.com/ljwqf/quant/internal/exchange"
	"github.com/ljwqf/quant/internal/storage"
	"github.com/ljwqf/quant/pkg/logger"
	"github.com/ljwqf/quant/pkg/types"
	"go.uber.org/zap"
)

type ConditionalOrderType string

const (
	ConditionalOrderTypePrice  ConditionalOrderType = "price"
	ConditionalOrderTypeTime   ConditionalOrderType = "time"
	ConditionalOrderTypeVolume ConditionalOrderType = "volume"
)

type ConditionalOrderStatus string

const (
	ConditionalOrderStatusPending   ConditionalOrderStatus = "pending"
	ConditionalOrderStatusTriggered ConditionalOrderStatus = "triggered"
	ConditionalOrderStatusCancelled ConditionalOrderStatus = "cancelled"
	ConditionalOrderStatusExpired   ConditionalOrderStatus = "expired"
)

type ConditionalOrder struct {
	ID            string                 `json:"id"`
	Symbol        string                 `json:"symbol"`
	Side          types.OrderSide        `json:"side"`
	Size          float64                `json:"size"`
	Type          ConditionalOrderType   `json:"type"`
	Condition     map[string]interface{} `json:"condition"`
	OrderType     types.OrderType        `json:"order_type"`
	Price         float64                `json:"price,omitempty"`
	Status        ConditionalOrderStatus `json:"status"`
	CreatedAt     time.Time              `json:"created_at"`
	TriggeredAt   *time.Time             `json:"triggered_at,omitempty"`
	CancelledAt   *time.Time             `json:"cancelled_at,omitempty"`
	OrderID       string                 `json:"order_id,omitempty"`
	TriggerPrice  float64                `json:"trigger_price,omitempty"`
	Reason        string                 `json:"reason,omitempty"`
}

type ConditionalOrderManager struct {
	cfg         *config.ManualTradingConfig
	db          *storage.Database
	exchange    exchange.Exchange
	orders      map[string]*ConditionalOrder
	mu          sync.RWMutex
	stopCh      chan struct{}
	running     bool
}

func NewConditionalOrderManager(cfg *config.ManualTradingConfig, db *storage.Database, exchange exchange.Exchange) *ConditionalOrderManager {
	return &ConditionalOrderManager{
		cfg:      cfg,
		db:       db,
		exchange: exchange,
		orders:   make(map[string]*ConditionalOrder),
		stopCh:   make(chan struct{}),
	}
}

func (cm *ConditionalOrderManager) Start() {
	cm.mu.Lock()
	if cm.running {
		cm.mu.Unlock()
		return
	}
	cm.running = true
	cm.mu.Unlock()

	go cm.monitor()
	logger.Info("条件单监控器已启动")
}

func (cm *ConditionalOrderManager) Stop() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if !cm.running {
		return
	}

	close(cm.stopCh)
	cm.running = false
	logger.Info("条件单监控器已停止")
}

func (cm *ConditionalOrderManager) monitor() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			cm.checkAndExecute()
		case <-cm.stopCh:
			return
		}
	}
}

func (cm *ConditionalOrderManager) checkAndExecute() {
	cm.mu.RLock()
	var toCheck []*ConditionalOrder
	for _, order := range cm.orders {
		if order.Status == ConditionalOrderStatusPending {
			toCheck = append(toCheck, order)
		}
	}
	cm.mu.RUnlock()

	for _, order := range toCheck {
		shouldTrigger, triggerPrice := cm.checkCondition(order)
		if shouldTrigger {
			if err := cm.executeOrder(order, triggerPrice); err != nil {
				logger.Error("执行条件单失败",
					zap.String("id", order.ID),
					zap.String("symbol", order.Symbol),
					zap.Error(err),
				)
			}
		}
	}
}

func (cm *ConditionalOrderManager) checkCondition(order *ConditionalOrder) (bool, float64) {
	switch order.Type {
	case ConditionalOrderTypePrice:
		return cm.checkPriceCondition(order)
	case ConditionalOrderTypeTime:
		return cm.checkTimeCondition(order)
	case ConditionalOrderTypeVolume:
		return cm.checkVolumeCondition(order)
	default:
		return false, 0
	}
}

func (cm *ConditionalOrderManager) checkPriceCondition(order *ConditionalOrder) (bool, float64) {
	ticker, err := cm.exchange.GetTicker(order.Symbol)
	if err != nil {
		logger.Warn("获取行情失败，跳过条件单检查", zap.String("symbol", order.Symbol), zap.Error(err))
		return false, 0
	}

	currentPrice := ticker.Price
	condition := order.Condition

	if price, ok := condition["price"].(float64); ok {
		if direction, ok := condition["direction"].(string); ok {
			switch direction {
			case "above":
				if currentPrice >= price {
					return true, currentPrice
				}
			case "below":
				if currentPrice <= price {
					return true, currentPrice
				}
			}
		}
	}

	return false, 0
}

func (cm *ConditionalOrderManager) checkTimeCondition(order *ConditionalOrder) (bool, float64) {
	condition := order.Condition

	if executeAtStr, ok := condition["execute_at"].(string); ok {
		executeAt, err := time.Parse(time.RFC3339, executeAtStr)
		if err != nil {
			logger.Error("解析执行时间失败", zap.Error(err))
			return false, 0
		}

		if !executeAt.After(time.Now()) {
			ticker, err := cm.exchange.GetTicker(order.Symbol)
			if err != nil {
				logger.Warn("获取行情失败，使用当前时间触发", zap.String("symbol", order.Symbol), zap.Error(err))
				return true, 0
			}
			return true, ticker.Price
		}
	}

	return false, 0
}

func (cm *ConditionalOrderManager) checkVolumeCondition(order *ConditionalOrder) (bool, float64) {
	// 实现成交量条件检查
	// 这里可以根据需要实现成交量相关的条件检查
	return false, 0
}

func (cm *ConditionalOrderManager) executeOrder(order *ConditionalOrder, triggerPrice float64) error {
	logger.Info("执行条件单",
		zap.String("id", order.ID),
		zap.String("symbol", order.Symbol),
		zap.String("side", string(order.Side)),
		zap.Float64("size", order.Size),
		zap.Float64("trigger_price", triggerPrice),
	)

	tradeOrder := &types.Order{
		Symbol:   order.Symbol,
		Side:     order.Side,
		Type:     order.OrderType,
		Quantity: order.Size,
	}

	if order.OrderType == types.OrderTypeLimit {
		tradeOrder.Price = order.Price
	} else {
		tradeOrder.Price = triggerPrice
	}

	result, err := cm.exchange.PlaceOrder(tradeOrder)
	if err != nil {
		cm.mu.Lock()
		order.Status = ConditionalOrderStatusExpired
		order.Reason = err.Error()
		cm.mu.Unlock()
		return err
	}

	now := time.Now()
	cm.mu.Lock()
	order.Status = ConditionalOrderStatusTriggered
	order.TriggeredAt = &now
	order.OrderID = result.OrderID
	order.TriggerPrice = triggerPrice
	cm.mu.Unlock()

	logger.Info("条件单执行成功",
		zap.String("id", order.ID),
		zap.String("order_id", result.OrderID),
		zap.Float64("trigger_price", triggerPrice),
	)

	return nil
}

func (cm *ConditionalOrderManager) CreateOrder(symbol string, side types.OrderSide, size float64, orderType types.OrderType, conditionalType ConditionalOrderType, condition map[string]interface{}, price float64) (*ConditionalOrder, error) {
	if size <= 0 {
		return nil, ErrInvalidSize
	}

	order := &ConditionalOrder{
		ID:        generateConditionalOrderID(),
		Symbol:    symbol,
		Side:      side,
		Size:      size,
		Type:      conditionalType,
		Condition: condition,
		OrderType: orderType,
		Price:     price,
		Status:    ConditionalOrderStatusPending,
		CreatedAt: time.Now(),
	}

	cm.mu.Lock()
	cm.orders[order.ID] = order
	cm.mu.Unlock()

	logger.Info("创建条件单成功",
		zap.String("id", order.ID),
		zap.String("symbol", symbol),
		zap.String("side", string(side)),
		zap.Float64("size", size),
		zap.String("type", string(conditionalType)),
	)

	return order, nil
}

func (cm *ConditionalOrderManager) CancelOrder(id string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	order, exists := cm.orders[id]
	if !exists {
		return ErrOrderNotFound
	}

	if order.Status != ConditionalOrderStatusPending {
		return ErrOrderNotPending
	}

	now := time.Now()
	order.Status = ConditionalOrderStatusCancelled
	order.CancelledAt = &now

	logger.Info("取消条件单成功",
		zap.String("id", id),
		zap.String("symbol", order.Symbol),
	)

	return nil
}

func (cm *ConditionalOrderManager) GetOrder(id string) (*ConditionalOrder, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	order, exists := cm.orders[id]
	if !exists {
		return nil, ErrOrderNotFound
	}

	return order, nil
}

func (cm *ConditionalOrderManager) ListOrders(status ConditionalOrderStatus) []*ConditionalOrder {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	var result []*ConditionalOrder
	for _, order := range cm.orders {
		if status == "" || order.Status == status {
			result = append(result, order)
		}
	}

	sortConditionalOrdersByCreatedAt(result)
	return result
}

func (cm *ConditionalOrderManager) GetPendingOrders() []*ConditionalOrder {
	return cm.ListOrders(ConditionalOrderStatusPending)
}

func (cm *ConditionalOrderManager) RemoveOrder(id string) {
	cm.mu.Lock()
	delete(cm.orders, id)
	cm.mu.Unlock()
}

func generateConditionalOrderID() string {
	timestamp := time.Now().Format("20060102150405")
	randomBytes := make([]byte, 3)
	if _, err := rand.Read(randomBytes); err != nil {
		return "CO_" + timestamp + "_" + hex.EncodeToString(randomBytes)
	}
	return "CO_" + timestamp + "_" + hex.EncodeToString(randomBytes)
}

func sortConditionalOrdersByCreatedAt(orders []*ConditionalOrder) {
	for i := 0; i < len(orders)-1; i++ {
		for j := i + 1; j < len(orders); j++ {
			if orders[i].CreatedAt.After(orders[j].CreatedAt) {
				orders[i], orders[j] = orders[j], orders[i]
			}
		}
	}
}
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

type TimedOrderStatus string

const (
	TimedOrderStatusPending   TimedOrderStatus = "pending"
	TimedOrderStatusExecuted  TimedOrderStatus = "executed"
	TimedOrderStatusCancelled TimedOrderStatus = "cancelled"
	TimedOrderStatusExpired   TimedOrderStatus = "expired"
)

type TimedOrder struct {
	ID            string           `json:"id"`
	Symbol        string           `json:"symbol"`
	Side          types.OrderSide  `json:"side"`
	Size          float64          `json:"size"`
	ExecuteAt     time.Time        `json:"execute_at"`
	Status        TimedOrderStatus `json:"status"`
	CreatedAt     time.Time        `json:"created_at"`
	ExecutedAt    *time.Time       `json:"executed_at,omitempty"`
	CancelledAt   *time.Time       `json:"cancelled_at,omitempty"`
	OrderID       string           `json:"order_id,omitempty"`
	ExecutePrice  float64          `json:"execute_price,omitempty"`
	Reason        string           `json:"reason,omitempty"`
}

type TimedOrderManager struct {
	cfg         *config.ManualTradingConfig
	db          *storage.Database
	exchange    exchange.Exchange
	orders      map[string]*TimedOrder
	mu          sync.RWMutex
	stopCh      chan struct{}
	running     bool
}

func NewTimedOrderManager(cfg *config.ManualTradingConfig, db *storage.Database, exchange exchange.Exchange) *TimedOrderManager {
	return &TimedOrderManager{
		cfg:      cfg,
		db:       db,
		exchange: exchange,
		orders:   make(map[string]*TimedOrder),
		stopCh:   make(chan struct{}),
	}
}

func (tm *TimedOrderManager) Start() {
	tm.mu.Lock()
	if tm.running {
		tm.mu.Unlock()
		return
	}
	tm.running = true
	tm.mu.Unlock()

	go tm.monitor()
	logger.Info("限时单监控器已启动")
}

func (tm *TimedOrderManager) Stop() {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if !tm.running {
		return
	}

	close(tm.stopCh)
	tm.running = false
	logger.Info("限时单监控器已停止")
}

func (tm *TimedOrderManager) monitor() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			tm.checkAndExecute()
		case <-tm.stopCh:
			return
		}
	}
}

func (tm *TimedOrderManager) checkAndExecute() {
	now := time.Now()

	tm.mu.RLock()
	var toExecute []*TimedOrder
	for _, order := range tm.orders {
		if order.Status == TimedOrderStatusPending && !order.ExecuteAt.After(now) {
			toExecute = append(toExecute, order)
		}
	}
	tm.mu.RUnlock()

	for _, order := range toExecute {
		if err := tm.executeOrder(order); err != nil {
			logger.Error("执行限时单失败",
				zap.String("id", order.ID),
				zap.String("symbol", order.Symbol),
				zap.Error(err),
			)
		}
	}
}

func (tm *TimedOrderManager) executeOrder(order *TimedOrder) error {
	logger.Info("执行限时单",
		zap.String("id", order.ID),
		zap.String("symbol", order.Symbol),
		zap.String("side", string(order.Side)),
		zap.Float64("size", order.Size),
	)

	marketOrder := &types.Order{
		Symbol:   order.Symbol,
		Side:     order.Side,
		Type:     types.OrderTypeMarket,
		Quantity: order.Size,
	}

	result, err := tm.exchange.PlaceOrder(marketOrder)
	if err != nil {
		tm.mu.Lock()
		order.Status = TimedOrderStatusExpired
		order.Reason = err.Error()
		tm.mu.Unlock()
		return err
	}

	now := time.Now()
	tm.mu.Lock()
	order.Status = TimedOrderStatusExecuted
	order.ExecutedAt = &now
	order.OrderID = result.OrderID
	order.ExecutePrice = result.Price
	tm.mu.Unlock()

	logger.Info("限时单执行成功",
		zap.String("id", order.ID),
		zap.String("order_id", result.OrderID),
		zap.Float64("execute_price", result.Price),
	)

	return nil
}

func (tm *TimedOrderManager) CreateOrder(symbol string, side types.OrderSide, size float64, executeAt time.Time) (*TimedOrder, error) {
	if size <= 0 {
		return nil, ErrInvalidSize
	}

	if executeAt.Before(time.Now()) {
		return nil, ErrInvalidExecuteTime
	}

	order := &TimedOrder{
		ID:        generateTimedOrderID(),
		Symbol:    symbol,
		Side:      side,
		Size:      size,
		ExecuteAt: executeAt,
		Status:    TimedOrderStatusPending,
		CreatedAt: time.Now(),
	}

	tm.mu.Lock()
	tm.orders[order.ID] = order
	tm.mu.Unlock()

	logger.Info("创建限时单成功",
		zap.String("id", order.ID),
		zap.String("symbol", symbol),
		zap.String("side", string(side)),
		zap.Float64("size", size),
		zap.Time("execute_at", executeAt),
	)

	return order, nil
}

func (tm *TimedOrderManager) CancelOrder(id string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	order, exists := tm.orders[id]
	if !exists {
		return ErrOrderNotFound
	}

	if order.Status != TimedOrderStatusPending {
		return ErrOrderNotPending
	}

	now := time.Now()
	order.Status = TimedOrderStatusCancelled
	order.CancelledAt = &now

	logger.Info("取消限时单成功",
		zap.String("id", id),
		zap.String("symbol", order.Symbol),
	)

	return nil
}

func (tm *TimedOrderManager) GetOrder(id string) (*TimedOrder, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	order, exists := tm.orders[id]
	if !exists {
		return nil, ErrOrderNotFound
	}

	return order, nil
}

func (tm *TimedOrderManager) ListOrders(status TimedOrderStatus) []*TimedOrder {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	var result []*TimedOrder
	for _, order := range tm.orders {
		if status == "" || order.Status == status {
			result = append(result, order)
		}
	}

	sortTimedOrdersByExecuteAt(result)
	return result
}

func (tm *TimedOrderManager) GetPendingOrders() []*TimedOrder {
	return tm.ListOrders(TimedOrderStatusPending)
}

func (tm *TimedOrderManager) RemoveOrder(id string) {
	tm.mu.Lock()
	delete(tm.orders, id)
	tm.mu.Unlock()
}

func generateTimedOrderID() string {
	timestamp := time.Now().Format("20060102150405")
	randomBytes := make([]byte, 3)
	if _, err := rand.Read(randomBytes); err != nil {
		return "TO_" + timestamp + "_" + hex.EncodeToString(randomBytes)
	}
	return "TO_" + timestamp + "_" + hex.EncodeToString(randomBytes)
}

func randomString(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		for i := range b {
			b[i] = byte(time.Now().UnixNano() % 256)
		}
	}
	return hex.EncodeToString(b)[:n]
}

func sortTimedOrdersByExecuteAt(orders []*TimedOrder) {
	for i := 0; i < len(orders)-1; i++ {
		for j := i + 1; j < len(orders); j++ {
			if orders[i].ExecuteAt.After(orders[j].ExecuteAt) {
				orders[i], orders[j] = orders[j], orders[i]
			}
		}
	}
}

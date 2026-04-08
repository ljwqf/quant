package manualtrading

import (
	"sync"
	"testing"
	"time"

	"github.com/ljwqf/quant/internal/config"
	"github.com/ljwqf/quant/internal/storage"
	"github.com/ljwqf/quant/pkg/types"
	"github.com/stretchr/testify/assert"
)

// mockExchange 测试用模拟交易所
type mockExchange struct {
	mu          sync.Mutex
	tickers     map[string]float64
	orders      []*types.Order
	executed    []*types.OrderResult
	orderBook   map[string]*types.OrderBook
	bars        map[string][]*types.Bar
}

func (m *mockExchange) GetTicker(symbol string) (*types.Tick, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if price, ok := m.tickers[symbol]; ok {
		return &types.Tick{
			Symbol:    symbol,
			Price:     price,
			Timestamp: time.Now(),
		}, nil
	}
	return &types.Tick{
		Symbol:    symbol,
		Price:     0,
		Timestamp: time.Now(),
	}, nil
}

func (m *mockExchange) PlaceOrder(order *types.Order) (*types.OrderResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.orders = append(m.orders, order)
	result := &types.OrderResult{
		OrderID:   "test_order_" + order.Symbol + "_" + time.Now().Format("20060102150405"),
		Symbol:    order.Symbol,
		Side:      order.Side,
		Type:      order.Type,
		Quantity:  order.Quantity,
		Status:    types.OrderStatusFilled,
		Timestamp: time.Now(),
	}
	m.executed = append(m.executed, result)
	return result, nil
}

func (m *mockExchange) SetPrice(symbol string, price float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.tickers == nil {
		m.tickers = make(map[string]float64)
	}
	m.tickers[symbol] = price
}

func (m *mockExchange) getExecutedOrders() []*types.OrderResult {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.executed
}

// 实现 exchange.Exchange 接口的其他方法
func (m *mockExchange) Connect() error                                          { return nil }
func (m *mockExchange) Disconnect() error                                       { return nil }
func (m *mockExchange) GetAccount() (*types.Account, error)                     { return nil, nil }
func (m *mockExchange) CancelOrder(orderID string) error                        { return nil }
func (m *mockExchange) GetOrder(orderID string) (*types.Order, error)           { return nil, nil }
func (m *mockExchange) GetOrders(symbol string, limit int) ([]*types.Order, error) { return m.orders, nil }
func (m *mockExchange) GetPositions() ([]*types.Position, error)                { return nil, nil }
func (m *mockExchange) GetOrderBook(symbol string, depth int) (*types.OrderBook, error) { return nil, nil }
func (m *mockExchange) SubscribeTicker(symbol string, handler func(*types.Tick)) error { return nil }
func (m *mockExchange) UnsubscribeTicker(symbol string) error                   { return nil }
func (m *mockExchange) IsConnected() bool                                       { return true }
func (m *mockExchange) SubscribeBar(symbol string, interval string, handler func(*types.Bar)) error { return nil }
func (m *mockExchange) SubscribeOrderBook(symbol string, handler func(*types.OrderBook)) error { return nil }
func (m *mockExchange) GetBars(symbol string, interval string, limit int) ([]*types.Bar, error) { return nil, nil }
func (m *mockExchange) SetLeverage(symbol string, leverage int, marginMode string) error { return nil }

func TestConditionalOrderManager_CreateOrder(t *testing.T) {
	cfg := &config.ManualTradingConfig{}
	db := &storage.Database{}
	mockEx := &mockExchange{}
	manager := NewConditionalOrderManager(cfg, db, mockEx)

	// 创建价格条件单
	order, err := manager.CreateOrder(
		"BTC-USDT",
		types.OrderSideBuy,
		0.1,
		types.OrderTypeMarket,
		ConditionalOrderTypePrice,
		map[string]interface{}{
			"direction": "above",
			"price":     50000.0,
		},
		0,
	)

	assert.NoError(t, err)
	assert.NotNil(t, order)
	assert.Equal(t, ConditionalOrderTypePrice, order.Type)
	assert.Equal(t, "BTC-USDT", order.Symbol)
	assert.Equal(t, types.OrderSideBuy, order.Side)
	assert.Equal(t, ConditionalOrderStatusPending, order.Status)

	// 创建时间条件单
	order, err = manager.CreateOrder(
		"ETH-USDT",
		types.OrderSideSell,
		1.0,
		types.OrderTypeMarket,
		ConditionalOrderTypeTime,
		map[string]interface{}{
			"execute_at": time.Now().Add(24 * time.Hour).Format(time.RFC3339),
		},
		0,
	)

	assert.NoError(t, err)
	assert.NotNil(t, order)
	assert.Equal(t, ConditionalOrderTypeTime, order.Type)
	assert.Equal(t, "ETH-USDT", order.Symbol)
	assert.Equal(t, types.OrderSideSell, order.Side)
	assert.Equal(t, ConditionalOrderStatusPending, order.Status)
}

func TestConditionalOrderManager_CancelOrder(t *testing.T) {
	cfg := &config.ManualTradingConfig{}
	db := &storage.Database{}
	mockEx := &mockExchange{}
	manager := NewConditionalOrderManager(cfg, db, mockEx)

	// 创建条件单
	order, err := manager.CreateOrder(
		"BTC-USDT",
		types.OrderSideBuy,
		0.1,
		types.OrderTypeMarket,
		ConditionalOrderTypePrice,
		map[string]interface{}{
			"direction": "above",
			"price":     50000.0,
		},
		0,
	)
	assert.NoError(t, err)

	// 取消订单
	err = manager.CancelOrder(order.ID)
	assert.NoError(t, err)

	// 验证状态
	order, _ = manager.GetOrder(order.ID)
	assert.Equal(t, ConditionalOrderStatusCancelled, order.Status)
}

func TestConditionalOrderManager_ListOrders(t *testing.T) {
	cfg := &config.ManualTradingConfig{}
	db := &storage.Database{}
	mockEx := &mockExchange{}
	manager := NewConditionalOrderManager(cfg, db, mockEx)

	// 创建多个条件单
	manager.CreateOrder(
		"BTC-USDT",
		types.OrderSideBuy,
		0.1,
		types.OrderTypeMarket,
		ConditionalOrderTypePrice,
		map[string]interface{}{
			"direction": "above",
			"price":     50000.0,
		},
		0,
	)

	manager.CreateOrder(
		"ETH-USDT",
		types.OrderSideSell,
		1.0,
		types.OrderTypeMarket,
		ConditionalOrderTypeTime,
		map[string]interface{}{
			"execute_at": time.Now().Add(24 * time.Hour).Format(time.RFC3339),
		},
		0,
	)

	// 获取所有订单
	orders := manager.ListOrders("")
	assert.Len(t, orders, 2)

	// 获取待执行订单
	pendingOrders := manager.GetPendingOrders()
	assert.Len(t, pendingOrders, 2)
}

func TestConditionalOrderManager_StartStop(t *testing.T) {
	cfg := &config.ManualTradingConfig{}
	db := &storage.Database{}
	mockEx := &mockExchange{}
	manager := NewConditionalOrderManager(cfg, db, mockEx)

	// 启动管理器
	manager.Start()
	// 这里我们无法直接访问 running 字段，所以只测试方法调用

	// 停止管理器
	manager.Stop()
}


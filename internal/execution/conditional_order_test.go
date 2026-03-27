package execution

import (
	"sync"
	"testing"
	"time"

	"github.com/ljwqf/quant/pkg/types"
	"github.com/stretchr/testify/assert"
)

// mockExchangeForConditional 测试用模拟交易所
type mockExchangeForConditional struct {
	mu           sync.Mutex
	orders       []*types.Order
	orderResults []*types.OrderResult
	placeOrderFn func(*types.Order) (*types.OrderResult, error)
}

func (m *mockExchangeForConditional) PlaceOrder(order *types.Order) (*types.OrderResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.placeOrderFn != nil {
		return m.placeOrderFn(order)
	}

	m.orders = append(m.orders, order)
	result := &types.OrderResult{
		OrderID:   "test_order_" + order.Symbol,
		Symbol:    order.Symbol,
		Side:      order.Side,
		Type:      order.Type,
		Quantity:  order.Quantity,
		Status:    types.OrderStatusPending,
		Timestamp: time.Now(),
	}
	m.orderResults = append(m.orderResults, result)
	return result, nil
}

func (m *mockExchangeForConditional) getPlacedOrders() []*types.Order {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.orders
}

// 其他接口方法
func (m *mockExchangeForConditional) Connect() error                                          { return nil }
func (m *mockExchangeForConditional) Disconnect() error                                       { return nil }
func (m *mockExchangeForConditional) GetAccount() (*types.Account, error)                     { return nil, nil }
func (m *mockExchangeForConditional) CancelOrder(orderID string) error                        { return nil }
func (m *mockExchangeForConditional) GetOrder(orderID string) (*types.Order, error)           { return nil, nil }
func (m *mockExchangeForConditional) GetOrders(symbol string, limit int) ([]*types.Order, error) {
	return nil, nil
}
func (m *mockExchangeForConditional) GetPositions() ([]*types.Position, error)                { return nil, nil }
func (m *mockExchangeForConditional) GetTicker(symbol string) (*types.Tick, error)            { return nil, nil }
func (m *mockExchangeForConditional) GetOrderBook(symbol string, depth int) (*types.OrderBook, error) {
	return nil, nil
}
func (m *mockExchangeForConditional) SubscribeTicker(symbol string, handler func(*types.Tick)) error {
	return nil
}
func (m *mockExchangeForConditional) UnsubscribeTicker(symbol string) error                   { return nil }
func (m *mockExchangeForConditional) IsConnected() bool                                       { return true }
func (m *mockExchangeForConditional) SubscribeBar(symbol string, interval string, handler func(*types.Bar)) error {
	return nil
}
func (m *mockExchangeForConditional) SubscribeOrderBook(symbol string, handler func(*types.OrderBook)) error {
	return nil
}
func (m *mockExchangeForConditional) GetBars(symbol string, interval string, limit int) ([]*types.Bar, error) {
	return nil, nil
}
func (m *mockExchangeForConditional) SetLeverage(symbol string, leverage int, marginMode string) error {
	return nil
}

func TestConditionalOrderManager_AddStopLoss(t *testing.T) {
	mockEx := &mockExchangeForConditional{}
	manager := NewConditionalOrderManager(mockEx)

	order, err := manager.AddStopLoss(
		"BTC-USDT",
		types.OrderSideSell,
		0.1,
		50000.0,
		WithPositionID("pos_001"),
	)

	assert.NoError(t, err)
	assert.NotNil(t, order)
	assert.Equal(t, ConditionalTypeStopLoss, order.Type)
	assert.Equal(t, "BTC-USDT", order.Symbol)
	assert.Equal(t, types.OrderSideSell, order.Side)
	assert.Equal(t, 50000.0, order.TriggerPrice)
	assert.Equal(t, ConditionalStatusPending, order.Status)
	assert.Equal(t, "pos_001", order.PositionID)
}

func TestConditionalOrderManager_AddTakeProfit(t *testing.T) {
	mockEx := &mockExchangeForConditional{}
	manager := NewConditionalOrderManager(mockEx)

	order, err := manager.AddTakeProfit(
		"BTC-USDT",
		types.OrderSideSell,
		0.1,
		60000.0,
		WithStrategy("test_strategy"),
	)

	assert.NoError(t, err)
	assert.NotNil(t, order)
	assert.Equal(t, ConditionalTypeTakeProfit, order.Type)
	assert.Equal(t, 60000.0, order.TriggerPrice)
	assert.Equal(t, "test_strategy", order.Strategy)
}

func TestConditionalOrderManager_AddTrailingStop(t *testing.T) {
	mockEx := &mockExchangeForConditional{}
	manager := NewConditionalOrderManager(mockEx)

	order, err := manager.AddTrailingStop(
		"BTC-USDT",
		types.OrderSideSell,
		0.1,
		0.05, // 5% 回撤
		55000.0,
	)

	assert.NoError(t, err)
	assert.NotNil(t, order)
	assert.Equal(t, ConditionalTypeTrailingStop, order.Type)
	assert.Equal(t, 0.05, order.TrailingPercent)
	assert.Equal(t, 55000.0, order.ActivationPrice)
	assert.Equal(t, 55000.0, order.HighestPrice) // 初始化为激活价格
}

func TestConditionalOrderManager_AddBreakout(t *testing.T) {
	mockEx := &mockExchangeForConditional{}
	manager := NewConditionalOrderManager(mockEx)

	order, err := manager.AddBreakout(
		"BTC-USDT",
		types.OrderSideBuy,
		0.1,
		55000.0, // 突破价
		55100.0, // 限价
	)

	assert.NoError(t, err)
	assert.NotNil(t, order)
	assert.Equal(t, ConditionalTypeBreakout, order.Type)
	assert.Equal(t, 55000.0, order.TriggerPrice)
	assert.Equal(t, 55100.0, order.OrderPrice)
	assert.Equal(t, types.OrderTypeLimit, order.OrderType)
}

func TestConditionalOrderManager_StopLossTrigger(t *testing.T) {
	mockEx := &mockExchangeForConditional{}
	manager := NewConditionalOrderManager(mockEx)

	// 创建做多止损单（价格跌破触发卖出）
	order, err := manager.AddStopLoss(
		"BTC-USDT",
		types.OrderSideSell,
		0.1,
		50000.0,
	)
	assert.NoError(t, err)

	// 价格在止损价之上，不应触发
	manager.OnPriceUpdate("BTC-USDT", 51000.0)
	order, _ = manager.GetOrder(order.ID)
	assert.Equal(t, ConditionalStatusPending, order.Status)

	// 价格跌破止损价，应触发
	manager.OnPriceUpdate("BTC-USDT", 49000.0)
	order, _ = manager.GetOrder(order.ID)
	assert.Equal(t, ConditionalStatusTriggered, order.Status)

	// 验证订单已下单
	orders := mockEx.getPlacedOrders()
	assert.Len(t, orders, 1)
	assert.Equal(t, types.OrderSideSell, orders[0].Side)
}

func TestConditionalOrderManager_TakeProfitTrigger(t *testing.T) {
	mockEx := &mockExchangeForConditional{}
	manager := NewConditionalOrderManager(mockEx)

	// 创建做多止盈单（价格涨破触发卖出）
	order, err := manager.AddTakeProfit(
		"BTC-USDT",
		types.OrderSideSell,
		0.1,
		60000.0,
	)
	assert.NoError(t, err)

	// 价格在止盈价之下，不应触发
	manager.OnPriceUpdate("BTC-USDT", 59000.0)
	order, _ = manager.GetOrder(order.ID)
	assert.Equal(t, ConditionalStatusPending, order.Status)

	// 价格涨破止盈价，应触发
	manager.OnPriceUpdate("BTC-USDT", 61000.0)
	order, _ = manager.GetOrder(order.ID)
	assert.Equal(t, ConditionalStatusTriggered, order.Status)
}

func TestConditionalOrderManager_TrailingStopTrigger(t *testing.T) {
	mockEx := &mockExchangeForConditional{}
	manager := NewConditionalOrderManager(mockEx)

	// 创建做多移动止损（5%回撤）
	order, err := manager.AddTrailingStop(
		"BTC-USDT",
		types.OrderSideSell,
		0.1,
		0.05,
		55000.0, // 激活价格
	)
	assert.NoError(t, err)

	// 价格低于激活价，不激活
	manager.OnPriceUpdate("BTC-USDT", 54000.0)
	order, _ = manager.GetOrder(order.ID)
	assert.Equal(t, ConditionalStatusPending, order.Status)
	assert.False(t, order.Activated)

	// 价格涨到激活价，激活并开始追踪
	manager.OnPriceUpdate("BTC-USDT", 55000.0)
	order, _ = manager.GetOrder(order.ID)
	assert.Equal(t, ConditionalStatusPending, order.Status)
	assert.True(t, order.Activated)
	assert.Equal(t, 55000.0, order.HighestPrice)

	// 价格继续上涨，更新最高价
	manager.OnPriceUpdate("BTC-USDT", 58000.0)
	order, _ = manager.GetOrder(order.ID)
	assert.Equal(t, ConditionalStatusPending, order.Status)
	assert.Equal(t, 58000.0, order.HighestPrice)

	// 价格回调但未到触发点（58000 * 0.95 = 55100）
	manager.OnPriceUpdate("BTC-USDT", 55500.0)
	order, _ = manager.GetOrder(order.ID)
	assert.Equal(t, ConditionalStatusPending, order.Status)

	// 价格回撤超过5%，触发
	manager.OnPriceUpdate("BTC-USDT", 54900.0)
	order, _ = manager.GetOrder(order.ID)
	assert.Equal(t, ConditionalStatusTriggered, order.Status)
}

func TestConditionalOrderManager_BreakoutTrigger(t *testing.T) {
	mockEx := &mockExchangeForConditional{}
	manager := NewConditionalOrderManager(mockEx)

	// 创建向上突破单
	order, err := manager.AddBreakout(
		"BTC-USDT",
		types.OrderSideBuy,
		0.1,
		55000.0,
		55100.0,
	)
	assert.NoError(t, err)

	// 价格在突破价之下，不应触发
	manager.OnPriceUpdate("BTC-USDT", 54000.0)
	order, _ = manager.GetOrder(order.ID)
	assert.Equal(t, ConditionalStatusPending, order.Status)

	// 价格突破，应触发
	manager.OnPriceUpdate("BTC-USDT", 55100.0)
	order, _ = manager.GetOrder(order.ID)
	assert.Equal(t, ConditionalStatusTriggered, order.Status)

	// 验证限价单已下单
	orders := mockEx.getPlacedOrders()
	assert.Len(t, orders, 1)
	assert.Equal(t, 55100.0, orders[0].Price)
}

func TestConditionalOrderManager_Cancel(t *testing.T) {
	mockEx := &mockExchangeForConditional{}
	manager := NewConditionalOrderManager(mockEx)

	order, err := manager.AddStopLoss("BTC-USDT", types.OrderSideSell, 0.1, 50000.0)
	assert.NoError(t, err)

	// 取消订单
	err = manager.Cancel(order.ID)
	assert.NoError(t, err)

	// 验证状态
	order, _ = manager.GetOrder(order.ID)
	assert.Equal(t, ConditionalStatusCancelled, order.Status)

	// 已取消的订单不应触发
	manager.OnPriceUpdate("BTC-USDT", 49000.0)
	orders := mockEx.getPlacedOrders()
	assert.Len(t, orders, 0)
}

func TestConditionalOrderManager_CancelByPosition(t *testing.T) {
	mockEx := &mockExchangeForConditional{}
	manager := NewConditionalOrderManager(mockEx)

	// 创建多个关联同一持仓的订单
	manager.AddStopLoss("BTC-USDT", types.OrderSideSell, 0.1, 50000.0, WithPositionID("pos_001"))
	manager.AddTakeProfit("BTC-USDT", types.OrderSideSell, 0.1, 60000.0, WithPositionID("pos_001"))
	manager.AddStopLoss("ETH-USDT", types.OrderSideSell, 1.0, 3000.0, WithPositionID("pos_002"))

	// 取消 pos_001 的所有订单
	cancelled := manager.CancelByPosition("pos_001")
	assert.Equal(t, 2, cancelled)

	// 验证 pos_002 的订单还在
	pendingOrders := manager.GetPendingOrders()
	assert.Len(t, pendingOrders, 1)
	assert.Equal(t, "ETH-USDT", pendingOrders[0].Symbol)
}

func TestConditionalOrderManager_Expiration(t *testing.T) {
	mockEx := &mockExchangeForConditional{}
	manager := NewConditionalOrderManager(mockEx)

	// 设置可注入时间
	now := time.Now()
	manager.SetNowFunc(func() time.Time { return now })

	// 创建带过期时间的订单
	expiresAt := now.Add(1 * time.Hour)
	order, err := manager.AddStopLoss(
		"BTC-USDT",
		types.OrderSideSell,
		0.1,
		50000.0,
		WithExpiresAt(expiresAt),
	)
	assert.NoError(t, err)

	// 在过期时间之前，价格触发应正常工作
	manager.OnPriceUpdate("BTC-USDT", 49000.0)
	order, _ = manager.GetOrder(order.ID)
	assert.Equal(t, ConditionalStatusTriggered, order.Status)
}

func TestConditionalOrderManager_Stats(t *testing.T) {
	mockEx := &mockExchangeForConditional{}
	manager := NewConditionalOrderManager(mockEx)

	// 创建各种类型的订单
	manager.AddStopLoss("BTC-USDT", types.OrderSideSell, 0.1, 50000.0)
	manager.AddTakeProfit("BTC-USDT", types.OrderSideSell, 0.1, 60000.0)
	manager.AddTrailingStop("ETH-USDT", types.OrderSideSell, 1.0, 0.05, 3000.0)
	manager.AddBreakout("SOL-USDT", types.OrderSideBuy, 10.0, 100.0, 101.0)

	// 取消一个
	pending := manager.GetPendingOrders()
	manager.Cancel(pending[0].ID)

	stats := manager.GetStats()
	assert.Equal(t, 4, stats.Total)
	assert.Equal(t, 3, stats.Pending)
	assert.Equal(t, 1, stats.Cancelled)
	assert.Equal(t, 1, stats.StopLoss)
	assert.Equal(t, 1, stats.TakeProfit)
	assert.Equal(t, 1, stats.TrailingStop)
	assert.Equal(t, 1, stats.Breakout)
}

func TestConditionalOrderManager_GetOrdersBySymbol(t *testing.T) {
	mockEx := &mockExchangeForConditional{}
	manager := NewConditionalOrderManager(mockEx)

	manager.AddStopLoss("BTC-USDT", types.OrderSideSell, 0.1, 50000.0)
	manager.AddTakeProfit("BTC-USDT", types.OrderSideSell, 0.1, 60000.0)
	manager.AddStopLoss("ETH-USDT", types.OrderSideSell, 1.0, 3000.0)

	btcOrders := manager.GetOrdersBySymbol("BTC-USDT")
	assert.Len(t, btcOrders, 2)

	ethOrders := manager.GetOrdersBySymbol("ETH-USDT")
	assert.Len(t, ethOrders, 1)

	unknownOrders := manager.GetOrdersBySymbol("UNKNOWN")
	assert.Len(t, unknownOrders, 0)
}

func TestConditionalOrderManager_Validation(t *testing.T) {
	mockEx := &mockExchangeForConditional{}
	manager := NewConditionalOrderManager(mockEx)

	// 数量无效
	_, err := manager.AddStopLoss("BTC-USDT", types.OrderSideSell, 0, 50000.0)
	assert.Error(t, err)

	// 价格无效
	_, err = manager.AddStopLoss("BTC-USDT", types.OrderSideSell, 0.1, 0)
	assert.Error(t, err)

	// 回撤百分比无效
	_, err = manager.AddTrailingStop("BTC-USDT", types.OrderSideSell, 0.1, 1.5, 50000.0)
	assert.Error(t, err)
}
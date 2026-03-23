package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ljwqf/quant/internal/strategy"
	"github.com/ljwqf/quant/pkg/types"
)

type subscriptionExchangeStub struct {
	tickerHandler    func(*types.Tick)
	barHandler       func(*types.Bar)
	orderBookHandler func(*types.OrderBook)
}

func (s *subscriptionExchangeStub) Connect() error                      { return nil }
func (s *subscriptionExchangeStub) Disconnect() error                   { return nil }
func (s *subscriptionExchangeStub) GetAccount() (*types.Account, error) { return &types.Account{}, nil }
func (s *subscriptionExchangeStub) PlaceOrder(order *types.Order) (*types.OrderResult, error) {
	return nil, nil
}
func (s *subscriptionExchangeStub) CancelOrder(orderID string) error              { return nil }
func (s *subscriptionExchangeStub) GetOrder(orderID string) (*types.Order, error) { return nil, nil }
func (s *subscriptionExchangeStub) GetOrders(symbol string, limit int) ([]*types.Order, error) {
	return nil, nil
}
func (s *subscriptionExchangeStub) GetPositions() ([]*types.Position, error) { return nil, nil }
func (s *subscriptionExchangeStub) SubscribeTicker(symbol string, handler func(*types.Tick)) error {
	s.tickerHandler = handler
	return nil
}
func (s *subscriptionExchangeStub) SubscribeBar(symbol string, interval string, handler func(*types.Bar)) error {
	s.barHandler = handler
	return nil
}
func (s *subscriptionExchangeStub) SubscribeOrderBook(symbol string, handler func(*types.OrderBook)) error {
	s.orderBookHandler = handler
	return nil
}
func (s *subscriptionExchangeStub) GetBars(symbol string, interval string, limit int) ([]*types.Bar, error) {
	return nil, nil
}
func (s *subscriptionExchangeStub) GetTicker(symbol string) (*types.Tick, error) {
	return &types.Tick{Symbol: symbol, Price: 100}, nil
}
func (s *subscriptionExchangeStub) GetOrderBook(symbol string, depth int) (*types.OrderBook, error) {
	return nil, nil
}
func (s *subscriptionExchangeStub) SetLeverage(symbol string, leverage int, marginMode string) error {
	return nil
}

type subscriptionStrategyStub struct{}

func (s *subscriptionStrategyStub) Name() string                             { return "SubscriptionStub" }
func (s *subscriptionStrategyStub) Init(params map[string]interface{}) error { return nil }
func (s *subscriptionStrategyStub) OnTick(tick *types.Tick) (*types.Signal, error) {
	return &types.Signal{Symbol: tick.Symbol, Type: types.SignalTypeBuy, Timestamp: time.Now()}, nil
}
func (s *subscriptionStrategyStub) OnBar(bar *types.Bar) (*types.Signal, error) {
	return &types.Signal{Symbol: bar.Symbol, Type: types.SignalTypeBuy, Timestamp: time.Now()}, nil
}
func (s *subscriptionStrategyStub) OnOrderBook(orderBook *types.OrderBook) (*types.Signal, error) {
	return &types.Signal{Symbol: orderBook.Symbol, Type: types.SignalTypeBuy, Timestamp: time.Now()}, nil
}
func (s *subscriptionStrategyStub) GetParams() map[string]interface{}       { return nil }
func (s *subscriptionStrategyStub) SetParams(params map[string]interface{}) {}
func (s *subscriptionStrategyStub) GetMetrics() map[string]interface{}      { return nil }
func (s *subscriptionStrategyStub) OnPositionFilled(symbol string, side types.OrderSide, entryPrice, size float64) {
}
func (s *subscriptionStrategyStub) OnPositionReduced(symbol string, exitPrice, pnl, remainingSize float64) {
}
func (s *subscriptionStrategyStub) OnPositionClosed(symbol string, exitPrice, pnl float64) {}
func (s *subscriptionStrategyStub) ConfirmRebalanceEntry(request *strategy.RebalanceRequest) (*strategy.RebalanceDecision, error) {
	return &strategy.RebalanceDecision{RejectReason: "test_stub"}, nil
}

func TestSubscribeStrategyMarketDataDispatchesEachSignalOnce(t *testing.T) {
	exchange := &subscriptionExchangeStub{}
	strategyEngine := strategy.NewEngine()
	require.NoError(t, strategyEngine.AddStrategy("SubscriptionStub", &subscriptionStrategyStub{}, map[string]interface{}{}))

	executed := make([]string, 0, 3)
	err := subscribeStrategyMarketData(exchange, "BTC-USDT", "1m", strategyEngine, func(signal *types.Signal) {
		executed = append(executed, signal.Symbol+":"+string(signal.Type))
	})

	require.NoError(t, err)
	require.NotNil(t, exchange.tickerHandler)
	require.NotNil(t, exchange.barHandler)
	require.NotNil(t, exchange.orderBookHandler)

	exchange.tickerHandler(&types.Tick{Symbol: "BTC-USDT", Price: 100, Timestamp: time.Now()})
	exchange.barHandler(&types.Bar{Symbol: "BTC-USDT", Close: 100, Timestamp: time.Now()})
	exchange.orderBookHandler(&types.OrderBook{Symbol: "BTC-USDT", Timestamp: time.Now()})

	assert.Len(t, executed, 3)
	assert.Equal(t, []string{"BTC-USDT:buy", "BTC-USDT:buy", "BTC-USDT:buy"}, executed)
}

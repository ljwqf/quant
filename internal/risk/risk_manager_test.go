package risk

import (
	"testing"
	"time"

	"github.com/ljwqf/quant/internal/config"
	"github.com/ljwqf/quant/pkg/types"
	"github.com/stretchr/testify/assert"
)

type stubExchange struct {
	account    *types.Account
	ticker     *types.Tick
	accountErr error
	tickerErr  error
}

func (s *stubExchange) Connect() error    { return nil }
func (s *stubExchange) Disconnect() error { return nil }
func (s *stubExchange) GetAccount() (*types.Account, error) {
	if s.account != nil {
		return s.account, s.accountErr
	}
	return &types.Account{TotalEquity: 1000}, s.accountErr
}
func (s *stubExchange) PlaceOrder(order *types.Order) (*types.OrderResult, error)      { return nil, nil }
func (s *stubExchange) CancelOrder(orderID string) error                               { return nil }
func (s *stubExchange) GetOrder(orderID string) (*types.Order, error)                  { return nil, nil }
func (s *stubExchange) GetOrders(symbol string, limit int) ([]*types.Order, error)     { return nil, nil }
func (s *stubExchange) GetPositions() ([]*types.Position, error)                       { return nil, nil }
func (s *stubExchange) SubscribeTicker(symbol string, handler func(*types.Tick)) error { return nil }
func (s *stubExchange) SubscribeBar(symbol string, interval string, handler func(*types.Bar)) error {
	return nil
}
func (s *stubExchange) SubscribeOrderBook(symbol string, handler func(*types.OrderBook)) error {
	return nil
}
func (s *stubExchange) GetBars(symbol string, interval string, limit int) ([]*types.Bar, error) {
	return nil, nil
}
func (s *stubExchange) GetTicker(symbol string) (*types.Tick, error) {
	if s.ticker != nil {
		return s.ticker, s.tickerErr
	}
	return &types.Tick{Symbol: symbol, Price: 10}, s.tickerErr
}
func (s *stubExchange) GetOrderBook(symbol string, depth int) (*types.OrderBook, error) {
	return nil, nil
}
func (s *stubExchange) SetLeverage(symbol string, leverage int, marginMode string) error {
	return nil
}

func TestManagerCheckRiskRejectsInvalidSignal(t *testing.T) {
	manager := NewManager(testRiskConfig(), &stubExchange{})

	assert.ErrorIs(t, manager.CheckRisk(nil), ErrInvalidSignal)
	assert.ErrorIs(t, manager.CheckRisk(&types.Signal{Type: types.SignalTypeBuy, Symbol: "BTC-USDT"}), ErrInvalidSignal)
}

func TestManagerCheckRiskAllowsNonEntrySignals(t *testing.T) {
	manager := NewManager(testRiskConfig(), &stubExchange{})

	assert.NoError(t, manager.CheckRisk(&types.Signal{Type: types.SignalTypeHold, Symbol: "BTC-USDT"}))
	assert.NoError(t, manager.CheckRisk(&types.Signal{Type: types.SignalTypeExit, Symbol: "BTC-USDT"}))
}

func TestManagerCheckOrderResetsExpiredDailyState(t *testing.T) {
	manager := NewManager(testRiskConfig(), &stubExchange{account: &types.Account{TotalEquity: 1000}})
	manager.dailyLoss = manager.config.MaxDailyLoss
	manager.dailyTrades = manager.config.MaxTradesPerDay
	manager.dailyReset = time.Now().Add(-25 * time.Hour)

	err := manager.CheckOrder(&types.Order{Symbol: "BTC-USDT", Side: types.OrderSideBuy, Type: types.OrderTypeLimit, Quantity: 1, Price: 10})

	assert.NoError(t, err)
	assert.Equal(t, 0.0, manager.GetDailyLoss())
	assert.Equal(t, 0, manager.dailyTrades)
}

func TestManagerCheckOrderAllowsReducingExposure(t *testing.T) {
	manager := NewManager(testRiskConfig(), &stubExchange{account: &types.Account{TotalEquity: 1000}})
	manager.UpdatePosition(&types.Position{Symbol: "BTC-USDT", Side: types.OrderSideBuy, Size: 10, MarkPrice: 10})

	err := manager.CheckOrder(&types.Order{Symbol: "BTC-USDT", Side: types.OrderSideSell, Type: types.OrderTypeLimit, Quantity: 5, Price: 10})

	assert.NoError(t, err)
}

func TestManagerCheckRiskRejectsExcessiveDrawdown(t *testing.T) {
	manager := NewManager(testRiskConfig(), &stubExchange{account: &types.Account{TotalEquity: 70}})
	manager.peakEquity = 100

	err := manager.CheckRisk(&types.Signal{Type: types.SignalTypeBuy, Symbol: "BTC-USDT", Quantity: 1, Price: 10})

	assert.ErrorIs(t, err, ErrMaxDrawdownExceeded)
}

func TestManagerCalculateRiskExposureUsesSpotExposureWhenLeverageUnset(t *testing.T) {
	manager := NewManager(testRiskConfig(), &stubExchange{})
	manager.UpdatePosition(&types.Position{Symbol: "BTC-USDT", Side: types.OrderSideBuy, Size: 2, MarkPrice: 50, Leverage: 0})

	exposure, err := manager.CalculateRiskExposure()

	assert.NoError(t, err)
	assert.Equal(t, 100.0, exposure)
}

func TestManagerGetRiskMetricsIncludesExposureAndTrades(t *testing.T) {
	manager := NewManager(&config.RiskConfig{
		Enable:            true,
		MaxPositionSize:   200,
		MaxDailyLoss:      100,
		MaxDrawdown:       0.2,
		StopLossPercent:   0.05,
		TakeProfitPercent: 0.1,
		MaxTradesPerDay:   10,
	}, &stubExchange{})
	manager.UpdatePosition(&types.Position{Symbol: "BTC-USDT", Side: types.OrderSideBuy, Size: 1, MarkPrice: 100})
	manager.IncrementDailyTrades()

	metrics := manager.GetRiskMetrics()

	assert.Equal(t, 1, metrics["daily_trades"])
	assert.Equal(t, 100.0, metrics["total_exposure"])
}

func TestManagerCheckOrderUsesConfiguredMaxRiskPerTrade(t *testing.T) {
	manager := NewManager(&config.RiskConfig{
		Enable:               true,
		MaxPositionSize:      10000,
		MaxDailyLoss:         1000,
		MaxDrawdown:          0.2,
		StopLossPercent:      0.05,
		TakeProfitPercent:    0.1,
		MaxTradesPerDay:      100,
		MaxRiskPerTrade:      0.1,
		MaxExposurePerSymbol: 0.5,
	}, &stubExchange{account: &types.Account{TotalEquity: 1000}})

	err := manager.CheckOrder(&types.Order{Symbol: "BTC-USDT", Side: types.OrderSideBuy, Type: types.OrderTypeLimit, Quantity: 5, Price: 10})

	assert.NoError(t, err)
}

func TestManagerCheckOrderUsesDefaultMaxRiskPerTradeFallback(t *testing.T) {
	manager := NewManager(&config.RiskConfig{
		Enable:               true,
		MaxPositionSize:      10000,
		MaxDailyLoss:         1000,
		MaxDrawdown:          0.2,
		StopLossPercent:      0.05,
		TakeProfitPercent:    0.1,
		MaxTradesPerDay:      100,
		MaxRiskPerTrade:      0,
		MaxExposurePerSymbol: 0,
	}, &stubExchange{account: &types.Account{TotalEquity: 1000}})

	err := manager.CheckOrder(&types.Order{Symbol: "BTC-USDT", Side: types.OrderSideBuy, Type: types.OrderTypeLimit, Quantity: 5, Price: 10})

	assert.ErrorIs(t, err, ErrSingleTradeRiskExceeded)
}

func TestManagerCheckOrderUsesConfiguredMaxExposurePerSymbol(t *testing.T) {
	manager := NewManager(&config.RiskConfig{
		Enable:               true,
		MaxPositionSize:      10000,
		MaxDailyLoss:         1000,
		MaxDrawdown:          0.2,
		StopLossPercent:      0.05,
		TakeProfitPercent:    0.1,
		MaxTradesPerDay:      100,
		MaxRiskPerTrade:      0.3,
		MaxExposurePerSymbol: 0.2,
	}, &stubExchange{account: &types.Account{TotalEquity: 1000}})

	manager.UpdatePosition(&types.Position{Symbol: "BTC-USDT", Side: types.OrderSideBuy, Size: 10, MarkPrice: 10})
	err := manager.CheckOrder(&types.Order{Symbol: "BTC-USDT", Side: types.OrderSideBuy, Type: types.OrderTypeLimit, Quantity: 15, Price: 10})

	assert.ErrorIs(t, err, ErrSymbolExposureExceeded)
}

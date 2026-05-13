package risk

import (
	"sync"
	"testing"
	"time"

	"github.com/ljwqf/quant/internal/config"
	"github.com/ljwqf/quant/pkg/types"
	"github.com/stretchr/testify/assert"
)

func TestNewEngine(t *testing.T) {
	cfg := testRiskConfig()
	engine := NewEngine(cfg)

	assert.NotNil(t, engine)
	assert.Equal(t, cfg, engine.config)
	assert.NotNil(t, engine.positions)
	assert.NotNil(t, engine.strategyWeights)
}

func TestCheckRiskAllowsValidSignal(t *testing.T) {
	engine := NewEngine(testRiskConfig())

	signal := &types.Signal{
		Type:     types.SignalTypeBuy,
		Symbol:   "BTC-USDT",
		Price:    50000,
		Quantity: 0.001,
	}

	err := engine.CheckRisk(signal)
	assert.NoError(t, err)
}

func TestCheckRiskRejectsWhenDailyLossExceeded(t *testing.T) {
	cfg := testRiskConfig()
	cfg.MaxDailyLoss = 100
	engine := NewEngine(cfg)
	engine.UpdatePnL("BTC-USDT", -150)

	signal := &types.Signal{
		Type:     types.SignalTypeBuy,
		Symbol:   "BTC-USDT",
		Price:    50000,
		Quantity: 0.001,
	}

	err := engine.CheckRisk(signal)
	assert.ErrorIs(t, err, ErrDailyLossExceeded)
}

func TestCheckRiskRejectsWhenMaxTradesExceeded(t *testing.T) {
	cfg := testRiskConfig()
	cfg.MaxTradesPerDay = 5
	engine := NewEngine(cfg)
	for i := 0; i < 6; i++ {
		engine.IncrementTrade()
	}

	signal := &types.Signal{
		Type:     types.SignalTypeBuy,
		Symbol:   "BTC-USDT",
		Price:    50000,
		Quantity: 0.001,
	}

	err := engine.CheckRisk(signal)
	assert.Error(t, err)
}

func TestCheckRiskAllowsSellSignal(t *testing.T) {
	engine := NewEngine(testRiskConfig())

	signal := &types.Signal{
		Type:     types.SignalTypeSell,
		Symbol:   "BTC-USDT",
		Price:    50000,
		Quantity: 0.001,
	}

	err := engine.CheckRisk(signal)
	assert.NoError(t, err)
}

func TestUpdatePosition(t *testing.T) {
	engine := NewEngine(testRiskConfig())

	position := &types.Position{
		Symbol:     "BTC-USDT",
		Side:       types.OrderSideBuy,
		Size:       1.0,
		EntryPrice: 50000,
	}

	engine.UpdatePosition(position)
	assert.NotNil(t, engine.positions["BTC-USDT"])
}

func TestUpdatePositionReplacesExisting(t *testing.T) {
	engine := NewEngine(testRiskConfig())

	engine.UpdatePosition(&types.Position{
		Symbol:     "BTC-USDT",
		Side:       types.OrderSideBuy,
		Size:       1.0,
		EntryPrice: 50000,
	})

	engine.UpdatePosition(&types.Position{
		Symbol:     "BTC-USDT",
		Side:       types.OrderSideSell,
		Size:       2.0,
		EntryPrice: 55000,
	})

	pos := engine.positions["BTC-USDT"]
	assert.Equal(t, 2.0, pos.Size)
	assert.Equal(t, 55000.0, pos.EntryPrice)
}

func TestRecordTrade(t *testing.T) {
	engine := NewEngine(testRiskConfig())
	initialTrades := engine.GetDailyTrades()

	engine.IncrementTrade()

	assert.Equal(t, initialTrades+1, engine.GetDailyTrades())
}

func TestRecordTradeUpdatesDailyLoss(t *testing.T) {
	cfg := testRiskConfig()
	cfg.MaxDailyLoss = 1000
	engine := NewEngine(cfg)

	engine.UpdatePnL("BTC-USDT", -500)

	assert.Equal(t, 500.0, engine.GetDailyLoss())
}

func TestRecordTradeUpdatesDailyProfit(t *testing.T) {
	engine := NewEngine(testRiskConfig())

	engine.UpdatePnL("BTC-USDT", 500)

	assert.Equal(t, 0.0, engine.GetDailyLoss())
}

func TestGetAvailableRiskBudget(t *testing.T) {
	cfg := testRiskConfig()
	cfg.MaxDailyLoss = 100
	engine := NewEngine(cfg)

	budget := engine.GetAvailableRiskBudget(1000)
	assert.Equal(t, 100.0, budget)
}

func TestGetAvailableRiskBudgetWithExistingLoss(t *testing.T) {
	cfg := testRiskConfig()
	cfg.MaxDailyLoss = 100
	engine := NewEngine(cfg)
	engine.UpdatePnL("BTC-USDT", -30)

	budget := engine.GetAvailableRiskBudget(1000)
	assert.Equal(t, 70.0, budget)
}

func TestGetAvailableRiskBudgetZeroWhenExceeded(t *testing.T) {
	cfg := testRiskConfig()
	cfg.MaxDailyLoss = 100
	engine := NewEngine(cfg)
	engine.UpdatePnL("BTC-USDT", -100)

	budget := engine.GetAvailableRiskBudget(1000)
	assert.Equal(t, 0.0, budget)
}

func TestSetStrategyWeights(t *testing.T) {
	engine := NewEngine(testRiskConfig())

	weights := map[string]float64{
		"Strategy1": 0.5,
		"Strategy2": 0.3,
		"Strategy3": 0.2,
	}

	engine.SetStrategyWeights(weights)
	result := engine.GetStrategyWeights()

	assert.Equal(t, 0.5, result["Strategy1"])
	assert.Equal(t, 0.3, result["Strategy2"])
	assert.Equal(t, 0.2, result["Strategy3"])
}

func TestGetStrategyWeightsReturnsCopy(t *testing.T) {
	engine := NewEngine(testRiskConfig())

	engine.SetStrategyWeights(map[string]float64{"Strategy1": 0.5})
	result := engine.GetStrategyWeights()
	result["Strategy1"] = 0.8

	assert.Equal(t, 0.5, engine.GetStrategyWeights()["Strategy1"])
}

func TestGetMetrics(t *testing.T) {
	engine := NewEngine(testRiskConfig())
	engine.UpdatePnL("BTC-USDT", -50)
	engine.IncrementTrade()

	metrics := engine.GetRiskMetrics()

	assert.Equal(t, 50.0, metrics["daily_loss"])
}

func TestResetDailyStats(t *testing.T) {
	engine := NewEngine(testRiskConfig())
	engine.UpdatePnL("BTC-USDT", -100)
	engine.IncrementTrade()

	engine.ResetDailyMetrics()

	assert.Equal(t, 0.0, engine.GetDailyLoss())
	assert.Equal(t, 0, engine.GetDailyTrades())
}

func TestCheckTimeFuseAllowsNormalTime(t *testing.T) {
	engine := NewEngine(testRiskConfig())
	engine.nowFunc = func() time.Time {
		return time.Date(2026, 3, 26, 10, 30, 0, 0, time.Local)
	}

	err := engine.checkTimeFuseLocked()

	assert.NoError(t, err)
}

func TestCheckTimeFuseBlocksSettlementWindow(t *testing.T) {
	engine := NewEngine(testRiskConfig())
	engine.nowFunc = func() time.Time {
		return time.Date(2026, 3, 26, 8, 0, 0, 0, time.Local)
	}

	err := engine.checkTimeFuseLocked()
	assert.ErrorIs(t, err, ErrMarketClosed)
}

func TestCheckRiskReturnsMarketClosedDuringFuseWindow(t *testing.T) {
	engine := NewEngine(testRiskConfig())
	engine.nowFunc = func() time.Time {
		return time.Date(2026, 3, 26, 16, 0, 0, 0, time.Local)
	}

	err := engine.CheckRisk(&types.Signal{
		Type:     types.SignalTypeBuy,
		Strategy: "NeedleStrategy",
		Symbol:   "BTC-USDT",
		Price:    100,
		Quantity: 0.01,
	})

	assert.ErrorIs(t, err, ErrMarketClosed)
}

func TestIsTimeInWindow(t *testing.T) {
	tests := []struct {
		current string
		start   string
		end     string
		expect  bool
	}{
		{"12:00", "11:00", "13:00", true},
		{"10:00", "11:00", "13:00", false},
		{"14:00", "11:00", "13:00", false},
		{"00:30", "23:55", "01:05", true},
		{"23:00", "23:55", "01:05", false},
		{"23:55", "23:55", "01:05", true},
		{"01:05", "23:55", "01:05", true},
		{"01:06", "23:55", "01:05", false},
	}

	for _, tt := range tests {
		t.Run(tt.current+"_"+tt.start+"_"+tt.end, func(t *testing.T) {
			result := isTimeInWindow(tt.current, tt.start, tt.end)
			assert.Equal(t, tt.expect, result)
		})
	}
}

func TestConcurrentAccess(t *testing.T) {
	engine := NewEngine(testRiskConfig())

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			signal := &types.Signal{
				Type:     types.SignalTypeBuy,
				Symbol:   "BTC-USDT",
				Price:    float64(id),
				Quantity: 0.001,
			}
			_ = engine.CheckRisk(signal)
			engine.IncrementTrade()
			engine.UpdatePosition(&types.Position{
				Symbol: "BTC-USDT",
				Size:   float64(id),
			})
		}(i)
	}

	wg.Wait()
}

func TestEngineWithNilConfig(t *testing.T) {
	engine := NewEngine(nil)
	assert.NotNil(t, engine)
}

func TestEngineWithDisabledRisk(t *testing.T) {
	cfg := &config.RiskConfig{
		Enable:          false,
		MaxDailyLoss:    100,
		MaxTradesPerDay: 10,
		MaxPositionSize: 100,
	}
	engine := NewEngine(cfg)

	signal := &types.Signal{
		Type:     types.SignalTypeBuy,
		Symbol:   "BTC-USDT",
		Price:    50000,
		Quantity: 0.001,
	}

	err := engine.CheckRisk(signal)
	assert.NoError(t, err)
}

func TestGetPositionCount(t *testing.T) {
	engine := NewEngine(testRiskConfig())

	assert.Equal(t, 0, len(engine.GetPositions()))

	engine.UpdatePosition(&types.Position{Symbol: "BTC-USDT", Size: 1})
	assert.Equal(t, 1, len(engine.GetPositions()))

	engine.UpdatePosition(&types.Position{Symbol: "ETH-USDT", Size: 1})
	assert.Equal(t, 2, len(engine.GetPositions()))
}

func TestGetTotalPositionValue(t *testing.T) {
	engine := NewEngine(testRiskConfig())

	engine.UpdatePosition(&types.Position{
		Symbol:    "BTC-USDT",
		Size:      1,
		MarkPrice: 50000,
	})
	engine.UpdatePosition(&types.Position{
		Symbol:    "ETH-USDT",
		Size:      10,
		MarkPrice: 3000,
	})

	positions := engine.GetPositions()
	totalValue := 0.0
	for _, pos := range positions {
		totalValue += pos.Size * pos.MarkPrice
	}
	assert.Equal(t, 80000.0, totalValue)
}

func TestCheckDrawdown(t *testing.T) {
	cfg := testRiskConfig()
	cfg.MaxDrawdown = 0.1
	engine := NewEngine(cfg)

	assert.False(t, engine.IsCircuitBreakerTriggered())
}

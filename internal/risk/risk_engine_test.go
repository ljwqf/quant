package risk

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/ljwqf/quant/internal/config"
	"github.com/ljwqf/quant/pkg/types"
)

func TestCheckRiskRejectsNilSignal(t *testing.T) {
	engine := NewEngine(testRiskConfig())

	err := engine.CheckRisk(nil)

	assert.ErrorIs(t, err, ErrInvalidSignal)
}

func TestCheckRiskAllowsExitWhenDailyLimitsExceeded(t *testing.T) {
	engine := NewEngine(testRiskConfig())
	engine.dailyLoss = 9999
	engine.dailyTrades = 9999

	err := engine.CheckRisk(&types.Signal{Type: types.SignalTypeExit, Symbol: "BTC-USDT"})

	assert.NoError(t, err)
}

func TestCheckRiskRejectsEntryAbovePositionLimit(t *testing.T) {
	engine := NewEngine(testRiskConfig())
	engine.UpdatePosition(&types.Position{Symbol: "BTC-USDT", Side: types.OrderSideBuy, Size: 8, MarkPrice: 10})

	err := engine.CheckRisk(&types.Signal{Type: types.SignalTypeBuy, Symbol: "ETH-USDT", Price: 30, Quantity: 1})

	assert.ErrorIs(t, err, ErrPositionLimitExceeded)
}

func TestGetAvailableRiskBudgetClampsToRemainingLoss(t *testing.T) {
	engine := NewEngine(testRiskConfig())
	engine.dailyLoss = 40

	budget := engine.GetAvailableRiskBudget(1000)

	assert.Equal(t, 60.0, budget)
}

func TestEngineStopIsIdempotent(t *testing.T) {
	engine := NewEngine(testRiskConfig())

	assert.NotPanics(t, func() {
		engine.Stop()
		engine.Stop()
	})
}

func testRiskConfig() *config.RiskConfig {
	return &config.RiskConfig{
		Enable:            true,
		MaxPositionSize:   100,
		MaxDailyLoss:      100,
		MaxDrawdown:       0.2,
		StopLossPercent:   0.05,
		TakeProfitPercent: 0.1,
		MaxTradesPerDay:   10,
	}
}

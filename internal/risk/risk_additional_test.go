package risk

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ljwqf/quant/internal/config"
	"github.com/ljwqf/quant/internal/exchange"
	"github.com/ljwqf/quant/pkg/types"
)

func TestRiskManagerAdditionalPaths(t *testing.T) {
	mgr := NewManager(&config.RiskConfig{
		Enable:            true,
		MaxPositionSize:   1000,
		MaxDailyLoss:      100,
		MaxDrawdown:       0.5,
		StopLossPercent:   0.05,
		TakeProfitPercent: 0.1,
		MaxTradesPerDay:   100,
	}, exchange.Exchange(nil))

	mgr.UpdateDailyLoss(-10)
	assert.Equal(t, 10.0, mgr.GetDailyLoss())

	posBuy := &types.Position{Symbol: "BTC-USDT", Side: types.OrderSideBuy, EntryPrice: 100, MarkPrice: 94, Size: 1}
	stop, err := mgr.CheckStopLoss(posBuy)
	require.NoError(t, err)
	assert.True(t, stop)

	posTake := &types.Position{Symbol: "BTC-USDT", Side: types.OrderSideBuy, EntryPrice: 100, MarkPrice: 112, Size: 1}
	take, err := mgr.CheckTakeProfit(posTake)
	require.NoError(t, err)
	assert.True(t, take)

	mgr.UpdatePosition(posTake)
	metrics := mgr.GetRiskMetrics()
	assert.Contains(t, metrics, "total_exposure")

	exposure, err := mgr.CalculateRiskExposure()
	require.NoError(t, err)
	assert.Greater(t, exposure, 0.0)
}

func TestRiskEngineAdditionalPaths(t *testing.T) {
	engine := NewEngine(&config.RiskConfig{
		Enable:            true,
		MaxPositionSize:   1000,
		MaxDailyLoss:      100,
		MaxDrawdown:       0.5,
		StopLossPercent:   0.05,
		TakeProfitPercent: 0.1,
		MaxTradesPerDay:   100,
	})

	engine.UpdatePosition(&types.Position{Symbol: "ETH-USDT", Side: types.OrderSideBuy, EntryPrice: 100, MarkPrice: 101, Size: 2})
	assert.NotNil(t, engine.GetPosition("ETH-USDT"))
	metrics := engine.GetRiskMetrics()
	assert.Contains(t, metrics, "position_count")

	engine.RemovePosition("ETH-USDT")
	assert.Nil(t, engine.GetPosition("ETH-USDT"))
}

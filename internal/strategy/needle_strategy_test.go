package strategy

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ljwqf/quant/pkg/types"
)

func TestNeedleStrategyTakeProfitProducesExitSignal(t *testing.T) {
	strategy := NewNeedleStrategy()
	defer strategy.Stop()

	strategy.SetPosition("BTC-USDT", types.OrderSideBuy, 100, 1)

	signal, err := strategy.OnTick(&types.Tick{
		Symbol:    "BTC-USDT",
		Price:     101,
		Timestamp: time.Now(),
	})

	assert.NoError(t, err)
	if assert.NotNil(t, signal) {
		assert.Equal(t, types.SignalTypeExit, signal.Type)
		assert.Equal(t, "BTC-USDT", signal.Symbol)
	}
}

func TestNeedleStrategyOnPositionReducedKeepsInternalStateConsistent(t *testing.T) {
	strategy := NewNeedleStrategy()
	defer strategy.Stop()

	strategy.SetPosition("BTC-USDT", types.OrderSideBuy, 100, 1)
	strategy.OnPositionReduced("BTC-USDT", 105, 3.5, 0.4)

	position := strategy.GetPosition()
	require.NotNil(t, position)
	assert.Equal(t, 0.4, position.Size)
	assert.Equal(t, 100.0, position.EntryPrice)
	assert.Equal(t, NeedleStateInPosition, strategy.state)
	assert.Equal(t, 1, strategy.tradeCount)
	assert.Equal(t, 1, strategy.winCount)
	assert.Equal(t, 3.5, strategy.totalPnL)
}

func TestNeedleStrategyOnPositionReducedClearsPositionWhenFlat(t *testing.T) {
	strategy := NewNeedleStrategy()
	defer strategy.Stop()

	strategy.SetPosition("BTC-USDT", types.OrderSideSell, 100, 1)
	strategy.OnPositionReduced("BTC-USDT", 98, -1.25, 0)

	assert.Nil(t, strategy.GetPosition())
	assert.Equal(t, NeedleStateIdle, strategy.state)
	assert.Equal(t, 1, strategy.tradeCount)
	assert.Equal(t, 0, strategy.winCount)
	assert.Equal(t, -1.25, strategy.totalPnL)
}

func TestNeedleStrategyConfirmRebalanceEntryApprovesWhenInternalSignalAligns(t *testing.T) {
	strategy := NewNeedleStrategy()
	defer strategy.Stop()

	strategy.smartFilter.UpdateOnChainData(-6000, 0.9, 0.95)
	strategy.supertrend = &Supertrend{Value: 100, Direction: 1}
	strategy.macd = &MACD{DIF: 1.2, DEA: 0.8, Histogram: 0.4}
	strategy.barHistory = []*types.Bar{{Symbol: "BTC-USDT", High: 100, Low: 95, Close: 98, Timestamp: time.Now()}}
	strategy.priceLows = []float64{105, 103, 101, 99, 97}
	strategy.macdLows = []float64{-2.5, -2.2, -1.8, -1.4, -1.0}
	strategy.priceHighs = []float64{110, 111, 109, 108, 107}
	strategy.macdHighs = []float64{0.6, 0.7, 0.5, 0.4, 0.3}

	decision, err := strategy.ConfirmRebalanceEntry(&RebalanceRequest{ShortfallAmount: 196})
	require.NoError(t, err)
	require.NotNil(t, decision)
	assert.True(t, decision.Approved)
	require.NotNil(t, decision.Signal)
	assert.Equal(t, types.SignalTypeBuy, decision.Signal.Type)
	assert.Equal(t, "BTC-USDT", decision.Signal.Symbol)
	assert.Equal(t, decision.Signal.Price, decision.RecommendedPrice)
	assert.InDelta(t, 196/decision.Signal.Price, decision.RecommendedQuantity, 1e-9)
}

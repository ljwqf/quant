package strategy

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ljwqf/quant/pkg/types"
)

func TestVolatilityBreakoutLifecycleAndSignals(t *testing.T) {
	s := NewVolatilityBreakoutStrategy()
	require.NotNil(t, s)
	assert.Equal(t, "VolatilityBreakoutStrategy", s.Name())

	err := s.Init(map[string]interface{}{"atr_period": 3, "volume_ma_period": 3, "signal_cooldown": int64(0), "breakout_multiplier": 0.5, "min_volume_ratio": 1.0, "max_holding_bars": 2})
	require.NoError(t, err)

	bars := []*types.Bar{
		{Symbol: "BTC-USDT", High: 101, Low: 99, Close: 100, Volume: 10, Timestamp: time.Now()},
		{Symbol: "BTC-USDT", High: 102, Low: 100, Close: 101, Volume: 12, Timestamp: time.Now()},
		{Symbol: "BTC-USDT", High: 103, Low: 101, Close: 102, Volume: 15, Timestamp: time.Now()},
		{Symbol: "BTC-USDT", High: 108, Low: 102, Close: 107, Volume: 30, Timestamp: time.Now()},
	}
	for _, b := range bars {
		_, err = s.OnBar(b)
		require.NoError(t, err)
	}

	s.atrMutex.Lock()
	s.atr = 2
	s.atrMutex.Unlock()
	s.volumeMutex.Lock()
	s.volumeMA = 10
	s.volumeMutex.Unlock()
	s.signalCooldown = 0
	s.lastSignalTime = time.Time{}
	s.pricesMutex.Lock()
	s.prices = append(s.prices, 100, 101)
	s.pricesMutex.Unlock()

	sig, err := s.checkEntrySignal("BTC-USDT", 104, 105, 100, 20)
	require.NoError(t, err)
	require.NotNil(t, sig)
	assert.Equal(t, types.SignalTypeBuy, sig.Type)

	s.OnPositionFilled("BTC-USDT", types.OrderSideBuy, 104, 1)
	s.IncrementPositionBars()
	s.IncrementPositionBars()

	sig, err = s.checkExitSignal("BTC-USDT", 100, s.positionBars)
	require.NoError(t, err)
	require.NotNil(t, sig)
	assert.Equal(t, types.SignalTypeExit, sig.Type)

	s.OnPositionReduced("BTC-USDT", 100, -4, 0.5)
	s.OnPositionClosed("BTC-USDT", 100, -4)
	assert.Nil(t, s.position)

	metrics := s.GetMetrics()
	assert.Contains(t, metrics, "atr")
	params := s.GetParams()
	assert.NotNil(t, params)
	s.SetParams(map[string]interface{}{"foo": "bar"})
	assert.Equal(t, "bar", s.params["foo"])

	decision, err := s.ConfirmRebalanceEntry(&RebalanceRequest{})
	require.NoError(t, err)
	require.NotNil(t, decision)
	assert.False(t, decision.Approved)

	s.SetSmartFilter(&SmartFilter{})
	s.UpdateOnChainData(1, 2, 3)

	_, err = s.OnTick(&types.Tick{Symbol: "BTC-USDT", Price: 100, Timestamp: time.Now()})
	require.NoError(t, err)
	_, err = s.OnOrderBook(&types.OrderBook{Symbol: "BTC-USDT"})
	require.NoError(t, err)
}

func TestVolatilityBreakoutHelpers(t *testing.T) {
	s := NewVolatilityBreakoutStrategy()
	assert.Equal(t, 0.0, s.calculateATR([]float64{1}, []float64{1}, []float64{1}, 2))
	assert.Equal(t, 0.0, s.calculateSMA([]float64{1}, 2))

	atr := s.calculateATR([]float64{101, 102, 103, 104}, []float64{99, 100, 101, 102}, []float64{100, 101, 102, 103}, 3)
	assert.Greater(t, atr, 0.0)
	sma := s.calculateSMA([]float64{10, 12, 14, 16}, 3)
	assert.Greater(t, sma, 0.0)

	s.state = VolStateNeutral
	assert.Equal(t, "neutral", s.stateToString())
	s.state = VolStateBreakoutUp
	assert.Equal(t, "breakout_up", s.stateToString())
	s.state = VolStateBreakoutDown
	assert.Equal(t, "breakout_down", s.stateToString())
}

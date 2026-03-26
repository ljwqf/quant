package strategy

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ljwqf/quant/pkg/types"
)

func TestTrendFollowingLifecycleAndSignals(t *testing.T) {
	s := NewTrendFollowingStrategy()
	require.NotNil(t, s)
	assert.Equal(t, "TrendFollowingStrategy", s.Name())

	err := s.Init(map[string]interface{}{"signal_cooldown": int64(0), "adx_threshold": 10.0, "trend_strength": 0.001})
	require.NoError(t, err)

	for i := 0; i < 40; i++ {
		price := float64(100 + i)
		_, err = s.OnTick(&types.Tick{Symbol: "BTC-USDT", Price: price, Timestamp: time.Now()})
		require.NoError(t, err)
	}

	s.emaMutex.Lock()
	s.emaShort = 120
	s.emaLong = 100
	s.emaMutex.Unlock()
	s.adxMutex.Lock()
	s.adx = 40
	s.plusDI = 30
	s.minusDI = 10
	s.adxMutex.Unlock()
	s.signalCooldown = 0
	s.lastSignalTime = time.Time{}
	s.updateTrendState()

	sig, err := s.checkEntrySignal("BTC-USDT", 120)
	require.NoError(t, err)
	require.NotNil(t, sig)
	assert.Equal(t, types.SignalTypeBuy, sig.Type)

	s.OnPositionFilled("BTC-USDT", types.OrderSideBuy, 120, 1)
	require.NotNil(t, s.position)

	s.state = TrendStateDowntrend
	sig, err = s.checkExitSignal("BTC-USDT", 118)
	require.NoError(t, err)
	require.NotNil(t, sig)
	assert.Equal(t, types.SignalTypeExit, sig.Type)

	s.OnPositionReduced("BTC-USDT", 118, -2, 0.5)
	s.OnPositionClosed("BTC-USDT", 118, -2)
	assert.Nil(t, s.position)

	metrics := s.GetMetrics()
	assert.Contains(t, metrics, "adx")
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

	_, err = s.OnOrderBook(&types.OrderBook{Symbol: "BTC-USDT"})
	require.NoError(t, err)
}

func TestTrendFollowingHelpers(t *testing.T) {
	s := NewTrendFollowingStrategy()
	assert.Equal(t, 0.0, s.calculateEMA([]float64{1, 2}, 5))
	assert.Greater(t, s.calculateEMA([]float64{1, 2, 3, 4, 5}, 3), 0.0)

	adx, plus, minus := s.calculateADX([]float64{1, 2, 3, 4, 5, 6}, 3)
	assert.GreaterOrEqual(t, adx, 0.0)
	assert.GreaterOrEqual(t, plus, 0.0)
	assert.GreaterOrEqual(t, minus, 0.0)

	s.state = TrendStateNeutral
	assert.Equal(t, "neutral", s.stateToString())
	s.state = TrendStateUptrend
	assert.Equal(t, "uptrend", s.stateToString())
	s.state = TrendStateDowntrend
	assert.Equal(t, "downtrend", s.stateToString())

	_, err := s.OnBar(&types.Bar{Symbol: "BTC-USDT", Close: 100, Timestamp: time.Now()})
	require.NoError(t, err)
}

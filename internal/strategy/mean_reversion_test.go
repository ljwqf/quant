package strategy

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ljwqf/quant/pkg/types"
)

func TestMeanReversionLifecycleAndSignals(t *testing.T) {
	s := NewMeanReversionStrategy()
	require.NotNil(t, s)
	assert.Equal(t, "MeanReversionStrategy", s.Name())

	err := s.Init(map[string]interface{}{
		"rsi_period":            5,
		"bb_period":             5,
		"bb_std_dev":            2.0,
		"signal_cooldown":       int64(0),
		"stop_loss_percent":     0.02,
		"trailing_stop_percent": 0.01,
	})
	require.NoError(t, err)

	base := []float64{100, 98, 96, 94, 92, 90, 89, 88, 87, 86}
	for _, p := range base {
		_, err = s.OnTick(&types.Tick{Symbol: "BTC-USDT", Price: p, Timestamp: time.Now()})
		require.NoError(t, err)
	}

	s.rsiMutex.Lock()
	s.rsi = 20
	s.rsiMutex.Unlock()
	s.bbMutex.Lock()
	s.bbLower = 90
	s.bbUpper = 110
	s.bbMiddle = 100
	s.bbMutex.Unlock()

	sig, err := s.checkEntrySignal("BTC-USDT", 85)
	require.NoError(t, err)
	require.NotNil(t, sig)
	assert.Equal(t, types.SignalTypeBuy, sig.Type)

	s.OnPositionFilled("BTC-USDT", types.OrderSideBuy, 85, 1)
	require.NotNil(t, s.position)

	sig, err = s.checkExitSignal("BTC-USDT", 82)
	require.NoError(t, err)
	require.NotNil(t, sig)
	assert.Equal(t, types.SignalTypeExit, sig.Type)

	s.OnPositionReduced("BTC-USDT", 84, -1, 0.5)
	s.OnPositionClosed("BTC-USDT", 84, -1)
	assert.Nil(t, s.position)

	metrics := s.GetMetrics()
	assert.Contains(t, metrics, "state")
	assert.Contains(t, metrics, "rsi")

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
}

func TestMeanReversionHelpers(t *testing.T) {
	s := NewMeanReversionStrategy()

	assert.Equal(t, 50.0, s.calculateRSI([]float64{1, 2}, 14))

	upper, middle, lower := s.calculateBollingerBands([]float64{1, 2, 3, 4, 5}, 5, 2)
	assert.Greater(t, upper, middle)
	assert.Less(t, lower, middle)

	s.state = MeanRevStateNeutral
	assert.Equal(t, "neutral", s.stateToString())
	s.state = MeanRevStateOverbought
	assert.Equal(t, "overbought", s.stateToString())
	s.state = MeanRevStateOversold
	assert.Equal(t, "oversold", s.stateToString())

	_, err := s.OnOrderBook(&types.OrderBook{Symbol: "BTC-USDT"})
	require.NoError(t, err)
}

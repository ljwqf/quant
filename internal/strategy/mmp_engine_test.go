package strategy

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/ljwqf/quant/pkg/types"
)

func TestMMPEngineUsesOrderBookSpread(t *testing.T) {
	engine := NewMMPEnginePro()
	defer engine.Stop()
	engine.state = MMPStateActive

	_, err := engine.OnOrderBook(&types.OrderBook{
		Symbol: "BTC-USDT",
		Bids:   []types.OrderBookLevel{{Price: 99.99, Size: 1}},
		Asks:   []types.OrderBookLevel{{Price: 100.01, Size: 1}},
	})
	assert.NoError(t, err)

	spread := engine.calculateSpread(&types.Tick{Symbol: "BTC-USDT", Price: 100, Timestamp: time.Now()})

	assert.InDelta(t, 0.0002, spread, 1e-6)
}

func TestMMPEngineGeneratesShortSignalOnNegativeDelta(t *testing.T) {
	engine := NewMMPEnginePro()
	defer engine.Stop()
	engine.state = MMPStateActive
	engine.atr = 1
	engine.volMean = 100

	_, err := engine.OnOrderBook(&types.OrderBook{
		Symbol: "BTC-USDT",
		Bids:   []types.OrderBookLevel{{Price: 99.99, Size: 1}},
		Asks:   []types.OrderBookLevel{{Price: 100.01, Size: 1}},
	})
	assert.NoError(t, err)

	for i := 0; i < 19; i++ {
		engine.addToRingBuffer(TickData{Price: 101, Volume: 50, Timestamp: time.Now()})
	}
	engine.addToRingBuffer(TickData{Price: 100, Volume: 50, Timestamp: time.Now()})

	signal := engine.generateSignal(&types.Tick{Symbol: "BTC-USDT", Price: 100, Size: 50, Timestamp: time.Now()})

	if assert.NotNil(t, signal) {
		assert.Equal(t, types.SignalTypeSell, signal.Type)
	}
}

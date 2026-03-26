package strategy

import (
	"sync"
	"testing"
	"time"

	"github.com/ljwqf/quant/pkg/types"
	"github.com/stretchr/testify/assert"
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

func TestMMPEngineRecordTradeResetsDailyLossOnNewDay(t *testing.T) {
	engine := NewMMPEnginePro()
	defer engine.Stop()

	oldDay := time.Date(2025, 12, 31, 23, 59, 0, 0, time.UTC)
	newDay := oldDay.Add(2 * time.Minute)

	engine.dailyLoss = 0.04
	engine.dailyLossReset = oldDay
	engine.nowFunc = func() time.Time { return newDay }

	engine.RecordTrade(-0.01)

	assert.InDelta(t, 0.01, engine.dailyLoss, 1e-9)
	assert.Equal(t, newDay, engine.dailyLossReset)
}

func TestMMPEngineCircuitBreakerResetsStaleDailyLoss(t *testing.T) {
	engine := NewMMPEnginePro()
	defer engine.Stop()

	oldDay := time.Date(2026, 1, 31, 23, 59, 0, 0, time.UTC)
	newDay := time.Date(2026, 2, 1, 0, 1, 0, 0, time.UTC)

	engine.dailyLoss = DailyLossLimit
	engine.dailyLossReset = oldDay
	engine.nowFunc = func() time.Time { return newDay }

	assert.False(t, engine.checkCircuitBreaker())
	assert.Equal(t, 0.0, engine.dailyLoss)
	assert.Equal(t, newDay, engine.dailyLossReset)
}

func TestMMPEngineOnTickHandlesUnexpectedPoolValue(t *testing.T) {
	engine := NewMMPEnginePro()
	defer engine.Stop()
	engine.state = MMPStateActive
	engine.nowFunc = func() time.Time { return time.Date(2026, 3, 26, 1, 2, 0, 0, time.UTC) }
	engine.tickPool = &sync.Pool{New: func() interface{} { return "bad" }}

	assert.NotPanics(t, func() {
		_, err := engine.OnTick(&types.Tick{Symbol: "BTC-USDT", Price: 100, Size: 1, Timestamp: engine.nowFunc()})
		assert.NoError(t, err)
	})
}

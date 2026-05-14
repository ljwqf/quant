package decision

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ljwqf/quant/internal/v2/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShadowPipelineProcessesOrderBookAndLogsSignal(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "shadow")
	pipeline, err := NewShadowPipeline(ShadowModeConfig{
		LogDir:           dir,
		MissedSignalBps:  200,
		OpenThreshold:    0.6,
		CooldownDuration: time.Second,
	})
	require.NoError(t, err)

	pipeline.ProcessTick(events.TickEvent{Symbol: "BTC-USDT", Price: 100, Quantity: 90, Direction: events.TradeDirectionBuy, Timestamp: time.Now()})
	pipeline.ProcessTick(events.TickEvent{Symbol: "BTC-USDT", Price: 100, Quantity: 10, Direction: events.TradeDirectionSell, Timestamp: time.Now()})

	snapshot := pipeline.ProcessOrderBook(events.OrderBookEvent{
		Symbol:    "BTC-USDT",
		Timestamp: time.Now(),
		Bids:      []events.OrderBookLevel{{Price: 99, Quantity: 4}, {Price: 98, Quantity: 18}, {Price: 97, Quantity: 4}},
		Asks:      []events.OrderBookLevel{{Price: 101, Quantity: 4}, {Price: 102, Quantity: 18}, {Price: 103, Quantity: 4}},
	})
	assert.Equal(t, "BTC-USDT", snapshot.Symbol)

	require.NoError(t, pipeline.Close())

	shadowLogPath := filepath.Join(dir, "shadow_signals.json")
	data, err := os.ReadFile(shadowLogPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "BTC-USDT")
}

func TestStatePersistenceSaveAndLoad(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "state")
	persist, err := NewStatePersistence(StatePersistenceConfig{StateDir: dir})
	require.NoError(t, err)

	now := time.Now()
	require.NoError(t, persist.Save("BTC-USDT", events.StrategyStateSweepingHunting, now, events.FactorSnapshot{Symbol: "BTC-USDT", MidPrice: 100}))

	record, err := persist.Load("BTC-USDT")
	require.NoError(t, err)
	require.NotNil(t, record)
	assert.Equal(t, "BTC-USDT", record.Symbol)
	assert.Equal(t, events.StrategyStateSweepingHunting, record.State)
}

func TestStatePersistenceLoadAll(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "state")
	persist, err := NewStatePersistence(StatePersistenceConfig{StateDir: dir})
	require.NoError(t, err)

	now := time.Now()
	require.NoError(t, persist.Save("BTC-USDT", events.StrategyStateIdle, now, events.FactorSnapshot{Symbol: "BTC-USDT"}))
	require.NoError(t, persist.Save("ETH-USDT", events.StrategyStateCooldown, now, events.FactorSnapshot{Symbol: "ETH-USDT"}))

	all, err := persist.LoadAll()
	require.NoError(t, err)
	assert.Len(t, all, 2)
	assert.Contains(t, all, "BTC-USDT")
	assert.Contains(t, all, "ETH-USDT")
}

func TestStatePersistenceRestore(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "state")
	persist, err := NewStatePersistence(StatePersistenceConfig{StateDir: dir})
	require.NoError(t, err)

	now := time.Now()
	require.NoError(t, persist.Save("BTC-USDT", events.StrategyStateConsolidationSetting, now, events.FactorSnapshot{Symbol: "BTC-USDT"}))

	machine := NewLiquidityStateMachine(DefaultLiquidityStrategyConfig())
	require.NoError(t, persist.RestoreState("BTC-USDT", machine))
	assert.Equal(t, events.StrategyStateConsolidationSetting, machine.State("BTC-USDT"))
}

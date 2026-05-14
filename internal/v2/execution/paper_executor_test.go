package execution

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/ljwqf/quant/internal/v2/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPaperExecutorOpensAndClosesPosition(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "paper")
	pool := NewProfitPool(ProfitPoolConfig{BaseRiskPercent: 0.01, ProfitRiskPercent: 0.02, BaseCapital: 10000})
	guard := NewRiskGuard(HardRiskConfig{MaxLossPerDayBps: 5000, MaxLossPerWeekBps: 5000, MaxLossPerStrategyBps: 5000, MaxConsecutiveLosses: 5, MaxSlippageBps: 50, MaxLossPerTradeBps: 100})
	executor, err := NewPaperExecutor(PaperExecutorConfig{LogDir: dir}, pool, guard)
	require.NoError(t, err)

	intent := events.SignalIntent{
		Symbol:         "BTC-USDT",
		StrategyID:     "LQT_Down01",
		Side:           events.SideSell,
		Score:          0.85,
		ExpectedEntry:  100,
		StructuralStop: 105,
		Snapshot:       events.FactorSnapshot{SpreadStable: true},
		Timestamp:      time.Now(),
	}

	pos, result := executor.OpenPosition(intent)
	require.True(t, result.Allowed)
	require.NotNil(t, pos)
	assert.Equal(t, events.SideSell, pos.Side)
	assert.Greater(t, pos.Size, 0.0)

	executor.UpdatePrice("BTC-USDT", 99, time.Now())

	executor.ClosePosition("BTC-USDT", "LQT_Down01", 98, time.Now(), "target_reached")

	stats := executor.Stats()
	assert.Equal(t, 1, stats["trade_count"])
	assert.Greater(t, stats["total_pnl"], 0.0)

	require.NoError(t, executor.Close())
}

func TestPaperExecutorRejectsEntryWhenSpreadUnstable(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "paper")
	pool := NewProfitPool(ProfitPoolConfig{BaseRiskPercent: 0.01, BaseCapital: 10000})
	guard := NewRiskGuard(HardRiskConfig{MaxLossPerDayBps: 5000, MaxLossPerWeekBps: 5000, MaxLossPerStrategyBps: 5000, MaxConsecutiveLosses: 5, MaxSlippageBps: 50, MaxLossPerTradeBps: 100})
	executor, err := NewPaperExecutor(PaperExecutorConfig{LogDir: dir}, pool, guard)
	require.NoError(t, err)

	intent := events.SignalIntent{
		Symbol:        "BTC-USDT",
		StrategyID:    "LQT_Down01",
		Side:          events.SideSell,
		Score:         0.8,
		ExpectedEntry: 100,
		Snapshot:      events.FactorSnapshot{SpreadStable: false},
		Timestamp:     time.Now(),
	}

	pos, result := executor.OpenPosition(intent)
	assert.False(t, result.Allowed)
	assert.Nil(t, pos)

	require.NoError(t, executor.Close())
}

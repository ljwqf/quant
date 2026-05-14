package execution

import (
	"testing"
	"time"

	"github.com/ljwqf/quant/internal/v2/events"
	"github.com/stretchr/testify/assert"
)

func TestProfitPoolAllocatesBaseRiskOnlyWithoutProfit(t *testing.T) {
	pool := NewProfitPool(ProfitPoolConfig{BaseRiskPercent: 0.01, ProfitRiskPercent: 0.02, BaseCapital: 10000})
	allocation := pool.AllocateRisk(0.6)
	assert.Equal(t, 100.0, allocation.BaseRisk)
	assert.Equal(t, 0.0, allocation.ExtraRisk)
	assert.Equal(t, 100.0, allocation.TotalRisk)
}

func TestProfitPoolAllocatesExtraRiskForHighScoreWithProfit(t *testing.T) {
	pool := NewProfitPool(ProfitPoolConfig{BaseRiskPercent: 0.01, ProfitRiskPercent: 0.02, BaseCapital: 10000})
	pool.RecordRealizedPnL(500)
	allocation := pool.AllocateRisk(0.85)
	assert.Equal(t, 100.0, allocation.BaseRisk)
	assert.Greater(t, allocation.ExtraRisk, 0.0)
	assert.Greater(t, allocation.TotalRisk, 100.0)
}

func TestProfitPoolNeverDecreasesBaseCapital(t *testing.T) {
	pool := NewProfitPool(ProfitPoolConfig{BaseRiskPercent: 0.01, ProfitRiskPercent: 0.02, BaseCapital: 10000})
	pool.RecordRealizedPnL(500)
	pool.RecordRealizedPnL(-300)
	assert.Equal(t, 10000.0, pool.BaseCapital())
	assert.Equal(t, 200.0, pool.ProfitPool())
}

func TestProfitPoolLossesOnlyDeductFromProfit(t *testing.T) {
	pool := NewProfitPool(ProfitPoolConfig{BaseRiskPercent: 0.01, ProfitRiskPercent: 0.02, BaseCapital: 10000})
	pool.RecordRealizedPnL(100)
	pool.RecordRealizedPnL(-50)
	assert.Equal(t, 10000.0, pool.BaseCapital())
	assert.Equal(t, 50.0, pool.ProfitPool())
}

func TestRiskGuardBlocksEntryWhenSpreadUnstable(t *testing.T) {
	guard := NewRiskGuard(DefaultHardRiskConfig())
	intent := events.SignalIntent{
		Symbol:     "BTC-USDT",
		StrategyID: "LQT_Down01",
		Side:       events.SideSell,
		Score:      0.8,
		Snapshot:   events.FactorSnapshot{SpreadStable: false},
		Timestamp:  time.Now(),
	}
	result := guard.CheckEntry(intent, 10000)
	assert.False(t, result.Allowed)
	assert.Equal(t, "spread_unstable", result.Reason)
}

func TestRiskGuardBlocksEntryWhenDailyLimitReached(t *testing.T) {
	guard := NewRiskGuard(HardRiskConfig{MaxLossPerDayBps: 500, MaxConsecutiveLosses: 5})
	guard.RecordLoss("LQT_Down01", 600)
	intent := events.SignalIntent{
		Symbol:     "BTC-USDT",
		StrategyID: "LQT_Down01",
		Side:       events.SideSell,
		Score:      0.8,
		Snapshot:   events.FactorSnapshot{SpreadStable: true},
		Timestamp:  time.Now(),
	}
	result := guard.CheckEntry(intent, 10000)
	assert.False(t, result.Allowed)
	assert.True(t, result.ReadOnlyMode)
}

func TestRiskGuardStructuralStopDetectsWallBreak(t *testing.T) {
	guard := NewRiskGuard(DefaultHardRiskConfig())
	rule := StructuralStopRule{
		StrategyID: "LQT_Down01",
		Side:       events.SideSell,
		WallPrice:  105,
		BufferBps:  20,
	}
	shouldStop, reason := guard.CheckStructuralStop(events.SideSell, 106, rule)
	assert.True(t, shouldStop)
	assert.Equal(t, "ask_wall_effectively_broken", reason)

	shouldStop, reason = guard.CheckStructuralStop(events.SideSell, 104, rule)
	assert.False(t, shouldStop)
}

func TestRiskGuardDisablesStrategyOnConsecutiveLosses(t *testing.T) {
	guard := NewRiskGuard(HardRiskConfig{MaxConsecutiveLosses: 3, MaxLossPerDayBps: 5000, MaxLossPerWeekBps: 5000, MaxLossPerStrategyBps: 5000, MaxSlippageBps: 50})
	for i := 0; i < 3; i++ {
		guard.RecordLoss("LQT_REV01", 10)
	}
	assert.True(t, guard.IsStrategyDisabled("LQT_REV01"))
}

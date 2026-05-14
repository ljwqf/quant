package decision

import (
	"testing"
	"time"

	"github.com/ljwqf/quant/internal/v2/events"
	"github.com/stretchr/testify/assert"
)

func TestLiquidityStateMachineTransitionsToTradeIntent(t *testing.T) {
	machine := NewLiquidityStateMachine(LiquidityStrategyConfig{OpenThreshold: 0.6, CooldownDuration: time.Second})
	now := time.Now()

	sweep := baseSnapshot(now)
	sweep.SpreadStable = false
	sweep.AggressiveBuyVolume = 90
	sweep.AggressiveSellVolume = 10
	sweep.Delta = 80
	intent := machine.Evaluate(sweep, events.MacroStateTrendDown)
	assert.Equal(t, events.StrategyStateSweepingHunting, intent.State)
	assert.Equal(t, "spread_unstable", intent.RejectReason)

	setting := baseSnapshot(now.Add(time.Second))
	setting.MomentumContinuation = 0.2
	intent = machine.Evaluate(setting, events.MacroStateTrendDown)
	assert.Equal(t, events.StrategyStateConsolidationSetting, intent.State)

	confirmed := baseSnapshot(now.Add(2 * time.Second))
	confirmed.AggressiveBuyVolume = 85
	confirmed.AggressiveSellVolume = 15
	confirmed.Delta = 70
	confirmed.WallAbsorptionScore = 0.95
	confirmed.BuyExhaustionScore = 0.9
	confirmed.TopAskWall.AbsorbRate = 8
	intent = machine.Evaluate(confirmed, events.MacroStateTrendDown)
	assert.Equal(t, events.StrategyStatePositionOpen, intent.State)
	assert.Empty(t, intent.RejectReason)
	assert.Equal(t, events.SideSell, intent.Side)
	assert.GreaterOrEqual(t, intent.Score, intent.OpenThreshold)
	assert.Len(t, machine.Transitions(), 4)
}

func TestScoreLiquiditySetupSelectsMomentumCollapse(t *testing.T) {
	snapshot := baseSnapshot(time.Now())
	snapshot.TopAskWall.Collapse = true
	snapshot.AskVacuum.Score = 0.9
	snapshot.AggressiveBuyVolume = 95
	snapshot.AggressiveSellVolume = 5
	snapshot.Delta = 90
	score, strategyID, side := ScoreLiquiditySetup(snapshot, events.MacroStateTrendUp)

	assert.Equal(t, "LQT_MOM01", strategyID)
	assert.Equal(t, events.SideBuy, side)
	assert.Greater(t, score, 0.7)
}

func baseSnapshot(ts time.Time) events.FactorSnapshot {
	return events.FactorSnapshot{
		Symbol:               "BTC-USDT",
		Timestamp:            ts,
		MidPrice:             100,
		Spread:               0.1,
		SpreadStable:         true,
		OBI:                  0.5,
		AggressiveBuyVolume:  30,
		AggressiveSellVolume: 20,
		Delta:                10,
		WallAbsorptionScore:  0.8,
		BuyExhaustionScore:   0.8,
		MomentumContinuation: 0.3,
		TopAskWall:           events.LiquidityWall{Side: events.SideSell, Price: 101, Size: 10, AbsorbRate: 4},
		TopBidWall:           events.LiquidityWall{Side: events.SideBuy, Price: 99, Size: 10, AbsorbRate: 2},
		AskVacuum:            events.VacuumZone{Side: events.SideSell, Start: 101, End: 102, Score: 0.2},
		BidVacuum:            events.VacuumZone{Side: events.SideBuy, Start: 98, End: 99, Score: 0.2},
	}
}

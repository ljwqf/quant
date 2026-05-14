package decision

import (
	"math"
	"time"

	"github.com/ljwqf/quant/internal/v2/events"
)

type LiquidityStrategyConfig struct {
	OpenThreshold       float64
	CooldownDuration    time.Duration
	SpreadPenalty       float64
	StructuralStopBps   float64
	HuntingDeltaRatio   float64
	ImbalanceDeltaRatio float64
}

func DefaultLiquidityStrategyConfig() LiquidityStrategyConfig {
	return LiquidityStrategyConfig{
		OpenThreshold:       0.72,
		CooldownDuration:    30 * time.Second,
		SpreadPenalty:       0.25,
		StructuralStopBps:   20,
		HuntingDeltaRatio:   0.65,
		ImbalanceDeltaRatio: 0.55,
	}
}

type LiquidityStateMachine struct {
	config      LiquidityStrategyConfig
	states      map[string]events.StrategyState
	lastChange  map[string]time.Time
	transitions []events.StateTransition
}

func NewLiquidityStateMachine(config LiquidityStrategyConfig) *LiquidityStateMachine {
	if config.OpenThreshold == 0 {
		config.OpenThreshold = DefaultLiquidityStrategyConfig().OpenThreshold
	}
	if config.CooldownDuration == 0 {
		config.CooldownDuration = DefaultLiquidityStrategyConfig().CooldownDuration
	}
	if config.SpreadPenalty == 0 {
		config.SpreadPenalty = DefaultLiquidityStrategyConfig().SpreadPenalty
	}
	if config.StructuralStopBps == 0 {
		config.StructuralStopBps = DefaultLiquidityStrategyConfig().StructuralStopBps
	}
	if config.HuntingDeltaRatio == 0 {
		config.HuntingDeltaRatio = DefaultLiquidityStrategyConfig().HuntingDeltaRatio
	}
	if config.ImbalanceDeltaRatio == 0 {
		config.ImbalanceDeltaRatio = DefaultLiquidityStrategyConfig().ImbalanceDeltaRatio
	}
	return &LiquidityStateMachine{
		config:      config,
		states:      make(map[string]events.StrategyState),
		lastChange:  make(map[string]time.Time),
		transitions: make([]events.StateTransition, 0),
	}
}

func (m *LiquidityStateMachine) State(symbol string) events.StrategyState {
	state := m.states[symbol]
	if state == "" {
		return events.StrategyStateIdle
	}
	return state
}

func (m *LiquidityStateMachine) SetState(symbol string, state events.StrategyState, lastChange time.Time) {
	m.states[symbol] = state
	m.lastChange[symbol] = lastChange
}

func (m *LiquidityStateMachine) Transitions() []events.StateTransition {
	result := make([]events.StateTransition, len(m.transitions))
	copy(result, m.transitions)
	return result
}

func (m *LiquidityStateMachine) Evaluate(snapshot events.FactorSnapshot, macro events.MacroState) events.SignalIntent {
	state := m.State(snapshot.Symbol)
	score, strategyID, side := ScoreLiquiditySetup(snapshot, macro)
	intent := events.SignalIntent{
		Symbol:         snapshot.Symbol,
		StrategyID:     strategyID,
		Side:           side,
		State:          state,
		MacroState:     macro,
		Score:          score,
		OpenThreshold:  m.config.OpenThreshold,
		ExpectedEntry:  snapshot.MidPrice,
		StructuralStop: structuralStop(snapshot, side, m.config.StructuralStopBps),
		RejectReason:   "state_not_tradeable",
		Snapshot:       snapshot,
		Timestamp:      snapshot.Timestamp,
	}

	next, reason := m.nextState(state, snapshot, score)
	if next != state {
		m.transition(snapshot.Symbol, state, next, reason, snapshot)
		intent.State = next
	}

	if next == events.StrategyStateImbalanceConfirmed && score >= m.config.OpenThreshold && snapshot.SpreadStable {
		intent.RejectReason = ""
		m.transition(snapshot.Symbol, next, events.StrategyStatePositionOpen, "score_above_open_threshold", snapshot)
		intent.State = events.StrategyStatePositionOpen
		return intent
	}
	if !snapshot.SpreadStable {
		intent.RejectReason = "spread_unstable"
	}
	return intent
}

func (m *LiquidityStateMachine) nextState(state events.StrategyState, snapshot events.FactorSnapshot, score float64) (events.StrategyState, string) {
	if snapshot.Symbol == "" {
		return state, "empty_symbol"
	}
	if isInvalidated(snapshot) {
		return events.StrategyStateInvalidated, "structure_invalidated"
	}

	switch state {
	case events.StrategyStateIdle:
		if isSweeping(snapshot, m.config.HuntingDeltaRatio) {
			return events.StrategyStateSweepingHunting, "price_sweep_with_spread_or_wall_collapse"
		}
	case events.StrategyStateSweepingHunting:
		if snapshot.SpreadStable && snapshot.MomentumContinuation < 0.5 {
			return events.StrategyStateConsolidationSetting, "flow_decayed_and_spread_stable"
		}
	case events.StrategyStateConsolidationSetting:
		if score >= m.config.OpenThreshold*0.85 && math.Abs(snapshot.DeltaRatio()) >= m.config.ImbalanceDeltaRatio {
			return events.StrategyStateImbalanceConfirmed, "rolling_delta_and_wall_asymmetry_confirmed"
		}
	case events.StrategyStatePositionOpen:
		return events.StrategyStatePositionOpen, "position_open"
	case events.StrategyStateInvalidated:
		return events.StrategyStateCooldown, "archive_snapshot_before_cooldown"
	case events.StrategyStateCooldown:
		if time.Since(m.lastChange[snapshot.Symbol]) >= m.config.CooldownDuration {
			return events.StrategyStateIdle, "cooldown_expired"
		}
	}
	return state, "state_unchanged"
}

func (m *LiquidityStateMachine) transition(symbol string, from, to events.StrategyState, reason string, snapshot events.FactorSnapshot) {
	now := snapshot.Timestamp
	if now.IsZero() {
		now = time.Now()
	}
	m.states[symbol] = to
	m.lastChange[symbol] = now
	m.transitions = append(m.transitions, events.StateTransition{
		Symbol:       symbol,
		From:         from,
		To:           to,
		Reason:       reason,
		Snapshot:     snapshot,
		TransitionAt: now,
	})
}

func ScoreLiquiditySetup(snapshot events.FactorSnapshot, macro events.MacroState) (float64, string, events.Side) {
	downScore := scoreLQTDown01(snapshot, macro)
	revScore, revSide := scoreLQTREV01(snapshot)
	momScore, momSide := scoreLQTMOM01(snapshot)

	strategyID := "LQT_Down01"
	side := events.SideSell
	score := downScore
	if revScore > score {
		strategyID = "LQT_REV01"
		side = revSide
		score = revScore
	}
	if momScore > score {
		strategyID = "LQT_MOM01"
		side = momSide
		score = momScore
	}
	return clamp01(score), strategyID, side
}

func scoreLQTDown01(snapshot events.FactorSnapshot, macro events.MacroState) float64 {
	trendDown := 0.0
	if macro == events.MacroStateTrendDown || macro == events.MacroStateMomentumDecay {
		trendDown = 1
	}
	spread := boolScore(snapshot.SpreadStable)
	absorption := snapshot.WallAbsorptionScore
	exhaustion := snapshot.BuyExhaustionScore
	obiBuy := clamp01((snapshot.OBI + 1) / 2)
	breakoutPenalty := snapshot.BreakoutStrengthScore
	return 0.25*trendDown + 0.2*absorption + 0.2*exhaustion + 0.15*spread + 0.1*obiBuy - 0.1*breakoutPenalty
}

func scoreLQTREV01(snapshot events.FactorSnapshot) (float64, events.Side) {
	deltaRatio := snapshot.DeltaRatio()
	spread := boolScore(snapshot.SpreadStable)
	reversalBase := clamp01(snapshot.MomentumContinuation + snapshot.PassiveActiveDivergence)
	if deltaRatio >= 0 {
		return clamp01(0.45*reversalBase + 0.25*spread + 0.3*snapshot.TopAskWall.AbsorbRate/(snapshot.TopAskWall.AbsorbRate+1)), events.SideSell
	}
	return clamp01(0.45*reversalBase + 0.25*spread + 0.3*snapshot.TopBidWall.AbsorbRate/(snapshot.TopBidWall.AbsorbRate+1)), events.SideBuy
}

func scoreLQTMOM01(snapshot events.FactorSnapshot) (float64, events.Side) {
	askCollapse := boolScore(snapshot.TopAskWall.Collapse)
	bidCollapse := boolScore(snapshot.TopBidWall.Collapse)
	if askCollapse >= bidCollapse {
		return clamp01(0.4*askCollapse + 0.25*snapshot.AskVacuum.Score + 0.25*math.Max(0, snapshot.DeltaRatio()) + 0.1*boolScore(snapshot.SpreadStable)), events.SideBuy
	}
	return clamp01(0.4*bidCollapse + 0.25*snapshot.BidVacuum.Score + 0.25*math.Max(0, -snapshot.DeltaRatio()) + 0.1*boolScore(snapshot.SpreadStable)), events.SideSell
}

func isSweeping(snapshot events.FactorSnapshot, threshold float64) bool {
	return !snapshot.SpreadStable || snapshot.TopAskWall.Collapse || snapshot.TopBidWall.Collapse || math.Abs(snapshot.DeltaRatio()) >= threshold
}

func isInvalidated(snapshot events.FactorSnapshot) bool {
	return snapshot.Spread > 0 && snapshot.MidPrice <= 0
}

func structuralStop(snapshot events.FactorSnapshot, side events.Side, bps float64) float64 {
	buffer := bps / 10000
	if side == events.SideSell {
		if snapshot.TopAskWall.Price > 0 {
			return snapshot.TopAskWall.Price * (1 + buffer)
		}
		return snapshot.MidPrice * (1 + buffer)
	}
	if snapshot.TopBidWall.Price > 0 {
		return snapshot.TopBidWall.Price * (1 - buffer)
	}
	return snapshot.MidPrice * (1 - buffer)
}

func boolScore(value bool) float64 {
	if value {
		return 1
	}
	return 0
}

func clamp01(value float64) float64 {
	if value < 0 || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

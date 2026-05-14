package execution

import (
	"math"
	"sync"
	"time"

	"github.com/ljwqf/quant/internal/v2/events"
)

type HardRiskConfig struct {
	MaxLossPerTradeBps    float64
	MaxLossPerDayBps      float64
	MaxLossPerWeekBps     float64
	MaxLossPerStrategyBps float64
	MaxSlippageBps        float64
	MaxConsecutiveLosses  int
}

func DefaultHardRiskConfig() HardRiskConfig {
	return HardRiskConfig{
		MaxLossPerTradeBps:    100,
		MaxLossPerDayBps:      500,
		MaxLossPerWeekBps:     1500,
		MaxLossPerStrategyBps: 2000,
		MaxSlippageBps:        50,
		MaxConsecutiveLosses:  5,
	}
}

type StructuralStopRule struct {
	StrategyID string
	Side       events.Side
	WallPrice  float64
	BufferBps  float64
	TimeStop   time.Duration
}

type RiskGuard struct {
	hardRisk           HardRiskConfig
	mu                 sync.Mutex
	dailyLoss          float64
	weeklyLoss         float64
	strategyLosses     map[string]float64
	consecutiveLosses  map[string]int
	weekStart          time.Time
	dayStart           time.Time
	readOnlyMode       bool
	disabledStrategies map[string]string
}

type RiskCheckResult struct {
	Allowed        bool    `json:"allowed"`
	Reason         string  `json:"reason,omitempty"`
	ReadOnlyMode   bool    `json:"read_only_mode"`
	MaxPositionBps float64 `json:"max_position_bps,omitempty"`
}

func NewRiskGuard(config HardRiskConfig) *RiskGuard {
	if config.MaxLossPerTradeBps == 0 {
		config.MaxLossPerTradeBps = DefaultHardRiskConfig().MaxLossPerTradeBps
	}
	if config.MaxLossPerDayBps == 0 {
		config.MaxLossPerDayBps = DefaultHardRiskConfig().MaxLossPerDayBps
	}
	if config.MaxLossPerWeekBps == 0 {
		config.MaxLossPerWeekBps = DefaultHardRiskConfig().MaxLossPerWeekBps
	}
	if config.MaxLossPerStrategyBps == 0 {
		config.MaxLossPerStrategyBps = DefaultHardRiskConfig().MaxLossPerStrategyBps
	}
	now := time.Now()
	return &RiskGuard{
		hardRisk:           config,
		dayStart:           now,
		weekStart:          now,
		strategyLosses:     make(map[string]float64),
		consecutiveLosses:  make(map[string]int),
		disabledStrategies: make(map[string]string),
	}
}

func (g *RiskGuard) CheckEntry(intent events.SignalIntent, baseCapital float64) RiskCheckResult {
	g.mu.Lock()
	defer g.mu.Unlock()

	now := intent.Timestamp
	if now.IsZero() {
		now = time.Now()
	}
	g.resetTimers(now)

	if g.readOnlyMode {
		return RiskCheckResult{Allowed: false, Reason: "read_only_mode_daily_limit_reached", ReadOnlyMode: true}
	}

	if reason, ok := g.disabledStrategies[intent.StrategyID]; ok {
		return RiskCheckResult{Allowed: false, Reason: "strategy_disabled:" + reason}
	}

	if !intent.Snapshot.SpreadStable {
		return RiskCheckResult{Allowed: false, Reason: "spread_unstable"}
	}

	if math.Abs(intent.Snapshot.DeltaRatio()) >= 0.65 && intent.State == events.StrategyStateSweepingHunting {
		return RiskCheckResult{Allowed: false, Reason: "sweeping_hunting_in_progress"}
	}

	return RiskCheckResult{Allowed: true, MaxPositionBps: g.hardRisk.MaxLossPerTradeBps}
}

func (g *RiskGuard) CheckStructuralStop(positionSide events.Side, currentPrice float64, stopRule StructuralStopRule) (shouldStop bool, reason string) {
	if stopRule.WallPrice <= 0 {
		return false, ""
	}

	buffer := stopRule.BufferBps / 10000
	if stopRule.Side == "" {
		stopRule.Side = stopRule.Side
	}

	switch stopRule.StrategyID {
	case "LQT_Down01":
		if positionSide == events.SideSell && currentPrice >= stopRule.WallPrice*(1+buffer) {
			return true, "ask_wall_effectively_broken"
		}
	case "LQT_REV01":
		sweepExtreme := stopRule.WallPrice
		if positionSide == events.SideBuy && currentPrice <= sweepExtreme*(1-buffer) {
			return true, "sweep_extreme_broken"
		}
		if positionSide == events.SideSell && currentPrice >= sweepExtreme*(1+buffer) {
			return true, "sweep_extreme_broken"
		}
	case "LQT_MOM01":
		if stopRule.Side == events.SideSell && currentPrice >= stopRule.WallPrice*(1-buffer) {
			return true, "collapsed_wall_rebuilt"
		}
		if stopRule.Side == events.SideBuy && currentPrice <= stopRule.WallPrice*(1-buffer) {
			return true, "collapsed_wall_rebuilt"
		}
	}

	return false, ""
}

func (g *RiskGuard) RecordLoss(strategyID string, lossBps float64) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.dailyLoss += lossBps
	g.weeklyLoss += lossBps
	g.strategyLosses[strategyID] += lossBps
	g.consecutiveLosses[strategyID]++

	if g.dailyLoss >= g.hardRisk.MaxLossPerDayBps {
		g.readOnlyMode = true
	}
	if g.weeklyLoss >= g.hardRisk.MaxLossPerWeekBps {
		g.readOnlyMode = true
	}
	if g.strategyLosses[strategyID] >= g.hardRisk.MaxLossPerStrategyBps {
		g.disabledStrategies[strategyID] = "strategy_loss_limit_reached"
	}
	if g.consecutiveLosses[strategyID] >= g.hardRisk.MaxConsecutiveLosses {
		g.disabledStrategies[strategyID] = "consecutive_loss_limit_reached"
	}
}

func (g *RiskGuard) RecordWin(strategyID string) {
	g.mu.Lock()
	g.consecutiveLosses[strategyID] = 0
	g.mu.Unlock()
}

func (g *RiskGuard) IsReadOnly() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.readOnlyMode
}

func (g *RiskGuard) IsStrategyDisabled(strategyID string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	_, ok := g.disabledStrategies[strategyID]
	return ok
}

func (g *RiskGuard) resetTimers(now time.Time) {
	if now.Sub(g.dayStart) > 24*time.Hour {
		g.dailyLoss = 0
		g.readOnlyMode = false
		g.dayStart = now
	}
	if now.Sub(g.weekStart) > 7*24*time.Hour {
		g.weeklyLoss = 0
		for k := range g.disabledStrategies {
			if g.strategyLosses[k] < g.hardRisk.MaxLossPerStrategyBps {
				delete(g.disabledStrategies, k)
			}
		}
		g.weekStart = now
	}
}

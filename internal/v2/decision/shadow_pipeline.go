package decision

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/ljwqf/quant/internal/v2/computation"
	"github.com/ljwqf/quant/internal/v2/events"
	"github.com/ljwqf/quant/internal/v2/monitor"
	"github.com/ljwqf/quant/pkg/logger"
	"go.uber.org/zap"
)

type ShadowModeConfig struct {
	LogDir              string
	MissedSignalBps     float64
	OpenThreshold       float64
	CooldownDuration    time.Duration
	SpreadPenalty       float64
	StructuralStopBps   float64
	HuntingDeltaRatio   float64
	ImbalanceDeltaRatio float64
}

func DefaultShadowModeConfig() ShadowModeConfig {
	return ShadowModeConfig{
		LogDir:              "logs/shadow",
		MissedSignalBps:     200,
		OpenThreshold:       0.72,
		CooldownDuration:    30 * time.Second,
		SpreadPenalty:       0.25,
		StructuralStopBps:   20,
		HuntingDeltaRatio:   0.65,
		ImbalanceDeltaRatio: 0.55,
	}
}

type ShadowPipeline struct {
	config     ShadowModeConfig
	calculator *computation.LiquidityFactorCalculator
	machine    *LiquidityStateMachine
	missLogger *monitor.MissedSignalLogger
	archiver   *monitor.SnapshotArchiver
	mu         sync.Mutex
	lastPrices map[string]float64
	lastTimes  map[string]time.Time
	shadowLog  *os.File
	encoder    *json.Encoder
}

type ShadowSignalRecord struct {
	Timestamp        time.Time                `json:"timestamp"`
	Symbol           string                   `json:"symbol"`
	Intent           events.SignalIntent      `json:"intent"`
	MacroState       events.MacroState        `json:"macro_state"`
	FactorSnapshot   events.FactorSnapshot    `json:"factor_snapshot"`
	StateTransitions []events.StateTransition `json:"state_transitions"`
	PostPrices       map[string]float64       `json:"post_prices,omitempty"`
}

func NewShadowPipeline(config ShadowModeConfig) (*ShadowPipeline, error) {
	if config.LogDir == "" {
		config.LogDir = DefaultShadowModeConfig().LogDir
	}
	if config.MissedSignalBps == 0 {
		config.MissedSignalBps = DefaultShadowModeConfig().MissedSignalBps
	}

	if err := os.MkdirAll(config.LogDir, 0755); err != nil {
		return nil, err
	}

	factorConfig := computation.DefaultLiquidityFactorConfig()
	calculator := computation.NewLiquidityFactorCalculator(factorConfig)

	strategyConfig := LiquidityStrategyConfig{
		OpenThreshold:       config.OpenThreshold,
		CooldownDuration:    config.CooldownDuration,
		SpreadPenalty:       config.SpreadPenalty,
		StructuralStopBps:   config.StructuralStopBps,
		HuntingDeltaRatio:   config.HuntingDeltaRatio,
		ImbalanceDeltaRatio: config.ImbalanceDeltaRatio,
	}
	machine := NewLiquidityStateMachine(strategyConfig)

	missLogger, err := monitor.NewMissedSignalLogger(monitor.MissedSignalConfig{
		LargeMoveThresholdBps: config.MissedSignalBps,
		LogDir:                filepath.Join(config.LogDir, "missed_signals"),
	})
	if err != nil {
		return nil, err
	}

	archiver, err := monitor.NewSnapshotArchiver(monitor.SnapshotArchiverConfig{
		ArchiveDir: filepath.Join(config.LogDir, "snapshots"),
	})
	if err != nil {
		return nil, err
	}

	shadowLogPath := filepath.Join(config.LogDir, "shadow_signals.json")
	shadowLog, err := os.OpenFile(shadowLogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}

	return &ShadowPipeline{
		config:     config,
		calculator: calculator,
		machine:    machine,
		missLogger: missLogger,
		archiver:   archiver,
		lastPrices: make(map[string]float64),
		lastTimes:  make(map[string]time.Time),
		shadowLog:  shadowLog,
		encoder:    json.NewEncoder(shadowLog),
	}, nil
}

func (p *ShadowPipeline) ProcessTick(tick events.TickEvent) {
	p.calculator.AddTrade(tick)
	p.mu.Lock()
	prevPrice := p.lastPrices[tick.Symbol]
	prevTime := p.lastTimes[tick.Symbol]
	p.lastPrices[tick.Symbol] = tick.Price
	p.lastTimes[tick.Symbol] = tick.Timestamp
	p.mu.Unlock()

	if prevPrice > 0 && prevTime.After(time.Time{}) {
		duration := tick.Timestamp.Sub(prevTime).Seconds()
		if duration > 0 {
			returnBps := (tick.Price - prevPrice) / prevPrice * 10000
			state := p.machine.State(tick.Symbol)
			p.missLogger.Check(returnBps, tick.Symbol, state, events.FactorSnapshot{Symbol: tick.Symbol, Timestamp: tick.Timestamp, MidPrice: tick.Price}, "no_signal_on_tick")
		}
	}
}

func (p *ShadowPipeline) ProcessOrderBook(book events.OrderBookEvent) events.SignalIntent {
	snapshot := p.calculator.Calculate(book)

	macro := inferMacro(snapshot)

	intent := p.machine.Evaluate(snapshot, macro)

	if intent.RejectReason == "" && intent.State == events.StrategyStatePositionOpen {
		logger.Info("shadow_signal_generated",
			zap.String("symbol", intent.Symbol),
			zap.String("strategy_id", intent.StrategyID),
			zap.String("side", string(intent.Side)),
			zap.Float64("score", intent.Score),
			zap.Float64("expected_entry", intent.ExpectedEntry),
			zap.Float64("structural_stop", intent.StructuralStop),
		)
	}

	transitions := p.machine.Transitions()

	for _, transition := range transitions {
		p.archiver.Archive(monitor.SnapshotArchive{
			Timestamp:   transition.TransitionAt,
			Symbol:      transition.Symbol,
			EventType:   "state_transition",
			StateBefore: transition.From,
			StateAfter:  transition.To,
			Snapshot:    transition.Snapshot,
		})
	}

	record := ShadowSignalRecord{
		Timestamp:        snapshot.Timestamp,
		Symbol:           snapshot.Symbol,
		Intent:           intent,
		MacroState:       macro,
		FactorSnapshot:   snapshot,
		StateTransitions: transitions,
	}
	p.mu.Lock()
	p.encoder.Encode(record)
	p.mu.Unlock()

	p.missLogger.Check(0, book.Symbol, intent.State, snapshot, intent.RejectReason)

	return intent
}

func (p *ShadowPipeline) Flush() error {
	return p.missLogger.Flush()
}

func (p *ShadowPipeline) Close() error {
	p.Flush()
	return p.shadowLog.Close()
}

func inferMacro(snapshot events.FactorSnapshot) events.MacroState {
	if snapshot.OBI > 0.4 && snapshot.Delta > 0 && snapshot.TopAskWall.AbsorbRate < 1 {
		return events.MacroStateTrendUp
	}
	if snapshot.OBI < -0.4 && snapshot.Delta < 0 && snapshot.TopBidWall.AbsorbRate < 1 {
		return events.MacroStateTrendDown
	}
	if math.Abs(snapshot.OBI) < 0.15 && snapshot.MomentumContinuation < 0.3 {
		return events.MacroStateMomentumDecay
	}
	return events.MacroStateUnknown
}

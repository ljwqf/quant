package decision

import (
	"context"
	"sync"
	"time"

	"github.com/ljwqf/quant/internal/v2/computation"
	"github.com/ljwqf/quant/internal/v2/events"
	"github.com/ljwqf/quant/internal/v2/execution"
	"github.com/ljwqf/quant/internal/v2/ingestion"
	"github.com/ljwqf/quant/internal/v2/monitor"
)

type FullPipelineConfig struct {
	Symbols              []string
	Intervals            []string
	LogDir               string
	MissedSignalBps      float64
	OpenThreshold        float64
	CooldownDuration     time.Duration
	SpreadPenalty        float64
	StructuralStopBps    float64
	HuntingDeltaRatio    float64
	ImbalanceDeltaRatio  float64
	BaseCapital          float64
	BaseRiskPercent      float64
	ProfitRiskPercent    float64
	MaxLossPerTradeBps   float64
	MaxLossPerDayBps     float64
	MaxLossPerWeekBps    float64
	MaxConsecutiveLosses int
}

func DefaultFullPipelineConfig() FullPipelineConfig {
	return FullPipelineConfig{
		Symbols:              []string{"BTC-USDT", "ETH-USDT"},
		Intervals:            []string{"1m", "5m", "1H", "4H"},
		LogDir:               "logs/v2_pipeline",
		MissedSignalBps:      200,
		OpenThreshold:        0.72,
		CooldownDuration:     30 * time.Second,
		SpreadPenalty:        0.25,
		StructuralStopBps:    20,
		HuntingDeltaRatio:    0.65,
		ImbalanceDeltaRatio:  0.55,
		BaseCapital:          10000,
		BaseRiskPercent:      0.01,
		ProfitRiskPercent:    0.02,
		MaxLossPerTradeBps:   100,
		MaxLossPerDayBps:     500,
		MaxLossPerWeekBps:    1500,
		MaxConsecutiveLosses: 5,
	}
}

type FullPipeline struct {
	config     FullPipelineConfig
	ingestion  *ingestion.IngestionManager
	calculator *computation.LiquidityFactorCalculator
	machine    *LiquidityStateMachine
	pool       *execution.ProfitPool
	guard      *execution.RiskGuard
	paper      *execution.PaperExecutor
	missLogger *monitor.MissedSignalLogger
	archiver   *monitor.SnapshotArchiver
	persist    *StatePersistence
	mu         sync.Mutex
	running    bool
}

func NewFullPipeline(config FullPipelineConfig, provider ingestion.DataProvider) (*FullPipeline, error) {
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

	pool := execution.NewProfitPool(execution.ProfitPoolConfig{
		BaseRiskPercent:   config.BaseRiskPercent,
		ProfitRiskPercent: config.ProfitRiskPercent,
		BaseCapital:       config.BaseCapital,
	})

	guard := execution.NewRiskGuard(execution.HardRiskConfig{
		MaxLossPerTradeBps:   config.MaxLossPerTradeBps,
		MaxLossPerDayBps:     config.MaxLossPerDayBps,
		MaxLossPerWeekBps:    config.MaxLossPerWeekBps,
		MaxConsecutiveLosses: config.MaxConsecutiveLosses,
	})

	ingestManager := ingestion.NewIngestionManager(provider, config.Symbols, config.Intervals)

	missLogger, err := monitor.NewMissedSignalLogger(monitor.MissedSignalConfig{
		LargeMoveThresholdBps: config.MissedSignalBps,
		LogDir:                config.LogDir + "/missed_signals",
	})
	if err != nil {
		return nil, err
	}

	archiver, err := monitor.NewSnapshotArchiver(monitor.SnapshotArchiverConfig{
		ArchiveDir: config.LogDir + "/snapshots",
	})
	if err != nil {
		return nil, err
	}

	persist, err := NewStatePersistence(StatePersistenceConfig{
		StateDir: config.LogDir + "/state",
	})
	if err != nil {
		return nil, err
	}

	paper, err := execution.NewPaperExecutor(execution.PaperExecutorConfig{
		LogDir: config.LogDir + "/paper_trades",
	}, pool, guard)
	if err != nil {
		return nil, err
	}

	for _, symbol := range config.Symbols {
		persist.RestoreState(symbol, machine)
	}

	return &FullPipeline{
		config:     config,
		ingestion:  ingestManager,
		calculator: calculator,
		machine:    machine,
		pool:       pool,
		guard:      guard,
		paper:      paper,
		missLogger: missLogger,
		archiver:   archiver,
		persist:    persist,
	}, nil
}

func (p *FullPipeline) Start(ctx context.Context) error {
	if err := p.ingestion.Start(ctx); err != nil {
		return err
	}

	p.mu.Lock()
	p.running = true
	p.mu.Unlock()

	go p.runComputationLoop(ctx)
	go p.runPersistenceLoop(ctx)

	return nil
}

func (p *FullPipeline) Stop() {
	p.mu.Lock()
	p.running = false
	p.mu.Unlock()

	p.ingestion.Stop()
	p.persistAll()
	p.missLogger.Close()
	p.paper.Close()
}

func (p *FullPipeline) runComputationLoop(ctx context.Context) {
	for _, symbol := range p.config.Symbols {
		bookCh := p.ingestion.OrderBookChannel(symbol)
		tickCh := p.ingestion.TickChannel(symbol)
		if bookCh == nil || tickCh == nil {
			continue
		}

		go p.processSymbol(ctx, symbol, tickCh, bookCh)
	}
}

func (p *FullPipeline) processSymbol(ctx context.Context, symbol string, tickCh chan events.TickEvent, bookCh chan events.OrderBookEvent) {
	for {
		select {
		case <-ctx.Done():
			return
		case tick, ok := <-tickCh:
			if !ok {
				return
			}
			p.calculator.AddTrade(tick)
		case book, ok := <-bookCh:
			if !ok {
				return
			}
			snapshot := p.calculator.Calculate(book)
			macro := inferMacro(snapshot)
			intent := p.machine.Evaluate(snapshot, macro)

			for _, transition := range p.machine.Transitions() {
				p.archiver.Archive(monitor.SnapshotArchive{
					Timestamp:   transition.TransitionAt,
					Symbol:      transition.Symbol,
					EventType:   "state_transition",
					StateBefore: transition.From,
					StateAfter:  transition.To,
					Snapshot:    transition.Snapshot,
				})
			}

			if intent.RejectReason == "" && intent.State == events.StrategyStatePositionOpen && !p.guard.IsReadOnly() {
				pos, riskResult := p.paper.OpenPosition(intent)
				if pos != nil && riskResult.Allowed {
					p.persist.Save(symbol, intent.State, intent.Timestamp, snapshot)
				}
			}

			p.missLogger.Check(0, symbol, intent.State, snapshot, intent.RejectReason)
			p.missLogger.Flush()
		}
	}
}

func (p *FullPipeline) runPersistenceLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.persistAll()
		}
	}
}

func (p *FullPipeline) persistAll() {
	for _, symbol := range p.config.Symbols {
		state := p.machine.State(symbol)
		p.persist.Save(symbol, state, time.Now(), events.FactorSnapshot{Symbol: symbol})
	}
}

func (p *FullPipeline) Status() map[string]interface{} {
	p.mu.Lock()
	defer p.mu.Unlock()

	status := make(map[string]interface{})
	for _, symbol := range p.config.Symbols {
		status[symbol+"_state"] = p.machine.State(symbol)
	}
	status["profit_pool"] = p.pool.ProfitPool()
	status["base_capital"] = p.pool.BaseCapital()
	status["read_only"] = p.guard.IsReadOnly()
	status["paper_stats"] = p.paper.Stats()
	status["running"] = p.running
	return status
}

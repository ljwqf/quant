package execution

import (
	"time"

	"github.com/ljwqf/quant/internal/v2/events"
)

type RiskAllocator interface {
	AllocateRisk(score float64) RiskAllocation
	RecordRealizedPnL(pnl float64)
	BaseCapital() float64
	ProfitPool() float64
}

type RiskChecker interface {
	CheckEntry(intent events.SignalIntent, baseCapital float64) RiskCheckResult
	CheckStructuralStop(positionSide events.Side, currentPrice float64, rule StructuralStopRule) (bool, string)
	IsReadOnly() bool
	IsStrategyDisabled(strategyID string) bool
}

type PositionManager interface {
	OpenPosition(intent events.SignalIntent) (*PaperPosition, RiskCheckResult)
	ClosePosition(symbol, strategyID string, exitPrice float64, now time.Time, reason string)
	UpdatePrice(symbol string, currentPrice float64, now time.Time)
	Stats() map[string]interface{}
}

type ExecutionPipeline interface {
	OnTick(tick events.TickEvent)
	OnOrderBook(book events.OrderBookEvent)
	OnKline(kline events.KlineEvent)
	Status() map[string]interface{}
}

type PipelineConfig struct {
	PaperExecutorConfig PaperExecutorConfig
	ProfitPoolConfig    ProfitPoolConfig
	HardRiskConfig      HardRiskConfig
	LogDir              string
	MissedSignalBps     float64
	OpenThreshold       float64
	CooldownDuration    time.Duration
	SpreadPenalty       float64
	StructuralStopBps   float64
	HuntingDeltaRatio   float64
	ImbalanceDeltaRatio float64
}

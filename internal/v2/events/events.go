package events

import "time"

type Side string

const (
	SideBuy  Side = "buy"
	SideSell Side = "sell"
)

type TradeDirection string

const (
	TradeDirectionBuy     TradeDirection = "aggressive_buy"
	TradeDirectionSell    TradeDirection = "aggressive_sell"
	TradeDirectionUnknown TradeDirection = "unknown"
)

type StrategyState string

const (
	StrategyStateIdle                 StrategyState = "Idle"
	StrategyStateSweepingHunting      StrategyState = "Sweeping_Hunting"
	StrategyStateConsolidationSetting StrategyState = "Consolidation_Setting"
	StrategyStateImbalanceConfirmed   StrategyState = "Imbalance_Confirmed"
	StrategyStatePositionOpen         StrategyState = "Position_Open"
	StrategyStateInvalidated          StrategyState = "Invalidated"
	StrategyStateCooldown             StrategyState = "Cooldown"
)

type MacroState string

const (
	MacroStateUnknown       MacroState = "Unknown"
	MacroStateTrendUp       MacroState = "Trend_Up"
	MacroStateTrendDown     MacroState = "Trend_Down"
	MacroStateMomentumDecay MacroState = "Momentum_Decay"
)

type OrderBookLevel struct {
	Price    float64
	Quantity float64
}

type TickEvent struct {
	Symbol    string
	Price     float64
	Quantity  float64
	Direction TradeDirection
	Timestamp time.Time
}

type OrderBookEvent struct {
	Symbol    string
	Bids      []OrderBookLevel
	Asks      []OrderBookLevel
	Timestamp time.Time
	Sequence  int64
	Checksum  int64
	Snapshot  bool
}

type KlineEvent struct {
	Symbol    string
	Open      float64
	High      float64
	Low       float64
	Close     float64
	Volume    float64
	Interval  string
	Timestamp time.Time
}

type LiquidityWall struct {
	Side             Side
	Price            float64
	Size             float64
	DistanceFromMid  float64
	Collapse         bool
	PreviousSize     float64
	WallDelta        float64
	AbsorbRate       float64
	PassiveAddVolume float64
}

type VacuumZone struct {
	Side  Side
	Start float64
	End   float64
	Score float64
}

type FactorSnapshot struct {
	Symbol                  string
	Timestamp               time.Time
	MidPrice                float64
	Spread                  float64
	SpreadStable            bool
	OBI                     float64
	Delta                   float64
	AggressiveBuyVolume     float64
	AggressiveSellVolume    float64
	TopBidWall              LiquidityWall
	TopAskWall              LiquidityWall
	BidVacuum               VacuumZone
	AskVacuum               VacuumZone
	WallAbsorptionScore     float64
	BuyExhaustionScore      float64
	BreakoutStrengthScore   float64
	MomentumContinuation    float64
	PassiveActiveDivergence float64
}

func (s FactorSnapshot) DeltaRatio() float64 {
	total := s.AggressiveBuyVolume + s.AggressiveSellVolume
	if total <= 0 {
		return 0
	}
	return s.Delta / total
}

type StateTransition struct {
	Symbol       string
	From         StrategyState
	To           StrategyState
	Reason       string
	Snapshot     FactorSnapshot
	TransitionAt time.Time
}

type SignalIntent struct {
	Symbol         string
	StrategyID     string
	Side           Side
	State          StrategyState
	MacroState     MacroState
	Score          float64
	OpenThreshold  float64
	ExpectedEntry  float64
	StructuralStop float64
	RejectReason   string
	Snapshot       FactorSnapshot
	Timestamp      time.Time
}

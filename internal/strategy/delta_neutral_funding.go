package strategy

import (
	"math"
	"sync"
	"time"

	"github.com/ljwqf/quant/pkg/logger"
	"github.com/ljwqf/quant/pkg/types"
	"go.uber.org/zap"
)

const (
	FundUsagePercent       = 0.9
	RebalanceThreshold     = 0.02
	BasisCircuitBreaker    = 0.01
	TargetHedgeRatio       = 1.0
	HedgeRatioTolerance    = 0.05
	DailyLossLimitFunding  = 0.02
	MarginBufferPercent    = 0.1
	CheckInterval          = 30 * time.Second
	SettlementWindowBefore = 30 * time.Minute
	SettlementWindowAfter  = 30 * time.Minute
)

type FundingState int

const (
	FundingStateIdle FundingState = iota
	FundingStateActive
	FundingStateRebalancing
	FundingStatePaused
	FundingStateStopped
)

type PositionSide int

const (
	PositionSideSpot PositionSide = iota
	PositionSidePerp
)

type DeltaPosition struct {
	Symbol     string
	Side       PositionSide
	Size       float64
	EntryPrice float64
	MarkPrice  float64
	Value      float64
	Timestamp  time.Time
}

type FundingData struct {
	Rate           float64
	NextRate       float64
	NextSettlement time.Time
	Timestamp      time.Time
}

type DeltaNeutralFundingPro struct {
	name                   string
	params                 map[string]interface{}
	metrics                map[string]interface{}
	state                  FundingState
	fundUsagePercent       float64
	rebalanceThreshold     float64
	basisCircuitBreaker    float64
	targetHedgeRatio       float64
	hedgeRatioTolerance    float64
	dailyLossLimit         float64
	marginBufferPercent    float64
	settlementWindowBefore time.Duration
	settlementWindowAfter  time.Duration
	spotSymbol             string
	perpSymbol             string
	spotPosition           *DeltaPosition
	perpPosition           *DeltaPosition
	positionMutex          sync.RWMutex
	fundingData            *FundingData
	fundingMutex           sync.RWMutex
	spotPrice              float64
	perpPrice              float64
	priceMutex             sync.RWMutex
	dailyLoss              float64
	dailyLossReset         time.Time
	totalPnL               float64
	fundingIncome          float64
	tradeCount             int
	metricsMutex           sync.Mutex
	nowFunc                func() time.Time
	stopChan               chan struct{}
	rebalanceChan          chan struct{}
	killSwitchChan         chan struct{}
	smartFilter            *SmartFilter
}

func NewDeltaNeutralFundingPro() *DeltaNeutralFundingPro {
	strategy := &DeltaNeutralFundingPro{
		name:                   "DeltaNeutralFunding-Pro",
		params:                 make(map[string]interface{}),
		metrics:                make(map[string]interface{}),
		state:                  FundingStateIdle,
		fundUsagePercent:       FundUsagePercent,
		rebalanceThreshold:     RebalanceThreshold,
		basisCircuitBreaker:    BasisCircuitBreaker,
		targetHedgeRatio:       TargetHedgeRatio,
		hedgeRatioTolerance:    HedgeRatioTolerance,
		dailyLossLimit:         DailyLossLimitFunding,
		marginBufferPercent:    MarginBufferPercent,
		settlementWindowBefore: SettlementWindowBefore,
		settlementWindowAfter:  SettlementWindowAfter,
		spotSymbol:             "BTC-USDT",
		perpSymbol:             "BTC-USDT-SWAP",
		nowFunc:                time.Now,
		stopChan:               make(chan struct{}),
		rebalanceChan:          make(chan struct{}, 1),
		killSwitchChan:         make(chan struct{}, 1),
	}
	strategy.dailyLossReset = strategy.nowFunc()
	return strategy
}

func (e *DeltaNeutralFundingPro) Name() string {
	return e.name
}

func (e *DeltaNeutralFundingPro) Init(params map[string]interface{}) error {
	for k, v := range params {
		e.params[k] = v
	}

	if symbol, ok := params["spot_symbol"].(string); ok {
		e.spotSymbol = symbol
	}
	if symbol, ok := params["perp_symbol"].(string); ok {
		e.perpSymbol = symbol
	}
	if value, ok := params["fund_usage_percent"].(float64); ok && value > 0 {
		e.fundUsagePercent = value
	}
	if value, ok := params["rebalance_threshold"].(float64); ok && value >= 0 {
		e.rebalanceThreshold = value
	}
	if value, ok := params["basis_circuit_breaker"].(float64); ok && value >= 0 {
		e.basisCircuitBreaker = value
	}
	if target, ok := params["target_hedge_ratio"].(float64); ok && target > 0 {
		e.targetHedgeRatio = target
	}
	if tolerance, ok := params["hedge_ratio_tolerance"].(float64); ok && tolerance >= 0 {
		e.hedgeRatioTolerance = tolerance
	}
	if value, ok := params["daily_loss_limit"].(float64); ok && value >= 0 {
		e.dailyLossLimit = value
	}
	if value, ok := params["margin_buffer_percent"].(float64); ok && value >= 0 {
		e.marginBufferPercent = value
	}
	if value, ok := params["settlement_window_before"].(time.Duration); ok && value >= 0 {
		e.settlementWindowBefore = value
	}
	if value, ok := params["settlement_window_after"].(time.Duration); ok && value >= 0 {
		e.settlementWindowAfter = value
	}

	e.state = FundingStateActive
	logger.Info("DeltaNeutralFunding-Pro初始化完成",
		zap.String("spot_symbol", e.spotSymbol),
		zap.String("perp_symbol", e.perpSymbol),
		zap.Float64("fund_usage_percent", e.fundUsagePercent),
		zap.Float64("rebalance_threshold", e.rebalanceThreshold),
		zap.Float64("basis_circuit_breaker", e.basisCircuitBreaker),
		zap.Float64("target_hedge_ratio", e.targetHedgeRatio),
		zap.Float64("hedge_ratio_tolerance", e.hedgeRatioTolerance),
		zap.Float64("daily_loss_limit", e.dailyLossLimit),
		zap.Float64("margin_buffer_percent", e.marginBufferPercent),
		zap.Duration("settlement_window_before", e.settlementWindowBefore),
		zap.Duration("settlement_window_after", e.settlementWindowAfter),
	)

	return nil
}

func (e *DeltaNeutralFundingPro) OnTick(tick *types.Tick) (*types.Signal, error) {
	if tick.Symbol == e.spotSymbol {
		e.UpdateSpotPrice(tick.Price)
	} else if tick.Symbol == e.perpSymbol {
		e.UpdatePerpPrice(tick.Price)
	}

	if rebalanceSignal := e.CheckRebalance(); rebalanceSignal != nil {
		logger.Info("检测到再平衡机会",
			zap.String("symbol", rebalanceSignal.Symbol),
			zap.Float64("drift", rebalanceSignal.Drift),
		)
	}

	return nil, nil
}

func (e *DeltaNeutralFundingPro) OnBar(bar *types.Bar) (*types.Signal, error) {
	if bar.Symbol == e.spotSymbol {
		e.UpdateSpotPrice(bar.Close)
	} else if bar.Symbol == e.perpSymbol {
		e.UpdatePerpPrice(bar.Close)
	}
	return nil, nil
}

func (e *DeltaNeutralFundingPro) OnOrderBook(orderBook *types.OrderBook) (*types.Signal, error) {
	return nil, nil
}

func (e *DeltaNeutralFundingPro) GetParams() map[string]interface{} {
	return e.params
}

func (e *DeltaNeutralFundingPro) SetParams(params map[string]interface{}) {
	for k, v := range params {
		e.params[k] = v
	}
}

func (e *DeltaNeutralFundingPro) GetMetrics() map[string]interface{} {
	e.metricsMutex.Lock()
	defer e.metricsMutex.Unlock()

	e.metrics["state"] = int(e.state)
	e.metrics["daily_loss"] = e.dailyLoss
	e.metrics["total_pnl"] = e.totalPnL
	e.metrics["funding_income"] = e.fundingIncome
	e.metrics["trade_count"] = e.tradeCount
	e.metrics["spot_price"] = e.spotPrice
	e.metrics["perp_price"] = e.perpPrice
	e.metrics["basis"] = e.calculateBasis()
	e.metrics["delta_drift"] = e.calculateDeltaDrift()

	if e.spotPosition != nil {
		e.metrics["spot_value"] = e.spotPosition.Value
	}
	if e.perpPosition != nil {
		e.metrics["perp_value"] = e.perpPosition.Value
	}

	if e.smartFilter != nil {
		filterMetrics := e.smartFilter.GetMetrics()
		for k, v := range filterMetrics {
			e.metrics["smart_filter_"+k] = v
		}
	}

	return e.metrics
}

func (e *DeltaNeutralFundingPro) UpdateSpotPrice(price float64) {
	e.priceMutex.Lock()
	defer e.priceMutex.Unlock()
	e.spotPrice = price
}

func (e *DeltaNeutralFundingPro) UpdatePerpPrice(price float64) {
	e.priceMutex.Lock()
	defer e.priceMutex.Unlock()
	e.perpPrice = price
}

func (e *DeltaNeutralFundingPro) UpdateFundingData(rate, nextRate float64, nextSettlement time.Time) {
	e.fundingMutex.Lock()
	defer e.fundingMutex.Unlock()

	e.fundingData = &FundingData{
		Rate:           rate,
		NextRate:       nextRate,
		NextSettlement: nextSettlement,
		Timestamp:      time.Now(),
	}

	logger.Info("更新资金费数据",
		zap.Float64("rate", rate),
		zap.Float64("next_rate", nextRate),
		zap.Time("next_settlement", nextSettlement),
	)
}

func (e *DeltaNeutralFundingPro) calculateBasis() float64 {
	e.priceMutex.RLock()
	defer e.priceMutex.RUnlock()

	if e.spotPrice <= 0 {
		return 0
	}

	return (e.perpPrice - e.spotPrice) / e.spotPrice
}

func (e *DeltaNeutralFundingPro) calculateDeltaDrift() float64 {
	e.positionMutex.RLock()
	defer e.positionMutex.RUnlock()

	if e.spotPosition == nil || e.perpPosition == nil {
		return 0
	}

	if e.spotPosition.Value <= 0 {
		return 0
	}

	return math.Abs(e.spotPosition.Value-e.perpPosition.Value) / e.spotPosition.Value
}

func calculateExposureDrift(spotExposure, perpExposure float64) float64 {
	if spotExposure <= 0 {
		if perpExposure > 0 {
			return 1
		}
		return 0
	}
	return math.Abs(spotExposure-perpExposure) / spotExposure
}

func calculateHedgeRatio(spotExposure, perpExposure float64) float64 {
	if spotExposure <= 0 {
		return 0
	}
	return perpExposure / spotExposure
}

func solveDeltaTopUp(spotExposure, perpExposure, shortfallAmount, targetRatio float64) (float64, float64) {
	if shortfallAmount <= 0 {
		return 0, 0
	}
	if targetRatio <= 0 {
		targetRatio = TargetHedgeRatio
	}

	spotAdd := (shortfallAmount - targetRatio*spotExposure + perpExposure) / (1 + targetRatio)
	if spotAdd < 0 {
		spotAdd = 0
	}
	if spotAdd > shortfallAmount {
		spotAdd = shortfallAmount
	}
	perpAdd := shortfallAmount - spotAdd
	if perpAdd < 0 {
		perpAdd = 0
	}
	return spotAdd, perpAdd
}

func (e *DeltaNeutralFundingPro) getExposureState() (float64, float64) {
	e.positionMutex.RLock()
	spotPosition := e.spotPosition
	perpPosition := e.perpPosition
	e.positionMutex.RUnlock()

	e.priceMutex.RLock()
	spotPrice := e.spotPrice
	perpPrice := e.perpPrice
	e.priceMutex.RUnlock()

	spotExposure := 0.0
	if spotPosition != nil {
		price := spotPosition.MarkPrice
		if spotPrice > 0 {
			price = spotPrice
		}
		spotExposure = spotPosition.Size * price
	}

	perpExposure := 0.0
	if perpPosition != nil {
		price := perpPosition.MarkPrice
		if perpPrice > 0 {
			price = perpPrice
		}
		perpExposure = perpPosition.Size * price
	}

	return spotExposure, perpExposure
}

func (e *DeltaNeutralFundingPro) isInSettlementWindow() bool {
	e.fundingMutex.RLock()
	defer e.fundingMutex.RUnlock()

	if e.fundingData == nil {
		return false
	}

	now := e.nowFunc()
	settlementTime := e.fundingData.NextSettlement

	if now.After(settlementTime.Add(-e.settlementWindowBefore)) &&
		now.Before(settlementTime.Add(e.settlementWindowAfter)) {
		return true
	}

	return false
}

func (e *DeltaNeutralFundingPro) checkCircuitBreaker() bool {
	e.resetDailyLossIfNeeded(e.nowFunc())

	if e.dailyLoss >= e.dailyLossLimit {
		return true
	}

	basis := e.calculateBasis()
	if math.Abs(basis) > e.basisCircuitBreaker {
		logger.Warn("基差熔断触发",
			zap.Float64("basis", basis),
			zap.Float64("threshold", e.basisCircuitBreaker),
		)
		return true
	}

	return false
}

func (e *DeltaNeutralFundingPro) InitializePosition(accountValue float64) error {
	if e.state != FundingStateActive {
		return nil
	}

	if e.isInSettlementWindow() {
		logger.Warn("在结算窗口内，延迟建仓")
		return nil
	}

	if !e.checkSmartFilter() {
		logger.Warn("SmartFilter 禁止中性策略运行，跳过建仓")
		return nil
	}

	positionValue := accountValue * e.fundUsagePercent * (1 - e.marginBufferPercent)
	halfValue := positionValue / 2

	e.priceMutex.RLock()
	spotPrice := e.spotPrice
	perpPrice := e.perpPrice
	e.priceMutex.RUnlock()

	if spotPrice <= 0 || perpPrice <= 0 {
		logger.Warn("价格数据无效，无法建仓")
		return nil
	}

	spotSize := halfValue / spotPrice
	perpSize := halfValue / perpPrice

	e.positionMutex.Lock()
	defer e.positionMutex.Unlock()

	e.spotPosition = &DeltaPosition{
		Symbol:     e.spotSymbol,
		Side:       PositionSideSpot,
		Size:       spotSize,
		EntryPrice: spotPrice,
		MarkPrice:  spotPrice,
		Value:      halfValue,
		Timestamp:  time.Now(),
	}

	e.perpPosition = &DeltaPosition{
		Symbol:     e.perpSymbol,
		Side:       PositionSidePerp,
		Size:       perpSize,
		EntryPrice: perpPrice,
		MarkPrice:  perpPrice,
		Value:      halfValue,
		Timestamp:  time.Now(),
	}

	e.tradeCount += 2

	logger.Info("DeltaNeutralFunding-Pro建仓完成",
		zap.Float64("account_value", accountValue),
		zap.Float64("position_value", positionValue),
		zap.Float64("spot_size", spotSize),
		zap.Float64("perp_size", perpSize),
	)

	return nil
}

func (e *DeltaNeutralFundingPro) CheckRebalance() *RebalanceSignal {
	if e.state != FundingStateActive {
		return nil
	}

	if e.isInSettlementWindow() {
		return nil
	}

	if e.checkCircuitBreaker() {
		return nil
	}

	if !e.checkSmartFilter() {
		logger.Warn("SmartFilter 禁止中性策略运行，跳过再平衡")
		return nil
	}

	e.positionMutex.RLock()
	spotPos := e.spotPosition
	perpPos := e.perpPosition
	e.positionMutex.RUnlock()

	if spotPos == nil || perpPos == nil {
		return nil
	}

	e.priceMutex.RLock()
	spotPrice := e.spotPrice
	perpPrice := e.perpPrice
	e.priceMutex.RUnlock()

	spotPos.MarkPrice = spotPrice
	spotPos.Value = spotPos.Size * spotPrice

	perpPos.MarkPrice = perpPrice
	perpPos.Value = perpPos.Size * perpPrice

	drift := e.calculateDeltaDrift()

	if drift > e.rebalanceThreshold {
		var adjustSide PositionSide
		var adjustSize float64

		if spotPos.Value > perpPos.Value {
			adjustSide = PositionSidePerp
			adjustSize = (spotPos.Value - perpPos.Value) / perpPrice
		} else {
			adjustSide = PositionSideSpot
			adjustSize = (perpPos.Value - spotPos.Value) / spotPrice
		}

		return &RebalanceSignal{
			Symbol:     e.perpSymbol,
			AdjustSide: adjustSide,
			AdjustSize: adjustSize,
			SpotValue:  spotPos.Value,
			PerpValue:  perpPos.Value,
			Drift:      drift,
			Timestamp:  time.Now(),
		}
	}

	return nil
}

func (e *DeltaNeutralFundingPro) ExecuteRebalance(signal *RebalanceSignal) error {
	e.positionMutex.Lock()
	defer e.positionMutex.Unlock()

	if signal.AdjustSide == PositionSidePerp {
		e.perpPosition.Size += signal.AdjustSize
		e.perpPosition.Value = e.perpPosition.Size * e.perpPosition.MarkPrice
	} else {
		e.spotPosition.Size += signal.AdjustSize
		e.spotPosition.Value = e.spotPosition.Size * e.spotPosition.MarkPrice
	}

	e.tradeCount++

	logger.Info("执行再平衡",
		zap.String("adjust_side", string(rune(signal.AdjustSide))),
		zap.Float64("adjust_size", signal.AdjustSize),
		zap.Float64("drift", signal.Drift),
	)

	return nil
}

func (e *DeltaNeutralFundingPro) RecordFundingIncome(income float64) {
	e.metricsMutex.Lock()
	defer e.metricsMutex.Unlock()

	e.fundingIncome += income
	e.totalPnL += income

	logger.Info("记录资金费收入",
		zap.Float64("income", income),
		zap.Float64("total_funding_income", e.fundingIncome),
	)
}

func (e *DeltaNeutralFundingPro) RecordPnL(pnl float64) {
	e.metricsMutex.Lock()
	defer e.metricsMutex.Unlock()
	e.resetDailyLossIfNeeded(e.nowFunc())

	e.totalPnL += pnl
	if pnl < 0 {
		e.dailyLoss += -pnl
	}
}

func (e *DeltaNeutralFundingPro) resetDailyLossIfNeeded(now time.Time) {
	if e.dailyLossReset.IsZero() || !isSameDay(now, e.dailyLossReset) {
		e.dailyLoss = 0
		e.dailyLossReset = now
	}
}

func (e *DeltaNeutralFundingPro) GetState() FundingState {
	return e.state
}

func (e *DeltaNeutralFundingPro) Pause() {
	e.state = FundingStatePaused
	logger.Info("DeltaNeutralFunding-Pro已暂停")
}

func (e *DeltaNeutralFundingPro) Resume() {
	e.state = FundingStateActive
	logger.Info("DeltaNeutralFunding-Pro已恢复")
}

func (e *DeltaNeutralFundingPro) Stop() {
	e.state = FundingStateStopped
	close(e.stopChan)
	logger.Info("DeltaNeutralFunding-Pro已停止")
}

func (e *DeltaNeutralFundingPro) EmergencyClose() error {
	logger.Warn("紧急平仓触发")

	e.positionMutex.Lock()
	defer e.positionMutex.Unlock()

	e.spotPosition = nil
	e.perpPosition = nil

	logger.Info("DeltaNeutralFunding-Pro紧急平仓完成")

	return nil
}

func (e *DeltaNeutralFundingPro) GetKillSwitchChannel() <-chan struct{} {
	return e.killSwitchChan
}

func (e *DeltaNeutralFundingPro) TriggerKillSwitch() {
	select {
	case e.killSwitchChan <- struct{}{}:
		logger.Warn("Kill Switch已触发")
	default:
	}
}

func (e *DeltaNeutralFundingPro) GetPositions() (*DeltaPosition, *DeltaPosition) {
	e.positionMutex.RLock()
	defer e.positionMutex.RUnlock()

	return e.spotPosition, e.perpPosition
}

func (e *DeltaNeutralFundingPro) GetFundingData() *FundingData {
	e.fundingMutex.RLock()
	defer e.fundingMutex.RUnlock()

	return e.fundingData
}

func (e *DeltaNeutralFundingPro) SetSmartFilter(filter *SmartFilter) {
	if filter == nil {
		return
	}

	e.smartFilter = filter
	logger.Info("DeltaNeutralFunding-Pro 已设置 SmartFilter")
}

func (e *DeltaNeutralFundingPro) UpdateOnChainData(netflow, sopr, mvrv float64) {
	if e.smartFilter == nil {
		e.smartFilter = NewSmartFilter()
	}

	e.smartFilter.UpdateOnChainData(netflow, sopr, mvrv)
	logger.Info("DeltaNeutralFunding-Pro 已更新链上数据",
		zap.Float64("netflow", netflow),
		zap.Float64("sopr", sopr),
		zap.Float64("mvrv", mvrv),
	)
}

func (e *DeltaNeutralFundingPro) checkSmartFilter() bool {
	if e.smartFilter == nil {
		return true
	}

	return e.smartFilter.CanRunNeutralStrategy()
}

type RebalanceSignal struct {
	Symbol     string
	AdjustSide PositionSide
	AdjustSize float64
	SpotValue  float64
	PerpValue  float64
	Drift      float64
	Timestamp  time.Time
}

func (e *DeltaNeutralFundingPro) OnPositionFilled(symbol string, side types.OrderSide, entryPrice, size float64) {
	logger.Info("DeltaNeutralFunding-Pro持仓已填充",
		zap.String("symbol", symbol),
		zap.String("side", string(side)),
		zap.Float64("entry_price", entryPrice),
		zap.Float64("size", size),
	)

	if symbol == e.spotSymbol {
		e.positionMutex.Lock()
		if e.spotPosition == nil {
			e.spotPosition = &DeltaPosition{
				Symbol:     symbol,
				Side:       PositionSideSpot,
				Size:       size,
				EntryPrice: entryPrice,
				MarkPrice:  entryPrice,
				Value:      entryPrice * size,
				Timestamp:  time.Now(),
			}
		} else {
			e.spotPosition.Size += size
			e.spotPosition.Value = e.spotPosition.Size * e.spotPosition.MarkPrice
		}
		e.positionMutex.Unlock()
		e.tradeCount++
	} else if symbol == e.perpSymbol {
		e.positionMutex.Lock()
		if e.perpPosition == nil {
			e.perpPosition = &DeltaPosition{
				Symbol:     symbol,
				Side:       PositionSidePerp,
				Size:       size,
				EntryPrice: entryPrice,
				MarkPrice:  entryPrice,
				Value:      entryPrice * size,
				Timestamp:  time.Now(),
			}
		} else {
			e.perpPosition.Size += size
			e.perpPosition.Value = e.perpPosition.Size * e.perpPosition.MarkPrice
		}
		e.positionMutex.Unlock()
		e.tradeCount++
	}
}

func (e *DeltaNeutralFundingPro) OnPositionClosed(symbol string, exitPrice, pnl float64) {
	logger.Info("DeltaNeutralFunding-Pro持仓已平仓",
		zap.String("symbol", symbol),
		zap.Float64("exit_price", exitPrice),
		zap.Float64("pnl", pnl),
	)

	e.RecordPnL(pnl)
	e.tradeCount++
}

func (e *DeltaNeutralFundingPro) OnPositionReduced(symbol string, exitPrice, pnl, remainingSize float64) {
	logger.Info("DeltaNeutralFunding-Pro持仓部分减仓",
		zap.String("symbol", symbol),
		zap.Float64("exit_price", exitPrice),
		zap.Float64("pnl", pnl),
		zap.Float64("remaining_size", remainingSize),
	)

	e.positionMutex.Lock()
	if symbol == e.spotSymbol && e.spotPosition != nil {
		e.spotPosition.Size = remainingSize
		e.spotPosition.MarkPrice = exitPrice
		e.spotPosition.Value = remainingSize * exitPrice
		if remainingSize <= 0 {
			e.spotPosition = nil
		}
	} else if symbol == e.perpSymbol && e.perpPosition != nil {
		e.perpPosition.Size = remainingSize
		e.perpPosition.MarkPrice = exitPrice
		e.perpPosition.Value = remainingSize * exitPrice
		if remainingSize <= 0 {
			e.perpPosition = nil
		}
	}
	e.positionMutex.Unlock()

	e.RecordPnL(pnl)
	e.tradeCount++
}

func (e *DeltaNeutralFundingPro) ConfirmRebalanceEntry(request *RebalanceRequest) (*RebalanceDecision, error) {
	if request == nil {
		return &RebalanceDecision{RejectReason: "nil_request"}, nil
	}
	if request.ShortfallAmount <= 0 {
		return &RebalanceDecision{RejectReason: "non_positive_shortfall"}, nil
	}

	e.priceMutex.RLock()
	spotPrice := e.spotPrice
	perpPrice := e.perpPrice
	e.priceMutex.RUnlock()

	if e.state != FundingStateActive {
		return &RebalanceDecision{
			RejectReason: "strategy_not_active",
		}, nil
	}
	if e.isInSettlementWindow() {
		return &RebalanceDecision{
			RejectReason: "settlement_window",
		}, nil
	}
	if e.checkCircuitBreaker() {
		return &RebalanceDecision{RejectReason: "circuit_breaker_active"}, nil
	}
	if !e.checkSmartFilter() {
		return &RebalanceDecision{RejectReason: "smart_filter_restricted"}, nil
	}
	if spotPrice <= 0 || perpPrice <= 0 {
		return &RebalanceDecision{RejectReason: "invalid_market_prices"}, nil
	}

	spotExposure, perpExposure := e.getExposureState()
	spotTopUpExposure, perpTopUpExposure := solveDeltaTopUp(spotExposure, perpExposure, request.ShortfallAmount, e.targetHedgeRatio)
	if spotTopUpExposure <= 0 && perpTopUpExposure <= 0 {
		return &RebalanceDecision{RejectReason: "unbalanced_shortfall_requires_reduction"}, nil
	}

	currentDrift := calculateExposureDrift(spotExposure, perpExposure)
	projectedSpotExposure := spotExposure + spotTopUpExposure
	projectedPerpExposure := perpExposure + perpTopUpExposure
	projectedDrift := calculateExposureDrift(projectedSpotExposure, projectedPerpExposure)
	projectedHedgeRatio := calculateHedgeRatio(projectedSpotExposure, projectedPerpExposure)
	if projectedDrift > currentDrift && currentDrift > 0 {
		return &RebalanceDecision{RejectReason: "projected_drift_worsens"}, nil
	}
	if math.Abs(projectedHedgeRatio-e.targetHedgeRatio) > e.hedgeRatioTolerance {
		return &RebalanceDecision{RejectReason: "target_hedge_ratio_not_met"}, nil
	}

	spotQuantity := spotTopUpExposure / spotPrice
	perpQuantity := perpTopUpExposure / perpPrice
	if spotQuantity <= 0 && perpQuantity <= 0 {
		return &RebalanceDecision{RejectReason: "invalid_topup_quantities"}, nil
	}

	plan := make([]RebalancePlanStep, 0, 2)
	if spotQuantity > 0 {
		plan = append(plan, RebalancePlanStep{
			Label:               "spot_leg",
			RecommendedPrice:    spotPrice,
			RecommendedQuantity: spotQuantity,
			Signal: &types.Signal{
				Symbol:    e.spotSymbol,
				Type:      types.SignalTypeBuy,
				Price:     spotPrice,
				Quantity:  spotQuantity,
				Timestamp: time.Now(),
				Metadata: map[string]interface{}{
					"paired_leg":         "spot",
					"target_hedge_ratio": e.targetHedgeRatio,
					"projected_drift":    projectedDrift,
				},
			},
		})
	}
	if perpQuantity > 0 {
		plan = append(plan, RebalancePlanStep{
			Label:               "perp_leg",
			RecommendedPrice:    perpPrice,
			RecommendedQuantity: perpQuantity,
			Signal: &types.Signal{
				Symbol:    e.perpSymbol,
				Type:      types.SignalTypeSell,
				Price:     perpPrice,
				Quantity:  perpQuantity,
				Timestamp: time.Now(),
				Metadata: map[string]interface{}{
					"paired_leg":         "perp",
					"target_hedge_ratio": e.targetHedgeRatio,
					"projected_drift":    projectedDrift,
				},
			},
		})
	}
	if len(plan) == 0 {
		return &RebalanceDecision{RejectReason: "invalid_topup_quantities"}, nil
	}

	return &RebalanceDecision{
		Approved: true,
		Plan:     plan,
	}, nil
}

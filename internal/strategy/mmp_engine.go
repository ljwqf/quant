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
	RingBufferSize       = 100
	MMPThreshold         = 0.15
	SpreadThreshold      = 0.0003
	MaxPositionValue     = 10000.0
	RiskPerTrade         = 0.005
	HardStopLossPercent  = 0.005
	TakeProfitRR         = 1.5
	SignalTTL            = 3 * time.Minute
	ConsecutiveLossLimit = 2
	LossPauseDuration    = 15 * time.Minute
	DailyLossLimit       = 0.06
	VolatilityPeriod     = 20
	ATRPeriod            = 14
)

type TickData struct {
	Price     float64
	Volume    float64
	Bid       float64
	Ask       float64
	Timestamp time.Time
}

type MMPState int

const (
	MMPStateIdle MMPState = iota
	MMPStateActive
	MMPStatePaused
	MMPStateStopped
)

type MMPEnginePro struct {
	name              string
	params            map[string]interface{}
	metrics           map[string]interface{}
	state             MMPState
	ringBuffer        []TickData
	bufferIndex       int
	bufferCount       int
	bufferMutex       sync.RWMutex
	tickPool          *sync.Pool
	oiData            *OIData
	oiMutex           sync.RWMutex
	quoteBySymbol     map[string]marketQuote
	quoteMutex        sync.RWMutex
	atr               float64
	volMean           float64
	consecutiveLosses int
	lastLossTime      time.Time
	dailyLoss         float64
	dailyLossReset    time.Time
	tradeCount        int
	winCount          int
	totalPnL          float64
	metricsMutex      sync.Mutex
	signalChan        chan *types.Signal
	executionChan     chan *ExecutionRequest
	stopChan          chan struct{}
	signalCallback    func(*types.Signal) // 信号回调函数
	nowFunc           func() time.Time
}

type OIData struct {
	Value     float64
	Timestamp time.Time
}

type marketQuote struct {
	Bid       float64
	Ask       float64
	Timestamp time.Time
}

type ExecutionRequest struct {
	Signal    *types.Signal
	Timestamp time.Time
}

func NewMMPEnginePro() *MMPEnginePro {
	engine := &MMPEnginePro{
		name:          "MMPEngine-Pro",
		params:        make(map[string]interface{}),
		metrics:       make(map[string]interface{}),
		state:         MMPStateIdle,
		ringBuffer:    make([]TickData, RingBufferSize),
		bufferIndex:   0,
		bufferCount:   0,
		quoteBySymbol: make(map[string]marketQuote),
		signalChan:    make(chan *types.Signal, 100),
		executionChan: make(chan *ExecutionRequest, 10),
		stopChan:      make(chan struct{}),
		nowFunc:       time.Now,
	}
	engine.dailyLossReset = engine.nowFunc()

	engine.tickPool = &sync.Pool{
		New: func() interface{} {
			return &TickData{}
		},
	}

	engine.initDefaultParams()
	go engine.runExecutionPipeline()

	return engine
}

func (e *MMPEnginePro) initDefaultParams() {
	e.params["mmp_threshold"] = MMPThreshold
	e.params["spread_threshold"] = SpreadThreshold
	e.params["max_position_value"] = MaxPositionValue
	e.params["risk_per_trade"] = RiskPerTrade
	e.params["hard_stop_loss"] = HardStopLossPercent
	e.params["take_profit_rr"] = TakeProfitRR
	e.params["signal_ttl"] = SignalTTL
	e.params["atr_period"] = ATRPeriod
	e.params["volatility_period"] = VolatilityPeriod
}

// SetSignalCallback 设置信号回调函数
func (e *MMPEnginePro) SetSignalCallback(callback func(*types.Signal)) {
	e.signalCallback = callback
}

func (e *MMPEnginePro) Name() string {
	return e.name
}

func (e *MMPEnginePro) Init(params map[string]interface{}) error {
	for k, v := range params {
		e.params[k] = v
	}
	e.state = MMPStateActive
	logger.Info("MMPEngine-Pro初始化完成", zap.Any("params", e.params))
	return nil
}

func (e *MMPEnginePro) OnTick(tick *types.Tick) (*types.Signal, error) {
	if e.state != MMPStateActive {
		return nil, nil
	}

	if !e.checkTimeWindow() {
		return nil, nil
	}

	if e.checkCircuitBreaker() {
		return nil, nil
	}

	tickData, ok := e.tickPool.Get().(*TickData)
	if !ok || tickData == nil {
		tickData = &TickData{}
	}
	tickData.Price = tick.Price
	tickData.Volume = tick.Size
	tickData.Bid = tick.Price
	tickData.Ask = tick.Price
	tickData.Timestamp = tick.Timestamp

	e.addToRingBuffer(*tickData)
	e.tickPool.Put(tickData)

	e.updateMetrics()

	signal := e.generateSignal(tick)
	if signal != nil {
		select {
		case e.signalChan <- signal:
		default:
			logger.Warn("信号通道已满，丢弃信号")
		}
	}

	return signal, nil
}

func (e *MMPEnginePro) OnBar(bar *types.Bar) (*types.Signal, error) {
	e.updateATR(bar)
	return nil, nil
}

func (e *MMPEnginePro) OnOrderBook(orderBook *types.OrderBook) (*types.Signal, error) {
	if orderBook == nil || len(orderBook.Bids) == 0 || len(orderBook.Asks) == 0 {
		return nil, nil
	}

	bid := orderBook.Bids[0].Price
	ask := orderBook.Asks[0].Price
	if bid <= 0 || ask <= 0 || ask <= bid {
		return nil, nil
	}

	e.quoteMutex.Lock()
	e.quoteBySymbol[normalizeMarketSymbol(orderBook.Symbol)] = marketQuote{
		Bid:       bid,
		Ask:       ask,
		Timestamp: time.Now(),
	}
	e.quoteMutex.Unlock()

	return nil, nil
}

func (e *MMPEnginePro) GetParams() map[string]interface{} {
	return e.params
}

func (e *MMPEnginePro) SetParams(params map[string]interface{}) {
	for k, v := range params {
		e.params[k] = v
	}
	// 热更新后立即重算指标，避免等待下一个 Bar
	e.updateMetrics()
}

func (e *MMPEnginePro) GetMetrics() map[string]interface{} {
	e.metricsMutex.Lock()
	defer e.metricsMutex.Unlock()

	e.metrics["state"] = int(e.state)
	e.metrics["buffer_count"] = e.bufferCount
	e.metrics["atr"] = e.atr
	e.metrics["vol_mean"] = e.volMean
	e.metrics["consecutive_losses"] = e.consecutiveLosses
	e.metrics["daily_loss"] = e.dailyLoss
	e.metrics["trade_count"] = e.tradeCount
	e.metrics["win_count"] = e.winCount
	e.metrics["win_rate"] = e.calculateWinRate()
	e.metrics["total_pnl"] = e.totalPnL

	return e.metrics
}

func (e *MMPEnginePro) addToRingBuffer(tick TickData) {
	e.bufferMutex.Lock()
	defer e.bufferMutex.Unlock()

	e.ringBuffer[e.bufferIndex] = tick
	e.bufferIndex = (e.bufferIndex + 1) % RingBufferSize
	if e.bufferCount < RingBufferSize {
		e.bufferCount++
	}
}

func (e *MMPEnginePro) getRecentTicks(n int) []TickData {
	e.bufferMutex.RLock()
	defer e.bufferMutex.RUnlock()

	if e.bufferCount < n {
		n = e.bufferCount
	}

	result := make([]TickData, n)
	startIdx := (e.bufferIndex - n + RingBufferSize) % RingBufferSize

	for i := 0; i < n; i++ {
		result[i] = e.ringBuffer[(startIdx+i)%RingBufferSize]
	}

	return result
}

func (e *MMPEnginePro) checkTimeWindow() bool {
	now := time.Now().UTC()
	hour := now.Hour()

	if hour >= 0 && hour < 1 {
		return false
	}

	return true
}

func (e *MMPEnginePro) checkCircuitBreaker() bool {
	e.resetDailyLossIfNeeded(e.nowFunc())

	if e.consecutiveLosses >= ConsecutiveLossLimit {
		if e.nowFunc().Sub(e.lastLossTime) < LossPauseDuration {
			return true
		}
		e.consecutiveLosses = 0
	}

	if e.dailyLoss >= DailyLossLimit {
		return true
	}

	return false
}

func (e *MMPEnginePro) updateATR(bar *types.Bar) {
	ticks := e.getRecentTicks(ATRPeriod)
	if len(ticks) < ATRPeriod {
		return
	}

	var trSum float64
	for i := 1; i < len(ticks); i++ {
		high := ticks[i].Price
		low := ticks[i].Price
		prevClose := ticks[i-1].Price

		tr := math.Max(high-low, math.Max(math.Abs(high-prevClose), math.Abs(low-prevClose)))
		trSum += tr
	}

	e.atr = trSum / float64(ATRPeriod-1)
}

func (e *MMPEnginePro) updateMetrics() {
	ticks := e.getRecentTicks(VolatilityPeriod)
	if len(ticks) < VolatilityPeriod {
		return
	}

	var volSum float64
	for _, tick := range ticks {
		volSum += tick.Volume
	}
	e.volMean = volSum / float64(len(ticks))
}

func (e *MMPEnginePro) calculateSpread(tick *types.Tick) float64 {
	if tick == nil {
		return 1.0
	}

	e.quoteMutex.RLock()
	quote, ok := e.quoteBySymbol[normalizeMarketSymbol(tick.Symbol)]
	e.quoteMutex.RUnlock()
	if !ok {
		return 1.0
	}

	if time.Since(quote.Timestamp) > 30*time.Second {
		return 1.0
	}

	mid := (quote.Bid + quote.Ask) / 2
	if mid <= 0 {
		return 1.0
	}

	return (quote.Ask - quote.Bid) / mid
}

func (e *MMPEnginePro) getPriceDelta() float64 {
	ticks := e.getRecentTicks(2)
	if len(ticks) < 2 {
		return 0
	}

	return ticks[len(ticks)-1].Price - ticks[len(ticks)-2].Price
}

func (e *MMPEnginePro) calculateMMP(tick *types.Tick) float64 {
	ticks := e.getRecentTicks(20)
	if len(ticks) < 2 || e.atr <= 0 {
		return 0
	}

	priceDelta := tick.Price - ticks[len(ticks)-2].Price
	absPriceDelta := math.Abs(priceDelta)

	var volCurrent float64
	for _, t := range ticks {
		volCurrent += t.Volume
	}
	volCurrent = volCurrent / float64(len(ticks))

	volFactor := 1.0
	if e.volMean > 0 {
		volFactor = volCurrent / e.volMean
	}

	atrNormalized := absPriceDelta / e.atr

	bodyRange := 1.0

	mmp := atrNormalized * (1 - volFactor) * bodyRange

	return mmp
}

func (e *MMPEnginePro) checkOIFilter() bool {
	e.oiMutex.RLock()
	defer e.oiMutex.RUnlock()

	if e.oiData == nil {
		return true
	}

	if time.Since(e.oiData.Timestamp) > 6*time.Minute {
		logger.Warn("OI数据过期，跳过信号")
		return false
	}

	return true
}

func (e *MMPEnginePro) generateSignal(tick *types.Tick) *types.Signal {
	spread := e.calculateSpread(tick)
	spreadThreshold := e.getFloatParam("spread_threshold", SpreadThreshold)
	if spread > spreadThreshold {
		return nil
	}

	if !e.checkOIFilter() {
		return nil
	}

	mmp := e.calculateMMP(tick)
	mmpThreshold := e.getFloatParam("mmp_threshold", MMPThreshold)

	if mmp < mmpThreshold {
		return nil
	}

	ticks := e.getRecentTicks(20)
	var volCurrent float64
	for _, t := range ticks {
		volCurrent += t.Volume
	}
	volCurrent = volCurrent / float64(len(ticks))

	volFactor := volCurrent / e.volMean
	if volFactor >= 1.0 {
		return nil
	}

	priceDelta := e.getPriceDelta()
	signalType := types.SignalTypeBuy
	stopLoss := tick.Price * (1 - HardStopLossPercent)
	takeProfit := tick.Price + (tick.Price-stopLoss)*TakeProfitRR
	if priceDelta < 0 {
		signalType = types.SignalTypeSell
		stopLoss = tick.Price * (1 + HardStopLossPercent)
		takeProfit = tick.Price - (stopLoss-tick.Price)*TakeProfitRR
	}

	signal := &types.Signal{
		Strategy:   e.name,
		Symbol:     tick.Symbol,
		Type:       signalType,
		Price:      tick.Price,
		Confidence: mmp / mmpThreshold,
		Timestamp:  time.Now(),
		Metadata: map[string]interface{}{
			"mmp":         mmp,
			"spread":      spread,
			"price_delta": priceDelta,
			"vol_factor":  volFactor,
			"stop_loss":   stopLoss,
			"take_profit": takeProfit,
			"atr":         e.atr,
			"signal_ttl":  SignalTTL,
		},
	}

	logger.Info("MMPEngine-Pro生成信号",
		zap.String("symbol", tick.Symbol),
		zap.String("type", string(signalType)),
		zap.Float64("price", tick.Price),
		zap.Float64("mmp", mmp),
		zap.Float64("spread", spread),
		zap.Float64("stop_loss", stopLoss),
		zap.Float64("take_profit", takeProfit),
	)

	return signal
}

func (e *MMPEnginePro) getFloatParam(key string, defaultValue float64) float64 {
	if v, ok := e.params[key]; ok {
		switch val := v.(type) {
		case float64:
			return val
		case float32:
			return float64(val)
		case int:
			return float64(val)
		}
	}
	return defaultValue
}

func (e *MMPEnginePro) calculateWinRate() float64 {
	if e.tradeCount == 0 {
		return 0
	}
	return float64(e.winCount) / float64(e.tradeCount)
}

func (e *MMPEnginePro) UpdateOIData(value float64) {
	e.oiMutex.Lock()
	defer e.oiMutex.Unlock()

	e.oiData = &OIData{
		Value:     value,
		Timestamp: time.Now(),
	}
}

func (e *MMPEnginePro) RecordTrade(pnl float64) {
	e.metricsMutex.Lock()
	defer e.metricsMutex.Unlock()
	e.resetDailyLossIfNeeded(e.nowFunc())

	e.tradeCount++
	e.totalPnL += pnl

	if pnl > 0 {
		e.winCount++
		e.consecutiveLosses = 0
	} else {
		e.consecutiveLosses++
		e.lastLossTime = e.nowFunc()
	}

	e.dailyLoss += -pnl
	if e.dailyLoss < 0 {
		e.dailyLoss = 0
	}
}

func (e *MMPEnginePro) resetDailyLossIfNeeded(now time.Time) {
	if e.dailyLossReset.IsZero() || !isSameDay(now, e.dailyLossReset) {
		e.dailyLoss = 0
		e.dailyLossReset = now
	}
}

func (e *MMPEnginePro) GetState() MMPState {
	return e.state
}

func (e *MMPEnginePro) Pause() {
	e.state = MMPStatePaused
	logger.Info("MMPEngine-Pro已暂停")
}

func (e *MMPEnginePro) Resume() {
	e.state = MMPStateActive
	logger.Info("MMPEngine-Pro已恢复")
}

func (e *MMPEnginePro) Stop() {
	e.state = MMPStateStopped
	close(e.stopChan)
	logger.Info("MMPEngine-Pro已停止")
}

func (e *MMPEnginePro) runExecutionPipeline() {
	for {
		select {
		case <-e.stopChan:
			return
		case req := <-e.executionChan:
			e.executeSignal(req)
		}
	}
}

func (e *MMPEnginePro) executeSignal(req *ExecutionRequest) {
	if time.Since(req.Timestamp) > SignalTTL {
		logger.Warn("信号已过期，放弃执行",
			zap.String("symbol", req.Signal.Symbol),
			zap.Duration("age", time.Since(req.Timestamp)),
		)
		return
	}

	logger.Info("执行信号",
		zap.String("symbol", req.Signal.Symbol),
		zap.String("type", string(req.Signal.Type)),
		zap.Float64("price", req.Signal.Price),
	)
}

func (e *MMPEnginePro) CalculatePositionSize(accountValue, entryPrice, stopLoss float64) float64 {
	riskAmount := accountValue * RiskPerTrade
	riskPerUnit := math.Abs(entryPrice - stopLoss)
	if riskPerUnit <= 0 {
		return 0
	}

	size := riskAmount / riskPerUnit

	maxPositionValue := e.getFloatParam("max_position_value", MaxPositionValue)
	maxSize := maxPositionValue / entryPrice
	if size > maxSize {
		size = maxSize
	}

	return size
}

func (e *MMPEnginePro) GetSignalChannel() <-chan *types.Signal {
	return e.signalChan
}

func (e *MMPEnginePro) SubmitForExecution(signal *types.Signal) {
	req := &ExecutionRequest{
		Signal:    signal,
		Timestamp: time.Now(),
	}

	select {
	case e.executionChan <- req:
	default:
		logger.Warn("执行队列已满，丢弃信号")
	}
}

func (e *MMPEnginePro) OnPositionFilled(symbol string, side types.OrderSide, entryPrice, size float64) {
	logger.Info("MMPEngine-Pro 持仓已填充",
		zap.String("symbol", symbol),
		zap.String("side", string(side)),
		zap.Float64("entry_price", entryPrice),
		zap.Float64("size", size),
	)
}

func (e *MMPEnginePro) OnPositionReduced(symbol string, exitPrice, pnl, remainingSize float64) {
	logger.Info("MMPEngine-Pro 持仓部分减仓",
		zap.String("symbol", symbol),
		zap.Float64("exit_price", exitPrice),
		zap.Float64("pnl", pnl),
		zap.Float64("remaining_size", remainingSize),
	)
	e.RecordTrade(pnl)
}

func (e *MMPEnginePro) OnPositionClosed(symbol string, exitPrice, pnl float64) {
	logger.Info("MMPEngine-Pro 持仓已平仓",
		zap.String("symbol", symbol),
		zap.Float64("exit_price", exitPrice),
		zap.Float64("pnl", pnl),
	)
	e.RecordTrade(pnl)
}

func (e *MMPEnginePro) ConfirmRebalanceEntry(request *RebalanceRequest) (*RebalanceDecision, error) {
	return &RebalanceDecision{RejectReason: "rebalance_entry_unsupported"}, nil
}

// isSameDay 判断两个时间是否在同一天
func isSameDay(t1, t2 time.Time) bool {
	return t1.Year() == t2.Year() && t1.YearDay() == t2.YearDay()
}

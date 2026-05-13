package strategy

import (
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/ljwqf/quant/pkg/logger"
	"github.com/ljwqf/quant/pkg/types"
	"go.uber.org/zap"
)

const (
	SupertrendPeriod     = 10
	SupertrendMultiplier = 3.0
	MACDFastPeriod       = 12
	MACDSlowPeriod       = 26
	MACDSignalPeriod     = 9
	MaxHoldingTime       = 2 * time.Minute
	NeedleDistance       = 0.003
	TakeProfitPercent    = 0.005
	StopLossPercent      = 0.008
)

type DivergenceType int

const (
	DivergenceNone DivergenceType = iota
	DivergenceBullish
	DivergenceBearish
)

func (d DivergenceType) String() string {
	switch d {
	case DivergenceBullish:
		return "Bullish"
	case DivergenceBearish:
		return "Bearish"
	default:
		return "None"
	}
}

type NeedleState int

const (
	NeedleStateIdle NeedleState = iota
	NeedleStateWaitingEntry
	NeedleStateInPosition
	NeedleStateExiting
)

type Supertrend struct {
	Value     float64
	Direction int
	Changed   bool
}

type MACD struct {
	DIF       float64
	DEA       float64
	Histogram float64
}

type NeedlePosition struct {
	Symbol     string
	Side       types.OrderSide
	EntryPrice float64
	Size       float64
	OpenTime   time.Time
	TakeProfit float64
	StopLoss   float64
	Divergence DivergenceType
}

type NeedleSignal struct {
	Symbol     string
	Side       types.OrderSide
	EntryPrice float64
	Divergence DivergenceType
	Supertrend float64
	MACD       *MACD
	Timestamp  time.Time
}

type NeedleStrategy struct {
	name           string
	params         map[string]interface{}
	metrics        map[string]interface{}
	state          NeedleState
	position       *NeedlePosition
	positionMutex  sync.RWMutex
	barHistory     []*types.Bar
	barMutex       sync.RWMutex
	supertrend     *Supertrend
	macd           *MACD
	macdHistory    []*MACD
	priceHighs     []float64
	priceLows      []float64
	macdHighs      []float64
	macdLows       []float64
	tradeCount     int
	winCount       int
	totalPnL       float64
	metricsMutex   sync.Mutex
	signalChan     chan *NeedleSignal
	stopChan       chan struct{}
	signalCallback func(*types.Signal) // 信号回调函数
	smartFilter    *SmartFilter
}

func NewNeedleStrategy() *NeedleStrategy {
	strategy := &NeedleStrategy{
		name:        "NeedleStrategy",
		params:      make(map[string]interface{}),
		metrics:     make(map[string]interface{}),
		state:       NeedleStateIdle,
		barHistory:  make([]*types.Bar, 0, 200), // 预分配容量
		macdHistory: make([]*MACD, 0, 100),      // 预分配容量
		priceHighs:  make([]float64, 0, 50),     // 预分配容量
		priceLows:   make([]float64, 0, 50),     // 预分配容量
		macdHighs:   make([]float64, 0, 50),     // 预分配容量
		macdLows:    make([]float64, 0, 50),     // 预分配容量
		signalChan:  make(chan *NeedleSignal, 100),
		stopChan:    make(chan struct{}),
		smartFilter: NewSmartFilter(),
	}

	// 启动持仓超时检查 goroutine
	go strategy.runTimeoutChecker()

	return strategy
}

// SetSignalCallback 设置信号回调函数
func (s *NeedleStrategy) SetSignalCallback(callback func(*types.Signal)) {
	s.signalCallback = callback
}

func (s *NeedleStrategy) SetSmartFilter(filter *SmartFilter) {
	if filter == nil {
		return
	}

	s.smartFilter = filter
}

func (s *NeedleStrategy) UpdateOnChainData(netflow, sopr, mvrv float64) {
	if s.smartFilter == nil {
		s.smartFilter = NewSmartFilter()
	}

	s.smartFilter.UpdateOnChainData(netflow, sopr, mvrv)
}

// runTimeoutChecker 定期检查持仓是否超时
func (s *NeedleStrategy) runTimeoutChecker() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// 超时检查现在通过OnTick处理，这里只做状态清理
			s.positionMutex.RLock()
			position := s.position
			s.positionMutex.RUnlock()

			if position != nil {
				holdingTime := time.Since(position.OpenTime)
				if holdingTime >= MaxHoldingTime {
					logger.Info("NeedleStrategy 持仓超时，将在下次OnTick时平仓",
						zap.String("symbol", position.Symbol),
						zap.Duration("holding_time", holdingTime),
						zap.Float64("entry_price", position.EntryPrice),
					)
				}
			}
		case <-s.stopChan:
			return
		}
	}
}

// checkPositionTimeout 检查持仓是否超时
func (s *NeedleStrategy) checkPositionTimeout() *types.Signal {
	s.positionMutex.RLock()
	position := s.position
	s.positionMutex.RUnlock()

	if position == nil {
		return nil
	}

	holdingTime := time.Since(position.OpenTime)
	if holdingTime >= MaxHoldingTime {
		logger.Info("NeedleStrategy 持仓超时，需要平仓",
			zap.String("symbol", position.Symbol),
			zap.Duration("holding_time", holdingTime),
			zap.Float64("entry_price", position.EntryPrice),
		)
		return s.createExitSignal(position)
	}
	return nil
}

func (s *NeedleStrategy) Name() string {
	return s.name
}

func (s *NeedleStrategy) Init(params map[string]interface{}) error {
	for k, v := range params {
		s.params[k] = v
	}
	s.state = NeedleStateIdle
	logger.Info("NeedleStrategy初始化完成", zap.Any("params", s.params))
	return nil
}

func (s *NeedleStrategy) OnTick(tick *types.Tick) (*types.Signal, error) {
	s.positionMutex.RLock()
	position := s.position
	s.positionMutex.RUnlock()

	if position == nil {
		return nil, nil
	}

	// 检查止盈止损
	exitSignal := s.checkTakeProfitStopLoss(tick.Price)
	if exitSignal != nil {
		return exitSignal, nil
	}

	// 检查超时
	holdingTime := time.Since(position.OpenTime)
	if holdingTime >= MaxHoldingTime {
		logger.Info("插针持仓超时，强制平仓",
			zap.String("symbol", position.Symbol),
			zap.Duration("holding_time", holdingTime),
		)
		return s.createExitSignal(position), nil
	}

	return nil, nil
}

// checkTakeProfitStopLoss 检查止盈止损
func (s *NeedleStrategy) checkTakeProfitStopLoss(currentPrice float64) *types.Signal {
	s.positionMutex.RLock()
	position := s.position
	s.positionMutex.RUnlock()

	if position == nil {
		return nil
	}

	var pnl float64
	var shouldExit bool
	exitReason := ""

	if position.Side == types.OrderSideBuy {
		pnl = (currentPrice - position.EntryPrice) / position.EntryPrice
		if currentPrice >= position.TakeProfit {
			shouldExit = true
			exitReason = "take_profit"
		} else if currentPrice <= position.StopLoss {
			shouldExit = true
			exitReason = "stop_loss"
		}
	} else {
		pnl = (position.EntryPrice - currentPrice) / position.EntryPrice
		if currentPrice <= position.TakeProfit {
			shouldExit = true
			exitReason = "take_profit"
		} else if currentPrice >= position.StopLoss {
			shouldExit = true
			exitReason = "stop_loss"
		}
	}

	if shouldExit {
		s.RecordTrade(pnl)

		logger.Info("NeedleStrategy 触发平仓",
			zap.String("symbol", position.Symbol),
			zap.String("exit_reason", exitReason),
			zap.Float64("pnl", pnl),
			zap.Float64("current_price", currentPrice),
			zap.Float64("entry_price", position.EntryPrice),
		)

		return &types.Signal{
			Strategy:   s.name,
			Symbol:     position.Symbol,
			Type:       types.SignalTypeExit,
			Price:      currentPrice,
			Confidence: 1.0,
			Timestamp:  time.Now(),
			Metadata: map[string]interface{}{
				"exit_reason": exitReason,
				"pnl":         pnl,
			},
		}
	}

	return nil
}

// IsPositionTimeout 检查持仓是否超时
func (s *NeedleStrategy) IsPositionTimeout() bool {
	s.positionMutex.RLock()
	position := s.position
	s.positionMutex.RUnlock()

	if position == nil {
		return false
	}

	return time.Since(position.OpenTime) >= MaxHoldingTime
}

// GetHoldingTimeRemaining 获取持仓剩余时间
func (s *NeedleStrategy) GetHoldingTimeRemaining() time.Duration {
	s.positionMutex.RLock()
	position := s.position
	s.positionMutex.RUnlock()

	if position == nil {
		return 0
	}

	elapsed := time.Since(position.OpenTime)
	if elapsed >= MaxHoldingTime {
		return 0
	}
	return MaxHoldingTime - elapsed
}

func (s *NeedleStrategy) OnBar(bar *types.Bar) (*types.Signal, error) {
	s.barMutex.Lock()
	s.barHistory = append(s.barHistory, bar)
	if len(s.barHistory) > 200 {
		s.barHistory = s.barHistory[len(s.barHistory)-200:]
	}
	s.barMutex.Unlock()

	s.calculateSupertrend()
	s.calculateMACD()

	divergence := s.detectDivergence()
	if divergence == DivergenceNone {
		return nil, nil
	}

	s.positionMutex.RLock()
	hasPosition := s.position != nil
	s.positionMutex.RUnlock()

	if hasPosition {
		return nil, nil
	}

	signal := s.generateSignal(bar, divergence)
	if signal != nil {
		s.state = NeedleStateWaitingEntry
	}

	return signal, nil
}

func (s *NeedleStrategy) OnOrderBook(orderBook *types.OrderBook) (*types.Signal, error) {
	return nil, nil
}

func (s *NeedleStrategy) GetParams() map[string]interface{} {
	return s.params
}

func (s *NeedleStrategy) SetParams(params map[string]interface{}) {
	for k, v := range params {
		s.params[k] = v
	}
	// 热更新：重新计算超级趋势指标和MACD
	s.calculateSupertrend()
	s.calculateMACD()
}

func (s *NeedleStrategy) GetMetrics() map[string]interface{} {
	s.metricsMutex.Lock()
	defer s.metricsMutex.Unlock()

	s.metrics["state"] = int(s.state)
	s.metrics["trade_count"] = s.tradeCount
	s.metrics["win_count"] = s.winCount
	s.metrics["win_rate"] = s.calculateWinRate()
	s.metrics["total_pnl"] = s.totalPnL

	if s.supertrend != nil {
		s.metrics["supertrend_value"] = s.supertrend.Value
		s.metrics["supertrend_direction"] = s.supertrend.Direction
	}

	if s.macd != nil {
		s.metrics["macd_dif"] = s.macd.DIF
		s.metrics["macd_dea"] = s.macd.DEA
		s.metrics["macd_histogram"] = s.macd.Histogram
	}

	// 添加SmartFilter指标
	if s.smartFilter != nil {
		marketState := s.smartFilter.GetMarketState()
		s.metrics["market_state"] = marketState.State.String()
		s.metrics["can_long"] = marketState.CanLong
		s.metrics["can_short"] = marketState.CanShort
		s.metrics["is_data_valid"] = s.smartFilter.IsDataValid()
	}

	return s.metrics
}

func (s *NeedleStrategy) calculateSupertrend() {
	s.barMutex.RLock()
	bars := s.barHistory
	s.barMutex.RUnlock()

	if len(bars) < SupertrendPeriod+1 {
		return
	}

	period := s.getIntParam("supertrend_period", SupertrendPeriod)
	multiplier := s.getFloatParam("supertrend_multiplier", SupertrendMultiplier)

	hl2 := make([]float64, len(bars))
	for i, bar := range bars {
		hl2[i] = (bar.High + bar.Low) / 2
	}

	tr := make([]float64, len(bars))
	for i := 1; i < len(bars); i++ {
		high := bars[i].High
		low := bars[i].Low
		prevClose := bars[i-1].Close

		tr[i] = math.Max(high-low, math.Max(math.Abs(high-prevClose), math.Abs(low-prevClose)))
	}

	atr := make([]float64, len(bars))
	for i := period; i < len(bars); i++ {
		sum := 0.0
		for j := 0; j < period; j++ {
			sum += tr[i-j]
		}
		atr[i] = sum / float64(period)
	}

	upperBand := hl2[len(bars)-1] + multiplier*atr[len(bars)-1]
	lowerBand := hl2[len(bars)-1] - multiplier*atr[len(bars)-1]

	prevSupertrend := s.supertrend
	var newSupertrend *Supertrend

	if prevSupertrend == nil {
		newSupertrend = &Supertrend{
			Value:     lowerBand,
			Direction: 1,
			Changed:   false,
		}
	} else {
		if bars[len(bars)-1].Close > prevSupertrend.Value {
			newSupertrend = &Supertrend{
				Value:     math.Max(lowerBand, prevSupertrend.Value),
				Direction: 1,
				Changed:   prevSupertrend.Direction != 1,
			}
		} else {
			newSupertrend = &Supertrend{
				Value:     math.Min(upperBand, prevSupertrend.Value),
				Direction: -1,
				Changed:   prevSupertrend.Direction != -1,
			}
		}
	}

	s.supertrend = newSupertrend
}

func (s *NeedleStrategy) calculateMACD() {
	s.barMutex.RLock()
	bars := s.barHistory
	s.barMutex.RUnlock()

	if len(bars) < MACDSlowPeriod+MACDSignalPeriod {
		return
	}

	closes := make([]float64, len(bars))
	for i, bar := range bars {
		closes[i] = bar.Close
	}

	fastPeriod := s.getIntParam("macd_fast_period", MACDFastPeriod)
	slowPeriod := s.getIntParam("macd_slow_period", MACDSlowPeriod)
	signalPeriod := s.getIntParam("macd_signal_period", MACDSignalPeriod)

	emaFast := calculateEMA(closes, fastPeriod)
	emaSlow := calculateEMA(closes, slowPeriod)

	dif := emaFast[len(emaFast)-1] - emaSlow[len(emaSlow)-1]

	difHistory := make([]float64, 0)
	if len(s.macdHistory) > 0 {
		for _, m := range s.macdHistory {
			difHistory = append(difHistory, m.DIF)
		}
	}
	difHistory = append(difHistory, dif)

	dea := calculateEMA(difHistory, signalPeriod)
	if len(dea) == 0 {
		dea = []float64{dif}
	}

	histogram := 2 * (dif - dea[len(dea)-1])

	s.macd = &MACD{
		DIF:       dif,
		DEA:       dea[len(dea)-1],
		Histogram: histogram,
	}

	s.macdHistory = append(s.macdHistory, s.macd)
	if len(s.macdHistory) > 100 {
		s.macdHistory = s.macdHistory[len(s.macdHistory)-100:]
	}

	s.updateExtremes(bars[len(bars)-1], s.macd)
}

func (s *NeedleStrategy) updateExtremes(bar *types.Bar, macd *MACD) {
	s.priceHighs = append(s.priceHighs, bar.High)
	s.priceLows = append(s.priceLows, bar.Low)
	s.macdHighs = append(s.macdHighs, macd.Histogram)
	s.macdLows = append(s.macdLows, macd.Histogram)

	maxLen := 50
	if len(s.priceHighs) > maxLen {
		s.priceHighs = s.priceHighs[len(s.priceHighs)-maxLen:]
		s.priceLows = s.priceLows[len(s.priceLows)-maxLen:]
		s.macdHighs = s.macdHighs[len(s.macdHighs)-maxLen:]
		s.macdLows = s.macdLows[len(s.macdLows)-maxLen:]
	}
}

func (s *NeedleStrategy) detectDivergence() DivergenceType {
	if len(s.priceHighs) < 5 || len(s.macdHighs) < 5 {
		return DivergenceNone
	}

	priceHigh1, priceHigh2, idx1, idx2 := findRecentHighs(s.priceHighs)
	if priceHigh1 > 0 && priceHigh2 > 0 {
		macdHigh1 := s.macdHighs[idx1]
		macdHigh2 := s.macdHighs[idx2]

		if priceHigh2 > priceHigh1 && macdHigh2 < macdHigh1 {
			if s.supertrend != nil && s.supertrend.Direction == -1 {
				logger.Info("检测到顶背离",
					zap.Float64("price_high1", priceHigh1),
					zap.Float64("price_high2", priceHigh2),
					zap.Float64("macd_high1", macdHigh1),
					zap.Float64("macd_high2", macdHigh2),
				)
				return DivergenceBearish
			}
		}
	}

	priceLow1, priceLow2, idx1, idx2 := findRecentLows(s.priceLows)
	if priceLow1 > 0 && priceLow2 > 0 {
		macdLow1 := s.macdLows[idx1]
		macdLow2 := s.macdLows[idx2]

		if priceLow2 < priceLow1 && macdLow2 > macdLow1 {
			if s.supertrend != nil && s.supertrend.Direction == 1 {
				logger.Info("检测到底背离",
					zap.Float64("price_low1", priceLow1),
					zap.Float64("price_low2", priceLow2),
					zap.Float64("macd_low1", macdLow1),
					zap.Float64("macd_low2", macdLow2),
				)
				return DivergenceBullish
			}
		}
	}

	return DivergenceNone
}

func findRecentHighs(highs []float64) (float64, float64, int, int) {
	if len(highs) < 5 {
		return 0, 0, 0, 0
	}

	n := len(highs)
	max1, idx1 := 0.0, 0
	max2, max2Idx := 0.0, 0

	for i := n - 5; i < n; i++ {
		if highs[i] > max1 {
			max1 = highs[i]
			idx1 = i
		}
	}

	for i := n - 5; i < n; i++ {
		if i != idx1 && highs[i] > max2 {
			max2 = highs[i]
			max2Idx = i
		}
	}

	if idx1 > max2Idx {
		return max2, max1, max2Idx, idx1
	}
	return max1, max2, idx1, max2Idx
}

func findRecentLows(lows []float64) (float64, float64, int, int) {
	if len(lows) < 5 {
		return 0, 0, 0, 0
	}

	n := len(lows)
	min1, idx1 := math.MaxFloat64, 0
	min2, min2Idx := math.MaxFloat64, 0

	for i := n - 5; i < n; i++ {
		if lows[i] < min1 {
			min1 = lows[i]
			idx1 = i
		}
	}

	for i := n - 5; i < n; i++ {
		if i != idx1 && lows[i] < min2 {
			min2 = lows[i]
			min2Idx = i
		}
	}

	if idx1 > min2Idx {
		return min2, min1, min2Idx, idx1
	}
	return min1, min2, idx1, min2Idx
}

func (s *NeedleStrategy) generateSignal(bar *types.Bar, divergence DivergenceType) *types.Signal {
	needleDistance := s.getFloatParam("needle_distance", NeedleDistance)
	takeProfitPercent := s.getFloatParam("take_profit_percent", TakeProfitPercent)
	stopLossPercent := s.getFloatParam("stop_loss_percent", StopLossPercent)

	var signalType types.SignalType
	var entryPrice, takeProfit, stopLoss float64
	var signalDirection string

	if divergence == DivergenceBearish {
		signalType = types.SignalTypeSell
		signalDirection = "short"
		entryPrice = bar.Close * (1 + needleDistance)
		takeProfit = entryPrice * (1 - takeProfitPercent)
		stopLoss = entryPrice * (1 + stopLossPercent)
	} else if divergence == DivergenceBullish {
		signalType = types.SignalTypeBuy
		signalDirection = "long"
		entryPrice = bar.Close * (1 - needleDistance)
		takeProfit = entryPrice * (1 + takeProfitPercent)
		stopLoss = entryPrice * (1 - stopLossPercent)
	} else {
		return nil
	}

	// 使用SmartFilter过滤信号
	if s.smartFilter != nil && !s.smartFilter.FilterSignal(signalDirection) {
		logger.Info("SmartFilter过滤信号",
			zap.String("symbol", bar.Symbol),
			zap.String("direction", signalDirection),
			zap.String("divergence", divergence.String()),
		)
		return nil
	}

	marketState := "Unknown"
	if s.smartFilter != nil {
		marketState = s.smartFilter.GetMarketState().State.String()
	}

	metadata := map[string]interface{}{
		"divergence":   divergence.String(),
		"take_profit":  takeProfit,
		"stop_loss":    stopLoss,
		"market_state": marketState,
	}

	signal := &types.Signal{
		Strategy:   s.name,
		Symbol:     bar.Symbol,
		Type:       signalType,
		Price:      entryPrice,
		Confidence: 0.7,
		Timestamp:  time.Now(),
		Metadata:   metadata,
	}

	// 安全地添加指标数据
	if s.supertrend != nil {
		metadata["supertrend"] = s.supertrend.Value
	}
	if s.macd != nil {
		metadata["macd_dif"] = s.macd.DIF
		metadata["macd_dea"] = s.macd.DEA
		metadata["macd_histogram"] = s.macd.Histogram
	}

	logger.Info("NeedleStrategy生成信号",
		zap.String("symbol", bar.Symbol),
		zap.String("type", string(signalType)),
		zap.String("divergence", divergence.String()),
		zap.Float64("entry_price", entryPrice),
		zap.Float64("take_profit", takeProfit),
		zap.Float64("stop_loss", stopLoss),
		zap.String("market_state", marketState),
	)

	return signal
}

func (s *NeedleStrategy) createExitSignal(position *NeedlePosition) *types.Signal {
	var exitType types.SignalType
	if position.Side == types.OrderSideBuy {
		exitType = types.SignalTypeSell
	} else {
		exitType = types.SignalTypeBuy
	}

	return &types.Signal{
		Strategy:   s.name,
		Symbol:     position.Symbol,
		Type:       exitType,
		Price:      0,
		Confidence: 1.0,
		Timestamp:  time.Now(),
		Metadata: map[string]interface{}{
			"exit_reason":  "timeout",
			"holding_time": time.Since(position.OpenTime).String(),
		},
	}
}

func (s *NeedleStrategy) SetPosition(symbol string, side types.OrderSide, entryPrice, size float64) {
	s.positionMutex.Lock()
	defer s.positionMutex.Unlock()

	takeProfitPercent := s.getFloatParam("take_profit_percent", TakeProfitPercent)
	stopLossPercent := s.getFloatParam("stop_loss_percent", StopLossPercent)

	var takeProfit, stopLoss float64
	if side == types.OrderSideBuy {
		takeProfit = entryPrice * (1 + takeProfitPercent)
		stopLoss = entryPrice * (1 - stopLossPercent)
	} else {
		takeProfit = entryPrice * (1 - takeProfitPercent)
		stopLoss = entryPrice * (1 + stopLossPercent)
	}

	s.position = &NeedlePosition{
		Symbol:     symbol,
		Side:       side,
		EntryPrice: entryPrice,
		Size:       size,
		OpenTime:   time.Now(),
		TakeProfit: takeProfit,
		StopLoss:   stopLoss,
	}

	s.state = NeedleStateInPosition

	logger.Info("NeedleStrategy设置持仓",
		zap.String("symbol", symbol),
		zap.String("side", string(side)),
		zap.Float64("entry_price", entryPrice),
		zap.Float64("size", size),
		zap.Float64("take_profit", takeProfit),
		zap.Float64("stop_loss", stopLoss),
	)
}

func (s *NeedleStrategy) ClearPosition() {
	s.positionMutex.Lock()
	defer s.positionMutex.Unlock()

	s.position = nil
	s.state = NeedleStateIdle
}

func (s *NeedleStrategy) GetPosition() *NeedlePosition {
	s.positionMutex.RLock()
	defer s.positionMutex.RUnlock()
	return s.position
}

func (s *NeedleStrategy) CheckExit(currentPrice float64) *types.Signal {
	s.positionMutex.RLock()
	position := s.position
	s.positionMutex.RUnlock()

	if position == nil {
		return nil
	}

	// 首先检查止盈止损
	exitSignal := s.checkTakeProfitStopLoss(currentPrice)
	if exitSignal != nil {
		return exitSignal
	}

	// 然后检查超时
	if time.Since(position.OpenTime) >= MaxHoldingTime {
		logger.Info("NeedleStrategy 持仓超时，需要平仓",
			zap.String("symbol", position.Symbol),
			zap.Duration("holding_time", time.Since(position.OpenTime)),
			zap.Float64("entry_price", position.EntryPrice),
		)
		return s.createExitSignal(position)
	}

	return nil
}

func (s *NeedleStrategy) RecordTrade(pnl float64) {
	s.metricsMutex.Lock()
	defer s.metricsMutex.Unlock()

	s.tradeCount++
	s.totalPnL += pnl
	if pnl > 0 {
		s.winCount++
	}
}

func (s *NeedleStrategy) calculateWinRate() float64 {
	if s.tradeCount == 0 {
		return 0
	}
	return float64(s.winCount) / float64(s.tradeCount)
}

func (s *NeedleStrategy) getIntParam(key string, defaultValue int) int {
	if v, ok := s.params[key]; ok {
		switch val := v.(type) {
		case int:
			return val
		case float64:
			return int(val)
		case float32:
			return int(val)
		default:
			logger.Warn("参数类型错误，使用默认值",
				zap.String("key", key),
				zap.String("expected", "int/float"),
				zap.String("got", fmt.Sprintf("%T", val)),
			)
			return defaultValue
		}
	}
	return defaultValue
}

func (s *NeedleStrategy) getFloatParam(key string, defaultValue float64) float64 {
	if v, ok := s.params[key]; ok {
		switch val := v.(type) {
		case float64:
			return val
		case float32:
			return float64(val)
		case int:
			return float64(val)
		default:
			logger.Warn("参数类型错误，使用默认值",
				zap.String("key", key),
				zap.String("expected", "float/int"),
				zap.String("got", fmt.Sprintf("%T", val)),
			)
			return defaultValue
		}
	}
	return defaultValue
}

func (s *NeedleStrategy) GetState() NeedleState {
	return s.state
}

func (s *NeedleStrategy) Stop() {
	select {
	case <-s.stopChan:
		// 已经关闭
	default:
		close(s.stopChan)
	}

	// 清空数据释放内存
	s.barMutex.Lock()
	s.barHistory = nil
	s.barMutex.Unlock()

	s.positionMutex.Lock()
	s.position = nil
	s.positionMutex.Unlock()

	logger.Info("NeedleStrategy 已停止，资源已清理")
}

func (s *NeedleStrategy) GetSupertrend() *Supertrend {
	return s.supertrend
}

func (s *NeedleStrategy) GetMACD() *MACD {
	return s.macd
}

func (s *NeedleStrategy) OnPositionFilled(symbol string, side types.OrderSide, entryPrice, size float64) {
	s.SetPosition(symbol, side, entryPrice, size)
	logger.Info("NeedleStrategy持仓已填充",
		zap.String("symbol", symbol),
		zap.String("side", string(side)),
		zap.Float64("entry_price", entryPrice),
		zap.Float64("size", size),
	)
}

func (s *NeedleStrategy) OnPositionReduced(symbol string, exitPrice, pnl, remainingSize float64) {
	s.RecordTrade(pnl)

	s.positionMutex.Lock()
	defer s.positionMutex.Unlock()

	if s.position == nil || s.position.Symbol != symbol {
		return
	}

	s.position.Size = remainingSize
	if remainingSize <= 0 {
		s.position = nil
		s.state = NeedleStateIdle
	} else {
		s.state = NeedleStateInPosition
	}

	logger.Info("NeedleStrategy持仓部分减仓",
		zap.String("symbol", symbol),
		zap.Float64("exit_price", exitPrice),
		zap.Float64("pnl", pnl),
		zap.Float64("remaining_size", remainingSize),
	)
}

func (s *NeedleStrategy) OnPositionClosed(symbol string, exitPrice float64, pnl float64) {
	s.RecordTrade(pnl)
	s.ClearPosition()
	logger.Info("NeedleStrategy持仓已平仓",
		zap.String("symbol", symbol),
		zap.Float64("exit_price", exitPrice),
		zap.Float64("pnl", pnl),
	)
}

func (s *NeedleStrategy) ConfirmRebalanceEntry(request *RebalanceRequest) (*RebalanceDecision, error) {
	if request == nil {
		return &RebalanceDecision{RejectReason: "nil_request"}, nil
	}
	if request.ShortfallAmount <= 0 {
		return &RebalanceDecision{RejectReason: "non_positive_shortfall"}, nil
	}

	s.positionMutex.RLock()
	position := s.position
	s.positionMutex.RUnlock()
	if position != nil {
		return &RebalanceDecision{
			RejectReason:        "existing_position",
			RecommendedPrice:    position.EntryPrice,
			RecommendedQuantity: 0,
		}, nil
	}

	s.barMutex.RLock()
	var latestBar *types.Bar
	if len(s.barHistory) > 0 {
		latestBar = s.barHistory[len(s.barHistory)-1]
	}
	s.barMutex.RUnlock()
	if latestBar == nil || latestBar.Close <= 0 {
		return &RebalanceDecision{RejectReason: "missing_market_context"}, nil
	}

	divergence := s.detectDivergence()
	if divergence == DivergenceNone {
		return &RebalanceDecision{RejectReason: "no_actionable_divergence"}, nil
	}

	signal := s.generateSignal(latestBar, divergence)
	if signal == nil {
		return &RebalanceDecision{RejectReason: "signal_filtered"}, nil
	}

	recommendedQuantity := 0.0
	if signal.Price > 0 {
		recommendedQuantity = request.ShortfallAmount / signal.Price
	}

	return &RebalanceDecision{
		Approved:            true,
		RecommendedPrice:    signal.Price,
		RecommendedQuantity: recommendedQuantity,
		Signal:              signal,
	}, nil
}

func (s *NeedleStrategy) IsInPosition() bool {
	s.positionMutex.RLock()
	defer s.positionMutex.RUnlock()
	return s.position != nil
}

func (s *NeedleStrategy) GetPositionSymbol() string {
	s.positionMutex.RLock()
	defer s.positionMutex.RUnlock()
	if s.position != nil {
		return s.position.Symbol
	}
	return ""
}

func (s *NeedleStrategy) GetHoldingTime() time.Duration {
	s.positionMutex.RLock()
	defer s.positionMutex.RUnlock()
	if s.position != nil {
		return time.Since(s.position.OpenTime)
	}
	return 0
}

func calculateEMA(data []float64, period int) []float64 {
	if len(data) < period {
		return data
	}

	ema := make([]float64, len(data))
	multiplier := 2.0 / float64(period+1)

	sum := 0.0
	for i := 0; i < period; i++ {
		sum += data[i]
	}
	ema[period-1] = sum / float64(period)

	for i := period; i < len(data); i++ {
		ema[i] = (data[i]-ema[i-1])*multiplier + ema[i-1]
	}

	return ema[period-1:]
}

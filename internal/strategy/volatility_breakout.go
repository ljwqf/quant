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
	DefaultVolATRPeriod          = 14
	DefaultVolVolumeMAPeriod     = 20
	DefaultVolBreakoutMultiplier = 1.5
	DefaultVolMinVolumeRatio     = 1.2
	DefaultVolMaxHoldingBars     = 10
	DefaultVolStopLossPercent    = 0.03
	DefaultVolTrailingStopPercent = 0.02
	DefaultVolSignalCooldown     = 3600
)

type VolatilityState int

const (
	VolStateNeutral VolatilityState = iota
	VolStateBreakoutUp
	VolStateBreakoutDown
)

type VolatilityBreakoutStrategy struct {
	name              string
	params            map[string]interface{}
	metrics           map[string]interface{}
	state             VolatilityState
	atrPeriod         int
	volumeMAPeriod    int
	breakoutMultiplier float64
	minVolumeRatio    float64
	maxHoldingBars    int
	stopLossPercent    float64
	trailingStopPercent float64
	signalCooldown     int64

	// 价格历史
	prices      []float64
	highs       []float64
	lows        []float64
	volumes     []float64
	pricesMutex sync.RWMutex

	// ATR
	atr         float64
	atrMutex    sync.RWMutex

	// 成交量MA
	volumeMA    float64
	volumeMutex sync.RWMutex

	// 持仓状态
	position        *Position
	positionBars    int
	trailingStop    float64
	lastSignalTime  time.Time
	positionMutex   sync.RWMutex

	// 指标
	tradeCount      int
	totalPnL        float64
	metricsMutex    sync.Mutex

	// SmartFilter
	smartFilter     *SmartFilter
}

func NewVolatilityBreakoutStrategy() *VolatilityBreakoutStrategy {
	return &VolatilityBreakoutStrategy{
		name:               "VolatilityBreakoutStrategy",
		params:             make(map[string]interface{}),
		metrics:            make(map[string]interface{}),
		state:              VolStateNeutral,
		atrPeriod:          DefaultVolATRPeriod,
		volumeMAPeriod:     DefaultVolVolumeMAPeriod,
		breakoutMultiplier: DefaultVolBreakoutMultiplier,
		minVolumeRatio:     DefaultVolMinVolumeRatio,
		maxHoldingBars:     DefaultVolMaxHoldingBars,
		stopLossPercent:    DefaultVolStopLossPercent,
		trailingStopPercent: DefaultVolTrailingStopPercent,
		signalCooldown:     DefaultVolSignalCooldown,
		prices:             make([]float64, 0, 100),
		highs:              make([]float64, 0, 100),
		lows:               make([]float64, 0, 100),
		volumes:            make([]float64, 0, 100),
	}
}

func (s *VolatilityBreakoutStrategy) Name() string {
	return s.name
}

func (s *VolatilityBreakoutStrategy) Init(params map[string]interface{}) error {
	for k, v := range params {
		s.params[k] = v
	}

	if period, ok := params["atr_period"].(int); ok && period > 0 {
		s.atrPeriod = period
	}
	if period, ok := params["volume_ma_period"].(int); ok && period > 0 {
		s.volumeMAPeriod = period
	}
	if multiplier, ok := params["breakout_multiplier"].(float64); ok && multiplier > 0 {
		s.breakoutMultiplier = multiplier
	}
	if ratio, ok := params["min_volume_ratio"].(float64); ok && ratio > 0 {
		s.minVolumeRatio = ratio
	}
	if bars, ok := params["max_holding_bars"].(int); ok && bars > 0 {
		s.maxHoldingBars = bars
	}
	if stopLoss, ok := params["stop_loss_percent"].(float64); ok && stopLoss > 0 {
		s.stopLossPercent = stopLoss
	}
	if trailingStop, ok := params["trailing_stop_percent"].(float64); ok && trailingStop > 0 {
		s.trailingStopPercent = trailingStop
	}
	if cooldown, ok := params["signal_cooldown"].(int64); ok && cooldown > 0 {
		s.signalCooldown = cooldown
	}

	logger.Info("VolatilityBreakoutStrategy初始化完成",
		zap.Int("atr_period", s.atrPeriod),
		zap.Int("volume_ma_period", s.volumeMAPeriod),
		zap.Float64("breakout_multiplier", s.breakoutMultiplier),
		zap.Float64("min_volume_ratio", s.minVolumeRatio),
		zap.Int("max_holding_bars", s.maxHoldingBars),
		zap.Float64("stop_loss_percent", s.stopLossPercent),
		zap.Float64("trailing_stop_percent", s.trailingStopPercent),
		zap.Int64("signal_cooldown", s.signalCooldown),
	)

	return nil
}

func (s *VolatilityBreakoutStrategy) OnTick(tick *types.Tick) (*types.Signal, error) {
	// 波动率突破策略更适合基于Bar数据
	return nil, nil
}

func (s *VolatilityBreakoutStrategy) OnBar(bar *types.Bar) (*types.Signal, error) {
	s.pricesMutex.Lock()
	s.prices = append(s.prices, bar.Close)
	s.highs = append(s.highs, bar.High)
	s.lows = append(s.lows, bar.Low)
	s.volumes = append(s.volumes, bar.Volume)

	if len(s.prices) > 200 {
		s.prices = s.prices[len(s.prices)-200:]
		s.highs = s.highs[len(s.highs)-200:]
		s.lows = s.lows[len(s.lows)-200:]
		s.volumes = s.volumes[len(s.volumes)-200:]
	}
	s.pricesMutex.Unlock()

	s.updateIndicators()

	return s.checkSignal(bar.Symbol, bar.Close, bar.High, bar.Low, bar.Volume)
}

func (s *VolatilityBreakoutStrategy) OnOrderBook(orderBook *types.OrderBook) (*types.Signal, error) {
	return nil, nil
}

func (s *VolatilityBreakoutStrategy) updateIndicators() {
	s.pricesMutex.RLock()
	prices := make([]float64, len(s.prices))
	highs := make([]float64, len(s.highs))
	lows := make([]float64, len(s.lows))
	volumes := make([]float64, len(s.volumes))
	copy(prices, s.prices)
	copy(highs, s.highs)
	copy(lows, s.lows)
	copy(volumes, s.volumes)
	s.pricesMutex.RUnlock()

	if len(prices) < s.atrPeriod+1 {
		return
	}

	// 计算ATR
	atr := s.calculateATR(highs, lows, prices, s.atrPeriod)
	s.atrMutex.Lock()
	s.atr = atr
	s.atrMutex.Unlock()

	// 计算成交量MA
	if len(volumes) >= s.volumeMAPeriod {
		volumeMA := s.calculateSMA(volumes, s.volumeMAPeriod)
		s.volumeMutex.Lock()
		s.volumeMA = volumeMA
		s.volumeMutex.Unlock()
	}
}

func (s *VolatilityBreakoutStrategy) calculateATR(highs, lows, closes []float64, period int) float64 {
	if len(highs) < period+1 || len(lows) < period+1 || len(closes) < period+1 {
		return 0
	}

	trSum := 0.0
	for i := len(highs) - period; i < len(highs); i++ {
		high := highs[i]
		low := lows[i]
		prevClose := closes[i-1]

		tr1 := high - low
		tr2 := math.Abs(high - prevClose)
		tr3 := math.Abs(low - prevClose)

		tr := math.Max(tr1, math.Max(tr2, tr3))
		trSum += tr
	}

	return trSum / float64(period)
}

func (s *VolatilityBreakoutStrategy) calculateSMA(values []float64, period int) float64 {
	if len(values) < period {
		return 0
	}

	sum := 0.0
	for i := len(values) - period; i < len(values); i++ {
		sum += values[i]
	}

	return sum / float64(period)
}

func (s *VolatilityBreakoutStrategy) checkSignal(symbol string, close, high, low, volume float64) (*types.Signal, error) {
	s.positionMutex.RLock()
	position := s.position
	positionBars := s.positionBars
	s.positionMutex.RUnlock()

	// 如果有持仓，检查是否需要平仓
	if position != nil {
		return s.checkExitSignal(symbol, close, positionBars)
	}

	// 检查入场信号
	return s.checkEntrySignal(symbol, close, high, low, volume)
}

func (s *VolatilityBreakoutStrategy) checkEntrySignal(symbol string, close, high, low, volume float64) (*types.Signal, error) {
	s.positionMutex.RLock()
	lastSignalTime := s.lastSignalTime
	s.positionMutex.RUnlock()

	if time.Since(lastSignalTime) < time.Duration(s.signalCooldown)*time.Second {
		return nil, nil
	}

	s.atrMutex.RLock()
	atr := s.atr
	s.atrMutex.RUnlock()

	s.volumeMutex.RLock()
	volumeMA := s.volumeMA
	s.volumeMutex.RUnlock()

	s.pricesMutex.RLock()
	if len(s.prices) < 2 {
		s.pricesMutex.RUnlock()
		return nil, nil
	}
	prevClose := s.prices[len(s.prices)-2]
	s.pricesMutex.RUnlock()

	if atr == 0 || volumeMA == 0 {
		return nil, nil
	}

	volumeRatio := volume / volumeMA
	if volumeRatio < s.minVolumeRatio {
		return nil, nil
	}

	var signalType types.SignalType
	shouldEnter := false

	// 向上突破
	if close > prevClose+atr*s.breakoutMultiplier {
		signalType = types.SignalTypeBuy
		shouldEnter = true
		s.state = VolStateBreakoutUp
	}

	// 向下突破
	if close < prevClose-atr*s.breakoutMultiplier {
		signalType = types.SignalTypeSell
		shouldEnter = true
		s.state = VolStateBreakoutDown
	}

	if !shouldEnter {
		return nil, nil
	}

	// 检查SmartFilter
	if s.smartFilter != nil {
		if s.state == VolStateBreakoutUp && !s.smartFilter.CanOpenLong() {
			return nil, nil
		}
		if s.state == VolStateBreakoutDown && !s.smartFilter.CanOpenShort() {
			return nil, nil
		}
	}

	s.positionMutex.Lock()
	s.lastSignalTime = time.Now()
	s.positionMutex.Unlock()

	logger.Info("VolatilityBreakoutStrategy生成信号",
		zap.String("symbol", symbol),
		zap.String("type", string(signalType)),
		zap.Float64("close", close),
		zap.Float64("atr", atr),
		zap.Float64("volume_ratio", volumeRatio),
		zap.String("state", s.stateToString()),
	)

	return &types.Signal{
		Strategy: s.name,
		Symbol:   symbol,
		Type:     signalType,
		Price:    close,
		Quantity: 1.0,
		Timestamp: time.Now(),
		Metadata: map[string]interface{}{
			"atr":           atr,
			"volume_ratio":  volumeRatio,
			"prev_close":    prevClose,
			"state":         s.stateToString(),
		},
	}, nil
}

func (s *VolatilityBreakoutStrategy) checkExitSignal(symbol string, close float64, positionBars int) (*types.Signal, error) {
	s.positionMutex.Lock()
	defer s.positionMutex.Unlock()

	position := s.position
	if position == nil {
		return nil, nil
	}

	shouldExit := false
	exitReason := ""

	// 固定止损检查
	if position.Side == types.OrderSideBuy {
		stopLossPrice := position.EntryPrice * (1 - s.stopLossPercent)
		if close < stopLossPrice {
			shouldExit = true
			exitReason = "stop_loss"
		}

		if s.trailingStop > 0 {
			if close > s.trailingStop+position.EntryPrice*s.trailingStopPercent {
				s.trailingStop = close - position.EntryPrice*s.trailingStopPercent
			}
			if close < s.trailingStop {
				shouldExit = true
				exitReason = "trailing_stop"
			}
		} else {
			s.trailingStop = position.EntryPrice * (1 - s.trailingStopPercent)
		}
	}

	if position.Side == types.OrderSideSell {
		stopLossPrice := position.EntryPrice * (1 + s.stopLossPercent)
		if close > stopLossPrice {
			shouldExit = true
			exitReason = "stop_loss"
		}

		if s.trailingStop > 0 {
			if close < s.trailingStop-position.EntryPrice*s.trailingStopPercent {
				s.trailingStop = close + position.EntryPrice*s.trailingStopPercent
			}
			if close > s.trailingStop {
				shouldExit = true
				exitReason = "trailing_stop"
			}
		} else {
			s.trailingStop = position.EntryPrice * (1 + s.trailingStopPercent)
		}
	}

	if !shouldExit {
		if positionBars >= s.maxHoldingBars {
			shouldExit = true
			exitReason = "max_holding_time"
		}
	}

	if !shouldExit {
		s.atrMutex.RLock()
		atr := s.atr
		s.atrMutex.RUnlock()

		s.pricesMutex.RLock()
		if len(s.prices) >= 2 {
			prevClose := s.prices[len(s.prices)-2]
			s.pricesMutex.RUnlock()

			if position.Side == types.OrderSideBuy && close < prevClose-atr*0.5 {
				shouldExit = true
				exitReason = "reverse_breakout"
			}
			if position.Side == types.OrderSideSell && close > prevClose+atr*0.5 {
				shouldExit = true
				exitReason = "reverse_breakout"
			}
		} else {
			s.pricesMutex.RUnlock()
		}
	}

	if shouldExit {
		s.trailingStop = 0

		logger.Info("VolatilityBreakoutStrategy平仓信号",
			zap.String("symbol", symbol),
			zap.String("side", string(position.Side)),
			zap.Float64("entry_price", position.EntryPrice),
			zap.Float64("exit_price", close),
			zap.String("reason", exitReason),
			zap.Int("position_bars", positionBars),
		)

		return &types.Signal{
			Strategy: s.name,
			Symbol:   symbol,
			Type:     types.SignalTypeExit,
			Price:    close,
			Quantity: position.Size,
			Timestamp: time.Now(),
			Metadata: map[string]interface{}{
				"reason":        exitReason,
				"position_bars": positionBars,
				"state":         s.stateToString(),
			},
		}, nil
	}

	return nil, nil
}

func (s *VolatilityBreakoutStrategy) stateToString() string {
	switch s.state {
	case VolStateBreakoutUp:
		return "breakout_up"
	case VolStateBreakoutDown:
		return "breakout_down"
	default:
		return "neutral"
	}
}

func (s *VolatilityBreakoutStrategy) GetParams() map[string]interface{} {
	return s.params
}

func (s *VolatilityBreakoutStrategy) SetParams(params map[string]interface{}) {
	for k, v := range params {
		s.params[k] = v
	}
	s.updateIndicators()
}

func (s *VolatilityBreakoutStrategy) GetMetrics() map[string]interface{} {
	s.metricsMutex.Lock()
	defer s.metricsMutex.Unlock()

	s.metrics["state"] = s.stateToString()
	s.metrics["atr"] = s.atr
	s.metrics["volume_ma"] = s.volumeMA
	s.metrics["trade_count"] = s.tradeCount
	s.metrics["total_pnl"] = s.totalPnL

	return s.metrics
}

func (s *VolatilityBreakoutStrategy) OnPositionFilled(symbol string, side types.OrderSide, entryPrice, size float64) {
	s.positionMutex.Lock()
	defer s.positionMutex.Unlock()

	s.position = &Position{
		Symbol:     symbol,
		Side:       side,
		EntryPrice: entryPrice,
		Size:       size,
		EntryTime:  time.Now(),
	}
	s.positionBars = 0

	s.tradeCount++

	logger.Info("VolatilityBreakoutStrategy持仓已填充",
		zap.String("symbol", symbol),
		zap.String("side", string(side)),
		zap.Float64("entry_price", entryPrice),
		zap.Float64("size", size),
	)
}

func (s *VolatilityBreakoutStrategy) OnPositionReduced(symbol string, exitPrice, pnl, remainingSize float64) {
	logger.Info("VolatilityBreakoutStrategy持仓部分减仓",
		zap.String("symbol", symbol),
		zap.Float64("exit_price", exitPrice),
		zap.Float64("pnl", pnl),
		zap.Float64("remaining_size", remainingSize),
	)
}

func (s *VolatilityBreakoutStrategy) OnPositionClosed(symbol string, exitPrice, pnl float64) {
	s.positionMutex.Lock()
	s.position = nil
	s.positionBars = 0
	s.trailingStop = 0
	s.totalPnL += pnl
	s.positionMutex.Unlock()

	logger.Info("VolatilityBreakoutStrategy持仓已平仓",
		zap.String("symbol", symbol),
		zap.Float64("exit_price", exitPrice),
		zap.Float64("pnl", pnl),
		zap.Float64("total_pnl", s.totalPnL),
	)
}

func (s *VolatilityBreakoutStrategy) ConfirmRebalanceEntry(request *RebalanceRequest) (*RebalanceDecision, error) {
	return &RebalanceDecision{RejectReason: "rebalance_entry_unsupported"}, nil
}

func (s *VolatilityBreakoutStrategy) SetSmartFilter(filter *SmartFilter) {
	s.smartFilter = filter
}

func (s *VolatilityBreakoutStrategy) UpdateOnChainData(netflow, sopr, mvrv float64) {
	// 波动率突破策略不直接使用链上数据，但通过SmartFilter间接使用
	logger.Debug("VolatilityBreakoutStrategy更新链上数据",
		zap.Float64("netflow", netflow),
		zap.Float64("sopr", sopr),
		zap.Float64("mvrv", mvrv),
	)
}

func (s *VolatilityBreakoutStrategy) IncrementPositionBars() {
	s.positionMutex.Lock()
	if s.position != nil {
		s.positionBars++
	}
	s.positionMutex.Unlock()
}

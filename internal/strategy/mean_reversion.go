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
	DefaultMeanRevRSIPeriod          = 14
	DefaultMeanRevRSIOverbought      = 70.0
	DefaultMeanRevRSIOversold        = 30.0
	DefaultMeanRevBollingerPeriod    = 20
	DefaultMeanRevBollingerStdDev    = 2.0
	DefaultMeanRevThreshold = 0.02
	DefaultMeanRevStopLossPercent    = 0.05
	DefaultMeanRevTrailingStopPercent = 0.03
	DefaultMeanRevSignalCooldown     = 3600
)

type MeanReversionState int

const (
	MeanRevStateNeutral MeanReversionState = iota
	MeanRevStateOverbought
	MeanRevStateOversold
)

type MeanReversionStrategy struct {
	name           string
	params         map[string]interface{}
	metrics        map[string]interface{}
	state          MeanReversionState
	rsiPeriod      int
	rsiOverbought  float64
	rsiOversold    float64
	bbPeriod       int
	bbStdDev       float64
	threshold      float64
	stopLossPercent    float64
	trailingStopPercent float64
	signalCooldown     int64

	// 价格历史
	prices      []float64
	pricesMutex sync.RWMutex

	// RSI
	rsi         float64
	rsiMutex    sync.RWMutex

	// 布林带
	bbUpper     float64
	bbMiddle    float64
	bbLower     float64
	bbMutex     sync.RWMutex

	// 持仓状态
	position    *Position
	trailingStop float64
	lastSignalTime time.Time
	positionMutex sync.RWMutex

	// 指标
	tradeCount  int
	totalPnL    float64
	metricsMutex sync.Mutex

	// SmartFilter
	smartFilter *SmartFilter
}

func NewMeanReversionStrategy() *MeanReversionStrategy {
	return &MeanReversionStrategy{
		name:          "MeanReversionStrategy",
		params:        make(map[string]interface{}),
		metrics:       make(map[string]interface{}),
		state:         MeanRevStateNeutral,
		rsiPeriod:     DefaultMeanRevRSIPeriod,
		rsiOverbought: DefaultMeanRevRSIOverbought,
		rsiOversold:   DefaultMeanRevRSIOversold,
		bbPeriod:      DefaultMeanRevBollingerPeriod,
		bbStdDev:      DefaultMeanRevBollingerStdDev,
		threshold:     DefaultMeanRevThreshold,
		stopLossPercent: DefaultMeanRevStopLossPercent,
		trailingStopPercent: DefaultMeanRevTrailingStopPercent,
		signalCooldown: DefaultMeanRevSignalCooldown,
		prices:        make([]float64, 0, 100),
	}
}

func (s *MeanReversionStrategy) Name() string {
	return s.name
}

func (s *MeanReversionStrategy) Init(params map[string]interface{}) error {
	for k, v := range params {
		s.params[k] = v
	}

	if period, ok := params["rsi_period"].(int); ok && period > 0 {
		s.rsiPeriod = period
	}
	if threshold, ok := params["rsi_overbought"].(float64); ok && threshold > 0 {
		s.rsiOverbought = threshold
	}
	if threshold, ok := params["rsi_oversold"].(float64); ok && threshold > 0 {
		s.rsiOversold = threshold
	}
	if period, ok := params["bb_period"].(int); ok && period > 0 {
		s.bbPeriod = period
	}
	if stdDev, ok := params["bb_std_dev"].(float64); ok && stdDev > 0 {
		s.bbStdDev = stdDev
	}
	if threshold, ok := params["threshold"].(float64); ok && threshold > 0 {
		s.threshold = threshold
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

	logger.Info("MeanReversionStrategy初始化完成",
		zap.Int("rsi_period", s.rsiPeriod),
		zap.Float64("rsi_overbought", s.rsiOverbought),
		zap.Float64("rsi_oversold", s.rsiOversold),
		zap.Int("bb_period", s.bbPeriod),
		zap.Float64("bb_std_dev", s.bbStdDev),
		zap.Float64("stop_loss_percent", s.stopLossPercent),
		zap.Float64("trailing_stop_percent", s.trailingStopPercent),
		zap.Int64("signal_cooldown", s.signalCooldown),
	)

	return nil
}

func (s *MeanReversionStrategy) OnTick(tick *types.Tick) (*types.Signal, error) {
	s.pricesMutex.Lock()
	s.prices = append(s.prices, tick.Price)
	
	if len(s.prices) > 200 {
		s.prices = s.prices[len(s.prices)-200:]
	}
	s.pricesMutex.Unlock()

	s.updateIndicators()

	return s.checkSignal(tick.Symbol, tick.Price)
}

func (s *MeanReversionStrategy) OnBar(bar *types.Bar) (*types.Signal, error) {
	s.pricesMutex.Lock()
	s.prices = append(s.prices, bar.Close)
	
	if len(s.prices) > 200 {
		s.prices = s.prices[len(s.prices)-200:]
	}
	s.pricesMutex.Unlock()

	s.updateIndicators()

	return s.checkSignal(bar.Symbol, bar.Close)
}

func (s *MeanReversionStrategy) OnOrderBook(orderBook *types.OrderBook) (*types.Signal, error) {
	return nil, nil
}

func (s *MeanReversionStrategy) updateIndicators() {
	s.pricesMutex.RLock()
	prices := make([]float64, len(s.prices))
	copy(prices, s.prices)
	s.pricesMutex.RUnlock()

	if len(prices) < s.bbPeriod {
		return
	}

	// 计算RSI
	if len(prices) >= s.rsiPeriod+1 {
		rsi := s.calculateRSI(prices, s.rsiPeriod)
		s.rsiMutex.Lock()
		s.rsi = rsi
		s.rsiMutex.Unlock()
	}

	// 计算布林带
	upper, middle, lower := s.calculateBollingerBands(prices, s.bbPeriod, s.bbStdDev)
	s.bbMutex.Lock()
	s.bbUpper = upper
	s.bbMiddle = middle
	s.bbLower = lower
	s.bbMutex.Unlock()

	// 更新状态
	s.updateState()
}

func (s *MeanReversionStrategy) calculateRSI(prices []float64, period int) float64 {
	if len(prices) < period+1 {
		return 50.0
	}

	gains := 0.0
	losses := 0.0

	for i := len(prices) - period; i < len(prices); i++ {
		change := prices[i] - prices[i-1]
		if change > 0 {
			gains += change
		} else {
			losses += -change
		}
	}

	avgGain := gains / float64(period)
	avgLoss := losses / float64(period)

	if avgLoss == 0 {
		return 100.0
	}

	rs := avgGain / avgLoss
	rsi := 100.0 - (100.0 / (1.0 + rs))

	return rsi
}

func (s *MeanReversionStrategy) calculateBollingerBands(prices []float64, period int, stdDev float64) (upper, middle, lower float64) {
	if len(prices) < period {
		return 0, 0, 0
	}

	// 计算中轨（简单移动平均线）
	sum := 0.0
	for i := len(prices) - period; i < len(prices); i++ {
		sum += prices[i]
	}
	middle = sum / float64(period)

	// 计算标准差
	variance := 0.0
	for i := len(prices) - period; i < len(prices); i++ {
		diff := prices[i] - middle
		variance += diff * diff
	}
	std := math.Sqrt(variance / float64(period))

	// 计算上下轨
	upper = middle + stdDev*std
	lower = middle - stdDev*std

	return upper, middle, lower
}

func (s *MeanReversionStrategy) updateState() {
	s.rsiMutex.RLock()
	rsi := s.rsi
	s.rsiMutex.RUnlock()

	if rsi >= s.rsiOverbought {
		s.state = MeanRevStateOverbought
	} else if rsi <= s.rsiOversold {
		s.state = MeanRevStateOversold
	} else {
		s.state = MeanRevStateNeutral
	}
}

func (s *MeanReversionStrategy) checkSignal(symbol string, price float64) (*types.Signal, error) {
	// 检查SmartFilter
	if s.smartFilter != nil {
		if s.state == MeanRevStateOversold && !s.smartFilter.CanOpenLong() {
			return nil, nil
		}
		if s.state == MeanRevStateOverbought && !s.smartFilter.CanOpenShort() {
			return nil, nil
		}
	}

	s.positionMutex.RLock()
	position := s.position
	s.positionMutex.RUnlock()

	// 如果有持仓，检查是否需要平仓
	if position != nil {
		return s.checkExitSignal(symbol, price)
	}

	// 检查入场信号
	return s.checkEntrySignal(symbol, price)
}

func (s *MeanReversionStrategy) checkEntrySignal(symbol string, price float64) (*types.Signal, error) {
	s.positionMutex.RLock()
	lastSignalTime := s.lastSignalTime
	s.positionMutex.RUnlock()

	if time.Since(lastSignalTime) < time.Duration(s.signalCooldown)*time.Second {
		return nil, nil
	}

	s.rsiMutex.RLock()
	rsi := s.rsi
	s.rsiMutex.RUnlock()

	s.bbMutex.RLock()
	bbUpper := s.bbUpper
	bbLower := s.bbLower
	s.bbMutex.RUnlock()

	var signalType types.SignalType
	shouldEnter := false

	// 超卖 + 价格低于下轨 = 买入信号
	if rsi <= s.rsiOversold && price < bbLower {
		signalType = types.SignalTypeBuy
		shouldEnter = true
	}

	// 超买 + 价格高于上轨 = 卖出信号
	if rsi >= s.rsiOverbought && price > bbUpper {
		signalType = types.SignalTypeSell
		shouldEnter = true
	}

	if !shouldEnter {
		return nil, nil
	}

	s.positionMutex.Lock()
	s.lastSignalTime = time.Now()
	s.positionMutex.Unlock()

	logger.Info("MeanReversionStrategy生成信号",
		zap.String("symbol", symbol),
		zap.String("type", string(signalType)),
		zap.Float64("price", price),
		zap.Float64("rsi", rsi),
		zap.Float64("bb_upper", bbUpper),
		zap.Float64("bb_lower", bbLower),
	)

	return &types.Signal{
		Strategy: s.name,
		Symbol:   symbol,
		Type:     signalType,
		Price:    price,
		Quantity: 1.0,
		Timestamp: time.Now(),
		Metadata: map[string]interface{}{
			"rsi":       rsi,
			"bb_upper":  bbUpper,
			"bb_middle": s.bbMiddle,
			"bb_lower":  bbLower,
			"state":     s.stateToString(),
		},
	}, nil
}

func (s *MeanReversionStrategy) checkExitSignal(symbol string, price float64) (*types.Signal, error) {
	s.positionMutex.Lock()
	defer s.positionMutex.Unlock()

	position := s.position
	if position == nil {
		return nil, nil
	}

	shouldExit := false
	exitReason := ""

	if position.Side == types.OrderSideBuy {
		stopLossPrice := position.EntryPrice * (1 - s.stopLossPercent)
		if price < stopLossPrice {
			shouldExit = true
			exitReason = "stop_loss"
		}

		if s.trailingStop > 0 {
			if price > s.trailingStop+position.EntryPrice*s.trailingStopPercent {
				s.trailingStop = price - position.EntryPrice*s.trailingStopPercent
			}
			if price < s.trailingStop {
				shouldExit = true
				exitReason = "trailing_stop"
			}
		} else {
			s.trailingStop = position.EntryPrice * (1 - s.trailingStopPercent)
		}
	}

	if position.Side == types.OrderSideSell {
		stopLossPrice := position.EntryPrice * (1 + s.stopLossPercent)
		if price > stopLossPrice {
			shouldExit = true
			exitReason = "stop_loss"
		}

		if s.trailingStop > 0 {
			if price < s.trailingStop-position.EntryPrice*s.trailingStopPercent {
				s.trailingStop = price + position.EntryPrice*s.trailingStopPercent
			}
			if price > s.trailingStop {
				shouldExit = true
				exitReason = "trailing_stop"
			}
		} else {
			s.trailingStop = position.EntryPrice * (1 + s.trailingStopPercent)
		}
	}

	if !shouldExit {
		s.rsiMutex.RLock()
		rsi := s.rsi
		s.rsiMutex.RUnlock()

		s.bbMutex.RLock()
		bbMiddle := s.bbMiddle
		s.bbMutex.RUnlock()

		if position.Side == types.OrderSideBuy {
			if price >= bbMiddle || rsi >= 50 {
				shouldExit = true
				exitReason = "price_returned_to_mean"
			}
		}

		if position.Side == types.OrderSideSell {
			if price <= bbMiddle || rsi <= 50 {
				shouldExit = true
				exitReason = "price_returned_to_mean"
			}
		}
	}

	if shouldExit {
		s.trailingStop = 0

		logger.Info("MeanReversionStrategy平仓信号",
			zap.String("symbol", symbol),
			zap.String("side", string(position.Side)),
			zap.Float64("entry_price", position.EntryPrice),
			zap.Float64("exit_price", price),
			zap.String("reason", exitReason),
		)

		return &types.Signal{
			Strategy: s.name,
			Symbol:   symbol,
			Type:     types.SignalTypeExit,
			Price:    price,
			Quantity: position.Size,
			Timestamp: time.Now(),
			Metadata: map[string]interface{}{
				"reason": exitReason,
				"state":  s.stateToString(),
			},
		}, nil
	}

	return nil, nil
}

func (s *MeanReversionStrategy) stateToString() string {
	switch s.state {
	case MeanRevStateOverbought:
		return "overbought"
	case MeanRevStateOversold:
		return "oversold"
	default:
		return "neutral"
	}
}

func (s *MeanReversionStrategy) GetParams() map[string]interface{} {
	return s.params
}

func (s *MeanReversionStrategy) SetParams(params map[string]interface{}) {
	for k, v := range params {
		s.params[k] = v
	}
}

func (s *MeanReversionStrategy) GetMetrics() map[string]interface{} {
	s.metricsMutex.Lock()
	defer s.metricsMutex.Unlock()

	s.metrics["state"] = s.stateToString()
	s.metrics["rsi"] = s.rsi
	s.metrics["bb_upper"] = s.bbUpper
	s.metrics["bb_middle"] = s.bbMiddle
	s.metrics["bb_lower"] = s.bbLower
	s.metrics["trade_count"] = s.tradeCount
	s.metrics["total_pnl"] = s.totalPnL

	return s.metrics
}

func (s *MeanReversionStrategy) OnPositionFilled(symbol string, side types.OrderSide, entryPrice, size float64) {
	s.positionMutex.Lock()
	defer s.positionMutex.Unlock()

	s.position = &Position{
		Symbol:     symbol,
		Side:       side,
		EntryPrice: entryPrice,
		Size:       size,
		EntryTime:  time.Now(),
	}

	s.tradeCount++

	logger.Info("MeanReversionStrategy持仓已填充",
		zap.String("symbol", symbol),
		zap.String("side", string(side)),
		zap.Float64("entry_price", entryPrice),
		zap.Float64("size", size),
	)
}

func (s *MeanReversionStrategy) OnPositionReduced(symbol string, exitPrice, pnl, remainingSize float64) {
	logger.Info("MeanReversionStrategy持仓部分减仓",
		zap.String("symbol", symbol),
		zap.Float64("exit_price", exitPrice),
		zap.Float64("pnl", pnl),
		zap.Float64("remaining_size", remainingSize),
	)
}

func (s *MeanReversionStrategy) OnPositionClosed(symbol string, exitPrice, pnl float64) {
	s.positionMutex.Lock()
	s.position = nil
	s.trailingStop = 0
	s.totalPnL += pnl
	s.positionMutex.Unlock()

	logger.Info("MeanReversionStrategy持仓已平仓",
		zap.String("symbol", symbol),
		zap.Float64("exit_price", exitPrice),
		zap.Float64("pnl", pnl),
		zap.Float64("total_pnl", s.totalPnL),
	)
}

func (s *MeanReversionStrategy) ConfirmRebalanceEntry(request *RebalanceRequest) (*RebalanceDecision, error) {
	return &RebalanceDecision{RejectReason: "rebalance_entry_unsupported"}, nil
}

func (s *MeanReversionStrategy) SetSmartFilter(filter *SmartFilter) {
	s.smartFilter = filter
}

func (s *MeanReversionStrategy) UpdateOnChainData(netflow, sopr, mvrv float64) {
	// 均值回归策略不直接使用链上数据，但通过SmartFilter间接使用
	logger.Debug("MeanReversionStrategy更新链上数据",
		zap.Float64("netflow", netflow),
		zap.Float64("sopr", sopr),
		zap.Float64("mvrv", mvrv),
	)
}

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
	DefaultTrendEMAShortPeriod      = 12
	DefaultTrendEMALongPeriod       = 26
	DefaultTrendADXPeriod           = 14
	DefaultTrendADXThreshold        = 25.0
	DefaultTrendStrength       = 0.02
	DefaultTrendStopLossPercent     = 0.05
	DefaultTrendTrailingStopPercent = 0.03
	DefaultTrendSignalCooldown     = 3600
)

type TrendFollowingState int

const (
	TrendStateNeutral TrendFollowingState = iota
	TrendStateUptrend
	TrendStateDowntrend
)

type TrendFollowingStrategy struct {
	name                 string
	params               map[string]interface{}
	metrics              map[string]interface{}
	state                TrendFollowingState
	emaShortPeriod       int
	emaLongPeriod        int
	adxPeriod            int
	adxThreshold         float64
	trendStrength        float64
	stopLossPercent      float64
	trailingStopPercent  float64

	// 价格历史
	prices      []float64
	pricesMutex sync.RWMutex

	// EMA值
	emaShort    float64
	emaLong     float64
	emaMutex    sync.RWMutex

	// ADX相关
	adx         float64
	plusDI      float64
	minusDI     float64
	adxMutex    sync.RWMutex

	// 持仓状态
	position         *Position
	positionMutex    sync.RWMutex
	lastEntryPrice   float64
	trailingStop     float64

	// 防止重复入场
	lastSignalTime   time.Time
	signalCooldown   int64

	// 指标
	tradeCount     int
	totalPnL       float64
	metricsMutex   sync.Mutex

	// SmartFilter
	smartFilter *SmartFilter
}

type Position struct {
	Symbol     string
	Side       types.OrderSide
	EntryPrice float64
	Size       float64
	EntryTime  time.Time
}

func NewTrendFollowingStrategy() *TrendFollowingStrategy {
	return &TrendFollowingStrategy{
		name:                "TrendFollowingStrategy",
		params:              make(map[string]interface{}),
		metrics:             make(map[string]interface{}),
		state:               TrendStateNeutral,
		emaShortPeriod:      DefaultTrendEMAShortPeriod,
		emaLongPeriod:       DefaultTrendEMALongPeriod,
		adxPeriod:           DefaultTrendADXPeriod,
		adxThreshold:        DefaultTrendADXThreshold,
		trendStrength:       DefaultTrendStrength,
		stopLossPercent:     DefaultTrendStopLossPercent,
		trailingStopPercent: DefaultTrendTrailingStopPercent,
		signalCooldown:      DefaultTrendSignalCooldown,
		prices:              make([]float64, 0, 200),
	}
}

func (s *TrendFollowingStrategy) Name() string {
	return s.name
}

func (s *TrendFollowingStrategy) Init(params map[string]interface{}) error {
	for k, v := range params {
		s.params[k] = v
	}

	// 读取并验证参数
	s.emaShortPeriod = getInt(params, "ema_short_period", DefaultTrendEMAShortPeriod)
	if err := validateIntRange(s.emaShortPeriod, 2, 100, "ema_short_period"); err != nil {
		return err
	}

	s.emaLongPeriod = getInt(params, "ema_long_period", DefaultTrendEMALongPeriod)
	if err := validateIntRange(s.emaLongPeriod, 5, 200, "ema_long_period"); err != nil {
		return err
	}

	if s.emaShortPeriod >= s.emaLongPeriod {
		return fmt.Errorf("ema_short_period (%d) must be less than ema_long_period (%d)", s.emaShortPeriod, s.emaLongPeriod)
	}

	s.adxPeriod = getInt(params, "adx_period", DefaultTrendADXPeriod)
	if err := validateIntRange(s.adxPeriod, 2, 50, "adx_period"); err != nil {
		return err
	}

	s.adxThreshold = getFloat64(params, "adx_threshold", DefaultTrendADXThreshold)
	if err := validateFloatRange(s.adxThreshold, 0, 100, "adx_threshold"); err != nil {
		return err
	}

	s.trendStrength = getFloat64(params, "trend_strength", DefaultTrendStrength)
	if err := validateFloatRange(s.trendStrength, 0.001, 0.5, "trend_strength"); err != nil {
		return err
	}

	s.stopLossPercent = getFloat64(params, "stop_loss_percent", DefaultTrendStopLossPercent)
	if err := validateFloatRange(s.stopLossPercent, 0.001, 0.5, "stop_loss_percent"); err != nil {
		return err
	}

	s.trailingStopPercent = getFloat64(params, "trailing_stop_percent", DefaultTrendTrailingStopPercent)
	if err := validateFloatRange(s.trailingStopPercent, 0.001, 0.5, "trailing_stop_percent"); err != nil {
		return err
	}

	s.signalCooldown = int64(getInt(params, "signal_cooldown", int(DefaultTrendSignalCooldown)))
	if err := validatePositiveInt(int(s.signalCooldown), "signal_cooldown"); err != nil {
		return err
	}

	logger.Info("TrendFollowingStrategy初始化完成",
		zap.Int("ema_short_period", s.emaShortPeriod),
		zap.Int("ema_long_period", s.emaLongPeriod),
		zap.Int("adx_period", s.adxPeriod),
		zap.Float64("adx_threshold", s.adxThreshold),
		zap.Float64("trend_strength", s.trendStrength),
		zap.Float64("stop_loss_percent", s.stopLossPercent),
		zap.Float64("trailing_stop_percent", s.trailingStopPercent),
		zap.Int64("signal_cooldown", s.signalCooldown),
	)

	return nil
}

func (s *TrendFollowingStrategy) OnTick(tick *types.Tick) (*types.Signal, error) {
	s.pricesMutex.Lock()
	s.prices = append(s.prices, tick.Price)
	
	if len(s.prices) > 200 {
		s.prices = s.prices[len(s.prices)-200:]
	}
	s.pricesMutex.Unlock()

	s.updateIndicators()

	return s.checkSignal(tick.Symbol, tick.Price)
}

func (s *TrendFollowingStrategy) OnBar(bar *types.Bar) (*types.Signal, error) {
	s.pricesMutex.Lock()
	s.prices = append(s.prices, bar.Close)
	
	if len(s.prices) > 200 {
		s.prices = s.prices[len(s.prices)-200:]
	}
	s.pricesMutex.Unlock()

	s.updateIndicators()

	return s.checkSignal(bar.Symbol, bar.Close)
}

func (s *TrendFollowingStrategy) OnOrderBook(orderBook *types.OrderBook) (*types.Signal, error) {
	return nil, nil
}

func (s *TrendFollowingStrategy) updateIndicators() {
	s.pricesMutex.RLock()
	prices := make([]float64, len(s.prices))
	copy(prices, s.prices)
	s.pricesMutex.RUnlock()

	if len(prices) < s.emaLongPeriod+1 {
		return
	}

	emaShort := s.calculateEMA(prices, s.emaShortPeriod)
	emaLong := s.calculateEMA(prices, s.emaLongPeriod)

	s.emaMutex.Lock()
	s.emaShort = emaShort
	s.emaLong = emaLong
	s.emaMutex.Unlock()

	if len(prices) >= s.adxPeriod*2 {
		adx, plusDI, minusDI := s.calculateADX(prices, s.adxPeriod)
		s.adxMutex.Lock()
		s.adx = adx
		s.plusDI = plusDI
		s.minusDI = minusDI
		s.adxMutex.Unlock()
	}

	s.updateTrendState()
}

func (s *TrendFollowingStrategy) calculateEMA(prices []float64, period int) float64 {
	if len(prices) < period {
		return 0
	}

	multiplier := 2.0 / float64(period+1)
	ema := prices[0]

	for i := 1; i < len(prices); i++ {
		ema = (prices[i]-ema)*multiplier + ema
	}

	return ema
}

func (s *TrendFollowingStrategy) calculateADX(prices []float64, period int) (adx, plusDI, minusDI float64) {
	if len(prices) < period+1 {
		return 0, 0, 0
	}

	trList := make([]float64, period)
	plusDMList := make([]float64, period)
	minusDMList := make([]float64, period)

	for i := 1; i <= period; i++ {
		idx := len(prices) - period - 1 + i
		if idx <= 0 {
			continue
		}

		high := prices[idx]
		low := prices[idx]
		prevClose := prices[idx-1]

		tr := math.Max(high-low, math.Max(math.Abs(high-prevClose), math.Abs(low-prevClose)))
		trList[i-1] = tr

		plusDM := 0.0
		minusDM := 0.0
		if idx > 0 {
			highDiff := high - prices[idx-1]
			lowDiff := prices[idx-1] - low
			if highDiff > lowDiff && highDiff > 0 {
				plusDM = highDiff
			}
			if lowDiff > highDiff && lowDiff > 0 {
				minusDM = lowDiff
			}
		}
		plusDMList[i-1] = plusDM
		minusDMList[i-1] = minusDM
	}

	atr := 0.0
	plusDMSum := 0.0
	minusDMSum := 0.0
	for i := 0; i < period; i++ {
		atr += trList[i]
		plusDMSum += plusDMList[i]
		minusDMSum += minusDMList[i]
	}
	atr /= float64(period)
	plusDMSum /= float64(period)
	minusDMSum /= float64(period)

	if atr > 0 {
		plusDI = (plusDMSum / atr) * 100
		minusDI = (minusDMSum / atr) * 100
	}

	dx := 0.0
	if plusDI+minusDI > 0 {
		dx = math.Abs(plusDI-minusDI) / (plusDI + minusDI) * 100
	}
	adx = dx

	return adx, plusDI, minusDI
}

func (s *TrendFollowingStrategy) updateTrendState() {
	s.emaMutex.RLock()
	emaShort := s.emaShort
	emaLong := s.emaLong
	s.emaMutex.RUnlock()

	s.adxMutex.RLock()
	adx := s.adx
	s.adxMutex.RUnlock()

	if emaShort == 0 || emaLong == 0 {
		return
	}

	emaDiff := (emaShort - emaLong) / emaLong

	if adx >= s.adxThreshold {
		if emaDiff > s.trendStrength {
			s.state = TrendStateUptrend
		} else if emaDiff < -s.trendStrength {
			s.state = TrendStateDowntrend
		} else {
			s.state = TrendStateNeutral
		}
	} else {
		s.state = TrendStateNeutral
	}
}

func (s *TrendFollowingStrategy) checkSignal(symbol string, price float64) (*types.Signal, error) {
	if s.smartFilter != nil {
		if s.state == TrendStateUptrend && !s.smartFilter.CanOpenLong() {
			return nil, nil
		}
		if s.state == TrendStateDowntrend && !s.smartFilter.CanOpenShort() {
			return nil, nil
		}
	}

	s.positionMutex.RLock()
	position := s.position
	s.positionMutex.RUnlock()

	if position != nil {
		return s.checkExitSignal(symbol, price)
	}

	return s.checkEntrySignal(symbol, price)
}

func (s *TrendFollowingStrategy) checkEntrySignal(symbol string, price float64) (*types.Signal, error) {
	s.adxMutex.RLock()
	adx := s.adx
	s.adxMutex.RUnlock()

	if adx < s.adxThreshold {
		return nil, nil
	}

	s.positionMutex.RLock()
	lastSignalTime := s.lastSignalTime
	signalCooldown := s.signalCooldown
	s.positionMutex.RUnlock()

	if time.Since(lastSignalTime) < time.Duration(signalCooldown)*time.Second {
		return nil, nil
	}

	var signalType types.SignalType
	shouldEnter := false

	if s.state == TrendStateUptrend {
		signalType = types.SignalTypeBuy
		shouldEnter = true
	} else if s.state == TrendStateDowntrend {
		signalType = types.SignalTypeSell
		shouldEnter = true
	}

	if !shouldEnter {
		return nil, nil
	}

	s.positionMutex.Lock()
	s.lastSignalTime = time.Now()
	s.lastEntryPrice = price
	s.trailingStop = price

	if signalType == types.SignalTypeBuy {
		s.trailingStop = price * (1 - s.stopLossPercent)
	} else {
		s.trailingStop = price * (1 + s.stopLossPercent)
	}
	s.positionMutex.Unlock()

	logger.Info("TrendFollowingStrategy生成信号",
		zap.String("symbol", symbol),
		zap.String("type", string(signalType)),
		zap.Float64("price", price),
		zap.Float64("adx", adx),
		zap.String("state", s.stateToString()),
	)

	return &types.Signal{
		Strategy:  s.name,
		Symbol:    symbol,
		Type:      signalType,
		Price:     price,
		Quantity:  1.0,
		Timestamp: time.Now(),
		Metadata: map[string]interface{}{
			"adx":           adx,
			"ema_short":     s.emaShort,
			"ema_long":      s.emaLong,
			"state":         s.stateToString(),
			"trailing_stop": s.trailingStop,
		},
	}, nil
}

func (s *TrendFollowingStrategy) checkExitSignal(symbol string, price float64) (*types.Signal, error) {
	s.positionMutex.RLock()
	position := s.position
	s.positionMutex.RUnlock()

	if position == nil {
		return nil, nil
	}

	shouldExit := false
	exitReason := ""

	if position.Side == types.OrderSideBuy {
		if price > s.lastEntryPrice {
			newTrailingStop := price * (1 - s.trailingStopPercent)
			if newTrailingStop > s.trailingStop {
				s.trailingStop = newTrailingStop
			}
		}
		if price <= s.trailingStop {
			shouldExit = true
			exitReason = "stop_loss"
		}
		if s.state == TrendStateDowntrend {
			shouldExit = true
			exitReason = "trend_reversal"
		}
	}

	if position.Side == types.OrderSideSell {
		if price < s.lastEntryPrice {
			newTrailingStop := price * (1 + s.trailingStopPercent)
			if newTrailingStop < s.trailingStop {
				s.trailingStop = newTrailingStop
			}
		}
		if price >= s.trailingStop {
			shouldExit = true
			exitReason = "stop_loss"
		}
		if s.state == TrendStateUptrend {
			shouldExit = true
			exitReason = "trend_reversal"
		}
	}

	if shouldExit {
		logger.Info("TrendFollowingStrategy平仓信号",
			zap.String("symbol", symbol),
			zap.String("side", string(position.Side)),
			zap.Float64("entry_price", position.EntryPrice),
			zap.Float64("exit_price", price),
			zap.String("reason", exitReason),
		)

		return &types.Signal{
			Strategy:  s.name,
			Symbol:    symbol,
			Type:      types.SignalTypeExit,
			Price:     price,
			Quantity:  position.Size,
			Timestamp: time.Now(),
			Metadata: map[string]interface{}{
				"reason": exitReason,
				"state":  s.stateToString(),
			},
		}, nil
	}

	return nil, nil
}

func (s *TrendFollowingStrategy) stateToString() string {
	switch s.state {
	case TrendStateUptrend:
		return "uptrend"
	case TrendStateDowntrend:
		return "downtrend"
	default:
		return "neutral"
	}
}

func (s *TrendFollowingStrategy) GetParams() map[string]interface{} {
	return s.params
}

func (s *TrendFollowingStrategy) SetParams(params map[string]interface{}) {
	for k, v := range params {
		s.params[k] = v
	}

	// 热更新：参数变化后立即重新计算指标，避免等待下一个 Bar
	s.updateIndicators()
}

func (s *TrendFollowingStrategy) GetMetrics() map[string]interface{} {
	s.metricsMutex.Lock()
	defer s.metricsMutex.Unlock()

	s.metrics["state"] = s.stateToString()
	s.metrics["ema_short"] = s.emaShort
	s.metrics["ema_long"] = s.emaLong
	s.metrics["adx"] = s.adx
	s.metrics["plus_di"] = s.plusDI
	s.metrics["minus_di"] = s.minusDI
	s.metrics["trade_count"] = s.tradeCount
	s.metrics["total_pnl"] = s.totalPnL
	s.metrics["trailing_stop"] = s.trailingStop

	return s.metrics
}

func (s *TrendFollowingStrategy) OnPositionFilled(symbol string, side types.OrderSide, entryPrice, size float64) {
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

	logger.Info("TrendFollowingStrategy持仓已填充",
		zap.String("symbol", symbol),
		zap.String("side", string(side)),
		zap.Float64("entry_price", entryPrice),
		zap.Float64("size", size),
	)
}

func (s *TrendFollowingStrategy) OnPositionReduced(symbol string, exitPrice, pnl, remainingSize float64) {
	logger.Info("TrendFollowingStrategy持仓部分减仓",
		zap.String("symbol", symbol),
		zap.Float64("exit_price", exitPrice),
		zap.Float64("pnl", pnl),
		zap.Float64("remaining_size", remainingSize),
	)
}

func (s *TrendFollowingStrategy) OnPositionClosed(symbol string, exitPrice, pnl float64) {
	s.positionMutex.Lock()
	s.position = nil
	s.totalPnL += pnl
	s.positionMutex.Unlock()

	logger.Info("TrendFollowingStrategy持仓已平仓",
		zap.String("symbol", symbol),
		zap.Float64("exit_price", exitPrice),
		zap.Float64("pnl", pnl),
		zap.Float64("total_pnl", s.totalPnL),
	)
}

func (s *TrendFollowingStrategy) ConfirmRebalanceEntry(request *RebalanceRequest) (*RebalanceDecision, error) {
	return &RebalanceDecision{RejectReason: "rebalance_entry_unsupported"}, nil
}

func (s *TrendFollowingStrategy) SetSmartFilter(filter *SmartFilter) {
	s.smartFilter = filter
}

func (s *TrendFollowingStrategy) UpdateOnChainData(netflow, sopr, mvrv float64) {
	logger.Debug("TrendFollowingStrategy更新链上数据",
		zap.Float64("netflow", netflow),
		zap.Float64("sopr", sopr),
		zap.Float64("mvrv", mvrv),
	)
}

func (s *TrendFollowingStrategy) GetParamSchema() ParamSchema {
	return ParamSchema{
		StrategyName: s.name,
		Params: []ParamDefinition{
			{
				Name:         "ema_short_period",
				Type:         ParamTypeInt,
				DefaultValue: DefaultTrendEMAShortPeriod,
				MinValue:     2,
				MaxValue:     100,
				Required:     false,
				Description:  "短期EMA周期",
			},
			{
				Name:         "ema_long_period",
				Type:         ParamTypeInt,
				DefaultValue: DefaultTrendEMALongPeriod,
				MinValue:     5,
				MaxValue:     200,
				Required:     false,
				Description:  "长期EMA周期",
			},
			{
				Name:         "adx_period",
				Type:         ParamTypeInt,
				DefaultValue: DefaultTrendADXPeriod,
				MinValue:     2,
				MaxValue:     50,
				Required:     false,
				Description:  "ADX周期",
			},
			{
				Name:         "adx_threshold",
				Type:         ParamTypeFloat,
				DefaultValue: DefaultTrendADXThreshold,
				MinValue:     0.0,
				MaxValue:     100.0,
				Required:     false,
				Description:  "ADX趋势强度阈值",
			},
			{
				Name:         "trend_strength",
				Type:         ParamTypeFloat,
				DefaultValue: DefaultTrendStrength,
				MinValue:     0.001,
				MaxValue:     0.5,
				Required:     false,
				Description:  "趋势强度阈值",
			},
			{
				Name:         "stop_loss_percent",
				Type:         ParamTypeFloat,
				DefaultValue: DefaultTrendStopLossPercent,
				MinValue:     0.001,
				MaxValue:     0.5,
				Required:     false,
				Description:  "止损百分比",
			},
			{
				Name:         "trailing_stop_percent",
				Type:         ParamTypeFloat,
				DefaultValue: DefaultTrendTrailingStopPercent,
				MinValue:     0.001,
				MaxValue:     0.5,
				Required:     false,
				Description:  "移动止损百分比",
			},
			{
				Name:         "signal_cooldown",
				Type:         ParamTypeInt,
				DefaultValue: int(DefaultTrendSignalCooldown),
				MinValue:     1,
				MaxValue:     86400,
				Required:     false,
				Description:  "信号冷却时间（秒）",
			},
		},
	}
}

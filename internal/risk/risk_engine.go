package risk

import (
	"sync"
	"time"

	"github.com/ljwqf/quant/internal/config"
	"github.com/ljwqf/quant/pkg/logger"
	"github.com/ljwqf/quant/pkg/types"
	"go.uber.org/zap"
)

// LiquidityChecker 流动性检查器接口
type LiquidityChecker interface {
	GetOrderBook(symbol string, depth int) (*types.OrderBook, error)
}

type Engine struct {
	config          *config.RiskConfig
	dailyLoss       float64
	dailyTrades     int
	positions       map[string]*types.Position
	lastResetTime   time.Time
	strategyWeights map[string]float64
	metrics         map[string]interface{}
	mutex           sync.RWMutex
	stopChan        chan struct{}
	stopOnce        sync.Once
	nowFunc         func() time.Time
	// 流动性检查相关字段
	liquidityChecker LiquidityChecker
	maxSlippage      float64
	orderBookDepth   int
	// 品种风险敞口跟踪
	symbolExposures map[string]float64
}

// EngineOption 引擎配置选项
type EngineOption func(*Engine)

// WithLiquidityChecker 设置流动性检查器
func WithLiquidityChecker(checker LiquidityChecker, maxSlippage float64, depth int) EngineOption {
	return func(e *Engine) {
		e.liquidityChecker = checker
		e.maxSlippage = maxSlippage
		e.orderBookDepth = depth
	}
}

func NewEngine(cfg *config.RiskConfig, opts ...EngineOption) *Engine {
	e := &Engine{
		config:        cfg,
		dailyLoss:     0,
		dailyTrades:   0,
		positions:     make(map[string]*types.Position),
		lastResetTime: time.Now(),
		strategyWeights: map[string]float64{
			"LiquidityHuntEngine":        0.10,
			"BetaArbitrageEngine":        0.08,
			"MMPEngine-Pro":              0.10,
			"DeltaNeutralFunding-Pro":    0.25,
			"NeedleStrategy":             0.12,
			"TrendFollowingStrategy":     0.15,
			"MeanReversionStrategy":      0.12,
			"VolatilityBreakoutStrategy": 0.08,
		},
		metrics:         make(map[string]interface{}),
		stopChan:        make(chan struct{}),
		nowFunc:         time.Now,
		maxSlippage:     0.0025, // 默认 0.25%
		orderBookDepth:  20,    // 默认深度
		symbolExposures: make(map[string]float64),
	}

	for _, opt := range opts {
		opt(e)
	}

	return e
}

func (e *Engine) CheckRisk(signal *types.Signal) error {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	e.checkDailyResetLocked()

	if signal == nil {
		return ErrInvalidSignal
	}

	if signal.Type == types.SignalTypeExit {
		return nil
	}

	if e.dailyLoss >= e.config.MaxDailyLoss {
		return ErrDailyLossExceeded
	}

	if e.dailyTrades >= e.config.MaxTradesPerDay {
		return ErrTradeLimitExceeded
	}

	if err := e.checkPositionLimitLocked(signal); err != nil {
		return err
	}

	if err := e.checkSymbolExposureLocked(signal); err != nil {
		return err
	}

	if err := e.checkLiquidityLocked(signal); err != nil {
		return err
	}

	if err := e.checkTimeFuseLocked(); err != nil {
		return err
	}

	return nil
}

func (e *Engine) UpdatePosition(position *types.Position) {
	if position == nil {
		return
	}

	e.mutex.Lock()
	defer e.mutex.Unlock()

	if position.Size == 0 {
		delete(e.positions, position.Symbol)
		delete(e.symbolExposures, position.Symbol)
		return
	}

	e.positions[position.Symbol] = position
	e.symbolExposures[position.Symbol] = absExposure(position.Size, position.MarkPrice, position.Leverage)
}

func (e *Engine) RemovePosition(symbol string) {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	delete(e.positions, symbol)
}

func (e *Engine) UpdatePnL(symbol string, pnl float64) {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	// 只累加亏损，盈利时不减少 dailyLoss
	if pnl < 0 {
		e.dailyLoss += -pnl
	}
}

func (e *Engine) IncrementTrade() {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	e.checkDailyResetLocked()
	e.dailyTrades++
}

func (e *Engine) GetRiskMetrics() map[string]interface{} {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	metrics := make(map[string]interface{})
	metrics["daily_loss"] = e.dailyLoss
	metrics["max_daily_loss"] = e.config.MaxDailyLoss
	metrics["daily_trades"] = e.dailyTrades
	metrics["max_trades_per_day"] = e.config.MaxTradesPerDay
	metrics["position_count"] = len(e.positions)

	totalExposure := 0.0
	for _, pos := range e.positions {
		exposure := absExposure(pos.Size, pos.MarkPrice, pos.Leverage)
		totalExposure += exposure
	}
	metrics["total_exposure"] = totalExposure

	symbolExposures := make(map[string]float64)
	for symbol, exposure := range e.symbolExposures {
		symbolExposures[symbol] = exposure
	}
	metrics["symbol_exposures"] = symbolExposures

	if e.config.SymbolExposureLimit.Enable {
		metrics["symbol_exposure_limit_enabled"] = true
		metrics["max_per_symbol"] = e.config.SymbolExposureLimit.MaxPerSymbol
		metrics["max_total_exposure"] = e.config.SymbolExposureLimit.MaxTotalExposure
	}

	return metrics
}

func (e *Engine) GetPositionSize(signal *types.Signal, accountBalance float64) float64 {
	if signal == nil || accountBalance <= 0 {
		return 0
	}

	e.mutex.RLock()
	defer e.mutex.RUnlock()

	totalRiskBudget := 0.02 * accountBalance

	weight := e.strategyWeights[signal.Strategy]
	if weight == 0 {
		weight = 0.25
	}

	strategyRiskBudget := totalRiskBudget * weight

	if signal.Price > 0 {
		return strategyRiskBudget / signal.Price
	}

	return 0
}

func (e *Engine) checkDailyResetLocked() {
	if e.nowFunc().Sub(e.lastResetTime) > 24*time.Hour {
		e.dailyLoss = 0
		e.dailyTrades = 0
		e.lastResetTime = e.nowFunc()
		logger.Info("每日风控数据重置",
			zap.Time("reset_time", e.lastResetTime),
		)
	}
}

func (e *Engine) checkPositionLimitLocked(signal *types.Signal) error {
	if signal == nil || signal.Type == types.SignalTypeExit {
		return nil
	}

	currentValue := 0.0
	for _, pos := range e.positions {
		currentValue += pos.Size * pos.MarkPrice
	}

	newValue := 0.0
	if signal.Price > 0 && signal.Quantity > 0 {
		newValue = signal.Quantity * signal.Price
	}

	if currentValue+newValue > e.config.MaxPositionSize {
		return ErrPositionLimitExceeded
	}

	return nil
}

func (e *Engine) checkSymbolExposureLocked(signal *types.Signal) error {
	if signal == nil || signal.Type == types.SignalTypeExit {
		return nil
	}

	if !e.config.SymbolExposureLimit.Enable {
		return nil
	}

	signalExposure := 0.0
	if signal.Price > 0 && signal.Quantity > 0 {
		signalExposure = signal.Quantity * signal.Price
	}

	if signalExposure <= 0 {
		return nil
	}

	currentExposure := e.symbolExposures[signal.Symbol]
	newExposure := currentExposure + signalExposure

	maxExposure := e.config.SymbolExposureLimit.MaxPerSymbol
	if customLimit, exists := e.config.SymbolExposureLimit.SymbolLimits[signal.Symbol]; exists {
		maxExposure = customLimit
	}

	if maxExposure > 0 && newExposure > maxExposure {
		logger.Warn("品种风险敞口超限",
			zap.String("symbol", signal.Symbol),
			zap.Float64("current", currentExposure),
			zap.Float64("new", signalExposure),
			zap.Float64("total", newExposure),
			zap.Float64("max", maxExposure),
		)
		return ErrSymbolExposureExceeded
	}

	if e.config.SymbolExposureLimit.MaxTotalExposure > 0 {
		totalExposure := 0.0
		for _, exp := range e.symbolExposures {
			totalExposure += exp
		}
		newTotalExposure := totalExposure + signalExposure

		if newTotalExposure > e.config.SymbolExposureLimit.MaxTotalExposure {
			logger.Warn("总风险敞口超限",
				zap.Float64("current", totalExposure),
				zap.Float64("new", signalExposure),
				zap.Float64("total", newTotalExposure),
				zap.Float64("max", e.config.SymbolExposureLimit.MaxTotalExposure),
			)
			return ErrTotalExposureExceeded
		}
	}

	return nil
}

func (e *Engine) checkLiquidityLocked(signal *types.Signal) error {
	// 退出信号不需要检查流动性
	if signal == nil || signal.Type == types.SignalTypeExit {
		return nil
	}

	// 未配置流动性检查器时跳过
	if e.liquidityChecker == nil {
		return nil
	}

	// 无数量时不检查
	if signal.Quantity <= 0 {
		return nil
	}

	// 获取订单簿
	orderBook, err := e.liquidityChecker.GetOrderBook(signal.Symbol, e.orderBookDepth)
	if err != nil {
		logger.Warn("获取订单簿失败，跳过流动性检查",
			zap.String("symbol", signal.Symbol),
			zap.Error(err),
		)
		return nil // 容错通过
	}

	if orderBook == nil {
		return nil
	}

	// 确定使用订单簿的哪一侧
	var levels []types.OrderBookLevel
	var bestPrice float64
	if signal.Type == types.SignalTypeBuy {
		levels = orderBook.Asks
		if len(levels) > 0 {
			bestPrice = levels[0].Price
		}
	} else {
		levels = orderBook.Bids
		if len(levels) > 0 {
			bestPrice = levels[0].Price
		}
	}

	if len(levels) == 0 || bestPrice <= 0 {
		return nil
	}

	// 计算可用流动性和预估成交价格
	remaining := signal.Quantity
	totalValue := 0.0
	availableQty := 0.0

	for _, level := range levels {
		if level.Price <= 0 || level.Size <= 0 {
			continue
		}
		fillQty := remaining
		if fillQty > level.Size {
			fillQty = level.Size
		}
		totalValue += fillQty * level.Price
		availableQty += level.Size
		remaining -= fillQty
		if remaining <= 0 {
			break
		}
	}

	// 流动性不足检查
	if remaining > 0 {
		logger.Warn("流动性不足",
			zap.String("symbol", signal.Symbol),
			zap.Float64("required", signal.Quantity),
			zap.Float64("available", availableQty),
		)
		return ErrLiquidityInsufficient
	}

	// 计算预估滑点
	if signal.Quantity <= 0 || totalValue <= 0 {
		return nil
	}
	avgPrice := totalValue / signal.Quantity

	var slippage float64
	if signal.Type == types.SignalTypeBuy {
		slippage = (avgPrice - bestPrice) / bestPrice
	} else {
		slippage = (bestPrice - avgPrice) / bestPrice
	}

	if slippage > e.maxSlippage {
		logger.Warn("预估滑点超过阈值",
			zap.String("symbol", signal.Symbol),
			zap.Float64("slippage", slippage),
			zap.Float64("max_slippage", e.maxSlippage),
		)
		return ErrPriceDeviationTooHigh
	}

	return nil
}

func (e *Engine) checkTimeFuseLocked() error {
	return e.checkTimeFuseAt(e.nowFunc())
}

func (e *Engine) checkTimeFuseAt(now time.Time) error {
	current := now.Format("15:04")

	timeFuseWindows := []struct {
		start string
		end   string
		name  string
	}{
		{"00:55", "01:05", "结算时段"},
		{"07:55", "08:05", "结算时段"},
		{"15:55", "16:05", "结算时段"},
		{"23:55", "00:05", "结算时段"},
	}

	for _, window := range timeFuseWindows {
		if isTimeInWindow(current, window.start, window.end) {
			logger.Warn("触发时间熔断，禁止新开仓",
				zap.String("current_time", current),
				zap.String("window_name", window.name),
				zap.String("window_start", window.start),
				zap.String("window_end", window.end),
			)
			return ErrMarketClosed
		}
	}

	return nil
}

func isTimeInWindow(current, start, end string) bool {
	if start <= end {
		return current >= start && current <= end
	}
	return current >= start || current <= end
}

func (e *Engine) GetDailyLoss() float64 {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	return e.dailyLoss
}

func (e *Engine) GetDailyTrades() int {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	return e.dailyTrades
}

func (e *Engine) GetStrategyWeights() map[string]float64 {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	weights := make(map[string]float64)
	for k, v := range e.strategyWeights {
		weights[k] = v
	}
	return weights
}

func (e *Engine) SetStrategyWeights(weights map[string]float64) {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	for k, v := range weights {
		e.strategyWeights[k] = v
	}
}

func (e *Engine) GetPositions() map[string]*types.Position {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	positions := make(map[string]*types.Position)
	for k, v := range e.positions {
		positions[k] = v
	}
	return positions
}

func (e *Engine) GetPosition(symbol string) *types.Position {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	return e.positions[symbol]
}

func (e *Engine) ResetDailyMetrics() {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	e.dailyLoss = 0
	e.dailyTrades = 0
	e.lastResetTime = time.Now()
}

func (e *Engine) Stop() {
	e.stopOnce.Do(func() {
		close(e.stopChan)
		logger.Info("风控引擎已停止")
	})
}

func (e *Engine) IsCircuitBreakerTriggered() bool {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	return e.dailyLoss >= e.config.MaxDailyLoss || e.dailyTrades >= e.config.MaxTradesPerDay
}

func (e *Engine) GetAvailableRiskBudget(accountBalance float64) float64 {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	if accountBalance <= 0 {
		return 0
	}

	remainingLossBudget := e.config.MaxDailyLoss - e.dailyLoss
	if remainingLossBudget < 0 {
		return 0
	}

	if remainingLossBudget > accountBalance {
		return accountBalance
	}

	return remainingLossBudget
}

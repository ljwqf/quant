package strategy

import (
	"math"
	"time"

	"github.com/ljwqf/quant/pkg/types"
)

// BetaArbitrageEngine 山寨贝塔套利引擎
type BetaArbitrageEngine struct {
	name             string
	params           map[string]interface{}
	priceHistory     map[string][]float64 // 价格历史，按币种存储
	btcPriceHistory  []float64            // BTC价格历史
	correlationData  map[string][]float64 // 相关性数据
	lastFundingCheck time.Time            // 上次资金费率检查时间
	positions        map[string]time.Time // 当前持仓，记录开仓时间
	metrics          map[string]interface{}
}

// NewBetaArbitrageEngine 创建山寨贝塔套利引擎
func NewBetaArbitrageEngine() *BetaArbitrageEngine {
	return &BetaArbitrageEngine{
		name:             "BetaArbitrageEngine",
		params:           make(map[string]interface{}),
		priceHistory:     make(map[string][]float64),
		btcPriceHistory:  make([]float64, 0),
		correlationData:  make(map[string][]float64),
		lastFundingCheck: time.Now(),
		positions:        make(map[string]time.Time),
		metrics:          make(map[string]interface{}),
	}
}

// Name 返回策略名称
func (e *BetaArbitrageEngine) Name() string {
	return e.name
}

// Init 初始化策略
func (e *BetaArbitrageEngine) Init(params map[string]interface{}) error {
	e.params = params

	// 设置默认参数
	if _, ok := e.params["benchmark"]; !ok {
		e.params["benchmark"] = "BTCUSDT"
	}

	if _, ok := e.params["correlation_period"]; !ok {
		e.params["correlation_period"] = 24 // 24小时
	}

	if _, ok := e.params["beta_threshold"]; !ok {
		e.params["beta_threshold"] = 1.5
	}

	if _, ok := e.params["rsi_period"]; !ok {
		e.params["rsi_period"] = 14
	}

	if _, ok := e.params["rsi_threshold"]; !ok {
		e.params["rsi_threshold"] = 75.0
	}

	if _, ok := e.params["funding_check_interval"]; !ok {
		e.params["funding_check_interval"] = 8 // 8小时
	}

	if _, ok := e.params["max_holding_time"]; !ok {
		e.params["max_holding_time"] = 2 // 2小时
	}

	if _, ok := e.params["trailing_activation"]; !ok {
		e.params["trailing_activation"] = 1.0 // 1R
	}

	// 初始化指标
	e.metrics["total_signals"] = 0
	e.metrics["win_rate"] = 0.0
	e.metrics["total_pnl"] = 0.0

	return nil
}

// OnTick 处理行情快照
func (e *BetaArbitrageEngine) OnTick(tick *types.Tick) (*types.Signal, error) {
	if tick == nil {
		return nil, nil
	}

	benchmarkSymbol := normalizeMarketSymbol(getString(e.params, "benchmark", "BTC-USDT"))
	tickSymbol := normalizeMarketSymbol(tick.Symbol)
	rsiPeriod := e.paramInt("rsi_period", 14)
	rsiThreshold := e.paramFloat("rsi_threshold", 75.0)
	betaThreshold := e.paramFloat("beta_threshold", 1.5)

	// 更新价格历史
	e.updatePriceHistory(tick)

	// 检查是否是BTC
	if tickSymbol == benchmarkSymbol {
		e.btcPriceHistory = append(e.btcPriceHistory, tick.Price)
		// 限制历史数据长度
		maxHistory := 24 * 60 // 24小时，每分钟一个数据点
		if len(e.btcPriceHistory) > maxHistory {
			e.btcPriceHistory = e.btcPriceHistory[len(e.btcPriceHistory)-maxHistory:]
		}
		return nil, nil
	}

	// 检查是否需要检查资金费率
	e.checkFundingRate()

	// 检查是否需要平仓（超过最大持有时间）
	signal := e.checkPositionTimeout(tick.Symbol)
	if signal != nil {
		return signal, nil
	}

	if _, exists := e.positions[tickSymbol]; exists {
		return nil, nil
	}

	// 计算BTC的RSI
	btcRsi := e.calculateRSI(e.btcPriceHistory, rsiPeriod)
	if btcRsi >= rsiThreshold {
		return nil, nil // BTC RSI过高，不激活扫描
	}

	// 计算BTC的收益率
	btcReturn := e.calculateReturn(e.btcPriceHistory, 60) // 1小时收益率
	if btcReturn <= 0.01 {                                // 比特币涨幅小于1%，不激活扫描
		return nil, nil
	}

	// 计算相对强度
	altReturn := e.calculateReturn(e.priceHistory[tickSymbol], 60) // 1小时收益率
	if altReturn <= 0 {
		return nil, nil // 山寨币下跌，不考虑
	}

	beta := altReturn / btcReturn

	// 检查beta是否超过阈值
	if beta < betaThreshold {
		return nil, nil
	}

	// 计算相关性
	correlation := e.calculateCorrelation(tickSymbol)
	if correlation < 0.8 {
		return nil, nil // 相关性不足
	}

	// 检查流动性
	if !e.checkLiquidity(tick.Symbol) {
		return nil, nil // 流动性不足
	}

	// 生成买入信号
	signal = &types.Signal{
		Strategy:   e.name,
		Symbol:     tick.Symbol,
		Type:       types.SignalTypeBuy,
		Price:      tick.Price,
		Timestamp:  time.Now(),
		Confidence: 0.7,
		Metadata: map[string]interface{}{
			"beta":                beta,
			"correlation":         correlation,
			"btc_return":          btcReturn,
			"alt_return":          altReturn,
			"btc_rsi":             btcRsi,
			"max_holding_time":    e.params["max_holding_time"],
			"trailing_activation": e.params["trailing_activation"],
		},
	}

	// 更新指标
	totalSignals := getInt(e.metrics, "total_signals", 0)
	e.metrics["total_signals"] = totalSignals + 1

	return signal, nil
}

// OnBar 处理K线数据
func (e *BetaArbitrageEngine) OnBar(bar *types.Bar) (*types.Signal, error) {
	symbolKey := normalizeMarketSymbol(bar.Symbol)
	benchmarkSymbol := normalizeMarketSymbol(getString(e.params, "benchmark", "BTC-USDT"))

	// 更新价格历史
	e.priceHistory[symbolKey] = append(e.priceHistory[symbolKey], bar.Close)

	// 限制历史数据长度
	maxHistory := 24 * 60 // 24小时，每分钟一个数据点
	if len(e.priceHistory[symbolKey]) > maxHistory {
		e.priceHistory[symbolKey] = e.priceHistory[symbolKey][len(e.priceHistory[symbolKey])-maxHistory:]
	}

	// 如果是BTC，更新BTC价格历史
	if symbolKey == benchmarkSymbol {
		e.btcPriceHistory = append(e.btcPriceHistory, bar.Close)
		if len(e.btcPriceHistory) > maxHistory {
			e.btcPriceHistory = e.btcPriceHistory[len(e.btcPriceHistory)-maxHistory:]
		}
	}

	return nil, nil
}

// OnOrderBook 处理订单簿数据
func (e *BetaArbitrageEngine) OnOrderBook(orderBook *types.OrderBook) (*types.Signal, error) {
	// 流动性检查
	// 这里可以实现更详细的流动性检查逻辑
	return nil, nil
}

// GetParams 获取策略参数
func (e *BetaArbitrageEngine) GetParams() map[string]interface{} {
	return e.params
}

// SetParams 设置策略参数
func (e *BetaArbitrageEngine) SetParams(params map[string]interface{}) {
	for k, v := range params {
		e.params[k] = v
	}
}

// GetMetrics 获取策略指标
func (e *BetaArbitrageEngine) GetMetrics() map[string]interface{} {
	return e.metrics
}

// updatePriceHistory 更新价格历史
func (e *BetaArbitrageEngine) updatePriceHistory(tick *types.Tick) {
	symbolKey := normalizeMarketSymbol(tick.Symbol)
	e.priceHistory[symbolKey] = append(e.priceHistory[symbolKey], tick.Price)

	// 限制历史数据长度
	maxHistory := 24 * 60 // 24小时，每分钟一个数据点
	if len(e.priceHistory[symbolKey]) > maxHistory {
		e.priceHistory[symbolKey] = e.priceHistory[symbolKey][len(e.priceHistory[symbolKey])-maxHistory:]
	}
}

// calculateRSI 计算RSI
func (e *BetaArbitrageEngine) calculateRSI(prices []float64, period int) float64 {
	if len(prices) < period+1 {
		return 50.0 // 默认值
	}

	var gains, losses float64
	for i := 1; i <= period; i++ {
		change := prices[len(prices)-i] - prices[len(prices)-i-1]
		if change > 0 {
			gains += change
		} else {
			losses += math.Abs(change)
		}
	}

	if losses == 0 {
		return 100.0
	}

	rs := gains / losses
	rsi := 100.0 - (100.0 / (1.0 + rs))
	return rsi
}

// calculateReturn 计算收益率
func (e *BetaArbitrageEngine) calculateReturn(prices []float64, period int) float64 {
	if len(prices) < period+1 {
		return 0.0
	}

	return (prices[len(prices)-1] - prices[len(prices)-period-1]) / prices[len(prices)-period-1]
}

// calculateCorrelation 计算相关性
func (e *BetaArbitrageEngine) calculateCorrelation(symbol string) float64 {
	altPrices := e.priceHistory[normalizeMarketSymbol(symbol)]
	btcPrices := e.btcPriceHistory

	minLen := len(altPrices)
	if len(btcPrices) < minLen {
		minLen = len(btcPrices)
	}

	if minLen < 2 {
		return 0.0
	}

	// 取最近的数据点
	altPrices = altPrices[len(altPrices)-minLen:]
	btcPrices = btcPrices[len(btcPrices)-minLen:]

	// 计算均值
	var altSum, btcSum float64
	for i := 0; i < minLen; i++ {
		altSum += altPrices[i]
		btcSum += btcPrices[i]
	}
	altMean := altSum / float64(minLen)
	btcMean := btcSum / float64(minLen)

	// 计算协方差和标准差
	var covariance, altVariance, btcVariance float64
	for i := 0; i < minLen; i++ {
		altDiff := altPrices[i] - altMean
		btcDiff := btcPrices[i] - btcMean
		covariance += altDiff * btcDiff
		altVariance += altDiff * altDiff
		btcVariance += btcDiff * btcDiff
	}

	if altVariance == 0 || btcVariance == 0 {
		return 0.0
	}

	correlation := covariance / (math.Sqrt(altVariance) * math.Sqrt(btcVariance))
	return correlation
}

// checkLiquidity 检查流动性
func (e *BetaArbitrageEngine) checkLiquidity(symbol string) bool {
	// 这里可以实现更详细的流动性检查逻辑
	// 例如检查订单簿深度、滑点等
	return true // 默认返回true
}

// checkFundingRate 检查资金费率
func (e *BetaArbitrageEngine) checkFundingRate() {
	fundingCheckInterval := e.paramInt("funding_check_interval", 8)
	if time.Since(e.lastFundingCheck) >= time.Duration(fundingCheckInterval)*time.Hour {
		// 这里可以实现资金费率检查逻辑
		e.lastFundingCheck = time.Now()
	}
}

// checkPositionTimeout 检查持仓是否超时
func (e *BetaArbitrageEngine) checkPositionTimeout(symbol string) *types.Signal {
	key := normalizeMarketSymbol(symbol)
	openTime, ok := e.positions[key]
	if !ok {
		return nil
	}

	maxHoldingTime := e.paramInt("max_holding_time", 2)
	if time.Since(openTime) >= time.Duration(maxHoldingTime)*time.Hour {
		// 生成平仓信号
		signal := &types.Signal{
			Strategy:   e.name,
			Symbol:     symbol,
			Type:       types.SignalTypeExit,
			Timestamp:  time.Now(),
			Confidence: 0.9,
			Metadata: map[string]interface{}{
				"reason": "position_timeout",
			},
		}

		// 移除持仓记录
		delete(e.positions, key)

		return signal
	}

	return nil
}

func (e *BetaArbitrageEngine) paramInt(name string, defaultValue int) int {
	value, ok := e.params[name]
	if !ok {
		return defaultValue
	}

	switch typed := value.(type) {
	case int:
		return typed
	case int8:
		return int(typed)
	case int16:
		return int(typed)
	case int32:
		return int(typed)
	case int64:
		return int(typed)
	case float32:
		return int(typed)
	case float64:
		return int(typed)
	default:
		return defaultValue
	}
}

func (e *BetaArbitrageEngine) paramFloat(name string, defaultValue float64) float64 {
	value, ok := e.params[name]
	if !ok {
		return defaultValue
	}

	switch typed := value.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case int8:
		return float64(typed)
	case int16:
		return float64(typed)
	case int32:
		return float64(typed)
	case int64:
		return float64(typed)
	default:
		return defaultValue
	}
}

func (e *BetaArbitrageEngine) OnPositionFilled(symbol string, side types.OrderSide, entryPrice, size float64) {
	if size <= 0 {
		return
	}
	e.positions[normalizeMarketSymbol(symbol)] = time.Now()
}

func (e *BetaArbitrageEngine) OnPositionReduced(symbol string, exitPrice, pnl, remainingSize float64) {
}

func (e *BetaArbitrageEngine) OnPositionClosed(symbol string, exitPrice, pnl float64) {
	delete(e.positions, normalizeMarketSymbol(symbol))
}

func (e *BetaArbitrageEngine) ConfirmRebalanceEntry(request *RebalanceRequest) (*RebalanceDecision, error) {
	return &RebalanceDecision{RejectReason: "rebalance_entry_unsupported"}, nil
}

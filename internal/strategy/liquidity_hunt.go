package strategy

import (
	"math"
	"sync"
	"time"

	"github.com/ljwqf/quant/pkg/types"
)

// LiquidityHuntEngine BTC/ETH机构博弈引擎
type LiquidityHuntEngine struct {
	mu               sync.Mutex
	name             string
	params           map[string]interface{}
	keyLevels        map[string][]float64 // 关键位，按币种存储
	priceHistory     map[string][]float64 // 价格历史，用于计算关键位
	volumeHistory    map[string][]float64 // 成交量历史，用于计算VWAP
	state            map[string]int       // 假突破状态机状态
	breakoutPrice    map[string]float64   // 突破价格
	confirmationTime map[string]time.Time // 确认时间
	metrics          map[string]interface{}
}

// NewLiquidityHuntEngine 创建BTC/ETH机构博弈引擎
func NewLiquidityHuntEngine() *LiquidityHuntEngine {
	return &LiquidityHuntEngine{
		name:             "LiquidityHuntEngine",
		params:           make(map[string]interface{}),
		keyLevels:        make(map[string][]float64),
		priceHistory:     make(map[string][]float64),
		volumeHistory:    make(map[string][]float64),
		state:            make(map[string]int),
		breakoutPrice:    make(map[string]float64),
		confirmationTime: make(map[string]time.Time),
		metrics:          make(map[string]interface{}),
	}
}

// Name 返回策略名称
func (e *LiquidityHuntEngine) Name() string {
	return e.name
}

// Init 初始化策略
func (e *LiquidityHuntEngine) Init(params map[string]interface{}) error {
	e.params = params

	// 设置默认参数
	if _, ok := e.params["fake_break_threshold"]; !ok {
		e.params["fake_break_threshold"] = 0.3 // 0.3%
	}

	if _, ok := e.params["funding_rate_threshold"]; !ok {
		e.params["funding_rate_threshold"] = 0.0005 // 0.05%
	}

	if _, ok := e.params["time_window"]; !ok {
		e.params["time_window"] = []string{"20:30", "23:00"} // 北京时间
	}

	if _, ok := e.params["oi_delta_threshold"]; !ok {
		e.params["oi_delta_threshold"] = 50.0 // 50BTC
	}

	// 初始化指标
	e.metrics["total_signals"] = 0
	e.metrics["win_rate"] = 0.0
	e.metrics["total_pnl"] = 0.0

	return nil
}

// OnTick 处理行情快照
func (e *LiquidityHuntEngine) OnTick(tick *types.Tick) (*types.Signal, error) {
	// 检查是否在时间窗口内
	if !e.isInTimeWindow() {
		return nil, nil
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	// 更新价格历史
	e.updatePriceHistory(tick)

	// 计算关键位
	e.calculateKeyLevels(tick.Symbol)

	// 检查假突破
	return e.checkFakeBreakout(tick)
}

// OnBar 处理K线数据
func (e *LiquidityHuntEngine) OnBar(bar *types.Bar) (*types.Signal, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// 更新价格和成交量历史
	e.priceHistory[bar.Symbol] = append(e.priceHistory[bar.Symbol], bar.Close)
	e.volumeHistory[bar.Symbol] = append(e.volumeHistory[bar.Symbol], bar.Volume)

	// 限制历史数据长度
	maxHistory := 200
	if len(e.priceHistory[bar.Symbol]) > maxHistory {
		e.priceHistory[bar.Symbol] = e.priceHistory[bar.Symbol][len(e.priceHistory[bar.Symbol])-maxHistory:]
	}

	if len(e.volumeHistory[bar.Symbol]) > maxHistory {
		e.volumeHistory[bar.Symbol] = e.volumeHistory[bar.Symbol][len(e.volumeHistory[bar.Symbol])-maxHistory:]
	}

	// 计算关键位
	e.calculateKeyLevels(bar.Symbol)

	return nil, nil
}

// OnOrderBook 处理订单簿数据
func (e *LiquidityHuntEngine) OnOrderBook(orderBook *types.OrderBook) (*types.Signal, error) {
	// 流动性验证
	// 检查Level 2订单簿，确认突破时是否伴随Bid/Ask Wall的突然撤单
	return nil, nil
}

// GetParams 获取策略参数
func (e *LiquidityHuntEngine) GetParams() map[string]interface{} {
	return e.params
}

// SetParams 设置策略参数
func (e *LiquidityHuntEngine) SetParams(params map[string]interface{}) {
	e.mu.Lock()
	defer e.mu.Unlock()
	for k, v := range params {
		e.params[k] = v
	}
}

// GetMetrics 获取策略指标
func (e *LiquidityHuntEngine) GetMetrics() map[string]interface{} {
	return e.metrics
}

// isInTimeWindow 检查是否在时间窗口内
func (e *LiquidityHuntEngine) isInTimeWindow() bool {
	timeWindow, ok := e.params["time_window"].([]string)
	if !ok || len(timeWindow) != 2 {
		return true // 默认返回true
	}

	now := time.Now().Format("15:04")
	return now >= timeWindow[0] && now <= timeWindow[1]
}

// updatePriceHistory 更新价格历史
func (e *LiquidityHuntEngine) updatePriceHistory(tick *types.Tick) {
	e.priceHistory[tick.Symbol] = append(e.priceHistory[tick.Symbol], tick.Price)

	// 限制历史数据长度
	maxHistory := 1000
	if len(e.priceHistory[tick.Symbol]) > maxHistory {
		e.priceHistory[tick.Symbol] = e.priceHistory[tick.Symbol][len(e.priceHistory[tick.Symbol])-maxHistory:]
	}
}

// calculateKeyLevels 计算关键位
func (e *LiquidityHuntEngine) calculateKeyLevels(symbol string) {
	prices, ok := e.priceHistory[symbol]
	if !ok || len(prices) < 20 {
		return
	}

	// 计算前高前低
	maxPrice := prices[0]
	minPrice := prices[0]
	for _, price := range prices {
		if price > maxPrice {
			maxPrice = price
		}
		if price < minPrice {
			minPrice = price
		}
	}

	// 计算VWAP
	volume, ok := e.volumeHistory[symbol]
	if !ok || len(volume) < 20 {
		return
	}

	totalVolume := 0.0
	totalValue := 0.0
	for i := 0; i < len(prices) && i < len(volume); i++ {
		totalVolume += volume[i]
		totalValue += prices[i] * volume[i]
	}

	vwap := 0.0
	if totalVolume > 0 {
		vwap = totalValue / totalVolume
	}

	// 计算VWAP标准差通道
	variance := 0.0
	for _, price := range prices {
		diff := price - vwap
		variance += diff * diff
	}

	stdDev := 0.0
	if len(prices) > 0 {
		stdDev = math.Sqrt(variance / float64(len(prices)))
	}

	upperBand := vwap + 2*stdDev
	lowerBand := vwap - 2*stdDev

	// 计算斐波那契回调位
	rangeHigh := maxPrice
	rangeLow := minPrice
	rangeDiff := rangeHigh - rangeLow

	fib618 := rangeLow + 0.618*rangeDiff
	fib786 := rangeLow + 0.786*rangeDiff

	// 存储关键位
	e.keyLevels[symbol] = []float64{
		maxPrice,  // 前高
		minPrice,  // 前低
		vwap,      // VWAP
		upperBand, // 上轨
		lowerBand, // 下轨
		fib618,    // 斐波那契0.618
		fib786,    // 斐波那契0.786
	}
}

// checkFakeBreakout 检查假突破
func (e *LiquidityHuntEngine) checkFakeBreakout(tick *types.Tick) (*types.Signal, error) {
	keyLevels, ok := e.keyLevels[tick.Symbol]
	if !ok || len(keyLevels) == 0 {
		return nil, nil
	}

	// 获取当前状态
	state, ok := e.state[tick.Symbol]
	if !ok {
		state = 0 // State 0: 观望
	}

	fakeBreakThreshold := getFloat64(e.params, "fake_break_threshold", 0.3)

	switch state {
	case 0: // 观望状态
		// 检查是否接近关键位
		for _, level := range keyLevels {
			priceDiff := math.Abs(tick.Price-level) / level * 100
			if priceDiff <= 0.2 { // 接近关键位±0.2%
				e.state[tick.Symbol] = 1 // 进入突破状态
				e.breakoutPrice[tick.Symbol] = level
				e.confirmationTime[tick.Symbol] = time.Now()
				return nil, nil
			}
		}

	case 1: // 突破状态
		breakoutPrice := e.breakoutPrice[tick.Symbol]
		priceDiff := math.Abs(tick.Price-breakoutPrice) / breakoutPrice * 100

		// 检查是否突破关键位
		if priceDiff > 0.5 { // 突破关键位±0.5%
			e.state[tick.Symbol] = 2 // 进入确认状态
			e.confirmationTime[tick.Symbol] = time.Now()
			return nil, nil
		}

	case 2: // 确认状态
		breakoutPrice := e.breakoutPrice[tick.Symbol]
		confirmationTime := e.confirmationTime[tick.Symbol]

		// 检查是否在5分钟内回到关键位内侧
		if time.Since(confirmationTime) <= 5*time.Minute {
			// 检查是否回到关键位内侧
			isBackInside := false
			if tick.Price < breakoutPrice && e.priceHistory[tick.Symbol][len(e.priceHistory[tick.Symbol])-2] > breakoutPrice {
				// 向上突破后回到下方
				isBackInside = true
			} else if tick.Price > breakoutPrice && e.priceHistory[tick.Symbol][len(e.priceHistory[tick.Symbol])-2] < breakoutPrice {
				// 向下突破后回到上方
				isBackInside = true
			}

			if isBackInside {
				// 计算收回幅度
				retracement := math.Abs(tick.Price-breakoutPrice) / math.Abs(e.priceHistory[tick.Symbol][len(e.priceHistory[tick.Symbol])-2]-breakoutPrice) * 100
				if retracement > fakeBreakThreshold {
					// 生成信号
					signal := &types.Signal{
						Strategy:   e.name,
						Symbol:     tick.Symbol,
						Timestamp:  time.Now(),
						Confidence: 0.8,
					}

					// 确定信号类型
					if tick.Price < breakoutPrice {
						// 向上假突破，做空
						signal.Type = types.SignalTypeSell
						signal.Price = breakoutPrice * 0.999 // 关键位内侧0.1%
					} else {
						// 向下假突破，做多
						signal.Type = types.SignalTypeBuy
						signal.Price = breakoutPrice * 1.001 // 关键位内侧0.1%
					}

					// 计算止损和止盈
					// 假突破极点外0.5%
					if signal.Type == types.SignalTypeSell {
						signal.Metadata = map[string]interface{}{
							"stop_loss":     breakoutPrice * 1.005,
							"take_profit_1": signal.Price * 0.985, // 1.5R
							"take_profit_2": signal.Price * 0.97,  // 3R
						}
					} else {
						signal.Metadata = map[string]interface{}{
							"stop_loss":     breakoutPrice * 0.995,
							"take_profit_1": signal.Price * 1.015, // 1.5R
							"take_profit_2": signal.Price * 1.03,  // 3R
						}
					}

					// 重置状态
					e.state[tick.Symbol] = 0

					// 更新指标
				totalSignals := getInt(e.metrics, "total_signals", 0)
				e.metrics["total_signals"] = totalSignals + 1

					return signal, nil
				}
			}
		} else {
			// 超过5分钟，重置状态
			e.state[tick.Symbol] = 0
		}
	}

	return nil, nil
}

func (e *LiquidityHuntEngine) OnPositionFilled(symbol string, side types.OrderSide, entryPrice, size float64) {
}

func (e *LiquidityHuntEngine) OnPositionReduced(symbol string, exitPrice, pnl, remainingSize float64) {
}

func (e *LiquidityHuntEngine) OnPositionClosed(symbol string, exitPrice, pnl float64) {
}

func (e *LiquidityHuntEngine) ConfirmRebalanceEntry(request *RebalanceRequest) (*RebalanceDecision, error) {
	return &RebalanceDecision{RejectReason: "rebalance_entry_unsupported"}, nil
}

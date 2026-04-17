package strategy

import (
	"math"
	"math/rand"
	"sync"
	"time"

	"github.com/ljwqf/quant/pkg/logger"
	"github.com/ljwqf/quant/pkg/types"
	"go.uber.org/zap"
)

const (
	MinWeight             = 0.1
	MaxWeight             = 0.7
	WeightChangeThreshold = 0.15
	CooldownDuration      = 24 * time.Hour
	RebalanceInterval     = 4 * time.Hour
	PortfolioLossLimit    = 0.05
	DefaultAlpha          = 1.0
	DefaultBeta           = 1.0
)

type StrategyPerformance struct {
	Name              string
	Alpha             float64
	Beta              float64
	CurrentWeight     float64
	TargetWeight      float64
	LastTradeResult   *TradeResult
	ConsecutiveLosses int
	CooldownUntil     time.Time
	IsCooldown        bool
	TotalTrades       int
	WinTrades         int
	TotalPnL          float64
}

type TradeResult struct {
	Strategy  string
	PnL       float64
	Timestamp time.Time
	IsProfit  bool
}

type WeightAllocation struct {
	Strategy string
	Weight   float64
	Amount   float64
}

type OnlineBayesianAllocator struct {
	name            string
	params          map[string]interface{}
	metrics         map[string]interface{}
	strategies      map[string]*StrategyPerformance
	strategiesMutex sync.RWMutex
	lastRebalance   time.Time
	totalCapital    float64
	dailyLoss       float64
	dailyLossReset  time.Time
	metricsMutex    sync.Mutex
	rand            *rand.Rand
	randMutex       sync.Mutex
}

func NewOnlineBayesianAllocator() *OnlineBayesianAllocator {
	return &OnlineBayesianAllocator{
		name:       "OnlineBayesianAllocator",
		params:     make(map[string]interface{}),
		metrics:    make(map[string]interface{}),
		strategies: make(map[string]*StrategyPerformance),
		rand:       rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (a *OnlineBayesianAllocator) Name() string {
	return a.name
}

func (a *OnlineBayesianAllocator) Init(params map[string]interface{}) error {
	for k, v := range params {
		a.params[k] = v
	}

	logger.Info("OnlineBayesianAllocator初始化完成")
	return nil
}

func (a *OnlineBayesianAllocator) RegisterStrategy(name string, initialWeight float64) {
	a.strategiesMutex.Lock()
	defer a.strategiesMutex.Unlock()

	// 检查策略是否已注册，避免重复注册
	if _, exists := a.strategies[name]; exists {
		logger.Debug("策略已注册，跳过重复注册",
			zap.String("strategy", name),
		)
		return
	}

	if initialWeight < MinWeight {
		initialWeight = MinWeight
	}
	if initialWeight > MaxWeight {
		initialWeight = MaxWeight
	}

	a.strategies[name] = &StrategyPerformance{
		Name:          name,
		Alpha:         DefaultAlpha,
		Beta:          DefaultBeta,
		CurrentWeight: initialWeight,
		TargetWeight:  initialWeight,
		IsCooldown:    false,
	}

	logger.Info("注册策略到贝叶斯分配器",
		zap.String("strategy", name),
		zap.Float64("initial_weight", initialWeight),
	)
}

func (a *OnlineBayesianAllocator) RecordTradeResult(strategy string, pnl float64) {
	a.strategiesMutex.Lock()
	defer a.strategiesMutex.Unlock()

	a.resetDailyLossIfNeededLocked()

	perf, exists := a.strategies[strategy]
	if !exists {
		return
	}

	perf.TotalTrades++
	perf.TotalPnL += pnl

	result := &TradeResult{
		Strategy:  strategy,
		PnL:       pnl,
		Timestamp: time.Now(),
		IsProfit:  pnl > 0,
	}

	perf.LastTradeResult = result

	if pnl > 0 {
		perf.Alpha += 1
		perf.WinTrades++
		perf.ConsecutiveLosses = 0
		perf.IsCooldown = false
	} else {
		perf.Beta += 1
		perf.ConsecutiveLosses++

		if perf.ConsecutiveLosses >= 2 {
			perf.IsCooldown = true
			perf.CooldownUntil = time.Now().Add(CooldownDuration)
			logger.Warn("策略进入冷却期",
				zap.String("strategy", strategy),
				zap.Int("consecutive_losses", perf.ConsecutiveLosses),
				zap.Time("cooldown_until", perf.CooldownUntil),
			)
		}
	}

	if pnl < 0 {
		a.dailyLoss += -pnl
	}

	a.updateTargetWeightsLocked()

	logger.Info("记录交易结果",
		zap.String("strategy", strategy),
		zap.Float64("pnl", pnl),
		zap.Float64("alpha", perf.Alpha),
		zap.Float64("beta", perf.Beta),
		zap.Int("consecutive_losses", perf.ConsecutiveLosses),
	)

	if a.dailyLoss >= a.getPortfolioLossLimitAmountLocked() {
		a.resetToUniformPrior()
		a.updateTargetWeightsLocked()
		logger.Warn("组合亏损超限，重置为先验均匀分布",
			zap.Float64("daily_loss", a.dailyLoss),
			zap.Float64("limit", a.getPortfolioLossLimitAmountLocked()),
		)
	}
}

func (a *OnlineBayesianAllocator) resetToUniformPrior() {
	for _, perf := range a.strategies {
		perf.Alpha = DefaultAlpha
		perf.Beta = DefaultBeta
		perf.IsCooldown = false
		perf.ConsecutiveLosses = 0
	}
	a.dailyLoss = 0
}

// sampleBeta 使用 Gamma 分布采样 Beta(alpha, beta) 分布
// Beta(α,β) = Gamma(α,1) / (Gamma(α,1) + Gamma(β,1))
func (a *OnlineBayesianAllocator) sampleBeta(alpha, beta float64) float64 {
	a.randMutex.Lock()
	defer a.randMutex.Unlock()

	if alpha <= 0 || beta <= 0 {
		return 0
	}

	ga := sampleGamma(a.rand, alpha)
	gb := sampleGamma(a.rand, beta)
	denom := ga + gb
	if denom == 0 {
		return 0
	}
	return ga / denom
}

// sampleGamma 使用 Marsaglia & Tsang 简单算法采样 Gamma(shape, 1) 分布
func sampleGamma(r *rand.Rand, shape float64) float64 {
	if shape <= 0 {
		return 0
	}
	if shape < 1 {
		// Gamma(shape, 1) = Gamma(shape+1, 1) * U^(1/shape)
		return sampleGamma(r, shape+1) * math.Pow(r.Float64(), 1.0/shape)
	}

	d := shape - 1.0/3.0
	c := 1.0 / math.Sqrt(9.0*d)

	for {
		var x, v float64
		for {
			x = r.NormFloat64()
			v = 1.0 + c*x
			if v > 0 {
				break
			}
		}

		v = v * v * v
		u := r.Float64()

		// 0 < u < 1 − 0.0331 * (x^2)^2
		if u < 1.0-0.0331*(x*x)*(x*x) {
			return d * v
		}

		// u < exp(-0.5 * x^2) 等价于 log(u) < -0.5 * x^2
		if math.Log(u) < 0.5*x*x+d*(1.0-v+math.Log(v)) {
			return d * v
		}
	}
}

func (a *OnlineBayesianAllocator) CalculateWeights() map[string]float64 {
	a.strategiesMutex.Lock()
	defer a.strategiesMutex.Unlock()

	a.updateTargetWeightsLocked()

	weights := make(map[string]float64, len(a.strategies))
	for name, perf := range a.strategies {
		weights[name] = perf.TargetWeight
	}

	return weights
}

func (a *OnlineBayesianAllocator) ShouldRebalance() bool {
	if time.Since(a.lastRebalance) < a.getDurationParam("rebalance_interval", RebalanceInterval) {
		return false
	}

	a.strategiesMutex.RLock()
	defer a.strategiesMutex.RUnlock()

	for name, perf := range a.strategies {
		weightChange := math.Abs(perf.TargetWeight - perf.CurrentWeight)
		if weightChange > a.getFloatParam("weight_change_threshold", WeightChangeThreshold) {
			logger.Info("触发再平衡",
				zap.String("strategy", name),
				zap.Float64("current_weight", perf.CurrentWeight),
				zap.Float64("target_weight", perf.TargetWeight),
				zap.Float64("change", weightChange),
			)
			return true
		}
	}

	return false
}

func (a *OnlineBayesianAllocator) Rebalance() []WeightAllocation {
	if !a.ShouldRebalance() {
		return nil
	}

	a.strategiesMutex.Lock()
	defer a.strategiesMutex.Unlock()

	a.updateTargetWeightsLocked()

	allocations := make([]WeightAllocation, 0, len(a.strategies))
	for name, perf := range a.strategies {
		weight := perf.TargetWeight
		amount := a.totalCapital * weight
		allocations = append(allocations, WeightAllocation{
			Strategy: name,
			Weight:   weight,
			Amount:   amount,
		})

		perf.CurrentWeight = weight
	}

	a.lastRebalance = time.Now()

	logger.Info("执行权重再平衡",
		zap.Any("allocations", allocations),
	)

	return allocations
}

func (a *OnlineBayesianAllocator) SetTotalCapital(capital float64) {
	a.strategiesMutex.Lock()
	defer a.strategiesMutex.Unlock()

	a.totalCapital = capital
	logger.Debug("SetTotalCapital",
		zap.Float64("capital", capital),
		zap.Int("strategyCount", len(a.strategies)),
	)
	a.updateTargetWeightsLocked()
}

func (a *OnlineBayesianAllocator) GetAllocation(strategy string) *WeightAllocation {
	a.strategiesMutex.RLock()
	defer a.strategiesMutex.RUnlock()

	perf, exists := a.strategies[strategy]
	if !exists {
		logger.Warn("GetAllocation: strategy not found",
			zap.String("strategy", strategy),
			zap.Float64("totalCapital", a.totalCapital),
		)
		return nil
	}

	amount := a.totalCapital * perf.CurrentWeight
	logger.Debug("GetAllocation",
		zap.String("strategy", strategy),
		zap.Float64("totalCapital", a.totalCapital),
		zap.Float64("currentWeight", perf.CurrentWeight),
		zap.Float64("amount", amount),
	)

	return &WeightAllocation{
		Strategy: strategy,
		Weight:   perf.CurrentWeight,
		Amount:   amount,
	}
}

func (a *OnlineBayesianAllocator) GetAllAllocations() []WeightAllocation {
	weights := a.CalculateWeights()

	allocations := make([]WeightAllocation, 0, len(weights))
	for name, weight := range weights {
		amount := a.totalCapital * weight
		allocations = append(allocations, WeightAllocation{
			Strategy: name,
			Weight:   weight,
			Amount:   amount,
		})
	}

	return allocations
}

func (a *OnlineBayesianAllocator) getFloatParam(key string, defaultValue float64) float64 {
	if value, ok := a.params[key]; ok {
		switch typed := value.(type) {
		case float64:
			return typed
		case float32:
			return float64(typed)
		case int:
			return float64(typed)
		case int64:
			return float64(typed)
		}
	}

	return defaultValue
}

func (a *OnlineBayesianAllocator) getDurationParam(key string, defaultValue time.Duration) time.Duration {
	if value, ok := a.params[key]; ok {
		switch typed := value.(type) {
		case time.Duration:
			return typed
		case int:
			return time.Duration(typed)
		case int64:
			return time.Duration(typed)
		case float64:
			return time.Duration(typed)
		}
	}

	return defaultValue
}

func (a *OnlineBayesianAllocator) getPortfolioLossLimitAmountLocked() float64 {
	limitRatio := a.getFloatParam("portfolio_loss_limit", PortfolioLossLimit)
	if a.totalCapital > 0 {
		return a.totalCapital * limitRatio
	}
	return limitRatio
}

func (a *OnlineBayesianAllocator) resetDailyLossIfNeededLocked() {
	if a.dailyLossReset.IsZero() {
		a.dailyLossReset = time.Now()
		return
	}

	if time.Since(a.dailyLossReset) >= 24*time.Hour {
		a.dailyLoss = 0
		a.dailyLossReset = time.Now()
	}
}

func (a *OnlineBayesianAllocator) updateTargetWeightsLocked() {
	if len(a.strategies) == 0 {
		return
	}

	now := time.Now()
	minWeight := a.getFloatParam("min_weight", MinWeight)
	maxWeight := a.getFloatParam("max_weight", MaxWeight)
	if minWeight < 0 {
		minWeight = 0
	}
	if maxWeight <= 0 {
		maxWeight = MaxWeight
	}

	scores := make(map[string]float64, len(a.strategies))
	totalScore := 0.0
	activeStrategies := 0

	for name, perf := range a.strategies {
		if perf.IsCooldown && now.After(perf.CooldownUntil) {
			perf.IsCooldown = false
			perf.ConsecutiveLosses = 0
		}

		if perf.IsCooldown && now.Before(perf.CooldownUntil) {
			scores[name] = 0
			perf.TargetWeight = 0
			continue
		}

		posteriorMean := perf.Alpha / (perf.Alpha + perf.Beta)
		winRate := 0.5
		if perf.TotalTrades > 0 {
			winRate = float64(perf.WinTrades) / float64(perf.TotalTrades)
		}

		pnlFactor := 1.0
		if a.totalCapital > 0 {
			pnlFactor += perf.TotalPnL / a.totalCapital
		}
		if pnlFactor < 0.25 {
			pnlFactor = 0.25
		}

		lossPenalty := 1.0 / (1.0 + 0.25*float64(perf.ConsecutiveLosses))
		score := posteriorMean * (0.5 + 0.5*winRate) * pnlFactor * lossPenalty
		if score < 0 {
			score = 0
		}

		scores[name] = score
		totalScore += score
		activeStrategies++
	}

	if totalScore == 0 {
		equalWeight := 1.0 / float64(len(a.strategies))
		for _, perf := range a.strategies {
			perf.TargetWeight = equalWeight
		}
		return
	}

	rawWeights := make(map[string]float64, len(scores))
	for name, score := range scores {
		if score == 0 {
			rawWeights[name] = 0
			continue
		}

		weight := score / totalScore
		if activeStrategies > 1 && weight < minWeight {
			weight = minWeight
		}
		if weight > maxWeight {
			weight = maxWeight
		}
		rawWeights[name] = weight
	}

	normalizedWeights := normalizeAllocatorWeights(rawWeights)
	for name, perf := range a.strategies {
		perf.TargetWeight = normalizedWeights[name]
	}
}

func normalizeAllocatorWeights(weights map[string]float64) map[string]float64 {
	normalized := make(map[string]float64, len(weights))
	totalWeight := 0.0
	positiveCount := 0

	for _, weight := range weights {
		if weight > 0 {
			totalWeight += weight
			positiveCount++
		}
	}

	if totalWeight == 0 {
		if len(weights) == 0 {
			return normalized
		}
		equalWeight := 1.0 / float64(len(weights))
		for name := range weights {
			normalized[name] = equalWeight
		}
		return normalized
	}

	for name, weight := range weights {
		if weight <= 0 {
			normalized[name] = 0
			continue
		}
		normalized[name] = weight / totalWeight
	}

	if positiveCount == 1 {
		// 单策略场景：尊重 maxWeight 限制，剩余部分保留为现金储备
		for name, weight := range normalized {
			if weight > 0 {
				if weight > MaxWeight {
					normalized[name] = MaxWeight
				}
				// 其余资金保留为现金，不再分配
			}
		}
	}

	return normalized
}

func (a *OnlineBayesianAllocator) GetParams() map[string]interface{} {
	return a.params
}

func (a *OnlineBayesianAllocator) SetParams(params map[string]interface{}) {
	for k, v := range params {
		a.params[k] = v
	}
}

func (a *OnlineBayesianAllocator) GetMetrics() map[string]interface{} {
	a.metricsMutex.Lock()
	defer a.metricsMutex.Unlock()

	a.metrics["total_capital"] = a.totalCapital
	a.metrics["daily_loss"] = a.dailyLoss
	a.metrics["last_rebalance"] = a.lastRebalance

	strategyMetrics := make(map[string]interface{})
	a.strategiesMutex.RLock()
	for name, perf := range a.strategies {
		winRate := 0.0
		if perf.TotalTrades > 0 {
			winRate = float64(perf.WinTrades) / float64(perf.TotalTrades)
		}

		strategyMetrics[name] = map[string]interface{}{
			"alpha":              perf.Alpha,
			"beta":               perf.Beta,
			"current_weight":     perf.CurrentWeight,
			"target_weight":      perf.TargetWeight,
			"total_trades":       perf.TotalTrades,
			"win_trades":         perf.WinTrades,
			"win_rate":           winRate,
			"total_pnl":          perf.TotalPnL,
			"consecutive_losses": perf.ConsecutiveLosses,
			"is_cooldown":        perf.IsCooldown,
		}
	}
	a.strategiesMutex.RUnlock()

	a.metrics["strategies"] = strategyMetrics

	return a.metrics
}

func (a *OnlineBayesianAllocator) GetStrategyPerformance(name string) *StrategyPerformance {
	a.strategiesMutex.RLock()
	defer a.strategiesMutex.RUnlock()

	return a.strategies[name]
}

func (a *OnlineBayesianAllocator) IsStrategyCooldown(name string) bool {
	a.strategiesMutex.RLock()
	defer a.strategiesMutex.RUnlock()

	if perf, exists := a.strategies[name]; exists {
		if perf.IsCooldown && time.Now().Before(perf.CooldownUntil) {
			return true
		}
	}
	return false
}

func (a *OnlineBayesianAllocator) ForceReset() {
	a.strategiesMutex.Lock()
	defer a.strategiesMutex.Unlock()

	for _, perf := range a.strategies {
		perf.Alpha = DefaultAlpha
		perf.Beta = DefaultBeta
		perf.IsCooldown = false
		perf.ConsecutiveLosses = 0
		perf.CurrentWeight = 1.0 / float64(len(a.strategies))
		perf.TargetWeight = perf.CurrentWeight
	}

	a.dailyLoss = 0

	logger.Warn("强制重置所有策略为先验均匀分布")
}

func (a *OnlineBayesianAllocator) OnTick(tick *types.Tick) (*types.Signal, error) {
	return nil, nil
}

func (a *OnlineBayesianAllocator) OnBar(bar *types.Bar) (*types.Signal, error) {
	return nil, nil
}

func (a *OnlineBayesianAllocator) OnOrderBook(orderBook *types.OrderBook) (*types.Signal, error) {
	return nil, nil
}

func (a *OnlineBayesianAllocator) OnPositionFilled(symbol string, side types.OrderSide, entryPrice, size float64) {
}

func (a *OnlineBayesianAllocator) OnPositionReduced(symbol string, exitPrice, pnl, remainingSize float64) {
}

func (a *OnlineBayesianAllocator) OnPositionClosed(symbol string, exitPrice, pnl float64) {
}

func (a *OnlineBayesianAllocator) ConfirmRebalanceEntry(request *RebalanceRequest) (*RebalanceDecision, error) {
	return &RebalanceDecision{RejectReason: "rebalance_entry_unsupported"}, nil
}

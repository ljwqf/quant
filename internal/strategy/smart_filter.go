package strategy

import (
	"sync"
	"time"

	"github.com/ljwqf/quant/pkg/logger"
	"github.com/ljwqf/quant/pkg/types"
	"go.uber.org/zap"
)

const (
	DataExpiryHours        = 25
	NetflowThreshold       = 5000.0
	SOPRHighThreshold      = 1.05
	SOPRLowThreshold       = 0.95
	MVRVLowThreshold       = 1.0
	MVRVDeepValueThreshold = 0.8
	CacheDuration          = 24 * time.Hour
)

type MarketState int

const (
	MarketStateAccumulation MarketState = iota
	MarketStateDistribution
	MarketStateCapitulation
	MarketStateNeutral
)

func (s MarketState) String() string {
	switch s {
	case MarketStateAccumulation:
		return "Accumulation"
	case MarketStateDistribution:
		return "Distribution"
	case MarketStateCapitulation:
		return "Capitulation"
	case MarketStateNeutral:
		return "Neutral"
	default:
		return "Unknown"
	}
}

type OnChainData struct {
	ExchangeNetflow float64
	SOPR            float64
	LTHMVRV         float64
	Timestamp       time.Time
}

type SmartFilterResult struct {
	State         MarketState
	CanLong       bool
	CanShort      bool
	CanNeutral    bool
	Reason        string
	DataTimestamp time.Time
	IsExpired     bool
}

type SmartFilter struct {
	name         string
	params       map[string]interface{}
	metrics      map[string]interface{}
	onChainData  *OnChainData
	dataMutex    sync.RWMutex
	cachedResult *SmartFilterResult
	cacheTime    time.Time
	cacheMutex   sync.RWMutex
	lastUpdate   time.Time
	metricsMutex sync.Mutex
}

func NewSmartFilter() *SmartFilter {
	return &SmartFilter{
		name:    "SmartFilter",
		params:  make(map[string]interface{}),
		metrics: make(map[string]interface{}),
	}
}

func (f *SmartFilter) Name() string {
	return f.name
}

func (f *SmartFilter) Init(params map[string]interface{}) error {
	for k, v := range params {
		f.params[k] = v
	}

	logger.Info("SmartFilter初始化完成")
	return nil
}

func (f *SmartFilter) UpdateOnChainData(netflow, sopr, mvrv float64) {
	f.dataMutex.Lock()
	defer f.dataMutex.Unlock()

	f.onChainData = &OnChainData{
		ExchangeNetflow: netflow,
		SOPR:            sopr,
		LTHMVRV:         mvrv,
		Timestamp:       time.Now(),
	}

	f.lastUpdate = time.Now()

	f.cacheMutex.Lock()
	f.cachedResult = nil
	f.cacheMutex.Unlock()

	logger.Info("SmartFilter更新链上数据",
		zap.Float64("netflow", netflow),
		zap.Float64("sopr", sopr),
		zap.Float64("mvrv", mvrv),
	)
}

func (f *SmartFilter) GetMarketState() *SmartFilterResult {
	f.cacheMutex.RLock()
	if f.cachedResult != nil && time.Since(f.cacheTime) < CacheDuration {
		result := f.cachedResult
		f.cacheMutex.RUnlock()
		return result
	}
	f.cacheMutex.RUnlock()

	f.dataMutex.RLock()
	data := f.onChainData
	f.dataMutex.RUnlock()

	result := &SmartFilterResult{
		DataTimestamp: time.Time{},
		IsExpired:     true,
		CanLong:       false,
		CanShort:      false,
		CanNeutral:    true,
		Reason:        "数据过期，默认禁止方向性交易",
		State:         MarketStateNeutral,
	}

	if data == nil {
		f.cacheResult(result)
		return result
	}

	result.DataTimestamp = data.Timestamp
	result.IsExpired = time.Since(data.Timestamp) > DataExpiryHours*time.Hour

	if result.IsExpired {
		result.Reason = "链上数据过期，默认禁止方向性交易"
		f.cacheResult(result)
		return result
	}

	state := f.analyzeMarketState(data)
	result.State = state

	switch state {
	case MarketStateAccumulation:
		result.CanLong = true
		result.CanShort = false
		result.CanNeutral = true
		result.Reason = "积累阶段：净流出+低SOPR，允许做多"

	case MarketStateDistribution:
		result.CanLong = false
		result.CanShort = true
		result.CanNeutral = true
		result.Reason = "派发阶段：大额流入+高SOPR，禁止做多"

	case MarketStateCapitulation:
		result.CanLong = true
		result.CanShort = false
		result.CanNeutral = true
		result.Reason = "投降阶段：MVRV<0.8+亏损卖出，允许抄底"

	case MarketStateNeutral:
		result.CanLong = false
		result.CanShort = false
		result.CanNeutral = true
		result.Reason = "中性阶段：允许中性策略，禁止方向性策略"
	}

	f.cacheResult(result)

	logger.Info("SmartFilter市场状态分析",
		zap.String("state", state.String()),
		zap.Bool("can_long", result.CanLong),
		zap.Bool("can_short", result.CanShort),
		zap.String("reason", result.Reason),
	)

	return result
}

func (f *SmartFilter) analyzeMarketState(data *OnChainData) MarketState {
	isLargeNetflow := data.ExchangeNetflow > NetflowThreshold
	isLargeOutflow := data.ExchangeNetflow < -NetflowThreshold
	isHighSOPR := data.SOPR > SOPRHighThreshold
	isLowSOPR := data.SOPR < SOPRLowThreshold
	isLowMVRV := data.LTHMVRV < MVRVLowThreshold
	isDeepValueMVRV := data.LTHMVRV < MVRVDeepValueThreshold

	if isLargeOutflow && isLowSOPR {
		return MarketStateAccumulation
	}

	if isLargeNetflow && isHighSOPR {
		return MarketStateDistribution
	}

	if isDeepValueMVRV && isLowSOPR {
		return MarketStateCapitulation
	}

	if isLowMVRV && isLowSOPR {
		return MarketStateAccumulation
	}

	return MarketStateNeutral
}

func (f *SmartFilter) cacheResult(result *SmartFilterResult) {
	f.cacheMutex.Lock()
	defer f.cacheMutex.Unlock()

	f.cachedResult = result
	f.cacheTime = time.Now()
}

func (f *SmartFilter) CanOpenLong() bool {
	result := f.GetMarketState()
	return result.CanLong && !result.IsExpired
}

func (f *SmartFilter) CanOpenShort() bool {
	result := f.GetMarketState()
	return result.CanShort && !result.IsExpired
}

func (f *SmartFilter) CanRunNeutralStrategy() bool {
	result := f.GetMarketState()
	return result.CanNeutral
}

func (f *SmartFilter) GetParams() map[string]interface{} {
	return f.params
}

func (f *SmartFilter) SetParams(params map[string]interface{}) {
	for k, v := range params {
		f.params[k] = v
	}
}

func (f *SmartFilter) GetMetrics() map[string]interface{} {
	f.metricsMutex.Lock()
	defer f.metricsMutex.Unlock()

	result := f.GetMarketState()

	f.metrics["market_state"] = result.State.String()
	f.metrics["can_long"] = result.CanLong
	f.metrics["can_short"] = result.CanShort
	f.metrics["can_neutral"] = result.CanNeutral
	f.metrics["is_expired"] = result.IsExpired
	f.metrics["reason"] = result.Reason
	f.metrics["last_update"] = f.lastUpdate

	if f.onChainData != nil {
		f.metrics["netflow"] = f.onChainData.ExchangeNetflow
		f.metrics["sopr"] = f.onChainData.SOPR
		f.metrics["mvrv"] = f.onChainData.LTHMVRV
	}

	return f.metrics
}

func (f *SmartFilter) FilterSignal(signalType string) bool {
	switch signalType {
	case "long":
		return f.CanOpenLong()
	case "short":
		return f.CanOpenShort()
	case "neutral":
		return f.CanRunNeutralStrategy()
	default:
		return false
	}
}

func (f *SmartFilter) GetOnChainData() *OnChainData {
	f.dataMutex.RLock()
	defer f.dataMutex.RUnlock()

	return f.onChainData
}

func (f *SmartFilter) IsDataValid() bool {
	f.dataMutex.RLock()
	defer f.dataMutex.RUnlock()

	if f.onChainData == nil {
		return false
	}

	return time.Since(f.onChainData.Timestamp) <= DataExpiryHours*time.Hour
}

func (f *SmartFilter) OnPositionFilled(symbol string, side types.OrderSide, entryPrice, size float64) {
}

func (f *SmartFilter) OnPositionReduced(symbol string, exitPrice, pnl, remainingSize float64) {
}

func (f *SmartFilter) OnPositionClosed(symbol string, exitPrice, pnl float64) {
}

func (f *SmartFilter) ConfirmRebalanceEntry(request *RebalanceRequest) (*RebalanceDecision, error) {
	return &RebalanceDecision{RejectReason: "rebalance_entry_unsupported"}, nil
}

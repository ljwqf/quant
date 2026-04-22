package execution

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/ljwqf/quant/internal/exchange"
	"github.com/ljwqf/quant/internal/risk"
	"github.com/ljwqf/quant/internal/strategy"
	"github.com/ljwqf/quant/pkg/logger"
	"github.com/ljwqf/quant/pkg/types"
	"go.uber.org/zap"
)

type PositionRepository interface {
	Upsert(position *PositionRecord) error
	Delete(strategy, symbol string) error
	ListByStrategy(strategy string) ([]*PositionRecord, error)
	ListAll() ([]*PositionRecord, error)
}

type PositionRecord struct {
	Strategy   string
	Symbol     string
	Side       string
	Size       float64
	EntryPrice float64
	OrderID    string
}

type Engine struct {
	exchange          exchange.Exchange
	riskEngine        *risk.Engine
	strategyEngine    *strategy.Engine
	alertHandler      AlertHandler
	rebalanceHandler  RebalanceEventHandler
	takeProfitManager *TakeProfitManager
	bayesianAllocator *strategy.OnlineBayesianAllocator
	stateStore        StateStore
	positionRepo      PositionRepository
	smartRouteConfig  SmartRouteConfig
	rebalanceConfig   RebalanceConfig
	orders            map[string]*types.Order
	algoOrders        map[string]*types.AlgoOrder
	strategyPositions map[string]map[string]*strategyPosition
	metrics           map[string]interface{}
	mutex             sync.RWMutex
	strategyMutex     sync.RWMutex
	tpMonitorStop     chan struct{}
	orderMonitorStop  chan struct{}

	// 信号去重防护
	signalDedupCooldown time.Duration
	lastSignalTime      map[string]time.Time // key: "strategy:symbol:side"
	pendingOrders       map[string][]string  // key: "strategy:symbol:side" → orderIDs
	closingPositions    map[string]time.Time // key: "strategy:symbol" → last close signal time
	simulated           bool
}

type EngineConfig struct {
	TakeProfitConfig         *TakeProfitConfig `json:"take_profit"`
	EnableStrategyTakeProfit bool              `json:"enable_strategy_take_profit"` // 是否启用策略级止盈
	SmartRouteConfig         SmartRouteConfig  `json:"smart_route"`
	RebalanceConfig          RebalanceConfig   `json:"rebalance"`
	Simulated                bool              `json:"simulated"` // 模拟盘跳过流动性检查
}

type SmartRouteConfig struct {
	OrderBookDepth       int     `json:"order_book_depth"`
	MaxEstimatedSlippage float64 `json:"max_estimated_slippage"`
}

type RebalanceConfig struct {
	Enabled              bool          `json:"enabled"`
	ReduceOnly           bool          `json:"reduce_only"`
	DriftThreshold       float64       `json:"drift_threshold"`
	UseMarketOrders      bool          `json:"use_market_orders"`
	MaxPositionsPerCycle int           `json:"max_positions_per_cycle"`
	CircuitAutoReset     bool          `json:"circuit_auto_reset"`
	CircuitCooldown      time.Duration `json:"circuit_cooldown"`
}

type RebalanceCircuitState struct {
	Open            bool
	Strategy        string
	Step            string
	Reason          string
	OpenedAt        time.Time
	CooldownUntil   time.Time
	LastResetAt     time.Time
	LastResetReason string
	AutoReset       bool
	Cooldown        time.Duration
}

type AlertContext struct {
	Labels  map[string]string
	Details map[string]interface{}
}

type AlertLevel string

const (
	AlertLevelInfo     AlertLevel = "info"
	AlertLevelWarning  AlertLevel = "warning"
	AlertLevelError    AlertLevel = "error"
	AlertLevelCritical AlertLevel = "critical"
)

type AlertHandler func(level AlertLevel, title, message string, labels map[string]string, details map[string]interface{})

type RebalanceEventType string

const (
	RebalanceEventOpen           RebalanceEventType = "open"
	RebalanceEventReset          RebalanceEventType = "reset"
	RebalanceEventRecoverStarted RebalanceEventType = "recover_started"
	RebalanceEventRecoverSuccess RebalanceEventType = "recover_succeeded"
	RebalanceEventRecoverFailed  RebalanceEventType = "recover_failed"
)

type RebalanceEvent struct {
	Type      RebalanceEventType
	Strategy  string
	Step      string
	Reason    string
	Message   string
	Timestamp time.Time
	Labels    map[string]string
	Details   map[string]interface{}
	Circuit   RebalanceCircuitState
}

type RebalanceEventHandler func(event RebalanceEvent)

type rebalanceResetNotification struct {
	strategyName   string
	step           string
	reason         string
	occurredAt     time.Time
	previousOpenAt time.Time
	state          RebalanceCircuitState
}

type trackedRebalanceOrder struct {
	orderID string
	order   *types.Order
}

type strategyPosition struct {
	Strategy   string
	Symbol     string
	Side       types.OrderSide
	Size       float64
	EntryPrice float64
	MarkPrice  float64
	UpdatedAt  time.Time
}

type strategyExposure struct {
	position *strategyPosition
	exposure float64
	price    float64
}

var ErrRebalanceCircuitOpen = errors.New("rebalance circuit open")

type rebalancePlanExecution struct {
	orderID string
	step    strategy.RebalancePlanStep
}

const (
	defaultOrderBookDepth           = 20
	defaultMaxEstimatedSlippage     = 0.0025
	defaultRebalanceCircuitCooldown = 15 * time.Minute
)

func NewEngine(ex exchange.Exchange, riskEngine *risk.Engine, strategyEngine *strategy.Engine) *Engine {
	return &Engine{
		exchange:          ex,
		riskEngine:        riskEngine,
		strategyEngine:    strategyEngine,
		takeProfitManager: NewTakeProfitManager(nil),
		bayesianAllocator: strategy.NewOnlineBayesianAllocator(),
		smartRouteConfig:  defaultSmartRouteConfig(),
		rebalanceConfig:   defaultRebalanceConfig(),
		orders:            make(map[string]*types.Order),
		algoOrders:        make(map[string]*types.AlgoOrder),
		strategyPositions: make(map[string]map[string]*strategyPosition),
		metrics:           make(map[string]interface{}),
		signalDedupCooldown: 60 * time.Second,
		lastSignalTime:      make(map[string]time.Time),
		pendingOrders:       make(map[string][]string),
		closingPositions:    make(map[string]time.Time),
	}
}

func NewEngineWithConfig(ex exchange.Exchange, riskEngine *risk.Engine, strategyEngine *strategy.Engine, config *EngineConfig) *Engine {
	engine := &Engine{
		exchange:          ex,
		riskEngine:        riskEngine,
		strategyEngine:    strategyEngine,
		bayesianAllocator: strategy.NewOnlineBayesianAllocator(),
		smartRouteConfig:  defaultSmartRouteConfig(),
		rebalanceConfig:   defaultRebalanceConfig(),
		orders:            make(map[string]*types.Order),
		algoOrders:        make(map[string]*types.AlgoOrder),
		strategyPositions: make(map[string]map[string]*strategyPosition),
		metrics:           make(map[string]interface{}),
		signalDedupCooldown: 60 * time.Second,
		lastSignalTime:      make(map[string]time.Time),
		pendingOrders:       make(map[string][]string),
		closingPositions:    make(map[string]time.Time),
	}

	// 默认启用策略级止盈
	enableStrategyTakeProfit := true
	if config != nil {
		enableStrategyTakeProfit = config.EnableStrategyTakeProfit
	}

	// 始终初始化止盈管理器，策略可以选择使用或不使用
	if config != nil && config.TakeProfitConfig != nil {
		engine.takeProfitManager = NewTakeProfitManager(config.TakeProfitConfig)
	} else {
		engine.takeProfitManager = NewTakeProfitManager(nil)
	}

	// 记录是否启用策略级止盈
	if enableStrategyTakeProfit {
		engine.metrics["strategy_take_profit_enabled"] = true
	}

	if config != nil {
		engine.smartRouteConfig = normalizeSmartRouteConfig(config.SmartRouteConfig)
		engine.rebalanceConfig = normalizeRebalanceConfig(config.RebalanceConfig)
		engine.simulated = config.Simulated
	}

	return engine
}

func (e *Engine) SetStateStore(store StateStore) {
	e.stateStore = store
}

func (e *Engine) SetPositionRepository(repo PositionRepository) {
	e.positionRepo = repo
}

func (e *Engine) SetAlertHandler(handler AlertHandler) {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	e.alertHandler = handler
}

func (e *Engine) SetRebalanceEventHandler(handler RebalanceEventHandler) {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	e.rebalanceHandler = handler
}

func (e *Engine) SaveStateSnapshot() error {
	if e.stateStore == nil {
		return nil
	}

	strategySnapshot := make(map[string][]PersistedStrategyPosition)
	e.strategyMutex.RLock()
	for strategyName, positionsBySymbol := range e.strategyPositions {
		positions := make([]PersistedStrategyPosition, 0, len(positionsBySymbol))
		for _, position := range positionsBySymbol {
			if position == nil {
				continue
			}
			positions = append(positions, PersistedStrategyPosition{
				Strategy:   position.Strategy,
				Symbol:     position.Symbol,
				Side:       position.Side,
				Size:       position.Size,
				EntryPrice: position.EntryPrice,
				MarkPrice:  position.MarkPrice,
				UpdatedAt:  position.UpdatedAt,
			})
		}
		strategySnapshot[strategyName] = positions
	}
	e.strategyMutex.RUnlock()

	pendingOrders := make([]*types.Order, 0)
	e.mutex.RLock()
	for _, order := range e.orders {
		if order == nil {
			continue
		}
		copyOrder := *order
		if order.Metadata != nil {
			copyOrder.Metadata = cloneMetadata(order.Metadata)
		}
		pendingOrders = append(pendingOrders, &copyOrder)
	}
	metrics := make(map[string]interface{}, len(e.metrics))
	for key, value := range e.metrics {
		metrics[key] = value
	}
	e.mutex.RUnlock()

	riskPositionsMap := e.riskEngine.GetPositions()
	riskPositions := make([]*types.Position, 0, len(riskPositionsMap))
	for _, position := range riskPositionsMap {
		if position == nil {
			continue
		}
		copyPosition := *position
		riskPositions = append(riskPositions, &copyPosition)
	}

	return e.stateStore.Save(&EngineStateSnapshot{
		SavedAt:           time.Now(),
		StrategyPositions: strategySnapshot,
		PendingOrders:     pendingOrders,
		RiskPositions:     riskPositions,
		Metrics:           metrics,
	})
}

func (e *Engine) LoadStateSnapshot() (bool, error) {
	if e.stateStore == nil || !e.stateStore.Exists() {
		return false, nil
	}

	snapshot := &EngineStateSnapshot{}
	if err := e.stateStore.Load(snapshot); err != nil {
		return false, err
	}

	loadedStrategyPositions := make(map[string]map[string]*strategyPosition)
	for strategyName, positions := range snapshot.StrategyPositions {
		positionsBySymbol := make(map[string]*strategyPosition, len(positions))
		for _, position := range positions {
			positionsBySymbol[position.Symbol] = &strategyPosition{
				Strategy:   position.Strategy,
				Symbol:     position.Symbol,
				Side:       position.Side,
				Size:       position.Size,
				EntryPrice: position.EntryPrice,
				MarkPrice:  position.MarkPrice,
				UpdatedAt:  position.UpdatedAt,
			}
		}
		loadedStrategyPositions[strategyName] = positionsBySymbol
	}

	loadedOrders := make(map[string]*types.Order, len(snapshot.PendingOrders))
	for _, order := range snapshot.PendingOrders {
		if order == nil || order.ID == "" {
			continue
		}
		copyOrder := *order
		if order.Metadata != nil {
			copyOrder.Metadata = cloneMetadata(order.Metadata)
		}
		loadedOrders[copyOrder.ID] = &copyOrder
	}

	e.strategyMutex.Lock()
	e.strategyPositions = loadedStrategyPositions
	e.strategyMutex.Unlock()

	e.mutex.Lock()
	e.orders = loadedOrders
	e.metrics = make(map[string]interface{}, len(snapshot.Metrics))
	for key, value := range snapshot.Metrics {
		e.metrics[key] = value
	}
	e.mutex.Unlock()

	for symbol := range e.riskEngine.GetPositions() {
		e.riskEngine.RemovePosition(symbol)
	}
	for _, position := range snapshot.RiskPositions {
		if position == nil {
			continue
		}
		copyPosition := *position
		e.riskEngine.UpdatePosition(&copyPosition)
	}

	return true, nil
}

func (e *Engine) ReconcileWithExchange() error {
	positions, err := e.exchange.GetPositions()
	if err != nil {
		return err
	}

	exchangePositions := make(map[string]*types.Position, len(positions))
	for _, position := range positions {
		if position == nil {
			continue
		}
		copyPosition := *position
		exchangePositions[position.Symbol] = &copyPosition
	}

	for symbol := range e.riskEngine.GetPositions() {
		e.riskEngine.RemovePosition(symbol)
	}
	for _, position := range exchangePositions {
		e.riskEngine.UpdatePosition(position)
	}

	e.strategyMutex.Lock()
	for strategyName, positionsBySymbol := range e.strategyPositions {
		for symbol, trackedPosition := range positionsBySymbol {
			exchangePosition, exists := exchangePositions[symbol]
			if !exists || exchangePosition.Size <= 0 {
				delete(positionsBySymbol, symbol)
				continue
			}
			trackedPosition.Side = exchangePosition.Side
			trackedPosition.Size = exchangePosition.Size
			trackedPosition.EntryPrice = exchangePosition.EntryPrice
			trackedPosition.MarkPrice = exchangePosition.MarkPrice
			trackedPosition.UpdatedAt = time.Now()
		}
		if len(positionsBySymbol) == 0 {
			delete(e.strategyPositions, strategyName)
		}
	}
	e.strategyMutex.Unlock()

	if err := e.reconcilePendingRebalancePlans("startup_reconcile"); err != nil {
		return err
	}
	e.MonitorOrders()

	return nil
}

// RestorePositionsFromDB 从数据库恢复活跃持仓（启动时快照缺失时的兜底）
func (e *Engine) RestorePositionsFromDB() error {
	if e.positionRepo == nil {
		return nil
	}

	positions, err := e.positionRepo.ListAll()
	if err != nil {
		return err
	}

	if len(positions) == 0 {
		return nil
	}

	logger.Info("从数据库恢复活跃持仓",
		zap.Int("count", len(positions)),
	)

	for _, pos := range positions {
		side := types.OrderSide(pos.Side)
		e.strategyMutex.Lock()
		positionsByStrategy, exists := e.strategyPositions[pos.Strategy]
		if !exists {
			positionsByStrategy = make(map[string]*strategyPosition)
			e.strategyPositions[pos.Strategy] = positionsByStrategy
		}
		positionsByStrategy[pos.Symbol] = &strategyPosition{
			Strategy:   pos.Strategy,
			Symbol:     pos.Symbol,
			Side:       side,
			Size:       pos.Size,
			EntryPrice: pos.EntryPrice,
			MarkPrice:  pos.EntryPrice,
			UpdatedAt:  time.Now(),
		}
		e.strategyMutex.Unlock()

		// 同步到风控引擎
		e.riskEngine.UpdatePosition(&types.Position{
			Symbol:     pos.Symbol,
			Side:       side,
			Size:       pos.Size,
			EntryPrice: pos.EntryPrice,
			MarkPrice:  pos.EntryPrice,
		})

		// 通知策略
		if e.strategyEngine != nil {
			e.strategyEngine.NotifyPositionFilled(
				pos.Strategy,
				pos.Symbol,
				side,
				pos.EntryPrice,
				pos.Size,
			)
		}

		logger.Info("恢复持仓",
			zap.String("strategy", pos.Strategy),
			zap.String("symbol", pos.Symbol),
			zap.String("side", string(side)),
			zap.Float64("size", pos.Size),
			zap.Float64("entry_price", pos.EntryPrice),
		)
	}

	return nil
}

func defaultSmartRouteConfig() SmartRouteConfig {
	return SmartRouteConfig{
		OrderBookDepth:       defaultOrderBookDepth,
		MaxEstimatedSlippage: defaultMaxEstimatedSlippage,
	}
}

func normalizeSmartRouteConfig(config SmartRouteConfig) SmartRouteConfig {
	defaults := defaultSmartRouteConfig()
	if config.OrderBookDepth <= 0 {
		config.OrderBookDepth = defaults.OrderBookDepth
	}
	if config.MaxEstimatedSlippage <= 0 {
		config.MaxEstimatedSlippage = defaults.MaxEstimatedSlippage
	}
	return config
}

func defaultRebalanceConfig() RebalanceConfig {
	return RebalanceConfig{
		Enabled:              false,
		ReduceOnly:           true,
		DriftThreshold:       0.1,
		UseMarketOrders:      true,
		MaxPositionsPerCycle: 2,
		CircuitAutoReset:     true,
		CircuitCooldown:      defaultRebalanceCircuitCooldown,
	}
}

func normalizeRebalanceConfig(config RebalanceConfig) RebalanceConfig {
	defaults := defaultRebalanceConfig()
	if config.DriftThreshold <= 0 {
		config.DriftThreshold = defaults.DriftThreshold
	}
	if config.MaxPositionsPerCycle <= 0 {
		config.MaxPositionsPerCycle = defaults.MaxPositionsPerCycle
	}
	if config.CircuitCooldown <= 0 {
		config.CircuitCooldown = defaults.CircuitCooldown
	}
	if !config.Enabled {
		config.UseMarketOrders = defaults.UseMarketOrders
	}
	return config
}

func (e *Engine) Execute(signal *types.Signal, accountBalance float64) (*types.OrderResult, error) {
	if signal == nil {
		return nil, risk.ErrInvalidSignal
	}

	// 再平衡等系统操作跳过去重检查
	isSystemSignal := signal.Metadata != nil
	if sigMeta, ok := signal.Metadata.(map[string]interface{}); ok && isSystemSignal {
		if src, exists := sigMeta["source"].(string); exists && src == "rebalance_entry" {
			return e.executeInternal(signal, accountBalance, "")
		}
	}

	// 信号去重检查：防止同一策略短时间重复下单
	dedupKey := signal.Strategy + ":" + signal.Symbol + ":" + string(signal.Type)
	if !e.checkAndRecordSignal(dedupKey) {
		logger.Warn("信号被去重拦截",
			zap.String("strategy", signal.Strategy),
			zap.String("symbol", signal.Symbol),
			zap.String("type", string(signal.Type)),
		)
		return nil, nil
	}

	return e.executeInternal(signal, accountBalance, dedupKey)
}

func (e *Engine) executeInternal(signal *types.Signal, accountBalance float64, dedupKey string) (*types.OrderResult, error) {
	// 确保策略已注册到BayesianAllocator（RegisterStrategy内部会检查是否已存在）
	e.bayesianAllocator.RegisterStrategy(signal.Strategy, 1.0/3.0)

	// 设置总资金
	e.bayesianAllocator.SetTotalCapital(accountBalance)

	// 获取策略分配的资金
	allocation := e.bayesianAllocator.GetAllocation(signal.Strategy)
	if allocation == nil {
		logger.Error("获取策略资金分配失败",
			zap.String("strategy", signal.Strategy),
		)
		return nil, fmt.Errorf("策略 %s 未注册到资金分配器", signal.Strategy)
	}

	var order *types.Order
	var result *types.OrderResult
	var err error

	if signal.Type == types.SignalTypeExit {
		if err := e.riskEngine.CheckRisk(signal); err != nil {
			logger.Error("风险检查失败",
				zap.String("strategy", signal.Strategy),
				zap.String("symbol", signal.Symbol),
				zap.Error(err),
			)
			return nil, err
		}
		order, result, err = e.executeExitSignal(signal, allocation.Amount)
	} else {
		plannedSignal := *signal
		// 始终使用风控引擎计算的仓位大小，忽略策略硬编码数量
		// （策略的 Quantity 仅作为信号指示，实际仓位由风控预算决定）
		logger.Info("计算仓位参数",
			zap.String("strategy", signal.Strategy),
			zap.Float64("price", signal.Price),
			zap.Float64("allocationAmount", allocation.Amount),
		)
		plannedSignal.Quantity = e.riskEngine.GetPositionSize(signal, allocation.Amount)
		if plannedSignal.Quantity <= 0 {
			logger.Error("计算仓位大小失败",
				zap.String("strategy", signal.Strategy),
				zap.String("symbol", signal.Symbol),
				zap.Float64("price", signal.Price),
				zap.Float64("allocationAmount", allocation.Amount),
			)
			return nil, risk.ErrInvalidSignal
		}

		if err := e.riskEngine.CheckRisk(&plannedSignal); err != nil {
			logger.Error("风险检查失败",
				zap.String("strategy", signal.Strategy),
				zap.String("symbol", signal.Symbol),
				zap.Float64("quantity", plannedSignal.Quantity),
				zap.Error(err),
			)
			return nil, err
		}

		order, result, err = e.executeEntrySignal(&plannedSignal, allocation.Amount)
	}

	if err != nil {
		return nil, err
	}

	if result != nil {
		metadata := map[string]interface{}{
			"strategy":         signal.Strategy,
			"allocated_amount": allocation.Amount,
			"allocated_weight": allocation.Weight,
			"signal_type":      string(signal.Type),
			"is_exit":          signal.Type == types.SignalTypeExit,
		}
		if signal.Metadata != nil {
			if signalMetadata, ok := signal.Metadata.(map[string]interface{}); ok {
				for key, value := range signalMetadata {
					metadata[key] = value
				}
			}
		}
		e.trackOrder(result.OrderID, order, metadata)

		// 跟踪待成交订单（用于信号去重防护）
		if dedupKey != "" {
			e.registerPendingOrder(dedupKey, result.OrderID)
		}

		e.riskEngine.IncrementTrade()
		e.updateMetrics()

		logger.Info("执行交易信号成功",
			zap.String("strategy", signal.Strategy),
			zap.String("symbol", signal.Symbol),
			zap.String("type", string(signal.Type)),
			zap.Float64("price", signal.Price),
			zap.Float64("allocated_amount", allocation.Amount),
			zap.Float64("allocated_weight", allocation.Weight),
			zap.String("order_id", result.OrderID),
		)
	}

	return result, nil
}

func (e *Engine) executeEntrySignal(signal *types.Signal, accountBalance float64) (*types.Order, *types.OrderResult, error) {
	quantity := signal.Quantity
	if quantity <= 0 {
		quantity = e.riskEngine.GetPositionSize(signal, accountBalance)
	}
	if quantity <= 0 {
		logger.Error("计算仓位大小失败",
			zap.String("strategy", signal.Strategy),
			zap.String("symbol", signal.Symbol),
		)
		return nil, nil, nil
	}

	order := &types.Order{
		Symbol:    signal.Symbol,
		Side:      e.getOrderSide(signal.Type),
		Type:      types.OrderTypeLimit,
		Quantity:  quantity,
		Price:     signal.Price,
		Leverage:  1,
		Timestamp: time.Now(),
	}

	order, err := e.smartRoute(order, signal)
	if err != nil {
		logger.Error("订单簿检查失败",
			zap.String("strategy", signal.Strategy),
			zap.String("symbol", signal.Symbol),
			zap.Error(err),
		)
		return nil, nil, err
	}

	result, err := e.exchange.PlaceOrder(order)
	if err != nil {
		logger.Error("创建开仓订单失败",
			zap.String("strategy", signal.Strategy),
			zap.String("symbol", signal.Symbol),
			zap.Error(err),
		)
		return nil, nil, err
	}

	return order, result, nil
}

func (e *Engine) PlaceOrder(order *types.Order) (*types.OrderResult, error) {
	if order == nil {
		return nil, risk.ErrInvalidSignal
	}

	if order.Symbol == "" || order.Quantity <= 0 {
		return nil, risk.ErrInvalidSignal
	}

	if order.Type == types.OrderTypeLimit && order.Price <= 0 {
		return nil, risk.ErrInvalidSignal
	}

	if order.Timestamp.IsZero() {
		order.Timestamp = time.Now()
	}

	result, err := e.exchange.PlaceOrder(order)
	if err != nil {
		return nil, err
	}

	if result != nil {
		e.trackOrder(result.OrderID, order, map[string]interface{}{
			"source":      "manual",
			"signal_type": "manual",
			"is_exit":     false,
		})
		e.updateMetrics()
	}

	return result, nil
}

func (e *Engine) ClosePosition(symbol string, price float64) (*types.OrderResult, error) {
	signal := &types.Signal{
		Symbol:    symbol,
		Type:      types.SignalTypeExit,
		Price:     price,
		Timestamp: time.Now(),
	}

	order, result, err := e.executeExitSignal(signal, 0)
	if err != nil {
		return nil, err
	}

	if result != nil {
		e.trackOrder(result.OrderID, order, map[string]interface{}{
			"source":      "manual_close",
			"signal_type": string(types.SignalTypeExit),
			"is_exit":     true,
		})
		e.updateMetrics()
	}

	return result, nil
}

func (e *Engine) executeExitSignal(signal *types.Signal, accountBalance float64) (*types.Order, *types.OrderResult, error) {
	// TOCTOU 防护：5 秒内同一策略对同交易对已发送过平仓信号则跳过
	closeKey := signal.Strategy + ":" + signal.Symbol
	e.mutex.RLock()
	if lastClose, exists := e.closingPositions[closeKey]; exists && time.Since(lastClose) < 5*time.Second {
		e.mutex.RUnlock()
		logger.Info("平仓信号被 TOCTOU 防护拦截",
			zap.String("strategy", signal.Strategy),
			zap.String("symbol", signal.Symbol),
		)
		return nil, nil, nil
	}
	e.mutex.RUnlock()

	positions, err := e.exchange.GetPositions()
	if err != nil {
		logger.Error("获取持仓失败",
			zap.String("symbol", signal.Symbol),
			zap.Error(err),
		)
		return nil, nil, err
	}

	var position *types.Position
	for _, pos := range positions {
		if pos.Symbol == signal.Symbol {
			position = pos
			break
		}
	}

	if position == nil || position.Size <= 0 {
		logger.Info("没有需要平仓的持仓",
			zap.String("symbol", signal.Symbol),
		)
		return nil, nil, nil
	}

	orderSide := types.OrderSideSell
	if position.Side == types.OrderSideSell {
		orderSide = types.OrderSideBuy
	}

	order := &types.Order{
		Symbol:    signal.Symbol,
		Side:      orderSide,
		Type:      types.OrderTypeMarket,
		Quantity:  position.Size,
		Leverage:  position.Leverage,
		Timestamp: time.Now(),
	}

	if signal.Price > 0 {
		order.Type = types.OrderTypeLimit
		order.Price = signal.Price
	}

	result, err := e.exchange.PlaceOrder(order)
	if err != nil {
		// OKX 返回"无持仓可平"等错误时视为成功（已被其他途径平仓）
		if isAlreadyClosedError(err) {
			logger.Info("平仓时交易对方已无持仓（可能已被其他途径平仓）",
				zap.String("strategy", signal.Strategy),
				zap.String("symbol", signal.Symbol),
				zap.Error(err),
			)
			return nil, nil, nil
		}
		logger.Error("创建平仓订单失败",
			zap.String("strategy", signal.Strategy),
			zap.String("symbol", signal.Symbol),
			zap.Error(err),
		)
		return nil, nil, err
	}

	// 标记已发送平仓信号
	e.mutex.Lock()
	e.closingPositions[closeKey] = time.Now()
	e.mutex.Unlock()

	return order, result, nil
}

// isAlreadyClosedError 判断是否为"无持仓可平"类错误
func isAlreadyClosedError(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "position") &&
		(strings.Contains(msg, "close") ||
			strings.Contains(msg, "available") ||
			strings.Contains(msg, "not enough") ||
			strings.Contains(msg, "no position"))
}

func (e *Engine) ExecuteWithTakeProfit(signal *types.Signal, accountBalance float64, takeProfitConfig *TakeProfitConfig) (*types.OrderResult, error) {
	if takeProfitConfig != nil {
		e.takeProfitManager.SetConfig(takeProfitConfig)
	}

	result, err := e.Execute(signal, accountBalance)
	if err != nil {
		return nil, err
	}

	if result != nil && result.Status == types.OrderStatusFilled {
		e.takeProfitManager.AddPosition(
			signal.Symbol,
			e.getOrderSide(signal.Type),
			signal.Price,
			result.Quantity,
		)
	}

	return result, nil
}

func (e *Engine) CancelOrder(orderID string) error {
	e.mutex.RLock()
	_, ok := e.orders[orderID]
	e.mutex.RUnlock()

	if !ok {
		return nil
	}

	err := e.exchange.CancelOrder(orderID)
	if err != nil {
		logger.Error("取消订单失败",
			zap.String("order_id", orderID),
			zap.Error(err),
		)
		return err
	}

	e.mutex.Lock()
	delete(e.orders, orderID)
	e.mutex.Unlock()

	logger.Info("取消订单成功",
		zap.String("order_id", orderID),
	)

	return nil
}

// checkAndRecordSignal 信号去重检查：60 秒冷却 + 同策略同方向订单未成交则拦截
func (e *Engine) checkAndRecordSignal(dedupKey string) bool {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	// 定期清理过期条目（每 100 次调用触发一次）
	if len(e.lastSignalTime)%100 == 0 {
		cutoff := time.Now().Add(-5 * time.Minute)
		for k, v := range e.lastSignalTime {
			if v.Before(cutoff) {
				delete(e.lastSignalTime, k)
			}
		}
		for k, v := range e.closingPositions {
			if v.Before(cutoff) {
				delete(e.closingPositions, k)
			}
		}
	}

	// 检查 1：冷却窗口
	if lastTime, exists := e.lastSignalTime[dedupKey]; exists {
		if time.Since(lastTime) < e.signalDedupCooldown {
			return false
		}
	}

	// 检查 2：同策略同方向是否有未成交订单
	if pendingList, exists := e.pendingOrders[dedupKey]; exists && len(pendingList) > 0 {
		return false
	}

	// 记录信号
	e.lastSignalTime[dedupKey] = time.Now()
	return true
}

// registerPendingOrder 跟踪新下单的订单到待成交列表
func (e *Engine) registerPendingOrder(dedupKey, orderID string) {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	e.pendingOrders[dedupKey] = append(e.pendingOrders[dedupKey], orderID)
}

// clearPendingOrder 订单成交或取消后从待成交列表移除
func (e *Engine) clearPendingOrder(dedupKey, orderID string) {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	list := e.pendingOrders[dedupKey]
	for i, id := range list {
		if id == orderID {
			e.pendingOrders[dedupKey] = append(list[:i], list[i+1:]...)
			break
		}
	}
	if len(e.pendingOrders[dedupKey]) == 0 {
		delete(e.pendingOrders, dedupKey)
	}
}

func (e *Engine) GetOrders() map[string]*types.Order {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	result := make(map[string]*types.Order)
	for k, v := range e.orders {
		result[k] = v
	}
	return result
}

func (e *Engine) GetOrder(orderID string) *types.Order {
	e.mutex.RLock()
	defer e.mutex.RUnlock()
	return e.orders[orderID]
}

func (e *Engine) HasOrder(orderID string) bool {
	e.mutex.RLock()
	defer e.mutex.RUnlock()
	_, exists := e.orders[orderID]
	return exists
}

func (e *Engine) GetOrderCount() int {
	e.mutex.RLock()
	defer e.mutex.RUnlock()
	return len(e.orders)
}

// TrackExternalOrder 将交易所侧发现的孤儿订单注入本地追踪
func (e *Engine) TrackExternalOrder(order *types.Order) {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	if _, ok := e.orders[order.ID]; !ok {
		order.Metadata = map[string]interface{}{
			"source": "reconciler",
		}
		e.orders[order.ID] = order
	}
}

// GetAlgoOrders 获取本地追踪的算法单
func (e *Engine) GetAlgoOrders() map[string]*types.AlgoOrder {
	e.mutex.RLock()
	defer e.mutex.RUnlock()
	result := make(map[string]*types.AlgoOrder)
	for k, v := range e.algoOrders {
		result[k] = v
	}
	return result
}

// TrackExternalAlgoOrder 将交易所侧发现的孤儿算法单注入本地追踪
func (e *Engine) TrackExternalAlgoOrder(algo *types.AlgoOrder) {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	if _, ok := e.algoOrders[algo.AlgoID]; !ok {
		e.algoOrders[algo.AlgoID] = algo
	}
}

func (e *Engine) GetMetrics() map[string]interface{} {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	result := make(map[string]interface{})
	for k, v := range e.metrics {
		result[k] = v
	}
	return result
}

func (e *Engine) emitAlert(level AlertLevel, title, message string, context *AlertContext) {
	e.mutex.RLock()
	handler := e.alertHandler
	e.mutex.RUnlock()
	if handler != nil {
		labels := map[string]string(nil)
		details := map[string]interface{}(nil)
		if context != nil {
			labels = cloneStringMap(context.Labels)
			details = cloneMetadata(context.Details)
		}
		handler(level, title, message, labels, details)
	}
}

func (e *Engine) emitRebalanceEvent(eventType RebalanceEventType, strategyName, step, reason, message string, context *AlertContext, circuit RebalanceCircuitState, occurredAt time.Time) {
	e.mutex.RLock()
	handler := e.rebalanceHandler
	e.mutex.RUnlock()
	if handler == nil {
		return
	}
	handler(RebalanceEvent{
		Type:      eventType,
		Strategy:  strategyName,
		Step:      step,
		Reason:    reason,
		Message:   message,
		Timestamp: occurredAt,
		Labels:    cloneStringMap(contextLabels(context)),
		Details:   cloneMetadata(contextDetails(context)),
		Circuit:   circuit,
	})
}

func contextLabels(context *AlertContext) map[string]string {
	if context == nil {
		return nil
	}
	return context.Labels
}

func contextDetails(context *AlertContext) map[string]interface{} {
	if context == nil {
		return nil
	}
	return context.Details
}

func (e *Engine) GetRebalanceCircuitState() RebalanceCircuitState {
	e.mutex.Lock()
	state, resetEvent := e.getRebalanceCircuitStateLocked(time.Now())
	e.mutex.Unlock()
	e.notifyRebalanceReset(resetEvent)
	return state
}

func (e *Engine) ResetRebalanceCircuit(reason string) bool {
	e.mutex.Lock()
	if !metricBool(e.metrics, "rebalance_circuit_open") {
		e.mutex.Unlock()
		return false
	}
	resetEvent := e.resetRebalanceCircuitLocked(reason, time.Now())
	e.mutex.Unlock()
	e.notifyRebalanceReset(resetEvent)
	return true
}

func (e *Engine) smartRoute(order *types.Order, signal *types.Signal) (*types.Order, error) {
	if order == nil {
		return nil, risk.ErrInvalidSignal
	}

	// 模拟盘跳过订单簿深度检查（OKX 模拟盘订单簿深度极浅）
	if e.simulated {
		return order, nil
	}

	orderBook, err := e.exchange.GetOrderBook(order.Symbol, e.smartRouteConfig.OrderBookDepth)
	if err != nil || orderBook == nil {
		logger.Warn("获取订单簿失败，跳过深度检查",
			zap.String("symbol", order.Symbol),
			zap.Error(err),
		)
		return order, nil
	}

	avgPrice, bestPrice, availableQty, sufficient := estimateOrderBookFill(order, orderBook)
	if !sufficient {
		return nil, risk.ErrLiquidityInsufficient
	}

	slippage := calculateEstimatedSlippage(order.Side, bestPrice, avgPrice)
	maxSlippage := signalMaxSlippage(signal, e.smartRouteConfig.MaxEstimatedSlippage)

	if order.Metadata == nil {
		order.Metadata = make(map[string]interface{})
	}
	order.Metadata["book_depth_checked"] = e.smartRouteConfig.OrderBookDepth
	order.Metadata["book_best_price"] = bestPrice
	order.Metadata["book_available_quantity"] = availableQty
	order.Metadata["estimated_avg_price"] = avgPrice
	order.Metadata["estimated_slippage"] = slippage
	order.Metadata["max_allowed_slippage"] = maxSlippage

	if slippage > maxSlippage {
		return nil, risk.ErrPriceDeviationTooHigh
	}

	if order.Type == types.OrderTypeLimit && order.Price <= 0 {
		order.Price = bestPrice
	}

	return order, nil
}

func (e *Engine) getOrderSide(signalType types.SignalType) types.OrderSide {
	switch signalType {
	case types.SignalTypeBuy:
		return types.OrderSideBuy
	case types.SignalTypeSell, types.SignalTypeExit:
		return types.OrderSideSell
	default:
		return types.OrderSideBuy
	}
}

func (e *Engine) updateMetrics() {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	e.metrics["total_orders"] = len(e.orders)
	e.metrics["timestamp"] = time.Now()
}

// recordSlippage 记录滑点数据
func (e *Engine) recordSlippage(slippage float64, symbol string, side types.OrderSide) {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	// 累计滑点统计
	totalCount, _ := e.metrics["slippage_count"].(int)
	totalCount++
	e.metrics["slippage_count"] = totalCount

	// 更新最大滑点
	maxSlippage, _ := e.metrics["max_slippage"].(float64)
	if slippage > maxSlippage {
		e.metrics["max_slippage"] = slippage
	}

	// 更新平均滑点
	avgSlippage, _ := e.metrics["avg_slippage"].(float64)
	e.metrics["avg_slippage"] = (avgSlippage*float64(totalCount-1) + slippage) / float64(totalCount)

	// 按交易对统计
	symbolKey := "slippage_" + symbol
	symbolSlippage, _ := e.metrics[symbolKey].(float64)
	symbolCount, _ := e.metrics[symbolKey+"_count"].(int)
	symbolCount++
	e.metrics[symbolKey+"_count"] = symbolCount
	e.metrics[symbolKey] = (symbolSlippage*float64(symbolCount-1) + slippage) / float64(symbolCount)

	// 滑点偏高告警
	if slippage > 0.01 { // 超过 1%
		logger.Warn("滑点异常偏高",
			zap.String("symbol", symbol),
			zap.String("side", string(side)),
			zap.Float64("slippage", slippage*100),
		)
	}
}

// GetSlippageMetrics 获取滑点指标
func (e *Engine) GetSlippageMetrics() map[string]interface{} {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	result := make(map[string]interface{})
	for k, v := range e.metrics {
		if k == "slippage_count" || k == "max_slippage" || k == "avg_slippage" ||
			(strings.HasPrefix(k, "slippage_") && !strings.HasSuffix(k, "_count")) {
			result[k] = v
		}
	}
	return result
}

func (e *Engine) MonitorOrders() {
	e.mutex.RLock()
	orderIDs := make([]string, 0, len(e.orders))
	ordersCopy := make(map[string]*types.Order)
	for orderID, order := range e.orders {
		orderIDs = append(orderIDs, orderID)
		ordersCopy[orderID] = order
	}
	e.mutex.RUnlock()

	for _, orderID := range orderIDs {
		orderInfo, err := e.exchange.GetOrder(orderID)
		if err != nil {
			// 订单查不到，可能已成交并从历史列表中消失
			if localOrder := ordersCopy[orderID]; localOrder != nil && localOrder.FilledQty > 0 {
				logger.Info("订单已从交易所历史消失，按已成交处理",
					zap.String("order_id", orderID),
					zap.Float64("filled_qty", localOrder.FilledQty),
				)
				e.handleOrderFilled(orderID, localOrder, localOrder)
			} else {
				logger.Warn("订单已从交易所历史消失，从监控列表移除",
					zap.String("order_id", orderID),
				)
				e.mutex.Lock()
				delete(e.orders, orderID)
				e.mutex.Unlock()
			}
			continue
		}

		if orderInfo.Status == types.OrderStatusFilled {
			e.handleOrderFilled(orderID, ordersCopy[orderID], orderInfo)
		} else if orderInfo.Status == types.OrderStatusPartially {
			e.handleOrderPartially(orderID, ordersCopy[orderID], orderInfo)
		} else if orderInfo.Status == types.OrderStatusCancelled || orderInfo.Status == types.OrderStatusFailed {
			e.handleOrderCompleted(orderID, orderInfo)
		}
	}
}

func (e *Engine) handleOrderFilled(orderID string, localOrder *types.Order, exchangeOrder *types.Order) {
	e.processObservedOrderFill(orderID, localOrder, exchangeOrder, true)
}

func (e *Engine) handleOrderPartially(orderID string, localOrder *types.Order, exchangeOrder *types.Order) {
	deltaFilled := e.processObservedOrderFill(orderID, localOrder, exchangeOrder, false)
	if deltaFilled <= 0 || !isMultiLegRebalanceOrder(localOrder) {
		return
	}
	if err := e.recoverRebalancePlan(orderID, orderMetadataString(localOrder, "strategy", ""), "partial_fill_detected"); err != nil {
		logger.Error("处理多腿再平衡部分成交失败",
			zap.String("order_id", orderID),
			zap.Error(err),
		)
	}
}

func (e *Engine) processObservedOrderFill(orderID string, localOrder *types.Order, exchangeOrder *types.Order, final bool) float64 {
	strategyName := ""
	isExitOrder := false
	var previousPosition *types.Position
	filledPrice := resolveFilledPrice(exchangeOrder, localOrder)
	totalFilled := resolveObservedFilledQty(exchangeOrder, localOrder)
	processedFilled := orderMetadataFloat(localOrder, "processed_filled_quantity", 0)
	deltaFilled := totalFilled - processedFilled
	if deltaFilled < 1e-9 {
		deltaFilled = 0
	}

	if localOrder != nil && localOrder.Metadata != nil {
		if value, ok := localOrder.Metadata["strategy"].(string); ok {
			strategyName = value
		}
		if value, ok := localOrder.Metadata["is_exit"].(bool); ok {
			isExitOrder = value
		}
	}

	if isExitOrder {
		previousPosition = e.riskEngine.GetPosition(exchangeOrder.Symbol)
	}

	e.updateTrackedOrderFillState(orderID, exchangeOrder, totalFilled, final)

	logger.Info("订单成交状态更新",
		zap.String("order_id", orderID),
		zap.String("symbol", exchangeOrder.Symbol),
		zap.String("side", string(exchangeOrder.Side)),
		zap.Float64("filled_qty", totalFilled),
		zap.Float64("delta_filled_qty", deltaFilled),
		zap.Float64("avg_price", exchangeOrder.AveragePrice),
		zap.String("status", string(exchangeOrder.Status)),
	)

	if deltaFilled <= 0 {
		return 0
	}

	// 滑点记录：预期价格 vs 实际成交价
	if localOrder != nil {
		expectedPrice := localOrder.Price
		if expectedPrice > 0 && filledPrice > 0 {
			slippage := math.Abs(filledPrice-expectedPrice) / expectedPrice
			e.recordSlippage(slippage, exchangeOrder.Symbol, exchangeOrder.Side)
		}
	}

	if strategyName != "" && e.strategyEngine != nil {
		if isExitOrder {
			trackedPosition, pnl, fullyClosed := e.applyStrategyExitFill(strategyName, exchangeOrder.Symbol, filledPrice, deltaFilled)
			if trackedPosition == nil {
				pnl = calculatePositionPnL(previousPosition, filledPrice)
				fullyClosed = true
			}
			e.riskEngine.UpdatePnL(exchangeOrder.Symbol, pnl)
			if fullyClosed {
				e.strategyEngine.NotifyPositionClosed(
					strategyName,
					exchangeOrder.Symbol,
					filledPrice,
					pnl,
				)
				// 同步移除风控引擎中的持仓
				e.riskEngine.RemovePosition(exchangeOrder.Symbol)
			} else {
				remainingSize := 0.0
				if currentPosition := e.getStrategyPosition(strategyName, exchangeOrder.Symbol); currentPosition != nil {
					remainingSize = currentPosition.Size
				}
				e.strategyEngine.NotifyPositionReduced(
					strategyName,
					exchangeOrder.Symbol,
					filledPrice,
					pnl,
					remainingSize,
				)
			}
			e.RecordTradeResult(strategyName, pnl)
		} else {
			e.recordStrategyEntryFill(strategyName, exchangeOrder.Symbol, exchangeOrder.Side, filledPrice, deltaFilled)
			e.strategyEngine.NotifyPositionFilled(
				strategyName,
				exchangeOrder.Symbol,
				exchangeOrder.Side,
				filledPrice,
				deltaFilled,
			)
		}
	} else if isExitOrder {
		// 没有策略时也需要同步移除风控引擎中的持仓
		e.riskEngine.RemovePosition(exchangeOrder.Symbol)
	}

	// 同步风控持仓状态
	e.syncRiskPosition(exchangeOrder.Symbol)

	if isExitOrder && e.takeProfitManager != nil {
		if e.riskEngine.GetPosition(exchangeOrder.Symbol) == nil {
			e.takeProfitManager.RemovePosition(exchangeOrder.Symbol)
		}
	}

	if !isExitOrder && e.takeProfitManager != nil {
		e.takeProfitManager.AddPosition(
			exchangeOrder.Symbol,
			exchangeOrder.Side,
			resolveFilledPrice(exchangeOrder, localOrder),
			deltaFilled,
		)
	}

	return deltaFilled
}

// RecordTradeResult 记录交易结果到BayesianAllocator
func (e *Engine) RecordTradeResult(strategy string, pnl float64) {
	e.bayesianAllocator.RecordTradeResult(strategy, pnl)

	// 检查是否需要再平衡
	if e.bayesianAllocator.ShouldRebalance() {
		allocations := e.bayesianAllocator.Rebalance()
		e.applyRebalanceAllocations(allocations)
		logger.Info("执行策略权重再平衡",
			zap.Any("allocations", allocations),
		)
	}
}

func (e *Engine) handleOrderCompleted(orderID string, orderInfo *types.Order) {
	e.mutex.Lock()
	delete(e.orders, orderID)
	e.mutex.Unlock()

	// 清除待成交订单跟踪
	if strategy, ok := orderInfo.Metadata["strategy"].(string); ok {
		if signalType, ok := orderInfo.Metadata["signal_type"].(string); ok {
			dedupKey := strategy + ":" + orderInfo.Symbol + ":" + signalType
			e.clearPendingOrder(dedupKey, orderID)
		}
	}

	e.syncRiskPosition(orderInfo.Symbol)

	logger.Info("订单完成",
		zap.String("order_id", orderID),
		zap.String("status", string(orderInfo.Status)),
	)
}

// GetBayesianAllocator 获取BayesianAllocator实例
func (e *Engine) GetBayesianAllocator() *strategy.OnlineBayesianAllocator {
	return e.bayesianAllocator
}

func (e *Engine) MonitorTakeProfit() {
	if e.takeProfitManager == nil {
		return
	}

	positions := e.takeProfitManager.GetAllPositions()

	for symbol, state := range positions {
		tick, err := e.exchange.GetTicker(symbol)
		if err != nil {
			logger.Error("获取行情失败",
				zap.String("symbol", symbol),
				zap.Error(err),
			)
			continue
		}

		e.takeProfitManager.UpdatePrice(symbol, tick.Price)

		tpSignal := e.takeProfitManager.CheckTakeProfit(symbol)
		if tpSignal != nil {
			e.executeTakeProfitSignal(tpSignal, state)
		}
	}
}

func (e *Engine) executeTakeProfitSignal(signal *TakeProfitSignal, state *PositionState) {
	if e.takeProfitManager == nil {
		return
	}

	order := &types.Order{
		Symbol:    signal.Symbol,
		Side:      signal.Side,
		Type:      types.OrderTypeMarket,
		Quantity:  signal.Quantity,
		Timestamp: time.Now(),
	}

	result, err := e.exchange.PlaceOrder(order)
	if err != nil {
		logger.Error("执行止盈订单失败",
			zap.String("symbol", signal.Symbol),
			zap.String("trigger_type", signal.TriggerType),
			zap.Error(err),
		)
		return
	}

	logger.Info("执行止盈成功",
		zap.String("symbol", signal.Symbol),
		zap.String("trigger_type", signal.TriggerType),
		zap.String("reason", signal.Reason),
		zap.Float64("quantity", signal.Quantity),
		zap.Float64("profit_percent", signal.ProfitPercent),
		zap.String("order_id", result.OrderID),
	)

	e.trackOrder(result.OrderID, order, map[string]interface{}{
		"take_profit_trigger": signal.TriggerType,
		"take_profit_reason":  signal.Reason,
		"tier_level":          signal.TierLevel,
	})

	if signal.TriggerType == "fixed_take_profit" ||
		(signal.TriggerType == "trailing_take_profit" && state.ClosedPercent == 0) ||
		(signal.TriggerType == "atr_take_profit" && state.ClosedPercent == 0) {
		if e.takeProfitManager != nil {
			e.takeProfitManager.RemovePosition(signal.Symbol)
		}
	}
}

func (e *Engine) UpdatePositionPrice(symbol string, price float64) {
	if e.takeProfitManager == nil {
		return
	}
	e.takeProfitManager.UpdatePrice(symbol, price)
}

func (e *Engine) UpdatePositionATR(symbol string, bar *types.Bar) {
	if e.takeProfitManager == nil {
		return
	}
	e.takeProfitManager.UpdateATR(symbol, bar)
}

func (e *Engine) AddPositionToTakeProfitManager(symbol string, side types.OrderSide, entryPrice, size float64) {
	if e.takeProfitManager == nil {
		return
	}
	e.takeProfitManager.AddPosition(symbol, side, entryPrice, size)
}

func (e *Engine) RemovePositionFromTakeProfitManager(symbol string) {
	if e.takeProfitManager == nil {
		return
	}
	e.takeProfitManager.RemovePosition(symbol)
}

func (e *Engine) GetTakeProfitManager() *TakeProfitManager {
	return e.takeProfitManager
}

func (e *Engine) GetPositionState(symbol string) *PositionState {
	if e.takeProfitManager == nil {
		return nil
	}
	return e.takeProfitManager.GetPositionState(symbol)
}

func (e *Engine) GetAllPositionStates() map[string]*PositionState {
	if e.takeProfitManager == nil {
		return make(map[string]*PositionState)
	}
	return e.takeProfitManager.GetAllPositions()
}

func (e *Engine) CloseAllPositions() error {
	positions, err := e.exchange.GetPositions()
	if err != nil {
		logger.Error("获取持仓失败",
			zap.Error(err),
		)
		return err
	}

	for _, position := range positions {
		if position.Size > 0 {
			orderSide := types.OrderSideSell
			if position.Side == types.OrderSideSell {
				orderSide = types.OrderSideBuy
			}

			order := &types.Order{
				Symbol:    position.Symbol,
				Side:      orderSide,
				Type:      types.OrderTypeMarket,
				Quantity:  position.Size,
				Leverage:  position.Leverage,
				Timestamp: time.Now(),
			}

			result, err := e.exchange.PlaceOrder(order)
			if err != nil {
				logger.Error("平仓失败",
					zap.String("symbol", position.Symbol),
					zap.Error(err),
				)
				continue
			}

			if e.takeProfitManager != nil {
				e.takeProfitManager.RemovePosition(position.Symbol)
			}

			logger.Info("平仓成功",
				zap.String("symbol", position.Symbol),
				zap.Float64("size", position.Size),
				zap.String("order_id", result.OrderID),
			)
		}
	}

	return nil
}

func (e *Engine) trackOrder(orderID string, order *types.Order, metadata map[string]interface{}) {
	if order == nil || orderID == "" {
		return
	}

	e.mutex.Lock()
	defer e.mutex.Unlock()

	if metadata != nil {
		if order.Metadata == nil {
			order.Metadata = make(map[string]interface{}, len(metadata))
		}
		for key, value := range metadata {
			order.Metadata[key] = value
		}
	}
	if order.ID == "" {
		order.ID = orderID
	}
	e.orders[orderID] = order
}

func resolveFilledPrice(exchangeOrder *types.Order, localOrder *types.Order) float64 {
	if exchangeOrder != nil {
		if exchangeOrder.AveragePrice > 0 {
			return exchangeOrder.AveragePrice
		}
		if exchangeOrder.Price > 0 {
			return exchangeOrder.Price
		}
	}

	if localOrder != nil {
		if localOrder.AveragePrice > 0 {
			return localOrder.AveragePrice
		}
		if localOrder.Price > 0 {
			return localOrder.Price
		}
	}

	return 0
}

func calculatePositionPnL(position *types.Position, exitPrice float64) float64 {
	if position == nil || exitPrice <= 0 || position.Size <= 0 || position.EntryPrice <= 0 {
		return 0
	}

	if position.Side == types.OrderSideSell {
		return (position.EntryPrice - exitPrice) * position.Size
	}

	return (exitPrice - position.EntryPrice) * position.Size
}

func calculateStrategyPositionPnL(position *strategyPosition, exitPrice, quantity float64) float64 {
	if position == nil || exitPrice <= 0 || quantity <= 0 || position.EntryPrice <= 0 {
		return 0
	}

	if quantity > position.Size {
		quantity = position.Size
	}

	if position.Side == types.OrderSideSell {
		return (position.EntryPrice - exitPrice) * quantity
	}

	return (exitPrice - position.EntryPrice) * quantity
}

func (e *Engine) recordStrategyEntryFill(strategyName, symbol string, side types.OrderSide, entryPrice, size float64) {
	if strategyName == "" || symbol == "" || size <= 0 {
		return
	}

	e.strategyMutex.Lock()
	defer e.strategyMutex.Unlock()

	positionsByStrategy, exists := e.strategyPositions[strategyName]
	if !exists {
		positionsByStrategy = make(map[string]*strategyPosition)
		e.strategyPositions[strategyName] = positionsByStrategy
	}

	if existing, ok := positionsByStrategy[symbol]; ok && existing.Side == side {
		totalSize := existing.Size + size
		if totalSize > 0 {
			existing.EntryPrice = ((existing.EntryPrice * existing.Size) + (entryPrice * size)) / totalSize
		}
		existing.Size = totalSize
		existing.MarkPrice = entryPrice
		existing.UpdatedAt = time.Now()
		// 同步更新风控引擎的持仓
		e.riskEngine.UpdatePosition(&types.Position{
			Symbol:     symbol,
			Side:       side,
			Size:       existing.Size,
			EntryPrice: existing.EntryPrice,
			MarkPrice:  existing.MarkPrice,
		})
		return
	}

	positionsByStrategy[symbol] = &strategyPosition{
		Strategy:   strategyName,
		Symbol:     symbol,
		Side:       side,
		Size:       size,
		EntryPrice: entryPrice,
		MarkPrice:  entryPrice,
		UpdatedAt:  time.Now(),
	}
	// 同步更新风控引擎的持仓
	e.riskEngine.UpdatePosition(&types.Position{
		Symbol:     symbol,
		Side:       side,
		Size:       size,
		EntryPrice: entryPrice,
		MarkPrice:  entryPrice,
	})

	// 持久化活跃持仓记录
	if e.positionRepo != nil {
		orderID := ""
		e.mutex.RLock()
		for _, order := range e.orders {
			if order != nil && order.Metadata != nil {
				if s, ok := order.Metadata["strategy"].(string); ok && s == strategyName {
					if sym, ok := order.Metadata["symbol"].(string); ok && sym == symbol {
						orderID = order.ID
						break
					}
				}
			}
		}
		e.mutex.RUnlock()

		if err := e.positionRepo.Upsert(&PositionRecord{
			Strategy:   strategyName,
			Symbol:     symbol,
			Side:       string(side),
			Size:       size,
			EntryPrice: entryPrice,
			OrderID:    orderID,
		}); err != nil {
			logger.Warn("持久化活跃持仓记录失败",
				zap.String("strategy", strategyName),
				zap.String("symbol", symbol),
				zap.Error(err),
			)
		}
	}
}

func (e *Engine) applyStrategyExitFill(strategyName, symbol string, exitPrice, filledQty float64) (*strategyPosition, float64, bool) {
	if strategyName == "" || symbol == "" || filledQty <= 0 {
		return nil, 0, true
	}

	e.strategyMutex.Lock()
	defer e.strategyMutex.Unlock()

	positionsByStrategy, exists := e.strategyPositions[strategyName]
	if !exists {
		return nil, 0, true
	}

	position, exists := positionsByStrategy[symbol]
	if !exists || position == nil {
		return nil, 0, true
	}

	previous := *position
	realizedQty := math.Min(position.Size, filledQty)
	pnl := calculateStrategyPositionPnL(&previous, exitPrice, realizedQty)
	position.Size -= realizedQty
	position.MarkPrice = exitPrice
	position.UpdatedAt = time.Now()

	if position.Size <= 1e-9 {
		delete(positionsByStrategy, symbol)
		if len(positionsByStrategy) == 0 {
			delete(e.strategyPositions, strategyName)
		}

		// 持久化删除活跃持仓记录
		if e.positionRepo != nil {
			if err := e.positionRepo.Delete(strategyName, symbol); err != nil {
				logger.Warn("删除活跃持仓记录失败",
					zap.String("strategy", strategyName),
					zap.String("symbol", symbol),
					zap.Error(err),
				)
			}
		}

		return &previous, pnl, true
	}

	return &previous, pnl, false
}

func (e *Engine) getStrategyPositions(strategyName string) []*strategyPosition {
	e.strategyMutex.RLock()
	defer e.strategyMutex.RUnlock()

	positionsByStrategy, exists := e.strategyPositions[strategyName]
	if !exists {
		return nil
	}

	positions := make([]*strategyPosition, 0, len(positionsByStrategy))
	for _, position := range positionsByStrategy {
		copyPosition := *position
		positions = append(positions, &copyPosition)
	}

	return positions
}

func (e *Engine) getStrategyPosition(strategyName, symbol string) *strategyPosition {
	e.strategyMutex.RLock()
	defer e.strategyMutex.RUnlock()

	positionsByStrategy, exists := e.strategyPositions[strategyName]
	if !exists {
		return nil
	}

	position, exists := positionsByStrategy[symbol]
	if !exists || position == nil {
		return nil
	}

	copyPosition := *position
	return &copyPosition
}

func (e *Engine) applyRebalanceAllocations(allocations []strategy.WeightAllocation) {
	if !e.rebalanceConfig.Enabled || len(allocations) == 0 {
		return
	}

	targets := make(map[string]strategy.WeightAllocation, len(allocations))
	for _, allocation := range allocations {
		targets[allocation.Strategy] = allocation
	}

	adjustments := 0
	for strategyName, allocation := range targets {
		if adjustments >= e.rebalanceConfig.MaxPositionsPerCycle {
			break
		}

		positions := e.getStrategyPositions(strategyName)
		currentExposure := 0.0
		exposures := make(map[string]strategyExposure, len(positions))
		for _, position := range positions {
			exposure, price := e.resolveStrategyPositionExposure(position)
			exposures[position.Symbol] = strategyExposure{position: position, exposure: exposure, price: price}
			currentExposure += exposure
		}

		targetExposure := allocation.Amount
		allowedExposure := targetExposure * (1 + e.rebalanceConfig.DriftThreshold)
		underExposureThreshold := targetExposure * (1 - e.rebalanceConfig.DriftThreshold)
		if targetExposure <= 0 {
			allowedExposure = 0
			underExposureThreshold = 0
		}

		if len(positions) > 0 && currentExposure > allowedExposure {
			excessExposure := currentExposure - targetExposure
			sort.Slice(positions, func(i, j int) bool {
				return exposures[positions[i].Symbol].exposure > exposures[positions[j].Symbol].exposure
			})

			for _, position := range positions {
				if adjustments >= e.rebalanceConfig.MaxPositionsPerCycle || excessExposure <= 0 {
					break
				}
				if e.hasPendingRebalanceOrder(strategyName, position.Symbol) {
					continue
				}

				exposureState := exposures[position.Symbol]
				if exposureState.exposure <= 0 || exposureState.price <= 0 {
					continue
				}

				reductionExposure := math.Min(excessExposure, exposureState.exposure)
				quantity := math.Min(position.Size, reductionExposure/exposureState.price)
				if quantity <= 0 {
					continue
				}

				if err := e.placeRebalanceReduction(position, allocation, currentExposure, quantity); err != nil {
					logger.Warn("提交再平衡调仓失败",
						zap.String("strategy", strategyName),
						zap.String("symbol", position.Symbol),
						zap.Error(err),
					)
					continue
				}

				reducedExposure := quantity * exposureState.price
				currentExposure -= reducedExposure
				excessExposure -= reducedExposure
				adjustments++
			}
			continue
		}

		if e.rebalanceConfig.ReduceOnly || targetExposure <= 0 || currentExposure >= underExposureThreshold {
			continue
		}
		if adjustments >= e.rebalanceConfig.MaxPositionsPerCycle || e.hasPendingRebalanceEntry(strategyName) {
			continue
		}

		currentWeight := 0.0
		if e.bayesianAllocator != nil {
			if currentAllocation := e.bayesianAllocator.GetAllocation(strategyName); currentAllocation != nil {
				currentWeight = currentAllocation.Weight
			}
		}

		request := &strategy.RebalanceRequest{
			Strategy:        strategyName,
			CurrentWeight:   currentWeight,
			TargetWeight:    allocation.Weight,
			CurrentExposure: currentExposure,
			TargetExposure:  targetExposure,
			ShortfallAmount: targetExposure - currentExposure,
			Positions:       e.buildRebalancePositions(exposures),
			Timestamp:       time.Now(),
		}

		if err := e.requestRebalanceEntry(strategyName, request); err != nil {
			logger.Warn("提交再平衡增配请求失败",
				zap.String("strategy", strategyName),
				zap.Error(err),
			)
			continue
		}

		adjustments++
	}
}

func (e *Engine) resolveStrategyPositionExposure(position *strategyPosition) (float64, float64) {
	if position == nil {
		return 0, 0
	}

	price := position.MarkPrice
	if tick, err := e.exchange.GetTicker(position.Symbol); err == nil && tick != nil && tick.Price > 0 {
		price = tick.Price
	}
	if price <= 0 {
		price = position.EntryPrice
	}

	return position.Size * price, price
}

func (e *Engine) placeRebalanceReduction(position *strategyPosition, allocation strategy.WeightAllocation, currentExposure, quantity float64) error {
	if position == nil || position.Size <= 0 || quantity <= 0 {
		return risk.ErrInvalidSignal
	}

	orderSide := types.OrderSideSell
	if position.Side == types.OrderSideSell {
		orderSide = types.OrderSideBuy
	}

	order := &types.Order{
		Symbol:    position.Symbol,
		Side:      orderSide,
		Quantity:  quantity,
		Timestamp: time.Now(),
		Leverage:  1,
	}
	if e.rebalanceConfig.UseMarketOrders {
		order.Type = types.OrderTypeMarket
	} else {
		order.Type = types.OrderTypeLimit
		if tick, err := e.exchange.GetTicker(position.Symbol); err == nil && tick != nil {
			order.Price = tick.Price
		}
	}
	order.Metadata = map[string]interface{}{
		"source":                     "rebalance",
		"strategy":                   position.Strategy,
		"signal_type":                string(types.SignalTypeExit),
		"is_exit":                    true,
		"rebalance_target_weight":    allocation.Weight,
		"rebalance_target_amount":    allocation.Amount,
		"rebalance_current_exposure": currentExposure,
	}

	result, err := e.exchange.PlaceOrder(order)
	if err != nil {
		return err
	}

	e.trackOrder(result.OrderID, order, order.Metadata)
	e.updateMetrics()

	logger.Info("提交再平衡减仓订单",
		zap.String("strategy", position.Strategy),
		zap.String("symbol", position.Symbol),
		zap.Float64("quantity", quantity),
		zap.Float64("current_exposure", currentExposure),
		zap.Float64("target_amount", allocation.Amount),
		zap.String("order_id", result.OrderID),
	)

	return nil
}

func (e *Engine) hasPendingRebalanceOrder(strategyName, symbol string) bool {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	for _, order := range e.orders {
		if order == nil || order.Symbol != symbol || order.Metadata == nil {
			continue
		}
		if source, ok := order.Metadata["source"].(string); !ok || source != "rebalance" {
			continue
		}
		if trackedStrategy, ok := order.Metadata["strategy"].(string); ok && trackedStrategy == strategyName {
			return true
		}
	}

	return false
}

func (e *Engine) hasPendingRebalanceEntry(strategyName string) bool {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	for _, order := range e.orders {
		if order == nil || order.Metadata == nil {
			continue
		}
		if source, ok := order.Metadata["source"].(string); !ok || source != "rebalance_entry" {
			continue
		}
		if trackedStrategy, ok := order.Metadata["strategy"].(string); ok && trackedStrategy == strategyName {
			return true
		}
	}

	return false
}

func (e *Engine) buildRebalancePositions(exposures map[string]strategyExposure) []strategy.RebalancePosition {
	positions := make([]strategy.RebalancePosition, 0, len(exposures))
	for _, exposure := range exposures {
		if exposure.position == nil {
			continue
		}
		positions = append(positions, strategy.RebalancePosition{
			Symbol:    exposure.position.Symbol,
			Side:      exposure.position.Side,
			Size:      exposure.position.Size,
			MarkPrice: exposure.price,
			Exposure:  exposure.exposure,
		})
	}
	return positions
}

func (e *Engine) requestRebalanceEntry(strategyName string, request *strategy.RebalanceRequest) error {
	if e.strategyEngine == nil || request == nil || request.ShortfallAmount <= 0 {
		return nil
	}
	if e.isRebalanceCircuitOpen() {
		return ErrRebalanceCircuitOpen
	}

	decision, err := e.strategyEngine.ConfirmRebalanceEntry(strategyName, request)
	if err != nil {
		return err
	}
	if decision == nil {
		return nil
	}
	if !decision.Approved {
		logger.Info("策略拒绝再平衡增配请求",
			zap.String("strategy", strategyName),
			zap.String("reason", decision.RejectReason),
			zap.Float64("recommended_price", decision.RecommendedPrice),
			zap.Float64("recommended_quantity", decision.RecommendedQuantity),
		)
		return nil
	}

	plan := decision.Plan
	if len(plan) == 0 && decision.Signal != nil {
		plan = []strategy.RebalancePlanStep{{
			Label:               "primary",
			Signal:              decision.Signal,
			RecommendedPrice:    decision.RecommendedPrice,
			RecommendedQuantity: decision.RecommendedQuantity,
		}}
	}
	if len(plan) == 0 {
		return nil
	}

	account, err := e.exchange.GetAccount()
	if err != nil {
		return err
	}

	executions := make([]rebalancePlanExecution, 0, len(plan))
	for index, step := range plan {
		signal := step.Signal
		if signal == nil {
			continue
		}
		signal.Strategy = strategyName
		if step.RecommendedPrice > 0 {
			signal.Price = step.RecommendedPrice
		}
		if step.RecommendedQuantity > 0 {
			signal.Quantity = step.RecommendedQuantity
		}

		metadata := signalMetadataMap(signal)
		metadata["source"] = "rebalance_entry"
		metadata["strategy"] = strategyName
		metadata["requested_shortfall_amount"] = request.ShortfallAmount
		metadata["target_exposure"] = request.TargetExposure
		metadata["current_exposure"] = request.CurrentExposure
		metadata["rebalance_plan_id"] = fmt.Sprintf("%s-%d", strategyName, request.Timestamp.UnixNano())
		metadata["rebalance_plan_index"] = index
		metadata["rebalance_plan_count"] = len(plan)
		if step.Label != "" {
			metadata["rebalance_plan_label"] = step.Label
		}
		if step.RecommendedPrice > 0 {
			metadata["recommended_price"] = step.RecommendedPrice
		}
		if step.RecommendedQuantity > 0 {
			metadata["recommended_quantity"] = step.RecommendedQuantity
		}
		signal.Metadata = metadata

		result, execErr := e.Execute(signal, account.TotalAvailable)
		if execErr != nil {
			if rollbackErr := e.rollbackRebalancePlan(strategyName, executions); rollbackErr != nil {
				logger.Error("多腿再平衡回滚失败",
					zap.String("strategy", strategyName),
					zap.Error(rollbackErr),
				)
				return fmt.Errorf("rebalance step failed: %w; rollback failed: %v", execErr, rollbackErr)
			}
			return execErr
		}
		if result != nil {
			executions = append(executions, rebalancePlanExecution{orderID: result.OrderID, step: step})
		}
	}

	return nil
}

func (e *Engine) rollbackRebalancePlan(strategyName string, executions []rebalancePlanExecution) error {
	var firstErr error
	for index := len(executions) - 1; index >= 0; index-- {
		execution := executions[index]
		if execution.orderID == "" {
			continue
		}

		if err := e.CancelOrder(execution.orderID); err == nil {
			continue
		} else if firstErr == nil {
			firstErr = err
		}

		orderInfo, err := e.exchange.GetOrder(execution.orderID)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if orderInfo == nil {
			continue
		}
		if orderInfo.Status != types.OrderStatusFilled && orderInfo.Status != types.OrderStatusPartially {
			continue
		}
		localOrder := e.GetOrder(execution.orderID)
		e.processObservedOrderFill(execution.orderID, localOrder, orderInfo, true)

		if err := e.placeCompensatingRebalanceOrder(strategyName, execution.step, orderInfo); err != nil {
			e.openRebalanceCircuit(strategyName, execution.step, err)
			if firstErr == nil {
				firstErr = err
			} else {
				firstErr = fmt.Errorf("%v; compensation failed: %w", firstErr, err)
			}
		}
	}
	return firstErr
}

func (e *Engine) isRebalanceCircuitOpen() bool {
	e.mutex.Lock()
	open, resetEvent := e.isRebalanceCircuitOpenLocked(time.Now())
	e.mutex.Unlock()
	e.notifyRebalanceReset(resetEvent)
	return open
}

func (e *Engine) openRebalanceCircuit(strategyName string, step strategy.RebalancePlanStep, err error) {
	e.mutex.Lock()
	now := time.Now()
	e.metrics["rebalance_circuit_open"] = true
	e.metrics["rebalance_circuit_strategy"] = strategyName
	e.metrics["rebalance_circuit_reason"] = err.Error()
	e.metrics["rebalance_circuit_opened_at"] = now
	if e.rebalanceConfig.CircuitAutoReset {
		e.metrics["rebalance_circuit_cooldown_until"] = now.Add(e.rebalanceConfig.CircuitCooldown)
	} else {
		delete(e.metrics, "rebalance_circuit_cooldown_until")
	}
	if step.Label != "" {
		e.metrics["rebalance_circuit_step"] = step.Label
	} else {
		delete(e.metrics, "rebalance_circuit_step")
	}
	if count, ok := e.metrics["rebalance_rollback_failures"].(int); ok {
		e.metrics["rebalance_rollback_failures"] = count + 1
	} else {
		e.metrics["rebalance_rollback_failures"] = 1
	}
	state, _ := e.getRebalanceCircuitStateLocked(now)
	e.mutex.Unlock()

	logger.Error("再平衡补偿失败，打开熔断保护",
		zap.String("strategy", strategyName),
		zap.String("step", step.Label),
		zap.Error(err),
	)
	context := &AlertContext{
		Labels: map[string]string{
			"component": "execution",
			"event":     "rebalance_circuit_open",
			"strategy":  strategyName,
			"step":      step.Label,
			"reason":    err.Error(),
		},
		Details: map[string]interface{}{
			"strategy":       strategyName,
			"step":           step.Label,
			"reason":         err.Error(),
			"signal_symbol":  stepSignalSymbol(step),
			"signal_type":    stepSignalType(step),
			"opened_at":      now,
			"cooldown_until": state.CooldownUntil,
		},
	}
	message := fmt.Sprintf("策略 %s 的再平衡步骤 %s 补偿失败: %v", strategyName, step.Label, err)
	e.emitAlert(AlertLevelCritical, "再平衡熔断已打开", message, context)
	e.emitRebalanceEvent(RebalanceEventOpen, strategyName, step.Label, err.Error(), message, context, state, now)
}

func (e *Engine) isRebalanceCircuitOpenLocked(now time.Time) (bool, *rebalanceResetNotification) {
	open := metricBool(e.metrics, "rebalance_circuit_open")
	if !open {
		return false, nil
	}
	if !e.rebalanceConfig.CircuitAutoReset || e.rebalanceConfig.CircuitCooldown <= 0 {
		return true, nil
	}
	openedAt, ok := metricTime(e.metrics, "rebalance_circuit_opened_at")
	if !ok || openedAt.IsZero() {
		return true, nil
	}
	if now.Sub(openedAt) < e.rebalanceConfig.CircuitCooldown {
		return true, nil
	}
	return false, e.resetRebalanceCircuitLocked("cooldown_elapsed", now)
}

func (e *Engine) getRebalanceCircuitStateLocked(now time.Time) (RebalanceCircuitState, *rebalanceResetNotification) {
	open, resetEvent := e.isRebalanceCircuitOpenLocked(now)
	state := RebalanceCircuitState{
		Open:      open,
		AutoReset: e.rebalanceConfig.CircuitAutoReset,
		Cooldown:  e.rebalanceConfig.CircuitCooldown,
	}
	if strategyName, ok := e.metrics["rebalance_circuit_strategy"].(string); ok {
		state.Strategy = strategyName
	}
	if step, ok := e.metrics["rebalance_circuit_step"].(string); ok {
		state.Step = step
	}
	if reason, ok := e.metrics["rebalance_circuit_reason"].(string); ok {
		state.Reason = reason
	}
	if openedAt, ok := metricTime(e.metrics, "rebalance_circuit_opened_at"); ok {
		state.OpenedAt = openedAt
		if e.rebalanceConfig.CircuitAutoReset && e.rebalanceConfig.CircuitCooldown > 0 {
			state.CooldownUntil = openedAt.Add(e.rebalanceConfig.CircuitCooldown)
		}
	}
	if cooldownUntil, ok := metricTime(e.metrics, "rebalance_circuit_cooldown_until"); ok {
		state.CooldownUntil = cooldownUntil
	}
	if lastResetAt, ok := metricTime(e.metrics, "rebalance_circuit_last_reset_at"); ok {
		state.LastResetAt = lastResetAt
	}
	if lastResetReason, ok := e.metrics["rebalance_circuit_last_reset_reason"].(string); ok {
		state.LastResetReason = lastResetReason
	}
	if resetEvent != nil {
		resetEvent.state = state
	}
	return state, resetEvent
}

func (e *Engine) resetRebalanceCircuitLocked(reason string, now time.Time) *rebalanceResetNotification {
	strategyName, _ := e.metrics["rebalance_circuit_strategy"].(string)
	step, _ := e.metrics["rebalance_circuit_step"].(string)
	previousOpenAt, _ := metricTime(e.metrics, "rebalance_circuit_opened_at")

	e.metrics["rebalance_circuit_open"] = false
	delete(e.metrics, "rebalance_circuit_strategy")
	delete(e.metrics, "rebalance_circuit_reason")
	delete(e.metrics, "rebalance_circuit_step")
	delete(e.metrics, "rebalance_circuit_opened_at")
	delete(e.metrics, "rebalance_circuit_cooldown_until")
	e.metrics["rebalance_circuit_last_reset_at"] = now
	if reason != "" {
		e.metrics["rebalance_circuit_last_reset_reason"] = reason
	} else {
		delete(e.metrics, "rebalance_circuit_last_reset_reason")
	}
	state := RebalanceCircuitState{
		Open:            false,
		Strategy:        strategyName,
		Step:            step,
		LastResetAt:     now,
		LastResetReason: reason,
		AutoReset:       e.rebalanceConfig.CircuitAutoReset,
		Cooldown:        e.rebalanceConfig.CircuitCooldown,
	}
	return &rebalanceResetNotification{
		strategyName:   strategyName,
		step:           step,
		reason:         reason,
		occurredAt:     now,
		previousOpenAt: previousOpenAt,
		state:          state,
	}
}

func (e *Engine) notifyRebalanceReset(event *rebalanceResetNotification) {
	if event == nil {
		return
	}

	logger.Info("再平衡熔断已重置",
		zap.String("strategy", event.strategyName),
		zap.String("step", event.step),
		zap.String("reason", event.reason),
	)
	context := &AlertContext{
		Labels: map[string]string{
			"component": "execution",
			"event":     "rebalance_circuit_reset",
			"strategy":  event.strategyName,
			"step":      event.step,
			"reason":    event.reason,
		},
		Details: map[string]interface{}{
			"strategy":         event.strategyName,
			"step":             event.step,
			"reason":           event.reason,
			"reset_at":         event.occurredAt,
			"reset_mode":       resetModeFromReason(event.reason),
			"previous_open_at": event.previousOpenAt,
		},
	}
	e.emitRebalanceEvent(RebalanceEventReset, event.strategyName, event.step, event.reason, fmt.Sprintf("再平衡熔断已重置: %s", event.reason), context, event.state, event.occurredAt)
}

func metricBool(metrics map[string]interface{}, key string) bool {
	value, _ := metrics[key].(bool)
	return value
}

func metricTime(metrics map[string]interface{}, key string) (time.Time, bool) {
	value, ok := metrics[key]
	if !ok {
		return time.Time{}, false
	}
	timestamp, ok := value.(time.Time)
	return timestamp, ok
}

func (e *Engine) placeCompensatingRebalanceOrder(strategyName string, step strategy.RebalancePlanStep, orderInfo *types.Order) error {
	if orderInfo == nil {
		return nil
	}
	quantity := orderInfo.FilledQty
	if quantity <= 0 {
		quantity = orderInfo.Quantity
	}
	if quantity <= 0 {
		return nil
	}

	compensatingSide := types.OrderSideBuy
	if orderInfo.Side == types.OrderSideBuy {
		compensatingSide = types.OrderSideSell
	}

	order := &types.Order{
		Symbol:    orderInfo.Symbol,
		Side:      compensatingSide,
		Type:      types.OrderTypeMarket,
		Quantity:  quantity,
		Leverage:  1,
		Timestamp: time.Now(),
		Metadata: map[string]interface{}{
			"source":               "rebalance_rollback",
			"strategy":             strategyName,
			"compensates_order_id": orderInfo.ID,
			"rebalance_plan_label": step.Label,
			"signal_type":          string(step.Signal.Type),
			"rollback_reason":      "multi_leg_failure",
		},
	}

	result, err := e.exchange.PlaceOrder(order)
	if err != nil {
		return err
	}
	e.trackOrder(result.OrderID, order, order.Metadata)
	e.updateMetrics()

	logger.Warn("提交再平衡补偿单",
		zap.String("strategy", strategyName),
		zap.String("symbol", order.Symbol),
		zap.String("side", string(order.Side)),
		zap.Float64("quantity", order.Quantity),
		zap.String("compensates_order_id", orderInfo.ID),
	)
	e.emitAlert(AlertLevelWarning, "提交再平衡补偿单", fmt.Sprintf("策略 %s 为订单 %s 提交补偿单 %s %s %.8f", strategyName, orderInfo.ID, order.Symbol, order.Side, order.Quantity), &AlertContext{
		Labels: map[string]string{
			"component": "execution",
			"event":     "rebalance_compensation_order",
			"strategy":  strategyName,
			"step":      step.Label,
			"reason":    orderMetadataString(order, "rollback_reason", "multi_leg_failure"),
		},
		Details: map[string]interface{}{
			"strategy":             strategyName,
			"step":                 step.Label,
			"reason":               orderMetadataString(order, "rollback_reason", "multi_leg_failure"),
			"symbol":               order.Symbol,
			"side":                 string(order.Side),
			"quantity":             order.Quantity,
			"compensates_order_id": orderInfo.ID,
			"plan_id":              orderMetadataString(orderInfo, "rebalance_plan_id", orderMetadataString(order, "rebalance_plan_id", "")),
		},
	})

	return nil
}

func (e *Engine) reconcilePendingRebalancePlans(reason string) error {
	orders := e.GetOrders()
	plans := make(map[string][]trackedRebalanceOrder)
	for orderID, order := range orders {
		if !isRebalanceEntryOrder(order) {
			continue
		}
		planKey := rebalancePlanKey(order, orderID)
		plans[planKey] = append(plans[planKey], trackedRebalanceOrder{orderID: orderID, order: order})
	}

	var firstErr error
	for _, group := range plans {
		needsRecovery := false
		for _, tracked := range group {
			orderInfo, err := e.exchange.GetOrder(tracked.orderID)
			if err != nil {
				if firstErr == nil {
					firstErr = err
				}
				needsRecovery = true
				break
			}
			if orderInfo == nil || orderInfo.Status == types.OrderStatusPending || orderInfo.Status == types.OrderStatusPartially {
				needsRecovery = true
				break
			}
		}
		if !needsRecovery {
			continue
		}
		strategyName := ""
		if len(group) > 0 {
			strategyName = orderMetadataString(group[0].order, "strategy", "")
		}
		if err := e.recoverTrackedRebalanceOrders(group, strategyName, reason); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	return firstErr
}

func (e *Engine) recoverRebalancePlan(orderID, strategyName, reason string) error {
	group := e.collectTrackedRebalancePlan(orderID)
	if len(group) == 0 {
		return nil
	}
	return e.recoverTrackedRebalanceOrders(group, strategyName, reason)
}

func (e *Engine) recoverTrackedRebalanceOrders(group []trackedRebalanceOrder, strategyName, reason string) error {
	if len(group) == 0 {
		return nil
	}
	step := orderMetadataString(group[0].order, "rebalance_plan_label", "")
	planID := orderMetadataString(group[0].order, "rebalance_plan_id", rebalancePlanKey(group[0].order, group[0].orderID))
	currentState := e.GetRebalanceCircuitState()
	startedContext := &AlertContext{
		Labels: map[string]string{
			"component": "execution",
			"event":     "rebalance_recover_started",
			"strategy":  strategyName,
			"step":      step,
			"reason":    reason,
		},
		Details: map[string]interface{}{
			"strategy":    strategyName,
			"step":        step,
			"reason":      reason,
			"plan_id":     planID,
			"plan_orders": rebalanceOrderIDs(group),
			"order_count": len(group),
			"phase":       "started",
		},
	}
	startedMessage := fmt.Sprintf("策略 %s 检测到未完成再平衡计划，原因=%s，开始统一回收", strategyName, reason)
	e.emitAlert(AlertLevelWarning, "启动/运行时再平衡恢复", startedMessage, startedContext)
	e.emitRebalanceEvent(RebalanceEventRecoverStarted, strategyName, step, reason, startedMessage, startedContext, currentState, time.Now())

	var firstErr error
	for _, tracked := range group {
		localOrder := tracked.order
		orderInfo, err := e.exchange.GetOrder(tracked.orderID)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}

		status := types.OrderStatusPending
		if orderInfo != nil {
			status = orderInfo.Status
		}

		if status == types.OrderStatusPending || status == types.OrderStatusPartially || orderInfo == nil {
			if cancelErr := e.CancelOrder(tracked.orderID); cancelErr != nil {
				if latest, latestErr := e.exchange.GetOrder(tracked.orderID); latestErr == nil && latest != nil {
					orderInfo = latest
					status = latest.Status
				} else if firstErr == nil {
					firstErr = cancelErr
				}
			} else if orderInfo == nil {
				orderInfo = &types.Order{
					ID:       tracked.orderID,
					Symbol:   localOrder.Symbol,
					Side:     localOrder.Side,
					Type:     localOrder.Type,
					Quantity: localOrder.Quantity,
					Price:    localOrder.Price,
					Status:   types.OrderStatusCancelled,
				}
				status = orderInfo.Status
			}
		}

		if orderInfo != nil && (status == types.OrderStatusFilled || status == types.OrderStatusPartially) {
			e.processObservedOrderFill(tracked.orderID, localOrder, orderInfo, true)
			step := rebalancePlanStepFromOrder(localOrder)
			if err := e.placeCompensatingRebalanceOrder(orderMetadataString(localOrder, "strategy", strategyName), step, orderInfo); err != nil {
				e.openRebalanceCircuit(orderMetadataString(localOrder, "strategy", strategyName), step, err)
				if firstErr == nil {
					firstErr = err
				} else {
					firstErr = fmt.Errorf("%v; compensation failed: %w", firstErr, err)
				}
			}
			continue
		}

		if orderInfo != nil && (status == types.OrderStatusCancelled || status == types.OrderStatusFailed) {
			e.handleOrderCompleted(tracked.orderID, orderInfo)
		}
	}

	resultContext := &AlertContext{
		Labels: map[string]string{
			"component": "execution",
			"strategy":  strategyName,
			"step":      step,
			"reason":    reason,
		},
		Details: map[string]interface{}{
			"strategy":    strategyName,
			"step":        step,
			"reason":      reason,
			"plan_id":     planID,
			"plan_orders": rebalanceOrderIDs(group),
			"order_count": len(group),
		},
	}
	if firstErr != nil {
		resultContext.Labels["event"] = "rebalance_recover_failed"
		resultContext.Details["phase"] = "failed"
		resultContext.Details["error"] = firstErr.Error()
		failureMessage := fmt.Sprintf("策略 %s 的再平衡恢复失败，原因=%s，错误=%v", strategyName, reason, firstErr)
		e.emitAlert(AlertLevelError, "启动/运行时再平衡恢复失败", failureMessage, resultContext)
		e.emitRebalanceEvent(RebalanceEventRecoverFailed, strategyName, step, reason, failureMessage, resultContext, e.GetRebalanceCircuitState(), time.Now())
		return firstErr
	}

	resultContext.Labels["event"] = "rebalance_recover_succeeded"
	resultContext.Details["phase"] = "succeeded"
	successMessage := fmt.Sprintf("策略 %s 的再平衡恢复已完成，原因=%s", strategyName, reason)
	e.emitAlert(AlertLevelInfo, "启动/运行时再平衡恢复完成", successMessage, resultContext)
	e.emitRebalanceEvent(RebalanceEventRecoverSuccess, strategyName, step, reason, successMessage, resultContext, e.GetRebalanceCircuitState(), time.Now())

	return nil
}

func (e *Engine) collectTrackedRebalancePlan(orderID string) []trackedRebalanceOrder {
	orders := e.GetOrders()
	baseOrder, ok := orders[orderID]
	if !ok || !isRebalanceEntryOrder(baseOrder) {
		return nil
	}
	planKey := rebalancePlanKey(baseOrder, orderID)
	group := make([]trackedRebalanceOrder, 0)
	for trackedID, order := range orders {
		if !isRebalanceEntryOrder(order) {
			continue
		}
		if rebalancePlanKey(order, trackedID) != planKey {
			continue
		}
		group = append(group, trackedRebalanceOrder{orderID: trackedID, order: order})
	}
	return group
}

func (e *Engine) updateTrackedOrderFillState(orderID string, exchangeOrder *types.Order, processedFilled float64, final bool) {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	trackedOrder, ok := e.orders[orderID]
	if !ok || trackedOrder == nil {
		return
	}
	trackedOrder.Status = exchangeOrder.Status
	trackedOrder.FilledQty = processedFilled
	trackedOrder.AveragePrice = exchangeOrder.AveragePrice
	if trackedOrder.Metadata == nil {
		trackedOrder.Metadata = map[string]interface{}{}
	}
	trackedOrder.Metadata["processed_filled_quantity"] = processedFilled
	if final {
		delete(e.orders, orderID)
	}
}

func resolveObservedFilledQty(exchangeOrder *types.Order, localOrder *types.Order) float64 {
	if exchangeOrder != nil {
		if exchangeOrder.FilledQty > 0 {
			return exchangeOrder.FilledQty
		}
		if exchangeOrder.Status == types.OrderStatusFilled && exchangeOrder.Quantity > 0 {
			return exchangeOrder.Quantity
		}
	}
	if localOrder != nil {
		if localOrder.FilledQty > 0 {
			return localOrder.FilledQty
		}
		if localOrder.Quantity > 0 {
			return localOrder.Quantity
		}
	}
	return 0
}

func rebalancePlanKey(order *types.Order, orderID string) string {
	if order == nil {
		return orderID
	}
	if planID := orderMetadataString(order, "rebalance_plan_id", ""); planID != "" {
		return planID
	}
	return orderID
}

func isRebalanceEntryOrder(order *types.Order) bool {
	return orderMetadataString(order, "source", "") == "rebalance_entry"
}

func isMultiLegRebalanceOrder(order *types.Order) bool {
	return isRebalanceEntryOrder(order) && orderMetadataInt(order, "rebalance_plan_count", 0) > 1
}

func rebalancePlanStepFromOrder(order *types.Order) strategy.RebalancePlanStep {
	price := orderMetadataFloat(order, "recommended_price", order.Price)
	quantity := orderMetadataFloat(order, "recommended_quantity", order.Quantity)
	signalType := types.SignalType(orderMetadataString(order, "signal_type", string(types.SignalTypeBuy)))
	return strategy.RebalancePlanStep{
		Label:               orderMetadataString(order, "rebalance_plan_label", ""),
		RecommendedPrice:    price,
		RecommendedQuantity: quantity,
		Signal: &types.Signal{
			Symbol:    order.Symbol,
			Type:      signalType,
			Price:     price,
			Quantity:  quantity,
			Timestamp: order.Timestamp,
		},
	}
}

func orderMetadataString(order *types.Order, key, defaultValue string) string {
	if order == nil || order.Metadata == nil {
		return defaultValue
	}
	value, ok := order.Metadata[key].(string)
	if !ok {
		return defaultValue
	}
	return value
}

func orderMetadataBool(order *types.Order, key string, defaultValue bool) bool {
	if order == nil || order.Metadata == nil {
		return defaultValue
	}
	value, ok := order.Metadata[key].(bool)
	if !ok {
		return defaultValue
	}
	return value
}

func orderMetadataFloat(order *types.Order, key string, defaultValue float64) float64 {
	if order == nil || order.Metadata == nil {
		return defaultValue
	}
	return metricAsFloat64(order.Metadata[key], defaultValue)
}

func metricAsFloat64(value interface{}, defaultValue float64) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	default:
		return defaultValue
	}
}

func orderMetadataInt(order *types.Order, key string, defaultValue int) int {
	if order == nil || order.Metadata == nil {
		return defaultValue
	}
	switch value := order.Metadata[key].(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		return int(value)
	default:
		return defaultValue
	}
}

func rebalanceOrderIDs(group []trackedRebalanceOrder) []string {
	if len(group) == 0 {
		return nil
	}
	ids := make([]string, 0, len(group))
	for _, tracked := range group {
		ids = append(ids, tracked.orderID)
	}
	sort.Strings(ids)
	return ids
}

func stepSignalSymbol(step strategy.RebalancePlanStep) string {
	if step.Signal == nil {
		return ""
	}
	return step.Signal.Symbol
}

func stepSignalType(step strategy.RebalancePlanStep) string {
	if step.Signal == nil {
		return ""
	}
	return string(step.Signal.Type)
}

func resetModeFromReason(reason string) string {
	if reason == "cooldown_elapsed" {
		return "automatic"
	}
	return "manual"
}

func cloneStringMap(source map[string]string) map[string]string {
	if source == nil {
		return nil
	}
	cloned := make(map[string]string, len(source))
	for key, value := range source {
		cloned[key] = value
	}
	return cloned
}

func signalMetadataMap(signal *types.Signal) map[string]interface{} {
	if signal == nil {
		return map[string]interface{}{}
	}
	if signal.Metadata == nil {
		signal.Metadata = map[string]interface{}{}
	}
	metadata, ok := signal.Metadata.(map[string]interface{})
	if !ok {
		metadata = map[string]interface{}{}
		signal.Metadata = metadata
	}
	return metadata
}

func estimateOrderBookFill(order *types.Order, orderBook *types.OrderBook) (float64, float64, float64, bool) {
	if order == nil || orderBook == nil || order.Quantity <= 0 {
		return 0, 0, 0, false
	}

	var levels []types.OrderBookLevel
	if order.Side == types.OrderSideBuy {
		levels = orderBook.Asks
	} else {
		levels = orderBook.Bids
	}

	if len(levels) == 0 {
		return 0, 0, 0, false
	}

	bestPrice := levels[0].Price
	remaining := order.Quantity
	totalValue := 0.0
	availableQty := 0.0

	for _, level := range levels {
		if level.Price <= 0 || level.Size <= 0 {
			continue
		}

		fillQty := math.Min(remaining, level.Size)
		totalValue += fillQty * level.Price
		availableQty += level.Size
		remaining -= fillQty

		if remaining <= 0 {
			break
		}
	}

	if remaining > 0 {
		return 0, bestPrice, availableQty, false
	}

	return totalValue / order.Quantity, bestPrice, availableQty, true
}

func calculateEstimatedSlippage(side types.OrderSide, bestPrice, avgPrice float64) float64 {
	if bestPrice <= 0 || avgPrice <= 0 {
		return 0
	}

	if side == types.OrderSideSell {
		return math.Max(0, (bestPrice-avgPrice)/bestPrice)
	}

	return math.Max(0, (avgPrice-bestPrice)/bestPrice)
}

func signalMaxSlippage(signal *types.Signal, defaultValue float64) float64 {
	if signal == nil || signal.Metadata == nil {
		return defaultValue
	}

	metadata, ok := signal.Metadata.(map[string]interface{})
	if !ok {
		return defaultValue
	}

	value, ok := metadata["max_slippage"]
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
	case int64:
		return float64(typed)
	default:
		return defaultValue
	}
}

func (e *Engine) syncRiskPosition(symbol string) {
	positions, err := e.exchange.GetPositions()
	if err != nil {
		logger.Warn("同步风控持仓失败",
			zap.String("symbol", symbol),
			zap.Error(err),
		)
		return
	}

	for _, position := range positions {
		if position.Symbol == symbol {
			e.riskEngine.UpdatePosition(position)
			return
		}
	}

	e.riskEngine.RemovePosition(symbol)
}

func (e *Engine) SetTakeProfitConfig(config *TakeProfitConfig) {
	if e.takeProfitManager == nil {
		return
	}
	e.takeProfitManager.SetConfig(config)
}

func (e *Engine) GetTakeProfitConfig() *TakeProfitConfig {
	if e.takeProfitManager == nil {
		return nil
	}
	return e.takeProfitManager.GetConfig()
}

func (e *Engine) StartTakeProfitMonitor() {
	e.tpMonitorStop = make(chan struct{})
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-e.tpMonitorStop:
				return
			case <-ticker.C:
				e.MonitorTakeProfit()
			}
		}
	}()

	logger.Info("止盈监控已启动")
}

func (e *Engine) StartOrderMonitor() {
	e.orderMonitorStop = make(chan struct{})
	go func() {
		// 调整轮询间隔至 2 秒，避免触发 OKX API 限频（私有接口限制：10 次/2s）
		// 添加 jitter 防止多实例同时请求
		baseInterval := 2 * time.Second
		jitter := time.Duration(float64(baseInterval) * 0.1) // 10% jitter
		initialDelay := time.Duration(float64(baseInterval) * 0.5) // 随机初始延迟 0-1s

		// 首次延迟避免启动时集中请求
		time.Sleep(initialDelay)

		ticker := time.NewTicker(baseInterval + jitter)
		defer ticker.Stop()

		for {
			select {
			case <-e.orderMonitorStop:
				return
			case <-ticker.C:
				e.MonitorOrders()
			}
		}
	}()

	logger.Info("订单监控已启动（轮询间隔：2 秒，含 10% jitter）")
}

func (e *Engine) StopTakeProfitMonitor() {
	if e.tpMonitorStop != nil {
		close(e.tpMonitorStop)
		e.tpMonitorStop = nil
		logger.Info("止盈监控已停止")
	}
}

func (e *Engine) StopOrderMonitor() {
	if e.orderMonitorStop != nil {
		close(e.orderMonitorStop)
		e.orderMonitorStop = nil
		logger.Info("订单监控已停止")
	}
}

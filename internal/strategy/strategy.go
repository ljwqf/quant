package strategy

import (
	"sync"
	"time"

	"github.com/ljwqf/quant/pkg/logger"
	"github.com/ljwqf/quant/pkg/types"
	"go.uber.org/zap"
)

type Strategy interface {
	Name() string
	Init(params map[string]interface{}) error
	OnTick(tick *types.Tick) (*types.Signal, error)
	OnBar(bar *types.Bar) (*types.Signal, error)
	OnOrderBook(orderBook *types.OrderBook) (*types.Signal, error)
	GetParams() map[string]interface{}
	SetParams(params map[string]interface{})
	GetMetrics() map[string]interface{}
	OnPositionFilled(symbol string, side types.OrderSide, entryPrice, size float64)
	OnPositionReduced(symbol string, exitPrice, pnl, remainingSize float64)
	OnPositionClosed(symbol string, exitPrice, pnl float64)
	ConfirmRebalanceEntry(request *RebalanceRequest) (*RebalanceDecision, error)
}

type RebalanceRequest struct {
	Strategy        string
	CurrentWeight   float64
	TargetWeight    float64
	CurrentExposure float64
	TargetExposure  float64
	ShortfallAmount float64
	Positions       []RebalancePosition
	Timestamp       time.Time
}

type RebalancePosition struct {
	Symbol    string
	Side      types.OrderSide
	Size      float64
	MarkPrice float64
	Exposure  float64
}

type RebalanceDecision struct {
	Approved            bool
	RejectReason        string
	RecommendedPrice    float64
	RecommendedQuantity float64
	Signal              *types.Signal
	Plan                []RebalancePlanStep
}

type RebalancePlanStep struct {
	Label               string
	Signal              *types.Signal
	RecommendedPrice    float64
	RecommendedQuantity float64
}

type Engine struct {
	strategies map[string]Strategy
	params     map[string]map[string]interface{}
	mutex      sync.RWMutex
}

func NewEngine() *Engine {
	return &Engine{
		strategies: make(map[string]Strategy),
		params:     make(map[string]map[string]interface{}),
	}
}

func (e *Engine) AddStrategy(name string, strategy Strategy, params map[string]interface{}) error {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	e.strategies[name] = strategy
	e.params[name] = params
	return strategy.Init(params)
}

func (e *Engine) RemoveStrategy(name string) {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	delete(e.strategies, name)
	delete(e.params, name)
}

func (e *Engine) GetStrategy(name string) Strategy {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	return e.strategies[name]
}

func (e *Engine) GetStrategies() map[string]Strategy {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	result := make(map[string]Strategy)
	for k, v := range e.strategies {
		result[k] = v
	}
	return result
}

type StrategyResult struct {
	Signals []*types.Signal
	Errors  []error
}

func (e *Engine) OnTick(tick *types.Tick) *StrategyResult {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	result := &StrategyResult{
		Signals: make([]*types.Signal, 0),
		Errors:  make([]error, 0),
	}

	for name, strategy := range e.strategies {
		signal, err := strategy.OnTick(tick)
		if err != nil {
			logger.Error("策略 OnTick 失败",
				zap.String("strategy", name),
				zap.Error(err))
			result.Errors = append(result.Errors, err)
			continue
		}
		if signal != nil {
			signal.Strategy = name
			result.Signals = append(result.Signals, signal)
		}
	}
	return result
}

func (e *Engine) OnBar(bar *types.Bar) *StrategyResult {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	result := &StrategyResult{
		Signals: make([]*types.Signal, 0),
		Errors:  make([]error, 0),
	}

	for name, strategy := range e.strategies {
		signal, err := strategy.OnBar(bar)
		if err != nil {
			logger.Error("策略 OnBar 失败",
				zap.String("strategy", name),
				zap.Error(err))
			result.Errors = append(result.Errors, err)
			continue
		}
		if signal != nil {
			signal.Strategy = name
			result.Signals = append(result.Signals, signal)
		}
	}
	return result
}

func (e *Engine) OnOrderBook(orderBook *types.OrderBook) *StrategyResult {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	result := &StrategyResult{
		Signals: make([]*types.Signal, 0),
		Errors:  make([]error, 0),
	}

	for name, strategy := range e.strategies {
		signal, err := strategy.OnOrderBook(orderBook)
		if err != nil {
			logger.Error("策略 OnOrderBook 失败",
				zap.String("strategy", name),
				zap.Error(err))
			result.Errors = append(result.Errors, err)
			continue
		}
		if signal != nil {
			signal.Strategy = name
			result.Signals = append(result.Signals, signal)
		}
	}
	return result
}

// GetStrategyErrors 获取所有策略的错误信息
func (e *Engine) GetStrategyErrors() map[string][]error {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	errors := make(map[string][]error)
	for name, strategy := range e.strategies {
		// 这里可以添加策略内部的错误收集机制
		// 目前只在调用方法时捕获错误
		_ = strategy
		errors[name] = make([]error, 0)
	}
	return errors
}

func (e *Engine) GetStrategyParams(name string) map[string]interface{} {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	params, exists := e.params[name]
	if !exists {
		return nil
	}

	result := make(map[string]interface{})
	for k, v := range params {
		result[k] = v
	}
	return result
}

func (e *Engine) SetStrategyParams(name string, params map[string]interface{}) error {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	strategy, exists := e.strategies[name]
	if !exists {
		return nil
	}

	strategy.SetParams(params)
	e.params[name] = params
	return strategy.Init(params)
}

func (e *Engine) GetStrategyMetrics(name string) map[string]interface{} {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	strategy, exists := e.strategies[name]
	if !exists {
		return nil
	}
	return strategy.GetMetrics()
}

func (e *Engine) GetAllStrategyMetrics() map[string]map[string]interface{} {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	metrics := make(map[string]map[string]interface{})
	for name, strategy := range e.strategies {
		metrics[name] = strategy.GetMetrics()
	}
	return metrics
}

func (e *Engine) GetStrategyCount() int {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	return len(e.strategies)
}

func (e *Engine) HasStrategy(name string) bool {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	_, exists := e.strategies[name]
	return exists
}

func (e *Engine) GetStrategyNames() []string {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	names := make([]string, 0, len(e.strategies))
	for name := range e.strategies {
		names = append(names, name)
	}
	return names
}

func (e *Engine) NotifyPositionFilled(name, symbol string, side types.OrderSide, entryPrice, size float64) {
	e.mutex.RLock()
	strategy, exists := e.strategies[name]
	e.mutex.RUnlock()

	if !exists {
		return
	}
	strategy.OnPositionFilled(symbol, side, entryPrice, size)
}

func (e *Engine) NotifyPositionClosed(name, symbol string, exitPrice, pnl float64) {
	e.mutex.RLock()
	strategy, exists := e.strategies[name]
	e.mutex.RUnlock()

	if !exists {
		return
	}
	strategy.OnPositionClosed(symbol, exitPrice, pnl)
}

func (e *Engine) NotifyPositionReduced(name, symbol string, exitPrice, pnl, remainingSize float64) {
	e.mutex.RLock()
	strategy, exists := e.strategies[name]
	e.mutex.RUnlock()

	if !exists {
		return
	}
	strategy.OnPositionReduced(symbol, exitPrice, pnl, remainingSize)
}

func (e *Engine) ConfirmRebalanceEntry(name string, request *RebalanceRequest) (*RebalanceDecision, error) {
	e.mutex.RLock()
	strategy, exists := e.strategies[name]
	e.mutex.RUnlock()

	if !exists {
		return nil, nil
	}

	decision, err := strategy.ConfirmRebalanceEntry(request)
	if decision != nil && decision.Signal != nil {
		decision.Signal.Strategy = name
	}

	return decision, err
}

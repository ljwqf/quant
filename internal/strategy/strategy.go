package strategy

import (
	"fmt"
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

type StrategyConfig struct {
	Name     string                 `json:"name"`
	Strategy Strategy               `json:"-"`
	Params   map[string]interface{} `json:"params"`
	Weight   float64                `json:"weight"`
	Enabled  bool                   `json:"enabled"`
}

type Engine struct {
	strategies map[string]*StrategyConfig
	params     map[string]map[string]interface{}
	mutex      sync.RWMutex
}

func NewEngine() *Engine {
	return &Engine{
		strategies: make(map[string]*StrategyConfig),
		params:     make(map[string]map[string]interface{}),
	}
}

func (e *Engine) AddStrategy(name string, strategy Strategy, params map[string]interface{}) error {
	return e.AddStrategyWithWeight(name, strategy, params, 1.0)
}

// AddStrategyWithWeight 添加策略并指定权重
func (e *Engine) AddStrategyWithWeight(name string, strategy Strategy, params map[string]interface{}, weight float64) error {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	if weight < 0 {
		return fmt.Errorf("strategy weight cannot be negative: %.4f", weight)
	}

	config := &StrategyConfig{
		Name:     name,
		Strategy: strategy,
		Params:   params,
		Weight:   weight,
		Enabled:  true,
	}

	if err := strategy.Init(params); err != nil {
		return err
	}

	e.strategies[name] = config
	e.params[name] = params

	logger.Info("策略添加成功",
		zap.String("name", name),
		zap.String("type", strategy.Name()),
		zap.Float64("weight", weight),
	)
	return nil
}

func (e *Engine) RemoveStrategy(name string) {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	delete(e.strategies, name)
	delete(e.params, name)
}

// GetStrategy 获取策略实例
func (e *Engine) GetStrategy(name string) Strategy {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	if config, exists := e.strategies[name]; exists {
		return config.Strategy
	}
	return nil
}

// GetStrategyConfig 获取策略配置
func (e *Engine) GetStrategyConfig(name string) *StrategyConfig {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	return e.strategies[name]
}

// GetStrategies 获取所有策略实例
func (e *Engine) GetStrategies() map[string]Strategy {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	result := make(map[string]Strategy)
	for k, v := range e.strategies {
		result[k] = v.Strategy
	}
	return result
}

// GetAllStrategyConfigs 获取所有策略配置
func (e *Engine) GetAllStrategyConfigs() map[string]*StrategyConfig {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	result := make(map[string]*StrategyConfig)
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

	for name, config := range e.strategies {
		if !config.Enabled {
			continue
		}

		signal, err := config.Strategy.OnTick(tick)
		if err != nil {
			logger.Error("策略 OnTick 失败",
				zap.String("strategy", name),
				zap.Error(err))
			result.Errors = append(result.Errors, err)
			continue
		}
		if signal != nil {
			signal.Strategy = name
			// TODO: 后续添加Weight字段到types.Signal结构
			// signal.Weight = config.Weight
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

	for name, config := range e.strategies {
		if !config.Enabled {
			continue
		}

		signal, err := config.Strategy.OnBar(bar)
		if err != nil {
			logger.Error("策略 OnBar 失败",
				zap.String("strategy", name),
				zap.Error(err))
			result.Errors = append(result.Errors, err)
			continue
		}
		if signal != nil {
			signal.Strategy = name
			// TODO: 后续添加Weight字段到types.Signal结构
			// signal.Weight = config.Weight
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

	for name, config := range e.strategies {
		if !config.Enabled {
			continue
		}

		signal, err := config.Strategy.OnOrderBook(orderBook)
		if err != nil {
			logger.Error("策略 OnOrderBook 失败",
				zap.String("strategy", name),
				zap.Error(err))
			result.Errors = append(result.Errors, err)
			continue
		}
		if signal != nil {
			signal.Strategy = name
			// TODO: 后续添加Weight字段到types.Signal结构
			// signal.Weight = config.Weight
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

	config, exists := e.strategies[name]
	if !exists {
		return fmt.Errorf("strategy not found: %s", name)
	}

	config.Strategy.SetParams(params)
	e.params[name] = params
	return config.Strategy.Init(params)
}

func (e *Engine) GetStrategyMetrics(name string) map[string]interface{} {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	config, exists := e.strategies[name]
	if !exists {
		return nil
	}
	return config.Strategy.GetMetrics()
}

func (e *Engine) GetAllStrategyMetrics() map[string]map[string]interface{} {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	metrics := make(map[string]map[string]interface{})
	for name, config := range e.strategies {
		metrics[name] = config.Strategy.GetMetrics()
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
	config, exists := e.strategies[name]
	e.mutex.RUnlock()

	if !exists || !config.Enabled {
		return
	}
	config.Strategy.OnPositionFilled(symbol, side, entryPrice, size)
}

func (e *Engine) NotifyPositionClosed(name, symbol string, exitPrice, pnl float64) {
	e.mutex.RLock()
	config, exists := e.strategies[name]
	e.mutex.RUnlock()

	if !exists || !config.Enabled {
		return
	}
	config.Strategy.OnPositionClosed(symbol, exitPrice, pnl)
}

func (e *Engine) NotifyPositionReduced(name, symbol string, exitPrice, pnl, remainingSize float64) {
	e.mutex.RLock()
	config, exists := e.strategies[name]
	e.mutex.RUnlock()

	if !exists || !config.Enabled {
		return
	}
	config.Strategy.OnPositionReduced(symbol, exitPrice, pnl, remainingSize)
}

// EnableStrategy 启用策略
func (e *Engine) EnableStrategy(name string) error {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	config, exists := e.strategies[name]
	if !exists {
		return fmt.Errorf("strategy not found: %s", name)
	}

	if !config.Enabled {
		config.Enabled = true
		logger.Info("策略已启用", zap.String("name", name))
	}
	return nil
}

// DisableStrategy 禁用策略
func (e *Engine) DisableStrategy(name string) error {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	config, exists := e.strategies[name]
	if !exists {
		return fmt.Errorf("strategy not found: %s", name)
	}

	if config.Enabled {
		config.Enabled = false
		logger.Info("策略已禁用", zap.String("name", name))
	}
	return nil
}

// SetStrategyWeight 设置策略权重
func (e *Engine) SetStrategyWeight(name string, weight float64) error {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	if weight < 0 {
		return fmt.Errorf("strategy weight cannot be negative: %.4f", weight)
	}

	config, exists := e.strategies[name]
	if !exists {
		return fmt.Errorf("strategy not found: %s", name)
	}

	oldWeight := config.Weight
	config.Weight = weight
	logger.Info("策略权重已更新",
		zap.String("name", name),
		zap.Float64("old_weight", oldWeight),
		zap.Float64("new_weight", weight),
	)
	return nil
}

// GetTotalWeight 获取所有启用策略的总权重
func (e *Engine) GetTotalWeight() float64 {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	total := 0.0
	for _, config := range e.strategies {
		if config.Enabled {
			total += config.Weight
		}
	}
	return total
}

func (e *Engine) ConfirmRebalanceEntry(name string, request *RebalanceRequest) (*RebalanceDecision, error) {
	e.mutex.RLock()
	config, exists := e.strategies[name]
	e.mutex.RUnlock()

	if !exists || !config.Enabled {
		return nil, nil
	}

	decision, err := config.Strategy.ConfirmRebalanceEntry(request)
	if decision != nil && decision.Signal != nil {
		decision.Signal.Strategy = name
	}

	return decision, err
}

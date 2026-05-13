package backtest

import (
	"fmt"
	"time"

	"github.com/ljwqf/quant/internal/strategy"
	"github.com/ljwqf/quant/pkg/types"
)

// StrategyAdapter 策略适配器，将strategy.Strategy适配为backtest.Strategy
type StrategyAdapter struct {
	strategy strategy.Strategy
	params   map[string]interface{}
}

// NewStrategyAdapter 创建策略适配器
func NewStrategyAdapter(strat strategy.Strategy) *StrategyAdapter {
	return &StrategyAdapter{
		strategy: strat,
		params:   make(map[string]interface{}),
	}
}

// Name 返回策略名称
func (a *StrategyAdapter) Name() string {
	return a.strategy.Name()
}

// Init 初始化策略
func (a *StrategyAdapter) Init(params map[string]interface{}) error {
	a.params = params
	return a.strategy.Init(params)
}

// OnBar 处理K线数据
func (a *StrategyAdapter) OnBar(bar *types.Bar) (*types.Signal, error) {
	signal, err := a.strategy.OnBar(bar)
	if err != nil {
		return nil, err
	}
	return signal, nil
}

// GetParameters 获取参数
func (a *StrategyAdapter) GetParameters() map[string]interface{} {
	return a.params
}

// MultiStrategyEngine 多策略组合回测引擎
type MultiStrategyEngine struct {
	engines    map[string]*Engine
	strategies map[string]Strategy
	weights    map[string]float64
	results    map[string]*Result
}

// NewMultiStrategyEngine 创建多策略组合回测引擎
func NewMultiStrategyEngine() *MultiStrategyEngine {
	return &MultiStrategyEngine{
		engines:    make(map[string]*Engine),
		strategies: make(map[string]Strategy),
		weights:    make(map[string]float64),
		results:    make(map[string]*Result),
	}
}

// AddStrategy 添加策略
func (m *MultiStrategyEngine) AddStrategy(name string, strat Strategy, initialBalance float64, weight float64) error {
	if weight < 0 {
		return fmt.Errorf("策略权重不能为负数")
	}

	engine := NewEngine(strat, initialBalance)
	m.engines[name] = engine
	m.strategies[name] = strat
	m.weights[name] = weight
	return nil
}

// AddData 添加历史数据
func (m *MultiStrategyEngine) AddData(symbol string, bars []*types.Bar) error {
	for _, engine := range m.engines {
		if err := engine.AddData(symbol, bars); err != nil {
			return err
		}
	}
	return nil
}

// Run 运行所有策略回测
func (m *MultiStrategyEngine) Run() error {
	for name, engine := range m.engines {
		if err := engine.Run(); err != nil {
			return fmt.Errorf("策略 %s 回测失败: %w", name, err)
		}
		m.results[name] = engine.GetResult()
	}
	return nil
}

// GetResults 获取所有策略的回测结果
func (m *MultiStrategyEngine) GetResults() map[string]*Result {
	return m.results
}

// GetCombinedResult 获取组合策略的回测结果
func (m *MultiStrategyEngine) GetCombinedResult() *Result {
	if len(m.results) == 0 {
		return nil
	}

	totalInitialBalance := 0.0
	totalFinalBalance := 0.0
	totalTrades := 0
	totalWinTrades := 0
	totalLossTrades := 0
	totalPnL := 0.0

	totalWeight := 0.0
	for _, weight := range m.weights {
		totalWeight += weight
	}

	var startTime, endTime *time.Time
	equityCurves := make(map[string][]EquityPoint)

	for name, result := range m.results {
		weight := m.weights[name]
		if totalWeight > 0 {
			weight = weight / totalWeight
		}

		totalInitialBalance += result.InitialBalance * weight
		totalFinalBalance += result.FinalBalance * weight
		totalTrades += result.TotalTrades
		totalWinTrades += result.WinTrades
		totalLossTrades += result.LossTrades
		totalPnL += result.TotalPnL * weight

		if startTime == nil || result.StartTime.Before(*startTime) {
			t := result.StartTime
			startTime = &t
		}
		if endTime == nil || result.EndTime.After(*endTime) {
			t := result.EndTime
			endTime = &t
		}

		equityCurves[name] = result.EquityCurve
	}

	combinedResult := &Result{
		TotalTrades:    totalTrades,
		WinTrades:      totalWinTrades,
		LossTrades:     totalLossTrades,
		TotalPnL:       totalPnL,
		InitialBalance: totalInitialBalance,
		FinalBalance:   totalFinalBalance,
	}

	if startTime != nil {
		combinedResult.StartTime = *startTime
	}
	if endTime != nil {
		combinedResult.EndTime = *endTime
	}

	if totalTrades > 0 {
		combinedResult.WinRate = float64(totalWinTrades) / float64(totalTrades)
	}

	return combinedResult
}

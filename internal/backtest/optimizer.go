package backtest

import (
	"fmt"
	"math"
	"sync"

	"github.com/ljwqf/quant/pkg/types"
)

// ParameterRange 参数范围
type ParameterRange struct {
	Name    string
	Start   float64
	End     float64
	Step    float64
	IsInt   bool
}

// OptimizationResult 优化结果
type OptimizationResult struct {
	Parameters map[string]interface{}
	Result     *Result
	Score      float64
}

// ParameterOptimizer 参数优化器
type ParameterOptimizer struct {
	strategyCreator func(params map[string]interface{}) Strategy
	paramRanges     []ParameterRange
	data            map[string][]*types.Bar
	initialBalance  float64
	results         []OptimizationResult
	mutex           sync.Mutex
}

// NewParameterOptimizer 创建参数优化器
func NewParameterOptimizer(
	strategyCreator func(params map[string]interface{}) Strategy,
	paramRanges []ParameterRange,
	data map[string][]*types.Bar,
	initialBalance float64,
) *ParameterOptimizer {
	return &ParameterOptimizer{
		strategyCreator: strategyCreator,
		paramRanges:     paramRanges,
		data:            data,
		initialBalance:  initialBalance,
		results:         make([]OptimizationResult, 0),
	}
}

// Optimize 执行参数优化
func (po *ParameterOptimizer) Optimize() ([]OptimizationResult, error) {
	if len(po.paramRanges) == 0 {
		return nil, fmt.Errorf("没有参数需要优化")
	}

	paramCombinations := po.generateParamCombinations()

	var wg sync.WaitGroup
	semaphore := make(chan struct{}, 4)

	for _, params := range paramCombinations {
		wg.Add(1)
		semaphore <- struct{}{}

		go func(p map[string]interface{}) {
			defer wg.Done()
			defer func() { <-semaphore }()

			result, err := po.runBacktestWithParams(p)
			if err != nil {
				return
			}

			score := po.calculateScore(result)

			po.mutex.Lock()
			po.results = append(po.results, OptimizationResult{
				Parameters: p,
				Result:     result,
				Score:      score,
			})
			po.mutex.Unlock()
		}(params)
	}

	wg.Wait()

	return po.results, nil
}

// generateParamCombinations 生成所有参数组合
func (po *ParameterOptimizer) generateParamCombinations() []map[string]interface{} {
	if len(po.paramRanges) == 0 {
		return nil
	}

	combinations := []map[string]interface{}{{}}

	for _, paramRange := range po.paramRanges {
		newCombinations := make([]map[string]interface{}, 0)

		for _, combo := range combinations {
			for val := paramRange.Start; val <= paramRange.End; val += paramRange.Step {
				newCombo := make(map[string]interface{})
				for k, v := range combo {
					newCombo[k] = v
				}

				if paramRange.IsInt {
					newCombo[paramRange.Name] = int(math.Round(val))
				} else {
					newCombo[paramRange.Name] = val
				}

				newCombinations = append(newCombinations, newCombo)
			}
		}

		combinations = newCombinations
	}

	return combinations
}

// runBacktestWithParams 使用特定参数运行回测
func (po *ParameterOptimizer) runBacktestWithParams(params map[string]interface{}) (*Result, error) {
	strat := po.strategyCreator(params)
	engine := NewEngine(strat, po.initialBalance)

	for symbol, bars := range po.data {
		if err := engine.AddData(symbol, bars); err != nil {
			return nil, err
		}
	}

	if err := engine.Run(); err != nil {
		return nil, err
	}

	return engine.GetResult(), nil
}

// calculateScore 计算回测得分
func (po *ParameterOptimizer) calculateScore(result *Result) float64 {
	if result.TotalTrades == 0 {
		return 0
	}

	score := 0.0

	score += result.SharpeRatio * 10
	score += result.WinRate * 5
	score -= result.MaxDrawdown * 20

	if result.ProfitFactor > 0 {
		score += result.ProfitFactor * 2
	}

	return score
}

// GetBestResult 获取最优结果
func (po *ParameterOptimizer) GetBestResult() *OptimizationResult {
	if len(po.results) == 0 {
		return nil
	}

	best := &po.results[0]
	for i := 1; i < len(po.results); i++ {
		if po.results[i].Score > best.Score {
			best = &po.results[i]
		}
	}

	return best
}

// GetTopResults 获取前N个最优结果
func (po *ParameterOptimizer) GetTopResults(n int) []OptimizationResult {
	if len(po.results) == 0 || n <= 0 {
		return nil
	}

	results := make([]OptimizationResult, len(po.results))
	copy(results, po.results)

	for i := 0; i < len(results)-1; i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].Score > results[i].Score {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	if n > len(results) {
		n = len(results)
	}

	return results[:n]
}

package indicator

import (
	"github.com/ljwqf/quant/pkg/types"
)

// Indicator 技术指标接口
type Indicator interface {
	// Calculate 计算指标值
	Calculate(data []*types.Bar) (float64, error)
	// GetName 获取指标名称
	GetName() string
	// GetParams 获取指标参数
	GetParams() map[string]interface{}
}

// IndicatorResult 指标计算结果
type IndicatorResult struct {
	Value     float64            `json:"value"`
	Name      string             `json:"name"`
	Params    map[string]interface{} `json:"params"`
	Timestamp int64              `json:"timestamp"`
}

// IndicatorSet 指标集合
type IndicatorSet struct {
	indicators map[string]Indicator
}

// NewIndicatorSet 创建指标集合
func NewIndicatorSet() *IndicatorSet {
	return &IndicatorSet{
		indicators: make(map[string]Indicator),
	}
}

// AddIndicator 添加指标
func (is *IndicatorSet) AddIndicator(name string, indicator Indicator) {
	is.indicators[name] = indicator
}

// CalculateAll 计算所有指标
func (is *IndicatorSet) CalculateAll(data []*types.Bar) (map[string]IndicatorResult, error) {
	results := make(map[string]IndicatorResult)
	
	for name, indicator := range is.indicators {
		value, err := indicator.Calculate(data)
		if err != nil {
			return nil, err
		}
		
		results[name] = IndicatorResult{
			Value:     value,
			Name:      indicator.GetName(),
			Params:    indicator.GetParams(),
			Timestamp: data[len(data)-1].Timestamp.Unix(),
		}
	}
	
	return results, nil
}

// GetIndicator 获取指标
func (is *IndicatorSet) GetIndicator(name string) (Indicator, bool) {
	indicator, exists := is.indicators[name]
	return indicator, exists
}

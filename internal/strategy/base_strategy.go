package strategy

import (
	"github.com/ljwqf/quant/internal/indicator"
	"github.com/ljwqf/quant/pkg/types"
)

// BaseStrategy 基础策略结构，包含技术指标计算
type BaseStrategy struct {
	indicatorSet *indicator.IndicatorSet
	bars         []*types.Bar
	maxBars      int
}

// NewBaseStrategy 创建基础策略
func NewBaseStrategy(maxBars int) *BaseStrategy {
	return &BaseStrategy{
		indicatorSet: indicator.NewIndicatorSet(),
		bars:         make([]*types.Bar, 0, maxBars),
		maxBars:      maxBars,
	}
}

// AddIndicator 添加技术指标
func (bs *BaseStrategy) AddIndicator(name string, ind indicator.Indicator) {
	bs.indicatorSet.AddIndicator(name, ind)
}

// UpdateBars 更新K线数据
func (bs *BaseStrategy) UpdateBars(bar *types.Bar) {
	bs.bars = append(bs.bars, bar)
	// 保持K线数据不超过最大长度
	if len(bs.bars) > bs.maxBars {
		bs.bars = bs.bars[len(bs.bars)-bs.maxBars:]
	}
}

// CalculateIndicators 计算所有技术指标
func (bs *BaseStrategy) CalculateIndicators() (map[string]indicator.IndicatorResult, error) {
	if len(bs.bars) == 0 {
		return nil, nil
	}
	return bs.indicatorSet.CalculateAll(bs.bars)
}

// GetBars 获取K线数据
func (bs *BaseStrategy) GetBars() []*types.Bar {
	return bs.bars
}

// GetIndicatorSet 获取指标集合
func (bs *BaseStrategy) GetIndicatorSet() *indicator.IndicatorSet {
	return bs.indicatorSet
}

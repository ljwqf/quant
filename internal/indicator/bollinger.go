package indicator

import (
	"errors"
	"math"

	"github.com/ljwqf/quant/pkg/types"
)

// Bollinger 布林带指标
type Bollinger struct {
	period int
	deviation float64
}

// NewBollinger 创建布林带指标
func NewBollinger(period int, deviation float64) *Bollinger {
	return &Bollinger{
		period:    period,
		deviation: deviation,
	}
}

// Calculate 计算布林带值（返回布林带宽度）
func (b *Bollinger) Calculate(data []*types.Bar) (float64, error) {
	if len(data) < b.period {
		return 0, errors.New("insufficient data for Bollinger Bands calculation")
	}

	// 计算移动平均线
	ma := b.calculateMA(data, b.period)

	// 计算标准差
	stdDev := b.calculateStdDev(data, b.period, ma)

	// 计算上轨和下轨
	upperBand := ma + b.deviation*stdDev
	lowerBand := ma - b.deviation*stdDev

	// 计算布林带宽度
	bandWidth := (upperBand - lowerBand) / ma

	return bandWidth, nil
}

// calculateMA 计算移动平均线
func (b *Bollinger) calculateMA(data []*types.Bar, period int) float64 {
	sum := 0.0
	for i := len(data) - period; i < len(data); i++ {
		sum += data[i].Close
	}
	return sum / float64(period)
}

// calculateStdDev 计算标准差
func (b *Bollinger) calculateStdDev(data []*types.Bar, period int, mean float64) float64 {
	sumSquaredDiff := 0.0
	for i := len(data) - period; i < len(data); i++ {
		diff := data[i].Close - mean
		sumSquaredDiff += diff * diff
	}
	variance := sumSquaredDiff / float64(period)
	return math.Sqrt(variance)
}

// GetName 获取指标名称
func (b *Bollinger) GetName() string {
	return "Bollinger Bands"
}

// GetParams 获取指标参数
func (b *Bollinger) GetParams() map[string]interface{} {
	return map[string]interface{}{
		"period":    b.period,
		"deviation": b.deviation,
	}
}

package indicator

import (
	"errors"

	"github.com/ljwqf/quant/pkg/types"
)

// ATR 平均真实范围指标
type ATR struct {
	period int
}

// NewATR 创建ATR指标
func NewATR(period int) *ATR {
	return &ATR{
		period: period,
	}
}

// Calculate 计算ATR值
func (a *ATR) Calculate(data []*types.Bar) (float64, error) {
	if len(data) < a.period+1 {
		return 0, errors.New("insufficient data for ATR calculation")
	}

	// 计算真实范围
	var trueRanges []float64

	for i := 1; i < len(data); i++ {
		high := data[i].High
		low := data[i].Low
		prevClose := data[i-1].Close

		// 计算真实范围
		tr1 := high - low
		tr2 := high - prevClose
		if tr2 < 0 {
			tr2 = -tr2
		}
		tr3 := low - prevClose
		if tr3 < 0 {
			tr3 = -tr3
		}

		// 取最大值
		trueRange := tr1
		if tr2 > trueRange {
			trueRange = tr2
		}
		if tr3 > trueRange {
			trueRange = tr3
		}

		trueRanges = append(trueRanges, trueRange)
	}

	// 计算ATR
	atrs := make([]float64, len(trueRanges))
	atrs[0] = a.calculateAverage(trueRanges[:a.period])

	for i := 1; i < len(atrs); i++ {
		atrs[i] = (atrs[i-1]*float64(a.period-1) + trueRanges[i]) / float64(a.period)
	}

	return atrs[len(atrs)-1], nil
}

// calculateAverage 计算平均值
func (a *ATR) calculateAverage(values []float64) float64 {
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

// GetName 获取指标名称
func (a *ATR) GetName() string {
	return "ATR"
}

// GetParams 获取指标参数
func (a *ATR) GetParams() map[string]interface{} {
	return map[string]interface{}{
		"period": a.period,
	}
}

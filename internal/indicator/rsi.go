package indicator

import (
	"errors"

	"github.com/ljwqf/quant/pkg/types"
)

// RSI 相对强弱指数
type RSI struct {
	period int
}

// NewRSI 创建RSI指标
func NewRSI(period int) *RSI {
	return &RSI{
		period: period,
	}
}

// Calculate 计算RSI值
func (r *RSI) Calculate(data []*types.Bar) (float64, error) {
	if len(data) < r.period+1 {
		return 0, errors.New("insufficient data for RSI calculation")
	}

	var gains, losses []float64

	// 计算价格变化
	for i := 1; i < len(data); i++ {
		change := data[i].Close - data[i-1].Close
		if change > 0 {
			gains = append(gains, change)
			losses = append(losses, 0)
		} else {
			gains = append(gains, 0)
			losses = append(losses, -change)
		}
	}

	// 计算平均增益和平均损失
	avgGain := r.calculateAverage(gains[:r.period])
	avgLoss := r.calculateAverage(losses[:r.period])

	// 计算RSI
	for i := r.period; i < len(gains); i++ {
		avgGain = (avgGain*float64(r.period-1) + gains[i]) / float64(r.period)
		avgLoss = (avgLoss*float64(r.period-1) + losses[i]) / float64(r.period)
	}

	if avgLoss == 0 {
		return 100, nil
	}

	rs := avgGain / avgLoss
	rsi := 100 - (100 / (1 + rs))

	return rsi, nil
}

// calculateAverage 计算平均值
func (r *RSI) calculateAverage(values []float64) float64 {
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

// GetName 获取指标名称
func (r *RSI) GetName() string {
	return "RSI"
}

// GetParams 获取指标参数
func (r *RSI) GetParams() map[string]interface{} {
	return map[string]interface{}{
		"period": r.period,
	}
}

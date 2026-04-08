package indicator

import (
	"errors"

	"github.com/ljwqf/quant/pkg/types"
)

// MACD 移动平均线收敛发散指标
type MACD struct {
	fastPeriod  int
	slowPeriod  int
	signalPeriod int
}

// NewMACD 创建MACD指标
func NewMACD(fastPeriod, slowPeriod, signalPeriod int) *MACD {
	return &MACD{
		fastPeriod:  fastPeriod,
		slowPeriod:  slowPeriod,
		signalPeriod: signalPeriod,
	}
}

// Calculate 计算MACD值
func (m *MACD) Calculate(data []*types.Bar) (float64, error) {
	if len(data) < m.slowPeriod {
		return 0, errors.New("insufficient data for MACD calculation")
	}

	// 计算EMA
	fastEMA := m.calculateEMA(data, m.fastPeriod)
	slowEMA := m.calculateEMA(data, m.slowPeriod)

	// 计算MACD线
	macdLine := fastEMA - slowEMA

	// 计算信号线
	signalLine := m.calculateSignalLine(data, macdLine)

	// 计算柱状图
	histogram := macdLine - signalLine

	return histogram, nil
}

// calculateEMA 计算指数移动平均线
func (m *MACD) calculateEMA(data []*types.Bar, period int) float64 {
	if len(data) < period {
		return 0
	}

	multiplier := 2.0 / float64(period+1)
	ema := data[0].Close

	for i := 1; i < len(data); i++ {
		ema = data[i].Close*multiplier + ema*(1-multiplier)
	}

	return ema
}

// calculateSignalLine 计算信号线
func (m *MACD) calculateSignalLine(data []*types.Bar, macdLine float64) float64 {
	if len(data) < m.slowPeriod+m.signalPeriod {
		return 0
	}

	// 这里简化处理，实际应该计算MACD线的EMA
	return m.calculateEMA(data[m.slowPeriod:], m.signalPeriod)
}

// GetName 获取指标名称
func (m *MACD) GetName() string {
	return "MACD"
}

// GetParams 获取指标参数
func (m *MACD) GetParams() map[string]interface{} {
	return map[string]interface{}{
		"fast_period":   m.fastPeriod,
		"slow_period":   m.slowPeriod,
		"signal_period": m.signalPeriod,
	}
}

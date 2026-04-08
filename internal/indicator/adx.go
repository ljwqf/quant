package indicator

import (
	"errors"
	"math"

	"github.com/ljwqf/quant/pkg/types"
)

// ADX 平均方向移动指数
type ADX struct {
	period int
}

// NewADX 创建ADX指标
func NewADX(period int) *ADX {
	return &ADX{
		period: period,
	}
}

// Calculate 计算ADX值
func (a *ADX) Calculate(data []*types.Bar) (float64, error) {
	if len(data) < a.period+1 {
		return 0, errors.New("insufficient data for ADX calculation")
	}

	// 计算上升方向和下降方向
	var plusDM, minusDM []float64
	var trs []float64

	for i := 1; i < len(data); i++ {
		current := data[i]
		previous := data[i-1]

		// 计算TR
		tr1 := current.High - current.Low
		tr2 := current.High - previous.Close
		if tr2 < 0 {
			tr2 = -tr2
		}
		tr3 := current.Low - previous.Close
		if tr3 < 0 {
			tr3 = -tr3
		}
		tr := math.Max(tr1, math.Max(tr2, tr3))
		trs = append(trs, tr)

		// 计算+DM和-DM
		upMove := current.High - previous.High
		downMove := previous.Low - current.Low

		if upMove > 0 && upMove > downMove {
			plusDM = append(plusDM, upMove)
			minusDM = append(minusDM, 0)
		} else if downMove > 0 && downMove > upMove {
			plusDM = append(plusDM, 0)
			minusDM = append(minusDM, downMove)
		} else {
			plusDM = append(plusDM, 0)
			minusDM = append(minusDM, 0)
		}
	}

	// 计算ATR
	atr := a.calculateATR(trs, a.period)

	// 计算+DI和-DI
	plusDI := a.calculateDI(plusDM, atr, a.period)
	minusDI := a.calculateDI(minusDM, atr, a.period)

	// 计算DX
	var dxs []float64
	for i := 0; i < len(plusDI); i++ {
		diff := math.Abs(plusDI[i] - minusDI[i])
		sum := plusDI[i] + minusDI[i]
		if sum != 0 {
			dx := (diff / sum) * 100
			dxs = append(dxs, dx)
		} else {
			dxs = append(dxs, 0)
		}
	}

	// 计算ADX
	adx := a.calculateADX(dxs, a.period)

	return adx, nil
}

// calculateATR 计算ATR
func (a *ADX) calculateATR(trs []float64, period int) []float64 {
	atrs := make([]float64, len(trs))
	atrs[0] = a.calculateSum(trs[:period]) / float64(period)

	for i := 1; i < len(atrs); i++ {
		atrs[i] = (atrs[i-1]*float64(period-1) + trs[i]) / float64(period)
	}

	return atrs
}

// calculateDI 计算DI
func (a *ADX) calculateDI(dm []float64, atr []float64, period int) []float64 {
	dis := make([]float64, len(atr))
	sumDM := a.calculateSum(dm[:period])
	dis[period-1] = (sumDM / atr[period-1]) * 100

	for i := period; i < len(dis); i++ {
		sumDM = sumDM - dm[i-period] + dm[i]
		dis[i] = (sumDM / atr[i]) * 100
	}

	return dis
}

// calculateADX 计算ADX
func (a *ADX) calculateADX(dxs []float64, period int) float64 {
	if len(dxs) < period {
		return 0
	}

	sumDX := a.calculateSum(dxs[:period])
	adx := sumDX / float64(period)

	for i := period; i < len(dxs); i++ {
		adx = (adx*float64(period-1) + dxs[i]) / float64(period)
	}

	return adx
}

// calculateSum 计算总和
func (a *ADX) calculateSum(values []float64) float64 {
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum
}

// GetName 获取指标名称
func (a *ADX) GetName() string {
	return "ADX"
}

// GetParams 获取指标参数
func (a *ADX) GetParams() map[string]interface{} {
	return map[string]interface{}{
		"period": a.period,
	}
}

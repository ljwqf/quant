package indicator

import (
	"testing"
	"time"

	"github.com/ljwqf/quant/pkg/types"
	"github.com/stretchr/testify/assert"
)

// 生成测试用K线数据
func generateTestBars(count int) []*types.Bar {
	bars := make([]*types.Bar, count)
	basePrice := 100.0

	for i := 0; i < count; i++ {
		// 生成模拟价格波动
		price := basePrice + float64(i%10)*5 - 25
		if price < 10 {
			price = 10
		}

		bars[i] = &types.Bar{
			Symbol:    "BTC-USDT",
			Timestamp: time.Now().Add(-time.Duration(count-i) * time.Minute),
			Open:      price - 2,
			High:      price + 3,
			Low:       price - 3,
			Close:     price,
			Volume:    1000,
		}
	}

	return bars
}

func TestMACD(t *testing.T) {
	bars := generateTestBars(100)

	macd := NewMACD(12, 26, 9)
	assert.NotNil(t, macd)
	assert.Equal(t, "MACD", macd.GetName())

	params := macd.GetParams()
	assert.Equal(t, 12, params["fast_period"])
	assert.Equal(t, 26, params["slow_period"])
	assert.Equal(t, 9, params["signal_period"])

	value, err := macd.Calculate(bars)
	assert.NoError(t, err)
	assert.True(t, value != 0)
}

func TestRSI(t *testing.T) {
	bars := generateTestBars(50)

	rsi := NewRSI(14)
	assert.NotNil(t, rsi)
	assert.Equal(t, "RSI", rsi.GetName())

	params := rsi.GetParams()
	assert.Equal(t, 14, params["period"])

	value, err := rsi.Calculate(bars)
	assert.NoError(t, err)
	assert.True(t, value >= 0 && value <= 100)

	// 测试边界情况 - 数据不足
	shortBars := generateTestBars(10)
	value, err = rsi.Calculate(shortBars)
	// RSI在数据不足时应该返回错误
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "insufficient data")
}

func TestBollinger(t *testing.T) {
	bars := generateTestBars(50)

	bb := NewBollinger(20, 2.0)
	assert.NotNil(t, bb)
	assert.Equal(t, "Bollinger Bands", bb.GetName())

	params := bb.GetParams()
	assert.Equal(t, 20, params["period"])
	assert.Equal(t, 2.0, params["deviation"])

	value, err := bb.Calculate(bars)
	assert.NoError(t, err)
	assert.True(t, value != 0)
}

func TestATR(t *testing.T) {
	bars := generateTestBars(50)

	atr := NewATR(14)
	assert.NotNil(t, atr)
	assert.Equal(t, "ATR", atr.GetName())

	params := atr.GetParams()
	assert.Equal(t, 14, params["period"])

	value, err := atr.Calculate(bars)
	assert.NoError(t, err)
	assert.True(t, value > 0)
}

func TestADX(t *testing.T) {
	bars := generateTestBars(100)

	adx := NewADX(14)
	assert.NotNil(t, adx)
	assert.Equal(t, "ADX", adx.GetName())

	params := adx.GetParams()
	assert.Equal(t, 14, params["period"])

	value, err := adx.Calculate(bars)
	assert.NoError(t, err)
	assert.True(t, value >= 0 && value <= 100)
}

func TestIndicatorSet(t *testing.T) {
	is := NewIndicatorSet()
	assert.NotNil(t, is)

	// 添加多个指标
	is.AddIndicator("macd", NewMACD(12, 26, 9))
	is.AddIndicator("rsi", NewRSI(14))
	is.AddIndicator("bb", NewBollinger(20, 2.0))

	assert.Equal(t, 3, len(is.indicators))

	// 测试获取指标
	ind, exists := is.GetIndicator("rsi")
	assert.True(t, exists)
	assert.Equal(t, "RSI", ind.GetName())

	// 测试批量计算
	bars := generateTestBars(100)
	results, err := is.CalculateAll(bars)
	assert.NoError(t, err)
	assert.Equal(t, 3, len(results))

	assert.Contains(t, results, "macd")
	assert.Contains(t, results, "rsi")
	assert.Contains(t, results, "bb")

	assert.Equal(t, "MACD", results["macd"].Name)
	assert.Equal(t, "RSI", results["rsi"].Name)
	assert.Equal(t, "Bollinger Bands", results["bb"].Name)

	assert.True(t, results["macd"].Value != 0)
	assert.True(t, results["rsi"].Value >= 0 && results["rsi"].Value <= 100)
	assert.True(t, results["bb"].Value != 0)
}

func TestIndicatorCalculationAccuracy(t *testing.T) {
	// 测试已知数据的指标计算准确性
	bars := []*types.Bar{
		{Close: 44.34},
		{Close: 44.09},
		{Close: 44.17},
		{Close: 43.61},
		{Close: 44.33},
		{Close: 44.83},
		{Close: 45.10},
		{Close: 45.42},
		{Close: 45.84},
		{Close: 46.08},
		{Close: 45.89},
		{Close: 46.03},
		{Close: 45.61},
		{Close: 46.28},
		{Close: 46.28},
	}

	rsi := NewRSI(14)
	value, err := rsi.Calculate(bars)
	assert.NoError(t, err)
	// 预期RSI值大约在70左右（上涨趋势）
	assert.True(t, value > 60 && value < 80)
}

func TestEdgeCases(t *testing.T) {
	// 测试空数据
	rsi := NewRSI(14)
	value, err := rsi.Calculate(nil)
	assert.Error(t, err)
	assert.Equal(t, 0.0, value)

	// 测试单条数据（不足）
	bars := generateTestBars(1)
	value, err = rsi.Calculate(bars)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "insufficient data")
	assert.Equal(t, 0.0, value)

	// 测试指标集空数据
	is := NewIndicatorSet()
	is.AddIndicator("rsi", NewRSI(14))
	results, err := is.CalculateAll(nil)
	assert.Error(t, err)
	assert.Nil(t, results)
}

package tests

import (
	"testing"
	"time"

	"github.com/ljwqf/quant/internal/indicator"
	"github.com/ljwqf/quant/internal/strategy"
	"github.com/ljwqf/quant/pkg/types"
)

func BenchmarkIndicatorMACDCalculate(b *testing.B) {
	bars := generateTestBars(1000)
	macd := indicator.NewMACD(12, 26, 9)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = macd.Calculate(bars)
	}
}

func BenchmarkIndicatorRSICalculate(b *testing.B) {
	bars := generateTestBars(1000)
	rsi := indicator.NewRSI(14)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = rsi.Calculate(bars)
	}
}

func BenchmarkIndicatorBollingerCalculate(b *testing.B) {
	bars := generateTestBars(1000)
	bb := indicator.NewBollinger(20, 2)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = bb.Calculate(bars)
	}
}

func BenchmarkIndicatorATRCalculate(b *testing.B) {
	bars := generateTestBars(1000)
	atr := indicator.NewATR(14)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = atr.Calculate(bars)
	}
}

func BenchmarkIndicatorADXCalculate(b *testing.B) {
	bars := generateTestBars(1000)
	adx := indicator.NewADX(14)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = adx.Calculate(bars)
	}
}

func BenchmarkTrendFollowingOnBar(b *testing.B) {
	bars := generateTestBars(1000)
	tf := strategy.NewTrendFollowingStrategy()
	_ = tf.Init(nil)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = tf.OnBar(bars[len(bars)-1])
	}
}

func BenchmarkMeanReversionOnBar(b *testing.B) {
	bars := generateTestBars(1000)
	mr := strategy.NewMeanReversionStrategy()
	_ = mr.Init(nil)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = mr.OnBar(bars[len(bars)-1])
	}
}

func BenchmarkVolatilityBreakoutOnBar(b *testing.B) {
	bars := generateTestBars(1000)
	vb := strategy.NewVolatilityBreakoutStrategy()
	_ = vb.Init(nil)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = vb.OnBar(bars[len(bars)-1])
	}
}

func BenchmarkParallelIndicatorCalculations(b *testing.B) {
	bars := generateTestBars(1000)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		macd := indicator.NewMACD(12, 26, 9)
		rsi := indicator.NewRSI(14)
		for pb.Next() {
			_, _ = macd.Calculate(bars)
			_, _ = rsi.Calculate(bars)
		}
	})
}

func generateTestBars(count int) []*types.Bar {
	bars := make([]*types.Bar, count)
	basePrice := 50000.0
	ts := time.Now().Add(-time.Duration(count) * time.Minute)
	
	for i := 0; i < count; i++ {
		price := basePrice + float64(i%1000-500)
		bars[i] = &types.Bar{
			Symbol:    "BTC-USDT-SWAP",
			Open:      price - 50,
			High:      price + 100,
			Low:       price - 100,
			Close:     price,
			Volume:    1000,
			Timestamp: ts.Add(time.Duration(i) * time.Minute),
			Interval:  "1m",
		}
	}
	
	return bars
}

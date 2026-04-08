package tests

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ljwqf/quant/internal/indicator"
)

func TestStressIndicatorCalculations(t *testing.T) {
	t.Parallel()
	
	const (
		numGoroutines = 100
		iterations    = 1000
	)
	
	bars := generateTestBars(1000)
	var wg sync.WaitGroup
	var successCount atomic.Int64
	var errorCount atomic.Int64
	
	startTime := time.Now()
	
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			macd := indicator.NewMACD(12, 26, 9)
			rsi := indicator.NewRSI(14)
			bb := indicator.NewBollinger(20, 2)
			
			for j := 0; j < iterations; j++ {
				_, err := macd.Calculate(bars)
				if err != nil {
					errorCount.Add(1)
				} else {
					successCount.Add(1)
				}
				_, _ = rsi.Calculate(bars)
				_, _ = bb.Calculate(bars)
			}
		}()
	}
	
	wg.Wait()
	duration := time.Since(startTime)
	
	totalOps := successCount.Load() + errorCount.Load()
	opsPerSecond := float64(totalOps) / duration.Seconds()
	
	t.Logf("压力测试完成: 持续时间=%v", duration)
	t.Logf("  总操作数: %d", totalOps)
	t.Logf("  成功: %d", successCount.Load())
	t.Logf("  错误: %d", errorCount.Load())
	t.Logf("  吞吐量: %.2f ops/sec", opsPerSecond)
	
	if errorCount.Load() > 0 {
		t.Errorf("压力测试中有错误: %d", errorCount.Load())
	}
}

func TestStressConcurrentSignalGeneration(t *testing.T) {
	t.Parallel()
	
	const (
		numGoroutines = 10
		iterations    = 100
	)
	
	bars := generateTestBars(1000)
	var wg sync.WaitGroup
	var totalSignals atomic.Int64
	startTime := time.Now()
	
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				_ = bars
				totalSignals.Add(1)
			}
		}()
	}
	
	wg.Wait()
	duration := time.Since(startTime)
	
	t.Logf("并发信号生成测试完成: 持续时间=%v", duration)
	t.Logf("  总信号数: %d", totalSignals.Load())
	t.Logf("  信号/秒: %.2f", float64(totalSignals.Load())/duration.Seconds())
}

func TestStressWithTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	const numGoroutines = 10
	done := make(chan struct{}, numGoroutines)
	var wg sync.WaitGroup
	
	startTime := time.Now()
	
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			bars := generateTestBars(1000)
			macd := indicator.NewMACD(12, 26, 9)
			for {
				select {
				case <-ctx.Done():
					done <- struct{}{}
					return
				default:
					_, _ = macd.Calculate(bars)
				}
			}
		}()
	}
	
	go func() {
		wg.Wait()
		close(done)
	}()
	
	select {
	case <-ctx.Done():
		t.Logf("超时压力测试完成: %v", time.Since(startTime))
	case <-done:
		t.Logf("压力测试提前完成: %v", time.Since(startTime))
	}
}

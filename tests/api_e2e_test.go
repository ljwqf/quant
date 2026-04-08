package tests

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPIMetricsEndpoint(t *testing.T) {
	t.Parallel()
	
	ctx := NewTestContext(t)
	defer ctx.Cleanup()
	
	t.Run("GetMetrics", func(t *testing.T) {
		resp := ctx.MakeHTTPRequest("GET", "/api/metrics", nil)
		defer resp.Body.Close()
		
		AssertResponseStatus(t, resp, 200)
		
		result := AssertJSONResponse(t, resp)
		assert.NotNil(t, result)
		assert.Contains(t, result, "system")
		assert.Contains(t, result, "api")
	})
	
	t.Run("GetPrometheusMetrics", func(t *testing.T) {
		resp := ctx.MakeHTTPRequest("GET", "/api/metrics/prometheus", nil)
		defer resp.Body.Close()
		
		AssertResponseStatus(t, resp, 200)
		assert.Contains(t, resp.Header.Get("Content-Type"), "text/plain")
	})
}

func TestAPIHealthEndpoint(t *testing.T) {
	t.Parallel()
	
	ctx := NewTestContext(t)
	defer ctx.Cleanup()
	
	resp := ctx.MakeHTTPRequest("GET", "/health", nil)
	defer resp.Body.Close()
	
	AssertResponseStatus(t, resp, 200)
	
	result := AssertJSONResponse(t, resp)
	assert.NotNil(t, result)
	assert.Contains(t, result, "status")
}

func TestAPIBacktestEndpoints(t *testing.T) {
	t.Parallel()
	
	ctx := NewTestContext(t)
	defer ctx.Cleanup()
	
	t.Run("GetBacktestStrategies", func(t *testing.T) {
		resp := ctx.MakeHTTPRequest("GET", "/api/backtest/strategies", nil)
		defer resp.Body.Close()
		
		AssertResponseStatus(t, resp, 200)
		
		result := AssertJSONResponse(t, resp)
		assert.NotNil(t, result)
	})
}

func TestSwaggerUIEndpoint(t *testing.T) {
	t.Parallel()
	
	ctx := NewTestContext(t)
	defer ctx.Cleanup()
	
	resp := ctx.MakeHTTPRequest("GET", "/swagger/index.html", nil)
	defer resp.Body.Close()
	
	assert.Equal(t, 200, resp.StatusCode)
}

func TestMetricsRecording(t *testing.T) {
	ctx := NewTestContext(t)
	defer ctx.Cleanup()
	
	// 记录一些指标
	ctx.Metrics.RecordBalance(10000.0)
	ctx.Metrics.RecordPosition("BTC-USDT", 1.0, 50000.0)
	ctx.Metrics.RecordTrade("BTC-USDT", "buy", 50000.0, 1.0, 100.0)
	
	// 验证指标
	balance := ctx.Metrics.GetBalance()
	assert.Equal(t, 10000.0, balance)
	
	positions := ctx.Metrics.GetPositions()
	assert.Contains(t, positions, "BTC-USDT")
	
	tradeStats := ctx.Metrics.GetTradeStats()
	assert.Equal(t, 1, tradeStats["trade_count"])
}

func TestSystemMetrics(t *testing.T) {
	ctx := NewTestContext(t)
	defer ctx.Cleanup()
	
	systemMetrics := ctx.Metrics.GetSystemMetrics()
	require.NotNil(t, systemMetrics)
	
	// 更新系统指标
	systemMetrics.Update()
	
	// 获取指标
	metrics := systemMetrics.GetMetrics()
	assert.Contains(t, metrics, "cpu_percent")
	assert.Contains(t, metrics, "memory_percent")
	assert.GreaterOrEqual(t, metrics["cpu_percent"].(float64), 0.0)
}

func TestAPIMetrics(t *testing.T) {
	ctx := NewTestContext(t)
	defer ctx.Cleanup()
	
	apiMetrics := ctx.Metrics.GetAPIMetrics()
	require.NotNil(t, apiMetrics)
	
	// 记录API请求
	apiMetrics.RecordRequest("/api/test", "GET")
	apiMetrics.RecordResponse("/api/test", "GET", 100*time.Millisecond, false)
	
	// 获取统计
	stats := apiMetrics.GetAllStats()
	assert.Equal(t, uint64(1), stats["total_requests"])
	assert.Equal(t, uint64(0), stats["total_errors"])
}

func TestStrategyMetrics(t *testing.T) {
	ctx := NewTestContext(t)
	defer ctx.Cleanup()
	
	strategyMetrics := ctx.Metrics.GetStrategyMetrics()
	require.NotNil(t, strategyMetrics)
	
	// 记录策略指标
	strategyMetrics.RecordSignal("TrendFollowing")
	strategyMetrics.RecordSignal("TrendFollowing")
	strategyMetrics.RecordTrade("TrendFollowing", 100.0)
	
	// 获取统计
	stats := strategyMetrics.GetStrategyStats("TrendFollowing")
	assert.Equal(t, uint64(2), stats["signal_count"])
	assert.Equal(t, uint64(1), stats["trade_count"])
	assert.Equal(t, 100.0, stats["total_pnl"])
}

func TestTradingMetrics(t *testing.T) {
	ctx := NewTestContext(t)
	defer ctx.Cleanup()
	
	tradingMetrics := ctx.Metrics.GetTradingMetrics()
	require.NotNil(t, tradingMetrics)
	
	// 记录交易指标
	tradingMetrics.RecordOrder("filled", 1000.0)
	tradingMetrics.RecordOrder("filled", 2000.0)
	tradingMetrics.RecordOrder("cancelled", 0.0)
	
	// 获取统计
	stats := tradingMetrics.GetStats()
	assert.Equal(t, uint64(3), stats["total_orders"])
	assert.Equal(t, uint64(2), stats["filled_orders"])
	assert.Equal(t, 3000.0, stats["total_volume"])
}

func TestMetricsIntegration(t *testing.T) {
	ctx := NewTestContext(t)
	defer ctx.Cleanup()
	
	// 集成测试：同时使用多个指标
	ctx.Metrics.RecordBalance(10000.0)
	
	ctx.Metrics.GetAPIMetrics().RecordRequest("/api/test", "GET")
	ctx.Metrics.GetAPIMetrics().RecordResponse("/api/test", "GET", 50*time.Millisecond, false)
	
	ctx.Metrics.GetStrategyMetrics().RecordSignal("TestStrategy")
	ctx.Metrics.GetStrategyMetrics().RecordTrade("TestStrategy", 50.0)
	
	ctx.Metrics.GetTradingMetrics().RecordOrder("filled", 500.0)
	
	// 获取所有指标
	allMetrics := ctx.Metrics.GetAllMetrics()
	
	// 验证
	assert.Contains(t, allMetrics, "balance")
	assert.Contains(t, allMetrics, "trade_stats")
	assert.Contains(t, allMetrics, "system")
	assert.Contains(t, allMetrics, "api")
	assert.Contains(t, allMetrics, "strategy")
	assert.Contains(t, allMetrics, "trading")
}

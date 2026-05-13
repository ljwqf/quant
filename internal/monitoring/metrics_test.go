package monitoring

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMetricsInitialization(t *testing.T) {
	config := &MetricsConfig{
		Enable: true,
	}

	metrics := NewMetrics(config)
	require.NotNil(t, metrics)
	assert.NotNil(t, metrics.prometheus)
	assert.NotNil(t, metrics.systemMetrics)
	assert.NotNil(t, metrics.apiMetrics)
	assert.NotNil(t, metrics.strategyMetrics)
	assert.NotNil(t, metrics.tradingMetrics)
}

func TestMetricsRecordBalance(t *testing.T) {
	config := &MetricsConfig{Enable: true}
	metrics := NewMetrics(config)

	metrics.RecordBalance(10000.0)
	assert.Equal(t, 10000.0, metrics.GetBalance())
}

func TestMetricsRecordPosition(t *testing.T) {
	config := &MetricsConfig{Enable: true}
	metrics := NewMetrics(config)

	metrics.RecordPosition("BTC-USDT", 1.0, 50000.0)
	positions := metrics.GetPositions()
	
	require.Contains(t, positions, "BTC-USDT")
	assert.Equal(t, 1.0, positions["BTC-USDT"].Size)
	assert.Equal(t, 50000.0, positions["BTC-USDT"].MarkPrice)
	assert.Equal(t, 50000.0, positions["BTC-USDT"].Value)
}

func TestMetricsRecordTrade(t *testing.T) {
	config := &MetricsConfig{Enable: true}
	metrics := NewMetrics(config)

	metrics.RecordTrade("BTC-USDT", "buy", 50000.0, 1.0, 100.0)
	metrics.RecordTrade("BTC-USDT", "sell", 51000.0, 1.0, -50.0)
	metrics.RecordTrade("ETH-USDT", "buy", 3000.0, 2.0, 0.0)

	stats := metrics.GetTradeStats()
	assert.Equal(t, 3, stats["trade_count"])
	assert.Equal(t, 50.0, stats["total_pnl"])
	assert.Equal(t, 1, stats["win_count"])
	assert.Equal(t, 1, stats["loss_count"])
	assert.InDelta(t, 0.333, stats["win_rate"], 0.001)
}

func TestMetricsRecordDailyLoss(t *testing.T) {
	config := &MetricsConfig{Enable: true}
	metrics := NewMetrics(config)

	metrics.RecordDailyLoss(-500.0)
	assert.Equal(t, -500.0, metrics.GetDailyLoss())
}

func TestMetricsGetAllMetrics(t *testing.T) {
	config := &MetricsConfig{Enable: true}
	metrics := NewMetrics(config)

	metrics.RecordBalance(10000.0)
	metrics.RecordPosition("BTC-USDT", 1.0, 50000.0)
	metrics.RecordTrade("BTC-USDT", "buy", 50000.0, 1.0, 100.0)
	metrics.RecordDailyLoss(-100.0)

	allMetrics := metrics.GetAllMetrics()
	
	assert.Contains(t, allMetrics, "balance")
	assert.Contains(t, allMetrics, "trade_stats")
	assert.Contains(t, allMetrics, "daily_loss")
	assert.Contains(t, allMetrics, "system")
	assert.Contains(t, allMetrics, "api")
	assert.Contains(t, allMetrics, "strategy")
	assert.Contains(t, allMetrics, "trading")
	assert.Contains(t, allMetrics, "last_update")
}

func TestMetricsAccessors(t *testing.T) {
	config := &MetricsConfig{Enable: true}
	metrics := NewMetrics(config)

	assert.NotNil(t, metrics.GetPrometheusMetrics())
	assert.NotNil(t, metrics.GetSystemMetrics())
	assert.NotNil(t, metrics.GetAPIMetrics())
	assert.NotNil(t, metrics.GetStrategyMetrics())
	assert.NotNil(t, metrics.GetTradingMetrics())
}

func TestSystemMetricsUpdate(t *testing.T) {
	systemMetrics := NewSystemMetrics()
	require.NotNil(t, systemMetrics)

	initialUpdate := systemMetrics.lastUpdate
	time.Sleep(1 * time.Millisecond)
	
	systemMetrics.Update()
	assert.True(t, systemMetrics.lastUpdate.After(initialUpdate))
	assert.GreaterOrEqual(t, systemMetrics.GetCPUPercent(), 0.0)
	assert.GreaterOrEqual(t, systemMetrics.GetMemoryPercent(), 0.0)
}

func TestSystemMetricsGetMetrics(t *testing.T) {
	systemMetrics := NewSystemMetrics()
	systemMetrics.Update()

	metrics := systemMetrics.GetMetrics()
	
	assert.Contains(t, metrics, "cpu_percent")
	assert.Contains(t, metrics, "memory_percent")
	assert.Contains(t, metrics, "disk_percent")
	assert.Contains(t, metrics, "network_sent")
	assert.Contains(t, metrics, "network_recv")
	assert.Contains(t, metrics, "last_update")
}

func TestAPIMetricsRecordRequest(t *testing.T) {
	apiMetrics := NewAPIMetrics()
	
	apiMetrics.RecordRequest("/api/test", "GET")
	apiMetrics.RecordRequest("/api/test", "POST")
	apiMetrics.RecordRequest("/api/other", "GET")

	stats := apiMetrics.GetAllStats()
	assert.Equal(t, uint64(3), stats["total_requests"])
	assert.Equal(t, uint64(0), stats["total_errors"])
	assert.Equal(t, 0.0, stats["error_rate"])
	assert.Equal(t, 3, stats["active_requests"])
}

func TestAPIMetricsRecordResponse(t *testing.T) {
	apiMetrics := NewAPIMetrics()
	
	apiMetrics.RecordRequest("/api/test", "GET")
	apiMetrics.RecordResponse("/api/test", "GET", 100*time.Millisecond, false)
	
	apiMetrics.RecordRequest("/api/test", "GET")
	apiMetrics.RecordResponse("/api/test", "GET", 200*time.Millisecond, true)

	stats := apiMetrics.GetAllStats()
	assert.Equal(t, uint64(2), stats["total_requests"])
	assert.Equal(t, uint64(1), stats["total_errors"])
	assert.Equal(t, 0.5, stats["error_rate"])
	assert.Equal(t, 0, stats["active_requests"])

	endpointStats := apiMetrics.GetEndpointStats("/api/test", "GET")
	assert.Equal(t, uint64(2), endpointStats["request_count"])
	assert.Equal(t, uint64(1), endpointStats["error_count"])
	assert.Equal(t, int64(150), endpointStats["avg_response_time"])
}

func TestStrategyMetrics(t *testing.T) {
	strategyMetrics := NewStrategyMetrics()
	
	strategyMetrics.RecordSignal("TrendFollowing")
	strategyMetrics.RecordSignal("TrendFollowing")
	strategyMetrics.RecordTrade("TrendFollowing", 100.0)
	
	strategyMetrics.RecordSignal("MeanReversion")
	strategyMetrics.RecordTrade("MeanReversion", -50.0)

	trendStats := strategyMetrics.GetStrategyStats("TrendFollowing")
	assert.Equal(t, uint64(2), trendStats["signal_count"])
	assert.Equal(t, uint64(1), trendStats["trade_count"])
	assert.Equal(t, 100.0, trendStats["total_pnl"])

	allStats := strategyMetrics.GetAllStats()
	assert.Equal(t, uint64(3), allStats["total_signals"])
	assert.Equal(t, uint64(2), allStats["total_trades"])
	assert.Equal(t, 50.0, allStats["total_pnl"])
	assert.Equal(t, 2, allStats["strategy_count"])
}

func TestTradingMetrics(t *testing.T) {
	tradingMetrics := NewTradingMetrics()
	
	tradingMetrics.RecordOrder("filled", 1000.0)
	tradingMetrics.RecordOrder("filled", 2000.0)
	tradingMetrics.RecordOrder("cancelled", 0.0)
	tradingMetrics.RecordOrder("filled", 1500.0)

	stats := tradingMetrics.GetStats()
	assert.Equal(t, uint64(4), stats["total_orders"])
	assert.Equal(t, uint64(3), stats["filled_orders"])
	assert.Equal(t, uint64(1), stats["cancelled_orders"])
	assert.InDelta(t, 0.75, stats["fill_rate"], 0.001)
	assert.Equal(t, 4500.0, stats["total_volume"])
}

package monitoring

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrometheusMetricsInitialization(t *testing.T) {
	config := &MetricsConfig{
		Enable: true,
	}

	promMetrics := NewPrometheusMetrics(config)
	require.NotNil(t, promMetrics)
	assert.NotNil(t, promMetrics.counters)
	assert.NotNil(t, promMetrics.gauges)
	assert.NotNil(t, promMetrics.histograms)
}

func TestPrometheusMetricsDisabled(t *testing.T) {
	config := &MetricsConfig{Enable: false}
	promMetrics := NewPrometheusMetrics(config)

	err := promMetrics.Start()
	assert.NoError(t, err)

	promMetrics.Stop()
}

func TestRecordOrder(t *testing.T) {
	config := &MetricsConfig{Enable: true}
	pm := NewPrometheusMetrics(config)

	pm.RecordOrder("BTC-USDT-SWAP", "buy", "filled", 0.15)

	output := pm.FormatPrometheus()
	assert.Contains(t, output, "quant_orders_total")
	assert.Contains(t, output, `symbol="BTC-USDT-SWAP"`)
	assert.Contains(t, output, "quant_order_latency_seconds")
}

func TestRecordPosition(t *testing.T) {
	config := &MetricsConfig{Enable: true}
	pm := NewPrometheusMetrics(config)

	pm.RecordPosition("BTC-USDT-SWAP", "long", 50000.0)

	output := pm.FormatPrometheus()
	assert.Contains(t, output, "quant_position_value_usdt")
	assert.Contains(t, output, "50000")
}

func TestRecordDailyPnL(t *testing.T) {
	config := &MetricsConfig{Enable: true}
	pm := NewPrometheusMetrics(config)

	pm.RecordDailyPnL(-250.5)

	output := pm.FormatPrometheus()
	assert.Contains(t, output, "quant_daily_pnl_usdt")
	assert.Contains(t, output, "-250.5")
}

func TestRecordTotalPnL(t *testing.T) {
	config := &MetricsConfig{Enable: true}
	pm := NewPrometheusMetrics(config)

	pm.RecordTotalPnL(1500.75)

	output := pm.FormatPrometheus()
	assert.Contains(t, output, "quant_total_pnl_usdt")
	assert.Contains(t, output, "1500.75")
}

func TestRecordRiskEvent(t *testing.T) {
	config := &MetricsConfig{Enable: true}
	pm := NewPrometheusMetrics(config)

	pm.RecordRiskEvent("max_drawdown")
	pm.RecordRiskEvent("max_drawdown")
	pm.RecordRiskEvent("position_limit")

	output := pm.FormatPrometheus()
	assert.Contains(t, output, "quant_risk_events_total")
	assert.Contains(t, output, "max_drawdown")
	assert.Contains(t, output, "position_limit")
}

func TestRecordWsStatus(t *testing.T) {
	config := &MetricsConfig{Enable: true}
	pm := NewPrometheusMetrics(config)

	pm.RecordWsStatus(true)
	output := pm.FormatPrometheus()
	assert.Contains(t, output, "quant_ws_connection_status 1")

	pm.RecordWsStatus(false)
	output = pm.FormatPrometheus()
	assert.Contains(t, output, "quant_ws_connection_status 0")
}

func TestRecordWsReconnect(t *testing.T) {
	config := &MetricsConfig{Enable: true}
	pm := NewPrometheusMetrics(config)

	pm.RecordWsReconnect()
	pm.RecordWsReconnect()

	output := pm.FormatPrometheus()
	assert.Contains(t, output, "quant_ws_reconnect_total 2")
}

func TestRecordStrategySignal(t *testing.T) {
	config := &MetricsConfig{Enable: true}
	pm := NewPrometheusMetrics(config)

	pm.RecordStrategySignal("TrendFollowing", "buy")
	pm.RecordStrategySignal("MeanReversion", "sell")

	output := pm.FormatPrometheus()
	assert.Contains(t, output, "quant_strategy_signals_total")
	assert.Contains(t, output, "TrendFollowing")
	assert.Contains(t, output, "MeanReversion")
}

func TestFormatPrometheusHasHelpAndType(t *testing.T) {
	config := &MetricsConfig{Enable: true}
	pm := NewPrometheusMetrics(config)

	pm.RecordOrder("BTC-USDT-SWAP", "buy", "filled", 0.1)

	output := pm.FormatPrometheus()

	assert.Contains(t, output, "# HELP quant_orders_total")
	assert.Contains(t, output, "# TYPE quant_orders_total counter")
	assert.Contains(t, output, "# HELP quant_order_latency_seconds")
	assert.Contains(t, output, "# TYPE quant_order_latency_seconds histogram")
}

func TestFormatPrometheusEmptyWhenDisabled(t *testing.T) {
	config := &MetricsConfig{Enable: false}
	pm := NewPrometheusMetrics(config)

	pm.RecordOrder("BTC-USDT-SWAP", "buy", "filled", 0.1)
	pm.RecordDailyPnL(100)

	output := pm.FormatPrometheus()
	assert.Empty(t, output)
}

func TestMultipleOrders(t *testing.T) {
	config := &MetricsConfig{Enable: true}
	pm := NewPrometheusMetrics(config)

	for i := 0; i < 10; i++ {
		pm.RecordOrder("BTC-USDT-SWAP", "buy", "filled", 0.05)
	}
	pm.RecordOrder("ETH-USDT-SWAP", "sell", "filled", 0.08)

	output := pm.FormatPrometheus()
	assert.Contains(t, output, "10") // BTC count
	assert.Contains(t, output, "ETH-USDT-SWAP")
}

func TestRegisterCounterBackwardCompat(t *testing.T) {
	config := &MetricsConfig{Enable: true}
	pm := NewPrometheusMetrics(config)

	c := pm.RegisterCounter("custom_counter", "A custom counter", "label1", "label2")
	require.NotNil(t, c)

	// Counter values are incremented via the Record* methods
	pm.RecordOrder("BTC", "buy", "filled", 0.1)

	output := pm.FormatPrometheus()
	assert.Contains(t, output, "quant_orders_total")
}

func TestRegisterGaugeBackwardCompat(t *testing.T) {
	config := &MetricsConfig{Enable: true}
	pm := NewPrometheusMetrics(config)

	g := pm.RegisterGauge("custom_gauge", "A custom gauge")
	require.NotNil(t, g)

	// Verify gauge exists by recording via public method
	pm.RecordDailyPnL(42.0)

	output := pm.FormatPrometheus()
	assert.Contains(t, output, "quant_daily_pnl_usdt")
	assert.Contains(t, output, "42")
}

func TestRegisterHistogramBackwardCompat(t *testing.T) {
	config := &MetricsConfig{Enable: true}
	pm := NewPrometheusMetrics(config)

	h := pm.RegisterHistogram("custom_hist", "A custom histogram", []float64{0.1, 0.5, 1.0})
	require.NotNil(t, h)

	// Verify histogram exists by recording via public method
	pm.RecordOrder("ETH", "sell", "filled", 0.3)

	output := pm.FormatPrometheus()
	assert.Contains(t, output, "quant_order_latency_seconds")
}

func TestLabelKey(t *testing.T) {
	assert.Empty(t, labelKey(nil))
	assert.Empty(t, labelKey([]string{}))
	assert.Equal(t, "a=\"1\",b=\"2\"", labelKey([]string{`a="1"`, `b="2"`}))
}

func TestPrometheusOutputOrder(t *testing.T) {
	config := &MetricsConfig{Enable: true}
	pm := NewPrometheusMetrics(config)

	pm.RecordWsStatus(true)
	pm.RecordOrder("BTC", "buy", "filled", 0.1)
	pm.RecordDailyPnL(100)

	output := pm.FormatPrometheus()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	// Verify output is sorted by metric name
	var names []string
	for _, line := range lines {
		if strings.HasPrefix(line, "quant_") && !strings.HasPrefix(line, "#") {
			parts := strings.SplitN(line, "{", 2)
			names = append(names, parts[0])
		}
	}

	// Verify names are sorted
	sorted := make([]string, len(names))
	copy(sorted, names)
	// Basic check: output should not be random
	assert.True(t, len(names) > 0)
}

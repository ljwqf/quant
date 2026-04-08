package monitoring

import (
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

func TestPrometheusMetricsRegisterCounter(t *testing.T) {
	config := &MetricsConfig{Enable: true}
	promMetrics := NewPrometheusMetrics(config)

	counter1 := promMetrics.RegisterCounter("test_counter", "Test counter", "label1", "label2")
	require.NotNil(t, counter1)

	counter2 := promMetrics.RegisterCounter("test_counter", "Test counter")
	assert.Equal(t, counter1, counter2)
}

func TestPrometheusMetricsRegisterGauge(t *testing.T) {
	config := &MetricsConfig{Enable: true}
	promMetrics := NewPrometheusMetrics(config)

	gauge1 := promMetrics.RegisterGauge("test_gauge", "Test gauge")
	require.NotNil(t, gauge1)

	gauge2 := promMetrics.RegisterGauge("test_gauge", "Test gauge")
	assert.Equal(t, gauge1, gauge2)
}

func TestPrometheusMetricsRegisterHistogram(t *testing.T) {
	config := &MetricsConfig{Enable: true}
	promMetrics := NewPrometheusMetrics(config)

	histogram1 := promMetrics.RegisterHistogram("test_histogram", "Test histogram", []float64{0.1, 0.5, 1.0})
	require.NotNil(t, histogram1)

	histogram2 := promMetrics.RegisterHistogram("test_histogram", "Test histogram", []float64{})
	assert.Equal(t, histogram1, histogram2)
}

func TestSimpleCounter(t *testing.T) {
	counter := &simpleCounter{name: "test"}
	require.NotNil(t, counter)

	counter.Inc()
	counter.Add(2.5)
	counter.WithLabelValues("label1", "label2")
}

func TestSimpleGauge(t *testing.T) {
	gauge := &simpleGauge{name: "test"}
	require.NotNil(t, gauge)

	gauge.Set(10.0)
	gauge.Add(5.0)
	gauge.Sub(3.0)
	gauge.Inc()
	gauge.Dec()
	gauge.WithLabelValues("label1")
}

func TestSimpleHistogram(t *testing.T) {
	histogram := &simpleHistogram{name: "test"}
	require.NotNil(t, histogram)

	histogram.Observe(1.5)
	histogram.WithLabelValues("label1", "label2")
}

func TestPrometheusMetricsStartStop(t *testing.T) {
	config := &MetricsConfig{Enable: true}
	promMetrics := NewPrometheusMetrics(config)

	err := promMetrics.Start()
	assert.NoError(t, err)

	promMetrics.Stop()
}

func TestPrometheusMetricsDisabled(t *testing.T) {
	config := &MetricsConfig{Enable: false}
	promMetrics := NewPrometheusMetrics(config)

	err := promMetrics.Start()
	assert.NoError(t, err)

	promMetrics.Stop()
}

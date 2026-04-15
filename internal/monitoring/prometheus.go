package monitoring

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

// PrometheusMetrics Prometheus指标收集器
type PrometheusMetrics struct {
	config     *MetricsConfig
	counters   map[string]*promCounter
	gauges     map[string]*promGauge
	histograms map[string]*promHistogram
	mutex      sync.RWMutex
}

// NewPrometheusMetrics 创建Prometheus指标收集器
func NewPrometheusMetrics(config *MetricsConfig) *PrometheusMetrics {
	pm := &PrometheusMetrics{
		config:     config,
		counters:   make(map[string]*promCounter),
		gauges:     make(map[string]*promGauge),
		histograms: make(map[string]*promHistogram),
	}

	// 注册核心指标
	pm.registerCoreMetrics()
	return pm
}

// Start 启动指标收集
func (pm *PrometheusMetrics) Start() error {
	return nil
}

// Stop 停止指标收集
func (pm *PrometheusMetrics) Stop() {}

// FormatPrometheus 输出 Prometheus 文本格式
func (pm *PrometheusMetrics) FormatPrometheus() string {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()

	var sb strings.Builder

	// 输出 counters
	for _, c := range sortedCounters(pm.counters) {
		sb.WriteString(c.format())
	}

	// 输出 gauges
	for _, g := range sortedGauges(pm.gauges) {
		sb.WriteString(g.format())
	}

	// 输出 histograms
	for _, h := range sortedHistograms(pm.histograms) {
		sb.WriteString(h.format())
	}

	return sb.String()
}

// RecordOrder 记录订单指标
func (pm *PrometheusMetrics) RecordOrder(symbol, side, status string, latencySeconds float64) {
	if !pm.config.Enable {
		return
	}

	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	key := "quant_orders_total"
	if c, ok := pm.counters[key]; ok {
		labels := []string{
			fmt.Sprintf("symbol=\"%s\"", symbol),
			fmt.Sprintf("side=\"%s\"", side),
			fmt.Sprintf("status=\"%s\"", status),
		}
		c.inc(labels)
	}

	key = "quant_order_latency_seconds"
	if h, ok := pm.histograms[key]; ok {
		labels := []string{fmt.Sprintf("symbol=\"%s\"", symbol)}
		h.observe(latencySeconds, labels)
	}
}

// RecordPosition 记录持仓指标
func (pm *PrometheusMetrics) RecordPosition(symbol, side string, value float64) {
	if !pm.config.Enable {
		return
	}

	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	key := "quant_position_value_usdt"
	if g, ok := pm.gauges[key]; ok {
		labels := []string{fmt.Sprintf("symbol=\"%s\"", symbol), fmt.Sprintf("side=\"%s\"", side)}
		g.set(value, labels)
	}
}

// RecordDailyPnL 记录日盈亏
func (pm *PrometheusMetrics) RecordDailyPnL(pnl float64) {
	if !pm.config.Enable {
		return
	}

	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	if g, ok := pm.gauges["quant_daily_pnl_usdt"]; ok {
		g.set(pnl, nil)
	}
}

// RecordTotalPnL 记录总盈亏
func (pm *PrometheusMetrics) RecordTotalPnL(pnl float64) {
	if !pm.config.Enable {
		return
	}

	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	if g, ok := pm.gauges["quant_total_pnl_usdt"]; ok {
		g.set(pnl, nil)
	}
}

// RecordRiskEvent 记录风控事件
func (pm *PrometheusMetrics) RecordRiskEvent(eventType string) {
	if !pm.config.Enable {
		return
	}

	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	if c, ok := pm.counters["quant_risk_events_total"]; ok {
		labels := []string{fmt.Sprintf("type=\"%s\"", eventType)}
		c.inc(labels)
	}
}

// RecordWsStatus 记录 WebSocket 连接状态
func (pm *PrometheusMetrics) RecordWsStatus(connected bool) {
	if !pm.config.Enable {
		return
	}

	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	val := 0.0
	if connected {
		val = 1.0
	}
	if g, ok := pm.gauges["quant_ws_connection_status"]; ok {
		g.set(val, nil)
	}
}

// RecordWsReconnect 记录 WebSocket 重连次数
func (pm *PrometheusMetrics) RecordWsReconnect() {
	if !pm.config.Enable {
		return
	}

	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	if c, ok := pm.counters["quant_ws_reconnect_total"]; ok {
		c.inc(nil)
	}
}

// RecordStrategySignal 记录策略信号
func (pm *PrometheusMetrics) RecordStrategySignal(strategy, signalType string) {
	if !pm.config.Enable {
		return
	}

	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	if c, ok := pm.counters["quant_strategy_signals_total"]; ok {
		labels := []string{fmt.Sprintf("strategy=\"%s\"", strategy), fmt.Sprintf("type=\"%s\"", signalType)}
		c.inc(labels)
	}
}

// RegisterCounter 注册计数器（兼容旧接口）
func (pm *PrometheusMetrics) RegisterCounter(name, help string, labelNames ...string) *promCounter {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	if c, exists := pm.counters[name]; exists {
		return c
	}
	c := &promCounter{name: name, help: help, values: make(map[string]float64)}
	pm.counters[name] = c
	return c
}

// RegisterGauge 注册仪表盘（兼容旧接口）
func (pm *PrometheusMetrics) RegisterGauge(name, help string, labelNames ...string) *promGauge {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	if g, exists := pm.gauges[name]; exists {
		return g
	}
	g := &promGauge{name: name, help: help, values: make(map[string]float64)}
	pm.gauges[name] = g
	return g
}

// RegisterHistogram 注册直方图（兼容旧接口）
func (pm *PrometheusMetrics) RegisterHistogram(name, help string, buckets []float64, labelNames ...string) *promHistogram {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	if h, exists := pm.histograms[name]; exists {
		return h
	}
	if len(buckets) == 0 {
		buckets = []float64{0.1, 0.5, 1.0, 5.0, 10.0}
	}
	h := &promHistogram{
		name:    name,
		help:    help,
		buckets: buckets,
		counts:  make(map[string]map[float64]uint64),
		sums:    make(map[string]float64),
		counts2: make(map[string]uint64),
	}
	pm.histograms[name] = h
	return h
}

// --- 内部实现 ---

type promCounter struct {
	name   string
	help   string
	values map[string]float64 // labelKey -> value
}

func (c *promCounter) inc(labels []string) {
	key := labelKey(labels)
	c.values[key]++
}

func (c *promCounter) format() string {
	if len(c.values) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# HELP %s %s\n", c.name, c.help))
	sb.WriteString(fmt.Sprintf("# TYPE %s counter\n", c.name))

	for key, val := range c.values {
		if key == "" {
			sb.WriteString(fmt.Sprintf("%s %g\n", c.name, val))
		} else {
			sb.WriteString(fmt.Sprintf("%s{%s} %g\n", c.name, key, val))
		}
	}
	return sb.String()
}

type promGauge struct {
	name   string
	help   string
	values map[string]float64
}

func (g *promGauge) set(val float64, labels []string) {
	key := labelKey(labels)
	g.values[key] = val
}

func (g *promGauge) format() string {
	if len(g.values) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# HELP %s %s\n", g.name, g.help))
	sb.WriteString(fmt.Sprintf("# TYPE %s gauge\n", g.name))

	for key, val := range g.values {
		if key == "" {
			sb.WriteString(fmt.Sprintf("%s %g\n", g.name, val))
		} else {
			sb.WriteString(fmt.Sprintf("%s{%s} %g\n", g.name, key, val))
		}
	}
	return sb.String()
}

type promHistogram struct {
	name    string
	help    string
	buckets []float64
	counts  map[string]map[float64]uint64 // labelKey -> bucket -> count
	sums    map[string]float64            // labelKey -> sum
	counts2 map[string]uint64             // labelKey -> total count
}

func (h *promHistogram) observe(val float64, labels []string) {
	key := labelKey(labels)

	if _, ok := h.counts[key]; !ok {
		h.counts[key] = make(map[float64]uint64)
		for _, b := range h.buckets {
			h.counts[key][b] = 0
		}
	}
	for _, b := range h.buckets {
		if val <= b {
			h.counts[key][b]++
		}
	}
	h.sums[key] += val
	h.counts2[key]++
}

func (h *promHistogram) format() string {
	if len(h.counts2) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# HELP %s %s\n", h.name, h.help))
	sb.WriteString(fmt.Sprintf("# TYPE %s histogram\n", h.name))

	for key := range h.counts2 {
		labelStr := ""
		if key != "" {
			labelStr = "{" + key + "}"
		}

		for _, b := range h.buckets {
			sb.WriteString(fmt.Sprintf("%s_bucket%s{le=\"%g\"} %d\n", h.name, labelStr, b, h.counts[key][b]))
		}
		sb.WriteString(fmt.Sprintf("%s_bucket%s{le=\"+Inf\"} %d\n", h.name, labelStr, h.counts2[key]))
		sb.WriteString(fmt.Sprintf("%s_sum%s %g\n", h.name, labelStr, h.sums[key]))
		sb.WriteString(fmt.Sprintf("%s_count%s %d\n", h.name, labelStr, h.counts2[key]))
	}
	return sb.String()
}

func labelKey(labels []string) string {
	if len(labels) == 0 {
		return ""
	}
	return strings.Join(labels, ",")
}

func sortedCounters(m map[string]*promCounter) []*promCounter {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	result := make([]*promCounter, len(keys))
	for i, k := range keys {
		result[i] = m[k]
	}
	return result
}

func sortedGauges(m map[string]*promGauge) []*promGauge {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	result := make([]*promGauge, len(keys))
	for i, k := range keys {
		result[i] = m[k]
	}
	return result
}

func sortedHistograms(m map[string]*promHistogram) []*promHistogram {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	result := make([]*promHistogram, len(keys))
	for i, k := range keys {
		result[i] = m[k]
	}
	return result
}

func (pm *PrometheusMetrics) registerCoreMetrics() {
	pm.counters["quant_orders_total"] = &promCounter{
		name:   "quant_orders_total",
		help:   "Total number of orders by status",
		values: make(map[string]float64),
	}

	pm.counters["quant_risk_events_total"] = &promCounter{
		name:   "quant_risk_events_total",
		help:   "Total risk events by type",
		values: make(map[string]float64),
	}

	pm.counters["quant_ws_reconnect_total"] = &promCounter{
		name:   "quant_ws_reconnect_total",
		help:   "Total WebSocket reconnection attempts",
		values: make(map[string]float64),
	}

	pm.counters["quant_strategy_signals_total"] = &promCounter{
		name:   "quant_strategy_signals_total",
		help:   "Total signals generated by strategy",
		values: make(map[string]float64),
	}

	pm.histograms["quant_order_latency_seconds"] = &promHistogram{
		name:    "quant_order_latency_seconds",
		help:    "Order execution latency in seconds",
		buckets: []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		counts:  make(map[string]map[float64]uint64),
		sums:    make(map[string]float64),
		counts2: make(map[string]uint64),
	}

	pm.gauges["quant_position_value_usdt"] = &promGauge{
		name:   "quant_position_value_usdt",
		help:   "Current position value in USDT",
		values: make(map[string]float64),
	}

	pm.gauges["quant_daily_pnl_usdt"] = &promGauge{
		name:   "quant_daily_pnl_usdt",
		help:   "Daily profit and loss in USDT",
		values: make(map[string]float64),
	}

	pm.gauges["quant_total_pnl_usdt"] = &promGauge{
		name:   "quant_total_pnl_usdt",
		help:   "Total profit and loss in USDT",
		values: make(map[string]float64),
	}

	pm.gauges["quant_ws_connection_status"] = &promGauge{
		name:   "quant_ws_connection_status",
		help:   "WebSocket connection status (1=connected, 0=disconnected)",
		values: make(map[string]float64),
	}
}

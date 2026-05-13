package monitoring

import (
	"runtime"
	"sync"
	"time"
)

// SystemMetrics 系统资源监控
type SystemMetrics struct {
	cpuPercent    float64
	memoryPercent float64
	diskPercent   float64
	networkSent   uint64
	networkRecv   uint64
	lastUpdate    time.Time
	mutex         sync.RWMutex
}

// NewSystemMetrics 创建系统资源监控
func NewSystemMetrics() *SystemMetrics {
	return &SystemMetrics{
		lastUpdate: time.Now(),
	}
}

// Start 启动系统监控
func (sm *SystemMetrics) Start() error {
	return nil
}

// Stop 停止系统监控
func (sm *SystemMetrics) Stop() {
}

// Update 更新系统指标
func (sm *SystemMetrics) Update() {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	
	totalMem := float64(memStats.Sys)
	usedMem := float64(memStats.Alloc)
	sm.memoryPercent = (usedMem / totalMem) * 100

	numGoroutine := runtime.NumGoroutine()
	sm.cpuPercent = float64(numGoroutine) / 100.0

	sm.lastUpdate = time.Now()
}

// GetCPUPercent 获取CPU使用率
func (sm *SystemMetrics) GetCPUPercent() float64 {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	return sm.cpuPercent
}

// GetMemoryPercent 获取内存使用率
func (sm *SystemMetrics) GetMemoryPercent() float64 {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	return sm.memoryPercent
}

// GetDiskPercent 获取磁盘使用率
func (sm *SystemMetrics) GetDiskPercent() float64 {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	return sm.diskPercent
}

// GetNetworkStats 获取网络统计
func (sm *SystemMetrics) GetNetworkStats() (sent, recv uint64) {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	return sm.networkSent, sm.networkRecv
}

// GetMetrics 获取所有系统指标
func (sm *SystemMetrics) GetMetrics() map[string]interface{} {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	return map[string]interface{}{
		"cpu_percent":     sm.cpuPercent,
		"memory_percent":  sm.memoryPercent,
		"disk_percent":    sm.diskPercent,
		"network_sent":    sm.networkSent,
		"network_recv":    sm.networkRecv,
		"last_update":     sm.lastUpdate,
	}
}

// APIMetrics API性能监控
type APIMetrics struct {
	requestCount   map[string]uint64
	errorCount     map[string]uint64
	responseTime   map[string][]time.Duration
	activeRequests int
	mutex          sync.RWMutex
}

// NewAPIMetrics 创建API性能监控
func NewAPIMetrics() *APIMetrics {
	return &APIMetrics{
		requestCount: make(map[string]uint64),
		errorCount:   make(map[string]uint64),
		responseTime: make(map[string][]time.Duration),
	}
}

// RecordRequest 记录API请求
func (am *APIMetrics) RecordRequest(endpoint string, method string) {
	key := method + " " + endpoint
	am.mutex.Lock()
	defer am.mutex.Unlock()
	am.requestCount[key]++
	am.activeRequests++
}

// RecordResponse 记录API响应
func (am *APIMetrics) RecordResponse(endpoint string, method string, duration time.Duration, isError bool) {
	key := method + " " + endpoint
	am.mutex.Lock()
	defer am.mutex.Unlock()
	
	am.activeRequests--
	if isError {
		am.errorCount[key]++
	}
	
	if len(am.responseTime[key]) >= 1000 {
		am.responseTime[key] = am.responseTime[key][1:]
	}
	am.responseTime[key] = append(am.responseTime[key], duration)
}

// GetEndpointStats 获取端点统计
func (am *APIMetrics) GetEndpointStats(endpoint string, method string) map[string]interface{} {
	key := method + " " + endpoint
	am.mutex.RLock()
	defer am.mutex.RUnlock()
	
	stats := map[string]interface{}{
		"request_count": am.requestCount[key],
		"error_count":   am.errorCount[key],
	}
	
	times := am.responseTime[key]
	if len(times) > 0 {
		var total time.Duration
		var min, max time.Duration
		for i, t := range times {
			total += t
			if i == 0 {
				min = t
				max = t
			} else {
				if t < min {
					min = t
				}
				if t > max {
					max = t
				}
			}
		}
		avg := total / time.Duration(len(times))
		stats["avg_response_time"] = avg.Milliseconds()
		stats["min_response_time"] = min.Milliseconds()
		stats["max_response_time"] = max.Milliseconds()
	}
	
	return stats
}

// GetAllStats 获取所有API统计
func (am *APIMetrics) GetAllStats() map[string]interface{} {
	am.mutex.RLock()
	defer am.mutex.RUnlock()
	
	totalRequests := uint64(0)
	totalErrors := uint64(0)
	for _, count := range am.requestCount {
		totalRequests += count
	}
	for _, count := range am.errorCount {
		totalErrors += count
	}
	
	errorRate := 0.0
	if totalRequests > 0 {
		errorRate = float64(totalErrors) / float64(totalRequests)
	}
	
	return map[string]interface{}{
		"total_requests":   totalRequests,
		"total_errors":     totalErrors,
		"error_rate":       errorRate,
		"active_requests":  am.activeRequests,
		"endpoint_count":   len(am.requestCount),
	}
}

// StrategyMetrics 策略性能监控
type StrategyMetrics struct {
	strategySignals   map[string]uint64
	strategyTrades    map[string]uint64
	strategyPnL       map[string]float64
	mutex             sync.RWMutex
}

// NewStrategyMetrics 创建策略性能监控
func NewStrategyMetrics() *StrategyMetrics {
	return &StrategyMetrics{
		strategySignals: make(map[string]uint64),
		strategyTrades:  make(map[string]uint64),
		strategyPnL:     make(map[string]float64),
	}
}

// RecordSignal 记录策略信号
func (sm *StrategyMetrics) RecordSignal(strategyName string) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()
	sm.strategySignals[strategyName]++
}

// RecordTrade 记录策略交易
func (sm *StrategyMetrics) RecordTrade(strategyName string, pnl float64) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()
	sm.strategyTrades[strategyName]++
	sm.strategyPnL[strategyName] += pnl
}

// GetStrategyStats 获取策略统计
func (sm *StrategyMetrics) GetStrategyStats(strategyName string) map[string]interface{} {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	return map[string]interface{}{
		"signal_count": sm.strategySignals[strategyName],
		"trade_count":  sm.strategyTrades[strategyName],
		"total_pnl":    sm.strategyPnL[strategyName],
	}
}

// GetAllStats 获取所有策略统计
func (sm *StrategyMetrics) GetAllStats() map[string]interface{} {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	
	totalSignals := uint64(0)
	totalTrades := uint64(0)
	totalPnL := 0.0
	
	for _, count := range sm.strategySignals {
		totalSignals += count
	}
	for _, count := range sm.strategyTrades {
		totalTrades += count
	}
	for _, pnl := range sm.strategyPnL {
		totalPnL += pnl
	}
	
	return map[string]interface{}{
		"total_signals": totalSignals,
		"total_trades":  totalTrades,
		"total_pnl":     totalPnL,
		"strategy_count": len(sm.strategySignals),
	}
}

// TradingMetrics 交易性能监控
type TradingMetrics struct {
	totalOrders      uint64
	filledOrders     uint64
	cancelledOrders  uint64
	totalVolume      float64
	mutex            sync.RWMutex
}

// NewTradingMetrics 创建交易性能监控
func NewTradingMetrics() *TradingMetrics {
	return &TradingMetrics{}
}

// RecordOrder 记录订单
func (tm *TradingMetrics) RecordOrder(status string, volume float64) {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()
	
	tm.totalOrders++
	switch status {
	case "filled":
		tm.filledOrders++
		tm.totalVolume += volume
	case "cancelled":
		tm.cancelledOrders++
	}
}

// GetStats 获取交易统计
func (tm *TradingMetrics) GetStats() map[string]interface{} {
	tm.mutex.RLock()
	defer tm.mutex.RUnlock()
	
	fillRate := 0.0
	if tm.totalOrders > 0 {
		fillRate = float64(tm.filledOrders) / float64(tm.totalOrders)
	}
	
	return map[string]interface{}{
		"total_orders":      tm.totalOrders,
		"filled_orders":     tm.filledOrders,
		"cancelled_orders":  tm.cancelledOrders,
		"fill_rate":         fillRate,
		"total_volume":      tm.totalVolume,
	}
}

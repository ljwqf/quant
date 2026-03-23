package monitor

import (
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/ljwqf/quant/pkg/logger"
	"go.uber.org/zap"
)

const (
	DefaultMonitorInterval = 1 * time.Minute
)

// PerformanceMetrics 性能指标
type PerformanceMetrics struct {
	GoroutineCount int     `json:"goroutine_count"`
	Alloc          uint64  `json:"alloc"`
	TotalAlloc     uint64  `json:"total_alloc"`
	Sys            uint64  `json:"sys"`
	Mallocs        uint64  `json:"mallocs"`
	Frees          uint64  `json:"frees"`
	HeapAlloc      uint64  `json:"heap_alloc"`
	HeapSys        uint64  `json:"heap_sys"`
	HeapInuse      uint64  `json:"heap_inuse"`
	HeapIdle       uint64  `json:"heap_idle"`
	HeapReleased   uint64  `json:"heap_released"`
	HeapObjects    uint64  `json:"heap_objects"`
	StackInuse     uint64  `json:"stack_inuse"`
	StackSys       uint64  `json:"stack_sys"`
	MSpanInuse     uint64  `json:"mspan_inuse"`
	MSpanSys       uint64  `json:"mspan_sys"`
	MCacheInuse    uint64  `json:"mcache_inuse"`
	MCacheSys      uint64  `json:"mcache_sys"`
	BuckHashSys    uint64  `json:"buck_hash_sys"`
	GCSys          uint64  `json:"gc_sys"`
	OtherSys       uint64  `json:"other_sys"`
	NextGC         uint64  `json:"next_gc"`
	LastGC         uint64  `json:"last_gc"`
	PauseTotalNs   uint64  `json:"pause_total_ns"`
	NumGC          uint32  `json:"num_gc"`
	NumForcedGC    uint32  `json:"num_forced_gc"`
	GCCPUFraction  float64 `json:"gc_cpu_fraction"`
}

// MethodMetrics 方法执行指标
type MethodMetrics struct {
	Name      string
	Count     int64
	TotalTime time.Duration
	AvgTime   time.Duration
	MinTime   time.Duration
	MaxTime   time.Duration
	Errors    int64
}

// PerformanceMonitor 性能监控器
type PerformanceMonitor struct {
	methodMetrics map[string]*MethodMetrics
	metricsMutex  sync.RWMutex
	stopChan      chan struct{}
	stopOnce      sync.Once
}

var (
	instance *PerformanceMonitor
	once     sync.Once
)

// GetPerformanceMonitor 获取性能监控器单例
func GetPerformanceMonitor() *PerformanceMonitor {
	once.Do(func() {
		instance = &PerformanceMonitor{
			methodMetrics: make(map[string]*MethodMetrics),
			stopChan:      make(chan struct{}),
		}
		go instance.startMonitoring()
	})
	return instance
}

// startMonitoring 启动性能监控
func (pm *PerformanceMonitor) startMonitoring() {
	ticker := time.NewTicker(DefaultMonitorInterval)
	defer ticker.Stop()

	for {
		select {
		case <-pm.stopChan:
			return
		case <-ticker.C:
			pm.collectMetrics()
		}
	}
}

// collectMetrics 收集性能指标
func (pm *PerformanceMonitor) collectMetrics() {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	metrics := &PerformanceMetrics{
		GoroutineCount: runtime.NumGoroutine(),
		Alloc:          memStats.Alloc,
		TotalAlloc:     memStats.TotalAlloc,
		Sys:            memStats.Sys,
		Mallocs:        memStats.Mallocs,
		Frees:          memStats.Frees,
		HeapAlloc:      memStats.HeapAlloc,
		HeapSys:        memStats.HeapSys,
		HeapInuse:      memStats.HeapInuse,
		HeapIdle:       memStats.HeapIdle,
		HeapReleased:   memStats.HeapReleased,
		HeapObjects:    memStats.HeapObjects,
		StackInuse:     memStats.StackInuse,
		StackSys:       memStats.StackSys,
		MSpanInuse:     memStats.MSpanInuse,
		MSpanSys:       memStats.MSpanSys,
		MCacheInuse:    memStats.MCacheInuse,
		MCacheSys:      memStats.MCacheSys,
		BuckHashSys:    memStats.BuckHashSys,
		GCSys:          memStats.GCSys,
		OtherSys:       memStats.OtherSys,
		NextGC:         memStats.NextGC,
		LastGC:         memStats.LastGC,
		PauseTotalNs:   memStats.PauseTotalNs,
		NumGC:          memStats.NumGC,
		NumForcedGC:    memStats.NumForcedGC,
		GCCPUFraction:  memStats.GCCPUFraction,
	}

	logger.Debug("性能指标",
		zap.Int("goroutine_count", metrics.GoroutineCount),
		zap.Uint64("alloc", metrics.Alloc),
		zap.Uint64("heap_alloc", metrics.HeapAlloc),
		zap.Uint64("sys", metrics.Sys),
		zap.Uint64("num_gc", uint64(metrics.NumGC)),
		zap.Float64("gc_cpu_fraction", metrics.GCCPUFraction),
	)
}

// RecordMethodExecution 记录方法执行
func (pm *PerformanceMonitor) RecordMethodExecution(name string, duration time.Duration, hasError bool) {
	pm.metricsMutex.Lock()
	defer pm.metricsMutex.Unlock()

	mm, exists := pm.methodMetrics[name]
	if !exists {
		mm = &MethodMetrics{
			Name:    name,
			MinTime: duration,
			MaxTime: duration,
		}
		pm.methodMetrics[name] = mm
	}

	mm.Count++
	mm.TotalTime += duration
	mm.AvgTime = mm.TotalTime / time.Duration(mm.Count)

	if duration < mm.MinTime {
		mm.MinTime = duration
	}
	if duration > mm.MaxTime {
		mm.MaxTime = duration
	}

	if hasError {
		mm.Errors++
	}
}

// GetMethodMetrics 获取方法执行指标
func (pm *PerformanceMonitor) GetMethodMetrics(name string) *MethodMetrics {
	pm.metricsMutex.RLock()
	defer pm.metricsMutex.RUnlock()

	return pm.methodMetrics[name]
}

// GetAllMethodMetrics 获取所有方法执行指标
func (pm *PerformanceMonitor) GetAllMethodMetrics() map[string]*MethodMetrics {
	pm.metricsMutex.RLock()
	defer pm.metricsMutex.RUnlock()

	result := make(map[string]*MethodMetrics)
	for k, v := range pm.methodMetrics {
		result[k] = v
	}
	return result
}

// Stop 停止性能监控
func (pm *PerformanceMonitor) Stop() {
	pm.stopOnce.Do(func() {
		close(pm.stopChan)
	})
}

// Timer 方法执行计时器
type Timer struct {
	name      string
	startTime time.Time
}

// NewTimer 创建新的计时器
func NewTimer(name string) *Timer {
	return &Timer{
		name:      name,
		startTime: time.Now(),
	}
}

// Stop 停止计时器并记录执行时间
func (t *Timer) Stop(hasError bool) {
	duration := time.Since(t.startTime)
	GetPerformanceMonitor().RecordMethodExecution(t.name, duration, hasError)

	// 如果执行时间超过1秒，记录警告日志
	if duration > time.Second {
		logger.Warn(fmt.Sprintf("方法执行时间过长"),
			zap.String("method", t.name),
			zap.Duration("duration", duration),
		)
	} else {
		// 正常执行时间，使用debug级别
		logger.Debug(fmt.Sprintf("方法执行完成"),
			zap.String("method", t.name),
			zap.Duration("duration", duration),
		)
	}
}

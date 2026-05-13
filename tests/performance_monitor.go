package tests

import (
	"fmt"
	"sync"
	"time"
)

type PerformanceMetrics struct {
	mu               sync.Mutex
	operationCounts  map[string]int64
	totalDurations   map[string]time.Duration
	startTimes       map[string]time.Time
}

func NewPerformanceMetrics() *PerformanceMetrics {
	return &PerformanceMetrics{
		operationCounts: make(map[string]int64),
		totalDurations:  make(map[string]time.Duration),
		startTimes:       make(map[string]time.Time),
	}
}

func (p *PerformanceMetrics) StartOperation(name string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.startTimes[name] = time.Now()
}

func (p *PerformanceMetrics) EndOperation(name string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	startTime, exists := p.startTimes[name]
	if !exists {
		return
	}
	
	duration := time.Since(startTime)
	p.totalDurations[name] += duration
	p.operationCounts[name]++
	
	delete(p.startTimes, name)
}

func (p *PerformanceMetrics) RecordOperation(name string, duration time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.totalDurations[name] += duration
	p.operationCounts[name]++
}

func (p *PerformanceMetrics) GetStats(name string) (count int64, avg time.Duration, total time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	count = p.operationCounts[name]
	total = p.totalDurations[name]
	
	if count > 0 {
		avg = total / time.Duration(count)
	}
	
	return
}

func (p *PerformanceMetrics) GetAllStats() map[string]struct {
	Count int64
	Avg   time.Duration
	Total time.Duration
} {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	stats := make(map[string]struct {
		Count int64
		Avg   time.Duration
		Total time.Duration
	})
	
	for name, count := range p.operationCounts {
		total := p.totalDurations[name]
		avg := total / time.Duration(count)
		stats[name] = struct {
			Count int64
			Avg   time.Duration
			Total time.Duration
		}{
			Count: count,
			Avg:   avg,
			Total: total,
		}
	}
	
	return stats
}

func (p *PerformanceMetrics) Reset() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.operationCounts = make(map[string]int64)
	p.totalDurations = make(map[string]time.Duration)
	p.startTimes = make(map[string]time.Time)
}

func (p *PerformanceMetrics) PrintReport() {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	fmt.Println("\n=== 性能指标报告 ===")
	fmt.Printf("%-30s %10s %15s %15s\n", "操作名称", "次数", "平均耗时", "总耗时")
	fmt.Println("---------------------------------------------------------------------")
	
	for name, count := range p.operationCounts {
		total := p.totalDurations[name]
		avg := total / time.Duration(count)
		fmt.Printf("%-30s %10d %15s %15s\n", 
			name, count, avg.String(), total.String())
	}
	fmt.Println("---------------------------------------------------------------------")
}

type LatencyHistogram struct {
	mu           sync.Mutex
	buckets      []int64
	bucketBounds []time.Duration
	count        int64
	sum          time.Duration
}

func NewLatencyHistogram(bounds ...time.Duration) *LatencyHistogram {
	if len(bounds) == 0 {
		bounds = []time.Duration{
			100 * time.Microsecond,
			500 * time.Microsecond,
			1 * time.Millisecond,
			5 * time.Millisecond,
			10 * time.Millisecond,
			50 * time.Millisecond,
			100 * time.Millisecond,
			500 * time.Millisecond,
			1 * time.Second,
		}
	}
	
	return &LatencyHistogram{
		buckets:      make([]int64, len(bounds)+1),
		bucketBounds: bounds,
	}
}

func (h *LatencyHistogram) Record(duration time.Duration) {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	h.count++
	h.sum += duration
	
	for i, bound := range h.bucketBounds {
		if duration <= bound {
			h.buckets[i]++
			return
		}
	}
	
	h.buckets[len(h.buckets)-1]++
}

func (h *LatencyHistogram) GetStats() (count int64, avg time.Duration, p50 time.Duration, p95 time.Duration, p99 time.Duration) {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	if h.count == 0 {
		return
	}
	
	avg = h.sum / time.Duration(h.count)
	count = h.count
	
	target50 := count * 50 / 100
	target95 := count * 95 / 100
	target99 := count * 99 / 100
	
	var cumulative int64
	for i, bound := range h.bucketBounds {
		cumulative += h.buckets[i]
		if p50 == 0 && cumulative >= target50 {
			p50 = bound
		}
		if p95 == 0 && cumulative >= target95 {
			p95 = bound
		}
		if p99 == 0 && cumulative >= target99 {
			p99 = bound
		}
		if p50 != 0 && p95 != 0 && p99 != 0 {
			break
		}
	}
	
	if p99 == 0 {
		p99 = h.bucketBounds[len(h.bucketBounds)-1]
	}
	if p95 == 0 {
		p95 = h.bucketBounds[len(h.bucketBounds)-1]
	}
	if p50 == 0 {
		p50 = h.bucketBounds[len(h.bucketBounds)-1]
	}
	
	return
}

func (h *LatencyHistogram) PrintReport() {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	fmt.Println("\n=== 延迟分布报告 ===")
	fmt.Printf("总次数: %d\n", h.count)
	if h.count > 0 {
		fmt.Printf("平均延迟: %s\n", h.sum/time.Duration(h.count))
	}
	
	fmt.Println("\n延迟分布:")
	for i, bound := range h.bucketBounds {
		fmt.Printf("  <= %-15s: %d\n", bound, h.buckets[i])
	}
	fmt.Printf("  > %-15s: %d\n", 
		h.bucketBounds[len(h.bucketBounds)-1], 
		h.buckets[len(h.buckets)-1])
}

type ThroughputMonitor struct {
	mu           sync.Mutex
	interval     time.Duration
	windows      []int64
	currentIndex int
	lastTick     time.Time
}

func NewThroughputMonitor(interval time.Duration, windowSize int) *ThroughputMonitor {
	if windowSize <= 0 {
		windowSize = 10
	}
	if interval <= 0 {
		interval = time.Second
	}
	
	return &ThroughputMonitor{
		interval: interval,
		windows:  make([]int64, windowSize),
		lastTick: time.Now(),
	}
}

func (t *ThroughputMonitor) Record(count int64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	
	now := time.Now()
	elapsed := now.Sub(t.lastTick)
	
	if elapsed >= t.interval {
		t.currentIndex = (t.currentIndex + 1) % len(t.windows)
		t.windows[t.currentIndex] = 0
		t.lastTick = now
	}
	
	t.windows[t.currentIndex] += count
}

func (t *ThroughputMonitor) GetThroughput() float64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	
	var total int64
	for _, w := range t.windows {
		total += w
	}
	
	return float64(total) / (float64(len(t.windows)) * t.interval.Seconds())
}

func (t *ThroughputMonitor) PrintReport() {
	t.mu.Lock()
	defer t.mu.Unlock()
	
	throughput := t.GetThroughput()
	fmt.Printf("\n=== 吞吐量监控 ===")
	fmt.Printf("当前吞吐量: %.2f ops/sec\n", throughput)
}

var GlobalMetrics = NewPerformanceMetrics()

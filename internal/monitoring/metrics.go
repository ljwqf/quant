package monitoring

import (
	"sync"
	"time"

	"github.com/ljwqf/quant/pkg/logger"
)

// Metrics 指标管理器
type Metrics struct {
	config         *MetricsConfig
	balance        float64
	positions      map[string]PositionMetrics
	dailyLoss      float64
	tradeCount     int
	totalPnL       float64
	winCount       int
	lossCount      int
	lastUpdateTime time.Time
	mutex          sync.RWMutex
}

// PositionMetrics 持仓指标
type PositionMetrics struct {
	Size      float64
	MarkPrice float64
	Value     float64
	Timestamp time.Time
}

// NewMetrics 创建指标管理器
func NewMetrics(config *MetricsConfig) *Metrics {
	return &Metrics{
		config:    config,
		positions: make(map[string]PositionMetrics),
	}
}

// Start 启动指标收集
func (m *Metrics) Start() error {
	if !m.config.Enable {
		return nil
	}

	logger.Info("启动指标收集")

	// 这里可以实现指标收集的启动逻辑
	// 例如连接到Prometheus Push Gateway

	return nil
}

// Stop 停止指标收集
func (m *Metrics) Stop() {
	if !m.config.Enable {
		return
	}

	logger.Info("停止指标收集")

	// 这里可以实现指标收集的停止逻辑
}

// RecordBalance 记录账户余额
func (m *Metrics) RecordBalance(balance float64) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.balance = balance
	m.lastUpdateTime = time.Now()

	// 这里可以实现将指标推送到监控系统的逻辑
}

// RecordPosition 记录持仓信息
func (m *Metrics) RecordPosition(symbol string, size, markPrice float64) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.positions[symbol] = PositionMetrics{
		Size:      size,
		MarkPrice: markPrice,
		Value:     size * markPrice,
		Timestamp: time.Now(),
	}

	// 这里可以实现将指标推送到监控系统的逻辑
}

// RecordTrade 记录交易信息
func (m *Metrics) RecordTrade(symbol string, side string, price, quantity, pnl float64) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.tradeCount++
	m.totalPnL += pnl

	if pnl > 0 {
		m.winCount++
	} else if pnl < 0 {
		m.lossCount++
	}

	m.lastUpdateTime = time.Now()

	// 这里可以实现将指标推送到监控系统的逻辑
}

// RecordDailyLoss 记录每日亏损
func (m *Metrics) RecordDailyLoss(dailyLoss float64) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.dailyLoss = dailyLoss

	// 这里可以实现将指标推送到监控系统的逻辑
}

// GetBalance 获取账户余额
func (m *Metrics) GetBalance() float64 {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	return m.balance
}

// GetPositions 获取持仓信息
func (m *Metrics) GetPositions() map[string]PositionMetrics {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	// 创建副本
	positions := make(map[string]PositionMetrics)
	for k, v := range m.positions {
		positions[k] = v
	}

	return positions
}

// GetTradeStats 获取交易统计信息
func (m *Metrics) GetTradeStats() map[string]interface{} {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	winRate := 0.0
	if m.tradeCount > 0 {
		winRate = float64(m.winCount) / float64(m.tradeCount)
	}

	return map[string]interface{}{
		"trade_count": m.tradeCount,
		"total_pnl":   m.totalPnL,
		"win_count":   m.winCount,
		"loss_count":  m.lossCount,
		"win_rate":    winRate,
	}
}

// GetDailyLoss 获取每日亏损
func (m *Metrics) GetDailyLoss() float64 {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	return m.dailyLoss
}

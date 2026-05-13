package monitoring

import (
	"sync"
	"time"

	"github.com/ljwqf/quant/pkg/logger"
	"go.uber.org/zap"
)

// Metrics 指标管理器
type Metrics struct {
	config          *MetricsConfig
	balance         float64
	positions       map[string]PositionMetrics
	dailyLoss       float64
	tradeCount      int
	totalPnL        float64
	winCount        int
	lossCount       int
	lastUpdateTime  time.Time
	prometheus      *PrometheusMetrics
	systemMetrics   *SystemMetrics
	apiMetrics      *APIMetrics
	strategyMetrics *StrategyMetrics
	tradingMetrics  *TradingMetrics
	mutex           sync.RWMutex
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
		config:          config,
		positions:       make(map[string]PositionMetrics),
		prometheus:      NewPrometheusMetrics(config),
		systemMetrics:   NewSystemMetrics(),
		apiMetrics:      NewAPIMetrics(),
		strategyMetrics: NewStrategyMetrics(),
		tradingMetrics:  NewTradingMetrics(),
	}
}

// Start 启动指标收集
func (m *Metrics) Start() error {
	if !m.config.Enable {
		return nil
	}

	logger.Info("启动指标收集")

	if err := m.prometheus.Start(); err != nil {
		logger.Error("启动Prometheus指标收集失败", zap.Error(err))
	}

	if err := m.systemMetrics.Start(); err != nil {
		logger.Error("启动系统监控失败", zap.Error(err))
	}

	return nil
}

// Stop 停止指标收集
func (m *Metrics) Stop() {
	if !m.config.Enable {
		return
	}

	logger.Info("停止指标收集")

	m.prometheus.Stop()
	m.systemMetrics.Stop()
}

// RecordBalance 记录账户余额
func (m *Metrics) RecordBalance(balance float64) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.balance = balance
	m.lastUpdateTime = time.Now()
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
}

// RecordDailyLoss 记录每日亏损
func (m *Metrics) RecordDailyLoss(dailyLoss float64) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.dailyLoss = dailyLoss
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

// GetPrometheusMetrics 获取Prometheus指标收集器
func (m *Metrics) GetPrometheusMetrics() *PrometheusMetrics {
	return m.prometheus
}

// GetSystemMetrics 获取系统资源监控
func (m *Metrics) GetSystemMetrics() *SystemMetrics {
	return m.systemMetrics
}

// GetAPIMetrics 获取API性能监控
func (m *Metrics) GetAPIMetrics() *APIMetrics {
	return m.apiMetrics
}

// GetStrategyMetrics 获取策略性能监控
func (m *Metrics) GetStrategyMetrics() *StrategyMetrics {
	return m.strategyMetrics
}

// GetTradingMetrics 获取交易性能监控
func (m *Metrics) GetTradingMetrics() *TradingMetrics {
	return m.tradingMetrics
}

// GetAllMetrics 获取所有监控指标
func (m *Metrics) GetAllMetrics() map[string]interface{} {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	return map[string]interface{}{
		"balance":        m.GetBalance(),
		"trade_stats":    m.GetTradeStats(),
		"daily_loss":     m.GetDailyLoss(),
		"system":         m.systemMetrics.GetMetrics(),
		"api":            m.apiMetrics.GetAllStats(),
		"strategy":       m.strategyMetrics.GetAllStats(),
		"trading":        m.tradingMetrics.GetStats(),
		"last_update":    m.lastUpdateTime,
	}
}

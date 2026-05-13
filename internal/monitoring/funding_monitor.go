package monitoring

import (
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/ljwqf/quant/pkg/logger"
	"github.com/ljwqf/quant/pkg/types"
	"go.uber.org/zap"
)

// FundingRateFetcher 获取资金费率的接口
type FundingRateFetcher interface {
	GetFundingRate(instId string) (*types.FundingRate, error)
}

// FundingRateHandler 处理资金费率数据的回调
type FundingRateHandler func(symbol string, rate *types.FundingRate)

// FundingRateMonitor 资金费率监控服务
type FundingRateMonitor struct {
	fetcher       FundingRateFetcher
	symbols       []string
	interval      time.Duration
	handler       FundingRateHandler
	stopCh        chan struct{}
	mutex         sync.RWMutex
	running       bool
	latestRates   map[string]*types.FundingRate
	rateMutex     sync.RWMutex
	alertHighRate float64 // 告警阈值（绝对值）
	alertManager  *AlertManager
}

// FundingRateMonitorConfig 资金费率监控配置
type FundingRateMonitorConfig struct {
	Enable        bool          `yaml:"enable"`
	Symbols       []string      `yaml:"symbols"`
	CheckInterval time.Duration `yaml:"check_interval"`
	AlertRate     float64       `yaml:"alert_rate"` // 触发告警的资金费率（绝对值）
}

// NewFundingRateMonitor 创建资金费率监控服务
func NewFundingRateMonitor(fetcher FundingRateFetcher, cfg *FundingRateMonitorConfig, alertMgr *AlertManager) *FundingRateMonitor {
	interval := cfg.CheckInterval
	if interval <= 0 {
		interval = 60 * time.Second
	}
	alertRate := cfg.AlertRate
	if alertRate <= 0 {
		alertRate = 0.001 // 默认 0.1%
	}
	return &FundingRateMonitor{
		fetcher:       fetcher,
		symbols:       cfg.Symbols,
		interval:      interval,
		stopCh:        make(chan struct{}),
		latestRates:   make(map[string]*types.FundingRate),
		alertHighRate: alertRate,
		alertManager:  alertMgr,
	}
}

// SetHandler 设置资金费率数据回调
func (m *FundingRateMonitor) SetHandler(h FundingRateHandler) {
	m.handler = h
}

// Start 启动监控
func (m *FundingRateMonitor) Start() error {
	if len(m.symbols) == 0 {
		return nil
	}
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if m.running {
		return nil
	}
	m.running = true
	logger.Info("启动资金费率监控",
		zap.Strings("symbols", m.symbols),
		zap.Duration("interval", m.interval),
	)
	go m.runLoop()
	return nil
}

// Stop 停止监控
func (m *FundingRateMonitor) Stop() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if !m.running {
		return
	}
	m.running = false
	close(m.stopCh)
	logger.Info("资金费率监控已停止")
}

// GetLatestRate 获取指定symbol的最新资金费率
func (m *FundingRateMonitor) GetLatestRate(symbol string) (*types.FundingRate, bool) {
	m.rateMutex.RLock()
	defer m.rateMutex.RUnlock()
	rate, ok := m.latestRates[symbol]
	return rate, ok
}

// GetAllRates 获取所有symbol的最新资金费率
func (m *FundingRateMonitor) GetAllRates() map[string]*types.FundingRate {
	m.rateMutex.RLock()
	defer m.rateMutex.RUnlock()
	result := make(map[string]*types.FundingRate, len(m.latestRates))
	for k, v := range m.latestRates {
		result[k] = v
	}
	return result
}

// IsNearSettlement 判断指定symbol是否临近结算
func (m *FundingRateMonitor) IsNearSettlement(symbol string, window time.Duration) bool {
	m.rateMutex.RLock()
	defer m.rateMutex.RUnlock()
	rate, ok := m.latestRates[symbol]
	if !ok {
		return false
	}
	now := time.Now()
	settlementTime := rate.NextSettlementTime
	if settlementTime.IsZero() {
		return false
	}
	diff := settlementTime.Sub(now)
	return diff > 0 && diff < window
}

func (m *FundingRateMonitor) runLoop() {
	// 立即执行一次
	m.fetchAll()

	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.fetchAll()
		case <-m.stopCh:
			return
		}
	}
}

func (m *FundingRateMonitor) fetchAll() {
	for _, sym := range m.symbols {
		rate, err := m.fetcher.GetFundingRate(sym)
		if err != nil {
			logger.Warn("获取资金费率失败",
				zap.String("symbol", sym),
				zap.Error(err),
			)
			continue
		}

		m.rateMutex.Lock()
		m.latestRates[sym] = rate
		m.rateMutex.Unlock()

		// 调用回调
		if m.handler != nil {
			m.handler(sym, rate)
		}

		// 检查高费率告警
		if math.Abs(rate.FundingRate) > m.alertHighRate {
			direction := "正"
			if rate.FundingRate < 0 {
				direction = "负"
			}
			msg := fmt.Sprintf("%s 资金费率异常: %s %.4f%%",
				sym, direction, rate.FundingRate*100)
			if m.alertManager != nil {
				m.alertManager.Alert(AlertTypeWarning, "资金费率异常", msg)
			} else {
				logger.Warn(msg)
			}
		}

		// 检查结算窗口
		m.logSettlementInfo(sym, rate)
	}
}

func (m *FundingRateMonitor) logSettlementInfo(symbol string, rate *types.FundingRate) {
	if rate.NextSettlementTime.IsZero() {
		return
	}
	now := time.Now()
	diff := rate.NextSettlementTime.Sub(now)
	if diff < 0 {
		return // 已过期
	}

	if diff < 30*time.Minute {
		logger.Info("资金费结算临近",
			zap.String("symbol", symbol),
			zap.Duration("time_until_settlement", diff),
			zap.Float64("current_rate", rate.FundingRate),
			zap.Float64("next_rate", rate.NextFundingRate),
		)
	} else if diff < 4*time.Hour {
		logger.Debug("资金费结算预告",
			zap.String("symbol", symbol),
			zap.Duration("time_until_settlement", diff),
		)
	}
}

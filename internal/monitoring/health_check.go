package monitoring

import (
	"encoding/json"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/ljwqf/quant/internal/exchange"
	"github.com/ljwqf/quant/internal/risk"
	"github.com/ljwqf/quant/internal/strategy"
	"github.com/ljwqf/quant/pkg/logger"
	"go.uber.org/zap"
)

// HealthStatus 健康状态
type HealthStatus struct {
	Status      string                     `json:"status"`      // healthy, unhealthy, degraded
	Timestamp   time.Time                  `json:"timestamp"`   // 检查时间
	Connection  bool                       `json:"connection"`  // 交易所连接状态
	Strategies  map[string]StrategyStatus  `json:"strategies"`  // 策略状态
	Risk        RiskStatus                 `json:"risk"`        // 风控状态
	Uptime      string                     `json:"uptime"`      // 运行时间
	StartTime   time.Time                  `json:"start_time"`  // 启动时间
}

// StrategyStatus 策略状态
type StrategyStatus struct {
	Name     string                 `json:"name"`     // 策略名称
	Active   bool                   `json:"active"`   // 是否活跃
	Metrics  map[string]interface{} `json:"metrics"`  // 策略指标
	Error    string                 `json:"error"`    // 错误信息（如果有）
}

// RiskStatus 风控状态
type RiskStatus struct {
	Enabled        bool    `json:"enabled"`         // 是否启用
	DailyLoss      float64 `json:"daily_loss"`      // 日亏损
	MaxDailyLoss   float64 `json:"max_daily_loss"`  // 最大日亏损
	DailyTrades    int     `json:"daily_trades"`    // 日交易次数
	MaxTrades      int     `json:"max_trades"`      // 最大交易次数
	CircuitBreaker bool    `json:"circuit_breaker"` // 熔断器状态
}

// HealthChecker 健康检查器
type HealthChecker struct {
	exchange      exchange.Exchange
	riskEngine    *risk.Engine
	strategyEngine *strategy.Engine
	startTime     time.Time
	mutex         sync.RWMutex
}

// NewHealthChecker 创建健康检查器
func NewHealthChecker(
	ex exchange.Exchange,
	re *risk.Engine,
	se *strategy.Engine,
) *HealthChecker {
	return &HealthChecker{
		exchange:       ex,
		riskEngine:     re,
		strategyEngine: se,
		startTime:      time.Now(),
	}
}

// GetStatus 获取健康状态
func (h *HealthChecker) GetStatus() *HealthStatus {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	status := &HealthStatus{
		Status:    "healthy",
		Timestamp: time.Now(),
		StartTime: h.startTime,
		Uptime:    time.Since(h.startTime).String(),
	}

	// 检查交易所连接
	// 注意：Exchange 接口没有 IsConnected 方法，这里假设连接正常
	// 实际使用时需要根据具体实现添加连接状态检查
	if h.exchange != nil {
		status.Connection = true // 暂时设为 true
		// 实际应该调用 h.exchange.IsConnected() 如果接口有的话
	}

	// 检查策略状态
	status.Strategies = make(map[string]StrategyStatus)
	if h.strategyEngine != nil {
		for name, s := range h.strategyEngine.GetStrategies() {
			metrics := s.GetMetrics()
			stratStatus := StrategyStatus{
				Name:    name,
				Active:  true,
				Metrics: metrics,
			}
			status.Strategies[name] = stratStatus
		}
	}

	// 检查风控状态
	if h.riskEngine != nil {
		metrics := h.riskEngine.GetRiskMetrics()
		status.Risk = RiskStatus{
			Enabled:      true,
			DailyTrades:  getIntMetric(metrics, "daily_trades"),
			DailyLoss:    getFloatMetric(metrics, "daily_loss"),
			MaxDailyLoss: getFloatMetric(metrics, "max_daily_loss"),
			MaxTrades:    getIntMetric(metrics, "max_trades_per_day"),
		}

		// 检查是否触发风控
		if status.Risk.DailyLoss >= status.Risk.MaxDailyLoss*0.8 {
			if status.Status == "healthy" {
				status.Status = "degraded"
			}
		}
		if status.Risk.DailyLoss >= status.Risk.MaxDailyLoss {
			status.Status = "unhealthy"
		}
	}

	return status
}

// GetStatusJSON 获取 JSON 格式的健康状态
func (h *HealthChecker) GetStatusJSON() ([]byte, error) {
	status := h.GetStatus()
	return json.MarshalIndent(status, "", "  ")
}

// HTTPHandler HTTP 健康检查处理器
func (h *HealthChecker) HTTPHandler(w http.ResponseWriter, r *http.Request) {
	status, err := h.GetStatusJSON()
	if err != nil {
		http.Error(w, "Failed to get health status", http.StatusInternalServerError)
		logger.Error("获取健康状态失败", zap.Error(err))
		return
	}

	healthStatus := h.GetStatus()
	if healthStatus.Status == "unhealthy" {
		w.WriteHeader(http.StatusServiceUnavailable)
	} else if healthStatus.Status == "degraded" {
		w.WriteHeader(http.StatusMultiStatus)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(status)
}

// StartHealthServer 启动健康检查 HTTP 服务器
func (h *HealthChecker) StartHealthServer(port int) {
	http.HandleFunc("/health", h.HTTPHandler)
	http.HandleFunc("/healthz", h.HTTPHandler) // Kubernetes 风格

	addr := ":" + strconv.Itoa(port)
	logger.Info("启动健康检查 HTTP 服务器", zap.String("address", addr))

	go func() {
		if err := http.ListenAndServe(addr, nil); err != nil {
			logger.Error("健康检查服务器失败", zap.Error(err))
		}
	}()
}

// GetStrategyHealth 获取单个策略的健康状态
func (h *HealthChecker) GetStrategyHealth(name string) *StrategyStatus {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	if h.strategyEngine == nil {
		return nil
	}

	s := h.strategyEngine.GetStrategy(name)
	if s == nil {
		return nil
	}

	return &StrategyStatus{
		Name:    name,
		Active:  true,
		Metrics: s.GetMetrics(),
	}
}

// CheckAllStrategies 检查所有策略
func (h *HealthChecker) CheckAllStrategies() map[string]bool {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	result := make(map[string]bool)
	if h.strategyEngine == nil {
		return result
	}

	for name, s := range h.strategyEngine.GetStrategies() {
		metrics := s.GetMetrics()
		// 简单检查：如果策略有 metrics，认为健康
		result[name] = metrics != nil
	}

	return result
}

// 辅助函数
func getIntMetric(metrics map[string]interface{}, key string) int {
	if v, ok := metrics[key]; ok {
		switch val := v.(type) {
		case int:
			return val
		case float64:
			return int(val)
		}
	}
	return 0
}

func getFloatMetric(metrics map[string]interface{}, key string) float64 {
	if v, ok := metrics[key]; ok {
		switch val := v.(type) {
		case float64:
			return val
		case int:
			return float64(val)
		}
	}
	return 0.0
}

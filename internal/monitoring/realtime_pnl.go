package monitoring

import (
	"fmt"
	"time"

	"github.com/ljwqf/quant/internal/exchange"
	"github.com/ljwqf/quant/internal/execution"
	"github.com/ljwqf/quant/internal/risk"
	"github.com/ljwqf/quant/internal/strategy"
	"github.com/ljwqf/quant/pkg/logger"
	"go.uber.org/zap"
)

// RealTimePnL 实时P&L监控
type RealTimePnL struct {
	exchange               exchange.Exchange
	riskEngine             *risk.Engine
	executionEngine        *execution.Engine
	strategyEngine         *strategy.Engine
	metrics                *Metrics
	alertManager           *AlertManager
	lastPnL                float64
	lastUpdateTime         time.Time
	pnlHistory             []PnLPoint
	consecutiveErrors      int
	isSimulationMode       bool
	simulationRequestCount int
	maxSimulationRequests  int
	circuitBreakerActive   bool          // 熔断器：网络故障时暂停请求
	circuitBreakerUntil    time.Time     // 熔断截止时间
}

// PnLPoint P&L数据点
type PnLPoint struct {
	Timestamp time.Time `json:"timestamp"`
	PnL       float64   `json:"pnl"`
	Equity    float64   `json:"equity"`
}

// NewRealTimePnL 创建实时P&L监控
func NewRealTimePnL(ex exchange.Exchange, riskEngine *risk.Engine, executionEngine *execution.Engine, strategyEngine *strategy.Engine, metrics *Metrics, alertManager *AlertManager, isSimulationMode bool) *RealTimePnL {
	return &RealTimePnL{
		exchange:               ex,
		riskEngine:             riskEngine,
		executionEngine:        executionEngine,
		strategyEngine:         strategyEngine,
		metrics:                metrics,
		alertManager:           alertManager,
		lastPnL:                0,
		lastUpdateTime:         time.Now(),
		pnlHistory:             make([]PnLPoint, 0),
		consecutiveErrors:      0,
		isSimulationMode:       isSimulationMode,
		simulationRequestCount: 0,
		maxSimulationRequests:  5,
	}
}

// Start 启动实时P&L监控
func (r *RealTimePnL) Start() error {
	logger.Info("启动实时P&L监控")

	// 启动监控循环
	go r.monitorLoop()

	return nil
}

// Stop 停止实时P&L监控
func (r *RealTimePnL) Stop() {
	logger.Info("停止实时P&L监控")
}

// GetPnL 获取当前P&L
func (r *RealTimePnL) GetPnL() float64 {
	return r.lastPnL
}

// GetPnLHistory 获取P&L历史
func (r *RealTimePnL) GetPnLHistory() []PnLPoint {
	return r.pnlHistory
}

// monitorLoop 监控循环
func (r *RealTimePnL) monitorLoop() {
	baseInterval := 5 * time.Second
	ticker := time.NewTicker(baseInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// 熔断器检查：网络故障时暂停请求，避免无限重试
			if r.circuitBreakerActive && time.Now().Before(r.circuitBreakerUntil) {
				continue
			}
			if r.circuitBreakerActive && time.Now().After(r.circuitBreakerUntil) {
				r.circuitBreakerActive = false
				logger.Info("P&L监控熔断器已解除")
			}

			// 如果是模拟模式或者连续错误较少，才尝试更新
			if r.isSimulationMode || r.consecutiveErrors < 10 {
				r.updatePnL()
				r.checkSystemStatus()
				r.checkStrategyHealth()
			}

			// 根据错误情况调整轮询间隔
			if r.consecutiveErrors > 5 {
				// 错误较多时，减慢轮询频率
				ticker.Stop()
				backoffInterval := time.Duration(min(r.consecutiveErrors*2, 60)) * time.Second
				ticker = time.NewTicker(backoffInterval)
				logger.Info("调整监控轮询间隔",
					zap.Int("consecutive_errors", r.consecutiveErrors),
					zap.Duration("interval", backoffInterval),
				)
			} else if r.consecutiveErrors == 0 {
				// 无错误时，恢复正常间隔
				ticker.Stop()
				ticker = time.NewTicker(baseInterval)
			}
		}
	}
}

// min 返回两个整数中的较小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// updatePnL 更新P&L
func (r *RealTimePnL) updatePnL() {
	// 如果是模拟模式且已超过最大请求次数，不再请求
	if r.isSimulationMode && r.simulationRequestCount >= r.maxSimulationRequests {
		return
	}

	// 获取账户信息
	account, err := r.exchange.GetAccount()
	if err != nil {
		r.consecutiveErrors++
		if r.isSimulationMode {
			r.simulationRequestCount++
			logger.Warn("模拟模式API请求失败",
				zap.Error(err),
				zap.Int("request_count", r.simulationRequestCount),
				zap.Int("max_requests", r.maxSimulationRequests),
			)
		} else {
			// 仅在前几次错误时打印完整日志，避免刷屏
			if r.consecutiveErrors <= 3 {
				logger.Error("获取账户信息失败",
					zap.Error(err),
					zap.Int("consecutive_errors", r.consecutiveErrors),
				)
			} else if r.consecutiveErrors%20 == 0 {
				// 每20次错误打印一次摘要
				logger.Warn("获取账户信息持续失败",
					zap.Error(err),
					zap.Int("consecutive_errors", r.consecutiveErrors),
					zap.Bool("circuit_breaker_active", r.circuitBreakerActive),
				)
			}
		}

		// 连续错误达到阈值时激活熔断器
		if r.consecutiveErrors >= 20 && !r.circuitBreakerActive {
			// 指数退避：初始5分钟，之后逐步增加
			backoffDuration := 5 * time.Minute
			r.circuitBreakerUntil = time.Now().Add(backoffDuration)
			r.circuitBreakerActive = true
			logger.Warn("P&L监控触发熔断，暂停请求",
				zap.Int("consecutive_errors", r.consecutiveErrors),
				zap.Duration("backoff", backoffDuration),
			)
		}
		return
	}

	// 成功，重置错误计数和熔断器
	r.consecutiveErrors = 0
	if r.circuitBreakerActive {
		r.circuitBreakerActive = false
		logger.Info("P&L监控熔断器已解除")
	}
	if r.isSimulationMode {
		r.simulationRequestCount++
		logger.Info("模拟模式API请求成功",
			zap.Int("request_count", r.simulationRequestCount),
			zap.Int("max_requests", r.maxSimulationRequests),
		)
	}

	// 计算P&L
	pnl := account.TotalPnL

	// 记录P&L历史
	r.pnlHistory = append(r.pnlHistory, PnLPoint{
		Timestamp: time.Now(),
		PnL:       pnl,
		Equity:    account.TotalEquity,
	})

	// 限制历史数据长度
	maxHistory := 10080
	if len(r.pnlHistory) > maxHistory {
		r.pnlHistory = r.pnlHistory[len(r.pnlHistory)-maxHistory:]
	}

	// 记录上次P&L
	r.lastPnL = pnl
	r.lastUpdateTime = time.Now()

	// 记录指标
	r.metrics.RecordBalance(account.TotalEquity)

	// 检查P&L变化
	r.checkPnLChange(pnl)

	logger.Info("更新实时P&L",
		zap.Float64("pnl", pnl),
		zap.Float64("equity", account.TotalEquity),
	)
}

// checkPnLChange 检查P&L变化
func (r *RealTimePnL) checkPnLChange(pnl float64) {
	// 检查是否有显著变化
	pnlChange := pnl - r.lastPnL
	if pnlChange > 1000 {
		r.alertManager.Alert(AlertTypeInfo, "P&L显著增加", fmt.Sprintf("P&L增加了 %.2f", pnlChange))
	} else if pnlChange < -1000 {
		r.alertManager.Alert(AlertTypeWarning, "P&L显著减少", fmt.Sprintf("P&L减少了 %.2f", -pnlChange))
	}
}

// checkSystemStatus 检查系统状态
func (r *RealTimePnL) checkSystemStatus() {
	// 检查交易所连接
	// 暂时注释掉，因为exchange.Exchange接口中没有IsConnected方法
	// if !r.exchange.IsConnected() {
	// 	r.alertManager.Alert(AlertTypeError, "交易所连接断开", "请检查网络连接和API密钥")
	// }

	// 检查订单状态
	r.executionEngine.MonitorOrders()

	// 检查风险指标
	// 风险指标记录可以通过其他方式实现

	// 检查每日亏损
	dailyLoss := r.riskEngine.GetDailyLoss()
	if dailyLoss > 0 {
		r.metrics.RecordDailyLoss(dailyLoss)
	}
}

// checkStrategyHealth 检查策略健康度
func (r *RealTimePnL) checkStrategyHealth() {
	// 获取所有策略
	strategies := r.strategyEngine.GetStrategies()

	for name, strategy := range strategies {
		// 获取策略指标
		metrics := strategy.GetMetrics()

		// 检查策略信号数
		totalSignals, ok := metrics["total_signals"].(int)
		if ok && totalSignals > 0 {
			// 检查胜率
			winRate, ok := metrics["win_rate"].(float64)
			if ok && winRate < 0.3 {
				r.alertManager.Alert(AlertTypeWarning, "策略胜率过低", fmt.Sprintf("策略 %s 的胜率为 %.2f%%", name, winRate*100))
			}
		}
	}
}

// GetSystemStatus 获取系统状态
func (r *RealTimePnL) GetSystemStatus() map[string]interface{} {
	status := make(map[string]interface{})

	// 获取账户信息
	account, err := r.exchange.GetAccount()
	if err == nil {
		status["account"] = account
	}

	// 获取风险指标
	status["risk"] = r.riskEngine.GetRiskMetrics()

	// 获取执行指标
	status["execution"] = r.executionEngine.GetMetrics()

	// 获取策略指标
	status["strategies"] = r.strategyEngine.GetAllStrategyMetrics()

	// 获取P&L
	status["pnl"] = r.lastPnL

	// 获取系统时间
	status["timestamp"] = time.Now()

	return status
}

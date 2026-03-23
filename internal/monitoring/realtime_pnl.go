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
	exchange      exchange.Exchange
	riskEngine    *risk.Engine
	executionEngine *execution.Engine
	strategyEngine *strategy.Engine
	metrics        *Metrics
	alertManager   *AlertManager
	lastPnL        float64 // 上次P&L
	lastUpdateTime time.Time // 上次更新时间
	pnlHistory     []PnLPoint // P&L历史
}

// PnLPoint P&L数据点
type PnLPoint struct {
	Timestamp time.Time `json:"timestamp"`
	PnL       float64   `json:"pnl"`
	Equity    float64   `json:"equity"`
}

// NewRealTimePnL 创建实时P&L监控
func NewRealTimePnL(ex exchange.Exchange, riskEngine *risk.Engine, executionEngine *execution.Engine, strategyEngine *strategy.Engine, metrics *Metrics, alertManager *AlertManager) *RealTimePnL {
	return &RealTimePnL{
		exchange:      ex,
		riskEngine:    riskEngine,
		executionEngine: executionEngine,
		strategyEngine: strategyEngine,
		metrics:        metrics,
		alertManager:   alertManager,
		lastPnL:        0,
		lastUpdateTime: time.Now(),
		pnlHistory:     make([]PnLPoint, 0),
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
	ticker := time.NewTicker(5 * time.Second) // 每5秒更新一次
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			r.updatePnL()
			r.checkSystemStatus()
			r.checkStrategyHealth()
		}
	}
}

// updatePnL 更新P&L
func (r *RealTimePnL) updatePnL() {
	// 获取账户信息
	account, err := r.exchange.GetAccount()
	if err != nil {
		logger.Error("获取账户信息失败",
			zap.Error(err),
		)
		return
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
	maxHistory := 10080 // 7天，每5秒一个数据点
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
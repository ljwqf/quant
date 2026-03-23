package monitoring

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ljwqf/quant/pkg/logger"
	"github.com/ljwqf/quant/pkg/types"
	"go.uber.org/zap"
)

// Monitor 监控管理器
type Monitor struct {
	config       *Config
	exchange     Exchange
	riskManager  RiskManager
	metrics      *Metrics
	alertManager *AlertManager
	tradeHistory []TradeRecord
	positions    map[string]*types.Position
	ctx          context.Context
	cancel       context.CancelFunc
	stopCh       chan struct{}
	mutex        sync.RWMutex
}

// Config 监控配置
type Config struct {
	Enable         bool           `yaml:"enable"`
	CheckInterval  time.Duration  `yaml:"check_interval"`
	AlertThreshold AlertThreshold `yaml:"alert_threshold"`
	MetricsConfig  MetricsConfig  `yaml:"metrics"`
	AlertConfig    AlertConfig    `yaml:"alert"`
}

// AlertThreshold 告警阈值
type AlertThreshold struct {
	MaxDrawdown   float64       `yaml:"max_drawdown"`
	MaxLoss       float64       `yaml:"max_loss"`
	PositionLimit float64       `yaml:"position_limit"`
	OrderTimeout  time.Duration `yaml:"order_timeout"`
}

// MetricsConfig 指标配置
type MetricsConfig struct {
	Enable         bool   `yaml:"enable"`
	PushGatewayURL string `yaml:"push_gateway_url"`
}

// AlertConfig 告警配置
type AlertConfig struct {
	Enable      bool          `yaml:"enable"`
	Channels    []string      `yaml:"channels"`
	WebhookURL  string        `yaml:"webhook_url"`
	DedupWindow time.Duration `yaml:"dedup_window"`
	MinInterval time.Duration `yaml:"min_interval"`
}

// TradeRecord 交易记录
type TradeRecord struct {
	ID        string          `json:"id"`
	Symbol    string          `json:"symbol"`
	Side      types.OrderSide `json:"side"`
	Price     float64         `json:"price"`
	Quantity  float64         `json:"quantity"`
	Status    string          `json:"status"`
	Timestamp time.Time       `json:"timestamp"`
	PnL       float64         `json:"pnl"`
}

// Exchange 交易所接口
type Exchange interface {
	GetPositions() ([]*types.Position, error)
	GetOrderHistory(symbol string, limit int) ([]*types.Order, error)
	GetAccountInfo() (*types.Account, error)
}

// RiskManager 风险管理接口
type RiskManager interface {
	CheckRisk(signal *types.Signal) error
	GetDailyLoss() float64
}

// NewMonitor 创建监控管理器
func NewMonitor(config *Config, exchange Exchange, riskManager RiskManager) *Monitor {
	ctx, cancel := context.WithCancel(context.Background())
	return &Monitor{
		config:       config,
		exchange:     exchange,
		riskManager:  riskManager,
		metrics:      NewMetrics(&config.MetricsConfig),
		alertManager: NewAlertManager(&config.AlertConfig),
		positions:    make(map[string]*types.Position),
		tradeHistory: make([]TradeRecord, 0),
		ctx:          ctx,
		cancel:       cancel,
		stopCh:       make(chan struct{}),
	}
}

// Start 启动监控
func (m *Monitor) Start() error {
	if !m.config.Enable {
		return nil
	}

	logger.Info("启动监控系统")

	// 启动指标收集
	if err := m.metrics.Start(); err != nil {
		logger.Error("启动指标收集失败", zap.Error(err))
	}

	// 启动告警管理器
	if err := m.alertManager.Start(); err != nil {
		logger.Error("启动告警管理器失败", zap.Error(err))
	}

	// 启动监控循环
	go m.monitorLoop()

	return nil
}

// Stop 停止监控
func (m *Monitor) Stop() {
	if !m.config.Enable {
		return
	}

	logger.Info("停止监控系统")

	// 发送停止信号
	close(m.stopCh)

	// 取消上下文
	m.cancel()

	// 停止指标收集
	m.metrics.Stop()

	// 停止告警管理器
	m.alertManager.Stop()
}

// monitorLoop 监控循环
func (m *Monitor) monitorLoop() {
	ticker := time.NewTicker(m.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.checkStatus()
		case <-m.stopCh:
			return
		case <-m.ctx.Done():
			return
		}
	}
}

// checkStatus 检查系统状态
func (m *Monitor) checkStatus() {
	// 检查账户信息
	m.checkAccount()

	// 检查持仓状态
	m.checkPositions()

	// 检查交易历史
	m.checkTrades()

	// 检查风险指标
	m.checkRisk()

	// 检查系统健康状态
	m.checkSystem()
}

// checkAccount 检查账户信息
func (m *Monitor) checkAccount() {
	account, err := m.exchange.GetAccountInfo()
	if err != nil {
		logger.Error("获取账户信息失败", zap.Error(err))
		m.alertManager.Alert(AlertTypeError, "获取账户信息失败", fmt.Sprintf("错误: %v", err))
		return
	}

	// 记录账户总权益
	m.metrics.RecordBalance(account.TotalEquity)

	// 检查账户总权益是否异常
	if account.TotalEquity < 0 {
		m.alertManager.Alert(AlertTypeCritical, "账户权益异常", fmt.Sprintf("账户总权益为负数: %.2f", account.TotalEquity))
	}
}

// checkPositions 检查持仓状态
func (m *Monitor) checkPositions() {
	positions, err := m.exchange.GetPositions()
	if err != nil {
		logger.Error("获取持仓信息失败", zap.Error(err))
		m.alertManager.Alert(AlertTypeError, "获取持仓信息失败", fmt.Sprintf("错误: %v", err))
		return
	}

	// 更新持仓信息
	m.mutex.Lock()
	m.positions = make(map[string]*types.Position)
	for _, pos := range positions {
		m.positions[pos.Symbol] = pos
		// 记录持仓信息
		m.metrics.RecordPosition(pos.Symbol, pos.Size, pos.MarkPrice)
	}
	m.mutex.Unlock()

	// 检查持仓是否超过限制
	for _, pos := range positions {
		if pos.Size > m.config.AlertThreshold.PositionLimit {
			m.alertManager.Alert(AlertTypeWarning, "持仓超过限制", fmt.Sprintf("符号: %s, 大小: %.2f, 限制: %.2f", pos.Symbol, pos.Size, m.config.AlertThreshold.PositionLimit))
		}
	}
}

// checkTrades 检查交易历史
func (m *Monitor) checkTrades() {
	// 这里可以实现检查交易历史的逻辑
	// 例如检查订单是否超时、是否有异常交易等
}

// checkRisk 检查风险指标
func (m *Monitor) checkRisk() {
	// 检查每日亏损
	dailyLoss := m.riskManager.GetDailyLoss()
	if dailyLoss > m.config.AlertThreshold.MaxLoss {
		m.alertManager.Alert(AlertTypeCritical, "每日亏损超过限制", fmt.Sprintf("每日亏损: %.2f, 限制: %.2f", dailyLoss, m.config.AlertThreshold.MaxLoss))
	}

	// 记录风险指标
	m.metrics.RecordDailyLoss(dailyLoss)
}

// checkSystem 检查系统健康状态
func (m *Monitor) checkSystem() {
	// 这里可以实现检查系统健康状态的逻辑
	// 例如检查网络连接、API响应时间等
}

// OnTrade 处理交易事件
func (m *Monitor) OnTrade(trade *types.Trade) {
	// 计算PnL
	pnl := 0.0

	// 尝试从持仓中获取PnL
	m.mutex.RLock()
	position, exists := m.positions[trade.Symbol]
	m.mutex.RUnlock()

	if exists && position != nil {
		// 计算简单的PnL
		if trade.Side == types.OrderSideBuy {
			// 买入时，PnL为负（成本增加）
			pnl = -trade.Price * trade.Quantity
		} else {
			// 卖出时，PnL为正（收益）
			pnl = trade.Price * trade.Quantity
		}
	}

	record := TradeRecord{
		ID:        trade.ID,
		Symbol:    trade.Symbol,
		Side:      trade.Side,
		Price:     trade.Price,
		Quantity:  trade.Quantity,
		Status:    "filled",
		Timestamp: time.Now(),
		PnL:       pnl,
	}

	m.mutex.Lock()
	m.tradeHistory = append(m.tradeHistory, record)
	// 只保留最近1000条交易记录
	if len(m.tradeHistory) > 1000 {
		m.tradeHistory = m.tradeHistory[len(m.tradeHistory)-1000:]
	}
	m.mutex.Unlock()

	// 记录交易指标
	m.metrics.RecordTrade(trade.Symbol, string(trade.Side), trade.Price, trade.Quantity, pnl)

	// 这里可以添加更复杂的交易盈亏计算逻辑
	// 例如通过比较当前交易价格和持仓均价来计算
}

// OnOrder 处理订单事件
func (m *Monitor) OnOrder(order *types.Order) {
	// 这里可以实现处理订单事件的逻辑
	// 例如检查订单状态、订单超时等
}

// GetPositions 获取当前持仓
func (m *Monitor) GetPositions() map[string]*types.Position {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	// 创建副本
	positions := make(map[string]*types.Position)
	for k, v := range m.positions {
		positions[k] = v
	}

	return positions
}

// GetTradeHistory 获取交易历史
func (m *Monitor) GetTradeHistory() []TradeRecord {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	// 创建副本
	records := make([]TradeRecord, len(m.tradeHistory))
	copy(records, m.tradeHistory)

	return records
}

// GetMetrics 获取指标
func (m *Monitor) GetMetrics() *Metrics {
	return m.metrics
}

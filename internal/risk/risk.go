package risk

import (
	"math"
	"sync"
	"time"

	"github.com/ljwqf/quant/internal/config"
	"github.com/ljwqf/quant/internal/exchange"
	"github.com/ljwqf/quant/pkg/logger"
	"github.com/ljwqf/quant/pkg/types"
	"go.uber.org/zap"
)

// Manager 风险管理管理器
type Manager struct {
	config      *config.RiskConfig
	exchange    exchange.Exchange
	dailyLoss   float64
	maxDrawdown float64
	peakEquity  float64 // 历史最高权益
	dailyTrades int     // 每日交易次数
	positions   map[string]*types.Position
	mutex       sync.RWMutex
	dailyReset  time.Time
}

// NewManager 创建风险管理管理器
func NewManager(cfg *config.RiskConfig, ex exchange.Exchange) *Manager {
	return &Manager{
		config:      cfg,
		exchange:    ex,
		positions:   make(map[string]*types.Position),
		dailyReset:  time.Now(),
		peakEquity:  0, // 初始化为0，后续会更新
		dailyTrades: 0, // 初始化为0，每日重置
	}
}

// CheckOrder 检查订单是否符合风险控制要求
func (m *Manager) CheckOrder(order *types.Order) error {
	if order == nil {
		return ErrInvalidOrder
	}

	// 检查仓位限制
	if err := m.checkPositionLimit(order); err != nil {
		return err
	}

	// 检查日亏损限制
	if err := m.checkDailyLoss(); err != nil {
		return err
	}

	// 检查交易次数限制
	if err := m.checkTradeLimit(); err != nil {
		return err
	}

	// 检查最大回撤
	if err := m.checkMaxDrawdown(); err != nil {
		return err
	}

	return nil
}

// checkPositionLimit 检查仓位限制
func (m *Manager) checkPositionLimit(order *types.Order) error {
	if order == nil || order.Symbol == "" || order.Quantity <= 0 {
		return ErrInvalidOrder
	}

	orderValue, err := m.resolveOrderValue(order)
	if err != nil {
		return err
	}

	m.mutex.RLock()
	defer m.mutex.RUnlock()

	// 计算当前仓位价值
	currentValue := 0.0
	currentSymbolValue := 0.0
	var currentSymbolSide types.OrderSide
	for _, pos := range m.positions {
		positionValue := absExposure(pos.Size, pos.MarkPrice, pos.Leverage)
		currentValue += positionValue
		if pos.Symbol == order.Symbol {
			currentSymbolValue += positionValue
			currentSymbolSide = pos.Side
		}
	}

	additionalValue := orderValue
	if currentSymbolValue > 0 && currentSymbolSide != "" && currentSymbolSide != order.Side {
		additionalValue -= currentSymbolValue
		if additionalValue < 0 {
			additionalValue = 0
		}
	}

	// 检查是否超过最大仓位价值
	if currentValue+additionalValue > m.config.MaxPositionSize {
		return ErrPositionLimitExceeded
	}

	return nil
}

// checkDailyLoss 检查日亏损限制
func (m *Manager) checkDailyLoss() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.checkDailyResetLocked()

	if m.dailyLoss >= m.config.MaxDailyLoss {
		return ErrDailyLossExceeded
	}

	return nil
}

// checkTradeLimit 检查交易次数限制
func (m *Manager) checkTradeLimit() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.checkDailyResetLocked()

	if m.dailyTrades >= m.config.MaxTradesPerDay {
		return ErrTradeLimitExceeded
	}

	return nil
}

// checkMaxDrawdown 检查最大回撤
func (m *Manager) checkMaxDrawdown() error {
	if m.exchange == nil {
		return nil
	}

	// 获取当前账户权益
	account, err := m.exchange.GetAccount()
	if err != nil {
		return err
	}

	currentEquity := account.TotalEquity

	m.mutex.Lock()
	defer m.mutex.Unlock()

	// 更新历史最高权益
	if currentEquity > m.peakEquity {
		m.peakEquity = currentEquity
	}

	// 计算当前回撤
	var currentDrawdown float64
	if m.peakEquity > 0 {
		currentDrawdown = (m.peakEquity - currentEquity) / m.peakEquity
	}

	// 更新最大回撤
	if currentDrawdown > m.maxDrawdown {
		m.maxDrawdown = currentDrawdown
	}

	// 检查是否超过配置的最大回撤限制
	if currentDrawdown > m.config.MaxDrawdown {
		return ErrMaxDrawdownExceeded
	}

	return nil
}

// UpdatePosition 更新仓位信息
func (m *Manager) UpdatePosition(position *types.Position) {
	if position == nil {
		return
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	if position.Size == 0 {
		delete(m.positions, position.Symbol)
		return
	}

	m.positions[position.Symbol] = position
}

// UpdateDailyLoss 更新日亏损
func (m *Manager) UpdateDailyLoss(loss float64) {
	if loss == 0 {
		return
	}
	if loss < 0 {
		loss = -loss
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.checkDailyResetLocked()
	m.dailyLoss += loss
}

// IncrementDailyTrades 增加每日交易次数
func (m *Manager) IncrementDailyTrades() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.checkDailyResetLocked()

	m.dailyTrades++
}

// CheckStopLoss 检查止损
func (m *Manager) CheckStopLoss(position *types.Position) (bool, error) {
	if position == nil {
		return false, nil
	}

	// 计算当前盈亏百分比
	if position.EntryPrice == 0 {
		return false, nil
	}

	var pnlPercent float64
	if position.Side == types.OrderSideBuy {
		pnlPercent = (position.MarkPrice - position.EntryPrice) / position.EntryPrice
	} else {
		pnlPercent = (position.EntryPrice - position.MarkPrice) / position.EntryPrice
	}

	// 检查是否达到止损
	if pnlPercent <= -m.config.StopLossPercent {
		logger.Info("触发止损",
			zap.String("symbol", position.Symbol),
			zap.Float64("entry_price", position.EntryPrice),
			zap.Float64("mark_price", position.MarkPrice),
			zap.Float64("pnl_percent", pnlPercent),
			zap.Float64("stop_loss", m.config.StopLossPercent),
		)
		return true, nil
	}

	return false, nil
}

// CheckTakeProfit 检查止盈
func (m *Manager) CheckTakeProfit(position *types.Position) (bool, error) {
	if position == nil {
		return false, nil
	}

	// 计算当前盈亏百分比
	if position.EntryPrice == 0 {
		return false, nil
	}

	var pnlPercent float64
	if position.Side == types.OrderSideBuy {
		pnlPercent = (position.MarkPrice - position.EntryPrice) / position.EntryPrice
	} else {
		pnlPercent = (position.EntryPrice - position.MarkPrice) / position.EntryPrice
	}

	// 检查是否达到止盈
	if pnlPercent >= m.config.TakeProfitPercent {
		logger.Info("触发止盈",
			zap.String("symbol", position.Symbol),
			zap.Float64("entry_price", position.EntryPrice),
			zap.Float64("mark_price", position.MarkPrice),
			zap.Float64("pnl_percent", pnlPercent),
			zap.Float64("take_profit", m.config.TakeProfitPercent),
		)
		return true, nil
	}

	return false, nil
}

// GetRiskMetrics 获取风险指标
func (m *Manager) GetRiskMetrics() map[string]interface{} {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	metrics := make(map[string]interface{})
	metrics["daily_loss"] = m.dailyLoss
	metrics["max_daily_loss"] = m.config.MaxDailyLoss
	metrics["daily_trades"] = m.dailyTrades
	metrics["max_trades_per_day"] = m.config.MaxTradesPerDay
	metrics["max_drawdown"] = m.maxDrawdown
	metrics["peak_equity"] = m.peakEquity
	metrics["max_position_size"] = m.config.MaxPositionSize
	metrics["stop_loss_percent"] = m.config.StopLossPercent
	metrics["take_profit_percent"] = m.config.TakeProfitPercent
	metrics["position_count"] = len(m.positions)

	totalExposure := 0.0
	for _, pos := range m.positions {
		totalExposure += absExposure(pos.Size, pos.MarkPrice, pos.Leverage)
	}
	metrics["total_exposure"] = totalExposure

	return metrics
}

// CalculateRiskExposure 计算风险敞口
func (m *Manager) CalculateRiskExposure() (float64, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	totalExposure := 0.0
	for _, pos := range m.positions {
		exposure := absExposure(pos.Size, pos.MarkPrice, pos.Leverage)
		totalExposure += exposure
	}

	return totalExposure, nil
}

// CheckRisk 检查风险
func (m *Manager) CheckRisk(signal *types.Signal) error {
	if signal == nil {
		return ErrInvalidSignal
	}

	switch signal.Type {
	case types.SignalTypeExit, types.SignalTypeHold:
		return nil
	case types.SignalTypeBuy, types.SignalTypeSell:
	default:
		return ErrInvalidSignal
	}

	if signal.Symbol == "" || signal.Quantity <= 0 {
		return ErrInvalidSignal
	}

	order := &types.Order{
		Symbol:   signal.Symbol,
		Quantity: signal.Quantity,
		Price:    signal.Price,
		Side:     types.OrderSide(signal.Type),
		Type:     types.OrderTypeLimit,
	}
	if signal.Price <= 0 {
		order.Type = types.OrderTypeMarket
	}

	return m.CheckOrder(order)
}

// GetDailyLoss 获取每日亏损
func (m *Manager) GetDailyLoss() float64 {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	return m.dailyLoss
}

func (m *Manager) checkDailyResetLocked() {
	if time.Since(m.dailyReset) > 24*time.Hour {
		m.dailyLoss = 0
		m.dailyTrades = 0
		m.dailyReset = time.Now()
	}
}

func (m *Manager) resolveOrderValue(order *types.Order) (float64, error) {
	price := order.Price
	if order.Type == types.OrderTypeMarket || price <= 0 {
		if m.exchange == nil {
			return 0, ErrInvalidOrder
		}

		ticker, err := m.exchange.GetTicker(order.Symbol)
		if err != nil {
			return 0, err
		}
		price = ticker.Price
	}

	if price <= 0 {
		return 0, ErrInvalidOrder
	}

	return absExposure(order.Quantity, price, order.Leverage), nil
}

func absExposure(size, price float64, leverage int) float64 {
	multiplier := 1.0
	if leverage > 1 {
		multiplier = float64(leverage)
	}

	return math.Abs(size*price) * multiplier
}

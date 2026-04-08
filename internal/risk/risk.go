package risk

import (
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/ljwqf/quant/internal/config"
	"github.com/ljwqf/quant/internal/exchange"
	"github.com/ljwqf/quant/pkg/logger"
	"github.com/ljwqf/quant/pkg/types"
	"go.uber.org/zap"
)

// RiskRule 风险规则接口
type RiskRule interface {
	Name() string
	Check(ctx *RiskContext) error
	GetConfig() map[string]interface{}
	SetConfig(config map[string]interface{}) error
}

// RiskContext 风险检查上下文
type RiskContext struct {
	Order     *types.Order
	Signal    *types.Signal
	Position  *types.Position
	Account   *types.Account
	Metrics   *RiskMetrics
	Timestamp time.Time
}

// RiskMetrics 风险指标
type RiskMetrics struct {
	DailyLoss     float64
	MaxDrawdown   float64
	PeakEquity    float64
	DailyTrades   int
	TotalExposure float64
	PositionCount int
	LastUpdated   time.Time
}

// AlertLevel 告警级别
type AlertLevel int

const (
	AlertLevelInfo AlertLevel = iota
	AlertLevelWarning
	AlertLevelCritical
	AlertLevelEmergency
)

// Alert 风险告警
type Alert struct {
	ID        string
	Level     AlertLevel
	Message   string
	Rule      string
	Timestamp time.Time
	Metadata  map[string]interface{}
}

// AlertHandler 告警处理函数
type AlertHandler func(alert *Alert) error

// Manager 风险管理管理器
type Manager struct {
	config        *config.RiskConfig
	exchange      exchange.Exchange
	rules         []RiskRule
	alertHandlers []AlertHandler
	dailyLoss     float64
	maxDrawdown   float64
	peakEquity    float64 // 历史最高权益
	dailyTrades   int     // 每日交易次数
	positions     map[string]*types.Position
	mutex         sync.RWMutex
	dailyReset    time.Time
	alerts        []*Alert
}

// NewManager 创建风险管理管理器
func NewManager(cfg *config.RiskConfig, ex exchange.Exchange) *Manager {
	m := &Manager{
		config:        cfg,
		exchange:      ex,
		rules:         make([]RiskRule, 0),
		alertHandlers: make([]AlertHandler, 0),
		positions:     make(map[string]*types.Position),
		dailyReset:    time.Now(),
		peakEquity:    0, // 初始化为0，后续会更新
		dailyTrades:   0, // 初始化为0，每日重置
		alerts:        make([]*Alert, 0, 100),
	}

	// 注册默认风险规则
	m.registerDefaultRules()
	return m
}

// registerDefaultRules 注册默认风险规则
func (m *Manager) registerDefaultRules() {
	// 仓位限制规则
	m.AddRule(&PositionLimitRule{
		manager:         m,
		maxPositionSize: m.config.MaxPositionSize,
	})

	// 日亏损限制规则
	m.AddRule(&DailyLossRule{
		manager:      m,
		maxDailyLoss: m.config.MaxDailyLoss,
	})

	// 交易次数限制规则
	m.AddRule(&TradeLimitRule{
		manager:         m,
		maxTradesPerDay: m.config.MaxTradesPerDay,
	})

	// 最大回撤规则
	m.AddRule(&MaxDrawdownRule{
		manager:     m,
		maxDrawdown: m.config.MaxDrawdown,
	})

	// 单笔交易风险规则（默认0.02 = 2%）
	maxRiskPerTrade := m.config.MaxRiskPerTrade
	if maxRiskPerTrade <= 0 {
		maxRiskPerTrade = 0.02
	}
	m.AddRule(&SingleTradeRiskRule{
		manager:         m,
		maxRiskPerTrade: maxRiskPerTrade,
	})

	// 品种风险敞口规则（默认0.3 = 30%）
	maxExposurePerSymbol := m.config.MaxExposurePerSymbol
	if maxExposurePerSymbol <= 0 {
		maxExposurePerSymbol = 0.3
	}
	m.AddRule(&SymbolExposureRule{
		manager:              m,
		maxExposurePerSymbol: maxExposurePerSymbol,
	})

	logger.Info("默认风险规则已注册",
		zap.Int("rule_count", len(m.rules)),
		zap.Float64("default_max_risk_per_trade", maxRiskPerTrade),
		zap.Float64("default_max_exposure_per_symbol", maxExposurePerSymbol),
	)
}

// AddRule 添加风险规则
func (m *Manager) AddRule(rule RiskRule) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.rules = append(m.rules, rule)
}

// RemoveRule 移除风险规则
func (m *Manager) RemoveRule(ruleName string) bool {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	for i, rule := range m.rules {
		if rule.Name() == ruleName {
			m.rules = append(m.rules[:i], m.rules[i+1:]...)
			return true
		}
	}
	return false
}

// AddAlertHandler 添加告警处理函数
func (m *Manager) AddAlertHandler(handler AlertHandler) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.alertHandlers = append(m.alertHandlers, handler)
}

// TriggerAlert 触发风险告警
func (m *Manager) TriggerAlert(level AlertLevel, message, rule string, metadata map[string]interface{}) {
	alert := &Alert{
		ID:        fmt.Sprintf("alert_%d", time.Now().UnixNano()),
		Level:     level,
		Message:   message,
		Rule:      rule,
		Timestamp: time.Now(),
		Metadata:  metadata,
	}

	m.mutex.Lock()
	m.alerts = append(m.alerts, alert)
	if len(m.alerts) > 1000 {
		m.alerts = m.alerts[len(m.alerts)-1000:]
	}
	handlers := make([]AlertHandler, len(m.alertHandlers))
	copy(handlers, m.alertHandlers)
	m.mutex.Unlock()

	// 异步发送告警
	go func() {
		for _, handler := range handlers {
			if err := handler(alert); err != nil {
				logger.Error("告警处理失败", zap.Error(err))
			}
		}
	}()

	logger.Warn("风险告警触发",
		zap.String("level", alertLevelToString(level)),
		zap.String("message", message),
		zap.String("rule", rule),
		zap.Any("metadata", metadata),
	)
}

// CheckOrder 检查订单是否符合风险控制要求
func (m *Manager) CheckOrder(order *types.Order) error {
	if order == nil {
		return ErrInvalidOrder
	}

	// 创建风险检查上下文
	ctx, err := m.createContext(order, nil, nil)
	if err != nil {
		return err
	}

	// 执行所有风险规则检查
	m.mutex.RLock()
	rules := make([]RiskRule, len(m.rules))
	copy(rules, m.rules)
	m.mutex.RUnlock()

	for _, rule := range rules {
		if err := rule.Check(ctx); err != nil {
			m.TriggerAlert(AlertLevelWarning,
				fmt.Sprintf("订单风险检查失败: %v", err),
				rule.Name(),
				map[string]interface{}{
					"order_id": order.ID,
					"symbol":   order.Symbol,
					"side":     order.Side,
					"quantity": order.Quantity,
					"price":    order.Price,
				},
			)
			return err
		}
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

// createContext 创建风险检查上下文
func (m *Manager) createContext(order *types.Order, signal *types.Signal, position *types.Position) (*RiskContext, error) {
	m.mutex.RLock()
	metrics := &RiskMetrics{
		DailyLoss:     m.dailyLoss,
		MaxDrawdown:   m.maxDrawdown,
		PeakEquity:    m.peakEquity,
		DailyTrades:   m.dailyTrades,
		PositionCount: len(m.positions),
		LastUpdated:   time.Now(),
	}
	m.mutex.RUnlock()

	// 计算总风险敞口
	totalExposure, err := m.CalculateRiskExposure()
	if err != nil {
		return nil, err
	}
	metrics.TotalExposure = totalExposure

	// 获取账户信息
	var account *types.Account
	if m.exchange != nil {
		acc, err := m.exchange.GetAccount()
		if err == nil {
			account = acc
		}
	}

	return &RiskContext{
		Order:     order,
		Signal:    signal,
		Position:  position,
		Account:   account,
		Metrics:   metrics,
		Timestamp: time.Now(),
	}, nil
}

// alertLevelToString 告警级别转字符串
func alertLevelToString(level AlertLevel) string {
	switch level {
	case AlertLevelInfo:
		return "info"
	case AlertLevelWarning:
		return "warning"
	case AlertLevelCritical:
		return "critical"
	case AlertLevelEmergency:
		return "emergency"
	default:
		return "unknown"
	}
}

// GetAlerts 获取历史告警
func (m *Manager) GetAlerts(limit int) []*Alert {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if limit <= 0 || limit > len(m.alerts) {
		limit = len(m.alerts)
	}

	result := make([]*Alert, limit)
	copy(result, m.alerts[len(m.alerts)-limit:])
	return result
}

// ============================== 风险规则实现 ==============================

// PositionLimitRule 仓位限制规则
type PositionLimitRule struct {
	manager         *Manager
	maxPositionSize float64
}

func (r *PositionLimitRule) Name() string { return "position_limit" }

func (r *PositionLimitRule) Check(ctx *RiskContext) error {
	if ctx.Order == nil {
		return nil
	}

	orderValue, err := r.manager.resolveOrderValue(ctx.Order)
	if err != nil {
		return err
	}

	r.manager.mutex.RLock()
	defer r.manager.mutex.RUnlock()

	// 计算当前仓位价值
	currentValue := 0.0
	currentSymbolValue := 0.0
	var currentSymbolSide types.OrderSide
	for _, pos := range r.manager.positions {
		positionValue := absExposure(pos.Size, pos.MarkPrice, pos.Leverage)
		currentValue += positionValue
		if pos.Symbol == ctx.Order.Symbol {
			currentSymbolValue += positionValue
			currentSymbolSide = pos.Side
		}
	}

	additionalValue := orderValue
	if currentSymbolValue > 0 && currentSymbolSide != "" && currentSymbolSide != ctx.Order.Side {
		additionalValue -= currentSymbolValue
		if additionalValue < 0 {
			additionalValue = 0
		}
	}

	// 检查是否超过最大仓位价值
	if currentValue+additionalValue > r.maxPositionSize {
		return ErrPositionLimitExceeded
	}

	return nil
}

func (r *PositionLimitRule) GetConfig() map[string]interface{} {
	return map[string]interface{}{
		"max_position_size": r.maxPositionSize,
	}
}

func (r *PositionLimitRule) SetConfig(config map[string]interface{}) error {
	if v, ok := config["max_position_size"].(float64); ok && v > 0 {
		r.maxPositionSize = v
		return nil
	}
	return fmt.Errorf("invalid max_position_size value")
}

// DailyLossRule 日亏损限制规则
type DailyLossRule struct {
	manager      *Manager
	maxDailyLoss float64
}

func (r *DailyLossRule) Name() string { return "daily_loss" }

func (r *DailyLossRule) Check(ctx *RiskContext) error {
	r.manager.mutex.Lock()
	defer r.manager.mutex.Unlock()

	r.manager.checkDailyResetLocked()

	if r.manager.dailyLoss >= r.maxDailyLoss {
		return ErrDailyLossExceeded
	}

	return nil
}

func (r *DailyLossRule) GetConfig() map[string]interface{} {
	return map[string]interface{}{
		"max_daily_loss": r.maxDailyLoss,
	}
}

func (r *DailyLossRule) SetConfig(config map[string]interface{}) error {
	if v, ok := config["max_daily_loss"].(float64); ok && v > 0 {
		r.maxDailyLoss = v
		return nil
	}
	return fmt.Errorf("invalid max_daily_loss value")
}

// TradeLimitRule 交易次数限制规则
type TradeLimitRule struct {
	manager         *Manager
	maxTradesPerDay int
}

func (r *TradeLimitRule) Name() string { return "trade_limit" }

func (r *TradeLimitRule) Check(ctx *RiskContext) error {
	r.manager.mutex.Lock()
	defer r.manager.mutex.Unlock()

	r.manager.checkDailyResetLocked()

	if r.manager.dailyTrades >= r.maxTradesPerDay {
		return ErrTradeLimitExceeded
	}

	return nil
}

func (r *TradeLimitRule) GetConfig() map[string]interface{} {
	return map[string]interface{}{
		"max_trades_per_day": r.maxTradesPerDay,
	}
}

func (r *TradeLimitRule) SetConfig(config map[string]interface{}) error {
	if v, ok := config["max_trades_per_day"].(int); ok && v > 0 {
		r.maxTradesPerDay = v
		return nil
	}
	return fmt.Errorf("invalid max_trades_per_day value")
}

// MaxDrawdownRule 最大回撤规则
type MaxDrawdownRule struct {
	manager     *Manager
	maxDrawdown float64
}

func (r *MaxDrawdownRule) Name() string { return "max_drawdown" }

func (r *MaxDrawdownRule) Check(ctx *RiskContext) error {
	if ctx.Account == nil {
		return nil
	}

	currentEquity := ctx.Account.TotalEquity

	r.manager.mutex.Lock()
	defer r.manager.mutex.Unlock()

	// 更新历史最高权益
	if currentEquity > r.manager.peakEquity {
		r.manager.peakEquity = currentEquity
	}

	// 计算当前回撤
	var currentDrawdown float64
	if r.manager.peakEquity > 0 {
		currentDrawdown = (r.manager.peakEquity - currentEquity) / r.manager.peakEquity
	}

	// 更新最大回撤
	if currentDrawdown > r.manager.maxDrawdown {
		r.manager.maxDrawdown = currentDrawdown
	}

	// 检查是否超过配置的最大回撤限制
	if currentDrawdown > r.maxDrawdown {
		return ErrMaxDrawdownExceeded
	}

	return nil
}

func (r *MaxDrawdownRule) GetConfig() map[string]interface{} {
	return map[string]interface{}{
		"max_drawdown": r.maxDrawdown,
	}
}

func (r *MaxDrawdownRule) SetConfig(config map[string]interface{}) error {
	if v, ok := config["max_drawdown"].(float64); ok && v > 0 && v < 1 {
		r.maxDrawdown = v
		return nil
	}
	return fmt.Errorf("invalid max_drawdown value, must be between 0 and 1")
}

// SingleTradeRiskRule 单笔交易风险规则
type SingleTradeRiskRule struct {
	manager         *Manager
	maxRiskPerTrade float64
}

func (r *SingleTradeRiskRule) Name() string { return "single_trade_risk" }

func (r *SingleTradeRiskRule) Check(ctx *RiskContext) error {
	if ctx.Order == nil || ctx.Account == nil {
		return nil
	}
	if ctx.Account.TotalEquity <= 0 {
		return nil
	}

	orderValue, err := r.manager.resolveOrderValue(ctx.Order)
	if err != nil {
		return err
	}

	r.manager.mutex.RLock()
	currentSymbolValue := 0.0
	var currentSymbolSide types.OrderSide
	for _, pos := range r.manager.positions {
		if pos.Symbol != ctx.Order.Symbol {
			continue
		}
		positionValue := absExposure(pos.Size, pos.MarkPrice, pos.Leverage)
		currentSymbolValue += positionValue
		if currentSymbolSide == "" {
			currentSymbolSide = pos.Side
		}
	}
	r.manager.mutex.RUnlock()

	additionalValue := orderValue
	if currentSymbolValue > 0 && currentSymbolSide != "" && currentSymbolSide != ctx.Order.Side {
		additionalValue -= currentSymbolValue
		if additionalValue < 0 {
			additionalValue = 0
		}
	}

	// 计算单笔交易风险占账户权益的比例
	riskRatio := additionalValue / ctx.Account.TotalEquity
	if riskRatio > r.maxRiskPerTrade {
		return fmt.Errorf("%w: single trade risk ratio %.4f exceeds limit %.4f",
			ErrSingleTradeRiskExceeded, riskRatio, r.maxRiskPerTrade)
	}

	return nil
}

func (r *SingleTradeRiskRule) GetConfig() map[string]interface{} {
	return map[string]interface{}{
		"max_risk_per_trade": r.maxRiskPerTrade,
	}
}

func (r *SingleTradeRiskRule) SetConfig(config map[string]interface{}) error {
	if v, ok := config["max_risk_per_trade"].(float64); ok && v > 0 && v < 1 {
		r.maxRiskPerTrade = v
		return nil
	}
	return fmt.Errorf("invalid max_risk_per_trade value, must be between 0 and 1")
}

// SymbolExposureRule 品种风险敞口规则
type SymbolExposureRule struct {
	manager              *Manager
	maxExposurePerSymbol float64
}

func (r *SymbolExposureRule) Name() string { return "symbol_exposure" }

func (r *SymbolExposureRule) Check(ctx *RiskContext) error {
	if ctx.Order == nil || ctx.Account == nil {
		return nil
	}
	if ctx.Account.TotalEquity <= 0 {
		return nil
	}

	orderValue, err := r.manager.resolveOrderValue(ctx.Order)
	if err != nil {
		return err
	}

	r.manager.mutex.RLock()
	defer r.manager.mutex.RUnlock()

	// 计算当前品种的仓位价值
	currentSymbolValue := 0.0
	var currentSymbolSide types.OrderSide
	for _, pos := range r.manager.positions {
		if pos.Symbol == ctx.Order.Symbol {
			positionValue := absExposure(pos.Size, pos.MarkPrice, pos.Leverage)
			currentSymbolValue += positionValue
			if currentSymbolSide == "" {
				currentSymbolSide = pos.Side
			}
		}
	}

	additionalValue := orderValue
	if currentSymbolValue > 0 && currentSymbolSide != "" && currentSymbolSide != ctx.Order.Side {
		additionalValue -= currentSymbolValue
		if additionalValue < 0 {
			additionalValue = 0
		}
	}

	totalSymbolExposure := currentSymbolValue + additionalValue
	exposureRatio := totalSymbolExposure / ctx.Account.TotalEquity

	if exposureRatio > r.maxExposurePerSymbol {
		return fmt.Errorf("%w: symbol %s exposure ratio %.4f exceeds limit %.4f",
			ErrSymbolExposureExceeded, ctx.Order.Symbol, exposureRatio, r.maxExposurePerSymbol)
	}

	return nil
}

func (r *SymbolExposureRule) GetConfig() map[string]interface{} {
	return map[string]interface{}{
		"max_exposure_per_symbol": r.maxExposurePerSymbol,
	}
}

func (r *SymbolExposureRule) SetConfig(config map[string]interface{}) error {
	if v, ok := config["max_exposure_per_symbol"].(float64); ok && v > 0 && v < 1 {
		r.maxExposurePerSymbol = v
		return nil
	}
	return fmt.Errorf("invalid max_exposure_per_symbol value, must be between 0 and 1")
}

func absExposure(size, price float64, leverage int) float64 {
	multiplier := 1.0
	if leverage > 1 {
		multiplier = float64(leverage)
	}

	return math.Abs(size*price) * multiplier
}

package llmanalysis

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ljwqf/quant/internal/alertservice"
	"github.com/ljwqf/quant/internal/exchange"
	"github.com/ljwqf/quant/internal/risk"
	"github.com/ljwqf/quant/pkg/logger"
	"github.com/ljwqf/quant/pkg/types"
	"go.uber.org/zap"
)

// PositionMonitorConfig LLM持仓监控配置
type PositionMonitorConfig struct {
	Enable         bool
	CheckInterval  time.Duration
	RiskThreshold  string  // 触发告警的最低风险等级: "low", "medium", "high"
	MinPnLPercent    float64 // 盈亏百分比阈值，超过才触发分析
	MaxConsecutiveFailures int // 最大连续失败次数
}

// DefaultPositionMonitorConfig 默认配置
func DefaultPositionMonitorConfig() *PositionMonitorConfig {
	return &PositionMonitorConfig{
		Enable:         false,
		CheckInterval:  5 * time.Minute,
		RiskThreshold:  "high",
		MinPnLPercent:  2.0,
		MaxConsecutiveFailures: 3,
	}
}

// PositionMonitor LLM持仓监控器
type PositionMonitor struct {
	exch        exchange.Exchange
	analyzer    *Analyzer
	alertSvc    *alertservice.AlertService
	riskEngine  *risk.Engine
	cfg         *PositionMonitorConfig
	stopCh      chan struct{}
	once        sync.Once
	running     bool
	mu          sync.RWMutex

	// 防抖：记录上次分析时间和结果
	lastAnalysisTime map[string]time.Time
	lastAlertTime    map[string]time.Time
	alertCooldown    time.Duration

	// 连续失败计数
	consecutiveFailures int
}

// NewPositionMonitor 创建LLM持仓监控器
func NewPositionMonitor(exch exchange.Exchange, analyzer *Analyzer, alertSvc *alertservice.AlertService, riskEngine *risk.Engine, cfg *PositionMonitorConfig) *PositionMonitor {
	if cfg == nil {
		cfg = DefaultPositionMonitorConfig()
	}
	if cfg.CheckInterval <= 0 {
		cfg.CheckInterval = 5 * time.Minute
	}
	if cfg.RiskThreshold == "" {
		cfg.RiskThreshold = "high"
	}
	return &PositionMonitor{
		exch:             exch,
		analyzer:         analyzer,
		alertSvc:         alertSvc,
		riskEngine:       riskEngine,
		cfg:              cfg,
		stopCh:           make(chan struct{}),
		lastAnalysisTime: make(map[string]time.Time),
		lastAlertTime:    make(map[string]time.Time),
		alertCooldown:    30 * time.Minute, // 同一持仓30分钟内不重复告警
	}
}

// Start 启动监控
func (pm *PositionMonitor) Start() {
	if !pm.cfg.Enable {
		logger.Info("LLM持仓监控未启用")
		return
	}
	if pm.analyzer == nil {
		logger.Warn("LLM分析器未初始化，跳过持仓监控启动")
		return
	}

	pm.mu.Lock()
	pm.running = true
	pm.mu.Unlock()

	go pm.run()
	logger.Info("LLM持仓监控已启动",
		zap.Duration("interval", pm.cfg.CheckInterval),
		zap.String("risk_threshold", pm.cfg.RiskThreshold))
}

// Stop 停止监控
func (pm *PositionMonitor) Stop() {
	pm.once.Do(func() {
		close(pm.stopCh)
		pm.mu.Lock()
		pm.running = false
		pm.mu.Unlock()
		logger.Info("LLM持仓监控已停止")
	})
}

func (pm *PositionMonitor) run() {
	ticker := time.NewTicker(pm.cfg.CheckInterval)
	defer ticker.Stop()

	// 启动时立即执行一次
	pm.checkPositions()

	for {
		select {
		case <-pm.stopCh:
			return
		case <-ticker.C:
			pm.checkPositions()
		}
	}
}

func (pm *PositionMonitor) checkPositions() {
	positions, err := pm.exch.GetPositions()
	if err != nil {
		pm.consecutiveFailures++
		logger.Warn("获取持仓失败，LLM监控跳过",
			zap.Error(err),
			zap.Int("consecutive_failures", pm.consecutiveFailures))

		if pm.consecutiveFailures >= pm.cfg.MaxConsecutiveFailures {
			logger.Error("LLM持仓监控连续失败次数过多，暂停监控",
				zap.Int("failures", pm.consecutiveFailures))
			pm.Stop()
		}
		return
	}

	// 重置失败计数
	pm.consecutiveFailures = 0

	if len(positions) == 0 {
		logger.Debug("无活跃持仓，跳过LLM分析")
		return
	}

	for _, pos := range positions {
		pm.analyzePosition(pos)
	}
}

func (pm *PositionMonitor) analyzePosition(pos *types.Position) {
	// 防抖：检查分析冷却
	pm.mu.RLock()
	lastTime, ok := pm.lastAnalysisTime[pos.Symbol]
	pm.mu.RUnlock()
	if ok && time.Since(lastTime) < pm.cfg.CheckInterval {
		return
	}

	// 盈亏未达阈值时跳过（除非风险等级已很高）
	pnlPercent := calculatePnLPercent(pos)
	if pnlPercent < pm.cfg.MinPnLPercent && pnlPercent > -pm.cfg.MinPnLPercent {
		// 小幅度波动不需要LLM分析
		return
	}

	// 检查告警冷却
	pm.mu.RLock()
	lastAlert, ok := pm.lastAlertTime[pos.Symbol]
	pm.mu.RUnlock()
	if ok && time.Since(lastAlert) < pm.alertCooldown {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 构建持仓分析数据
	positionData := map[string]interface{}{
		"symbol":         pos.Symbol,
		"side":           string(pos.Side),
		"size":           pos.Size,
		"entry_price":    pos.EntryPrice,
		"mark_price":     pos.MarkPrice,
		"unrealized_pnl": pos.UnrealizedPnL,
		"leverage":       pos.Leverage,
		"pnl_percent":    pnlPercent,
		"liquidation":    pos.LiquidationPrice,
	}

	result, err := pm.analyzer.AnalyzePosition(ctx, pos.Symbol, positionData)
	if err != nil {
		logger.Warn("LLM持仓分析失败",
			zap.String("symbol", pos.Symbol),
			zap.Error(err))
		return
	}

	pm.mu.Lock()
	pm.lastAnalysisTime[pos.Symbol] = time.Now()
	pm.mu.Unlock()

	// 判断是否需要告警
	if pm.shouldAlert(result, pnlPercent) {
		pm.sendPositionAlert(pos, result, pnlPercent)
		pm.mu.Lock()
		pm.lastAlertTime[pos.Symbol] = time.Now()
		pm.mu.Unlock()
	}
}

func (pm *PositionMonitor) shouldAlert(result *AnalysisResult, pnlPercent float64) bool {
	riskLevels := map[string]int{
		"low":    1,
		"medium": 2,
		"high":   3,
		"critical": 4,
	}

	threshold := riskLevels[strings.ToLower(pm.cfg.RiskThreshold)]
	if threshold == 0 {
		threshold = riskLevels["high"]
	}

	currentLevel := riskLevels[strings.ToLower(result.RiskLevel)]
	if currentLevel >= threshold {
		return true
	}

	// 检查建议中是否包含明确的平仓/退出信号
	summary := strings.ToLower(result.Summary)
	closeKeywords := []string{"平仓", "退出", "close", "exit", "减仓", "reduce", "止损", "清仓", "sell off"}
	for _, kw := range closeKeywords {
		if strings.Contains(summary, kw) {
			return true
		}
	}

	return false
}

func (pm *PositionMonitor) sendPositionAlert(pos *types.Position, result *AnalysisResult, pnlPercent float64) {
	pnlDirection := "盈利"
	if pnlPercent < 0 {
		pnlDirection = "亏损"
	}

	level := alertservice.AlertLevelWarning
	if strings.ToLower(result.RiskLevel) == "critical" || strings.ToLower(result.RiskLevel) == "高" {
		level = alertservice.AlertLevelCritical
	}

	title := fmt.Sprintf("LLM持仓告警 - %s", pos.Symbol)
	message := fmt.Sprintf(
		"%s %s %.2f%% (未实现盈亏: %.2f USDT)\n"+
			"风险等级: %s\n"+
			"杠杆倍数: %dx\n"+
			"LLM分析建议: %s\n"+
			"入场价格: %.2f | 标记价格: %.2f",
		pos.Symbol,
		pnlDirection,
		pnlPercent,
		pos.UnrealizedPnL,
		result.RiskLevel,
		pos.Leverage,
		result.Summary,
		pos.EntryPrice,
		pos.MarkPrice,
	)

	alert := &alertservice.Alert{
		Type:    alertservice.AlertTypeRiskWarning,
		Level:   level,
		Title:   title,
		Message: message,
		Symbol:  pos.Symbol,
	}

	if err := pm.alertSvc.SendAlert(alert); err != nil {
		logger.Warn("发送LLM持仓告警失败",
			zap.String("symbol", pos.Symbol),
			zap.Error(err))
	} else {
		logger.Info("LLM持仓告警已发送",
			zap.String("symbol", pos.Symbol),
			zap.String("risk_level", result.RiskLevel),
			zap.String("level", string(level)))
	}
}

func calculatePnLPercent(pos *types.Position) float64 {
	if pos.EntryPrice <= 0 || pos.Size <= 0 {
		return 0
	}

	// 计算盈亏百分比
	notionalValue := pos.EntryPrice * pos.Size
	if notionalValue == 0 {
		return 0
	}

	pnlPercent := (pos.UnrealizedPnL / notionalValue) * 100

	// 杠杆调整后的盈亏百分比
	if pos.Leverage > 0 {
		pnlPercent = pnlPercent * float64(pos.Leverage)
	}

	return pnlPercent
}

// GetStatus 获取监控状态
func (pm *PositionMonitor) GetStatus() map[string]interface{} {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	status := map[string]interface{}{
		"running":              pm.running,
		"enabled":              pm.cfg.Enable,
		"check_interval":       pm.cfg.CheckInterval.String(),
		"risk_threshold":       pm.cfg.RiskThreshold,
		"consecutive_failures": pm.consecutiveFailures,
		"last_analysis_count":  len(pm.lastAnalysisTime),
		"alert_cooldown":       pm.alertCooldown.String(),
	}

	// 最近分析的币种
	symbols := make([]string, 0, len(pm.lastAnalysisTime))
	for sym, t := range pm.lastAnalysisTime {
		symbols = append(symbols, fmt.Sprintf("%s@%s", sym, t.Format(time.RFC3339)))
	}
	status["last_analyzed_symbols"] = symbols

	return status
}

package alertservice

import (
	"fmt"
	"sync"
	"time"

	"github.com/ljwqf/quant/internal/config"
	"github.com/ljwqf/quant/internal/storage"
	"github.com/ljwqf/quant/internal/storage/repository"
	"github.com/ljwqf/quant/pkg/logger"
	"go.uber.org/zap"
)

// AlertType 提醒类型
type AlertType string

const (
	AlertTypePriceChange   AlertType = "price_change"
	AlertTypeNews          AlertType = "news"
	AlertTypeEconomicEvent AlertType = "economic_event"
	AlertTypeRiskWarning   AlertType = "risk_warning"
	AlertTypeSystem        AlertType = "system"
)

// AlertLevel 提醒级别
type AlertLevel string

const (
	AlertLevelInfo    AlertLevel = "info"
	AlertLevelWarning AlertLevel = "warning"
	AlertLevelError   AlertLevel = "error"
	AlertLevelCritical AlertLevel = "critical"
)

// AlertService 提醒服务
type AlertService struct {
	cfg          *config.Config
	alertRepo    repository.AlertRecordRepository
	channels     []AlertChannel
	running      bool
	stopCh       chan struct{}
	wg           sync.WaitGroup
	mutex        sync.RWMutex
	priceMonitor map[string]float64
}

// AlertChannel 提醒通道接口
type AlertChannel interface {
	Send(alert *Alert) error
	Name() string
}

// Alert 提醒消息
type Alert struct {
	ID        int64      `json:"id"`
	Type      AlertType  `json:"type"`
	Level     AlertLevel `json:"level"`
	Title     string     `json:"title"`
	Message   string     `json:"message"`
	Symbol    string     `json:"symbol,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	SentAt    *time.Time `json:"sent_at,omitempty"`
}

// NewAlertService 创建提醒服务
func NewAlertService(cfg *config.Config, db *storage.Database) *AlertService {
	channels := make([]AlertChannel, 0)

	if cfg.Alert.Enable {
		for _, channelName := range cfg.Alert.Channels {
			logger.Info("初始化提醒通道", zap.String("channel", channelName))
		}
	}

	return &AlertService{
		cfg:          cfg,
		alertRepo:    repository.NewAlertRecordRepository(db.DB()),
		channels:     channels,
		priceMonitor: make(map[string]float64),
		stopCh:       make(chan struct{}),
	}
}

// Start 启动提醒服务
func (s *AlertService) Start() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.running {
		return nil
	}

	s.running = true
	s.stopCh = make(chan struct{})

	logger.Info("提醒服务启动")

	s.wg.Add(1)
	go s.alertLoop()

	return nil
}

// Stop 停止提醒服务
func (s *AlertService) Stop() {
	s.mutex.Lock()
	if !s.running {
		s.mutex.Unlock()
		return
	}
	s.running = false
	close(s.stopCh)
	s.mutex.Unlock()

	s.wg.Wait()
	logger.Info("提醒服务已停止")
}

// IsRunning 检查服务是否运行
func (s *AlertService) IsRunning() bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.running
}

// alertLoop 提醒检查循环
func (s *AlertService) alertLoop() {
	defer s.wg.Done()

	interval := s.cfg.Alert.CheckInterval
	if interval <= 0 {
		interval = 1 * time.Minute
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.checkAlerts()
		}
	}
}

// checkAlerts 检查并发送提醒
func (s *AlertService) checkAlerts() {
	logger.Debug("检查提醒条件")

	if s.cfg.Alert.Enable {
		s.checkPriceAlerts()
		s.checkNewsAlerts()
		s.checkEconomicEventAlerts()
	}
}

// checkPriceAlerts 检查价格提醒
func (s *AlertService) checkPriceAlerts() {
}

// checkNewsAlerts 检查新闻提醒
func (s *AlertService) checkNewsAlerts() {
}

// checkEconomicEventAlerts 检查经济事件提醒
func (s *AlertService) checkEconomicEventAlerts() {
}

// SendAlert 发送提醒
func (s *AlertService) SendAlert(alert *Alert) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	alert.CreatedAt = time.Now()

	record := &storage.AlertRecord{
		AlertType: string(alert.Type),
		Level:     string(alert.Level),
		Title:     alert.Title,
		Message:   alert.Message,
		Symbol:    alert.Symbol,
		CreatedAt: alert.CreatedAt,
	}

	if err := s.alertRepo.Create(record); err != nil {
		logger.Warn("保存提醒记录失败", zap.Error(err))
	}

	alert.ID = record.ID

	for _, channel := range s.channels {
		if err := channel.Send(alert); err != nil {
			logger.Warn("发送提醒失败",
				zap.String("channel", channel.Name()),
				zap.Error(err))
		}
	}

	logger.Info("提醒已发送",
		zap.String("type", string(alert.Type)),
		zap.String("level", string(alert.Level)),
		zap.String("title", alert.Title))

	return nil
}

// CreatePriceAlert 创建价格提醒
func (s *AlertService) CreatePriceAlert(symbol string, price float64, condition string) error {
	alert := &Alert{
		Type:    AlertTypePriceChange,
		Level:   AlertLevelWarning,
		Title:   "价格提醒",
		Message: fmt.Sprintf("%s 价格达到 %.2f，条件: %s", symbol, price, condition),
		Symbol:  symbol,
	}
	return s.SendAlert(alert)
}

// CreateNewsAlert 创建新闻提醒
func (s *AlertService) CreateNewsAlert(title string, message string, symbol string) error {
	alert := &Alert{
		Type:    AlertTypeNews,
		Level:   AlertLevelInfo,
		Title:   title,
		Message: message,
		Symbol:  symbol,
	}
	return s.SendAlert(alert)
}

// CreateRiskAlert 创建风险提醒
func (s *AlertService) CreateRiskAlert(title string, message string) error {
	alert := &Alert{
		Type:    AlertTypeRiskWarning,
		Level:   AlertLevelError,
		Title:   title,
		Message: message,
	}
	return s.SendAlert(alert)
}

// CreateSystemAlert 创建系统提醒
func (s *AlertService) CreateSystemAlert(title string, message string, level AlertLevel) error {
	alert := &Alert{
		Type:    AlertTypeSystem,
		Level:   level,
		Title:   title,
		Message: message,
	}
	return s.SendAlert(alert)
}

// GetRecentAlerts 获取最近的提醒
func (s *AlertService) GetRecentAlerts(limit int) ([]*storage.AlertRecord, error) {
	return s.alertRepo.ListRecent(limit)
}

// GetAlertsByType 获取指定类型的提醒
func (s *AlertService) GetAlertsByType(alertType string, limit int) ([]*storage.AlertRecord, error) {
	return s.alertRepo.ListByType(alertType, limit, 0)
}

// RegisterChannel 注册提醒通道
func (s *AlertService) RegisterChannel(channel AlertChannel) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.channels = append(s.channels, channel)
	logger.Info("注册提醒通道", zap.String("channel", channel.Name()))
}

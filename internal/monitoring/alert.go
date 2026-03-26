package monitoring

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ljwqf/quant/pkg/logger"
	"go.uber.org/zap"
)

// AlertType 告警类型
type AlertType string

const (
	AlertTypeInfo     AlertType = "info"
	AlertTypeWarning  AlertType = "warning"
	AlertTypeError    AlertType = "error"
	AlertTypeCritical AlertType = "critical"
)

// Alert 告警信息
type Alert struct {
	Type      AlertType              `json:"type"`
	Title     string                 `json:"title"`
	Message   string                 `json:"message"`
	Timestamp time.Time              `json:"timestamp"`
	Severity  int                    `json:"severity"`
	Labels    map[string]string      `json:"labels,omitempty"`
	Details   map[string]interface{} `json:"details,omitempty"`
}

type WebhookPayload struct {
	EventID     string                 `json:"event_id"`
	Fingerprint string                 `json:"fingerprint"`
	Source      string                 `json:"source"`
	Type        AlertType              `json:"type"`
	Title       string                 `json:"title"`
	Message     string                 `json:"message"`
	Severity    int                    `json:"severity"`
	Timestamp   time.Time              `json:"timestamp"`
	Environment string                 `json:"environment,omitempty"`
	Labels      map[string]string      `json:"labels"`
	Details     map[string]interface{} `json:"details"`
}

type deliveryState struct {
	lastAccepted time.Time
	lastWebhook  time.Time
}

const (
	defaultAlertDedupWindow = 30 * time.Second
	defaultAlertMinInterval = 10 * time.Second
)

// AlertManager 告警管理器
type AlertManager struct {
	config     *AlertConfig
	alerts     []Alert
	httpClient *http.Client
	mutex      sync.RWMutex
	stateByKey map[string]deliveryState
}

// NewAlertManager 创建告警管理器
func NewAlertManager(config *AlertConfig) *AlertManager {
	if config == nil {
		config = &AlertConfig{}
	}
	return &AlertManager{
		config:     config,
		alerts:     make([]Alert, 0),
		httpClient: &http.Client{Timeout: 10 * time.Second},
		stateByKey: make(map[string]deliveryState),
	}
}

// Start 启动告警管理器
func (am *AlertManager) Start() error {
	if !am.config.Enable {
		return nil
	}

	logger.Info("启动告警管理器")

	// 这里可以实现告警管理器的启动逻辑

	return nil
}

// Stop 停止告警管理器
func (am *AlertManager) Stop() {
	if !am.config.Enable {
		return
	}

	logger.Info("停止告警管理器")

	// 这里可以实现告警管理器的停止逻辑
}

// Alert 发送告警
func (am *AlertManager) Alert(alertType AlertType, title, message string) error {
	return am.AlertWithContext(alertType, title, message, nil, nil)
}

func (am *AlertManager) AlertWithContext(alertType AlertType, title, message string, labels map[string]string, details map[string]interface{}) error {
	if !am.config.Enable {
		return nil
	}

	alert := Alert{
		Type:      alertType,
		Title:     title,
		Message:   message,
		Timestamp: time.Now(),
		Severity:  am.getSeverity(alertType),
		Labels:    normalizeLabels(alertType, title, labels),
		Details:   cloneInterfaceMap(details),
	}
	fingerprint := alertFingerprint(alert)
	allowHistory, allowWebhook := am.allowDelivery(fingerprint, alert.Timestamp)
	if !allowHistory {
		logger.Info("告警在去重窗口内被抑制",
			zap.String("type", string(alertType)),
			zap.String("title", title),
		)
		return nil
	}

	// 添加到告警历史
	am.mutex.Lock()
	am.alerts = append(am.alerts, alert)
	// 只保留最近100条告警
	if len(am.alerts) > 100 {
		am.alerts = am.alerts[len(am.alerts)-100:]
	}
	am.mutex.Unlock()

	// 记录告警
	logger.Info("发送告警",
		zap.String("type", string(alertType)),
		zap.String("title", title),
		zap.String("message", message),
	)

	// 发送到不同渠道
	for _, channel := range am.config.Channels {
		switch strings.ToLower(channel) {
		case "console":
			am.sendToConsole(alert)
		case "webhook":
			if allowWebhook {
				am.sendToWebhook(alert, fingerprint)
			} else {
				logger.Info("Webhook告警在限频窗口内被跳过",
					zap.String("title", title),
				)
			}
			// 可以添加其他渠道，如邮件、短信等
		}
	}

	return nil
}

// getSeverity 获取告警严重程度
func (am *AlertManager) getSeverity(alertType AlertType) int {
	switch alertType {
	case AlertTypeInfo:
		return 1
	case AlertTypeWarning:
		return 2
	case AlertTypeError:
		return 3
	case AlertTypeCritical:
		return 4
	default:
		return 1
	}
}

// sendToConsole 发送到控制台（记录到日志）
func (am *AlertManager) sendToConsole(alert Alert) {
	logger.Info("告警通知",
		zap.String("timestamp", alert.Timestamp.Format("2006-01-02 15:04:05")),
		zap.String("type", string(alert.Type)),
		zap.String("title", alert.Title),
		zap.String("message", alert.Message),
	)
}

// sendToWebhook 发送到Webhook
func (am *AlertManager) sendToWebhook(alert Alert, fingerprint string) {
	if am.config.WebhookURL == "" {
		return
	}
	payload := WebhookPayload{
		EventID:     fmt.Sprintf("%d-%s", alert.Timestamp.UnixNano(), fingerprint[:12]),
		Fingerprint: fingerprint,
		Source:      "okx-quant",
		Type:        alert.Type,
		Title:       alert.Title,
		Message:     alert.Message,
		Severity:    alert.Severity,
		Timestamp:   alert.Timestamp,
		Labels:      cloneStringMap(alert.Labels),
		Details:     cloneInterfaceMap(alert.Details),
	}
	if payload.Labels == nil {
		payload.Labels = map[string]string{}
	}
	if payload.Details == nil {
		payload.Details = map[string]interface{}{}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		logger.Warn("序列化Webhook告警失败", zap.Error(err))
		return
	}
	req, err := http.NewRequest(http.MethodPost, am.config.WebhookURL, bytes.NewReader(body))
	if err != nil {
		logger.Warn("创建Webhook请求失败", zap.Error(err))
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "okx-quant-alert/1.0")

	resp, err := am.httpClient.Do(req)
	if err != nil {
		logger.Warn("发送Webhook告警失败", zap.Error(err), zap.String("url", am.config.WebhookURL))
		return
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if closeErr := resp.Body.Close(); closeErr != nil {
			logger.Warn("关闭Webhook响应体失败", zap.Error(closeErr), zap.String("url", am.config.WebhookURL))
		}
		logger.Warn("Webhook告警返回非成功状态",
			zap.String("url", am.config.WebhookURL),
			zap.Int("status_code", resp.StatusCode),
		)
		return
	}
	if closeErr := resp.Body.Close(); closeErr != nil {
		logger.Warn("关闭Webhook响应体失败", zap.Error(closeErr), zap.String("url", am.config.WebhookURL))
	}
	logger.Info("发送告警到Webhook",
		zap.String("url", am.config.WebhookURL),
		zap.String("fingerprint", fingerprint),
	)
}

// GetAlerts 获取告警历史
func (am *AlertManager) GetAlerts() []Alert {
	// 创建副本
	am.mutex.RLock()
	defer am.mutex.RUnlock()
	alerts := make([]Alert, len(am.alerts))
	copy(alerts, am.alerts)

	return alerts
}

// GetRecentAlerts 获取最近的告警
func (am *AlertManager) GetRecentAlerts(limit int) []Alert {
	am.mutex.RLock()
	defer am.mutex.RUnlock()
	if limit <= 0 {
		limit = 10
	}

	if len(am.alerts) <= limit {
		alerts := make([]Alert, len(am.alerts))
		copy(alerts, am.alerts)
		return alerts
	}

	alerts := make([]Alert, limit)
	copy(alerts, am.alerts[len(am.alerts)-limit:])
	return alerts
}

func (am *AlertManager) allowDelivery(fingerprint string, now time.Time) (bool, bool) {
	dedupWindow := am.config.DedupWindow
	if dedupWindow <= 0 {
		dedupWindow = defaultAlertDedupWindow
	}
	minInterval := am.config.MinInterval
	if minInterval <= 0 {
		minInterval = defaultAlertMinInterval
	}

	am.mutex.Lock()
	defer am.mutex.Unlock()
	state := am.stateByKey[fingerprint]
	if !state.lastAccepted.IsZero() && now.Sub(state.lastAccepted) < dedupWindow {
		return false, false
	}
	allowWebhook := state.lastWebhook.IsZero() || now.Sub(state.lastWebhook) >= minInterval
	state.lastAccepted = now
	if allowWebhook {
		state.lastWebhook = now
	}
	am.stateByKey[fingerprint] = state
	return true, allowWebhook
}

func alertFingerprint(alert Alert) string {
	payload := map[string]interface{}{
		"type":    string(alert.Type),
		"title":   alert.Title,
		"message": alert.Message,
		"labels":  sortedLabelPairs(alert.Labels),
		"details": alert.Details,
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		encoded = []byte(strings.Join([]string{string(alert.Type), alert.Title, alert.Message}, "\n"))
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}

func normalizeLabels(alertType AlertType, title string, labels map[string]string) map[string]string {
	merged := map[string]string{
		"source":   "okx-quant",
		"type":     string(alertType),
		"severity": strconv.Itoa((&AlertManager{}).getSeverity(alertType)),
		"title":    title,
	}
	for key, value := range labels {
		if strings.TrimSpace(key) == "" {
			continue
		}
		merged[key] = value
	}
	return merged
}

func cloneStringMap(source map[string]string) map[string]string {
	if source == nil {
		return nil
	}
	cloned := make(map[string]string, len(source))
	for key, value := range source {
		cloned[key] = value
	}
	return cloned
}

func cloneInterfaceMap(source map[string]interface{}) map[string]interface{} {
	if source == nil {
		return nil
	}
	cloned := make(map[string]interface{}, len(source))
	for key, value := range source {
		cloned[key] = value
	}
	return cloned
}

func sortedLabelPairs(labels map[string]string) []string {
	if len(labels) == 0 {
		return nil
	}
	pairs := make([]string, 0, len(labels))
	for key, value := range labels {
		pairs = append(pairs, key+"="+value)
	}
	sort.Strings(pairs)
	return pairs
}

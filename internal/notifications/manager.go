package notifications

import (
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/ljwqf/quant/pkg/logger"
	"go.uber.org/zap"
)

type NotificationManager struct {
	channels     map[string]NotificationChannel
	results      []*NotificationResult
	mu           sync.RWMutex
	maxResults   int
}

type NotificationManagerConfig struct {
	MaxResults int
}

func NewNotificationManager(config *NotificationManagerConfig) *NotificationManager {
	if config == nil {
		config = &NotificationManagerConfig{
			MaxResults: 1000,
		}
	}
	if config.MaxResults <= 0 {
		config.MaxResults = 1000
	}

	return &NotificationManager{
		channels:   make(map[string]NotificationChannel),
		results:    make([]*NotificationResult, 0, config.MaxResults),
		maxResults: config.MaxResults,
	}
}

func (m *NotificationManager) RegisterChannel(channel NotificationChannel) {
	m.mu.Lock()
	defer m.mu.Unlock()

	name := channel.Name()
	m.channels[name] = channel
	logger.Info("通知渠道已注册", zap.String("channel", name))
}

func (m *NotificationManager) UnregisterChannel(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.channels[name]; exists {
		delete(m.channels, name)
		logger.Info("通知渠道已注销", zap.String("channel", name))
	}
}

func (m *NotificationManager) GetChannel(name string) (NotificationChannel, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	channel, exists := m.channels[name]
	return channel, exists
}

func (m *NotificationManager) ListChannels() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	channels := make([]string, 0, len(m.channels))
	for name := range m.channels {
		channels = append(channels, name)
	}
	return channels
}

func (m *NotificationManager) Send(notification *Notification) []*NotificationResult {
	if notification.ID == "" {
		notification.ID = uuid.New().String()
	}
	if notification.CreatedAt.IsZero() {
		notification.CreatedAt = time.Now()
	}

	m.mu.RLock()
	var targetChannels []NotificationChannel
	if len(notification.Channels) > 0 {
		for _, name := range notification.Channels {
			if channel, exists := m.channels[name]; exists && channel.IsEnabled() {
				targetChannels = append(targetChannels, channel)
			}
		}
	} else {
		for _, channel := range m.channels {
			if channel.IsEnabled() {
				targetChannels = append(targetChannels, channel)
			}
		}
	}
	m.mu.RUnlock()

	results := make([]*NotificationResult, 0, len(targetChannels))
	for _, channel := range targetChannels {
		result := &NotificationResult{
			NotificationID: notification.ID,
			Channel:        channel.Name(),
			SentAt:         time.Now(),
		}

		err := channel.Send(notification)
		if err != nil {
			result.Success = false
			result.Error = err
			logger.Error("通知发送失败",
				zap.String("notification_id", notification.ID),
				zap.String("channel", channel.Name()),
				zap.Error(err))
		} else {
			result.Success = true
			logger.Debug("通知发送成功",
				zap.String("notification_id", notification.ID),
				zap.String("channel", channel.Name()))
		}

		results = append(results, result)
		m.addResult(result)
	}

	return results
}

func (m *NotificationManager) SendAsync(notification *Notification) <-chan []*NotificationResult {
	resultChan := make(chan []*NotificationResult, 1)

	go func() {
		results := m.Send(notification)
		resultChan <- results
		close(resultChan)
	}()

	return resultChan
}

func (m *NotificationManager) GetResults() []*NotificationResult {
	m.mu.RLock()
	defer m.mu.RUnlock()

	results := make([]*NotificationResult, len(m.results))
	copy(results, m.results)
	return results
}

func (m *NotificationManager) GetResultsByNotification(notificationID string) []*NotificationResult {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var results []*NotificationResult
	for _, result := range m.results {
		if result.NotificationID == notificationID {
			results = append(results, result)
		}
	}
	return results
}

func (m *NotificationManager) GetResultsByChannel(channel string) []*NotificationResult {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var results []*NotificationResult
	for _, result := range m.results {
		if result.Channel == channel {
			results = append(results, result)
		}
	}
	return results
}

func (m *NotificationManager) ClearResults() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.results = make([]*NotificationResult, 0, m.maxResults)
	logger.Debug("通知结果已清空")
}

func (m *NotificationManager) addResult(result *NotificationResult) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.results = append(m.results, result)
	if len(m.results) > m.maxResults {
		m.results = m.results[len(m.results)-m.maxResults:]
	}
}

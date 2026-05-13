package notifications

import (
	"fmt"
	"sync"

	"github.com/ljwqf/quant/pkg/logger"
	"go.uber.org/zap"
)

type ConsoleChannel struct {
	name    string
	enabled bool
	mu      sync.RWMutex
}

func NewConsoleChannel() *ConsoleChannel {
	return &ConsoleChannel{
		name:    "console",
		enabled: true,
	}
}

func (c *ConsoleChannel) Name() string {
	return c.name
}

func (c *ConsoleChannel) Send(notification *Notification) error {
	c.mu.RLock()
	enabled := c.enabled
	c.mu.RUnlock()

	if !enabled {
		return nil
	}

	msg := fmt.Sprintf("[%s] [%s] %s: %s",
		notification.CreatedAt.Format("2006-01-02 15:04:05"),
		notification.Type,
		notification.Title,
		notification.Message)

	switch notification.Type {
	case NotificationTypeInfo:
		logger.Info(msg, zap.String("notification_id", notification.ID))
	case NotificationTypeWarning:
		logger.Warn(msg, zap.String("notification_id", notification.ID))
	case NotificationTypeError:
		logger.Error(msg, zap.String("notification_id", notification.ID))
	case NotificationTypeSuccess:
		logger.Info(msg, zap.String("notification_id", notification.ID))
	default:
		logger.Info(msg, zap.String("notification_id", notification.ID))
	}

	return nil
}

func (c *ConsoleChannel) IsEnabled() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.enabled
}

func (c *ConsoleChannel) Enable() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.enabled = true
	logger.Info("控制台通知渠道已启用")
}

func (c *ConsoleChannel) Disable() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.enabled = false
	logger.Info("控制台通知渠道已禁用")
}

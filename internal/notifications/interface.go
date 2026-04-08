package notifications

import "time"

// NotificationType 通知类型
type NotificationType string

const (
	NotificationTypeInfo    NotificationType = "info"
	NotificationTypeWarning NotificationType = "warning"
	NotificationTypeError   NotificationType = "error"
	NotificationTypeSuccess NotificationType = "success"
)

// NotificationPriority 通知优先级
type NotificationPriority string

const (
	NotificationPriorityLow    NotificationPriority = "low"
	NotificationPriorityMedium NotificationPriority = "medium"
	NotificationPriorityHigh   NotificationPriority = "high"
	NotificationPriorityUrgent NotificationPriority = "urgent"
)

// Notification 通知消息结构
type Notification struct {
	ID        string              `json:"id"`
	Type      NotificationType    `json:"type"`
	Priority  NotificationPriority `json:"priority"`
	Title     string              `json:"title"`
	Message   string              `json:"message"`
	Channels  []string            `json:"channels,omitempty"`
	Metadata  map[string]string   `json:"metadata,omitempty"`
	CreatedAt time.Time           `json:"created_at"`
}

// NotificationChannel 通知渠道接口
type NotificationChannel interface {
	// Name 返回渠道名称
	Name() string
	
	// Send 发送通知
	Send(notification *Notification) error
	
	// IsEnabled 检查渠道是否启用
	IsEnabled() bool
}

// NotificationResult 通知发送结果
type NotificationResult struct {
	NotificationID string
	Channel        string
	Success        bool
	Error          error
	SentAt         time.Time
}

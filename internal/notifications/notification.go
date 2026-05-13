package notifications

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrQueueNotRunning = errors.New("通知队列未运行")
	ErrQueueStopped    = errors.New("通知队列已停止")
	ErrQueueFull       = errors.New("通知队列已满")
	ErrQueueTimeout    = errors.New("通知入队超时")
)

type NotificationBuilder struct {
	notification *Notification
}

func NewNotification() *NotificationBuilder {
	return &NotificationBuilder{
		notification: &Notification{
			ID:        uuid.New().String(),
			Type:      NotificationTypeInfo,
			Priority:  NotificationPriorityMedium,
			CreatedAt: time.Now(),
			Metadata:  make(map[string]string),
		},
	}
}

func (b *NotificationBuilder) WithID(id string) *NotificationBuilder {
	b.notification.ID = id
	return b
}

func (b *NotificationBuilder) WithType(typ NotificationType) *NotificationBuilder {
	b.notification.Type = typ
	return b
}

func (b *NotificationBuilder) WithPriority(priority NotificationPriority) *NotificationBuilder {
	b.notification.Priority = priority
	return b
}

func (b *NotificationBuilder) WithTitle(title string) *NotificationBuilder {
	b.notification.Title = title
	return b
}

func (b *NotificationBuilder) WithMessage(message string) *NotificationBuilder {
	b.notification.Message = message
	return b
}

func (b *NotificationBuilder) WithChannels(channels ...string) *NotificationBuilder {
	b.notification.Channels = channels
	return b
}

func (b *NotificationBuilder) WithMetadata(key, value string) *NotificationBuilder {
	b.notification.Metadata[key] = value
	return b
}

func (b *NotificationBuilder) Build() *Notification {
	return b.notification
}

func NewInfoNotification(title, message string) *Notification {
	return NewNotification().
		WithType(NotificationTypeInfo).
		WithTitle(title).
		WithMessage(message).
		Build()
}

func NewWarningNotification(title, message string) *Notification {
	return NewNotification().
		WithType(NotificationTypeWarning).
		WithTitle(title).
		WithMessage(message).
		Build()
}

func NewErrorNotification(title, message string) *Notification {
	return NewNotification().
		WithType(NotificationTypeError).
		WithTitle(title).
		WithMessage(message).
		Build()
}

func NewSuccessNotification(title, message string) *Notification {
	return NewNotification().
		WithType(NotificationTypeSuccess).
		WithTitle(title).
		WithMessage(message).
		Build()
}

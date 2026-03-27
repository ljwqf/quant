package api

import (
	"time"

	"github.com/ljwqf/quant/internal/alertservice"
	"github.com/ljwqf/quant/pkg/logger"
	"go.uber.org/zap"
)

// AlertMessage WebSocket提醒消息
type AlertMessage struct {
	ID        int64     `json:"id"`
	Type      string    `json:"type"`
	Level     string    `json:"level"`
	Title     string    `json:"title"`
	Message   string    `json:"message"`
	Symbol    string    `json:"symbol,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// WebSocketAlertChannel WebSocket提醒通道
// 将提醒推送到WebSocket客户端
type WebSocketAlertChannel struct {
	hub *WebSocketHub
}

// NewWebSocketAlertChannel 创建WebSocket提醒通道
func NewWebSocketAlertChannel(hub *WebSocketHub) *WebSocketAlertChannel {
	return &WebSocketAlertChannel{
		hub: hub,
	}
}

// Send 发送提醒到WebSocket客户端
func (c *WebSocketAlertChannel) Send(alert *AlertMessage) error {
	logger.Debug("WebSocket通道发送提醒",
		zap.Int64("id", alert.ID),
		zap.String("type", alert.Type),
		zap.String("level", alert.Level),
		zap.String("title", alert.Title))

	c.hub.BroadcastTo(EventTypeAlert, alert)
	return nil
}

// Name 返回通道名称
func (c *WebSocketAlertChannel) Name() string {
	return "websocket"
}

// AlertServiceChannelAdapter 适配器
// 实现alertservice.AlertChannel接口，将alertservice.Alert转换为AlertMessage
type AlertServiceChannelAdapter struct {
	Channel *WebSocketAlertChannel
}

// NewAlertServiceChannelAdapter 创建适配器
func NewAlertServiceChannelAdapter(hub *WebSocketHub) *AlertServiceChannelAdapter {
	return &AlertServiceChannelAdapter{
		Channel: NewWebSocketAlertChannel(hub),
	}
}

// Send 实现alertservice.AlertChannel接口
func (a *AlertServiceChannelAdapter) Send(alert *alertservice.Alert) error {
	msg := &AlertMessage{
		ID:        alert.ID,
		Type:      string(alert.Type),
		Level:     string(alert.Level),
		Title:     alert.Title,
		Message:   alert.Message,
		Symbol:    alert.Symbol,
		CreatedAt: alert.CreatedAt,
	}
	return a.Channel.Send(msg)
}

// Name 实现alertservice.AlertChannel接口
func (a *AlertServiceChannelAdapter) Name() string {
	return "websocket"
}
package notifications

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/ljwqf/quant/pkg/logger"
	"go.uber.org/zap"
)

type DiscordConfig struct {
	WebhookURLs []string
	Username    string
	AvatarURL   string
}

type DiscordChannel struct {
	name     string
	config   *DiscordConfig
	enabled  bool
	client   *http.Client
	mu       sync.RWMutex
}

type discordEmbed struct {
	Title       string              `json:"title"`
	Description string              `json:"description"`
	Color       int                 `json:"color"`
	Fields      []discordEmbedField `json:"fields,omitempty"`
	Timestamp   string              `json:"timestamp,omitempty"`
	Footer      discordEmbedFooter  `json:"footer,omitempty"`
}

type discordEmbedField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline,omitempty"`
}

type discordEmbedFooter struct {
	Text string `json:"text"`
}

type discordWebhookRequest struct {
	Username  string         `json:"username,omitempty"`
	AvatarURL string         `json:"avatar_url,omitempty"`
	Embeds    []discordEmbed `json:"embeds"`
}

type discordResponse struct {
	Code    int    `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

func NewDiscordChannel(config *DiscordConfig) (*DiscordChannel, error) {
	if config == nil {
		return nil, fmt.Errorf("Discord配置不能为空")
	}
	if len(config.WebhookURLs) == 0 {
		return nil, fmt.Errorf("Webhook URLs不能为空")
	}

	return &DiscordChannel{
		name:    "discord",
		config:  config,
		enabled: true,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}, nil
}

func (c *DiscordChannel) Name() string {
	return c.name
}

func (c *DiscordChannel) Send(notification *Notification) error {
	c.mu.RLock()
	enabled := c.enabled
	config := *c.config
	c.mu.RUnlock()

	if !enabled {
		return nil
	}

	embed := formatDiscordEmbed(notification)

	var targetWebhooks []string
	if len(notification.Channels) > 0 {
		targetWebhooks = notification.Channels
	} else {
		targetWebhooks = config.WebhookURLs
	}

	if len(targetWebhooks) == 0 {
		return fmt.Errorf("没有目标Webhook URL")
	}

	var lastErr error
	for _, webhookURL := range targetWebhooks {
		if err := c.sendWebhook(webhookURL, config.Username, config.AvatarURL, embed); err != nil {
			logger.Error("发送Discord消息失败",
				zap.String("notification_id", notification.ID),
				zap.String("webhook_url", webhookURL[:50]+"..."),
				zap.Error(err))
			lastErr = err
		} else {
			logger.Info("Discord消息发送成功",
				zap.String("notification_id", notification.ID),
				zap.String("webhook_url", webhookURL[:50]+"..."))
		}
	}

	return lastErr
}

func (c *DiscordChannel) sendWebhook(webhookURL, username, avatarURL string, embed discordEmbed) error {
	reqBody := discordWebhookRequest{
		Username:  username,
		AvatarURL: avatarURL,
		Embeds:    []discordEmbed{embed},
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("序列化请求失败: %w", err)
	}

	req, err := http.NewRequest("POST", webhookURL, bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("发送请求失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("读取响应失败: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var result discordResponse
		_ = json.Unmarshal(respBody, &result)
		if result.Message != "" {
			return fmt.Errorf("Discord API错误: %s", result.Message)
		}
		return fmt.Errorf("Discord API错误: HTTP %d", resp.StatusCode)
	}

	return nil
}

func (c *DiscordChannel) IsEnabled() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.enabled
}

func (c *DiscordChannel) Enable() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.enabled = true
	logger.Info("Discord通知渠道已启用")
}

func (c *DiscordChannel) Disable() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.enabled = false
	logger.Info("Discord通知渠道已禁用")
}

func (c *DiscordChannel) UpdateConfig(config *DiscordConfig) error {
	if config == nil {
		return fmt.Errorf("Discord配置不能为空")
	}
	if len(config.WebhookURLs) == 0 {
		return fmt.Errorf("Webhook URLs不能为空")
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.config = config
	logger.Info("Discord配置已更新")
	return nil
}

func formatDiscordEmbed(notification *Notification) discordEmbed {
	var color int
	switch notification.Type {
	case NotificationTypeInfo:
		color = 0x3B82F6
	case NotificationTypeWarning:
		color = 0xF59E0B
	case NotificationTypeError:
		color = 0xEF4444
	case NotificationTypeSuccess:
		color = 0x10B981
	default:
		color = 0x6B7280
	}

	embed := discordEmbed{
		Title:       notification.Title,
		Description: notification.Message,
		Color:       color,
		Timestamp:   notification.CreatedAt.Format(time.RFC3339),
		Footer: discordEmbedFooter{
			Text: "OKX Quant Trading System",
		},
	}

	fields := []discordEmbedField{
		{
			Name:   "类型",
			Value:  string(notification.Type),
			Inline: true,
		},
		{
			Name:   "优先级",
			Value:  string(notification.Priority),
			Inline: true,
		},
	}

	if len(notification.Metadata) > 0 {
		for key, value := range notification.Metadata {
			fields = append(fields, discordEmbedField{
				Name:   key,
				Value:  value,
				Inline: true,
			})
		}
	}

	embed.Fields = fields
	return embed
}

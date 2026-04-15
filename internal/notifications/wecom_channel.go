package notifications

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/ljwqf/quant/pkg/logger"
	"go.uber.org/zap"
)

// WeComConfig 企业微信机器人配置
type WeComConfig struct {
	WebhookURL string `mapstructure:"webhook_url"`
	MentionedList []string `mapstructure:"mentioned_list"` // @指定用户ID
	MentionedMobileList []string `mapstructure:"mentioned_mobile_list"` // @指定手机号
}

// WeComChannel 企业微信通知渠道
type WeComChannel struct {
	name    string
	config  *WeComConfig
	enabled bool
	client  *http.Client
	mu      sync.RWMutex
}

// weComMessage 企业微信消息结构
type weComMessage struct {
	MsgType string              `json:"msgtype"`
	Markdown weComMarkdownContent `json:"markdown"`
}

type weComMarkdownContent struct {
	Content string `json:"content"`
}

type weComResponse struct {
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
}

// NewWeComChannel 创建企业微信通知渠道
func NewWeComChannel(config *WeComConfig) (*WeComChannel, error) {
	if config == nil {
		return nil, fmt.Errorf("企业微信配置不能为空")
	}
	if config.WebhookURL == "" {
		return nil, fmt.Errorf("企业微信Webhook URL不能为空")
	}

	return &WeComChannel{
		name:    "wecom",
		config:  config,
		enabled: true,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}, nil
}

func (c *WeComChannel) Name() string {
	return c.name
}

func (c *WeComChannel) Send(notification *Notification) error {
	c.mu.RLock()
	enabled := c.enabled
	config := *c.config
	c.mu.RUnlock()

	if !enabled {
		return nil
	}

	msg := formatWeComMessage(notification)

	jsonBody, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("序列化企业微信消息失败: %w", err)
	}

	req, err := http.NewRequest("POST", config.WebhookURL, bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("创建企业微信请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("发送企业微信请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("企业微信API错误: HTTP %d", resp.StatusCode)
	}

	var result weComResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("解析企业微信响应失败: %w", err)
	}
	if result.ErrCode != 0 {
		return fmt.Errorf("企业微信API错误(%d): %s", result.ErrCode, result.ErrMsg)
	}

	logger.Info("企业微信消息发送成功",
		zap.String("notification_id", notification.ID),
		zap.String("channel", c.name))

	return nil
}

func (c *WeComChannel) IsEnabled() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.enabled
}

func (c *WeComChannel) Enable() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.enabled = true
}

func (c *WeComChannel) Disable() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.enabled = false
}

func (c *WeComChannel) UpdateConfig(config *WeComConfig) error {
	if config == nil {
		return fmt.Errorf("企业微信配置不能为空")
	}
	if config.WebhookURL == "" {
		return fmt.Errorf("企业微信Webhook URL不能为空")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.config = config
	return nil
}

func formatWeComMessage(notification *Notification) weComMessage {
	emoji := ""
	switch notification.Type {
	case NotificationTypeInfo:
		emoji = "<font color=\"info\">[INFO]</font>"
	case NotificationTypeWarning:
		emoji = "<font color=\"warning\">[WARN]</font>"
	case NotificationTypeError:
		emoji = "<font color=\"warning\">[ERROR]</font>"
	case NotificationTypeSuccess:
		emoji = "<font color=\"info\">[OK]</font>"
	default:
		emoji = "[MSG]"
	}

	content := fmt.Sprintf("## %s %s\n", emoji, notification.Title)
	content += fmt.Sprintf("> **类型**: %s  \n> **优先级**: %s  \n> **时间**: %s  \n\n",
		notification.Type,
		notification.Priority,
		notification.CreatedAt.Format("2006-01-02 15:04:05"))
	content += fmt.Sprintf("### %s\n\n", notification.Message)

	if len(notification.Metadata) > 0 {
		content += "---\n"
		for k, v := range notification.Metadata {
			content += fmt.Sprintf("- **%s**: %s  \n", k, v)
		}
	}

	return weComMessage{
		MsgType: "markdown",
		Markdown: weComMarkdownContent{
			Content: content,
		},
	}
}

package notifications

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/ljwqf/quant/pkg/logger"
	"go.uber.org/zap"
)

// DingTalkConfig 钉钉机器人配置
type DingTalkConfig struct {
	WebhookURL string `mapstructure:"webhook_url"`
	Secret     string `mapstructure:"secret"` // 加签密钥（可选）
	AtMobiles  []string `mapstructure:"at_mobiles"` // @指定手机号
	AtAll      bool   `mapstructure:"at_all"`
}

// DingTalkChannel 钉钉通知渠道
type DingTalkChannel struct {
	name    string
	config  *DingTalkConfig
	enabled bool
	client  *http.Client
	mu      sync.RWMutex
}

// dingtalkMessage 钉钉消息结构
type dingtalkMessage struct {
	MsgType string               `json:"msgtype"`
	Markdown dingtalkMarkdownContent `json:"markdown"`
	At      dingtalkAtConfig     `json:"at"`
}

type dingtalkMarkdownContent struct {
	Title string `json:"title"`
	Text  string `json:"text"`
}

type dingtalkAtConfig struct {
	AtMobiles []string `json:"atMobiles,omitempty"`
	IsAtAll   bool     `json:"isAtAll"`
}

type dingtalkResponse struct {
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
}

// NewDingTalkChannel 创建钉钉通知渠道
func NewDingTalkChannel(config *DingTalkConfig) (*DingTalkChannel, error) {
	if config == nil {
		return nil, fmt.Errorf("钉钉配置不能为空")
	}
	if config.WebhookURL == "" {
		return nil, fmt.Errorf("钉钉Webhook URL不能为空")
	}

	return &DingTalkChannel{
		name:    "dingtalk",
		config:  config,
		enabled: true,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}, nil
}

func (c *DingTalkChannel) Name() string {
	return c.name
}

func (c *DingTalkChannel) Send(notification *Notification) error {
	c.mu.RLock()
	enabled := c.enabled
	config := *c.config
	c.mu.RUnlock()

	if !enabled {
		return nil
	}

	webhookURL := config.WebhookURL
	if config.Secret != "" {
		webhookURL = c.addSignToURL(webhookURL)
	}

	msg := formatDingTalkMessage(notification, config.AtMobiles, config.AtAll)

	jsonBody, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("序列化钉钉消息失败: %w", err)
	}

	req, err := http.NewRequest("POST", webhookURL, bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("创建钉钉请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("发送钉钉请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("钉钉API错误: HTTP %d", resp.StatusCode)
	}

	var result dingtalkResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("解析钉钉响应失败: %w", err)
	}
	if result.ErrCode != 0 {
		return fmt.Errorf("钉钉API错误(%d): %s", result.ErrCode, result.ErrMsg)
	}

	logger.Info("钉钉消息发送成功",
		zap.String("notification_id", notification.ID),
		zap.String("channel", c.name))

	return nil
}

func (c *DingTalkChannel) IsEnabled() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.enabled
}

func (c *DingTalkChannel) Enable() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.enabled = true
}

func (c *DingTalkChannel) Disable() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.enabled = false
}

func (c *DingTalkChannel) UpdateConfig(config *DingTalkConfig) error {
	if config == nil {
		return fmt.Errorf("钉钉配置不能为空")
	}
	if config.WebhookURL == "" {
		return fmt.Errorf("钉钉Webhook URL不能为空")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.config = config
	return nil
}

func (c *DingTalkChannel) addSignToURL(webhookURL string) string {
	timestamp := time.Now().UnixMilli()
	stringToSign := fmt.Sprintf("%d\n%s", timestamp, c.config.Secret)

	h := hmac.New(sha256.New, []byte(c.config.Secret))
	h.Write([]byte(stringToSign))
	sign := base64.StdEncoding.EncodeToString(h.Sum(nil))

	u, err := url.Parse(webhookURL)
	if err != nil {
		return webhookURL
	}
	q := u.Query()
	q.Set("timestamp", fmt.Sprintf("%d", timestamp))
	q.Set("sign", sign)
	u.RawQuery = q.Encode()
	return u.String()
}

func formatDingTalkMessage(notification *Notification, atMobiles []string, atAll bool) dingtalkMessage {
	emoji := ""
	switch notification.Type {
	case NotificationTypeInfo:
		emoji = "[INFO]"
	case NotificationTypeWarning:
		emoji = "[WARN]"
	case NotificationTypeError:
		emoji = "[ERROR]"
	case NotificationTypeSuccess:
		emoji = "[OK]"
	default:
		emoji = "[MSG]"
	}

	text := fmt.Sprintf("## %s %s\n\n", emoji, notification.Title)
	text += fmt.Sprintf("**类型**: %s  \n", notification.Type)
	text += fmt.Sprintf("**优先级**: %s  \n", notification.Priority)
	text += fmt.Sprintf("**时间**: %s  \n\n", notification.CreatedAt.Format("2006-01-02 15:04:05"))
	text += fmt.Sprintf("### %s\n\n", notification.Message)

	if len(notification.Metadata) > 0 {
		text += "---\n\n"
		for k, v := range notification.Metadata {
			text += fmt.Sprintf("- **%s**: %s  \n", k, v)
		}
	}

	return dingtalkMessage{
		MsgType: "markdown",
		Markdown: dingtalkMarkdownContent{
			Title: notification.Title,
			Text:  text,
		},
		At: dingtalkAtConfig{
			AtMobiles: atMobiles,
			IsAtAll:   atAll,
		},
	}
}

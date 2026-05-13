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

type TelegramConfig struct {
	BotToken string
	ChatIDs  []string
}

type TelegramChannel struct {
	name     string
	config   *TelegramConfig
	enabled  bool
	client   *http.Client
	mu       sync.RWMutex
}

type telegramSendMessageRequest struct {
	ChatID    string `json:"chat_id"`
	Text      string `json:"text"`
	ParseMode string `json:"parse_mode,omitempty"`
}

type telegramResponse struct {
	OK     bool   `json:"ok"`
	Result any    `json:"result,omitempty"`
	Error  string `json:"description,omitempty"`
}

func NewTelegramChannel(config *TelegramConfig) (*TelegramChannel, error) {
	if config == nil {
		return nil, fmt.Errorf("Telegram配置不能为空")
	}
	if config.BotToken == "" {
		return nil, fmt.Errorf("Bot Token不能为空")
	}
	if len(config.ChatIDs) == 0 {
		return nil, fmt.Errorf("Chat IDs不能为空")
	}

	return &TelegramChannel{
		name:    "telegram",
		config:  config,
		enabled: true,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}, nil
}

func (c *TelegramChannel) Name() string {
	return c.name
}

func (c *TelegramChannel) Send(notification *Notification) error {
	c.mu.RLock()
	enabled := c.enabled
	config := *c.config
	c.mu.RUnlock()

	if !enabled {
		return nil
	}

	text := formatTelegramMessage(notification)

	var targetChatIDs []string
	if len(notification.Channels) > 0 {
		targetChatIDs = notification.Channels
	} else {
		targetChatIDs = config.ChatIDs
	}

	if len(targetChatIDs) == 0 {
		return fmt.Errorf("没有目标Chat ID")
	}

	var lastErr error
	for _, chatID := range targetChatIDs {
		if err := c.sendMessage(config.BotToken, chatID, text); err != nil {
			logger.Error("发送Telegram消息失败",
				zap.String("notification_id", notification.ID),
				zap.String("chat_id", chatID),
				zap.Error(err))
			lastErr = err
		} else {
			logger.Info("Telegram消息发送成功",
				zap.String("notification_id", notification.ID),
				zap.String("chat_id", chatID))
		}
	}

	return lastErr
}

func (c *TelegramChannel) sendMessage(botToken, chatID, text string) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)

	reqBody := telegramSendMessageRequest{
		ChatID:    chatID,
		Text:      text,
		ParseMode: "MarkdownV2",
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("序列化请求失败: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(jsonBody))
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

	var result telegramResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return fmt.Errorf("解析响应失败: %w", err)
	}

	if !result.OK {
		return fmt.Errorf("Telegram API错误: %s", result.Error)
	}

	return nil
}

func (c *TelegramChannel) IsEnabled() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.enabled
}

func (c *TelegramChannel) Enable() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.enabled = true
	logger.Info("Telegram通知渠道已启用")
}

func (c *TelegramChannel) Disable() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.enabled = false
	logger.Info("Telegram通知渠道已禁用")
}

func (c *TelegramChannel) UpdateConfig(config *TelegramConfig) error {
	if config == nil {
		return fmt.Errorf("Telegram配置不能为空")
	}
	if config.BotToken == "" {
		return fmt.Errorf("Bot Token不能为空")
	}
	if len(config.ChatIDs) == 0 {
		return fmt.Errorf("Chat IDs不能为空")
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.config = config
	logger.Info("Telegram配置已更新")
	return nil
}

func formatTelegramMessage(notification *Notification) string {
	var emoji string
	switch notification.Type {
	case NotificationTypeInfo:
		emoji = "ℹ️"
	case NotificationTypeWarning:
		emoji = "⚠️"
	case NotificationTypeError:
		emoji = "❌"
	case NotificationTypeSuccess:
		emoji = "✅"
	default:
		emoji = "📢"
	}

	text := fmt.Sprintf("*%s %s*\n\n", emoji, escapeMarkdown(notification.Title))
	text += fmt.Sprintf("%s\n\n", escapeMarkdown(notification.Message))
	text += fmt.Sprintf("*类型*: `%s`\n", notification.Type)
	text += fmt.Sprintf("*优先级*: `%s`\n", notification.Priority)
	text += fmt.Sprintf("*时间*: `%s`", notification.CreatedAt.Format("2006-01-02 15:04:05"))

	if len(notification.Metadata) > 0 {
		text += "\n\n*附加信息*:\n"
		for key, value := range notification.Metadata {
			text += fmt.Sprintf("• %s: `%s`\n", escapeMarkdown(key), escapeMarkdown(value))
		}
	}

	return text
}

func escapeMarkdown(text string) string {
	replacer := func(r rune) rune {
		switch r {
		case '_', '*', '[', ']', '(', ')', '~', '`', '>', '#', '+', '-', '=', '|', '{', '}', '.', '!':
			return '\\'
		}
		return r
	}

	var result string
	for _, r := range text {
		if replacer(r) == '\\' {
			result += "\\" + string(r)
		} else {
			result += string(r)
		}
	}
	return result
}

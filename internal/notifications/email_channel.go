package notifications

import (
	"bytes"
	"fmt"
	"html/template"
	"net/smtp"
	"sync"

	"github.com/ljwqf/quant/pkg/logger"
	"go.uber.org/zap"
)

type EmailConfig struct {
	SMTPHost     string
	SMTPPort     int
	Username     string
	Password     string
	FromEmail    string
	FromName     string
	ToEmails     []string
	UseTLS       bool
}

type EmailChannel struct {
	name     string
	config   *EmailConfig
	enabled  bool
	template *template.Template
	mu       sync.RWMutex
}

func NewEmailChannel(config *EmailConfig) (*EmailChannel, error) {
	if config == nil {
		return nil, fmt.Errorf("邮件配置不能为空")
	}
	if config.SMTPHost == "" {
		return nil, fmt.Errorf("SMTP主机不能为空")
	}
	if config.SMTPPort <= 0 {
		return nil, fmt.Errorf("SMTP端口无效")
	}
	if config.Username == "" {
		return nil, fmt.Errorf("用户名不能为空")
	}
	if config.FromEmail == "" {
		return nil, fmt.Errorf("发件人邮箱不能为空")
	}

	tmpl, err := template.New("email").Parse(defaultEmailTemplate)
	if err != nil {
		return nil, fmt.Errorf("解析邮件模板失败: %w", err)
	}

	return &EmailChannel{
		name:     "email",
		config:   config,
		enabled:  true,
		template: tmpl,
	}, nil
}

func (c *EmailChannel) Name() string {
	return c.name
}

func (c *EmailChannel) Send(notification *Notification) error {
	c.mu.RLock()
	enabled := c.enabled
	config := *c.config
	c.mu.RUnlock()

	if !enabled {
		return nil
	}

	subject := fmt.Sprintf("[%s] %s", notification.Type, notification.Title)

	var body bytes.Buffer
	data := map[string]interface{}{
		"Type":      notification.Type,
		"Priority":  notification.Priority,
		"Title":     notification.Title,
		"Message":   notification.Message,
		"CreatedAt": notification.CreatedAt.Format("2006-01-02 15:04:05"),
		"Metadata":  notification.Metadata,
	}

	if err := c.template.Execute(&body, data); err != nil {
		logger.Error("渲染邮件模板失败", zap.Error(err))
		return err
	}

	auth := smtp.PlainAuth("", config.Username, config.Password, config.SMTPHost)

	from := config.FromEmail
	if config.FromName != "" {
		from = fmt.Sprintf("%s <%s>", config.FromName, config.FromEmail)
	}

	to := config.ToEmails
	if len(notification.Channels) > 0 {
		to = notification.Channels
	}

	if len(to) == 0 {
		return fmt.Errorf("没有收件人")
	}

	msg := buildEmailMessage(from, to, subject, body.String())

	addr := fmt.Sprintf("%s:%d", config.SMTPHost, config.SMTPPort)

	logger.Info("发送邮件",
		zap.String("notification_id", notification.ID),
		zap.String("subject", subject),
		zap.Strings("to", to))

	if err := smtp.SendMail(addr, auth, config.FromEmail, to, msg); err != nil {
		logger.Error("发送邮件失败", zap.Error(err))
		return err
	}

	logger.Info("邮件发送成功",
		zap.String("notification_id", notification.ID),
		zap.Strings("to", to))

	return nil
}

func (c *EmailChannel) IsEnabled() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.enabled
}

func (c *EmailChannel) Enable() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.enabled = true
	logger.Info("邮件通知渠道已启用")
}

func (c *EmailChannel) Disable() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.enabled = false
	logger.Info("邮件通知渠道已禁用")
}

func (c *EmailChannel) UpdateConfig(config *EmailConfig) error {
	if config == nil {
		return fmt.Errorf("邮件配置不能为空")
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.config = config
	logger.Info("邮件配置已更新")
	return nil
}

func (c *EmailChannel) SetTemplate(templateStr string) error {
	tmpl, err := template.New("email").Parse(templateStr)
	if err != nil {
		return fmt.Errorf("解析邮件模板失败: %w", err)
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.template = tmpl
	logger.Info("邮件模板已更新")
	return nil
}

func buildEmailMessage(from string, to []string, subject, body string) []byte {
	var msg bytes.Buffer

	msg.WriteString(fmt.Sprintf("From: %s\r\n", from))
	msg.WriteString(fmt.Sprintf("To: %s\r\n", formatAddresses(to)))
	msg.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	msg.WriteString("MIME-Version: 1.0\r\n")
	msg.WriteString("Content-Type: text/html; charset=\"UTF-8\"\r\n")
	msg.WriteString("\r\n")
	msg.WriteString(body)

	return msg.Bytes()
}

func formatAddresses(emails []string) string {
	if len(emails) == 0 {
		return ""
	}
	if len(emails) == 1 {
		return emails[0]
	}
	result := emails[0]
	for i := 1; i < len(emails); i++ {
		result += ", " + emails[i]
	}
	return result
}

const defaultEmailTemplate = `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>{{.Title}}</title>
    <style>
        body {
            font-family: Arial, sans-serif;
            line-height: 1.6;
            color: #333;
            max-width: 600px;
            margin: 0 auto;
            padding: 20px;
        }
        .header {
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            color: white;
            padding: 20px;
            border-radius: 8px 8px 0 0;
            text-align: center;
        }
        .content {
            background: #f9f9f9;
            padding: 20px;
            border-radius: 0 0 8px 8px;
        }
        .type-badge {
            display: inline-block;
            padding: 4px 12px;
            border-radius: 20px;
            font-size: 12px;
            font-weight: bold;
        }
        .type-info { background: #3b82f6; color: white; }
        .type-warning { background: #f59e0b; color: white; }
        .type-error { background: #ef4444; color: white; }
        .type-success { background: #10b981; color: white; }
        .metadata {
            margin-top: 20px;
            padding: 15px;
            background: #eee;
            border-radius: 4px;
            font-size: 14px;
        }
        .footer {
            margin-top: 30px;
            text-align: center;
            color: #888;
            font-size: 12px;
        }
    </style>
</head>
<body>
    <div class="header">
        <h1>{{.Title}}</h1>
        <div>
            <span class="type-badge type-{{.Type}}">{{.Type}}</span>
            <span style="margin-left: 10px;">优先级: {{.Priority}}</span>
        </div>
    </div>
    <div class="content">
        <p style="font-size: 16px; white-space: pre-wrap;">{{.Message}}</p>
        {{if .Metadata}}
        <div class="metadata">
            <strong>附加信息:</strong>
            <ul>
            {{range $key, $value := .Metadata}}
                <li>{{$key}}: {{$value}}</li>
            {{end}}
            </ul>
        </div>
        {{end}}
    </div>
    <div class="footer">
        <p>发送时间: {{.CreatedAt}}</p>
        <p>OKX Quant Trading System</p>
    </div>
</body>
</html>
`

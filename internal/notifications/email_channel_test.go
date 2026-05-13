package notifications

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEmailChannel(t *testing.T) {
	t.Parallel()

	t.Run("创建邮件渠道 - 正常配置", func(t *testing.T) {
		config := &EmailConfig{
			SMTPHost:  "smtp.example.com",
			SMTPPort:  587,
			Username:  "test@example.com",
			Password:  "password",
			FromEmail: "test@example.com",
			FromName:  "Test User",
			ToEmails:  []string{"recipient@example.com"},
		}

		channel, err := NewEmailChannel(config)
		require.NoError(t, err)
		assert.NotNil(t, channel)
		assert.Equal(t, "email", channel.Name())
		assert.True(t, channel.IsEnabled())
	})

	t.Run("创建邮件渠道 - 空配置", func(t *testing.T) {
		channel, err := NewEmailChannel(nil)
		assert.Error(t, err)
		assert.Nil(t, channel)
	})

	t.Run("创建邮件渠道 - 缺少SMTP主机", func(t *testing.T) {
		config := &EmailConfig{
			SMTPPort:  587,
			Username:  "test@example.com",
			FromEmail: "test@example.com",
		}

		channel, err := NewEmailChannel(config)
		assert.Error(t, err)
		assert.Nil(t, channel)
	})

	t.Run("创建邮件渠道 - 无效端口", func(t *testing.T) {
		config := &EmailConfig{
			SMTPHost:  "smtp.example.com",
			SMTPPort:  0,
			Username:  "test@example.com",
			FromEmail: "test@example.com",
		}

		channel, err := NewEmailChannel(config)
		assert.Error(t, err)
		assert.Nil(t, channel)
	})

	t.Run("创建邮件渠道 - 缺少用户名", func(t *testing.T) {
		config := &EmailConfig{
			SMTPHost:  "smtp.example.com",
			SMTPPort:  587,
			FromEmail: "test@example.com",
		}

		channel, err := NewEmailChannel(config)
		assert.Error(t, err)
		assert.Nil(t, channel)
	})

	t.Run("创建邮件渠道 - 缺少发件人邮箱", func(t *testing.T) {
		config := &EmailConfig{
			SMTPHost: "smtp.example.com",
			SMTPPort: 587,
			Username: "test@example.com",
		}

		channel, err := NewEmailChannel(config)
		assert.Error(t, err)
		assert.Nil(t, channel)
	})

	t.Run("启用和禁用", func(t *testing.T) {
		config := &EmailConfig{
			SMTPHost:  "smtp.example.com",
			SMTPPort:  587,
			Username:  "test@example.com",
			FromEmail: "test@example.com",
		}

		channel, err := NewEmailChannel(config)
		require.NoError(t, err)

		assert.True(t, channel.IsEnabled())

		channel.Disable()
		assert.False(t, channel.IsEnabled())

		channel.Enable()
		assert.True(t, channel.IsEnabled())
	})

	t.Run("更新配置", func(t *testing.T) {
		config := &EmailConfig{
			SMTPHost:  "smtp.example.com",
			SMTPPort:  587,
			Username:  "test@example.com",
			FromEmail: "test@example.com",
		}

		channel, err := NewEmailChannel(config)
		require.NoError(t, err)

		newConfig := &EmailConfig{
			SMTPHost:  "smtp.new.com",
			SMTPPort:  465,
			Username:  "new@example.com",
			FromEmail: "new@example.com",
		}

		err = channel.UpdateConfig(newConfig)
		assert.NoError(t, err)
	})

	t.Run("更新配置 - 空配置", func(t *testing.T) {
		config := &EmailConfig{
			SMTPHost:  "smtp.example.com",
			SMTPPort:  587,
			Username:  "test@example.com",
			FromEmail: "test@example.com",
		}

		channel, err := NewEmailChannel(config)
		require.NoError(t, err)

		err = channel.UpdateConfig(nil)
		assert.Error(t, err)
	})

	t.Run("设置模板", func(t *testing.T) {
		config := &EmailConfig{
			SMTPHost:  "smtp.example.com",
			SMTPPort:  587,
			Username:  "test@example.com",
			FromEmail: "test@example.com",
		}

		channel, err := NewEmailChannel(config)
		require.NoError(t, err)

		customTemplate := `
<!DOCTYPE html>
<html>
<head>
    <title>{{.Title}}</title>
</head>
<body>
    <h1>{{.Title}}</h1>
    <p>{{.Message}}</p>
</body>
</html>`

		err = channel.SetTemplate(customTemplate)
		assert.NoError(t, err)
	})

	t.Run("设置模板 - 无效模板", func(t *testing.T) {
		config := &EmailConfig{
			SMTPHost:  "smtp.example.com",
			SMTPPort:  587,
			Username:  "test@example.com",
			FromEmail: "test@example.com",
		}

		channel, err := NewEmailChannel(config)
		require.NoError(t, err)

		invalidTemplate := `{{.Invalid`
		err = channel.SetTemplate(invalidTemplate)
		assert.Error(t, err)
	})

	t.Run("禁用时不发送", func(t *testing.T) {
		config := &EmailConfig{
			SMTPHost:  "smtp.example.com",
			SMTPPort:  587,
			Username:  "test@example.com",
			FromEmail: "test@example.com",
			ToEmails:  []string{"recipient@example.com"},
		}

		channel, err := NewEmailChannel(config)
		require.NoError(t, err)

		channel.Disable()

		notification := NewInfoNotification("测试", "测试消息")
		err = channel.Send(notification)
		assert.NoError(t, err)
	})
}

func TestFormatAddresses(t *testing.T) {
	t.Parallel()

	t.Run("空列表", func(t *testing.T) {
		result := formatAddresses([]string{})
		assert.Empty(t, result)
	})

	t.Run("单个地址", func(t *testing.T) {
		result := formatAddresses([]string{"test@example.com"})
		assert.Equal(t, "test@example.com", result)
	})

	t.Run("多个地址", func(t *testing.T) {
		result := formatAddresses([]string{"a@example.com", "b@example.com", "c@example.com"})
		assert.Equal(t, "a@example.com, b@example.com, c@example.com", result)
	})
}

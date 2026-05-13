package notifications

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTelegramChannel(t *testing.T) {
	t.Parallel()

	t.Run("创建Telegram渠道 - 正常配置", func(t *testing.T) {
		config := &TelegramConfig{
			BotToken: "test-bot-token",
			ChatIDs:  []string{"123456789", "987654321"},
		}

		channel, err := NewTelegramChannel(config)
		require.NoError(t, err)
		assert.NotNil(t, channel)
		assert.Equal(t, "telegram", channel.Name())
		assert.True(t, channel.IsEnabled())
	})

	t.Run("创建Telegram渠道 - 空配置", func(t *testing.T) {
		channel, err := NewTelegramChannel(nil)
		assert.Error(t, err)
		assert.Nil(t, channel)
	})

	t.Run("创建Telegram渠道 - 缺少Bot Token", func(t *testing.T) {
		config := &TelegramConfig{
			ChatIDs: []string{"123456789"},
		}

		channel, err := NewTelegramChannel(config)
		assert.Error(t, err)
		assert.Nil(t, channel)
	})

	t.Run("创建Telegram渠道 - 缺少Chat IDs", func(t *testing.T) {
		config := &TelegramConfig{
			BotToken: "test-bot-token",
		}

		channel, err := NewTelegramChannel(config)
		assert.Error(t, err)
		assert.Nil(t, channel)
	})

	t.Run("启用和禁用", func(t *testing.T) {
		config := &TelegramConfig{
			BotToken: "test-bot-token",
			ChatIDs:  []string{"123456789"},
		}

		channel, err := NewTelegramChannel(config)
		require.NoError(t, err)

		assert.True(t, channel.IsEnabled())

		channel.Disable()
		assert.False(t, channel.IsEnabled())

		channel.Enable()
		assert.True(t, channel.IsEnabled())
	})

	t.Run("更新配置", func(t *testing.T) {
		config := &TelegramConfig{
			BotToken: "test-bot-token",
			ChatIDs:  []string{"123456789"},
		}

		channel, err := NewTelegramChannel(config)
		require.NoError(t, err)

		newConfig := &TelegramConfig{
			BotToken: "new-bot-token",
			ChatIDs:  []string{"987654321"},
		}

		err = channel.UpdateConfig(newConfig)
		assert.NoError(t, err)
	})

	t.Run("更新配置 - 空配置", func(t *testing.T) {
		config := &TelegramConfig{
			BotToken: "test-bot-token",
			ChatIDs:  []string{"123456789"},
		}

		channel, err := NewTelegramChannel(config)
		require.NoError(t, err)

		err = channel.UpdateConfig(nil)
		assert.Error(t, err)
	})

	t.Run("更新配置 - 缺少Bot Token", func(t *testing.T) {
		config := &TelegramConfig{
			BotToken: "test-bot-token",
			ChatIDs:  []string{"123456789"},
		}

		channel, err := NewTelegramChannel(config)
		require.NoError(t, err)

		newConfig := &TelegramConfig{
			ChatIDs: []string{"987654321"},
		}

		err = channel.UpdateConfig(newConfig)
		assert.Error(t, err)
	})

	t.Run("禁用时不发送", func(t *testing.T) {
		config := &TelegramConfig{
			BotToken: "test-bot-token",
			ChatIDs:  []string{"123456789"},
		}

		channel, err := NewTelegramChannel(config)
		require.NoError(t, err)

		channel.Disable()

		notification := NewInfoNotification("测试", "测试消息")
		err = channel.Send(notification)
		assert.NoError(t, err)
	})
}

func TestFormatTelegramMessage(t *testing.T) {
	t.Parallel()

	t.Run("格式化Info消息", func(t *testing.T) {
		notification := NewInfoNotification("测试标题", "测试消息内容")
		text := formatTelegramMessage(notification)

		assert.Contains(t, text, "测试标题")
		assert.Contains(t, text, "测试消息内容")
		assert.Contains(t, text, "ℹ️")
	})

	t.Run("格式化Warning消息", func(t *testing.T) {
		notification := NewWarningNotification("警告", "警告内容")
		text := formatTelegramMessage(notification)

		assert.Contains(t, text, "⚠️")
	})

	t.Run("格式化Error消息", func(t *testing.T) {
		notification := NewErrorNotification("错误", "错误内容")
		text := formatTelegramMessage(notification)

		assert.Contains(t, text, "❌")
	})

	t.Run("格式化Success消息", func(t *testing.T) {
		notification := NewSuccessNotification("成功", "成功内容")
		text := formatTelegramMessage(notification)

		assert.Contains(t, text, "✅")
	})

	t.Run("包含元数据", func(t *testing.T) {
		notification := NewNotification().
			WithTitle("测试").
			WithMessage("消息").
			WithMetadata("key1", "value1").
			WithMetadata("key2", "value2").
			Build()

		text := formatTelegramMessage(notification)

		assert.Contains(t, text, "key1")
		assert.Contains(t, text, "value1")
		assert.Contains(t, text, "key2")
		assert.Contains(t, text, "value2")
	})
}

func TestEscapeMarkdown(t *testing.T) {
	t.Parallel()

	t.Run("转义特殊字符", func(t *testing.T) {
		input := "Hello *world* _test_ [link](url)"
		result := escapeMarkdown(input)

		assert.Contains(t, result, `\*`)
		assert.Contains(t, result, `\_`)
		assert.Contains(t, result, `\[`)
		assert.Contains(t, result, `\]`)
		assert.Contains(t, result, `\(`)
		assert.Contains(t, result, `\)`)
	})

	t.Run("普通文本不转义", func(t *testing.T) {
		input := "Hello world 123"
		result := escapeMarkdown(input)

		assert.Equal(t, input, result)
	})
}

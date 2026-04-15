package notifications

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiscordChannel(t *testing.T) {
	t.Parallel()

	t.Run("创建Discord渠道 - 正常配置", func(t *testing.T) {
		config := &DiscordConfig{
			WebhookURLs: []string{"https://discord.com/api/webhooks/123/abc"},
			Username:    "Quant Bot",
			AvatarURL:   "https://example.com/avatar.png",
		}

		channel, err := NewDiscordChannel(config)
		require.NoError(t, err)
		assert.NotNil(t, channel)
		assert.Equal(t, "discord", channel.Name())
		assert.True(t, channel.IsEnabled())
	})

	t.Run("创建Discord渠道 - 空配置", func(t *testing.T) {
		channel, err := NewDiscordChannel(nil)
		assert.Error(t, err)
		assert.Nil(t, channel)
	})

	t.Run("创建Discord渠道 - 缺少Webhook URLs", func(t *testing.T) {
		config := &DiscordConfig{
			Username: "Quant Bot",
		}

		channel, err := NewDiscordChannel(config)
		assert.Error(t, err)
		assert.Nil(t, channel)
	})

	t.Run("启用和禁用", func(t *testing.T) {
		config := &DiscordConfig{
			WebhookURLs: []string{"https://discord.com/api/webhooks/123/abc"},
		}

		channel, err := NewDiscordChannel(config)
		require.NoError(t, err)

		assert.True(t, channel.IsEnabled())

		channel.Disable()
		assert.False(t, channel.IsEnabled())

		channel.Enable()
		assert.True(t, channel.IsEnabled())
	})

	t.Run("更新配置", func(t *testing.T) {
		config := &DiscordConfig{
			WebhookURLs: []string{"https://discord.com/api/webhooks/123/abc"},
		}

		channel, err := NewDiscordChannel(config)
		require.NoError(t, err)

		newConfig := &DiscordConfig{
			WebhookURLs: []string{"https://discord.com/api/webhooks/456/def"},
		}

		err = channel.UpdateConfig(newConfig)
		assert.NoError(t, err)
	})

	t.Run("更新配置 - 空配置", func(t *testing.T) {
		config := &DiscordConfig{
			WebhookURLs: []string{"https://discord.com/api/webhooks/123/abc"},
		}

		channel, err := NewDiscordChannel(config)
		require.NoError(t, err)

		err = channel.UpdateConfig(nil)
		assert.Error(t, err)
	})

	t.Run("更新配置 - 缺少Webhook URLs", func(t *testing.T) {
		config := &DiscordConfig{
			WebhookURLs: []string{"https://discord.com/api/webhooks/123/abc"},
		}

		channel, err := NewDiscordChannel(config)
		require.NoError(t, err)

		newConfig := &DiscordConfig{
			Username: "New Bot",
		}

		err = channel.UpdateConfig(newConfig)
		assert.Error(t, err)
	})

	t.Run("禁用时不发送", func(t *testing.T) {
		config := &DiscordConfig{
			WebhookURLs: []string{"https://discord.com/api/webhooks/123/abc"},
		}

		channel, err := NewDiscordChannel(config)
		require.NoError(t, err)

		channel.Disable()

		notification := NewInfoNotification("测试", "测试消息")
		err = channel.Send(notification)
		assert.NoError(t, err)
	})
}

func TestFormatDiscordEmbed(t *testing.T) {
	t.Parallel()

	t.Run("格式化Info消息", func(t *testing.T) {
		notification := NewInfoNotification("测试标题", "测试消息内容")
		embed := formatDiscordEmbed(notification)

		assert.Equal(t, "测试标题", embed.Title)
		assert.Equal(t, "测试消息内容", embed.Description)
		assert.Equal(t, 0x3B82F6, embed.Color)
	})

	t.Run("格式化Warning消息", func(t *testing.T) {
		notification := NewWarningNotification("警告", "警告内容")
		embed := formatDiscordEmbed(notification)

		assert.Equal(t, 0xF59E0B, embed.Color)
	})

	t.Run("格式化Error消息", func(t *testing.T) {
		notification := NewErrorNotification("错误", "错误内容")
		embed := formatDiscordEmbed(notification)

		assert.Equal(t, 0xEF4444, embed.Color)
	})

	t.Run("格式化Success消息", func(t *testing.T) {
		notification := NewSuccessNotification("成功", "成功内容")
		embed := formatDiscordEmbed(notification)

		assert.Equal(t, 0x10B981, embed.Color)
	})

	t.Run("包含类型和优先级字段", func(t *testing.T) {
		notification := NewInfoNotification("测试", "消息")
		embed := formatDiscordEmbed(notification)

		assert.Len(t, embed.Fields, 2)
		assert.Equal(t, "类型", embed.Fields[0].Name)
		assert.Equal(t, "info", embed.Fields[0].Value)
		assert.Equal(t, "优先级", embed.Fields[1].Name)
		assert.Equal(t, "medium", embed.Fields[1].Value)
	})

	t.Run("包含元数据", func(t *testing.T) {
		notification := NewNotification().
			WithTitle("测试").
			WithMessage("消息").
			WithMetadata("key1", "value1").
			WithMetadata("key2", "value2").
			Build()

		embed := formatDiscordEmbed(notification)

		assert.Len(t, embed.Fields, 4)
		// Map iteration order is non-deterministic, check that both metadata fields exist
		metadataFields := embed.Fields[2:]
		names := make(map[string]string)
		for _, f := range metadataFields {
			names[f.Name] = f.Value
		}
		assert.Equal(t, "value1", names["key1"])
		assert.Equal(t, "value2", names["key2"])
	})

	t.Run("包含Footer", func(t *testing.T) {
		notification := NewInfoNotification("测试", "消息")
		embed := formatDiscordEmbed(notification)

		assert.Equal(t, "OKX Quant Trading System", embed.Footer.Text)
	})

	t.Run("包含时间戳", func(t *testing.T) {
		notification := NewInfoNotification("测试", "消息")
		embed := formatDiscordEmbed(notification)

		assert.NotEmpty(t, embed.Timestamp)
	})
}

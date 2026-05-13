package notifications

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDingTalkChannelNilConfig(t *testing.T) {
	_, err := NewDingTalkChannel(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "配置不能为空")
}

func TestNewDingTalkChannelEmptyWebhook(t *testing.T) {
	_, err := NewDingTalkChannel(&DingTalkConfig{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Webhook URL不能为空")
}

func TestNewDingTalkChannelValid(t *testing.T) {
	ch, err := NewDingTalkChannel(&DingTalkConfig{
		WebhookURL: "https://oapi.dingtalk.com/robot/send?access_token=test",
	})
	require.NoError(t, err)
	require.NotNil(t, ch)
	assert.Equal(t, "dingtalk", ch.Name())
	assert.True(t, ch.IsEnabled())
}

func TestDingTalkChannelEnableDisable(t *testing.T) {
	ch, err := NewDingTalkChannel(&DingTalkConfig{
		WebhookURL: "https://oapi.dingtalk.com/robot/send?access_token=test",
	})
	require.NoError(t, err)

	ch.Disable()
	assert.False(t, ch.IsEnabled())

	ch.Enable()
	assert.True(t, ch.IsEnabled())
}

func TestDingTalkChannelUpdateConfig(t *testing.T) {
	ch, err := NewDingTalkChannel(&DingTalkConfig{
		WebhookURL: "https://oapi.dingtalk.com/robot/send?access_token=old",
	})
	require.NoError(t, err)

	err = ch.UpdateConfig(&DingTalkConfig{
		WebhookURL: "https://oapi.dingtalk.com/robot/send?access_token=new",
		Secret:     "new-secret",
	})
	require.NoError(t, err)
}

func TestDingTalkSendDisabled(t *testing.T) {
	ch, _ := NewDingTalkChannel(&DingTalkConfig{
		WebhookURL: "https://oapi.dingtalk.com/robot/send?access_token=test",
	})
	ch.Disable()

	// Should not error when disabled, just return nil
	err := ch.Send(&Notification{
		ID:        "test-1",
		Type:      NotificationTypeInfo,
		Priority:  NotificationPriorityLow,
		Title:     "Test",
		Message:   "Test message",
		CreatedAt: time.Now(),
	})
	assert.NoError(t, err)
}

func TestDingTalkChannelAddSignToURL(t *testing.T) {
	ch, _ := NewDingTalkChannel(&DingTalkConfig{
		WebhookURL: "https://oapi.dingtalk.com/robot/send?access_token=test",
		Secret:     "my-secret",
	})

	signedURL := ch.addSignToURL(ch.config.WebhookURL)
	assert.Contains(t, signedURL, "timestamp=")
	assert.Contains(t, signedURL, "sign=")
	assert.NotEqual(t, ch.config.WebhookURL, signedURL)
}

func TestFormatDingTalkMessage(t *testing.T) {
	notification := &Notification{
		ID:        "test-1",
		Type:      NotificationTypeWarning,
		Priority:  NotificationPriorityHigh,
		Title:     "价格告警",
		Message:   "BTC突破50000",
		CreatedAt: time.Now(),
		Metadata:  map[string]string{"symbol": "BTC-USDT"},
	}

	msg := formatDingTalkMessage(notification, nil, false)
	assert.Equal(t, "markdown", msg.MsgType)
	assert.Equal(t, "价格告警", msg.Markdown.Title)
	assert.Contains(t, msg.Markdown.Text, "价格告警")
	assert.Contains(t, msg.Markdown.Text, "BTC突破50000")
	assert.Contains(t, msg.Markdown.Text, "[WARN]")
}

// WeCom tests

func TestNewWeComChannelNilConfig(t *testing.T) {
	_, err := NewWeComChannel(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "配置不能为空")
}

func TestNewWeComChannelEmptyWebhook(t *testing.T) {
	_, err := NewWeComChannel(&WeComConfig{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Webhook URL不能为空")
}

func TestNewWeComChannelValid(t *testing.T) {
	ch, err := NewWeComChannel(&WeComConfig{
		WebhookURL: "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=test",
	})
	require.NoError(t, err)
	require.NotNil(t, ch)
	assert.Equal(t, "wecom", ch.Name())
	assert.True(t, ch.IsEnabled())
}

func TestWeComChannelEnableDisable(t *testing.T) {
	ch, _ := NewWeComChannel(&WeComConfig{
		WebhookURL: "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=test",
	})

	ch.Disable()
	assert.False(t, ch.IsEnabled())

	ch.Enable()
	assert.True(t, ch.IsEnabled())
}

func TestWeComSendDisabled(t *testing.T) {
	ch, _ := NewWeComChannel(&WeComConfig{
		WebhookURL: "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=test",
	})
	ch.Disable()

	err := ch.Send(&Notification{
		ID:        "test-2",
		Type:      NotificationTypeError,
		Priority:  NotificationPriorityUrgent,
		Title:     "系统错误",
		Message:   "数据库连接失败",
		CreatedAt: time.Now(),
	})
	assert.NoError(t, err)
}

func TestFormatWeComMessage(t *testing.T) {
	notification := &Notification{
		ID:        "test-3",
		Type:      NotificationTypeError,
		Priority:  NotificationPriorityUrgent,
		Title:     "系统错误",
		Message:   "数据库连接失败",
		CreatedAt: time.Now(),
	}

	msg := formatWeComMessage(notification)
	assert.Equal(t, "markdown", msg.MsgType)
	assert.Contains(t, msg.Markdown.Content, "系统错误")
	assert.Contains(t, msg.Markdown.Content, "数据库连接失败")
	assert.Contains(t, msg.Markdown.Content, "[ERROR]")
}

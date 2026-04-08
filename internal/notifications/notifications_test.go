package notifications

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNotificationBuilder(t *testing.T) {
	t.Parallel()

	t.Run("创建默认通知", func(t *testing.T) {
		notification := NewNotification().Build()

		assert.NotEmpty(t, notification.ID)
		assert.Equal(t, NotificationTypeInfo, notification.Type)
		assert.Equal(t, NotificationPriorityMedium, notification.Priority)
		assert.NotZero(t, notification.CreatedAt)
	})

	t.Run("构建完整通知", func(t *testing.T) {
		notification := NewNotification().
			WithID("test-id").
			WithType(NotificationTypeSuccess).
			WithPriority(NotificationPriorityHigh).
			WithTitle("测试标题").
			WithMessage("测试消息").
			WithChannels("email", "telegram").
			WithMetadata("key1", "value1").
			WithMetadata("key2", "value2").
			Build()

		assert.Equal(t, "test-id", notification.ID)
		assert.Equal(t, NotificationTypeSuccess, notification.Type)
		assert.Equal(t, NotificationPriorityHigh, notification.Priority)
		assert.Equal(t, "测试标题", notification.Title)
		assert.Equal(t, "测试消息", notification.Message)
		assert.Equal(t, []string{"email", "telegram"}, notification.Channels)
		assert.Equal(t, "value1", notification.Metadata["key1"])
		assert.Equal(t, "value2", notification.Metadata["key2"])
	})

	t.Run("便捷创建函数", func(t *testing.T) {
		info := NewInfoNotification("Info", "Info message")
		warning := NewWarningNotification("Warning", "Warning message")
		err := NewErrorNotification("Error", "Error message")
		success := NewSuccessNotification("Success", "Success message")

		assert.Equal(t, NotificationTypeInfo, info.Type)
		assert.Equal(t, NotificationTypeWarning, warning.Type)
		assert.Equal(t, NotificationTypeError, err.Type)
		assert.Equal(t, NotificationTypeSuccess, success.Type)
	})
}

func TestNotificationManager(t *testing.T) {
	t.Parallel()

	t.Run("创建管理器", func(t *testing.T) {
		manager := NewNotificationManager(nil)
		assert.NotNil(t, manager)
	})

	t.Run("注册和注销渠道", func(t *testing.T) {
		manager := NewNotificationManager(nil)
		channel := NewConsoleChannel()

		manager.RegisterChannel(channel)
		channels := manager.ListChannels()
		assert.Len(t, channels, 1)
		assert.Contains(t, channels, "console")

		retrieved, exists := manager.GetChannel("console")
		assert.True(t, exists)
		assert.Equal(t, "console", retrieved.Name())

		manager.UnregisterChannel("console")
		channels = manager.ListChannels()
		assert.Len(t, channels, 0)
	})

	t.Run("发送通知", func(t *testing.T) {
		manager := NewNotificationManager(nil)
		channel := NewConsoleChannel()
		manager.RegisterChannel(channel)

		notification := NewInfoNotification("测试", "测试消息")
		results := manager.Send(notification)

		assert.Len(t, results, 1)
		assert.Equal(t, notification.ID, results[0].NotificationID)
		assert.Equal(t, "console", results[0].Channel)
		assert.True(t, results[0].Success)
	})

	t.Run("指定渠道发送", func(t *testing.T) {
		manager := NewNotificationManager(nil)
		channel1 := NewConsoleChannel()
		manager.RegisterChannel(channel1)

		notification := NewInfoNotification("测试", "测试消息")
		notification.Channels = []string{"console", "nonexistent"}
		results := manager.Send(notification)

		assert.Len(t, results, 1)
		assert.Equal(t, "console", results[0].Channel)
	})

	t.Run("异步发送通知", func(t *testing.T) {
		manager := NewNotificationManager(nil)
		channel := NewConsoleChannel()
		manager.RegisterChannel(channel)

		notification := NewInfoNotification("测试", "测试消息")
		resultChan := manager.SendAsync(notification)

		results := <-resultChan
		assert.Len(t, results, 1)
		assert.True(t, results[0].Success)
	})

	t.Run("获取通知结果", func(t *testing.T) {
		manager := NewNotificationManager(nil)
		channel := NewConsoleChannel()
		manager.RegisterChannel(channel)

		notification := NewInfoNotification("测试", "测试消息")
		manager.Send(notification)

		results := manager.GetResults()
		assert.Len(t, results, 1)

		byNotification := manager.GetResultsByNotification(notification.ID)
		assert.Len(t, byNotification, 1)

		byChannel := manager.GetResultsByChannel("console")
		assert.Len(t, byChannel, 1)

		manager.ClearResults()
		results = manager.GetResults()
		assert.Len(t, results, 0)
	})
}

func TestNotificationQueue(t *testing.T) {
	t.Parallel()

	t.Run("创建队列", func(t *testing.T) {
		manager := NewNotificationManager(nil)
		queue := NewNotificationQueue(manager, nil)
		assert.NotNil(t, queue)
		assert.False(t, queue.IsRunning())
	})

	t.Run("启动和停止队列", func(t *testing.T) {
		manager := NewNotificationManager(nil)
		queue := NewNotificationQueue(manager, nil)

		queue.Start()
		assert.True(t, queue.IsRunning())

		queue.Stop()
		assert.False(t, queue.IsRunning())
	})

	t.Run("入队和处理通知", func(t *testing.T) {
		manager := NewNotificationManager(nil)
		channel := NewConsoleChannel()
		manager.RegisterChannel(channel)

		queue := NewNotificationQueue(manager, &NotificationQueueConfig{
			QueueSize:   10,
			WorkerCount: 2,
		})

		queue.Start()
		defer queue.Stop()

		notification := NewInfoNotification("测试", "测试消息")
		err := queue.Enqueue(notification)
		require.NoError(t, err)

		time.Sleep(100 * time.Millisecond)

		results := manager.GetResults()
		assert.Len(t, results, 1)
	})

	t.Run("队列满时返回错误", func(t *testing.T) {
		manager := NewNotificationManager(nil)
		queue := NewNotificationQueue(manager, &NotificationQueueConfig{
			QueueSize: 2,
		})

		queue.mu.Lock()
		queue.isRunning = true
		queue.mu.Unlock()

		err := queue.Enqueue(NewInfoNotification("1", "1"))
		require.NoError(t, err)

		err = queue.Enqueue(NewInfoNotification("2", "2"))
		require.NoError(t, err)

		err = queue.Enqueue(NewInfoNotification("3", "3"))
		assert.Error(t, err)
		assert.Equal(t, ErrQueueFull, err)
	})

	t.Run("未启动时入队返回错误", func(t *testing.T) {
		manager := NewNotificationManager(nil)
		queue := NewNotificationQueue(manager, nil)

		err := queue.Enqueue(NewInfoNotification("测试", "测试消息"))
		assert.Error(t, err)
		assert.Equal(t, ErrQueueNotRunning, err)
	})
}

func TestConsoleChannel(t *testing.T) {
	t.Parallel()

	t.Run("创建控制台渠道", func(t *testing.T) {
		channel := NewConsoleChannel()
		assert.Equal(t, "console", channel.Name())
		assert.True(t, channel.IsEnabled())
	})

	t.Run("启用和禁用", func(t *testing.T) {
		channel := NewConsoleChannel()

		channel.Disable()
		assert.False(t, channel.IsEnabled())

		channel.Enable()
		assert.True(t, channel.IsEnabled())
	})

	t.Run("发送通知", func(t *testing.T) {
		channel := NewConsoleChannel()

		notification := NewInfoNotification("测试", "测试消息")
		err := channel.Send(notification)
		assert.NoError(t, err)
	})

	t.Run("禁用时不发送", func(t *testing.T) {
		channel := NewConsoleChannel()
		channel.Disable()

		notification := NewInfoNotification("测试", "测试消息")
		err := channel.Send(notification)
		assert.NoError(t, err)
	})
}

package notifications

import (
	"context"
	"sync"
	"time"

	"github.com/ljwqf/quant/pkg/logger"
	"go.uber.org/zap"
)

type NotificationQueue struct {
	queue     chan *Notification
	manager   *NotificationManager
	wg        sync.WaitGroup
	ctx       context.Context
	cancel    context.CancelFunc
	workers   int
	isRunning bool
	mu        sync.Mutex
}

type NotificationQueueConfig struct {
	QueueSize   int
	WorkerCount int
}

func NewNotificationQueue(manager *NotificationManager, config *NotificationQueueConfig) *NotificationQueue {
	if config == nil {
		config = &NotificationQueueConfig{
			QueueSize:   1000,
			WorkerCount: 4,
		}
	}
	if config.QueueSize <= 0 {
		config.QueueSize = 1000
	}
	if config.WorkerCount <= 0 {
		config.WorkerCount = 4
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &NotificationQueue{
		queue:   make(chan *Notification, config.QueueSize),
		manager: manager,
		ctx:     ctx,
		cancel:  cancel,
		workers: config.WorkerCount,
	}
}

func (q *NotificationQueue) Start() {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.isRunning {
		logger.Warn("通知队列已在运行中")
		return
	}

	q.isRunning = true
	logger.Info("启动通知队列", zap.Int("workers", q.workers))

	for i := 0; i < q.workers; i++ {
		q.wg.Add(1)
		go q.worker(i)
	}
}

func (q *NotificationQueue) Stop() {
	q.mu.Lock()
	defer q.mu.Unlock()

	if !q.isRunning {
		logger.Warn("通知队列未在运行")
		return
	}

	logger.Info("停止通知队列")
	q.cancel()
	close(q.queue)
	q.wg.Wait()
	q.isRunning = false
	logger.Info("通知队列已停止")
}

func (q *NotificationQueue) Enqueue(notification *Notification) error {
	q.mu.Lock()
	isRunning := q.isRunning
	q.mu.Unlock()

	if !isRunning {
		return ErrQueueNotRunning
	}

	select {
	case q.queue <- notification:
		logger.Debug("通知已加入队列", zap.String("notification_id", notification.ID))
		return nil
	case <-q.ctx.Done():
		return ErrQueueStopped
	default:
		return ErrQueueFull
	}
}

func (q *NotificationQueue) EnqueueWithTimeout(notification *Notification, timeout time.Duration) error {
	q.mu.Lock()
	isRunning := q.isRunning
	q.mu.Unlock()

	if !isRunning {
		return ErrQueueNotRunning
	}

	select {
	case q.queue <- notification:
		logger.Debug("通知已加入队列", zap.String("notification_id", notification.ID))
		return nil
	case <-time.After(timeout):
		return ErrQueueTimeout
	case <-q.ctx.Done():
		return ErrQueueStopped
	}
}

func (q *NotificationQueue) QueueSize() int {
	return len(q.queue)
}

func (q *NotificationQueue) IsRunning() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.isRunning
}

func (q *NotificationQueue) worker(id int) {
	defer q.wg.Done()
	logger.Debug("通知工作线程启动", zap.Int("worker_id", id))

	for {
		select {
		case notification, ok := <-q.queue:
			if !ok {
				logger.Debug("通知队列已关闭，工作线程退出", zap.Int("worker_id", id))
				return
			}

			logger.Debug("工作线程处理通知",
				zap.Int("worker_id", id),
				zap.String("notification_id", notification.ID))

			q.manager.Send(notification)

		case <-q.ctx.Done():
			logger.Debug("收到停止信号，工作线程退出", zap.Int("worker_id", id))
			return
		}
	}
}

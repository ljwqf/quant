package api

import (
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// 默认发送缓冲区大小
	defaultSendBufferSize = 256
	// 默认写入超时
	defaultWriteTimeout = 10 * time.Second
	// 默认心跳间隔
	defaultHeartbeatInterval = 30 * time.Second
)

// ConnectionStatus 连接状态
type ConnectionStatus string

const (
	// ConnectionStatusConnected 已连接
	ConnectionStatusConnected ConnectionStatus = "connected"
	// ConnectionStatusDisconnected 已断开
	ConnectionStatusDisconnected ConnectionStatus = "disconnected"
	// ConnectionStatusReconnecting 重连中
	ConnectionStatusReconnecting ConnectionStatus = "reconnecting"
)

// WSClient WebSocket客户端封装
type WSClient struct {
	conn         *websocket.Conn
	subscriptions map[EventType]bool
	sendCh       chan []byte
	done         chan struct{}
	mutex        sync.RWMutex

	// 连接状态信息
	connectedAt    time.Time
	status         ConnectionStatus
	messageSent    int64
	messageReceived int64
	lastActivity   time.Time
	clientID       string
}

// NewWSClient 创建新的WebSocket客户端
func NewWSClient(conn *websocket.Conn) *WSClient {
	now := time.Now()
	return &WSClient{
		conn:            conn,
		subscriptions:    make(map[EventType]bool),
		sendCh:          make(chan []byte, defaultSendBufferSize),
		done:            make(chan struct{}),
		connectedAt:     now,
		status:          ConnectionStatusConnected,
		lastActivity:    now,
		clientID:        generateClientID(),
	}
}

// generateClientID 生成唯一的客户端ID
func generateClientID() string {
	return "client_" + time.Now().Format("20060102150405")
}

// GetClientID 获取客户端ID
func (c *WSClient) GetClientID() string {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.clientID
}

// GetStatus 获取连接状态
func (c *WSClient) GetStatus() ConnectionStatus {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.status
}

// GetConnectionStats 获取连接统计信息
func (c *WSClient) GetConnectionStats() (time.Time, int64, int64, time.Time) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.connectedAt, c.messageSent, c.messageReceived, c.lastActivity
}

// SetStatus 设置连接状态
func (c *WSClient) SetStatus(status ConnectionStatus) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.status = status
}

// RecordMessageSent 记录发送的消息
func (c *WSClient) RecordMessageSent() {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.messageSent++
	c.lastActivity = time.Now()
}

// RecordMessageReceived 记录接收的消息
func (c *WSClient) RecordMessageReceived() {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.messageReceived++
	c.lastActivity = time.Now()
}

// Subscribe 订阅事件类型
func (c *WSClient) Subscribe(events []EventType) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	for _, event := range events {
		c.subscriptions[event] = true
	}
}

// Unsubscribe 取消订阅事件类型
func (c *WSClient) Unsubscribe(events []EventType) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	for _, event := range events {
		delete(c.subscriptions, event)
	}
}

// IsSubscribed 检查是否订阅了指定事件类型
// 如果没有订阅任何事件，则默认订阅所有事件
func (c *WSClient) IsSubscribed(eventType EventType) bool {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	// 如果没有订阅任何事件，默认订阅所有
	if len(c.subscriptions) == 0 {
		return true
	}

	return c.subscriptions[eventType]
}

// Send 发送消息到客户端
func (c *WSClient) Send(message []byte) error {
	select {
	case c.sendCh <- message:
		return nil
	default:
		// 缓冲区满，丢弃消息
		return nil
	}
}

// SendBlocking 阻塞发送消息
func (c *WSClient) SendBlocking(message []byte) error {
	select {
	case c.sendCh <- message:
		return nil
	case <-c.done:
		return websocket.ErrCloseSent
	}
}

// WritePump 启动写入协程
func (c *WSClient) WritePump() {
	ticker := time.NewTicker(defaultHeartbeatInterval)
	defer func() {
		ticker.Stop()
		c.conn.Close()
		c.SetStatus(ConnectionStatusDisconnected)
	}()

	for {
		select {
		case message, ok := <-c.sendCh:
			c.conn.SetWriteDeadline(time.Now().Add(defaultWriteTimeout))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
			c.RecordMessageSent()

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(defaultWriteTimeout))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}

		case <-c.done:
			return
		}
	}
}

// Close 关闭客户端连接
func (c *WSClient) Close() {
	close(c.done)
	c.conn.Close()
}

// GetSubscriptions 获取当前订阅列表
func (c *WSClient) GetSubscriptions() []EventType {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	events := make([]EventType, 0, len(c.subscriptions))
	for event := range c.subscriptions {
		events = append(events, event)
	}
	return events
}

// HasSubscriptions 检查是否有任何订阅
func (c *WSClient) HasSubscriptions() bool {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return len(c.subscriptions) > 0
}
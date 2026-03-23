package okx

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/ljwqf/quant/internal/config"
	"github.com/ljwqf/quant/pkg/logger"
	"go.uber.org/zap"
)

const (
	wsStateDisconnected int32 = 0
	wsStateConnected    int32 = 1
	wsStateReconnecting int32 = 2
)

type wsClient struct {
	config          *config.OKXConfig
	conn            *websocket.Conn
	state           int32
	mutex           sync.Mutex
	connMutex       sync.Mutex
	heartbeatTicker *time.Ticker
	messageHandler  func([]byte)
	subscriptions   map[string]bool
	reconnectChan   chan struct{}
	ctx             context.Context
	cancel          context.CancelFunc
}

type wsMessage struct {
	Op   string      `json:"op"`
	Args []wsArg     `json:"args,omitempty"`
	Data interface{} `json:"data,omitempty"`
}

type wsArg struct {
	Channel string `json:"channel"`
	InstId  string `json:"instId"`
	Bar     string `json:"bar,omitempty"`
}

func newWSClient(cfg *config.OKXConfig, messageHandler func([]byte)) (*wsClient, error) {
	ctx, cancel := context.WithCancel(context.Background())
	return &wsClient{
		config:         cfg,
		messageHandler: messageHandler,
		subscriptions:  make(map[string]bool),
		reconnectChan:  make(chan struct{}, 1),
		ctx:            ctx,
		cancel:         cancel,
	}, nil
}

func (w *wsClient) connect() error {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	if atomic.LoadInt32(&w.state) == wsStateConnected {
		return nil
	}

	wsURL, err := url.Parse(w.config.WSURL)
	if err != nil {
		return fmt.Errorf("解析 WebSocket URL 失败: %w", err)
	}

	conn, _, err := websocket.DefaultDialer.Dial(wsURL.String(), nil)
	if err != nil {
		return fmt.Errorf("WebSocket 连接失败: %w", err)
	}

	w.connMutex.Lock()
	w.conn = conn
	w.connMutex.Unlock()

	atomic.StoreInt32(&w.state, wsStateConnected)

	go w.heartbeatLoop()
	go w.readLoop()
	go w.reconnectWorker()

	logger.Info("WebSocket 连接成功")
	return nil
}

func (w *wsClient) disconnect() error {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	if atomic.LoadInt32(&w.state) == wsStateDisconnected {
		return nil
	}

	if w.cancel != nil {
		w.cancel()
	}

	if w.heartbeatTicker != nil {
		w.heartbeatTicker.Stop()
		w.heartbeatTicker = nil
	}

	w.connMutex.Lock()
	if w.conn != nil {
		if err := w.conn.Close(); err != nil {
			logger.Error("WebSocket 关闭失败", zap.Error(err))
		}
		w.conn = nil
	}
	w.connMutex.Unlock()

	atomic.StoreInt32(&w.state, wsStateDisconnected)
	logger.Info("WebSocket 连接已关闭")
	return nil
}

func (w *wsClient) heartbeatLoop() {
	w.mutex.Lock()
	w.heartbeatTicker = time.NewTicker(30 * time.Second)
	ticker := w.heartbeatTicker
	w.mutex.Unlock()

	defer ticker.Stop()

	for {
		select {
		case <-w.ctx.Done():
			return
		case <-ticker.C:
			if err := w.sendHeartbeat(); err != nil {
				logger.Error("发送心跳失败", zap.Error(err))
				w.triggerReconnect()
				return
			}
		}
	}
}

func (w *wsClient) readLoop() {
	for {
		select {
		case <-w.ctx.Done():
			return
		default:
			w.connMutex.Lock()
			conn := w.conn
			w.connMutex.Unlock()

			if conn == nil {
				w.triggerReconnect()
				return
			}

			_, message, err := conn.ReadMessage()
			if err != nil {
				logger.Error("读取 WebSocket 消息失败", zap.Error(err))
				w.triggerReconnect()
				return
			}

			w.messageHandler(message)
		}
	}
}

func (w *wsClient) triggerReconnect() {
	select {
	case w.reconnectChan <- struct{}{}:
	default:
	}
}

func (w *wsClient) reconnectWorker() {
	for {
		select {
		case <-w.ctx.Done():
			return
		case <-w.reconnectChan:
			w.doReconnect()
		}
	}
}

func (w *wsClient) doReconnect() {
	if !atomic.CompareAndSwapInt32(&w.state, wsStateConnected, wsStateReconnecting) {
		currentState := atomic.LoadInt32(&w.state)
		if currentState == wsStateReconnecting {
			return
		}
		if currentState == wsStateDisconnected {
			return
		}
	}

	logger.Info("正在重连 WebSocket...")

	w.connMutex.Lock()
	if w.conn != nil {
		w.conn.Close()
		w.conn = nil
	}
	w.connMutex.Unlock()

	for i := 0; i < 5; i++ {
		select {
		case <-w.ctx.Done():
			return
		default:
		}

		wsURL, err := url.Parse(w.config.WSURL)
		if err != nil {
			logger.Error("解析WebSocket URL失败", zap.Error(err))
			time.Sleep(time.Duration(i+1) * time.Second)
			continue
		}

		conn, _, err := websocket.DefaultDialer.Dial(wsURL.String(), nil)
		if err != nil {
			logger.Error("重连失败", zap.Error(err), zap.Int("attempt", i+1))
			time.Sleep(time.Duration(i+1) * time.Second)
			continue
		}

		w.connMutex.Lock()
		w.conn = conn
		w.connMutex.Unlock()

		atomic.StoreInt32(&w.state, wsStateConnected)

		go w.heartbeatLoop()
		go w.readLoop()

		w.resubscribe()
		logger.Info("WebSocket 重连成功")
		return
	}

	logger.Error("WebSocket 重连失败，已达到最大尝试次数")
	atomic.StoreInt32(&w.state, wsStateDisconnected)
}

func (w *wsClient) sendHeartbeat() error {
	heartbeat := wsMessage{
		Op: "ping",
	}

	return w.send(heartbeat)
}

func (w *wsClient) send(message interface{}) error {
	w.connMutex.Lock()
	defer w.connMutex.Unlock()

	if atomic.LoadInt32(&w.state) != wsStateConnected || w.conn == nil {
		return fmt.Errorf("WebSocket 未连接")
	}

	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("序列化消息失败: %w", err)
	}

	if err := w.conn.WriteMessage(websocket.TextMessage, data); err != nil {
		return fmt.Errorf("发送消息失败: %w", err)
	}

	return nil
}

func (w *wsClient) subscribe(channel, symbol, interval string) error {
	w.mutex.Lock()
	subscriptionKey := fmt.Sprintf("%s:%s:%s", channel, symbol, interval)
	w.subscriptions[subscriptionKey] = true
	w.mutex.Unlock()

	subscribeMsg := wsMessage{
		Op: "subscribe",
		Args: []wsArg{
			{
				Channel: channel,
				InstId:  symbol,
				Bar:     interval,
			},
		},
	}

	return w.send(subscribeMsg)
}

func (w *wsClient) unsubscribe(channel, symbol, interval string) error {
	w.mutex.Lock()
	subscriptionKey := fmt.Sprintf("%s:%s:%s", channel, symbol, interval)
	delete(w.subscriptions, subscriptionKey)
	w.mutex.Unlock()

	unsubscribeMsg := wsMessage{
		Op: "unsubscribe",
		Args: []wsArg{
			{
				Channel: channel,
				InstId:  symbol,
				Bar:     interval,
			},
		},
	}

	return w.send(unsubscribeMsg)
}

func (w *wsClient) resubscribe() {
	w.mutex.Lock()
	subs := make(map[string]bool, len(w.subscriptions))
	for k, v := range w.subscriptions {
		subs[k] = v
	}
	w.mutex.Unlock()

	for key := range subs {
		var channel, symbol, interval string
		fmt.Sscanf(key, "%s:%s:%s", &channel, &symbol, &interval)
		if err := w.subscribe(channel, symbol, interval); err != nil {
			logger.Error("重新订阅失败",
				zap.Error(err),
				zap.String("channel", channel),
				zap.String("symbol", symbol),
				zap.String("interval", interval),
			)
		}
	}
}

func (w *wsClient) isConnected() bool {
	return atomic.LoadInt32(&w.state) == wsStateConnected
}

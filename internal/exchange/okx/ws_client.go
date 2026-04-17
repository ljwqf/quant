package okx

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/ljwqf/quant/internal/config"
	"github.com/ljwqf/quant/pkg/logger"
	"go.uber.org/zap"
	"golang.org/x/net/proxy"
)

const (
	wsStateDisconnected int32 = 0
	wsStateConnected    int32 = 1
	wsStateReconnecting int32 = 2
	maxReconnectAttempt       = 5
)

type wsClient struct {
	config           *config.OKXConfig
	conn             *websocket.Conn
	state            int32
	mutex            sync.Mutex
	connMutex        sync.Mutex
	heartbeatTicker  *time.Ticker
	messageHandler   func([]byte)
	subscriptions    map[string]bool
	reconnectChan    chan struct{}
	ctx              context.Context
	cancel           context.CancelFunc
	connCtx          context.Context    // per-connection context, replaced on each connect/reconnect
	connCancel       context.CancelFunc // cancels connCtx to kill old readLoop/heartbeatLoop
	connMu           sync.Mutex         // protects connCtx/connCancel access
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
	client := &wsClient{
		config:         cfg,
		messageHandler: messageHandler,
		subscriptions:  make(map[string]bool),
		reconnectChan:  make(chan struct{}, 1),
		ctx:            ctx,
		cancel:         cancel,
	}

	go client.reconnectWorker()
	return client, nil
}

// buildDialer 构建 WebSocket 拨号器（支持 SOCKS5 和 HTTP 代理）
func (w *wsClient) buildDialer() *websocket.Dialer {
	dialer := *websocket.DefaultDialer

	if w.config.ProxyURL != "" {
		proxyParsed, err := url.Parse(w.config.ProxyURL)
		if err == nil {
			switch proxyParsed.Scheme {
			case "socks5":
				socksDialer, err := proxy.SOCKS5("tcp", proxyParsed.Host, nil, proxy.Direct)
				if err == nil {
					dialer.NetDial = func(network, addr string) (net.Conn, error) {
						return socksDialer.Dial(network, addr)
					}
				}
			case "http", "https":
				dialer.Proxy = http.ProxyURL(proxyParsed)
			}
		}
		if w.config.ProxySkipVerify {
			dialer.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
		}
	}

	return &dialer
}

func (w *wsClient) connect() error {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	if atomic.LoadInt32(&w.state) == wsStateConnected {
		return nil
	}

	// SOCKS5 proxy may need a moment to initialize; delay first connect attempt.
	if w.config.ProxyURL != "" {
		time.Sleep(2 * time.Second)
	}

	wsURL, err := url.Parse(w.config.WSURL)
	if err != nil {
		return fmt.Errorf("解析 WebSocket URL 失败: %w", err)
	}

	conn, _, err := w.buildDialer().Dial(wsURL.String(), nil)
	if err != nil {
		return fmt.Errorf("WebSocket 连接失败: %w", err)
	}

	// Cancel any stale connection context before establishing new one
	w.cancelConnection()

	w.connMutex.Lock()
	w.conn = conn
	w.connMutex.Unlock()

	// Create a fresh per-connection context — old goroutines will exit when this is cancelled
	connCtx, connCancel := context.WithCancel(w.ctx)
	w.connMu.Lock()
	w.connCtx = connCtx
	w.connCancel = connCancel
	w.connMu.Unlock()

	atomic.StoreInt32(&w.state, wsStateConnected)

	go w.heartbeatLoop(connCtx)
	go w.readLoop(connCtx)

	logger.Info("WebSocket 连接成功")
	return nil
}

func (w *wsClient) disconnect() error {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	if atomic.LoadInt32(&w.state) == wsStateDisconnected {
		return nil
	}

	// Cancel the per-connection context to kill active readLoop/heartbeatLoop
	w.cancelConnection()

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

// cancelConnection cancels the current per-connection context and cleans up references.
// Must be called with w.mutex or w.connMu held.
func (w *wsClient) cancelConnection() {
	w.connMu.Lock()
	defer w.connMu.Unlock()

	if w.connCancel != nil {
		w.connCancel()
		w.connCancel = nil
	}
	w.connCtx = nil
}

func (w *wsClient) heartbeatLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	w.mutex.Lock()
	w.heartbeatTicker = ticker
	w.mutex.Unlock()

	for {
		select {
		case <-ctx.Done():
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

func (w *wsClient) readLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
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
	for {
		currentState := atomic.LoadInt32(&w.state)
		if currentState == wsStateReconnecting {
			return
		}
		if atomic.CompareAndSwapInt32(&w.state, currentState, wsStateReconnecting) {
			break
		}
	}

	logger.Info("正在重连 WebSocket...")

	// Cancel old connection goroutines before reconnecting
	w.cancelConnection()

	w.connMutex.Lock()
	if w.conn != nil {
		w.conn.Close()
		w.conn = nil
	}
	w.connMutex.Unlock()

	for i := 0; i < maxReconnectAttempt; i++ {
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

		conn, _, err := w.buildDialer().Dial(wsURL.String(), nil)
		if err != nil {
			logger.Error("重连失败", zap.Error(err), zap.Int("attempt", i+1))
			time.Sleep(time.Duration(i+1) * time.Second)
			continue
		}

		w.connMutex.Lock()
		w.conn = conn
		w.connMutex.Unlock()

		// Create a fresh per-connection context so old goroutines are properly terminated
		connCtx, connCancel := context.WithCancel(w.ctx)
		w.connMu.Lock()
		w.connCtx = connCtx
		w.connCancel = connCancel
		w.connMu.Unlock()

		atomic.StoreInt32(&w.state, wsStateConnected)

		go w.heartbeatLoop(connCtx)
		go w.readLoop(connCtx)

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
		channel, symbol, interval, ok := parseSubscriptionKey(key)
		if !ok {
			logger.Warn("忽略无效订阅键", zap.String("key", key))
			continue
		}
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

func parseSubscriptionKey(key string) (channel, symbol, interval string, ok bool) {
	parts := strings.SplitN(key, ":", 3)
	if len(parts) != 3 {
		return "", "", "", false
	}
	if parts[0] == "" || parts[1] == "" {
		return "", "", "", false
	}
	return parts[0], parts[1], parts[2], true
}

func (w *wsClient) isConnected() bool {
	return atomic.LoadInt32(&w.state) == wsStateConnected
}

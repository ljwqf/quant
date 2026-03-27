package api

import (
	"encoding/json"
	"net/http"
	"net/url"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/ljwqf/quant/pkg/logger"
	"go.uber.org/zap"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true
		}
		originURL, err := url.Parse(origin)
		if err != nil {
			return false
		}
		return originURL.Host == r.Host
	},
}

// WebSocketHub WebSocket中心
type WebSocketHub struct {
	server     *Server
	clients    map[*websocket.Conn]*WSClient
	broadcast  chan []byte
	register   chan *websocket.Conn
	unregister chan *websocket.Conn
	mutex      sync.RWMutex
}

// WSMessage WebSocket消息 (保留向后兼容)
type WSMessage struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

// NewWebSocketHub 创建WebSocket中心
func NewWebSocketHub(server *Server) *WebSocketHub {
	return &WebSocketHub{
		server:     server,
		clients:    make(map[*websocket.Conn]*WSClient),
		broadcast:  make(chan []byte, 256),
		register:   make(chan *websocket.Conn),
		unregister: make(chan *websocket.Conn),
	}
}

// Run 运行WebSocket中心
func (h *WebSocketHub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mutex.Lock()
			h.clients[client] = NewWSClient(client)
			h.mutex.Unlock()
			logger.Debug("WebSocket客户端连接", zap.Int("count", len(h.clients)))

		case conn := <-h.unregister:
			h.mutex.Lock()
			if client, ok := h.clients[conn]; ok {
				delete(h.clients, conn)
				client.Close()
			}
			h.mutex.Unlock()
			logger.Debug("WebSocket客户端断开", zap.Int("count", len(h.clients)))

		case message := <-h.broadcast:
			h.mutex.RLock()
			for _, client := range h.clients {
				client.Send(message)
			}
			h.mutex.RUnlock()
		}
	}
}

// Broadcast 广播消息 (保留向后兼容)
func (h *WebSocketHub) Broadcast(msgType string, data interface{}) {
	envelope := NewWSEnvelope(EventType(msgType), data)
	jsonData, err := json.Marshal(envelope)
	if err != nil {
		logger.Error("序列化WebSocket消息失败", zap.Error(err))
		return
	}
	h.broadcast <- jsonData
}

// BroadcastTo 向订阅了指定事件类型的客户端广播
func (h *WebSocketHub) BroadcastTo(eventType EventType, data interface{}) {
	envelope := NewWSEnvelope(eventType, data)
	jsonData, err := json.Marshal(envelope)
	if err != nil {
		logger.Error("序列化WebSocket消息失败", zap.Error(err))
		return
	}

	h.mutex.RLock()
	for _, client := range h.clients {
		if client.IsSubscribed(eventType) {
			client.Send(jsonData)
		}
	}
	h.mutex.RUnlock()
}

// HandleWebSocket 处理WebSocket连接
func (h *WebSocketHub) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	if !isLocalRequest(r) && !h.server.hasValidToken(r.URL.Query().Get("token")) {
		http.Error(w, "WebSocket requires local access or a valid token", http.StatusForbidden)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Error("WebSocket升级失败", zap.Error(err))
		return
	}

	h.register <- conn

	defer func() {
		h.unregister <- conn
	}()

	// 启动写入协程
	wsClient := NewWSClient(conn)
	go wsClient.WritePump()

	// 读取客户端消息（订阅/取消订阅命令）
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			break
		}

		var cmd WSClientCommand
		if err := json.Unmarshal(message, &cmd); err != nil {
			continue
		}

		h.handleClientCommand(conn, wsClient, &cmd)
	}
}

// handleClientCommand 处理客户端命令
func (h *WebSocketHub) handleClientCommand(conn *websocket.Conn, client *WSClient, cmd *WSClientCommand) {
	switch cmd.Action {
	case "subscribe":
		events := make([]EventType, 0, len(cmd.Events))
		for _, e := range cmd.Events {
			events = append(events, EventType(e))
		}
		client.Subscribe(events)

		// 发送订阅确认
		resp := &WSSubscriptionResponse{
			Success: true,
			Events:  cmd.Events,
			Message: "订阅成功",
		}
		envelope := NewWSEnvelope(EventTypeSubscription, resp)
		if data, err := json.Marshal(envelope); err == nil {
			client.Send(data)
		}

	case "unsubscribe":
		events := make([]EventType, 0, len(cmd.Events))
		for _, e := range cmd.Events {
			events = append(events, EventType(e))
		}
		client.Unsubscribe(events)

		// 发送取消订阅确认
		resp := &WSSubscriptionResponse{
			Success: true,
			Events:  cmd.Events,
			Message: "取消订阅成功",
		}
		envelope := NewWSEnvelope(EventTypeSubscription, resp)
		if data, err := json.Marshal(envelope); err == nil {
			client.Send(data)
		}
	}
}

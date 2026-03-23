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
	clients    map[*websocket.Conn]bool
	broadcast  chan []byte
	register   chan *websocket.Conn
	unregister chan *websocket.Conn
	mutex      sync.RWMutex
}

// WSMessage WebSocket消息
type WSMessage struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

// NewWebSocketHub 创建WebSocket中心
func NewWebSocketHub(server *Server) *WebSocketHub {
	return &WebSocketHub{
		server:     server,
		clients:    make(map[*websocket.Conn]bool),
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
			h.clients[client] = true
			h.mutex.Unlock()
			logger.Debug("WebSocket客户端连接", zap.Int("count", len(h.clients)))

		case client := <-h.unregister:
			h.mutex.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				client.Close()
			}
			h.mutex.Unlock()
			logger.Debug("WebSocket客户端断开", zap.Int("count", len(h.clients)))

		case message := <-h.broadcast:
			h.mutex.RLock()
			for client := range h.clients {
				err := client.WriteMessage(websocket.TextMessage, message)
				if err != nil {
					client.Close()
					delete(h.clients, client)
				}
			}
			h.mutex.RUnlock()
		}
	}
}

// Broadcast 广播消息
func (h *WebSocketHub) Broadcast(msgType string, data interface{}) {
	message := WSMessage{
		Type: msgType,
		Data: data,
	}
	jsonData, err := json.Marshal(message)
	if err != nil {
		logger.Error("序列化WebSocket消息失败", zap.Error(err))
		return
	}
	h.broadcast <- jsonData
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

	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

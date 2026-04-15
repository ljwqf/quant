package api

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/ljwqf/quant/pkg/logger"
	"github.com/ljwqf/quant/pkg/types"
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
	server        *Server
	clients       map[*websocket.Conn]*WSClient
	broadcast     chan []byte
	register      chan *websocket.Conn
	unregister    chan *websocket.Conn
	mutex         sync.RWMutex
	totalMessages int64
	startedAt     time.Time
}

// WSMessage WebSocket消息 (保留向后兼容)
type WSMessage struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

// NewWebSocketHub 创建WebSocket中心
func NewWebSocketHub(server *Server) *WebSocketHub {
	return &WebSocketHub{
		server:        server,
		clients:       make(map[*websocket.Conn]*WSClient),
		broadcast:     make(chan []byte, 256),
		register:      make(chan *websocket.Conn),
		unregister:    make(chan *websocket.Conn),
		startedAt:     time.Now(),
	}
}

// BroadcastStatus 广播系统状态
func (h *WebSocketHub) BroadcastStatus() {
	h.mutex.RLock()
	clientCount := len(h.clients)
	h.mutex.RUnlock()

	data := &StatusData{
		SystemStatus: "running",
		ConnectedAt:  h.startedAt,
		Uptime:       time.Since(h.startedAt),
		ClientCount:   clientCount,
		MessageCount:  h.totalMessages,
	}
	h.BroadcastTo(EventTypeStatus, data)
}

// BroadcastAlert 广播告警消息
func (h *WebSocketHub) BroadcastAlert(level, source, message, code string, metadata map[string]interface{}) {
	data := &AlertData{
		Level:     level,
		Source:    source,
		Message:   message,
		Code:      code,
		Timestamp: time.Now(),
		Metadata:  metadata,
	}
	h.BroadcastTo(EventTypeAlert, data)
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

	h.mutex.Lock()
	h.totalMessages++
	h.mutex.Unlock()

	h.mutex.RLock()
	for _, client := range h.clients {
		if client.IsSubscribed(eventType) {
			client.Send(jsonData)
		}
	}
	h.mutex.RUnlock()
}

// BroadcastTicker 广播行情数据
func (h *WebSocketHub) BroadcastTicker(ticker *types.Tick) {
	data := &TickerData{
		Symbol:    ticker.Symbol,
		Price:     ticker.Price,
		Timestamp: ticker.Timestamp,
	}
	h.BroadcastTo(EventTypeTicker, data)
}

// BroadcastKline 广播K线数据
func (h *WebSocketHub) BroadcastKline(bar *types.Bar, interval string) {
	data := &KlineData{
		Symbol:    bar.Symbol,
		Interval:  interval,
		Open:      bar.Open,
		High:      bar.High,
		Low:       bar.Low,
		Close:     bar.Close,
		Volume:    bar.Volume,
		Timestamp: bar.Timestamp,
	}
	h.BroadcastTo(EventTypeKline, data)
}

// BroadcastOrderBook 广播订单簿数据
func (h *WebSocketHub) BroadcastOrderBook(orderBook *types.OrderBook) {
	// 转换订单簿数据格式
	asks := make([][2]float64, 0, len(orderBook.Asks))
	bids := make([][2]float64, 0, len(orderBook.Bids))

	for _, ask := range orderBook.Asks {
		asks = append(asks, [2]float64{ask.Price, ask.Size})
	}
	for _, bid := range orderBook.Bids {
		bids = append(bids, [2]float64{bid.Price, bid.Size})
	}

	data := &OrderBookData{
		Symbol:    orderBook.Symbol,
		Asks:      asks,
		Bids:      bids,
		Timestamp: orderBook.Timestamp,
	}
	h.BroadcastTo(EventTypeOrderBook, data)
}

// GetClientCount 获取当前连接的客户端数量
func (h *WebSocketHub) GetClientCount() int {
	h.mutex.RLock()
	defer h.mutex.RUnlock()
	return len(h.clients)
}

// BroadcastOrderUpdate 广播订单更新
func (h *WebSocketHub) BroadcastOrderUpdate(order *types.Order) {
	data := &OrderUpdateData{
		OrderID:   order.ID,
		Symbol:    order.Symbol,
		Side:      string(order.Side),
		Status:    string(order.Status),
		FilledQty: order.FilledQty,
		AvgPrice:  order.AveragePrice,
	}
	h.BroadcastTo(EventTypeOrderUpdate, data)
}

// BroadcastPositionChange 广播持仓变化
func (h *WebSocketHub) BroadcastPositionChange(position *types.Position, changeType string) {
	data := &PositionChangeData{
		Symbol:     position.Symbol,
		Side:       string(position.Side),
		ChangeType: changeType,
		Size:       position.Size,
		EntryPrice: position.EntryPrice,
		PnL:        position.UnrealizedPnL,
	}
	h.BroadcastTo(EventTypePositionChange, data)
}

// BroadcastTrade 广播交易成交
func (h *WebSocketHub) BroadcastTrade(trade *types.Trade) {
	data := &TradeData{
		TradeID:  trade.ID,
		OrderID:  trade.OrderID,
		Symbol:   trade.Symbol,
		Side:     string(trade.Side),
		Price:    trade.Price,
		Quantity: trade.Quantity,
		Fee:      trade.Fee,
		IsMaker:  trade.IsMaker,
	}
	h.BroadcastTo(EventTypeTrade, data)
}

// HandleWebSocket 处理WebSocket连接
func (h *WebSocketHub) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		// 也支持 Authorization 头（Bearer 前缀）
		auth := r.Header.Get("Authorization")
		token = strings.TrimPrefix(auth, "Bearer ")
		if token == auth {
			token = "" // 没有 Bearer 前缀，不使用
		}
	}
	if !isLocalRequest(r) && !h.server.hasValidToken(token) {
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

		wsClient.RecordMessageReceived()

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

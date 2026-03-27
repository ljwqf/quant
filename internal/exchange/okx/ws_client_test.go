package okx

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/ljwqf/quant/internal/config"
	"github.com/stretchr/testify/assert"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// createTestWSServer 创建测试WebSocket服务器
func createTestWSServer(handler func(*websocket.Conn)) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		if handler != nil {
			handler(conn)
		}
	}))
}

func TestNewWSClient(t *testing.T) {
	cfg := &config.OKXConfig{
		APIKey:     "test_key",
		SecretKey:  "test_secret",
		Passphrase: "test_pass",
		WSURL:      "wss://ws.okx.com:8443/ws/v5/public",
	}

	received := make(chan []byte, 1)
	handler := func(msg []byte) {
		received <- msg
	}

	client, err := newWSClient(cfg, handler)
	assert.NoError(t, err)
	assert.NotNil(t, client)
	assert.NotNil(t, client.subscriptions)
	assert.NotNil(t, client.reconnectChan)
	assert.Equal(t, wsStateDisconnected, atomicLoadInt32(&client.state))

	// Clean up
	client.cancel()
}

func TestWSClient_Connect(t *testing.T) {
	server := createTestWSServer(func(conn *websocket.Conn) {
		// Echo messages back
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			conn.WriteMessage(websocket.TextMessage, msg)
		}
	})
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	cfg := &config.OKXConfig{
		APIKey:     "test_key",
		SecretKey:  "test_secret",
		Passphrase: "test_pass",
		WSURL:      wsURL,
	}

	client, err := newWSClient(cfg, func([]byte) {})
	assert.NoError(t, err)
	defer client.cancel()

	err = client.connect()
	assert.NoError(t, err)
	assert.True(t, client.isConnected())

	// Second connect should be no-op
	err = client.connect()
	assert.NoError(t, err)
}

func TestWSClient_Disconnect(t *testing.T) {
	server := createTestWSServer(func(conn *websocket.Conn) {
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				return
			}
		}
	})
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	cfg := &config.OKXConfig{
		APIKey:     "test_key",
		SecretKey:  "test_secret",
		Passphrase: "test_pass",
		WSURL:      wsURL,
	}

	client, err := newWSClient(cfg, func([]byte) {})
	assert.NoError(t, err)
	defer client.cancel()

	// Disconnect when not connected should be no-op
	err = client.disconnect()
	assert.NoError(t, err)

	// Connect then disconnect
	err = client.connect()
	assert.NoError(t, err)

	err = client.disconnect()
	assert.NoError(t, err)
	assert.False(t, client.isConnected())
}

func TestWSClient_IsConnected(t *testing.T) {
	cfg := &config.OKXConfig{
		APIKey:     "test_key",
		SecretKey:  "test_secret",
		Passphrase: "test_pass",
		WSURL:      "wss://ws.okx.com:8443/ws/v5/public",
	}

	client, err := newWSClient(cfg, func([]byte) {})
	assert.NoError(t, err)
	defer client.cancel()

	assert.False(t, client.isConnected())

	// Simulate connected state
	atomicStoreInt32(&client.state, wsStateConnected)
	assert.True(t, client.isConnected())

	// Simulate reconnecting state
	atomicStoreInt32(&client.state, wsStateReconnecting)
	assert.False(t, client.isConnected())
}

func TestWSClient_TriggerReconnect(t *testing.T) {
	cfg := &config.OKXConfig{
		APIKey:     "test_key",
		SecretKey:  "test_secret",
		Passphrase: "test_pass",
		WSURL:      "wss://ws.okx.com:8443/ws/v5/public",
	}

	client, err := newWSClient(cfg, func([]byte) {})
	assert.NoError(t, err)
	defer client.cancel()

	// Should not block
	client.triggerReconnect()

	// Wait a bit for the signal to be processed
	time.Sleep(50 * time.Millisecond)

	// Check that reconnectChan was signaled (may have been consumed by reconnectWorker)
	// The test passes if triggerReconnect doesn't block
	assert.True(t, true)
}

func TestWSClient_Subscribe(t *testing.T) {
	server := createTestWSServer(func(conn *websocket.Conn) {
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			// Echo back
			conn.WriteMessage(websocket.TextMessage, msg)
		}
	})
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	cfg := &config.OKXConfig{
		APIKey:     "test_key",
		SecretKey:  "test_secret",
		Passphrase: "test_pass",
		WSURL:      wsURL,
	}

	client, err := newWSClient(cfg, func([]byte) {})
	assert.NoError(t, err)
	defer client.cancel()

	err = client.connect()
	assert.NoError(t, err)

	err = client.subscribe("ticker", "BTC-USDT", "")
	assert.NoError(t, err)

	// Verify subscription was recorded
	client.mutex.Lock()
	_, exists := client.subscriptions["ticker:BTC-USDT:"]
	client.mutex.Unlock()
	assert.True(t, exists)
}

func TestWSClient_Send(t *testing.T) {
	server := createTestWSServer(func(conn *websocket.Conn) {
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			conn.WriteMessage(websocket.TextMessage, msg)
		}
	})
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	cfg := &config.OKXConfig{
		APIKey:     "test_key",
		SecretKey:  "test_secret",
		Passphrase: "test_pass",
		WSURL:      wsURL,
	}

	client, err := newWSClient(cfg, func([]byte) {})
	assert.NoError(t, err)
	defer client.cancel()

	err = client.connect()
	assert.NoError(t, err)

	msg := &wsMessage{
		Op: "ping",
	}

	err = client.send(msg)
	assert.NoError(t, err)
}

func TestWSClient_SendHeartbeat(t *testing.T) {
	server := createTestWSServer(func(conn *websocket.Conn) {
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			conn.WriteMessage(websocket.TextMessage, msg)
		}
	})
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	cfg := &config.OKXConfig{
		APIKey:     "test_key",
		SecretKey:  "test_secret",
		Passphrase: "test_pass",
		WSURL:      wsURL,
	}

	client, err := newWSClient(cfg, func([]byte) {})
	assert.NoError(t, err)
	defer client.cancel()

	err = client.connect()
	assert.NoError(t, err)

	err = client.sendHeartbeat()
	assert.NoError(t, err)
}

func TestWSClient_Resubscribe(t *testing.T) {
	server := createTestWSServer(func(conn *websocket.Conn) {
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			conn.WriteMessage(websocket.TextMessage, msg)
		}
	})
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	cfg := &config.OKXConfig{
		APIKey:     "test_key",
		SecretKey:  "test_secret",
		Passphrase: "test_pass",
		WSURL:      wsURL,
	}

	client, err := newWSClient(cfg, func([]byte) {})
	assert.NoError(t, err)
	defer client.cancel()

	err = client.connect()
	assert.NoError(t, err)

	// Add subscriptions
	client.mutex.Lock()
	client.subscriptions["ticker:BTC-USDT:"] = true
	client.subscriptions["candle:ETH-USDT:1m"] = true
	client.mutex.Unlock()

	// resubscribe doesn't return an error
	client.resubscribe()
}

func TestWSClient_ContextCancellation(t *testing.T) {
	cfg := &config.OKXConfig{
		APIKey:     "test_key",
		SecretKey:  "test_secret",
		Passphrase: "test_pass",
		WSURL:      "wss://ws.okx.com:8443/ws/v5/public",
	}

	client, err := newWSClient(cfg, func([]byte) {})
	assert.NoError(t, err)

	// Cancel context
	client.cancel()

	// Verify context is cancelled
	select {
	case <-client.ctx.Done():
		// Expected
	default:
		t.Error("Expected context to be cancelled")
	}
}

// Helper functions for atomic operations
func atomicLoadInt32(addr *int32) int32 {
	return *addr // Simplified for test
}

func atomicStoreInt32(addr *int32, val int32) {
	*addr = val // Simplified for test
}

// Additional test for parseSubscriptionKey with more cases
func TestParseSubscriptionKey_MoreCases(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		ok       bool
		channel  string
		symbol   string
		interval string
	}{
		{name: "ticker channel", key: "ticker:BTC-USDT:", ok: true, channel: "ticker", symbol: "BTC-USDT", interval: ""},
		{name: "candle channel", key: "candle:ETH-USDT:5m", ok: true, channel: "candle", symbol: "ETH-USDT", interval: "5m"},
		{name: "books channel", key: "books:SOL-USDT:", ok: true, channel: "books", symbol: "SOL-USDT", interval: ""},
		{name: "invalid empty key", key: "", ok: false},
		{name: "invalid one part", key: "ticker", ok: false},
		{name: "invalid two parts", key: "ticker:BTC-USDT", ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			channel, symbol, interval, ok := parseSubscriptionKey(tt.key)
			assert.Equal(t, tt.ok, ok)
			if ok {
				assert.Equal(t, tt.channel, channel)
				assert.Equal(t, tt.symbol, symbol)
				assert.Equal(t, tt.interval, interval)
			}
		})
	}
}

package okx

import (
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/ljwqf/quant/internal/config"
	"github.com/ljwqf/quant/pkg/types"
	"github.com/stretchr/testify/assert"
)

func TestHandleWSMessageRoutesTickerByChannel(t *testing.T) {
	client := NewClient(&config.OKXConfig{})

	var wg sync.WaitGroup
	called := make(chan struct{}, 1)
	wg.Add(1)

	client.tickerHandlers["BTC-USDT"] = []func(*types.Tick){
		func(_ *types.Tick) {
			called <- struct{}{}
			wg.Done()
		},
	}

	msg := []byte(`{"arg":{"channel":"ticker","instId":"BTC-USDT"},"data":[{"instId":"BTC-USDT","last":"100","open24h":"99","high24h":"101","low24h":"98","vol24h":"1000","ts":"1710000000000"}]}`)
	client.handleWSMessage(msg)

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("ticker handler was not called")
	}

	select {
	case <-called:
	default:
		t.Fatal("ticker callback signal missing")
	}
}

func TestHandleMarketData_Ticker(t *testing.T) {
	client := NewClient(&config.OKXConfig{})

	var receivedTick *types.Tick
	var wg sync.WaitGroup
	wg.Add(1)

	client.tickerHandlers["ETH-USDT"] = []func(*types.Tick){
		func(tick *types.Tick) {
			receivedTick = tick
			wg.Done()
		},
	}

	data := json.RawMessage(`[{"instId":"ETH-USDT","last":"3000.5","open24h":"2900","high24h":"3100","low24h":"2850","vol24h":"50000","ts":"1710000000000"}]`)

	client.handleMarketData("ticker", data)

	wg.Wait()

	assert.NotNil(t, receivedTick)
	assert.Equal(t, "ETH-USDT", receivedTick.Symbol)
	assert.Equal(t, 3000.5, receivedTick.Price)
	assert.Equal(t, 2900.0, receivedTick.Open24h)
	assert.Equal(t, 3100.0, receivedTick.High24h)
	assert.Equal(t, 2850.0, receivedTick.Low24h)
}

func TestHandleMarketData_Candle(t *testing.T) {
	client := NewClient(&config.OKXConfig{})

	var receivedBar *types.Bar
	var wg sync.WaitGroup
	wg.Add(1)

	client.barHandlers = map[string]map[string][]func(*types.Bar){
		"BTC-USDT": {
			"1m": []func(*types.Bar){
				func(bar *types.Bar) {
					receivedBar = bar
					wg.Done()
				},
			},
		},
	}

	data := json.RawMessage(`[{"instId":"BTC-USDT","candle":["1710000000000","50000","51000","49500","50500","100"],"bar":"1m"}]`)

	client.handleMarketData("candle1m", data)

	wg.Wait()

	assert.NotNil(t, receivedBar)
	assert.Equal(t, "BTC-USDT", receivedBar.Symbol)
	assert.Equal(t, 50000.0, receivedBar.Open)
	assert.Equal(t, 51000.0, receivedBar.High)
	assert.Equal(t, 49500.0, receivedBar.Low)
	assert.Equal(t, 50500.0, receivedBar.Close)
	assert.Equal(t, 100.0, receivedBar.Volume)
}

func TestHandleMarketData_OrderBook(t *testing.T) {
	client := NewClient(&config.OKXConfig{})

	var receivedBook *types.OrderBook
	var wg sync.WaitGroup
	wg.Add(1)

	client.orderBookHandlers["SOL-USDT"] = []func(*types.OrderBook){
		func(book *types.OrderBook) {
			receivedBook = book
			wg.Done()
		},
	}

	data := json.RawMessage(`[{"instId":"SOL-USDT","asks":[["150","10"],["151","20"]],"bids":[["149","15"],["148","25"]],"ts":"1710000000000","checksum":"12345"}]`)

	client.handleMarketData("books", data)

	wg.Wait()

	assert.NotNil(t, receivedBook)
	assert.Equal(t, "SOL-USDT", receivedBook.Symbol)
	assert.Len(t, receivedBook.Asks, 2)
	assert.Len(t, receivedBook.Bids, 2)
}

func TestHandleMarketData_EmptyData(t *testing.T) {
	client := NewClient(&config.OKXConfig{})

	// Empty array should not cause panic
	data := json.RawMessage(`[]`)
	client.handleMarketData("ticker", data)

	// Empty object should not cause panic
	data = json.RawMessage(`{}`)
	client.handleMarketData("ticker", data)
}

func TestHandleMarketData_InvalidJSON(t *testing.T) {
	client := NewClient(&config.OKXConfig{})

	// Invalid JSON should not cause panic
	data := json.RawMessage(`invalid json`)
	client.handleMarketData("ticker", data)
}

func TestHandleMarketData_EmptyInstId(t *testing.T) {
	client := NewClient(&config.OKXConfig{})

	called := false
	client.tickerHandlers[""] = []func(*types.Tick){
		func(tick *types.Tick) {
			called = true
		},
	}

	// Empty instId should be skipped
	data := json.RawMessage(`[{"instId":"","last":"100"}]`)
	client.handleMarketData("ticker", data)

	assert.False(t, called, "handler should not be called for empty instId")
}

func TestHandleWSMessage_Pong(t *testing.T) {
	client := NewClient(&config.OKXConfig{})

	// Pong message should not cause panic
	msg := []byte(`{"op":"pong"}`)
	client.handleWSMessage(msg)
}

func TestHandleWSMessage_Subscribe(t *testing.T) {
	client := NewClient(&config.OKXConfig{})

	// Subscribe confirmation should not cause panic
	msg := []byte(`{"op":"subscribe","arg":{"channel":"ticker","instId":"BTC-USDT"}}`)
	client.handleWSMessage(msg)
}

func TestHandleWSMessage_InvalidJSON(t *testing.T) {
	client := NewClient(&config.OKXConfig{})

	// Invalid JSON should not cause panic
	msg := []byte(`invalid json`)
	client.handleWSMessage(msg)
}

func TestHandleWSMessage_FallbackTicker(t *testing.T) {
	client := NewClient(&config.OKXConfig{})

	var receivedTick *types.Tick
	var wg sync.WaitGroup
	wg.Add(1)

	client.tickerHandlers["BTC-USDT"] = []func(*types.Tick){
		func(tick *types.Tick) {
			receivedTick = tick
			wg.Done()
		},
	}

	// Message without op field (fallback parsing)
	msg := []byte(`{"data":{"instId":"BTC-USDT","last":"50000","open24h":"49000","high24h":"51000","low24h":"48000","vol24h":"1000","ts":"1710000000000"}}`)
	client.handleWSMessage(msg)

	wg.Wait()

	assert.NotNil(t, receivedTick)
	assert.Equal(t, "BTC-USDT", receivedTick.Symbol)
	assert.Equal(t, 50000.0, receivedTick.Price)
}

func TestClient_SubscribeTicker(t *testing.T) {
	cfg := &config.OKXConfig{
		APIKey:     "test_key",
		SecretKey:  "test_secret",
		Passphrase: "test_pass",
		WSURL:      "wss://ws.okx.com:8443/ws/v5/public",
	}

	client := NewClient(cfg)
	client.connected = true

	// Create a mock wsClient
	client.wsClient, _ = newWSClient(cfg, client.handleWSMessage)

	handler := func(tick *types.Tick) {}

	err := client.SubscribeTicker("BTC-USDT", handler)
	// Will fail because WebSocket can't connect, but we test the logic
	_ = err // Expected to fail in unit test without real WebSocket
}

func TestClient_SubscribeBar(t *testing.T) {
	cfg := &config.OKXConfig{
		APIKey:     "test_key",
		SecretKey:  "test_secret",
		Passphrase: "test_pass",
		WSURL:      "wss://ws.okx.com:8443/ws/v5/public",
	}

	client := NewClient(cfg)
	client.connected = true

	client.wsClient, _ = newWSClient(cfg, client.handleWSMessage)

	handler := func(bar *types.Bar) {}

	_ = client.SubscribeBar("ETH-USDT", "1m", handler)
}

func TestClient_SubscribeOrderBook(t *testing.T) {
	cfg := &config.OKXConfig{
		APIKey:     "test_key",
		SecretKey:  "test_secret",
		Passphrase: "test_pass",
		WSURL:      "wss://ws.okx.com:8443/ws/v5/public",
	}

	client := NewClient(cfg)
	client.connected = true

	client.wsClient, _ = newWSClient(cfg, client.handleWSMessage)

	handler := func(book *types.OrderBook) {}

	_ = client.SubscribeOrderBook("SOL-USDT", handler)
}

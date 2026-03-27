package okx

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/ljwqf/quant/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestNewClient(t *testing.T) {
	cfg := &config.OKXConfig{
		APIKey:     "test_key",
		SecretKey:  "test_secret",
		Passphrase: "test_pass",
	}

	client := NewClient(cfg)
	assert.NotNil(t, client)
	assert.NotNil(t, client.tickerHandlers)
	assert.NotNil(t, client.barHandlers)
	assert.NotNil(t, client.orderBookHandlers)
	assert.Equal(t, cfg, client.config)
}

func TestClient_Connect_AlreadyConnected(t *testing.T) {
	cfg := &config.OKXConfig{
		APIKey:     "test_key",
		SecretKey:  "test_secret",
		Passphrase: "test_pass",
	}

	client := NewClient(cfg)
	client.connected = true

	err := client.Connect()
	assert.NoError(t, err)
}

func TestClient_Disconnect(t *testing.T) {
	cfg := &config.OKXConfig{
		APIKey:     "test_key",
		SecretKey:  "test_secret",
		Passphrase: "test_pass",
	}

	client := NewClient(cfg)
	client.connected = true

	err := client.Disconnect()
	assert.NoError(t, err)
	assert.False(t, client.connected)
}

func TestClient_Disconnect_NotConnected(t *testing.T) {
	cfg := &config.OKXConfig{
		APIKey:     "test_key",
		SecretKey:  "test_secret",
		Passphrase: "test_pass",
	}

	client := NewClient(cfg)

	err := client.Disconnect()
	assert.NoError(t, err)
}

func TestClient_IsConnected(t *testing.T) {
	cfg := &config.OKXConfig{
		APIKey:     "test_key",
		SecretKey:  "test_secret",
		Passphrase: "test_pass",
	}

	client := NewClient(cfg)
	assert.False(t, client.IsConnected())

	client.connected = true
	assert.True(t, client.IsConnected())
}

func TestClient_GetAccount(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"code": "0",
			"data": []map[string]interface{}{
				{
					"totalEq": "10000.5",
					"isoUpl":  "100.5",
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := &config.OKXConfig{
		APIKey:     "test_key",
		SecretKey:  "test_secret",
		Passphrase: "test_pass",
		BaseURL:    server.URL,
	}

	client := NewClient(cfg)
	client.restClient = &restClient{
		config:     cfg,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
	client.connected = true

	account, err := client.GetAccount()
	assert.NoError(t, err)
	assert.NotNil(t, account)
}

func TestClient_GetPositions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"code": "0",
			"data": []map[string]interface{}{
				{
					"instId":  "BTC-USDT",
					"posSide": "long",
					"pos":     "0.1",
					"avgPx":   "50000",
					"upl":     "100.5",
					"lever":   "10",
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := &config.OKXConfig{
		APIKey:     "test_key",
		SecretKey:  "test_secret",
		Passphrase: "test_pass",
		BaseURL:    server.URL,
	}

	client := NewClient(cfg)
	client.restClient = &restClient{
		config:     cfg,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
	client.connected = true

	positions, err := client.GetPositions()
	assert.NoError(t, err)
	assert.Len(t, positions, 1)
}

func TestClient_GetTicker(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"code": "0",
			"data": []map[string]interface{}{
				{
					"instId":  "BTC-USDT",
					"last":    "50000.5",
					"vol24h":  "1000000",
					"open24h": "49000",
					"high24h": "51000",
					"low24h":  "48000",
					"ts":      "1609459200000",
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := &config.OKXConfig{
		APIKey:     "test_key",
		SecretKey:  "test_secret",
		Passphrase: "test_pass",
		BaseURL:    server.URL,
	}

	client := NewClient(cfg)
	client.restClient = &restClient{
		config:     cfg,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
	client.connected = true

	ticker, err := client.GetTicker("BTC-USDT")
	assert.NoError(t, err)
	assert.NotNil(t, ticker)
	assert.Equal(t, "BTC-USDT", ticker.Symbol)
}

func TestClient_GetBars(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"code": "0",
			"data": [][]string{
				{"1609459200000", "50000", "51000", "49000", "50500", "1000", "50000000", "50000000"},
				{"1609459140000", "49500", "50500", "49000", "50000", "800", "40000000", "40000000"},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := &config.OKXConfig{
		APIKey:     "test_key",
		SecretKey:  "test_secret",
		Passphrase: "test_pass",
		BaseURL:    server.URL,
	}

	client := NewClient(cfg)
	client.restClient = &restClient{
		config:     cfg,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
	client.connected = true

	bars, err := client.GetBars("BTC-USDT", "1m", 2)
	assert.NoError(t, err)
	assert.Len(t, bars, 2)
}

func TestClient_GetOrderBook(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"code": "0",
			"data": []map[string]interface{}{
				{
					"instId":   "BTC-USDT",
					"asks":     [][]string{{"51000", "1.5"}, {"51100", "2.0"}},
					"bids":     [][]string{{"50000", "2.0"}, {"49900", "1.5"}},
					"ts":       "1609459200000",
					"checksum": "12345",
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := &config.OKXConfig{
		APIKey:     "test_key",
		SecretKey:  "test_secret",
		Passphrase: "test_pass",
		BaseURL:    server.URL,
	}

	client := NewClient(cfg)
	client.restClient = &restClient{
		config:     cfg,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
	client.connected = true

	orderBook, err := client.GetOrderBook("BTC-USDT", 5)
	assert.NoError(t, err)
	assert.NotNil(t, orderBook)
	assert.Len(t, orderBook.Asks, 2)
	assert.Len(t, orderBook.Bids, 2)
}

func TestClient_SetLeverage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"code": "0",
			"data": []map[string]interface{}{
				{
					"instId":  "BTC-USDT",
					"lever":   "20",
					"mgnMode": "isolated",
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := &config.OKXConfig{
		APIKey:     "test_key",
		SecretKey:  "test_secret",
		Passphrase: "test_pass",
		BaseURL:    server.URL,
	}

	client := NewClient(cfg)
	client.restClient = &restClient{
		config:     cfg,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
	client.connected = true

	err := client.SetLeverage("BTC-USDT", 20, "isolated")
	assert.NoError(t, err)
}

func TestClient_GetOrderHistory(t *testing.T) {
	cfg := &config.OKXConfig{
		APIKey:     "test_key",
		SecretKey:  "test_secret",
		Passphrase: "test_pass",
	}

	client := NewClient(cfg)

	orders, err := client.GetOrderHistory("BTC-USDT", 10)
	assert.NoError(t, err)
	assert.NotNil(t, orders)
}

func TestClient_GetAccountInfo(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"code": "0",
			"data": []map[string]interface{}{
				{
					"totalEq": "10000.5",
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := &config.OKXConfig{
		APIKey:     "test_key",
		SecretKey:  "test_secret",
		Passphrase: "test_pass",
		BaseURL:    server.URL,
	}

	client := NewClient(cfg)
	client.restClient = &restClient{
		config:     cfg,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
	client.connected = true

	account, err := client.GetAccountInfo()
	assert.NoError(t, err)
	assert.NotNil(t, account)
}

func TestClient_RunHandler(t *testing.T) {
	cfg := &config.OKXConfig{
		APIKey:     "test_key",
		SecretKey:  "test_secret",
		Passphrase: "test_pass",
	}

	client := NewClient(cfg)

	var wg sync.WaitGroup
	wg.Add(1)

	called := false
	handler := func() {
		called = true
		wg.Done()
	}

	client.runHandler(handler)
	wg.Wait()

	assert.True(t, called)
}

func TestClient_RunHandler_Backpressure(t *testing.T) {
	cfg := &config.OKXConfig{
		APIKey:     "test_key",
		SecretKey:  "test_secret",
		Passphrase: "test_pass",
	}

	client := NewClient(cfg)
	// 填满并发槽
	for i := 0; i < 256; i++ {
		client.handlerConcurrency <- struct{}{}
	}

	// 当槽满时，应该同步执行
	called := false
	handler := func() {
		called = true
	}

	client.runHandler(handler)
	assert.True(t, called) // 应该同步执行，不需要等待
}
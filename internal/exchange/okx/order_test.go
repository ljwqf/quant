package okx

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ljwqf/quant/internal/config"
	"github.com/ljwqf/quant/pkg/types"
	"github.com/stretchr/testify/assert"
)

// createTestClient 创建测试客户端
func createTestClient(server *httptest.Server) *Client {
	cfg := &config.OKXConfig{
		APIKey:     "test_key",
		SecretKey:  "test_secret",
		Passphrase: "test_pass",
		BaseURL:    server.URL,
	}

	return &Client{
		config: cfg,
		restClient: &restClient{
			config:     cfg,
			httpClient: &http.Client{Timeout: 10 * time.Second},
		},
		connected: true,
	}
}

func TestClient_PlaceOrder_Success(t *testing.T) {
	// 创建模拟服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 验证请求方法
		assert.Equal(t, "POST", r.Method)

		// 返回成功响应
		resp := map[string]interface{}{
			"code": "0",
			"msg":  "",
			"data": []map[string]interface{}{
				{
					"ordId":     "order123",
					"clOrdId":   "client123",
					"instId":    "BTC-USDT",
					"side":      "buy",
					"ordType":   "limit",
					"sz":        "0.1",
					"px":        "50000",
					"ordStatus": "live",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := createTestClient(server)

	// 测试下单
	order := &types.Order{
		Symbol:   "BTC-USDT",
		Side:     types.OrderSideBuy,
		Type:     types.OrderTypeLimit,
		Quantity: 0.1,
		Price:    50000,
		Leverage: 10,
	}

	result, err := client.PlaceOrder(order)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "order123", result.OrderID)
	assert.Equal(t, "BTC-USDT", result.Symbol)
	assert.Equal(t, types.OrderSideBuy, result.Side)
}

func TestClient_PlaceOrder_Market(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"code": "0",
			"data": []map[string]interface{}{
				{
					"ordId":     "market123",
					"instId":    "ETH-USDT",
					"side":      "sell",
					"ordType":   "market",
					"sz":        "1.0",
					"ordStatus": "live",
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := createTestClient(server)

	order := &types.Order{
		Symbol:   "ETH-USDT",
		Side:     types.OrderSideSell,
		Type:     types.OrderTypeMarket,
		Quantity: 1.0,
		Leverage: 5,
	}

	result, err := client.PlaceOrder(order)
	assert.NoError(t, err)
	assert.Equal(t, "market123", result.OrderID)
	assert.Equal(t, types.OrderTypeMarket, result.Type)
}

func TestClient_PlaceOrder_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"code": "50001",
			"msg":  "Invalid API key",
			"data": []map[string]interface{}{},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := createTestClient(server)

	order := &types.Order{
		Symbol:   "BTC-USDT",
		Side:     types.OrderSideBuy,
		Type:     types.OrderTypeMarket,
		Quantity: 0.1,
		Leverage: 1,
	}

	result, err := client.PlaceOrder(order)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "API 错误")
}

func TestClient_CancelOrder_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)

		resp := map[string]interface{}{
			"code": "0",
			"msg":  "",
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := createTestClient(server)

	err := client.CancelOrder("order123")
	assert.NoError(t, err)
}

func TestClient_CancelOrder_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"code": "51401",
			"msg":  "Order does not exist",
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := createTestClient(server)

	err := client.CancelOrder("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Order does not exist")
}

func TestClient_GetOrder_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)

		resp := map[string]interface{}{
			"code": "0",
			"data": []map[string]interface{}{
				{
					"ordId":       "order123",
					"clOrdId":     "client123",
					"instId":      "BTC-USDT",
					"side":        "buy",
					"ordType":     "limit",
					"sz":          "0.5",
					"px":          "45000",
					"avgPx":       "45000",
					"accFillSz":   "0.5",
					"ordStatus":   "filled",
					"lever":       "10",
					"cTime":       "1609459200000",
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := createTestClient(server)

	order, err := client.GetOrder("order123")
	assert.NoError(t, err)
	assert.NotNil(t, order)
	assert.Equal(t, "order123", order.ID)
	assert.Equal(t, "BTC-USDT", order.Symbol)
	assert.Equal(t, types.OrderSideBuy, order.Side)
	assert.Equal(t, types.OrderStatusFilled, order.Status)
	assert.Equal(t, 0.5, order.Quantity)
	assert.Equal(t, 45000.0, order.AveragePrice)
}

func TestClient_GetOrders_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"code": "0",
			"data": []map[string]interface{}{
				{
					"ordId":     "order1",
					"instId":    "BTC-USDT",
					"side":      "buy",
					"ordType":   "limit",
					"sz":        "0.1",
					"px":        "50000",
					"ordStatus": "filled",
					"lever":     "5",
					"cTime":     "1609459200000",
				},
				{
					"ordId":     "order2",
					"instId":    "BTC-USDT",
					"side":      "sell",
					"ordType":   "market",
					"sz":        "0.2",
					"ordStatus": "cancelled",
					"lever":     "5",
					"cTime":     "1609459260000",
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := createTestClient(server)

	orders, err := client.GetOrders("BTC-USDT", 10)
	assert.NoError(t, err)
	assert.Len(t, orders, 2)
	assert.Equal(t, "order1", orders[0].ID)
	assert.Equal(t, "order2", orders[1].ID)
	assert.Equal(t, types.OrderStatusFilled, orders[0].Status)
	assert.Equal(t, types.OrderStatusCancelled, orders[1].Status)
}

func TestClient_GetOrders_Empty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"code": "0",
			"data": []map[string]interface{}{},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := createTestClient(server)

	orders, err := client.GetOrders("UNKNOWN-USDT", 10)
	assert.NoError(t, err)
	assert.Len(t, orders, 0)
}

func TestParseOrderStatus(t *testing.T) {
	tests := []struct {
		input    string
		expected types.OrderStatus
	}{
		{"live", types.OrderStatusPending},
		{"filled", types.OrderStatusFilled},
		{"partially_filled", types.OrderStatusPartially},
		{"cancelled", types.OrderStatusCancelled},
		{"rejected", types.OrderStatusFailed},
		{"unknown", types.OrderStatusPending},
	}

	for _, tt := range tests {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := map[string]interface{}{
				"code": "0",
				"data": []map[string]interface{}{
					{
						"ordId":     "test",
						"instId":    "BTC-USDT",
						"side":      "buy",
						"ordType":   "limit",
						"sz":        "1",
						"ordStatus": tt.input,
						"lever":     "1",
						"cTime":     "1609459200000",
					},
				},
			}
			json.NewEncoder(w).Encode(resp)
		}))

		client := createTestClient(server)

		order, err := client.GetOrder("test")
		assert.NoError(t, err)
		assert.Equal(t, tt.expected, order.Status, "status: %s", tt.input)

		server.Close()
	}
}
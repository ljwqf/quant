package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ljwqf/quant/internal/config"
	"github.com/ljwqf/quant/pkg/types"
)

func readWSMessage(t *testing.T, server *Server) WSMessage {
	t.Helper()
	select {
	case payload := <-server.wsHub.broadcast:
		var msg WSMessage
		require.NoError(t, json.Unmarshal(payload, &msg))
		return msg
	default:
		t.Fatal("expected websocket broadcast")
		return WSMessage{}
	}
}

func TestGetConfigMasksSecrets(t *testing.T) {
	s := NewServer("127.0.0.1", 8765, testConfig(), "", nil)

	recorder := performRequest(t, s, http.MethodGet, "/api/config", nil, "127.0.0.1:12345", nil)
	require.Equal(t, http.StatusOK, recorder.Code)

	body := recorder.Body.String()
	assert.NotContains(t, body, "real-api-key")
	assert.NotContains(t, body, "real-secret")
	assert.NotContains(t, body, "real-passphrase")

	var masked config.Config
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &masked))
	assert.Equal(t, maskedSecretValue, masked.Exchange.OKX.APIKey)
	assert.Equal(t, maskedSecretValue, masked.Exchange.OKX.SecretKey)
	assert.Equal(t, maskedSecretValue, masked.Exchange.OKX.Passphrase)
	assert.Equal(t, "okx-quant", masked.Basic.AppName)
}

func TestSaveConfigPreservesMaskedSecrets(t *testing.T) {
	original := testConfig()
	s := NewServer("127.0.0.1", 8765, original, "", nil)

	updated := testConfig()
	updated.Basic.AppName = "updated-app"
	updated.Exchange.OKX.APIKey = maskedSecretValue
	updated.Exchange.OKX.SecretKey = ""
	updated.Exchange.OKX.Passphrase = maskedSecretValue

	payload, err := json.Marshal(updated)
	require.NoError(t, err)

	recorder := performRequest(t, s, http.MethodPost, "/api/config", bytes.NewReader(payload), "127.0.0.1:12345", map[string]string{"Content-Type": "application/json"})
	require.Equal(t, http.StatusOK, recorder.Code)

	require.NotNil(t, s.cfg)
	assert.Equal(t, "updated-app", s.cfg.Basic.AppName)
	assert.Equal(t, original.Exchange.OKX.APIKey, s.cfg.Exchange.OKX.APIKey)
	assert.Equal(t, original.Exchange.OKX.SecretKey, s.cfg.Exchange.OKX.SecretKey)
	assert.Equal(t, original.Exchange.OKX.Passphrase, s.cfg.Exchange.OKX.Passphrase)
}

func TestSaveConfigRejectsRemoteWithoutToken(t *testing.T) {
	original := testConfig()
	s := NewServer("127.0.0.1", 8765, original, "", nil)

	updated := testConfig()
	updated.Basic.AppName = "should-not-apply"
	payload, err := json.Marshal(updated)
	require.NoError(t, err)

	recorder := performRequest(t, s, http.MethodPost, "/api/config", bytes.NewReader(payload), "203.0.113.10:4321", map[string]string{"Content-Type": "application/json"})

	require.Equal(t, http.StatusForbidden, recorder.Code)
	assert.Equal(t, original.Basic.AppName, s.cfg.Basic.AppName)
}

func TestSaveConfigRejectsInvalidConfig(t *testing.T) {
	original := testConfig()
	s := NewServer("127.0.0.1", 8765, original, "", nil)

	updated := testConfig()
	updated.Basic.AppName = ""
	payload, err := json.Marshal(updated)
	require.NoError(t, err)

	recorder := performRequest(t, s, http.MethodPost, "/api/config", bytes.NewReader(payload), "127.0.0.1:12345", map[string]string{"Content-Type": "application/json"})

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "应用名称不能为空")
	assert.Equal(t, original.Basic.AppName, s.cfg.Basic.AppName)
}

func TestRemoteStrategyStartRequiresToken(t *testing.T) {
	called := false
	s := NewServer("127.0.0.1", 8765, testConfig(), "", &ActionHandlers{
		StartStrategy: func(name string) (*StrategyStatus, error) {
			called = true
			return &StrategyStatus{Name: name, Running: true, Enabled: true}, nil
		},
	})

	recorder := performRequest(t, s, http.MethodPost, "/api/strategy/start/NeedleStrategy", nil, "203.0.113.10:4321", nil)
	require.Equal(t, http.StatusForbidden, recorder.Code)
	assert.False(t, called)
}

func TestCreateOrderWithTokenCallsAction(t *testing.T) {
	t.Setenv("OKX_QUANT_API_TOKEN", "token-123")

	var captured *types.Order
	s := NewServer("127.0.0.1", 8765, testConfig(), "", &ActionHandlers{
		CreateOrder: func(order *types.Order) (*types.OrderResult, error) {
			captured = order
			return &types.OrderResult{
				OrderID:   "ord-1",
				Symbol:    order.Symbol,
				Side:      order.Side,
				Type:      order.Type,
				Quantity:  order.Quantity,
				Price:     order.Price,
				Status:    types.OrderStatusPending,
				Timestamp: time.Now(),
			}, nil
		},
	})

	payload := []byte(`{"symbol":"BTC-USDT","side":"buy","type":"limit","price":100.5,"size":2}`)
	recorder := performRequest(t, s, http.MethodPost, "/api/order/create", bytes.NewReader(payload), "203.0.113.10:4321", map[string]string{
		"Content-Type": "application/json",
		"X-API-Token":  "token-123",
	})

	require.Equal(t, http.StatusOK, recorder.Code)
	require.NotNil(t, captured)
	assert.Equal(t, "BTC-USDT", captured.Symbol)
	assert.Equal(t, types.OrderSideBuy, captured.Side)
	assert.Equal(t, types.OrderTypeLimit, captured.Type)
	assert.Equal(t, 2.0, captured.Quantity)
	assert.Len(t, s.orders, 1)
	assert.Equal(t, "manual", s.orders[0].Strategy)
}

func TestClosePositionWithTokenCallsAction(t *testing.T) {
	t.Setenv("OKX_QUANT_API_TOKEN", "token-123")

	closedSymbol := ""
	s := NewServer("127.0.0.1", 8765, testConfig(), "", &ActionHandlers{
		ClosePosition: func(symbol string) (*types.OrderResult, error) {
			closedSymbol = symbol
			return &types.OrderResult{OrderID: "close-1", Symbol: symbol, Status: types.OrderStatusPending, Timestamp: time.Now()}, nil
		},
	})

	recorder := performRequest(t, s, http.MethodPost, "/api/position/close/BTC-USDT", nil, "203.0.113.10:4321", map[string]string{"X-API-Token": "token-123"})
	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "BTC-USDT", closedSymbol)
}

func TestCreateOrderRejectsInvalidPayload(t *testing.T) {
	t.Setenv("OKX_QUANT_API_TOKEN", "token-123")

	called := false
	s := NewServer("127.0.0.1", 8765, testConfig(), "", &ActionHandlers{
		CreateOrder: func(order *types.Order) (*types.OrderResult, error) {
			called = true
			return nil, nil
		},
	})

	payload := []byte(`{"symbol":"BTC-USDT","side":"buy","type":"limit","price":0,"size":2}`)
	recorder := performRequest(t, s, http.MethodPost, "/api/order/create", bytes.NewReader(payload), "203.0.113.10:4321", map[string]string{
		"Content-Type": "application/json",
		"X-API-Token":  "token-123",
	})

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "limit order price must be greater than 0")
	assert.False(t, called)
}

func TestCreateOrderReturnsBadRequestWhenActionFails(t *testing.T) {
	t.Setenv("OKX_QUANT_API_TOKEN", "token-123")

	s := NewServer("127.0.0.1", 8765, testConfig(), "", &ActionHandlers{
		CreateOrder: func(order *types.Order) (*types.OrderResult, error) {
			return nil, assert.AnError
		},
	})

	payload := []byte(`{"symbol":"BTC-USDT","side":"buy","type":"market","size":2}`)
	recorder := performRequest(t, s, http.MethodPost, "/api/order/create", bytes.NewReader(payload), "203.0.113.10:4321", map[string]string{
		"Content-Type": "application/json",
		"X-API-Token":  "token-123",
	})

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Contains(t, recorder.Body.String(), assert.AnError.Error())
}

func TestStrategyStartReturnsBadRequestWhenActionFails(t *testing.T) {
	t.Setenv("OKX_QUANT_API_TOKEN", "token-123")

	s := NewServer("127.0.0.1", 8765, testConfig(), "", &ActionHandlers{
		StartStrategy: func(name string) (*StrategyStatus, error) {
			return nil, assert.AnError
		},
	})

	recorder := performRequest(t, s, http.MethodPost, "/api/strategy/start/NeedleStrategy", nil, "203.0.113.10:4321", map[string]string{"X-API-Token": "token-123"})

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Contains(t, recorder.Body.String(), assert.AnError.Error())
}

func TestStrategyStopReturnsNotImplementedWithoutHandler(t *testing.T) {
	t.Setenv("OKX_QUANT_API_TOKEN", "token-123")

	s := NewServer("127.0.0.1", 8765, testConfig(), "", &ActionHandlers{})
	recorder := performRequest(t, s, http.MethodPost, "/api/strategy/stop/NeedleStrategy", nil, "203.0.113.10:4321", map[string]string{"X-API-Token": "token-123"})

	require.Equal(t, http.StatusNotImplemented, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "Strategy control is not available")
}

func TestClosePositionRequiresSymbol(t *testing.T) {
	t.Setenv("OKX_QUANT_API_TOKEN", "token-123")

	s := NewServer("127.0.0.1", 8765, testConfig(), "", &ActionHandlers{
		ClosePosition: func(symbol string) (*types.OrderResult, error) {
			return &types.OrderResult{}, nil
		},
	})

	recorder := performRequest(t, s, http.MethodPost, "/api/position/close/", nil, "203.0.113.10:4321", map[string]string{"X-API-Token": "token-123"})

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "symbol is required")
}

func TestGetRebalanceCircuitReturnsState(t *testing.T) {
	s := NewServer("127.0.0.1", 8765, testConfig(), "", &ActionHandlers{
		GetRebalanceCircuit: func() (*RebalanceCircuitInfo, error) {
			return &RebalanceCircuitInfo{Open: true, Strategy: "DeltaNeutralFunding-Pro", Step: "spot_leg", Reason: "rollback failed", Cooldown: "15m0s"}, nil
		},
	})

	recorder := performRequest(t, s, http.MethodGet, "/api/rebalance/circuit", nil, "127.0.0.1:12345", nil)
	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "DeltaNeutralFunding-Pro")
	assert.Contains(t, recorder.Body.String(), "rollback failed")
}

func TestGetRecentRebalanceEventsReturnsCachedHistory(t *testing.T) {
	s := NewServer("127.0.0.1", 8765, testConfig(), "", nil)
	s.BroadcastRebalanceEvent(&RebalanceEventInfo{Type: "open", Strategy: "DeltaNeutralFunding-Pro", Reason: "rollback_failed", Message: "熔断打开", Timestamp: time.Now().Add(-time.Minute)})
	s.BroadcastRebalanceEvent(&RebalanceEventInfo{Type: "recover", Strategy: "DeltaNeutralFunding-Pro", Reason: "startup_reconcile", Message: "开始恢复", Timestamp: time.Now()})

	recorder := performRequest(t, s, http.MethodGet, "/api/rebalance/events", nil, "127.0.0.1:12345", nil)
	require.Equal(t, http.StatusOK, recorder.Code)

	var events []*RebalanceEventInfo
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &events))
	require.Len(t, events, 2)
	assert.Equal(t, "open", events[0].Type)
	assert.Equal(t, "recover", events[1].Type)
}

func TestResetRebalanceCircuitRequiresToken(t *testing.T) {
	called := false
	s := NewServer("127.0.0.1", 8765, testConfig(), "", &ActionHandlers{
		ResetRebalanceCircuit: func(reason string) (*RebalanceCircuitInfo, error) {
			called = true
			return &RebalanceCircuitInfo{Open: false, LastResetReason: reason}, nil
		},
	})

	recorder := performRequest(t, s, http.MethodPost, "/api/rebalance/circuit/reset", bytes.NewReader([]byte(`{"reason":"manual"}`)), "203.0.113.10:4321", map[string]string{"Content-Type": "application/json"})
	require.Equal(t, http.StatusForbidden, recorder.Code)
	assert.False(t, called)
}

func TestResetRebalanceCircuitWithTokenCallsAction(t *testing.T) {
	t.Setenv("OKX_QUANT_API_TOKEN", "token-123")
	var capturedReason string
	s := NewServer("127.0.0.1", 8765, testConfig(), "", &ActionHandlers{
		ResetRebalanceCircuit: func(reason string) (*RebalanceCircuitInfo, error) {
			capturedReason = reason
			return &RebalanceCircuitInfo{Open: false, LastResetReason: reason}, nil
		},
	})

	recorder := performRequest(t, s, http.MethodPost, "/api/rebalance/circuit/reset", bytes.NewReader([]byte(`{"reason":"operator_reset"}`)), "203.0.113.10:4321", map[string]string{"Content-Type": "application/json", "X-API-Token": "token-123"})
	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "operator_reset", capturedReason)
	assert.Contains(t, recorder.Body.String(), "operator_reset")

	stateMsg := readWSMessage(t, s)
	assert.Equal(t, "rebalance_circuit", stateMsg.Type)
	resetMsg := readWSMessage(t, s)
	assert.Equal(t, "rebalance_circuit_reset", resetMsg.Type)
	resetData, ok := resetMsg.Data.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, true, resetData["success"])
	assert.Equal(t, "operator_reset", resetData["reason"])
}

func TestResetRebalanceCircuitBroadcastsFailure(t *testing.T) {
	t.Setenv("OKX_QUANT_API_TOKEN", "token-123")
	s := NewServer("127.0.0.1", 8765, testConfig(), "", &ActionHandlers{
		GetRebalanceCircuit: func() (*RebalanceCircuitInfo, error) {
			return &RebalanceCircuitInfo{Open: true, Strategy: "DeltaNeutralFunding-Pro", Reason: "still open"}, nil
		},
		ResetRebalanceCircuit: func(reason string) (*RebalanceCircuitInfo, error) {
			return nil, assert.AnError
		},
	})

	recorder := performRequest(t, s, http.MethodPost, "/api/rebalance/circuit/reset", bytes.NewReader([]byte(`{"reason":"operator_reset"}`)), "203.0.113.10:4321", map[string]string{"Content-Type": "application/json", "X-API-Token": "token-123"})
	require.Equal(t, http.StatusBadRequest, recorder.Code)

	resetMsg := readWSMessage(t, s)
	assert.Equal(t, "rebalance_circuit_reset", resetMsg.Type)
	stateMsg := readWSMessage(t, s)
	assert.Equal(t, "rebalance_circuit", stateMsg.Type)
}

func TestBroadcastRebalanceEventPublishesLifecyclePayload(t *testing.T) {
	s := NewServer("127.0.0.1", 8765, testConfig(), "", nil)
	s.BroadcastRebalanceEvent(&RebalanceEventInfo{
		Type:      "recover_started",
		Strategy:  "DeltaNeutralFunding-Pro",
		Step:      "spot_leg",
		Reason:    "startup_reconcile",
		Message:   "发现未完成再平衡计划，开始恢复",
		Timestamp: time.Now(),
		Labels:    map[string]string{"event": "rebalance_recover_started"},
		Details:   map[string]interface{}{"plan_id": "plan-1"},
	})

	msg := readWSMessage(t, s)
	assert.Equal(t, "rebalance_event", msg.Type)
	data, ok := msg.Data.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "recover_started", data["type"])
	assert.Equal(t, "DeltaNeutralFunding-Pro", data["strategy"])
	assert.Equal(t, "startup_reconcile", data["reason"])
	labels, ok := data["labels"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "rebalance_recover_started", labels["event"])
}

func TestWebSocketRejectsRemoteWithoutToken(t *testing.T) {
	s := NewServer("127.0.0.1", 8765, testConfig(), "", nil)

	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	req.RemoteAddr = "203.0.113.10:4321"
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	recorder := httptest.NewRecorder()

	s.wsHub.HandleWebSocket(recorder, req)

	require.Equal(t, http.StatusForbidden, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "valid token")
}

func TestWebSocketOriginValidation(t *testing.T) {
	sameOriginReq := httptest.NewRequest(http.MethodGet, "/ws", nil)
	sameOriginReq.Host = "example.com"
	sameOriginReq.Header.Set("Origin", "https://example.com")
	assert.True(t, upgrader.CheckOrigin(sameOriginReq))

	crossOriginReq := httptest.NewRequest(http.MethodGet, "/ws", nil)
	crossOriginReq.Host = "example.com"
	crossOriginReq.Header.Set("Origin", "https://evil.example")
	assert.False(t, upgrader.CheckOrigin(crossOriginReq))
}

func TestWebSocketAllowsTokenWithSameOrigin(t *testing.T) {
	t.Setenv("OKX_QUANT_API_TOKEN", "token-123")

	s := NewServer("127.0.0.1", 0, testConfig(), "", nil)
	go s.wsHub.Run()

	httpServer := httptest.NewServer(s.mux)
	defer httpServer.Close()

	wsURL := websocketURL(t, httpServer.URL+"/ws?token=token-123")
	headers := http.Header{}
	headers.Set("Origin", httpServer.URL)

	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, headers)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusSwitchingProtocols, resp.StatusCode)
	require.NoError(t, conn.Close())
}

func performRequest(t *testing.T, server *Server, method, path string, body *bytes.Reader, remoteAddr string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()

	var requestBody *bytes.Reader
	if body == nil {
		requestBody = bytes.NewReader(nil)
	} else {
		requestBody = body
	}

	req := httptest.NewRequest(method, path, requestBody)
	req.RemoteAddr = remoteAddr
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	recorder := httptest.NewRecorder()
	server.mux.ServeHTTP(recorder, req)
	return recorder
}

func testConfig() *config.Config {
	return &config.Config{
		Basic: config.BasicConfig{AppName: "okx-quant", Env: "test", LogLevel: "info", LogFile: "./logs/test.log"},
		Exchange: config.ExchangeConfig{OKX: config.OKXConfig{
			APIKey:     "real-api-key",
			SecretKey:  "real-secret",
			Passphrase: "real-passphrase",
			BaseURL:    "https://www.okx.com",
			WSURL:      "wss://ws.okx.com:8443/ws/v5/public",
			Timeout:    30 * time.Second,
			RetryCount: 3,
		}},
		Risk:     config.RiskConfig{Enable: true, MaxPositionSize: 10000, MaxDailyLoss: 1000, MaxDrawdown: 0.2, StopLossPercent: 0.05, TakeProfitPercent: 0.1, MaxTradesPerDay: 100},
		Backtest: config.BacktestConfig{Enable: false, InitialBalance: 10000},
		Monitoring: config.MonitoringConfig{
			Enable:         true,
			CheckInterval:  30 * time.Second,
			AlertThreshold: config.AlertThreshold{MaxDrawdown: 0.2, MaxLoss: 1000, PositionLimit: 10000, OrderTimeout: time.Minute},
			Metrics:        config.MetricsConfig{Enable: true},
			Alert:          config.AlertConfig{Enable: true, WebhookURL: "https://example.com/webhook"},
		},
		Strategy: config.StrategyConfig{Enable: false, Name: "example"},
		Server:   config.ServerConfig{Enable: true, Host: "127.0.0.1", Port: 8765},
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}

func websocketURL(t *testing.T, rawURL string) string {
	t.Helper()
	parsed, err := url.Parse(rawURL)
	require.NoError(t, err)
	if parsed.Scheme == "http" {
		parsed.Scheme = "ws"
	} else {
		parsed.Scheme = "wss"
	}
	return parsed.String()
}

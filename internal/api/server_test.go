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
	"github.com/ljwqf/quant/internal/config"
	"github.com/ljwqf/quant/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestSaveConfigRejectsLocalWithoutTokenWhenAPITokenConfigured(t *testing.T) {
	t.Setenv("OKX_QUANT_API_TOKEN", "token-123")
	original := testConfig()
	s := NewServer("127.0.0.1", 8765, original, "", nil)

	updated := testConfig()
	updated.Basic.AppName = "should-not-apply"
	payload, err := json.Marshal(updated)
	require.NoError(t, err)

	recorder := performRequest(t, s, http.MethodPost, "/api/config", bytes.NewReader(payload), "127.0.0.1:12345", map[string]string{"Content-Type": "application/json"})

	require.Equal(t, http.StatusForbidden, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "valid X-API-Token")
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

func TestLocalStrategyStartRequiresTokenWhenAPITokenConfigured(t *testing.T) {
	t.Setenv("OKX_QUANT_API_TOKEN", "token-123")
	called := false
	s := NewServer("127.0.0.1", 8765, testConfig(), "", &ActionHandlers{
		StartStrategy: func(name string) (*StrategyStatus, error) {
			called = true
			return &StrategyStatus{Name: name, Running: true, Enabled: true}, nil
		},
	})

	recorder := performRequest(t, s, http.MethodPost, "/api/strategy/start/NeedleStrategy", nil, "127.0.0.1:12345", nil)
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

func TestHealthEndpointReturnsOK(t *testing.T) {
	s := NewServer("127.0.0.1", 8765, testConfig(), "", nil)

	recorder := performRequest(t, s, http.MethodGet, "/health", nil, "127.0.0.1:12345", nil)
	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"status":"ok"`)
}

func TestReadyEndpointReturnsUnavailableWhenSystemNotRunning(t *testing.T) {
	s := NewServer("127.0.0.1", 8765, testConfig(), "", nil)

	recorder := performRequest(t, s, http.MethodGet, "/ready", nil, "127.0.0.1:12345", nil)
	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"ready":false`)
	assert.Contains(t, recorder.Body.String(), `"system_status":"not running"`)
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

// Additional tests for better coverage

func TestSetManualTradeManager(t *testing.T) {
	s := NewServer("127.0.0.1", 8765, testConfig(), "", nil)

	// Test that nil doesn't panic
	s.SetManualTradeManager(nil)
	assert.Nil(t, s.manualTradeMgr)
}

func TestSetAnalyzer(t *testing.T) {
	s := NewServer("127.0.0.1", 8765, testConfig(), "", nil)

	s.SetAnalyzer(nil) // Should not panic
	assert.Nil(t, s.analyzer)
}

func TestSetDataService(t *testing.T) {
	s := NewServer("127.0.0.1", 8765, testConfig(), "", nil)

	s.SetDataService(nil) // Should not panic
	assert.Nil(t, s.dataService)
}

func TestSetAlertService(t *testing.T) {
	s := NewServer("127.0.0.1", 8765, testConfig(), "", nil)

	s.SetAlertService(nil) // Should not panic
	assert.Nil(t, s.alertService)
}

func TestUpdateSystemStatus(t *testing.T) {
	s := NewServer("127.0.0.1", 8765, testConfig(), "", nil)

	status := &SystemStatus{
		Running:           true,
		ExchangeConnected: true,
		AccountBalance:    10000.0,
	}
	s.UpdateSystemStatus(status)
	assert.True(t, s.systemStatus.Running)

	// Broadcast to websocket
	msg := readWSMessage(t, s)
	assert.Equal(t, "status", msg.Type)
}

func TestUpdateStrategyStatus(t *testing.T) {
	s := NewServer("127.0.1", 8765, testConfig(), "", nil)

	status := &StrategyStatus{
		Name:    "NeedleStrategy",
		Enabled: true,
		Running: true,
		PnL:     100.0,
	}
	s.UpdateStrategyStatus("NeedleStrategy", status)

	stored, ok := s.strategies["NeedleStrategy"]
	assert.True(t, ok)
	assert.True(t, stored.Running)

	// Broadcast to websocket
	msg := readWSMessage(t, s)
	assert.Equal(t, "strategy", msg.Type)
}

func TestUpdatePositions(t *testing.T) {
	s := NewServer("127.0.0.1", 8765, testConfig(), "", nil)

	positions := []*PositionInfo{
		{Symbol: "BTC-USDT", Side: "long", Size: 0.1, EntryPrice: 50000},
		{Symbol: "ETH-USDT", Side: "short", Size: 1.0, EntryPrice: 3000},
	}

	s.UpdatePositions(positions)
	assert.Len(t, s.positions, 2)

	// Broadcast to websocket
	msg := readWSMessage(t, s)
	assert.Equal(t, "positions", msg.Type)
}

func TestAddSignal(t *testing.T) {
	s := NewServer("127.0.0.1", 8765, testConfig(), "", nil)

	signal := &SignalInfo{
		Strategy:   "NeedleStrategy",
		Symbol:     "BTC-USDT",
		Side:       "buy",
		Confidence: 0.8,
	}

	s.AddSignal(signal)
	assert.Len(t, s.signals, 1)
	assert.Equal(t, "NeedleStrategy", s.signals[0].Strategy)

	// Broadcast to websocket
	msg := readWSMessage(t, s)
	assert.Equal(t, "signal", msg.Type)
}

func TestHandleStatus(t *testing.T) {
	s := NewServer("127.0.0.1", 8765, testConfig(), "", nil)
	s.UpdateSystemStatus(&SystemStatus{Running: true, ExchangeConnected: true})

	recorder := performRequest(t, s, http.MethodGet, "/api/status", nil, "127.0.0.1:12345", nil)
	require.Equal(t, http.StatusOK, recorder.Code)

	var status SystemStatus
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &status))
	assert.True(t, status.Running)
}

func TestHandleStrategies(t *testing.T) {
	s := NewServer("127.0.0.1", 8765, testConfig(), "", nil)
	s.UpdateStrategyStatus("NeedleStrategy", &StrategyStatus{Running: true, Enabled: true})
	s.UpdateStrategyStatus("MMPEngine", &StrategyStatus{Running: false, Enabled: false})

	recorder := performRequest(t, s, http.MethodGet, "/api/strategies", nil, "127.0.0.1:12345", nil)
	require.Equal(t, http.StatusOK, recorder.Code)

	var strategies []*StrategyStatus
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &strategies))
	assert.Len(t, strategies, 2)
}

func TestHandlePositions(t *testing.T) {
	s := NewServer("127.0.0.1", 8765, testConfig(), "", nil)
	s.UpdatePositions([]*PositionInfo{
		{Symbol: "BTC-USDT", Side: "long", Size: 0.1, EntryPrice: 50000},
	})

	recorder := performRequest(t, s, http.MethodGet, "/api/positions", nil, "127.0.0.1:12345", nil)
	require.Equal(t, http.StatusOK, recorder.Code)

	var positions []*PositionInfo
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &positions))
	assert.Len(t, positions, 1)
}

func TestHandleOrders(t *testing.T) {
	s := NewServer("127.0.0.1", 8765, testConfig(), "", nil)
	s.AddOrder(&OrderInfo{OrderID: "ord-1", Symbol: "BTC-USDT", Status: "pending"})

	recorder := performRequest(t, s, http.MethodGet, "/api/orders", nil, "127.0.0.1:12345", nil)
	require.Equal(t, http.StatusOK, recorder.Code)

	var orders []*OrderInfo
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &orders))
	assert.Len(t, orders, 1)
}

func TestHandleSignals(t *testing.T) {
	s := NewServer("127.0.0.1", 8765, testConfig(), "", nil)
	s.AddSignal(&SignalInfo{Strategy: "test", Symbol: "BTC-USDT", Side: "buy"})

	recorder := performRequest(t, s, http.MethodGet, "/api/signals", nil, "127.0.0.1:12345", nil)
	require.Equal(t, http.StatusOK, recorder.Code)

	var signals []*SignalInfo
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &signals))
	assert.Len(t, signals, 1)
}

func TestHandleAccount(t *testing.T) {
	s := NewServer("127.0.0.1", 8765, testConfig(), "", nil)

	recorder := performRequest(t, s, http.MethodGet, "/api/account", nil, "127.0.0.1:12345", nil)
	require.Equal(t, http.StatusOK, recorder.Code)
}

func TestStrategyStartSuccess(t *testing.T) {
	t.Setenv("OKX_QUANT_API_TOKEN", "token-123")

	s := NewServer("127.0.0.1", 8765, testConfig(), "", &ActionHandlers{
		StartStrategy: func(name string) (*StrategyStatus, error) {
			return &StrategyStatus{Name: name, Running: true, Enabled: true}, nil
		},
	})

	recorder := performRequest(t, s, http.MethodPost, "/api/strategy/start/TestStrategy", nil, "203.0.113.10:4321", map[string]string{"X-API-Token": "token-123"})
	require.Equal(t, http.StatusOK, recorder.Code)
}

func TestStrategyStopSuccess(t *testing.T) {
	t.Setenv("OKX_QUANT_API_TOKEN", "token-123")

	s := NewServer("127.0.0.1", 8765, testConfig(), "", &ActionHandlers{
		StopStrategy: func(name string) (*StrategyStatus, error) {
			return &StrategyStatus{Name: name, Running: false, Enabled: true}, nil
		},
	})

	recorder := performRequest(t, s, http.MethodPost, "/api/strategy/stop/TestStrategy", nil, "203.0.113.10:4321", map[string]string{"X-API-Token": "token-123"})
	require.Equal(t, http.StatusOK, recorder.Code)
}

func TestStrategyStopFails(t *testing.T) {
	t.Setenv("OKX_QUANT_API_TOKEN", "token-123")

	s := NewServer("127.0.0.1", 8765, testConfig(), "", &ActionHandlers{
		StopStrategy: func(name string) (*StrategyStatus, error) {
			return nil, assert.AnError
		},
	})

	recorder := performRequest(t, s, http.MethodPost, "/api/strategy/stop/TestStrategy", nil, "203.0.113.10:4321", map[string]string{"X-API-Token": "token-123"})
	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestParseTrustedProxies(t *testing.T) {
	tests := []struct {
		input    []string
		expected int // number of networks expected
	}{
		{[]string{}, 0},
		{[]string{"127.0.0.1"}, 1},
		{[]string{"127.0.0.1", "10.0.0.0/8"}, 2},
		{[]string{"  127.0.0.1  ", "  10.0.0.1  "}, 2},
	}

	for _, tt := range tests {
		result := parseTrustedProxies(tt.input)
		assert.Len(t, result, tt.expected)
	}
}

func TestStartStop(t *testing.T) {
	s := NewServer("127.0.0.1", 0, testConfig(), "", nil)

	// Start the server in a goroutine
	go func() {
		s.Start()
	}()

	// Wait a bit for the server to start
	time.Sleep(100 * time.Millisecond)
	assert.True(t, s.systemStatus.Running)

	// Stop the server
	err := s.Stop()
	require.NoError(t, err)
	assert.False(t, s.systemStatus.Running)
}

func TestIsTrustedRequest(t *testing.T) {
	s := NewServer("127.0.0.1", 8765, testConfig(), "", nil)

	tests := []struct {
		remoteAddr string
		expected   bool
	}{
		{"127.0.0.1:12345", true},
		{"localhost:12345", true},
		{"[::1]:12345", true},
		{"192.168.1.1:12345", false},
		{"10.0.0.1:12345", false},
	}

	for _, tt := range tests {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = tt.remoteAddr
		result := s.isTrustedRequest(req)
		assert.Equal(t, tt.expected, result, "remoteAddr=%s", tt.remoteAddr)
	}
}

func TestHasValidToken(t *testing.T) {
	s := NewServer("127.0.0.1", 8765, testConfig(), "", nil)
	s.apiToken = "test-token"

	assert.True(t, s.hasValidToken("test-token"))
	assert.False(t, s.hasValidToken("wrong-token"))
	assert.False(t, s.hasValidToken(""))
}

func TestMaskSecret(t *testing.T) {
	assert.Equal(t, maskedSecretValue, maskSecret("secret-value"))
	assert.Equal(t, maskedSecretValue, maskSecret("  secret-value  "))
	assert.Equal(t, "", maskSecret(""))
	assert.Equal(t, "", maskSecret("   "))
}

func TestShouldPreserveSecret(t *testing.T) {
	assert.True(t, shouldPreserveSecret(""))
	assert.True(t, shouldPreserveSecret("   "))
	assert.True(t, shouldPreserveSecret(maskedSecretValue))
	assert.False(t, shouldPreserveSecret("real-secret"))
}

func TestBuildOrderFromRequest(t *testing.T) {
	tests := []struct {
		name        string
		req         *createOrderRequest
		expectError bool
	}{
		{"valid limit order", &createOrderRequest{Symbol: "BTC-USDT", Side: "buy", Type: "limit", Price: 50000, Size: 0.1}, false},
		{"valid market order", &createOrderRequest{Symbol: "BTC-USDT", Side: "sell", Type: "market", Size: 0.1}, false},
		{"missing symbol", &createOrderRequest{Side: "buy", Type: "limit", Price: 50000, Size: 0.1}, true},
		{"missing size", &createOrderRequest{Symbol: "BTC-USDT", Side: "buy", Type: "limit", Price: 50000}, true},
		{"invalid side", &createOrderRequest{Symbol: "BTC-USDT", Side: "invalid", Type: "limit", Price: 50000, Size: 0.1}, true},
		{"invalid type", &createOrderRequest{Symbol: "BTC-USDT", Side: "buy", Type: "invalid", Price: 50000, Size: 0.1}, true},
		{"limit without price", &createOrderRequest{Symbol: "BTC-USDT", Side: "buy", Type: "limit", Size: 0.1}, true},
		{"nil request", nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			order, err := buildOrderFromRequest(tt.req)
			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, order)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, order)
				assert.Equal(t, tt.req.Symbol, order.Symbol)
			}
		})
	}
}

func TestHandleManualCreateOrderWithoutManager(t *testing.T) {
	s := NewServer("127.0.0.1", 8765, testConfig(), "", nil)

	payload := []byte(`{"symbol":"BTC-USDT","side":"buy","type":"market","size":0.1}`)
	recorder := performRequest(t, s, http.MethodPost, "/api/manual/order", bytes.NewReader(payload), "127.0.0.1:12345", map[string]string{"Content-Type": "application/json"})

	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "Manual trading not enabled")
}

func TestHandleManualListOrdersWithoutManager(t *testing.T) {
	s := NewServer("127.0.0.1", 8765, testConfig(), "", nil)

	recorder := performRequest(t, s, http.MethodGet, "/api/manual/orders", nil, "127.0.0.1:12345", nil)

	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
}

func TestHandleGetTickerWithActions(t *testing.T) {
	s := NewServer("127.0.0.1", 8765, testConfig(), "", &ActionHandlers{
		GetTicker: func(symbol string) (*types.Tick, error) {
			return &types.Tick{Symbol: symbol, Price: 50000.0}, nil
		},
	})

	recorder := performRequest(t, s, http.MethodGet, "/api/market/ticker?symbol=BTC-USDT", nil, "127.0.0.1:12345", nil)
	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "BTC-USDT")
}

func TestHandleGetBarsWithActions(t *testing.T) {
	s := NewServer("127.0.0.1", 8765, testConfig(), "", &ActionHandlers{
		GetBars: func(symbol string, interval string, limit int) ([]*types.Bar, error) {
			return []*types.Bar{{Symbol: symbol}}, nil
		},
	})

	recorder := performRequest(t, s, http.MethodGet, "/api/market/bars?symbol=BTC-USDT&interval=1m", nil, "127.0.0.1:12345", nil)
	require.Equal(t, http.StatusOK, recorder.Code)
}

func TestHandleGetOrderBookWithActions(t *testing.T) {
	s := NewServer("127.0.0.1", 8765, testConfig(), "", &ActionHandlers{
		GetOrderBook: func(symbol string, depth int) (*types.OrderBook, error) {
			return &types.OrderBook{Symbol: symbol}, nil
		},
	})

	recorder := performRequest(t, s, http.MethodGet, "/api/market/orderbook?symbol=BTC-USDT&depth=5", nil, "127.0.0.1:12345", nil)
	require.Equal(t, http.StatusOK, recorder.Code)
}

func TestHandleLLMWithoutAnalyzer(t *testing.T) {
	s := NewServer("127.0.0.1", 8765, testConfig(), "", nil)

	// Test LLM analyze positions - GET request
	recorder := performRequest(t, s, http.MethodGet, "/api/llm/analyze/positions", nil, "127.0.0.1:12345", nil)
	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)

	// Test LLM history - GET request
	recorder = performRequest(t, s, http.MethodGet, "/api/llm/history", nil, "127.0.0.1:12345", nil)
	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
}

func TestHandleLLMAnalyzeTradeWithoutAnalyzer(t *testing.T) {
	s := NewServer("127.0.0.1", 8765, testConfig(), "", nil)

	// Test LLM analyze trade - POST request
	payload := []byte(`{"symbol":"BTC-USDT","side":"buy","size":0.1}`)
	recorder := performRequest(t, s, http.MethodPost, "/api/llm/analyze/trade", bytes.NewReader(payload), "127.0.0.1:12345", map[string]string{"Content-Type": "application/json"})
	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
}

func TestHandleLLMAnalyzeMarketWithoutAnalyzer(t *testing.T) {
	s := NewServer("127.0.0.1", 8765, testConfig(), "", nil)

	payload := []byte(`{"symbol":"BTC-USDT"}`)
	recorder := performRequest(t, s, http.MethodPost, "/api/llm/analyze/market", bytes.NewReader(payload), "127.0.0.1:12345", map[string]string{"Content-Type": "application/json"})
	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
}

func TestHandleLLMAnalyzeOrdersWithoutAnalyzer(t *testing.T) {
	s := NewServer("127.0.0.1", 8765, testConfig(), "", nil)

	payload := []byte(`{"orders":[{"symbol":"BTC-USDT","side":"buy","size":0.1}]}`)
	recorder := performRequest(t, s, http.MethodPost, "/api/llm/analyze/orders", bytes.NewReader(payload), "127.0.0.1:12345", map[string]string{"Content-Type": "application/json"})
	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
}

func TestHandleDataWithoutService(t *testing.T) {
	s := NewServer("127.0.0.1", 8765, testConfig(), "", nil)

	// Test get news
	recorder := performRequest(t, s, http.MethodGet, "/api/data/news", nil, "127.0.0.1:12345", nil)
	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)

	// Test get events
	recorder = performRequest(t, s, http.MethodGet, "/api/data/events", nil, "127.0.0.1:12345", nil)
	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)

	// Test collect now
	recorder = performRequest(t, s, http.MethodPost, "/api/data/collect", nil, "127.0.0.1:12345", nil)
	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
}

func TestHandleAlertsWithoutService(t *testing.T) {
	s := NewServer("127.0.0.1", 8765, testConfig(), "", nil)

	// Test get alerts
	recorder := performRequest(t, s, http.MethodGet, "/api/alerts", nil, "127.0.0.1:12345", nil)
	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)

	// Test send alert
	payload := []byte(`{"title":"Test","message":"Test message"}`)
	recorder = performRequest(t, s, http.MethodPost, "/api/alerts/send", bytes.NewReader(payload), "127.0.0.1:12345", map[string]string{"Content-Type": "application/json"})
	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
}

func TestCloneRebalanceEventInfo(t *testing.T) {
	original := &RebalanceEventInfo{
		Type:      "open",
		Strategy:  "DeltaNeutralFunding-Pro",
		Step:      "spot_leg",
		Reason:    "rollback_failed",
		Message:   "Test message",
		Timestamp: time.Now(),
		Labels:    map[string]string{"event": "test"},
		Details:   map[string]interface{}{"plan_id": "plan-1"},
		Circuit:   &RebalanceCircuitInfo{Open: true, Strategy: "DeltaNeutralFunding-Pro"},
	}

	cloned := cloneRebalanceEventInfo(original)
	assert.Equal(t, original.Type, cloned.Type)
	assert.Equal(t, original.Strategy, cloned.Strategy)
	assert.Equal(t, original.Labels["event"], cloned.Labels["event"])
	assert.Equal(t, original.Circuit.Open, cloned.Circuit.Open)

	// Verify it's a deep copy
	cloned.Labels["event"] = "modified"
	assert.NotEqual(t, original.Labels["event"], cloned.Labels["event"])
}

func TestGetRecentRebalanceEventsEmpty(t *testing.T) {
	s := NewServer("127.0.0.1", 8765, testConfig(), "", nil)

	events := s.GetRecentRebalanceEvents(10)
	assert.Empty(t, events)
}

func TestHandleManualCancelOrderWithoutManager(t *testing.T) {
	s := NewServer("127.0.0.1", 8765, testConfig(), "", nil)

	recorder := performRequest(t, s, http.MethodDelete, "/api/manual/order/ord-123", nil, "127.0.0.1:12345", nil)
	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
}

func TestHandleManualClosePositionWithoutManager(t *testing.T) {
	s := NewServer("127.0.0.1", 8765, testConfig(), "", nil)

	payload := []byte(`{"symbol":"BTC-USDT"}`)
	recorder := performRequest(t, s, http.MethodPost, "/api/manual/position/close", bytes.NewReader(payload), "127.0.0.1:12345", map[string]string{"Content-Type": "application/json"})
	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
}

func TestHandleManualSetTpSlWithoutManager(t *testing.T) {
	s := NewServer("127.0.0.1", 8765, testConfig(), "", nil)

	payload := []byte(`{"symbol":"BTC-USDT","take_profit":55000,"stop_loss":45000}`)
	recorder := performRequest(t, s, http.MethodPost, "/api/manual/position/tp-sl", bytes.NewReader(payload), "127.0.0.1:12345", map[string]string{"Content-Type": "application/json"})
	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
}

func TestHandleManualSetLeverageWithoutManager(t *testing.T) {
	s := NewServer("127.0.0.1", 8765, testConfig(), "", nil)

	payload := []byte(`{"symbol":"BTC-USDT","leverage":10}`)
	recorder := performRequest(t, s, http.MethodPost, "/api/manual/position/leverage", bytes.NewReader(payload), "127.0.0.1:12345", map[string]string{"Content-Type": "application/json"})
	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
}

func TestHandleManualSetTrailingStopWithoutManager(t *testing.T) {
	s := NewServer("127.0.0.1", 8765, testConfig(), "", nil)

	payload := []byte(`{"symbol":"BTC-USDT","stop_distance":500}`)
	recorder := performRequest(t, s, http.MethodPost, "/api/manual/position/trailing-stop", bytes.NewReader(payload), "127.0.0.1:12345", map[string]string{"Content-Type": "application/json"})
	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
}

func TestHandleCreateTimedOrderWithoutManager(t *testing.T) {
	s := NewServer("127.0.0.1", 8765, testConfig(), "", nil)

	payload := []byte(`{"symbol":"BTC-USDT","side":"buy","size":0.1,"execute_at":"2024-01-01T12:00:00Z"}`)
	recorder := performRequest(t, s, http.MethodPost, "/api/manual/timed-order", bytes.NewReader(payload), "127.0.0.1:12345", map[string]string{"Content-Type": "application/json"})
	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
}

func TestHandleCancelTimedOrderWithoutManager(t *testing.T) {
	s := NewServer("127.0.0.1", 8765, testConfig(), "", nil)

	recorder := performRequest(t, s, http.MethodDelete, "/api/manual/timed-order/ord-123", nil, "127.0.0.1:12345", nil)
	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
}

func TestHandleListTimedOrdersWithoutManager(t *testing.T) {
	s := NewServer("127.0.0.1", 8765, testConfig(), "", nil)

	recorder := performRequest(t, s, http.MethodGet, "/api/manual/timed-orders", nil, "127.0.0.1:12345", nil)
	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
}

func TestHandleCollectNowWithoutService(t *testing.T) {
	s := NewServer("127.0.0.1", 8765, testConfig(), "", nil)

	recorder := performRequest(t, s, http.MethodPost, "/api/data/collect", nil, "127.0.0.1:12345", nil)
	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
}

func TestHandleSendAlertWithoutService(t *testing.T) {
	s := NewServer("127.0.0.1", 8765, testConfig(), "", nil)

	payload := []byte(`{"title":"Test","message":"Test message"}`)
	recorder := performRequest(t, s, http.MethodPost, "/api/alerts/send", bytes.NewReader(payload), "127.0.0.1:12345", map[string]string{"Content-Type": "application/json"})
	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
}

func TestHandleManualCreateOrderValidation(t *testing.T) {
	// Test with nil manager
	s := NewServer("127.0.0.1", 8765, testConfig(), "", nil)

	// Test missing symbol
	payload := []byte(`{"side":"buy","type":"market","size":0.1}`)
	recorder := performRequest(t, s, http.MethodPost, "/api/manual/order", bytes.NewReader(payload), "127.0.0.1:12345", map[string]string{"Content-Type": "application/json"})
	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
}

func TestHandleCreateTimedOrderValidation(t *testing.T) {
	s := NewServer("127.0.0.1", 8765, testConfig(), "", nil)

	// Test missing symbol - still returns service unavailable first
	payload := []byte(`{"side":"buy","size":0.1,"execute_at":"2024-01-01T12:00:00Z"}`)
	recorder := performRequest(t, s, http.MethodPost, "/api/manual/timed-order", bytes.NewReader(payload), "127.0.0.1:12345", map[string]string{"Content-Type": "application/json"})
	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
}

func TestHandleRebalanceCircuitWithoutActions(t *testing.T) {
	s := NewServer("127.0.0.1", 8765, testConfig(), "", nil)

	recorder := performRequest(t, s, http.MethodGet, "/api/rebalance/circuit", nil, "127.0.0.1:12345", nil)
	require.Equal(t, http.StatusNotImplemented, recorder.Code)
}

func TestHandleRebalanceCircuitResetWithoutActions(t *testing.T) {
	s := NewServer("127.0.0.1", 8765, testConfig(), "", nil)

	payload := []byte(`{"reason":"manual"}`)
	recorder := performRequest(t, s, http.MethodPost, "/api/rebalance/circuit/reset", bytes.NewReader(payload), "127.0.0.1:12345", map[string]string{"Content-Type": "application/json"})
	require.Equal(t, http.StatusNotImplemented, recorder.Code)
}

func TestHandleCreateOrderWithoutActions(t *testing.T) {
	s := NewServer("127.0.0.1", 8765, testConfig(), "", nil)

	payload := []byte(`{"symbol":"BTC-USDT","side":"buy","type":"market","size":0.1}`)
	recorder := performRequest(t, s, http.MethodPost, "/api/order/create", bytes.NewReader(payload), "127.0.0.1:12345", map[string]string{"Content-Type": "application/json"})
	require.Equal(t, http.StatusNotImplemented, recorder.Code)
}

func TestHandleClosePositionWithoutActions(t *testing.T) {
	s := NewServer("127.0.0.1", 8765, testConfig(), "", nil)

	recorder := performRequest(t, s, http.MethodPost, "/api/position/close/BTC-USDT", nil, "127.0.0.1:12345", nil)
	require.Equal(t, http.StatusNotImplemented, recorder.Code)
}

func TestHandleStrategyStartWithoutActions(t *testing.T) {
	s := NewServer("127.0.0.1", 8765, testConfig(), "", nil)

	recorder := performRequest(t, s, http.MethodPost, "/api/strategy/start/TestStrategy", nil, "127.0.0.1:12345", nil)
	require.Equal(t, http.StatusNotImplemented, recorder.Code)
}

func TestHandleStrategyStopWithoutActions(t *testing.T) {
	s := NewServer("127.0.0.1", 8765, testConfig(), "", nil)

	recorder := performRequest(t, s, http.MethodPost, "/api/strategy/stop/TestStrategy", nil, "127.0.0.1:12345", nil)
	require.Equal(t, http.StatusNotImplemented, recorder.Code)
}

func TestHandleRebalanceEventsWithLimit(t *testing.T) {
	s := NewServer("127.0.0.1", 8765, testConfig(), "", nil)

	// Add some events
	for i := 0; i < 5; i++ {
		s.BroadcastRebalanceEvent(&RebalanceEventInfo{
			Type:      "test",
			Strategy:  "TestStrategy",
			Timestamp: time.Now(),
		})
	}

	recorder := performRequest(t, s, http.MethodGet, "/api/rebalance/events", nil, "127.0.0.1:12345", nil)
	require.Equal(t, http.StatusOK, recorder.Code)

	var events []*RebalanceEventInfo
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &events))
	assert.Len(t, events, 5)
}

func TestUpdateRebalanceCircuit(t *testing.T) {
	s := NewServer("127.0.0.1", 8765, testConfig(), "", nil)

	info := &RebalanceCircuitInfo{
		Open:     true,
		Strategy: "TestStrategy",
		Reason:   "test",
	}
	s.UpdateRebalanceCircuit(info)

	msg := readWSMessage(t, s)
	assert.Equal(t, "rebalance_circuit", msg.Type)
}

func TestBroadcastRebalanceCircuitReset(t *testing.T) {
	s := NewServer("127.0.0.1", 8765, testConfig(), "", nil)

	event := &RebalanceCircuitResetEvent{
		Success:   true,
		Message:   "test reset",
		Timestamp: time.Now(),
	}
	s.BroadcastRebalanceCircuitReset(event)

	msg := readWSMessage(t, s)
	assert.Equal(t, "rebalance_circuit_reset", msg.Type)
}

func TestHandleGetNewsWithoutService(t *testing.T) {
	s := NewServer("127.0.0.1", 8765, testConfig(), "", nil)

	recorder := performRequest(t, s, http.MethodGet, "/api/data/news", nil, "127.0.0.1:12345", nil)
	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
}

func TestHandleGetEventsWithoutService(t *testing.T) {
	s := NewServer("127.0.0.1", 8765, testConfig(), "", nil)

	recorder := performRequest(t, s, http.MethodGet, "/api/data/events", nil, "127.0.0.1:12345", nil)
	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
}

func TestHandleGetAlertsWithoutService(t *testing.T) {
	s := NewServer("127.0.0.1", 8765, testConfig(), "", nil)

	recorder := performRequest(t, s, http.MethodGet, "/api/alerts", nil, "127.0.0.1:12345", nil)
	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
}

func TestHandleConfigMethodNotAllowed(t *testing.T) {
	s := NewServer("127.0.0.1", 8765, testConfig(), "", nil)

	recorder := performRequest(t, s, http.MethodDelete, "/api/config", nil, "127.0.0.1:12345", nil)
	require.Equal(t, http.StatusMethodNotAllowed, recorder.Code)
}

func TestHandleManualCreateOrderWrongMethod(t *testing.T) {
	s := NewServer("127.0.0.1", 8765, testConfig(), "", nil)

	recorder := performRequest(t, s, http.MethodGet, "/api/manual/order", nil, "127.0.0.1:12345", nil)
	require.Equal(t, http.StatusMethodNotAllowed, recorder.Code)
}

func TestHandleManualListOrdersWrongMethod(t *testing.T) {
	s := NewServer("127.0.0.1", 8765, testConfig(), "", nil)

	recorder := performRequest(t, s, http.MethodPost, "/api/manual/orders", nil, "127.0.0.1:12345", nil)
	require.Equal(t, http.StatusMethodNotAllowed, recorder.Code)
}

func TestHandleGetTickerWrongMethod(t *testing.T) {
	s := NewServer("127.0.0.1", 8765, testConfig(), "", &ActionHandlers{
		GetTicker: func(symbol string) (*types.Tick, error) {
			return &types.Tick{Symbol: symbol}, nil
		},
	})

	recorder := performRequest(t, s, http.MethodPost, "/api/market/ticker", nil, "127.0.0.1:12345", nil)
	require.Equal(t, http.StatusMethodNotAllowed, recorder.Code)
}

func TestHandleGetBarsWrongMethod(t *testing.T) {
	s := NewServer("127.0.0.1", 8765, testConfig(), "", &ActionHandlers{
		GetBars: func(symbol string, interval string, limit int) ([]*types.Bar, error) {
			return []*types.Bar{}, nil
		},
	})

	recorder := performRequest(t, s, http.MethodPost, "/api/market/bars", nil, "127.0.0.1:12345", nil)
	require.Equal(t, http.StatusMethodNotAllowed, recorder.Code)
}

func TestHandleRebalanceCircuitWrongMethod(t *testing.T) {
	s := NewServer("127.0.0.1", 8765, testConfig(), "", nil)

	recorder := performRequest(t, s, http.MethodPost, "/api/rebalance/circuit", nil, "127.0.0.1:12345", nil)
	require.Equal(t, http.StatusMethodNotAllowed, recorder.Code)
}

func TestHandleRebalanceEventsWrongMethod(t *testing.T) {
	s := NewServer("127.0.0.1", 8765, testConfig(), "", nil)

	recorder := performRequest(t, s, http.MethodPost, "/api/rebalance/events", nil, "127.0.0.1:12345", nil)
	require.Equal(t, http.StatusMethodNotAllowed, recorder.Code)
}

func TestHandleLLMAnalyzeTradeWrongMethod(t *testing.T) {
	s := NewServer("127.0.0.1", 8765, testConfig(), "", nil)

	recorder := performRequest(t, s, http.MethodGet, "/api/llm/analyze/trade", nil, "127.0.0.1:12345", nil)
	require.Equal(t, http.StatusMethodNotAllowed, recorder.Code)
}

func TestHandleLLMAnalyzePositionsWrongMethod(t *testing.T) {
	s := NewServer("127.0.0.1", 8765, testConfig(), "", nil)

	recorder := performRequest(t, s, http.MethodPost, "/api/llm/analyze/positions", nil, "127.0.0.1:12345", nil)
	require.Equal(t, http.StatusMethodNotAllowed, recorder.Code)
}

func TestHandleLLMAnalyzeMarketWrongMethod(t *testing.T) {
	s := NewServer("127.0.0.1", 8765, testConfig(), "", nil)

	recorder := performRequest(t, s, http.MethodGet, "/api/llm/analyze/market", nil, "127.0.0.1:12345", nil)
	require.Equal(t, http.StatusMethodNotAllowed, recorder.Code)
}

func TestHandleLLMHistoryWrongMethod(t *testing.T) {
	s := NewServer("127.0.0.1", 8765, testConfig(), "", nil)

	recorder := performRequest(t, s, http.MethodPost, "/api/llm/history", nil, "127.0.0.1:12345", nil)
	require.Equal(t, http.StatusMethodNotAllowed, recorder.Code)
}

func TestHandleDataNewsWrongMethod(t *testing.T) {
	s := NewServer("127.0.0.1", 8765, testConfig(), "", nil)

	recorder := performRequest(t, s, http.MethodPost, "/api/data/news", nil, "127.0.0.1:12345", nil)
	require.Equal(t, http.StatusMethodNotAllowed, recorder.Code)
}

func TestHandleDataEventsWrongMethod(t *testing.T) {
	s := NewServer("127.0.0.1", 8765, testConfig(), "", nil)

	recorder := performRequest(t, s, http.MethodPost, "/api/data/events", nil, "127.0.0.1:12345", nil)
	require.Equal(t, http.StatusMethodNotAllowed, recorder.Code)
}

func TestHandleAlertsWrongMethod(t *testing.T) {
	s := NewServer("127.0.0.1", 8765, testConfig(), "", nil)

	recorder := performRequest(t, s, http.MethodPost, "/api/alerts", nil, "127.0.0.1:12345", nil)
	require.Equal(t, http.StatusMethodNotAllowed, recorder.Code)
}

func TestHandleManualCancelOrderWrongMethod(t *testing.T) {
	s := NewServer("127.0.0.1", 8765, testConfig(), "", nil)

	recorder := performRequest(t, s, http.MethodPost, "/api/manual/order/ord-123", nil, "127.0.0.1:12345", nil)
	require.Equal(t, http.StatusMethodNotAllowed, recorder.Code)
}

func TestHandleManualClosePositionWrongMethod(t *testing.T) {
	s := NewServer("127.0.0.1", 8765, testConfig(), "", nil)

	recorder := performRequest(t, s, http.MethodGet, "/api/manual/position/close", nil, "127.0.0.1:12345", nil)
	require.Equal(t, http.StatusMethodNotAllowed, recorder.Code)
}

func TestHandleManualSetTpSlWrongMethod(t *testing.T) {
	s := NewServer("127.0.0.1", 8765, testConfig(), "", nil)

	recorder := performRequest(t, s, http.MethodGet, "/api/manual/position/tp-sl", nil, "127.0.0.1:12345", nil)
	require.Equal(t, http.StatusMethodNotAllowed, recorder.Code)
}

func TestHandleManualSetLeverageWrongMethod(t *testing.T) {
	s := NewServer("127.0.0.1", 8765, testConfig(), "", nil)

	recorder := performRequest(t, s, http.MethodGet, "/api/manual/position/leverage", nil, "127.0.0.1:12345", nil)
	require.Equal(t, http.StatusMethodNotAllowed, recorder.Code)
}

func TestHandleManualSetTrailingStopWrongMethod(t *testing.T) {
	s := NewServer("127.0.0.1", 8765, testConfig(), "", nil)

	recorder := performRequest(t, s, http.MethodGet, "/api/manual/position/trailing-stop", nil, "127.0.0.1:12345", nil)
	require.Equal(t, http.StatusMethodNotAllowed, recorder.Code)
}

func TestHandleCreateTimedOrderWrongMethod(t *testing.T) {
	s := NewServer("127.0.0.1", 8765, testConfig(), "", nil)

	recorder := performRequest(t, s, http.MethodGet, "/api/manual/timed-order", nil, "127.0.0.1:12345", nil)
	require.Equal(t, http.StatusMethodNotAllowed, recorder.Code)
}

func TestHandleCancelTimedOrderWrongMethod(t *testing.T) {
	s := NewServer("127.0.0.1", 8765, testConfig(), "", nil)

	recorder := performRequest(t, s, http.MethodPost, "/api/manual/timed-order/ord-123", nil, "127.0.0.1:12345", nil)
	require.Equal(t, http.StatusMethodNotAllowed, recorder.Code)
}

func TestHandleListTimedOrdersWrongMethod(t *testing.T) {
	s := NewServer("127.0.0.1", 8765, testConfig(), "", nil)

	recorder := performRequest(t, s, http.MethodPost, "/api/manual/timed-orders", nil, "127.0.0.1:12345", nil)
	require.Equal(t, http.StatusMethodNotAllowed, recorder.Code)
}

func TestHandleCollectNowWrongMethod(t *testing.T) {
	s := NewServer("127.0.0.1", 8765, testConfig(), "", nil)

	recorder := performRequest(t, s, http.MethodGet, "/api/data/collect", nil, "127.0.0.1:12345", nil)
	require.Equal(t, http.StatusMethodNotAllowed, recorder.Code)
}

func TestHandleSendAlertWrongMethod(t *testing.T) {
	s := NewServer("127.0.0.1", 8765, testConfig(), "", nil)

	recorder := performRequest(t, s, http.MethodGet, "/api/alerts/send", nil, "127.0.0.1:12345", nil)
	require.Equal(t, http.StatusMethodNotAllowed, recorder.Code)
}

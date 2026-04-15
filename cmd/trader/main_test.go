package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ljwqf/quant/internal/config"
	"github.com/ljwqf/quant/internal/execution"
	"github.com/ljwqf/quant/internal/monitoring"
	"github.com/ljwqf/quant/internal/storage"
	"github.com/ljwqf/quant/internal/strategy"
	"github.com/ljwqf/quant/pkg/types"
)

type baseStrategyStub struct{}

func (s *baseStrategyStub) Name() string { return "stop-hook-stub" }

func (s *baseStrategyStub) Init(params map[string]interface{}) error { return nil }

func (s *baseStrategyStub) OnTick(tick *types.Tick) (*types.Signal, error) { return nil, nil }

func (s *baseStrategyStub) OnBar(bar *types.Bar) (*types.Signal, error) { return nil, nil }

func (s *baseStrategyStub) OnOrderBook(orderBook *types.OrderBook) (*types.Signal, error) {
	return nil, nil
}

func (s *baseStrategyStub) GetParams() map[string]interface{} { return nil }

func (s *baseStrategyStub) SetParams(params map[string]interface{}) {}

func (s *baseStrategyStub) GetMetrics() map[string]interface{} { return nil }

func (s *baseStrategyStub) OnPositionFilled(symbol string, side types.OrderSide, entryPrice, size float64) {
}

func (s *baseStrategyStub) OnPositionReduced(symbol string, exitPrice, pnl, remainingSize float64) {}

func (s *baseStrategyStub) OnPositionClosed(symbol string, exitPrice, pnl float64) {}

func (s *baseStrategyStub) ConfirmRebalanceEntry(request *strategy.RebalanceRequest) (*strategy.RebalanceDecision, error) {
	return &strategy.RebalanceDecision{RejectReason: "stub"}, nil
}

type stopHookStrategyStub struct {
	baseStrategyStub
	pauseCalls int
	stopCalls  int
}

func (s *stopHookStrategyStub) Pause() {
	s.pauseCalls++
}

func (s *stopHookStrategyStub) Stop() {
	s.stopCalls++
}

type stopOnlyStrategyStub struct {
	baseStrategyStub
	stopCalls int
}

func (s *stopOnlyStrategyStub) Stop() {
	s.stopCalls++
}

func TestInvokeStrategyStopHookPrefersPauseOverStop(t *testing.T) {
	stub := &stopHookStrategyStub{}

	hook := invokeStrategyStopHook(stub)

	assert.Equal(t, "pause", hook)
	assert.Equal(t, 1, stub.pauseCalls)
	assert.Equal(t, 0, stub.stopCalls)
}

func TestInvokeStrategyStopHookUsesStopWhenPauseUnavailable(t *testing.T) {
	stopper := &stopOnlyStrategyStub{}
	hook := invokeStrategyStopHook(stopper)

	assert.Equal(t, "stop", hook)
	assert.Equal(t, 1, stopper.stopCalls)
}

func TestInvokeStrategyStopHookReturnsEmptyForNilStrategy(t *testing.T) {
	var stub strategy.Strategy

	hook := invokeStrategyStopHook(stub)

	assert.Equal(t, "", hook)
}

func TestNormalizeRebalanceRoutingFieldsFromOpenAlert(t *testing.T) {
	labels, details := normalizeRebalanceRoutingFields("", map[string]string{
		"component": "execution",
		"event":     "rebalance_circuit_open",
		"strategy":  "DeltaNeutralFunding-Pro",
		"step":      "spot_leg",
		"reason":    "rollback_failed",
	}, map[string]interface{}{})

	assert.Equal(t, "rebalance", labels["domain"])
	assert.Equal(t, "open", labels["event_type"])
	assert.Equal(t, "rebalance/open", labels["route"])
	assert.Equal(t, "rollback_failed", details["reason"])
	assert.Equal(t, "open", details["event_type"])
	assert.Equal(t, "rebalance/open", details["route"])
}

func TestNormalizeRebalanceRoutingFieldsForManualReset(t *testing.T) {
	labels, details := normalizeRebalanceRoutingFields("reset", map[string]string{
		"event":    "rebalance_circuit_reset_manual",
		"strategy": "DeltaNeutralFunding-Pro",
		"step":     "perp_leg",
		"reason":   "operator_reset",
	}, map[string]interface{}{"reset_mode": "manual"})

	assert.Equal(t, "reset", labels["event_type"])
	assert.Equal(t, "manual", labels["reset_mode"])
	assert.Equal(t, "rebalance/reset/manual", labels["route"])
	assert.Equal(t, "rebalance/reset/manual", details["route"])
	assert.Equal(t, "manual", details["reset_mode"])
}

func TestRebalanceEventInfoFromExecutionUsesNormalizedRouting(t *testing.T) {
	event := execution.RebalanceEvent{
		Type:     execution.RebalanceEventRecoverStarted,
		Strategy: "DeltaNeutralFunding-Pro",
		Step:     "spot_leg",
		Reason:   "startup_reconcile",
		Message:  "开始恢复未完成计划",
		Labels:   map[string]string{"event": "rebalance_recover_started"},
		Details:  map[string]interface{}{"plan_id": "plan-1"},
	}

	info := rebalanceEventInfoFromExecution(event)
	require.NotNil(t, info)
	assert.Equal(t, "recover_started", info.Type)
	assert.Equal(t, "rebalance/recover/started", info.Labels["route"])
	assert.Equal(t, "recover_started", info.Details["event_type"])
	assert.Equal(t, "plan-1", info.Details["plan_id"])
}

func TestNormalizeRebalanceRoutingFieldsForRecoverFailure(t *testing.T) {
	labels, details := normalizeRebalanceRoutingFields("", map[string]string{
		"event":    "rebalance_recover_failed",
		"strategy": "DeltaNeutralFunding-Pro",
		"step":     "spot_leg",
		"reason":   "startup_reconcile",
	}, map[string]interface{}{"error": "cancel failed"})

	assert.Equal(t, "recover_failed", labels["event_type"])
	assert.Equal(t, "rebalance/recover/failed", labels["route"])
	assert.Equal(t, "recover_failed", details["event_type"])
	assert.Equal(t, "rebalance/recover/failed", details["route"])
	assert.Equal(t, "cancel failed", details["error"])
}

// --- New tests for uncovered functions ---

func TestDefaultFloat64(t *testing.T) {
	assert.InDelta(t, 5.0, defaultFloat64(5.0, 1.0), 0.001)
	assert.InDelta(t, 1.0, defaultFloat64(0, 1.0), 0.001)
	assert.InDelta(t, 1.0, defaultFloat64(-1.0, 1.0), 0.001)
}

func TestDefaultDuration(t *testing.T) {
	assert.Equal(t, 5*time.Second, defaultDuration(5*time.Second, 1*time.Second))
	assert.Equal(t, 1*time.Second, defaultDuration(0, 1*time.Second))
	assert.Equal(t, 1*time.Second, defaultDuration(-1*time.Second, 1*time.Second))
}

func TestMapExecutionAlertLevel(t *testing.T) {
	tests := []struct {
		level    execution.AlertLevel
		expected monitoring.AlertType
	}{
		{execution.AlertLevelInfo, monitoring.AlertTypeInfo},
		{execution.AlertLevelWarning, monitoring.AlertTypeWarning},
		{execution.AlertLevelError, monitoring.AlertTypeError},
		{execution.AlertLevelCritical, monitoring.AlertTypeCritical},
		{execution.AlertLevel("unknown"), monitoring.AlertTypeInfo}, // default
	}
	for _, tt := range tests {
		t.Run(string(tt.level), func(t *testing.T) {
			assert.Equal(t, tt.expected, mapExecutionAlertLevel(tt.level))
		})
	}
}

func TestInferResetMode(t *testing.T) {
	assert.Equal(t, "automatic", inferResetMode("cooldown_elapsed"))
	assert.Equal(t, "automatic", inferResetMode("  cooldown_elapsed  "))
	assert.Equal(t, "manual", inferResetMode("operator_reset"))
	assert.Equal(t, "manual", inferResetMode(""))
	assert.Equal(t, "manual", inferResetMode("   "))
}

func TestEnvFloat64OrDefault(t *testing.T) {
	t.Setenv("TEST_FLOAT", "3.14")
	assert.InDelta(t, 3.14, envFloat64OrDefault("TEST_FLOAT", 0.0), 0.001)

	assert.InDelta(t, 2.5, envFloat64OrDefault("NONEXISTENT_FLOAT", 2.5), 0.001)

	t.Setenv("TEST_FLOAT_INVALID", "not-a-number")
	assert.InDelta(t, 1.0, envFloat64OrDefault("TEST_FLOAT_INVALID", 1.0), 0.001)
}

func TestStringDetail(t *testing.T) {
	assert.Equal(t, "hello", stringDetail("hello"))
	assert.Equal(t, "hello", stringDetail("  hello  "))
	assert.Equal(t, "", stringDetail(""))
	assert.Equal(t, "", stringDetail(nil))
	assert.Equal(t, "", stringDetail(123)) // not a string
}

func TestFirstNonEmptyString(t *testing.T) {
	assert.Equal(t, "b", firstNonEmptyString("", "b", "c"))
	assert.Equal(t, "a", firstNonEmptyString("a", "b", "c"))
	assert.Equal(t, "c", firstNonEmptyString("", "  ", "c"))
	assert.Equal(t, "", firstNonEmptyString("", "", ""))
	assert.Equal(t, "", firstNonEmptyString())
}

func TestCloneInterfaceMapMain(t *testing.T) {
	original := map[string]interface{}{
		"a": 1,
		"b": "hello",
		"c": true,
	}

	cloned := cloneInterfaceMapMain(original)
	assert.Equal(t, original, cloned)
	assert.NotSame(t, &original, &cloned) // different map objects

	// Modifying cloned should not affect original
	cloned["a"] = 999
	assert.Equal(t, 1, original["a"])

	// Nil input
	assert.Nil(t, cloneInterfaceMapMain(nil))
}

func TestBuildSmartFilterRefreshConfig(t *testing.T) {
	cfg := &config.Config{
		Strategy: config.StrategyConfig{
			SmartFilter: config.SmartFilterConfig{
				Source: "http",
				CryptoQuant: config.CryptoQuantConfig{
					APIKey: "test-key",
					Asset:  "eth",
				},
			},
		},
	}

	result := buildSmartFilterRefreshConfig(cfg)
	assert.True(t, result.Enabled)
	assert.Equal(t, "http", result.Source)
	assert.Equal(t, "eth", result.CryptoQuantAsset)
	assert.Equal(t, "test-key", result.CryptoQuantAPIKey)
}

func TestBuildSmartFilterRefreshConfigDefaults(t *testing.T) {
	cfg := &config.Config{
		Strategy: config.StrategyConfig{},
	}

	result := buildSmartFilterRefreshConfig(cfg)
	assert.True(t, result.Enabled)
	assert.Equal(t, "auto", result.Source)
	assert.Equal(t, 5*time.Minute, result.Interval)
	assert.Equal(t, "btc", result.CryptoQuantAsset)
}

func TestPositionRepoAdapterUpsert(t *testing.T) {
	mockRepo := &mockActivePositionRepo{}
	adapter := &positionRepoAdapter{repo: mockRepo}

	err := adapter.Upsert(&execution.PositionRecord{
		Strategy:   "test-strategy",
		Symbol:     "BTC-USDT",
		Side:       "long",
		Size:       1.0,
		EntryPrice: 50000,
		OrderID:    "order-1",
	})
	require.NoError(t, err)
	assert.Equal(t, 1, mockRepo.upsertCalls)
}

func TestPositionRepoAdapterDelete(t *testing.T) {
	mockRepo := &mockActivePositionRepo{}
	adapter := &positionRepoAdapter{repo: mockRepo}

	err := adapter.Delete("test-strategy", "BTC-USDT")
	require.NoError(t, err)
	assert.Equal(t, 1, mockRepo.deleteCalls)
}

func TestPositionRepoAdapterListByStrategy(t *testing.T) {
	mockRepo := &mockActivePositionRepo{
		positions: []*storage.ActivePosition{
			{Strategy: "test", Symbol: "BTC-USDT", Side: "long", Size: 1.0, EntryPrice: 50000, OrderID: "1"},
		},
	}
	adapter := &positionRepoAdapter{repo: mockRepo}

	list, err := adapter.ListByStrategy("test")
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "test", list[0].Strategy)
	assert.Equal(t, "BTC-USDT", list[0].Symbol)
	assert.Equal(t, 1, mockRepo.listByStrategyCalls)
}

func TestPositionRepoAdapterListAll(t *testing.T) {
	mockRepo := &mockActivePositionRepo{
		allPositions: []*storage.ActivePosition{
			{Strategy: "test1", Symbol: "BTC-USDT"},
			{Strategy: "test2", Symbol: "ETH-USDT"},
		},
	}
	adapter := &positionRepoAdapter{repo: mockRepo}

	list, err := adapter.ListAll()
	require.NoError(t, err)
	require.Len(t, list, 2)
	assert.Equal(t, "test1", list[0].Strategy)
	assert.Equal(t, "test2", list[1].Strategy)
	assert.Equal(t, 1, mockRepo.listAllCalls)
}

// Mock repository for positionRepoAdapter tests
type mockActivePositionRepo struct {
	upsertCalls       int
	deleteCalls       int
	listByStrategyCalls int
	listAllCalls      int
	positions         []*storage.ActivePosition
	allPositions      []*storage.ActivePosition
}

func (m *mockActivePositionRepo) Upsert(_ *storage.ActivePosition) error {
	m.upsertCalls++
	return nil
}

func (m *mockActivePositionRepo) Delete(_, _ string) error {
	m.deleteCalls++
	return nil
}

func (m *mockActivePositionRepo) ListByStrategy(_ string) ([]*storage.ActivePosition, error) {
	m.listByStrategyCalls++
	return m.positions, nil
}

func (m *mockActivePositionRepo) ListAll() ([]*storage.ActivePosition, error) {
	m.listAllCalls++
	return m.allPositions, nil
}

func TestRebalanceCircuitInfoFromState(t *testing.T) {
	state := execution.RebalanceCircuitState{
		Open:            true,
		Strategy:        "TestStrategy",
		Step:            "spot_leg",
		Reason:          "test_reason",
		OpenedAt:        time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		CooldownUntil:   time.Date(2026, 1, 1, 1, 0, 0, 0, time.UTC),
		LastResetAt:     time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC),
		LastResetReason: "manual_reset",
		AutoReset:       false,
		Cooldown:        time.Hour,
	}

	info := rebalanceCircuitInfoFromState(state)
	require.NotNil(t, info)
	assert.True(t, info.Open)
	assert.Equal(t, "TestStrategy", info.Strategy)
	assert.Equal(t, "spot_leg", info.Step)
	assert.Equal(t, "test_reason", info.Reason)
	assert.Equal(t, "1h0m0s", info.Cooldown)
}

func TestRebalanceRoute(t *testing.T) {
	assert.Equal(t, "rebalance/reset/manual", buildRebalanceRoute("reset", "manual"))
	assert.Equal(t, "rebalance/reset/automatic", buildRebalanceRoute("reset", "automatic"))
	assert.Equal(t, "rebalance/open", buildRebalanceRoute("open", ""))
	assert.Equal(t, "rebalance/recover/started", buildRebalanceRoute("recover_started", ""))
	assert.Equal(t, "rebalance/recover/failed", buildRebalanceRoute("recover_failed", ""))
	assert.Equal(t, "rebalance/unknown", buildRebalanceRoute("unknown", ""))
}

func TestInferRebalanceEventType(t *testing.T) {
	tests := []struct {
		event    string
		expected string
	}{
		{"rebalance_circuit_open", "open"},
		{"rebalance_recover_started", "recover_started"},
		{"rebalance_recover_succeeded", "recover_succeeded"},
		{"rebalance_recover_failed", "recover_failed"},
		{"rebalance_circuit_reset_manual", "reset"},
		{"rebalance_circuit_reset", "reset"},
		{"unknown_event", ""},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.event, func(t *testing.T) {
			labels := map[string]string{"event": tt.event}
			assert.Equal(t, tt.expected, inferRebalanceEventType(labels))
		})
	}
}

func TestCloneStringMapMain(t *testing.T) {
	original := map[string]string{"a": "1", "b": "2"}
	cloned := cloneStringMapMain(original)
	assert.Equal(t, original, cloned)
	assert.NotSame(t, &original, &cloned)

	cloned["a"] = "999"
	assert.Equal(t, "1", original["a"])

	assert.Nil(t, cloneStringMapMain(nil))
}

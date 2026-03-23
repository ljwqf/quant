package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ljwqf/quant/internal/execution"
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

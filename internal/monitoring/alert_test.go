package monitoring

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAlertManagerSendsWebhookPayload(t *testing.T) {
	var requestCount atomic.Int32
	var payload WebhookPayload
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "application/json", r.Header.Get("Content-Type"))
		require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	manager := NewAlertManager(&AlertConfig{Enable: true, Channels: []string{"webhook"}, WebhookURL: server.URL, DedupWindow: time.Millisecond, MinInterval: time.Millisecond})
	require.NoError(t, manager.AlertWithContext(AlertTypeCritical, "再平衡熔断已打开", "策略 DeltaNeutralFunding-Pro 的补偿失败", map[string]string{"component": "execution", "event": "rebalance_circuit_open"}, map[string]interface{}{"strategy": "DeltaNeutralFunding-Pro", "step": "spot_leg"}))

	assert.Equal(t, int32(1), requestCount.Load())
	assert.Equal(t, AlertTypeCritical, payload.Type)
	assert.Equal(t, "再平衡熔断已打开", payload.Title)
	assert.NotEmpty(t, payload.EventID)
	assert.NotEmpty(t, payload.Fingerprint)
	assert.Equal(t, "okx-quant", payload.Source)
	assert.Equal(t, "execution", payload.Labels["component"])
	assert.Equal(t, "rebalance_circuit_open", payload.Labels["event"])
	assert.Equal(t, "DeltaNeutralFunding-Pro", payload.Details["strategy"])
	assert.Equal(t, "spot_leg", payload.Details["step"])
}

func TestAlertManagerDeduplicatesAndRateLimitsWebhook(t *testing.T) {
	var requestCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	manager := NewAlertManager(&AlertConfig{Enable: true, Channels: []string{"webhook"}, WebhookURL: server.URL, DedupWindow: 50 * time.Millisecond, MinInterval: 50 * time.Millisecond})
	require.NoError(t, manager.Alert(AlertTypeWarning, "恢复未完成 rebalance", "startup_reconcile"))
	require.NoError(t, manager.Alert(AlertTypeWarning, "恢复未完成 rebalance", "startup_reconcile"))
	assert.Equal(t, int32(1), requestCount.Load())
	assert.Len(t, manager.GetAlerts(), 1)

	time.Sleep(60 * time.Millisecond)
	require.NoError(t, manager.Alert(AlertTypeWarning, "恢复未完成 rebalance", "startup_reconcile"))
	assert.Equal(t, int32(2), requestCount.Load())
	assert.Len(t, manager.GetAlerts(), 2)

	require.NoError(t, manager.Alert(AlertTypeWarning, "恢复未完成 rebalance", "startup_reconcile"))
	assert.Equal(t, int32(2), requestCount.Load())
	assert.Len(t, manager.GetAlerts(), 2)
}

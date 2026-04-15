package llmanalysis

import (
	"testing"
	"time"

	"github.com/ljwqf/quant/pkg/types"
	"github.com/stretchr/testify/assert"
)

func TestPositionMonitorDisabled(t *testing.T) {
	cfg := &PositionMonitorConfig{Enable: false}
	pm := NewPositionMonitor(nil, nil, nil, nil, cfg)

	pm.Start()
	// Should not panic, just log and return
	pm.Stop()
	pm.Stop() // idempotent
}

func TestPositionMonitorNilAnalyzer(t *testing.T) {
	cfg := &PositionMonitorConfig{Enable: true}
	pm := NewPositionMonitor(nil, nil, nil, nil, cfg)

	pm.Start()
	// Should warn about nil analyzer and not start
	pm.mu.RLock()
	running := pm.running
	pm.mu.RUnlock()
	assert.False(t, running)
}

func TestPositionMonitorConfigDefaults(t *testing.T) {
	cfg := &PositionMonitorConfig{
		Enable:        true,
		CheckInterval: 0, // should default
		RiskThreshold: "", // should default
	}

	pm := NewPositionMonitor(nil, nil, nil, nil, cfg)
	assert.Equal(t, 5*time.Minute, pm.cfg.CheckInterval)
	assert.Equal(t, "high", pm.cfg.RiskThreshold)
}

func TestCalculatePnLPercent(t *testing.T) {
	tests := []struct {
		name     string
		pos      *types.Position
		expected float64
	}{
		{
			name: "long profit",
			pos: &types.Position{
				Symbol:       "BTC-USDT",
				Side:         types.OrderSideBuy,
				Size:         1.0,
				EntryPrice:   50000,
				MarkPrice:    51000,
				UnrealizedPnL: 1000,
				Leverage:     10,
			},
			expected: 20.0, // (1000/50000)*100*10 = 20%
		},
		{
			name: "zero entry price",
			pos: &types.Position{
				Symbol:       "BTC-USDT",
				Side:         types.OrderSideBuy,
				Size:         1.0,
				EntryPrice:   0,
				MarkPrice:    50000,
				UnrealizedPnL: 0,
				Leverage:     10,
			},
			expected: 0,
		},
		{
			name: "zero size",
			pos: &types.Position{
				Symbol:       "BTC-USDT",
				Side:         types.OrderSideBuy,
				Size:         0,
				EntryPrice:   50000,
				MarkPrice:    50000,
				UnrealizedPnL: 0,
				Leverage:     10,
			},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculatePnLPercent(tt.pos)
			assert.InDelta(t, tt.expected, result, 0.01)
		})
	}
}

func TestShouldAlert(t *testing.T) {
	tests := []struct {
		name       string
		threshold  string
		result     *AnalysisResult
		pnlPercent float64
		expected   bool
	}{
		{
			name:      "high risk level triggers alert",
			threshold: "high",
			result:    &AnalysisResult{RiskLevel: "high", Summary: "继续持有风险较大"},
			expected:  true,
		},
		{
			name:      "medium risk below high threshold",
			threshold: "high",
			result:    &AnalysisResult{RiskLevel: "medium", Summary: "可以继续持有"},
			expected:  false,
		},
		{
			name:      "close keyword triggers alert",
			threshold: "high",
			result:    &AnalysisResult{RiskLevel: "low", Summary: "建议平仓"},
			expected:  true,
		},
		{
			name:      "exit keyword triggers alert",
			threshold: "high",
			result:    &AnalysisResult{RiskLevel: "low", Summary: "建议exit position"},
			expected:  true,
		},
		{
			name:      "减仓 keyword triggers alert",
			threshold: "high",
			result:    &AnalysisResult{RiskLevel: "medium", Summary: "建议减仓至半仓"},
			expected:  true,
		},
		{
			name:      "critical risk triggers alert",
			threshold: "high",
			result:    &AnalysisResult{RiskLevel: "critical", Summary: "立即清仓"},
			expected:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pm := &PositionMonitor{
				cfg: &PositionMonitorConfig{
					RiskThreshold: tt.threshold,
				},
			}
			result := pm.shouldAlert(tt.result, tt.pnlPercent)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPositionMonitorGetStatus(t *testing.T) {
	cfg := &PositionMonitorConfig{
		Enable:        true,
		CheckInterval: 5 * time.Minute,
		RiskThreshold: "high",
	}

	pm := NewPositionMonitor(nil, nil, nil, nil, cfg)
	status := pm.GetStatus()

	assert.True(t, status["enabled"].(bool))
	assert.False(t, status["running"].(bool)) // not started
	assert.Equal(t, "5m0s", status["check_interval"])
	assert.Equal(t, "high", status["risk_threshold"])
}

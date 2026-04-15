package monitoring

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAlertRuleManager(t *testing.T) {
	am := NewAlertManager(&AlertConfig{Enable: true})
	m := NewMetrics(&MetricsConfig{Enable: true})

	mgr := NewAlertRuleManager(am, m)
	require.NotNil(t, mgr)
	assert.NotNil(t, mgr.conditions)
}

func TestAlertRuleManagerRegisterCondition(t *testing.T) {
	am := NewAlertManager(&AlertConfig{Enable: true})
	m := NewMetrics(&MetricsConfig{Enable: true})
	mgr := NewAlertRuleManager(am, m)

	cond := &AlertCondition{
		ID:       "test_cond",
		Name:     "测试告警",
		Severity: AlertTypeWarning,
		Enabled:  true,
		Check:    func() (bool, string, error) { return false, "", nil },
	}

	mgr.RegisterCondition(cond)

	conditions := mgr.GetConditions()
	assert.Len(t, conditions, 1)
	assert.Equal(t, "test_cond", conditions[0].ID)
}

func TestAlertRuleManagerUnregisterCondition(t *testing.T) {
	am := NewAlertManager(&AlertConfig{Enable: true})
	m := NewMetrics(&MetricsConfig{Enable: true})
	mgr := NewAlertRuleManager(am, m)

	mgr.RegisterCondition(&AlertCondition{
		ID:   "test_unregister",
		Name: "测试",
		Check: func() (bool, string, error) { return false, "", nil },
	})

	mgr.UnregisterCondition("test_unregister")
	conditions := mgr.GetConditions()
	assert.Empty(t, conditions)
}

func TestAlertRuleManagerEnableDisableCondition(t *testing.T) {
	am := NewAlertManager(&AlertConfig{Enable: true})
	m := NewMetrics(&MetricsConfig{Enable: true})
	mgr := NewAlertRuleManager(am, m)

	mgr.RegisterCondition(&AlertCondition{
		ID:      "test_toggle",
		Name:    "测试",
		Enabled: true,
		Check:   func() (bool, string, error) { return false, "", nil },
	})

	// Disable
	err := mgr.DisableCondition("test_toggle")
	require.NoError(t, err)
	conditions := mgr.GetConditions()
	assert.False(t, conditions[0].Enabled)

	// Enable
	err = mgr.EnableCondition("test_toggle")
	require.NoError(t, err)
	conditions = mgr.GetConditions()
	assert.True(t, conditions[0].Enabled)
}

func TestAlertRuleManagerEnableDisableNonExistent(t *testing.T) {
	am := NewAlertManager(&AlertConfig{Enable: true})
	m := NewMetrics(&MetricsConfig{Enable: true})
	mgr := NewAlertRuleManager(am, m)

	err := mgr.EnableCondition("nonexistent")
	assert.Error(t, err)

	err = mgr.DisableCondition("nonexistent")
	assert.Error(t, err)
}

func TestAlertRuleManagerTriggerCondition(t *testing.T) {
	am := NewAlertManager(&AlertConfig{Enable: true})
	m := NewMetrics(&MetricsConfig{Enable: true})
	mgr := NewAlertRuleManager(am, m)

	mgr.RegisterCondition(&AlertCondition{
		ID:       "trigger_test",
		Name:     "触发测试",
		Severity: AlertTypeWarning,
		Enabled:  true,
		Check:    func() (bool, string, error) { return true, "条件满足", nil },
	})

	err := mgr.TriggerCondition("trigger_test")
	assert.NoError(t, err)
}

func TestAlertRuleManagerTriggerConditionNotMet(t *testing.T) {
	am := NewAlertManager(&AlertConfig{Enable: true})
	m := NewMetrics(&MetricsConfig{Enable: true})
	mgr := NewAlertRuleManager(am, m)

	mgr.RegisterCondition(&AlertCondition{
		ID:       "no_trigger",
		Name:     "不触发",
		Severity: AlertTypeWarning,
		Enabled:  true,
		Check:    func() (bool, string, error) { return false, "", nil },
	})

	err := mgr.TriggerCondition("no_trigger")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "未触发")
}

func TestAlertRuleManagerTriggerNonExistent(t *testing.T) {
	am := NewAlertManager(&AlertConfig{Enable: true})
	m := NewMetrics(&MetricsConfig{Enable: true})
	mgr := NewAlertRuleManager(am, m)

	err := mgr.TriggerCondition("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "不存在")
}

func TestAlertRuleManagerCheckConditionsWithCooldown(t *testing.T) {
	am := NewAlertManager(&AlertConfig{Enable: true})
	m := NewMetrics(&MetricsConfig{Enable: true})
	mgr := NewAlertRuleManager(am, m)

	callCount := 0
	mgr.RegisterCondition(&AlertCondition{
		ID:       "cooldown_test",
		Name:     "冷却测试",
		Severity: AlertTypeWarning,
		Enabled:  true,
		Cooldown: time.Hour,
		Check: func() (bool, string, error) {
			callCount++
			return true, "触发", nil
		},
	})

	// First check should trigger
	mgr.checkConditions()
	assert.Equal(t, 1, callCount)

	// Second check within cooldown should not trigger
	mgr.checkConditions()
	assert.Equal(t, 1, callCount) // still 1
}

func TestAlertRuleManagerCheckConditionDisabled(t *testing.T) {
	am := NewAlertManager(&AlertConfig{Enable: true})
	m := NewMetrics(&MetricsConfig{Enable: true})
	mgr := NewAlertRuleManager(am, m)

	callCount := 0
	mgr.RegisterCondition(&AlertCondition{
		ID:       "disabled_test",
		Name:     "禁用测试",
		Severity: AlertTypeWarning,
		Enabled:  false,
		Check: func() (bool, string, error) {
			callCount++
			return true, "触发", nil
		},
	})

	mgr.checkConditions()
	assert.Equal(t, 0, callCount)
}

func TestAlertRuleManagerCheckConditionError(t *testing.T) {
	am := NewAlertManager(&AlertConfig{Enable: true})
	m := NewMetrics(&MetricsConfig{Enable: true})
	mgr := NewAlertRuleManager(am, m)

	mgr.RegisterCondition(&AlertCondition{
		ID:       "error_test",
		Name:     "错误测试",
		Severity: AlertTypeWarning,
		Enabled:  true,
		Check:    func() (bool, string, error) { return false, "", fmt.Errorf("模拟错误") },
	})

	// Should not panic, just log the error
	mgr.checkConditions()
}

func TestAlertRuleManagerStartStop(t *testing.T) {
	am := NewAlertManager(&AlertConfig{Enable: true})
	m := NewMetrics(&MetricsConfig{Enable: true})
	mgr := NewAlertRuleManager(am, m)

	mgr.Start()
	mgr.Stop()
	// Should not panic
}

func TestAlertRuleManagerDefaultConditions(t *testing.T) {
	am := NewAlertManager(&AlertConfig{Enable: true})
	m := NewMetrics(&MetricsConfig{Enable: true})
	mgr := NewAlertRuleManager(am, m)

	mgr.registerDefaultConditions()
	conditions := mgr.GetConditions()

	// Should have 6 default conditions
	assert.True(t, len(conditions) >= 6)

	// Verify specific IDs exist
	ids := make(map[string]bool)
	for _, c := range conditions {
		ids[c.ID] = true
	}

	expectedIDs := []string{
		"high_cpu_usage",
		"high_memory_usage",
		"high_disk_usage",
		"high_api_error_rate",
		"low_fill_rate",
		"strategy_loss",
	}
	for _, id := range expectedIDs {
		assert.True(t, ids[id], "expected condition %s to exist", id)
	}
}

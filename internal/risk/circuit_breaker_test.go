package risk

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCircuitBreakerStateTransitions(t *testing.T) {
	cb := NewCircuitBreaker(&CircuitBreakerConfig{
		FailureThreshold:   2,
		SuccessThreshold:   2,
		Timeout:            10 * time.Millisecond,
		MaxConcurrentCalls: 10,
	})

	errOp := errors.New("boom")
	require.ErrorIs(t, cb.Execute(context.Background(), func() error { return errOp }), errOp)
	require.ErrorIs(t, cb.Execute(context.Background(), func() error { return errOp }), errOp)
	assert.Equal(t, CircuitBreakerStateOpen, cb.GetState())

	err := cb.Execute(context.Background(), func() error { return nil })
	assert.ErrorIs(t, err, ErrCircuitBreakerOpen)

	time.Sleep(15 * time.Millisecond)
	require.NoError(t, cb.Execute(context.Background(), func() error { return nil }))
	assert.Equal(t, CircuitBreakerStateHalfOpen, cb.GetState())

	require.NoError(t, cb.Execute(context.Background(), func() error { return nil }))
	assert.Equal(t, CircuitBreakerStateClosed, cb.GetState())
}

func TestCircuitBreakerConcurrencyLimit(t *testing.T) {
	cb := NewCircuitBreaker(&CircuitBreakerConfig{
		FailureThreshold:   5,
		SuccessThreshold:   1,
		Timeout:            time.Second,
		MaxConcurrentCalls: 1,
	})

	started := make(chan struct{})
	release := make(chan struct{})
	done := make(chan error, 1)

	go func() {
		done <- cb.Execute(context.Background(), func() error {
			close(started)
			<-release
			return nil
		})
	}()

	<-started
	err := cb.Execute(context.Background(), func() error { return nil })
	assert.ErrorIs(t, err, ErrCircuitBreakerOpen)

	close(release)
	require.NoError(t, <-done)
}

func TestKillSwitchAndGlobalRiskController(t *testing.T) {
	triggered := false
	ks := NewKillSwitch(func() error {
		triggered = true
		return nil
	})

	require.NoError(t, ks.Trigger("manual"))
	assert.True(t, triggered)
	assert.True(t, ks.IsTriggered())

	status := ks.GetStatus()
	assert.Equal(t, "manual", status["reason"])

	ks.Reset()
	assert.False(t, ks.IsTriggered())

	controller := NewGlobalRiskController(1000, 10, nil)
	assert.NotNil(t, controller.GetCircuitBreaker())
	assert.NotNil(t, controller.GetKillSwitch())

	controller.RecordTrade(100)
	assert.True(t, controller.CanTrade())

	controller.RecordTrade(-2000)
	require.NoError(t, controller.CheckAndTrigger())
	assert.False(t, controller.CanTrade())

	metrics := controller.GetMetrics()
	assert.NotNil(t, metrics)

	controller.Reset()
	assert.True(t, controller.CanTrade())
}

func TestCircuitBreakerCallbacksAndString(t *testing.T) {
	cb := NewCircuitBreaker(nil)
	changed := false
	cb.OnStateChange(func(old, new CircuitBreakerState) {
		_ = old
		_ = new
		changed = true
	})

	errOp := errors.New("fail")
	for i := 0; i < cb.config.FailureThreshold; i++ {
		_ = cb.Execute(context.Background(), func() error { return errOp })
	}
	assert.True(t, changed)

	cb.Reset()
	assert.Equal(t, CircuitBreakerStateClosed, cb.GetState())

	assert.Equal(t, "Unknown", CircuitBreakerState(99).String())
}

package risk

import (
	"context"
	"sync"
	"time"

	"github.com/ljwqf/quant/pkg/logger"
	"go.uber.org/zap"
)

type CircuitBreakerState int

const (
	CircuitBreakerStateClosed CircuitBreakerState = iota
	CircuitBreakerStateOpen
	CircuitBreakerStateHalfOpen
)

func (s CircuitBreakerState) String() string {
	switch s {
	case CircuitBreakerStateClosed:
		return "Closed"
	case CircuitBreakerStateOpen:
		return "Open"
	case CircuitBreakerStateHalfOpen:
		return "HalfOpen"
	default:
		return "Unknown"
	}
}

type CircuitBreakerConfig struct {
	FailureThreshold   int
	SuccessThreshold   int
	Timeout            time.Duration
	MaxConcurrentCalls int
}

var DefaultCircuitBreakerConfig = &CircuitBreakerConfig{
	FailureThreshold:   5,
	SuccessThreshold:   3,
	Timeout:            30 * time.Second,
	MaxConcurrentCalls: 100,
}

type CircuitBreaker struct {
	config           *CircuitBreakerConfig
	state            CircuitBreakerState
	failures         int
	successes        int
	lastFailureTime  time.Time
	concurrentCalls  int
	mutex            sync.RWMutex
	onStateChange    func(old, new CircuitBreakerState)
}

func NewCircuitBreaker(config *CircuitBreakerConfig) *CircuitBreaker {
	if config == nil {
		config = DefaultCircuitBreakerConfig
	}
	return &CircuitBreaker{
		config: config,
		state:  CircuitBreakerStateClosed,
	}
}

func (cb *CircuitBreaker) Execute(ctx context.Context, operation func() error) error {
	if !cb.allowRequest() {
		return ErrCircuitBreakerOpen
	}

	cb.incrementConcurrentCalls()
	defer cb.decrementConcurrentCalls()

	err := operation()

	if err != nil {
		cb.recordFailure()
		return err
	}

	cb.recordSuccess()
	return nil
}

func (cb *CircuitBreaker) allowRequest() bool {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	if cb.state == CircuitBreakerStateOpen {
		if time.Since(cb.lastFailureTime) > cb.config.Timeout {
			cb.setState(CircuitBreakerStateHalfOpen)
			return true
		}
		return false
	}

	if cb.concurrentCalls >= cb.config.MaxConcurrentCalls {
		return false
	}

	return true
}

func (cb *CircuitBreaker) recordFailure() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	cb.failures++
	cb.lastFailureTime = time.Now()

	if cb.state == CircuitBreakerStateHalfOpen {
		cb.setState(CircuitBreakerStateOpen)
		logger.Warn("熔断器从半开状态转为打开状态")
	} else if cb.failures >= cb.config.FailureThreshold {
		cb.setState(CircuitBreakerStateOpen)
		logger.Warn("熔断器触发，转为打开状态",
			zap.Int("failures", cb.failures),
			zap.Int("threshold", cb.config.FailureThreshold),
		)
	}
}

func (cb *CircuitBreaker) recordSuccess() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	cb.successes++

	if cb.state == CircuitBreakerStateHalfOpen {
		if cb.successes >= cb.config.SuccessThreshold {
			cb.setState(CircuitBreakerStateClosed)
			cb.failures = 0
			cb.successes = 0
			logger.Info("熔断器从半开状态转为关闭状态")
		}
	}
}

func (cb *CircuitBreaker) setState(state CircuitBreakerState) {
	oldState := cb.state
	cb.state = state

	if cb.onStateChange != nil {
		cb.onStateChange(oldState, state)
	}
}

func (cb *CircuitBreaker) incrementConcurrentCalls() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()
	cb.concurrentCalls++
}

func (cb *CircuitBreaker) decrementConcurrentCalls() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()
	cb.concurrentCalls--
}

func (cb *CircuitBreaker) GetState() CircuitBreakerState {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()
	return cb.state
}

func (cb *CircuitBreaker) GetMetrics() map[string]interface{} {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()

	return map[string]interface{}{
		"state":            cb.state.String(),
		"failures":         cb.failures,
		"successes":        cb.successes,
		"concurrent_calls": cb.concurrentCalls,
		"last_failure":     cb.lastFailureTime,
	}
}

func (cb *CircuitBreaker) Reset() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	cb.state = CircuitBreakerStateClosed
	cb.failures = 0
	cb.successes = 0
	cb.concurrentCalls = 0

	logger.Info("熔断器已重置")
}

func (cb *CircuitBreaker) OnStateChange(handler func(old, new CircuitBreakerState)) {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()
	cb.onStateChange = handler
}

type KillSwitch struct {
	isTriggered    bool
	triggerTime    time.Time
	reason         string
	emergencyClose func() error
	mutex          sync.RWMutex
}

func NewKillSwitch(emergencyClose func() error) *KillSwitch {
	return &KillSwitch{
		emergencyClose: emergencyClose,
	}
}

func (k *KillSwitch) Trigger(reason string) error {
	k.mutex.Lock()
	defer k.mutex.Unlock()

	if k.isTriggered {
		return nil
	}

	k.isTriggered = true
	k.triggerTime = time.Now()
	k.reason = reason

	logger.Warn("Kill Switch触发",
		zap.String("reason", reason),
		zap.Time("trigger_time", k.triggerTime),
	)

	if k.emergencyClose != nil {
		if err := k.emergencyClose(); err != nil {
			logger.Error("紧急平仓失败", zap.Error(err))
			return err
		}
	}

	return nil
}

func (k *KillSwitch) IsTriggered() bool {
	k.mutex.RLock()
	defer k.mutex.RUnlock()
	return k.isTriggered
}

func (k *KillSwitch) Reset() {
	k.mutex.Lock()
	defer k.mutex.Unlock()

	k.isTriggered = false
	k.triggerTime = time.Time{}
	k.reason = ""

	logger.Info("Kill Switch已重置")
}

func (k *KillSwitch) GetStatus() map[string]interface{} {
	k.mutex.RLock()
	defer k.mutex.RUnlock()

	return map[string]interface{}{
		"is_triggered": k.isTriggered,
		"trigger_time": k.triggerTime,
		"reason":       k.reason,
	}
}

type GlobalRiskController struct {
	circuitBreaker *CircuitBreaker
	killSwitch     *KillSwitch
	dailyLossLimit float64
	currentLoss    float64
	tradeCount     int
	maxTrades      int
	mutex          sync.RWMutex
}

func NewGlobalRiskController(dailyLossLimit float64, maxTrades int, emergencyClose func() error) *GlobalRiskController {
	return &GlobalRiskController{
		circuitBreaker: NewCircuitBreaker(nil),
		killSwitch:     NewKillSwitch(emergencyClose),
		dailyLossLimit: dailyLossLimit,
		maxTrades:      maxTrades,
	}
}

func (g *GlobalRiskController) CanTrade() bool {
	g.mutex.RLock()
	defer g.mutex.RUnlock()

	if g.killSwitch.IsTriggered() {
		return false
	}

	if g.currentLoss >= g.dailyLossLimit {
		return false
	}

	if g.tradeCount >= g.maxTrades {
		return false
	}

	if g.circuitBreaker.GetState() == CircuitBreakerStateOpen {
		return false
	}

	return true
}

func (g *GlobalRiskController) RecordTrade(pnl float64) {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	g.tradeCount++
	if pnl < 0 {
		g.currentLoss += -pnl
	}
}

func (g *GlobalRiskController) CheckAndTrigger() error {
	g.mutex.RLock()
	defer g.mutex.RUnlock()

	if g.currentLoss >= g.dailyLossLimit {
		return g.killSwitch.Trigger("日亏损超限")
	}

	if g.tradeCount >= g.maxTrades {
		return g.killSwitch.Trigger("交易次数超限")
	}

	return nil
}

func (g *GlobalRiskController) GetCircuitBreaker() *CircuitBreaker {
	return g.circuitBreaker
}

func (g *GlobalRiskController) GetKillSwitch() *KillSwitch {
	return g.killSwitch
}

func (g *GlobalRiskController) GetMetrics() map[string]interface{} {
	g.mutex.RLock()
	defer g.mutex.RUnlock()

	return map[string]interface{}{
		"circuit_breaker": g.circuitBreaker.GetMetrics(),
		"kill_switch":     g.killSwitch.GetStatus(),
		"daily_loss":      g.currentLoss,
		"daily_loss_limit": g.dailyLossLimit,
		"trade_count":     g.tradeCount,
		"max_trades":      g.maxTrades,
		"can_trade":       g.CanTrade(),
	}
}

func (g *GlobalRiskController) Reset() {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	g.currentLoss = 0
	g.tradeCount = 0
	g.circuitBreaker.Reset()
	g.killSwitch.Reset()

	logger.Info("全局风控器已重置")
}

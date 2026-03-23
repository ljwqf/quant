package okx

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/ljwqf/quant/pkg/logger"
	"go.uber.org/zap"
)

type RetryConfig struct {
	MaxRetries      int
	InitialDelay    time.Duration
	MaxDelay        time.Duration
	Multiplier      float64
	RetryableErrors []string
}

var DefaultRetryConfig = &RetryConfig{
	MaxRetries:      3,
	InitialDelay:    100 * time.Millisecond,
	MaxDelay:        5 * time.Second,
	Multiplier:      2.0,
	RetryableErrors: []string{"rate_limit", "timeout", "connection_reset", "503", "502", "429"},
}

type RateLimiter struct {
	tokens         float64
	maxTokens      float64
	refillRate     float64
	lastRefillTime time.Time
	mutex          sync.Mutex
}

func NewRateLimiter(maxTokens, refillRate float64) *RateLimiter {
	return &RateLimiter{
		tokens:         maxTokens,
		maxTokens:      maxTokens,
		refillRate:     refillRate,
		lastRefillTime: time.Now(),
	}
}

func (r *RateLimiter) Allow() bool {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	now := time.Now()
	elapsed := now.Sub(r.lastRefillTime).Seconds()
	r.tokens = math.Min(r.maxTokens, r.tokens+elapsed*r.refillRate)
	r.lastRefillTime = now

	if r.tokens >= 1 {
		r.tokens--
		return true
	}
	return false
}

func (r *RateLimiter) Wait(ctx context.Context) error {
	for {
		if r.Allow() {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func (c *Client) retryOperation(ctx context.Context, operation func() error, config *RetryConfig) error {
	if config == nil {
		config = DefaultRetryConfig
	}

	var lastErr error
	delay := config.InitialDelay

	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		if attempt > 0 {
			logger.Warn("重试操作",
				zap.Int("attempt", attempt),
				zap.Duration("delay", delay),
				zap.Error(lastErr),
			)

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}

			delay = time.Duration(math.Min(float64(delay)*config.Multiplier, float64(config.MaxDelay)))
		}

		err := operation()
		if err == nil {
			return nil
		}

		lastErr = err

		if !c.isRetryableError(err, config.RetryableErrors) {
			return err
		}
	}

	return fmt.Errorf("操作失败，已达到最大重试次数: %w", lastErr)
}

func (c *Client) isRetryableError(err error, retryableErrors []string) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()
	for _, retryableErr := range retryableErrors {
		if contains(errStr, retryableErr) {
			return true
		}
	}
	return false
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func (c *Client) ExecuteWithRetry(ctx context.Context, operation func() error) error {
	return c.retryOperation(ctx, operation, DefaultRetryConfig)
}

func (c *Client) ExecuteWithRetryConfig(ctx context.Context, operation func() error, config *RetryConfig) error {
	return c.retryOperation(ctx, operation, config)
}

type ContextKey string

const (
	ContextKeyRequestID ContextKey = "request_id"
	ContextKeyTimeout   ContextKey = "timeout"
)

func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, ContextKeyRequestID, requestID)
}

func WithTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, timeout)
}

func GetRequestID(ctx context.Context) string {
	if id, ok := ctx.Value(ContextKeyRequestID).(string); ok {
		return id
	}
	return ""
}

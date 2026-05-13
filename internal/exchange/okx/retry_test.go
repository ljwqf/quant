package okx

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRateLimiter_Allow(t *testing.T) {
	// 创建速率限制器：最大10个token，每秒补充1个
	limiter := NewRateLimiter(10, 1)

	// 初始应该允许
	for i := 0; i < 10; i++ {
		assert.True(t, limiter.Allow(), "初始token应该允许请求")
	}

	// 用完后不应该允许
	assert.False(t, limiter.Allow(), "token用完后不应该允许")

	// 等待补充
	time.Sleep(1100 * time.Millisecond)

	// 应该又允许了
	assert.True(t, limiter.Allow(), "补充后应该允许")
}

func TestRateLimiter_Wait(t *testing.T) {
	limiter := NewRateLimiter(1, 10) // 只有1个token，快速补充

	ctx := context.Background()

	// 第一次应该立即可用
	err := limiter.Wait(ctx)
	assert.NoError(t, err)

	// 第二次需要等待
	start := time.Now()
	err = limiter.Wait(ctx)
	elapsed := time.Since(start)
	assert.NoError(t, err)
	assert.True(t, elapsed >= 90*time.Millisecond, "应该有等待时间")
}

func TestRateLimiter_WaitContextCancel(t *testing.T) {
	limiter := NewRateLimiter(0, 0) // 不允许任何请求

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	err := limiter.Wait(ctx)
	assert.Error(t, err)
	assert.Equal(t, context.DeadlineExceeded, err)
}

func TestRetryConfig_Default(t *testing.T) {
	config := DefaultRetryConfig

	assert.Equal(t, 3, config.MaxRetries)
	assert.Equal(t, 100*time.Millisecond, config.InitialDelay)
	assert.Equal(t, 5*time.Second, config.MaxDelay)
	assert.Equal(t, 2.0, config.Multiplier)
	assert.Contains(t, config.RetryableErrors, "rate_limit")
	assert.Contains(t, config.RetryableErrors, "timeout")
}

func TestClient_RetryOperation_Success(t *testing.T) {
	client := &Client{}
	ctx := context.Background()

	attempts := 0
	operation := func() error {
		attempts++
		if attempts < 2 {
			return errors.New("rate_limit exceeded")
		}
		return nil
	}

	err := client.retryOperation(ctx, operation, DefaultRetryConfig)
	assert.NoError(t, err)
	assert.Equal(t, 2, attempts)
}

func TestClient_RetryOperation_MaxRetries(t *testing.T) {
	client := &Client{}
	ctx := context.Background()

	config := &RetryConfig{
		MaxRetries:      2,
		InitialDelay:    10 * time.Millisecond,
		MaxDelay:        100 * time.Millisecond,
		Multiplier:      2.0,
		RetryableErrors: []string{"timeout"},
	}

	attempts := 0
	operation := func() error {
		attempts++
		return errors.New("timeout error")
	}

	err := client.retryOperation(ctx, operation, config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "最大重试次数")
	assert.Equal(t, 3, attempts) // 初始 + 2次重试
}

func TestClient_RetryOperation_NonRetryable(t *testing.T) {
	client := &Client{}
	ctx := context.Background()

	attempts := 0
	operation := func() error {
		attempts++
		return errors.New("invalid_api_key")
	}

	err := client.retryOperation(ctx, operation, DefaultRetryConfig)
	assert.Error(t, err)
	assert.Equal(t, "invalid_api_key", err.Error())
	assert.Equal(t, 1, attempts) // 不重试
}

func TestClient_RetryOperation_ContextCancel(t *testing.T) {
	client := &Client{}
	ctx, cancel := context.WithCancel(context.Background())

	attempts := 0
	operation := func() error {
		attempts++
		if attempts == 1 {
			cancel()
		}
		return errors.New("timeout")
	}

	err := client.retryOperation(ctx, operation, &RetryConfig{
		MaxRetries:      5,
		InitialDelay:    time.Second, // 长延迟
		MaxDelay:        time.Second,
		Multiplier:      1.0,
		RetryableErrors: []string{"timeout"},
	})
	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)
}

func TestClient_IsRetryableError(t *testing.T) {
	client := &Client{}
	retryableErrors := []string{"rate_limit", "timeout", "503"}

	tests := []struct {
		err      error
		expected bool
	}{
		{errors.New("rate_limit exceeded"), true},
		{errors.New("request timeout"), true},
		{errors.New("503 service unavailable"), true},
		{errors.New("invalid api key"), false},
		{errors.New("insufficient balance"), false},
		{nil, false},
	}

	for _, tt := range tests {
		result := client.isRetryableError(tt.err, retryableErrors)
		assert.Equal(t, tt.expected, result)
	}
}

func TestClient_ExecuteWithRetry(t *testing.T) {
	client := &Client{}
	ctx := context.Background()

	attempts := 0
	operation := func() error {
		attempts++
		return nil
	}

	err := client.ExecuteWithRetry(ctx, operation)
	assert.NoError(t, err)
	assert.Equal(t, 1, attempts)
}

func TestClient_ExecuteWithRetryConfig(t *testing.T) {
	client := &Client{}
	ctx := context.Background()

	config := &RetryConfig{
		MaxRetries:      1,
		InitialDelay:    10 * time.Millisecond,
		MaxDelay:        100 * time.Millisecond,
		Multiplier:      2.0,
		RetryableErrors: []string{"error"},
	}

	attempts := 0
	operation := func() error {
		attempts++
		if attempts <= 1 {
			return errors.New("error")
		}
		return nil
	}

	err := client.ExecuteWithRetryConfig(ctx, operation, config)
	assert.NoError(t, err)
	assert.Equal(t, 2, attempts)
}

func TestContains(t *testing.T) {
	tests := []struct {
		s, substr string
		expected  bool
	}{
		{"hello world", "world", true},
		{"hello world", "hello", true},
		{"hello world", "foo", false},
		{"", "", true},
		{"a", "a", true},
		{"ab", "ab", true},
		{"abc", "bc", true},
		{"abc", "d", false},
	}

	for _, tt := range tests {
		result := contains(tt.s, tt.substr)
		assert.Equal(t, tt.expected, result, "contains(%q, %q)", tt.s, tt.substr)
	}
}

func TestContextKeys(t *testing.T) {
	ctx := context.Background()

	// Test RequestID
	ctx = WithRequestID(ctx, "req-123")
	requestID := GetRequestID(ctx)
	assert.Equal(t, "req-123", requestID)

	// Test Timeout
	ctx, cancel := WithTimeout(ctx, 5*time.Second)
	defer cancel()

	deadline, ok := ctx.Deadline()
	assert.True(t, ok)
	assert.True(t, time.Until(deadline) <= 5*time.Second)
}
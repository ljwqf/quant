package recovery

import (
	"fmt"
	"time"

	"github.com/ljwqf/quant/pkg/errors"
	"github.com/ljwqf/quant/pkg/logger"
	"go.uber.org/zap"
)

// RetryConfig 重试配置
type RetryConfig struct {
	MaxRetries    int
	InitialDelay  time.Duration
	MaxDelay      time.Duration
	BackoffFactor float64
}

// DefaultRetryConfig 默认重试配置
var DefaultRetryConfig = &RetryConfig{
	MaxRetries:    3,
	InitialDelay:  1 * time.Second,
	MaxDelay:      30 * time.Second,
	BackoffFactor: 2.0,
}

// Retry 重试函数
func Retry(fn func() error, config *RetryConfig) error {
	if config == nil {
		config = DefaultRetryConfig
	}

	var err error
	for i := 0; i <= config.MaxRetries; i++ {
		err = fn()
		if err == nil {
			return nil
		}

		if i == config.MaxRetries {
			break
		}

		delay := time.Duration(float64(config.InitialDelay) * (config.BackoffFactor * float64(i)))
		if delay > config.MaxDelay {
			delay = config.MaxDelay
		}

		logger.Warn("操作失败，将重试",
			zap.Int("attempt", i+1),
			zap.Int("max_retries", config.MaxRetries),
			zap.Duration("delay", delay),
			zap.Error(err),
		)

		time.Sleep(delay)
	}

	return err
}

// RecoveryHandler 错误恢复处理器
type RecoveryHandler struct {
	name string
}

// NewRecoveryHandler 创建新的恢复处理器
func NewRecoveryHandler(name string) *RecoveryHandler {
	return &RecoveryHandler{
		name: name,
	}
}

// HandleError 处理错误
func (rh *RecoveryHandler) HandleError(err error, metadata map[string]interface{}) error {
	if err == nil {
		return nil
	}

	// 记录错误
	logger.Error(fmt.Sprintf("%s 错误", rh.name),
		zap.Error(err),
		zap.Any("metadata", metadata),
	)

	// 根据错误类型进行不同的处理
	switch {
	case errors.IsNetworkError(err):
		// 网络错误，可能需要重试
		logger.Info("网络错误，将进行重试")
	case errors.IsExchangeError(err):
		// 交易所错误，可能需要等待或降级
		logger.Info("交易所错误，将检查状态")
	case errors.IsStrategyError(err):
		// 策略错误，可能需要调整策略参数
		logger.Info("策略错误，将检查策略状态")
	case errors.IsRiskError(err):
		// 风险错误，需要停止交易
		logger.Warn("风险错误，可能需要停止交易")
	case errors.IsExecutionError(err):
		// 执行错误，可能需要重新执行
		logger.Info("执行错误，将重新执行")
	case errors.IsValidationError(err):
		// 验证错误，需要修复输入
		logger.Warn("验证错误，需要修复输入参数")
	case errors.IsInternalError(err):
		// 内部错误，需要修复代码
		logger.Error("内部错误，需要修复代码")
	default:
		// 未知错误
		logger.Error("未知错误类型")
	}

	return err
}

// RecoverFromPanic 从panic中恢复
func RecoverFromPanic(name string) {
	if r := recover(); r != nil {
		logger.Error(fmt.Sprintf("%s 发生panic", name),
			zap.Any("panic", r),
		)
	}
}

// WithRecovery 包装函数，添加错误处理和panic恢复
func WithRecovery(name string, fn func() error, metadata map[string]interface{}) error {
	handler := NewRecoveryHandler(name)

	defer RecoverFromPanic(name)

	err := fn()
	return handler.HandleError(err, metadata)
}

// WithRetry 包装函数，添加重试机制
func WithRetry(name string, fn func() error, config *RetryConfig, metadata map[string]interface{}) error {
	handler := NewRecoveryHandler(name)

	err := Retry(func() error {
		defer RecoverFromPanic(name)
		return fn()
	}, config)

	return handler.HandleError(err, metadata)
}

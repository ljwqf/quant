package errors

import (
	"fmt"
	"time"
)

// ErrorType 错误类型
type ErrorType string

const (
	ErrorTypeNetwork    ErrorType = "network"
	ErrorTypeExchange   ErrorType = "exchange"
	ErrorTypeStrategy   ErrorType = "strategy"
	ErrorTypeRisk       ErrorType = "risk"
	ErrorTypeExecution  ErrorType = "execution"
	ErrorTypeValidation ErrorType = "validation"
	ErrorTypeInternal   ErrorType = "internal"
)

// AppError 应用错误结构体
type AppError struct {
	Type      ErrorType
	Message   string
	Original  error
	Timestamp time.Time
	Metadata  map[string]interface{}
}

// Error 实现error接口
func (e *AppError) Error() string {
	if e.Original != nil {
		return fmt.Sprintf("%s: %s (original: %v)", e.Type, e.Message, e.Original)
	}
	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

// New 创建新的应用错误
func New(errorType ErrorType, message string, original error, metadata map[string]interface{}) *AppError {
	return &AppError{
		Type:      errorType,
		Message:   message,
		Original:  original,
		Timestamp: time.Now(),
		Metadata:  metadata,
	}
}

// NewNetworkError 创建网络错误
func NewNetworkError(message string, original error, metadata map[string]interface{}) *AppError {
	return New(ErrorTypeNetwork, message, original, metadata)
}

// NewExchangeError 创建交易所错误
func NewExchangeError(message string, original error, metadata map[string]interface{}) *AppError {
	return New(ErrorTypeExchange, message, original, metadata)
}

// NewStrategyError 创建策略错误
func NewStrategyError(message string, original error, metadata map[string]interface{}) *AppError {
	return New(ErrorTypeStrategy, message, original, metadata)
}

// NewRiskError 创建风险错误
func NewRiskError(message string, original error, metadata map[string]interface{}) *AppError {
	return New(ErrorTypeRisk, message, original, metadata)
}

// NewExecutionError 创建执行错误
func NewExecutionError(message string, original error, metadata map[string]interface{}) *AppError {
	return New(ErrorTypeExecution, message, original, metadata)
}

// NewValidationError 创建验证错误
func NewValidationError(message string, original error, metadata map[string]interface{}) *AppError {
	return New(ErrorTypeValidation, message, original, metadata)
}

// NewInternalError 创建内部错误
func NewInternalError(message string, original error, metadata map[string]interface{}) *AppError {
	return New(ErrorTypeInternal, message, original, metadata)
}

// IsNetworkError 检查是否为网络错误
func IsNetworkError(err error) bool {
	if appErr, ok := err.(*AppError); ok {
		return appErr.Type == ErrorTypeNetwork
	}
	return false
}

// IsExchangeError 检查是否为交易所错误
func IsExchangeError(err error) bool {
	if appErr, ok := err.(*AppError); ok {
		return appErr.Type == ErrorTypeExchange
	}
	return false
}

// IsStrategyError 检查是否为策略错误
func IsStrategyError(err error) bool {
	if appErr, ok := err.(*AppError); ok {
		return appErr.Type == ErrorTypeStrategy
	}
	return false
}

// IsRiskError 检查是否为风险错误
func IsRiskError(err error) bool {
	if appErr, ok := err.(*AppError); ok {
		return appErr.Type == ErrorTypeRisk
	}
	return false
}

// IsExecutionError 检查是否为执行错误
func IsExecutionError(err error) bool {
	if appErr, ok := err.(*AppError); ok {
		return appErr.Type == ErrorTypeExecution
	}
	return false
}

// IsValidationError 检查是否为验证错误
func IsValidationError(err error) bool {
	if appErr, ok := err.(*AppError); ok {
		return appErr.Type == ErrorTypeValidation
	}
	return false
}

// IsInternalError 检查是否为内部错误
func IsInternalError(err error) bool {
	if appErr, ok := err.(*AppError); ok {
		return appErr.Type == ErrorTypeInternal
	}
	return false
}

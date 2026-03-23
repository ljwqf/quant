package errors

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewError(t *testing.T) {
	originalErr := errors.New("original error")
	err := New(ErrorTypeInternal, "test message", originalErr, map[string]interface{}{
		"key": "value",
	})

	assert.NotNil(t, err)
	assert.Equal(t, ErrorTypeInternal, err.Type)
	assert.Equal(t, "test message", err.Message)
	assert.Equal(t, originalErr, err.Original)
	assert.Equal(t, "value", err.Metadata["key"])
	assert.False(t, err.Timestamp.IsZero())
}

func TestErrorString(t *testing.T) {
	originalErr := errors.New("original")
	err := New(ErrorTypeNetwork, "test", originalErr, nil)

	str := err.Error()
	assert.Contains(t, str, "network")
	assert.Contains(t, str, "test")
	assert.Contains(t, str, "original")
}

func TestErrorTypeHelpers(t *testing.T) {
	tests := []struct {
		name       string
		errorType  ErrorType
		checkFunc  func(error) bool
		shouldPass bool
	}{
		{
			name:       "network error",
			errorType:  ErrorTypeNetwork,
			checkFunc:  IsNetworkError,
			shouldPass: true,
		},
		{
			name:       "exchange error",
			errorType:  ErrorTypeExchange,
			checkFunc:  IsExchangeError,
			shouldPass: true,
		},
		{
			name:       "strategy error",
			errorType:  ErrorTypeStrategy,
			checkFunc:  IsStrategyError,
			shouldPass: true,
		},
		{
			name:       "risk error",
			errorType:  ErrorTypeRisk,
			checkFunc:  IsRiskError,
			shouldPass: true,
		},
		{
			name:       "execution error",
			errorType:  ErrorTypeExecution,
			checkFunc:  IsExecutionError,
			shouldPass: true,
		},
		{
			name:       "validation error",
			errorType:  ErrorTypeValidation,
			checkFunc:  IsValidationError,
			shouldPass: true,
		},
		{
			name:       "internal error",
			errorType:  ErrorTypeInternal,
			checkFunc:  IsInternalError,
			shouldPass: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := New(tt.errorType, "test", nil, nil)
			assert.True(t, tt.checkFunc(err))

			otherErr := New(ErrorTypeNetwork, "other", nil, nil)
			if tt.errorType != ErrorTypeNetwork {
				assert.False(t, tt.checkFunc(otherErr))
			}
		})
	}
}

func TestStandardError(t *testing.T) {
	stdErr := errors.New("standard error")
	assert.False(t, IsNetworkError(stdErr))
	assert.False(t, IsExchangeError(stdErr))
	assert.False(t, IsStrategyError(stdErr))
	assert.False(t, IsRiskError(stdErr))
	assert.False(t, IsExecutionError(stdErr))
	assert.False(t, IsValidationError(stdErr))
	assert.False(t, IsInternalError(stdErr))
}

func TestHelperFunctions(t *testing.T) {
	tests := []struct {
		name     string
		fn       func(string, error, map[string]interface{}) *AppError
		errType  ErrorType
	}{
		{
			name:     "NewNetworkError",
			fn:       NewNetworkError,
			errType:  ErrorTypeNetwork,
		},
		{
			name:     "NewExchangeError",
			fn:       NewExchangeError,
			errType:  ErrorTypeExchange,
		},
		{
			name:     "NewStrategyError",
			fn:       NewStrategyError,
			errType:  ErrorTypeStrategy,
		},
		{
			name:     "NewRiskError",
			fn:       NewRiskError,
			errType:  ErrorTypeRisk,
		},
		{
			name:     "NewExecutionError",
			fn:       NewExecutionError,
			errType:  ErrorTypeExecution,
		},
		{
			name:     "NewValidationError",
			fn:       NewValidationError,
			errType:  ErrorTypeValidation,
		},
		{
			name:     "NewInternalError",
			fn:       NewInternalError,
			errType:  ErrorTypeInternal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.fn("test message", nil, nil)
			assert.Equal(t, tt.errType, err.Type)
			assert.Equal(t, "test message", err.Message)
		})
	}
}

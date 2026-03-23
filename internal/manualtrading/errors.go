package manualtrading

import "errors"

var (
	ErrInvalidSize         = errors.New("invalid order size")
	ErrInvalidExecuteTime  = errors.New("execute time must be in the future")
	ErrOrderNotFound       = errors.New("order not found")
	ErrOrderNotPending     = errors.New("order is not in pending status")
	ErrInvalidStopDistance = errors.New("stop distance must be greater than 0")
	ErrPositionNotFound    = errors.New("position not found")
)

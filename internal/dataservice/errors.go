package dataservice

import "errors"

var (
	// ErrQueueFull 队列已满
	ErrQueueFull = errors.New("data queue is full")
	// ErrQueueEmpty 队列为空
	ErrQueueEmpty = errors.New("data queue is empty")
	// ErrSourceNotFound 数据源未找到
	ErrSourceNotFound = errors.New("data source not found")
	// ErrSourceAlreadyExists 数据源已存在
	ErrSourceAlreadyExists = errors.New("data source already exists")
	// ErrSourceNotInitialized 数据源未初始化
	ErrSourceNotInitialized = errors.New("data source not initialized")
	// ErrSourceUnhealthy 数据源不健康
	ErrSourceUnhealthy = errors.New("data source is unhealthy")
	// ErrInvalidConfig 无效的配置
	ErrInvalidConfig = errors.New("invalid configuration")
	// ErrSymbolNotFound 交易对未找到
	ErrSymbolNotFound = errors.New("symbol not found")
	// ErrFetchFailed 获取数据失败
	ErrFetchFailed = errors.New("failed to fetch data")
)

package dataservice

import (
	"time"

	"github.com/ljwqf/quant/pkg/types"
)

// DataSourceType 数据源类型
type DataSourceType string

const (
	// DataSourceTypeExchange 交易所数据源
	DataSourceTypeExchange DataSourceType = "exchange"
	// DataSourceTypeNews 新闻数据源
	DataSourceTypeNews DataSourceType = "news"
	// DataSourceTypeEconomic 经济事件数据源
	DataSourceTypeEconomic DataSourceType = "economic"
	// DataSourceTypeOnChain 链上数据源
	DataSourceTypeOnChain DataSourceType = "onchain"
)

// DataSource 数据源接口
type DataSource interface {
	// Name 获取数据源名称
	Name() string
	// Type 获取数据源类型
	Type() DataSourceType
	// Initialize 初始化数据源
	Initialize(config map[string]interface{}) error
	// FetchTick 获取行情数据
	FetchTick(symbol string) (*types.Tick, error)
	// FetchBars 获取K线数据
	FetchBars(symbol string, interval string, limit int) ([]*types.Bar, error)
	// FetchOrderBook 获取订单簿数据
	FetchOrderBook(symbol string, depth int) (*types.OrderBook, error)
	// IsHealthy 检查数据源健康状态
	IsHealthy() bool
	// Close 关闭数据源连接
	Close() error
}

// MarketData 市场数据
type MarketData struct {
	Symbol     string
	DataType   string // tick, kline, orderbook
	Data       interface{}
	Timestamp  time.Time
	DataSource string
}

// DataQueue 数据队列接口
type DataQueue interface {
	// Push 推送数据到队列
	Push(data *MarketData) error
	// Pop 从队列弹出数据
	Pop() (*MarketData, error)
	// Size 获取队列大小
	Size() int
	// Close 关闭队列
	Close() error
}

// MemoryQueue 内存队列实现
type MemoryQueue struct {
	data chan *MarketData
	size int
}

// NewMemoryQueue 创建内存队列
func NewMemoryQueue(size int) *MemoryQueue {
	return &MemoryQueue{
		data: make(chan *MarketData, size),
		size: size,
	}
}

// Push 推送数据到队列
func (q *MemoryQueue) Push(data *MarketData) error {
	select {
	case q.data <- data:
		return nil
	default:
		return ErrQueueFull
	}
}

// Pop 从队列弹出数据
func (q *MemoryQueue) Pop() (*MarketData, error) {
	select {
	case data := <-q.data:
		return data, nil
	default:
		return nil, ErrQueueEmpty
	}
}

// Size 获取队列大小
func (q *MemoryQueue) Size() int {
	return len(q.data)
}

// Close 关闭队列
func (q *MemoryQueue) Close() error {
	close(q.data)
	return nil
}

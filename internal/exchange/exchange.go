package exchange

import (
	"github.com/ljwqf/quant/pkg/types"
)

// Exchange 交易所接口
type Exchange interface {
	// Connect 连接到交易所
	Connect() error

	// Disconnect 断开连接
	Disconnect() error

	// GetAccount 获取账户信息
	GetAccount() (*types.Account, error)

	// PlaceOrder 下单
	PlaceOrder(order *types.Order) (*types.OrderResult, error)

	// CancelOrder 撤单
	CancelOrder(orderID string) error

	// GetOrder 获取订单信息
	GetOrder(orderID string) (*types.Order, error)

	// GetOrders 获取订单列表
	GetOrders(symbol string, limit int) ([]*types.Order, error)

	// GetPositions 获取持仓信息
	GetPositions() ([]*types.Position, error)

	// SubscribeTicker 订阅行情
	SubscribeTicker(symbol string, handler func(*types.Tick)) error

	// SubscribeBar 订阅K线
	SubscribeBar(symbol string, interval string, handler func(*types.Bar)) error

	// SubscribeOrderBook 订阅订单簿
	SubscribeOrderBook(symbol string, handler func(*types.OrderBook)) error

	// GetBars 获取历史K线
	GetBars(symbol string, interval string, limit int) ([]*types.Bar, error)

	// GetTicker 获取最新行情
	GetTicker(symbol string) (*types.Tick, error)

	// GetOrderBook 获取订单簿
	GetOrderBook(symbol string, depth int) (*types.OrderBook, error)

	// SetLeverage 调整杠杆
	SetLeverage(symbol string, leverage int, marginMode string) error
}

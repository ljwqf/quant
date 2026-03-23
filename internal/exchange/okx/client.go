package okx

import (
	"sync"

	"github.com/ljwqf/quant/internal/config"
	"github.com/ljwqf/quant/pkg/logger"
	"github.com/ljwqf/quant/pkg/types"
	"go.uber.org/zap"
)

// Client OKX 客户端
type Client struct {
	config         *config.OKXConfig
	restClient     *restClient
	wsClient       *wsClient
	connected      bool
	mutex          sync.RWMutex
	tickerHandlers map[string][]func(*types.Tick)
	barHandlers    map[string]map[string][]func(*types.Bar) // symbol -> interval -> handlers
	orderBookHandlers map[string][]func(*types.OrderBook)
}

// NewClient 创建 OKX 客户端
func NewClient(cfg *config.OKXConfig) *Client {
	return &Client{
		config:            cfg,
		tickerHandlers:    make(map[string][]func(*types.Tick)),
		barHandlers:       make(map[string]map[string][]func(*types.Bar)),
		orderBookHandlers: make(map[string][]func(*types.OrderBook)),
	}
}

// Connect 连接到 OKX
func (c *Client) Connect() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.connected {
		return nil
	}

	// 初始化 REST 客户端
	c.restClient = newRestClient(c.config)

	// 初始化 WebSocket 客户端
	var err error
	c.wsClient, err = newWSClient(c.config, c.handleWSMessage)
	if err != nil {
		logger.Warn("创建 WebSocket 客户端失败，将仅使用 REST API", zap.Error(err))
		c.connected = true
		return nil
	}

	// 连接 WebSocket
	if err := c.wsClient.connect(); err != nil {
		logger.Warn("WebSocket 连接失败，将仅使用 REST API", zap.Error(err))
		c.connected = true
		return nil
	}

	c.connected = true
	logger.Info("OKX 客户端已连接")
	return nil
}

// Disconnect 断开连接
func (c *Client) Disconnect() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if !c.connected {
		return nil
	}

	// 断开 WebSocket 连接
	if c.wsClient != nil {
		if err := c.wsClient.disconnect(); err != nil {
			logger.Error("断开 WebSocket 连接失败", zap.Error(err))
		}
	}

	c.connected = false
	logger.Info("OKX 客户端已断开连接")
	return nil
}

// GetAccount 获取账户信息
func (c *Client) GetAccount() (*types.Account, error) {
	if !c.connected {
		if err := c.Connect(); err != nil {
			return nil, err
		}
	}

	return c.restClient.getAccount()
}

// GetPositions 获取持仓信息
func (c *Client) GetPositions() ([]*types.Position, error) {
	if !c.connected {
		if err := c.Connect(); err != nil {
			return nil, err
		}
	}

	return c.restClient.getPositions()
}

// GetTicker 获取最新行情
func (c *Client) GetTicker(symbol string) (*types.Tick, error) {
	if !c.connected {
		if err := c.Connect(); err != nil {
			return nil, err
		}
	}

	return c.restClient.getTicker(symbol)
}

// GetBars 获取历史K线
func (c *Client) GetBars(symbol string, interval string, limit int) ([]*types.Bar, error) {
	if !c.connected {
		if err := c.Connect(); err != nil {
			return nil, err
		}
	}

	return c.restClient.getBars(symbol, interval, limit)
}

// GetOrderBook 获取订单簿
func (c *Client) GetOrderBook(symbol string, depth int) (*types.OrderBook, error) {
	if !c.connected {
		if err := c.Connect(); err != nil {
			return nil, err
		}
	}

	return c.restClient.getOrderBook(symbol, depth)
}

// IsConnected 检查是否已连接
func (c *Client) IsConnected() bool {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.connected
}

// GetOrderHistory 获取订单历史
func (c *Client) GetOrderHistory(symbol string, limit int) ([]*types.Order, error) {
	// 这里可以实现获取订单历史的逻辑
	// 暂时返回空列表
	return []*types.Order{}, nil
}

// GetAccountInfo 获取账户信息
func (c *Client) GetAccountInfo() (*types.Account, error) {
	return c.GetAccount()
}

// SetLeverage 调整杠杆
func (c *Client) SetLeverage(symbol string, leverage int, marginMode string) error {
	if !c.connected {
		if err := c.Connect(); err != nil {
			return err
		}
	}

	return c.restClient.setLeverage(symbol, leverage, marginMode)
}



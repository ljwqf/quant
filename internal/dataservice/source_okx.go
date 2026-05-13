package dataservice

import (
	"fmt"

	"github.com/ljwqf/quant/internal/config"
	"github.com/ljwqf/quant/internal/exchange/okx"
	"github.com/ljwqf/quant/pkg/logger"
	"github.com/ljwqf/quant/pkg/types"
	"go.uber.org/zap"
)

// OKXSource OKX交易所数据源
type OKXSource struct {
	name     string
	client   *okx.Client
	config   *config.OKXConfig
	healthy  bool
}

// NewOKXSource 创建OKX数据源
func NewOKXSource() *OKXSource {
	return &OKXSource{
		name:    "okx",
		healthy: false,
	}
}

// Name 获取数据源名称
func (s *OKXSource) Name() string {
	return s.name
}

// Type 获取数据源类型
func (s *OKXSource) Type() DataSourceType {
	return DataSourceTypeExchange
}

// Initialize 初始化数据源
func (s *OKXSource) Initialize(cfg map[string]interface{}) error {
	apiKey, ok := cfg["api_key"].(string)
	if !ok {
		return fmt.Errorf("api_key required")
	}

	secretKey, ok := cfg["secret_key"].(string)
	if !ok {
		return fmt.Errorf("secret_key required")
	}

	passphrase, ok := cfg["passphrase"].(string)
	if !ok {
		return fmt.Errorf("passphrase required")
	}

	simulated, _ := cfg["simulated"].(bool)

	s.config = &config.OKXConfig{
		APIKey:     apiKey,
		SecretKey:  secretKey,
		Passphrase: passphrase,
		Simulated:  simulated,
	}

	s.client = okx.NewClient(s.config)
	
	if err := s.client.Connect(); err != nil {
		logger.Error("OKX数据源连接失败", zap.Error(err))
		return err
	}

	s.healthy = true
	logger.Info("OKX数据源初始化成功")
	return nil
}

// FetchTick 获取行情数据
func (s *OKXSource) FetchTick(symbol string) (*types.Tick, error) {
	if !s.healthy {
		return nil, ErrSourceUnhealthy
	}

	return s.client.GetTicker(symbol)
}

// FetchBars 获取K线数据
func (s *OKXSource) FetchBars(symbol string, interval string, limit int) ([]*types.Bar, error) {
	if !s.healthy {
		return nil, ErrSourceUnhealthy
	}

	return s.client.GetBars(symbol, interval, limit)
}

// FetchOrderBook 获取订单簿数据
func (s *OKXSource) FetchOrderBook(symbol string, depth int) (*types.OrderBook, error) {
	if !s.healthy {
		return nil, ErrSourceUnhealthy
	}

	return s.client.GetOrderBook(symbol, depth)
}

// IsHealthy 检查数据源健康状态
func (s *OKXSource) IsHealthy() bool {
	if !s.healthy {
		return false
	}
	return s.client.IsConnected()
}

// Close 关闭数据源连接
func (s *OKXSource) Close() error {
	s.healthy = false
	if s.client != nil {
		return s.client.Disconnect()
	}
	return nil
}

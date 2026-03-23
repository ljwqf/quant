package manualtrading

import (
	"github.com/ljwqf/quant/internal/config"
	"github.com/ljwqf/quant/internal/storage"
)

// MarketData 市场数据管理器
type MarketData struct {
	cfg *config.ManualTradingConfig
	db  *storage.Database
}

// NewMarketData 创建市场数据管理器
func NewMarketData(cfg *config.ManualTradingConfig, db *storage.Database) *MarketData {
	return &MarketData{
		cfg: cfg,
		db:  db,
	}
}

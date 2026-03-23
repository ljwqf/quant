package manualtrading

import (
	"github.com/ljwqf/quant/internal/config"
	"github.com/ljwqf/quant/internal/exchange"
	"github.com/ljwqf/quant/internal/storage"
	"github.com/ljwqf/quant/pkg/logger"
)

type Manager struct {
	cfg           *config.ManualTradingConfig
	db            *storage.Database
	exchange      exchange.Exchange
	orderMgr      *OrderManager
	posMgr        *PositionManager
	timedOrderMgr *TimedOrderManager
}

func NewManager(cfg *config.ManualTradingConfig, db *storage.Database, exchange exchange.Exchange) *Manager {
	if !cfg.Enable {
		logger.Info("手动交易功能未启用")
		return nil
	}

	m := &Manager{
		cfg:      cfg,
		db:       db,
		exchange: exchange,
	}

	m.orderMgr = NewOrderManager(cfg, db, exchange)
	m.posMgr = NewPositionManager(cfg, db, exchange)
	m.timedOrderMgr = NewTimedOrderManager(cfg, db, exchange)

	logger.Info("手动交易管理器初始化成功")
	return m
}

func (m *Manager) Start() {
	if m.timedOrderMgr != nil {
		m.timedOrderMgr.Start()
	}
	if m.posMgr != nil {
		m.posMgr.Start()
	}
}

func (m *Manager) Stop() {
	if m.timedOrderMgr != nil {
		m.timedOrderMgr.Stop()
	}
	if m.posMgr != nil {
		m.posMgr.Stop()
	}
}

func (m *Manager) OrderManager() *OrderManager {
	return m.orderMgr
}

func (m *Manager) PositionManager() *PositionManager {
	return m.posMgr
}

func (m *Manager) TimedOrderManager() *TimedOrderManager {
	return m.timedOrderMgr
}

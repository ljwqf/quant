package manualtrading

import (
	"sync"
	"time"

	"github.com/ljwqf/quant/internal/config"
	"github.com/ljwqf/quant/internal/exchange"
	"github.com/ljwqf/quant/internal/storage"
	"github.com/ljwqf/quant/pkg/logger"
	"github.com/ljwqf/quant/pkg/types"
	"go.uber.org/zap"
)

type TrailingStop struct {
	Symbol       string
	Side         types.OrderSide
	EntryPrice   float64
	StopDistance float64
	LastPrice    float64
	TriggerPrice float64
	Activated    bool
	CreatedAt    time.Time
}

type PositionManager struct {
	cfg           *config.ManualTradingConfig
	db            *storage.Database
	exchange      exchange.Exchange
	trailingStops map[string]*TrailingStop
	mu            sync.RWMutex
	stopCh        chan struct{}
	running       bool
}

func NewPositionManager(cfg *config.ManualTradingConfig, db *storage.Database, exchange exchange.Exchange) *PositionManager {
	return &PositionManager{
		cfg:           cfg,
		db:            db,
		exchange:      exchange,
		trailingStops: make(map[string]*TrailingStop),
		stopCh:        make(chan struct{}),
	}
}

func (pm *PositionManager) Start() {
	pm.mu.Lock()
	if pm.running {
		pm.mu.Unlock()
		return
	}
	pm.running = true
	pm.mu.Unlock()

	go pm.monitorTrailingStops()
	logger.Info("移动止损监控器已启动")
}

func (pm *PositionManager) Stop() {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if !pm.running {
		return
	}

	close(pm.stopCh)
	pm.running = false
	logger.Info("移动止损监控器已停止")
}

func (pm *PositionManager) monitorTrailingStops() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			pm.checkAllTrailingStops()
		case <-pm.stopCh:
			return
		}
	}
}

func (pm *PositionManager) checkAllTrailingStops() {
	pm.mu.RLock()
	symbols := make([]string, 0, len(pm.trailingStops))
	for symbol, ts := range pm.trailingStops {
		if ts.Activated {
			symbols = append(symbols, symbol)
		}
	}
	pm.mu.RUnlock()

	for _, symbol := range symbols {
		ticker, err := pm.exchange.GetTicker(symbol)
		if err != nil {
			logger.Warn("获取行情失败，跳过移动止损检查", zap.String("symbol", symbol), zap.Error(err))
			continue
		}

		ts, shouldTrigger := pm.UpdateTrailingStop(symbol, ticker.Price)
		if shouldTrigger && ts != nil {
			if err := pm.ClosePosition(symbol, 0); err != nil {
				logger.Error("移动止损平仓失败", zap.String("symbol", symbol), zap.Error(err))
			} else {
				pm.RemoveTrailingStop(symbol)
				logger.Info("移动止损平仓成功", zap.String("symbol", symbol))
			}
		}
	}
}

func (pm *PositionManager) ClosePosition(symbol string, size float64) error {
	logger.Info("平仓", zap.String("symbol", symbol), zap.Float64("size", size))

	positions, err := pm.exchange.GetPositions()
	if err != nil {
		logger.Error("获取持仓失败", zap.Error(err))
		return err
	}

	var targetPos *types.Position
	for _, pos := range positions {
		if pos.Symbol == symbol {
			targetPos = pos
			break
		}
	}

	if targetPos == nil {
		logger.Warn("未找到持仓", zap.String("symbol", symbol))
		return nil
	}

	side := types.OrderSideSell
	closeSize := targetPos.Size
	if targetPos.Size < 0 {
		side = types.OrderSideBuy
		closeSize = -targetPos.Size
	}

	if size > 0 && size < closeSize {
		closeSize = size
	}

	order := &types.Order{
		Symbol:   symbol,
		Side:     side,
		Type:     types.OrderTypeMarket,
		Quantity: closeSize,
	}

	_, err = pm.exchange.PlaceOrder(order)
	if err != nil {
		logger.Error("平仓订单失败", zap.Error(err))
		return err
	}

	logger.Info("平仓订单已提交", zap.String("symbol", symbol))
	return nil
}

func (pm *PositionManager) SetTakeProfitStopLoss(symbol string, takeProfit, stopLoss float64) error {
	logger.Info("设置止盈止损",
		zap.String("symbol", symbol),
		zap.Float64("take_profit", takeProfit),
		zap.Float64("stop_loss", stopLoss),
	)
	return nil
}

func (pm *PositionManager) GetPositions() ([]*types.Position, error) {
	return pm.exchange.GetPositions()
}

func (pm *PositionManager) SetLeverage(symbol string, leverage int, marginMode string) error {
	logger.Info("调整杠杆",
		zap.String("symbol", symbol),
		zap.Int("leverage", leverage),
		zap.String("margin_mode", marginMode),
	)

	if err := pm.exchange.SetLeverage(symbol, leverage, marginMode); err != nil {
		logger.Error("调整杠杆失败", zap.Error(err))
		return err
	}

	logger.Info("杠杆调整成功",
		zap.String("symbol", symbol),
		zap.Int("leverage", leverage),
	)
	return nil
}

func (pm *PositionManager) SetTrailingStop(symbol string, stopDistance float64) error {
	logger.Info("设置移动止损",
		zap.String("symbol", symbol),
		zap.Float64("stop_distance", stopDistance),
	)

	positions, err := pm.exchange.GetPositions()
	if err != nil {
		logger.Error("获取持仓失败", zap.Error(err))
		return err
	}

	var targetPos *types.Position
	for _, pos := range positions {
		if pos.Symbol == symbol {
			targetPos = pos
			break
		}
	}

	if targetPos == nil {
		logger.Warn("未找到持仓", zap.String("symbol", symbol))
		return nil
	}

	ticker, err := pm.exchange.GetTicker(symbol)
	if err != nil {
		logger.Error("获取行情失败", zap.Error(err))
		return err
	}

	currentPrice := ticker.Price

	var triggerPrice float64
	if targetPos.Side == types.OrderSideBuy {
		triggerPrice = currentPrice * (1 - stopDistance/100)
	} else {
		triggerPrice = currentPrice * (1 + stopDistance/100)
	}

	pm.mu.Lock()
	pm.trailingStops[symbol] = &TrailingStop{
		Symbol:       symbol,
		Side:         targetPos.Side,
		EntryPrice:   targetPos.EntryPrice,
		StopDistance: stopDistance,
		LastPrice:    currentPrice,
		TriggerPrice: triggerPrice,
		Activated:    true,
		CreatedAt:    time.Now(),
	}
	pm.mu.Unlock()

	logger.Info("移动止损设置成功",
		zap.String("symbol", symbol),
		zap.Float64("current_price", currentPrice),
		zap.Float64("trigger_price", triggerPrice),
	)
	return nil
}

func (pm *PositionManager) UpdateTrailingStop(symbol string, currentPrice float64) (*TrailingStop, bool) {
	pm.mu.RLock()
	ts, exists := pm.trailingStops[symbol]
	pm.mu.RUnlock()

	if !exists || !ts.Activated {
		return nil, false
	}

	shouldTrigger := false
	var newTriggerPrice float64

	if ts.Side == types.OrderSideBuy {
		if currentPrice > ts.LastPrice {
			newTriggerPrice = currentPrice * (1 - ts.StopDistance/100)
			if newTriggerPrice > ts.TriggerPrice {
				ts.TriggerPrice = newTriggerPrice
				ts.LastPrice = currentPrice
				logger.Debug("移动止损触发价更新",
					zap.String("symbol", symbol),
					zap.Float64("new_trigger_price", newTriggerPrice),
				)
			}
		}
		if currentPrice <= ts.TriggerPrice {
			shouldTrigger = true
		}
	} else {
		if currentPrice < ts.LastPrice {
			newTriggerPrice = currentPrice * (1 + ts.StopDistance/100)
			if newTriggerPrice < ts.TriggerPrice {
				ts.TriggerPrice = newTriggerPrice
				ts.LastPrice = currentPrice
				logger.Debug("移动止损触发价更新",
					zap.String("symbol", symbol),
					zap.Float64("new_trigger_price", newTriggerPrice),
				)
			}
		}
		if currentPrice >= ts.TriggerPrice {
			shouldTrigger = true
		}
	}

	if shouldTrigger {
		logger.Info("移动止损触发",
			zap.String("symbol", symbol),
			zap.Float64("current_price", currentPrice),
			zap.Float64("trigger_price", ts.TriggerPrice),
		)
		return ts, true
	}

	return ts, false
}

func (pm *PositionManager) RemoveTrailingStop(symbol string) {
	pm.mu.Lock()
	delete(pm.trailingStops, symbol)
	pm.mu.Unlock()
	logger.Info("移除移动止损", zap.String("symbol", symbol))
}

func (pm *PositionManager) GetTrailingStop(symbol string) *TrailingStop {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.trailingStops[symbol]
}

func (pm *PositionManager) GetAllTrailingStops() map[string]*TrailingStop {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	result := make(map[string]*TrailingStop)
	for k, v := range pm.trailingStops {
		result[k] = v
	}
	return result
}

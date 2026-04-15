package execution

import (
	"math"
	"sync"

	"github.com/ljwqf/quant/internal/exchange"
	"github.com/ljwqf/quant/pkg/logger"
	"github.com/ljwqf/quant/pkg/types"
	"go.uber.org/zap"
)

// ExchangeAlgoOrderManager 交易所侧算法单管理器（替代本地 TP/SL 监控）
type ExchangeAlgoOrderManager struct {
	exchange exchange.Exchange
	orders   map[string]*trackedAlgoOrder // key: algoID
	mutex    sync.RWMutex
	tdMode   string
}

type trackedAlgoOrder struct {
	AlgoID      string
	Symbol      string
	Side        types.OrderSide
	EntryPrice  float64
	Size        float64
	SlTriggerPx float64
	TpTriggerPx float64
}

// NewExchangeAlgoOrderManager 创建交易所侧算法单管理器
func NewExchangeAlgoOrderManager(ex exchange.Exchange, tdMode string) *ExchangeAlgoOrderManager {
	if tdMode == "" {
		tdMode = "isolated"
	}
	return &ExchangeAlgoOrderManager{
		exchange: ex,
		orders:   make(map[string]*trackedAlgoOrder),
		tdMode:   tdMode,
	}
}

// PlaceTPSL 同时下发止盈止损条件单（OKX 一单支持 TP+SL）
func (m *ExchangeAlgoOrderManager) PlaceTPSL(symbol string, side types.OrderSide, size, entryPrice, tpPx, slPx float64) (string, error) {
	exitSide := exitSideAlgo(side)

	algoOrder := &types.AlgoOrder{
		Symbol:   symbol,
		Side:     exitSide,
		OrdType:  types.AlgoOrderConditional,
		Size:     size,
		TdMode:   m.tdMode,
	}

	if tpPx > 0 {
		algoOrder.TpTriggerPx = tpPx
		algoOrder.TpOrderPx = -1 // 市价止盈
	}
	if slPx > 0 {
		algoOrder.SlTriggerPx = slPx
		algoOrder.SlOrderPx = -1 // 市价止损
	}

	if tpPx <= 0 && slPx <= 0 {
		return "", nil
	}

	result, err := m.exchange.PlaceAlgoOrder(algoOrder)
	if err != nil {
		logger.Warn("下发交易所侧 TP/SL 单失败",
			zap.String("symbol", symbol),
			zap.Float64("tp_px", tpPx),
			zap.Float64("sl_px", slPx),
			zap.Error(err),
		)
		return "", err
	}

	m.mutex.Lock()
	m.orders[result.AlgoID] = &trackedAlgoOrder{
		AlgoID:      result.AlgoID,
		Symbol:      symbol,
		Side:        side,
		EntryPrice:  entryPrice,
		Size:        size,
		SlTriggerPx: slPx,
		TpTriggerPx: tpPx,
	}
	m.mutex.Unlock()

	logger.Info("交易所侧 TP/SL 单已下发",
		zap.String("symbol", symbol),
		zap.String("algo_id", result.AlgoID),
		zap.Float64("entry_price", entryPrice),
		zap.Float64("tp_px", tpPx),
		zap.Float64("sl_px", slPx),
	)

	return result.AlgoID, nil
}

// PlaceStopLoss 单独下发止损单
func (m *ExchangeAlgoOrderManager) PlaceStopLoss(symbol string, side types.OrderSide, size, entryPrice, stopPx float64) (string, error) {
	exitSide := exitSideAlgo(side)

	algoOrder := &types.AlgoOrder{
		Symbol:      symbol,
		Side:        exitSide,
		OrdType:     types.AlgoOrderConditional,
		Size:        size,
		SlTriggerPx: stopPx,
		SlOrderPx:   -1,
		TdMode:      m.tdMode,
	}

	result, err := m.exchange.PlaceAlgoOrder(algoOrder)
	if err != nil {
		logger.Warn("下发交易所侧止损单失败",
			zap.String("symbol", symbol),
			zap.Float64("stop_px", stopPx),
			zap.Error(err),
		)
		return "", err
	}

	m.mutex.Lock()
	m.orders[result.AlgoID] = &trackedAlgoOrder{
		AlgoID:      result.AlgoID,
		Symbol:      symbol,
		Side:        side,
		EntryPrice:  entryPrice,
		Size:        size,
		SlTriggerPx: stopPx,
	}
	m.mutex.Unlock()

	logger.Info("交易所侧止损单已下发",
		zap.String("symbol", symbol),
		zap.String("algo_id", result.AlgoID),
		zap.Float64("stop_px", stopPx),
	)

	return result.AlgoID, nil
}

// PlaceTakeProfit 单独下发止盈单
func (m *ExchangeAlgoOrderManager) PlaceTakeProfit(symbol string, side types.OrderSide, size, entryPrice, tpPx float64) (string, error) {
	exitSide := exitSideAlgo(side)

	algoOrder := &types.AlgoOrder{
		Symbol:      symbol,
		Side:        exitSide,
		OrdType:     types.AlgoOrderConditional,
		Size:        size,
		TpTriggerPx: tpPx,
		TpOrderPx:   -1,
		TdMode:      m.tdMode,
	}

	result, err := m.exchange.PlaceAlgoOrder(algoOrder)
	if err != nil {
		logger.Warn("下发交易所侧止盈单失败",
			zap.String("symbol", symbol),
			zap.Float64("tp_px", tpPx),
			zap.Error(err),
		)
		return "", err
	}

	m.mutex.Lock()
	m.orders[result.AlgoID] = &trackedAlgoOrder{
		AlgoID:      result.AlgoID,
		Symbol:      symbol,
		Side:        side,
		EntryPrice:  entryPrice,
		Size:        size,
		TpTriggerPx: tpPx,
	}
	m.mutex.Unlock()

	logger.Info("交易所侧止盈单已下发",
		zap.String("symbol", symbol),
		zap.String("algo_id", result.AlgoID),
		zap.Float64("tp_px", tpPx),
	)

	return result.AlgoID, nil
}

// CancelAlgoOrder 撤销指定算法单
func (m *ExchangeAlgoOrderManager) CancelAlgoOrder(algoID, symbol string) error {
	err := m.exchange.CancelAlgoOrder(algoID, symbol)
	if err != nil {
		return err
	}

	m.mutex.Lock()
	delete(m.orders, algoID)
	m.mutex.Unlock()

	logger.Info("算法单已撤销",
		zap.String("algo_id", algoID),
		zap.String("symbol", symbol),
	)

	return nil
}

// CancelAllForSymbol 撤销指定交易对的所有算法单
func (m *ExchangeAlgoOrderManager) CancelAllForSymbol(symbol string) error {
	m.mutex.RLock()
	algos := make([]*trackedAlgoOrder, 0, len(m.orders))
	for _, order := range m.orders {
		if order.Symbol == symbol {
			algos = append(algos, order)
		}
	}
	m.mutex.RUnlock()

	var firstErr error
	for _, order := range algos {
		if err := m.CancelAlgoOrder(order.AlgoID, order.Symbol); err != nil {
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

// CancelAll 撤销所有算法单
func (m *ExchangeAlgoOrderManager) CancelAll() error {
	m.mutex.RLock()
	algos := make([]*trackedAlgoOrder, 0, len(m.orders))
	for _, order := range m.orders {
		algos = append(algos, order)
	}
	m.mutex.RUnlock()

	var firstErr error
	for _, order := range algos {
		if err := m.CancelAlgoOrder(order.AlgoID, order.Symbol); err != nil {
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

// SyncAlgoOrders 同步算法单状态（从交易所拉取待挂单，移除已触发的本地记录）
func (m *ExchangeAlgoOrderManager) SyncAlgoOrders(symbol string) error {
	algos, err := m.exchange.GetAlgoOrders(symbol, "conditional")
	if err != nil {
		return err
	}

	// 构建交易所侧活跃 AlgoID 集合
	activeIDs := make(map[string]bool, len(algos))
	for _, algo := range algos {
		activeIDs[algo.AlgoID] = true
	}

	// 清理本地已触发/已取消的记录
	m.mutex.Lock()
	for algoID, tracked := range m.orders {
		if !activeIDs[algoID] && tracked.Symbol == symbol {
			logger.Debug("算法单已结束（从本地清理）",
				zap.String("algo_id", algoID),
				zap.String("symbol", tracked.Symbol),
			)
			delete(m.orders, algoID)
		}
	}
	m.mutex.Unlock()

	return nil
}

// GetActiveCount 获取活跃算法单数量
func (m *ExchangeAlgoOrderManager) GetActiveCount() int {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return len(m.orders)
}

// CalculateStopLoss 计算止损价格
func CalculateStopLoss(side types.OrderSide, entryPrice, percent float64) float64 {
	if side == types.OrderSideBuy {
		return entryPrice * (1 - percent)
	}
	return entryPrice * (1 + percent)
}

// CalculateTakeProfit 计算止盈价格
func CalculateTakeProfit(side types.OrderSide, entryPrice, percent float64) float64 {
	if side == types.OrderSideBuy {
		return entryPrice * (1 + percent)
	}
	return entryPrice * (1 - percent)
}

// CalculateStopLossATR 基于ATR计算止损价格
func CalculateStopLossATR(side types.OrderSide, entryPrice, atr, multiplier float64) float64 {
	stopDistance := atr * multiplier
	if side == types.OrderSideBuy {
		return math.Max(entryPrice-stopDistance, entryPrice*0.9)
	}
	return math.Min(entryPrice+stopDistance, entryPrice*1.1)
}

func exitSideAlgo(side types.OrderSide) types.OrderSide {
	if side == types.OrderSideBuy {
		return types.OrderSideSell
	}
	return types.OrderSideBuy
}

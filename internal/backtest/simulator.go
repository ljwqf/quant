package backtest

import (
	"fmt"
	"time"

	"github.com/ljwqf/quant/pkg/types"
)

// Position 模拟持仓
type Position struct {
	Symbol     string
	Side       types.OrderSide
	Quantity   float64
	EntryPrice float64
	EntryTime  time.Time
}

// Simulator 交易模拟器
type Simulator struct {
	balance    float64
	positions  map[string]*Position
	trades     []Trade
	lastBar    *types.Bar
}

// NewSimulator 创建交易模拟器
func NewSimulator(initialBalance float64) *Simulator {
	return &Simulator{
		balance:   initialBalance,
		positions: make(map[string]*Position),
		trades:    make([]Trade, 0),
	}
}

// GetEquity 获取当前权益
func (s *Simulator) GetEquity() float64 {
	equity := s.balance

	// 计算未实现盈亏
	for _, pos := range s.positions {
		if s.lastBar != nil && pos.Symbol == s.lastBar.Symbol {
			var pnl float64
			if pos.Side == types.OrderSideBuy {
				pnl = (s.lastBar.Close - pos.EntryPrice) * pos.Quantity
			} else {
				pnl = (pos.EntryPrice - s.lastBar.Close) * pos.Quantity
			}
			equity += pnl
		}
	}

	return equity
}

// ExecuteTrade 执行交易
func (s *Simulator) ExecuteTrade(signal *types.Signal, bar *types.Bar) (*Trade, error) {
	// 保存最后一根K线
	s.lastBar = bar

	// 检查是否已有持仓
	existingPos, exists := s.positions[signal.Symbol]

	// 确定信号方向
	var signalSide types.OrderSide
	if signal.Type == types.SignalTypeBuy {
		signalSide = types.OrderSideBuy
	} else if signal.Type == types.SignalTypeSell {
		signalSide = types.OrderSideSell
	} else {
		return nil, nil
	}

	// 如果是相同方向的信号，忽略
	if exists && existingPos.Side == signalSide {
		return nil, nil
	}

	// 计算交易数量
	quantity := signal.Quantity
	if quantity <= 0 {
		// 默认使用固定数量
		quantity = 1
	}

	// 平仓现有持仓
	var closedTrade *Trade
	if exists {
		var err error
		closedTrade, err = s.closePosition(existingPos, bar)
		if err != nil {
			return nil, err
		}
	}

	// 开新仓
	if signal.Type == types.SignalTypeBuy || signal.Type == types.SignalTypeSell {
		s.openPosition(signal, signalSide, bar, quantity)
	}

	return closedTrade, nil
}

// openPosition 开仓
func (s *Simulator) openPosition(signal *types.Signal, side types.OrderSide, bar *types.Bar, quantity float64) {
	position := &Position{
		Symbol:     signal.Symbol,
		Side:       side,
		Quantity:   quantity,
		EntryPrice: bar.Close,
		EntryTime:  bar.Timestamp,
	}

	s.positions[signal.Symbol] = position
}

// closePosition 平仓
func (s *Simulator) closePosition(pos *Position, bar *types.Bar) (*Trade, error) {
	// 计算盈亏
	var pnl float64
	if pos.Side == types.OrderSideBuy {
		pnl = (bar.Close - pos.EntryPrice) * pos.Quantity
	} else {
		pnl = (pos.EntryPrice - bar.Close) * pos.Quantity
	}

	// 更新余额
	s.balance += pnl

	// 计算盈亏百分比
	pnlPercent := pnl / (pos.EntryPrice * pos.Quantity)

	// 创建交易记录
	trade := &Trade{
		ID:            fmt.Sprintf("trade_%d", len(s.trades)+1),
		Symbol:        pos.Symbol,
		Side:          pos.Side,
		EntryPrice:    pos.EntryPrice,
		ExitPrice:     bar.Close,
		Quantity:      pos.Quantity,
		PnL:           pnl,
		PnLPercent:    pnlPercent,
		EntryTime:     pos.EntryTime,
		ExitTime:      bar.Timestamp,
		HoldingPeriod: bar.Timestamp.Sub(pos.EntryTime),
	}

	// 添加到交易记录
	s.trades = append(s.trades, *trade)

	// 移除持仓
	delete(s.positions, pos.Symbol)

	return trade, nil
}

// CloseAllPositions 平仓所有持仓
func (s *Simulator) CloseAllPositions(bar *types.Bar) ([]Trade, error) {
	trades := make([]Trade, 0)

	// 复制持仓列表，避免遍历过程中修改
	positions := make(map[string]*Position)
	for k, v := range s.positions {
		positions[k] = v
	}

	// 平仓所有持仓
	for _, pos := range positions {
		trade, err := s.closePosition(pos, bar)
		if err != nil {
			continue
		}
		trades = append(trades, *trade)
	}

	return trades, nil
}

// GetPositions 获取当前持仓
func (s *Simulator) GetPositions() map[string]*Position {
	return s.positions
}

// GetBalance 获取当前余额
func (s *Simulator) GetBalance() float64 {
	return s.balance
}

// GetTrades 获取所有交易记录
func (s *Simulator) GetTrades() []Trade {
	return s.trades
}

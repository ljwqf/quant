package execution

import (
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/ljwqf/quant/pkg/logger"
	"github.com/ljwqf/quant/pkg/types"
	"go.uber.org/zap"
)

type TakeProfitType string

const (
	TakeProfitTypeFixed    TakeProfitType = "fixed"
	TakeProfitTypeTrailing TakeProfitType = "trailing"
	TakeProfitTypeTiered   TakeProfitType = "tiered"
	TakeProfitTypeATR      TakeProfitType = "atr"
)

type TakeProfitConfig struct {
	Enabled              bool             `json:"enabled"`
	Type                 TakeProfitType   `json:"type"`
	FixedProfitPercent   float64          `json:"fixed_profit_percent"`
	TrailingActivation   float64          `json:"trailing_activation"`
	TrailingDistance     float64          `json:"trailing_distance"`
	TrailingStep         float64          `json:"trailing_step"`
	TieredLevels         []TieredLevel    `json:"tiered_levels"`
	ATRPeriod            int              `json:"atr_period"`
	ATRMultiplier        float64          `json:"atr_multiplier"`
	MaxHoldingTime       time.Duration    `json:"max_holding_time"`
	PullbackPercent      float64          `json:"pullback_percent"`
}

type TieredLevel struct {
	ProfitPercent float64 `json:"profit_percent"`
	ClosePercent  float64 `json:"close_percent"`
}

type PositionState struct {
	Symbol           string
	Side             types.OrderSide
	EntryPrice       float64
	CurrentPrice     float64
	Size             float64
	HighestPrice     float64
	LowestPrice      float64
	UnrealizedPnL    float64
	UnrealizedPnLPercent float64
	OpenTime         time.Time
	LastUpdateTime   time.Time
	ATR              float64
	TakeProfitOrders map[string]*TakeProfitOrder
	ClosedPercent    float64
}

type TakeProfitOrder struct {
	ID             string
	Symbol         string
	Type           TakeProfitType
	TriggerPrice   float64
	Quantity       float64
	Status         string
	CreatedAt      time.Time
	TriggeredAt    time.Time
	TierLevel      int
}

type TakeProfitManager struct {
	config         *TakeProfitConfig
	positions      map[string]*PositionState
	mutex          sync.RWMutex
	atrCalculator  *ATRCalculator
}

type ATRCalculator struct {
	period     int
	barHistory map[string][]*types.Bar
	mutex      sync.RWMutex
}

func NewATRCalculator(period int) *ATRCalculator {
	return &ATRCalculator{
		period:     period,
		barHistory: make(map[string][]*types.Bar),
	}
}

func (a *ATRCalculator) Update(bar *types.Bar) {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	symbol := bar.Symbol
	history := a.barHistory[symbol]
	history = append(history, bar)

	if len(history) > a.period*2 {
		history = history[len(history)-a.period*2:]
	}
	a.barHistory[symbol] = history
}

func (a *ATRCalculator) Calculate(symbol string) float64 {
	a.mutex.RLock()
	defer a.mutex.RUnlock()

	history := a.barHistory[symbol]
	if len(history) < a.period {
		return 0
	}

	var trSum float64
	count := 0

	for i := len(history) - 1; i >= 1 && count < a.period; i-- {
		current := history[i]
		previous := history[i-1]

		tr := math.Max(
			current.High-current.Low,
			math.Max(
				math.Abs(current.High-previous.Close),
				math.Abs(current.Low-previous.Close),
			),
		)
		trSum += tr
		count++
	}

	if count == 0 {
		return 0
	}

	return trSum / float64(count)
}

func NewTakeProfitManager(config *TakeProfitConfig) *TakeProfitManager {
	if config == nil {
		config = &TakeProfitConfig{
			Enabled:            true,
			Type:               TakeProfitTypeTrailing,
			TrailingActivation: 1.0,
			TrailingDistance:   0.5,
			TrailingStep:       0.3,
			ATRPeriod:          14,
			ATRMultiplier:      2.0,
			MaxHoldingTime:     2 * time.Hour,
			PullbackPercent:    0.3,
			TieredLevels: []TieredLevel{
				{ProfitPercent: 1.5, ClosePercent: 30},
				{ProfitPercent: 3.0, ClosePercent: 40},
				{ProfitPercent: 5.0, ClosePercent: 30},
			},
		}
	}

	return &TakeProfitManager{
		config:        config,
		positions:     make(map[string]*PositionState),
		atrCalculator: NewATRCalculator(config.ATRPeriod),
	}
}

func (m *TakeProfitManager) AddPosition(symbol string, side types.OrderSide, entryPrice, size float64) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	state := &PositionState{
		Symbol:           symbol,
		Side:             side,
		EntryPrice:       entryPrice,
		CurrentPrice:     entryPrice,
		Size:             size,
		HighestPrice:     entryPrice,
		LowestPrice:      entryPrice,
		OpenTime:         time.Now(),
		LastUpdateTime:   time.Now(),
		TakeProfitOrders: make(map[string]*TakeProfitOrder),
		ClosedPercent:    0,
	}

	m.positions[symbol] = state

	logger.Info("添加持仓到止盈管理器",
		zap.String("symbol", symbol),
		zap.String("side", string(side)),
		zap.Float64("entry_price", entryPrice),
		zap.Float64("size", size),
	)
}

func (m *TakeProfitManager) RemovePosition(symbol string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	delete(m.positions, symbol)

	logger.Info("从止盈管理器移除持仓",
		zap.String("symbol", symbol),
	)
}

func (m *TakeProfitManager) UpdatePrice(symbol string, currentPrice float64) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	state, exists := m.positions[symbol]
	if !exists {
		return
	}

	state.CurrentPrice = currentPrice
	state.LastUpdateTime = time.Now()

	if state.Side == types.OrderSideBuy {
		if currentPrice > state.HighestPrice {
			state.HighestPrice = currentPrice
		}
		if currentPrice < state.LowestPrice {
			state.LowestPrice = currentPrice
		}
		state.UnrealizedPnL = (currentPrice - state.EntryPrice) * state.Size
		state.UnrealizedPnLPercent = (currentPrice - state.EntryPrice) / state.EntryPrice * 100
	} else {
		if currentPrice > state.HighestPrice {
			state.HighestPrice = currentPrice
		}
		if currentPrice < state.LowestPrice {
			state.LowestPrice = currentPrice
		}
		state.UnrealizedPnL = (state.EntryPrice - currentPrice) * state.Size
		state.UnrealizedPnLPercent = (state.EntryPrice - currentPrice) / state.EntryPrice * 100
	}
}

func (m *TakeProfitManager) UpdateATR(symbol string, bar *types.Bar) {
	m.atrCalculator.Update(bar)
	atr := m.atrCalculator.Calculate(symbol)

	m.mutex.Lock()
	defer m.mutex.Unlock()

	if state, exists := m.positions[symbol]; exists {
		state.ATR = atr
	}
}

func (m *TakeProfitManager) CheckTakeProfit(symbol string) *TakeProfitSignal {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	state, exists := m.positions[symbol]
	if !exists {
		return nil
	}

	if !m.config.Enabled {
		return nil
	}

	switch m.config.Type {
	case TakeProfitTypeFixed:
		return m.checkFixedTakeProfit(state)
	case TakeProfitTypeTrailing:
		return m.checkTrailingTakeProfit(state)
	case TakeProfitTypeTiered:
		return m.checkTieredTakeProfit(state)
	case TakeProfitTypeATR:
		return m.checkATRTakeProfit(state)
	default:
		return m.checkTrailingTakeProfit(state)
	}
}

func (m *TakeProfitManager) checkFixedTakeProfit(state *PositionState) *TakeProfitSignal {
	profitPercent := state.UnrealizedPnLPercent

	if profitPercent >= m.config.FixedProfitPercent {
		return &TakeProfitSignal{
			Symbol:      state.Symbol,
			Side:        getExitSide(state.Side),
			Quantity:    state.Size * (1 - state.ClosedPercent/100),
			TriggerType: "fixed_take_profit",
			Reason:      "达到固定止盈目标",
			ProfitPercent: profitPercent,
		}
	}

	return nil
}

func (m *TakeProfitManager) checkTrailingTakeProfit(state *PositionState) *TakeProfitSignal {
	profitPercent := state.UnrealizedPnLPercent

	if profitPercent < m.config.TrailingActivation {
		return nil
	}

	var triggerPrice float64
	var pullback float64

	if state.Side == types.OrderSideBuy {
		triggerPrice = state.HighestPrice * (1 - m.config.TrailingDistance/100)
		pullback = (state.HighestPrice - state.CurrentPrice) / state.HighestPrice * 100
	} else {
		triggerPrice = state.LowestPrice * (1 + m.config.TrailingDistance/100)
		pullback = (state.CurrentPrice - state.LowestPrice) / state.LowestPrice * 100
	}

	if pullback >= m.config.PullbackPercent {
		return &TakeProfitSignal{
			Symbol:        state.Symbol,
			Side:          getExitSide(state.Side),
			Quantity:      state.Size * (1 - state.ClosedPercent/100),
			TriggerType:   "trailing_take_profit",
			Reason:        "追踪止盈触发",
			TriggerPrice:  triggerPrice,
			ProfitPercent: profitPercent,
			HighestPrice:  state.HighestPrice,
			LowestPrice:   state.LowestPrice,
		}
	}

	holdingTime := time.Since(state.OpenTime)
	if m.config.MaxHoldingTime > 0 && holdingTime > m.config.MaxHoldingTime {
		return &TakeProfitSignal{
			Symbol:        state.Symbol,
			Side:          getExitSide(state.Side),
			Quantity:      state.Size * (1 - state.ClosedPercent/100),
			TriggerType:   "time_based_exit",
			Reason:        "持仓时间超限",
			ProfitPercent: profitPercent,
			HoldingTime:   holdingTime,
		}
	}

	return nil
}

func (m *TakeProfitManager) checkTieredTakeProfit(state *PositionState) *TakeProfitSignal {
	profitPercent := state.UnrealizedPnLPercent

	for i, level := range m.config.TieredLevels {
		orderID := generateTieredOrderID(state.Symbol, i)
		
		if _, exists := state.TakeProfitOrders[orderID]; exists {
			continue
		}

		if profitPercent >= level.ProfitPercent {
			// Close a fixed fraction of the ORIGINAL position size for this tier
			quantity := state.Size * (level.ClosePercent / 100)

			if quantity <= 0 {
				continue
			}

			state.TakeProfitOrders[orderID] = &TakeProfitOrder{
				ID:           orderID,
				Symbol:       state.Symbol,
				Type:         TakeProfitTypeTiered,
				TriggerPrice: state.CurrentPrice,
				Quantity:     quantity,
				Status:       "triggered",
				TriggeredAt:  time.Now(),
				TierLevel:    i,
			}

			// Track cumulative closed percentage (sum of original-tier percentages)
			state.ClosedPercent += level.ClosePercent

			return &TakeProfitSignal{
				Symbol:        state.Symbol,
				Side:          getExitSide(state.Side),
				Quantity:      quantity,
				TriggerType:   "tiered_take_profit",
				Reason:        "分级止盈触发",
				TierLevel:     i + 1,
				ProfitPercent: profitPercent,
			}
		}
	}

	return nil
}

func (m *TakeProfitManager) checkATRTakeProfit(state *PositionState) *TakeProfitSignal {
	if state.ATR <= 0 {
		return nil
	}

	profitPercent := state.UnrealizedPnLPercent
	atrDistance := state.ATR * m.config.ATRMultiplier

	var triggerPrice float64
	var currentDistance float64

	if state.Side == types.OrderSideBuy {
		triggerPrice = state.HighestPrice - atrDistance
		currentDistance = state.CurrentPrice - triggerPrice

		if state.CurrentPrice <= triggerPrice && profitPercent > 0 {
			return &TakeProfitSignal{
				Symbol:        state.Symbol,
				Side:          getExitSide(state.Side),
				Quantity:      state.Size * (1 - state.ClosedPercent/100),
				TriggerType:   "atr_take_profit",
				Reason:        "ATR止盈触发",
				TriggerPrice:  triggerPrice,
				ProfitPercent: profitPercent,
				ATRValue:      state.ATR,
			}
		}
	} else {
		triggerPrice = state.LowestPrice + atrDistance
		currentDistance = triggerPrice - state.CurrentPrice

		if state.CurrentPrice >= triggerPrice && profitPercent > 0 {
			return &TakeProfitSignal{
				Symbol:        state.Symbol,
				Side:          getExitSide(state.Side),
				Quantity:      state.Size * (1 - state.ClosedPercent/100),
				TriggerType:   "atr_take_profit",
				Reason:        "ATR止盈触发",
				TriggerPrice:  triggerPrice,
				ProfitPercent: profitPercent,
				ATRValue:      state.ATR,
			}
		}
	}

	_ = currentDistance

	return nil
}

func (m *TakeProfitManager) GetPositionState(symbol string) *PositionState {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	return m.positions[symbol]
}

func (m *TakeProfitManager) GetAllPositions() map[string]*PositionState {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	result := make(map[string]*PositionState)
	for k, v := range m.positions {
		result[k] = v
	}
	return result
}

func (m *TakeProfitManager) GetConfig() *TakeProfitConfig {
	return m.config
}

func (m *TakeProfitManager) SetConfig(config *TakeProfitConfig) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.config = config
}

type TakeProfitSignal struct {
	Symbol        string
	Side          types.OrderSide
	Quantity      float64
	TriggerType   string
	Reason        string
	TriggerPrice  float64
	ProfitPercent float64
	HighestPrice  float64
	LowestPrice   float64
	TierLevel     int
	HoldingTime   time.Duration
	ATRValue      float64
}

func getExitSide(side types.OrderSide) types.OrderSide {
	if side == types.OrderSideBuy {
		return types.OrderSideSell
	}
	return types.OrderSideBuy
}

func generateTieredOrderID(symbol string, level int) string {
	return fmt.Sprintf("%s_tier_%d", symbol, level)
}

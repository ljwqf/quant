package execution

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ljwqf/quant/internal/config"
	"github.com/ljwqf/quant/internal/risk"
	"github.com/ljwqf/quant/internal/strategy"
	"github.com/ljwqf/quant/pkg/types"
)

type recordingStrategy struct {
	filledCalls int
	closedCalls int
	lastPnL     float64
	lastExit    float64
}

func (s *recordingStrategy) Name() string { return "NeedleStrategy" }

func (s *recordingStrategy) Init(params map[string]interface{}) error { return nil }

func (s *recordingStrategy) OnTick(tick *types.Tick) (*types.Signal, error) { return nil, nil }

func (s *recordingStrategy) OnBar(bar *types.Bar) (*types.Signal, error) { return nil, nil }

func (s *recordingStrategy) OnOrderBook(orderBook *types.OrderBook) (*types.Signal, error) {
	return nil, nil
}

func (s *recordingStrategy) GetParams() map[string]interface{} { return nil }

func (s *recordingStrategy) SetParams(params map[string]interface{}) {}

func (s *recordingStrategy) GetMetrics() map[string]interface{} { return nil }

func (s *recordingStrategy) OnPositionFilled(symbol string, side types.OrderSide, entryPrice, size float64) {
	s.filledCalls++
}

func (s *recordingStrategy) OnPositionReduced(symbol string, exitPrice, pnl, remainingSize float64) {
	s.closedCalls++
	s.lastExit = exitPrice
	s.lastPnL = pnl
}

func (s *recordingStrategy) OnPositionClosed(symbol string, exitPrice, pnl float64) {
	s.closedCalls++
	s.lastExit = exitPrice
	s.lastPnL = pnl
}

func (s *recordingStrategy) ConfirmRebalanceEntry(request *strategy.RebalanceRequest) (*strategy.RebalanceDecision, error) {
	return &strategy.RebalanceDecision{RejectReason: "test_stub"}, nil
}

func TestExecuteUsesCalculatedQuantityForRiskCheck(t *testing.T) {
	exchange := &stubExchange{}
	riskEngine := risk.NewEngine(&config.RiskConfig{
		Enable:            true,
		MaxPositionSize:   5, // 降低仓位限制，确保触发 ErrPositionLimitExceeded
		MaxDailyLoss:      1000,
		MaxDrawdown:       0.2,
		StopLossPercent:   0.05,
		TakeProfitPercent: 0.1,
		MaxTradesPerDay:   100,
	})
	engine := NewEngine(exchange, riskEngine, strategy.NewEngine())

	result, err := engine.Execute(&types.Signal{
		Strategy: "NeedleStrategy",
		Symbol:   "BTC-USDT",
		Type:     types.SignalTypeBuy,
		Price:    10,
	}, 10000)

	assert.Nil(t, result)
	assert.ErrorIs(t, err, risk.ErrPositionLimitExceeded)
	assert.Empty(t, exchange.placedOrders)
}

func TestCloseAllPositionsUsesOppositeSideForShortPosition(t *testing.T) {
	exchange := &stubExchange{
		positions: []*types.Position{{
			Symbol:    "BTC-USDT",
			Side:      types.OrderSideSell,
			Size:      2,
			Leverage:  3,
			MarkPrice: 100,
		}},
	}
	engine := NewEngine(exchange, risk.NewEngine(&config.RiskConfig{
		Enable:            true,
		MaxPositionSize:   1000,
		MaxDailyLoss:      1000,
		MaxDrawdown:       0.2,
		StopLossPercent:   0.05,
		TakeProfitPercent: 0.1,
		MaxTradesPerDay:   100,
	}), strategy.NewEngine())

	err := engine.CloseAllPositions()

	require.NoError(t, err)
	require.Len(t, exchange.placedOrders, 1)
	assert.Equal(t, types.OrderSideBuy, exchange.placedOrders[0].Side)
	assert.Equal(t, 2.0, exchange.placedOrders[0].Quantity)
}

func TestPlaceOrderRejectsInvalidManualOrder(t *testing.T) {
	engine := NewEngine(&stubExchange{}, risk.NewEngine(&config.RiskConfig{
		Enable:            true,
		MaxPositionSize:   1000,
		MaxDailyLoss:      1000,
		MaxDrawdown:       0.2,
		StopLossPercent:   0.05,
		TakeProfitPercent: 0.1,
		MaxTradesPerDay:   100,
	}), strategy.NewEngine())

	result, err := engine.PlaceOrder(&types.Order{Symbol: "BTC-USDT", Type: types.OrderTypeLimit, Quantity: 1})

	assert.Nil(t, result)
	assert.ErrorIs(t, err, risk.ErrInvalidSignal)
}

func TestPlaceOrderTracksManualOrder(t *testing.T) {
	exchange := &stubExchange{}
	engine := NewEngine(exchange, risk.NewEngine(&config.RiskConfig{
		Enable:            true,
		MaxPositionSize:   1000,
		MaxDailyLoss:      1000,
		MaxDrawdown:       0.2,
		StopLossPercent:   0.05,
		TakeProfitPercent: 0.1,
		MaxTradesPerDay:   100,
	}), strategy.NewEngine())

	result, err := engine.PlaceOrder(&types.Order{Symbol: "BTC-USDT", Side: types.OrderSideBuy, Type: types.OrderTypeMarket, Quantity: 1})

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, exchange.placedOrders, 1)
	tracked := engine.GetOrder(result.OrderID)
	require.NotNil(t, tracked)
	assert.Equal(t, "manual", tracked.Metadata["source"])
}

func TestClosePositionUsesOppositeSideForShortPosition(t *testing.T) {
	exchange := &stubExchange{
		positions: []*types.Position{{
			Symbol:    "BTC-USDT",
			Side:      types.OrderSideSell,
			Size:      3,
			Leverage:  2,
			MarkPrice: 100,
		}},
	}
	engine := NewEngine(exchange, risk.NewEngine(&config.RiskConfig{
		Enable:            true,
		MaxPositionSize:   1000,
		MaxDailyLoss:      1000,
		MaxDrawdown:       0.2,
		StopLossPercent:   0.05,
		TakeProfitPercent: 0.1,
		MaxTradesPerDay:   100,
	}), strategy.NewEngine())

	result, err := engine.ClosePosition("BTC-USDT", 0)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, exchange.placedOrders, 1)
	assert.Equal(t, types.OrderSideBuy, exchange.placedOrders[0].Side)
	assert.Equal(t, 3.0, exchange.placedOrders[0].Quantity)
	tracked := engine.GetOrder(result.OrderID)
	require.NotNil(t, tracked)
	assert.Equal(t, "manual_close", tracked.Metadata["source"])
}

func TestClosePositionReturnsNilWhenPositionMissing(t *testing.T) {
	engine := NewEngine(&stubExchange{}, risk.NewEngine(&config.RiskConfig{
		Enable:            true,
		MaxPositionSize:   1000,
		MaxDailyLoss:      1000,
		MaxDrawdown:       0.2,
		StopLossPercent:   0.05,
		TakeProfitPercent: 0.1,
		MaxTradesPerDay:   100,
	}), strategy.NewEngine())

	result, err := engine.ClosePosition("BTC-USDT", 0)

	assert.NoError(t, err)
	assert.Nil(t, result)
}

func TestHandleOrderFilledNotifiesStrategyOnExit(t *testing.T) {
	exchange := &stubExchange{}
	riskEngine := risk.NewEngine(&config.RiskConfig{
		Enable:            true,
		MaxPositionSize:   1000,
		MaxDailyLoss:      1000,
		MaxDrawdown:       0.2,
		StopLossPercent:   0.05,
		TakeProfitPercent: 0.1,
		MaxTradesPerDay:   100,
	})
	strategyEngine := strategy.NewEngine()
	recorder := &recordingStrategy{}
	require.NoError(t, strategyEngine.AddStrategy("NeedleStrategy", recorder, map[string]interface{}{}))

	engine := NewEngine(exchange, riskEngine, strategyEngine)
	riskEngine.UpdatePosition(&types.Position{
		Symbol:     "BTC-USDT",
		Side:       types.OrderSideBuy,
		Size:       2,
		EntryPrice: 100,
		MarkPrice:  100,
	})

	engine.handleOrderFilled("order-1", &types.Order{
		Symbol: "BTC-USDT",
		Metadata: map[string]interface{}{
			"strategy": "NeedleStrategy",
			"is_exit":  true,
		},
	}, &types.Order{
		Symbol:       "BTC-USDT",
		Side:         types.OrderSideSell,
		AveragePrice: 110,
		FilledQty:    2,
		Status:       types.OrderStatusFilled,
	})

	assert.Equal(t, 1, recorder.closedCalls)
	assert.Equal(t, 20.0, recorder.lastPnL)
	assert.Equal(t, 110.0, recorder.lastExit)
	assert.Nil(t, riskEngine.GetPosition("BTC-USDT"))
	assert.Equal(t, 0.0, riskEngine.GetDailyLoss())
}

func TestHandleOrderFilledUpdatesDailyLossOnLosingExit(t *testing.T) {
	exchange := &stubExchange{}
	riskEngine := risk.NewEngine(&config.RiskConfig{
		Enable:            true,
		MaxPositionSize:   1000,
		MaxDailyLoss:      1000,
		MaxDrawdown:       0.2,
		StopLossPercent:   0.05,
		TakeProfitPercent: 0.1,
		MaxTradesPerDay:   100,
	})
	strategyEngine := strategy.NewEngine()
	recorder := &recordingStrategy{}
	require.NoError(t, strategyEngine.AddStrategy("NeedleStrategy", recorder, map[string]interface{}{}))

	engine := NewEngine(exchange, riskEngine, strategyEngine)
	riskEngine.UpdatePosition(&types.Position{
		Symbol:     "BTC-USDT",
		Side:       types.OrderSideBuy,
		Size:       1.5,
		EntryPrice: 100,
		MarkPrice:  100,
	})

	engine.handleOrderFilled("order-2", &types.Order{
		Symbol: "BTC-USDT",
		Metadata: map[string]interface{}{
			"strategy": "NeedleStrategy",
			"is_exit":  true,
		},
	}, &types.Order{
		Symbol:       "BTC-USDT",
		Side:         types.OrderSideSell,
		AveragePrice: 90,
		FilledQty:    1.5,
		Status:       types.OrderStatusFilled,
	})

	assert.Equal(t, 1, recorder.closedCalls)
	assert.Equal(t, -15.0, recorder.lastPnL)
	assert.Equal(t, 15.0, riskEngine.GetDailyLoss())
}

func TestExecuteEntrySignalRejectsInsufficientOrderBookDepth(t *testing.T) {
	exchange := &stubExchange{
		orderBook: &types.OrderBook{
			Symbol: "BTC-USDT",
			Asks: []types.OrderBookLevel{
				{Price: 100, Size: 0.5},
				{Price: 101, Size: 0.5},
			},
		},
	}
	engine := NewEngine(exchange, risk.NewEngine(&config.RiskConfig{
		Enable:            true,
		MaxPositionSize:   1000,
		MaxDailyLoss:      1000,
		MaxDrawdown:       0.2,
		StopLossPercent:   0.05,
		TakeProfitPercent: 0.1,
		MaxTradesPerDay:   100,
	}), strategy.NewEngine())

	order, result, err := engine.executeEntrySignal(&types.Signal{
		Strategy: "NeedleStrategy",
		Symbol:   "BTC-USDT",
		Type:     types.SignalTypeBuy,
		Price:    100,
		Quantity: 2,
	}, 0)

	assert.Nil(t, order)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, risk.ErrLiquidityInsufficient)
	assert.Empty(t, exchange.placedOrders)
}

func TestExecuteEntrySignalStoresEstimatedSlippageMetadata(t *testing.T) {
	exchange := &stubExchange{
		orderBook: &types.OrderBook{
			Symbol: "BTC-USDT",
			Asks: []types.OrderBookLevel{
				{Price: 100, Size: 1},
				{Price: 101, Size: 1},
			},
		},
	}
	engine := NewEngine(exchange, risk.NewEngine(&config.RiskConfig{
		Enable:            true,
		MaxPositionSize:   1000,
		MaxDailyLoss:      1000,
		MaxDrawdown:       0.2,
		StopLossPercent:   0.05,
		TakeProfitPercent: 0.1,
		MaxTradesPerDay:   100,
	}), strategy.NewEngine())

	order, result, err := engine.executeEntrySignal(&types.Signal{
		Strategy: "NeedleStrategy",
		Symbol:   "BTC-USDT",
		Type:     types.SignalTypeBuy,
		Price:    100,
		Quantity: 1.5,
		Metadata: map[string]interface{}{"max_slippage": 0.01},
	}, 0)

	require.NoError(t, err)
	require.NotNil(t, order)
	require.NotNil(t, result)
	require.Len(t, exchange.placedOrders, 1)
	metadata := exchange.placedOrders[0].Metadata
	require.NotNil(t, metadata)
	assert.Equal(t, defaultOrderBookDepth, metadata["book_depth_checked"])
	assert.InDelta(t, 100.3333333, metadata["estimated_avg_price"], 0.0001)
	assert.InDelta(t, 0.0033333, metadata["estimated_slippage"], 0.0001)
}

func TestHandleOrderFilledRebalancesAllocatorAfterExitPnL(t *testing.T) {
	exchange := &stubExchange{}
	riskEngine := risk.NewEngine(&config.RiskConfig{
		Enable:            true,
		MaxPositionSize:   1000,
		MaxDailyLoss:      1000,
		MaxDrawdown:       0.2,
		StopLossPercent:   0.05,
		TakeProfitPercent: 0.1,
		MaxTradesPerDay:   100,
	})
	strategyEngine := strategy.NewEngine()
	recorder := &recordingStrategy{}
	require.NoError(t, strategyEngine.AddStrategy("loser", recorder, map[string]interface{}{}))

	engine := NewEngine(exchange, riskEngine, strategyEngine)
	allocator := engine.GetBayesianAllocator()
	require.NoError(t, allocator.Init(map[string]interface{}{
		"rebalance_interval":      time.Duration(0),
		"weight_change_threshold": 0.01,
		"min_weight":              0.05,
		"max_weight":              0.9,
	}))
	allocator.SetTotalCapital(1000)
	allocator.RegisterStrategy("winner", 0.5)
	allocator.RegisterStrategy("loser", 0.5)

	riskEngine.UpdatePosition(&types.Position{
		Symbol:     "BTC-USDT",
		Side:       types.OrderSideBuy,
		Size:       1,
		EntryPrice: 100,
		MarkPrice:  100,
	})

	engine.handleOrderFilled("order-3", &types.Order{
		Symbol: "BTC-USDT",
		Metadata: map[string]interface{}{
			"strategy": "loser",
			"is_exit":  true,
		},
	}, &types.Order{
		Symbol:       "BTC-USDT",
		Side:         types.OrderSideSell,
		AveragePrice: 90,
		FilledQty:    1,
		Status:       types.OrderStatusFilled,
	})

	loserPerf := allocator.GetStrategyPerformance("loser")
	winnerPerf := allocator.GetStrategyPerformance("winner")
	require.NotNil(t, loserPerf)
	require.NotNil(t, winnerPerf)
	assert.Equal(t, 1, loserPerf.TotalTrades)
	assert.Less(t, loserPerf.CurrentWeight, winnerPerf.CurrentWeight)
}

func TestStateSnapshotRoundTrip(t *testing.T) {
	store, err := NewFileStateStore(filepath.Join(t.TempDir(), "runtime"))
	require.NoError(t, err)

	riskEngine := risk.NewEngine(&config.RiskConfig{
		Enable:            true,
		MaxPositionSize:   1000,
		MaxDailyLoss:      1000,
		MaxDrawdown:       0.2,
		StopLossPercent:   0.05,
		TakeProfitPercent: 0.1,
		MaxTradesPerDay:   100,
	})
	strategyEngine := strategy.NewEngine()
	engine := NewEngine(&stubExchange{}, riskEngine, strategyEngine)
	engine.SetStateStore(store)

	engine.strategyPositions["NeedleStrategy"] = map[string]*strategyPosition{
		"BTC-USDT": {
			Strategy:   "NeedleStrategy",
			Symbol:     "BTC-USDT",
			Side:       types.OrderSideBuy,
			Size:       1.25,
			EntryPrice: 100,
			MarkPrice:  101,
			UpdatedAt:  time.Now(),
		},
	}
	engine.orders["persisted-order"] = &types.Order{
		ID:       "persisted-order",
		Symbol:   "BTC-USDT",
		Side:     types.OrderSideBuy,
		Type:     types.OrderTypeMarket,
		Quantity: 1.25,
		Metadata: map[string]interface{}{"strategy": "NeedleStrategy"},
	}
	riskEngine.UpdatePosition(&types.Position{
		Symbol:     "BTC-USDT",
		Side:       types.OrderSideBuy,
		Size:       1.25,
		EntryPrice: 100,
		MarkPrice:  101,
	})

	require.NoError(t, engine.SaveStateSnapshot())

	restoredRiskEngine := risk.NewEngine(&config.RiskConfig{
		Enable:            true,
		MaxPositionSize:   1000,
		MaxDailyLoss:      1000,
		MaxDrawdown:       0.2,
		StopLossPercent:   0.05,
		TakeProfitPercent: 0.1,
		MaxTradesPerDay:   100,
	})
	restoredEngine := NewEngine(&stubExchange{}, restoredRiskEngine, strategy.NewEngine())
	restoredEngine.SetStateStore(store)

	loaded, err := restoredEngine.LoadStateSnapshot()
	require.NoError(t, err)
	assert.True(t, loaded)

	position := restoredEngine.strategyPositions["NeedleStrategy"]["BTC-USDT"]
	require.NotNil(t, position)
	assert.Equal(t, 1.25, position.Size)
	assert.Equal(t, 100.0, position.EntryPrice)

	trackedOrder := restoredEngine.GetOrder("persisted-order")
	require.NotNil(t, trackedOrder)
	assert.Equal(t, "NeedleStrategy", trackedOrder.Metadata["strategy"])

	riskPosition := restoredRiskEngine.GetPosition("BTC-USDT")
	require.NotNil(t, riskPosition)
	assert.Equal(t, 1.25, riskPosition.Size)
}

func TestReconcileWithExchangeSyncsLivePositions(t *testing.T) {
	exchange := &stubExchange{
		positions: []*types.Position{
			{
				Symbol:     "BTC-USDT",
				Side:       types.OrderSideBuy,
				Size:       2,
				EntryPrice: 105,
				MarkPrice:  106,
			},
		},
	}
	riskEngine := risk.NewEngine(&config.RiskConfig{
		Enable:            true,
		MaxPositionSize:   1000,
		MaxDailyLoss:      1000,
		MaxDrawdown:       0.2,
		StopLossPercent:   0.05,
		TakeProfitPercent: 0.1,
		MaxTradesPerDay:   100,
	})
	engine := NewEngine(exchange, riskEngine, strategy.NewEngine())

	riskEngine.UpdatePosition(&types.Position{
		Symbol:     "ETH-USDT",
		Side:       types.OrderSideBuy,
		Size:       1,
		EntryPrice: 2000,
		MarkPrice:  2001,
	})
	engine.strategyPositions["NeedleStrategy"] = map[string]*strategyPosition{
		"BTC-USDT": {
			Strategy:   "NeedleStrategy",
			Symbol:     "BTC-USDT",
			Side:       types.OrderSideBuy,
			Size:       0.5,
			EntryPrice: 100,
			MarkPrice:  100,
			UpdatedAt:  time.Now().Add(-time.Minute),
		},
		"ETH-USDT": {
			Strategy:   "NeedleStrategy",
			Symbol:     "ETH-USDT",
			Side:       types.OrderSideBuy,
			Size:       1,
			EntryPrice: 2000,
			MarkPrice:  2001,
			UpdatedAt:  time.Now().Add(-time.Minute),
		},
	}

	require.NoError(t, engine.ReconcileWithExchange())

	riskPosition := riskEngine.GetPosition("BTC-USDT")
	require.NotNil(t, riskPosition)
	assert.Equal(t, 2.0, riskPosition.Size)
	assert.Nil(t, riskEngine.GetPosition("ETH-USDT"))

	btcPosition := engine.strategyPositions["NeedleStrategy"]["BTC-USDT"]
	require.NotNil(t, btcPosition)
	assert.Equal(t, 2.0, btcPosition.Size)
	assert.Equal(t, 105.0, btcPosition.EntryPrice)
	_, exists := engine.strategyPositions["NeedleStrategy"]["ETH-USDT"]
	assert.False(t, exists)
}

type stubExchange struct {
	placedOrders []*types.Order
	positions    []*types.Position
	orderBook    *types.OrderBook
}

func (s *stubExchange) Connect() error { return nil }

func (s *stubExchange) Disconnect() error { return nil }

func (s *stubExchange) GetAccount() (*types.Account, error) { return &types.Account{}, nil }

func (s *stubExchange) PlaceOrder(order *types.Order) (*types.OrderResult, error) {
	s.placedOrders = append(s.placedOrders, order)
	return &types.OrderResult{
		OrderID:   "order-1",
		Symbol:    order.Symbol,
		Side:      order.Side,
		Type:      order.Type,
		Quantity:  order.Quantity,
		Price:     order.Price,
		Status:    types.OrderStatusPending,
		Timestamp: time.Now(),
	}, nil
}

func (s *stubExchange) CancelOrder(orderID string) error { return nil }

func (s *stubExchange) GetOrder(orderID string) (*types.Order, error) { return nil, nil }

func (s *stubExchange) GetOrders(symbol string, limit int) ([]*types.Order, error) { return nil, nil }

func (s *stubExchange) GetPositions() ([]*types.Position, error) { return s.positions, nil }

func (s *stubExchange) SubscribeTicker(symbol string, handler func(*types.Tick)) error { return nil }

func (s *stubExchange) SubscribeBar(symbol string, interval string, handler func(*types.Bar)) error {
	return nil
}

func (s *stubExchange) SubscribeOrderBook(symbol string, handler func(*types.OrderBook)) error {
	return nil
}

func (s *stubExchange) GetBars(symbol string, interval string, limit int) ([]*types.Bar, error) {
	return nil, nil
}

func (s *stubExchange) GetTicker(symbol string) (*types.Tick, error) {
	return &types.Tick{Symbol: symbol, Price: 100}, nil
}

func (s *stubExchange) GetOrderBook(symbol string, depth int) (*types.OrderBook, error) {
	return s.orderBook, nil
}

func (s *stubExchange) SetLeverage(symbol string, leverage int, marginMode string) error {
	return nil
}
func (s *stubExchange) PlaceAlgoOrder(order *types.AlgoOrder) (*types.AlgoOrderResult, error) {
	return &types.AlgoOrderResult{AlgoID: "algo_" + order.Symbol, Symbol: order.Symbol, Status: "active"}, nil
}
func (s *stubExchange) CancelAlgoOrder(algoID, symbol string) error { return nil }
func (s *stubExchange) GetAlgoOrders(symbol string, orderType string) ([]*types.AlgoOrder, error) {
	return nil, nil
}
func (s *stubExchange) GetFundingRate(instId string) (*types.FundingRate, error) { return nil, nil }

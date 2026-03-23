package execution

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ljwqf/quant/internal/config"
	"github.com/ljwqf/quant/internal/risk"
	"github.com/ljwqf/quant/internal/strategy"
	"github.com/ljwqf/quant/pkg/types"
)

type flowStrategy struct {
	inPosition   bool
	filledCalls  int
	reducedCalls int
	closedCalls  int
	approveTopUp bool
	rejectReason string
}

type alertRecord struct {
	level   AlertLevel
	title   string
	message string
	labels  map[string]string
	details map[string]interface{}
}

func (s *flowStrategy) Name() string                             { return "FlowStrategy" }
func (s *flowStrategy) Init(params map[string]interface{}) error { return nil }
func (s *flowStrategy) OnTick(tick *types.Tick) (*types.Signal, error) {
	if !s.inPosition {
		return &types.Signal{Symbol: tick.Symbol, Type: types.SignalTypeBuy, Price: tick.Price, Timestamp: tick.Timestamp}, nil
	}
	return &types.Signal{Symbol: tick.Symbol, Type: types.SignalTypeExit, Price: tick.Price, Timestamp: tick.Timestamp}, nil
}
func (s *flowStrategy) OnBar(bar *types.Bar) (*types.Signal, error) { return nil, nil }
func (s *flowStrategy) OnOrderBook(orderBook *types.OrderBook) (*types.Signal, error) {
	return nil, nil
}
func (s *flowStrategy) GetParams() map[string]interface{}       { return nil }
func (s *flowStrategy) SetParams(params map[string]interface{}) {}
func (s *flowStrategy) GetMetrics() map[string]interface{}      { return nil }
func (s *flowStrategy) OnPositionFilled(symbol string, side types.OrderSide, entryPrice, size float64) {
	s.inPosition = true
	s.filledCalls++
}
func (s *flowStrategy) OnPositionReduced(symbol string, exitPrice, pnl, remainingSize float64) {
	s.reducedCalls++
	if remainingSize <= 0 {
		s.inPosition = false
	}
}
func (s *flowStrategy) OnPositionClosed(symbol string, exitPrice, pnl float64) {
	s.inPosition = false
	s.closedCalls++
}
func (s *flowStrategy) ConfirmRebalanceEntry(request *strategy.RebalanceRequest) (*strategy.RebalanceDecision, error) {
	if !s.approveTopUp {
		return &strategy.RebalanceDecision{RejectReason: s.rejectReason}, nil
	}
	return &strategy.RebalanceDecision{
		Approved:            true,
		RecommendedPrice:    49,
		RecommendedQuantity: 4,
		Signal: &types.Signal{
			Symbol:    "ETH-USDT",
			Type:      types.SignalTypeBuy,
			Price:     50,
			Quantity:  1,
			Timestamp: time.Now(),
		},
	}, nil
}

type flowExchangeStub struct {
	nextOrderID       int
	placedOrders      []*types.Order
	pending           map[string]*types.Order
	orderSnapshots    map[string][]*types.Order
	getOrderCalls     map[string]int
	appliedFilledQty  map[string]float64
	positions         map[string]*types.Position
	tickerPrices      map[string]float64
	orderBook         *types.OrderBook
	orderBooks        map[string]*types.OrderBook
	account           *types.Account
	failOnPlace       map[string]error
	failOnPlaceBySide map[string]error
	failCancelSymbols map[string]error
	cancelledIDs      []string
}

func newFlowExchangeStub() *flowExchangeStub {
	stub := &flowExchangeStub{
		nextOrderID:      1,
		pending:          make(map[string]*types.Order),
		orderSnapshots:   make(map[string][]*types.Order),
		getOrderCalls:    make(map[string]int),
		appliedFilledQty: make(map[string]float64),
		positions:        make(map[string]*types.Position),
		tickerPrices:     map[string]float64{"BTC-USDT": 100},
		orderBook: &types.OrderBook{
			Symbol: "BTC-USDT",
			Asks:   []types.OrderBookLevel{{Price: 100, Size: 10}, {Price: 101, Size: 10}},
			Bids:   []types.OrderBookLevel{{Price: 100, Size: 10}, {Price: 99, Size: 10}},
		},
		account: &types.Account{TotalAvailable: 1000, TotalEquity: 1000},
	}
	stub.orderBooks = map[string]*types.OrderBook{
		"BTC-USDT": stub.orderBook,
		"ETH-USDT": {
			Symbol: "ETH-USDT",
			Asks:   []types.OrderBookLevel{{Price: 50, Size: 20}, {Price: 51, Size: 20}},
			Bids:   []types.OrderBookLevel{{Price: 50, Size: 20}, {Price: 49, Size: 20}},
		},
	}
	stub.tickerPrices["ETH-USDT"] = 50
	return stub
}

func (s *flowExchangeStub) Connect() error                      { return nil }
func (s *flowExchangeStub) Disconnect() error                   { return nil }
func (s *flowExchangeStub) GetAccount() (*types.Account, error) { return s.account, nil }
func (s *flowExchangeStub) PlaceOrder(order *types.Order) (*types.OrderResult, error) {
	if err, ok := s.failOnPlaceBySide[order.Symbol+":"+string(order.Side)]; ok {
		return nil, err
	}
	if err, ok := s.failOnPlace[order.Symbol]; ok {
		return nil, err
	}
	orderCopy := *order
	orderID := fmt.Sprintf("order-%d", s.nextOrderID)
	s.nextOrderID++
	orderCopy.ID = orderID
	s.placedOrders = append(s.placedOrders, &orderCopy)
	s.pending[orderID] = &orderCopy
	return &types.OrderResult{OrderID: orderID, Symbol: order.Symbol, Side: order.Side, Type: order.Type, Quantity: order.Quantity, Price: order.Price, Status: types.OrderStatusPending, Timestamp: time.Now()}, nil
}
func (s *flowExchangeStub) CancelOrder(orderID string) error {
	if pending, exists := s.pending[orderID]; exists {
		if err, shouldFail := s.failCancelSymbols[pending.Symbol]; shouldFail {
			s.cancelledIDs = append(s.cancelledIDs, orderID)
			return err
		}
	}
	s.cancelledIDs = append(s.cancelledIDs, orderID)
	delete(s.pending, orderID)
	return nil
}
func (s *flowExchangeStub) GetOrder(orderID string) (*types.Order, error) {
	if snapshots, ok := s.orderSnapshots[orderID]; ok && len(snapshots) > 0 {
		index := s.getOrderCalls[orderID]
		if index >= len(snapshots) {
			index = len(snapshots) - 1
		}
		s.getOrderCalls[orderID]++
		snapshot := *snapshots[index]
		if pending, exists := s.pending[orderID]; exists {
			if snapshot.Symbol == "" {
				snapshot.Symbol = pending.Symbol
			}
			if snapshot.Side == "" {
				snapshot.Side = pending.Side
			}
			if snapshot.Type == "" {
				snapshot.Type = pending.Type
			}
			if snapshot.Quantity <= 0 {
				snapshot.Quantity = pending.Quantity
			}
			if snapshot.Price <= 0 {
				snapshot.Price = pending.Price
			}
		}
		snapshot.ID = orderID
		if snapshot.AveragePrice <= 0 {
			snapshot.AveragePrice = s.tickerPrices[snapshot.Symbol]
		}
		deltaFilled := snapshot.FilledQty - s.appliedFilledQty[orderID]
		if deltaFilled > 0 {
			s.applyFillDelta(&snapshot, deltaFilled)
			s.appliedFilledQty[orderID] = snapshot.FilledQty
		}
		if snapshot.Status == types.OrderStatusFilled || snapshot.Status == types.OrderStatusCancelled || snapshot.Status == types.OrderStatusFailed {
			delete(s.pending, orderID)
		}
		return &snapshot, nil
	}
	pending, exists := s.pending[orderID]
	if !exists {
		return nil, nil
	}
	filled := *pending
	filled.Status = types.OrderStatusFilled
	filled.FilledQty = pending.Quantity
	filled.AveragePrice = pending.Price
	if filled.AveragePrice <= 0 {
		filled.AveragePrice = s.tickerPrices[pending.Symbol]
	}
	s.applyFill(&filled)
	delete(s.pending, orderID)
	return &filled, nil
}
func (s *flowExchangeStub) GetOrders(symbol string, limit int) ([]*types.Order, error) {
	return nil, nil
}
func (s *flowExchangeStub) GetPositions() ([]*types.Position, error) {
	positions := make([]*types.Position, 0, len(s.positions))
	for _, position := range s.positions {
		copyPosition := *position
		positions = append(positions, &copyPosition)
	}
	return positions, nil
}
func (s *flowExchangeStub) SubscribeTicker(symbol string, handler func(*types.Tick)) error {
	return nil
}
func (s *flowExchangeStub) SubscribeBar(symbol string, interval string, handler func(*types.Bar)) error {
	return nil
}
func (s *flowExchangeStub) SubscribeOrderBook(symbol string, handler func(*types.OrderBook)) error {
	return nil
}
func (s *flowExchangeStub) GetBars(symbol string, interval string, limit int) ([]*types.Bar, error) {
	return nil, nil
}
func (s *flowExchangeStub) GetTicker(symbol string) (*types.Tick, error) {
	return &types.Tick{Symbol: symbol, Price: s.tickerPrices[symbol], Timestamp: time.Now()}, nil
}
func (s *flowExchangeStub) GetOrderBook(symbol string, depth int) (*types.OrderBook, error) {
	if orderBook, ok := s.orderBooks[symbol]; ok {
		return orderBook, nil
	}
	return s.orderBook, nil
}
func (s *flowExchangeStub) SetLeverage(symbol string, leverage int, marginMode string) error {
	return nil
}

func (s *flowExchangeStub) applyFill(order *types.Order) {
	existing := s.positions[order.Symbol]
	price := order.AveragePrice
	if price <= 0 {
		price = s.tickerPrices[order.Symbol]
	}
	if order.Side == types.OrderSideBuy {
		if existing == nil {
			s.positions[order.Symbol] = &types.Position{Symbol: order.Symbol, Side: types.OrderSideBuy, Size: order.Quantity, EntryPrice: price, MarkPrice: price, Timestamp: time.Now()}
			return
		}
		existing.Size += order.Quantity
		existing.MarkPrice = price
		return
	}
	if existing == nil {
		return
	}
	existing.Size -= order.Quantity
	if existing.Size <= 0 {
		delete(s.positions, order.Symbol)
		return
	}
	existing.MarkPrice = price
}

func (s *flowExchangeStub) applyFillDelta(order *types.Order, quantity float64) {
	copyOrder := *order
	copyOrder.Quantity = quantity
	copyOrder.FilledQty = quantity
	s.applyFill(&copyOrder)
}

func TestExecutionEngineCompletesEntryAndExitLifecycleWithStrategyEngine(t *testing.T) {
	exchange := newFlowExchangeStub()
	riskEngine := risk.NewEngine(&config.RiskConfig{Enable: true, MaxPositionSize: 10000, MaxDailyLoss: 1000, MaxDrawdown: 0.2, StopLossPercent: 0.05, TakeProfitPercent: 0.1, MaxTradesPerDay: 100})
	strategyEngine := strategy.NewEngine()
	flow := &flowStrategy{}
	require.NoError(t, strategyEngine.AddStrategy("FlowStrategy", flow, map[string]interface{}{}))

	engine := NewEngineWithConfig(exchange, riskEngine, strategyEngine, &EngineConfig{
		SmartRouteConfig: SmartRouteConfig{OrderBookDepth: 2, MaxEstimatedSlippage: 0.01},
	})
	allocator := engine.GetBayesianAllocator()
	require.NoError(t, allocator.Init(map[string]interface{}{"rebalance_interval": time.Duration(0), "weight_change_threshold": 0.01}))
	allocator.RegisterStrategy("FlowStrategy", 0.5)

	entryResult := strategyEngine.OnTick(&types.Tick{Symbol: "BTC-USDT", Price: 100, Timestamp: time.Now()})
	require.Len(t, entryResult.Signals, 1)
	_, err := engine.Execute(entryResult.Signals[0], 1000)
	require.NoError(t, err)
	engine.MonitorOrders()

	assert.True(t, flow.inPosition)
	assert.Equal(t, 1, flow.filledCalls)
	assert.NotNil(t, riskEngine.GetPosition("BTC-USDT"))

	exchange.tickerPrices["BTC-USDT"] = 110
	exitResult := strategyEngine.OnTick(&types.Tick{Symbol: "BTC-USDT", Price: 110, Timestamp: time.Now()})
	require.Len(t, exitResult.Signals, 1)
	assert.Equal(t, types.SignalTypeExit, exitResult.Signals[0].Type)
	_, err = engine.Execute(exitResult.Signals[0], 1000)
	require.NoError(t, err)
	engine.MonitorOrders()

	assert.False(t, flow.inPosition)
	assert.Equal(t, 1, flow.closedCalls)
	assert.Nil(t, riskEngine.GetPosition("BTC-USDT"))
	assert.GreaterOrEqual(t, riskEngine.GetDailyLoss(), 0.0)
}

func TestRebalancePlacesActualReductionOrderForOverallocatedStrategy(t *testing.T) {
	exchange := newFlowExchangeStub()
	riskEngine := risk.NewEngine(&config.RiskConfig{Enable: true, MaxPositionSize: 10000, MaxDailyLoss: 1000, MaxDrawdown: 0.2, StopLossPercent: 0.05, TakeProfitPercent: 0.1, MaxTradesPerDay: 100})
	strategyEngine := strategy.NewEngine()
	recorder := &recordingStrategy{}
	require.NoError(t, strategyEngine.AddStrategy("loser", recorder, map[string]interface{}{}))

	engine := NewEngineWithConfig(exchange, riskEngine, strategyEngine, &EngineConfig{
		RebalanceConfig: RebalanceConfig{Enabled: true, ReduceOnly: true, DriftThreshold: 0.05, UseMarketOrders: true, MaxPositionsPerCycle: 1},
	})
	allocator := engine.GetBayesianAllocator()
	require.NoError(t, allocator.Init(map[string]interface{}{"rebalance_interval": time.Duration(0), "weight_change_threshold": 0.01, "min_weight": 0.05, "max_weight": 0.9}))
	allocator.SetTotalCapital(1000)
	allocator.RegisterStrategy("winner", 0.5)
	allocator.RegisterStrategy("loser", 0.5)

	engine.recordStrategyEntryFill("loser", "BTC-USDT", types.OrderSideBuy, 100, 8)
	exchange.positions["BTC-USDT"] = &types.Position{Symbol: "BTC-USDT", Side: types.OrderSideBuy, Size: 8, EntryPrice: 100, MarkPrice: 100, Timestamp: time.Now()}

	engine.RecordTradeResult("loser", -100)

	require.NotEmpty(t, exchange.placedOrders)
	lastOrder := exchange.placedOrders[len(exchange.placedOrders)-1]
	assert.Equal(t, "BTC-USDT", lastOrder.Symbol)
	assert.Equal(t, types.OrderSideSell, lastOrder.Side)
	assert.Less(t, lastOrder.Quantity, 8.0)
	require.NotNil(t, lastOrder.Metadata)
	assert.Equal(t, "rebalance", lastOrder.Metadata["source"])
	assert.Equal(t, "loser", lastOrder.Metadata["strategy"])
}

func TestRebalanceEntryRequiresStrategyApproval(t *testing.T) {
	exchange := newFlowExchangeStub()
	riskEngine := risk.NewEngine(&config.RiskConfig{Enable: true, MaxPositionSize: 10000, MaxDailyLoss: 1000, MaxDrawdown: 0.2, StopLossPercent: 0.05, TakeProfitPercent: 0.1, MaxTradesPerDay: 100})
	strategyEngine := strategy.NewEngine()
	winner := &flowStrategy{}
	loser := &flowStrategy{rejectReason: "insufficient_signal_quality"}
	require.NoError(t, strategyEngine.AddStrategy("winner", winner, map[string]interface{}{}))
	require.NoError(t, strategyEngine.AddStrategy("loser", loser, map[string]interface{}{}))

	engine := NewEngineWithConfig(exchange, riskEngine, strategyEngine, &EngineConfig{
		RebalanceConfig:  RebalanceConfig{Enabled: true, ReduceOnly: false, DriftThreshold: 0.05, UseMarketOrders: true, MaxPositionsPerCycle: 2},
		SmartRouteConfig: SmartRouteConfig{OrderBookDepth: 2, MaxEstimatedSlippage: 0.02},
	})
	allocator := engine.GetBayesianAllocator()
	require.NoError(t, allocator.Init(map[string]interface{}{"rebalance_interval": time.Duration(0), "weight_change_threshold": 0.01, "min_weight": 0.05, "max_weight": 0.9, "portfolio_loss_limit": 0.2}))
	allocator.SetTotalCapital(1000)
	allocator.RegisterStrategy("winner", 0.5)
	allocator.RegisterStrategy("loser", 0.5)

	engine.recordStrategyEntryFill("loser", "BTC-USDT", types.OrderSideBuy, 100, 8)
	exchange.positions["BTC-USDT"] = &types.Position{Symbol: "BTC-USDT", Side: types.OrderSideBuy, Size: 8, EntryPrice: 100, MarkPrice: 100, Timestamp: time.Now()}

	engine.RecordTradeResult("loser", -100)
	assert.Len(t, exchange.placedOrders, 1)

	winner.approveTopUp = true
	engine.RecordTradeResult("winner", 80)

	foundTopUp := false
	for _, order := range exchange.placedOrders {
		if order.Symbol == "ETH-USDT" && order.Side == types.OrderSideBuy {
			foundTopUp = true
			assert.Equal(t, 49.0, order.Price)
			assert.Equal(t, 4.0, order.Quantity)
		}
	}
	assert.True(t, foundTopUp)

	foundTrackedTopUp := false
	for _, order := range engine.GetOrders() {
		if order.Symbol == "ETH-USDT" && order.Side == types.OrderSideBuy {
			foundTrackedTopUp = true
			require.NotNil(t, order.Metadata)
			assert.Equal(t, "rebalance_entry", order.Metadata["source"])
			assert.Equal(t, 49.0, order.Metadata["recommended_price"])
			assert.Equal(t, 4.0, order.Metadata["recommended_quantity"])
		}
	}
	assert.True(t, foundTrackedTopUp)
}

func TestRebalanceEntryExecutesApprovedMultiLegPlan(t *testing.T) {
	exchange := newFlowExchangeStub()
	exchange.orderBooks["BTC-USDT-SWAP"] = &types.OrderBook{
		Symbol: "BTC-USDT-SWAP",
		Asks:   []types.OrderBookLevel{{Price: 100.5, Size: 20}, {Price: 101, Size: 20}},
		Bids:   []types.OrderBookLevel{{Price: 100.5, Size: 20}, {Price: 100, Size: 20}},
	}
	exchange.tickerPrices["BTC-USDT-SWAP"] = 100.5
	riskEngine := risk.NewEngine(&config.RiskConfig{Enable: true, MaxPositionSize: 10000, MaxDailyLoss: 1000, MaxDrawdown: 0.2, StopLossPercent: 0.05, TakeProfitPercent: 0.1, MaxTradesPerDay: 100})
	strategyEngine := strategy.NewEngine()
	delta := strategy.NewDeltaNeutralFundingPro()
	require.NoError(t, delta.Init(map[string]interface{}{"spot_symbol": "BTC-USDT", "perp_symbol": "BTC-USDT-SWAP"}))
	delta.UpdateSpotPrice(100)
	delta.UpdatePerpPrice(100.5)
	require.NoError(t, strategyEngine.AddStrategy("DeltaNeutralFunding-Pro", delta, map[string]interface{}{}))

	engine := NewEngineWithConfig(exchange, riskEngine, strategyEngine, &EngineConfig{
		RebalanceConfig:  RebalanceConfig{Enabled: true, ReduceOnly: false, DriftThreshold: 0.05, UseMarketOrders: false, MaxPositionsPerCycle: 2},
		SmartRouteConfig: SmartRouteConfig{OrderBookDepth: 2, MaxEstimatedSlippage: 0.02},
	})

	err := engine.requestRebalanceEntry("DeltaNeutralFunding-Pro", &strategy.RebalanceRequest{
		Strategy:        "DeltaNeutralFunding-Pro",
		CurrentExposure: 100,
		TargetExposure:  500,
		ShortfallAmount: 400,
		Timestamp:       time.Now(),
	})
	require.NoError(t, err)
	require.Len(t, exchange.placedOrders, 2)

	assert.Equal(t, "BTC-USDT", exchange.placedOrders[0].Symbol)
	assert.Equal(t, types.OrderSideBuy, exchange.placedOrders[0].Side)
	assert.InDelta(t, 2.0, exchange.placedOrders[0].Quantity, 1e-9)

	assert.Equal(t, "BTC-USDT-SWAP", exchange.placedOrders[1].Symbol)
	assert.Equal(t, types.OrderSideSell, exchange.placedOrders[1].Side)
	assert.InDelta(t, 200.0/100.5, exchange.placedOrders[1].Quantity, 1e-9)

	tracked := engine.GetOrders()
	require.Len(t, tracked, 2)
	for _, order := range tracked {
		require.NotNil(t, order.Metadata)
		assert.Equal(t, "rebalance_entry", order.Metadata["source"])
		assert.Equal(t, "DeltaNeutralFunding-Pro", order.Metadata["strategy"])
		assert.Equal(t, 2, order.Metadata["rebalance_plan_count"])
		_, hasLabel := order.Metadata["rebalance_plan_label"]
		assert.True(t, hasLabel)
	}
}

func TestRebalanceEntryRollsBackEarlierLegWhenLaterLegPlacementFails(t *testing.T) {
	exchange := newFlowExchangeStub()
	exchange.orderBooks["BTC-USDT-SWAP"] = &types.OrderBook{
		Symbol: "BTC-USDT-SWAP",
		Asks:   []types.OrderBookLevel{{Price: 100.5, Size: 20}, {Price: 101, Size: 20}},
		Bids:   []types.OrderBookLevel{{Price: 100.5, Size: 20}, {Price: 100, Size: 20}},
	}
	exchange.tickerPrices["BTC-USDT-SWAP"] = 100.5
	exchange.failOnPlace = map[string]error{"BTC-USDT-SWAP": fmt.Errorf("perp unavailable")}

	riskEngine := risk.NewEngine(&config.RiskConfig{Enable: true, MaxPositionSize: 10000, MaxDailyLoss: 1000, MaxDrawdown: 0.2, StopLossPercent: 0.05, TakeProfitPercent: 0.1, MaxTradesPerDay: 100})
	strategyEngine := strategy.NewEngine()
	delta := strategy.NewDeltaNeutralFundingPro()
	require.NoError(t, delta.Init(map[string]interface{}{"spot_symbol": "BTC-USDT", "perp_symbol": "BTC-USDT-SWAP"}))
	delta.UpdateSpotPrice(100)
	delta.UpdatePerpPrice(100.5)
	require.NoError(t, strategyEngine.AddStrategy("DeltaNeutralFunding-Pro", delta, map[string]interface{}{}))

	engine := NewEngineWithConfig(exchange, riskEngine, strategyEngine, &EngineConfig{
		RebalanceConfig:  RebalanceConfig{Enabled: true, ReduceOnly: false, DriftThreshold: 0.05, UseMarketOrders: false, MaxPositionsPerCycle: 2},
		SmartRouteConfig: SmartRouteConfig{OrderBookDepth: 2, MaxEstimatedSlippage: 0.02},
	})

	err := engine.requestRebalanceEntry("DeltaNeutralFunding-Pro", &strategy.RebalanceRequest{
		Strategy:        "DeltaNeutralFunding-Pro",
		CurrentExposure: 100,
		TargetExposure:  500,
		ShortfallAmount: 400,
		Timestamp:       time.Now(),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "perp unavailable")
	assert.Len(t, exchange.placedOrders, 1)
	assert.Len(t, exchange.cancelledIDs, 1)
	assert.Empty(t, engine.GetOrders())
	assert.Empty(t, exchange.pending)
}

func TestRebalanceEntryPlacesCompensationOrderWhenCancelLosesRaceToFill(t *testing.T) {
	exchange := newFlowExchangeStub()
	exchange.orderBooks["BTC-USDT-SWAP"] = &types.OrderBook{
		Symbol: "BTC-USDT-SWAP",
		Asks:   []types.OrderBookLevel{{Price: 100.5, Size: 20}, {Price: 101, Size: 20}},
		Bids:   []types.OrderBookLevel{{Price: 100.5, Size: 20}, {Price: 100, Size: 20}},
	}
	exchange.tickerPrices["BTC-USDT-SWAP"] = 100.5
	exchange.failOnPlace = map[string]error{"BTC-USDT-SWAP": fmt.Errorf("perp unavailable")}
	exchange.failCancelSymbols = map[string]error{"BTC-USDT": fmt.Errorf("cancel rejected")}

	riskEngine := risk.NewEngine(&config.RiskConfig{Enable: true, MaxPositionSize: 10000, MaxDailyLoss: 1000, MaxDrawdown: 0.2, StopLossPercent: 0.05, TakeProfitPercent: 0.1, MaxTradesPerDay: 100})
	strategyEngine := strategy.NewEngine()
	delta := strategy.NewDeltaNeutralFundingPro()
	require.NoError(t, delta.Init(map[string]interface{}{"spot_symbol": "BTC-USDT", "perp_symbol": "BTC-USDT-SWAP"}))
	delta.UpdateSpotPrice(100)
	delta.UpdatePerpPrice(100.5)
	require.NoError(t, strategyEngine.AddStrategy("DeltaNeutralFunding-Pro", delta, map[string]interface{}{}))

	engine := NewEngineWithConfig(exchange, riskEngine, strategyEngine, &EngineConfig{
		RebalanceConfig:  RebalanceConfig{Enabled: true, ReduceOnly: false, DriftThreshold: 0.05, UseMarketOrders: false, MaxPositionsPerCycle: 2},
		SmartRouteConfig: SmartRouteConfig{OrderBookDepth: 2, MaxEstimatedSlippage: 0.02},
	})

	err := engine.requestRebalanceEntry("DeltaNeutralFunding-Pro", &strategy.RebalanceRequest{
		Strategy:        "DeltaNeutralFunding-Pro",
		CurrentExposure: 100,
		TargetExposure:  500,
		ShortfallAmount: 400,
		Timestamp:       time.Now(),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "perp unavailable")
	require.Len(t, exchange.placedOrders, 2)

	original := exchange.placedOrders[0]
	rollback := exchange.placedOrders[1]
	assert.Equal(t, "BTC-USDT", original.Symbol)
	assert.Equal(t, types.OrderSideBuy, original.Side)
	assert.Equal(t, "BTC-USDT", rollback.Symbol)
	assert.Equal(t, types.OrderSideSell, rollback.Side)
	assert.Equal(t, types.OrderTypeMarket, rollback.Type)
	require.NotNil(t, rollback.Metadata)
	assert.Equal(t, "rebalance_rollback", rollback.Metadata["source"])
	assert.Equal(t, original.ID, rollback.Metadata["compensates_order_id"])

	position := exchange.positions["BTC-USDT"]
	require.NotNil(t, position)
	assert.InDelta(t, original.Quantity, position.Size, 1e-9)
	assert.Len(t, exchange.cancelledIDs, 1)
	assert.Len(t, exchange.pending, 1)

	trackedOrders := engine.GetOrders()
	assert.Len(t, trackedOrders, 1)
	trackedRollback, exists := trackedOrders[rollback.ID]
	assert.True(t, exists)
	require.NotNil(t, trackedRollback)
	assert.Equal(t, "rebalance_rollback", trackedRollback.Metadata["source"])
}

func TestRebalanceEntryOpensCircuitWhenCompensationOrderFails(t *testing.T) {
	exchange := newFlowExchangeStub()
	exchange.orderBooks["BTC-USDT-SWAP"] = &types.OrderBook{
		Symbol: "BTC-USDT-SWAP",
		Asks:   []types.OrderBookLevel{{Price: 100.5, Size: 20}, {Price: 101, Size: 20}},
		Bids:   []types.OrderBookLevel{{Price: 100.5, Size: 20}, {Price: 100, Size: 20}},
	}
	exchange.tickerPrices["BTC-USDT-SWAP"] = 100.5
	exchange.failOnPlace = map[string]error{"BTC-USDT-SWAP": fmt.Errorf("perp unavailable")}
	exchange.failCancelSymbols = map[string]error{"BTC-USDT": fmt.Errorf("cancel rejected")}
	exchange.failOnPlaceBySide = map[string]error{"BTC-USDT:sell": fmt.Errorf("rollback unavailable")}

	riskEngine := risk.NewEngine(&config.RiskConfig{Enable: true, MaxPositionSize: 10000, MaxDailyLoss: 1000, MaxDrawdown: 0.2, StopLossPercent: 0.05, TakeProfitPercent: 0.1, MaxTradesPerDay: 100})
	strategyEngine := strategy.NewEngine()
	delta := strategy.NewDeltaNeutralFundingPro()
	require.NoError(t, delta.Init(map[string]interface{}{"spot_symbol": "BTC-USDT", "perp_symbol": "BTC-USDT-SWAP"}))
	delta.UpdateSpotPrice(100)
	delta.UpdatePerpPrice(100.5)
	require.NoError(t, strategyEngine.AddStrategy("DeltaNeutralFunding-Pro", delta, map[string]interface{}{}))

	engine := NewEngineWithConfig(exchange, riskEngine, strategyEngine, &EngineConfig{
		RebalanceConfig:  RebalanceConfig{Enabled: true, ReduceOnly: false, DriftThreshold: 0.05, UseMarketOrders: false, MaxPositionsPerCycle: 2},
		SmartRouteConfig: SmartRouteConfig{OrderBookDepth: 2, MaxEstimatedSlippage: 0.02},
	})

	err := engine.requestRebalanceEntry("DeltaNeutralFunding-Pro", &strategy.RebalanceRequest{
		Strategy:        "DeltaNeutralFunding-Pro",
		CurrentExposure: 100,
		TargetExposure:  500,
		ShortfallAmount: 400,
		Timestamp:       time.Now(),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rollback failed")
	assert.Contains(t, err.Error(), "rollback unavailable")

	metrics := engine.GetMetrics()
	assert.Equal(t, true, metrics["rebalance_circuit_open"])
	assert.Equal(t, "DeltaNeutralFunding-Pro", metrics["rebalance_circuit_strategy"])
	assert.Equal(t, "spot_leg", metrics["rebalance_circuit_step"])
	assert.Equal(t, 1, metrics["rebalance_rollback_failures"])

	err = engine.requestRebalanceEntry("DeltaNeutralFunding-Pro", &strategy.RebalanceRequest{
		Strategy:        "DeltaNeutralFunding-Pro",
		CurrentExposure: 100,
		TargetExposure:  500,
		ShortfallAmount: 400,
		Timestamp:       time.Now(),
	})
	assert.ErrorIs(t, err, ErrRebalanceCircuitOpen)

	position := exchange.positions["BTC-USDT"]
	require.NotNil(t, position)
	assert.Greater(t, position.Size, 0.0)
}

func TestRebalanceCircuitCanBeManuallyReset(t *testing.T) {
	exchange := newFlowExchangeStub()
	exchange.orderBooks["BTC-USDT-SWAP"] = &types.OrderBook{
		Symbol: "BTC-USDT-SWAP",
		Asks:   []types.OrderBookLevel{{Price: 100.5, Size: 20}, {Price: 101, Size: 20}},
		Bids:   []types.OrderBookLevel{{Price: 100.5, Size: 20}, {Price: 100, Size: 20}},
	}
	exchange.tickerPrices["BTC-USDT-SWAP"] = 100.5
	exchange.failOnPlace = map[string]error{"BTC-USDT-SWAP": fmt.Errorf("perp unavailable")}
	exchange.failCancelSymbols = map[string]error{"BTC-USDT": fmt.Errorf("cancel rejected")}
	exchange.failOnPlaceBySide = map[string]error{"BTC-USDT:sell": fmt.Errorf("rollback unavailable")}

	riskEngine := risk.NewEngine(&config.RiskConfig{Enable: true, MaxPositionSize: 10000, MaxDailyLoss: 1000, MaxDrawdown: 0.2, StopLossPercent: 0.05, TakeProfitPercent: 0.1, MaxTradesPerDay: 100})
	strategyEngine := strategy.NewEngine()
	delta := strategy.NewDeltaNeutralFundingPro()
	require.NoError(t, delta.Init(map[string]interface{}{"spot_symbol": "BTC-USDT", "perp_symbol": "BTC-USDT-SWAP"}))
	delta.UpdateSpotPrice(100)
	delta.UpdatePerpPrice(100.5)
	require.NoError(t, strategyEngine.AddStrategy("DeltaNeutralFunding-Pro", delta, map[string]interface{}{}))

	engine := NewEngineWithConfig(exchange, riskEngine, strategyEngine, &EngineConfig{
		RebalanceConfig:  RebalanceConfig{Enabled: true, ReduceOnly: false, DriftThreshold: 0.05, UseMarketOrders: false, MaxPositionsPerCycle: 2, CircuitAutoReset: false},
		SmartRouteConfig: SmartRouteConfig{OrderBookDepth: 2, MaxEstimatedSlippage: 0.02},
	})

	err := engine.requestRebalanceEntry("DeltaNeutralFunding-Pro", &strategy.RebalanceRequest{
		Strategy:        "DeltaNeutralFunding-Pro",
		CurrentExposure: 100,
		TargetExposure:  500,
		ShortfallAmount: 400,
		Timestamp:       time.Now(),
	})
	require.Error(t, err)
	assert.ErrorIs(t, engine.requestRebalanceEntry("DeltaNeutralFunding-Pro", &strategy.RebalanceRequest{
		Strategy:        "DeltaNeutralFunding-Pro",
		CurrentExposure: 100,
		TargetExposure:  500,
		ShortfallAmount: 400,
		Timestamp:       time.Now(),
	}), ErrRebalanceCircuitOpen)

	assert.True(t, engine.ResetRebalanceCircuit("operator_acknowledged"))
	state := engine.GetRebalanceCircuitState()
	assert.False(t, state.Open)
	assert.Equal(t, "operator_acknowledged", state.LastResetReason)
	assert.False(t, state.LastResetAt.IsZero())

	exchange.failOnPlace = nil
	exchange.failCancelSymbols = nil
	exchange.failOnPlaceBySide = nil
	err = engine.requestRebalanceEntry("DeltaNeutralFunding-Pro", &strategy.RebalanceRequest{
		Strategy:        "DeltaNeutralFunding-Pro",
		CurrentExposure: 100,
		TargetExposure:  500,
		ShortfallAmount: 400,
		Timestamp:       time.Now(),
	})
	require.NoError(t, err)
	assert.Len(t, exchange.placedOrders, 3)

	metrics := engine.GetMetrics()
	assert.Equal(t, false, metrics["rebalance_circuit_open"])
	assert.Equal(t, "operator_acknowledged", metrics["rebalance_circuit_last_reset_reason"])
}

func TestRebalanceCircuitAutoResetsAfterCooldown(t *testing.T) {
	exchange := newFlowExchangeStub()
	exchange.orderBooks["BTC-USDT-SWAP"] = &types.OrderBook{
		Symbol: "BTC-USDT-SWAP",
		Asks:   []types.OrderBookLevel{{Price: 100.5, Size: 20}, {Price: 101, Size: 20}},
		Bids:   []types.OrderBookLevel{{Price: 100.5, Size: 20}, {Price: 100, Size: 20}},
	}
	exchange.tickerPrices["BTC-USDT-SWAP"] = 100.5
	exchange.failOnPlace = map[string]error{"BTC-USDT-SWAP": fmt.Errorf("perp unavailable")}
	exchange.failCancelSymbols = map[string]error{"BTC-USDT": fmt.Errorf("cancel rejected")}
	exchange.failOnPlaceBySide = map[string]error{"BTC-USDT:sell": fmt.Errorf("rollback unavailable")}

	riskEngine := risk.NewEngine(&config.RiskConfig{Enable: true, MaxPositionSize: 10000, MaxDailyLoss: 1000, MaxDrawdown: 0.2, StopLossPercent: 0.05, TakeProfitPercent: 0.1, MaxTradesPerDay: 100})
	strategyEngine := strategy.NewEngine()
	delta := strategy.NewDeltaNeutralFundingPro()
	require.NoError(t, delta.Init(map[string]interface{}{"spot_symbol": "BTC-USDT", "perp_symbol": "BTC-USDT-SWAP"}))
	delta.UpdateSpotPrice(100)
	delta.UpdatePerpPrice(100.5)
	require.NoError(t, strategyEngine.AddStrategy("DeltaNeutralFunding-Pro", delta, map[string]interface{}{}))

	engine := NewEngineWithConfig(exchange, riskEngine, strategyEngine, &EngineConfig{
		RebalanceConfig:  RebalanceConfig{Enabled: true, ReduceOnly: false, DriftThreshold: 0.05, UseMarketOrders: false, MaxPositionsPerCycle: 2, CircuitAutoReset: true, CircuitCooldown: 10 * time.Millisecond},
		SmartRouteConfig: SmartRouteConfig{OrderBookDepth: 2, MaxEstimatedSlippage: 0.02},
	})

	err := engine.requestRebalanceEntry("DeltaNeutralFunding-Pro", &strategy.RebalanceRequest{
		Strategy:        "DeltaNeutralFunding-Pro",
		CurrentExposure: 100,
		TargetExposure:  500,
		ShortfallAmount: 400,
		Timestamp:       time.Now(),
	})
	require.Error(t, err)
	assert.ErrorIs(t, engine.requestRebalanceEntry("DeltaNeutralFunding-Pro", &strategy.RebalanceRequest{
		Strategy:        "DeltaNeutralFunding-Pro",
		CurrentExposure: 100,
		TargetExposure:  500,
		ShortfallAmount: 400,
		Timestamp:       time.Now(),
	}), ErrRebalanceCircuitOpen)

	time.Sleep(25 * time.Millisecond)
	exchange.failOnPlace = nil
	exchange.failCancelSymbols = nil
	exchange.failOnPlaceBySide = nil
	err = engine.requestRebalanceEntry("DeltaNeutralFunding-Pro", &strategy.RebalanceRequest{
		Strategy:        "DeltaNeutralFunding-Pro",
		CurrentExposure: 100,
		TargetExposure:  500,
		ShortfallAmount: 400,
		Timestamp:       time.Now(),
	})
	require.NoError(t, err)

	state := engine.GetRebalanceCircuitState()
	assert.False(t, state.Open)
	assert.Equal(t, "cooldown_elapsed", state.LastResetReason)
	assert.False(t, state.LastResetAt.IsZero())

	metrics := engine.GetMetrics()
	assert.Equal(t, false, metrics["rebalance_circuit_open"])
	assert.Equal(t, "cooldown_elapsed", metrics["rebalance_circuit_last_reset_reason"])
}

func TestRebalanceEntryRecoversMultiLegPartialFill(t *testing.T) {
	exchange := newFlowExchangeStub()
	exchange.orderBooks["BTC-USDT-SWAP"] = &types.OrderBook{
		Symbol: "BTC-USDT-SWAP",
		Asks:   []types.OrderBookLevel{{Price: 100.5, Size: 20}, {Price: 101, Size: 20}},
		Bids:   []types.OrderBookLevel{{Price: 100.5, Size: 20}, {Price: 100, Size: 20}},
	}
	exchange.tickerPrices["BTC-USDT-SWAP"] = 100.5

	riskEngine := risk.NewEngine(&config.RiskConfig{Enable: true, MaxPositionSize: 10000, MaxDailyLoss: 1000, MaxDrawdown: 0.2, StopLossPercent: 0.05, TakeProfitPercent: 0.1, MaxTradesPerDay: 100})
	strategyEngine := strategy.NewEngine()
	delta := strategy.NewDeltaNeutralFundingPro()
	require.NoError(t, delta.Init(map[string]interface{}{"spot_symbol": "BTC-USDT", "perp_symbol": "BTC-USDT-SWAP"}))
	delta.UpdateSpotPrice(100)
	delta.UpdatePerpPrice(100.5)
	require.NoError(t, strategyEngine.AddStrategy("DeltaNeutralFunding-Pro", delta, map[string]interface{}{}))

	engine := NewEngineWithConfig(exchange, riskEngine, strategyEngine, &EngineConfig{
		RebalanceConfig:  RebalanceConfig{Enabled: true, ReduceOnly: false, DriftThreshold: 0.05, UseMarketOrders: false, MaxPositionsPerCycle: 2},
		SmartRouteConfig: SmartRouteConfig{OrderBookDepth: 2, MaxEstimatedSlippage: 0.02},
	})

	alerts := make([]alertRecord, 0)
	events := make([]RebalanceEvent, 0)
	engine.SetAlertHandler(func(level AlertLevel, title, message string, labels map[string]string, details map[string]interface{}) {
		alerts = append(alerts, alertRecord{level: level, title: title, message: message, labels: labels, details: details})
	})
	engine.SetRebalanceEventHandler(func(event RebalanceEvent) {
		events = append(events, event)
	})

	err := engine.requestRebalanceEntry("DeltaNeutralFunding-Pro", &strategy.RebalanceRequest{
		Strategy:        "DeltaNeutralFunding-Pro",
		CurrentExposure: 100,
		TargetExposure:  500,
		ShortfallAmount: 400,
		Timestamp:       time.Now(),
	})
	require.NoError(t, err)
	require.Len(t, exchange.placedOrders, 2)

	spotOrderID := exchange.placedOrders[0].ID
	perpOrderID := exchange.placedOrders[1].ID
	exchange.orderSnapshots[spotOrderID] = []*types.Order{{Status: types.OrderStatusPartially, FilledQty: 1, AveragePrice: 100}}
	exchange.orderSnapshots[perpOrderID] = []*types.Order{{Status: types.OrderStatusPending, FilledQty: 0, AveragePrice: 100.5}}

	engine.MonitorOrders()

	assert.GreaterOrEqual(t, len(exchange.cancelledIDs), 2)
	assert.Len(t, exchange.placedOrders, 3)
	rollback := exchange.placedOrders[2]
	assert.Equal(t, "BTC-USDT", rollback.Symbol)
	assert.Equal(t, types.OrderSideSell, rollback.Side)
	assert.Equal(t, types.OrderTypeMarket, rollback.Type)
	startedAlert := findAlertRecordByEvent(t, alerts, "rebalance_recover_started")
	assert.Equal(t, "启动/运行时再平衡恢复", startedAlert.title)
	assert.Equal(t, "partial_fill_detected", startedAlert.details["reason"])
	succeededAlert := findAlertRecordByEvent(t, alerts, "rebalance_recover_succeeded")
	assert.Equal(t, "启动/运行时再平衡恢复完成", succeededAlert.title)
	assert.Equal(t, "succeeded", succeededAlert.details["phase"])
	require.Len(t, events, 2)
	assert.Equal(t, RebalanceEventRecoverStarted, events[0].Type)
	assert.Equal(t, RebalanceEventRecoverSuccess, events[1].Type)
	assert.Equal(t, "DeltaNeutralFunding-Pro", events[0].Strategy)
	assert.Equal(t, "partial_fill_detected", events[0].Reason)
}

func TestReconcileWithExchangeRecoversUnfinishedRebalancePlan(t *testing.T) {
	exchange := newFlowExchangeStub()
	exchange.tickerPrices["BTC-USDT-SWAP"] = 100.5
	riskEngine := risk.NewEngine(&config.RiskConfig{Enable: true, MaxPositionSize: 10000, MaxDailyLoss: 1000, MaxDrawdown: 0.2, StopLossPercent: 0.05, TakeProfitPercent: 0.1, MaxTradesPerDay: 100})
	strategyEngine := strategy.NewEngine()
	flow := &flowStrategy{}
	require.NoError(t, strategyEngine.AddStrategy("DeltaNeutralFunding-Pro", flow, map[string]interface{}{}))

	engine := NewEngine(exchange, riskEngine, strategyEngine)
	alerts := make([]alertRecord, 0)
	engine.SetAlertHandler(func(level AlertLevel, title, message string, labels map[string]string, details map[string]interface{}) {
		alerts = append(alerts, alertRecord{level: level, title: title, message: message, labels: labels, details: details})
	})

	planID := "startup-plan"
	spotOrder := &types.Order{ID: "persisted-spot", Symbol: "BTC-USDT", Side: types.OrderSideBuy, Type: types.OrderTypeLimit, Quantity: 2, Price: 100, Timestamp: time.Now(), Metadata: map[string]interface{}{"source": "rebalance_entry", "strategy": "DeltaNeutralFunding-Pro", "rebalance_plan_id": planID, "rebalance_plan_count": 2, "rebalance_plan_index": 0, "rebalance_plan_label": "spot_leg", "signal_type": string(types.SignalTypeBuy), "recommended_price": 100.0, "recommended_quantity": 2.0}}
	perpOrder := &types.Order{ID: "persisted-perp", Symbol: "BTC-USDT-SWAP", Side: types.OrderSideSell, Type: types.OrderTypeLimit, Quantity: 2, Price: 100.5, Timestamp: time.Now(), Metadata: map[string]interface{}{"source": "rebalance_entry", "strategy": "DeltaNeutralFunding-Pro", "rebalance_plan_id": planID, "rebalance_plan_count": 2, "rebalance_plan_index": 1, "rebalance_plan_label": "perp_leg", "signal_type": string(types.SignalTypeSell), "recommended_price": 100.5, "recommended_quantity": 2.0}}
	engine.orders[spotOrder.ID] = spotOrder
	engine.orders[perpOrder.ID] = perpOrder
	exchange.pending[spotOrder.ID] = spotOrder
	exchange.pending[perpOrder.ID] = perpOrder
	exchange.orderSnapshots[spotOrder.ID] = []*types.Order{{Symbol: "BTC-USDT", Side: types.OrderSideBuy, Type: types.OrderTypeLimit, Quantity: 2, Price: 100, Status: types.OrderStatusFilled, FilledQty: 2, AveragePrice: 100}}
	exchange.orderSnapshots[perpOrder.ID] = []*types.Order{{Symbol: "BTC-USDT-SWAP", Side: types.OrderSideSell, Type: types.OrderTypeLimit, Quantity: 2, Price: 100.5, Status: types.OrderStatusPending, FilledQty: 0, AveragePrice: 100.5}}

	err := engine.ReconcileWithExchange()
	require.NoError(t, err)
	assert.Len(t, exchange.placedOrders, 1)
	assert.Equal(t, types.OrderSideSell, exchange.placedOrders[0].Side)
	startedAlert := findAlertRecordByEvent(t, alerts, "rebalance_recover_started")
	assert.Equal(t, "启动/运行时再平衡恢复", startedAlert.title)
	assert.Equal(t, planID, startedAlert.details["plan_id"])
	assert.Equal(t, "startup_reconcile", startedAlert.details["reason"])
	succeededAlert := findAlertRecordByEvent(t, alerts, "rebalance_recover_succeeded")
	assert.Equal(t, "启动/运行时再平衡恢复完成", succeededAlert.title)
	assert.Equal(t, "succeeded", succeededAlert.details["phase"])
}

func findAlertRecordByEvent(t *testing.T, alerts []alertRecord, expectedEvent string) alertRecord {
	t.Helper()
	for _, alert := range alerts {
		if alert.labels["event"] == expectedEvent {
			return alert
		}
	}
	t.Fatalf("expected alert event %s", expectedEvent)
	return alertRecord{}
}

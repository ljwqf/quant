package strategy

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/ljwqf/quant/pkg/types"
)

// mockStrategy 是一个用于测试的模拟策略
 type mockStrategy struct {
	name   string
	params map[string]interface{}
	metrics map[string]interface{}
	tickCalled     int
	barCalled      int
	orderBookCalled int
	positionFilledCalled     int
	positionReducedCalled    int
	positionClosedCalled     int
	rebalanceCalled          int
}

func newMockStrategy(name string) *mockStrategy {
	return &mockStrategy{
		name:    name,
		params:  make(map[string]interface{}),
		metrics: make(map[string]interface{}),
	}
}

func (m *mockStrategy) Name() string {
	return m.name
}

func (m *mockStrategy) Init(params map[string]interface{}) error {
	m.params = params
	return nil
}

func (m *mockStrategy) OnTick(tick *types.Tick) (*types.Signal, error) {
	m.tickCalled++
	return &types.Signal{
		Symbol:    tick.Symbol,
		Type:      types.SignalTypeBuy,
		Price:     tick.Price,
		Timestamp: time.Now(),
	}, nil
}

func (m *mockStrategy) OnBar(bar *types.Bar) (*types.Signal, error) {
	m.barCalled++
	return &types.Signal{
		Symbol:    bar.Symbol,
		Type:      types.SignalTypeSell,
		Price:     bar.Close,
		Timestamp: time.Now(),
	}, nil
}

func (m *mockStrategy) OnOrderBook(orderBook *types.OrderBook) (*types.Signal, error) {
	m.orderBookCalled++
	return nil, nil
}

func (m *mockStrategy) GetParams() map[string]interface{} {
	return m.params
}

func (m *mockStrategy) SetParams(params map[string]interface{}) {
	m.params = params
}

func (m *mockStrategy) GetMetrics() map[string]interface{} {
	return m.metrics
}

func (m *mockStrategy) OnPositionFilled(symbol string, side types.OrderSide, entryPrice, size float64) {
	m.positionFilledCalled++
}

func (m *mockStrategy) OnPositionReduced(symbol string, exitPrice, pnl, remainingSize float64) {
	m.positionReducedCalled++
}

func (m *mockStrategy) OnPositionClosed(symbol string, exitPrice, pnl float64) {
	m.positionClosedCalled++
}

func (m *mockStrategy) ConfirmRebalanceEntry(request *RebalanceRequest) (*RebalanceDecision, error) {
	m.rebalanceCalled++
	return &RebalanceDecision{
		Approved: true,
	}, nil
}

func TestEngineAddStrategy(t *testing.T) {
	engine := NewEngine()
	strategy := newMockStrategy("test-strategy")
	params := map[string]interface{}{
		"param1": "value1",
		"param2": 123,
	}

	err := engine.AddStrategy("test-strategy", strategy, params)
	assert.NoError(t, err)
	assert.Equal(t, 1, engine.GetStrategyCount())
	assert.True(t, engine.HasStrategy("test-strategy"))
}

func TestEngineRemoveStrategy(t *testing.T) {
	engine := NewEngine()
	strategy := newMockStrategy("test-strategy")
	params := map[string]interface{}{
		"param1": "value1",
	}

	err := engine.AddStrategy("test-strategy", strategy, params)
	assert.NoError(t, err)
	assert.Equal(t, 1, engine.GetStrategyCount())

	engine.RemoveStrategy("test-strategy")
	assert.Equal(t, 0, engine.GetStrategyCount())
	assert.False(t, engine.HasStrategy("test-strategy"))
}

func TestEngineGetStrategy(t *testing.T) {
	engine := NewEngine()
	strategy := newMockStrategy("test-strategy")
	params := map[string]interface{}{
		"param1": "value1",
	}

	err := engine.AddStrategy("test-strategy", strategy, params)
	assert.NoError(t, err)

	retrievedStrategy := engine.GetStrategy("test-strategy")
	assert.NotNil(t, retrievedStrategy)
	assert.Equal(t, "test-strategy", retrievedStrategy.Name())
}

func TestEngineGetStrategies(t *testing.T) {
	engine := NewEngine()

	// 添加两个策略
	strategy1 := newMockStrategy("strategy1")
	strategy2 := newMockStrategy("strategy2")

	err := engine.AddStrategy("strategy1", strategy1, map[string]interface{}{})
	assert.NoError(t, err)
	err = engine.AddStrategy("strategy2", strategy2, map[string]interface{}{})
	assert.NoError(t, err)

	strategies := engine.GetStrategies()
	assert.Len(t, strategies, 2)
	assert.Contains(t, strategies, "strategy1")
	assert.Contains(t, strategies, "strategy2")
}

func TestEngineOnTick(t *testing.T) {
	engine := NewEngine()
	strategy := newMockStrategy("test-strategy")

	err := engine.AddStrategy("test-strategy", strategy, map[string]interface{}{})
	assert.NoError(t, err)

	tick := &types.Tick{
		Symbol:    "BTC-USDT",
		Price:     10000,
		Timestamp: time.Now(),
	}

	result := engine.OnTick(tick)
	assert.NotNil(t, result)
	assert.Len(t, result.Signals, 1)
	assert.Equal(t, "test-strategy", result.Signals[0].Strategy)
	assert.Equal(t, "BTC-USDT", result.Signals[0].Symbol)
	assert.Equal(t, 1, strategy.tickCalled)
}

func TestEngineOnBar(t *testing.T) {
	engine := NewEngine()
	strategy := newMockStrategy("test-strategy")

	err := engine.AddStrategy("test-strategy", strategy, map[string]interface{}{})
	assert.NoError(t, err)

	bar := &types.Bar{
		Symbol:    "BTC-USDT",
		Open:      9900,
		High:      10100,
		Low:       9800,
		Close:     10000,
		Volume:    1000,
		Timestamp: time.Now(),
		Interval:  "1m",
	}

	result := engine.OnBar(bar)
	assert.NotNil(t, result)
	assert.Len(t, result.Signals, 1)
	assert.Equal(t, "test-strategy", result.Signals[0].Strategy)
	assert.Equal(t, "BTC-USDT", result.Signals[0].Symbol)
	assert.Equal(t, 1, strategy.barCalled)
}

func TestEngineOnOrderBook(t *testing.T) {
	engine := NewEngine()
	strategy := newMockStrategy("test-strategy")

	err := engine.AddStrategy("test-strategy", strategy, map[string]interface{}{})
	assert.NoError(t, err)

	orderBook := &types.OrderBook{
		Symbol: "BTC-USDT",
		Asks: []types.OrderBookLevel{
			{Price: 10010, Size: 0.5},
			{Price: 10020, Size: 1.0},
		},
		Bids: []types.OrderBookLevel{
			{Price: 10000, Size: 0.5},
			{Price: 9990, Size: 1.0},
		},
		Timestamp: time.Now(),
	}

	result := engine.OnOrderBook(orderBook)
	assert.NotNil(t, result)
	assert.Len(t, result.Signals, 0) // mock策略返回nil信号
	assert.Equal(t, 1, strategy.orderBookCalled)
}

func TestEngineGetStrategyParams(t *testing.T) {
	engine := NewEngine()
	strategy := newMockStrategy("test-strategy")
	params := map[string]interface{}{
		"param1": "value1",
		"param2": 123,
	}

	err := engine.AddStrategy("test-strategy", strategy, params)
	assert.NoError(t, err)

	retrievedParams := engine.GetStrategyParams("test-strategy")
	assert.NotNil(t, retrievedParams)
	assert.Equal(t, "value1", retrievedParams["param1"])
	assert.Equal(t, 123, retrievedParams["param2"])
}

func TestEngineSetStrategyParams(t *testing.T) {
	engine := NewEngine()
	strategy := newMockStrategy("test-strategy")
	initialParams := map[string]interface{}{
		"param1": "value1",
	}

	err := engine.AddStrategy("test-strategy", strategy, initialParams)
	assert.NoError(t, err)

	newParams := map[string]interface{}{
		"param1": "value2",
		"param3": 456,
	}

	err = engine.SetStrategyParams("test-strategy", newParams)
	assert.NoError(t, err)

	retrievedParams := engine.GetStrategyParams("test-strategy")
	assert.NotNil(t, retrievedParams)
	assert.Equal(t, "value2", retrievedParams["param1"])
	assert.Equal(t, 456, retrievedParams["param3"])
}

func TestEngineGetStrategyMetrics(t *testing.T) {
	engine := NewEngine()
	strategy := newMockStrategy("test-strategy")
	strategy.metrics = map[string]interface{}{
		"metric1": 1.0,
		"metric2": 2,
	}

	err := engine.AddStrategy("test-strategy", strategy, map[string]interface{}{})
	assert.NoError(t, err)

	metrics := engine.GetStrategyMetrics("test-strategy")
	assert.NotNil(t, metrics)
	assert.Equal(t, 1.0, metrics["metric1"])
	assert.Equal(t, 2, metrics["metric2"])
}

func TestEngineGetAllStrategyMetrics(t *testing.T) {
	engine := NewEngine()

	// 添加两个策略
	strategy1 := newMockStrategy("strategy1")
	strategy1.metrics = map[string]interface{}{
		"metric1": 1.0,
	}

	strategy2 := newMockStrategy("strategy2")
	strategy2.metrics = map[string]interface{}{
		"metric2": 2.0,
	}

	err := engine.AddStrategy("strategy1", strategy1, map[string]interface{}{})
	assert.NoError(t, err)
	err = engine.AddStrategy("strategy2", strategy2, map[string]interface{}{})
	assert.NoError(t, err)

	allMetrics := engine.GetAllStrategyMetrics()
	assert.NotNil(t, allMetrics)
	assert.Len(t, allMetrics, 2)
	assert.Contains(t, allMetrics, "strategy1")
	assert.Contains(t, allMetrics, "strategy2")
	assert.Equal(t, 1.0, allMetrics["strategy1"]["metric1"])
	assert.Equal(t, 2.0, allMetrics["strategy2"]["metric2"])
}

func TestEngineGetStrategyNames(t *testing.T) {
	engine := NewEngine()

	// 添加两个策略
	strategy1 := newMockStrategy("strategy1")
	strategy2 := newMockStrategy("strategy2")

	err := engine.AddStrategy("strategy1", strategy1, map[string]interface{}{})
	assert.NoError(t, err)
	err = engine.AddStrategy("strategy2", strategy2, map[string]interface{}{})
	assert.NoError(t, err)

	names := engine.GetStrategyNames()
	assert.Len(t, names, 2)
	assert.Contains(t, names, "strategy1")
	assert.Contains(t, names, "strategy2")
}

func TestEngineNotifyPositionFilled(t *testing.T) {
	engine := NewEngine()
	strategy := newMockStrategy("test-strategy")

	err := engine.AddStrategy("test-strategy", strategy, map[string]interface{}{})
	assert.NoError(t, err)

	engine.NotifyPositionFilled("test-strategy", "BTC-USDT", types.OrderSideBuy, 10000, 0.1)
	assert.Equal(t, 1, strategy.positionFilledCalled)
}

func TestEngineNotifyPositionClosed(t *testing.T) {
	engine := NewEngine()
	strategy := newMockStrategy("test-strategy")

	err := engine.AddStrategy("test-strategy", strategy, map[string]interface{}{})
	assert.NoError(t, err)

	engine.NotifyPositionClosed("test-strategy", "BTC-USDT", 10100, 10)
	assert.Equal(t, 1, strategy.positionClosedCalled)
}

func TestEngineNotifyPositionReduced(t *testing.T) {
	engine := NewEngine()
	strategy := newMockStrategy("test-strategy")

	err := engine.AddStrategy("test-strategy", strategy, map[string]interface{}{})
	assert.NoError(t, err)

	engine.NotifyPositionReduced("test-strategy", "BTC-USDT", 10050, 5, 0.05)
	assert.Equal(t, 1, strategy.positionReducedCalled)
}

func TestEngineConfirmRebalanceEntry(t *testing.T) {
	engine := NewEngine()
	strategy := newMockStrategy("test-strategy")

	err := engine.AddStrategy("test-strategy", strategy, map[string]interface{}{})
	assert.NoError(t, err)

	request := &RebalanceRequest{
		Strategy:        "test-strategy",
		CurrentWeight:   0.5,
		TargetWeight:    0.7,
		CurrentExposure: 5000,
		TargetExposure:  7000,
		ShortfallAmount: 2000,
		Timestamp:       time.Now(),
	}

	decision, err := engine.ConfirmRebalanceEntry("test-strategy", request)
	assert.NoError(t, err)
	assert.NotNil(t, decision)
	assert.True(t, decision.Approved)
	assert.Equal(t, 1, strategy.rebalanceCalled)
}

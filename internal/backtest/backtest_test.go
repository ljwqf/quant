package backtest

import (
	"testing"
	"time"

	"github.com/ljwqf/quant/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testBacktestStrategy struct {
	name       string
	signals    []*types.Signal
	callCount  int
}

func (ts *testBacktestStrategy) Name() string {
	return ts.name
}

func (ts *testBacktestStrategy) Init(params map[string]interface{}) error {
	return nil
}

func (ts *testBacktestStrategy) OnBar(bar *types.Bar) (*types.Signal, error) {
	ts.callCount++
	if ts.callCount <= len(ts.signals) {
		return ts.signals[ts.callCount-1], nil
	}
	return nil, nil
}

func (ts *testBacktestStrategy) GetParameters() map[string]interface{} {
	return nil
}

func generateTestBars(count int) []*types.Bar {
	bars := make([]*types.Bar, 0, count)
	baseTime := time.Now().Add(-time.Duration(count) * time.Hour)
	
	for i := 0; i < count; i++ {
		bars = append(bars, &types.Bar{
			Symbol:    "BTC-USDT",
			Timestamp: baseTime.Add(time.Duration(i) * time.Hour),
			Open:      50000.0 + float64(i)*10,
			High:      50000.0 + float64(i)*10 + 50,
			Low:       50000.0 + float64(i)*10 - 50,
			Close:     50000.0 + float64(i)*10 + 20,
			Volume:    1000.0,
		})
	}
	return bars
}

func TestNewEngine(t *testing.T) {
	strategy := &testBacktestStrategy{name: "test"}
	engine := NewEngine(strategy, 10000.0)
	
	require.NotNil(t, engine)
	assert.NotNil(t, engine.strategy)
	assert.NotNil(t, engine.dataManager)
	assert.NotNil(t, engine.simulator)
	assert.NotNil(t, engine.result)
	assert.Equal(t, 10000.0, engine.initialBalance)
}

func TestEngineAddData(t *testing.T) {
	strategy := &testBacktestStrategy{name: "test"}
	engine := NewEngine(strategy, 10000.0)
	
	bars := generateTestBars(10)
	err := engine.AddData("BTC-USDT", bars)
	
	require.NoError(t, err)
}

func TestEngineRunNoData(t *testing.T) {
	strategy := &testBacktestStrategy{name: "test"}
	engine := NewEngine(strategy, 10000.0)
	
	err := engine.Run()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "没有数据")
}

func TestEngineRunWithData(t *testing.T) {
	signals := []*types.Signal{}
	
	strategy := &testBacktestStrategy{name: "test", signals: signals}
	engine := NewEngine(strategy, 10000.0)
	
	bars := generateTestBars(20)
	engine.AddData("BTC-USDT", bars)
	
	err := engine.Run()
	require.NoError(t, err)
	
	result := engine.GetResult()
	assert.NotNil(t, result)
	assert.Equal(t, 10000.0, result.InitialBalance)
}

func TestDataManager(t *testing.T) {
	dm := NewDataManager()
	require.NotNil(t, dm)
	
	bars := generateTestBars(10)
	err := dm.AddData("BTC-USDT", bars)
	require.NoError(t, err)
	
	symbols := dm.GetSymbols()
	assert.Contains(t, symbols, "BTC-USDT")
	
	retrievedBars, err := dm.GetData("BTC-USDT")
	require.NoError(t, err)
	assert.Len(t, retrievedBars, 10)
	
	sortedData, err := dm.GetSortedData()
	require.NoError(t, err)
	assert.Len(t, sortedData, 10)
}

func TestSimulator(t *testing.T) {
	sim := NewSimulator(10000.0)
	require.NotNil(t, sim)
	
	balance := sim.GetBalance()
	assert.Equal(t, 10000.0, balance)
	
	equity := sim.GetEquity()
	assert.Equal(t, 10000.0, equity)
}

func TestReportGenerator(t *testing.T) {
	result := &Result{
		TotalTrades:    10,
		WinTrades:      6,
		LossTrades:     4,
		TotalPnL:       1000.0,
		InitialBalance: 10000.0,
		FinalBalance:   11000.0,
		StartTime:      time.Now().Add(-24 * time.Hour),
		EndTime:        time.Now(),
	}
	
	rg := NewReportGenerator(result)
	require.NotNil(t, rg)
	
	report := rg.Generate("test_strategy")
	require.NotNil(t, report)
	assert.Equal(t, 10, report.Metrics.TotalTrades)
	assert.Equal(t, 6, report.Metrics.WinTrades)
}

func TestMultiStrategyEngine(t *testing.T) {
	mse := NewMultiStrategyEngine()
	require.NotNil(t, mse)
	
	strategy1 := &testBacktestStrategy{name: "test1"}
	err := mse.AddStrategy("test1", strategy1, 10000.0, 0.5)
	require.NoError(t, err)
	
	strategy2 := &testBacktestStrategy{name: "test2"}
	err = mse.AddStrategy("test2", strategy2, 10000.0, 0.5)
	require.NoError(t, err)
	
	bars := generateTestBars(10)
	err = mse.AddData("BTC-USDT", bars)
	require.NoError(t, err)
}

package dataservice

import (
	"testing"
	"time"

	"github.com/ljwqf/quant/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testDataSource struct {
	name        string
	sourceType  DataSourceType
	initialized bool
	closed      bool
}

func (ts *testDataSource) Name() string {
	return ts.name
}

func (ts *testDataSource) Type() DataSourceType {
	return ts.sourceType
}

func (ts *testDataSource) Initialize(config map[string]interface{}) error {
	ts.initialized = true
	return nil
}

func (ts *testDataSource) FetchTick(symbol string) (*types.Tick, error) {
	return &types.Tick{
		Symbol:    symbol,
		Price:     50000.0,
		Timestamp: time.Now(),
	}, nil
}

func (ts *testDataSource) FetchBars(symbol string, interval string, limit int) ([]*types.Bar, error) {
	bars := make([]*types.Bar, 0, limit)
	baseTime := time.Now().Add(-time.Duration(limit) * time.Hour)
	for i := 0; i < limit; i++ {
		bars = append(bars, &types.Bar{
			Symbol:    symbol,
			Timestamp: baseTime.Add(time.Duration(i) * time.Hour),
			Open:      50000.0 + float64(i)*10,
			High:      50000.0 + float64(i)*10 + 50,
			Low:       50000.0 + float64(i)*10 - 50,
			Close:     50000.0 + float64(i)*10 + 20,
			Volume:    1000.0,
		})
	}
	return bars, nil
}

func (ts *testDataSource) FetchOrderBook(symbol string, depth int) (*types.OrderBook, error) {
	return &types.OrderBook{
		Symbol:    symbol,
		Timestamp: time.Now(),
	}, nil
}

func (ts *testDataSource) IsHealthy() bool {
	return true
}

func (ts *testDataSource) Close() error {
	ts.closed = true
	return nil
}

func TestMemoryQueue(t *testing.T) {
	queue := NewMemoryQueue(10)
	require.NotNil(t, queue)
	
	assert.Equal(t, 0, queue.Size())
	
	data := &MarketData{
		Symbol:     "BTC-USDT",
		DataType:   "tick",
		Data:       &types.Tick{Symbol: "BTC-USDT", Price: 50000.0},
		Timestamp:  time.Now(),
		DataSource: "test",
	}
	
	err := queue.Push(data)
	require.NoError(t, err)
	assert.Equal(t, 1, queue.Size())
	
	popped, err := queue.Pop()
	require.NoError(t, err)
	assert.Equal(t, "BTC-USDT", popped.Symbol)
	assert.Equal(t, 0, queue.Size())
	
	_, err = queue.Pop()
	assert.Error(t, err)
	assert.Equal(t, ErrQueueEmpty, err)
}

func TestMemoryQueueFull(t *testing.T) {
	queue := NewMemoryQueue(2)
	require.NotNil(t, queue)
	
	data1 := &MarketData{Symbol: "BTC-USDT", DataType: "tick", Data: &types.Tick{}}
	data2 := &MarketData{Symbol: "ETH-USDT", DataType: "tick", Data: &types.Tick{}}
	data3 := &MarketData{Symbol: "SOL-USDT", DataType: "tick", Data: &types.Tick{}}
	
	err := queue.Push(data1)
	require.NoError(t, err)
	
	err = queue.Push(data2)
	require.NoError(t, err)
	
	err = queue.Push(data3)
	assert.Error(t, err)
	assert.Equal(t, ErrQueueFull, err)
}

func TestMemoryQueueClose(t *testing.T) {
	queue := NewMemoryQueue(10)
	require.NotNil(t, queue)
	
	err := queue.Close()
	require.NoError(t, err)
}

func TestSourceManager(t *testing.T) {
	manager := NewSourceManager()
	require.NotNil(t, manager)
	
	source1 := &testDataSource{name: "test1", sourceType: DataSourceTypeExchange}
	source2 := &testDataSource{name: "test2", sourceType: DataSourceTypeNews}
	
	err := manager.RegisterSource(source1)
	require.NoError(t, err)
	
	err = manager.RegisterSource(source2)
	require.NoError(t, err)
	
	err = manager.RegisterSource(source1)
	assert.Error(t, err)
	assert.Equal(t, ErrSourceAlreadyExists, err)
	
	sources := manager.GetAllSources()
	assert.Len(t, sources, 2)
	
	exchangeSources := manager.GetSourcesByType(DataSourceTypeExchange)
	assert.Len(t, exchangeSources, 1)
	
	newsSources := manager.GetSourcesByType(DataSourceTypeNews)
	assert.Len(t, newsSources, 1)
	
	healthySources := manager.GetHealthySources()
	assert.Len(t, healthySources, 2)
	
	retrievedSource, err := manager.GetSource("test1")
	require.NoError(t, err)
	assert.Equal(t, "test1", retrievedSource.Name())
	
	_, err = manager.GetSource("non_existent")
	assert.Error(t, err)
	assert.Equal(t, ErrSourceNotFound, err)
	
	err = manager.UnregisterSource("test1")
	require.NoError(t, err)
	
	sources = manager.GetAllSources()
	assert.Len(t, sources, 1)
}

func TestSourceManagerUnregisterNonExistent(t *testing.T) {
	manager := NewSourceManager()
	
	err := manager.UnregisterSource("non_existent")
	assert.Error(t, err)
	assert.Equal(t, ErrSourceNotFound, err)
}

func TestDataSource(t *testing.T) {
	source := &testDataSource{name: "test", sourceType: DataSourceTypeExchange}
	
	assert.Equal(t, "test", source.Name())
	assert.Equal(t, DataSourceTypeExchange, source.Type())
	
	err := source.Initialize(map[string]interface{}{})
	require.NoError(t, err)
	assert.True(t, source.initialized)
	
	tick, err := source.FetchTick("BTC-USDT")
	require.NoError(t, err)
	assert.Equal(t, "BTC-USDT", tick.Symbol)
	
	bars, err := source.FetchBars("BTC-USDT", "1h", 10)
	require.NoError(t, err)
	assert.Len(t, bars, 10)
	
	orderBook, err := source.FetchOrderBook("BTC-USDT", 10)
	require.NoError(t, err)
	assert.Equal(t, "BTC-USDT", orderBook.Symbol)
	
	assert.True(t, source.IsHealthy())
	
	err = source.Close()
	require.NoError(t, err)
	assert.True(t, source.closed)
}

package strategy

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/ljwqf/quant/pkg/types"
)

func TestBetaArbitrageRecognizesNormalizedBenchmarkSymbol(t *testing.T) {
	engine := NewBetaArbitrageEngine()
	err := engine.Init(map[string]interface{}{"benchmark": "BTCUSDT"})
	assert.NoError(t, err)

	signal, err := engine.OnTick(&types.Tick{Symbol: "BTC-USDT", Price: 100, Timestamp: time.Now()})

	assert.NoError(t, err)
	assert.Nil(t, signal)
	assert.Len(t, engine.btcPriceHistory, 1)
}

func TestBetaArbitrageSkipsDuplicateEntryWhenPositionExists(t *testing.T) {
	engine := NewBetaArbitrageEngine()
	err := engine.Init(map[string]interface{}{"benchmark": "BTC-USDT"})
	assert.NoError(t, err)

	engine.positions[normalizeMarketSymbol("ETH-USDT")] = time.Now()

	signal, err := engine.OnTick(&types.Tick{Symbol: "ETH-USDT", Price: 100, Timestamp: time.Now()})

	assert.NoError(t, err)
	assert.Nil(t, signal)
}

func TestBetaArbitrageOnBarRecognizesNormalizedBenchmarkSymbol(t *testing.T) {
	engine := NewBetaArbitrageEngine()
	err := engine.Init(map[string]interface{}{"benchmark": "BTCUSDT"})
	assert.NoError(t, err)

	signal, err := engine.OnBar(&types.Bar{Symbol: "BTC-USDT", Close: 101})

	assert.NoError(t, err)
	assert.Nil(t, signal)
	assert.Len(t, engine.btcPriceHistory, 1)
	assert.Equal(t, []float64{101}, engine.priceHistory[normalizeMarketSymbol("BTC-USDT")])
}

func TestBetaArbitrageMergesPriceHistoryAcrossSymbolFormats(t *testing.T) {
	engine := NewBetaArbitrageEngine()
	err := engine.Init(map[string]interface{}{"benchmark": "BTC-USDT"})
	assert.NoError(t, err)

	_, err = engine.OnTick(&types.Tick{Symbol: "ETH-USDT", Price: 100, Timestamp: time.Now()})
	assert.NoError(t, err)

	_, err = engine.OnTick(&types.Tick{Symbol: "ETHUSDT", Price: 105, Timestamp: time.Now()})
	assert.NoError(t, err)

	normalizedSymbol := normalizeMarketSymbol("ETH-USDT")
	assert.Equal(t, []float64{100, 105}, engine.priceHistory[normalizedSymbol])
	assert.NotContains(t, engine.priceHistory, "ETH-USDT")
}

func TestBetaArbitrageTracksPositionOnlyAfterFillCallback(t *testing.T) {
	engine := NewBetaArbitrageEngine()
	err := engine.Init(map[string]interface{}{
		"benchmark":      "BTC-USDT",
		"beta_threshold": 0.1,
		"rsi_threshold":  101.0,
	})
	assert.NoError(t, err)

	engine.btcPriceHistory = make([]float64, 0, 61)
	for i := 0; i < 61; i++ {
		engine.btcPriceHistory = append(engine.btcPriceHistory, 100+float64(i))
	}

	normalizedSymbol := normalizeMarketSymbol("ETH-USDT")
	engine.priceHistory[normalizedSymbol] = make([]float64, 0, 61)
	for i := 0; i < 60; i++ {
		engine.priceHistory[normalizedSymbol] = append(engine.priceHistory[normalizedSymbol], 50+float64(i))
	}

	signal, err := engine.OnTick(&types.Tick{Symbol: "ETH-USDT", Price: 110, Timestamp: time.Now()})

	assert.NoError(t, err)
	assert.NotNil(t, signal)
	assert.NotContains(t, engine.positions, normalizedSymbol)

	engine.OnPositionFilled("ETHUSDT", types.OrderSideBuy, 110, 1)
	assert.Contains(t, engine.positions, normalizedSymbol)
}

func TestBetaArbitragePositionTimeout(t *testing.T) {
	engine := NewBetaArbitrageEngine()
	err := engine.Init(map[string]interface{}{
		"benchmark":       "BTC-USDT",
		"max_holding_time": 0, // 0 hours for testing
	})
	assert.NoError(t, err)

	// Add a position with a past timestamp
	normalizedSymbol := normalizeMarketSymbol("ETH-USDT")
	engine.positions[normalizedSymbol] = time.Now().Add(-1 * time.Hour)

	// Check for timeout signal
	signal := engine.checkPositionTimeout("ETH-USDT")
	assert.NotNil(t, signal)
	assert.Equal(t, types.SignalTypeExit, signal.Type)
	assert.NotContains(t, engine.positions, normalizedSymbol)
}

func TestBetaArbitrageCalculateRSI(t *testing.T) {
	engine := NewBetaArbitrageEngine()

	// Test with insufficient data
	rsi := engine.calculateRSI([]float64{100, 101}, 14)
	assert.Equal(t, 50.0, rsi)

	// Test with all gains
	prices := make([]float64, 15)
	for i := 0; i < 15; i++ {
		prices[i] = 100 + float64(i)
	}
	rsi = engine.calculateRSI(prices, 14)
	assert.Equal(t, 100.0, rsi)

	// Test with mixed gains and losses
	prices = []float64{100, 101, 99, 100, 102, 101, 103}
	rsi = engine.calculateRSI(prices, 6)
	assert.Greater(t, rsi, 50.0)
}

func TestBetaArbitrageCalculateReturn(t *testing.T) {
	engine := NewBetaArbitrageEngine()

	// Test with insufficient data
	ret := engine.calculateReturn([]float64{100, 101}, 60)
	assert.Equal(t, 0.0, ret)

	// Test with positive return
	prices := make([]float64, 62)
	for i := 0; i < 62; i++ {
		prices[i] = 100 + float64(i)
	}
	ret = engine.calculateReturn(prices, 60)
	assert.Greater(t, ret, 0.0)

	// Test with negative return
	prices = make([]float64, 62)
	for i := 0; i < 62; i++ {
		prices[i] = 200 - float64(i)
	}
	ret = engine.calculateReturn(prices, 60)
	assert.Less(t, ret, 0.0)
}

func TestBetaArbitrageCalculateCorrelation(t *testing.T) {
	engine := NewBetaArbitrageEngine()

	// Test with insufficient data
	engine.priceHistory[normalizeMarketSymbol("ETH-USDT")] = []float64{100}
	engine.btcPriceHistory = []float64{10000}
	corr := engine.calculateCorrelation("ETH-USDT")
	assert.Equal(t, 0.0, corr)

	// Test with perfectly correlated data
	engine.priceHistory[normalizeMarketSymbol("ETH-USDT")] = []float64{100, 101, 102, 103}
	engine.btcPriceHistory = []float64{10000, 10100, 10200, 10300}
	corr = engine.calculateCorrelation("ETH-USDT")
	assert.Equal(t, 1.0, corr)
}

func TestBetaArbitrageParamFunctions(t *testing.T) {
	engine := NewBetaArbitrageEngine()
	err := engine.Init(map[string]interface{}{
		"rsi_period": 14,
		"rsi_threshold": 75.0,
	})
	assert.NoError(t, err)

	// Test paramInt with existing value
	assert.Equal(t, 14, engine.paramInt("rsi_period", 10))

	// Test paramInt with default value
	assert.Equal(t, 10, engine.paramInt("non_existent", 10))

	// Test paramFloat with existing value
	assert.Equal(t, 75.0, engine.paramFloat("rsi_threshold", 50.0))

	// Test paramFloat with default value
	assert.Equal(t, 50.0, engine.paramFloat("non_existent", 50.0))
}

func TestBetaArbitrageOnPositionClosed(t *testing.T) {
	engine := NewBetaArbitrageEngine()
	err := engine.Init(map[string]interface{}{"benchmark": "BTC-USDT"})
	assert.NoError(t, err)

	// Add a position
	normalizedSymbol := normalizeMarketSymbol("ETH-USDT")
	engine.positions[normalizedSymbol] = time.Now()
	assert.Contains(t, engine.positions, normalizedSymbol)

	// Close the position
	engine.OnPositionClosed("ETH-USDT", 110, 10)
	assert.NotContains(t, engine.positions, normalizedSymbol)
}

func TestBetaArbitrageCheckFundingRate(t *testing.T) {
	engine := NewBetaArbitrageEngine()
	err := engine.Init(map[string]interface{}{
		"benchmark":              "BTC-USDT",
		"funding_check_interval": 0, // 0 hours for testing
	})
	assert.NoError(t, err)

	// Set last funding check to past
	engine.lastFundingCheck = time.Now().Add(-1 * time.Hour)

	// Call checkFundingRate
	engine.checkFundingRate()

	// Verify lastFundingCheck was updated
	assert.GreaterOrEqual(t, engine.lastFundingCheck, time.Now().Add(-1*time.Minute))
}

func TestBetaArbitrageGettersAndSetters(t *testing.T) {
	engine := NewBetaArbitrageEngine()
	err := engine.Init(map[string]interface{}{
		"benchmark": "BTC-USDT",
		"rsi_period": 14,
	})
	assert.NoError(t, err)

	// Test GetParams
	params := engine.GetParams()
	assert.Equal(t, "BTC-USDT", params["benchmark"])
	assert.Equal(t, 14, params["rsi_period"])

	// Test SetParams
	engine.SetParams(map[string]interface{}{"rsi_period": 21})
	params = engine.GetParams()
	assert.Equal(t, 21, params["rsi_period"])

	// Test GetMetrics
	metrics := engine.GetMetrics()
	assert.Equal(t, 0, metrics["total_signals"])
	assert.Equal(t, 0.0, metrics["win_rate"])
	assert.Equal(t, 0.0, metrics["total_pnl"])
}

func TestBetaArbitrageOnOrderBook(t *testing.T) {
	engine := NewBetaArbitrageEngine()
	err := engine.Init(map[string]interface{}{"benchmark": "BTC-USDT"})
	assert.NoError(t, err)

	// Test OnOrderBook returns nil
	signal, err := engine.OnOrderBook(&types.OrderBook{Symbol: "ETH-USDT"})
	assert.NoError(t, err)
	assert.Nil(t, signal)
}

func TestBetaArbitrageCheckLiquidity(t *testing.T) {
	engine := NewBetaArbitrageEngine()

	// Test checkLiquidity returns true by default
	assert.True(t, engine.checkLiquidity("ETH-USDT"))
}
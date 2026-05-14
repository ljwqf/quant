package computation

import (
	"testing"
	"time"

	"github.com/ljwqf/quant/internal/v2/events"
	"github.com/stretchr/testify/assert"
)

func TestLiquidityFactorCalculatorDetectsWallCollapseVacuumAndDelta(t *testing.T) {
	calculator := NewLiquidityFactorCalculator(LiquidityFactorConfig{DepthLevels: 5, WallStdMultiplier: 1.0, CollapseThreshold: 0.2, VacuumDepthRatio: 0.3})
	now := time.Now()
	calculator.AddTrade(events.TickEvent{Symbol: "BTC-USDT", Quantity: 8, Direction: events.TradeDirectionBuy, Timestamp: now})
	calculator.AddTrade(events.TickEvent{Symbol: "BTC-USDT", Quantity: 2, Direction: events.TradeDirectionSell, Timestamp: now})

	first := calculator.Calculate(events.OrderBookEvent{
		Symbol:    "BTC-USDT",
		Timestamp: now,
		Bids: []events.OrderBookLevel{
			{Price: 99, Quantity: 4}, {Price: 98, Quantity: 12}, {Price: 97, Quantity: 4}, {Price: 96, Quantity: 1}, {Price: 95, Quantity: 4},
		},
		Asks: []events.OrderBookLevel{
			{Price: 101, Quantity: 4}, {Price: 102, Quantity: 18}, {Price: 103, Quantity: 1}, {Price: 104, Quantity: 1}, {Price: 105, Quantity: 4},
		},
	})
	assert.Equal(t, 102.0, first.TopAskWall.Price)
	assert.Equal(t, 98.0, first.TopBidWall.Price)
	assert.Equal(t, 6.0, first.Delta)
	assert.Greater(t, first.AskVacuum.Score, 0.5)

	second := calculator.Calculate(events.OrderBookEvent{
		Symbol:    "BTC-USDT",
		Timestamp: now.Add(time.Second),
		Bids: []events.OrderBookLevel{
			{Price: 99, Quantity: 4}, {Price: 98, Quantity: 12}, {Price: 97, Quantity: 4}, {Price: 96, Quantity: 1}, {Price: 95, Quantity: 4},
		},
		Asks: []events.OrderBookLevel{
			{Price: 101, Quantity: 4}, {Price: 102, Quantity: 8}, {Price: 103, Quantity: 1}, {Price: 104, Quantity: 1}, {Price: 105, Quantity: 4},
		},
	})
	assert.True(t, second.TopAskWall.Collapse)
	assert.Less(t, second.TopAskWall.WallDelta, 0.0)
	assert.True(t, second.SpreadStable)
}

func TestLiquidityFactorCalculatorIgnoresExpiredTrades(t *testing.T) {
	calculator := NewLiquidityFactorCalculator(LiquidityFactorConfig{DeltaWindow: time.Second})
	now := time.Now()
	calculator.AddTrade(events.TickEvent{Symbol: "BTC-USDT", Quantity: 10, Direction: events.TradeDirectionBuy, Timestamp: now.Add(-2 * time.Second)})
	calculator.AddTrade(events.TickEvent{Symbol: "BTC-USDT", Quantity: 3, Direction: events.TradeDirectionSell, Timestamp: now})

	snapshot := calculator.Calculate(events.OrderBookEvent{
		Symbol:    "BTC-USDT",
		Timestamp: now,
		Bids:      []events.OrderBookLevel{{Price: 99, Quantity: 4}},
		Asks:      []events.OrderBookLevel{{Price: 101, Quantity: 4}},
	})
	assert.Equal(t, -3.0, snapshot.Delta)
}

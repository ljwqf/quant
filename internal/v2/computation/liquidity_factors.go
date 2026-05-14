package computation

import (
	"math"
	"sort"
	"time"

	"github.com/ljwqf/quant/internal/v2/events"
)

type LiquidityFactorConfig struct {
	DepthLevels            int
	WallStdMultiplier      float64
	CollapseThreshold      float64
	NormalSpreadMultiplier float64
	VacuumDepthRatio       float64
	SpreadHistory          int
	DeltaWindow            time.Duration
	AbsorbEpsilon          float64
}

func DefaultLiquidityFactorConfig() LiquidityFactorConfig {
	return LiquidityFactorConfig{
		DepthLevels:            20,
		WallStdMultiplier:      1.5,
		CollapseThreshold:      0.25,
		NormalSpreadMultiplier: 1.5,
		VacuumDepthRatio:       0.35,
		SpreadHistory:          60,
		DeltaWindow:            10 * time.Second,
		AbsorbEpsilon:          1e-9,
	}
}

type LiquidityFactorCalculator struct {
	config        LiquidityFactorConfig
	previousWalls map[string]map[events.Side]events.LiquidityWall
	spreads       map[string][]float64
	trades        map[string][]events.TickEvent
}

func NewLiquidityFactorCalculator(config LiquidityFactorConfig) *LiquidityFactorCalculator {
	if config.DepthLevels <= 0 {
		config.DepthLevels = DefaultLiquidityFactorConfig().DepthLevels
	}
	if config.WallStdMultiplier == 0 {
		config.WallStdMultiplier = DefaultLiquidityFactorConfig().WallStdMultiplier
	}
	if config.CollapseThreshold == 0 {
		config.CollapseThreshold = DefaultLiquidityFactorConfig().CollapseThreshold
	}
	if config.NormalSpreadMultiplier == 0 {
		config.NormalSpreadMultiplier = DefaultLiquidityFactorConfig().NormalSpreadMultiplier
	}
	if config.VacuumDepthRatio == 0 {
		config.VacuumDepthRatio = DefaultLiquidityFactorConfig().VacuumDepthRatio
	}
	if config.SpreadHistory <= 0 {
		config.SpreadHistory = DefaultLiquidityFactorConfig().SpreadHistory
	}
	if config.DeltaWindow <= 0 {
		config.DeltaWindow = DefaultLiquidityFactorConfig().DeltaWindow
	}
	if config.AbsorbEpsilon == 0 {
		config.AbsorbEpsilon = DefaultLiquidityFactorConfig().AbsorbEpsilon
	}

	return &LiquidityFactorCalculator{
		config:        config,
		previousWalls: make(map[string]map[events.Side]events.LiquidityWall),
		spreads:       make(map[string][]float64),
		trades:        make(map[string][]events.TickEvent),
	}
}

func (c *LiquidityFactorCalculator) AddTrade(tick events.TickEvent) {
	if tick.Symbol == "" || tick.Timestamp.IsZero() {
		return
	}

	trades := append(c.trades[tick.Symbol], tick)
	cutoff := tick.Timestamp.Add(-c.config.DeltaWindow)
	start := 0
	for start < len(trades) && trades[start].Timestamp.Before(cutoff) {
		start++
	}
	c.trades[tick.Symbol] = trades[start:]
}

func (c *LiquidityFactorCalculator) Calculate(book events.OrderBookEvent) events.FactorSnapshot {
	snapshot := events.FactorSnapshot{Symbol: book.Symbol, Timestamp: book.Timestamp}
	if len(book.Bids) == 0 || len(book.Asks) == 0 {
		return snapshot
	}

	bids := sortedLevels(book.Bids, true, c.config.DepthLevels)
	asks := sortedLevels(book.Asks, false, c.config.DepthLevels)
	bestBid := bids[0].Price
	bestAsk := asks[0].Price
	snapshot.MidPrice = (bestBid + bestAsk) / 2
	snapshot.Spread = math.Max(0, bestAsk-bestBid)
	snapshot.SpreadStable = c.updateSpreadStability(book.Symbol, snapshot.Spread)
	snapshot.OBI = calculateOBI(bids, asks)
	snapshot.AggressiveBuyVolume, snapshot.AggressiveSellVolume, snapshot.Delta = c.calculateDelta(book.Symbol)
	snapshot.TopBidWall = c.detectWall(book.Symbol, events.SideBuy, bids, snapshot.MidPrice, snapshot.AggressiveSellVolume)
	snapshot.TopAskWall = c.detectWall(book.Symbol, events.SideSell, asks, snapshot.MidPrice, snapshot.AggressiveBuyVolume)
	snapshot.BidVacuum = c.detectVacuum(events.SideBuy, bids)
	snapshot.AskVacuum = c.detectVacuum(events.SideSell, asks)
	snapshot.WallAbsorptionScore = clamp01(1 - math.Max(0, -snapshot.TopAskWall.WallDelta)/math.Max(snapshot.TopAskWall.PreviousSize, c.config.AbsorbEpsilon))
	snapshot.BuyExhaustionScore = clamp01((snapshot.TopAskWall.AbsorbRate - 1) / math.Max(snapshot.TopAskWall.AbsorbRate, 1))
	snapshot.BreakoutStrengthScore = clamp01(math.Max(0, -snapshot.TopAskWall.WallDelta) / math.Max(snapshot.TopAskWall.PreviousSize, c.config.AbsorbEpsilon))
	snapshot.MomentumContinuation = clamp01((math.Abs(snapshot.Delta) / math.Max(snapshot.AggressiveBuyVolume+snapshot.AggressiveSellVolume, c.config.AbsorbEpsilon)))
	snapshot.PassiveActiveDivergence = clamp01(math.Abs(snapshot.Delta) / math.Max(snapshot.TopAskWall.PassiveAddVolume+snapshot.TopBidWall.PassiveAddVolume+c.config.AbsorbEpsilon, 1))

	return snapshot
}

func sortedLevels(levels []events.OrderBookLevel, desc bool, limit int) []events.OrderBookLevel {
	result := make([]events.OrderBookLevel, 0, len(levels))
	for _, level := range levels {
		if level.Price > 0 && level.Quantity > 0 {
			result = append(result, level)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if desc {
			return result[i].Price > result[j].Price
		}
		return result[i].Price < result[j].Price
	})
	if len(result) > limit {
		result = result[:limit]
	}
	return result
}

func calculateOBI(bids, asks []events.OrderBookLevel) float64 {
	bidDepth := sumQuantity(bids)
	askDepth := sumQuantity(asks)
	if bidDepth+askDepth == 0 {
		return 0
	}
	return (bidDepth - askDepth) / (bidDepth + askDepth)
}

func (c *LiquidityFactorCalculator) updateSpreadStability(symbol string, spread float64) bool {
	history := append(c.spreads[symbol], spread)
	if len(history) > c.config.SpreadHistory {
		history = history[len(history)-c.config.SpreadHistory:]
	}
	c.spreads[symbol] = history
	if len(history) < 3 {
		return true
	}
	avg := average(history)
	if avg <= 0 {
		return true
	}
	return spread <= avg*c.config.NormalSpreadMultiplier
}

func (c *LiquidityFactorCalculator) calculateDelta(symbol string) (float64, float64, float64) {
	var buyVolume, sellVolume float64
	for _, trade := range c.trades[symbol] {
		switch trade.Direction {
		case events.TradeDirectionBuy:
			buyVolume += trade.Quantity
		case events.TradeDirectionSell:
			sellVolume += trade.Quantity
		}
	}
	return buyVolume, sellVolume, buyVolume - sellVolume
}

func (c *LiquidityFactorCalculator) detectWall(symbol string, side events.Side, levels []events.OrderBookLevel, midPrice, aggressiveVolume float64) events.LiquidityWall {
	wall := events.LiquidityWall{Side: side}
	if len(levels) == 0 {
		return wall
	}

	quantities := make([]float64, len(levels))
	for i, level := range levels {
		quantities[i] = level.Quantity
	}
	mean := average(quantities)
	std := stddev(quantities, mean)
	threshold := mean + c.config.WallStdMultiplier*std

	for _, level := range levels {
		if level.Quantity >= threshold && level.Quantity > wall.Size {
			wall.Price = level.Price
			wall.Size = level.Quantity
		}
	}
	if wall.Price == 0 {
		for _, level := range levels {
			if level.Quantity > wall.Size {
				wall.Price = level.Price
				wall.Size = level.Quantity
			}
		}
	}
	if midPrice > 0 {
		wall.DistanceFromMid = math.Abs(wall.Price-midPrice) / midPrice
	}

	previous := c.previousWall(symbol, side)
	wall.PreviousSize = previous.Size
	wall.WallDelta = wall.Size - previous.Size
	if previous.Size > 0 {
		wall.Collapse = wall.WallDelta < -previous.Size*c.config.CollapseThreshold
	}
	wall.AbsorbRate = aggressiveVolume / math.Max(math.Abs(wall.WallDelta), c.config.AbsorbEpsilon)
	wall.PassiveAddVolume = math.Max(0, wall.WallDelta)
	c.storeWall(symbol, wall)
	return wall
}

func (c *LiquidityFactorCalculator) detectVacuum(side events.Side, levels []events.OrderBookLevel) events.VacuumZone {
	zone := events.VacuumZone{Side: side}
	if len(levels) < 2 {
		return zone
	}
	avgQty := averageQuantities(levels)
	if avgQty <= 0 {
		return zone
	}

	for i := 0; i < len(levels)-1; i++ {
		pairDepth := (levels[i].Quantity + levels[i+1].Quantity) / 2
		if pairDepth <= avgQty*c.config.VacuumDepthRatio {
			zone.Start = math.Min(levels[i].Price, levels[i+1].Price)
			zone.End = math.Max(levels[i].Price, levels[i+1].Price)
			zone.Score = clamp01(1 - pairDepth/avgQty)
			return zone
		}
	}
	return zone
}

func (c *LiquidityFactorCalculator) previousWall(symbol string, side events.Side) events.LiquidityWall {
	if c.previousWalls[symbol] == nil {
		return events.LiquidityWall{Side: side}
	}
	return c.previousWalls[symbol][side]
}

func (c *LiquidityFactorCalculator) storeWall(symbol string, wall events.LiquidityWall) {
	if c.previousWalls[symbol] == nil {
		c.previousWalls[symbol] = make(map[events.Side]events.LiquidityWall)
	}
	c.previousWalls[symbol][wall.Side] = wall
}

func sumQuantity(levels []events.OrderBookLevel) float64 {
	var total float64
	for _, level := range levels {
		total += level.Quantity
	}
	return total
}

func averageQuantities(levels []events.OrderBookLevel) float64 {
	if len(levels) == 0 {
		return 0
	}
	return sumQuantity(levels) / float64(len(levels))
}

func average(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	var total float64
	for _, value := range values {
		total += value
	}
	return total / float64(len(values))
}

func stddev(values []float64, mean float64) float64 {
	if len(values) == 0 {
		return 0
	}
	var variance float64
	for _, value := range values {
		diff := value - mean
		variance += diff * diff
	}
	return math.Sqrt(variance / float64(len(values)))
}

func clamp01(value float64) float64 {
	if value < 0 || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

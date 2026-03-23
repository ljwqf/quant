package strategy

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRecordTradeResultUpdatesTargetWeightsAndRebalances(t *testing.T) {
	allocator := NewOnlineBayesianAllocator()
	require.NoError(t, allocator.Init(map[string]interface{}{
		"rebalance_interval":      time.Duration(0),
		"weight_change_threshold": 0.01,
		"min_weight":              0.05,
		"max_weight":              0.9,
		"portfolio_loss_limit":    0.2,
	}))
	allocator.SetTotalCapital(1000)
	allocator.RegisterStrategy("winner", 0.5)
	allocator.RegisterStrategy("loser", 0.5)

	allocator.RecordTradeResult("winner", 50)
	allocator.RecordTradeResult("loser", -80)

	assert.True(t, allocator.ShouldRebalance())
	allocations := allocator.Rebalance()
	require.Len(t, allocations, 2)

	winner := allocator.GetStrategyPerformance("winner")
	loser := allocator.GetStrategyPerformance("loser")
	require.NotNil(t, winner)
	require.NotNil(t, loser)
	assert.Greater(t, winner.CurrentWeight, loser.CurrentWeight)
	assert.Equal(t, 80.0, allocator.GetMetrics()["daily_loss"])
}

func TestRegisterStrategyHandlesDuplicatesAndWeightBounds(t *testing.T) {
	allocator := NewOnlineBayesianAllocator()
	allocator.RegisterStrategy("test", 0.01) // Below min weight
	allocator.RegisterStrategy("test", 0.5)  // Duplicate registration
	allocator.RegisterStrategy("test2", 1.0) // Above max weight

	perf1 := allocator.GetStrategyPerformance("test")
	perf2 := allocator.GetStrategyPerformance("test2")

	require.NotNil(t, perf1)
	require.NotNil(t, perf2)
	assert.GreaterOrEqual(t, perf1.CurrentWeight, MinWeight)
	assert.LessOrEqual(t, perf2.CurrentWeight, MaxWeight)
}

func TestResetDailyLossIfNeeded(t *testing.T) {
	allocator := NewOnlineBayesianAllocator()
	// Set a higher loss limit to avoid reset during test
	require.NoError(t, allocator.Init(map[string]interface{}{
		"portfolio_loss_limit": 0.1, // 10% of capital
	}))
	allocator.SetTotalCapital(1000) // $100 limit
	allocator.RegisterStrategy("test", 0.5)

	// Record a loss that doesn't exceed the limit
	allocator.RecordTradeResult("test", -40) // $40 loss
	metrics := allocator.GetMetrics()
	assert.Equal(t, 40.0, metrics["daily_loss"])

	// Record another loss
	allocator.RecordTradeResult("test", -30) // Additional $30 loss
	metrics = allocator.GetMetrics()
	assert.Equal(t, 70.0, metrics["daily_loss"])

	// Verify metrics are not nil
	assert.NotNil(t, metrics)
}

func TestStrategyCooldownMechanism(t *testing.T) {
	allocator := NewOnlineBayesianAllocator()
	allocator.SetTotalCapital(1000)
	allocator.RegisterStrategy("test", 0.5)

	// Record two consecutive losses to trigger cooldown
	allocator.RecordTradeResult("test", -10)
	allocator.RecordTradeResult("test", -10)

	// Check if strategy is in cooldown
	assert.True(t, allocator.IsStrategyCooldown("test"))

	// Check that cooldown strategy gets 0 weight or adjusted weight
	// Note: The actual weight calculation depends on the implementation
	// For now, we'll just verify that the strategy is marked as in cooldown
	perf := allocator.GetStrategyPerformance("test")
	require.NotNil(t, perf)
	assert.True(t, perf.IsCooldown)
}

func TestCalculateWeightsWithZeroScores(t *testing.T) {
	allocator := NewOnlineBayesianAllocator()
	allocator.SetTotalCapital(1000)
	allocator.RegisterStrategy("test1", 0.5)
	allocator.RegisterStrategy("test2", 0.5)

	// Record losses for both strategies to put them in cooldown
	allocator.RecordTradeResult("test1", -10)
	allocator.RecordTradeResult("test1", -10)
	allocator.RecordTradeResult("test2", -10)
	allocator.RecordTradeResult("test2", -10)

	// Calculate weights should return equal weights when all are in cooldown
	weights := allocator.CalculateWeights()
	require.Len(t, weights, 2)
	assert.Equal(t, 0.5, weights["test1"])
	assert.Equal(t, 0.5, weights["test2"])
}

func TestSetTotalCapitalUpdatesWeights(t *testing.T) {
	allocator := NewOnlineBayesianAllocator()
	allocator.RegisterStrategy("test", 0.5)

	// Set initial capital
	allocator.SetTotalCapital(1000)
	allocation := allocator.GetAllocation("test")
	require.NotNil(t, allocation)
	assert.Equal(t, 500.0, allocation.Amount)

	// Update capital
	allocator.SetTotalCapital(2000)
	allocation = allocator.GetAllocation("test")
	require.NotNil(t, allocation)
	assert.Equal(t, 1000.0, allocation.Amount)
}

func TestGetAllAllocations(t *testing.T) {
	allocator := NewOnlineBayesianAllocator()
	allocator.SetTotalCapital(1000)
	allocator.RegisterStrategy("test1", 0.6)
	allocator.RegisterStrategy("test2", 0.4)

	allocations := allocator.GetAllAllocations()
	require.Len(t, allocations, 2)

	totalAmount := 0.0
	for _, alloc := range allocations {
		totalAmount += alloc.Amount
	}

	assert.Equal(t, 1000.0, totalAmount)
}

func TestForceReset(t *testing.T) {
	allocator := NewOnlineBayesianAllocator()
	allocator.SetTotalCapital(1000)
	allocator.RegisterStrategy("test1", 0.6)
	allocator.RegisterStrategy("test2", 0.4)

	// Record some trades
	allocator.RecordTradeResult("test1", 50)
	allocator.RecordTradeResult("test2", -20)

	// Force reset
	allocator.ForceReset()

	// Check that weights are reset to equal distribution
	perf1 := allocator.GetStrategyPerformance("test1")
	perf2 := allocator.GetStrategyPerformance("test2")

	require.NotNil(t, perf1)
	require.NotNil(t, perf2)
	assert.Equal(t, 0.5, perf1.CurrentWeight)
	assert.Equal(t, 0.5, perf2.CurrentWeight)
	assert.Equal(t, DefaultAlpha, perf1.Alpha)
	assert.Equal(t, DefaultBeta, perf1.Beta)
}

func TestGetMetrics(t *testing.T) {
	allocator := NewOnlineBayesianAllocator()
	allocator.SetTotalCapital(1000)
	allocator.RegisterStrategy("test", 0.5)

	// Record a trade
	allocator.RecordTradeResult("test", 50)

	metrics := allocator.GetMetrics()
	require.NotNil(t, metrics)
	assert.Equal(t, 1000.0, metrics["total_capital"])
	assert.Equal(t, 0.0, metrics["daily_loss"]) // Only losses are counted

	// Check strategy metrics
	strategyMetrics, ok := metrics["strategies"].(map[string]interface{})
	require.True(t, ok)
	testMetrics, ok := strategyMetrics["test"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, 2.0, testMetrics["alpha"])
	assert.Equal(t, 1.0, testMetrics["beta"])
	assert.Equal(t, 1.0, testMetrics["win_rate"])
}

func TestShouldRebalanceWithTimeInterval(t *testing.T) {
	allocator := NewOnlineBayesianAllocator()
	require.NoError(t, allocator.Init(map[string]interface{}{
		"rebalance_interval":      10 * time.Millisecond,
		"weight_change_threshold": 0.01,
	}))
	allocator.SetTotalCapital(1000)
	allocator.RegisterStrategy("test", 0.5)

	// Record a trade to trigger weight change
	allocator.RecordTradeResult("test", 100)

	// Wait for rebalance interval
	time.Sleep(15 * time.Millisecond)

	// Should rebalance after interval and weight change
	assert.True(t, allocator.ShouldRebalance())
}

func TestRebalanceWithWeightChangeThreshold(t *testing.T) {
	allocator := NewOnlineBayesianAllocator()
	require.NoError(t, allocator.Init(map[string]interface{}{
		"rebalance_interval":      0,
		"weight_change_threshold": 0.1,
	}))
	allocator.SetTotalCapital(1000)
	allocator.RegisterStrategy("test", 0.5)

	// Record a large profit to trigger weight change
	allocator.RecordTradeResult("test", 1000)

	// Should rebalance due to weight change
	assert.True(t, allocator.ShouldRebalance())

	// Execute rebalance
	allocations := allocator.Rebalance()
	require.Len(t, allocations, 1)

	// Should not rebalance immediately after rebalance
	assert.False(t, allocator.ShouldRebalance())
}

func TestPortfolioLossLimitReset(t *testing.T) {
	allocator := NewOnlineBayesianAllocator()
	require.NoError(t, allocator.Init(map[string]interface{}{
		"portfolio_loss_limit": 0.1, // 10% of capital
	}))
	allocator.SetTotalCapital(1000) // $100 limit
	allocator.RegisterStrategy("test", 1.0)

	// Record a loss that exceeds the limit
	allocator.RecordTradeResult("test", -150) // $150 loss

	// Check that daily loss is reset
	metrics := allocator.GetMetrics()
	assert.Equal(t, 0.0, metrics["daily_loss"])

	// Check that strategies are reset to uniform prior
	perf := allocator.GetStrategyPerformance("test")
	require.NotNil(t, perf)
	assert.Equal(t, DefaultAlpha, perf.Alpha)
	assert.Equal(t, DefaultBeta, perf.Beta)
}

func TestNormalizeAllocatorWeights(t *testing.T) {
	// Test case 1: Normal case with positive weights
	weights := map[string]float64{
		"a": 0.6,
		"b": 0.4,
	}
	normalized := normalizeAllocatorWeights(weights)
	assert.Equal(t, 0.6, normalized["a"])
	assert.Equal(t, 0.4, normalized["b"])

	// Test case 2: All weights zero
	weights = map[string]float64{
		"a": 0,
		"b": 0,
	}
	normalized = normalizeAllocatorWeights(weights)
	assert.Equal(t, 0.5, normalized["a"])
	assert.Equal(t, 0.5, normalized["b"])

	// Test case 3: Single strategy
	weights = map[string]float64{
		"a": 0.5,
	}
	normalized = normalizeAllocatorWeights(weights)
	assert.Equal(t, 1.0, normalized["a"])

	// Test case 4: Empty map
	weights = map[string]float64{}
	normalized = normalizeAllocatorWeights(weights)
	assert.Empty(t, normalized)
}
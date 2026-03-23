package strategy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSmartFilterUpdateOnChainData(t *testing.T) {
	filter := NewSmartFilter()

	// 测试更新链上数据
	filter.UpdateOnChainData(-5000, 0.8, 0.9)

	// 验证数据是否正确更新
	data := filter.GetOnChainData()
	assert.NotNil(t, data)
	assert.Equal(t, -5000.0, data.ExchangeNetflow)
	assert.Equal(t, 0.8, data.SOPR)
	assert.Equal(t, 0.9, data.LTHMVRV)
}

func TestSmartFilterGetMarketState(t *testing.T) {
	filter := NewSmartFilter()

	// 测试积累阶段：净流出 + 低SOPR
	filter.UpdateOnChainData(-5000, 0.8, 0.9)
	result := filter.GetMarketState()
	assert.Equal(t, MarketStateAccumulation, result.State)
	assert.True(t, result.CanLong)
	assert.False(t, result.CanShort)
	assert.Contains(t, result.Reason, "积累阶段")

	// 测试分布阶段：净流入 + 高SOPR
	filter.UpdateOnChainData(5001, 1.2, 1.1)
	result = filter.GetMarketState()
	assert.Equal(t, MarketStateDistribution, result.State)
	assert.False(t, result.CanLong)
	assert.True(t, result.CanShort)
	assert.Contains(t, result.Reason, "派发阶段")

	// 测试投降阶段：低MVRV + 低SOPR
	filter.UpdateOnChainData(-1000, 0.9, 0.7)
	result = filter.GetMarketState()
	assert.Equal(t, MarketStateCapitulation, result.State)
	assert.True(t, result.CanLong)
	assert.False(t, result.CanShort)
	assert.Contains(t, result.Reason, "投降阶段")

	// 测试中性阶段：接近零的净流量 + 接近1的SOPR
	filter.UpdateOnChainData(100, 1.0, 1.0)
	result = filter.GetMarketState()
	assert.Equal(t, MarketStateNeutral, result.State)
	assert.False(t, result.CanLong)
	assert.False(t, result.CanShort)
	assert.Contains(t, result.Reason, "中性阶段")
}

func TestSmartFilterCanOpenLong(t *testing.T) {
	filter := NewSmartFilter()

	// 测试可以开多
	filter.UpdateOnChainData(-5000, 0.8, 0.9)
	assert.True(t, filter.CanOpenLong())

	// 测试不能开多
	filter.UpdateOnChainData(5000, 1.2, 1.1)
	assert.False(t, filter.CanOpenLong())
}

func TestSmartFilterCanOpenShort(t *testing.T) {
	filter := NewSmartFilter()

	// 测试可以开空
	filter.UpdateOnChainData(5001, 1.2, 1.1)
	assert.True(t, filter.CanOpenShort())

	// 测试不能开空
	filter.UpdateOnChainData(-5000, 0.8, 0.9)
	assert.False(t, filter.CanOpenShort())
}

func TestSmartFilterCanRunNeutralStrategy(t *testing.T) {
	filter := NewSmartFilter()

	// 测试可以运行中性策略
	filter.UpdateOnChainData(100, 1.0, 1.0)
	assert.True(t, filter.CanRunNeutralStrategy())

	// 测试积累阶段也可以运行中性策略
	filter.UpdateOnChainData(-5000, 0.8, 0.9)
	assert.True(t, filter.CanRunNeutralStrategy())

	// 测试分布阶段也可以运行中性策略
	filter.UpdateOnChainData(5000, 1.2, 1.1)
	assert.True(t, filter.CanRunNeutralStrategy())
}

func TestSmartFilterFilterSignal(t *testing.T) {
	filter := NewSmartFilter()

	// 测试积累阶段
	filter.UpdateOnChainData(-5000, 0.8, 0.9)
	assert.True(t, filter.FilterSignal("long"))
	assert.False(t, filter.FilterSignal("short"))
	assert.True(t, filter.FilterSignal("neutral"))

	// 测试分布阶段
	filter.UpdateOnChainData(5001, 1.2, 1.1)
	assert.False(t, filter.FilterSignal("long"))
	assert.True(t, filter.FilterSignal("short"))
	assert.True(t, filter.FilterSignal("neutral"))

	// 测试中性阶段
	filter.UpdateOnChainData(100, 1.0, 1.0)
	assert.False(t, filter.FilterSignal("long"))
	assert.False(t, filter.FilterSignal("short"))
	assert.True(t, filter.FilterSignal("neutral"))
}

func TestSmartFilterIsDataValid(t *testing.T) {
	filter := NewSmartFilter()

	// 测试数据无效（未更新）
	assert.False(t, filter.IsDataValid())

	// 测试数据有效
	filter.UpdateOnChainData(-5000, 0.8, 0.9)
	assert.True(t, filter.IsDataValid())
}

func TestSmartFilterGetMetrics(t *testing.T) {
	filter := NewSmartFilter()

	// 测试获取指标
	filter.UpdateOnChainData(-5000, 0.8, 0.9)
	metrics := filter.GetMetrics()
	assert.NotNil(t, metrics)
	assert.Equal(t, "Accumulation", metrics["market_state"])
	assert.Equal(t, true, metrics["can_long"])
	assert.Equal(t, false, metrics["can_short"])
	assert.Equal(t, true, metrics["can_neutral"])
	assert.Equal(t, -5000.0, metrics["netflow"])
	assert.Equal(t, 0.8, metrics["sopr"])
	assert.Equal(t, 0.9, metrics["mvrv"])
}


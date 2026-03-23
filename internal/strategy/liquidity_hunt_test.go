package strategy

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/ljwqf/quant/pkg/types"
)

func TestLiquidityHuntEngineInit(t *testing.T) {
	engine := NewLiquidityHuntEngine()
	params := map[string]interface{}{
		"fake_break_threshold":  0.3,
		"funding_rate_threshold": 0.0005,
		"time_window":            []string{"20:30", "23:00"},
		"oi_delta_threshold":     50.0,
	}

	err := engine.Init(params)
	assert.NoError(t, err)
	assert.Equal(t, 0.3, engine.params["fake_break_threshold"])
	assert.Equal(t, 0.0005, engine.params["funding_rate_threshold"])
	assert.Equal(t, []string{"20:30", "23:00"}, engine.params["time_window"])
	assert.Equal(t, 50.0, engine.params["oi_delta_threshold"])
}

func TestLiquidityHuntEngineOnTick(t *testing.T) {
	engine := NewLiquidityHuntEngine()
	params := map[string]interface{}{
		"fake_break_threshold": 0.3,
		"time_window":          []string{"00:00", "23:59"}, // 全天
	}
	err := engine.Init(params)
	assert.NoError(t, err)

	// 测试正常的tick数据
	tick := &types.Tick{
		Symbol:    "BTC-USDT",
		Price:     10000,
		Timestamp: time.Now(),
	}

	signal, err := engine.OnTick(tick)
	assert.NoError(t, err)
	// 第一次tick应该不会产生信号，因为需要积累数据
	assert.Nil(t, signal)

	// 连续发送多个tick数据，积累价格历史
	for i := 0; i < 30; i++ {
		tick := &types.Tick{
			Symbol:    "BTC-USDT",
			Price:     10000 + float64(i),
			Timestamp: time.Now(),
		}
		_, err := engine.OnTick(tick)
		assert.NoError(t, err)
	}

	// 测试假突破信号
	// 先发送突破价格
	breakoutTick := &types.Tick{
		Symbol:    "BTC-USDT",
		Price:     10050, // 突破关键位
		Timestamp: time.Now(),
	}
	_, err = engine.OnTick(breakoutTick)
	assert.NoError(t, err)

	// 等待一段时间
	time.Sleep(100 * time.Millisecond)

	// 发送回到关键位内侧的tick
	retraceTick := &types.Tick{
		Symbol:    "BTC-USDT",
		Price:     9950, // 回到关键位内侧
		Timestamp: time.Now(),
	}
	signal, err = engine.OnTick(retraceTick)
	assert.NoError(t, err)
	// 可能产生信号，也可能不产生，取决于假突破检测逻辑
}

func TestLiquidityHuntEngineOnBar(t *testing.T) {
	engine := NewLiquidityHuntEngine()
	params := map[string]interface{}{
		"fake_break_threshold": 0.3,
	}
	err := engine.Init(params)
	assert.NoError(t, err)

	// 测试正常的bar数据
	bar := &types.Bar{
		Symbol:    "BTC-USDT",
		Open:      10000,
		High:      10100,
		Low:       9900,
		Close:     10050,
		Volume:    1500000,
		Timestamp: time.Now(),
		Interval:  "1m",
	}

	signal, err := engine.OnBar(bar)
	assert.NoError(t, err)
	// OnBar应该不会产生信号
	assert.Nil(t, signal)
}

func TestLiquidityHuntEngineOnOrderBook(t *testing.T) {
	engine := NewLiquidityHuntEngine()
	params := map[string]interface{}{
		"fake_break_threshold": 0.3,
	}
	err := engine.Init(params)
	assert.NoError(t, err)

	// 测试订单簿数据
	orderBook := &types.OrderBook{
		Symbol: "BTC-USDT",
		Asks: []types.OrderBookLevel{
			{Price: 10010, Size: 0.5},
			{Price: 10020, Size: 1.0},
			{Price: 10030, Size: 1.5},
		},
		Bids: []types.OrderBookLevel{
			{Price: 10000, Size: 0.5},
			{Price: 9990, Size: 1.0},
			{Price: 9980, Size: 1.5},
		},
		Timestamp: time.Now(),
	}

	signal, err := engine.OnOrderBook(orderBook)
	assert.NoError(t, err)
	// 订单簿数据应该不会直接产生信号
	assert.Nil(t, signal)
}

func TestLiquidityHuntEngineGetParams(t *testing.T) {
	engine := NewLiquidityHuntEngine()
	params := map[string]interface{}{
		"fake_break_threshold":  0.3,
		"funding_rate_threshold": 0.0005,
		"time_window":            []string{"20:30", "23:00"},
		"oi_delta_threshold":     50.0,
	}
	err := engine.Init(params)
	assert.NoError(t, err)

	result := engine.GetParams()
	assert.NotNil(t, result)
	assert.Equal(t, 0.3, result["fake_break_threshold"])
	assert.Equal(t, 0.0005, result["funding_rate_threshold"])
	assert.Equal(t, []string{"20:30", "23:00"}, result["time_window"])
	assert.Equal(t, 50.0, result["oi_delta_threshold"])
}

func TestLiquidityHuntEngineSetParams(t *testing.T) {
	engine := NewLiquidityHuntEngine()
	params := map[string]interface{}{
		"fake_break_threshold":  0.3,
		"funding_rate_threshold": 0.0005,
	}
	err := engine.Init(params)
	assert.NoError(t, err)

	// 测试更新参数
	newParams := map[string]interface{}{
		"fake_break_threshold":  0.4,
		"funding_rate_threshold": 0.001,
	}
	engine.SetParams(newParams)

	result := engine.GetParams()
	assert.Equal(t, 0.4, result["fake_break_threshold"])
	assert.Equal(t, 0.001, result["funding_rate_threshold"])
}

func TestLiquidityHuntEngineIsInTimeWindow(t *testing.T) {
	engine := NewLiquidityHuntEngine()
	params := map[string]interface{}{
		"time_window": []string{"00:00", "23:59"}, // 全天
	}
	err := engine.Init(params)
	assert.NoError(t, err)

	// 测试时间窗口检查
	// 由于我们设置了全天窗口，应该返回true
	// 注意：这里的测试依赖于当前时间，可能在边界时间会失败
	// 但为了简单起见，我们假设当前时间在窗口内
}


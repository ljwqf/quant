package execution

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ljwqf/quant/pkg/types"
)

func TestATRCalculatorUpdateAndCalculate(t *testing.T) {
	atr := NewATRCalculator(3)
	atr.Update(&types.Bar{Symbol: "BTC-USDT", High: 110, Low: 100, Close: 105})
	atr.Update(&types.Bar{Symbol: "BTC-USDT", High: 111, Low: 103, Close: 108})
	atr.Update(&types.Bar{Symbol: "BTC-USDT", High: 116, Low: 107, Close: 115})
	atr.Update(&types.Bar{Symbol: "BTC-USDT", High: 118, Low: 112, Close: 113})

	value := atr.Calculate("BTC-USDT")
	assert.Greater(t, value, 0.0)
}

func TestTakeProfitManagerFixedTrailingTieredATR(t *testing.T) {
	mgr := NewTakeProfitManager(&TakeProfitConfig{
		Enabled:            true,
		Type:               TakeProfitTypeFixed,
		FixedProfitPercent: 1.0,
		TrailingActivation: 0.5,
		TrailingDistance:   0.5,
		TrailingStep:       0.2,
		MaxHoldingTime:     5 * time.Millisecond,
		PullbackPercent:    0.1,
		ATRPeriod:          3,
		ATRMultiplier:      1.0,
		TieredLevels: []TieredLevel{
			{ProfitPercent: 1.0, ClosePercent: 50},
			{ProfitPercent: 2.0, ClosePercent: 50},
		},
	})

	mgr.AddPosition("BTC-USDT", types.OrderSideBuy, 100, 2)
	mgr.UpdatePrice("BTC-USDT", 102)
	sig := mgr.CheckTakeProfit("BTC-USDT")
	require.NotNil(t, sig)
	assert.Equal(t, "fixed_take_profit", sig.TriggerType)

	cfg := mgr.GetConfig()
	cfg.Type = TakeProfitTypeTrailing
	mgr.SetConfig(cfg)
	mgr.UpdatePrice("BTC-USDT", 104)
	mgr.UpdatePrice("BTC-USDT", 103)
	sig = mgr.CheckTakeProfit("BTC-USDT")
	require.NotNil(t, sig)
	assert.Contains(t, []string{"trailing_take_profit", "time_based_exit"}, sig.TriggerType)

	cfg.Type = TakeProfitTypeTiered
	mgr.SetConfig(cfg)
	mgr.UpdatePrice("BTC-USDT", 103)
	sig = mgr.CheckTakeProfit("BTC-USDT")
	require.NotNil(t, sig)
	assert.Equal(t, "tiered_take_profit", sig.TriggerType)

	cfg.Type = TakeProfitTypeATR
	mgr.SetConfig(cfg)
	mgr.UpdateATR("BTC-USDT", &types.Bar{Symbol: "BTC-USDT", High: 105, Low: 100, Close: 104})
	mgr.UpdateATR("BTC-USDT", &types.Bar{Symbol: "BTC-USDT", High: 106, Low: 102, Close: 105})
	mgr.UpdateATR("BTC-USDT", &types.Bar{Symbol: "BTC-USDT", High: 107, Low: 103, Close: 106})
	mgr.UpdatePrice("BTC-USDT", 104)
	sig = mgr.CheckTakeProfit("BTC-USDT")
	if sig != nil {
		assert.Equal(t, "atr_take_profit", sig.TriggerType)
	}

	state := mgr.GetPositionState("BTC-USDT")
	require.NotNil(t, state)
	all := mgr.GetAllPositions()
	assert.Contains(t, all, "BTC-USDT")

	mgr.RemovePosition("BTC-USDT")
	assert.Nil(t, mgr.GetPositionState("BTC-USDT"))
}

func TestTakeProfitHelpers(t *testing.T) {
	assert.Equal(t, types.OrderSideSell, getExitSide(types.OrderSideBuy))
	assert.Equal(t, types.OrderSideBuy, getExitSide(types.OrderSideSell))
	assert.Equal(t, "BTC-USDT_tier_1", generateTieredOrderID("BTC-USDT", 1))
}

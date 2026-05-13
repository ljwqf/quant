package execution

import (
	"testing"
	"time"

	"github.com/ljwqf/quant/internal/config"
	"github.com/ljwqf/quant/internal/risk"
	"github.com/ljwqf/quant/internal/strategy"
	"github.com/ljwqf/quant/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type reconcilerStubExchange struct {
	orders     map[string]*types.Order
	algoOrders []*types.AlgoOrder
}

func (s *reconcilerStubExchange) Connect() error                                         { return nil }
func (s *reconcilerStubExchange) Disconnect() error                                      { return nil }
func (s *reconcilerStubExchange) GetAccount() (*types.Account, error)                    { return &types.Account{}, nil }
func (s *reconcilerStubExchange) PlaceOrder(order *types.Order) (*types.OrderResult, error) {
	return &types.OrderResult{OrderID: order.ID, Status: types.OrderStatusPending}, nil
}
func (s *reconcilerStubExchange) CancelOrder(orderID string) error                   { return nil }
func (s *reconcilerStubExchange) GetOrder(orderID string) (*types.Order, error)      { return nil, nil }
func (s *reconcilerStubExchange) GetPositions() ([]*types.Position, error)           { return nil, nil }
func (s *reconcilerStubExchange) SubscribeTicker(symbol string, handler func(*types.Tick)) error { return nil }
func (s *reconcilerStubExchange) SubscribeBar(symbol string, interval string, handler func(*types.Bar)) error {
	return nil
}
func (s *reconcilerStubExchange) SubscribeOrderBook(symbol string, handler func(*types.OrderBook)) error {
	return nil
}
func (s *reconcilerStubExchange) GetBars(symbol string, interval string, limit int) ([]*types.Bar, error) {
	return nil, nil
}
func (s *reconcilerStubExchange) GetTicker(symbol string) (*types.Tick, error) { return nil, nil }
func (s *reconcilerStubExchange) GetOrderBook(symbol string, depth int) (*types.OrderBook, error) {
	return nil, nil
}
func (s *reconcilerStubExchange) SetLeverage(symbol string, leverage int, marginMode string) error {
	return nil
}
func (s *reconcilerStubExchange) PlaceAlgoOrder(order *types.AlgoOrder) (*types.AlgoOrderResult, error) {
	return &types.AlgoOrderResult{AlgoID: order.AlgoID}, nil
}
func (s *reconcilerStubExchange) CancelAlgoOrder(algoID, symbol string) error { return nil }
func (s *reconcilerStubExchange) GetFundingRate(instId string) (*types.FundingRate, error) {
	return nil, nil
}

func (s *reconcilerStubExchange) GetOrders(symbol string, limit int) ([]*types.Order, error) {
	result := make([]*types.Order, 0, len(s.orders))
	for _, o := range s.orders {
		if o.Symbol == symbol {
			result = append(result, o)
		}
	}
	return result, nil
}

func (s *reconcilerStubExchange) GetAlgoOrders(symbol string, orderType string) ([]*types.AlgoOrder, error) {
	result := make([]*types.AlgoOrder, 0, len(s.algoOrders))
	for _, a := range s.algoOrders {
		if a.Symbol == symbol {
			result = append(result, a)
		}
	}
	return result, nil
}

func newTestRiskEngine() *risk.Engine {
	return risk.NewEngine(&config.RiskConfig{
		Enable:          true,
		MaxPositionSize: 10000,
		MaxDailyLoss:    1000,
		MaxDrawdown:     0.2,
		StopLossPercent: 0.05,
		TakeProfitPercent: 0.1,
		MaxTradesPerDay: 100,
	})
}

func TestOrderReconcilerStartStop(t *testing.T) {
	ex := &reconcilerStubExchange{orders: make(map[string]*types.Order)}
	riskEngine := newTestRiskEngine()
	execEngine := NewEngine(ex, riskEngine, strategy.NewEngine())

	cfg := &OrderReconcilerConfig{
		Enabled:  true,
		Symbols:  []string{"BTC-USDT"},
		Interval: 100 * time.Millisecond,
	}

	reconciler := NewOrderReconciler(ex, riskEngine, execEngine, cfg)
	reconciler.Start()
	reconciler.Stop()
	// Second stop should not panic
	reconciler.Stop()
}

func TestOrderReconcilerDetectsOrphanedOrder(t *testing.T) {
	ex := &reconcilerStubExchange{
		orders: map[string]*types.Order{
			"orphan-1": {
				ID:        "orphan-1",
				Symbol:    "BTC-USDT",
				Side:      types.OrderSideBuy,
				Type:      types.OrderTypeLimit,
				Quantity:  1.0,
				Price:     50000,
				Status:    types.OrderStatusPending,
				FilledQty: 0,
				Timestamp: time.Now().Add(-5 * time.Minute),
			},
		},
	}
	riskEngine := newTestRiskEngine()
	execEngine := NewEngine(ex, riskEngine, strategy.NewEngine())

	cfg := &OrderReconcilerConfig{
		Enabled:  true,
		Symbols:  []string{"BTC-USDT"},
		Interval: time.Hour,
	}

	reconciler := NewOrderReconciler(ex, riskEngine, execEngine, cfg)
	reconciler.reconcileSymbol("BTC-USDT")

	// Orphaned order should be tracked
	order := execEngine.GetOrder("orphan-1")
	require.NotNil(t, order)
	assert.Equal(t, "reconciler", order.Metadata["source"])

	metrics := reconciler.GetMetrics()
	assert.Equal(t, int64(1), metrics["discrepancies_found"])
	assert.Equal(t, int64(1), metrics["discrepancies_fixed"])
}

func TestOrderReconcilerNoDiscrepancy(t *testing.T) {
	ex := &reconcilerStubExchange{orders: make(map[string]*types.Order)}
	riskEngine := newTestRiskEngine()
	execEngine := NewEngine(ex, riskEngine, strategy.NewEngine())

	cfg := &OrderReconcilerConfig{
		Enabled:  true,
		Symbols:  []string{"ETH-USDT"},
		Interval: time.Hour,
	}

	reconciler := NewOrderReconciler(ex, riskEngine, execEngine, cfg)
	reconciler.reconcileSymbol("ETH-USDT")

	metrics := reconciler.GetMetrics()
	assert.Equal(t, int64(0), metrics["discrepancies_found"])
}

func TestOrderReconcilerSkipsOldOrders(t *testing.T) {
	ex := &reconcilerStubExchange{
		orders: map[string]*types.Order{
			"old-order": {
				ID:        "old-order",
				Symbol:    "BTC-USDT",
				Side:      types.OrderSideSell,
				Type:      types.OrderTypeMarket,
				Quantity:  0.5,
				Status:    types.OrderStatusFilled,
				FilledQty: 0.5,
				Timestamp: time.Now().Add(-48 * time.Hour), // 2 days old
			},
		},
	}
	riskEngine := newTestRiskEngine()
	execEngine := NewEngine(ex, riskEngine, strategy.NewEngine())

	cfg := &OrderReconcilerConfig{
		Enabled:  true,
		Symbols:  []string{"BTC-USDT"},
		Interval: time.Hour,
	}

	reconciler := NewOrderReconciler(ex, riskEngine, execEngine, cfg)
	reconciler.reconcileSymbol("BTC-USDT")

	// Old order should be skipped
	order := execEngine.GetOrder("old-order")
	assert.Nil(t, order)
}

func TestOrderReconcilerConfigDefaultInterval(t *testing.T) {
	ex := &reconcilerStubExchange{orders: make(map[string]*types.Order)}
	riskEngine := newTestRiskEngine()
	execEngine := NewEngine(ex, riskEngine, strategy.NewEngine())

	cfg := &OrderReconcilerConfig{
		Enabled:  true,
		Symbols:  []string{"BTC-USDT"},
		Interval: 0, // zero → should default to 30s
	}

	reconciler := NewOrderReconciler(ex, riskEngine, execEngine, cfg)
	assert.Equal(t, 30*time.Second, reconciler.interval)
}

func TestOrderReconcilerTrackExternalAlgoOrder(t *testing.T) {
	ex := &reconcilerStubExchange{orders: make(map[string]*types.Order)}
	riskEngine := newTestRiskEngine()
	execEngine := NewEngine(ex, riskEngine, strategy.NewEngine())

	algo := &types.AlgoOrder{
		AlgoID:      "algo-1",
		Symbol:      "BTC-USDT-SWAP",
		Side:        types.OrderSideSell,
		SlTriggerPx: 48000,
		State:       "live",
	}

	execEngine.TrackExternalAlgoOrder(algo)

	algoOrders := execEngine.GetAlgoOrders()
	require.Len(t, algoOrders, 1)
	assert.Equal(t, "algo-1", algoOrders["algo-1"].AlgoID)
}

func TestOrderReconcilerDetectsOrphanedAlgoOrder(t *testing.T) {
	ex := &reconcilerStubExchange{
		orders: make(map[string]*types.Order),
		algoOrders: []*types.AlgoOrder{
			{
				AlgoID:      "algo-orphan-1",
				Symbol:      "BTC-USDT-SWAP",
				Side:        types.OrderSideSell,
				SlTriggerPx: 48000,
				State:       "live",
			},
		},
	}
	riskEngine := newTestRiskEngine()
	execEngine := NewEngine(ex, riskEngine, strategy.NewEngine())

	cfg := &OrderReconcilerConfig{
		Enabled:  true,
		Symbols:  []string{"BTC-USDT-SWAP"},
		Interval: time.Hour,
	}

	reconciler := NewOrderReconciler(ex, riskEngine, execEngine, cfg)
	reconciler.reconcileSymbol("BTC-USDT-SWAP")

	// Orphaned algo order should be tracked
	algoOrders := execEngine.GetAlgoOrders()
	require.Len(t, algoOrders, 1)
	assert.Equal(t, "algo-orphan-1", algoOrders["algo-orphan-1"].AlgoID)

	metrics := reconciler.GetMetrics()
	assert.Equal(t, int64(1), metrics["discrepancies_found"])
	assert.Equal(t, int64(1), metrics["discrepancies_fixed"])
}

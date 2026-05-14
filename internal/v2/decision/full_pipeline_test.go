package decision

import (
	"context"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ljwqf/quant/internal/v2/events"
	"github.com/ljwqf/quant/internal/v2/ingestion"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockDataProvider struct {
	tickHandlers   map[string]func(*events.TickEvent)
	bookHandlers   map[string]func(*events.OrderBookEvent)
	klineHandlers  map[string]func(*events.KlineEvent)
	subscribeCount atomic.Int32
}

func newMockDataProvider() *mockDataProvider {
	return &mockDataProvider{
		tickHandlers:  make(map[string]func(*events.TickEvent)),
		bookHandlers:  make(map[string]func(*events.OrderBookEvent)),
		klineHandlers: make(map[string]func(*events.KlineEvent)),
	}
}

func (m *mockDataProvider) SubscribeTicker(symbol string, handler func(*events.TickEvent)) error {
	m.tickHandlers[symbol] = handler
	m.subscribeCount.Add(1)
	return nil
}

func (m *mockDataProvider) SubscribeOrderBook(symbol string, handler func(*events.OrderBookEvent)) error {
	m.bookHandlers[symbol] = handler
	m.subscribeCount.Add(1)
	return nil
}

func (m *mockDataProvider) SubscribeKline(symbol string, interval string, handler func(*events.KlineEvent)) error {
	m.klineHandlers[symbol] = handler
	m.subscribeCount.Add(1)
	return nil
}

func (m *mockDataProvider) GetAccount() (*ingestion.AccountSnapshot, error) {
	return &ingestion.AccountSnapshot{TotalEquity: 10000, TotalAvailable: 10000}, nil
}

func (m *mockDataProvider) GetOrderBook(symbol string, depth int) (*events.OrderBookEvent, error) {
	return nil, nil
}

func (m *mockDataProvider) emitBook(symbol string, book events.OrderBookEvent) {
	if handler, ok := m.bookHandlers[symbol]; ok {
		handler(&book)
	}
}

func (m *mockDataProvider) emitTick(symbol string, tick events.TickEvent) {
	if handler, ok := m.tickHandlers[symbol]; ok {
		handler(&tick)
	}
}

func TestFullPipelineStartsAndProcessesData(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "pipeline")
	provider := newMockDataProvider()

	config := FullPipelineConfig{
		Symbols:              []string{"BTC-USDT"},
		Intervals:            []string{"1m"},
		LogDir:               dir,
		MissedSignalBps:      200,
		OpenThreshold:        0.6,
		CooldownDuration:     time.Second,
		BaseCapital:          10000,
		BaseRiskPercent:      0.01,
		ProfitRiskPercent:    0.02,
		MaxLossPerTradeBps:   100,
		MaxLossPerDayBps:     5000,
		MaxLossPerWeekBps:    5000,
		MaxConsecutiveLosses: 5,
	}

	pipeline, err := NewFullPipeline(config, provider)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	require.NoError(t, pipeline.Start(ctx))

	assert.GreaterOrEqual(t, int(provider.subscribeCount.Load()), 2)

	now := time.Now()
	provider.emitBook("BTC-USDT", events.OrderBookEvent{
		Symbol:    "BTC-USDT",
		Timestamp: now,
		Bids:      []events.OrderBookLevel{{Price: 99, Quantity: 4}, {Price: 98, Quantity: 18}, {Price: 97, Quantity: 4}},
		Asks:      []events.OrderBookLevel{{Price: 101, Quantity: 4}, {Price: 102, Quantity: 18}, {Price: 103, Quantity: 4}},
	})

	time.Sleep(200 * time.Millisecond)

	status := pipeline.Status()
	assert.Contains(t, status, "BTC-USDT_state")

	cancel()
	pipeline.Stop()
}

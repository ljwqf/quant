package ingestion

import (
	"errors"
	"testing"
	"time"

	"github.com/ljwqf/quant/internal/v2/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrderBookRebuilderLoadsSnapshotWithSortedDepth(t *testing.T) {
	rebuilder := NewOrderBookRebuilder("BTC-USDT", OrderBookRebuilderConfig{Depth: 2})
	now := time.Now()

	book, err := rebuilder.LoadSnapshot(events.OrderBookEvent{
		Symbol:    "BTC-USDT",
		Timestamp: now,
		Sequence:  10,
		Snapshot:  true,
		Bids: []events.OrderBookLevel{
			{Price: 99, Quantity: 3},
			{Price: 101, Quantity: 1},
			{Price: 100, Quantity: 2},
		},
		Asks: []events.OrderBookLevel{
			{Price: 104, Quantity: 3},
			{Price: 102, Quantity: 1},
			{Price: 103, Quantity: 2},
		},
	})

	require.NoError(t, err)
	assert.Equal(t, int64(10), book.Sequence)
	assert.Equal(t, []events.OrderBookLevel{{Price: 101, Quantity: 1}, {Price: 100, Quantity: 2}}, book.Bids)
	assert.Equal(t, []events.OrderBookLevel{{Price: 102, Quantity: 1}, {Price: 103, Quantity: 2}}, book.Asks)
	assert.Equal(t, OrderBookChecksum(book.Bids, book.Asks), book.Checksum)
}

func TestOrderBookRebuilderAppliesDeltaAndDeletesZeroQuantity(t *testing.T) {
	rebuilder := NewOrderBookRebuilder("BTC-USDT", OrderBookRebuilderConfig{Depth: 5})
	now := time.Now()

	_, err := rebuilder.LoadSnapshot(events.OrderBookEvent{
		Symbol:   "BTC-USDT",
		Sequence: 10,
		Snapshot: true,
		Bids:     []events.OrderBookLevel{{Price: 101, Quantity: 1}, {Price: 100, Quantity: 2}},
		Asks:     []events.OrderBookLevel{{Price: 102, Quantity: 1}, {Price: 103, Quantity: 2}},
	})
	require.NoError(t, err)

	book, err := rebuilder.ApplyDelta(events.OrderBookEvent{
		Symbol:    "BTC-USDT",
		Timestamp: now,
		Sequence:  11,
		Bids: []events.OrderBookLevel{
			{Price: 101, Quantity: 0},
			{Price: 99, Quantity: 4},
		},
		Asks: []events.OrderBookLevel{
			{Price: 102, Quantity: 5},
		},
	})

	require.NoError(t, err)
	assert.Equal(t, []events.OrderBookLevel{{Price: 100, Quantity: 2}, {Price: 99, Quantity: 4}}, book.Bids)
	assert.Equal(t, []events.OrderBookLevel{{Price: 102, Quantity: 5}, {Price: 103, Quantity: 2}}, book.Asks)
	assert.Equal(t, now, book.Timestamp)
}

func TestOrderBookRebuilderRejectsSequenceGap(t *testing.T) {
	rebuilder := NewOrderBookRebuilder("BTC-USDT", OrderBookRebuilderConfig{})

	_, err := rebuilder.LoadSnapshot(events.OrderBookEvent{
		Symbol:   "BTC-USDT",
		Sequence: 10,
		Snapshot: true,
		Bids:     []events.OrderBookLevel{{Price: 101, Quantity: 1}},
		Asks:     []events.OrderBookLevel{{Price: 102, Quantity: 1}},
	})
	require.NoError(t, err)

	_, err = rebuilder.ApplyDelta(events.OrderBookEvent{
		Symbol:   "BTC-USDT",
		Sequence: 12,
		Bids:     []events.OrderBookLevel{{Price: 100, Quantity: 2}},
	})

	assert.True(t, errors.Is(err, ErrOrderBookSequenceGap))
}

func TestOrderBookRebuilderRejectsChecksumMismatch(t *testing.T) {
	rebuilder := NewOrderBookRebuilder("BTC-USDT", OrderBookRebuilderConfig{})

	_, err := rebuilder.LoadSnapshot(events.OrderBookEvent{
		Symbol:   "BTC-USDT",
		Sequence: 10,
		Snapshot: true,
		Checksum: 123,
		Bids:     []events.OrderBookLevel{{Price: 101, Quantity: 1}},
		Asks:     []events.OrderBookLevel{{Price: 102, Quantity: 1}},
	})

	assert.True(t, errors.Is(err, ErrOrderBookChecksumMismatch))
}

func TestOrderBookRebuilderAcceptsMatchingChecksum(t *testing.T) {
	rebuilder := NewOrderBookRebuilder("BTC-USDT", OrderBookRebuilderConfig{})
	bids := []events.OrderBookLevel{{Price: 101, Quantity: 1}}
	asks := []events.OrderBookLevel{{Price: 102, Quantity: 1}}
	checksum := OrderBookChecksum(bids, asks)

	book, err := rebuilder.LoadSnapshot(events.OrderBookEvent{
		Symbol:   "BTC-USDT",
		Sequence: 10,
		Snapshot: true,
		Checksum: checksum,
		Bids:     bids,
		Asks:     asks,
	})

	require.NoError(t, err)
	assert.Equal(t, checksum, book.Checksum)
}

func TestOrderBookRebuilderTreatsSnapshotAsResync(t *testing.T) {
	rebuilder := NewOrderBookRebuilder("BTC-USDT", OrderBookRebuilderConfig{})

	_, err := rebuilder.LoadSnapshot(events.OrderBookEvent{
		Symbol:   "BTC-USDT",
		Sequence: 10,
		Snapshot: true,
		Bids:     []events.OrderBookLevel{{Price: 101, Quantity: 1}},
		Asks:     []events.OrderBookLevel{{Price: 102, Quantity: 1}},
	})
	require.NoError(t, err)

	book, err := rebuilder.Rebuild(events.OrderBookEvent{
		Symbol:   "BTC-USDT",
		Sequence: 20,
		Snapshot: true,
		Bids:     []events.OrderBookLevel{{Price: 99, Quantity: 2}},
		Asks:     []events.OrderBookLevel{{Price: 100, Quantity: 2}},
	})

	require.NoError(t, err)
	assert.Equal(t, int64(20), book.Sequence)
	assert.Equal(t, []events.OrderBookLevel{{Price: 99, Quantity: 2}}, book.Bids)
	assert.Equal(t, []events.OrderBookLevel{{Price: 100, Quantity: 2}}, book.Asks)
}

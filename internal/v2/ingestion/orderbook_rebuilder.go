package ingestion

import (
	"errors"
	"hash/crc32"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ljwqf/quant/internal/v2/events"
)

var (
	ErrOrderBookSequenceGap      = errors.New("order book sequence gap")
	ErrOrderBookChecksumMismatch = errors.New("order book checksum mismatch")
	ErrOrderBookEmpty            = errors.New("order book has no valid levels")
)

type OrderBookRebuilderConfig struct {
	Depth int
}

type OrderBookRebuilder struct {
	config      OrderBookRebuilderConfig
	symbol      string
	bids        map[float64]float64
	asks        map[float64]float64
	initialized bool
	sequence    int64
}

func NewOrderBookRebuilder(symbol string, config OrderBookRebuilderConfig) *OrderBookRebuilder {
	if config.Depth <= 0 {
		config.Depth = 25
	}
	return &OrderBookRebuilder{
		config: config,
		symbol: symbol,
		bids:   make(map[float64]float64),
		asks:   make(map[float64]float64),
	}
}

func (r *OrderBookRebuilder) Rebuild(evt events.OrderBookEvent) (events.OrderBookEvent, error) {
	if evt.Snapshot || !r.initialized || evt.Sequence <= 0 {
		return r.LoadSnapshot(evt)
	}
	return r.ApplyDelta(evt)
}

func (r *OrderBookRebuilder) LoadSnapshot(evt events.OrderBookEvent) (events.OrderBookEvent, error) {
	r.bids = make(map[float64]float64, len(evt.Bids))
	r.asks = make(map[float64]float64, len(evt.Asks))
	r.applyLevels(r.bids, evt.Bids)
	r.applyLevels(r.asks, evt.Asks)
	r.initialized = true
	r.sequence = evt.Sequence

	return r.snapshot(evt.Timestamp, evt.Checksum)
}

func (r *OrderBookRebuilder) ApplyDelta(evt events.OrderBookEvent) (events.OrderBookEvent, error) {
	if !r.initialized {
		return r.LoadSnapshot(evt)
	}
	if r.sequence > 0 && evt.Sequence != r.sequence+1 {
		r.initialized = false
		return events.OrderBookEvent{}, ErrOrderBookSequenceGap
	}

	r.applyLevels(r.bids, evt.Bids)
	r.applyLevels(r.asks, evt.Asks)
	r.sequence = evt.Sequence

	return r.snapshot(evt.Timestamp, evt.Checksum)
}

func (r *OrderBookRebuilder) applyLevels(book map[float64]float64, levels []events.OrderBookLevel) {
	for _, level := range levels {
		if level.Price <= 0 {
			continue
		}
		if level.Quantity <= 0 {
			delete(book, level.Price)
			continue
		}
		book[level.Price] = level.Quantity
	}
}

func (r *OrderBookRebuilder) snapshot(timestamp time.Time, expectedChecksum int64) (events.OrderBookEvent, error) {
	bids := mapToSortedLevels(r.bids, true, r.config.Depth)
	asks := mapToSortedLevels(r.asks, false, r.config.Depth)
	if len(bids) == 0 && len(asks) == 0 {
		return events.OrderBookEvent{}, ErrOrderBookEmpty
	}

	checksum := OrderBookChecksum(bids, asks)
	if expectedChecksum != 0 && expectedChecksum != checksum {
		r.initialized = false
		return events.OrderBookEvent{}, ErrOrderBookChecksumMismatch
	}

	return events.OrderBookEvent{
		Symbol:    r.symbol,
		Bids:      bids,
		Asks:      asks,
		Timestamp: timestamp,
		Sequence:  r.sequence,
		Checksum:  checksum,
		Snapshot:  true,
	}, nil
}

func mapToSortedLevels(book map[float64]float64, desc bool, depth int) []events.OrderBookLevel {
	levels := make([]events.OrderBookLevel, 0, len(book))
	for price, quantity := range book {
		if quantity <= 0 {
			continue
		}
		levels = append(levels, events.OrderBookLevel{Price: price, Quantity: quantity})
	}
	sort.Slice(levels, func(i, j int) bool {
		if desc {
			return levels[i].Price > levels[j].Price
		}
		return levels[i].Price < levels[j].Price
	})
	if depth > 0 && len(levels) > depth {
		levels = levels[:depth]
	}
	return levels
}

func OrderBookChecksum(bids []events.OrderBookLevel, asks []events.OrderBookLevel) int64 {
	limit := 25
	if len(bids) > limit {
		bids = bids[:limit]
	}
	if len(asks) > limit {
		asks = asks[:limit]
	}

	parts := make([]string, 0, (len(bids)+len(asks))*2)
	for i := 0; i < len(bids) || i < len(asks); i++ {
		if i < len(bids) {
			parts = append(parts, formatBookFloat(bids[i].Price), formatBookFloat(bids[i].Quantity))
		}
		if i < len(asks) {
			parts = append(parts, formatBookFloat(asks[i].Price), formatBookFloat(asks[i].Quantity))
		}
	}

	return int64(int32(crc32.ChecksumIEEE([]byte(strings.Join(parts, ":")))))
}

func formatBookFloat(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

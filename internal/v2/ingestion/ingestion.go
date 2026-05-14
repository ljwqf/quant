package ingestion

import (
	"context"

	"github.com/ljwqf/quant/internal/v2/events"
)

type DataProvider interface {
	SubscribeTicker(symbol string, handler func(*events.TickEvent)) error
	SubscribeOrderBook(symbol string, handler func(*events.OrderBookEvent)) error
	SubscribeKline(symbol string, interval string, handler func(*events.KlineEvent)) error
	GetAccount() (*AccountSnapshot, error)
	GetOrderBook(symbol string, depth int) (*events.OrderBookEvent, error)
}

type AccountSnapshot struct {
	TotalEquity     float64            `json:"total_equity"`
	TotalAvailable  float64            `json:"total_available"`
	TotalUnrealized float64            `json:"total_unrealized"`
	Positions       []PositionSnapshot `json:"positions"`
}

type PositionSnapshot struct {
	Symbol     string  `json:"symbol"`
	Side       string  `json:"side"`
	Size       float64 `json:"size"`
	EntryPrice float64 `json:"entry_price"`
	MarkPrice  float64 `json:"mark_price"`
}

type IngestionManager struct {
	provider   DataProvider
	symbols    []string
	intervals  []string
	ctx        context.Context
	tickChans  map[string]chan events.TickEvent
	bookChans  map[string]chan events.OrderBookEvent
	klineChans map[string]chan events.KlineEvent
	rebuilders map[string]*OrderBookRebuilder
}

func NewIngestionManager(provider DataProvider, symbols []string, intervals []string) *IngestionManager {
	return &IngestionManager{
		provider:   provider,
		symbols:    symbols,
		intervals:  intervals,
		tickChans:  make(map[string]chan events.TickEvent),
		bookChans:  make(map[string]chan events.OrderBookEvent),
		klineChans: make(map[string]chan events.KlineEvent),
		rebuilders: make(map[string]*OrderBookRebuilder),
	}
}

func (m *IngestionManager) TickChannel(symbol string) chan events.TickEvent {
	return m.tickChans[symbol]
}

func (m *IngestionManager) OrderBookChannel(symbol string) chan events.OrderBookEvent {
	return m.bookChans[symbol]
}

func (m *IngestionManager) KlineChannel(symbol string) chan events.KlineEvent {
	return m.klineChans[symbol]
}

func (m *IngestionManager) Start(ctx context.Context) error {
	m.ctx = ctx

	for _, symbol := range m.symbols {
		tickCh := make(chan events.TickEvent, 256)
		m.tickChans[symbol] = tickCh

		bookCh := make(chan events.OrderBookEvent, 64)
		m.bookChans[symbol] = bookCh
		rebuilder := NewOrderBookRebuilder(symbol, OrderBookRebuilderConfig{Depth: 25})
		m.rebuilders[symbol] = rebuilder

		klineCh := make(chan events.KlineEvent, 64)
		m.klineChans[symbol] = klineCh

		if err := m.provider.SubscribeTicker(symbol, func(evt *events.TickEvent) {
			select {
			case tickCh <- *evt:
			default:
			}
		}); err != nil {
			return err
		}

		if err := m.provider.SubscribeOrderBook(symbol, func(evt *events.OrderBookEvent) {
			book, err := rebuilder.Rebuild(*evt)
			if err != nil {
				return
			}
			select {
			case bookCh <- book:
			default:
			}
		}); err != nil {
			return err
		}

		for _, interval := range m.intervals {
			if err := m.provider.SubscribeKline(symbol, interval, func(evt *events.KlineEvent) {
				select {
				case klineCh <- *evt:
				default:
				}
			}); err != nil {
				return err
			}
		}
	}

	return nil
}

func (m *IngestionManager) Stop() {
	for _, ch := range m.tickChans {
		close(ch)
	}
	for _, ch := range m.bookChans {
		close(ch)
	}
	for _, ch := range m.klineChans {
		close(ch)
	}
}

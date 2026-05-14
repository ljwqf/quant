package adapter

import (
	"github.com/ljwqf/quant/internal/exchange"
	"github.com/ljwqf/quant/internal/v2/events"
	"github.com/ljwqf/quant/internal/v2/ingestion"
	"github.com/ljwqf/quant/pkg/types"
)

type V1ExchangeAdapter struct {
	exchange exchange.Exchange
}

func NewV1ExchangeAdapter(exchange exchange.Exchange) *V1ExchangeAdapter {
	return &V1ExchangeAdapter{exchange: exchange}
}

func (a *V1ExchangeAdapter) SubscribeTicker(symbol string, handler func(*events.TickEvent)) error {
	return a.exchange.SubscribeTicker(symbol, func(tick *types.Tick) {
		evt := convertTick(tick)
		handler(&evt)
	})
}

func (a *V1ExchangeAdapter) SubscribeOrderBook(symbol string, handler func(*events.OrderBookEvent)) error {
	return a.exchange.SubscribeOrderBook(symbol, func(book *types.OrderBook) {
		evt := convertOrderBook(book)
		handler(&evt)
	})
}

func (a *V1ExchangeAdapter) SubscribeKline(symbol string, interval string, handler func(*events.KlineEvent)) error {
	return a.exchange.SubscribeBar(symbol, interval, func(bar *types.Bar) {
		evt := convertKline(bar)
		handler(&evt)
	})
}

func (a *V1ExchangeAdapter) GetAccount() (*ingestion.AccountSnapshot, error) {
	account, err := a.exchange.GetAccount()
	if err != nil {
		return nil, err
	}
	snapshot := &ingestion.AccountSnapshot{
		TotalEquity:     account.TotalEquity,
		TotalAvailable:  account.TotalAvailable,
		TotalUnrealized: account.TotalUnrealizedPnL,
	}
	for _, pos := range account.Positions {
		snapshot.Positions = append(snapshot.Positions, ingestion.PositionSnapshot{
			Symbol:     pos.Symbol,
			Side:       string(pos.Side),
			Size:       pos.Size,
			EntryPrice: pos.EntryPrice,
			MarkPrice:  pos.MarkPrice,
		})
	}
	return snapshot, nil
}

func (a *V1ExchangeAdapter) GetOrderBook(symbol string, depth int) (*events.OrderBookEvent, error) {
	book, err := a.exchange.GetOrderBook(symbol, depth)
	if err != nil {
		return nil, err
	}
	evt := convertOrderBook(book)
	return &evt, nil
}

func convertTick(tick *types.Tick) events.TickEvent {
	direction := events.TradeDirectionUnknown
	if tick.Side == types.OrderSideBuy {
		direction = events.TradeDirectionBuy
	} else if tick.Side == types.OrderSideSell {
		direction = events.TradeDirectionSell
	}
	return events.TickEvent{
		Symbol:    tick.Symbol,
		Price:     tick.Price,
		Quantity:  tick.Size,
		Direction: direction,
		Timestamp: tick.Timestamp,
	}
}

func convertOrderBook(book *types.OrderBook) events.OrderBookEvent {
	bids := make([]events.OrderBookLevel, len(book.Bids))
	for i, level := range book.Bids {
		bids[i] = events.OrderBookLevel{Price: level.Price, Quantity: level.Size}
	}
	asks := make([]events.OrderBookLevel, len(book.Asks))
	for i, level := range book.Asks {
		asks[i] = events.OrderBookLevel{Price: level.Price, Quantity: level.Size}
	}
	return events.OrderBookEvent{
		Symbol:    book.Symbol,
		Bids:      bids,
		Asks:      asks,
		Timestamp: book.Timestamp,
		Checksum:  book.Checksum,
		Snapshot:  true,
	}
}

func convertKline(bar *types.Bar) events.KlineEvent {
	return events.KlineEvent{
		Symbol:    bar.Symbol,
		Open:      bar.Open,
		High:      bar.High,
		Low:       bar.Low,
		Close:     bar.Close,
		Volume:    bar.Volume,
		Interval:  bar.Interval,
		Timestamp: bar.Timestamp,
	}
}

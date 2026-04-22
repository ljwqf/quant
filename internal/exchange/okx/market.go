package okx

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/ljwqf/quant/pkg/logger"
	"github.com/ljwqf/quant/pkg/types"
	"go.uber.org/zap"
)

// normalizeBarInterval 将 K线周期转换为 OKX WebSocket 频道格式（分钟用大写 M）
func normalizeBarInterval(interval string) string {
	if before, found := strings.CutSuffix(interval, "m"); found {
		return before + "M"
	}
	return interval
}

// SubscribeTicker 订阅行情
func (c *Client) SubscribeTicker(symbol string, handler func(*types.Tick)) error {
	if !c.connected {
		if err := c.Connect(); err != nil {
			return err
		}
	}

	c.mutex.Lock()
	c.tickerHandlers[symbol] = append(c.tickerHandlers[symbol], handler)
	c.mutex.Unlock()

	return c.wsClient.subscribe("tickers", symbol, "")
}

// SubscribeBar 订阅K线
func (c *Client) SubscribeBar(symbol string, interval string, handler func(*types.Bar)) error {
	if !c.connected {
		if err := c.Connect(); err != nil {
			return err
		}
	}

	c.mutex.Lock()
	if c.barHandlers[symbol] == nil {
		c.barHandlers[symbol] = make(map[string][]func(*types.Bar))
	}
	c.barHandlers[symbol][interval] = append(c.barHandlers[symbol][interval], handler)
	c.mutex.Unlock()

	return c.wsClient.subscribe("candle"+normalizeBarInterval(interval), symbol, "")
}

// SubscribeOrderBook 订阅订单簿
func (c *Client) SubscribeOrderBook(symbol string, handler func(*types.OrderBook)) error {
	if !c.connected {
		if err := c.Connect(); err != nil {
			return err
		}
	}

	c.mutex.Lock()
	c.orderBookHandlers[symbol] = append(c.orderBookHandlers[symbol], handler)
	c.mutex.Unlock()

	return c.wsClient.subscribe("books5", symbol, "")
}

// handleWSMessage 处理 WebSocket 消息
func (c *Client) handleWSMessage(msg []byte) {
	// 解析消息
	var message struct {
		Op   string          `json:"op"`
		Arg  json.RawMessage `json:"arg,omitempty"`
		Data json.RawMessage `json:"data,omitempty"`
	}

	if err := json.Unmarshal(msg, &message); err != nil {
		logger.Error("解析 WebSocket 消息失败", zap.Error(err))
		return
	}

	// 处理不同类型的消息
	switch message.Op {
	case "pong":
		// 心跳响应，无需处理
	case "subscribe":
		// 订阅确认，无需处理
	case "unsubscribe":
		// 取消订阅确认，无需处理
	default:
		var arg wsArg
		if len(message.Arg) > 0 {
			if err := json.Unmarshal(message.Arg, &arg); err != nil {
				logger.Warn("解析 WebSocket arg 失败", zap.Error(err))
			}
		}
		// 市场数据消息
		c.handleMarketData(arg.Channel, message.Data)
	}
}

// handleMarketData 处理市场数据
func (c *Client) handleMarketData(channel string, data json.RawMessage) {
	if strings.HasPrefix(channel, "ticker") {
		var tickerData []struct {
			InstId  string `json:"instId"`
			Last    string `json:"last"`
			Open24h string `json:"open24h"`
			High24h string `json:"high24h"`
			Low24h  string `json:"low24h"`
			Vol24h  string `json:"vol24h"`
			Ts      string `json:"ts"`
		}
		if err := json.Unmarshal(data, &tickerData); err == nil && len(tickerData) > 0 {
			for i := range tickerData {
				item := tickerData[i]
				if item.InstId == "" {
					continue
				}
				c.handleTickerData(&item)
			}
			return
		}
	}

	if strings.HasPrefix(channel, "candle") {
		var candleData []struct {
			InstId string   `json:"instId"`
			Candle []string `json:"candle"`
			Bar    string   `json:"bar"`
		}

		if err := json.Unmarshal(data, &candleData); err == nil && len(candleData) > 0 {
			c.handleCandleData(candleData)
			return
		}
	}

	if strings.HasPrefix(channel, "books") {
		var bookData []struct {
			InstId   string     `json:"instId"`
			Asks     [][]string `json:"asks"`
			Bids     [][]string `json:"bids"`
			Ts       string     `json:"ts"`
			Checksum string     `json:"checksum"`
		}
		if err := json.Unmarshal(data, &bookData); err == nil && len(bookData) > 0 {
			for i := range bookData {
				item := bookData[i]
				if item.InstId == "" {
					continue
				}
				c.handleBookData(&item)
			}
			return
		}
	}

	// Fallback for older payloads without channel info.
	// 尝试解析为行情数据
	var tickerData struct {
		InstId  string `json:"instId"`
		Last    string `json:"last"`
		Open24h string `json:"open24h"`
		High24h string `json:"high24h"`
		Low24h  string `json:"low24h"`
		Vol24h  string `json:"vol24h"`
		Ts      string `json:"ts"`
	}

	if err := json.Unmarshal(data, &tickerData); err == nil && tickerData.InstId != "" {
		c.handleTickerData(&tickerData)
		return
	}

	// 尝试解析为K线数据
	var candleData []struct {
		InstId string   `json:"instId"`
		Candle []string `json:"candle"`
		Bar    string   `json:"bar"`
	}

	if err := json.Unmarshal(data, &candleData); err == nil && len(candleData) > 0 {
		c.handleCandleData(candleData)
		return
	}

	// 尝试解析为订单簿数据
	var bookData struct {
		InstId   string     `json:"instId"`
		Asks     [][]string `json:"asks"`
		Bids     [][]string `json:"bids"`
		Ts       string     `json:"ts"`
		Checksum string     `json:"checksum"`
	}

	if err := json.Unmarshal(data, &bookData); err == nil && bookData.InstId != "" {
		c.handleBookData(&bookData)
		return
	}
}

// handleTickerData 处理行情数据
func (c *Client) handleTickerData(data *struct {
	InstId  string `json:"instId"`
	Last    string `json:"last"`
	Open24h string `json:"open24h"`
	High24h string `json:"high24h"`
	Low24h  string `json:"low24h"`
	Vol24h  string `json:"vol24h"`
	Ts      string `json:"ts"`
}) {
	// 解析数据并检查错误
	lastPrice, err := parseFloat(data.Last)
	if err != nil {
		logger.Warn("解析 Last 价格失败", zap.String("symbol", data.InstId), zap.String("value", data.Last), zap.Error(err))
		return
	}
	open24h, err := parseFloat(data.Open24h)
	if err != nil {
		logger.Warn("解析 Open24h 价格失败", zap.String("symbol", data.InstId), zap.String("value", data.Open24h), zap.Error(err))
		return
	}
	high24h, err := parseFloat(data.High24h)
	if err != nil {
		logger.Warn("解析 High24h 价格失败", zap.String("symbol", data.InstId), zap.String("value", data.High24h), zap.Error(err))
		return
	}
	low24h, err := parseFloat(data.Low24h)
	if err != nil {
		logger.Warn("解析 Low24h 价格失败", zap.String("symbol", data.InstId), zap.String("value", data.Low24h), zap.Error(err))
		return
	}
	volume24h, err := parseFloat(data.Vol24h)
	if err != nil {
		logger.Warn("解析 Vol24h 价格失败", zap.String("symbol", data.InstId), zap.String("value", data.Vol24h), zap.Error(err))
		return
	}
	timestamp, err := parseInt(data.Ts)
	if err != nil {
		logger.Warn("解析 Ts 价格失败", zap.String("symbol", data.InstId), zap.String("value", data.Ts), zap.Error(err))
		return
	}

	t := time.Now()
	if timestamp > 0 {
		t = time.Unix(int64(timestamp)/1000, 0)
	}

	// 创建行情对象
	tick := &types.Tick{
		Symbol:    data.InstId,
		Price:     lastPrice,
		Timestamp: t,
		Open24h:   open24h,
		High24h:   high24h,
		Low24h:    low24h,
		Volume24h: volume24h,
	}

	// 调用回调函数
	c.mutex.RLock()
	handlers := c.tickerHandlers[data.InstId]
	c.mutex.RUnlock()

	for _, handler := range handlers {
		h := handler
		c.runHandler(func() {
			h(tick)
		})
	}
}

// handleCandleData 处理K线数据
func (c *Client) handleCandleData(data []struct {
	InstId string   `json:"instId"`
	Candle []string `json:"candle"`
	Bar    string   `json:"bar"`
}) {
	for _, item := range data {
		if len(item.Candle) < 6 {
			continue
		}

		// 解析数据并检查错误
		timestamp, err := parseInt(item.Candle[0])
		if err != nil {
			logger.Warn("解析 K线 timestamp 失败", zap.String("symbol", item.InstId), zap.String("value", item.Candle[0]), zap.Error(err))
			continue
		}
		open, err := parseFloat(item.Candle[1])
		if err != nil {
			logger.Warn("解析 K线 open 价格失败", zap.String("symbol", item.InstId), zap.String("value", item.Candle[1]), zap.Error(err))
			continue
		}
		high, err := parseFloat(item.Candle[2])
		if err != nil {
			logger.Warn("解析 K线 high 价格失败", zap.String("symbol", item.InstId), zap.String("value", item.Candle[2]), zap.Error(err))
			continue
		}
		low, err := parseFloat(item.Candle[3])
		if err != nil {
			logger.Warn("解析 K线 low 价格失败", zap.String("symbol", item.InstId), zap.String("value", item.Candle[3]), zap.Error(err))
			continue
		}
		close, err := parseFloat(item.Candle[4])
		if err != nil {
			logger.Warn("解析 K线 close 价格失败", zap.String("symbol", item.InstId), zap.String("value", item.Candle[4]), zap.Error(err))
			continue
		}
		volume, err := parseFloat(item.Candle[5])
		if err != nil {
			logger.Warn("解析 K线 volume 价格失败", zap.String("symbol", item.InstId), zap.String("value", item.Candle[5]), zap.Error(err))
			continue
		}

		t := time.Now()
		if timestamp > 0 {
			t = time.Unix(int64(timestamp)/1000, 0)
		}

		// 创建K线对象
		bar := &types.Bar{
			Symbol:    item.InstId,
			Open:      open,
			High:      high,
			Low:       low,
			Close:     close,
			Volume:    volume,
			Timestamp: t,
			Interval:  item.Bar,
		}

		// 调用回调函数
		c.mutex.RLock()
		symbolHandlers := c.barHandlers[item.InstId]
		c.mutex.RUnlock()

		if symbolHandlers != nil {
			handlers, ok := symbolHandlers[normalizeBarInterval(item.Bar)]
			if ok {
				for _, handler := range handlers {
					h := handler
					c.runHandler(func() {
						h(bar)
					})
				}
			}
		}
	}
}

// handleBookData 处理订单簿数据
func (c *Client) handleBookData(data *struct {
	InstId   string     `json:"instId"`
	Asks     [][]string `json:"asks"`
	Bids     [][]string `json:"bids"`
	Ts       string     `json:"ts"`
	Checksum string     `json:"checksum"`
}) {
	// 解析数据并检查错误
	timestamp, err := parseInt(data.Ts)
	if err != nil {
		logger.Warn("解析订单簿 Ts 失败", zap.String("symbol", data.InstId), zap.String("value", data.Ts), zap.Error(err))
		// 继续处理，使用当前时间作为 fallback
	}
	checksum, _ := parseInt(data.Checksum) // checksum 可选，不需要错误处理

	t := time.Now()
	if timestamp > 0 {
		t = time.Unix(int64(timestamp)/1000, 0)
	}

	// 创建订单簿对象
	orderBook := &types.OrderBook{
		Symbol:    data.InstId,
		Asks:      make([]types.OrderBookLevel, 0, len(data.Asks)),
		Bids:      make([]types.OrderBookLevel, 0, len(data.Bids)),
		Timestamp: t,
		Checksum:  int64(checksum),
	}

	// 解析卖单
	hasValidAsks := false
	for _, ask := range data.Asks {
		if len(ask) < 2 {
			continue
		}
		price, err := parseFloat(ask[0])
		if err != nil {
			logger.Warn("解析卖单价格失败", zap.String("symbol", data.InstId), zap.String("value", ask[0]), zap.Error(err))
			continue
		}
		size, err := parseFloat(ask[1])
		if err != nil {
			logger.Warn("解析卖单数量失败", zap.String("symbol", data.InstId), zap.String("value", ask[1]), zap.Error(err))
			continue
		}
		orderBook.Asks = append(orderBook.Asks, types.OrderBookLevel{
			Price: price,
			Size:  size,
		})
		hasValidAsks = true
	}

	// 解析买单
	hasValidBids := false
	for _, bid := range data.Bids {
		if len(bid) < 2 {
			continue
		}
		price, err := parseFloat(bid[0])
		if err != nil {
			logger.Warn("解析买单价格失败", zap.String("symbol", data.InstId), zap.String("value", bid[0]), zap.Error(err))
			continue
		}
		size, err := parseFloat(bid[1])
		if err != nil {
			logger.Warn("解析买单数量失败", zap.String("symbol", data.InstId), zap.String("value", bid[1]), zap.Error(err))
			continue
		}
		orderBook.Bids = append(orderBook.Bids, types.OrderBookLevel{
			Price: price,
			Size:  size,
		})
		hasValidBids = true
	}

	// 如果没有有效的价格数据，不要继续处理
	if !hasValidAsks && !hasValidBids {
		logger.Warn("订单簿没有有效的价格数据", zap.String("symbol", data.InstId))
		return
	}

	// 调用回调函数
	c.mutex.RLock()
	handlers := c.orderBookHandlers[data.InstId]
	c.mutex.RUnlock()

	for _, handler := range handlers {
		h := handler
		c.runHandler(func() {
			h(orderBook)
		})
	}
}

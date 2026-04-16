package okx

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/ljwqf/quant/internal/config"
	"github.com/ljwqf/quant/pkg/types"
	"golang.org/x/net/proxy"
)

// restClient REST API 客户端
type restClient struct {
	config     *config.OKXConfig
	httpClient *http.Client
}

// newRestClient 创建 REST 客户端
func newRestClient(cfg *config.OKXConfig) *restClient {
	httpClient := &http.Client{Timeout: 30 * time.Second}

	// 配置代理
	if cfg.ProxyURL != "" {
		transport := &http.Transport{}

		if cfg.ProxySkipVerify {
			transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
		}

		proxyParsed, err := url.Parse(cfg.ProxyURL)
		if err == nil {
			switch proxyParsed.Scheme {
			case "socks5":
				dialer, err := proxy.SOCKS5("tcp", proxyParsed.Host, nil, proxy.Direct)
				if err == nil {
					transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
						return dialer.Dial(network, addr)
					}
				}
			case "http", "https":
				transport.Proxy = http.ProxyURL(proxyParsed)
			}
		}

		httpClient.Transport = transport
	}

	return &restClient{
		config:     cfg,
		httpClient: httpClient,
	}
}

// sign 生成签名
func (r *restClient) sign(method, requestPath, body string) (string, string) {
	timestamp := time.Now().UTC().Format("2006-01-02T15:04:05.999Z07:00")
	signatureString := timestamp + method + requestPath + body

	h := hmac.New(sha256.New, []byte(r.config.SecretKey))
	h.Write([]byte(signatureString))
	signature := base64.StdEncoding.EncodeToString(h.Sum(nil))

	return timestamp, signature
}

// request 发送 HTTP 请求
func (r *restClient) request(method, endpoint string, params map[string]interface{}, data interface{}) ([]byte, error) {
	// 构建请求 URL
	baseURL, err := url.Parse(r.config.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("解析 BaseURL 失败: %w", err)
	}

	// 处理查询参数
	requestPath := "/api/v5" + endpoint
	if len(params) > 0 {
		values := url.Values{}
		for k, v := range params {
			values.Add(k, fmt.Sprintf("%v", v))
		}
		requestPath += "?" + values.Encode()
	}

	// 构建请求体
	var body []byte
	var bodyStr string
	if data != nil {
		body, err = json.Marshal(data)
		if err != nil {
			return nil, fmt.Errorf("序列化请求体失败: %w", err)
		}
		bodyStr = string(body)
	}

	// 生成签名
	timestamp, signature := r.sign(method, requestPath, bodyStr)

	// 创建请求
	req, err := http.NewRequest(method, baseURL.String()+requestPath, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("OK-ACCESS-KEY", r.config.APIKey)
	req.Header.Set("OK-ACCESS-SIGN", signature)
	req.Header.Set("OK-ACCESS-TIMESTAMP", timestamp)
	req.Header.Set("OK-ACCESS-PASSPHRASE", r.config.Passphrase)

	// 模拟盘模式
	if r.config.Simulated {
		req.Header.Set("x-simulated-trading", "1")
	}

	// 发送请求
	resp, err := r.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("发送请求失败: %w", err)
	}

	// 读取响应
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		if closeErr := resp.Body.Close(); closeErr != nil {
			return nil, fmt.Errorf("读取响应失败: %w (close body: %v)", err, closeErr)
		}
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	if closeErr := resp.Body.Close(); closeErr != nil {
		return nil, fmt.Errorf("关闭响应体失败: %w", closeErr)
	}

	// 检查响应状态
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API 请求失败 (状态码: %d): %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// getAccount 获取账户信息
func (r *restClient) getAccount() (*types.Account, error) {
	respBody, err := r.request("GET", "/account/balance", nil, nil)
	if err != nil {
		return nil, err
	}

	// 解析响应
	var response struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
		Data []struct {
			TotalEquity    string `json:"totalEq"`
			TotalMargin    string `json:"totalMargin"`
			TotalAvailable string `json:"availBal"`
			TotalPnL       string `json:"totalPnL"`
			UnrealizedPnL  string `json:"unrealizedPnL"`
			RealizedPnL    string `json:"realizedPnL"`
			Balances       []struct {
				Currency  string `json:"ccy"`
				Total     string `json:"eq"`
				Available string `json:"availBal"`
				Frozen    string `json:"frozenBal"`
			}
		}
	}

	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	if response.Code != "0" {
		return nil, fmt.Errorf("API 错误: %s - %s", response.Code, response.Msg)
	}

	// 构建账户信息
	if len(response.Data) == 0 {
		return nil, fmt.Errorf("账户数据为空")
	}

	account := &types.Account{
		Timestamp: time.Now(),
		Balance:   make(map[string]types.Balance),
	}

	// 解析余额数据
	for _, balance := range response.Data[0].Balances {
		total := parseFloatDefault(balance.Total, 0)
		available := parseFloatDefault(balance.Available, 0)
		frozen := parseFloatDefault(balance.Frozen, 0)

		account.Balance[balance.Currency] = types.Balance{
			Currency:  balance.Currency,
			Total:     total,
			Available: available,
			Frozen:    frozen,
			Timestamp: time.Now(),
		}
	}

	// 解析账户汇总数据
	account.TotalEquity = parseFloatDefault(response.Data[0].TotalEquity, 0)
	account.TotalMargin = parseFloatDefault(response.Data[0].TotalMargin, 0)
	account.TotalAvailable = parseFloatDefault(response.Data[0].TotalAvailable, 0)
	account.TotalPnL = parseFloatDefault(response.Data[0].TotalPnL, 0)
	account.TotalUnrealizedPnL = parseFloatDefault(response.Data[0].UnrealizedPnL, 0)
	account.TotalRealizedPnL = parseFloatDefault(response.Data[0].RealizedPnL, 0)

	// 获取持仓信息
	positions, err := r.getPositions()
	if err == nil {
		account.Positions = positions
	}

	return account, nil
}

// getPositions 获取持仓信息
func (r *restClient) getPositions() ([]*types.Position, error) {
	respBody, err := r.request("GET", "/position/list", nil, nil)
	if err != nil {
		return nil, err
	}

	// 解析响应
	var response struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
		Data []struct {
			Symbol           string `json:"instId"`
			Side             string `json:"posSide"`
			Size             string `json:"pos"`
			EntryPrice       string `json:"avgPx"`
			MarkPrice        string `json:"markPx"`
			UnrealizedPnL    string `json:"unrealizedPnL"`
			Leverage         string `json:"lever"`
			LiquidationPrice string `json:"liqPx"`
		}
	}

	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	if response.Code != "0" {
		return nil, fmt.Errorf("API 错误: %s - %s", response.Code, response.Msg)
	}

	// 构建持仓信息
	positions := make([]*types.Position, 0, len(response.Data))
	for _, data := range response.Data {
		size := parseFloatDefault(data.Size, 0)
		if size == 0 {
			continue // 跳过空持仓
		}

		entryPrice := parseFloatDefault(data.EntryPrice, 0)
		markPrice := parseFloatDefault(data.MarkPrice, 0)
		unrealizedPnL := parseFloatDefault(data.UnrealizedPnL, 0)
		leverage := parseIntDefault(data.Leverage, 1)
		liquidationPrice := parseFloatDefault(data.LiquidationPrice, 0)

		side := types.OrderSideBuy
		if data.Side == "short" {
			side = types.OrderSideSell
		}

		positions = append(positions, &types.Position{
			Symbol:           data.Symbol,
			Side:             side,
			Size:             size,
			EntryPrice:       entryPrice,
			MarkPrice:        markPrice,
			UnrealizedPnL:    unrealizedPnL,
			Leverage:         leverage,
			LiquidationPrice: liquidationPrice,
			Timestamp:        time.Now(),
		})
	}

	return positions, nil
}

// getTicker 获取最新行情
func (r *restClient) getTicker(symbol string) (*types.Tick, error) {
	params := map[string]interface{}{
		"instId": symbol,
	}

	respBody, err := r.request("GET", "/market/ticker", params, nil)
	if err != nil {
		return nil, err
	}

	// 解析响应
	var response struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
		Data []struct {
			Symbol    string `json:"instId"`
			LastPrice string `json:"last"`
			Open24h   string `json:"open24h"`
			High24h   string `json:"high24h"`
			Low24h    string `json:"low24h"`
			Volume24h string `json:"vol24h"`
		}
	}

	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	if response.Code != "0" {
		return nil, fmt.Errorf("API 错误: %s - %s", response.Code, response.Msg)
	}

	if len(response.Data) == 0 {
		return nil, fmt.Errorf("行情数据为空")
	}

	// 构建行情数据
	lastPrice := parseFloatDefault(response.Data[0].LastPrice, 0)
	open24h := parseFloatDefault(response.Data[0].Open24h, 0)
	high24h := parseFloatDefault(response.Data[0].High24h, 0)
	low24h := parseFloatDefault(response.Data[0].Low24h, 0)
	volume24h := parseFloatDefault(response.Data[0].Volume24h, 0)

	return &types.Tick{
		Symbol:    response.Data[0].Symbol,
		Price:     lastPrice,
		Timestamp: time.Now(),
		Open24h:   open24h,
		High24h:   high24h,
		Low24h:    low24h,
		Volume24h: volume24h,
	}, nil
}

// getBars 获取历史K线
func (r *restClient) getBars(symbol string, interval string, limit int) ([]*types.Bar, error) {
	params := map[string]interface{}{
		"instId": symbol,
		"bar":    interval,
		"limit":  limit,
	}

	respBody, err := r.request("GET", "/market/candles", params, nil)
	if err != nil {
		return nil, err
	}

	// 解析响应
	var response struct {
		Code string     `json:"code"`
		Msg  string     `json:"msg"`
		Data [][]string `json:"data"`
	}

	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	if response.Code != "0" {
		return nil, fmt.Errorf("API 错误: %s - %s", response.Code, response.Msg)
	}

	// 构建K线数据
	bars := make([]*types.Bar, 0, len(response.Data))
	for _, data := range response.Data {
		if len(data) < 6 {
			continue
		}

		timestamp := parseIntDefault(data[0], 0)
		open := parseFloatDefault(data[1], 0)
		high := parseFloatDefault(data[2], 0)
		low := parseFloatDefault(data[3], 0)
		close := parseFloatDefault(data[4], 0)
		volume := parseFloatDefault(data[5], 0)

		bars = append(bars, &types.Bar{
			Symbol:    symbol,
			Open:      open,
			High:      high,
			Low:       low,
			Close:     close,
			Volume:    volume,
			Timestamp: time.Unix(int64(timestamp)/1000, 0),
			Interval:  interval,
		})
	}

	return bars, nil
}

// getOrderBook 获取订单簿
func (r *restClient) getOrderBook(symbol string, depth int) (*types.OrderBook, error) {
	params := map[string]interface{}{
		"instId": symbol,
		"depth":  depth,
	}

	respBody, err := r.request("GET", "/market/books", params, nil)
	if err != nil {
		return nil, err
	}

	// 解析响应
	var response struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
		Data []struct {
			Symbol    string     `json:"instId"`
			Asks      [][]string `json:"asks"`
			Bids      [][]string `json:"bids"`
			Timestamp string     `json:"ts"`
			Checksum  string     `json:"checksum"`
		}
	}

	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	if response.Code != "0" {
		return nil, fmt.Errorf("API 错误: %s - %s", response.Code, response.Msg)
	}

	if len(response.Data) == 0 {
		return nil, fmt.Errorf("订单簿数据为空")
	}

	// 构建订单簿数据
	orderBook := &types.OrderBook{
		Symbol:    response.Data[0].Symbol,
		Asks:      make([]types.OrderBookLevel, 0, len(response.Data[0].Asks)),
		Bids:      make([]types.OrderBookLevel, 0, len(response.Data[0].Bids)),
		Timestamp: time.Now(),
	}

	// 解析卖单
	for _, ask := range response.Data[0].Asks {
		if len(ask) < 2 {
			continue
		}
		price := parseFloatDefault(ask[0], 0)
		size := parseFloatDefault(ask[1], 0)
		orderBook.Asks = append(orderBook.Asks, types.OrderBookLevel{
			Price: price,
			Size:  size,
		})
	}

	// 解析买单
	for _, bid := range response.Data[0].Bids {
		if len(bid) < 2 {
			continue
		}
		price := parseFloatDefault(bid[0], 0)
		size := parseFloatDefault(bid[1], 0)
		orderBook.Bids = append(orderBook.Bids, types.OrderBookLevel{
			Price: price,
			Size:  size,
		})
	}

	// 解析时间戳
	if ts, err := parseInt(response.Data[0].Timestamp); err == nil {
		orderBook.Timestamp = time.Unix(int64(ts)/1000, 0)
	}

	// 解析校验和
	if checksum, err := parseInt(response.Data[0].Checksum); err == nil {
		orderBook.Checksum = int64(checksum)
	}

	return orderBook, nil
}

// setLeverage 调整杠杆
func (r *restClient) setLeverage(symbol string, leverage int, marginMode string) error {
	if marginMode == "" {
		marginMode = "cross"
	}

	data := map[string]interface{}{
		"instId":  symbol,
		"lever":   leverage,
		"mgnMode": marginMode,
	}

	respBody, err := r.request("POST", "/account/set-leverage", nil, data)
	if err != nil {
		return err
	}

	var response struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
	}

	if err := json.Unmarshal(respBody, &response); err != nil {
		return fmt.Errorf("解析响应失败: %w", err)
	}

	if response.Code != "0" {
		return fmt.Errorf("API 错误: %s - %s", response.Code, response.Msg)
	}

	return nil
}

// getFundingRate 获取资金费率
func (r *restClient) getFundingRate(instId string) (*types.FundingRate, error) {
	params := map[string]interface{}{
		"instId": instId,
	}

	respBody, err := r.request("GET", "/public/funding-rate", params, nil)
	if err != nil {
		return nil, err
	}

	var response struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
		Data []struct {
			InstId         string `json:"instId"`
			FundingRate    string `json:"fundingRate"`
			NextFundingRate string `json:"nextFundingRate"`
			NextSettlementTime string `json:"nextFundingTime"`
			Ts             string `json:"ts"`
		}
	}

	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	if response.Code != "0" {
		return nil, fmt.Errorf("API 错误: %s - %s", response.Code, response.Msg)
	}

	if len(response.Data) == 0 {
		return nil, fmt.Errorf("资金费率数据为空")
	}

	fundingRate, _ := parseFloat(response.Data[0].FundingRate)
	nextFundingRate, _ := parseFloat(response.Data[0].NextFundingRate)
	ts, _ := parseInt(response.Data[0].Ts)
	t := time.Now()
	if ts > 0 {
		t = time.Unix(int64(ts)/1000, 0)
	}

	return &types.FundingRate{
		InstId:             response.Data[0].InstId,
		FundingRate:        fundingRate,
		NextFundingRate:    nextFundingRate,
		NextSettlementTime: t,
		Timestamp:          time.Now(),
	}, nil
}

// parseFloat 解析字符串为 float64
func parseFloat(s string) (float64, error) {
	var f float64
	_, err := fmt.Sscanf(s, "%f", &f)
	return f, err
}

// parseInt 解析字符串为 int
func parseInt(s string) (int, error) {
	var i int
	_, err := fmt.Sscanf(s, "%d", &i)
	return i, err
}

// parseFloatDefault 安全解析float64，失败时返回默认值
func parseFloatDefault(s string, defaultValue float64) float64 {
	f, err := parseFloat(s)
	if err != nil {
		return defaultValue
	}
	return f
}

// parseIntDefault 安全解析int，失败时返回默认值
func parseIntDefault(s string, defaultValue int) int {
	i, err := parseInt(s)
	if err != nil {
		return defaultValue
	}
	return i
}

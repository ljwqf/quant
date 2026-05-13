package okx

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/ljwqf/quant/pkg/logger"
	"github.com/ljwqf/quant/pkg/types"
	"go.uber.org/zap"
)

// PlaceOrder 下单
func (c *Client) PlaceOrder(order *types.Order) (*types.OrderResult, error) {
	if !c.connected {
		if err := c.Connect(); err != nil {
			return nil, err
		}
	}

	return c.restClient.placeOrder(order)
}

// CancelOrder 撤单
func (c *Client) CancelOrder(orderID string) error {
	if !c.connected {
		if err := c.Connect(); err != nil {
			return err
		}
	}

	return c.restClient.cancelOrder(orderID)
}

// GetOrder 获取订单信息
func (c *Client) GetOrder(orderID string) (*types.Order, error) {
	if !c.connected {
		if err := c.Connect(); err != nil {
			return nil, err
		}
	}

	// 从订阅中获取已知交易对，避免硬编码
	symbols := c.getKnownSymbols()
	return c.restClient.getOrder(orderID, symbols)
}

// GetOrders 获取订单列表
func (c *Client) GetOrders(symbol string, limit int) ([]*types.Order, error) {
	if !c.connected {
		if err := c.Connect(); err != nil {
			return nil, err
		}
	}

	return c.restClient.getOrders(symbol, limit)
}

// placeOrder 下单
func (r *restClient) placeOrder(order *types.Order) (*types.OrderResult, error) {
	// 构建下单请求
	tdMode := r.config.MarginMode
	if tdMode == "" {
		isSwapContract := strings.HasSuffix(order.Symbol, "-SWAP")
		if isSwapContract {
			tdMode = "cross" // 合约使用全仓模式 (需要账户设置为保证金模式)
		} else {
			tdMode = "cash" // 现货使用 cash 模式
		}
	}

	// OKX 合约下单 sz 必须为整数张数，现货可为小数
	isSwapContract := strings.HasSuffix(order.Symbol, "-SWAP")
	var szStr string
	if isSwapContract {
		contracts := int64(math.Round(order.Quantity))
		if contracts < 1 {
			contracts = 1 // 合约至少 1 张
		}
		szStr = strconv.FormatInt(contracts, 10)
	} else {
		szStr = strconv.FormatFloat(order.Quantity, 'f', -1, 64)
	}

	data := map[string]any{
		"instId":  order.Symbol,
		"tdMode":  tdMode,
		"side":    string(order.Side),
		"ordType": string(order.Type),
		"sz":      szStr,
	}

	// cross/isolated 模式需要 lever 参数（合约和现货 margin 都需要）
	if tdMode != "cash" {
		data["lever"] = strconv.Itoa(order.Leverage)
	}

	// 限价单需要价格
	if order.Type == types.OrderTypeLimit && order.Price > 0 {
		data["px"] = strconv.FormatFloat(order.Price, 'f', -1, 64)
	}

	// 添加客户端订单ID
	if order.ClientID != "" {
		data["clOrdId"] = order.ClientID
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	respBody, err := r.postRequestWithContext(ctx, "/trade/order", data)
	if err != nil {
		return nil, err
	}

	// 解析响应
	var response struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
		Data []struct {
			OrderID  string `json:"ordId"`
			ClientID string `json:"clOrdId"`
			Symbol   string `json:"instId"`
			Side     string `json:"side"`
			Type     string `json:"ordType"`
			Quantity string `json:"sz"`
			Price    string `json:"px"`
			Status   string `json:"ordStatus"`
		}
	}

	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	if response.Code != "0" {
		logger.Error("OKX 下单 API 返回错误",
			zap.String("rawResponse", string(respBody)),
			zap.String("code", response.Code),
			zap.String("msg", response.Msg),
		)
		return nil, fmt.Errorf("API 错误: %s - %s", response.Code, response.Msg)
	}

	if len(response.Data) == 0 {
		return nil, fmt.Errorf("订单数据为空")
	}

	// 构建订单结果
	quantity := parseFloatDefault(response.Data[0].Quantity, 0)
	price := parseFloatDefault(response.Data[0].Price, 0)

	status := types.OrderStatusPending
	switch response.Data[0].Status {
	case "filled":
		status = types.OrderStatusFilled
	case "partially_filled":
		status = types.OrderStatusPartially
	case "cancelled":
		status = types.OrderStatusCancelled
	case "rejected":
		status = types.OrderStatusFailed
	}

	side := types.OrderSideBuy
	if response.Data[0].Side == "sell" {
		side = types.OrderSideSell
	}

	orderType := types.OrderTypeLimit
	if response.Data[0].Type == "market" {
		orderType = types.OrderTypeMarket
	}

	return &types.OrderResult{
		OrderID:   response.Data[0].OrderID,
		ClientID:  response.Data[0].ClientID,
		Symbol:    response.Data[0].Symbol,
		Side:      side,
		Type:      orderType,
		Quantity:  quantity,
		Price:     price,
		Status:    status,
		Timestamp: time.Now(),
	}, nil
}

// cancelOrder 撤单
func (r *restClient) cancelOrder(orderID string) error {
	data := map[string]interface{}{
		"ordId": orderID,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	respBody, err := r.postRequestWithContext(ctx, "/trade/cancel-order", data)
	if err != nil {
		return err
	}

	// 解析响应
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

// getOrder 获取订单信息
func (r *restClient) getOrder(orderID string, knownSymbols []string) (*types.Order, error) {
	// 先尝试查询活跃订单（不需要 instId）
	params := map[string]interface{}{
		"ordId": orderID,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	respBody, err := r.requestWithContext(ctx, "GET", "/trade/order", params, nil)
	if err == nil {
		return r.parseOrderResponse(respBody)
	}

	// 活跃订单查询失败，尝试从历史订单中查找
	// 使用已知交易对（从订阅中提取），避免硬编码
	symbols := knownSymbols
	if len(symbols) == 0 {
		// 回退到常见交易对
		symbols = []string{"BTC-USDT-SWAP", "ETH-USDT-SWAP", "BTC-USDT", "ETH-USDT"}
	}
	for _, symbol := range symbols {
		historyParams := map[string]interface{}{
			"instId": symbol,
			"ordId":  orderID,
		}
		respBody, err := r.requestWithContext(ctx, "GET", "/trade/orders-history", historyParams, nil)
		if err == nil {
			return r.parseOrderResponse(respBody)
		}
	}

	return nil, fmt.Errorf("未找到订单: %s", orderID)
}

func (r *restClient) parseOrderResponse(respBody []byte) (*types.Order, error) {

	// 解析响应
	var response struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
		Data []struct {
			OrderID       string `json:"ordId"`
			ClientID      string `json:"clOrdId"`
			Symbol        string `json:"instId"`
			Side          string `json:"side"`
			Type          string `json:"ordType"`
			Quantity      string `json:"sz"`
			Price         string `json:"px"`
			AveragePrice  string `json:"avgPx"`
			FilledQty     string `json:"accFillSz"`
			Status        string `json:"ordStatus"`
			Leverage      string `json:"lever"`
			Timestamp     string `json:"cTime"`
		}
	}

	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	if response.Code != "0" {
		return nil, fmt.Errorf("API 错误: %s - %s", response.Code, response.Msg)
	}

	if len(response.Data) == 0 {
		return nil, fmt.Errorf("订单数据为空")
	}

	// 构建订单信息
	quantity, err := parseFloat(response.Data[0].Quantity)
	if err != nil {
		logger.Warn("解析订单数量失败", zap.String("orderId", response.Data[0].OrderID), zap.Error(err))
	}
	price, err := parseFloat(response.Data[0].Price)
	if err != nil {
		logger.Warn("解析订单价格失败", zap.String("orderId", response.Data[0].OrderID), zap.Error(err))
	}
	averagePrice, err := parseFloat(response.Data[0].AveragePrice)
	if err != nil {
		logger.Warn("解析订单均价失败", zap.String("orderId", response.Data[0].OrderID), zap.Error(err))
	}
	filledQty, err := parseFloat(response.Data[0].FilledQty)
	if err != nil {
		logger.Warn("解析已成交数量失败", zap.String("orderId", response.Data[0].OrderID), zap.Error(err))
	}
	leverage, err := parseInt(response.Data[0].Leverage)
	if err != nil {
		logger.Warn("解析杠杆失败", zap.String("orderId", response.Data[0].OrderID), zap.Error(err))
	}
	timestamp, err := parseInt(response.Data[0].Timestamp)
	if err != nil {
		logger.Warn("解析时间戳失败", zap.String("orderId", response.Data[0].OrderID), zap.Error(err))
	}

	status := types.OrderStatusPending
	switch response.Data[0].Status {
	case "filled":
		status = types.OrderStatusFilled
	case "partially_filled":
		status = types.OrderStatusPartially
	case "cancelled":
		status = types.OrderStatusCancelled
	case "rejected":
		status = types.OrderStatusFailed
	}

	side := types.OrderSideBuy
	if response.Data[0].Side == "sell" {
		side = types.OrderSideSell
	}

	orderType := types.OrderTypeLimit
	if response.Data[0].Type == "market" {
		orderType = types.OrderTypeMarket
	}

	t := time.Now()
	if timestamp > 0 {
		t = time.Unix(int64(timestamp)/1000, 0)
	}
	time := t

	return &types.Order{
		ID:           response.Data[0].OrderID,
		Symbol:       response.Data[0].Symbol,
		Side:         side,
		Type:         orderType,
		Quantity:     quantity,
		Price:        price,
		AveragePrice: averagePrice,
		FilledQty:    filledQty,
		Status:       status,
		Leverage:     leverage,
		Timestamp:    time,
		ClientID:     response.Data[0].ClientID,
	}, nil
}

// getOrders 获取订单列表
func (r *restClient) getOrders(symbol string, limit int) ([]*types.Order, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	params := map[string]interface{}{
		"instId": symbol,
		"limit":  limit,
	}

	respBody, err := r.requestWithContext(ctx, "GET", "/trade/orders-history", params, nil)
	if err != nil {
		return nil, err
	}

	// 解析响应
	var response struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
		Data []struct {
			OrderID       string `json:"ordId"`
			ClientID      string `json:"clOrdId"`
			Symbol        string `json:"instId"`
			Side          string `json:"side"`
			Type          string `json:"ordType"`
			Quantity      string `json:"sz"`
			Price         string `json:"px"`
			AveragePrice  string `json:"avgPx"`
			FilledQty     string `json:"accFillSz"`
			Status        string `json:"ordStatus"`
			Leverage      string `json:"lever"`
			Timestamp     string `json:"cTime"`
		}
	}

	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	if response.Code != "0" {
		return nil, fmt.Errorf("API 错误: %s - %s", response.Code, response.Msg)
	}

	// 构建订单列表
	orders := make([]*types.Order, 0, len(response.Data))
	for _, data := range response.Data {
		quantity, err := parseFloat(data.Quantity)
		if err != nil {
			logger.Warn("解析订单数量失败", zap.String("orderId", data.OrderID), zap.Error(err))
		}
		price, err := parseFloat(data.Price)
		if err != nil {
			logger.Warn("解析订单价格失败", zap.String("orderId", data.OrderID), zap.Error(err))
		}
		averagePrice, err := parseFloat(data.AveragePrice)
		if err != nil {
			logger.Warn("解析订单均价失败", zap.String("orderId", data.OrderID), zap.Error(err))
		}
		filledQty, err := parseFloat(data.FilledQty)
		if err != nil {
			logger.Warn("解析已成交数量失败", zap.String("orderId", data.OrderID), zap.Error(err))
		}
		leverage, err := parseInt(data.Leverage)
		if err != nil {
			logger.Warn("解析杠杆失败", zap.String("orderId", data.OrderID), zap.Error(err))
		}
		timestamp, err := parseInt(data.Timestamp)
		if err != nil {
			logger.Warn("解析时间戳失败", zap.String("orderId", data.OrderID), zap.Error(err))
		}

		status := types.OrderStatusPending
		switch data.Status {
		case "filled":
			status = types.OrderStatusFilled
		case "partially_filled":
			status = types.OrderStatusPartially
		case "cancelled":
			status = types.OrderStatusCancelled
		case "rejected":
			status = types.OrderStatusFailed
		}

		side := types.OrderSideBuy
		if data.Side == "sell" {
			side = types.OrderSideSell
		}

		orderType := types.OrderTypeLimit
		if data.Type == "market" {
			orderType = types.OrderTypeMarket
		}

		t := time.Now()
		if timestamp > 0 {
			t = time.Unix(int64(timestamp)/1000, 0)
		}
		time := t

		orders = append(orders, &types.Order{
			ID:           data.OrderID,
			Symbol:       data.Symbol,
			Side:         side,
			Type:         orderType,
			Quantity:     quantity,
			Price:        price,
			AveragePrice: averagePrice,
			FilledQty:    filledQty,
			Status:       status,
			Leverage:     leverage,
			Timestamp:    time,
			ClientID:     data.ClientID,
		})
	}

	return orders, nil
}

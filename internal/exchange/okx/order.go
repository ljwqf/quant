package okx

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/ljwqf/quant/pkg/types"
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

	return c.restClient.getOrder(orderID)
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
	data := map[string]interface{}{
		"instId": order.Symbol,
		"tdMode": "isolated", // 隔离保证金模式
		"side":   string(order.Side),
		"ordType": string(order.Type),
		"sz":     fmt.Sprintf("%f", order.Quantity),
		"lever":  fmt.Sprintf("%d", order.Leverage),
	}

	// 限价单需要价格
	if order.Type == types.OrderTypeLimit && order.Price > 0 {
		data["px"] = fmt.Sprintf("%f", order.Price)
	}

	// 添加客户端订单ID
	if order.ClientID != "" {
		data["clOrdId"] = order.ClientID
	}

	respBody, err := r.request("POST", "/trade/order", nil, data)
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

	respBody, err := r.request("POST", "/trade/cancel-order", nil, data)
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
func (r *restClient) getOrder(orderID string) (*types.Order, error) {
	params := map[string]interface{}{
		"ordId": orderID,
	}

	respBody, err := r.request("GET", "/trade/order", params, nil)
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

	if len(response.Data) == 0 {
		return nil, fmt.Errorf("订单数据为空")
	}

	// 构建订单信息
	quantity, _ := parseFloat(response.Data[0].Quantity)
	price, _ := parseFloat(response.Data[0].Price)
	averagePrice, _ := parseFloat(response.Data[0].AveragePrice)
	filledQty, _ := parseFloat(response.Data[0].FilledQty)
	leverage, _ := parseInt(response.Data[0].Leverage)
	timestamp, _ := parseInt(response.Data[0].Timestamp)

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
	params := map[string]interface{}{
		"instId": symbol,
		"limit":  limit,
	}

	respBody, err := r.request("GET", "/trade/orders-history", params, nil)
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
		quantity, _ := parseFloat(data.Quantity)
		price, _ := parseFloat(data.Price)
		averagePrice, _ := parseFloat(data.AveragePrice)
		filledQty, _ := parseFloat(data.FilledQty)
		leverage, _ := parseInt(data.Leverage)
		timestamp, _ := parseInt(data.Timestamp)

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

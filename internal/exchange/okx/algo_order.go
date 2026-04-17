package okx

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/ljwqf/quant/pkg/types"
)

// PlaceAlgoOrder 下发算法单（条件单/止盈止损）
func (c *Client) PlaceAlgoOrder(order *types.AlgoOrder) (*types.AlgoOrderResult, error) {
	if !c.connected {
		if err := c.Connect(); err != nil {
			return nil, err
		}
	}

	return c.restClient.placeAlgoOrder(order)
}

// CancelAlgoOrder 撤销算法单
func (c *Client) CancelAlgoOrder(algoID, symbol string) error {
	if !c.connected {
		if err := c.Connect(); err != nil {
			return err
		}
	}

	return c.restClient.cancelAlgoOrder(algoID, symbol)
}

// GetAlgoOrders 获取算法单列表
func (c *Client) GetAlgoOrders(symbol string, orderType string) ([]*types.AlgoOrder, error) {
	if !c.connected {
		if err := c.Connect(); err != nil {
			return nil, err
		}
	}

	return c.restClient.getAlgoOrders(symbol, orderType)
}

// placeAlgoOrder 下发算法单
func (r *restClient) placeAlgoOrder(order *types.AlgoOrder) (*types.AlgoOrderResult, error) {
	data := map[string]interface{}{
		"instId":  order.Symbol,
		"tdMode":  order.TdMode,
		"side":    string(order.Side),
		"ordType": string(order.OrdType),
		"sz":      strconv.FormatFloat(order.Size, 'f', -1, 64),
	}

	if order.SlTriggerPx > 0 {
		data["slTriggerPx"] = strconv.FormatFloat(order.SlTriggerPx, 'f', -1, 64)
		if order.SlOrderPx > 0 {
			data["slOrderPx"] = strconv.FormatFloat(order.SlOrderPx, 'f', -1, 64)
		} else {
			data["slOrderPx"] = "-1" // 市价止损
		}
	}

	if order.TpTriggerPx > 0 {
		data["tpTriggerPx"] = strconv.FormatFloat(order.TpTriggerPx, 'f', -1, 64)
		if order.TpOrderPx > 0 {
			data["tpOrderPx"] = strconv.FormatFloat(order.TpOrderPx, 'f', -1, 64)
		} else {
			data["tpOrderPx"] = "-1" // 市价止盈
		}
	}

	if order.CloseFraction > 0 && order.CloseFraction <= 1 {
		data["closeFraction"] = strconv.FormatFloat(order.CloseFraction, 'f', -1, 64)
	}

	if order.ClientID != "" {
		data["clOrdId"] = order.ClientID
	}

	respBody, err := r.request("POST", "/trade/order-algo", nil, data)
	if err != nil {
		return nil, err
	}

	var response struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
		Data []struct {
			AlgoID  string `json:"algoId"`
			Symbol  string `json:"instId"`
			ClientID string `json:"clOrdId"`
		}
	}

	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("解析算法单响应失败: %w", err)
	}

	if response.Code != "0" {
		return nil, fmt.Errorf("API 错误: %s - %s", response.Code, response.Msg)
	}

	if len(response.Data) == 0 {
		return nil, fmt.Errorf("算法单数据为空")
	}

	return &types.AlgoOrderResult{
		AlgoID:   response.Data[0].AlgoID,
		Symbol:   response.Data[0].Symbol,
		ClientID: response.Data[0].ClientID,
		Status:   "active",
		Timestamp: time.Now(),
	}, nil
}

// cancelAlgoOrder 撤销算法单
func (r *restClient) cancelAlgoOrder(algoID, symbol string) error {
	data := map[string]interface{}{
		"algoId": algoID,
		"instId": symbol,
	}

	respBody, err := r.request("POST", "/trade/cancel-algos", nil, data)
	if err != nil {
		return err
	}

	var response struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
	}

	if err := json.Unmarshal(respBody, &response); err != nil {
		return fmt.Errorf("解析算法单撤单响应失败: %w", err)
	}

	if response.Code != "0" {
		return fmt.Errorf("API 错误: %s - %s", response.Code, response.Msg)
	}

	return nil
}

// getAlgoOrders 获取算法单列表
func (r *restClient) getAlgoOrders(symbol string, orderType string) ([]*types.AlgoOrder, error) {
	body := map[string]interface{}{
		"instId":  symbol,
		"ordType": orderType,
	}

	respBody, err := r.postRequest("/trade/orders-algo-pending", body)
	if err != nil {
		return nil, err
	}

	var response struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
		Data []struct {
			AlgoID      string `json:"algoId"`
			Symbol      string `json:"instId"`
			Side        string `json:"side"`
			OrdType     string `json:"ordType"`
			Size        string `json:"sz"`
			SlTriggerPx string `json:"slTriggerPx"`
			SlOrderPx   string `json:"slOrderPx"`
			TpTriggerPx string `json:"tpTriggerPx"`
			TpOrderPx   string `json:"tpOrderPx"`
			State       string `json:"state"`
			ClientID    string `json:"clOrdId"`
		}
	}

	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("解析算法单列表响应失败: %w", err)
	}

	if response.Code != "0" {
		return nil, fmt.Errorf("API 错误: %s - %s", response.Code, response.Msg)
	}

	orders := make([]*types.AlgoOrder, 0, len(response.Data))
	for _, d := range response.Data {
		size, _ := strconv.ParseFloat(d.Size, 64)
		slTrigger, _ := strconv.ParseFloat(d.SlTriggerPx, 64)
		slOrder, _ := strconv.ParseFloat(d.SlOrderPx, 64)
		tpTrigger, _ := strconv.ParseFloat(d.TpTriggerPx, 64)
		tpOrder, _ := strconv.ParseFloat(d.TpOrderPx, 64)

		side := types.OrderSideBuy
		if d.Side == "sell" {
			side = types.OrderSideSell
		}

		orders = append(orders, &types.AlgoOrder{
			AlgoID:      d.AlgoID,
			Symbol:      d.Symbol,
			Side:        side,
			OrdType:     types.AlgoOrderType(d.OrdType),
			Size:        size,
			SlTriggerPx: slTrigger,
			SlOrderPx:   slOrder,
			TpTriggerPx: tpTrigger,
			TpOrderPx:   tpOrder,
			ClientID:    d.ClientID,
			State:       d.State,
		})
	}

	return orders, nil
}

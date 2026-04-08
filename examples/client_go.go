package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	defaultBaseURL = "http://localhost:8080"
	defaultTimeout = 30 * time.Second
)

type QuantClient struct {
	baseURL    string
	apiToken   string
	httpClient *http.Client
}

func NewQuantClient(baseURL, apiToken string) *QuantClient {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return &QuantClient{
		baseURL: baseURL,
		apiToken: apiToken,
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
	}
}

func (c *QuantClient) doRequest(method, path string, body interface{}) ([]byte, error) {
	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("序列化请求体失败: %w", err)
		}
		reqBody = bytes.NewReader(jsonData)
	}

	url := c.baseURL + path
	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.apiToken != "" {
		req.Header.Set("X-API-Token", c.apiToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("发送请求失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("请求失败: %s, 响应: %s", resp.Status, string(respBody))
	}

	return respBody, nil
}

func (c *QuantClient) GetHealth() (map[string]interface{}, error) {
	respBody, err := c.doRequest("GET", "/health", nil)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}
	return result, nil
}

func (c *QuantClient) GetStatus() (map[string]interface{}, error) {
	respBody, err := c.doRequest("GET", "/api/status", nil)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}
	return result, nil
}

func (c *QuantClient) GetStrategies() ([]map[string]interface{}, error) {
	respBody, err := c.doRequest("GET", "/api/strategies", nil)
	if err != nil {
		return nil, err
	}

	var result []map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}
	return result, nil
}

func (c *QuantClient) GetPositions() ([]map[string]interface{}, error) {
	respBody, err := c.doRequest("GET", "/api/positions", nil)
	if err != nil {
		return nil, err
	}

	var result []map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}
	return result, nil
}

func (c *QuantClient) GetOrders() ([]map[string]interface{}, error) {
	respBody, err := c.doRequest("GET", "/api/orders", nil)
	if err != nil {
		return nil, err
	}

	var result []map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}
	return result, nil
}

func (c *QuantClient) GetMetrics() (map[string]interface{}, error) {
	respBody, err := c.doRequest("GET", "/api/metrics", nil)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}
	return result, nil
}

func (c *QuantClient) StartStrategy(strategyName string) (map[string]interface{}, error) {
	path := fmt.Sprintf("/api/strategy/start/%s", strategyName)
	respBody, err := c.doRequest("POST", path, nil)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}
	return result, nil
}

func (c *QuantClient) StopStrategy(strategyName string) (map[string]interface{}, error) {
	path := fmt.Sprintf("/api/strategy/stop/%s", strategyName)
	respBody, err := c.doRequest("POST", path, nil)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}
	return result, nil
}

type CreateOrderRequest struct {
	Symbol string  `json:"symbol"`
	Side   string  `json:"side"`
	Type   string  `json:"type"`
	Price  float64 `json:"price"`
	Size   float64 `json:"size"`
}

func (c *QuantClient) CreateOrder(req CreateOrderRequest) (map[string]interface{}, error) {
	respBody, err := c.doRequest("POST", "/api/order/create", req)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}
	return result, nil
}

func (c *QuantClient) GetTicker(symbol string) (map[string]interface{}, error) {
	path := fmt.Sprintf("/api/market/ticker?symbol=%s", symbol)
	respBody, err := c.doRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}
	return result, nil
}

func (c *QuantClient) GetBars(symbol, interval string, limit int) ([]map[string]interface{}, error) {
	path := fmt.Sprintf("/api/market/bars?symbol=%s&interval=%s&limit=%d", symbol, interval, limit)
	respBody, err := c.doRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}

	var result []map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}
	return result, nil
}

func (c *QuantClient) GetBacktestStrategies() (map[string]interface{}, error) {
	respBody, err := c.doRequest("GET", "/api/backtest/strategies", nil)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}
	return result, nil
}

func main() {
	fmt.Println("============================================================")
	fmt.Println("OKX Quant Trading System - Go API Client Example")
	fmt.Println("============================================================")

	client := NewQuantClient(defaultBaseURL, "")

	fmt.Println("\n1. 检查服务健康状态...")
	health, err := client.GetHealth()
	if err != nil {
		fmt.Printf("   错误: %v\n", err)
		fmt.Println("   请确保服务已启动")
		return
	}
	fmt.Printf("   健康状态: %v\n", health)

	fmt.Println("\n2. 获取系统状态...")
	status, err := client.GetStatus()
	if err != nil {
		fmt.Printf("   错误: %v\n", err)
	} else {
		statusJSON, _ := json.MarshalIndent(status, "", "  ")
		fmt.Printf("   系统状态: %s\n", statusJSON)
	}

	fmt.Println("\n3. 获取策略列表...")
	strategies, err := client.GetStrategies()
	if err != nil {
		fmt.Printf("   错误: %v\n", err)
	} else {
		fmt.Printf("   策略数量: %d\n", len(strategies))
	}

	fmt.Println("\n4. 获取系统指标...")
	metrics, err := client.GetMetrics()
	if err != nil {
		fmt.Printf("   错误: %v\n", err)
	} else {
		keys := make([]string, 0, len(metrics))
		for k := range metrics {
			keys = append(keys, k)
		}
		fmt.Printf("   指标类型: %v\n", keys)
	}

	fmt.Println("\n5. 获取回测策略...")
	backtestStrategies, err := client.GetBacktestStrategies()
	if err != nil {
		fmt.Printf("   错误: %v\n", err)
	} else {
		btJSON, _ := json.MarshalIndent(backtestStrategies, "", "  ")
		fmt.Printf("   回测策略: %s\n", btJSON)
	}

	fmt.Println("\n============================================================")
	fmt.Println("示例完成!")
	fmt.Println("============================================================")
}

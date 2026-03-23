package cryptoquant

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/ljwqf/quant/pkg/logger"
	"go.uber.org/zap"
)

const (
	// APIBaseURL is the base URL for CryptoQuant API
	APIBaseURL = "https://api.cryptoquant.com/v1"
	
	// DefaultCacheDuration is the default cache duration
	DefaultCacheDuration = 1 * time.Hour
	
	// DefaultRequestInterval is the default interval between requests
	DefaultRequestInterval = 60 * time.Second
)

// Client is a CryptoQuant API client
type Client struct {
	apiKey           string
	client           *http.Client
	cache            map[string]cacheItem
	cacheMutex       sync.RWMutex
	lastRequestTime  time.Time
	requestMutex     sync.Mutex
	cacheDuration    time.Duration
	requestInterval  time.Duration
}

// cacheItem is a cached API response
type cacheItem struct {
	Data      interface{}
	Timestamp time.Time
}

// NewClient creates a new CryptoQuant API client
func NewClient(apiKey string) *Client {
	return &Client{
		apiKey:          apiKey,
		client:          &http.Client{Timeout: 10 * time.Second},
		cache:           make(map[string]cacheItem),
		cacheDuration:   DefaultCacheDuration,
		requestInterval: DefaultRequestInterval,
	}
}

// SetCacheDuration sets the cache duration
func (c *Client) SetCacheDuration(duration time.Duration) {
	c.cacheDuration = duration
}

// SetRequestInterval sets the request interval
func (c *Client) SetRequestInterval(interval time.Duration) {
	c.requestInterval = interval
}

// GetExchangeFlow gets exchange flow data
func (c *Client) GetExchangeFlow(asset string) (float64, error) {
	endpoint := fmt.Sprintf("/exchange/flow")
	params := map[string]string{
		"asset":     asset,
		"exchange":  "all",
		"window":    "1d",
		"limit":     "1",
	}

	result, err := c.get(endpoint, params)
	if err != nil {
		return 0, err
	}

	data, ok := result.(map[string]interface{})
	if !ok {
		return 0, fmt.Errorf("invalid response format")
	}

	resultArray, ok := data["result"].([]interface{})
	if !ok || len(resultArray) == 0 {
		return 0, fmt.Errorf("no result data")
	}

	item, ok := resultArray[0].(map[string]interface{})
	if !ok {
		return 0, fmt.Errorf("invalid result item")
	}

	netFlow, ok := item["net_flow"].(float64)
	if !ok {
		// Try to convert from string
		if netFlowStr, ok := item["net_flow"].(string); ok {
			if _, err := fmt.Sscanf(netFlowStr, "%f", &netFlow); err != nil {
				return 0, fmt.Errorf("invalid net_flow value: %s", netFlowStr)
			}
		} else {
			return 0, fmt.Errorf("invalid net_flow type: %T", item["net_flow"])
		}
	}

	return netFlow, nil
}

// GetSOPR gets SOPR (Spent Output Profit Ratio) data
func (c *Client) GetSOPR(asset string) (float64, error) {
	endpoint := fmt.Sprintf("/onchain/sopr")
	params := map[string]string{
		"asset":     asset,
		"window":    "1d",
		"limit":     "1",
	}

	result, err := c.get(endpoint, params)
	if err != nil {
		return 0, err
	}

	data, ok := result.(map[string]interface{})
	if !ok {
		return 0, fmt.Errorf("invalid response format")
	}

	resultArray, ok := data["result"].([]interface{})
	if !ok || len(resultArray) == 0 {
		return 0, fmt.Errorf("no result data")
	}

	item, ok := resultArray[0].(map[string]interface{})
	if !ok {
		return 0, fmt.Errorf("invalid result item")
	}

	sopr, ok := item["sopr"].(float64)
	if !ok {
		// Try to convert from string
		if soprStr, ok := item["sopr"].(string); ok {
			if _, err := fmt.Sscanf(soprStr, "%f", &sopr); err != nil {
				return 0, fmt.Errorf("invalid sopr value: %s", soprStr)
			}
		} else {
			return 0, fmt.Errorf("invalid sopr type: %T", item["sopr"])
		}
	}

	return sopr, nil
}

// GetMVRV gets MVRV (Market Value to Realized Value) data
func (c *Client) GetMVRV(asset string) (float64, error) {
	endpoint := fmt.Sprintf("/onchain/mvrv")
	params := map[string]string{
		"asset":     asset,
		"window":    "1d",
		"limit":     "1",
	}

	result, err := c.get(endpoint, params)
	if err != nil {
		return 0, err
	}

	data, ok := result.(map[string]interface{})
	if !ok {
		return 0, fmt.Errorf("invalid response format")
	}

	resultArray, ok := data["result"].([]interface{})
	if !ok || len(resultArray) == 0 {
		return 0, fmt.Errorf("no result data")
	}

	item, ok := resultArray[0].(map[string]interface{})
	if !ok {
		return 0, fmt.Errorf("invalid result item")
	}

	mvrv, ok := item["mvrv"].(float64)
	if !ok {
		// Try to convert from string
		if mvrvStr, ok := item["mvrv"].(string); ok {
			if _, err := fmt.Sscanf(mvrvStr, "%f", &mvrv); err != nil {
				return 0, fmt.Errorf("invalid mvrv value: %s", mvrvStr)
			}
		} else {
			return 0, fmt.Errorf("invalid mvrv type: %T", item["mvrv"])
		}
	}

	return mvrv, nil
}

// GetOnChainData gets all on-chain data
func (c *Client) GetOnChainData(asset string) (float64, float64, float64, error) {
	netFlow, err := c.GetExchangeFlow(asset)
	if err != nil {
		logger.Warn("Failed to get exchange flow", zap.Error(err))
		netFlow = -6000.0 // Default value
	}

	sopr, err := c.GetSOPR(asset)
	if err != nil {
		logger.Warn("Failed to get SOPR", zap.Error(err))
		sopr = 0.94 // Default value
	}

	mvrv, err := c.GetMVRV(asset)
	if err != nil {
		logger.Warn("Failed to get MVRV", zap.Error(err))
		mvrv = 0.95 // Default value
	}

	return netFlow, sopr, mvrv, nil
}

// get makes an API request with caching and rate limiting
func (c *Client) get(endpoint string, params map[string]string) (interface{}, error) {
	// Generate cache key
	cacheKey := c.generateCacheKey(endpoint, params)

	// Check cache
	if cached := c.getFromCache(cacheKey); cached != nil {
		logger.Debug("Using cached data", zap.String("endpoint", endpoint))
		return cached, nil
	}

	// Rate limiting
	c.rateLimit()

	// Build URL
	url := APIBaseURL + endpoint
	if len(params) > 0 {
		url += "?"
		first := true
		for k, v := range params {
			if !first {
				url += "&"
			}
			url += k + "=" + v
			first = false
		}
	}

	// Create request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	// Set headers
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	// Send request
	logger.Debug("Making API request", zap.String("url", url))
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("API error: status=%d, body=%s", resp.StatusCode, string(body))
	}

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Parse response
	var result interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	// Cache result
	c.cacheResult(cacheKey, result)

	return result, nil
}

// generateCacheKey generates a cache key for the given endpoint and params
func (c *Client) generateCacheKey(endpoint string, params map[string]string) string {
	// Simple cache key generation
	return endpoint + "_" + fmt.Sprintf("%v", params)
}

// getFromCache gets data from cache
func (c *Client) getFromCache(key string) interface{} {
	c.cacheMutex.RLock()
	defer c.cacheMutex.RUnlock()

	item, ok := c.cache[key]
	if !ok {
		return nil
	}

	if time.Since(item.Timestamp) > c.cacheDuration {
		// Cache expired
		return nil
	}

	return item.Data
}

// cacheResult caches the result
func (c *Client) cacheResult(key string, data interface{}) {
	c.cacheMutex.Lock()
	defer c.cacheMutex.Unlock()

	c.cache[key] = cacheItem{
		Data:      data,
		Timestamp: time.Now(),
	}
}

// rateLimit enforces rate limiting
func (c *Client) rateLimit() {
	c.requestMutex.Lock()
	defer c.requestMutex.Unlock()

	elapsed := time.Since(c.lastRequestTime)
	if elapsed < c.requestInterval {
		wait := c.requestInterval - elapsed
		logger.Debug("Rate limiting", zap.Duration("wait", wait))
		time.Sleep(wait)
	}

	c.lastRequestTime = time.Now()
}

// NewClientFromEnv creates a new CryptoQuant client from environment variables
func NewClientFromEnv() *Client {
	apiKey := os.Getenv("CRYPTOQUANT_API_KEY")
	if apiKey == "" {
		logger.Warn("CRYPTOQUANT_API_KEY not set")
	}

	return NewClient(apiKey)
}

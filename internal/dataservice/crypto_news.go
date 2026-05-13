package dataservice

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/ljwqf/quant/internal/storage"
	"github.com/ljwqf/quant/pkg/logger"
	"go.uber.org/zap"
)

// CryptoNewsConfig 加密货币新闻配置
type CryptoNewsConfig struct {
	APIKey     string
	BaseURL    string
	Categories []string
	Limit      int
	Timeout    time.Duration
}

// CryptoNewsClient 加密货币新闻客户端
type CryptoNewsClient struct {
	config     *CryptoNewsConfig
	client     *http.Client
	lastFetch  time.Time
	cache      []*storage.NewsEvent
	cacheTTL   time.Duration
	mu         sync.RWMutex
}

// DefaultCryptoNewsConfig 默认新闻配置
func DefaultCryptoNewsConfig() *CryptoNewsConfig {
	return &CryptoNewsConfig{
		BaseURL:    "https://min-api.cryptocompare.com/data/v2/news/",
		Categories: []string{"BTC", "ETH", "DeFi", "Regulation"},
		Limit:      20,
		Timeout:    15 * time.Second,
	}
}

// NewCryptoNewsClient 创建新闻客户端
func NewCryptoNewsClient(cfg *CryptoNewsConfig) *CryptoNewsClient {
	if cfg == nil {
		cfg = DefaultCryptoNewsConfig()
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 15 * time.Second
	}
	if cfg.Limit <= 0 {
		cfg.Limit = 20
	}

	return &CryptoNewsClient{
		config:   cfg,
		client:   &http.Client{Timeout: cfg.Timeout},
		cacheTTL: 5 * time.Minute,
	}
}

// FetchNews 获取新闻数据
func (c *CryptoNewsClient) FetchNews() ([]*storage.NewsEvent, error) {
	c.mu.RLock()
	if len(c.cache) > 0 && time.Since(c.lastFetch) < c.cacheTTL {
		cached := make([]*storage.NewsEvent, len(c.cache))
		copy(cached, c.cache)
		c.mu.RUnlock()
		return cached, nil
	}
	c.mu.RUnlock()

	news, err := c.fetchFromAPI()
	if err != nil {
		logger.Warn("从CryptoCompare获取新闻失败", zap.Error(err))
		// 返回缓存（如果有）
		c.mu.RLock()
		cached := c.cache
		c.mu.RUnlock()
		return cached, err
	}

	c.mu.Lock()
	c.cache = news
	c.lastFetch = time.Now()
	c.mu.Unlock()

	return news, nil
}

type cryptoCompareNewsResponse struct {
	Response string                        `json:"Response"`
	Data     []cryptoCompareNewsArticle    `json:"Data"`
}

type cryptoCompareNewsArticle struct {
	ID            int    `json:"id"`
	Title         string `json:"title"`
	Body          string `json:"body"`
	URL           string `json:"url"`
	Source        string `json:"source"`
	PublishedOn   int64  `json:"published_on"`
	Categories    string `json:"categories"`
	Upvotes       int    `json:"upvotes"`
	Downvotes     int    `json:"downvotes"`
	Images        string `json:"images"`
	Importance    int    `json:"importance"`
}

func (c *CryptoNewsClient) fetchFromAPI() ([]*storage.NewsEvent, error) {
	params := url.Values{}
	params.Set("limit", fmt.Sprintf("%d", c.config.Limit))
	params.Set("sortOrder", "latest")

	if len(c.config.Categories) > 0 {
		params.Set("categories", strings.Join(c.config.Categories, ","))
	}
	if c.config.APIKey != "" {
		params.Set("api_key", c.config.APIKey)
	}

	apiURL := c.config.BaseURL + "?" + params.Encode()

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP错误: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	var apiResp cryptoCompareNewsResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("解析JSON失败: %w", err)
	}

	events := make([]*storage.NewsEvent, 0, len(apiResp.Data))
	for _, article := range apiResp.Data {
		event := &storage.NewsEvent{
			ExternalID:     fmt.Sprintf("cc-%d", article.ID),
			Title:          article.Title,
			Summary:        truncateString(article.Body, 200),
			Content:        article.Body,
			Source:         article.Source,
			URL:            article.URL,
			Category:       article.Categories,
			Importance:     mapImportance(article.Importance),
			RelatedSymbols: extractSymbols(article.Categories),
			PublishedAt:    time.Unix(article.PublishedOn, 0),
			CreatedAt:      time.Now(),
		}
		events = append(events, event)
	}

	logger.Info("从CryptoCompare获取新闻成功",
		zap.Int("count", len(events)),
		zap.String("categories", strings.Join(c.config.Categories, ",")))

	return events, nil
}

// mapImportance 将CryptoCompare的重要性映射到系统标准(1-5)
func mapImportance(importance int) int {
	switch {
	case importance >= 80:
		return 5
	case importance >= 60:
		return 4
	case importance >= 40:
		return 3
	case importance >= 20:
		return 2
	default:
		return 1
	}
}

// extractSymbols 从分类中提取相关币种
func extractSymbols(categories string) string {
	symbols := []string{}
	upper := strings.ToUpper(categories)
	if strings.Contains(upper, "BTC") || strings.Contains(upper, "BITCOIN") {
		symbols = append(symbols, "BTC-USDT")
	}
	if strings.Contains(upper, "ETH") || strings.Contains(upper, "ETHEREUM") {
		symbols = append(symbols, "ETH-USDT")
	}
	if strings.Contains(upper, "SOL") {
		symbols = append(symbols, "SOL-USDT")
	}
	if strings.Contains(upper, "XRP") {
		symbols = append(symbols, "XRP-USDT")
	}
	if strings.Contains(upper, "DEFI") {
		symbols = append(symbols, "BTC-USDT", "ETH-USDT")
	}
	if len(symbols) == 0 {
		return "BTC-USDT"
	}
	return strings.Join(symbols, ",")
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

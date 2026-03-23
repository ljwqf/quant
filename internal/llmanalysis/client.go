package llmanalysis

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/ljwqf/quant/internal/config"
	"github.com/ljwqf/quant/internal/llmanalysis/providers"
	"github.com/ljwqf/quant/pkg/logger"
	"go.uber.org/zap"
)

// CacheEntry 缓存条目
type CacheEntry struct {
	Response  *providers.ChatResponse
	Timestamp time.Time
}

// Client 大模型 API 客户端
type Client struct {
	cfg           *config.LLMConfig
	provider      providers.Provider
	cache         map[string]CacheEntry
	cacheMutex    sync.RWMutex
	cacheDuration time.Duration
	maxRetries    int
	retryDelay    time.Duration
}

// NewClient 创建大模型客户端
func NewClient(cfg *config.LLMConfig) *Client {
	if !cfg.Enable {
		logger.Info("大模型分析功能未启用")
		return nil
	}

	c := &Client{
		cfg:           cfg,
		cache:         make(map[string]CacheEntry),
		cacheDuration: 5 * time.Minute,
		maxRetries:    3,
		retryDelay:    1 * time.Second,
	}

	if err := c.initProvider(); err != nil {
		logger.Error("初始化大模型提供商失败", zap.Error(err))
		return nil
	}

	logger.Info("大模型客户端初始化成功", zap.String("provider", cfg.Provider))
	return c
}

// initProvider 初始化提供商
func (c *Client) initProvider() error {
	var providerConfig config.LLMProviderConfig

	switch c.cfg.Provider {
	case "openai":
		c.provider = providers.NewOpenAIProvider()
		providerConfig = c.cfg.Providers.OpenAI
	case "claude":
		c.provider = providers.NewClaudeProvider()
		providerConfig = c.cfg.Providers.Claude
	case "qwen":
		c.provider = providers.NewQwenProvider()
		providerConfig = c.cfg.Providers.Qwen
	default:
		return fmt.Errorf("不支持的大模型提供商: %s", c.cfg.Provider)
	}

	if providerConfig.APIKey != "" {
		c.provider.SetAPIKey(providerConfig.APIKey)
	}
	if providerConfig.BaseURL != "" {
		c.provider.SetBaseURL(providerConfig.BaseURL)
	}
	if c.cfg.Timeout > 0 {
		c.provider.SetTimeout(c.cfg.Timeout)
	}

	return nil
}

// Chat 发送聊天请求（带重试和缓存）
func (c *Client) Chat(ctx context.Context, req *providers.ChatRequest) (*providers.ChatResponse, error) {
	if c.provider == nil {
		return nil, fmt.Errorf("大模型提供商未初始化")
	}

	cacheKey := c.generateCacheKey(req)
	
	if cached := c.getFromCache(cacheKey); cached != nil {
		logger.Debug("使用缓存的大模型响应", zap.String("cache_key", cacheKey))
		return cached, nil
	}

	var lastErr error
	for i := 0; i < c.maxRetries; i++ {
		resp, err := c.provider.Chat(ctx, req)
		if err == nil {
			c.setToCache(cacheKey, resp)
			return resp, nil
		}

		lastErr = err
		logger.Warn("大模型请求失败，准备重试", 
			zap.Int("attempt", i+1), 
			zap.Int("max_retries", c.maxRetries),
			zap.Error(err))

		if i < c.maxRetries-1 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(c.retryDelay * time.Duration(i+1)):
			}
		}
	}

	return nil, fmt.Errorf("大模型请求失败，已重试 %d 次: %w", c.maxRetries, lastErr)
}

// generateCacheKey 生成缓存键
func (c *Client) generateCacheKey(req *providers.ChatRequest) string {
	data, err := json.Marshal(req)
	if err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// getFromCache 从缓存获取
func (c *Client) getFromCache(key string) *providers.ChatResponse {
	c.cacheMutex.RLock()
	defer c.cacheMutex.RUnlock()

	entry, exists := c.cache[key]
	if !exists {
		return nil
	}

	if time.Since(entry.Timestamp) > c.cacheDuration {
		return nil
	}

	return entry.Response
}

// setToCache 设置缓存
func (c *Client) setToCache(key string, resp *providers.ChatResponse) {
	c.cacheMutex.Lock()
	defer c.cacheMutex.Unlock()

	c.cache[key] = CacheEntry{
		Response:  resp,
		Timestamp: time.Now(),
	}
}

// ClearCache 清除缓存
func (c *Client) ClearCache() {
	c.cacheMutex.Lock()
	defer c.cacheMutex.Unlock()
	c.cache = make(map[string]CacheEntry)
	logger.Info("大模型缓存已清除")
}

// Provider 获取当前提供商
func (c *Client) Provider() providers.Provider {
	return c.provider
}

package llmanalysis

import (
	"context"
	"testing"
	"time"

	"github.com/ljwqf/quant/internal/config"
	"github.com/ljwqf/quant/internal/llmanalysis/providers"
	"github.com/ljwqf/quant/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnalyzerNilClientMethods(t *testing.T) {
	a := NewAnalyzer(nil, nil, nil)
	assert.Nil(t, a)
}

func TestAnalyzerAnalyzeTradeNilClient(t *testing.T) {
	a := &Analyzer{client: nil}
	result, err := a.AnalyzeTrade(context.TODO(), &TradeDecisionData{Symbol: "BTC-USDT"})
	assert.Nil(t, result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "未初始化")
}

func TestAnalyzerAnalyzePositionNilClient(t *testing.T) {
	a := &Analyzer{client: nil}
	result, err := a.AnalyzePosition(context.TODO(), "BTC-USDT", nil)
	assert.Nil(t, result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "未初始化")
}

func TestAnalyzerAnalyzeMarketNilClient(t *testing.T) {
	a := &Analyzer{client: nil}
	result, err := a.AnalyzeMarket(context.TODO(), []string{"BTC-USDT"})
	assert.Nil(t, result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "未初始化")
}

func TestAnalyzerAnalyzeOrdersNilClient(t *testing.T) {
	a := &Analyzer{client: nil}
	result, err := a.AnalyzeOrders(context.TODO(), &OrderData{Symbol: "BTC-USDT"})
	assert.Nil(t, result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "未初始化")
}

func TestAnalyzerGetLatestAnalysisNoopRepo(t *testing.T) {
	a := &Analyzer{
		client: nil,
		aiRepo: &noopAIAnalysisRepository{},
	}
	result, err := a.GetLatestAnalysis("BTC-USDT", AnalysisTypeTrade)
	assert.Nil(t, result)
	assert.Nil(t, err)
}

func TestAnalyzerListAnalysesNoopRepo(t *testing.T) {
	a := &Analyzer{
		client: nil,
		aiRepo: &noopAIAnalysisRepository{},
	}
	results, err := a.ListAnalyses("BTC-USDT", 10)
	assert.NoError(t, err)
	assert.Empty(t, results)
}

func TestConvertMessages(t *testing.T) {
	msgs := []Message{
		{Role: "system", Content: "sys"},
		{Role: "user", Content: "usr"},
		{Role: "assistant", Content: "asst"},
	}
	converted := convertMessages(msgs)
	require.Len(t, converted, 3)
	assert.Equal(t, "system", converted[0].Role)
	assert.Equal(t, "sys", converted[0].Content)
	assert.Equal(t, "user", converted[1].Role)
	assert.Equal(t, "usr", converted[1].Content)
	assert.Equal(t, "assistant", converted[2].Role)
	assert.Equal(t, "asst", converted[2].Content)
}

func TestNoopRepositoryMethods(t *testing.T) {
	r := &noopAIAnalysisRepository{}

	err := r.Create(nil)
	assert.NoError(t, err)

	rec, err := r.GetByID(1)
	assert.Nil(t, rec)
	assert.NoError(t, err)

	list, err := r.ListBySymbol("BTC", 10, 0)
	assert.NoError(t, err)
	assert.NotNil(t, list)

	list, err = r.ListByType("trade", 10, 0)
	assert.NoError(t, err)
	assert.NotNil(t, list)

	latest, err := r.GetLatestBySymbolAndType("BTC", "trade")
	assert.Nil(t, latest)
	assert.NoError(t, err)
}

func TestShouldAlertEmptyRiskLevel(t *testing.T) {
	pm := &PositionMonitor{
		cfg: &PositionMonitorConfig{
			RiskThreshold: "high",
		},
	}
	result := pm.shouldAlert(&AnalysisResult{RiskLevel: "", Summary: "safe"}, 0.5)
	assert.False(t, result)
}

func TestPositionMonitorStopNotRunning(t *testing.T) {
	cfg := &PositionMonitorConfig{Enable: true}
	pm := NewPositionMonitor(nil, nil, nil, nil, cfg)
	pm.Stop()
}

func TestCalculatePnLPercentZeroEntry(t *testing.T) {
	pos := &types.Position{
		Symbol:       "BTC-USDT",
		Side:         types.OrderSideBuy,
		Size:         1.0,
		EntryPrice:   0,
		MarkPrice:    50000,
		UnrealizedPnL: 0,
		Leverage:     10,
	}
	result := calculatePnLPercent(pos)
	assert.Equal(t, float64(0), result)
}

func TestDefaultModelForProvider(t *testing.T) {
	assert.Equal(t, "gpt-4", defaultModelForProvider("openai"))
	assert.Equal(t, "claude-3-5-sonnet", defaultModelForProvider("claude"))
	assert.Equal(t, "qwen-plus", defaultModelForProvider("qwen"))
	assert.Equal(t, "gpt-4", defaultModelForProvider(""))
}

func TestNewAnalyzerUsesDefaultModelWhenConfigNil(t *testing.T) {
	a := NewAnalyzer(&Client{}, nil, nil)
	if assert.NotNil(t, a) {
		assert.Equal(t, "gpt-4", a.model)
	}
}

func TestNewAnalyzerUsesProviderDefaultWhenModelEmpty(t *testing.T) {
	cfg := &config.LLMConfig{Provider: "claude"}
	a := NewAnalyzer(&Client{}, nil, cfg)
	if assert.NotNil(t, a) {
		assert.Equal(t, "claude-3-5-sonnet", a.model)
	}
}

// mockProvider implements providers.Provider interface
type mockProvider struct {
	chatFunc func(ctx context.Context, req *providers.ChatRequest) (*providers.ChatResponse, error)
}

func (m *mockProvider) Name() string               { return "mock" }
func (m *mockProvider) Chat(ctx context.Context, req *providers.ChatRequest) (*providers.ChatResponse, error) {
	if m.chatFunc != nil {
		return m.chatFunc(ctx, req)
	}
	return &providers.ChatResponse{Content: "mock response"}, nil
}
func (m *mockProvider) SetAPIKey(key string)       {}
func (m *mockProvider) SetBaseURL(url string)      {}
func (m *mockProvider) SetTimeout(t time.Duration) {}

func TestClientChatNilProvider(t *testing.T) {
	c := &Client{provider: nil}
	resp, err := c.Chat(context.Background(), &providers.ChatRequest{})
	assert.Nil(t, resp)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "未初始化")
}

func TestClientCacheHit(t *testing.T) {
	c := &Client{
		provider:      &mockProvider{},
		cache:         make(map[string]CacheEntry),
		cacheDuration: 5 * time.Minute,
		maxRetries:    1,
	}

	req := &providers.ChatRequest{Model: "test", Messages: []providers.Message{{Role: "user", Content: "hello"}}}

	// First call - should hit provider
	resp1, err := c.Chat(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, "mock response", resp1.Content)

	// Second call - should hit cache
	resp2, err := c.Chat(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, "mock response", resp2.Content)
}

func TestClientCacheExpired(t *testing.T) {
	c := &Client{
		provider:      &mockProvider{},
		cache:         make(map[string]CacheEntry),
		cacheDuration: 1 * time.Nanosecond, // very short
		maxRetries:    1,
	}

	req := &providers.ChatRequest{Model: "test", Messages: []providers.Message{{Role: "user", Content: "hello"}}}

	// First call
	resp1, err := c.Chat(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, "mock response", resp1.Content)

	// Wait for cache to expire
	time.Sleep(10 * time.Millisecond)

	// Second call - cache expired, should call provider again
	resp2, err := c.Chat(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, "mock response", resp2.Content)
}

func TestClientClearCache(t *testing.T) {
	c := &Client{
		provider:      &mockProvider{},
		cache:         make(map[string]CacheEntry),
		cacheDuration: 5 * time.Minute,
		maxRetries:    1,
	}

	req := &providers.ChatRequest{Model: "test", Messages: []providers.Message{{Role: "user", Content: "test"}}}
	_, err := c.Chat(context.Background(), req)
	require.NoError(t, err)

	// Verify cache has entries
	c.cacheMutex.RLock()
	assert.NotEmpty(t, c.cache)
	c.cacheMutex.RUnlock()

	// Clear cache
	c.ClearCache()

	// Verify cache is empty
	c.cacheMutex.RLock()
	assert.Empty(t, c.cache)
	c.cacheMutex.RUnlock()
}

func TestClientProvider(t *testing.T) {
	p := &mockProvider{}
	c := &Client{provider: p}
	assert.Equal(t, p, c.Provider())
}

func TestClientGenerateCacheKey(t *testing.T) {
	c := &Client{}
	req1 := &providers.ChatRequest{Model: "test", Messages: []providers.Message{{Role: "user", Content: "hello"}}}
	req2 := &providers.ChatRequest{Model: "test", Messages: []providers.Message{{Role: "user", Content: "hello"}}}
	req3 := &providers.ChatRequest{Model: "test", Messages: []providers.Message{{Role: "user", Content: "world"}}}

	key1 := c.generateCacheKey(req1)
	key2 := c.generateCacheKey(req2)
	key3 := c.generateCacheKey(req3)

	assert.Equal(t, key1, key2) // same request = same key
	assert.NotEqual(t, key1, key3) // different content = different key
	assert.NotEmpty(t, key1)
}

func TestClientGetFromCacheNonExistent(t *testing.T) {
	c := &Client{
		cache:         make(map[string]CacheEntry),
		cacheDuration: 5 * time.Minute,
	}
	result := c.getFromCache("nonexistent")
	assert.Nil(t, result)
}

func TestClientSetAndGetCache(t *testing.T) {
	c := &Client{
		cache:         make(map[string]CacheEntry),
		cacheDuration: 5 * time.Minute,
	}

	resp := &providers.ChatResponse{Content: "cached"}
	c.setToCache("test-key", resp)

	result := c.getFromCache("test-key")
	require.NotNil(t, result)
	assert.Equal(t, "cached", result.Content)
}

func TestNewClientDisabled(t *testing.T) {
	cfg := &config.LLMConfig{Enable: false}
	c := NewClient(cfg)
	assert.Nil(t, c)
}

func TestNewClientUnsupportedProvider(t *testing.T) {
	cfg := &config.LLMConfig{
		Enable:   true,
		Provider: "unsupported",
	}
	c := NewClient(cfg)
	assert.Nil(t, c)
}

func TestDefaultPositionMonitorConfig(t *testing.T) {
	cfg := DefaultPositionMonitorConfig()
	assert.False(t, cfg.Enable)
	assert.Equal(t, 5*time.Minute, cfg.CheckInterval)
	assert.Equal(t, "high", cfg.RiskThreshold)
	assert.InDelta(t, 2.0, cfg.MinPnLPercent, 0.01)
	assert.Equal(t, 3, cfg.MaxConsecutiveFailures)
}

// mockChatClient wraps mockProvider into a Client for testing Analyzer methods
func newTestAnalyzerWithMock(t *testing.T) *Analyzer {
	t.Helper()
	c := &Client{
		provider:      &mockProvider{},
		cache:         make(map[string]CacheEntry),
		cacheDuration: 5 * time.Minute,
		maxRetries:    1,
	}
	return &Analyzer{
		client: c,
		aiRepo: &noopAIAnalysisRepository{},
		model:  "test-model",
	}
}

func TestAnalyzeTradeWithMockClient(t *testing.T) {
	a := newTestAnalyzerWithMock(t)
	require.NotNil(t, a)

	result, err := a.AnalyzeTrade(context.TODO(), &TradeDecisionData{
		Symbol:          "BTC-USDT",
		Side:            "buy",
		EntryPrice:      50000,
		StopLoss:        48000,
		TakeProfit:      55000,
		PositionSize:    0.1,
		CurrentPrice:    50000,
		TimeFrame:       "1h",
		RiskRewardRatio: 2.5,
		MarketCondition: "bullish",
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "trade", result.Type)
	assert.Equal(t, "BTC-USDT", result.Symbol)
}

func TestAnalyzePositionWithMockClient(t *testing.T) {
	a := newTestAnalyzerWithMock(t)
	require.NotNil(t, a)

	posData := map[string]interface{}{
		"side":           "long",
		"entry_price":    float64(50000),
		"mark_price":     float64(51000),
		"size":           float64(1.0),
		"unrealized_pnl": float64(1000),
		"leverage":       10,
		"pnl_percent":    float64(20.0),
		"liquidation":    float64(45000),
	}

	result, err := a.AnalyzePosition(context.TODO(), "BTC-USDT", posData)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "position", result.Type)
	assert.Equal(t, "BTC-USDT", result.Symbol)
}

func TestAnalyzeMarketWithMockClient(t *testing.T) {
	a := newTestAnalyzerWithMock(t)
	require.NotNil(t, a)

	result, err := a.AnalyzeMarket(context.TODO(), []string{"BTC-USDT", "ETH-USDT"})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "market", result.Type)
	assert.Equal(t, "market", result.Symbol)
}

func TestAnalyzeOrdersWithMockClient(t *testing.T) {
	a := newTestAnalyzerWithMock(t)
	require.NotNil(t, a)

	result, err := a.AnalyzeOrders(context.TODO(), &OrderData{
		Symbol:       "BTC-USDT",
		AnalysisType: "active",
		TimeRange:    "24h",
		Orders:       []map[string]interface{}{{"id": "1", "side": "buy"}},
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "orders", result.Type)
	assert.Equal(t, "BTC-USDT", result.Symbol)
}

func TestAnalyzeOrdersEmptySymbol(t *testing.T) {
	a := newTestAnalyzerWithMock(t)
	require.NotNil(t, a)

	result, err := a.AnalyzeOrders(context.TODO(), &OrderData{
		AnalysisType: "all",
		Orders:       []map[string]interface{}{},
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "orders", result.Type)
	assert.Equal(t, "all", result.Symbol) // empty symbol defaults to "all"
}

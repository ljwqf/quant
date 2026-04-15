package dataservice

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCryptoNewsClientNilConfig(t *testing.T) {
	client := NewCryptoNewsClient(nil)
	require.NotNil(t, client)
	assert.Equal(t, 20, client.config.Limit)
}

func TestNewCryptoNewsClientCustomConfig(t *testing.T) {
	cfg := &CryptoNewsConfig{
		BaseURL:    "https://custom.api/news",
		Categories: []string{"BTC", "ETH"},
		Limit:      5,
	}
	client := NewCryptoNewsClient(cfg)
	require.NotNil(t, client)
	assert.Equal(t, "https://custom.api/news", client.config.BaseURL)
	assert.Equal(t, 5, client.config.Limit)
	assert.Equal(t, []string{"BTC", "ETH"}, client.config.Categories)
}

func TestCryptoNewsFetchFromAPI(t *testing.T) {
	client := NewCryptoNewsClient(nil)
	// Without API key, should return empty or built-in (current impl returns empty from API)
	news, err := client.FetchNews()
	// The API call will fail without a real API key, but should not panic
	_ = err
	_ = news
}

func TestMapImportance(t *testing.T) {
	tests := []struct {
		input    int
		expected int
	}{
		{100, 5},
		{80, 5},
		{79, 4},
		{60, 4},
		{59, 3},
		{40, 3},
		{39, 2},
		{20, 2},
		{19, 1},
		{0, 1},
	}
	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			assert.Equal(t, tt.expected, mapImportance(tt.input))
		})
	}
}

func TestExtractSymbols(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"BTC,Bitcoin", "BTC-USDT"},
		{"ETH,Ethereum", "ETH-USDT"},
		{"SOL,Solana", "SOL-USDT"},
		{"XRP,Ripple", "XRP-USDT"},
		{"DeFi,defi", "BTC-USDT,ETH-USDT"},
		{"Random,Other", "BTC-USDT"},
		{"", "BTC-USDT"},
		{"btc,eth", "BTC-USDT,ETH-USDT"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, extractSymbols(tt.input))
		})
	}
}

func TestTruncateString(t *testing.T) {
	assert.Equal(t, "short", truncateString("short", 100))
	assert.Equal(t, "long text ...", truncateString("long text that exceeds", 10))
	assert.Equal(t, "exact", truncateString("exact", 5))
}

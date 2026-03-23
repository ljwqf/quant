package strategy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalizeMarketSymbol(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"BTC-USDT", "BTCUSDT"},
		{"BTC-USDT-SWAP", "BTCUSDTSWAP"},
		{"BTC/USDT", "BTCUSDT"},
		{"btc-usdt", "BTCUSDT"},
		{"BTC_USDT", "BTCUSDT"},
		{"ETH-USDT-SWAP", "ETHUSDTSWAP"},
		{"ETH/USDT-231229", "ETHUSDT231229"},
	}

	for _, tc := range testCases {
		result := normalizeMarketSymbol(tc.input)
		assert.Equal(t, tc.expected, result)
	}
}


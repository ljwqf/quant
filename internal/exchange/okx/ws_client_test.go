package okx

import "testing"

func TestParseSubscriptionKey(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		ok       bool
		channel  string
		symbol   string
		interval string
	}{
		{name: "valid with interval", key: "candle:BTC-USDT:1m", ok: true, channel: "candle", symbol: "BTC-USDT", interval: "1m"},
		{name: "valid empty interval", key: "ticker:BTC-USDT:", ok: true, channel: "ticker", symbol: "BTC-USDT", interval: ""},
		{name: "invalid missing parts", key: "ticker:BTC-USDT", ok: false},
		{name: "invalid empty symbol", key: "ticker::", ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			channel, symbol, interval, ok := parseSubscriptionKey(tt.key)
			if ok != tt.ok {
				t.Fatalf("ok mismatch: got %v want %v", ok, tt.ok)
			}
			if !ok {
				return
			}
			if channel != tt.channel || symbol != tt.symbol || interval != tt.interval {
				t.Fatalf("parsed mismatch: got (%s,%s,%s) want (%s,%s,%s)", channel, symbol, interval, tt.channel, tt.symbol, tt.interval)
			}
		})
	}
}

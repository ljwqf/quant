package okx

import (
	"sync"
	"testing"
	"time"

	"github.com/ljwqf/quant/internal/config"
	"github.com/ljwqf/quant/pkg/types"
)

func TestHandleWSMessageRoutesTickerByChannel(t *testing.T) {
	client := NewClient(&config.OKXConfig{})

	var wg sync.WaitGroup
	called := make(chan struct{}, 1)
	wg.Add(1)

	client.tickerHandlers["BTC-USDT"] = []func(*types.Tick){
		func(_ *types.Tick) {
			called <- struct{}{}
			wg.Done()
		},
	}

	msg := []byte(`{"arg":{"channel":"ticker","instId":"BTC-USDT"},"data":[{"instId":"BTC-USDT","last":"100","open24h":"99","high24h":"101","low24h":"98","vol24h":"1000","ts":"1710000000000"}]}`)
	client.handleWSMessage(msg)

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("ticker handler was not called")
	}

	select {
	case <-called:
	default:
		t.Fatal("ticker callback signal missing")
	}
}

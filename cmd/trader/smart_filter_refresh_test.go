package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ljwqf/quant/internal/strategy"
)

func TestParseSmartFilterPayloadSupportsNestedData(t *testing.T) {
	snapshot, err := parseSmartFilterPayload([]byte(`{"data":{"netflow":-5000,"sopr":"0.93","mvrv":0.91}}`))

	require.NoError(t, err)
	assert.Equal(t, -5000.0, snapshot.Netflow)
	assert.Equal(t, 0.93, snapshot.SOPR)
	assert.Equal(t, 0.91, snapshot.MVRV)
}

func TestLoadSmartFilterSnapshotFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "snapshot.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"netflow":-4200,"sopr":0.92,"mvrv":0.9}`), 0o644))

	snapshot, err := loadSmartFilterSnapshotFromFile(path)

	require.NoError(t, err)
	assert.Equal(t, -4200.0, snapshot.Netflow)
	assert.Equal(t, 0.92, snapshot.SOPR)
	assert.Equal(t, 0.9, snapshot.MVRV)
}

func TestStartSmartFilterAutoRefreshFromEnv(t *testing.T) {
	t.Setenv("SMART_FILTER_REFRESH_ENABLED", "true")
	t.Setenv("SMART_FILTER_SOURCE", "env")
	t.Setenv("SMART_FILTER_REFRESH_INTERVAL", "20ms")
	t.Setenv("SMART_FILTER_NETFLOW", "-7000")
	t.Setenv("SMART_FILTER_SOPR", "0.91")
	t.Setenv("SMART_FILTER_MVRV", "0.89")

	filter := strategy.NewSmartFilter()
	require.NoError(t, filter.Init(map[string]interface{}{}))

	cfg := &smartFilterRefreshConfig{
		Enabled:  true,
		Source:   "env",
		Interval: 20 * time.Millisecond,
	}
	stop := startSmartFilterAutoRefresh(filter, cfg)
	defer stop()

	deadline := time.Now().Add(400 * time.Millisecond)
	for time.Now().Before(deadline) {
		data := filter.GetOnChainData()
		if data != nil {
			assert.Equal(t, -7000.0, data.ExchangeNetflow)
			assert.Equal(t, 0.91, data.SOPR)
			assert.Equal(t, 0.89, data.LTHMVRV)
			return
		}
		time.Sleep(20 * time.Millisecond)
	}

	t.Fatal("smart filter data was not refreshed in time")
}

func TestLoadSmartFilterSnapshotWithSourceFallsBackHTTPToFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "upstream error", http.StatusBadGateway)
	}))
	defer server.Close()

	dir := t.TempDir()
	path := filepath.Join(dir, "fallback.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"netflow":-3333,"sopr":0.92,"mvrv":0.9}`), 0o644))

	snapshot, usedSource, err := loadSmartFilterSnapshotWithSource(smartFilterRefreshConfig{
		Source:      "http",
		HTTPURL:     server.URL,
		HTTPTimeout: 200 * time.Millisecond,
		FilePath:    path,
	})

	require.NoError(t, err)
	assert.Equal(t, "file", usedSource)
	assert.Equal(t, -3333.0, snapshot.Netflow)
	assert.Equal(t, 0.92, snapshot.SOPR)
	assert.Equal(t, 0.9, snapshot.MVRV)
}

func TestLoadSmartFilterSnapshotWithSourceFallsBackFileToEnv(t *testing.T) {
	t.Setenv("SMART_FILTER_NETFLOW", "-1111")
	t.Setenv("SMART_FILTER_SOPR", "0.9")
	t.Setenv("SMART_FILTER_MVRV", "0.88")

	snapshot, usedSource, err := loadSmartFilterSnapshotWithSource(smartFilterRefreshConfig{
		Source:   "file",
		FilePath: filepath.Join(t.TempDir(), "missing.json"),
	})

	require.NoError(t, err)
	assert.Equal(t, "env", usedSource)
	assert.Equal(t, -1111.0, snapshot.Netflow)
	assert.Equal(t, 0.9, snapshot.SOPR)
	assert.Equal(t, 0.88, snapshot.MVRV)
}

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

func TestLoadSmartFilterSnapshot(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "snapshot.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"netflow":-999,"sopr":0.95,"mvrv":1.1}`), 0o644))

	snapshot, err := loadSmartFilterSnapshot(smartFilterRefreshConfig{
		Source:   "file",
		FilePath: path,
	})

	require.NoError(t, err)
	assert.Equal(t, -999.0, snapshot.Netflow)
	assert.Equal(t, 0.95, snapshot.SOPR)
	assert.Equal(t, 1.1, snapshot.MVRV)
}

func TestEnvDurationOrDefault(t *testing.T) {
	t.Setenv("TEST_DUR", "5m")
	assert.Equal(t, 5*time.Minute, envDurationOrDefault("TEST_DUR", 1*time.Hour))

	assert.Equal(t, 30*time.Second, envDurationOrDefault("NONEXISTENT_DUR", 30*time.Second))

	t.Setenv("TEST_DUR_INVALID", "not-a-duration")
	assert.Equal(t, 10*time.Second, envDurationOrDefault("TEST_DUR_INVALID", 10*time.Second))
}

func TestExtractNumericField(t *testing.T) {
	// From float64
	val, err := extractNumericField(map[string]interface{}{"key": float64(42.5)}, "key")
	require.NoError(t, err)
	assert.InDelta(t, 42.5, val, 0.001)

	// From int (not supported by the function)
	_, err = extractNumericField(map[string]interface{}{"key": 42}, "key")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid type")

	// From string
	val, err = extractNumericField(map[string]interface{}{"key": "3.14"}, "key")
	require.NoError(t, err)
	assert.InDelta(t, 3.14, val, 0.001)

	// Missing key
	_, err = extractNumericField(map[string]interface{}{}, "missing")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing")

	// From invalid type
	_, err = extractNumericField(map[string]interface{}{"key": true}, "key")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid type")

	// From invalid string
	_, err = extractNumericField(map[string]interface{}{"key": "not-a-number"}, "key")
	require.Error(t, err)
}

func TestSmartFilterCandidateSourcesAuto(t *testing.T) {
	cfg := smartFilterRefreshConfig{
		Source:            "auto",
		HTTPURL:           "http://example.com/api",
		FilePath:          "/tmp/snapshot.json",
		CryptoQuantAPIKey: "test-key",
	}
	sources, err := smartFilterCandidateSources(cfg)
	require.NoError(t, err)
	assert.Contains(t, sources, "cryptoquant")
	assert.Contains(t, sources, "http")
	assert.Contains(t, sources, "file")
	assert.Contains(t, sources, "env")
}

func TestSmartFilterCandidateSourcesExplicit(t *testing.T) {
	cfg := smartFilterRefreshConfig{Source: "env"}
	sources, err := smartFilterCandidateSources(cfg)
	require.NoError(t, err)
	assert.Len(t, sources, 1)
	assert.Equal(t, "env", sources[0])
}

func TestSmartFilterCandidateSourcesEmpty(t *testing.T) {
	cfg := smartFilterRefreshConfig{Source: ""}
	sources, err := smartFilterCandidateSources(cfg)
	require.NoError(t, err)
	// Empty source defaults to "auto"
	assert.Contains(t, sources, "env")
}

func TestLoadSmartFilterSnapshotBySourceEnv(t *testing.T) {
	t.Setenv("SMART_FILTER_NETFLOW", "-5000")
	t.Setenv("SMART_FILTER_SOPR", "0.94")
	t.Setenv("SMART_FILTER_MVRV", "0.92")

	snapshot, err := loadSmartFilterSnapshotBySource(smartFilterRefreshConfig{}, "env")
	require.NoError(t, err)
	assert.Equal(t, -5000.0, snapshot.Netflow)
	assert.Equal(t, 0.94, snapshot.SOPR)
	assert.Equal(t, 0.92, snapshot.MVRV)
}

func TestLoadSmartFilterSnapshotBySourceFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"netflow":-2000,"sopr":0.96,"mvrv":1.05}`), 0o644))

	snapshot, err := loadSmartFilterSnapshotBySource(smartFilterRefreshConfig{
		FilePath: path,
	}, "file")
	require.NoError(t, err)
	assert.Equal(t, -2000.0, snapshot.Netflow)
	assert.Equal(t, 0.96, snapshot.SOPR)
	assert.Equal(t, 1.05, snapshot.MVRV)
}

func TestLoadSmartFilterSnapshotBySourceUnknown(t *testing.T) {
	_, err := loadSmartFilterSnapshotBySource(smartFilterRefreshConfig{}, "unknown")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown")
}

func TestLoadSmartFilterSnapshotFromHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer server.Close()

	_, err := loadSmartFilterSnapshotFromHTTP(server.URL, 200*time.Millisecond)
	require.Error(t, err)
	// Should get an error due to non-200 status
}

func TestLoadSmartFilterSnapshotFromHTTPInvalidURL(t *testing.T) {
	_, err := loadSmartFilterSnapshotFromHTTP("", 200*time.Millisecond)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty")
}

func TestLoadSmartFilterSnapshotFromFileEmptyPath(t *testing.T) {
	_, err := loadSmartFilterSnapshotFromFile("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty")
}

func TestParseSmartFilterPayloadInvalidJSON(t *testing.T) {
	_, err := parseSmartFilterPayload([]byte(`{invalid json}`))
	require.Error(t, err)
}

func TestParseSmartFilterPayloadEmptyData(t *testing.T) {
	_, err := parseSmartFilterPayload([]byte{})
	require.Error(t, err)
}

func TestParseSmartFilterPayloadFlatFields(t *testing.T) {
	snapshot, err := parseSmartFilterPayload([]byte(`{"netflow":-3000,"sopr":0.95,"mvrv":1.0}`))
	require.NoError(t, err)
	assert.Equal(t, -3000.0, snapshot.Netflow)
	assert.Equal(t, 0.95, snapshot.SOPR)
	assert.Equal(t, 1.0, snapshot.MVRV)
}

func TestStartSmartFilterAutoRefreshStop(t *testing.T) {
	t.Setenv("SMART_FILTER_NETFLOW", "-1000")
	t.Setenv("SMART_FILTER_SOPR", "0.9")
	t.Setenv("SMART_FILTER_MVRV", "0.85")

	cfg := &smartFilterRefreshConfig{
		Enabled:  true,
		Source:   "env",
		Interval: 50 * time.Millisecond,
	}

	filter := strategy.NewSmartFilter()
	require.NoError(t, filter.Init(map[string]interface{}{}))

	stop := startSmartFilterAutoRefresh(filter, cfg)
	// Stop immediately - should not panic
	stop()
}

func TestSmartFilterCandidateSourcesCryptoQuant(t *testing.T) {
	cfg := smartFilterRefreshConfig{Source: "cryptoquant"}
	sources, err := smartFilterCandidateSources(cfg)
	require.NoError(t, err)
	// cryptoquant includes all fallback sources
	assert.Equal(t, []string{"cryptoquant", "http", "file", "env"}, sources)
}

func TestSmartFilterCandidateSourcesHTTP(t *testing.T) {
	cfg := smartFilterRefreshConfig{Source: "http", HTTPURL: "http://example.com"}
	sources, err := smartFilterCandidateSources(cfg)
	require.NoError(t, err)
	assert.Equal(t, []string{"http", "file", "env"}, sources)
}

func TestSmartFilterCandidateSourcesFile(t *testing.T) {
	cfg := smartFilterRefreshConfig{Source: "file", FilePath: "/tmp/test.json"}
	sources, err := smartFilterCandidateSources(cfg)
	require.NoError(t, err)
	assert.Equal(t, []string{"file", "env"}, sources)
}

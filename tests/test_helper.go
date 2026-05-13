package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ljwqf/quant/internal/api"
	"github.com/ljwqf/quant/internal/config"
	"github.com/ljwqf/quant/internal/monitoring"
	"github.com/ljwqf/quant/pkg/logger"
	"github.com/ljwqf/quant/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	TestTimeout = 30 * time.Second
	TestAPIPort = 8888
)

// TestContext 测试上下文
type TestContext struct {
	T           *testing.T
	Config      *config.Config
	APIServer   *api.Server
	Metrics     *monitoring.Metrics
	TestServer  *httptest.Server
	TempDir     string
	CleanupFunc func()
}

// NewTestContext 创建测试上下文
func NewTestContext(t *testing.T) *TestContext {
	t.Helper()

	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "quant-test-")
	require.NoError(t, err)

	// 初始化日志
	logPath := filepath.Join(tempDir, "test.log")
	require.NoError(t, logger.Init("debug", logPath))

	// 加载测试配置
	cfg := loadTestConfig(t)

	// 创建指标管理器
	metrics := monitoring.NewMetrics(&monitoring.MetricsConfig{
		Enable: true,
	})

	// 创建API服务器
	apiServer := api.NewServer("127.0.0.1", TestAPIPort, cfg, "", nil)
	apiServer.SetMetrics(metrics)

	// 创建测试服务器
	testServer := httptest.NewServer(apiServer.GetMux())

	ctx := &TestContext{
		T:          t,
		Config:     cfg,
		APIServer:  apiServer,
		Metrics:    metrics,
		TestServer: testServer,
		TempDir:    tempDir,
		CleanupFunc: func() {
			testServer.Close()
			os.RemoveAll(tempDir)
		},
	}

	t.Cleanup(ctx.Cleanup)

	return ctx
}

// Cleanup 清理测试资源
func (ctx *TestContext) Cleanup() {
	if ctx.CleanupFunc != nil {
		ctx.CleanupFunc()
	}
}

// loadTestConfig 加载测试配置
func loadTestConfig(t *testing.T) *config.Config {
	t.Helper()

	cfg := &config.Config{
		Server: config.ServerConfig{
			Port: TestAPIPort,
			Host: "localhost",
		},
		Exchange: config.ExchangeConfig{
			OKX: config.OKXConfig{
				Simulated: true,
			},
		},
	}

	return cfg
}

// WaitForCondition 等待条件满足
func WaitForCondition(t *testing.T, timeout time.Duration, check func() bool) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if check() {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}

	t.Fatalf("条件未在 %v 内满足", timeout)
}

// AssertResponseStatus 断言HTTP响应状态
func AssertResponseStatus(t *testing.T, resp *http.Response, expectedStatus int) {
	t.Helper()
	assert.Equal(t, expectedStatus, resp.StatusCode)
}

// AssertJSONResponse 断言JSON响应
func AssertJSONResponse(t *testing.T, resp *http.Response) map[string]interface{} {
	t.Helper()

	require.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	var result map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))

	return result
}

// MakeHTTPRequest 发送HTTP请求
func (ctx *TestContext) MakeHTTPRequest(method, path string, body interface{}) *http.Response {
	ctx.T.Helper()

	var req *http.Request
	var err error

	if body != nil {
		reqBody, err := json.Marshal(body)
		require.NoError(ctx.T, err)

		req, err = http.NewRequest(method, ctx.TestServer.URL+path, bytes.NewReader(reqBody))
		require.NoError(ctx.T, err)
		req.Header.Set("Content-Type", "application/json")
	} else {
		req, err = http.NewRequest(method, ctx.TestServer.URL+path, nil)
		require.NoError(ctx.T, err)
	}

	ctx.T.Logf("请求: %s %s", method, path)

	client := &http.Client{
		Timeout: TestTimeout,
	}

	resp, err := client.Do(req)
	require.NoError(ctx.T, err)

	return resp
}

// WithTestTimeout 创建带超时的context
func WithTestTimeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), TestTimeout)
}

// GenerateTestBars 生成测试用K线数据
func GenerateTestBars(symbol string, count int) []*types.Bar {
	bars := make([]*types.Bar, 0, count)
	baseTime := time.Now().Add(-time.Duration(count) * time.Hour)

	for i := 0; i < count; i++ {
		basePrice := 50000.0 + float64(i)*10
		bars = append(bars, &types.Bar{
			Symbol:    symbol,
			Timestamp: baseTime.Add(time.Duration(i) * time.Hour),
			Open:      basePrice,
			High:      basePrice + 50,
			Low:       basePrice - 50,
			Close:     basePrice + 20,
			Volume:    1000.0 + float64(i)*10,
			Interval:  "1h",
		})
	}

	return bars
}

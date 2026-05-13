package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ljwqf/quant/internal/data/cryptoquant"
	"github.com/ljwqf/quant/internal/strategy"
	"github.com/ljwqf/quant/pkg/logger"
	"go.uber.org/zap"
)

const (
	defaultSmartFilterRefreshInterval = 5 * time.Minute
	defaultSmartFilterHTTPTimeout     = 10 * time.Second
)

type smartFilterSnapshot struct {
	Netflow float64
	SOPR    float64
	MVRV    float64
}

type smartFilterRefreshConfig struct {
	Enabled              bool
	Source               string
	Interval             time.Duration
	FilePath             string
	HTTPURL              string
	HTTPTimeout          time.Duration
	CryptoQuantAsset     string
	CryptoQuantAPIKey    string
}

type OnChainDataUpdater interface {
	UpdateOnChainData(netflow, sopr, mvrv float64)
}

func startSmartFilterAutoRefresh(filter *strategy.SmartFilter, cfg *smartFilterRefreshConfig, updaters ...OnChainDataUpdater) func() {
	if filter == nil {
		return func() {}
	}

	if !cfg.Enabled {
		logger.Info("SmartFilter定时刷新已禁用")
		return func() {}
	}

	if cfg.Interval <= 0 {
		logger.Warn("SmartFilter刷新间隔无效，跳过定时刷新", zap.Duration("interval", cfg.Interval))
		return func() {}
	}

	applySnapshot := func(stage string) {
		snapshot, usedSource, err := loadSmartFilterSnapshotWithSource(*cfg)
		if err != nil {
			logger.Warn("拉取SmartFilter外部数据失败",
				zap.String("stage", stage),
				zap.String("source", cfg.Source),
				zap.Error(err),
			)
			return
		}

		filter.UpdateOnChainData(snapshot.Netflow, snapshot.SOPR, snapshot.MVRV)
		for _, updater := range updaters {
			if updater != nil {
				updater.UpdateOnChainData(snapshot.Netflow, snapshot.SOPR, snapshot.MVRV)
			}
		}
		logger.Info("SmartFilter外部数据已刷新",
			zap.String("stage", stage),
			zap.String("source", usedSource),
			zap.Float64("netflow", snapshot.Netflow),
			zap.Float64("sopr", snapshot.SOPR),
			zap.Float64("mvrv", snapshot.MVRV),
		)
	}

	applySnapshot("startup")

	stopCh := make(chan struct{})
	ticker := time.NewTicker(cfg.Interval)
	go func() {
		for {
			select {
			case <-ticker.C:
				applySnapshot("ticker")
			case <-stopCh:
				return
			}
		}
	}()

	logger.Info("SmartFilter定时刷新已启动",
		zap.String("source", cfg.Source),
		zap.Duration("interval", cfg.Interval),
	)

	var once sync.Once
	return func() {
		once.Do(func() {
			ticker.Stop()
			close(stopCh)
		})
	}
}



func loadSmartFilterSnapshot(cfg smartFilterRefreshConfig) (smartFilterSnapshot, error) {
	snapshot, _, err := loadSmartFilterSnapshotWithSource(cfg)
	return snapshot, err
}

func loadSmartFilterSnapshotWithSource(cfg smartFilterRefreshConfig) (smartFilterSnapshot, string, error) {
	candidates, err := smartFilterCandidateSources(cfg)
	if err != nil {
		return smartFilterSnapshot{}, "", err
	}

	attemptErrors := make([]string, 0, len(candidates))
	for _, source := range candidates {
		snapshot, loadErr := loadSmartFilterSnapshotBySource(cfg, source)
		if loadErr == nil {
			return snapshot, source, nil
		}
		attemptErrors = append(attemptErrors, fmt.Sprintf("%s: %v", source, loadErr))
	}

	return smartFilterSnapshot{}, "", fmt.Errorf("all sources failed: %s", strings.Join(attemptErrors, "; "))
}

func smartFilterCandidateSources(cfg smartFilterRefreshConfig) ([]string, error) {
	source := strings.ToLower(strings.TrimSpace(cfg.Source))
	if source == "" {
		source = "auto"
	}

	if source == "auto" {
		// 检查是否设置了CryptoQuant API Key
		if strings.TrimSpace(cfg.CryptoQuantAPIKey) != "" {
			return []string{"cryptoquant", "http", "file", "env"}, nil
		}
		if strings.TrimSpace(cfg.HTTPURL) != "" {
			return []string{"http", "file", "env"}, nil
		}
		if strings.TrimSpace(cfg.FilePath) != "" {
			return []string{"file", "env"}, nil
		}
		return []string{"env"}, nil
	}

	switch source {
	case "cryptoquant":
		return []string{"cryptoquant", "http", "file", "env"}, nil
	case "http":
		return []string{"http", "file", "env"}, nil
	case "file":
		return []string{"file", "env"}, nil
	case "env":
		return []string{"env"}, nil
	default:
		return nil, fmt.Errorf("unsupported smart filter source: %s", source)
	}
}

func loadSmartFilterSnapshotBySource(cfg smartFilterRefreshConfig, source string) (smartFilterSnapshot, error) {
	switch source {
	case "cryptoquant":
		return loadSmartFilterSnapshotFromCryptoQuant(cfg.CryptoQuantAPIKey, cfg.CryptoQuantAsset)
	case "http":
		return loadSmartFilterSnapshotFromHTTP(cfg.HTTPURL, cfg.HTTPTimeout)
	case "file":
		return loadSmartFilterSnapshotFromFile(cfg.FilePath)
	case "env":
		snapshot := loadSmartFilterSnapshotFromEnv()
		if snapshot.Netflow == 0 && snapshot.SOPR == 0 && snapshot.MVRV == 0 {
			return smartFilterSnapshot{}, fmt.Errorf("no SMART_FILTER_NETFLOW/SOPR/MVRV env vars set")
		}
		return snapshot, nil
	default:
		return smartFilterSnapshot{}, fmt.Errorf("unsupported smart filter source: %s", source)
	}
}

func loadSmartFilterSnapshotFromEnv() smartFilterSnapshot {
	// 仅当环境变量显式设置了SMART_FILTER_NETFLOW/SOPR/MVRV时才使用
	// 避免使用硬编码默认值导致策略基于假数据做决策
	netflow, hasNetflow := os.LookupEnv("SMART_FILTER_NETFLOW")
	sopr, hasSOPR := os.LookupEnv("SMART_FILTER_SOPR")
	mvrv, hasMVRV := os.LookupEnv("SMART_FILTER_MVRV")

	if !hasNetflow && !hasSOPR && !hasMVRV {
		// 没有任何环境变量，返回空快照让调用者返回错误
		return smartFilterSnapshot{}
	}

	nf, sf, mf := -6000.0, 0.94, 0.95 // 各变量的兜底值
	if hasNetflow {
		if v, err := strconv.ParseFloat(strings.TrimSpace(netflow), 64); err == nil {
			nf = v
		}
	}
	if hasSOPR {
		if v, err := strconv.ParseFloat(strings.TrimSpace(sopr), 64); err == nil {
			sf = v
		}
	}
	if hasMVRV {
		if v, err := strconv.ParseFloat(strings.TrimSpace(mvrv), 64); err == nil {
			mf = v
		}
	}

	return smartFilterSnapshot{
		Netflow: nf,
		SOPR:    sf,
		MVRV:    mf,
	}
}

func loadSmartFilterSnapshotFromCryptoQuant(apiKey, asset string) (smartFilterSnapshot, error) {
	// 创建CryptoQuant客户端
	client := cryptoquant.NewClient(apiKey)

	// 获取链上数据
	netflow, sopr, mvrv, err := client.GetOnChainData(asset)
	if err != nil {
		// 不返回硬编码默认值，返回错误让调用者知道数据不可用
		logger.Warn("从CryptoQuant获取链上数据失败",
			zap.String("asset", asset),
			zap.Error(err),
		)
		return smartFilterSnapshot{}, fmt.Errorf("CryptoQuant API error: %w", err)
	}

	logger.Info("从CryptoQuant获取数据成功",
		zap.String("asset", asset),
		zap.Float64("netflow", netflow),
		zap.Float64("sopr", sopr),
		zap.Float64("mvrv", mvrv),
	)

	return smartFilterSnapshot{
		Netflow: netflow,
		SOPR:    sopr,
		MVRV:    mvrv,
	}, nil
}

func loadSmartFilterSnapshotFromFile(path string) (smartFilterSnapshot, error) {
	if strings.TrimSpace(path) == "" {
		return smartFilterSnapshot{}, fmt.Errorf("SMART_FILTER_FILE_PATH is empty")
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return smartFilterSnapshot{}, err
	}

	return parseSmartFilterPayload(raw)
}

func loadSmartFilterSnapshotFromHTTP(url string, timeout time.Duration) (smartFilterSnapshot, error) {
	if strings.TrimSpace(url) == "" {
		return smartFilterSnapshot{}, fmt.Errorf("SMART_FILTER_HTTP_URL is empty")
	}
	if timeout <= 0 {
		timeout = defaultSmartFilterHTTPTimeout
	}

	client := &http.Client{Timeout: timeout}
	resp, err := client.Get(url)
	if err != nil {
		return smartFilterSnapshot{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return smartFilterSnapshot{}, fmt.Errorf("http status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return smartFilterSnapshot{}, err
	}

	return parseSmartFilterPayload(raw)
}

func parseSmartFilterPayload(raw []byte) (smartFilterSnapshot, error) {
	var obj map[string]interface{}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return smartFilterSnapshot{}, err
	}

	if nested, ok := obj["data"].(map[string]interface{}); ok {
		obj = nested
	}

	netflow, err := extractNumericField(obj, "netflow")
	if err != nil {
		return smartFilterSnapshot{}, err
	}
	sopr, err := extractNumericField(obj, "sopr")
	if err != nil {
		return smartFilterSnapshot{}, err
	}
	mvrv, err := extractNumericField(obj, "mvrv")
	if err != nil {
		return smartFilterSnapshot{}, err
	}

	return smartFilterSnapshot{Netflow: netflow, SOPR: sopr, MVRV: mvrv}, nil
}

func extractNumericField(obj map[string]interface{}, key string) (float64, error) {
	value, ok := obj[key]
	if !ok {
		return 0, fmt.Errorf("missing field: %s", key)
	}

	switch typed := value.(type) {
	case float64:
		return typed, nil
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		if err != nil {
			return 0, fmt.Errorf("invalid %s: %w", key, err)
		}
		return parsed, nil
	default:
		return 0, fmt.Errorf("invalid type for %s: %T", key, value)
	}
}

func envDurationOrDefault(key string, fallback time.Duration) time.Duration {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}

	parsed, err := time.ParseDuration(raw)
	if err != nil {
		logger.Warn("解析环境变量时长失败，使用默认值",
			zap.String("key", key),
			zap.String("value", raw),
			zap.Duration("fallback", fallback),
			zap.Error(err),
		)
		return fallback
	}

	return parsed
}

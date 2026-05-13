package dataservice

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/ljwqf/quant/internal/storage"
	"github.com/ljwqf/quant/pkg/logger"
	"go.uber.org/zap"
)

// EconomicCalendarConfig 经济日历配置
type EconomicCalendarConfig struct {
	BaseURL   string
	APIKey    string
	Countries []string
	DaysAhead int
	Timeout   time.Duration
}

// EconomicCalendarClient 经济日历客户端
type EconomicCalendarClient struct {
	config    *EconomicCalendarConfig
	client    *http.Client
	lastFetch time.Time
	cache     []*storage.EconomicEvent
	cacheTTL  time.Duration
	mu        sync.RWMutex
}

// DefaultEconomicCalendarConfig 默认配置
func DefaultEconomicCalendarConfig() *EconomicCalendarConfig {
	return &EconomicCalendarConfig{
		BaseURL:   "https://nfs.faireconomy.media/ff_calendar_thisweek.xml",
		Countries: []string{"US", "CN", "JP", "EU"},
		DaysAhead: 7,
		Timeout:   15 * time.Second,
	}
}

// NewEconomicCalendarClient 创建经济日历客户端
func NewEconomicCalendarClient(cfg *EconomicCalendarConfig) *EconomicCalendarClient {
	if cfg == nil {
		cfg = DefaultEconomicCalendarConfig()
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 15 * time.Second
	}
	if cfg.DaysAhead <= 0 {
		cfg.DaysAhead = 7
	}

	return &EconomicCalendarClient{
		config:   cfg,
		client:   &http.Client{Timeout: cfg.Timeout},
		cacheTTL: 30 * time.Minute, // 经济事件变化慢，缓存更久
	}
}

// FetchEvents 获取经济事件
func (c *EconomicCalendarClient) FetchEvents() ([]*storage.EconomicEvent, error) {
	c.mu.RLock()
	if len(c.cache) > 0 && time.Since(c.lastFetch) < c.cacheTTL {
		cached := make([]*storage.EconomicEvent, len(c.cache))
		copy(cached, c.cache)
		c.mu.RUnlock()
		return cached, nil
	}
	c.mu.RUnlock()

	events, err := c.fetchFromAPI()
	if err != nil {
		logger.Warn("获取经济事件失败，使用缓存", zap.Error(err))
		c.mu.RLock()
		cached := c.cache
		c.mu.RUnlock()
		return cached, err
	}

	c.mu.Lock()
	c.cache = events
	c.lastFetch = time.Now()
	c.mu.Unlock()

	return events, nil
}

type investingComEvent struct {
	Date    string `json:"Date"`
	Time    string `json:"Time"`
	Country string `json:"Country"`
	Event   string `json:"Event"`
	Actual  string `json:"Actual"`
	Forecast string `json:"Forecast"`
	Previous string `json:"Previous"`
	Importance int   `json:"Importance"`
	Currency string `json:"Currency"`
}

func (c *EconomicCalendarClient) fetchFromAPI() ([]*storage.EconomicEvent, error) {
	// 使用 TradingEconomics 风格的 API 获取经济日历
	// 如果有 API Key 则使用正式接口，否则使用模拟数据
	if c.config.APIKey == "" {
		return c.getBuiltInEvents(), nil
	}

	return c.fetchTradingEconomics()
}

func (c *EconomicCalendarClient) fetchTradingEconomics() ([]*storage.EconomicEvent, error) {
	params := url.Values{}
	params.Set("c", strings.Join(c.config.Countries, ","))
	params.Set("d1", time.Now().Format("2006-01-02"))
	params.Set("d2", time.Now().AddDate(0, 0, c.config.DaysAhead).Format("2006-01-02"))

	apiURL := fmt.Sprintf("https://api.tradingeconomics.com/calendar?%s", params.Encode())

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.config.APIKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	var events []investingComEvent
	if err := json.Unmarshal(body, &events); err != nil {
		return nil, fmt.Errorf("解析JSON失败: %w", err)
	}

	return c.convertEvents(events), nil
}

// getBuiltInEvents 内置经济事件（当没有API Key时使用）
func (c *EconomicCalendarClient) getBuiltInEvents() []*storage.EconomicEvent {
	now := time.Now()
	weekday := int(now.Weekday())
	if weekday == 0 {
		weekday = 7
	}

	events := []*storage.EconomicEvent{
		{
			Title:      "美联储利率决议",
			Country:    "US",
			Indicator:  "Federal Funds Rate",
			EventTime:  now.AddDate(0, 0, 10-weekday),
			Importance: 5,
			Currency:   "USD",
			CreatedAt:  now,
		},
		{
			Title:      "美国非农就业人数",
			Country:    "US",
			Indicator:  "Non-Farm Employment Change",
			EventTime:  now.AddDate(0, 0, 14-weekday),
			Importance: 5,
			Currency:   "USD",
			CreatedAt:  now,
		},
		{
			Title:      "美国CPI月率",
			Country:    "US",
			Indicator:  "Consumer Price Index",
			EventTime:  now.AddDate(0, 0, 12-weekday),
			Importance: 4,
			Currency:   "USD",
			CreatedAt:  now,
		},
		{
			Title:      "欧洲央行利率决议",
			Country:    "EU",
			Indicator:  "Main Refinancing Rate",
			EventTime:  now.AddDate(0, 0, 11-weekday),
			Importance: 4,
			Currency:   "EUR",
			CreatedAt:  now,
		},
		{
			Title:      "中国CPI年率",
			Country:    "CN",
			Indicator:  "Consumer Price Index",
			EventTime:  now.AddDate(0, 0, 13-weekday),
			Importance: 3,
			Currency:   "CNY",
			CreatedAt:  now,
		},
		{
			Title:      "美国初请失业金人数",
			Country:    "US",
			Indicator:  "Initial Jobless Claims",
			EventTime:  now.AddDate(0, 0, 4-weekday),
			Importance: 3,
			Currency:   "USD",
			CreatedAt:  now,
		},
	}

	return events
}

func (c *EconomicCalendarClient) convertEvents(events []investingComEvent) []*storage.EconomicEvent {
	result := make([]*storage.EconomicEvent, 0, len(events))
	for _, e := range events {
		event := &storage.EconomicEvent{
			Title:      e.Event,
			Country:    e.Country,
			Indicator:  e.Event,
			Currency:   e.Currency,
			Importance: e.Importance,
			CreatedAt:  time.Now(),
		}

		// 解析时间
		if e.Date != "" && e.Time != "" {
			dt := fmt.Sprintf("%s %s", e.Date, e.Time)
			if t, err := time.Parse("2006-01-02 15:04", dt); err == nil {
				event.EventTime = t
			}
		}

		// 解析数值
		event.Actual = parseFloatString(e.Actual)
		event.Forecast = parseFloatString(e.Forecast)
		event.Previous = parseFloatString(e.Previous)

		result = append(result, event)
	}

	logger.Info("获取经济事件成功", zap.Int("count", len(result)))
	return result
}

func parseFloatString(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" || s == "-" {
		return 0
	}
	// 去掉百分号
	s = strings.TrimSuffix(s, "%")
	var v float64
	fmt.Sscanf(s, "%f", &v)
	return v
}

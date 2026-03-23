package storage

import "time"

// ManualTrade 手动交易记录
type ManualTrade struct {
	ID               int64     `db:"id"`
	OrderID          string    `db:"order_id"`
	Symbol           string    `db:"symbol"`
	Side             string    `db:"side"`
	Type             string    `db:"type"`
	Price            float64   `db:"price"`
	Size             float64   `db:"size"`
	FilledSize       float64   `db:"filled_size"`
	Status           string    `db:"status"`
	Leverage         int       `db:"leverage"`
	TakeProfit       float64   `db:"take_profit"`
	StopLoss         float64   `db:"stop_loss"`
	AIAnalysisID     int64     `db:"ai_analysis_id"`
	AIAnalysisSummary string   `db:"ai_analysis_summary"`
	CreatedAt        time.Time `db:"created_at"`
	UpdatedAt        time.Time `db:"updated_at"`
}

// AIAnalysis AI分析记录
type AIAnalysis struct {
	ID               int64     `db:"id"`
	Symbol           string    `db:"symbol"`
	AnalysisType     string    `db:"analysis_type"`
	Provider         string    `db:"provider"`
	Model            string    `db:"model"`
	Prompt           string    `db:"prompt"`
	Content          string    `db:"content"`
	RiskLevel        string    `db:"risk_level"`
	Suggestions      string    `db:"suggestions"`
	Warnings         string    `db:"warnings"`
	ConfidenceScore  float64   `db:"confidence_score"`
	PromptTokens     int       `db:"prompt_tokens"`
	CompletionTokens int       `db:"completion_tokens"`
	TotalTokens      int       `db:"total_tokens"`
	LatencyMs        int64     `db:"latency_ms"`
	CreatedAt        time.Time `db:"created_at"`
}

// NewsEvent 新闻事件
type NewsEvent struct {
	ID             int64     `db:"id"`
	ExternalID     string    `db:"external_id"`
	Title          string    `db:"title"`
	Summary        string    `db:"summary"`
	Content        string    `db:"content"`
	Source         string    `db:"source"`
	URL            string    `db:"url"`
	ImageURL       string    `db:"image_url"`
	Category       string    `db:"category"`
	Tags           string    `db:"tags"`
	Importance     int       `db:"importance"`
	Sentiment      string    `db:"sentiment"`
	RelatedSymbols string    `db:"related_symbols"`
	PublishedAt    time.Time `db:"published_at"`
	CreatedAt      time.Time `db:"created_at"`
}

// EconomicEvent 经济事件
type EconomicEvent struct {
	ID         int64     `db:"id"`
	ExternalID string    `db:"external_id"`
	Title      string    `db:"title"`
	Country    string    `db:"country"`
	Currency   string    `db:"currency"`
	Indicator  string    `db:"indicator"`
	Actual     float64   `db:"actual"`
	Forecast   float64   `db:"forecast"`
	Previous   float64   `db:"previous"`
	Unit       string    `db:"unit"`
	Importance int       `db:"importance"`
	Impact     string    `db:"impact"`
	EventTime  time.Time `db:"event_time"`
	CreatedAt  time.Time `db:"created_at"`
}

// AlertRecord 提醒记录
type AlertRecord struct {
	ID        int64     `db:"id"`
	AlertType string    `db:"alert_type"`
	Level     string    `db:"level"`
	Title     string    `db:"title"`
	Message   string    `db:"message"`
	Symbol    string    `db:"symbol"`
	Metadata  string    `db:"metadata"`
	Channels  string    `db:"channels"`
	Read      bool      `db:"read"`
	CreatedAt time.Time `db:"created_at"`
}

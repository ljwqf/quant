package llmanalysis

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetTechnicalAnalysisPrompt(t *testing.T) {
	data := &TechnicalAnalysisData{
		Symbol:        "BTC-USDT",
		CurrentPrice:  50000.0,
		MA50:          48000.0,
		MA200:         45000.0,
		RSI:           65.0,
		MACD:          100.0,
		MacdSignal:    80.0,
		MacdHistogram: 20.0,
		BBUpper:       52000.0,
		BBMiddle:      50000.0,
		BBLower:       48000.0,
		Volume24h:     1000000000.0,
		MarketCap:     1000000000000.0,
	}

	prompt := GetTechnicalAnalysisPrompt(data)
	assert.NotNil(t, prompt)
	assert.NotEmpty(t, prompt.SystemPrompt)
	assert.NotEmpty(t, prompt.UserPrompt)
	assert.Contains(t, prompt.UserPrompt, "BTC-USDT")
	assert.Contains(t, prompt.UserPrompt, "50000.00")
}

func TestGetNewsAnalysisPrompt(t *testing.T) {
	data := &NewsAnalysisData{
		Symbol:        "BTC-USDT",
		NewsTitle:     "Bitcoin Price Surges",
		NewsContent:   "Bitcoin price has increased by 10% in the last 24 hours.",
		Source:        "CoinDesk",
		PublishedAt:   "2024-01-01T12:00:00Z",
		Importance:    8,
	}

	prompt := GetNewsAnalysisPrompt(data)
	assert.NotNil(t, prompt)
	assert.NotEmpty(t, prompt.SystemPrompt)
	assert.NotEmpty(t, prompt.UserPrompt)
	assert.Contains(t, prompt.UserPrompt, "Bitcoin Price Surges")
	assert.Contains(t, prompt.UserPrompt, "CoinDesk")
}

func TestGetEconomicAnalysisPrompt(t *testing.T) {
	data := &EconomicAnalysisData{
		EventName:     "US Fed Interest Rate Decision",
		EventDate:     "2024-01-01",
		Actual:        5.25,
		Forecast:      5.25,
		Previous:      5.25,
		Currency:      "USD",
		Importance:    10,
	}

	prompt := GetEconomicAnalysisPrompt(data)
	assert.NotNil(t, prompt)
	assert.NotEmpty(t, prompt.SystemPrompt)
	assert.NotEmpty(t, prompt.UserPrompt)
	assert.Contains(t, prompt.UserPrompt, "US Fed Interest Rate Decision")
	assert.Contains(t, prompt.UserPrompt, "5.25")
}

func TestGetTradeDecisionPrompt(t *testing.T) {
	data := &TradeDecisionData{
		Symbol:           "BTC-USDT",
		Side:             "buy",
		EntryPrice:       50000.0,
		StopLoss:         48000.0,
		TakeProfit:       55000.0,
		PositionSize:     0.1,
		CurrentPrice:     50000.0,
		TimeFrame:        "1h",
		RiskRewardRatio:  2.5,
		MarketCondition:  "bullish",
	}

	prompt := GetTradeDecisionPrompt(data)
	assert.NotNil(t, prompt)
	assert.NotEmpty(t, prompt.SystemPrompt)
	assert.NotEmpty(t, prompt.UserPrompt)
	assert.Contains(t, prompt.UserPrompt, "BTC-USDT")
	assert.Contains(t, prompt.UserPrompt, "buy")
	assert.Contains(t, prompt.UserPrompt, "2.50")
}

func TestBuildMessages(t *testing.T) {
	template := &PromptTemplate{
		SystemPrompt: "You are a helpful assistant.",
		UserPrompt:   "Hello, how are you?",
	}

	messages := BuildMessages(template)
	assert.Len(t, messages, 2)
	assert.Equal(t, "system", messages[0].Role)
	assert.Equal(t, "You are a helpful assistant.", messages[0].Content)
	assert.Equal(t, "user", messages[1].Role)
	assert.Equal(t, "Hello, how are you?", messages[1].Content)
}

func TestParseAnalysisResult(t *testing.T) {
	content := `趋势判断：上涨
风险等级：中
关键支撑：48000
关键阻力：52000
交易建议：买入
详细分析：市场处于上涨趋势中，建议逢低买入。`

	result := ParseAnalysisResult(content)
	assert.NotNil(t, result)
	assert.Equal(t, "上涨", result["趋势判断"])
	assert.Equal(t, "中", result["风险等级"])
	assert.Equal(t, "48000", result["关键支撑"])
	assert.Equal(t, "52000", result["关键阻力"])
	assert.Equal(t, "买入", result["交易建议"])
	assert.Contains(t, result["详细分析"], "上涨趋势")
}

func TestParseAnalysisResultEmpty(t *testing.T) {
	content := ""
	result := ParseAnalysisResult(content)
	assert.NotNil(t, result)
	assert.Empty(t, result)
}

func TestParseAnalysisResultMultiline(t *testing.T) {
	content := `市场情绪：乐观
风险等级：低

市场热点：DeFi 项目爆发

投资建议：关注 DeFi 领域

详细分析：
当前市场情绪乐观，DeFi 项目表现活跃。
建议重点关注 DeFi 领域的投资机会。`

	result := ParseAnalysisResult(content)
	assert.NotNil(t, result)
	assert.Equal(t, "乐观", result["市场情绪"])
	assert.Equal(t, "低", result["风险等级"])
	assert.Equal(t, "DeFi 项目爆发", result["市场热点"])
	assert.Equal(t, "关注 DeFi 领域", result["投资建议"])
	assert.Contains(t, result["详细分析"], "DeFi 项目表现活跃")
}

func TestGetOrderAnalysisPrompt(t *testing.T) {
	data := &OrderData{
		Symbol:       "BTC-USDT",
		AnalysisType: "active",
		TimeRange:    "24h",
		Orders: []map[string]interface{}{
			{"id": "1", "side": "buy", "price": 50000.0},
			{"id": "2", "side": "sell", "price": 51000.0},
		},
	}

	prompt := GetOrderAnalysisPrompt(data)
	assert.NotNil(t, prompt)
	assert.NotEmpty(t, prompt.SystemPrompt)
	assert.NotEmpty(t, prompt.UserPrompt)
	assert.Contains(t, prompt.UserPrompt, "BTC-USDT")
	assert.Contains(t, prompt.UserPrompt, "active")
	assert.Contains(t, prompt.UserPrompt, "24h")
	assert.Contains(t, prompt.UserPrompt, "2") // order count
}

func TestGetOrderAnalysisPromptEmptySymbol(t *testing.T) {
	data := &OrderData{
		AnalysisType: "historical",
		TimeRange:    "7d",
		Orders:       []map[string]interface{}{},
	}

	prompt := GetOrderAnalysisPrompt(data)
	assert.NotNil(t, prompt)
	assert.Contains(t, prompt.UserPrompt, "historical")
	assert.Contains(t, prompt.UserPrompt, "0") // zero order count
}

func TestParseAnalysisResultNoColon(t *testing.T) {
	content := `This is just plain text without any colons.
It should return an empty map.`
	result := ParseAnalysisResult(content)
	assert.NotNil(t, result)
	assert.Empty(t, result)
}

func TestParseAnalysisResultMixedColons(t *testing.T) {
	content := "风险等级：高\nSome text without colon.\n最终建议：减仓\nMore text here."
	result := ParseAnalysisResult(content)
	assert.Contains(t, result["风险等级"], "高")
	assert.Contains(t, result["风险等级"], "Some text without colon.")
	assert.Contains(t, result["最终建议"], "减仓")
	assert.Contains(t, result["最终建议"], "More text here.")
}

func TestParseAnalysisResultFullWidthColon(t *testing.T) {
	// The parser uses full-width colon "："
	content := "key：value"
	result := ParseAnalysisResult(content)
	assert.Equal(t, "value", result["key"])
}

func TestParseAnalysisResultContinuationLines(t *testing.T) {
	content := `详细分析：第一行
第二行内容
第三行内容
其他字段：值`
	result := ParseAnalysisResult(content)
	assert.Contains(t, result["详细分析"], "第一行")
	assert.Contains(t, result["详细分析"], "第二行内容")
	assert.Contains(t, result["详细分析"], "第三行内容")
	assert.Equal(t, "值", result["其他字段"])
}

func TestPromptConstants(t *testing.T) {
	assert.Equal(t, PromptType("technical_analysis"), PromptTypeTechnicalAnalysis)
	assert.Equal(t, PromptType("news_analysis"), PromptTypeNewsAnalysis)
	assert.Equal(t, PromptType("economic_analysis"), PromptTypeEconomicAnalysis)
	assert.Equal(t, PromptType("trade_decision"), PromptTypeTradeDecision)
}

func TestAnalysisTypeConstants(t *testing.T) {
	assert.Equal(t, AnalysisType("trade"), AnalysisTypeTrade)
	assert.Equal(t, AnalysisType("position"), AnalysisTypePosition)
	assert.Equal(t, AnalysisType("market"), AnalysisTypeMarket)
	assert.Equal(t, AnalysisType("orders"), AnalysisTypeOrders)
}

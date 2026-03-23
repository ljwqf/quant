package llmanalysis

import (
	"fmt"
	"strings"
)

// PromptType 提示词类型
type PromptType string

const (
	// PromptTypeTechnicalAnalysis 技术分析提示词
	PromptTypeTechnicalAnalysis PromptType = "technical_analysis"
	// PromptTypeNewsAnalysis 新闻事件分析提示词
	PromptTypeNewsAnalysis PromptType = "news_analysis"
	// PromptTypeEconomicAnalysis 宏观经济数据分析提示词
	PromptTypeEconomicAnalysis PromptType = "economic_analysis"
	// PromptTypeTradeDecision 交易决策建议提示词
	PromptTypeTradeDecision PromptType = "trade_decision"
)

// PromptTemplate 提示词模板
type PromptTemplate struct {
	SystemPrompt string
	UserPrompt   string
}

// TechnicalAnalysisData 技术分析数据
type TechnicalAnalysisData struct {
	Symbol        string
	CurrentPrice  float64
	MA50          float64
	MA200         float64
	RSI           float64
	MACD          float64
	MacdSignal    float64
	MacdHistogram float64
	BBUpper       float64
	BBMiddle      float64
	BBLower       float64
	Volume24h     float64
	MarketCap     float64
}

// NewsAnalysisData 新闻分析数据
type NewsAnalysisData struct {
	Symbol    string
	NewsTitle string
	NewsContent string
	Source     string
	PublishedAt string
	Importance int
}

// EconomicAnalysisData 经济分析数据
type EconomicAnalysisData struct {
	EventName       string
	EventDate       string
	Actual         float64
	Forecast       float64
	Previous       float64
	Currency       string
	Importance     int
}

// TradeDecisionData 交易决策数据
type TradeDecisionData struct {
	Symbol           string
	Side             string
	EntryPrice       float64
	StopLoss         float64
	TakeProfit       float64
	PositionSize     float64
	CurrentPrice     float64
	TimeFrame        string
	RiskRewardRatio  float64
	MarketCondition  string
}

// GetTechnicalAnalysisPrompt 获取技术分析提示词
func GetTechnicalAnalysisPrompt(data *TechnicalAnalysisData) *PromptTemplate {
	systemPrompt := `你是一位资深的加密货币技术分析师，专注于技术指标分析和市场趋势判断。
请基于提供的技术指标数据，进行专业的技术分析，并给出明确的分析结论。

分析要求：
1. 综合分析所有技术指标（MA、RSI、MACD、布林带等）
2. 判断当前市场趋势（上涨、下跌、震荡）
3. 识别关键支撑位和阻力位
4. 评估市场风险程度（低、中、高）
5. 给出具体的交易建议（买入、卖出、观望）
6. 分析结果要客观、专业、有依据

输出格式：
- 趋势判断：[上涨/下跌/震荡]
- 风险等级：[低/中/高]
- 关键支撑：[价格]
- 关键阻力：[价格]
- 交易建议：[买入/卖出/观望]
- 详细分析：[具体分析内容]`

	userPrompt := fmt.Sprintf(`请分析以下技术指标数据：

交易对：%s
当前价格：%.2f
MA50：%.2f
MA200：%.2f
RSI (14)：%.2f
MACD：%.2f
MACD信号线：%.2f
MACD柱状图：%.2f
布林带上轨：%.2f
布林带中轨：%.2f
布林带下轨：%.2f
24小时成交量：%.2f
市值：%.2f

请基于以上数据进行技术分析。`,
		data.Symbol,
		data.CurrentPrice,
		data.MA50,
		data.MA200,
		data.RSI,
		data.MACD,
		data.MacdSignal,
		data.MacdHistogram,
		data.BBUpper,
		data.BBMiddle,
		data.BBLower,
		data.Volume24h,
		data.MarketCap,
	)

	return &PromptTemplate{
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
	}
}

// GetNewsAnalysisPrompt 获取新闻分析提示词
func GetNewsAnalysisPrompt(data *NewsAnalysisData) *PromptTemplate {
	systemPrompt := `你是一位专业的加密货币市场新闻分析师，擅长评估新闻事件对市场的影响。
请分析提供的新闻事件，评估其对相关加密货币的潜在影响。

分析要求：
1. 评估新闻事件的重要性（低、中、高）
2. 判断新闻对市场的影响方向（正面、负面、中性）
3. 分析可能的市场反应
4. 给出风险提示和建议
5. 分析结果要客观、及时、有深度

输出格式：
- 重要性评级：[低/中/高]
- 影响方向：[正面/负面/中性]
- 预期影响：[描述]
- 风险提示：[具体风险]
- 操作建议：[建议]
- 详细分析：[具体分析内容]`

	userPrompt := fmt.Sprintf(`请分析以下新闻事件：

交易对：%s
新闻标题：%s
新闻内容：%s
来源：%s
发布时间：%s
重要性评分：%d（1-10，10最高）

请基于以上信息进行新闻分析。`,
		data.Symbol,
		data.NewsTitle,
		data.NewsContent,
		data.Source,
		data.PublishedAt,
		data.Importance,
	)

	return &PromptTemplate{
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
	}
}

// GetEconomicAnalysisPrompt 获取经济分析提示词
func GetEconomicAnalysisPrompt(data *EconomicAnalysisData) *PromptTemplate {
	systemPrompt := `你是一位资深的宏观经济分析师，专注于经济数据对金融市场的影响分析。
请分析提供的经济事件数据，评估其对加密货币市场的潜在影响。

分析要求：
1. 评估经济数据与预期的差异
2. 判断对市场的影响方向（正面、负面、中性）
3. 分析可能的市场反应
4. 关联相关加密货币的可能表现
5. 给出风险管理建议

输出格式：
- 数据差异：[与预期的比较]
- 影响方向：[正面/负面/中性]
- 市场影响：[描述]
- 加密货币影响：[相关币种分析]
- 风险管理：[建议]
- 详细分析：[具体分析内容]`

	userPrompt := fmt.Sprintf(`请分析以下经济事件：

事件名称：%s
事件日期：%s
实际值：%.2f
预期值：%.2f
前值：%.2f
货币：%s
重要性：%d（1-10，10最高）

请基于以上数据进行经济分析。`,
		data.EventName,
		data.EventDate,
		data.Actual,
		data.Forecast,
		data.Previous,
		data.Currency,
		data.Importance,
	)

	return &PromptTemplate{
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
	}
}

// GetTradeDecisionPrompt 获取交易决策提示词
func GetTradeDecisionPrompt(data *TradeDecisionData) *PromptTemplate {
	systemPrompt := `你是一位经验丰富的加密货币交易顾问，擅长交易决策和风险管理。
请基于提供的交易信息，进行综合分析并给出专业的交易建议。

分析要求：
1. 评估交易的风险收益比
2. 分析入场点位的合理性
3. 评估止损和止盈设置
4. 判断当前市场条件是否适合此交易
5. 给出明确的交易建议（执行、调整、放弃）
6. 提供风险管理建议

输出格式：
- 风险收益比：[数值]
- 交易评级：[推荐/谨慎/不推荐]
- 入场建议：[评估]
- 止损止盈：[评估]
- 市场条件：[评估]
- 最终建议：[执行/调整/放弃]
- 详细分析：[具体分析内容]`

	userPrompt := fmt.Sprintf(`请分析以下交易机会：

交易对：%s
交易方向：%s
入场价格：%.2f
止损价格：%.2f
止盈价格：%.2f
仓位大小：%.2f
当前价格：%.2f
时间周期：%s
风险收益比：%.2f
市场条件：%s

请基于以上信息进行交易决策分析。`,
		data.Symbol,
		data.Side,
		data.EntryPrice,
		data.StopLoss,
		data.TakeProfit,
		data.PositionSize,
		data.CurrentPrice,
		data.TimeFrame,
		data.RiskRewardRatio,
		data.MarketCondition,
	)

	return &PromptTemplate{
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
	}
}

// BuildMessages 构建消息列表
func BuildMessages(template *PromptTemplate) []Message {
	messages := []Message{
		{
			Role:    "system",
			Content: template.SystemPrompt,
		},
		{
			Role:    "user",
			Content: template.UserPrompt,
		},
	}
	return messages
}

// Message 消息结构（与 providers 包兼容）
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ParseAnalysisResult 解析分析结果
func ParseAnalysisResult(content string) map[string]string {
	result := make(map[string]string)
	lines := strings.Split(content, "\n")
	
	var currentKey string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		
		if strings.Contains(line, "：") {
			parts := strings.SplitN(line, "：", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])
				result[key] = value
				currentKey = key
			}
		} else if currentKey != "" {
			if result[currentKey] != "" {
				result[currentKey] += "\n" + line
			} else {
				result[currentKey] = line
			}
		}
	}
	
	return result
}

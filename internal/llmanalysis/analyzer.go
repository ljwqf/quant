package llmanalysis

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ljwqf/quant/internal/config"
	"github.com/ljwqf/quant/internal/llmanalysis/providers"
	"github.com/ljwqf/quant/internal/storage"
	"github.com/ljwqf/quant/internal/storage/repository"
	"github.com/ljwqf/quant/pkg/logger"
	"go.uber.org/zap"
)

// AnalysisType 分析类型
type AnalysisType string

const (
	// AnalysisTypeTrade 交易前分析
	AnalysisTypeTrade AnalysisType = "trade"
	// AnalysisTypePosition 持仓分析
	AnalysisTypePosition AnalysisType = "position"
	// AnalysisTypeMarket 市场概览分析
	AnalysisTypeMarket AnalysisType = "market"
	// AnalysisTypeOrders 订单分析
	AnalysisTypeOrders AnalysisType = "orders"
)

// AnalysisResult 分析结果
type AnalysisResult struct {
	ID        int64     `json:"id"`
	Type      string    `json:"type"`
	Symbol    string    `json:"symbol"`
	Content   string    `json:"content"`
	Summary   string    `json:"summary"`
	RiskLevel string    `json:"risk_level"`
	CreatedAt time.Time `json:"created_at"`
}

// Analyzer 分析引擎
type Analyzer struct {
	client       *Client
	aiRepo       repository.AIAnalysisRepository
	model        string
}

// NewAnalyzer 创建分析引擎
func NewAnalyzer(client *Client, db *storage.Database, cfg *config.LLMConfig) *Analyzer {
	if client == nil {
		logger.Info("大模型客户端未初始化，分析引擎不可用")
		return nil
	}

	model := ""
	if cfg != nil {
		switch cfg.Provider {
		case "openai":
			model = cfg.Providers.OpenAI.Model
		case "claude":
			model = cfg.Providers.Claude.Model
		case "qwen":
			model = cfg.Providers.Qwen.Model
		}
		if model == "" {
			model = "gpt-4"
		}
	}

	return &Analyzer{
		client:       client,
		aiRepo:       repository.NewAIAnalysisRepository(db.DB()),
		model:        model,
	}
}

// AnalyzeTrade 交易前分析
func (a *Analyzer) AnalyzeTrade(ctx context.Context, data *TradeDecisionData) (*AnalysisResult, error) {
	if a.client == nil {
		return nil, fmt.Errorf("大模型客户端未初始化")
	}

	template := GetTradeDecisionPrompt(data)
	messages := BuildMessages(template)

	req := &providers.ChatRequest{
		Model:       a.model,
		Messages:    convertMessages(messages),
		Temperature: 0.7,
		MaxTokens:   2000,
	}

	resp, err := a.client.Chat(ctx, req)
	if err != nil {
		logger.Error("交易分析失败", zap.Error(err))
		return nil, fmt.Errorf("交易分析失败: %w", err)
	}

	parsed := ParseAnalysisResult(resp.Content)
	summary := parsed["最终建议"]
	if summary == "" {
		summary = parsed["交易评级"]
	}
	riskLevel := parsed["风险等级"]
	if riskLevel == "" {
		riskLevel = "medium"
	}

	result := &storage.AIAnalysis{
		AnalysisType: string(AnalysisTypeTrade),
		Symbol:       data.Symbol,
		Content:      resp.Content,
		Suggestions:  summary,
		RiskLevel:    riskLevel,
		CreatedAt:    time.Now(),
	}

	if err := a.aiRepo.Create(result); err != nil {
		logger.Warn("保存分析结果失败", zap.Error(err))
	}

	return &AnalysisResult{
		ID:        result.ID,
		Type:      result.AnalysisType,
		Symbol:    result.Symbol,
		Content:   result.Content,
		Summary:   result.Suggestions,
		RiskLevel: result.RiskLevel,
		CreatedAt: result.CreatedAt,
	}, nil
}

// AnalyzePosition 持仓分析
func (a *Analyzer) AnalyzePosition(ctx context.Context, symbol string, positionData map[string]interface{}) (*AnalysisResult, error) {
	if a.client == nil {
		return nil, fmt.Errorf("大模型客户端未初始化")
	}

	tradeData := &TradeDecisionData{
		Symbol:          symbol,
		Side:            "hold",
		MarketCondition: "existing_position",
	}

	template := GetTradeDecisionPrompt(tradeData)
	messages := BuildMessages(template)

	req := &providers.ChatRequest{
		Model:       a.model,
		Messages:    convertMessages(messages),
		Temperature: 0.7,
		MaxTokens:   1500,
	}

	resp, err := a.client.Chat(ctx, req)
	if err != nil {
		logger.Error("持仓分析失败", zap.Error(err))
		return nil, fmt.Errorf("持仓分析失败: %w", err)
	}

	parsed := ParseAnalysisResult(resp.Content)
	summary := parsed["最终建议"]
	if summary == "" {
		summary = parsed["交易评级"]
	}
	riskLevel := parsed["风险等级"]
	if riskLevel == "" {
		riskLevel = "medium"
	}

	result := &storage.AIAnalysis{
		AnalysisType: string(AnalysisTypePosition),
		Symbol:       symbol,
		Content:      resp.Content,
		Suggestions:  summary,
		RiskLevel:    riskLevel,
		CreatedAt:    time.Now(),
	}

	if err := a.aiRepo.Create(result); err != nil {
		logger.Warn("保存分析结果失败", zap.Error(err))
	}

	return &AnalysisResult{
		ID:        result.ID,
		Type:      result.AnalysisType,
		Symbol:    result.Symbol,
		Content:   result.Content,
		Summary:   result.Suggestions,
		RiskLevel: result.RiskLevel,
		CreatedAt: result.CreatedAt,
	}, nil
}

// AnalyzeMarket 市场概览分析
func (a *Analyzer) AnalyzeMarket(ctx context.Context, symbols []string) (*AnalysisResult, error) {
	if a.client == nil {
		return nil, fmt.Errorf("大模型客户端未初始化")
	}

	systemPrompt := `你是一位资深的加密货币市场分析师，擅长市场概览分析和趋势判断。
请基于当前市场状况，进行全面的市场分析。

分析要求：
1. 评估整体市场情绪（乐观、中性、谨慎）
2. 分析主要币种的趋势
3. 识别市场机会和风险
4. 给出投资建议
5. 分析结果要客观、全面、有深度

输出格式：
- 市场情绪：[乐观/中性/谨慎]
- 风险等级：[低/中/高]
- 市场热点：[描述]
- 投资建议：[建议]
- 详细分析：[具体分析内容]`

	userPrompt := fmt.Sprintf(`请分析当前加密货币市场。

关注币种：%v

请进行全面的市场概览分析。`, strings.Join(symbols, ", "))

	template := &PromptTemplate{
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
	}

	messages := BuildMessages(template)

	req := &providers.ChatRequest{
		Model:       a.model,
		Messages:    convertMessages(messages),
		Temperature: 0.7,
		MaxTokens:   2000,
	}

	resp, err := a.client.Chat(ctx, req)
	if err != nil {
		logger.Error("市场分析失败", zap.Error(err))
		return nil, fmt.Errorf("市场分析失败: %w", err)
	}

	parsed := ParseAnalysisResult(resp.Content)
	summary := parsed["投资建议"]
	if summary == "" {
		summary = parsed["市场情绪"]
	}
	riskLevel := parsed["风险等级"]
	if riskLevel == "" {
		riskLevel = "medium"
	}

	result := &storage.AIAnalysis{
		AnalysisType: string(AnalysisTypeMarket),
		Symbol:       "market",
		Content:      resp.Content,
		Suggestions:  summary,
		RiskLevel:    riskLevel,
		CreatedAt:    time.Now(),
	}

	if err := a.aiRepo.Create(result); err != nil {
		logger.Warn("保存分析结果失败", zap.Error(err))
	}

	return &AnalysisResult{
		ID:        result.ID,
		Type:      result.AnalysisType,
		Symbol:    result.Symbol,
		Content:   result.Content,
		Summary:   result.Suggestions,
		RiskLevel: result.RiskLevel,
		CreatedAt: result.CreatedAt,
	}, nil
}

// AnalyzeOrders 订单分析
func (a *Analyzer) AnalyzeOrders(ctx context.Context, data *OrderData) (*AnalysisResult, error) {
	if a.client == nil {
		return nil, fmt.Errorf("大模型客户端未初始化")
	}

	template := GetOrderAnalysisPrompt(data)
	messages := BuildMessages(template)

	req := &providers.ChatRequest{
		Model:       a.model,
		Messages:    convertMessages(messages),
		Temperature: 0.7,
		MaxTokens:   2000,
	}

	resp, err := a.client.Chat(ctx, req)
	if err != nil {
		logger.Error("订单分析失败", zap.Error(err))
		return nil, fmt.Errorf("订单分析失败: %w", err)
	}

	parsed := ParseAnalysisResult(resp.Content)
	summary := parsed["改进建议"]
	if summary == "" {
		summary = parsed["执行质量"]
	}
	riskLevel := parsed["风险等级"]
	if riskLevel == "" {
		riskLevel = "medium"
	}

	symbol := data.Symbol
	if symbol == "" {
		symbol = "all"
	}

	result := &storage.AIAnalysis{
		AnalysisType: string(AnalysisTypeOrders),
		Symbol:       symbol,
		Content:      resp.Content,
		Suggestions:  summary,
		RiskLevel:    riskLevel,
		CreatedAt:    time.Now(),
	}

	if err := a.aiRepo.Create(result); err != nil {
		logger.Warn("保存分析结果失败", zap.Error(err))
	}

	return &AnalysisResult{
		ID:        result.ID,
		Type:      result.AnalysisType,
		Symbol:    result.Symbol,
		Content:   result.Content,
		Summary:   result.Suggestions,
		RiskLevel: result.RiskLevel,
		CreatedAt: result.CreatedAt,
	}, nil
}

// GetLatestAnalysis 获取最新分析
func (a *Analyzer) GetLatestAnalysis(symbol string, analysisType AnalysisType) (*AnalysisResult, error) {
	record, err := a.aiRepo.GetLatestBySymbolAndType(symbol, string(analysisType))
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, nil
	}

	return &AnalysisResult{
		ID:        record.ID,
		Type:      record.AnalysisType,
		Symbol:    record.Symbol,
		Content:   record.Content,
		Summary:   record.Suggestions,
		RiskLevel: record.RiskLevel,
		CreatedAt: record.CreatedAt,
	}, nil
}

// ListAnalyses 列出分析记录
func (a *Analyzer) ListAnalyses(symbol string, limit int) ([]*AnalysisResult, error) {
	records, err := a.aiRepo.ListBySymbol(symbol, limit, 0)
	if err != nil {
		return nil, err
	}

	results := make([]*AnalysisResult, len(records))
	for i, record := range records {
		results[i] = &AnalysisResult{
			ID:        record.ID,
			Type:      record.AnalysisType,
			Symbol:    record.Symbol,
			Content:   record.Content,
			Summary:   record.Suggestions,
			RiskLevel: record.RiskLevel,
			CreatedAt: record.CreatedAt,
		}
	}

	return results, nil
}

// convertMessages 转换消息类型
func convertMessages(messages []Message) []providers.Message {
	result := make([]providers.Message, len(messages))
	for i, msg := range messages {
		result[i] = providers.Message{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}
	return result
}

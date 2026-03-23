package repository

import (
	"database/sql"
	"fmt"

	"github.com/ljwqf/quant/internal/storage"
)

// AIAnalysisRepository AI分析数据访问接口
type AIAnalysisRepository interface {
	Create(analysis *storage.AIAnalysis) error
	GetByID(id int64) (*storage.AIAnalysis, error)
	ListBySymbol(symbol string, limit, offset int) ([]*storage.AIAnalysis, error)
	ListByType(analysisType string, limit, offset int) ([]*storage.AIAnalysis, error)
	GetLatestBySymbolAndType(symbol, analysisType string) (*storage.AIAnalysis, error)
}

// aiAnalysisRepository AI分析数据访问实现
type aiAnalysisRepository struct {
	db *sql.DB
}

// NewAIAnalysisRepository 创建AI分析数据访问
func NewAIAnalysisRepository(db *sql.DB) AIAnalysisRepository {
	return &aiAnalysisRepository{db: db}
}

// Create 创建AI分析记录
func (r *aiAnalysisRepository) Create(analysis *storage.AIAnalysis) error {
	query := `
		INSERT INTO ai_analyses (
			symbol, analysis_type, provider, model, prompt, content, risk_level,
			suggestions, warnings, confidence_score, prompt_tokens, completion_tokens,
			total_tokens, latency_ms
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	result, err := r.db.Exec(query,
		analysis.Symbol, analysis.AnalysisType, analysis.Provider,
		analysis.Model, analysis.Prompt, analysis.Content, analysis.RiskLevel,
		analysis.Suggestions, analysis.Warnings, analysis.ConfidenceScore,
		analysis.PromptTokens, analysis.CompletionTokens,
		analysis.TotalTokens, analysis.LatencyMs,
	)
	if err != nil {
		return fmt.Errorf("创建AI分析记录失败: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("获取插入ID失败: %w", err)
	}

	analysis.ID = id
	return nil
}

// GetByID 根据ID获取AI分析记录
func (r *aiAnalysisRepository) GetByID(id int64) (*storage.AIAnalysis, error) {
	query := `
		SELECT id, symbol, analysis_type, provider, model, prompt, content, risk_level,
			suggestions, warnings, confidence_score, prompt_tokens, completion_tokens,
			total_tokens, latency_ms, created_at
		FROM ai_analyses WHERE id = ?
	`

	analysis := &storage.AIAnalysis{}
	err := r.db.QueryRow(query, id).Scan(
		&analysis.ID, &analysis.Symbol, &analysis.AnalysisType, &analysis.Provider,
		&analysis.Model, &analysis.Prompt, &analysis.Content, &analysis.RiskLevel,
		&analysis.Suggestions, &analysis.Warnings, &analysis.ConfidenceScore,
		&analysis.PromptTokens, &analysis.CompletionTokens, &analysis.TotalTokens,
		&analysis.LatencyMs, &analysis.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("查询AI分析记录失败: %w", err)
	}

	return analysis, nil
}

// ListBySymbol 根据交易对列出AI分析记录
func (r *aiAnalysisRepository) ListBySymbol(symbol string, limit, offset int) ([]*storage.AIAnalysis, error) {
	query := `
		SELECT id, symbol, analysis_type, provider, model, prompt, content, risk_level,
			suggestions, warnings, confidence_score, prompt_tokens, completion_tokens,
			total_tokens, latency_ms, created_at
		FROM ai_analyses WHERE symbol = ?
		ORDER BY created_at DESC LIMIT ? OFFSET ?
	`

	rows, err := r.db.Query(query, symbol, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("查询AI分析记录列表失败: %w", err)
	}
	defer rows.Close()

	var analyses []*storage.AIAnalysis
	for rows.Next() {
		analysis := &storage.AIAnalysis{}
		err := rows.Scan(
			&analysis.ID, &analysis.Symbol, &analysis.AnalysisType, &analysis.Provider,
			&analysis.Model, &analysis.Prompt, &analysis.Content, &analysis.RiskLevel,
			&analysis.Suggestions, &analysis.Warnings, &analysis.ConfidenceScore,
			&analysis.PromptTokens, &analysis.CompletionTokens, &analysis.TotalTokens,
			&analysis.LatencyMs, &analysis.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("扫描AI分析记录失败: %w", err)
		}
		analyses = append(analyses, analysis)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历AI分析记录失败: %w", err)
	}

	return analyses, nil
}

// ListByType 根据分析类型列出AI分析记录
func (r *aiAnalysisRepository) ListByType(analysisType string, limit, offset int) ([]*storage.AIAnalysis, error) {
	query := `
		SELECT id, symbol, analysis_type, provider, model, prompt, content, risk_level,
			suggestions, warnings, confidence_score, prompt_tokens, completion_tokens,
			total_tokens, latency_ms, created_at
		FROM ai_analyses WHERE analysis_type = ?
		ORDER BY created_at DESC LIMIT ? OFFSET ?
	`

	rows, err := r.db.Query(query, analysisType, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("查询AI分析记录列表失败: %w", err)
	}
	defer rows.Close()

	var analyses []*storage.AIAnalysis
	for rows.Next() {
		analysis := &storage.AIAnalysis{}
		err := rows.Scan(
			&analysis.ID, &analysis.Symbol, &analysis.AnalysisType, &analysis.Provider,
			&analysis.Model, &analysis.Prompt, &analysis.Content, &analysis.RiskLevel,
			&analysis.Suggestions, &analysis.Warnings, &analysis.ConfidenceScore,
			&analysis.PromptTokens, &analysis.CompletionTokens, &analysis.TotalTokens,
			&analysis.LatencyMs, &analysis.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("扫描AI分析记录失败: %w", err)
		}
		analyses = append(analyses, analysis)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历AI分析记录失败: %w", err)
	}

	return analyses, nil
}

// GetLatestBySymbolAndType 获取最新的AI分析记录
func (r *aiAnalysisRepository) GetLatestBySymbolAndType(symbol, analysisType string) (*storage.AIAnalysis, error) {
	query := `
		SELECT id, symbol, analysis_type, provider, model, prompt, content, risk_level,
			suggestions, warnings, confidence_score, prompt_tokens, completion_tokens,
			total_tokens, latency_ms, created_at
		FROM ai_analyses WHERE symbol = ? AND analysis_type = ?
		ORDER BY created_at DESC LIMIT 1
	`

	analysis := &storage.AIAnalysis{}
	err := r.db.QueryRow(query, symbol, analysisType).Scan(
		&analysis.ID, &analysis.Symbol, &analysis.AnalysisType, &analysis.Provider,
		&analysis.Model, &analysis.Prompt, &analysis.Content, &analysis.RiskLevel,
		&analysis.Suggestions, &analysis.Warnings, &analysis.ConfidenceScore,
		&analysis.PromptTokens, &analysis.CompletionTokens, &analysis.TotalTokens,
		&analysis.LatencyMs, &analysis.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("查询AI分析记录失败: %w", err)
	}

	return analysis, nil
}

package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ClaudeProvider Claude (Anthropic) 提供商
type ClaudeProvider struct {
	apiKey  string
	baseURL string
	client  *http.Client
	timeout time.Duration
}

// NewClaudeProvider 创建 Claude 提供商
func NewClaudeProvider() *ClaudeProvider {
	return &ClaudeProvider{
		baseURL: "https://api.anthropic.com/v1",
		timeout: 30 * time.Second,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Name 获取提供商名称
func (p *ClaudeProvider) Name() string {
	return "claude"
}

// claudeRequest Claude 请求结构
type claudeRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	MaxTokens   int       `json:"max_tokens"`
	Temperature float64   `json:"temperature,omitempty"`
}

// claudeResponse Claude 响应结构
type claudeResponse struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Role    string `json:"role"`
	Model   string `json:"model"`
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	StopReason   string `json:"stop_reason"`
	StopSequence string `json:"stop_sequence"`
	Usage        struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

// Chat 发送聊天请求
func (p *ClaudeProvider) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	if p.apiKey == "" {
		return nil, fmt.Errorf("Claude API key 未设置")
	}

	model := req.Model
	if model == "" {
		model = "claude-3-5-sonnet-20241022"
	}

	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}

	aiReq := claudeRequest{
		Model:       model,
		Messages:    req.Messages,
		MaxTokens:   maxTokens,
		Temperature: req.Temperature,
	}

	reqBody, err := json.Marshal(aiReq)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/messages", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("发送请求失败: %w", err)
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		if closeErr := resp.Body.Close(); closeErr != nil {
			return nil, fmt.Errorf("读取响应失败: %w (close body: %v)", err, closeErr)
		}
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}
	if closeErr := resp.Body.Close(); closeErr != nil {
		return nil, fmt.Errorf("关闭响应体失败: %w", closeErr)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API 请求失败 (status: %d): %s", resp.StatusCode, string(respBody))
	}

	var aiResp claudeResponse
	if err := json.Unmarshal(respBody, &aiResp); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	content := ""
	for _, c := range aiResp.Content {
		if c.Type == "text" {
			content += c.Text
		}
	}

	if content == "" {
		return nil, fmt.Errorf("API 返回空响应")
	}

	return &ChatResponse{
		ID:      aiResp.ID,
		Model:   aiResp.Model,
		Content: content,
		Usage: &Usage{
			PromptTokens:     aiResp.Usage.InputTokens,
			CompletionTokens: aiResp.Usage.OutputTokens,
			TotalTokens:      aiResp.Usage.InputTokens + aiResp.Usage.OutputTokens,
		},
	}, nil
}

// SetAPIKey 设置 API 密钥
func (p *ClaudeProvider) SetAPIKey(apiKey string) {
	p.apiKey = apiKey
}

// SetBaseURL 设置基础 URL
func (p *ClaudeProvider) SetBaseURL(baseURL string) {
	p.baseURL = baseURL
}

// SetTimeout 设置超时时间
func (p *ClaudeProvider) SetTimeout(timeout time.Duration) {
	p.timeout = timeout
	p.client.Timeout = timeout
}

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

// QwenProvider 阿里百炼 Qwen 提供商
type QwenProvider struct {
	apiKey  string
	baseURL string
	client  *http.Client
	timeout time.Duration
}

// NewQwenProvider 创建阿里百炼 Qwen 提供商
func NewQwenProvider() *QwenProvider {
	return &QwenProvider{
		baseURL: "https://dashscope.aliyuncs.com/compatible-mode/v1",
		timeout: 30 * time.Second,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Name 获取提供商名称
func (p *QwenProvider) Name() string {
	return "qwen"
}

// qwenRequest Qwen 请求结构（兼容 OpenAI 格式）
type qwenRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
}

// qwenResponse Qwen 响应结构（兼容 OpenAI 格式）
type qwenResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// Chat 发送聊天请求
func (p *QwenProvider) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	if p.apiKey == "" {
		return nil, fmt.Errorf("Qwen API key 未设置")
	}

	model := req.Model
	if model == "" {
		model = "qwen-turbo"
	}

	aiReq := qwenRequest{
		Model:       model,
		Messages:    req.Messages,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
	}

	reqBody, err := json.Marshal(aiReq)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/chat/completions", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

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

	var aiResp qwenResponse
	if err := json.Unmarshal(respBody, &aiResp); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	if len(aiResp.Choices) == 0 {
		return nil, fmt.Errorf("API 返回空响应")
	}

	return &ChatResponse{
		ID:      aiResp.ID,
		Model:   aiResp.Model,
		Content: aiResp.Choices[0].Message.Content,
		Usage: &Usage{
			PromptTokens:     aiResp.Usage.PromptTokens,
			CompletionTokens: aiResp.Usage.CompletionTokens,
			TotalTokens:      aiResp.Usage.TotalTokens,
		},
	}, nil
}

// SetAPIKey 设置 API 密钥
func (p *QwenProvider) SetAPIKey(apiKey string) {
	p.apiKey = apiKey
}

// SetBaseURL 设置基础 URL
func (p *QwenProvider) SetBaseURL(baseURL string) {
	p.baseURL = baseURL
}

// SetTimeout 设置超时时间
func (p *QwenProvider) SetTimeout(timeout time.Duration) {
	p.timeout = timeout
	p.client.Timeout = timeout
}

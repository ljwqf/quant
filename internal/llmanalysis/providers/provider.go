package providers

import (
	"context"
	"time"
)

// Message 聊天消息
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatRequest 聊天请求
type ChatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
}

// ChatResponse 聊天响应
type ChatResponse struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	Content string `json:"content"`
	Usage   *Usage `json:"usage,omitempty"`
}

// Usage 使用量
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// Provider 大模型提供商接口
type Provider interface {
	// Name 获取提供商名称
	Name() string
	
	// Chat 发送聊天请求
	Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error)
	
	// SetAPIKey 设置 API 密钥
	SetAPIKey(apiKey string)
	
	// SetBaseURL 设置基础 URL
	SetBaseURL(baseURL string)
	
	// SetTimeout 设置超时时间
	SetTimeout(timeout time.Duration)
}

package llmanalysis

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ljwqf/quant/internal/config"
)

func TestDefaultModelForProvider(t *testing.T) {
	assert.Equal(t, "gpt-4", defaultModelForProvider("openai"))
	assert.Equal(t, "claude-3-5-sonnet", defaultModelForProvider("claude"))
	assert.Equal(t, "qwen-plus", defaultModelForProvider("qwen"))
	assert.Equal(t, "gpt-4", defaultModelForProvider(""))
}

func TestNewAnalyzerUsesDefaultModelWhenConfigNil(t *testing.T) {
	a := NewAnalyzer(&Client{}, nil, nil)
	if assert.NotNil(t, a) {
		assert.Equal(t, "gpt-4", a.model)
	}
}

func TestNewAnalyzerUsesProviderDefaultWhenModelEmpty(t *testing.T) {
	cfg := &config.LLMConfig{Provider: "claude"}
	a := NewAnalyzer(&Client{}, nil, cfg)
	if assert.NotNil(t, a) {
		assert.Equal(t, "claude-3-5-sonnet", a.model)
	}
}

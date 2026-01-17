package providers

import (
	"context"
)

// Provider defines the interface for LLM providers
type Provider interface {
	// Name returns the provider name (e.g., "openai", "gemini", "ollama")
	Name() string

	// Ask sends a prompt and streams the response via callback
	Ask(ctx context.Context, prompt string, callback func(string)) error

	// AskNonStreaming sends a prompt and returns the full response
	AskNonStreaming(ctx context.Context, prompt string) (string, error)

	// IsReady returns true if the provider is configured and ready
	IsReady() bool

	// GetModel returns the current model name
	GetModel() string

	// ListModels returns available models for this provider (optional)
	ListModels(ctx context.Context) ([]string, error)
}

// ChatMessage represents a message in a conversation
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ProviderConfig holds common configuration for all providers
type ProviderConfig struct {
	Provider       string `yaml:"provider" json:"provider"`
	Model          string `yaml:"model" json:"model"`
	Endpoint       string `yaml:"endpoint" json:"endpoint"`
	APIKey         string `yaml:"api_key" json:"api_key"`
	Region         string `yaml:"region" json:"region"`                     // For AWS Bedrock
	AzureDeployment string `yaml:"azure_deployment" json:"azure_deployment"` // For Azure OpenAI
	SkipTLSVerify  bool   `yaml:"skip_tls_verify" json:"skip_tls_verify"`
}

// RetryConfig holds retry configuration
type RetryConfig struct {
	MaxAttempts int     `yaml:"max_attempts" json:"max_attempts"`
	MaxBackoff  float64 `yaml:"max_backoff" json:"max_backoff"`   // seconds
	JitterRatio float64 `yaml:"jitter_ratio" json:"jitter_ratio"` // 0.0 - 1.0
}

// DefaultRetryConfig returns default retry configuration
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxAttempts: 5,
		MaxBackoff:  10.0,
		JitterRatio: 0.1,
	}
}

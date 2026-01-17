package ai

import (
	"context"
	"fmt"

	"github.com/kube-ai-dashbaord/kube-ai-dashboard-cli/pkg/ai/providers"
	"github.com/kube-ai-dashbaord/kube-ai-dashboard-cli/pkg/config"
)

// Client wraps an LLM provider with additional functionality
type Client struct {
	cfg      *config.LLMConfig
	provider providers.Provider
}

// NewClient creates a new AI client using the provider factory
func NewClient(cfg *config.LLMConfig) (*Client, error) {
	providerCfg := &providers.ProviderConfig{
		Provider:        cfg.Provider,
		Model:           cfg.Model,
		Endpoint:        cfg.Endpoint,
		APIKey:          cfg.APIKey,
		Region:          cfg.Region,
		AzureDeployment: cfg.AzureDeployment,
		SkipTLSVerify:   cfg.SkipTLSVerify,
	}

	factory := providers.GetFactory()
	provider, err := factory.Create(providerCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create provider: %w", err)
	}

	// Wrap with retry logic if configured
	if cfg.RetryEnabled {
		retryCfg := &providers.RetryConfig{
			MaxAttempts: cfg.MaxRetries,
			MaxBackoff:  cfg.MaxBackoff,
			JitterRatio: 0.1,
		}
		if retryCfg.MaxAttempts == 0 {
			retryCfg.MaxAttempts = 5
		}
		if retryCfg.MaxBackoff == 0 {
			retryCfg.MaxBackoff = 10.0
		}
		provider = providers.CreateWithRetry(provider, retryCfg)
	}

	return &Client{
		cfg:      cfg,
		provider: provider,
	}, nil
}

// Ask sends a prompt to the AI provider and streams the response via callback
func (c *Client) Ask(ctx context.Context, prompt string, callback func(string)) error {
	if c.provider == nil {
		return fmt.Errorf("AI provider not initialized")
	}
	return c.provider.Ask(ctx, prompt, callback)
}

// AskNonStreaming sends a prompt and returns the full response
func (c *Client) AskNonStreaming(ctx context.Context, prompt string) (string, error) {
	if c.provider == nil {
		return "", fmt.Errorf("AI provider not initialized")
	}
	return c.provider.AskNonStreaming(ctx, prompt)
}

// CheckStatus verifies the AI provider is responding
func (c *Client) CheckStatus(ctx context.Context) error {
	_, err := c.AskNonStreaming(ctx, "ping")
	return err
}

// ListModels returns available models from the provider
func (c *Client) ListModels(ctx context.Context) ([]string, error) {
	if c.provider == nil {
		return nil, fmt.Errorf("AI provider not initialized")
	}
	return c.provider.ListModels(ctx)
}

// IsReady returns true if the client is configured and ready to use
func (c *Client) IsReady() bool {
	if c == nil || c.provider == nil {
		return false
	}
	return c.provider.IsReady()
}

// GetModel returns the current model name
func (c *Client) GetModel() string {
	if c == nil || c.provider == nil {
		return ""
	}
	return c.provider.GetModel()
}

// GetProvider returns the current provider name
func (c *Client) GetProvider() string {
	if c == nil || c.provider == nil {
		return ""
	}
	return c.provider.Name()
}

// GetAvailableProviders returns a list of available provider names
func GetAvailableProviders() string {
	return providers.GetFactory().ListProviders()
}

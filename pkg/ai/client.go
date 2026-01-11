package ai

import (
	"context"
	"fmt"

	"net/url"

	"github.com/GoogleCloudPlatform/kubectl-ai/gollm"
	"github.com/kube-ai-dashbaord/kube-ai-dashboard-cli/pkg/config"
)

type Client struct {
	llm gollm.Client
	cfg *config.LLMConfig
}

func NewClient(cfg *config.LLMConfig) (*Client, error) {
	var client gollm.Client
	var err error

	ctx := context.Background()
	u, err := url.Parse(cfg.Endpoint)
	if err != nil && cfg.Endpoint != "" {
		return nil, err
	}

	opts := gollm.ClientOptions{
		URL: u,
	}

	switch cfg.Provider {
	case "openai":
		client, err = gollm.NewOpenAIClient(ctx, opts)
	case "ollama":
		client, err = gollm.NewOllamaClient(ctx, opts)
	default:
		return nil, fmt.Errorf("unsupported provider: %s", cfg.Provider)
	}

	if err != nil {
		return nil, err
	}

	return &Client{
		llm: client,
		cfg: cfg,
	}, nil
}

func (c *Client) Ask(ctx context.Context, prompt string, callback func(string)) error {
	chat := c.llm.StartChat("You are a helpful Kubernetes assistant.", c.cfg.Model)

	stream, err := chat.SendStreaming(ctx, prompt)
	if err != nil {
		return err
	}

	for response, err := range stream {
		if err != nil {
			return err
		}
		if response == nil {
			break
		}
		if len(response.Candidates()) > 0 {
			candidate := response.Candidates()[0]
			for _, part := range candidate.Parts() {
				if text, ok := part.AsText(); ok {
					callback(text)
				}
			}
		}
	}

	return nil
}

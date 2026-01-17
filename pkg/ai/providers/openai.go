package providers

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// OpenAIProvider implements the Provider interface for OpenAI and compatible APIs
type OpenAIProvider struct {
	config     *ProviderConfig
	httpClient *http.Client
	endpoint   string
}

type openAIChatRequest struct {
	Model    string           `json:"model"`
	Messages []ChatMessage    `json:"messages"`
	Stream   bool             `json:"stream"`
	Tools    []ToolDefinition `json:"tools,omitempty"`
}

type openAIChatResponse struct {
	ID      string `json:"id"`
	Choices []struct {
		Message struct {
			Content   string     `json:"content"`
			ToolCalls []ToolCall `json:"tool_calls,omitempty"`
		} `json:"message"`
		Delta struct {
			Content   string     `json:"content"`
			ToolCalls []ToolCall `json:"tool_calls,omitempty"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
}

type openAIModelsResponse struct {
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
}

// NewOpenAIProvider creates a new OpenAI provider
func NewOpenAIProvider(cfg *ProviderConfig) (Provider, error) {
	endpoint := cfg.Endpoint
	if endpoint == "" {
		endpoint = "https://api.openai.com/v1"
	}
	endpoint = strings.TrimSuffix(endpoint, "/")

	return &OpenAIProvider{
		config:     cfg,
		httpClient: newHTTPClient(cfg.SkipTLSVerify),
		endpoint:   endpoint,
	}, nil
}

func (p *OpenAIProvider) Name() string {
	return "openai"
}

func (p *OpenAIProvider) GetModel() string {
	return p.config.Model
}

func (p *OpenAIProvider) IsReady() bool {
	return p.config != nil && p.config.APIKey != ""
}

func (p *OpenAIProvider) Ask(ctx context.Context, prompt string, callback func(string)) error {
	endpoint := p.endpoint + "/chat/completions"

	reqBody := openAIChatRequest{
		Model: p.config.Model,
		Messages: []ChatMessage{
			{Role: "system", Content: "You are a helpful Kubernetes assistant. Help users manage Kubernetes clusters using natural language. When users ask to create resources, generate the appropriate kubectl commands."},
			{Role: "user", Content: prompt},
		},
		Stream: true,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.config.APIKey)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	reader := bufio.NewReader(resp.Body)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("error reading response: %w", err)
		}

		line = strings.TrimSpace(line)
		if line == "" || !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var chatResp openAIChatResponse
		if err := json.Unmarshal([]byte(data), &chatResp); err != nil {
			continue
		}

		for _, choice := range chatResp.Choices {
			if choice.Delta.Content != "" {
				callback(choice.Delta.Content)
			}
		}
	}

	return nil
}

func (p *OpenAIProvider) AskNonStreaming(ctx context.Context, prompt string) (string, error) {
	endpoint := p.endpoint + "/chat/completions"

	reqBody := openAIChatRequest{
		Model: p.config.Model,
		Messages: []ChatMessage{
			{Role: "system", Content: "You are a helpful Kubernetes assistant."},
			{Role: "user", Content: prompt},
		},
		Stream: false,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.config.APIKey)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var chatResp openAIChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("no response from API")
	}

	return chatResp.Choices[0].Message.Content, nil
}

func (p *OpenAIProvider) ListModels(ctx context.Context) ([]string, error) {
	endpoint := p.endpoint + "/models"

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+p.config.APIKey)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to list models: status %d", resp.StatusCode)
	}

	var modelsResp openAIModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	models := make([]string, len(modelsResp.Data))
	for i, m := range modelsResp.Data {
		models[i] = m.ID
	}
	return models, nil
}

// AskWithTools implements the ToolProvider interface for agentic tool calling
func (p *OpenAIProvider) AskWithTools(ctx context.Context, prompt string, tools []ToolDefinition, callback func(string), toolCallback ToolCallback) error {
	endpoint := p.endpoint + "/chat/completions"

	messages := []ChatMessage{
		{Role: "system", Content: `You are a helpful Kubernetes assistant with access to tools for managing clusters.
When users ask about Kubernetes resources, use the kubectl tool to get information or make changes.
Always use tools when you need to interact with the cluster - don't just suggest commands.
After executing a tool, summarize the results for the user.`},
		{Role: "user", Content: prompt},
	}

	// Agentic loop - continue until no more tool calls
	maxIterations := 10
	for i := 0; i < maxIterations; i++ {
		reqBody := openAIChatRequest{
			Model:    p.config.Model,
			Messages: messages,
			Stream:   true,
			Tools:    tools,
		}

		jsonBody, err := json.Marshal(reqBody)
		if err != nil {
			return fmt.Errorf("failed to marshal request: %w", err)
		}

		req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewBuffer(jsonBody))
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+p.config.APIKey)

		resp, err := p.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("request failed: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
		}

		// Parse streaming response
		var contentBuilder strings.Builder
		var toolCalls []ToolCall
		var finishReason string

		reader := bufio.NewReader(resp.Body)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					break
				}
				resp.Body.Close()
				return fmt.Errorf("error reading response: %w", err)
			}

			line = strings.TrimSpace(line)
			if line == "" || !strings.HasPrefix(line, "data: ") {
				continue
			}

			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				break
			}

			var chatResp openAIChatResponse
			if err := json.Unmarshal([]byte(data), &chatResp); err != nil {
				continue
			}

			for _, choice := range chatResp.Choices {
				// Handle content
				if choice.Delta.Content != "" {
					contentBuilder.WriteString(choice.Delta.Content)
					if callback != nil {
						callback(choice.Delta.Content)
					}
				}

				// Handle tool calls (accumulate)
				if len(choice.Delta.ToolCalls) > 0 {
					for _, tc := range choice.Delta.ToolCalls {
						// Find existing tool call by index or add new
						found := false
						for idx := range toolCalls {
							if toolCalls[idx].ID == tc.ID || (tc.ID == "" && idx == len(toolCalls)-1) {
								// Append arguments
								toolCalls[idx].Function.Arguments += tc.Function.Arguments
								if tc.Function.Name != "" {
									toolCalls[idx].Function.Name = tc.Function.Name
								}
								if tc.ID != "" {
									toolCalls[idx].ID = tc.ID
								}
								found = true
								break
							}
						}
						if !found && (tc.ID != "" || tc.Function.Name != "") {
							toolCalls = append(toolCalls, tc)
						}
					}
				}

				if choice.FinishReason != "" {
					finishReason = choice.FinishReason
				}
			}
		}
		resp.Body.Close()

		content := contentBuilder.String()

		// If no tool calls, we're done
		if finishReason != "tool_calls" || len(toolCalls) == 0 {
			return nil
		}

		// Add assistant message with tool calls to history
		assistantMsg := ChatMessage{
			Role:      "assistant",
			Content:   content,
			ToolCalls: toolCalls,
		}
		messages = append(messages, assistantMsg)

		// Execute each tool call and add results
		for _, tc := range toolCalls {
			if callback != nil {
				callback(fmt.Sprintf("\n\n🔧 Executing: %s\n", tc.Function.Name))
			}

			result := toolCallback(tc)

			// Add tool result to messages
			toolMsg := ChatMessage{
				Role:       "tool",
				Content:    result.Content,
				ToolCallID: tc.ID,
			}
			messages = append(messages, toolMsg)

			if callback != nil {
				if result.IsError {
					callback(fmt.Sprintf("❌ Error: %s\n", result.Content))
				} else {
					// Truncate long outputs
					output := result.Content
					if len(output) > 1000 {
						output = output[:1000] + "\n... (truncated)"
					}
					callback(fmt.Sprintf("```\n%s\n```\n", output))
				}
			}
		}
	}

	return fmt.Errorf("exceeded maximum tool call iterations")
}

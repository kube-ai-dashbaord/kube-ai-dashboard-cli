package ai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kube-ai-dashbaord/kube-ai-dashboard-cli/pkg/config"
)

func TestNewClient(t *testing.T) {
	cfg := &config.LLMConfig{
		Provider: "openai",
		Model:    "gpt-4",
		Endpoint: "http://localhost:8080",
		APIKey:   "test-key",
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	if client == nil {
		t.Fatal("expected client to be created")
	}

	if client.cfg != cfg {
		t.Error("client config doesn't match")
	}
}

func TestClient_IsReady(t *testing.T) {
	tests := []struct {
		name string
		cfg  *config.LLMConfig
		want bool
	}{
		{
			name: "nil client",
			cfg:  nil,
			want: false,
		},
		{
			name: "valid config",
			cfg: &config.LLMConfig{
				Provider: "openai",
				Model:    "gpt-4",
				Endpoint: "http://localhost:8080",
				APIKey:   "test-key",
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var client *Client
			if tt.cfg != nil {
				client, _ = NewClient(tt.cfg)
			}
			got := client.IsReady()
			if got != tt.want {
				t.Errorf("IsReady() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClient_GetModel(t *testing.T) {
	tests := []struct {
		name string
		cfg  *config.LLMConfig
		want string
	}{
		{
			name: "nil client",
			cfg:  nil,
			want: "",
		},
		{
			name: "valid config",
			cfg: &config.LLMConfig{
				Provider: "openai",
				Model:    "gpt-4",
				Endpoint: "http://localhost:8080",
				APIKey:   "test-key",
			},
			want: "gpt-4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var client *Client
			if tt.cfg != nil {
				client, _ = NewClient(tt.cfg)
			}
			got := client.GetModel()
			if got != tt.want {
				t.Errorf("GetModel() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClient_GetProvider(t *testing.T) {
	tests := []struct {
		name string
		cfg  *config.LLMConfig
		want string
	}{
		{
			name: "nil client",
			cfg:  nil,
			want: "",
		},
		{
			name: "valid config",
			cfg: &config.LLMConfig{
				Provider: "openai",
				Model:    "gpt-4",
				Endpoint: "http://localhost:8080",
				APIKey:   "test-key",
			},
			want: "openai",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var client *Client
			if tt.cfg != nil {
				client, _ = NewClient(tt.cfg)
			}
			got := client.GetProvider()
			if got != tt.want {
				t.Errorf("GetProvider() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClient_AskNonStreaming(t *testing.T) {
	// Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != "POST" {
			t.Errorf("expected POST method, got %s", r.Method)
		}

		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected application/json content-type")
		}

		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("expected Bearer test-key authorization")
		}

		// Return mock response in OpenAI format
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"test-123","choices":[{"message":{"content":"Hello from AI"},"finish_reason":"stop"}]}`))
	}))
	defer server.Close()

	cfg := &config.LLMConfig{
		Provider: "openai",
		Model:    "gpt-4",
		Endpoint: server.URL,
		APIKey:   "test-key",
	}

	client, _ := NewClient(cfg)

	response, err := client.AskNonStreaming(context.Background(), "Hello")
	if err != nil {
		t.Fatalf("AskNonStreaming() error = %v", err)
	}

	if response != "Hello from AI" {
		t.Errorf("AskNonStreaming() = %v, want 'Hello from AI'", response)
	}
}

func TestClient_AskNonStreaming_Error(t *testing.T) {
	// Create a mock server that returns an error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal server error"))
	}))
	defer server.Close()

	cfg := &config.LLMConfig{
		Provider: "openai",
		Model:    "gpt-4",
		Endpoint: server.URL,
		APIKey:   "test-key",
	}

	client, _ := NewClient(cfg)

	_, err := client.AskNonStreaming(context.Background(), "Hello")
	if err == nil {
		t.Error("expected error for 500 response")
	}
}

func TestClient_Ask_Streaming(t *testing.T) {
	// Create a mock server for streaming
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)

		// Send streaming response
		resp1 := `{"id":"test","choices":[{"delta":{"content":"Hello"},"finish_reason":null}]}`
		resp2 := `{"id":"test","choices":[{"delta":{"content":" World"},"finish_reason":null}]}`

		w.Write([]byte("data: " + resp1 + "\n\n"))
		flusher.Flush()
		w.Write([]byte("data: " + resp2 + "\n\n"))
		flusher.Flush()
		w.Write([]byte("data: [DONE]\n\n"))
		flusher.Flush()
	}))
	defer server.Close()

	cfg := &config.LLMConfig{
		Provider: "openai",
		Model:    "gpt-4",
		Endpoint: server.URL,
		APIKey:   "test-key",
	}

	client, _ := NewClient(cfg)

	var result string
	err := client.Ask(context.Background(), "Hello", func(chunk string) {
		result += chunk
	})

	if err != nil {
		t.Fatalf("Ask() error = %v", err)
	}

	if result != "Hello World" {
		t.Errorf("Ask() streamed result = %v, want 'Hello World'", result)
	}
}

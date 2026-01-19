package providers

import (
	"context"
	"errors"
	"testing"
)

// mockProvider implements Provider for testing
type mockProvider struct {
	name           string
	model          string
	ready          bool
	askErr         error
	askContent     string
	supportsTools  bool
	toolsErr       error
}

func (m *mockProvider) Name() string                                      { return m.name }
func (m *mockProvider) GetModel() string                                  { return m.model }
func (m *mockProvider) IsReady() bool                                     { return m.ready }
func (m *mockProvider) ListModels(ctx context.Context) ([]string, error)  { return nil, nil }

func (m *mockProvider) Ask(ctx context.Context, prompt string, callback func(string)) error {
	if m.askErr != nil {
		return m.askErr
	}
	if callback != nil && m.askContent != "" {
		callback(m.askContent)
	}
	return nil
}

func (m *mockProvider) AskNonStreaming(ctx context.Context, prompt string) (string, error) {
	if m.askErr != nil {
		return "", m.askErr
	}
	return m.askContent, nil
}

// mockToolProvider implements both Provider and ToolProvider
type mockToolProvider struct {
	mockProvider
	toolsErr       error
	toolCallsCount int
}

func (m *mockToolProvider) AskWithTools(ctx context.Context, prompt string, tools []ToolDefinition, callback func(string), toolCallback ToolCallback) error {
	m.toolCallsCount++
	return m.toolsErr
}

func TestRetryProviderSupportsTools(t *testing.T) {
	tests := []struct {
		name          string
		provider      Provider
		wantSupports  bool
	}{
		{
			name:         "provider without tool support",
			provider:     &mockProvider{name: "mock", ready: true},
			wantSupports: false,
		},
		{
			name:         "provider with tool support",
			provider:     &mockToolProvider{mockProvider: mockProvider{name: "mock-tools", ready: true}},
			wantSupports: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			retryProv := CreateWithRetry(tt.provider, DefaultRetryConfig())
			rp := retryProv.(*retryProvider)

			if got := rp.SupportsTools(); got != tt.wantSupports {
				t.Errorf("SupportsTools() = %v, want %v", got, tt.wantSupports)
			}
		})
	}
}

func TestRetryProviderAskWithTools(t *testing.T) {
	tests := []struct {
		name        string
		provider    Provider
		wantErr     bool
		errContains string
	}{
		{
			name:        "provider without tool support returns error",
			provider:    &mockProvider{name: "mock", ready: true},
			wantErr:     true,
			errContains: "does not support tool calling",
		},
		{
			name:        "provider with tool support succeeds",
			provider:    &mockToolProvider{mockProvider: mockProvider{name: "mock-tools", ready: true}},
			wantErr:     false,
		},
		{
			name: "provider with tool error is retried",
			provider: &mockToolProvider{
				mockProvider: mockProvider{name: "mock-tools", ready: true},
				toolsErr:     errors.New("status 503: service unavailable"),
			},
			wantErr:     true,
			errContains: "max retries exceeded",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &RetryConfig{MaxAttempts: 2, MaxBackoff: 0.001, JitterRatio: 0}
			retryProv := CreateWithRetry(tt.provider, cfg)

			// Type assert to access AskWithTools
			toolProv, ok := retryProv.(ToolProvider)
			if !ok {
				t.Fatal("retryProvider should implement ToolProvider")
			}

			err := toolProv.AskWithTools(context.Background(), "test", nil, nil, nil)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error but got nil")
				} else if tt.errContains != "" && !containsString(err.Error(), tt.errContains) {
					t.Errorf("error = %v, want to contain %q", err, tt.errContains)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestRetryProviderImplementsToolProvider(t *testing.T) {
	// Verify that retryProvider implements ToolProvider at compile time
	var _ ToolProvider = (*retryProvider)(nil)
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStringHelper(s, substr))
}

func containsStringHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

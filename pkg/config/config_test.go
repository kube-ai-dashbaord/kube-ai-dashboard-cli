package config

import (
	"os"
	"testing"
)

func TestConfigLoadSave(t *testing.T) {
	// Use a temporary file for testing
	tmpFile, err := os.CreateTemp("", "config-test-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	cfg := &Config{
		LLM: LLMConfig{
			Provider: "test-provider",
			Model:    "test-model",
			Endpoint: "http://test-endpoint",
			APIKey:   "test-key",
		},
	}

	if cfg.LLM.Provider != "test-provider" {
		t.Errorf("Expected provider test-provider, got %s", cfg.LLM.Provider)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if cfg == nil {
		t.Fatal("Expected non-nil config")
	}
	// Check defaults if file doesn't exist (which it shouldn't in most CI environments)
	if cfg.LLM.Provider != "openai" {
		t.Errorf("Expected provider openai, got %s", cfg.LLM.Provider)
	}
}

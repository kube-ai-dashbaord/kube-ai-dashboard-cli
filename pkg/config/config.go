package config

import (
	"os"
	"path/filepath"

	"github.com/adrg/xdg"
	"gopkg.in/yaml.v3"
)

type Config struct {
	LLM          LLMConfig `yaml:"llm" json:"llm"`
	ReportPath   string    `yaml:"report_path" json:"report_path"`
	EnableAudit  bool      `yaml:"enable_audit" json:"enable_audit"`
	Language     string    `yaml:"language" json:"language"`
	BeginnerMode bool      `yaml:"beginner_mode" json:"beginner_mode"`
	LogLevel     string    `yaml:"log_level" json:"log_level"`
}

type LLMConfig struct {
	Provider        string  `yaml:"provider" json:"provider"`
	Model           string  `yaml:"model" json:"model"`
	Endpoint        string  `yaml:"endpoint" json:"endpoint"`
	APIKey          string  `yaml:"api_key" json:"api_key"`
	Region          string  `yaml:"region" json:"region"`                       // For AWS Bedrock
	AzureDeployment string  `yaml:"azure_deployment" json:"azure_deployment"`   // For Azure OpenAI
	SkipTLSVerify   bool    `yaml:"skip_tls_verify" json:"skip_tls_verify"`
	RetryEnabled    bool    `yaml:"retry_enabled" json:"retry_enabled"`
	MaxRetries      int     `yaml:"max_retries" json:"max_retries"`
	MaxBackoff      float64 `yaml:"max_backoff" json:"max_backoff"`             // seconds
}

func GetConfigPath() string {
	return filepath.Join(xdg.ConfigHome, "k13s", "config.yaml")
}

// GetConfigDir returns the k13s configuration directory
func GetConfigDir() (string, error) {
	dir := filepath.Join(xdg.ConfigHome, "k13s")
	return dir, nil
}

func NewDefaultConfig() *Config {
	return &Config{
		LLM: LLMConfig{
			Provider:     "openai",
			Model:        "gpt-4",
			RetryEnabled: true,
			MaxRetries:   5,
			MaxBackoff:   10.0,
		},
		Language:     "en",
		BeginnerMode: true,
		LogLevel:     "debug",
		ReportPath:   "report.md",
		EnableAudit:  true,
	}
}

func LoadConfig() (*Config, error) {
	path := GetConfigPath()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return NewDefaultConfig(), nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return NewDefaultConfig(), nil // Fail gracefully to defaults
	}

	cfg := NewDefaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return NewDefaultConfig(), nil // Fail gracefully to defaults
	}

	return cfg, nil
}

func (c *Config) Save() error {
	path := GetConfigPath()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

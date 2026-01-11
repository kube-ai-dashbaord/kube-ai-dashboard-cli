package config

import (
	"os"
	"path/filepath"

	"github.com/adrg/xdg"
	"gopkg.in/yaml.v3"
)

type Config struct {
	LLM          LLMConfig `json:"llm"`
	ReportPath   string    `json:"report_path"`
	EnableAudit  bool      `json:"enable_audit"`
	Language     string    `json:"language"`
	BeginnerMode bool      `json:"beginner_mode"`
}

type LLMConfig struct {
	Provider string `yaml:"provider"`
	Model    string `yaml:"model"`
	Endpoint string `yaml:"endpoint"`
	APIKey   string `yaml:"api_key"`
}

func GetConfigPath() string {
	return filepath.Join(xdg.ConfigHome, "k13s", "config.yaml")
}

func LoadConfig() (*Config, error) {
	path := GetConfigPath()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return &Config{
			LLM: LLMConfig{
				Provider: "openai",
				Model:    "gpt-4",
			},
		}, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
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

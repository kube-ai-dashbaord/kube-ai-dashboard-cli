# k13s Configuration Guide

`k13s` is designed to be resilient and works out-of-the-box without any manual configuration. However, you can customize your experience by editing the configuration file.

## Configuration File Structure

```
~/.config/k13s/
├── config.yaml       # Main configuration
├── hotkeys.yaml      # Custom hotkey bindings (future)
├── plugins.yaml      # External plugins (future)
└── skins/
    └── default.yaml  # Theme customization (future)
```

## Configuration File Path

| Platform | Path |
|----------|------|
| macOS | `~/Library/Application Support/k13s/config.yaml` |
| Linux | `~/.config/k13s/config.yaml` |
| Windows | `%APPDATA%\k13s\config.yaml` |

## Main Configuration (config.yaml)

### Core Settings

| Key | Description | Default | Options |
|-----|-------------|---------|---------|
| `language` | UI Language | `en` | `en`, `ko`, `ja`, `zh` |
| `beginner_mode` | Simplified AI explanations | `true` | `true`, `false` |
| `enable_audit` | Audit logging | `true` | `true`, `false` |
| `report_path` | Report output path | `report.md` | Any valid path |
| `log_level` | Logging verbosity | `info` | `debug`, `info`, `warn`, `error` |

### LLM Settings

Configure your AI provider in the `llm` block:

```yaml
llm:
  provider: "openai"    # LLM provider
  model: "gpt-4"        # Model name
  endpoint: ""          # Custom API endpoint (optional)
  api_key: "sk-..."     # API key (or use env var)
```

**Supported Providers:**

| Provider | Value | Environment Variable |
|----------|-------|---------------------|
| OpenAI | `openai` | `OPENAI_API_KEY` |
| Azure OpenAI | `azure` | `AZURE_OPENAI_API_KEY` |
| Anthropic | `anthropic` | `ANTHROPIC_API_KEY` |
| Ollama (local) | `ollama` | - |
| Google Vertex | `vertex` | `GOOGLE_APPLICATION_CREDENTIALS` |

### Full Example

```yaml
# k13s Configuration
language: en
beginner_mode: true
enable_audit: true
report_path: ~/reports/k13s-report.md
log_level: info

llm:
  provider: openai
  model: gpt-4-turbo
  api_key: ${OPENAI_API_KEY}  # Uses environment variable
```

## Using Environment Variables

API keys can be set via environment variables instead of the config file:

```bash
# OpenAI
export OPENAI_API_KEY="sk-..."

# Or for Azure
export AZURE_OPENAI_API_KEY="..."
export AZURE_OPENAI_ENDPOINT="https://your-resource.openai.azure.com"
```

## In-App Settings

Press `s` to open the Settings modal where you can modify:
- Language
- LLM Provider and Model
- Beginner Mode toggle
- Audit logging toggle

Changes are saved automatically to `config.yaml`.

## Resilience Features

- **Zero-Config**: Works without any configuration file
- **Fail-Safe**: Corrupted config falls back to defaults
- **Auto-Create**: Config directory is created on first save
- **Environment Override**: Env vars take precedence over config file

## Troubleshooting

### Config not loading?
1. Check file permissions: `ls -la ~/.config/k13s/`
2. Validate YAML syntax: `cat ~/.config/k13s/config.yaml | python -c "import yaml, sys; yaml.safe_load(sys.stdin)"`
3. Check logs: `~/.config/k13s/k13s.log`

### AI not responding?
1. Verify API key is set
2. Check network connectivity to the provider
3. Try a different model (e.g., `gpt-3.5-turbo` instead of `gpt-4`)

### Reset to defaults
Delete the config file to reset:
```bash
rm ~/.config/k13s/config.yaml
```

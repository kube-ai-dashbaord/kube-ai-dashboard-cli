# k13s Configuration Guide

`k13s` is designed to be resilient and works out-of-the-box without any manual configuration. However, you can customize your experience by editing the configuration file.

## Configuration File Path
- **macOS**: `~/Library/Application Support/k13s/config.yaml`
- **Linux**: `~/.config/k13s/config.yaml`

## Core Settings

| Key | Description | Default |
|-----|-------------|---------|
| `language` | UI Language (`en`, `ko`, `ja`, `zh`) | `en` |
| `beginner_mode` | Enables pedagogical AI explanations | `true` |
| `enable_audit` | Logs all AI interactions for enterprise auditing | `true` |
| `report_path` | Filename for generated reports | `report.md` |

## LLM Settings
Configure your AI provider in the `llm` block:

```yaml
llm:
  provider: "openai"  # options: openai, azure, anthropic, ollama, vertex
  model: "gpt-4"      # your preferred model
  endpoint: ""        # optional override
  api_key: "sk-..."   # your API key
```

## Resilience Features
- **Zero-Config**: If the file is missing, `k13s` uses safe defaults.
- **Fail-Safe**: If the configuration file is corrupted or unreadable, `k13s` will automatically fallback to the default settings instead of crashing.

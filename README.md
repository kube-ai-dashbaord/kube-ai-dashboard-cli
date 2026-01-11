# kube-ai-dashboard-cli

A terminal-based Kubernetes dashboard with an integrated agentic AI assistant. It combines the power of `k9s` with natural language intelligence to help you manage and understand your cluster more effectively.

## 🚀 Features

- **Split-Screen TUI**: Dashboard on the left, AI Assistant on the right.
- **K9s-like Navigation**: Use `:` commands to switch between resources (e.g., `:pods`, `:nodes`, `:svc`, `:deploy`, `:ns`).
- **Context-Aware AI**: Selecting a resource in the dashboard automatically informs the AI assistant of the context.
- **AI Shortcuts**: 
  - `l`: Fetch and explain logs for the selected resource.
  - `d`: Describe the selected resource using AI analysis.
- **LLM Benchmarking**: Includes a dedicated evaluation tool to measure LLM performance on Kubernetes tasks.
- **Persistent Configuration**: XDG-compliant configuration management for LLM providers (OpenAI, Ollama, etc.).
- **Multi-Architecture**: Supports Darwin/Linux and AMD64/ARM64.

## 🛠 Prerequisites

- [Go](https://go.dev/dl/) 1.24 or higher.
- A functional Kubernetes cluster (context should be set in `~/.kube/config`).
- Access to an LLM provider (OpenAI API key or local Ollama instance).

## 🔨 Build Instructions

### Core TUI Application
```bash
go build -o kube-ai-dashboard-cli ./cmd/kube-ai-dashboard-cli/main.go
```

### Evaluation Benchmark Tool
```bash
go build -o kube-ai-eval ./cmd/eval/main.go
```

## 🏃 Execution

### Running the Dashboard
```bash
./kube-ai-dashboard-cli
```

### Running the Benchmark Suite
```bash
./kube-ai-eval
```

## ⚙️ Configuration

The application stores its configuration in `~/.config/kube-ai-dashboard-cli/config.yaml`. You can update these settings directly within the TUI by pressing `s`.

Example `config.yaml`:
```yaml
llm:
  provider: openai
  model: gpt-4o
  endpoint: https://api.openai.com/v1
  api_key: your-api-key-here
```

## 🧪 Testing

Run unit tests:
```bash
go test ./...
```

## 📦 CI/CD

This project uses **GitHub Actions** and **GoReleaser** for automated multi-architecture builds. Tagging a commit with `v*` (e.g., `v1.0.0`) will trigger the release pipeline.

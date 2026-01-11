# k13s (formerly kube-ai-dashboard-cli)

**k13s** is a high-fidelity terminal Kubernetes dashboard merged with an integrated agentic AI assistant. It bridges the gap between traditional TUI management (`k9s`) and natural language intelligence (`kubectl-ai`), helping you manage, debug, and understand your cluster with ease.

## 🚀 Features

- **Split-Screen Power TUI**: Dynamic resizable dashboard and AI Assistant panels (`Ctrl+H`/`Ctrl+L` to resize, `Left`/`Right`/`Tab` to switch focus).
- **Prograde K9s Parity**:
  - **Expanded Resource Views**: Full support for Pods, Nodes, Services, Deployments, Events, ConfigMaps, Secrets, Ingresses, RBAC (Roles, Bindings, SAs), and Storage (PV, PVC, SC).
  - **Vim-style Navigation**: Use `h`, `j`, `k` for intuitive movement.
  - **Quick Commands**: Use `:` to switch resources and `/` for real-time filtering.
  - **Numerical Shortcuts**: Select rows (0-9) instantly. In the `ns` view, these shortcuts quick-switch your namespace context.
- **Real-time Log Streaming**: High-performance log viewer with the `l` shortcut.
- **Agentic AI Assistant**:
  - **Context-Aware**: AI automatically understands which resource you've selected.
  - **MCP Integration**: Enabled Multi-Context Protocol for tool-use (bash, kubectl, and custom MCP servers).
  - **Interactive Verification**: AI confirms resource-modifying actions via a TUI choice list.
- **LLM Benchmarking**: Dedicated tool to measure AI performance on Kubernetes tasks.
- **Persistent Config**: XDG-compliant setup for OpenAI, Ollama, and more.

## 🛠 Prerequisites

- [Go](https://go.dev/dl/) 1.24 or higher.
- A functional Kubernetes cluster (context should be set in `~/.kube/config`).
- Access to an LLM provider (OpenAI API key or local Ollama instance).

## 🔨 Build Instructions

### Core TUI Application
```bash
go build -o k13s ./cmd/kube-ai-dashboard-cli/main.go
```

### Evaluation Benchmark Tool
```bash
go build -o k13s-eval ./cmd/eval/main.go
```

## 🏃 Execution

### Running the Dashboard
```bash
./k13s
```

### Running the Benchmark Suite
```bash
./k13s-eval
```

## ⚙️ Configuration

The application stores configuration in `~/.config/k13s/config.yaml`. Access the settings UI directly by pressing `s`.

Example `config.yaml`:
```yaml
llm:
  provider: openai
  model: gpt-4o
  endpoint: https://api.openai.com/v1
  api_key: your-api-key-here
```

## 🧪 Testing

```bash
go test ./...
```

## 📦 Releases

This project uses **GitHub Actions** and **GoReleaser**. Tagging a commit with `v*` (e.g., `v1.0.0`) triggers a multi-architecture release pipeline.

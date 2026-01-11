# k13s: The AI-Powered Kubernetes Explorer 🚀

<p align="center">
  <b>k9s Management + kubectl-ai Intelligence</b>
</p>

**k13s** is a high-fidelity terminal Kubernetes dashboard merged with an integrated agentic AI assistant. It bridges the gap between traditional TUI management and natural language intelligence, helping you manage, debug, and understand your cluster with unprecedented ease.

---

## ✨ Features

### 🛠 Professional Dashboard (k9s Parity)
- **Deep Resource Support**: Pods, Nodes, Services, Deployments, Events, ConfigMaps, Secrets, Ingresses, RBAC, and more.
- **Fast Navigation**: Vim-style keys (`h/j/k/l`), quick switching (`:pods`, `:svc`), and real-time filtering (`/`).
- **Interactive Operations**: Scale, Restart, Port-Forward, and Delete with safe confirmation flows.
- **Auditing**: Built-in SQLite database to track every action and AI tool invocation.

### 🤖 Agentic AI Assistant
- **100% kubectl-ai Parity**: Leverages the full agentic loop with tool-use (Kubectl, Bash, MCP).
- **Deep Synergy**: Press `L` on any resource to trigger an AI Analyze session with full context (YAML + Events + Logs).
- **Pedagogical Education**: **Beginner Mode** provides simple explanations for complex resources (press `h`).
- **Safety First**: AI-proposed modifications require explicit user approval via interactive choice lists.

### 🌍 Global & Accessible
- **Full i18n**: Native support for **English**, **한국어**, **简体中文**, and **日本語**.
- **Embedded DB**: No external dependencies. Uses CGO-free SQLite for persistent history and settings.

---

## 🚀 Getting Started

### Installation

```bash
# Build from source
go build -o k13s ./cmd/kube-ai-dashboard-cli/main.go
```

### Usage

1.  Run the application: `./k13s`
2.  Press **s** to configure your LLM provider (OpenAI, Ollama, Anthropic).
3.  Select a resource and press **L** to see the AI in action!

---

## 📖 Documentation

- [User Guide](docs/USER_GUIDE.md) - Mastery of navigation and shortcuts.
- [Contributing Guide](CONTRIBUTING.md) - How to help build the future of k13s.
- [Support Policy](SUPPORT.md) - Getting help and reporting issues.

---

## 🛡 Security

We take security seriously. Please see our [Security Policy](SECURITY.md) for reporting vulnerabilities.

---

## 📜 License

Distributed under the MIT License. See `LICENSE` for more information.

---

<p align="center">
  Built with ❤️ for the Kubernetes Community.
</p>

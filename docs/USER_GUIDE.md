# k13s User Guide

Welcome to the **k13s** user guide! This document will help you master the ultimate Kubernetes AI Explorer.

## ⌨️ Core Navigation

k13s uses Vim-style navigation and intuitive shortcuts to keep your hands on the keyboard.

- **j / k**: Move selection up/down in the dashboard.
- **Left / Right / Tab**: Switch focus between the Dashboard (left) and AI Assistant (right).
- **Ctrl+H / Ctrl+L**: Resize the panels to your preference.
- **ESC**: Close any modal or return to the main view.

## 🚀 Dashboard Actions

- **:** : Switch resources (e.g., type `:pods`, `:svc`, `:deploy`).
- **/** : Filter the current table in real-time.
- **y** : View the full YAML manifest of the selected resource.
- **l** : Stream real-time logs (works for Pods).
- **d** : **Native Describe** - View a detailed textual status of the resource.
- **L** : **AI Analyze** - Send the resource context to the AI for intelligent debugging.
- **h** : **Explain This** - Get a pedagogical explanation (great for beginners!).
- **s** : Scale replicas (Deployments/StatefulSets).
- **r** : Trigger a Rollout Restart.
- **Shift+F** : Setup Port Forwarding.
- **Ctrl+D** : Delete the selected resource (with confirmation).

## 🤖 AI Assistant Synergy

The AI Assistant is fully integrated with the Dashboard.

### Analyzing Resources
When you select a resource and press **L**, the Assistant receives the YAML, events, and status. It can then tell you *why* a Pod is failing and how to fix it.

### Executing Commands
You can ask the Assistant to perform tasks like:
> "Scale the nginx deployment to 5 replicas"
> "Delete all pods with labels app=old"

The AI will often present a **Decision Required** view. Review the proposed actions and select **Execute** to proceed.

### Jumping to Dashboard
If the AI mentions a resource, you can jump to it using the command bar:
- `:view pods nginx-bot-123 my-namespace`

## ⚙️ Settings & Customization

Press **s** to open the Settings menu. Here you can:
- Change the **LLM Provider** (OpenAI, Ollama, etc.).
- Switch **Language** (English, Korean, Chinese, Japanese).
- Toggle **Beginner Mode** for simpler AI explanations.
- Enable/Disable the **Audit Log**.

## 🛡 Auditing

Internal actions and AI tool calls are recorded in the Audit Log for transparency and security. View it by typing `:audit`.

---

Need more help? Check out our [Support Policy](../SUPPORT.md) or join our community discussions!

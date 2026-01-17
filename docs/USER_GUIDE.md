# k13s User Guide

Welcome to the **k13s** user guide! This document will help you master the ultimate Kubernetes AI Explorer.

## Core Navigation

k13s uses Vim-style navigation and intuitive shortcuts to keep your hands on the keyboard.

| Key | Action |
|-----|--------|
| `j` / `k` | Move selection up/down |
| `g` | Jump to top of list |
| `G` | Jump to bottom of list |
| `Ctrl+U` | Page up (10 rows) |
| `Ctrl+F` | Page down (10 rows) |
| `Left` / `Right` / `Tab` | Switch focus between Dashboard and AI Assistant |
| `Ctrl+H` / `Ctrl+L` | Resize panels |
| `0-9` | Quick namespace switch |
| `ESC` | Close modal / Clear filter / Return to main view |

## Resource Commands

Switch between resources using the command bar (`:` prefix):

| Command | Resource |
|---------|----------|
| `:pods` or `:po` | Pods |
| `:deploy` or `:deployments` | Deployments |
| `:svc` or `:services` | Services |
| `:ds` or `:daemonsets` | DaemonSets |
| `:sts` or `:statefulsets` | StatefulSets |
| `:jobs` | Jobs |
| `:cj` or `:cronjobs` | CronJobs |
| `:hpa` | Horizontal Pod Autoscalers |
| `:netpol` or `:networkpolicies` | Network Policies |
| `:cm` or `:configmaps` | ConfigMaps |
| `:secrets` | Secrets |
| `:ing` or `:ingresses` | Ingresses |
| `:pv` | Persistent Volumes |
| `:pvc` | Persistent Volume Claims |
| `:sc` or `:storageclasses` | Storage Classes |
| `:sa` or `:serviceaccounts` | Service Accounts |
| `:roles` | Roles |
| `:rb` or `:rolebindings` | Role Bindings |
| `:clusterroles` | Cluster Roles |
| `:crb` | Cluster Role Bindings |
| `:nodes` or `:no` | Nodes |
| `:ns` or `:namespaces` | Namespaces |
| `:ctx` or `:contexts` | Kubernetes Contexts |
| `:events` or `:ev` | Events |

## Dashboard Actions

| Key | Action |
|-----|--------|
| `/` | Filter current table (supports regex: `/pattern/`) |
| `y` | View YAML manifest |
| `e` | Edit resource in $EDITOR (opens YAML, applies on save) |
| `l` | Stream logs (Pods) |
| `s` | Shell into Pod (select container and shell) |
| `d` | Native Describe |
| `L` | AI Analyze - Send resource context to AI |
| `h` | Explain This - Pedagogical AI explanation |
| `S` | Scale replicas (Deployments/StatefulSets/DaemonSets) |
| `r` | Rollout Restart |
| `F` | Port Forwarding setup |
| `Ctrl+D` | Delete resource (with confirmation) |

## Multi-Select

Select multiple resources for bulk operations:

| Key | Action |
|-----|--------|
| `Space` | Toggle selection on current row |
| `Ctrl+Space` | Clear all selections |

Selected rows are marked with `●` and highlighted in cyan.

## Filtering

### Substring Filter
Type `/` followed by text to filter:
```
/nginx
```

### Regex Filter
Use `/pattern/` for regex matching:
```
/nginx-[0-9]+/
/^api-.*/
```

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

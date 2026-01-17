# k13s User Guide

Welcome to the **k13s** user guide! This document will help you master the ultimate Kubernetes AI Explorer with **k9s-compatible key bindings**.

## Core Navigation (k9s Compatible)

k13s uses Vim-style navigation with k9s-compatible shortcuts to keep your hands on the keyboard.

| Key | Action |
|-----|--------|
| `j` / `k` or `↑` / `↓` | Move selection up/down |
| `g` | Jump to top of list |
| `G` | Jump to bottom of list |
| `Ctrl+U` | Page up (10 rows) |
| `Ctrl+F` | Page down (10 rows) |
| `Enter` | Drill down to related resources |
| `Esc` | Go back to previous view |
| `Tab` | Switch focus to AI Assistant |
| `0-9` | Quick namespace switch |

### Resource Drill-Down (Enter Key)

When you press `Enter` on a resource, k13s navigates to related resources:

| From Resource | Navigates To |
|---------------|--------------|
| Service | Pods (matching selector) |
| Deployment | Pods |
| ReplicaSet | Pods |
| StatefulSet | Pods |
| DaemonSet | Pods |
| Job | Pods |
| CronJob | Jobs |
| Node | Pods running on node |
| Namespace | Switch to namespace, show Pods |
| Pod | Logs view |

Press `Esc` to go back to the previous view (navigation history is maintained).

## Resource Commands

Switch between resources using the command bar (`:` prefix):

| Command | Resource |
|---------|----------|
| `:pods` or `:po` | Pods |
| `:deploy` or `:deployments` | Deployments |
| `:svc` or `:services` | Services |
| `:ds` or `:daemonsets` | DaemonSets |
| `:sts` or `:statefulsets` | StatefulSets |
| `:rs` or `:replicasets` | ReplicaSets |
| `:jobs` or `:job` | Jobs |
| `:cj` or `:cronjobs` | CronJobs |
| `:hpa` | Horizontal Pod Autoscalers |
| `:netpol` or `:networkpolicies` | Network Policies |
| `:cm` or `:configmaps` | ConfigMaps |
| `:sec` or `:secrets` | Secrets |
| `:ing` or `:ingresses` | Ingresses |
| `:pv` or `:persistentvolumes` | Persistent Volumes |
| `:pvc` or `:persistentvolumeclaims` | Persistent Volume Claims |
| `:sc` or `:storageclasses` | Storage Classes |
| `:sa` or `:serviceaccounts` | Service Accounts |
| `:roles` or `:role` | Roles |
| `:rb` or `:rolebindings` | Role Bindings |
| `:cr` or `:clusterroles` | Cluster Roles |
| `:crb` or `:clusterrolebindings` | Cluster Role Bindings |
| `:nodes` or `:no` | Nodes |
| `:ns` or `:namespaces` | Namespaces |
| `:ctx` or `:context` | Kubernetes Contexts |
| `:events` or `:ev` | Events |
| `:crd` | Custom Resource Definitions |
| `:pdb` | Pod Disruption Budgets |
| `:quota` | Resource Quotas |
| `:limits` | Limit Ranges |
| `:ep` | Endpoints |

## Dashboard Actions (k9s Compatible)

### General Actions

| Key | Action |
|-----|--------|
| `d` | Describe resource (detailed info like `kubectl describe`) |
| `y` | View YAML manifest |
| `e` | Edit resource in $EDITOR |
| `/` | Filter current table (supports regex: `/pattern/`) |
| `r` | Refresh current view |
| `c` | Switch Kubernetes context |
| `?` | Show help |
| `q` | Quit |

### Pod Actions

| Key | Action |
|-----|--------|
| `l` | View logs |
| `p` | View previous container logs |
| `s` | Shell into Pod (`/bin/bash` or `/bin/sh`) |
| `a` | Attach to container |
| `o` | Show node where pod is running |
| `k` or `Ctrl+K` | Kill (force delete) pod |
| `Shift+F` | Port forward |

### Workload Actions (Deployments, StatefulSets, DaemonSets)

| Key | Action |
|-----|--------|
| `Shift+S` | Scale replicas |
| `Shift+R` | Rollout restart |
| `z` | Show related resources (ReplicaSets for Deployments) |

### CronJob Actions

| Key | Action |
|-----|--------|
| `t` | Trigger (manually create job from cronjob) |

### Namespace Actions

| Key | Action |
|-----|--------|
| `u` | Use namespace (switch to selected namespace) |

### Dangerous Actions

| Key | Action |
|-----|--------|
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

## AI Assistant Synergy

The AI Assistant is fully integrated with the Dashboard.

### Asking Questions

Press `Tab` to switch focus to the AI input field. You can ask questions like:
- "Why is this pod failing?"
- "How do I scale this deployment?"
- "Explain what this resource does"

### Decision Required

When the AI suggests kubectl commands that modify resources, k13s shows a **Decision Required** prompt:

```
━━━ DECISION REQUIRED ━━━

? [1] Confirm: kubectl scale deployment nginx --replicas=5
⚠ [2] DANGEROUS: kubectl delete pod nginx-abc123 --force
   • This command will delete resources
   • Force flag may cause data loss

Press 1-9 to execute, A to execute all, Esc to cancel
```

- **Yellow `?`**: Command requires confirmation
- **Red `⚠`**: Dangerous command with warnings
- Press **1-9** to execute a specific command
- Press **A** to execute all commands (with extra confirmation for dangerous ones)
- Press **Esc** to cancel all

### Command Safety Analysis

The AI automatically analyzes suggested commands:

| Category | Examples | Behavior |
|----------|----------|----------|
| Read-only | `get`, `describe`, `logs` | No confirmation needed |
| Write | `apply`, `create`, `scale` | Confirmation required |
| Dangerous | `delete --all`, `drain --force` | Extra warnings shown |
| Interactive | `exec -it`, `port-forward` | Not auto-executed |

### Jumping to Dashboard

If the AI mentions a resource, you can jump to it using the command bar:
- `:view pods nginx-bot-123 my-namespace`

## Settings & Customization

Type `:health` or `:status` to check system status including:
- Kubernetes connectivity
- AI provider status
- Current configuration

Configuration is stored in `~/.kube-ai-dashboard/config.yaml`. See the [Configuration Guide](CONFIGURATION_GUIDE.md) for details.

## Auditing

Internal actions and AI tool calls are recorded in the Audit Log for transparency and security. View it by typing `:audit`.

---

## Quick Reference Card

```
Navigation          Resource Actions      AI Assistant
─────────────────   ─────────────────     ─────────────
↑/↓ or j/k: Move    d: Describe           Tab: Focus AI
g/G: Top/Bottom     y: YAML               Enter: Send
Enter: Drill down   e: Edit               Esc: Cancel
Esc: Go back        l: Logs (pods)        1-9: Execute cmd
Ctrl+U/F: Page      s: Shell (pods)       A: Execute all
0-9: Namespace      Ctrl+D: Delete
/: Filter           S: Scale
:: Command          R: Restart
?: Help             F: Port-forward
q: Quit             t: Trigger (cj)
```

---

Need more help? Check out our [Support Policy](../SUPPORT.md) or join our community discussions!

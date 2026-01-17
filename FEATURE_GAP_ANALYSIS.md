# k13s Feature Gap Analysis

## Overview

This document compares k13s with three major Kubernetes dashboard/AI projects to identify missing features.

**Analyzed Projects:**
- **kubernetes-dashboard** - Official Kubernetes Dashboard
- **headlamp** - Cloud-native Kubernetes UI with plugin system
- **kubectl-ai** - AI-powered kubectl assistant

---

## Current k13s Features (Baseline)

### CLI/TUI Features
- [x] Resource listing: Pods, Deployments, Services, Nodes, Namespaces, Events, ConfigMaps, Secrets, DaemonSets, StatefulSets, Jobs, CronJobs, Ingresses
- [x] Namespace switching
- [x] Context switching
- [x] Real-time table filtering with highlight
- [x] Command autocomplete with hints
- [x] AI chat integration (streaming)
- [x] YAML view for resources
- [x] Pod logs viewer
- [x] Delete with confirmation
- [x] Port forwarding
- [x] Shell exec (spawns external terminal)
- [x] Health check command
- [x] i18n support (en, ko, ja, zh)

### Web UI Features (index.html)
- [x] Login page with username/password
- [x] Dark theme (Tokyo Night style)
- [x] Sidebar navigation with resource groups
- [x] Resource table with status colors
- [x] Namespace selector dropdown
- [x] AI chat panel with streaming responses
- [x] Resizable panels (drag handle)
- [x] Settings modal (language, LLM config)
- [x] Audit log viewer
- [x] Reports viewer
- [x] Refresh button per resource
- [x] User badge and logout
- [x] Tab system (Table/YAML/Logs)
- [x] Message history in AI panel

### Web Server/API Features
- [x] REST API for resources (pods, deployments, services, namespaces, nodes, events)
- [x] AI chat endpoint (SSE streaming)
- [x] AI chat endpoint (non-streaming)
- [x] JWT authentication with session management
- [x] LDAP authentication support
- [x] Audit logging (SQLite)
- [x] Report generation API
- [x] Settings API (GET/PUT)
- [x] LLM settings API
- [x] Health check endpoint
- [x] CORS middleware
- [x] Static file serving (embedded)

### Backend Features
- [x] Single LLM provider support (OpenAI-compatible)
- [x] Kubernetes client with metrics API
- [x] Dynamic client for CRDs
- [x] Context switching support

---

## Priority 1: Critical Missing Features

### 1.1 Multi-Provider LLM Support (from kubectl-ai)
**Gap:** k13s only supports OpenAI-compatible API
**Required:**
- [ ] Google Gemini provider
- [ ] AWS Bedrock provider (Claude)
- [ ] Azure OpenAI provider
- [ ] Ollama (local) provider
- [ ] Llama.cpp provider
- [ ] Grok provider
- [ ] Provider factory pattern with dynamic selection

**Files to create:**
```
pkg/ai/providers/
├── factory.go       # Provider factory with registration
├── openai.go        # (refactor from client.go)
├── gemini.go
├── bedrock.go
├── azopenai.go
├── ollama.go
└── llamacpp.go
```

### 1.2 AI Safety & Validation (from kubectl-ai)
**Gap:** No safety checks before executing AI-suggested commands
**Required:**
- [ ] Command classification (read-only vs write)
- [ ] Interactive command detection and blocking
- [ ] Composite command detection (pipe attacks)
- [ ] Permission request flow for destructive operations
- [ ] "Yes/No/Don't ask again" user choice system

**Files to create:**
```
pkg/ai/
├── filter.go        # Command safety filter
├── permission.go    # Permission request system
└── sandbox.go       # Sandbox execution support
```

### 1.3 In-Browser Terminal (from kubernetes-dashboard, headlamp)
**Gap:** Shell exec spawns external terminal, not integrated
**Required:**
- [ ] WebSocket-based terminal
- [ ] xterm.js frontend integration
- [ ] Bidirectional stdin/stdout streaming
- [ ] Terminal resize support
- [ ] Container selection UI

**Files to create:**
```
pkg/web/
├── terminal.go      # WebSocket terminal handler
└── static/terminal/ # xterm.js frontend
```

### 1.4 Plugin System (from headlamp)
**Gap:** No extensibility mechanism
**Required:**
- [ ] Plugin registry system
- [ ] Dynamic plugin loading from URL
- [ ] Plugin configuration management
- [ ] Extension points:
  - Custom sidebar items
  - Custom routes
  - Custom resource detail views
  - Custom table columns
  - Custom themes

**Files to create:**
```
pkg/plugins/
├── registry.go      # Plugin registry
├── loader.go        # Dynamic plugin loading
├── config.go        # Plugin configuration
└── types.go         # Plugin interfaces
```

---

## Priority 2: Important Missing Features

### 2.1 Session Persistence (from kubectl-ai)
**Gap:** No conversation history persistence
**Required:**
- [ ] Session storage interface
- [ ] Filesystem backend
- [ ] Session metadata (ID, timestamps, model)
- [ ] Resume/load previous sessions
- [ ] Clear conversation command

**Files to create:**
```
pkg/ai/sessions/
├── interface.go     # Session store interface
├── filesystem.go    # File-based storage
└── memory.go        # In-memory storage
```

### 2.2 Helm Integration (from headlamp)
**Gap:** No Helm support
**Required:**
- [ ] List Helm releases
- [ ] Install/upgrade/delete releases
- [ ] View release history
- [ ] Repository management
- [ ] Values inspection

**Files to create:**
```
pkg/helm/
├── client.go        # Helm client wrapper
├── release.go       # Release operations
└── repository.go    # Repository management
```

### 2.3 Resource Graph Visualization (from headlamp)
**Gap:** No relationship visualization
**Required:**
- [ ] Pod-to-Service-to-Deployment graph
- [ ] Resource relationship mapping
- [ ] Interactive graph navigation
- [ ] Namespace grouping

**Implementation:** Use @xyflow/react or similar for web UI

### 2.4 Metrics Visualization (from kubernetes-dashboard)
**Gap:** Metrics exist but no visualization
**Required:**
- [ ] CPU/Memory sparklines in table
- [ ] Resource usage charts
- [ ] Historical metrics storage
- [ ] Metrics aggregation

**Files to create:**
```
pkg/metrics/
├── scraper.go       # Metrics collection
├── storage.go       # Time-series storage
└── aggregation.go   # Metric aggregation
```

### 2.5 Deployment Operations (from kubernetes-dashboard)
**Gap:** Limited deployment management
**Required:**
- [ ] Rollback to previous revision
- [ ] Pause/resume deployment
- [ ] Restart (rollout restart)
- [ ] View old/new ReplicaSets
- [ ] Scaling UI

### 2.6 CronJob Operations (from kubernetes-dashboard)
**Gap:** No manual trigger
**Required:**
- [ ] Trigger CronJob manually (create Job)
- [ ] View job history
- [ ] Suspend/resume CronJob

### 2.7 Node Operations (from kubernetes-dashboard)
**Gap:** Basic node listing only
**Required:**
- [ ] Node drain operation
- [ ] Node cordon/uncordon
- [ ] Node pods view

---

## Priority 3: Enhancement Features

### 3.1 OIDC/OAuth2 Authentication (from headlamp)
**Gap:** Only JWT and LDAP auth
**Required:**
- [ ] OIDC provider integration
- [ ] PKCE flow support
- [ ] Token refresh automation
- [ ] JMESPath claim extraction
- [ ] Multiple auth method support

### 3.2 Multi-Cluster Support (from headlamp)
**Gap:** Single context only per session
**Required:**
- [ ] Cluster switcher UI
- [ ] Per-cluster authentication
- [ ] Kubeconfig file watcher
- [ ] Dynamic cluster addition

### 3.3 Advanced Search (from headlamp)
**Gap:** Basic filter only
**Required:**
- [ ] Global search across clusters
- [ ] Label selector filtering
- [ ] Field selector filtering
- [ ] Recent searches tracking
- [ ] Full-text search with fuse.js

### 3.4 MCP (Model Context Protocol) Support (from kubectl-ai)
**Gap:** No MCP integration
**Required:**
- [ ] MCP client mode (use external tools)
- [ ] MCP server mode (expose tools)
- [ ] Tool registration system
- [ ] External tool discovery

### 3.5 Custom Resource Definition Support (from kubernetes-dashboard, headlamp)
**Gap:** Limited CRD support
**Required:**
- [ ] List all CRDs
- [ ] Browse CRD instances
- [ ] Create/edit custom resources
- [ ] CRD schema validation
- [ ] OpenAPI documentation display

### 3.6 Retry Logic with Backoff (from kubectl-ai)
**Gap:** No retry mechanism for AI calls
**Required:**
- [ ] Exponential backoff
- [ ] Jitter support
- [ ] Retryable error detection
- [ ] Max attempts configuration

### 3.7 YAML Editor with Validation (from headlamp)
**Gap:** Plain text YAML display
**Required:**
- [ ] Monaco editor integration
- [ ] Syntax highlighting
- [ ] Schema validation
- [ ] Auto-completion
- [ ] Inline documentation

---

## Priority 4: UI/UX Improvements

### 4.1 From kubernetes-dashboard
- [ ] Deployment creation wizard (form-based)
- [ ] Image reference validation
- [ ] Protocol validation for services
- [ ] Log download as file
- [ ] Multi-container log selection
- [ ] Sparkline metrics in list views
- [ ] CSRF protection for API

### 4.2 From headlamp
- [ ] Dark/Light theme toggle
- [ ] Custom theme support
- [ ] Responsive mobile layout
- [ ] Breadcrumb navigation
- [ ] Action confirmation dialogs
- [ ] Empty state handling
- [ ] Loading animations
- [ ] Error boundary UI
- [ ] Internationalization (12 languages)

### 4.3 From kubectl-ai
- [ ] Streaming log UI for AI responses
- [ ] Meta-commands (clear, model, tools, sessions)
- [ ] Tool call visualization
- [ ] Audit/journal logging with structured output

---

## API Feature Gaps (Backend)

### From kubernetes-dashboard API
| Endpoint | Description | k13s Status |
|----------|-------------|-------------|
| `POST /appdeployment` | Deploy application | Missing |
| `POST /appdeployment/validate/*` | Validation endpoints | Missing |
| `PUT /deployment/rollback` | Rollback deployment | Missing |
| `PUT /deployment/pause` | Pause deployment | Missing |
| `PUT /deployment/resume` | Resume deployment | Missing |
| `PUT /deployment/restart` | Restart deployment | Missing |
| `POST /cronjob/trigger` | Trigger CronJob | Missing |
| `POST /node/drain` | Drain node | Missing |
| `GET /metrics/*` | Metrics endpoints | Partial |
| `GET /_raw/*` | Raw resource access | Missing |
| `GET /csrftoken/*` | CSRF tokens | Missing |
| `WS /shell/*` | WebSocket terminal | Missing |
| `GET /log/file/*` | Download logs | Missing |

### From headlamp API
| Feature | Description | k13s Status |
|---------|-------------|-------------|
| `/plugins/*` | Plugin management | Missing |
| `/helm/*` | Helm operations | Missing |
| `WS /exec` | Pod exec WebSocket | Missing |
| `/portforward/*` | Port forward management | Partial |
| `/cluster/*` | Multi-cluster management | Missing |

---

## Implementation Roadmap

### Phase 1: AI Enhancement (Week 1-2)
1. Multi-provider LLM support
2. AI safety validation
3. Session persistence
4. Retry with backoff

### Phase 2: Terminal & Operations (Week 3-4)
1. In-browser terminal (WebSocket)
2. Deployment operations (rollback, pause, restart)
3. CronJob trigger
4. Node drain

### Phase 3: Visualization (Week 5-6)
1. Metrics visualization
2. Resource graph
3. YAML editor with validation

### Phase 4: Extensibility (Week 7-8)
1. Plugin system
2. Multi-cluster support
3. CRD management
4. Helm integration

### Phase 5: Polish (Week 9-10)
1. OIDC authentication
2. Advanced search
3. Theme system
4. Mobile responsive UI

---

## File Structure (Proposed Additions)

```
pkg/
├── ai/
│   ├── providers/           # NEW: Multi-provider support
│   │   ├── factory.go
│   │   ├── openai.go
│   │   ├── gemini.go
│   │   ├── bedrock.go
│   │   └── ollama.go
│   ├── filter.go            # NEW: Safety validation
│   ├── permission.go        # NEW: Permission flow
│   ├── sessions/            # NEW: Session persistence
│   │   ├── interface.go
│   │   └── filesystem.go
│   └── client.go            # MODIFY
├── web/
│   ├── terminal.go          # NEW: WebSocket terminal
│   ├── helm.go              # NEW: Helm API
│   ├── plugins.go           # NEW: Plugin API
│   └── static/
│       └── terminal/        # NEW: xterm.js
├── plugins/                 # NEW: Plugin system
│   ├── registry.go
│   ├── loader.go
│   └── types.go
├── helm/                    # NEW: Helm integration
│   ├── client.go
│   └── release.go
├── metrics/                 # NEW: Metrics system
│   ├── scraper.go
│   └── storage.go
└── mcp/                     # NEW: MCP support
    ├── client.go
    └── server.go
```

---

## Summary

| Category | kubernetes-dashboard | headlamp | kubectl-ai | k13s Missing |
|----------|---------------------|----------|------------|--------------|
| LLM Providers | N/A | N/A | 7+ providers | 6 providers |
| AI Safety | N/A | N/A | Full | Full system |
| Terminal | WebSocket | WebSocket | N/A | WebSocket |
| Plugins | No | Yes (full) | No | Full system |
| Helm | No | Yes | No | Full |
| Multi-cluster | No | Yes | No | Full |
| Metrics Viz | Yes (sparkline) | Yes | No | Sparklines |
| Resource Graph | No | Yes | No | Full |
| Session Persist | N/A | No | Yes | Full |
| OIDC Auth | Yes | Yes (full) | No | Full |
| CRD Management | Yes | Yes | No | Full |
| Node Drain | Yes | Yes | No | API only |
| Deployment Ops | Yes (full) | Partial | No | Full |

**Total Missing Features: ~45 major features**

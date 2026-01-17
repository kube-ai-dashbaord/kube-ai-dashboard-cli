# k13s: The AI-Powered Kubernetes Explorer

<p align="center">
  <b>k9s Management + kubectl-ai Intelligence</b><br>
  <i>TUI Dashboard & Web UI with Integrated AI Assistant</i>
</p>

**k13s** is a comprehensive Kubernetes management tool that provides both a terminal-based UI (TUI) and a web-based dashboard with an integrated AI assistant. It bridges the gap between traditional cluster management and natural language intelligence, helping you manage, debug, and understand your cluster with unprecedented ease.

---

## Features

### TUI Dashboard (Terminal User Interface)
- **Deep Resource Support**: Pods, Nodes, Services, Deployments, Events, ConfigMaps, Secrets, Ingresses, RBAC, and more
- **Fast Navigation**: Vim-style keys (`h/j/k/l`), quick switching (`:pods`, `:svc`), and real-time filtering (`/`)
- **Interactive Operations**: Scale, Restart, Port-Forward, and Delete with safe confirmation flows
- **AI Integration**: Press `a` to open AI panel, `L` to analyze resources with AI context

### Web UI Dashboard
- **Modern Web Interface**: Responsive design with resizable panels
- **SSE Streaming AI Chat**: Real-time streaming responses with live cursor animation
- **Auto-Refresh**: Configurable auto-refresh (10s-5min) with manual refresh button
- **Authentication System**: Session-based authentication with user management
- **LDAP/SSO Support**: Enterprise authentication with group-based role mapping
- **Audit Logging**: Track all actions and AI interactions in SQLite database
- **Reports Generation**: LLM-powered comprehensive cluster analysis with PDF/CSV download
- **Settings Management**: Configure LLM providers, streaming, auto-refresh, and language settings
- **Pod Terminal**: Interactive xterm.js terminal directly in browser (WebSocket exec)
- **Log Viewer**: Real-time log streaming with search, filtering, and ANSI color support
- **Metrics Charts**: CPU/Memory usage graphs with Chart.js, top consumers list
- **Port Forwarding**: Start/stop port forwards through UI with status tracking
- **AI-Dashboard Integration**: AI commands can navigate, highlight resources, and open modals

### Agentic AI Assistant
- **100% kubectl-ai Parity**: Full agentic loop with tool-use (Kubectl, Bash)
- **MCP Tool Execution**: AI directly executes kubectl commands with automatic tool calling
- **Deep Synergy**: AI analysis with full context (YAML + Events + Logs)
- **Pedagogical Education**: **Beginner Mode** provides simple explanations for complex resources
- **Safety First**: AI-proposed modifications require explicit user approval
- **Decision Required**: Interactive approval flow for kubectl commands with safety analysis
  - Commands categorized as Read-only, Write, Dangerous, or Interactive
  - Warnings displayed for dangerous operations (delete --all, force, etc.)
  - Press 1-9 to execute specific commands, A for all, Esc to cancel
- **Agentic Mode**: When using OpenAI-compatible providers with tool support, AI can directly query and modify cluster resources

### Global & Accessible
- **Full i18n**: Native support for **English**, **Korean**, **Chinese**, and **Japanese**
- **Embedded DB**: No external dependencies. Uses CGO-free SQLite for persistent history and settings

---

## Getting Started

### Installation

**Quick Install (Current Platform):**
```bash
# Build from source
make build

# Or directly with go
go build -o k13s ./cmd/kube-ai-dashboard-cli/main.go
```

**Cross-Platform Builds:**
```bash
# Build for all platforms (Linux, macOS, Windows)
make build-all

# Build for specific platforms
make build-linux    # linux/amd64, linux/arm64, linux/arm
make build-darwin   # darwin/amd64, darwin/arm64
make build-windows  # windows/amd64

# Create release packages with checksums
make package
```

**Supported Architectures:**
| Platform | Architecture | Binary |
|----------|-------------|--------|
| Linux | amd64 | `k13s-linux-amd64` |
| Linux | arm64 | `k13s-linux-arm64` |
| Linux | arm | `k13s-linux-arm` |
| macOS | amd64 (Intel) | `k13s-darwin-amd64` |
| macOS | arm64 (Apple Silicon) | `k13s-darwin-arm64` |
| Windows | amd64 | `k13s-windows-amd64.exe` |

### Air-Gapped / Offline Installation

For environments without internet access:

```bash
# On a machine with internet access:
# 1. Create offline bundle with vendored dependencies
make bundle-offline

# 2. Transfer the bundle to air-gapped environment
scp dist/k13s-offline-bundle-*.tar.gz user@airgapped-host:~/

# On the air-gapped machine:
# 3. Extract and build
tar -xzvf k13s-offline-bundle-*.tar.gz
cd offline-bundle
make build-offline

# Or build directly with go
go build -mod=vendor -o k13s ./cmd/kube-ai-dashboard-cli/main.go
```

### Docker

k13s provides Docker images for easy deployment in any environment, including air-gapped networks.

**Quick Start:**
```bash
# Run with Docker (mount your kubeconfig)
docker run -d -p 8080:8080 \
  -v ~/.kube/config:/home/k13s/.kube/config:ro \
  -e K13S_AUTH_MODE=local \
  -e K13S_USERNAME=admin \
  -e K13S_PASSWORD=admin \
  youngjukim/k13s:latest

# Access at http://localhost:8080
```

**Build from Source:**
```bash
# Standard build (requires Go 1.25+)
docker build -t k13s:latest .

# Build with pre-compiled binary (recommended)
go build -o k13s ./cmd/kube-ai-dashboard-cli/main.go
docker build -f Dockerfile.prebuilt -t k13s:latest .
```

**Using Docker Compose:**
```bash
# Basic usage
docker-compose up -d

# With custom password
K13S_PASSWORD=mysecret docker-compose up -d

# With OpenAI
K13S_LLM_PROVIDER=openai K13S_LLM_API_KEY=sk-xxx docker-compose up -d

# With local Ollama (air-gapped)
docker-compose --profile with-ollama up -d
```

**Kubernetes Deployment:**
```bash
# Deploy to Kubernetes cluster
kubectl apply -f kubernetes/deployment.yaml

# Access via port-forward
kubectl port-forward -n k13s svc/k13s 8080:80
```

**Environment Variables:**
| Variable | Default | Description |
|----------|---------|-------------|
| `K13S_PORT` | 8080 | Web server port |
| `K13S_AUTH_MODE` | local | Authentication mode (local/token) |
| `K13S_USERNAME` | admin | Login username |
| `K13S_PASSWORD` | admin | Login password |
| `K13S_LLM_PROVIDER` | - | LLM provider (openai/ollama) |
| `K13S_LLM_MODEL` | - | LLM model name |
| `K13S_LLM_ENDPOINT` | - | LLM API endpoint |
| `K13S_LLM_API_KEY` | - | LLM API key |
| `KUBECONFIG` | - | Path to kubeconfig file |

**Air-Gapped Deployment:**

For environments without internet access:
1. Build the image on a connected machine: `docker build -f Dockerfile.prebuilt -t k13s:latest .`
2. Save the image: `docker save k13s:latest | gzip > k13s-image.tar.gz`
3. Transfer to air-gapped environment
4. Load the image: `docker load < k13s-image.tar.gz`
5. Run with local Ollama for AI features (optional):
   ```bash
   docker-compose --profile with-ollama up -d
   docker exec -it k13s-ollama ollama pull llama3.2
   ```

### TUI Mode (Default)

```bash
# Run TUI dashboard
./k13s
```

**Key Bindings (k9s Compatible):**
| Key | Action |
|-----|--------|
| `j/k` or `↑/↓` | Navigate up/down |
| `g/G` | Jump to top/bottom |
| `Enter` | Drill down to related resources |
| `Esc` | Go back to previous view |
| `d` | Describe resource |
| `y` | View YAML |
| `e` | Edit resource |
| `l` | View logs (pods) |
| `s` | Shell into pod |
| `Ctrl+D` | Delete resource |
| `S` | Scale (deployments/sts) |
| `R` | Restart (deployments/sts) |
| `:pods`, `:svc`, `:deploy` | Quick resource switch |
| `/` | Filter resources |
| `Tab` | Focus AI panel |
| `?` | Show help |
| `q` | Quit |

**Resource Drill-Down (Enter key):**
- Service → Pods | Deployment → Pods | Node → Pods
- CronJob → Jobs | Job → Pods | Namespace → Switch & Pods

### Web UI Mode

```bash
# Start web server on port 8080
./k13s -web -port 8080

# Access in browser
open http://localhost:8080
```

**Default Credentials:**
- Username: `admin`
- Password: `admin123`

**Web UI Features:**
- Left sidebar with resource navigation
- Main content area with resource tables
- Resizable AI chat panel with SSE streaming
- Auto-refresh controls (toggle, interval selector, last update time)
- Settings modal with LLM, streaming, and auto-refresh configuration
- Audit logs viewer
- Reports generation

---

## Configuration

Configuration is stored in `~/.config/k13s/config.yaml`:

```yaml
llm:
  provider: openai
  model: gpt-4
  endpoint: http://localhost:11434/v1  # For Ollama
  api_key: your-api-key

language: en  # en, ko, zh, ja
beginner_mode: true
enable_audit: true
log_level: debug
```

### Supported LLM Providers

| Provider | Tool Calling | Notes |
|----------|-------------|-------|
| **OpenAI** | ✅ Yes | GPT-4, GPT-4o, GPT-3.5-turbo (Full agentic mode) |
| **Ollama** | ⚠️ Model-dependent | llama3.1, mistral-nemo support tools |
| **Azure OpenAI** | ✅ Yes | Enterprise deployment with tool support |
| **Anthropic** | ⚠️ Partial | Claude models (via API adapter) |
| **Local LLMs** | ⚠️ Varies | Any OpenAI-compatible API |

**For Air-Gapped Environments:**
- Use **Ollama** with local models (no internet required after model download)
- Configure endpoint to local server: `endpoint: http://localhost:11434/v1`
- Recommended models: `llama3.1:8b`, `mistral-nemo`, `codellama`

### LDAP Configuration (Optional)

```yaml
auth:
  enabled: true
  ldap:
    enabled: true
    host: ldap.example.com
    port: 389
    use_tls: false
    bind_dn: cn=admin,dc=example,dc=com
    bind_password: secret
    base_dn: dc=example,dc=com
    user_search_filter: "(uid=%s)"
    user_search_base: ou=users,dc=example,dc=com
    group_search_base: ou=groups,dc=example,dc=com
    admin_groups:
      - k8s-admins
    user_groups:
      - k8s-users
    viewer_groups:
      - k8s-viewers
```

---

## Architecture

```
k13s/
├── cmd/
│   └── kube-ai-dashboard-cli/   # Main entry point
├── pkg/
│   ├── ai/              # AI client (OpenAI-compatible)
│   │   ├── tools/       # MCP tool definitions (kubectl, bash)
│   │   ├── providers/   # LLM provider implementations
│   │   └── sessions/    # Conversation history
│   ├── config/          # Configuration management
│   ├── db/              # SQLite database for audit logs
│   ├── i18n/            # Internationalization
│   ├── k8s/             # Kubernetes client wrapper
│   ├── ui/              # TUI components (tview)
│   └── web/             # Web server and API handlers
│       ├── auth.go      # Authentication system
│       ├── ldap.go      # LDAP/SSO integration
│       ├── reports.go   # Report generation
│       ├── server.go    # HTTP server
│       └── static/      # Frontend assets
├── dist/                # Cross-compiled binaries
├── Makefile             # Build automation
└── docs/                # Documentation
```

---

## API Endpoints (Web Mode)

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/health` | GET | Health check |
| `/api/auth/login` | POST | User login |
| `/api/auth/logout` | POST | User logout |
| `/api/auth/me` | GET | Current user info |
| `/api/auth/ldap/status` | GET | LDAP status |
| `/api/auth/ldap/test` | GET | Test LDAP connection |
| `/api/k8s/namespaces` | GET | List namespaces |
| `/api/k8s/pods` | GET | List pods |
| `/api/k8s/deployments` | GET | List deployments |
| `/api/k8s/services` | GET | List services |
| `/api/chat/stream` | POST | AI query (SSE streaming) |
| `/api/audit` | GET | Audit logs |
| `/api/reports` | GET | Generate reports |
| `/api/settings` | GET/PUT | Application settings |

---

## Documentation

- [User Guide](docs/USER_GUIDE.md) - Navigation and shortcuts
- [Documentation Website](docs/website/index.html) - Comprehensive online docs
- [Contributing Guide](CONTRIBUTING.md) - How to contribute
- [Support Policy](SUPPORT.md) - Getting help

## Makefile Targets

| Target | Description |
|--------|-------------|
| `make build` | Build for current platform |
| `make build-all` | Build for all platforms |
| `make build-linux` | Build for Linux (amd64, arm64, arm) |
| `make build-darwin` | Build for macOS (amd64, arm64) |
| `make build-windows` | Build for Windows (amd64) |
| `make package` | Create release packages with checksums |
| `make bundle-offline` | Create offline bundle with vendored deps |
| `make docker` | Build Docker image |
| `make docker-multiarch` | Build multi-arch Docker image |
| `make test` | Run tests |
| `make clean` | Clean build artifacts |

---

## Security

We take security seriously. Please see our [Security Policy](SECURITY.md) for reporting vulnerabilities.

---

## License

Distributed under the MIT License. See `LICENSE` for more information.

---

<p align="center">
  Built with care for the Kubernetes Community.
</p>

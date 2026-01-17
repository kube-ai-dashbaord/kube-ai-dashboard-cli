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
- **Authentication System**: Session-based authentication with user management
- **LDAP/SSO Support**: Enterprise authentication with group-based role mapping
- **Audit Logging**: Track all actions and AI interactions in SQLite database
- **Reports Generation**: Cluster health, resource usage, security audit, and AI interaction reports
- **Real-time AI Chat**: Streaming responses with syntax highlighting for commands
- **Settings Management**: Configure LLM providers, language, and application settings

### Agentic AI Assistant
- **100% kubectl-ai Parity**: Full agentic loop with tool-use (Kubectl, Bash, MCP)
- **Deep Synergy**: AI analysis with full context (YAML + Events + Logs)
- **Pedagogical Education**: **Beginner Mode** provides simple explanations for complex resources
- **Safety First**: AI-proposed modifications require explicit user approval

### Global & Accessible
- **Full i18n**: Native support for **English**, **Korean**, **Chinese**, and **Japanese**
- **Embedded DB**: No external dependencies. Uses CGO-free SQLite for persistent history and settings

---

## Getting Started

### Installation

```bash
# Build from source
go build -o k13s ./cmd/kube-ai-dashboard-cli/main.go
```

### Docker

```bash
# Build Docker image
docker build -t k13s:latest .

# Run with Docker
docker run -d -p 8080:8080 \
  -v ~/.kube:/home/k13s/.kube:ro \
  k13s:latest

# Or use Docker Compose
docker-compose up -d
```

### TUI Mode (Default)

```bash
# Run TUI dashboard
./k13s
```

**Key Bindings:**
| Key | Action |
|-----|--------|
| `h/j/k/l` | Navigate (vim-style) |
| `a` | Toggle AI panel |
| `L` | AI analyze selected resource |
| `:pods`, `:svc` | Quick resource switch |
| `/` | Filter resources |
| `s` | Open settings |
| `?` | Show help |
| `q` | Quit |

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
- Resizable AI chat panel
- Settings modal with LLM configuration
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
- **OpenAI**: GPT-4, GPT-3.5
- **Ollama**: Local models (llama2, codellama, etc.)
- **Anthropic**: Claude models
- **Any OpenAI-compatible API**

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
│   ├── ai/         # AI client (OpenAI-compatible)
│   ├── config/     # Configuration management
│   ├── db/         # SQLite database for audit logs
│   ├── i18n/       # Internationalization
│   ├── k8s/        # Kubernetes client wrapper
│   ├── ui/         # TUI components (tview)
│   └── web/        # Web server and API handlers
│       ├── auth.go      # Authentication system
│       ├── ldap.go      # LDAP/SSO integration
│       ├── reports.go   # Report generation
│       ├── server.go    # HTTP server
│       └── static/      # Frontend assets
└── docs/           # Documentation
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
- [Contributing Guide](CONTRIBUTING.md) - How to contribute
- [Support Policy](SUPPORT.md) - Getting help

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

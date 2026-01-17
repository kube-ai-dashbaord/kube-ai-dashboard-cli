package web

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/kube-ai-dashbaord/kube-ai-dashboard-cli/pkg/ai"
	"github.com/kube-ai-dashbaord/kube-ai-dashboard-cli/pkg/config"
	"github.com/kube-ai-dashbaord/kube-ai-dashboard-cli/pkg/db"
	"github.com/kube-ai-dashbaord/kube-ai-dashboard-cli/pkg/k8s"
	corev1 "k8s.io/api/core/v1"
)

//go:embed static/*
var staticFiles embed.FS

type Server struct {
	cfg             *config.Config
	aiClient        *ai.Client
	k8sClient       *k8s.Client
	authManager     *AuthManager
	reportGenerator *ReportGenerator
	port            int
	server          *http.Server

	// Tool approval management
	pendingApprovals     map[string]*PendingToolApproval
	pendingApprovalMutex sync.RWMutex
}

// PendingToolApproval represents a tool call waiting for user approval
type PendingToolApproval struct {
	ID        string    `json:"id"`
	ToolName  string    `json:"tool_name"`
	Command   string    `json:"command"`
	Category  string    `json:"category"` // read-only, write, dangerous
	Timestamp time.Time `json:"timestamp"`
	Response  chan bool `json:"-"`
}

type ChatRequest struct {
	Message string `json:"message"`
}

type ChatResponse struct {
	Response string `json:"response"`
	Command  string `json:"command,omitempty"`
	Error    string `json:"error,omitempty"`
}

type K8sResourceResponse struct {
	Kind      string                   `json:"kind"`
	Items     []map[string]interface{} `json:"items"`
	Error     string                   `json:"error,omitempty"`
	Timestamp time.Time                `json:"timestamp"`
}

type SSEWriter struct {
	w       http.ResponseWriter
	flusher http.Flusher
	mu      sync.Mutex
}

func (s *SSEWriter) Write(data string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := fmt.Fprintf(s.w, "data: %s\n\n", data)
	if err != nil {
		return err
	}
	s.flusher.Flush()
	return nil
}

func NewServer(cfg *config.Config, port int) (*Server, error) {
	var aiClient *ai.Client
	var err error

	fmt.Printf("Starting k13s web server...\n")
	fmt.Printf("  LLM Provider: %s, Model: %s\n", cfg.LLM.Provider, cfg.LLM.Model)

	if cfg.LLM.Endpoint != "" {
		aiClient, err = ai.NewClient(&cfg.LLM)
		if err != nil {
			fmt.Printf("  AI client creation failed: %v\n", err)
			aiClient = nil
		} else {
			fmt.Printf("  AI client: Ready\n")
		}
	} else {
		fmt.Printf("  AI client: Not configured\n")
	}

	k8sClient, err := k8s.NewClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create k8s client: %w", err)
	}
	fmt.Printf("  K8s client: Ready\n")

	// Initialize database
	if err := db.Init(""); err != nil {
		fmt.Printf("  Database: Failed to initialize (%v)\n", err)
	} else {
		fmt.Printf("  Database: Ready\n")
	}

	// Initialize auth manager
	authConfig := &AuthConfig{
		Enabled:         cfg.EnableAudit, // Use audit flag to control auth for now
		AuthMode:        "local",         // Use local authentication
		SessionDuration: 24 * time.Hour,
		DefaultAdmin:    "admin",
		DefaultPassword: "admin123",
	}
	authManager := NewAuthManager(authConfig)
	fmt.Printf("  Authentication: %s\n", map[bool]string{true: "Enabled", false: "Disabled"}[authConfig.Enabled])

	server := &Server{
		cfg:              cfg,
		aiClient:         aiClient,
		k8sClient:        k8sClient,
		authManager:      authManager,
		port:             port,
		pendingApprovals: make(map[string]*PendingToolApproval),
	}

	server.reportGenerator = NewReportGenerator(server)
	fmt.Printf("  Reports: Ready\n")

	return server, nil
}

func (s *Server) Start() error {
	mux := http.NewServeMux()

	// Public routes (no auth required)
	mux.HandleFunc("/api/health", s.handleHealth)
	mux.HandleFunc("/api/auth/login", s.authManager.HandleLogin)
	mux.HandleFunc("/api/auth/logout", s.authManager.HandleLogout)

	// Protected routes
	mux.HandleFunc("/api/auth/me", s.authManager.AuthMiddleware(s.authManager.HandleCurrentUser))
	mux.HandleFunc("/api/chat", s.authManager.AuthMiddleware(s.handleChat))
	mux.HandleFunc("/api/chat/stream", s.authManager.AuthMiddleware(s.handleChatStream))
	mux.HandleFunc("/api/chat/agentic", s.authManager.AuthMiddleware(s.handleAgenticChat))
	mux.HandleFunc("/api/tool/approve", s.authManager.AuthMiddleware(s.handleToolApprove))
	mux.HandleFunc("/api/k8s/", s.authManager.AuthMiddleware(s.handleK8sResource))
	mux.HandleFunc("/api/audit", s.authManager.AuthMiddleware(s.handleAuditLogs))
	mux.HandleFunc("/api/reports", s.authManager.AuthMiddleware(s.reportGenerator.HandleReports))
	mux.HandleFunc("/api/settings", s.authManager.AuthMiddleware(s.handleSettings))
	mux.HandleFunc("/api/settings/llm", s.authManager.AuthMiddleware(s.handleLLMSettings))

	// WebSocket terminal handler
	terminalHandler := NewTerminalHandler(s.k8sClient)
	mux.HandleFunc("/api/terminal/", s.authManager.AuthMiddleware(terminalHandler.HandleTerminal))

	// Metrics endpoints
	mux.HandleFunc("/api/metrics/pods", s.authManager.AuthMiddleware(s.handlePodMetrics))
	mux.HandleFunc("/api/metrics/nodes", s.authManager.AuthMiddleware(s.handleNodeMetrics))

	// Port forwarding endpoints
	mux.HandleFunc("/api/portforward/start", s.authManager.AuthMiddleware(s.handlePortForwardStart))
	mux.HandleFunc("/api/portforward/list", s.authManager.AuthMiddleware(s.handlePortForwardList))
	mux.HandleFunc("/api/portforward/", s.authManager.AuthMiddleware(s.handlePortForwardStop))

	// Static files
	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		mux.Handle("/", http.FileServer(http.Dir("pkg/web/static")))
	} else {
		mux.Handle("/", http.FileServer(http.FS(staticFS)))
	}

	s.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", s.port),
		Handler: corsMiddleware(mux),
	}

	fmt.Printf("\n  Web server started at http://localhost:%d\n", s.port)
	return s.server.ListenAndServe()
}

func (s *Server) Stop() error {
	db.Close()
	if s.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return s.server.Shutdown(ctx)
	}
	return nil
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	status := map[string]interface{}{
		"status":       "ok",
		"timestamp":    time.Now(),
		"ai_ready":     s.aiClient != nil && s.aiClient.IsReady(),
		"k8s_ready":    s.k8sClient != nil,
		"db_ready":     db.DB != nil,
		"auth_enabled": s.authManager.config.Enabled,
		"auth_mode":    s.authManager.GetAuthMode(),
		"version":      "1.0.0",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	username := r.Header.Get("X-Username")
	if username == "" {
		username = "anonymous"
	}

	// Record audit log for AI query
	db.RecordAudit(db.AuditEntry{
		User:       username,
		Action:     "ai_query",
		Resource:   "chat",
		Details:    fmt.Sprintf("Query: %s", truncateString(req.Message, 100)),
		LLMRequest: req.Message,
	})

	if s.aiClient == nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ChatResponse{
			Error: "AI client not configured",
		})
		return
	}

	var response strings.Builder
	err := s.aiClient.Ask(r.Context(), req.Message, func(text string) {
		response.WriteString(text)
	})

	w.Header().Set("Content-Type", "application/json")

	if err != nil {
		json.NewEncoder(w).Encode(ChatResponse{
			Error: err.Error(),
		})
		return
	}

	// Update audit log with response
	db.RecordAudit(db.AuditEntry{
		User:        username,
		Action:      "ai_response",
		Resource:    "chat",
		Details:     fmt.Sprintf("Response length: %d chars", len(response.String())),
		LLMResponse: truncateString(response.String(), 500),
	})

	json.NewEncoder(w).Encode(ChatResponse{
		Response: response.String(),
	})
}

func (s *Server) handleChatStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	username := r.Header.Get("X-Username")
	if username == "" {
		username = "anonymous"
	}

	// Record audit log
	db.RecordAudit(db.AuditEntry{
		User:       username,
		Action:     "ai_query_stream",
		Resource:   "chat",
		Details:    fmt.Sprintf("Query: %s", truncateString(req.Message, 100)),
		LLMRequest: req.Message,
	})

	if s.aiClient == nil {
		http.Error(w, "AI client not configured", http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	sse := &SSEWriter{w: w, flusher: flusher}

	err := s.aiClient.Ask(r.Context(), req.Message, func(text string) {
		escaped := strings.ReplaceAll(text, "\n", "\\n")
		sse.Write(escaped)
	})

	if err != nil {
		sse.Write(fmt.Sprintf("[ERROR] %s", err.Error()))
	}

	sse.Write("[DONE]")
}

func (s *Server) handleK8sResource(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/k8s/")
	parts := strings.Split(path, "/")
	resource := parts[0]

	namespace := r.URL.Query().Get("namespace")
	if namespace == "" {
		namespace = "default"
	}

	username := r.Header.Get("X-Username")
	if username == "" {
		username = "anonymous"
	}

	// Record audit log
	db.RecordAudit(db.AuditEntry{
		User:     username,
		Action:   "view",
		Resource: resource,
		Details:  fmt.Sprintf("namespace=%s", namespace),
	})

	w.Header().Set("Content-Type", "application/json")

	var items []map[string]interface{}
	var err error

	switch resource {
	case "pods":
		pods, e := s.k8sClient.ListPods(r.Context(), namespace)
		err = e
		if err == nil {
			items = make([]map[string]interface{}, len(pods))
			for i, pod := range pods {
				items[i] = map[string]interface{}{
					"name":      pod.Name,
					"namespace": pod.Namespace,
					"status":    string(pod.Status.Phase),
					"ready":     getPodReadyCount(&pod),
					"restarts":  getPodRestarts(&pod),
					"age":       time.Since(pod.CreationTimestamp.Time).Round(time.Second).String(),
					"node":      pod.Spec.NodeName,
					"ip":        pod.Status.PodIP,
				}
			}
		}

	case "deployments":
		deps, e := s.k8sClient.ListDeployments(r.Context(), namespace)
		err = e
		if err == nil {
			items = make([]map[string]interface{}, len(deps))
			for i, dep := range deps {
				replicas := int32(1)
				if dep.Spec.Replicas != nil {
					replicas = *dep.Spec.Replicas
				}
				items[i] = map[string]interface{}{
					"name":      dep.Name,
					"namespace": dep.Namespace,
					"ready":     fmt.Sprintf("%d/%d", dep.Status.ReadyReplicas, replicas),
					"upToDate":  dep.Status.UpdatedReplicas,
					"available": dep.Status.AvailableReplicas,
					"age":       time.Since(dep.CreationTimestamp.Time).Round(time.Second).String(),
				}
			}
		}

	case "services":
		svcs, e := s.k8sClient.ListServices(r.Context(), namespace)
		err = e
		if err == nil {
			items = make([]map[string]interface{}, len(svcs))
			for i, svc := range svcs {
				ports := make([]string, len(svc.Spec.Ports))
				for j, p := range svc.Spec.Ports {
					ports[j] = fmt.Sprintf("%d/%s", p.Port, p.Protocol)
				}
				items[i] = map[string]interface{}{
					"name":       svc.Name,
					"namespace":  svc.Namespace,
					"type":       string(svc.Spec.Type),
					"clusterIP":  svc.Spec.ClusterIP,
					"externalIP": getExternalIPs(&svc),
					"ports":      strings.Join(ports, ", "),
					"age":        time.Since(svc.CreationTimestamp.Time).Round(time.Second).String(),
				}
			}
		}

	case "namespaces":
		nss, e := s.k8sClient.ListNamespaces(r.Context())
		err = e
		if err == nil {
			items = make([]map[string]interface{}, len(nss))
			for i, ns := range nss {
				items[i] = map[string]interface{}{
					"name":   ns.Name,
					"status": string(ns.Status.Phase),
					"age":    time.Since(ns.CreationTimestamp.Time).Round(time.Second).String(),
				}
			}
		}

	case "nodes":
		nodes, e := s.k8sClient.ListNodes(r.Context())
		err = e
		if err == nil {
			items = make([]map[string]interface{}, len(nodes))
			for i, node := range nodes {
				items[i] = map[string]interface{}{
					"name":    node.Name,
					"status":  getNodeStatus(&node),
					"roles":   getNodeRoles(&node),
					"version": node.Status.NodeInfo.KubeletVersion,
					"age":     time.Since(node.CreationTimestamp.Time).Round(time.Second).String(),
				}
			}
		}

	case "events":
		events, e := s.k8sClient.ListEvents(r.Context(), namespace)
		err = e
		if err == nil {
			items = make([]map[string]interface{}, len(events))
			for i, ev := range events {
				items[i] = map[string]interface{}{
					"name":      ev.Name,
					"namespace": ev.Namespace,
					"type":      ev.Type,
					"reason":    ev.Reason,
					"message":   ev.Message,
					"count":     ev.Count,
					"lastSeen":  ev.LastTimestamp.Time.Format(time.RFC3339),
				}
			}
		}

	default:
		http.Error(w, fmt.Sprintf("Unknown resource type: %s", resource), http.StatusNotFound)
		return
	}

	if err != nil {
		json.NewEncoder(w).Encode(K8sResourceResponse{
			Kind:      resource,
			Error:     err.Error(),
			Timestamp: time.Now(),
		})
		return
	}

	json.NewEncoder(w).Encode(K8sResourceResponse{
		Kind:      resource,
		Items:     items,
		Timestamp: time.Now(),
	})
}

func (s *Server) handleAuditLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	logs, err := db.GetAuditLogs()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"logs":      logs,
		"timestamp": time.Now(),
	})
}

func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		// Return current settings (without sensitive data)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"language":      s.cfg.Language,
			"beginner_mode": s.cfg.BeginnerMode,
			"enable_audit":  s.cfg.EnableAudit,
			"log_level":     s.cfg.LogLevel,
			"llm": map[string]interface{}{
				"provider": s.cfg.LLM.Provider,
				"model":    s.cfg.LLM.Model,
				"endpoint": s.cfg.LLM.Endpoint,
			},
		})

	case http.MethodPut:
		var newSettings struct {
			Language     string `json:"language"`
			BeginnerMode bool   `json:"beginner_mode"`
			EnableAudit  bool   `json:"enable_audit"`
			LogLevel     string `json:"log_level"`
		}

		if err := json.NewDecoder(r.Body).Decode(&newSettings); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Update settings
		s.cfg.Language = newSettings.Language
		s.cfg.BeginnerMode = newSettings.BeginnerMode
		s.cfg.EnableAudit = newSettings.EnableAudit
		s.cfg.LogLevel = newSettings.LogLevel

		// Save to disk
		if err := s.cfg.Save(); err != nil {
			http.Error(w, "Failed to save settings", http.StatusInternalServerError)
			return
		}

		// Record audit
		username := r.Header.Get("X-Username")
		db.RecordAudit(db.AuditEntry{
			User:     username,
			Action:   "update_settings",
			Resource: "settings",
			Details:  "Settings updated",
		})

		json.NewEncoder(w).Encode(map[string]string{"status": "saved"})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleLLMSettings(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodPut:
		var llmSettings struct {
			Provider string `json:"provider"`
			Model    string `json:"model"`
			Endpoint string `json:"endpoint"`
			APIKey   string `json:"api_key"`
		}

		if err := json.NewDecoder(r.Body).Decode(&llmSettings); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Update LLM settings
		s.cfg.LLM.Provider = llmSettings.Provider
		s.cfg.LLM.Model = llmSettings.Model
		s.cfg.LLM.Endpoint = llmSettings.Endpoint
		if llmSettings.APIKey != "" {
			s.cfg.LLM.APIKey = llmSettings.APIKey
		}

		// Recreate AI client
		if s.cfg.LLM.Endpoint != "" {
			newClient, err := ai.NewClient(&s.cfg.LLM)
			if err != nil {
				http.Error(w, fmt.Sprintf("Failed to create AI client: %v", err), http.StatusBadRequest)
				return
			}
			s.aiClient = newClient
		}

		// Save to disk
		if err := s.cfg.Save(); err != nil {
			http.Error(w, "Failed to save settings", http.StatusInternalServerError)
			return
		}

		// Record audit
		username := r.Header.Get("X-Username")
		db.RecordAudit(db.AuditEntry{
			User:     username,
			Action:   "update_llm_settings",
			Resource: "settings",
			Details:  fmt.Sprintf("Provider: %s, Model: %s", llmSettings.Provider, llmSettings.Model),
		})

		json.NewEncoder(w).Encode(map[string]string{"status": "saved"})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// Helper functions
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func getPodReadyCount(pod *corev1.Pod) string {
	ready := 0
	total := len(pod.Status.ContainerStatuses)
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.Ready {
			ready++
		}
	}
	return fmt.Sprintf("%d/%d", ready, total)
}

func getPodRestarts(pod *corev1.Pod) int32 {
	var restarts int32
	for _, cs := range pod.Status.ContainerStatuses {
		restarts += cs.RestartCount
	}
	return restarts
}

func getExternalIPs(svc *corev1.Service) string {
	if len(svc.Status.LoadBalancer.Ingress) > 0 {
		ips := make([]string, len(svc.Status.LoadBalancer.Ingress))
		for i, ing := range svc.Status.LoadBalancer.Ingress {
			if ing.IP != "" {
				ips[i] = ing.IP
			} else {
				ips[i] = ing.Hostname
			}
		}
		return strings.Join(ips, ", ")
	}
	if len(svc.Spec.ExternalIPs) > 0 {
		return strings.Join(svc.Spec.ExternalIPs, ", ")
	}
	return "<none>"
}

func getNodeStatus(node *corev1.Node) string {
	for _, cond := range node.Status.Conditions {
		if cond.Type == corev1.NodeReady {
			if cond.Status == corev1.ConditionTrue {
				return "Ready"
			}
			return "NotReady"
		}
	}
	return "Unknown"
}

func getNodeRoles(node *corev1.Node) string {
	roles := []string{}
	for label := range node.Labels {
		if strings.HasPrefix(label, "node-role.kubernetes.io/") {
			role := strings.TrimPrefix(label, "node-role.kubernetes.io/")
			roles = append(roles, role)
		}
	}
	if len(roles) == 0 {
		return "<none>"
	}
	return strings.Join(roles, ", ")
}

// classifyCommand categorizes a kubectl command for safety
func classifyCommand(command string) string {
	command = strings.ToLower(command)

	// Dangerous commands
	dangerousPatterns := []string{
		"delete", "--force", "--grace-period=0", "drain", "cordon",
		"taint", "--all", "replace --force", "rollout undo",
	}
	for _, pattern := range dangerousPatterns {
		if strings.Contains(command, pattern) {
			return "dangerous"
		}
	}

	// Write commands
	writePatterns := []string{
		"create", "apply", "patch", "edit", "scale", "set",
		"label", "annotate", "expose", "run", "exec", "cp",
		"rollout restart", "rollout pause", "rollout resume",
	}
	for _, pattern := range writePatterns {
		if strings.Contains(command, pattern) {
			return "write"
		}
	}

	// Read-only commands
	return "read-only"
}

// handleAgenticChat handles AI chat with tool calling (Decision Required flow)
func (s *Server) handleAgenticChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	username := r.Header.Get("X-Username")
	if username == "" {
		username = "anonymous"
	}

	// Record audit log
	db.RecordAudit(db.AuditEntry{
		User:       username,
		Action:     "ai_agentic_query",
		Resource:   "chat",
		Details:    fmt.Sprintf("Query: %s", truncateString(req.Message, 100)),
		LLMRequest: req.Message,
	})

	if s.aiClient == nil {
		http.Error(w, "AI client not configured", http.StatusServiceUnavailable)
		return
	}

	// Check if provider supports tool calling
	if !s.aiClient.SupportsTools() {
		http.Error(w, "AI provider does not support tool calling", http.StatusBadRequest)
		return
	}

	// Set up SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	sse := &SSEWriter{w: w, flusher: flusher}

	// Tool approval callback
	toolApprovalCallback := func(toolName string, argsJSON string) bool {
		// Parse arguments to get the command
		var args map[string]interface{}
		json.Unmarshal([]byte(argsJSON), &args)

		command := ""
		if cmd, ok := args["command"].(string); ok {
			command = cmd
		}

		// Classify the command
		category := classifyCommand(command)

		// Auto-approve read-only commands
		if category == "read-only" {
			return true
		}

		// Create pending approval
		approvalID := fmt.Sprintf("approval_%d", time.Now().UnixNano())
		approval := &PendingToolApproval{
			ID:        approvalID,
			ToolName:  toolName,
			Command:   command,
			Category:  category,
			Timestamp: time.Now(),
			Response:  make(chan bool, 1),
		}

		s.pendingApprovalMutex.Lock()
		s.pendingApprovals[approvalID] = approval
		s.pendingApprovalMutex.Unlock()

		// Send approval request via SSE
		approvalJSON, _ := json.Marshal(map[string]interface{}{
			"type":      "approval_required",
			"id":        approvalID,
			"tool_name": toolName,
			"command":   command,
			"category":  category,
		})
		sse.WriteEvent("approval", string(approvalJSON))

		// Wait for approval with timeout
		select {
		case approved := <-approval.Response:
			// Cleanup
			s.pendingApprovalMutex.Lock()
			delete(s.pendingApprovals, approvalID)
			s.pendingApprovalMutex.Unlock()

			if approved {
				// Log the approved action
				db.RecordAudit(db.AuditEntry{
					User:     username,
					Action:   "tool_approved",
					Resource: toolName,
					Details:  fmt.Sprintf("Command: %s", command),
				})
			}
			return approved

		case <-time.After(60 * time.Second):
			// Timeout - cleanup and reject
			s.pendingApprovalMutex.Lock()
			delete(s.pendingApprovals, approvalID)
			s.pendingApprovalMutex.Unlock()

			sse.WriteEvent("approval_timeout", approvalID)
			return false

		case <-r.Context().Done():
			// Request cancelled
			s.pendingApprovalMutex.Lock()
			delete(s.pendingApprovals, approvalID)
			s.pendingApprovalMutex.Unlock()
			return false
		}
	}

	// Run agentic chat
	err := s.aiClient.AskWithTools(r.Context(), req.Message, func(text string) {
		escaped := strings.ReplaceAll(text, "\n", "\\n")
		sse.Write(escaped)
	}, toolApprovalCallback)

	if err != nil {
		sse.Write(fmt.Sprintf("[ERROR] %s", err.Error()))
	}

	sse.Write("[DONE]")
}

// handleToolApprove handles user approval/rejection of tool calls
func (s *Server) handleToolApprove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID       string `json:"id"`
		Approved bool   `json:"approved"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	s.pendingApprovalMutex.RLock()
	approval, exists := s.pendingApprovals[req.ID]
	s.pendingApprovalMutex.RUnlock()

	if !exists {
		http.Error(w, "Approval not found or expired", http.StatusNotFound)
		return
	}

	// Send response (non-blocking)
	select {
	case approval.Response <- req.Approved:
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	default:
		http.Error(w, "Approval already processed", http.StatusConflict)
	}
}

// SSEWriter helper for writing SSE events
func (sse *SSEWriter) WriteEvent(event string, data string) {
	fmt.Fprintf(sse.w, "event: %s\ndata: %s\n\n", event, data)
	sse.flusher.Flush()
}

// ==========================================
// Metrics Handlers
// ==========================================

// PodMetricItem represents pod resource usage for API response
type PodMetricItem struct {
	Name      string  `json:"name"`
	Namespace string  `json:"namespace"`
	CPU       float64 `json:"cpu"`    // millicores
	Memory    float64 `json:"memory"` // MiB
}

func (s *Server) handlePodMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	namespace := r.URL.Query().Get("namespace")
	w.Header().Set("Content-Type", "application/json")

	// Try to get metrics from metrics-server
	metricsMap, err := s.k8sClient.GetPodMetrics(r.Context(), namespace)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "Metrics server not available: " + err.Error(),
			"items": []PodMetricItem{},
		})
		return
	}

	// Convert map to slice
	var items []PodMetricItem
	for name, values := range metricsMap {
		if len(values) >= 2 {
			items = append(items, PodMetricItem{
				Name:      name,
				Namespace: namespace,
				CPU:       float64(values[0]),
				Memory:    float64(values[1]),
			})
		}
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"items": items,
	})
}

func (s *Server) handleNodeMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	metricsMap, err := s.k8sClient.GetNodeMetrics(r.Context())
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "Metrics server not available: " + err.Error(),
			"items": []interface{}{},
		})
		return
	}

	// Convert map to slice
	var items []map[string]interface{}
	for name, values := range metricsMap {
		if len(values) >= 2 {
			items = append(items, map[string]interface{}{
				"name":   name,
				"cpu":    values[0],
				"memory": values[1],
			})
		}
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"items": items,
	})
}

// ==========================================
// Port Forwarding Handlers
// ==========================================

// PortForwardSession represents an active port forward
type PortForwardSession struct {
	ID         string    `json:"id"`
	Namespace  string    `json:"namespace"`
	Pod        string    `json:"pod"`
	LocalPort  int       `json:"localPort"`
	RemotePort int       `json:"remotePort"`
	Active     bool      `json:"active"`
	StartedAt  time.Time `json:"startedAt"`
	stopChan   chan struct{}
}

var (
	portForwardSessions = make(map[string]*PortForwardSession)
	pfMutex             sync.Mutex
)

func (s *Server) handlePortForwardStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Namespace  string `json:"namespace"`
		Pod        string `json:"pod"`
		LocalPort  int    `json:"localPort"`
		RemotePort int    `json:"remotePort"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Generate session ID
	sessionID := fmt.Sprintf("pf-%d", time.Now().UnixNano())

	session := &PortForwardSession{
		ID:         sessionID,
		Namespace:  req.Namespace,
		Pod:        req.Pod,
		LocalPort:  req.LocalPort,
		RemotePort: req.RemotePort,
		Active:     true,
		StartedAt:  time.Now(),
		stopChan:   make(chan struct{}),
	}

	// Start port forward in goroutine
	go func() {
		err := s.k8sClient.StartPortForward(
			req.Namespace,
			req.Pod,
			req.LocalPort,
			req.RemotePort,
			session.stopChan,
		)
		if err != nil {
			fmt.Printf("Port forward error: %v\n", err)
		}
		pfMutex.Lock()
		if s, ok := portForwardSessions[sessionID]; ok {
			s.Active = false
		}
		pfMutex.Unlock()
	}()

	pfMutex.Lock()
	portForwardSessions[sessionID] = session
	pfMutex.Unlock()

	// Record audit
	username := r.Header.Get("X-Username")
	db.RecordAudit(db.AuditEntry{
		User:     username,
		Action:   "port_forward_start",
		Resource: "pod",
		Details:  fmt.Sprintf("%s/%s local:%d remote:%d", req.Namespace, req.Pod, req.LocalPort, req.RemotePort),
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(session)
}

func (s *Server) handlePortForwardList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	pfMutex.Lock()
	sessions := make([]*PortForwardSession, 0, len(portForwardSessions))
	for _, s := range portForwardSessions {
		sessions = append(sessions, s)
	}
	pfMutex.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"items": sessions,
	})
}

func (s *Server) handlePortForwardStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract session ID from path
	path := strings.TrimPrefix(r.URL.Path, "/api/portforward/")
	sessionID := strings.TrimSuffix(path, "/")

	pfMutex.Lock()
	session, ok := portForwardSessions[sessionID]
	if !ok {
		pfMutex.Unlock()
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}

	// Stop the port forward
	close(session.stopChan)
	delete(portForwardSessions, sessionID)
	pfMutex.Unlock()

	// Record audit
	username := r.Header.Get("X-Username")
	db.RecordAudit(db.AuditEntry{
		User:     username,
		Action:   "port_forward_stop",
		Resource: "pod",
		Details:  fmt.Sprintf("%s/%s", session.Namespace, session.Pod),
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "stopped"})
}

package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/kube-ai-dashbaord/kube-ai-dashboard-cli/pkg/ai"
	"github.com/kube-ai-dashbaord/kube-ai-dashboard-cli/pkg/config"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/kube-ai-dashbaord/kube-ai-dashboard-cli/pkg/k8s"
)

// E2E Test helpers
func setupTestServer(t *testing.T) (*Server, *AuthManager) {
	t.Helper()

	cfg := &config.Config{
		Language:     "en",
		BeginnerMode: false,
		EnableAudit:  true,
		LogLevel:     "debug",
		LLM: config.LLMConfig{
			Provider: "openai",
			Model:    "gpt-4",
			Endpoint: "",
		},
	}

	authConfig := &AuthConfig{
		Enabled:         true,
		SessionDuration: time.Hour,
		AuthMode:        "local", // Use local auth mode for tests
		DefaultAdmin:    "admin",
		DefaultPassword: "admin123",
	}
	authManager := NewAuthManager(authConfig)

	// Create fake k8s client
	fakeClientset := fake.NewSimpleClientset(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-pod-1",
				Namespace: "default",
			},
			Spec: corev1.PodSpec{
				NodeName: "test-node",
			},
			Status: corev1.PodStatus{
				Phase: corev1.PodRunning,
				PodIP: "10.0.0.1",
			},
		},
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-service",
				Namespace: "default",
			},
			Spec: corev1.ServiceSpec{
				Type:      corev1.ServiceTypeClusterIP,
				ClusterIP: "10.0.0.100",
			},
		},
		&corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-node",
				Labels: map[string]string{
					"node-role.kubernetes.io/control-plane": "",
				},
			},
			Status: corev1.NodeStatus{
				Conditions: []corev1.NodeCondition{
					{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
				},
			},
		},
	)

	k8sClient := &k8s.Client{
		Clientset: fakeClientset,
	}

	server := &Server{
		cfg:         cfg,
		aiClient:    nil,
		k8sClient:   k8sClient,
		authManager: authManager,
		port:        8080,
	}
	server.reportGenerator = NewReportGenerator(server)

	return server, authManager
}

// E2E Test: Full login flow
func TestE2E_LoginFlow(t *testing.T) {
	_, authManager := setupTestServer(t)

	// Step 1: Login
	loginBody, _ := json.Marshal(map[string]string{
		"username": "admin",
		"password": "admin123",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBuffer(loginBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	authManager.HandleLogin(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("login failed: expected 200, got %d", w.Code)
	}

	var loginResp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &loginResp); err != nil {
		t.Fatalf("failed to parse login response: %v", err)
	}

	token, ok := loginResp["token"].(string)
	if !ok || token == "" {
		t.Fatal("expected token in login response")
	}

	// Step 2: Access protected endpoint with token
	meReq := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	meReq.Header.Set("Authorization", "Bearer "+token)
	meW := httptest.NewRecorder()

	authManager.AuthMiddleware(authManager.HandleCurrentUser).ServeHTTP(meW, meReq)

	if meW.Code != http.StatusOK {
		t.Errorf("expected 200 for authenticated request, got %d", meW.Code)
	}

	// Step 3: Logout
	logoutReq := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	logoutReq.AddCookie(&http.Cookie{Name: "k13s_session", Value: token})
	logoutW := httptest.NewRecorder()

	authManager.HandleLogout(logoutW, logoutReq)

	if logoutW.Code != http.StatusOK {
		t.Errorf("logout failed: expected 200, got %d", logoutW.Code)
	}

	// Step 4: Verify session is invalidated
	meReq2 := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	meReq2.Header.Set("Authorization", "Bearer "+token)
	meW2 := httptest.NewRecorder()

	authManager.AuthMiddleware(authManager.HandleCurrentUser).ServeHTTP(meW2, meReq2)

	if meW2.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for invalidated session, got %d", meW2.Code)
	}
}

// E2E Test: K8s resources access
func TestE2E_K8sResourcesAccess(t *testing.T) {
	server, authManager := setupTestServer(t)

	// Login first
	session, err := authManager.Authenticate("admin", "admin123")
	if err != nil {
		t.Fatalf("authentication failed: %v", err)
	}

	tests := []struct {
		name           string
		resource       string
		expectedKind   string
		expectedStatus int
	}{
		{"pods", "pods", "pods", http.StatusOK},
		{"services", "services", "services", http.StatusOK},
		{"namespaces", "namespaces", "namespaces", http.StatusOK},
		{"nodes", "nodes", "nodes", http.StatusOK},
		{"unknown", "unknown-resource", "", http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/k8s/"+tt.resource+"?namespace=default", nil)
			req.Header.Set("Authorization", "Bearer "+session.ID)
			w := httptest.NewRecorder()

			authManager.AuthMiddleware(http.HandlerFunc(server.handleK8sResource)).ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if tt.expectedStatus == http.StatusOK {
				var resp K8sResourceResponse
				if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
					t.Fatalf("failed to parse response: %v", err)
				}

				if resp.Kind != tt.expectedKind {
					t.Errorf("expected kind %s, got %s", tt.expectedKind, resp.Kind)
				}
			}
		})
	}
}

// E2E Test: Health endpoint
func TestE2E_HealthEndpoint(t *testing.T) {
	server, _ := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	w := httptest.NewRecorder()

	server.handleHealth(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("health check failed: expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse health response: %v", err)
	}

	if resp["status"] != "ok" {
		t.Errorf("expected status 'ok', got %v", resp["status"])
	}

	if resp["k8s_ready"] != true {
		t.Errorf("expected k8s_ready to be true")
	}
}

// E2E Test: Settings endpoint
func TestE2E_SettingsEndpoint(t *testing.T) {
	server, authManager := setupTestServer(t)

	session, _ := authManager.Authenticate("admin", "admin123")

	// GET settings
	req := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
	req.Header.Set("Authorization", "Bearer "+session.ID)
	w := httptest.NewRecorder()

	authManager.AuthMiddleware(http.HandlerFunc(server.handleSettings)).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("get settings failed: expected 200, got %d", w.Code)
	}

	var settings map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &settings); err != nil {
		t.Fatalf("failed to parse settings: %v", err)
	}

	if settings["language"] != "en" {
		t.Errorf("expected language 'en', got %v", settings["language"])
	}
}

// E2E Test: User management flow
func TestE2E_UserManagement(t *testing.T) {
	_, authManager := setupTestServer(t)

	// Create a new user
	err := authManager.CreateUser("testuser", "testpass123", "user")
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	// Login with new user
	session, err := authManager.Authenticate("testuser", "testpass123")
	if err != nil {
		t.Fatalf("failed to login with new user: %v", err)
	}

	if session.Role != "user" {
		t.Errorf("expected role 'user', got %s", session.Role)
	}

	// Change password
	err = authManager.ChangePassword("testuser", "testpass123", "newpass456")
	if err != nil {
		t.Fatalf("failed to change password: %v", err)
	}

	// Login with new password
	_, err = authManager.Authenticate("testuser", "newpass456")
	if err != nil {
		t.Error("failed to login with new password")
	}

	// Old password should fail
	_, err = authManager.Authenticate("testuser", "testpass123")
	if err == nil {
		t.Error("old password should not work")
	}

	// Delete user
	err = authManager.DeleteUser("testuser")
	if err != nil {
		t.Fatalf("failed to delete user: %v", err)
	}

	// Login should fail
	_, err = authManager.Authenticate("testuser", "newpass456")
	if err == nil {
		t.Error("deleted user should not be able to login")
	}
}

// E2E Test: LDAP integration (when disabled)
func TestE2E_LDAPDisabled(t *testing.T) {
	authConfig := &AuthConfig{
		Enabled:         true,
		SessionDuration: time.Hour,
		LDAP:            nil,
	}
	authManager := NewAuthManager(authConfig)

	if authManager.IsLDAPEnabled() {
		t.Error("LDAP should be disabled")
	}

	ldapConfig := authManager.GetLDAPConfig()
	if ldapConfig != nil {
		t.Error("LDAP config should be nil when disabled")
	}
}

// E2E Test: LDAP integration (when enabled)
func TestE2E_LDAPEnabled(t *testing.T) {
	authConfig := &AuthConfig{
		Enabled:         true,
		SessionDuration: time.Hour,
		LDAP: &LDAPConfig{
			Enabled:      true,
			Host:         "ldap.example.com",
			Port:         389,
			AdminGroups:  []string{"k8s-admins"},
			UserGroups:   []string{"k8s-users"},
			ViewerGroups: []string{"k8s-viewers"},
		},
	}
	authManager := NewAuthManager(authConfig)

	if !authManager.IsLDAPEnabled() {
		t.Error("LDAP should be enabled")
	}

	ldapConfig := authManager.GetLDAPConfig()
	if ldapConfig == nil {
		t.Fatal("LDAP config should not be nil")
	}

	if ldapConfig.Host != "ldap.example.com" {
		t.Errorf("expected host 'ldap.example.com', got %s", ldapConfig.Host)
	}

	// Test LDAP status endpoint
	req := httptest.NewRequest(http.MethodGet, "/api/auth/ldap/status", nil)
	w := httptest.NewRecorder()

	authManager.HandleLDAPStatus(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("LDAP status failed: expected 200, got %d", w.Code)
	}

	var status map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &status); err != nil {
		t.Fatalf("failed to parse LDAP status: %v", err)
	}

	if status["enabled"] != true {
		t.Error("expected LDAP to be enabled in status")
	}
}

// E2E Test: Reports generation
func TestE2E_ReportsGeneration(t *testing.T) {
	server, authManager := setupTestServer(t)

	session, _ := authManager.Authenticate("admin", "admin123")

	tests := []struct {
		name       string
		reportType string
		wantStatus int
	}{
		{"cluster health", "cluster-health", http.StatusOK},
		{"resource usage", "resource-usage", http.StatusOK},
		{"security audit", "security-audit", http.StatusOK},
		{"ai interactions", "ai-interactions", http.StatusOK},
		{"unknown type returns available types", "unknown-type", http.StatusOK}, // Returns available_types list
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/reports?type="+tt.reportType, nil)
			req.Header.Set("Authorization", "Bearer "+session.ID)
			w := httptest.NewRecorder()

			authManager.AuthMiddleware(http.HandlerFunc(server.reportGenerator.HandleReports)).ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d", tt.wantStatus, w.Code)
			}
		})
	}
}

// E2E Test: Reports available types
func TestE2E_ReportsAvailableTypes(t *testing.T) {
	server, authManager := setupTestServer(t)

	session, _ := authManager.Authenticate("admin", "admin123")

	req := httptest.NewRequest(http.MethodGet, "/api/reports", nil)
	req.Header.Set("Authorization", "Bearer "+session.ID)
	w := httptest.NewRecorder()

	authManager.AuthMiddleware(http.HandlerFunc(server.reportGenerator.HandleReports)).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	types, ok := resp["available_types"].([]interface{})
	if !ok {
		t.Fatal("expected available_types in response")
	}

	expectedTypes := []string{"cluster-health", "resource-usage", "security-audit", "ai-interactions"}
	if len(types) != len(expectedTypes) {
		t.Errorf("expected %d types, got %d", len(expectedTypes), len(types))
	}
}

// E2E Test: Chat endpoint without AI client
func TestE2E_ChatWithoutAI(t *testing.T) {
	server, authManager := setupTestServer(t)

	session, _ := authManager.Authenticate("admin", "admin123")

	body, _ := json.Marshal(ChatRequest{Message: "Hello"})
	req := httptest.NewRequest(http.MethodPost, "/api/chat", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+session.ID)
	w := httptest.NewRecorder()

	authManager.AuthMiddleware(http.HandlerFunc(server.handleChat)).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp ChatResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp.Error != "AI client not configured" {
		t.Errorf("expected 'AI client not configured' error, got %s", resp.Error)
	}
}

// E2E Test: Chat endpoint with mocked AI client
func TestE2E_ChatWithMockedAI(t *testing.T) {
	server, authManager := setupTestServer(t)

	// Create mock AI server
	mockAIServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)

		resp := `{"id":"test","choices":[{"delta":{"content":"AI Response"},"finish_reason":"stop"}]}`
		w.Write([]byte("data: " + resp + "\n\n"))
		flusher.Flush()
		w.Write([]byte("data: [DONE]\n\n"))
		flusher.Flush()
	}))
	defer mockAIServer.Close()

	// Set up AI client
	aiCfg := &config.LLMConfig{
		Provider: "openai",
		Model:    "gpt-4",
		Endpoint: mockAIServer.URL,
		APIKey:   "test-key",
	}
	aiClient, err := ai.NewClient(aiCfg)
	if err != nil {
		t.Fatalf("failed to create AI client: %v", err)
	}
	server.aiClient = aiClient

	session, _ := authManager.Authenticate("admin", "admin123")

	body, _ := json.Marshal(ChatRequest{Message: "Hello AI"})
	req := httptest.NewRequest(http.MethodPost, "/api/chat", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+session.ID)
	w := httptest.NewRecorder()

	authManager.AuthMiddleware(http.HandlerFunc(server.handleChat)).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp ChatResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp.Response != "AI Response" {
		t.Errorf("expected 'AI Response', got '%s'", resp.Response)
	}
}

// E2E Test: CORS middleware
func TestE2E_CORSHeaders(t *testing.T) {
	handler := corsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Test preflight request
	req := httptest.NewRequest(http.MethodOptions, "/api/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("expected Access-Control-Allow-Origin: *")
	}

	if w.Header().Get("Access-Control-Allow-Methods") == "" {
		t.Error("expected Access-Control-Allow-Methods header")
	}

	if w.Header().Get("Access-Control-Allow-Headers") == "" {
		t.Error("expected Access-Control-Allow-Headers header")
	}
}

// E2E Test: Session expiration
func TestE2E_SessionExpiration(t *testing.T) {
	authConfig := &AuthConfig{
		Enabled:         true,
		SessionDuration: 100 * time.Millisecond, // Very short for testing
		AuthMode:        "local",
		DefaultAdmin:    "admin",
		DefaultPassword: "admin123",
	}
	authManager := NewAuthManager(authConfig)

	// Login
	session, err := authManager.Authenticate("admin", "admin123")
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	// Verify session works
	_, err = authManager.ValidateSession(session.ID)
	if err != nil {
		t.Error("session should be valid immediately after login")
	}

	// Wait for expiration
	time.Sleep(200 * time.Millisecond)

	// Session should be expired
	_, err = authManager.ValidateSession(session.ID)
	if err == nil {
		t.Error("session should be expired")
	}
}

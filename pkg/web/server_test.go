package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSSEWriter(t *testing.T) {
	w := httptest.NewRecorder()

	sse := &SSEWriter{
		w:       w,
		flusher: w,
	}

	err := sse.Write("test message")
	if err != nil {
		t.Errorf("SSEWriter.Write() error = %v", err)
	}

	expected := "data: test message\n\n"
	if w.Body.String() != expected {
		t.Errorf("SSEWriter output = %q, want %q", w.Body.String(), expected)
	}
}

func TestTruncateString(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{"short string", "hello", 10, "hello"},
		{"exact length", "hello", 5, "hello"},
		{"long string", "hello world", 5, "hello..."},
		{"empty string", "", 5, ""},
		{"zero max", "hello", 0, "..."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateString(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncateString() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGetPodReadyCount(t *testing.T) {
	tests := []struct {
		name string
		pod  *corev1.Pod
		want string
	}{
		{
			name: "all ready",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					ContainerStatuses: []corev1.ContainerStatus{
						{Ready: true},
						{Ready: true},
					},
				},
			},
			want: "2/2",
		},
		{
			name: "partial ready",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					ContainerStatuses: []corev1.ContainerStatus{
						{Ready: true},
						{Ready: false},
					},
				},
			},
			want: "1/2",
		},
		{
			name: "no containers",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					ContainerStatuses: []corev1.ContainerStatus{},
				},
			},
			want: "0/0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getPodReadyCount(tt.pod)
			if got != tt.want {
				t.Errorf("getPodReadyCount() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGetPodRestarts(t *testing.T) {
	tests := []struct {
		name string
		pod  *corev1.Pod
		want int32
	}{
		{
			name: "no restarts",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					ContainerStatuses: []corev1.ContainerStatus{
						{RestartCount: 0},
					},
				},
			},
			want: 0,
		},
		{
			name: "multiple restarts",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					ContainerStatuses: []corev1.ContainerStatus{
						{RestartCount: 3},
						{RestartCount: 2},
					},
				},
			},
			want: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getPodRestarts(tt.pod)
			if got != tt.want {
				t.Errorf("getPodRestarts() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestGetExternalIPs(t *testing.T) {
	tests := []struct {
		name string
		svc  *corev1.Service
		want string
	}{
		{
			name: "load balancer IP",
			svc: &corev1.Service{
				Status: corev1.ServiceStatus{
					LoadBalancer: corev1.LoadBalancerStatus{
						Ingress: []corev1.LoadBalancerIngress{
							{IP: "10.0.0.1"},
						},
					},
				},
			},
			want: "10.0.0.1",
		},
		{
			name: "load balancer hostname",
			svc: &corev1.Service{
				Status: corev1.ServiceStatus{
					LoadBalancer: corev1.LoadBalancerStatus{
						Ingress: []corev1.LoadBalancerIngress{
							{Hostname: "lb.example.com"},
						},
					},
				},
			},
			want: "lb.example.com",
		},
		{
			name: "external IPs",
			svc: &corev1.Service{
				Spec: corev1.ServiceSpec{
					ExternalIPs: []string{"192.168.1.1", "192.168.1.2"},
				},
			},
			want: "192.168.1.1, 192.168.1.2",
		},
		{
			name: "no external IPs",
			svc:  &corev1.Service{},
			want: "<none>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getExternalIPs(tt.svc)
			if got != tt.want {
				t.Errorf("getExternalIPs() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGetNodeStatus(t *testing.T) {
	tests := []struct {
		name string
		node *corev1.Node
		want string
	}{
		{
			name: "ready",
			node: &corev1.Node{
				Status: corev1.NodeStatus{
					Conditions: []corev1.NodeCondition{
						{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
					},
				},
			},
			want: "Ready",
		},
		{
			name: "not ready",
			node: &corev1.Node{
				Status: corev1.NodeStatus{
					Conditions: []corev1.NodeCondition{
						{Type: corev1.NodeReady, Status: corev1.ConditionFalse},
					},
				},
			},
			want: "NotReady",
		},
		{
			name: "no conditions",
			node: &corev1.Node{
				Status: corev1.NodeStatus{
					Conditions: []corev1.NodeCondition{},
				},
			},
			want: "Unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getNodeStatus(tt.node)
			if got != tt.want {
				t.Errorf("getNodeStatus() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGetNodeRoles(t *testing.T) {
	tests := []struct {
		name string
		node *corev1.Node
		want string
	}{
		{
			name: "control-plane",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"node-role.kubernetes.io/control-plane": "",
					},
				},
			},
			want: "control-plane",
		},
		{
			name: "no roles",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"other-label": "value",
					},
				},
			},
			want: "<none>",
		},
		{
			name: "empty labels",
			node: &corev1.Node{},
			want: "<none>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getNodeRoles(tt.node)
			if got != tt.want {
				t.Errorf("getNodeRoles() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCorsMiddleware(t *testing.T) {
	handlerCalled := false
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	tests := []struct {
		name           string
		method         string
		expectedStatus int
		handlerCalled  bool
	}{
		{"OPTIONS preflight", http.MethodOptions, http.StatusOK, false},
		{"GET request", http.MethodGet, http.StatusOK, true},
		{"POST request", http.MethodPost, http.StatusOK, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handlerCalled = false
			req := httptest.NewRequest(tt.method, "/test", nil)
			w := httptest.NewRecorder()

			corsMiddleware(testHandler).ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.expectedStatus)
			}

			if handlerCalled != tt.handlerCalled {
				t.Errorf("handler called = %v, want %v", handlerCalled, tt.handlerCalled)
			}

			// Check CORS headers
			if w.Header().Get("Access-Control-Allow-Origin") != "*" {
				t.Error("expected Access-Control-Allow-Origin header")
			}
		})
	}
}

func TestChatRequest_JSON(t *testing.T) {
	req := ChatRequest{Message: "test message"}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var decoded ChatRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if decoded.Message != req.Message {
		t.Errorf("decoded message = %q, want %q", decoded.Message, req.Message)
	}
}

func TestChatResponse_JSON(t *testing.T) {
	resp := ChatResponse{
		Response: "AI response",
		Command:  "kubectl get pods",
		Error:    "",
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var decoded ChatResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if decoded.Response != resp.Response {
		t.Errorf("decoded response = %q, want %q", decoded.Response, resp.Response)
	}
}

func TestK8sResourceResponse_JSON(t *testing.T) {
	resp := K8sResourceResponse{
		Kind: "pods",
		Items: []map[string]interface{}{
			{"name": "test-pod", "namespace": "default"},
		},
		Timestamp: time.Now(),
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var decoded K8sResourceResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if decoded.Kind != resp.Kind {
		t.Errorf("decoded kind = %q, want %q", decoded.Kind, resp.Kind)
	}

	if len(decoded.Items) != len(resp.Items) {
		t.Errorf("decoded items count = %d, want %d", len(decoded.Items), len(resp.Items))
	}
}

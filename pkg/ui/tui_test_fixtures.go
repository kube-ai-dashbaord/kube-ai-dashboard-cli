package ui

import (
	"github.com/kube-ai-dashbaord/kube-ai-dashboard-cli/pkg/ai"
	"github.com/kube-ai-dashbaord/kube-ai-dashboard-cli/pkg/config"
	"github.com/kube-ai-dashbaord/kube-ai-dashboard-cli/pkg/k8s"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/utils/ptr"
)

// TestConfig returns a config suitable for testing.
func TestConfig() *config.Config {
	return &config.Config{
		Language:     "en",
		BeginnerMode: false,
	}
}

// TestConfigKorean returns a Korean config for i18n testing.
func TestConfigKorean() *config.Config {
	return &config.Config{
		Language:     "ko",
		BeginnerMode: false,
	}
}

// TestConfigBeginner returns a beginner mode config.
func TestConfigBeginner() *config.Config {
	return &config.Config{
		Language:     "en",
		BeginnerMode: true,
	}
}

// NewFakeK8sClient creates a k8s.Client with fake clientset containing test data.
func NewFakeK8sClient() *k8s.Client {
	objects := []runtime.Object{
		// Namespaces
		&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: "default"},
		},
		&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: "kube-system"},
		},
		&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: "production"},
		},

		// Pods
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "nginx-pod-1",
				Namespace: "default",
				Labels:    map[string]string{"app": "nginx"},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{Name: "nginx", Image: "nginx:1.21"},
				},
			},
			Status: corev1.PodStatus{
				Phase: corev1.PodRunning,
				Conditions: []corev1.PodCondition{
					{Type: corev1.PodReady, Status: corev1.ConditionTrue},
				},
			},
		},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "nginx-pod-2",
				Namespace: "default",
				Labels:    map[string]string{"app": "nginx"},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{Name: "nginx", Image: "nginx:1.21"},
				},
			},
			Status: corev1.PodStatus{
				Phase: corev1.PodRunning,
			},
		},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "failing-pod",
				Namespace: "default",
				Labels:    map[string]string{"app": "broken"},
			},
			Status: corev1.PodStatus{
				Phase: corev1.PodFailed,
				ContainerStatuses: []corev1.ContainerStatus{
					{
						Name: "app",
						State: corev1.ContainerState{
							Waiting: &corev1.ContainerStateWaiting{
								Reason:  "CrashLoopBackOff",
								Message: "Back-off 5m0s restarting failed container",
							},
						},
						RestartCount: 5,
					},
				},
			},
		},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "coredns-abc123",
				Namespace: "kube-system",
				Labels:    map[string]string{"k8s-app": "kube-dns"},
			},
			Status: corev1.PodStatus{
				Phase: corev1.PodRunning,
			},
		},

		// Deployments
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "nginx-deployment",
				Namespace: "default",
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: ptr.To(int32(3)),
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"app": "nginx"},
				},
			},
			Status: appsv1.DeploymentStatus{
				Replicas:          3,
				ReadyReplicas:     3,
				AvailableReplicas: 3,
			},
		},
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "api-server",
				Namespace: "production",
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: ptr.To(int32(5)),
			},
			Status: appsv1.DeploymentStatus{
				Replicas:          5,
				ReadyReplicas:     4,
				AvailableReplicas: 4,
			},
		},

		// Services
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "nginx-service",
				Namespace: "default",
			},
			Spec: corev1.ServiceSpec{
				Type: corev1.ServiceTypeClusterIP,
				Ports: []corev1.ServicePort{
					{Port: 80, TargetPort: intstr.FromInt(8080)},
				},
				Selector: map[string]string{"app": "nginx"},
			},
		},
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kubernetes",
				Namespace: "default",
			},
			Spec: corev1.ServiceSpec{
				Type:      corev1.ServiceTypeClusterIP,
				ClusterIP: "10.96.0.1",
				Ports: []corev1.ServicePort{
					{Port: 443},
				},
			},
		},

		// ConfigMaps
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "app-config",
				Namespace: "default",
			},
			Data: map[string]string{
				"config.yaml": "debug: true\nport: 8080",
			},
		},

		// Secrets
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "db-credentials",
				Namespace: "default",
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{
				"username": []byte("admin"),
				"password": []byte("secret123"),
			},
		},

		// Nodes
		&corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "node-1",
				Labels: map[string]string{
					"kubernetes.io/hostname":                "node-1",
					"node-role.kubernetes.io/control-plane": "",
				},
			},
			Status: corev1.NodeStatus{
				Conditions: []corev1.NodeCondition{
					{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
				},
				Capacity: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("4"),
					corev1.ResourceMemory: resource.MustParse("16Gi"),
				},
			},
		},
		&corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "node-2",
				Labels: map[string]string{
					"kubernetes.io/hostname": "node-2",
				},
			},
			Status: corev1.NodeStatus{
				Conditions: []corev1.NodeCondition{
					{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
				},
			},
		},

		// Events
		&corev1.Event{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "nginx-pod-1.event1",
				Namespace: "default",
			},
			InvolvedObject: corev1.ObjectReference{
				Kind:      "Pod",
				Name:      "nginx-pod-1",
				Namespace: "default",
			},
			Reason:  "Scheduled",
			Message: "Successfully assigned default/nginx-pod-1 to node-1",
			Type:    "Normal",
		},
		&corev1.Event{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "failing-pod.event1",
				Namespace: "default",
			},
			InvolvedObject: corev1.ObjectReference{
				Kind:      "Pod",
				Name:      "failing-pod",
				Namespace: "default",
			},
			Reason:  "BackOff",
			Message: "Back-off restarting failed container",
			Type:    "Warning",
		},

		// ServiceAccounts
		&corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "default",
				Namespace: "default",
			},
		},
	}

	fakeClient := fake.NewSimpleClientset(objects...)
	return &k8s.Client{Clientset: fakeClient}
}

// NewEmptyK8sClient creates a k8s.Client with empty fake clientset.
func NewEmptyK8sClient() *k8s.Client {
	return &k8s.Client{Clientset: fake.NewSimpleClientset()}
}

// NewFakeAIClient creates an AI client suitable for testing.
// Note: The LLM field is nil, so AI features won't actually call any provider.
func NewFakeAIClient() *ai.Client {
	return &ai.Client{}
}

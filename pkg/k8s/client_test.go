package k8s

import (
	"context"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestListPods(t *testing.T) {
	ctx := context.Background()
	clientset := fake.NewSimpleClientset(&v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
		},
	})
	client := &Client{Clientset: clientset}

	pods, err := client.ListPods(ctx, "default")
	if err != nil {
		t.Fatalf("ListPods failed: %v", err)
	}

	if len(pods) != 1 {
		t.Errorf("Expected 1 pod, got %d", len(pods))
	}
	if pods[0].Name != "test-pod" {
		t.Errorf("Expected pod name test-pod, got %s", pods[0].Name)
	}
}

func TestListNodes(t *testing.T) {
	ctx := context.Background()
	clientset := fake.NewSimpleClientset(&v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-node",
		},
	})
	client := &Client{Clientset: clientset}

	nodes, err := client.ListNodes(ctx)
	if err != nil {
		t.Fatalf("ListNodes failed: %v", err)
	}

	if len(nodes) != 1 {
		t.Errorf("Expected 1 node, got %d", len(nodes))
	}
}

package web

import (
	"testing"
)

func TestCalculateHealthScore(t *testing.T) {
	tests := []struct {
		name         string
		healthyNodes int
		totalNodes   int
		runningPods  int
		totalPods    int
		want         float64
	}{
		{"all healthy", 3, 3, 10, 10, 100.0},
		{"half healthy", 1, 2, 5, 10, 50.0},
		{"no nodes", 0, 0, 5, 10, 100.0},
		{"no pods", 2, 4, 0, 0, 100.0},
		{"mixed health", 2, 4, 8, 10, 65.0},
		{"all unhealthy", 0, 2, 0, 10, 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculateHealthScore(tt.healthyNodes, tt.totalNodes, tt.runningPods, tt.totalPods)
			if got != tt.want {
				t.Errorf("calculateHealthScore(%d, %d, %d, %d) = %v, want %v",
					tt.healthyNodes, tt.totalNodes, tt.runningPods, tt.totalPods, got, tt.want)
			}
		})
	}
}

func TestReport_Struct(t *testing.T) {
	report := &Report{
		ID:          "test-123",
		Title:       "Test Report",
		Type:        "cluster-health",
		GeneratedBy: "test-user",
		Data: map[string]interface{}{
			"key": "value",
		},
	}

	if report.ID != "test-123" {
		t.Errorf("expected ID 'test-123', got %s", report.ID)
	}

	if report.Type != "cluster-health" {
		t.Errorf("expected type 'cluster-health', got %s", report.Type)
	}

	if report.Data["key"] != "value" {
		t.Error("expected data to contain key 'key' with value 'value'")
	}
}

func TestNewReportGenerator(t *testing.T) {
	// Test with nil server
	rg := NewReportGenerator(nil)
	if rg == nil {
		t.Fatal("expected ReportGenerator to be created even with nil server")
	}

	if rg.server != nil {
		t.Error("expected server to be nil")
	}
}

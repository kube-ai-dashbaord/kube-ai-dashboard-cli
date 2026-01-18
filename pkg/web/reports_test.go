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
		{"no nodes", 0, 0, 5, 10, 75.0},  // nodeScore=50 (default), podScore=25
		{"no pods", 2, 4, 0, 0, 75.0},    // nodeScore=25, podScore=50 (default)
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

func TestComprehensiveReport_Struct(t *testing.T) {
	report := &ComprehensiveReport{
		GeneratedBy: "test-user",
		HealthScore: 95.5,
		ClusterInfo: ClusterInfo{
			ServerVersion: "v1.28.0",
			Platform:      "kubernetes",
			TotalNodes:    3,
			TotalPods:     10,
		},
	}

	if report.GeneratedBy != "test-user" {
		t.Errorf("expected GeneratedBy 'test-user', got %s", report.GeneratedBy)
	}

	if report.HealthScore != 95.5 {
		t.Errorf("expected HealthScore 95.5, got %f", report.HealthScore)
	}

	if report.ClusterInfo.TotalNodes != 3 {
		t.Errorf("expected TotalNodes 3, got %d", report.ClusterInfo.TotalNodes)
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

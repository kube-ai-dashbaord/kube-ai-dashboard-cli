package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/kube-ai-dashbaord/kube-ai-dashboard-cli/pkg/db"
)

// Report represents a generated report
type Report struct {
	ID          string                 `json:"id"`
	Title       string                 `json:"title"`
	Type        string                 `json:"type"` // cluster-health, resource-usage, security-audit, ai-interactions
	GeneratedAt time.Time              `json:"generated_at"`
	GeneratedBy string                 `json:"generated_by"`
	Data        map[string]interface{} `json:"data"`
}

// ReportGenerator handles report generation
type ReportGenerator struct {
	server *Server
}

// NewReportGenerator creates a new report generator
func NewReportGenerator(server *Server) *ReportGenerator {
	return &ReportGenerator{server: server}
}

// GenerateClusterHealthReport generates a cluster health report
func (rg *ReportGenerator) GenerateClusterHealthReport(username string) (*Report, error) {
	ctx := context.Background()

	// Get nodes
	nodes, err := rg.server.k8sClient.ListNodes(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get nodes: %w", err)
	}

	// Get namespaces
	namespaces, err := rg.server.k8sClient.ListNamespaces(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get namespaces: %w", err)
	}

	// Count resources across namespaces
	totalPods := 0
	runningPods := 0
	pendingPods := 0
	failedPods := 0

	totalDeployments := 0
	healthyDeployments := 0

	for _, ns := range namespaces {
		pods, _ := rg.server.k8sClient.ListPods(ctx, ns.Name)
		totalPods += len(pods)
		for _, pod := range pods {
			switch pod.Status.Phase {
			case "Running":
				runningPods++
			case "Pending":
				pendingPods++
			case "Failed":
				failedPods++
			}
		}

		deps, _ := rg.server.k8sClient.ListDeployments(ctx, ns.Name)
		totalDeployments += len(deps)
		for _, dep := range deps {
			if dep.Status.ReadyReplicas == *dep.Spec.Replicas {
				healthyDeployments++
			}
		}
	}

	// Node health
	healthyNodes := 0
	for _, node := range nodes {
		for _, cond := range node.Status.Conditions {
			if cond.Type == "Ready" && cond.Status == "True" {
				healthyNodes++
				break
			}
		}
	}

	report := &Report{
		ID:          fmt.Sprintf("health-%d", time.Now().Unix()),
		Title:       "Cluster Health Report",
		Type:        "cluster-health",
		GeneratedAt: time.Now(),
		GeneratedBy: username,
		Data: map[string]interface{}{
			"summary": map[string]interface{}{
				"total_nodes":         len(nodes),
				"healthy_nodes":       healthyNodes,
				"total_namespaces":    len(namespaces),
				"total_pods":          totalPods,
				"running_pods":        runningPods,
				"pending_pods":        pendingPods,
				"failed_pods":         failedPods,
				"total_deployments":   totalDeployments,
				"healthy_deployments": healthyDeployments,
			},
			"health_score": calculateHealthScore(healthyNodes, len(nodes), runningPods, totalPods),
		},
	}

	return report, nil
}

// GenerateResourceUsageReport generates a resource usage report
func (rg *ReportGenerator) GenerateResourceUsageReport(username string) (*Report, error) {
	ctx := context.Background()

	// Get node metrics if available
	nodeMetrics, _ := rg.server.k8sClient.GetNodeMetrics(ctx)

	// Get pod metrics
	podMetrics, _ := rg.server.k8sClient.GetPodMetrics(ctx, "")

	// Namespace resource counts
	namespaces, _ := rg.server.k8sClient.ListNamespaces(ctx)
	nsResources := make([]map[string]interface{}, 0)

	for _, ns := range namespaces {
		pods, _ := rg.server.k8sClient.ListPods(ctx, ns.Name)
		deps, _ := rg.server.k8sClient.ListDeployments(ctx, ns.Name)
		svcs, _ := rg.server.k8sClient.ListServices(ctx, ns.Name)

		nsResources = append(nsResources, map[string]interface{}{
			"namespace":   ns.Name,
			"pods":        len(pods),
			"deployments": len(deps),
			"services":    len(svcs),
		})
	}

	report := &Report{
		ID:          fmt.Sprintf("usage-%d", time.Now().Unix()),
		Title:       "Resource Usage Report",
		Type:        "resource-usage",
		GeneratedAt: time.Now(),
		GeneratedBy: username,
		Data: map[string]interface{}{
			"node_metrics":          nodeMetrics,
			"pod_metrics":           podMetrics,
			"namespace_resources":   nsResources,
			"total_namespaces":      len(namespaces),
		},
	}

	return report, nil
}

// GenerateAuditReport generates an audit log report
func (rg *ReportGenerator) GenerateAuditReport(username string) (*Report, error) {
	logs, err := db.GetAuditLogs()
	if err != nil {
		return nil, fmt.Errorf("failed to get audit logs: %w", err)
	}

	// Aggregate by action type
	actionCounts := make(map[string]int)
	userCounts := make(map[string]int)
	resourceCounts := make(map[string]int)

	for _, log := range logs {
		if action, ok := log["action"].(string); ok {
			actionCounts[action]++
		}
		if user, ok := log["user"].(string); ok {
			userCounts[user]++
		}
		if resource, ok := log["resource"].(string); ok {
			resourceCounts[resource]++
		}
	}

	report := &Report{
		ID:          fmt.Sprintf("audit-%d", time.Now().Unix()),
		Title:       "Security Audit Report",
		Type:        "security-audit",
		GeneratedAt: time.Now(),
		GeneratedBy: username,
		Data: map[string]interface{}{
			"total_entries":   len(logs),
			"action_counts":   actionCounts,
			"user_counts":     userCounts,
			"resource_counts": resourceCounts,
			"recent_logs":     logs,
		},
	}

	return report, nil
}

// GenerateAIInteractionsReport generates AI interaction report
func (rg *ReportGenerator) GenerateAIInteractionsReport(username string) (*Report, error) {
	// Get AI-related audit logs
	logs, err := db.GetAuditLogs()
	if err != nil {
		return nil, fmt.Errorf("failed to get audit logs: %w", err)
	}

	aiLogs := make([]map[string]interface{}, 0)
	for _, log := range logs {
		if action, ok := log["action"].(string); ok && action == "ai_query" {
			aiLogs = append(aiLogs, log)
		}
	}

	report := &Report{
		ID:          fmt.Sprintf("ai-%d", time.Now().Unix()),
		Title:       "AI Interactions Report",
		Type:        "ai-interactions",
		GeneratedAt: time.Now(),
		GeneratedBy: username,
		Data: map[string]interface{}{
			"total_queries": len(aiLogs),
			"recent_logs":   aiLogs,
		},
	}

	return report, nil
}

func calculateHealthScore(healthyNodes, totalNodes, runningPods, totalPods int) float64 {
	if totalNodes == 0 || totalPods == 0 {
		return 100.0
	}

	nodeScore := float64(healthyNodes) / float64(totalNodes) * 50
	podScore := float64(runningPods) / float64(totalPods) * 50

	return nodeScore + podScore
}

// HandleReports handles report-related API requests
func (rg *ReportGenerator) HandleReports(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	username := r.Header.Get("X-Username")
	if username == "" {
		username = "anonymous"
	}

	switch r.Method {
	case http.MethodGet:
		reportType := r.URL.Query().Get("type")
		var report *Report
		var err error

		switch reportType {
		case "cluster-health":
			report, err = rg.GenerateClusterHealthReport(username)
		case "resource-usage":
			report, err = rg.GenerateResourceUsageReport(username)
		case "security-audit":
			report, err = rg.GenerateAuditReport(username)
		case "ai-interactions":
			report, err = rg.GenerateAIInteractionsReport(username)
		default:
			// Return available report types
			json.NewEncoder(w).Encode(map[string]interface{}{
				"available_types": []string{
					"cluster-health",
					"resource-usage",
					"security-audit",
					"ai-interactions",
				},
			})
			return
		}

		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		json.NewEncoder(w).Encode(report)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

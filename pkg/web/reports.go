package web

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/kube-ai-dashbaord/kube-ai-dashboard-cli/pkg/db"
	corev1 "k8s.io/api/core/v1"
)

// ComprehensiveReport contains all cluster information for export
type ComprehensiveReport struct {
	GeneratedAt      time.Time           `json:"generated_at"`
	GeneratedBy      string              `json:"generated_by"`
	ClusterInfo      ClusterInfo         `json:"cluster_info"`
	NodeSummary      NodeSummary         `json:"node_summary"`
	Nodes            []NodeInfo          `json:"nodes"`
	NamespaceSummary NamespaceSummary    `json:"namespace_summary"`
	Namespaces       []NamespaceInfo     `json:"namespaces"`
	Workloads        WorkloadSummary     `json:"workloads"`
	Pods             []PodInfo           `json:"pods"`
	Deployments      []DeploymentInfo    `json:"deployments"`
	Services         []ServiceInfo       `json:"services"`
	SecurityInfo     SecurityInfo        `json:"security_info"`
	FinOpsAnalysis   FinOpsAnalysis      `json:"finops_analysis"`
	Images           []ImageInfo         `json:"images"`
	Events           []EventInfo         `json:"events"`
	AIAnalysis       string              `json:"ai_analysis,omitempty"`
	HealthScore      float64             `json:"health_score"`
}

// FinOpsAnalysis contains cost optimization insights
type FinOpsAnalysis struct {
	TotalEstimatedMonthlyCost float64                `json:"total_estimated_monthly_cost"`
	CostByNamespace           []NamespaceCost        `json:"cost_by_namespace"`
	ResourceEfficiency        ResourceEfficiencyInfo `json:"resource_efficiency"`
	CostOptimizations         []CostOptimization     `json:"cost_optimizations"`
	UnderutilizedResources    []UnderutilizedResource `json:"underutilized_resources"`
	OverprovisionedWorkloads  []OverprovisionedWorkload `json:"overprovisioned_workloads"`
}

// NamespaceCost represents estimated cost per namespace
type NamespaceCost struct {
	Namespace      string  `json:"namespace"`
	PodCount       int     `json:"pod_count"`
	CPURequests    string  `json:"cpu_requests"`
	MemoryRequests string  `json:"memory_requests"`
	EstimatedCost  float64 `json:"estimated_cost"`
	CostPercentage float64 `json:"cost_percentage"`
}

// ResourceEfficiencyInfo contains resource utilization metrics
type ResourceEfficiencyInfo struct {
	TotalCPURequests       string  `json:"total_cpu_requests"`
	TotalCPULimits         string  `json:"total_cpu_limits"`
	TotalMemoryRequests    string  `json:"total_memory_requests"`
	TotalMemoryLimits      string  `json:"total_memory_limits"`
	CPURequestsVsCapacity  float64 `json:"cpu_requests_vs_capacity"`
	MemoryRequestsVsCapacity float64 `json:"memory_requests_vs_capacity"`
	PodsWithoutRequests    int     `json:"pods_without_requests"`
	PodsWithoutLimits      int     `json:"pods_without_limits"`
}

// CostOptimization represents a cost saving recommendation
type CostOptimization struct {
	Category        string  `json:"category"`
	Description     string  `json:"description"`
	Impact          string  `json:"impact"`
	EstimatedSaving float64 `json:"estimated_saving"`
	Priority        string  `json:"priority"` // high, medium, low
}

// UnderutilizedResource represents a resource with low utilization
type UnderutilizedResource struct {
	Name         string  `json:"name"`
	Namespace    string  `json:"namespace"`
	ResourceType string  `json:"resource_type"`
	CPUUsage     float64 `json:"cpu_usage_percent"`
	MemoryUsage  float64 `json:"memory_usage_percent"`
	Suggestion   string  `json:"suggestion"`
}

// OverprovisionedWorkload represents a workload with excessive resources
type OverprovisionedWorkload struct {
	Name           string `json:"name"`
	Namespace      string `json:"namespace"`
	WorkloadType   string `json:"workload_type"`
	CurrentReplicas int   `json:"current_replicas"`
	SuggestedReplicas int `json:"suggested_replicas"`
	Reason         string `json:"reason"`
}

type ClusterInfo struct {
	ServerVersion string `json:"server_version"`
	Platform      string `json:"platform"`
	TotalNodes    int    `json:"total_nodes"`
	TotalPods     int    `json:"total_pods"`
}

type NodeSummary struct {
	Total     int `json:"total"`
	Ready     int `json:"ready"`
	NotReady  int `json:"not_ready"`
}

type NodeInfo struct {
	Name              string   `json:"name"`
	Status            string   `json:"status"`
	Roles             []string `json:"roles"`
	KubeletVersion    string   `json:"kubelet_version"`
	OS                string   `json:"os"`
	Architecture      string   `json:"architecture"`
	CPUCapacity       string   `json:"cpu_capacity"`
	MemoryCapacity    string   `json:"memory_capacity"`
	PodCapacity       string   `json:"pod_capacity"`
	ContainerRuntime  string   `json:"container_runtime"`
	InternalIP        string   `json:"internal_ip"`
	CreationTime      string   `json:"creation_time"`
}

type NamespaceSummary struct {
	Total  int `json:"total"`
	Active int `json:"active"`
}

type NamespaceInfo struct {
	Name         string `json:"name"`
	Status       string `json:"status"`
	PodCount     int    `json:"pod_count"`
	DeployCount  int    `json:"deploy_count"`
	ServiceCount int    `json:"service_count"`
	CreationTime string `json:"creation_time"`
}

type WorkloadSummary struct {
	TotalPods         int `json:"total_pods"`
	RunningPods       int `json:"running_pods"`
	PendingPods       int `json:"pending_pods"`
	FailedPods        int `json:"failed_pods"`
	TotalDeployments  int `json:"total_deployments"`
	HealthyDeploys    int `json:"healthy_deployments"`
	TotalServices     int `json:"total_services"`
	TotalConfigMaps   int `json:"total_configmaps"`
	TotalSecrets      int `json:"total_secrets"`
}

type PodInfo struct {
	Name       string   `json:"name"`
	Namespace  string   `json:"namespace"`
	Status     string   `json:"status"`
	Ready      string   `json:"ready"`
	Restarts   int      `json:"restarts"`
	Node       string   `json:"node"`
	IP         string   `json:"ip"`
	Images     []string `json:"images"`
	Age        string   `json:"age"`
}

type DeploymentInfo struct {
	Name       string `json:"name"`
	Namespace  string `json:"namespace"`
	Ready      string `json:"ready"`
	UpToDate   int    `json:"up_to_date"`
	Available  int    `json:"available"`
	Strategy   string `json:"strategy"`
	Age        string `json:"age"`
}

type ServiceInfo struct {
	Name       string `json:"name"`
	Namespace  string `json:"namespace"`
	Type       string `json:"type"`
	ClusterIP  string `json:"cluster_ip"`
	ExternalIP string `json:"external_ip"`
	Ports      string `json:"ports"`
	Age        string `json:"age"`
}

type SecurityInfo struct {
	ServiceAccounts      int             `json:"service_accounts"`
	Roles                int             `json:"roles"`
	RoleBindings         int             `json:"role_bindings"`
	ClusterRoles         int             `json:"cluster_roles"`
	ClusterRoleBindings  int             `json:"cluster_role_bindings"`
	Secrets              int             `json:"secrets"`
	PrivilegedPods       int             `json:"privileged_pods"`
	HostNetworkPods      int             `json:"host_network_pods"`
	RootContainers       int             `json:"root_containers"`
}

type ImageInfo struct {
	Image      string `json:"image"`
	Repository string `json:"repository"`
	Tag        string `json:"tag"`
	PodCount   int    `json:"pod_count"`
}

type EventInfo struct {
	Type      string `json:"type"`
	Reason    string `json:"reason"`
	Object    string `json:"object"`
	Message   string `json:"message"`
	Count     int    `json:"count"`
	FirstSeen string `json:"first_seen"`
	LastSeen  string `json:"last_seen"`
}

// ReportGenerator handles report generation
type ReportGenerator struct {
	server *Server
}

// NewReportGenerator creates a new report generator
func NewReportGenerator(server *Server) *ReportGenerator {
	return &ReportGenerator{server: server}
}

// GenerateComprehensiveReport gathers all cluster data
func (rg *ReportGenerator) GenerateComprehensiveReport(ctx context.Context, username string) (*ComprehensiveReport, error) {
	report := &ComprehensiveReport{
		GeneratedAt: time.Now(),
		GeneratedBy: username,
	}

	// Get nodes
	nodes, err := rg.server.k8sClient.ListNodes(ctx)
	if err == nil {
		report.NodeSummary.Total = len(nodes)
		for _, node := range nodes {
			info := NodeInfo{
				Name:           node.Name,
				KubeletVersion: node.Status.NodeInfo.KubeletVersion,
				OS:             node.Status.NodeInfo.OSImage,
				Architecture:   node.Status.NodeInfo.Architecture,
				ContainerRuntime: node.Status.NodeInfo.ContainerRuntimeVersion,
				CPUCapacity:    node.Status.Capacity.Cpu().String(),
				MemoryCapacity: node.Status.Capacity.Memory().String(),
				PodCapacity:    node.Status.Capacity.Pods().String(),
				CreationTime:   node.CreationTimestamp.Format(time.RFC3339),
			}

			// Get roles
			for label := range node.Labels {
				if strings.HasPrefix(label, "node-role.kubernetes.io/") {
					role := strings.TrimPrefix(label, "node-role.kubernetes.io/")
					info.Roles = append(info.Roles, role)
				}
			}
			if len(info.Roles) == 0 {
				info.Roles = []string{"worker"}
			}

			// Get status
			for _, cond := range node.Status.Conditions {
				if cond.Type == corev1.NodeReady {
					if cond.Status == corev1.ConditionTrue {
						info.Status = "Ready"
						report.NodeSummary.Ready++
					} else {
						info.Status = "NotReady"
						report.NodeSummary.NotReady++
					}
					break
				}
			}

			// Get IP
			for _, addr := range node.Status.Addresses {
				if addr.Type == corev1.NodeInternalIP {
					info.InternalIP = addr.Address
					break
				}
			}

			report.Nodes = append(report.Nodes, info)
		}
	}

	// Get namespaces
	namespaces, err := rg.server.k8sClient.ListNamespaces(ctx)
	if err == nil {
		report.NamespaceSummary.Total = len(namespaces)
		for _, ns := range namespaces {
			info := NamespaceInfo{
				Name:         ns.Name,
				Status:       string(ns.Status.Phase),
				CreationTime: ns.CreationTimestamp.Format(time.RFC3339),
			}

			if ns.Status.Phase == corev1.NamespaceActive {
				report.NamespaceSummary.Active++
			}

			// Count resources in namespace
			pods, _ := rg.server.k8sClient.ListPods(ctx, ns.Name)
			info.PodCount = len(pods)

			deps, _ := rg.server.k8sClient.ListDeployments(ctx, ns.Name)
			info.DeployCount = len(deps)

			svcs, _ := rg.server.k8sClient.ListServices(ctx, ns.Name)
			info.ServiceCount = len(svcs)

			report.Namespaces = append(report.Namespaces, info)
		}
	}

	// Gather workload data
	imageCount := make(map[string]int)

	for _, ns := range namespaces {
		// Pods
		pods, _ := rg.server.k8sClient.ListPods(ctx, ns.Name)
		for _, pod := range pods {
			report.Workloads.TotalPods++

			switch pod.Status.Phase {
			case corev1.PodRunning:
				report.Workloads.RunningPods++
			case corev1.PodPending:
				report.Workloads.PendingPods++
			case corev1.PodFailed:
				report.Workloads.FailedPods++
			}

			// Count restarts
			restarts := 0
			for _, cs := range pod.Status.ContainerStatuses {
				restarts += int(cs.RestartCount)
			}

			// Get images
			var images []string
			for _, c := range pod.Spec.Containers {
				images = append(images, c.Image)
				imageCount[c.Image]++
			}

			// Security checks
			for _, c := range pod.Spec.Containers {
				if c.SecurityContext != nil {
					if c.SecurityContext.Privileged != nil && *c.SecurityContext.Privileged {
						report.SecurityInfo.PrivilegedPods++
					}
					if c.SecurityContext.RunAsUser != nil && *c.SecurityContext.RunAsUser == 0 {
						report.SecurityInfo.RootContainers++
					}
				}
			}
			if pod.Spec.HostNetwork {
				report.SecurityInfo.HostNetworkPods++
			}

			ready := 0
			total := len(pod.Status.ContainerStatuses)
			for _, cs := range pod.Status.ContainerStatuses {
				if cs.Ready {
					ready++
				}
			}

			podInfo := PodInfo{
				Name:      pod.Name,
				Namespace: pod.Namespace,
				Status:    string(pod.Status.Phase),
				Ready:     fmt.Sprintf("%d/%d", ready, total),
				Restarts:  restarts,
				Node:      pod.Spec.NodeName,
				IP:        pod.Status.PodIP,
				Images:    images,
				Age:       time.Since(pod.CreationTimestamp.Time).Round(time.Second).String(),
			}
			report.Pods = append(report.Pods, podInfo)
		}

		// Deployments
		deps, _ := rg.server.k8sClient.ListDeployments(ctx, ns.Name)
		for _, dep := range deps {
			report.Workloads.TotalDeployments++

			replicas := int32(1)
			if dep.Spec.Replicas != nil {
				replicas = *dep.Spec.Replicas
			}

			if dep.Status.ReadyReplicas == replicas {
				report.Workloads.HealthyDeploys++
			}

			strategy := "RollingUpdate"
			if dep.Spec.Strategy.Type != "" {
				strategy = string(dep.Spec.Strategy.Type)
			}

			depInfo := DeploymentInfo{
				Name:      dep.Name,
				Namespace: dep.Namespace,
				Ready:     fmt.Sprintf("%d/%d", dep.Status.ReadyReplicas, replicas),
				UpToDate:  int(dep.Status.UpdatedReplicas),
				Available: int(dep.Status.AvailableReplicas),
				Strategy:  strategy,
				Age:       time.Since(dep.CreationTimestamp.Time).Round(time.Second).String(),
			}
			report.Deployments = append(report.Deployments, depInfo)
		}

		// Services
		svcs, _ := rg.server.k8sClient.ListServices(ctx, ns.Name)
		for _, svc := range svcs {
			report.Workloads.TotalServices++

			ports := make([]string, len(svc.Spec.Ports))
			for i, p := range svc.Spec.Ports {
				ports[i] = fmt.Sprintf("%d/%s", p.Port, p.Protocol)
			}

			externalIP := "<none>"
			if len(svc.Status.LoadBalancer.Ingress) > 0 {
				ips := []string{}
				for _, ing := range svc.Status.LoadBalancer.Ingress {
					if ing.IP != "" {
						ips = append(ips, ing.IP)
					} else if ing.Hostname != "" {
						ips = append(ips, ing.Hostname)
					}
				}
				if len(ips) > 0 {
					externalIP = strings.Join(ips, ", ")
				}
			}

			svcInfo := ServiceInfo{
				Name:       svc.Name,
				Namespace:  svc.Namespace,
				Type:       string(svc.Spec.Type),
				ClusterIP:  svc.Spec.ClusterIP,
				ExternalIP: externalIP,
				Ports:      strings.Join(ports, ", "),
				Age:        time.Since(svc.CreationTimestamp.Time).Round(time.Second).String(),
			}
			report.Services = append(report.Services, svcInfo)
		}

		// ConfigMaps & Secrets count
		configmaps, _ := rg.server.k8sClient.ListConfigMaps(ctx, ns.Name)
		report.Workloads.TotalConfigMaps += len(configmaps)

		secrets, _ := rg.server.k8sClient.ListSecrets(ctx, ns.Name)
		report.SecurityInfo.Secrets += len(secrets)
	}

	// Build image list
	for image, count := range imageCount {
		parts := strings.Split(image, ":")
		repo := parts[0]
		tag := "latest"
		if len(parts) > 1 {
			tag = parts[1]
		}

		report.Images = append(report.Images, ImageInfo{
			Image:      image,
			Repository: repo,
			Tag:        tag,
			PodCount:   count,
		})
	}

	// Sort images by pod count
	sort.Slice(report.Images, func(i, j int) bool {
		return report.Images[i].PodCount > report.Images[j].PodCount
	})

	// Get events (warnings only, last 50)
	events, _ := rg.server.k8sClient.ListEvents(ctx, "")
	warningEvents := []EventInfo{}
	for _, event := range events {
		if event.Type == "Warning" {
			warningEvents = append(warningEvents, EventInfo{
				Type:      event.Type,
				Reason:    event.Reason,
				Object:    fmt.Sprintf("%s/%s", event.InvolvedObject.Kind, event.InvolvedObject.Name),
				Message:   event.Message,
				Count:     int(event.Count),
				FirstSeen: event.FirstTimestamp.Format(time.RFC3339),
				LastSeen:  event.LastTimestamp.Format(time.RFC3339),
			})
		}
	}
	// Keep only last 50 warning events
	if len(warningEvents) > 50 {
		warningEvents = warningEvents[:50]
	}
	report.Events = warningEvents

	// Calculate health score
	report.HealthScore = calculateHealthScore(
		report.NodeSummary.Ready, report.NodeSummary.Total,
		report.Workloads.RunningPods, report.Workloads.TotalPods,
	)

	// Set cluster info
	report.ClusterInfo = ClusterInfo{
		TotalNodes: report.NodeSummary.Total,
		TotalPods:  report.Workloads.TotalPods,
	}

	// Generate FinOps analysis
	report.FinOpsAnalysis = rg.generateFinOpsAnalysis(ctx, namespaces, report)

	return report, nil
}

// generateFinOpsAnalysis analyzes cost and resource efficiency
func (rg *ReportGenerator) generateFinOpsAnalysis(ctx context.Context, namespaces []corev1.Namespace, report *ComprehensiveReport) FinOpsAnalysis {
	analysis := FinOpsAnalysis{
		CostByNamespace:          []NamespaceCost{},
		CostOptimizations:        []CostOptimization{},
		UnderutilizedResources:   []UnderutilizedResource{},
		OverprovisionedWorkloads: []OverprovisionedWorkload{},
	}

	// Reference costs (approximate AWS EKS pricing)
	// vCPU: ~$0.04/hour, Memory: ~$0.004/GB/hour
	const cpuHourlyCost = 0.04    // per vCPU
	const memoryHourlyCost = 0.004 // per GB

	var totalCPURequests, totalCPULimits int64   // millicores
	var totalMemRequests, totalMemLimits int64   // bytes
	var totalNodeCPUCapacity int64               // millicores
	var totalNodeMemCapacity int64               // bytes
	var podsWithoutRequests, podsWithoutLimits int

	// Calculate node capacity
	nodes, _ := rg.server.k8sClient.ListNodes(ctx)
	for _, node := range nodes {
		cpu := node.Status.Capacity.Cpu()
		mem := node.Status.Capacity.Memory()
		totalNodeCPUCapacity += cpu.MilliValue()
		totalNodeMemCapacity += mem.Value()
	}

	// Analyze each namespace
	nsCosts := make(map[string]*NamespaceCost)

	for _, ns := range namespaces {
		pods, _ := rg.server.k8sClient.ListPods(ctx, ns.Name)

		nsCost := &NamespaceCost{
			Namespace: ns.Name,
			PodCount:  len(pods),
		}

		var nsCPU, nsMem int64

		for _, pod := range pods {
			podHasRequests := false
			podHasLimits := false

			for _, container := range pod.Spec.Containers {
				// Requests
				if cpuReq := container.Resources.Requests.Cpu(); cpuReq != nil {
					nsCPU += cpuReq.MilliValue()
					totalCPURequests += cpuReq.MilliValue()
					podHasRequests = true
				}
				if memReq := container.Resources.Requests.Memory(); memReq != nil {
					nsMem += memReq.Value()
					totalMemRequests += memReq.Value()
					podHasRequests = true
				}

				// Limits
				if cpuLim := container.Resources.Limits.Cpu(); cpuLim != nil {
					totalCPULimits += cpuLim.MilliValue()
					podHasLimits = true
				}
				if memLim := container.Resources.Limits.Memory(); memLim != nil {
					totalMemLimits += memLim.Value()
					podHasLimits = true
				}
			}

			if !podHasRequests {
				podsWithoutRequests++
			}
			if !podHasLimits {
				podsWithoutLimits++
			}
		}

		// Calculate namespace cost (monthly estimate)
		cpuCores := float64(nsCPU) / 1000.0
		memGB := float64(nsMem) / (1024 * 1024 * 1024)
		monthlyHours := 730.0 // average hours per month

		nsCost.CPURequests = fmt.Sprintf("%.2f cores", cpuCores)
		nsCost.MemoryRequests = fmt.Sprintf("%.2f GB", memGB)
		nsCost.EstimatedCost = (cpuCores*cpuHourlyCost + memGB*memoryHourlyCost) * monthlyHours

		nsCosts[ns.Name] = nsCost
	}

	// Calculate total and percentages
	var totalCost float64
	for _, nsCost := range nsCosts {
		totalCost += nsCost.EstimatedCost
	}

	for _, nsCost := range nsCosts {
		if totalCost > 0 {
			nsCost.CostPercentage = (nsCost.EstimatedCost / totalCost) * 100
		}
		analysis.CostByNamespace = append(analysis.CostByNamespace, *nsCost)
	}

	// Sort by cost descending
	sort.Slice(analysis.CostByNamespace, func(i, j int) bool {
		return analysis.CostByNamespace[i].EstimatedCost > analysis.CostByNamespace[j].EstimatedCost
	})

	analysis.TotalEstimatedMonthlyCost = totalCost

	// Resource efficiency
	analysis.ResourceEfficiency = ResourceEfficiencyInfo{
		TotalCPURequests:    fmt.Sprintf("%.2f cores", float64(totalCPURequests)/1000.0),
		TotalCPULimits:      fmt.Sprintf("%.2f cores", float64(totalCPULimits)/1000.0),
		TotalMemoryRequests: fmt.Sprintf("%.2f GB", float64(totalMemRequests)/(1024*1024*1024)),
		TotalMemoryLimits:   fmt.Sprintf("%.2f GB", float64(totalMemLimits)/(1024*1024*1024)),
		PodsWithoutRequests: podsWithoutRequests,
		PodsWithoutLimits:   podsWithoutLimits,
	}

	if totalNodeCPUCapacity > 0 {
		analysis.ResourceEfficiency.CPURequestsVsCapacity = float64(totalCPURequests) / float64(totalNodeCPUCapacity) * 100
	}
	if totalNodeMemCapacity > 0 {
		analysis.ResourceEfficiency.MemoryRequestsVsCapacity = float64(totalMemRequests) / float64(totalNodeMemCapacity) * 100
	}

	// Generate cost optimization recommendations
	analysis.CostOptimizations = rg.generateCostOptimizations(report, &analysis)

	// Analyze underutilized deployments
	for _, dep := range report.Deployments {
		// Check for deployments with many unavailable replicas
		parts := strings.Split(dep.Ready, "/")
		if len(parts) == 2 {
			ready := 0
			total := 0
			fmt.Sscanf(parts[0], "%d", &ready)
			fmt.Sscanf(parts[1], "%d", &total)

			if total > 1 && ready < total {
				analysis.OverprovisionedWorkloads = append(analysis.OverprovisionedWorkloads, OverprovisionedWorkload{
					Name:              dep.Name,
					Namespace:         dep.Namespace,
					WorkloadType:      "Deployment",
					CurrentReplicas:   total,
					SuggestedReplicas: ready,
					Reason:           fmt.Sprintf("Only %d/%d replicas are ready - consider reducing replicas or investigating issues", ready, total),
				})
			}
		}
	}

	return analysis
}

// generateCostOptimizations creates cost saving recommendations
func (rg *ReportGenerator) generateCostOptimizations(report *ComprehensiveReport, analysis *FinOpsAnalysis) []CostOptimization {
	var optimizations []CostOptimization

	// Check for pods without resource requests
	if analysis.ResourceEfficiency.PodsWithoutRequests > 0 {
		optimizations = append(optimizations, CostOptimization{
			Category:        "Resource Management",
			Description:     fmt.Sprintf("%d pods are running without resource requests defined", analysis.ResourceEfficiency.PodsWithoutRequests),
			Impact:          "Without resource requests, pods may be scheduled inefficiently leading to resource contention or waste",
			EstimatedSaving: float64(analysis.ResourceEfficiency.PodsWithoutRequests) * 5.0, // $5 per pod monthly estimate
			Priority:        "high",
		})
	}

	// Check for pods without resource limits
	if analysis.ResourceEfficiency.PodsWithoutLimits > 0 {
		optimizations = append(optimizations, CostOptimization{
			Category:        "Resource Management",
			Description:     fmt.Sprintf("%d pods are running without resource limits defined", analysis.ResourceEfficiency.PodsWithoutLimits),
			Impact:          "Without limits, pods can consume unbounded resources affecting cluster stability",
			EstimatedSaving: float64(analysis.ResourceEfficiency.PodsWithoutLimits) * 3.0,
			Priority:        "medium",
		})
	}

	// Check for low cluster utilization
	if analysis.ResourceEfficiency.CPURequestsVsCapacity < 30 {
		optimizations = append(optimizations, CostOptimization{
			Category:        "Cluster Sizing",
			Description:     fmt.Sprintf("CPU utilization is only %.1f%% of cluster capacity", analysis.ResourceEfficiency.CPURequestsVsCapacity),
			Impact:          "Consider reducing node count or using smaller instance types",
			EstimatedSaving: analysis.TotalEstimatedMonthlyCost * 0.3, // 30% potential savings
			Priority:        "high",
		})
	}

	if analysis.ResourceEfficiency.MemoryRequestsVsCapacity < 30 {
		optimizations = append(optimizations, CostOptimization{
			Category:        "Cluster Sizing",
			Description:     fmt.Sprintf("Memory utilization is only %.1f%% of cluster capacity", analysis.ResourceEfficiency.MemoryRequestsVsCapacity),
			Impact:          "Consider using memory-optimized instances or reducing node count",
			EstimatedSaving: analysis.TotalEstimatedMonthlyCost * 0.2,
			Priority:        "medium",
		})
	}

	// Check for failed pods wasting resources
	if report.Workloads.FailedPods > 0 {
		optimizations = append(optimizations, CostOptimization{
			Category:        "Workload Health",
			Description:     fmt.Sprintf("%d pods are in failed state", report.Workloads.FailedPods),
			Impact:          "Failed pods may still consume resources and indicate configuration issues",
			EstimatedSaving: float64(report.Workloads.FailedPods) * 10.0,
			Priority:        "high",
		})
	}

	// Check for pending pods
	if report.Workloads.PendingPods > 0 {
		optimizations = append(optimizations, CostOptimization{
			Category:        "Scheduling",
			Description:     fmt.Sprintf("%d pods are pending and cannot be scheduled", report.Workloads.PendingPods),
			Impact:          "Pending pods indicate resource constraints or scheduling issues",
			EstimatedSaving: 0,
			Priority:        "high",
		})
	}

	// Check for many restarts indicating instability
	totalRestarts := 0
	for _, pod := range report.Pods {
		totalRestarts += pod.Restarts
	}
	if totalRestarts > 10 {
		optimizations = append(optimizations, CostOptimization{
			Category:        "Stability",
			Description:     fmt.Sprintf("Total of %d container restarts detected across pods", totalRestarts),
			Impact:          "Frequent restarts waste compute resources and may indicate memory/OOM issues",
			EstimatedSaving: float64(totalRestarts) * 0.5,
			Priority:        "medium",
		})
	}

	// LoadBalancer service costs
	lbCount := 0
	for _, svc := range report.Services {
		if svc.Type == "LoadBalancer" {
			lbCount++
		}
	}
	if lbCount > 3 {
		optimizations = append(optimizations, CostOptimization{
			Category:        "Networking",
			Description:     fmt.Sprintf("%d LoadBalancer services detected", lbCount),
			Impact:          "Each LoadBalancer incurs cloud provider costs (~$18/month each). Consider using Ingress controller",
			EstimatedSaving: float64(lbCount-1) * 18.0, // Keep 1, consolidate others
			Priority:        "medium",
		})
	}

	return optimizations
}

// GenerateAIAnalysis uses LLM to analyze the cluster state with FinOps focus
func (rg *ReportGenerator) GenerateAIAnalysis(ctx context.Context, report *ComprehensiveReport) (string, error) {
	if rg.server.aiClient == nil || !rg.server.aiClient.IsReady() {
		return "", fmt.Errorf("AI client not available")
	}

	// Build cost optimization summary
	var costOptSummary strings.Builder
	for i, opt := range report.FinOpsAnalysis.CostOptimizations {
		if i >= 5 {
			break
		}
		costOptSummary.WriteString(fmt.Sprintf("- [%s] %s (Est. saving: $%.2f/mo)\n", opt.Priority, opt.Description, opt.EstimatedSaving))
	}

	// Build summary for AI with FinOps focus
	prompt := fmt.Sprintf(`You are a Kubernetes and FinOps expert. Analyze this cluster state and provide a comprehensive professional report (max 600 words) with special focus on cost optimization.

Cluster Summary:
- Nodes: %d total, %d ready, %d not ready
- Pods: %d total, %d running, %d pending, %d failed
- Deployments: %d total, %d healthy
- Services: %d
- Health Score: %.1f%%

FinOps / Cost Analysis:
- Estimated Monthly Cost: $%.2f
- CPU Utilization vs Capacity: %.1f%%
- Memory Utilization vs Capacity: %.1f%%
- Pods without Resource Requests: %d
- Pods without Resource Limits: %d

Top Cost Optimization Opportunities:
%s

Security Concerns:
- Privileged Pods: %d
- Host Network Pods: %d
- Root Containers: %d

Warning Events: %d

Top Images Used:
%s

Please provide:
1. Overall cluster health assessment
2. **FinOps Cost Analysis** (prioritize this section):
   - Current spending efficiency
   - Top cost drivers
   - Immediate cost reduction opportunities
   - Long-term optimization recommendations
3. Resource optimization recommendations
4. Security observations
5. Action items with priority levels

Be concise, actionable, and focus on ROI for each recommendation.`,
		report.NodeSummary.Total, report.NodeSummary.Ready, report.NodeSummary.NotReady,
		report.Workloads.TotalPods, report.Workloads.RunningPods, report.Workloads.PendingPods, report.Workloads.FailedPods,
		report.Workloads.TotalDeployments, report.Workloads.HealthyDeploys,
		report.Workloads.TotalServices,
		report.HealthScore,
		report.FinOpsAnalysis.TotalEstimatedMonthlyCost,
		report.FinOpsAnalysis.ResourceEfficiency.CPURequestsVsCapacity,
		report.FinOpsAnalysis.ResourceEfficiency.MemoryRequestsVsCapacity,
		report.FinOpsAnalysis.ResourceEfficiency.PodsWithoutRequests,
		report.FinOpsAnalysis.ResourceEfficiency.PodsWithoutLimits,
		costOptSummary.String(),
		report.SecurityInfo.PrivilegedPods, report.SecurityInfo.HostNetworkPods, report.SecurityInfo.RootContainers,
		len(report.Events),
		formatTopImages(report.Images, 5),
	)

	analysis, err := rg.server.aiClient.AskNonStreaming(ctx, prompt)
	if err != nil {
		return "", err
	}

	return analysis, nil
}

func formatTopImages(images []ImageInfo, limit int) string {
	var sb strings.Builder
	for i, img := range images {
		if i >= limit {
			break
		}
		sb.WriteString(fmt.Sprintf("- %s (used by %d pods)\n", img.Image, img.PodCount))
	}
	return sb.String()
}

func calculateHealthScore(healthyNodes, totalNodes, runningPods, totalPods int) float64 {
	if totalNodes == 0 && totalPods == 0 {
		return 100.0
	}

	nodeScore := 50.0
	if totalNodes > 0 {
		nodeScore = float64(healthyNodes) / float64(totalNodes) * 50
	}

	podScore := 50.0
	if totalPods > 0 {
		podScore = float64(runningPods) / float64(totalPods) * 50
	}

	return nodeScore + podScore
}

// ExportToCSV generates CSV format report
func (rg *ReportGenerator) ExportToCSV(report *ComprehensiveReport) ([]byte, error) {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	// Write header section
	writer.Write([]string{"K13s Cluster Report"})
	writer.Write([]string{"Generated At:", report.GeneratedAt.Format(time.RFC3339)})
	writer.Write([]string{"Generated By:", report.GeneratedBy})
	writer.Write([]string{"Health Score:", fmt.Sprintf("%.1f%%", report.HealthScore)})
	writer.Write([]string{""})

	// Cluster Summary
	writer.Write([]string{"=== CLUSTER SUMMARY ==="})
	writer.Write([]string{"Metric", "Value"})
	writer.Write([]string{"Total Nodes", fmt.Sprintf("%d", report.NodeSummary.Total)})
	writer.Write([]string{"Ready Nodes", fmt.Sprintf("%d", report.NodeSummary.Ready)})
	writer.Write([]string{"Total Pods", fmt.Sprintf("%d", report.Workloads.TotalPods)})
	writer.Write([]string{"Running Pods", fmt.Sprintf("%d", report.Workloads.RunningPods)})
	writer.Write([]string{"Pending Pods", fmt.Sprintf("%d", report.Workloads.PendingPods)})
	writer.Write([]string{"Failed Pods", fmt.Sprintf("%d", report.Workloads.FailedPods)})
	writer.Write([]string{"Total Deployments", fmt.Sprintf("%d", report.Workloads.TotalDeployments)})
	writer.Write([]string{"Healthy Deployments", fmt.Sprintf("%d", report.Workloads.HealthyDeploys)})
	writer.Write([]string{"Total Services", fmt.Sprintf("%d", report.Workloads.TotalServices)})
	writer.Write([]string{""})

	// Nodes
	writer.Write([]string{"=== NODES ==="})
	writer.Write([]string{"Name", "Status", "Roles", "Version", "CPU", "Memory", "IP"})
	for _, node := range report.Nodes {
		writer.Write([]string{
			node.Name,
			node.Status,
			strings.Join(node.Roles, ","),
			node.KubeletVersion,
			node.CPUCapacity,
			node.MemoryCapacity,
			node.InternalIP,
		})
	}
	writer.Write([]string{""})

	// Namespaces
	writer.Write([]string{"=== NAMESPACES ==="})
	writer.Write([]string{"Name", "Status", "Pods", "Deployments", "Services"})
	for _, ns := range report.Namespaces {
		writer.Write([]string{
			ns.Name,
			ns.Status,
			fmt.Sprintf("%d", ns.PodCount),
			fmt.Sprintf("%d", ns.DeployCount),
			fmt.Sprintf("%d", ns.ServiceCount),
		})
	}
	writer.Write([]string{""})

	// Pods
	writer.Write([]string{"=== PODS ==="})
	writer.Write([]string{"Name", "Namespace", "Status", "Ready", "Restarts", "Node", "IP", "Age"})
	for _, pod := range report.Pods {
		writer.Write([]string{
			pod.Name,
			pod.Namespace,
			pod.Status,
			pod.Ready,
			fmt.Sprintf("%d", pod.Restarts),
			pod.Node,
			pod.IP,
			pod.Age,
		})
	}
	writer.Write([]string{""})

	// Deployments
	writer.Write([]string{"=== DEPLOYMENTS ==="})
	writer.Write([]string{"Name", "Namespace", "Ready", "Up-to-date", "Available", "Strategy", "Age"})
	for _, dep := range report.Deployments {
		writer.Write([]string{
			dep.Name,
			dep.Namespace,
			dep.Ready,
			fmt.Sprintf("%d", dep.UpToDate),
			fmt.Sprintf("%d", dep.Available),
			dep.Strategy,
			dep.Age,
		})
	}
	writer.Write([]string{""})

	// Services
	writer.Write([]string{"=== SERVICES ==="})
	writer.Write([]string{"Name", "Namespace", "Type", "ClusterIP", "ExternalIP", "Ports", "Age"})
	for _, svc := range report.Services {
		writer.Write([]string{
			svc.Name,
			svc.Namespace,
			svc.Type,
			svc.ClusterIP,
			svc.ExternalIP,
			svc.Ports,
			svc.Age,
		})
	}
	writer.Write([]string{""})

	// Images
	writer.Write([]string{"=== CONTAINER IMAGES ==="})
	writer.Write([]string{"Image", "Repository", "Tag", "Pod Count"})
	for _, img := range report.Images {
		writer.Write([]string{
			img.Image,
			img.Repository,
			img.Tag,
			fmt.Sprintf("%d", img.PodCount),
		})
	}
	writer.Write([]string{""})

	// Security
	writer.Write([]string{"=== SECURITY SUMMARY ==="})
	writer.Write([]string{"Metric", "Value"})
	writer.Write([]string{"Secrets Count", fmt.Sprintf("%d", report.SecurityInfo.Secrets)})
	writer.Write([]string{"Privileged Pods", fmt.Sprintf("%d", report.SecurityInfo.PrivilegedPods)})
	writer.Write([]string{"Host Network Pods", fmt.Sprintf("%d", report.SecurityInfo.HostNetworkPods)})
	writer.Write([]string{"Root Containers", fmt.Sprintf("%d", report.SecurityInfo.RootContainers)})
	writer.Write([]string{""})

	// FinOps Analysis
	writer.Write([]string{"=== FINOPS COST ANALYSIS ==="})
	writer.Write([]string{"Metric", "Value"})
	writer.Write([]string{"Estimated Monthly Cost", fmt.Sprintf("$%.2f", report.FinOpsAnalysis.TotalEstimatedMonthlyCost)})
	writer.Write([]string{"Total CPU Requests", report.FinOpsAnalysis.ResourceEfficiency.TotalCPURequests})
	writer.Write([]string{"Total CPU Limits", report.FinOpsAnalysis.ResourceEfficiency.TotalCPULimits})
	writer.Write([]string{"Total Memory Requests", report.FinOpsAnalysis.ResourceEfficiency.TotalMemoryRequests})
	writer.Write([]string{"Total Memory Limits", report.FinOpsAnalysis.ResourceEfficiency.TotalMemoryLimits})
	writer.Write([]string{"CPU Utilization vs Capacity", fmt.Sprintf("%.1f%%", report.FinOpsAnalysis.ResourceEfficiency.CPURequestsVsCapacity)})
	writer.Write([]string{"Memory Utilization vs Capacity", fmt.Sprintf("%.1f%%", report.FinOpsAnalysis.ResourceEfficiency.MemoryRequestsVsCapacity)})
	writer.Write([]string{"Pods Without Requests", fmt.Sprintf("%d", report.FinOpsAnalysis.ResourceEfficiency.PodsWithoutRequests)})
	writer.Write([]string{"Pods Without Limits", fmt.Sprintf("%d", report.FinOpsAnalysis.ResourceEfficiency.PodsWithoutLimits)})
	writer.Write([]string{""})

	// Cost by Namespace
	if len(report.FinOpsAnalysis.CostByNamespace) > 0 {
		writer.Write([]string{"=== COST BY NAMESPACE ==="})
		writer.Write([]string{"Namespace", "Pod Count", "CPU Requests", "Memory Requests", "Est. Cost/Month", "% of Total"})
		for _, ns := range report.FinOpsAnalysis.CostByNamespace {
			writer.Write([]string{
				ns.Namespace,
				fmt.Sprintf("%d", ns.PodCount),
				ns.CPURequests,
				ns.MemoryRequests,
				fmt.Sprintf("$%.2f", ns.EstimatedCost),
				fmt.Sprintf("%.1f%%", ns.CostPercentage),
			})
		}
		writer.Write([]string{""})
	}

	// Cost Optimization Recommendations
	if len(report.FinOpsAnalysis.CostOptimizations) > 0 {
		writer.Write([]string{"=== COST OPTIMIZATION RECOMMENDATIONS ==="})
		writer.Write([]string{"Priority", "Category", "Description", "Impact", "Est. Saving/Month"})
		for _, opt := range report.FinOpsAnalysis.CostOptimizations {
			writer.Write([]string{
				opt.Priority,
				opt.Category,
				opt.Description,
				opt.Impact,
				fmt.Sprintf("$%.2f", opt.EstimatedSaving),
			})
		}
		writer.Write([]string{""})
	}

	// Warning Events
	if len(report.Events) > 0 {
		writer.Write([]string{"=== WARNING EVENTS ==="})
		writer.Write([]string{"Type", "Reason", "Object", "Message", "Count", "Last Seen"})
		for _, event := range report.Events {
			msg := event.Message
			if len(msg) > 100 {
				msg = msg[:100] + "..."
			}
			writer.Write([]string{
				event.Type,
				event.Reason,
				event.Object,
				msg,
				fmt.Sprintf("%d", event.Count),
				event.LastSeen,
			})
		}
		writer.Write([]string{""})
	}

	// AI Analysis
	if report.AIAnalysis != "" {
		writer.Write([]string{"=== AI ANALYSIS ==="})
		// Split analysis into lines for CSV
		lines := strings.Split(report.AIAnalysis, "\n")
		for _, line := range lines {
			writer.Write([]string{line})
		}
	}

	writer.Flush()
	return buf.Bytes(), writer.Error()
}

// ExportToHTML generates HTML format for PDF conversion
func (rg *ReportGenerator) ExportToHTML(report *ComprehensiveReport) string {
	var sb strings.Builder

	sb.WriteString(`<!DOCTYPE html>
<html>
<head>
<meta charset="UTF-8">
<title>K13s Cluster Report</title>
<style>
body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; margin: 40px; color: #333; }
h1 { color: #1a1b26; border-bottom: 3px solid #7aa2f7; padding-bottom: 10px; }
h2 { color: #24283b; margin-top: 30px; border-bottom: 1px solid #ddd; padding-bottom: 5px; }
table { width: 100%; border-collapse: collapse; margin: 15px 0; font-size: 12px; }
th, td { padding: 8px 12px; text-align: left; border: 1px solid #ddd; }
th { background: #24283b; color: white; }
tr:nth-child(even) { background: #f5f5f5; }
.metric-card { display: inline-block; background: #f0f0f0; padding: 15px 25px; margin: 10px; border-radius: 8px; text-align: center; }
.metric-value { font-size: 28px; font-weight: bold; color: #7aa2f7; }
.metric-label { font-size: 12px; color: #666; margin-top: 5px; }
.health-score { font-size: 48px; font-weight: bold; color: #9ece6a; }
.health-score.warning { color: #e0af68; }
.health-score.critical { color: #f7768e; }
.status-running { color: #9ece6a; font-weight: bold; }
.status-pending { color: #e0af68; font-weight: bold; }
.status-failed { color: #f7768e; font-weight: bold; }
.ai-analysis { background: #f8f9fa; border-left: 4px solid #7aa2f7; padding: 20px; margin: 20px 0; white-space: pre-wrap; }
.warning { background: #fff3cd; border-left: 4px solid #e0af68; padding: 10px 15px; margin: 10px 0; }
.cost-card { display: inline-block; background: #e8f5e9; padding: 15px 25px; margin: 10px; border-radius: 8px; text-align: center; border: 2px solid #4caf50; }
.cost-value { font-size: 24px; font-weight: bold; color: #2e7d32; }
.cost-label { font-size: 11px; color: #666; margin-top: 5px; }
.priority-high { color: #f44336; font-weight: bold; }
.priority-medium { color: #ff9800; font-weight: bold; }
.priority-low { color: #4caf50; font-weight: bold; }
.savings-badge { background: #4caf50; color: white; padding: 2px 8px; border-radius: 4px; font-size: 11px; }
.footer { margin-top: 40px; text-align: center; color: #999; font-size: 11px; }
@media print { body { margin: 20px; } }
</style>
</head>
<body>
`)

	// Header
	sb.WriteString(fmt.Sprintf(`<h1>🚀 K13s Cluster Report</h1>
<p><strong>Generated:</strong> %s | <strong>By:</strong> %s</p>
`, report.GeneratedAt.Format("2006-01-02 15:04:05"), report.GeneratedBy))

	// Health Score
	healthClass := ""
	if report.HealthScore < 70 {
		healthClass = "critical"
	} else if report.HealthScore < 90 {
		healthClass = "warning"
	}
	sb.WriteString(fmt.Sprintf(`<div style="text-align: center; margin: 30px 0;">
<div class="health-score %s">%.0f%%</div>
<div style="color: #666;">Cluster Health Score</div>
</div>`, healthClass, report.HealthScore))

	// Summary Cards
	sb.WriteString(`<div style="text-align: center;">`)
	sb.WriteString(fmt.Sprintf(`<div class="metric-card"><div class="metric-value">%d</div><div class="metric-label">Nodes (%d Ready)</div></div>`,
		report.NodeSummary.Total, report.NodeSummary.Ready))
	sb.WriteString(fmt.Sprintf(`<div class="metric-card"><div class="metric-value">%d</div><div class="metric-label">Pods (%d Running)</div></div>`,
		report.Workloads.TotalPods, report.Workloads.RunningPods))
	sb.WriteString(fmt.Sprintf(`<div class="metric-card"><div class="metric-value">%d</div><div class="metric-label">Deployments</div></div>`,
		report.Workloads.TotalDeployments))
	sb.WriteString(fmt.Sprintf(`<div class="metric-card"><div class="metric-value">%d</div><div class="metric-label">Services</div></div>`,
		report.Workloads.TotalServices))
	sb.WriteString(fmt.Sprintf(`<div class="metric-card"><div class="metric-value">%d</div><div class="metric-label">Namespaces</div></div>`,
		report.NamespaceSummary.Total))
	sb.WriteString(`</div>`)

	// AI Analysis (if available)
	if report.AIAnalysis != "" {
		sb.WriteString(`<h2>🤖 AI Analysis</h2>`)
		sb.WriteString(fmt.Sprintf(`<div class="ai-analysis">%s</div>`, report.AIAnalysis))
	}

	// Nodes
	sb.WriteString(`<h2>📦 Nodes</h2>`)
	sb.WriteString(`<table><tr><th>Name</th><th>Status</th><th>Roles</th><th>Version</th><th>CPU</th><th>Memory</th><th>IP</th></tr>`)
	for _, node := range report.Nodes {
		statusClass := "status-running"
		if node.Status != "Ready" {
			statusClass = "status-failed"
		}
		sb.WriteString(fmt.Sprintf(`<tr><td>%s</td><td class="%s">%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>`,
			node.Name, statusClass, node.Status, strings.Join(node.Roles, ", "), node.KubeletVersion, node.CPUCapacity, node.MemoryCapacity, node.InternalIP))
	}
	sb.WriteString(`</table>`)

	// Namespaces
	sb.WriteString(`<h2>📁 Namespaces</h2>`)
	sb.WriteString(`<table><tr><th>Name</th><th>Status</th><th>Pods</th><th>Deployments</th><th>Services</th></tr>`)
	for _, ns := range report.Namespaces {
		sb.WriteString(fmt.Sprintf(`<tr><td>%s</td><td>%s</td><td>%d</td><td>%d</td><td>%d</td></tr>`,
			ns.Name, ns.Status, ns.PodCount, ns.DeployCount, ns.ServiceCount))
	}
	sb.WriteString(`</table>`)

	// Pods (limit to first 50 for readability)
	sb.WriteString(`<h2>🔸 Pods</h2>`)
	if len(report.Pods) > 50 {
		sb.WriteString(fmt.Sprintf(`<p><em>Showing first 50 of %d pods</em></p>`, len(report.Pods)))
	}
	sb.WriteString(`<table><tr><th>Name</th><th>Namespace</th><th>Status</th><th>Ready</th><th>Restarts</th><th>Node</th><th>Age</th></tr>`)
	for i, pod := range report.Pods {
		if i >= 50 {
			break
		}
		statusClass := "status-running"
		switch pod.Status {
		case "Pending":
			statusClass = "status-pending"
		case "Failed", "CrashLoopBackOff", "Error":
			statusClass = "status-failed"
		}
		sb.WriteString(fmt.Sprintf(`<tr><td>%s</td><td>%s</td><td class="%s">%s</td><td>%s</td><td>%d</td><td>%s</td><td>%s</td></tr>`,
			pod.Name, pod.Namespace, statusClass, pod.Status, pod.Ready, pod.Restarts, pod.Node, pod.Age))
	}
	sb.WriteString(`</table>`)

	// Deployments
	sb.WriteString(`<h2>🚀 Deployments</h2>`)
	sb.WriteString(`<table><tr><th>Name</th><th>Namespace</th><th>Ready</th><th>Up-to-date</th><th>Available</th><th>Strategy</th><th>Age</th></tr>`)
	for _, dep := range report.Deployments {
		sb.WriteString(fmt.Sprintf(`<tr><td>%s</td><td>%s</td><td>%s</td><td>%d</td><td>%d</td><td>%s</td><td>%s</td></tr>`,
			dep.Name, dep.Namespace, dep.Ready, dep.UpToDate, dep.Available, dep.Strategy, dep.Age))
	}
	sb.WriteString(`</table>`)

	// Services
	sb.WriteString(`<h2>🌐 Services</h2>`)
	sb.WriteString(`<table><tr><th>Name</th><th>Namespace</th><th>Type</th><th>ClusterIP</th><th>ExternalIP</th><th>Ports</th></tr>`)
	for _, svc := range report.Services {
		sb.WriteString(fmt.Sprintf(`<tr><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>`,
			svc.Name, svc.Namespace, svc.Type, svc.ClusterIP, svc.ExternalIP, svc.Ports))
	}
	sb.WriteString(`</table>`)

	// Images
	sb.WriteString(`<h2>🐳 Container Images</h2>`)
	sb.WriteString(`<table><tr><th>Image</th><th>Tag</th><th>Pod Count</th></tr>`)
	for i, img := range report.Images {
		if i >= 20 {
			sb.WriteString(fmt.Sprintf(`<tr><td colspan="3"><em>... and %d more images</em></td></tr>`, len(report.Images)-20))
			break
		}
		sb.WriteString(fmt.Sprintf(`<tr><td>%s</td><td>%s</td><td>%d</td></tr>`,
			img.Repository, img.Tag, img.PodCount))
	}
	sb.WriteString(`</table>`)

	// FinOps Cost Analysis
	sb.WriteString(`<h2>💰 FinOps Cost Analysis</h2>`)
	sb.WriteString(`<div style="text-align: center; margin: 20px 0;">`)
	sb.WriteString(fmt.Sprintf(`<div class="cost-card"><div class="cost-value">$%.2f</div><div class="cost-label">Est. Monthly Cost</div></div>`,
		report.FinOpsAnalysis.TotalEstimatedMonthlyCost))
	sb.WriteString(fmt.Sprintf(`<div class="cost-card"><div class="cost-value">%.1f%%</div><div class="cost-label">CPU Utilization</div></div>`,
		report.FinOpsAnalysis.ResourceEfficiency.CPURequestsVsCapacity))
	sb.WriteString(fmt.Sprintf(`<div class="cost-card"><div class="cost-value">%.1f%%</div><div class="cost-label">Memory Utilization</div></div>`,
		report.FinOpsAnalysis.ResourceEfficiency.MemoryRequestsVsCapacity))
	sb.WriteString(`</div>`)

	// Resource Efficiency
	sb.WriteString(`<h3>Resource Efficiency</h3>`)
	sb.WriteString(`<table><tr><th>Metric</th><th>Value</th></tr>`)
	sb.WriteString(fmt.Sprintf(`<tr><td>Total CPU Requests</td><td>%s</td></tr>`, report.FinOpsAnalysis.ResourceEfficiency.TotalCPURequests))
	sb.WriteString(fmt.Sprintf(`<tr><td>Total CPU Limits</td><td>%s</td></tr>`, report.FinOpsAnalysis.ResourceEfficiency.TotalCPULimits))
	sb.WriteString(fmt.Sprintf(`<tr><td>Total Memory Requests</td><td>%s</td></tr>`, report.FinOpsAnalysis.ResourceEfficiency.TotalMemoryRequests))
	sb.WriteString(fmt.Sprintf(`<tr><td>Total Memory Limits</td><td>%s</td></tr>`, report.FinOpsAnalysis.ResourceEfficiency.TotalMemoryLimits))
	sb.WriteString(fmt.Sprintf(`<tr><td>Pods Without Requests</td><td>%d</td></tr>`, report.FinOpsAnalysis.ResourceEfficiency.PodsWithoutRequests))
	sb.WriteString(fmt.Sprintf(`<tr><td>Pods Without Limits</td><td>%d</td></tr>`, report.FinOpsAnalysis.ResourceEfficiency.PodsWithoutLimits))
	sb.WriteString(`</table>`)

	// Cost by Namespace
	if len(report.FinOpsAnalysis.CostByNamespace) > 0 {
		sb.WriteString(`<h3>Cost by Namespace</h3>`)
		sb.WriteString(`<table><tr><th>Namespace</th><th>Pods</th><th>CPU Requests</th><th>Memory Requests</th><th>Est. Cost/Month</th><th>% of Total</th></tr>`)
		for i, ns := range report.FinOpsAnalysis.CostByNamespace {
			if i >= 10 {
				sb.WriteString(fmt.Sprintf(`<tr><td colspan="6"><em>... and %d more namespaces</em></td></tr>`, len(report.FinOpsAnalysis.CostByNamespace)-10))
				break
			}
			sb.WriteString(fmt.Sprintf(`<tr><td>%s</td><td>%d</td><td>%s</td><td>%s</td><td>$%.2f</td><td>%.1f%%</td></tr>`,
				ns.Namespace, ns.PodCount, ns.CPURequests, ns.MemoryRequests, ns.EstimatedCost, ns.CostPercentage))
		}
		sb.WriteString(`</table>`)
	}

	// Cost Optimization Recommendations
	if len(report.FinOpsAnalysis.CostOptimizations) > 0 {
		totalSavings := 0.0
		for _, opt := range report.FinOpsAnalysis.CostOptimizations {
			totalSavings += opt.EstimatedSaving
		}
		sb.WriteString(`<h3>Cost Optimization Recommendations</h3>`)
		sb.WriteString(fmt.Sprintf(`<p><strong>Total Potential Savings:</strong> <span class="savings-badge">$%.2f/month</span></p>`, totalSavings))
		sb.WriteString(`<table><tr><th>Priority</th><th>Category</th><th>Description</th><th>Impact</th><th>Est. Savings</th></tr>`)
		for _, opt := range report.FinOpsAnalysis.CostOptimizations {
			priorityClass := "priority-" + opt.Priority
			sb.WriteString(fmt.Sprintf(`<tr><td class="%s">%s</td><td>%s</td><td>%s</td><td>%s</td><td>$%.2f</td></tr>`,
				priorityClass, strings.ToUpper(opt.Priority), opt.Category, opt.Description, opt.Impact, opt.EstimatedSaving))
		}
		sb.WriteString(`</table>`)
	}

	// Security Summary
	sb.WriteString(`<h2>🔒 Security Summary</h2>`)
	if report.SecurityInfo.PrivilegedPods > 0 || report.SecurityInfo.HostNetworkPods > 0 || report.SecurityInfo.RootContainers > 0 {
		sb.WriteString(`<div class="warning">⚠️ Security concerns detected - review privileged and root containers</div>`)
	}
	sb.WriteString(`<table><tr><th>Metric</th><th>Count</th></tr>`)
	sb.WriteString(fmt.Sprintf(`<tr><td>Secrets</td><td>%d</td></tr>`, report.SecurityInfo.Secrets))
	sb.WriteString(fmt.Sprintf(`<tr><td>Privileged Pods</td><td>%d</td></tr>`, report.SecurityInfo.PrivilegedPods))
	sb.WriteString(fmt.Sprintf(`<tr><td>Host Network Pods</td><td>%d</td></tr>`, report.SecurityInfo.HostNetworkPods))
	sb.WriteString(fmt.Sprintf(`<tr><td>Root Containers</td><td>%d</td></tr>`, report.SecurityInfo.RootContainers))
	sb.WriteString(`</table>`)

	// Warning Events
	if len(report.Events) > 0 {
		sb.WriteString(`<h2>⚠️ Warning Events</h2>`)
		sb.WriteString(`<table><tr><th>Reason</th><th>Object</th><th>Message</th><th>Count</th></tr>`)
		for i, event := range report.Events {
			if i >= 20 {
				break
			}
			msg := event.Message
			if len(msg) > 80 {
				msg = msg[:80] + "..."
			}
			sb.WriteString(fmt.Sprintf(`<tr><td>%s</td><td>%s</td><td>%s</td><td>%d</td></tr>`,
				event.Reason, event.Object, msg, event.Count))
		}
		sb.WriteString(`</table>`)
	}

	// Footer
	sb.WriteString(`<div class="footer">Generated by k13s - AI-Powered Kubernetes Dashboard</div>`)
	sb.WriteString(`</body></html>`)

	return sb.String()
}

// HandleReports handles report-related API requests
func (rg *ReportGenerator) HandleReports(w http.ResponseWriter, r *http.Request) {
	username := r.Header.Get("X-Username")
	if username == "" {
		username = "anonymous"
	}

	format := r.URL.Query().Get("format")       // json, csv, html
	includeAI := r.URL.Query().Get("ai") == "true"
	download := r.URL.Query().Get("download") == "true" // Force download (vs preview)

	switch r.Method {
	case http.MethodGet:
		// Generate comprehensive report
		report, err := rg.GenerateComprehensiveReport(r.Context(), username)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Add AI analysis if requested
		if includeAI {
			analysis, err := rg.GenerateAIAnalysis(r.Context(), report)
			if err == nil {
				report.AIAnalysis = analysis
			}
		}

		// Record audit
		db.RecordAudit(db.AuditEntry{
			User:     username,
			Action:   "generate_report",
			Resource: "cluster",
			Details:  fmt.Sprintf("Format: %s, AI: %v, Download: %v", format, includeAI, download),
		})

		// Return in requested format
		switch format {
		case "csv":
			csvData, err := rg.ExportToCSV(report)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "text/csv; charset=utf-8")
			if download {
				w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=k13s-report-%s.csv", time.Now().Format("20060102-150405")))
			}
			w.Write(csvData)

		case "html":
			htmlData := rg.ExportToHTML(report)
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			if download {
				w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=k13s-report-%s.html", time.Now().Format("20060102-150405")))
			}
			w.Write([]byte(htmlData))

		default: // json
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(report)
		}

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// HandleReportPreview handles report preview in a new window
func (rg *ReportGenerator) HandleReportPreview(w http.ResponseWriter, r *http.Request) {
	username := r.Header.Get("X-Username")
	if username == "" {
		username = "anonymous"
	}

	includeAI := r.URL.Query().Get("ai") == "true"

	// Generate report
	report, err := rg.GenerateComprehensiveReport(r.Context(), username)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Add AI analysis if requested
	if includeAI {
		analysis, err := rg.GenerateAIAnalysis(r.Context(), report)
		if err == nil {
			report.AIAnalysis = analysis
		}
	}

	// Record audit
	db.RecordAudit(db.AuditEntry{
		User:     username,
		Action:   "preview_report",
		Resource: "cluster",
		Details:  fmt.Sprintf("AI: %v", includeAI),
	})

	// Return HTML for preview (no Content-Disposition header)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	htmlData := rg.ExportToHTML(report)
	w.Write([]byte(htmlData))
}

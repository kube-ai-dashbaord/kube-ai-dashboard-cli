package ui

import (
	"context"
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/kube-ai-dashbaord/kube-ai-dashboard-cli/pkg/k8s"
	"github.com/rivo/tview"
)

type Dashboard struct {
	Root             *tview.List
	K8s              *k8s.Client
	CurrentResource  string
	CurrentNamespace string
}

func NewDashboard(k8sClient *k8s.Client, onSelected func(string)) *Dashboard {
	d := &Dashboard{
		Root:             tview.NewList(),
		K8s:              k8sClient,
		CurrentResource:  "pods",
		CurrentNamespace: "", // All namespaces
	}
	d.Root.SetBorder(true).SetTitle(" Dashboard (Pods) ")
	d.Root.SetSelectedFunc(func(index int, mainText string, secondaryText string, shortcut rune) {
		if onSelected != nil {
			onSelected(mainText)
		}
	})

	d.Root.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Rune() == 'l' {
			// In a real k9s, this would switch to a log view.
			// For now, let's ask the AI to fetch and explain the logs.
			if onSelected != nil {
				onSelected(fmt.Sprintf("Fetch and explain logs for %s %s", d.CurrentResource, d.Root.GetTitle()))
			}
			return nil
		}
		if event.Rune() == 'd' {
			// Describe resource via AI
			if onSelected != nil {
				_, name := d.Root.GetItemText(d.Root.GetCurrentItem())
				onSelected(fmt.Sprintf("Describe the %s %s", d.CurrentResource, name))
			}
			return nil
		}
		return event
	})

	d.Refresh()
	return d
}

func (d *Dashboard) SetResource(resourceType string) {
	d.CurrentResource = resourceType
	d.Root.SetTitle(fmt.Sprintf(" Dashboard (%s) ", resourceType))
	d.Refresh()
}

func (d *Dashboard) Refresh() {
	if d.K8s == nil {
		d.Root.AddItem("K8s Client not initialized", "", 0, nil)
		return
	}

	d.Root.Clear()
	ctx := context.Background()

	switch d.CurrentResource {
	case "pods", "po":
		pods, err := d.K8s.ListPods(ctx, d.CurrentNamespace)
		if err != nil {
			d.Root.AddItem(fmt.Sprintf("Error: %v", err), "", 0, nil)
			return
		}
		for _, pod := range pods {
			d.Root.AddItem(pod.Name, string(pod.Status.Phase), 0, nil)
		}
	case "nodes", "no":
		nodes, err := d.K8s.ListNodes(ctx)
		if err != nil {
			d.Root.AddItem(fmt.Sprintf("Error: %v", err), "", 0, nil)
			return
		}
		for _, node := range nodes {
			d.Root.AddItem(node.Name, "Node", 0, nil)
		}
	case "deployments", "deploy":
		deps, err := d.K8s.ListDeployments(ctx, d.CurrentNamespace)
		if err != nil {
			d.Root.AddItem(fmt.Sprintf("Error: %v", err), "", 0, nil)
			return
		}
		for _, dep := range deps {
			d.Root.AddItem(dep.Name, "Deployment", 0, nil)
		}
	case "services", "svc":
		svcs, err := d.K8s.ListServices(ctx, d.CurrentNamespace)
		if err != nil {
			d.Root.AddItem(fmt.Sprintf("Error: %v", err), "", 0, nil)
			return
		}
		for _, svc := range svcs {
			d.Root.AddItem(svc.Name, "Service", 0, nil)
		}
	case "configmaps", "cm":
		cms, err := d.K8s.ListConfigMaps(ctx, d.CurrentNamespace)
		if err != nil {
			d.Root.AddItem(fmt.Sprintf("Error: %v", err), "", 0, nil)
			return
		}
		for _, cm := range cms {
			d.Root.AddItem(cm.Name, "ConfigMap", 0, nil)
		}
	case "secrets":
		secrets, err := d.K8s.ListSecrets(ctx, d.CurrentNamespace)
		if err != nil {
			d.Root.AddItem(fmt.Sprintf("Error: %v", err), "", 0, nil)
			return
		}
		for _, secret := range secrets {
			d.Root.AddItem(secret.Name, "Secret", 0, nil)
		}
	case "ingresses", "ing":
		ings, err := d.K8s.ListIngresses(ctx, d.CurrentNamespace)
		if err != nil {
			d.Root.AddItem(fmt.Sprintf("Error: %v", err), "", 0, nil)
			return
		}
		for _, ing := range ings {
			d.Root.AddItem(ing.Name, "Ingress", 0, nil)
		}
	case "namespaces", "ns":
		namespaces, err := d.K8s.ListNamespaces(ctx)
		if err != nil {
			d.Root.AddItem(fmt.Sprintf("Error: %v", err), "", 0, nil)
			return
		}
		for _, ns := range namespaces {
			d.Root.AddItem(ns.Name, "Namespace", 0, nil)
		}
	default:
		d.Root.AddItem(fmt.Sprintf("Unknown resource: %s", d.CurrentResource), "", 0, nil)
	}
}

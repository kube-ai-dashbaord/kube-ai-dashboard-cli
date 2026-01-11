package ui

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/kube-ai-dashbaord/kube-ai-dashboard-cli/pkg/i18n"
	"github.com/kube-ai-dashbaord/kube-ai-dashboard-cli/pkg/k8s"
	"github.com/kube-ai-dashbaord/kube-ai-dashboard-cli/pkg/ui/resources"
	"github.com/rivo/tview"
)

func (d *Dashboard) SetFilter(filter string) {
	d.Filter = filter
	d.Refresh()
}

func (d *Dashboard) SetResource(resourceType string) {
	d.CurrentResource = resourceType
	d.Refresh()
}

type Dashboard struct {
	App                *tview.Application
	Root               *tview.Table
	K8s                *k8s.Client
	CurrentResource    string
	CurrentNamespace   string
	OnSelected         func(string)
	OnSelectionChanged func(string)
	OnLogs             func(namespace, name string)
	OnDeleteRequested  func(namespace, name string)
	OnYaml             func(namespace, name string)
	OnDescribe         func(namespace, name string)
	OnScale            func(namespace, name string)
	OnRestart          func(namespace, name string)
	OnPortForward      func(namespace, name string)
	OnExplainRequested func(namespace, name string)
	Filter             string
	isRefreshing       atomic.Bool
}

func NewDashboard(app *tview.Application, k8sClient *k8s.Client, onSelected func(string), onSelectionChanged func(string), onLogs func(namespace, name string)) *Dashboard {
	d := &Dashboard{
		App:                app,
		Root:               tview.NewTable().SetSelectable(true, false).SetSeparator('|'),
		K8s:                k8sClient,
		CurrentResource:    "pods",
		CurrentNamespace:   "", // All namespaces
		OnSelected:         onSelected,
		OnSelectionChanged: onSelectionChanged,
		OnLogs:             onLogs,
	}
	d.Root.SetBorder(true).SetTitle(fmt.Sprintf(" %s (%s) ", i18n.T("dashboard_pods"), "pods"))

	d.Root.SetSelectionChangedFunc(func(row, column int) {
		if row > 0 {
			cell := d.Root.GetCell(row, 1)
			if d.OnSelectionChanged != nil {
				d.OnSelectionChanged(cell.Text)
			}
		}
	})

	d.Root.SetSelectedFunc(func(row, column int) {
		if row > 0 { // Skip header
			if d.CurrentResource == "contexts" || d.CurrentResource == "ctx" {
				ctxName := d.Root.GetCell(row, 0).Text
				if err := d.K8s.SwitchContext(ctxName); err != nil {
					// Show error in a cell or title? For now title
					d.Root.SetTitle(fmt.Sprintf(" Error: %v ", err))
				} else {
					d.CurrentNamespace = "" // Reset namespace for new context
					d.SetResource("pods")
				}
				return
			}
			cell := d.Root.GetCell(row, 1) // Column 1 is usually NAME
			if d.OnSelected != nil {
				d.OnSelected(cell.Text)
			}
		}
	})

	d.Root.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		// handle 0-9 for quick selection
		if event.Rune() >= '0' && event.Rune() <= '9' {
			idx := int(event.Rune() - '0')
			if idx+1 < d.Root.GetRowCount() {
				if d.CurrentResource == "namespaces" || d.CurrentResource == "ns" {
					// Quick-switch namespace and jump to pods
					nsName := d.Root.GetCell(idx+1, 1).Text
					d.CurrentNamespace = nsName
					d.SetResource("pods")
					return nil
				}
				d.Root.Select(idx+1, 0)
				return nil
			}
		}

		// handle h, j, k, l for navigation
		if event.Rune() == 'j' {
			return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
		}
		if event.Rune() == 'k' {
			return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
		}
		if event.Rune() == 'h' {
			// In dashboard, 'h' is 'explain this'
			row, _ := d.Root.GetSelection()
			if row > 0 {
				name := d.Root.GetCell(row, 1).Text
				ns := d.CurrentNamespace
				if d.OnExplainRequested != nil {
					d.OnExplainRequested(ns, name)
				}
			}
			return nil
		}
		if event.Rune() == 'l' {
			// In dashboard, 'l' is logs, so we don't map it to Right
			// unless we decide otherwise. K9s uses 'l' for logs too.
		}

		if event.Rune() == '/' {
			return event // Let App handle /
		}

		if event.Key() == tcell.KeyEscape {
			if d.Filter != "" {
				d.Filter = ""
				d.Refresh()
				return nil
			}
		}

		if event.Rune() == 'l' { // Logs
			row, _ := d.Root.GetSelection()
			if row > 0 {
				name := d.Root.GetCell(row, 1).Text
				// Find namespace from table if possible or use current
				ns := d.CurrentNamespace
				if ns == "" {
					// In "all" namespaces mode, we might need to find it from a column
					// (not implemented yet, assuming CurrentNamespace for now)
				}
				if d.OnLogs != nil {
					d.OnLogs(ns, name)
				}
			}
			return nil
		}
		if event.Rune() == 'y' { // View YAML
			row, _ := d.Root.GetSelection()
			if row > 0 {
				name := d.Root.GetCell(row, 1).Text
				ns := d.CurrentNamespace
				if d.OnYaml != nil {
					d.OnYaml(ns, name)
				}
			}
			return nil
		}

		if event.Rune() == 'd' { // Describe (AI)
			row, _ := d.Root.GetSelection()
			if row > 0 {
				name := d.Root.GetCell(row, 1).Text
				ns := d.CurrentNamespace
				if d.OnDescribe != nil {
					d.OnDescribe(ns, name)
				}
			}
			return nil
		}

		if event.Rune() == 's' { // Scale
			row, _ := d.Root.GetSelection()
			if row > 0 {
				name := d.Root.GetCell(row, 1).Text
				ns := d.CurrentNamespace
				if d.OnScale != nil {
					d.OnScale(ns, name)
				}
			}
			return nil
		}
		if event.Rune() == 'r' { // Restart (Rollout)
			row, _ := d.Root.GetSelection()
			if row > 0 {
				name := d.Root.GetCell(row, 1).Text
				ns := d.CurrentNamespace
				if d.OnRestart != nil {
					d.OnRestart(ns, name)
				}
			}
			return nil
		}

		if event.Rune() == 'F' { // Port Forward (Shift-F)
			row, _ := d.Root.GetSelection()
			if row > 0 {
				name := d.Root.GetCell(row, 1).Text
				ns := d.CurrentNamespace
				if d.OnPortForward != nil {
					d.OnPortForward(ns, name)
				}
			}
			return nil
		}

		if event.Key() == tcell.KeyCtrlD { // Delete
			row, _ := d.Root.GetSelection()
			if row > 0 {
				name := d.Root.GetCell(row, 1).Text
				ns := d.CurrentNamespace
				if ns == "" {
					// ns logic
				}
				if d.OnDeleteRequested != nil {
					d.OnDeleteRequested(ns, name)
				}
			}
			return nil
		}
		return event
	})

	d.Refresh()
	return d
}

func (d *Dashboard) Refresh() {
	if d.K8s == nil {
		d.Root.Clear()
		d.Root.SetCell(0, 0, tview.NewTableCell(i18n.T("error_client")).SetTextColor(tcell.ColorRed))
		return
	}

	// Clear the table and show a loading message immediately
	d.Root.Clear()
	d.Root.SetCell(0, 0, tview.NewTableCell(i18n.T("loading")).SetTextColor(tcell.ColorGray))

	if d.isRefreshing.Swap(true) {
		return // Already refreshing
	}

	// Fetch data in background
	go func() {
		defer d.isRefreshing.Store(false)
		ctx := context.Background()
		// We'll prepare rows and headers in advance
		var headers []string
		var rows [][]resources.TableCell
		var fetchErr error

		switch d.CurrentResource {
		case "pods", "po":
			view, err := resources.GetPodsView(ctx, d.K8s, d.CurrentNamespace, d.Filter, nil)
			if err != nil {
				fetchErr = err
			} else {
				headers = view.Headers
				rows = view.Rows
			}
		case "nodes", "no":
			headers = []string{"NAME", "STATUS", "ROLES", "VERSION", "CPU(m)", "MEM(MB)", "AGE"}
			view, err := resources.GetNodesView(ctx, d.K8s, d.Filter)
			if err != nil {
				fetchErr = err
			} else {
				headers = view.Headers
				rows = view.Rows
			}
		case "namespaces", "ns":
			view, err := resources.GetNamespacesView(ctx, d.K8s, d.Filter)
			if err != nil {
				fetchErr = err
			} else {
				headers = view.Headers
				rows = view.Rows
			}
		case "services", "svc":
			view, err := resources.GetServicesView(ctx, d.K8s, d.CurrentNamespace, d.Filter)
			if err != nil {
				fetchErr = err
			} else {
				headers = view.Headers
				rows = view.Rows
			}
		case "deployments", "deploy":
			view, err := resources.GetDeploymentsView(ctx, d.K8s, d.CurrentNamespace, d.Filter)
			if err != nil {
				fetchErr = err
			} else {
				headers = view.Headers
				rows = view.Rows
			}
		case "statefulsets", "sts":
			view, err := resources.GetStatefulSetsView(ctx, d.K8s, d.CurrentNamespace, d.Filter)
			if err != nil {
				fetchErr = err
			} else {
				headers = view.Headers
				rows = view.Rows
			}
		case "events", "ev":
			view, err := resources.GetEventsView(ctx, d.K8s, d.CurrentNamespace, d.Filter)
			if err != nil {
				fetchErr = err
			} else {
				headers = view.Headers
				rows = view.Rows
			}
		case "configmaps", "cm":
			view, err := resources.GetConfigMapsView(ctx, d.K8s, d.CurrentNamespace, d.Filter)
			if err != nil {
				fetchErr = err
			} else {
				headers = view.Headers
				rows = view.Rows
			}
		case "secrets":
			view, err := resources.GetSecretsView(ctx, d.K8s, d.CurrentNamespace, d.Filter)
			if err != nil {
				fetchErr = err
			} else {
				headers = view.Headers
				rows = view.Rows
			}
		case "ingresses", "ing":
			view, err := resources.GetIngressesView(ctx, d.K8s, d.CurrentNamespace, d.Filter)
			if err != nil {
				fetchErr = err
			} else {
				headers = view.Headers
				rows = view.Rows
			}
		case "roles":
			view, err := resources.GetRolesView(ctx, d.K8s, d.CurrentNamespace, d.Filter)
			if err != nil {
				fetchErr = err
			} else {
				headers = view.Headers
				rows = view.Rows
			}
		case "rolebindings", "rb":
			view, err := resources.GetRoleBindingsView(ctx, d.K8s, d.CurrentNamespace, d.Filter)
			if err != nil {
				fetchErr = err
			} else {
				headers = view.Headers
				rows = view.Rows
			}
		case "clusterroles":
			view, err := resources.GetClusterRolesView(ctx, d.K8s, d.Filter)
			if err != nil {
				fetchErr = err
			} else {
				headers = view.Headers
				rows = view.Rows
			}
		case "clusterrolebindings", "crb":
			view, err := resources.GetClusterRoleBindingsView(ctx, d.K8s, d.Filter)
			if err != nil {
				fetchErr = err
			} else {
				headers = view.Headers
				rows = view.Rows
			}
		case "persistentvolumes", "pv":
			view, err := resources.GetPersistentVolumesView(ctx, d.K8s, d.Filter)
			if err != nil {
				fetchErr = err
			} else {
				headers = view.Headers
				rows = view.Rows
			}
		case "persistentvolumeclaims", "pvc":
			view, err := resources.GetPersistentVolumeClaimsView(ctx, d.K8s, d.CurrentNamespace, d.Filter)
			if err != nil {
				fetchErr = err
			} else {
				headers = view.Headers
				rows = view.Rows
			}
		case "storageclasses", "sc":
			view, err := resources.GetStorageClassesView(ctx, d.K8s, d.Filter)
			if err != nil {
				fetchErr = err
			} else {
				headers = view.Headers
				rows = view.Rows
			}
		case "serviceaccounts", "sa":
			view, err := resources.GetServiceAccountsView(ctx, d.K8s, d.CurrentNamespace, d.Filter)
			if err != nil {
				fetchErr = err
			} else {
				headers = view.Headers
				rows = view.Rows
			}
		case "contexts", "ctx":
			view, _, err := resources.GetContextsView(d.K8s)
			if err != nil {
				fetchErr = err
			} else {
				headers = view.Headers
				rows = view.Rows
			}
		default:
			headers = []string{"#", "NAME", "AGE"}
			rows = [][]resources.TableCell{
				{{Text: "Unknown resource", Color: tcell.ColorRed}, {Text: d.CurrentResource, Color: tcell.ColorWhite}},
			}
		}

		// Apply updates to the UI
		d.App.QueueUpdateDraw(func() {
			d.Root.Clear()
			if fetchErr != nil {
				d.Root.SetCell(0, 0, tview.NewTableCell(fmt.Sprintf("Error: %v", fetchErr)).SetTextColor(tcell.ColorRed))
				return
			}
			// Headers
			for i, h := range headers {
				d.Root.SetCell(0, i, tview.NewTableCell(h).SetTextColor(tcell.ColorYellow).SetAttributes(tcell.AttrBold).SetSelectable(false))
			}
			// Rows
			for r, rowCells := range rows {
				for i, cell := range rowCells {
					d.Root.SetCell(r+1, i, tview.NewTableCell(cell.Text).SetTextColor(cell.Color))
				}
			}
			d.Root.ScrollToBeginning()
		})
	}()
}

func (d *Dashboard) getStatusColor(status string) tcell.Color {
	switch strings.ToLower(status) {
	case "running", "active", "ready", "complete", "succeeded":
		return tcell.ColorGreen
	case "pending", "containercreating", "podinitializing":
		return tcell.ColorYellow
	case "failed", "error", "crashloopbackoff", "notready", "deleted":
		return tcell.ColorRed
	default:
		return tcell.ColorWhite
	}
}

func formatAge(dur time.Duration) string {
	if dur.Hours() > 24 {
		return fmt.Sprintf("%dd", int(dur.Hours()/24))
	} else if dur.Hours() > 1 {
		return fmt.Sprintf("%dh", int(dur.Hours()))
	} else {
		return fmt.Sprintf("%dm", int(dur.Minutes()))
	}
}

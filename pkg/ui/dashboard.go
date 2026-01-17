package ui

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/kube-ai-dashbaord/kube-ai-dashboard-cli/pkg/i18n"
	"github.com/kube-ai-dashbaord/kube-ai-dashboard-cli/pkg/k8s"
	"github.com/kube-ai-dashbaord/kube-ai-dashboard-cli/pkg/log"
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
	OnAnalyze          func(namespace, name string)
	OnScale            func(namespace, name string)
	OnRestart          func(namespace, name string)
	OnPortForward      func(namespace, name string)
	OnExplainRequested func(namespace, name string)
	OnShell            func(namespace, name string) // Shell into pod (s key)
	OnEdit             func(namespace, name string) // Edit resource (e key)
	OnRefresh          func()
	Filter             string
	isRefreshing       atomic.Bool
	QuickNamespaces    []string
	SelectedRows       map[int]bool // Multi-select: tracks selected row indices
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
		SelectedRows:       make(map[int]bool),
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
		if event.Key() == tcell.KeyRune {
			Debugf("Dashboard key pressed: %c", event.Rune())
		}
		// handle 0-9 for quick selection
		if event.Rune() >= '0' && event.Rune() <= '9' {
			idx := int(event.Rune() - '0')
			if len(d.QuickNamespaces) > idx {
				nsName := d.QuickNamespaces[idx]
				d.CurrentNamespace = nsName
				d.Refresh()
				return nil
			}
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

		// g - Go to top of list
		if event.Rune() == 'g' {
			d.Root.ScrollToBeginning()
			d.Root.Select(1, 0) // Skip header row
			return nil
		}

		// G - Go to bottom of list
		if event.Rune() == 'G' {
			d.Root.ScrollToEnd()
			rowCount := d.Root.GetRowCount()
			if rowCount > 1 {
				d.Root.Select(rowCount-1, 0)
			}
			return nil
		}

		// Ctrl+U - Page up (move up 10 rows)
		if event.Key() == tcell.KeyCtrlU {
			row, col := d.Root.GetSelection()
			newRow := row - 10
			if newRow < 1 {
				newRow = 1 // Skip header
			}
			d.Root.Select(newRow, col)
			return nil
		}

		// Ctrl+F - Page down (move down 10 rows)
		if event.Key() == tcell.KeyCtrlF {
			row, col := d.Root.GetSelection()
			rowCount := d.Root.GetRowCount()
			newRow := row + 10
			if newRow >= rowCount {
				newRow = rowCount - 1
			}
			if newRow < 1 {
				newRow = 1
			}
			d.Root.Select(newRow, col)
			return nil
		}

		// Space - Toggle multi-select on current row
		if event.Rune() == ' ' {
			row, _ := d.Root.GetSelection()
			if row > 0 { // Skip header
				d.toggleRowSelection(row)
			}
			return nil
		}

		// Ctrl+Space - Clear all selections
		if event.Key() == tcell.KeyCtrlSpace {
			d.clearAllSelections()
			return nil
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

		if event.Rune() == 'd' { // Describe (Native)
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
		if event.Rune() == 'L' { // Analyze (AI)
			row, _ := d.Root.GetSelection()
			if row > 0 {
				name := d.Root.GetCell(row, 1).Text
				ns := d.CurrentNamespace
				if d.OnAnalyze != nil {
					d.OnAnalyze(ns, name)
				}
			}
			return nil
		}

		if event.Rune() == 'S' { // Scale (Shift-S)
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

		if event.Rune() == 's' { // Shell into pod
			// Only for pods resource
			if d.CurrentResource == "pods" || d.CurrentResource == "po" {
				row, _ := d.Root.GetSelection()
				if row > 0 {
					name := d.Root.GetCell(row, 1).Text
					ns := d.CurrentNamespace
					if ns == "" {
						// Get namespace from table if in "all namespaces" mode
						if nsCell := d.Root.GetCell(row, 0); nsCell != nil {
							nsText := nsCell.Text
							if len(nsText) > 2 && nsText[:2] == "● " {
								nsText = nsText[2:]
							}
							ns = nsText
						}
					}
					if d.OnShell != nil {
						d.OnShell(ns, name)
					}
				}
			}
			return nil
		}

		if event.Rune() == 'e' { // Edit resource
			row, _ := d.Root.GetSelection()
			if row > 0 {
				name := d.Root.GetCell(row, 1).Text
				ns := d.CurrentNamespace
				if ns == "" {
					// Get namespace from table if in "all namespaces" mode
					if nsCell := d.Root.GetCell(row, 0); nsCell != nil {
						nsText := nsCell.Text
						if len(nsText) > 2 && nsText[:2] == "● " {
							nsText = nsText[2:]
						}
						ns = nsText
					}
				}
				if d.OnEdit != nil {
					d.OnEdit(ns, name)
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
		if event.Rune() == 'h' { // Explain (AI)
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
		return event
	})

	// Note: Refresh is NOT called here. It will be triggered by SetAfterDrawFunc in app.Run()
	// after the event loop starts, ensuring QueueUpdateDraw callbacks are processed.
	return d
}

func (d *Dashboard) Refresh() {
	if d.K8s == nil {
		log.Errorf("Dashboard.Refresh: K8s client is nil")
		d.App.QueueUpdateDraw(func() {
			d.Root.Clear()
			d.Root.SetCell(0, 0, tview.NewTableCell(i18n.T("error_client")).SetTextColor(tcell.ColorRed))
		})
		return
	}

	if d.isRefreshing.Swap(true) {
		log.Infof("Dashboard refresh skipped: already in progress")
		return // Already refreshing
	}

	// Only clear and show loading if we are actually starting a new refresh
	d.Root.Clear()
	d.Root.SetCell(0, 0, tview.NewTableCell(i18n.T("loading")).SetTextColor(tcell.ColorGray))

	log.Infof("Dashboard refresh starting (with namespace: %s, resource: %s)", d.CurrentNamespace, d.CurrentResource)

	// Fetch data in background
	go func() {
		defer d.isRefreshing.Store(false)
		defer func() {
			if r := recover(); r != nil {
				Errorf("PANIC in Refresh goroutine: %v", r)
				log.Errorf("PANIC in Refresh goroutine: %v", r)
			}
		}()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		// We'll prepare rows and headers in advance
		var headers []string
		var rows [][]resources.TableCell
		var fetchErr error

		go d.UpdateQuickNamespaces(context.Background())

		Infof("Refreshing resource: %s in namespace: %s", d.CurrentResource, d.CurrentNamespace)
		switch d.CurrentResource {
		case "pods", "po":
			log.Infof("Refresh: Dispatching GetPodsView...")
			view, err := resources.GetPodsView(ctx, d.K8s, d.CurrentNamespace, d.Filter, nil)
			if err != nil {
				log.Errorf("Refresh: GetPodsView error: %v", err)
				fetchErr = err
			} else {
				headers = view.Headers
				rows = view.Rows
				log.Infof("Refresh: GetPodsView success (fetched %d rows)", len(rows))
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
		case "daemonsets", "ds":
			view, err := resources.GetDaemonSetsView(ctx, d.K8s, d.CurrentNamespace, d.Filter)
			if err != nil {
				fetchErr = err
			} else {
				headers = view.Headers
				rows = view.Rows
			}
		case "jobs":
			view, err := resources.GetJobsView(ctx, d.K8s, d.CurrentNamespace, d.Filter)
			if err != nil {
				fetchErr = err
			} else {
				headers = view.Headers
				rows = view.Rows
			}
		case "cronjobs", "cj":
			view, err := resources.GetCronJobsView(ctx, d.K8s, d.CurrentNamespace, d.Filter)
			if err != nil {
				fetchErr = err
			} else {
				headers = view.Headers
				rows = view.Rows
			}
		case "hpa", "horizontalpodautoscalers":
			view, err := resources.GetHPAView(ctx, d.K8s, d.CurrentNamespace, d.Filter)
			if err != nil {
				fetchErr = err
			} else {
				headers = view.Headers
				rows = view.Rows
			}
		case "networkpolicies", "netpol":
			view, err := resources.GetNetworkPoliciesView(ctx, d.K8s, d.CurrentNamespace, d.Filter)
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
		log.Infof("Refresh: Queuing UI update (headers: %d, rows: %d, err: %v)", len(headers), len(rows), fetchErr)
		d.App.QueueUpdateDraw(func() {
			log.Infof("Refresh: QueueUpdateDraw EXECUTING (setting %d rows)", len(rows))
			d.Root.Clear()
			if fetchErr != nil {
				log.Errorf("Refresh: Displaying error: %v", fetchErr)
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
			log.Infof("Refresh: Table updated successfully with %d rows", len(rows))
			d.Root.ScrollToBeginning()
			if d.OnRefresh != nil {
				d.OnRefresh()
			}
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
func (d *Dashboard) UpdateQuickNamespaces(ctx context.Context) {
	Infof("Updating quick namespaces...")
	log.Infof("Updating quick namespaces...")
	if d.K8s == nil {
		log.Errorf("K8s is nil in UpdateQuickNamespaces")
		return
	}
	nss, err := d.K8s.ListNamespaces(ctx)
	if err != nil {
		Errorf("Failed to list namespaces: %v", err)
		log.Errorf("Failed to list namespaces: %v", err)
		d.QuickNamespaces = []string{""} // Fallback to all
		return
	}
	Infof("Found %d namespaces", len(nss))
	log.Infof("Found %d namespaces", len(nss))

	// 0 is always "all"
	mapping := []string{""}

	// Try to find kube-system for 1
	var kubeSystemFound bool
	for _, ns := range nss {
		if ns.Name == "kube-system" {
			mapping = append(mapping, "kube-system")
			kubeSystemFound = true
			break
		}
	}

	// Add other namespaces up to 9
	count := len(mapping)
	for _, ns := range nss {
		if count >= 10 {
			break
		}
		if ns.Name == "kube-system" && kubeSystemFound {
			continue
		}
		mapping = append(mapping, ns.Name)
		count++
	}
	d.QuickNamespaces = mapping
}

func (d *Dashboard) GetNamespaceMapping() string {
	res := ""
	for i, ns := range d.QuickNamespaces {
		name := ns
		if ns == "" {
			name = "all"
		}
		res += fmt.Sprintf("[%d] %s  ", i, name)
	}
	return res
}

// toggleRowSelection toggles the selection state of a row and updates visual marker
func (d *Dashboard) toggleRowSelection(row int) {
	if d.SelectedRows[row] {
		delete(d.SelectedRows, row)
		// Remove marker
		d.updateRowMarker(row, false)
	} else {
		d.SelectedRows[row] = true
		// Add marker
		d.updateRowMarker(row, true)
	}
}

// updateRowMarker updates the visual selection marker for a row
func (d *Dashboard) updateRowMarker(row int, selected bool) {
	if row <= 0 || row >= d.Root.GetRowCount() {
		return
	}
	// Get the first cell (usually NAMESPACE or NAME)
	cell := d.Root.GetCell(row, 0)
	if cell == nil {
		return
	}
	text := cell.Text

	// Remove existing marker if present
	if len(text) > 2 && text[:2] == "● " {
		text = text[2:]
	}

	// Add marker if selected
	if selected {
		text = "● " + text
		cell.SetTextColor(tcell.ColorAqua)
	} else {
		cell.SetTextColor(tcell.ColorWhite)
	}
	cell.SetText(text)
}

// clearAllSelections clears all row selections
func (d *Dashboard) clearAllSelections() {
	for row := range d.SelectedRows {
		d.updateRowMarker(row, false)
	}
	d.SelectedRows = make(map[int]bool)
}

// GetSelectedItems returns a list of selected resource names with their namespaces
func (d *Dashboard) GetSelectedItems() []struct{ Namespace, Name string } {
	var items []struct{ Namespace, Name string }
	for row := range d.SelectedRows {
		if row > 0 && row < d.Root.GetRowCount() {
			ns := d.CurrentNamespace
			name := d.Root.GetCell(row, 1).Text
			// If namespace column exists (column 0), use it
			if nsCell := d.Root.GetCell(row, 0); nsCell != nil {
				nsText := nsCell.Text
				// Remove marker if present
				if len(nsText) > 2 && nsText[:2] == "● " {
					nsText = nsText[2:]
				}
				ns = nsText
			}
			items = append(items, struct{ Namespace, Name string }{ns, name})
		}
	}
	return items
}

// HasMultipleSelections returns true if more than one row is selected
func (d *Dashboard) HasMultipleSelections() bool {
	return len(d.SelectedRows) > 1
}

// GetSelectionCount returns the number of selected rows
func (d *Dashboard) GetSelectionCount() int {
	return len(d.SelectedRows)
}

// MatchesFilter checks if text matches the current filter
// Supports regex patterns when filter starts with /
func (d *Dashboard) MatchesFilter(text string) bool {
	if d.Filter == "" {
		return true
	}

	// Check for regex mode (filter starts with /)
	if strings.HasPrefix(d.Filter, "/") {
		pattern := d.Filter[1:]
		// Remove trailing / if present
		if strings.HasSuffix(pattern, "/") {
			pattern = pattern[:len(pattern)-1]
		}
		if pattern == "" {
			return true
		}
		re, err := regexp.Compile("(?i)" + pattern) // Case-insensitive
		if err != nil {
			// If invalid regex, fall back to substring match
			return strings.Contains(strings.ToLower(text), strings.ToLower(d.Filter))
		}
		return re.MatchString(text)
	}

	// Default: case-insensitive substring match
	return strings.Contains(strings.ToLower(text), strings.ToLower(d.Filter))
}

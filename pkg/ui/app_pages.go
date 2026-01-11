package ui

import (
	"fmt"
	"strings"

	"github.com/kube-ai-dashbaord/kube-ai-dashboard-cli/pkg/i18n"
	"github.com/rivo/tview"
)

func (a *App) initPages() {
	a.DashboardWidth = 60
	a.AssistantWidth = 40

	mainContent := tview.NewFlex().
		AddItem(a.Dashboard.Root, 0, a.DashboardWidth, true).
		AddItem(a.Assistant.Root, 0, a.AssistantWidth, false)

	a.ShortcutBar = tview.NewTextView().
		SetDynamicColors(true).
		SetRegions(true).
		SetWrap(false).
		SetTextAlign(tview.AlignCenter).
		SetText(fmt.Sprintf(" [yellow]%s  [white]|  [yellow]%s  [white]|  [yellow]%s  [white]|  [yellow]%s  [white]|  [yellow]%s  [white]|  [yellow]%s  [white]|  [yellow]%s  [white]|  [yellow]%s ",
			i18n.T("shortcut_help"),
			i18n.T("shortcut_cmd"),
			i18n.T("shortcut_settings"),
			i18n.T("shortcut_yaml"),
			i18n.T("shortcut_describe"),
			i18n.T("shortcut_analyze"),
			i18n.T("shortcut_forward"),
			i18n.T("shortcut_quit")))

	a.CommandBar = NewCommandBar(func(cmd string) {
		if strings.HasPrefix(cmd, ":") {
			a.handleCommand(cmd[1:])
			return
		}
		a.Dashboard.SetResource(cmd)
	}, func(filter string) {
		a.Dashboard.SetFilter(filter)
	})

	a.Help = NewHelp(func() {
		a.Pages.HidePage("help")
	})
	a.Help.Table.SetTitle(fmt.Sprintf(" %s ", i18n.T("help_title")))

	a.Root = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(a.HeaderContainer, 3, 0, false).
		AddItem(mainContent, 0, 1, true).
		AddItem(a.ShortcutBar, 1, 0, false).
		AddItem(a.CommandBar.Input, 1, 0, false)

	a.Pages.AddPage("main", a.Root, true, true)
	a.Pages.AddPage("settings", a.Settings.Form, true, false)
	a.Pages.AddPage("help", a.Help.Table, true, false)
	a.AuditViewer.Table.SetTitle(fmt.Sprintf(" %s ", i18n.T("audit_logs")))
	a.Pages.AddPage("audit", a.AuditViewer.Table, true, false)
}

func (a *App) handleCommand(cmd string) {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return
	}

	mainCmd := parts[0]
	switch mainCmd {
	case "audit":
		a.Pages.SwitchToPage("audit")
		a.Application.SetFocus(a.AuditViewer.Table)
	case "view":
		if len(parts) >= 3 {
			// :view <type> <name> [<ns>]
			resType := parts[1]
			name := parts[2]
			ns := ""
			if len(parts) >= 4 {
				ns = parts[3]
			}
			a.ViewResource(resType, ns, name)
		}
	}
}

func (a *App) ViewResource(resType, ns, name string) {
	if ns != "" {
		a.Dashboard.CurrentNamespace = ns
	}
	a.Dashboard.SetResource(resType)
	// We could also select the row with the name... but Refresh() is async.
	// For now, just switching the view is a huge step.
}

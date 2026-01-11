package ui

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/kube-ai-dashbaord/kube-ai-dashboard-cli/pkg/i18n"
	"github.com/rivo/tview"
)

func (a *App) RefreshShortcuts() {
	res := a.Dashboard.CurrentResource
	shortcuts := []string{
		fmt.Sprintf("[yellow]%s[white]", i18n.T("shortcut_help")),
		fmt.Sprintf("[yellow]%s[white]", i18n.T("shortcut_cmd")),
		fmt.Sprintf("[yellow]%s[white]", i18n.T("shortcut_settings")),
		fmt.Sprintf("[yellow]%s[white]", i18n.T("shortcut_yaml")),
	}

	// Contextual shortcuts
	if res == "pods" || res == "po" {
		shortcuts = append(shortcuts, fmt.Sprintf("[yellow]%s[white]", i18n.T("desc_logs")))
		shortcuts = append(shortcuts, fmt.Sprintf("[yellow]%s[white]", i18n.T("desc_describe")))
		shortcuts = append(shortcuts, fmt.Sprintf("[yellow]L [white]%s", i18n.T("desc_analyze")))
		shortcuts = append(shortcuts, fmt.Sprintf("[yellow]S [white]%s", i18n.T("desc_scale")))
		shortcuts = append(shortcuts, fmt.Sprintf("[yellow]r [white]%s", i18n.T("desc_restart")))
	} else if res == "deploy" || res == "deployments" || res == "sts" || res == "statefulsets" {
		shortcuts = append(shortcuts, fmt.Sprintf("[yellow]%s[white]", i18n.T("desc_describe")))
		shortcuts = append(shortcuts, fmt.Sprintf("[yellow]L [white]%s", i18n.T("desc_analyze")))
		shortcuts = append(shortcuts, fmt.Sprintf("[yellow]S [white]%s", i18n.T("desc_scale")))
		shortcuts = append(shortcuts, fmt.Sprintf("[yellow]r [white]%s", i18n.T("desc_restart")))
	} else if res != "" {
		shortcuts = append(shortcuts, fmt.Sprintf("[yellow]%s[white]", i18n.T("desc_describe")))
		shortcuts = append(shortcuts, fmt.Sprintf("[yellow]L [white]%s", i18n.T("desc_analyze")))
	}

	shortcuts = append(shortcuts, fmt.Sprintf("[yellow]%s[white]", i18n.T("shortcut_quit")))

	a.ShortcutBar.SetText(" " + strings.Join(shortcuts, "  |  "))
}

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
		SetTextAlign(tview.AlignCenter)
	a.RefreshShortcuts()

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

	a.Application.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		// Global Keys
		if event.Rune() == '?' {
			a.Pages.ShowPage("help")
			return nil
		}
		if event.Rune() == 's' {
			a.Pages.SwitchToPage("settings")
			return nil
		}
		if event.Rune() == '/' {
			a.Application.SetFocus(a.CommandBar.Input)
			return nil
		}
		if event.Key() == tcell.KeyTab {
			if a.Dashboard.Root.HasFocus() {
				a.Application.SetFocus(a.Assistant.Input)
			} else {
				a.Application.SetFocus(a.Dashboard.Root)
			}
			return nil
		}
		if event.Key() == tcell.KeyCtrlH {
			if a.DashboardWidth > 10 {
				a.DashboardWidth -= 2
				a.AssistantWidth += 2
				mainContent.ResizeItem(a.Dashboard.Root, 0, a.DashboardWidth)
				mainContent.ResizeItem(a.Assistant.Root, 0, a.AssistantWidth)
			}
			return nil
		}
		if event.Key() == tcell.KeyCtrlL {
			if a.AssistantWidth > 10 {
				a.DashboardWidth += 2
				a.AssistantWidth -= 2
				mainContent.ResizeItem(a.Dashboard.Root, 0, a.DashboardWidth)
				mainContent.ResizeItem(a.Assistant.Root, 0, a.AssistantWidth)
			}
			return nil
		}
		if event.Key() == tcell.KeyEscape {
			// If in a sub-page, return to main
			front, _ := a.Pages.GetFrontPage()
			if front != "main" {
				a.Pages.SwitchToPage("main")
				a.Application.SetFocus(a.Dashboard.Root)
				return nil
			}
		}

		return event
	})
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
	case "pulse", "events":
		a.PulseViewer.Refresh()
		a.Pages.AddPage("pulse", a.PulseViewer.Table, true, true)
		a.Pages.SwitchToPage("pulse")
	}
}

func (a *App) ViewResource(resType, ns, name string) {
	if ns != "" {
		a.Dashboard.CurrentNamespace = ns
	}
	a.Dashboard.SetResource(resType)
	a.RefreshShortcuts()
}

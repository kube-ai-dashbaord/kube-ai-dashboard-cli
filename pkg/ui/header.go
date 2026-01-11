package ui

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

const Logo = `
 _  _   _   ____   ____ 
| |/ / / | |___ \ / ___|
|   /  | |   __) |\___ \ 
|  \   | |  / __/  ___) |
|_|\_\ |_| |_____||____/ 
    Kubernetes AI Dashboard (k13s)
`

func NewHeader(context, cluster, user, k8sVersion, namespace string, aiStatus string, nsMap string, resource string, w int) *tview.Flex {
	infoTable := tview.NewTable().SetSelectable(false, false)
	infoTable.SetBackgroundColor(tview.Styles.PrimitiveBackgroundColor)

	sections := []string{"Context", "Cluster", "User", "K8s Rev", "Namespace", "Quick NS"}
	values := []string{context, cluster, user, k8sVersion, namespace, nsMap}

	for i, section := range sections {
		infoTable.SetCell(i, 0, tview.NewTableCell(section+":").SetTextColor(tview.Styles.SecondaryTextColor))
		infoTable.SetCell(i, 1, tview.NewTableCell(values[i]).SetTextColor(tview.Styles.PrimaryTextColor).SetAttributes(tcell.AttrBold))
	}

	logoView := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter).
		SetText("[aqua]" + Logo + "[white]")

	statusView := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignRight)

	statusText := fmt.Sprintf("\n\n[yellow]LLM Status: [white]%s", aiStatus)
	if aiStatus == "Online" {
		statusText = fmt.Sprintf("\n\n[yellow]LLM Status: [green]● %s", aiStatus)
	} else if aiStatus == "Offline" || aiStatus == "Error" {
		statusText = fmt.Sprintf("\n\n[yellow]LLM Status: [red]● %s", aiStatus)
	}
	statusView.SetText(statusText)

	breadcrumb := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter)

	breadcrumbText := fmt.Sprintf("[blue]%s [white]> [green]%s [white]> [yellow]%s", context, namespace, resource)
	breadcrumb.SetText(breadcrumbText)

	innerFlex := tview.NewFlex()
	if w > 120 {
		innerFlex.AddItem(infoTable, 0, 1, false).
			AddItem(logoView, 0, 1, false).
			AddItem(statusView, 0, 1, false)
	} else if w > 80 {
		innerFlex.AddItem(infoTable, 0, 2, false).
			AddItem(statusView, 0, 1, false)
	} else {
		innerFlex.AddItem(infoTable, 0, 1, false)
	}

	header := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(innerFlex, 0, 1, false).
		AddItem(breadcrumb, 1, 0, false)

	return header
}

func NewShortcutBar() *tview.TextView {
	bar := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft).
		SetText(" [yellow]:[white]command  [yellow]l[white]logs  [yellow]d[white]describe  [yellow]s[white]settings  [yellow]?[white]help  [yellow]ctrl-c[white]quit")
	return bar
}

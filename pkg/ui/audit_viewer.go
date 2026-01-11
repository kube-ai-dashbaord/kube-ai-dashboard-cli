package ui

import (
	"fmt"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/kube-ai-dashbaord/kube-ai-dashboard-cli/pkg/db"
	"github.com/rivo/tview"
)

type AuditViewer struct {
	Table *tview.Table
	App   *tview.Application
}

func NewAuditViewer(app *tview.Application) *AuditViewer {
	t := tview.NewTable().SetSelectable(true, false).SetSeparator('|')
	t.SetBorder(true).SetTitle(" Audit Logs ")

	v := &AuditViewer{
		Table: t,
		App:   app,
	}

	return v
}

func (v *AuditViewer) Refresh() {
	logs, err := db.GetAuditLogs()
	v.Table.Clear()

	// Headers
	headers := []string{"TIME", "USER", "ACTION", "RESOURCE", "DETAILS"}
	for i, h := range headers {
		v.Table.SetCell(0, i, tview.NewTableCell(h).SetTextColor(tcell.ColorYellow).SetAttributes(tcell.AttrBold))
	}

	if err != nil {
		v.Table.SetCell(1, 0, tview.NewTableCell(fmt.Sprintf("Error: %v", err)).SetTextColor(tcell.ColorRed))
		return
	}

	for r, log := range logs {
		ts := log["timestamp"].(time.Time).Format("15:04:05")
		user := log["user"].(string)
		action := log["action"].(string)
		resource := log["resource"].(string)
		details := log["details"].(string)

		v.Table.SetCell(r+1, 0, tview.NewTableCell(ts))
		v.Table.SetCell(r+1, 1, tview.NewTableCell(user))
		v.Table.SetCell(r+1, 2, tview.NewTableCell(action).SetTextColor(v.getActionColor(action)))
		v.Table.SetCell(r+1, 3, tview.NewTableCell(resource))
		v.Table.SetCell(r+1, 4, tview.NewTableCell(details))
	}
}

func (v *AuditViewer) getActionColor(action string) tcell.Color {
	switch action {
	case "DELETE":
		return tcell.ColorRed
	case "TOOL_CALL":
		return tcell.ColorAqua
	case "CHAT":
		return tcell.ColorGreen
	default:
		return tcell.ColorWhite
	}
}

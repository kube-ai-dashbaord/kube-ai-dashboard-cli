package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type Help struct {
	Table *tview.Table
}

func NewHelp(onClose func()) *Help {
	table := tview.NewTable().
		SetBorders(true).
		SetSelectable(false, false)

	shortcuts := [][]string{
		{":<resource>", "Switch to resource (pods, nodes, svc, deploy, etc.)"},
		{":ns", "List namespaces (then type name while in ns list to switch)"},
		{"l", "AI-powered log analysis for selected resource"},
		{"d", "AI-powered description of selected resource"},
		{"s", "Open LLM Settings and configuration"},
		{"?", "Show this help screen"},
		{"esc", "Close current view or command bar"},
		{"ctrl-c", "Exit the application"},
	}

	table.SetCell(0, 0, tview.NewTableCell("Key").SetTextColor(tcell.ColorYellow).SetAttributes(tcell.AttrBold))
	table.SetCell(0, 1, tview.NewTableCell("Description").SetTextColor(tcell.ColorYellow).SetAttributes(tcell.AttrBold))

	for i, s := range shortcuts {
		table.SetCell(i+1, 0, tview.NewTableCell(s[0]).SetTextColor(tcell.ColorAqua))
		table.SetCell(i+1, 1, tview.NewTableCell(s[1]).SetTextColor(tcell.ColorWhite))
	}

	table.SetBorder(true).SetTitle(" k15s Shortcuts & Help ")

	table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape || event.Rune() == '?' {
			onClose()
			return nil
		}
		return event
	})

	return &Help{Table: table}
}

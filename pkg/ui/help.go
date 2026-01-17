package ui

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/kube-ai-dashbaord/kube-ai-dashboard-cli/pkg/i18n"
	"github.com/rivo/tview"
)

type Help struct {
	Table *tview.Table
}

func NewHelp(onClose func()) *Help {
	table := tview.NewTable().
		SetBorders(false).
		SetSelectable(false, false).
		SetFixed(1, 1)

	type shortcut struct {
		key  string
		desc string
	}

	categories := []struct {
		name      string
		shortcuts []shortcut
	}{
		{
			i18n.T("cat_nav"),
			[]shortcut{
				{"j/k, ↑/↓", i18n.T("desc_move_row")},
				{"g", i18n.T("desc_goto_top")},
				{"G", i18n.T("desc_goto_bottom")},
				{"Ctrl-U", i18n.T("desc_page_up")},
				{"Ctrl-F", i18n.T("desc_page_down")},
				{":<res>", i18n.T("desc_switch")},
				{":view <t> <n> [ns]", "Focus a specific resource (Synergy)"},
				{":ctx", i18n.T("desc_ctx")},
				{":audit", i18n.T("desc_audit")},
				{"/<query>", i18n.T("desc_filter")},
				{"/regex/", i18n.T("desc_regex_filter")},
				{"Esc", i18n.T("desc_clear_filter")},
				{"s", i18n.T("shortcut_settings")},
				{"?", i18n.T("shortcut_help")},
			},
		},
		{
			i18n.T("cat_selection"),
			[]shortcut{
				{"Space", i18n.T("desc_toggle_select")},
				{"Ctrl-Space", i18n.T("desc_clear_select")},
			},
		},
		{
			i18n.T("cat_res"),
			[]shortcut{
				{"y", i18n.T("desc_yaml")},
				{"e", i18n.T("desc_edit")},
				{"d", i18n.T("desc_describe")},
				{"L", i18n.T("desc_analyze")},
				{"h", i18n.T("desc_explain")},
				{"s", i18n.T("desc_shell")},
				{"S", i18n.T("desc_scale")},
				{"r", i18n.T("desc_restart")},
				{"Shift-F", i18n.T("desc_forward")},
				{"l", i18n.T("desc_logs")},
				{"Ctrl-D", i18n.T("desc_delete")},
			},
		},
	}

	row := 0
	for _, cat := range categories {
		table.SetCell(row, 0, tview.NewTableCell(" "+cat.name+" ").SetTextColor(tcell.ColorYellow).SetAttributes(tcell.AttrBold))
		table.SetCell(row, 1, tview.NewTableCell(""))
		row++
		for _, s := range cat.shortcuts {
			table.SetCell(row, 0, tview.NewTableCell("   "+s.key).SetTextColor(tcell.ColorAqua))
			table.SetCell(row, 1, tview.NewTableCell(s.desc).SetTextColor(tcell.ColorWhite))
			row++
		}
		row++ // spacer
	}

	table.SetBorder(true).SetTitle(fmt.Sprintf(" %s ", i18n.T("help_title"))).SetTitleAlign(tview.AlignCenter)

	table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape || event.Rune() == '?' {
			onClose()
			return nil
		}
		return event
	})

	return &Help{Table: table}
}

package ui

import (
	"context"
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/kube-ai-dashbaord/kube-ai-dashboard-cli/pkg/i18n"
	"github.com/kube-ai-dashbaord/kube-ai-dashboard-cli/pkg/k8s"
	"github.com/kube-ai-dashbaord/kube-ai-dashboard-cli/pkg/ui/resources"
	"github.com/rivo/tview"
)

type PulseViewer struct {
	Table *tview.Table
	K8s   *k8s.Client
	App   *tview.Application
}

func NewPulseViewer(app *tview.Application, k8sClient *k8s.Client, onBack func()) *PulseViewer {
	table := tview.NewTable().SetSelectable(true, false).SetSeparator('|')
	table.SetBorder(true).SetTitle(fmt.Sprintf(" %s ", i18n.T("pulse_events")))

	table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			if onBack != nil {
				onBack()
			}
			return nil
		}
		return event
	})

	pv := &PulseViewer{
		Table: table,
		K8s:   k8sClient,
		App:   app,
	}
	return pv
}

func (pv *PulseViewer) Refresh() {
	pv.Table.Clear()
	pv.Table.SetCell(0, 0, tview.NewTableCell(i18n.T("loading")).SetTextColor(tview.Styles.SecondaryTextColor))

	go func() {
		ctx := context.Background()
		view, err := resources.GetEventsView(ctx, pv.K8s, "", "") // All namespaces
		pv.App.QueueUpdateDraw(func() {
			pv.Table.Clear()
			if err != nil {
				pv.Table.SetCell(0, 0, tview.NewTableCell(fmt.Sprintf("Error: %v", err)).SetTextColor(tview.Styles.ContrastBackgroundColor))
				return
			}

			for i, h := range view.Headers {
				pv.Table.SetCell(0, i, tview.NewTableCell(h).SetTextColor(tview.Styles.SecondaryTextColor).SetAttributes(tcell.AttrBold).SetSelectable(false))
			}

			for r, row := range view.Rows {
				for i, cell := range row {
					pv.Table.SetCell(r+1, i, tview.NewTableCell(cell.Text).SetTextColor(cell.Color))
				}
			}
		})
	}()
}

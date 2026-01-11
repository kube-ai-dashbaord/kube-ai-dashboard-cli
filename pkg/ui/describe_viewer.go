package ui

import (
	"context"
	"fmt"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/kube-ai-dashbaord/kube-ai-dashboard-cli/pkg/k8s"
	"github.com/rivo/tview"
)

type DescribeViewer struct {
	View *tview.TextView
	K8s  *k8s.Client
}

func NewDescribeViewer(app *tview.Application, k8sClient *k8s.Client, onExit func()) *DescribeViewer {
	d := &DescribeViewer{
		View: tview.NewTextView().
			SetDynamicColors(true).
			SetRegions(true).
			SetWordWrap(true),
		K8s: k8sClient,
	}

	d.View.SetBorder(true).SetTitle(" Describe View (ESC to exit) ")
	d.View.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape || event.Rune() == 'q' {
			onExit()
			return nil
		}
		return event
	})

	return d
}

func (d *DescribeViewer) Show(res, ns, name string) {
	d.View.SetTitle(fmt.Sprintf(" Describe: %s %s/%s ", res, ns, name))
	d.View.SetText("Loading details...")

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		content, err := d.K8s.GetResourceContext(ctx, ns, name, res)
		if err != nil {
			d.View.SetText(fmt.Sprintf("[red]Error describing resource: %v", err))
			return
		}

		// Simple highlighting for Describe-like output
		// GetResourceContext returns a mix of YAML and text.
		d.View.SetText(content)
		d.View.ScrollToBeginning()
	}()
}

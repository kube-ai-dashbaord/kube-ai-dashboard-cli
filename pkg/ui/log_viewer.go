package ui

import (
	"context"
	"fmt"
	"io"

	"github.com/gdamore/tcell/v2"
	"github.com/kube-ai-dashbaord/kube-ai-dashboard-cli/pkg/k8s"
	"github.com/rivo/tview"
)

type LogViewer struct {
	View   *tview.TextView
	App    *tview.Application
	K8s    *k8s.Client
	Cancel context.CancelFunc
}

func NewLogViewer(app *tview.Application, k8sClient *k8s.Client, onExit func()) *LogViewer {
	view := tview.NewTextView().
		SetDynamicColors(true).
		SetRegions(true).
		SetWordWrap(true).
		SetChangedFunc(func() {
			app.Draw()
		})

	view.SetBorder(true).SetTitle(" Logs ")
	view.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape || event.Rune() == 'q' {
			onExit()
			return nil
		}
		return event
	})

	return &LogViewer{
		View: view,
		App:  app,
		K8s:  k8sClient,
	}
}

func (l *LogViewer) StreamLogs(namespace, name string) {
	ctx, cancel := context.WithCancel(context.Background())
	l.Cancel = cancel

	l.View.SetTitle(fmt.Sprintf(" Logs (%s/%s) ", namespace, name))
	l.View.Clear()

	go func() {
		stream, err := l.K8s.GetPodLogsStream(ctx, namespace, name)
		if err != nil {
			fmt.Fprintf(l.View, "[red]Error opening log stream: %v[white]\n", err)
			return
		}
		defer stream.Close()

		go func() {
			<-ctx.Done()
			stream.Close()
		}()

		buf := make([]byte, 4096)
		for {
			n, err := stream.Read(buf)
			if n > 0 {
				fmt.Fprint(l.View, tview.Escape(string(buf[:n])))
				l.App.QueueUpdateDraw(func() {
					l.View.ScrollToEnd()
				})
			}
			if err != nil {
				if err != io.EOF {
					fmt.Fprintf(l.View, "\n[red]Stream error: %v[white]\n", err)
				}
				break
			}
		}
	}()
}

func (l *LogViewer) Stop() {
	if l.Cancel != nil {
		l.Cancel()
	}
}

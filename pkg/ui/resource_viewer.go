package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/kube-ai-dashbaord/kube-ai-dashboard-cli/pkg/i18n"
	"github.com/kube-ai-dashbaord/kube-ai-dashboard-cli/pkg/k8s"
	"github.com/rivo/tview"
)

const (
	TabDescribe = 0
	TabYAML     = 1
	TabEvents   = 2
)

type ResourceViewer struct {
	Root      *tview.Flex
	Tabs      *tview.TextView
	Content   *tview.TextView
	K8s       *k8s.Client
	App       *tview.Application
	ActiveTab int
	Namespace string
	Name      string
	Resource  string
	OnExit    func()
}

func NewResourceViewer(app *tview.Application, k8sClient *k8s.Client, onExit func()) *ResourceViewer {
	r := &ResourceViewer{
		Root:    tview.NewFlex().SetDirection(tview.FlexRow),
		Tabs:    tview.NewTextView().SetDynamicColors(true).SetRegions(true).SetWrap(false),
		Content: tview.NewTextView().SetDynamicColors(true).SetRegions(true).SetWordWrap(true),
		K8s:     k8sClient,
		App:     app,
		OnExit:  onExit,
	}

	r.Tabs.SetTextAlign(tview.AlignLeft).SetBorder(false)
	r.Content.SetBorder(true)

	r.Root.AddItem(r.Tabs, 1, 0, false)
	r.Root.AddItem(r.Content, 0, 1, true)

	r.Root.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape || event.Rune() == 'q' {
			r.OnExit()
			return nil
		}
		if event.Key() == tcell.KeyTab {
			r.ActiveTab = (r.ActiveTab + 1) % 3
			r.refresh()
			return nil
		}
		if event.Key() == tcell.KeyBacktab {
			r.ActiveTab = (r.ActiveTab - 1 + 3) % 3
			r.refresh()
			return nil
		}
		switch event.Rune() {
		case '1', 'd':
			r.ActiveTab = TabDescribe
			r.refresh()
		case '2', 'y':
			r.ActiveTab = TabYAML
			r.refresh()
		case '3', 'e':
			r.ActiveTab = TabEvents
			r.refresh()
		}
		return event
	})

	return r
}

func (r *ResourceViewer) Show(res, ns, name string) {
	r.Resource = res
	r.Namespace = ns
	r.Name = name
	r.ActiveTab = TabDescribe
	r.refresh()
}

func (r *ResourceViewer) refresh() {
	r.updateTabs()
	r.Content.SetText(i18n.T("loading"))

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		var text string
		var err error

		switch r.ActiveTab {
		case TabDescribe:
			r.Content.SetTitle(fmt.Sprintf(" %s: %s %s/%s ", i18n.T("tab_describe"), r.Resource, r.Namespace, r.Name))
			text, err = r.K8s.GetResourceContext(ctx, r.Namespace, r.Name, r.Resource)
		case TabYAML:
			r.Content.SetTitle(fmt.Sprintf(" %s: %s %s/%s ", i18n.T("tab_yaml"), r.Resource, r.Namespace, r.Name))
			gvr, ok := r.K8s.GetGVR(r.Resource)
			if !ok {
				err = fmt.Errorf("unknown resource: %s", r.Resource)
			} else {
				text, err = r.K8s.GetResourceYAML(ctx, r.Namespace, r.Name, gvr)
				if err == nil {
					text = r.highlightYAML(text)
				}
			}
		case TabEvents:
			r.Content.SetTitle(fmt.Sprintf(" %s: %s %s/%s ", i18n.T("tab_events"), r.Resource, r.Namespace, r.Name))
			events, eventErr := r.K8s.ListEvents(ctx, r.Namespace)
			if eventErr != nil {
				err = eventErr
			} else {
				var sb strings.Builder
				found := false
				for _, ev := range events {
					if ev.InvolvedObject.Name == r.Name || strings.Contains(ev.Message, r.Name) {
						sb.WriteString(fmt.Sprintf("[yellow]%s [white]%s: %s\n", ev.LastTimestamp.Format("15:04:05"), ev.Reason, ev.Message))
						found = true
					}
				}
				if !found {
					sb.WriteString("No events found")
				}
				text = sb.String()
			}
		}

		r.App.QueueUpdateDraw(func() {
			if err != nil {
				r.Content.SetText(fmt.Sprintf("[red]Error: %v", err))
			} else {
				r.Content.SetText(text)
				r.Content.ScrollToBeginning()
			}
		})
	}()
}

func (r *ResourceViewer) updateTabs() {
	tabs := []string{
		fmt.Sprintf(" [1] %s ", i18n.T("tab_describe")),
		fmt.Sprintf(" [2] %s ", i18n.T("tab_yaml")),
		fmt.Sprintf(" [3] %s ", i18n.T("tab_events")),
	}

	var sb strings.Builder
	for i, tab := range tabs {
		if i == r.ActiveTab {
			sb.WriteString(fmt.Sprintf(" [black:yellow:b]%s[white:black:-] ", strings.TrimSpace(tab)))
		} else {
			sb.WriteString(fmt.Sprintf(" [white:black]%s ", strings.TrimSpace(tab)))
		}
	}
	r.Tabs.SetText(sb.String())
}

func (r *ResourceViewer) highlightYAML(raw string) string {
	lines := strings.Split(raw, "\n")
	var highlighted strings.Builder
	for _, line := range lines {
		if strings.Contains(line, ":") {
			parts := strings.SplitN(line, ":", 2)
			highlighted.WriteString(fmt.Sprintf("[yellow]%s:[white]%s\n", parts[0], parts[1]))
		} else {
			highlighted.WriteString(line + "\n")
		}
	}
	return highlighted.String()
}

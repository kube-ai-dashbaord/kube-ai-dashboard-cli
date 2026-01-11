package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/kube-ai-dashbaord/kube-ai-dashboard-cli/pkg/k8s"
	"github.com/rivo/tview"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

type YamlViewer struct {
	View *tview.TextView
	K8s  *k8s.Client
}

func NewYamlViewer(app *tview.Application, k8sClient *k8s.Client, onExit func()) *YamlViewer {
	y := &YamlViewer{
		View: tview.NewTextView().
			SetDynamicColors(true).
			SetRegions(true).
			SetWordWrap(true),
		K8s: k8sClient,
	}

	y.View.SetBorder(true).SetTitle(" YAML View (ESC to exit) ")
	y.View.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape || event.Rune() == 'q' {
			onExit()
			return nil
		}
		return event
	})

	return y
}

func (y *YamlViewer) Show(res, ns, name string) {
	gvr, ok := y.K8s.GetGVR(res)
	if !ok {
		y.View.SetText(fmt.Sprintf("[red]Unknown resource type: %s", res))
		return
	}

	y.View.SetTitle(fmt.Sprintf(" YAML: %s %s/%s ", res, ns, name))
	y.View.SetText("Loading...")

	go func() {
		obj, err := y.K8s.Dynamic.Resource(gvr).Namespace(ns).Get(context.Background(), name, metav1.GetOptions{})
		if err != nil {
			y.View.SetText(fmt.Sprintf("[red]Error fetching resource: %v", err))
			return
		}

		// Remove managed fields for cleaner view
		obj.SetManagedFields(nil)

		buf, err := json.Marshal(obj.Object)
		if err != nil {
			y.View.SetText(fmt.Sprintf("[red]Error encoding resource: %v", err))
			return
		}

		yText, err := yaml.JSONToYAML(buf)
		if err != nil {
			y.View.SetText(fmt.Sprintf("[red]Error converting to YAML: %v", err))
			return
		}

		// Highlight keys (simple approach)
		lines := strings.Split(string(yText), "\n")
		var highlighted strings.Builder
		for _, line := range lines {
			if strings.Contains(line, ":") {
				parts := strings.SplitN(line, ":", 2)
				highlighted.WriteString(fmt.Sprintf("[yellow]%s:[white]%s\n", parts[0], parts[1]))
			} else {
				highlighted.WriteString(line + "\n")
			}
		}

		y.View.SetText(highlighted.String())
		y.View.ScrollToBeginning()
	}()
}

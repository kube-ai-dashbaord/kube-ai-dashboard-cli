package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type CommandBar struct {
	Input     *tview.InputField
	OnCommand func(string)
	OnFilter  func(string)
}

func NewCommandBar(onCommand func(string), onFilter func(string)) *CommandBar {
	c := &CommandBar{
		Input: tview.NewInputField().
			SetFieldBackgroundColor(tcell.ColorBlack).
			SetLabelColor(tcell.ColorYellow),
		OnCommand: onCommand,
		OnFilter:  onFilter,
	}

	resources := []string{
		"pods", "po", "nodes", "no", "deployments", "deploy",
		"services", "svc", "namespaces", "ns", "configmaps", "cm",
		"secrets", "ingresses", "ing", "statefulsets", "sts",
		"daemonsets", "ds", "jobs", "cronjobs", "cj",
		"hpa", "horizontalpodautoscalers", "networkpolicies", "netpol",
		"roles", "rolebindings", "rb", "clusterroles", "clusterrolebindings", "crb",
		"persistentvolumes", "pv", "persistentvolumeclaims", "pvc",
		"storageclasses", "sc", "serviceaccounts", "sa",
		"events", "ev", "contexts", "ctx",
	}

	c.Input.SetAutocompleteFunc(func(currentText string) (entries []string) {
		if len(currentText) == 0 {
			return nil
		}
		for _, res := range resources {
			if len(res) >= len(currentText) && res[:len(currentText)] == currentText {
				entries = append(entries, res)
			}
		}
		return entries
	})

	c.Input.SetDoneFunc(func(key tcell.Key) {
		text := c.Input.GetText()
		if key == tcell.KeyEnter {
			if c.Input.GetLabel() == "/" {
				if c.OnFilter != nil {
					c.OnFilter(text)
				}
			} else {
				if c.OnCommand != nil {
					c.OnCommand(text)
				}
			}
			c.Input.SetText("")
		} else if key == tcell.KeyEscape {
			if c.Input.GetLabel() == "/" && c.OnFilter != nil {
				c.OnFilter("") // Clear filter on Esc if in filter mode
			}
			c.Input.SetText("")
		}
	})

	return c
}

func (c *CommandBar) Show(prefix string) {
	c.Input.SetLabel(prefix)
	c.Input.SetText("")
}

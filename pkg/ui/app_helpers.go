package ui

import "github.com/rivo/tview"

func (a *App) FormGetText(form *tview.Form, label string) string {
	item := form.GetFormItemByLabel(label)
	if item == nil {
		return ""
	}
	if input, ok := item.(*tview.InputField); ok {
		return input.GetText()
	}
	return ""
}

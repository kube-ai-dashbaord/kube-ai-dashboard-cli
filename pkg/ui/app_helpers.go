package ui

import "github.com/rivo/tview"

func (a *App) FormGetText(form *tview.Form, label string) string {
	for i := 0; i < form.GetFormItemCount(); i++ {
		item := form.GetFormItem(i)
		if item.GetLabel() == label {
			if input, ok := item.(*tview.InputField); ok {
				return input.GetText()
			}
		}
	}
	return ""
}

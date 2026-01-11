package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type CommandBar struct {
	Input *tview.InputField
}

func NewCommandBar(onDone func(cmd string)) *CommandBar {
	c := &CommandBar{
		Input: tview.NewInputField().
			SetLabel(":").
			SetFieldBackgroundColor(tcell.ColorBlack).
			SetLabelColor(tcell.ColorYellow),
	}

	c.Input.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter {
			onDone(c.Input.GetText())
			c.Input.SetText("")
		} else if key == tcell.KeyEscape {
			onDone("")
			c.Input.SetText("")
		}
	})

	return c
}

package ui

import (
	"github.com/rivo/tview"
)

const Logo = `
  _  _    __  _____     
 | || |  /  || ____| ___ 
 | || |_ ` + "`" + `| || |__  / __|
 |__   _| | ||___ \ \__ \
    |_|   |_||____/ |___/
    Kubernetes AI Dashboard
`

func NewHeader() *tview.TextView {
	header := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter).
		SetText("[aqua]" + Logo + "[white]")
	return header
}

func NewShortcutBar() *tview.TextView {
	bar := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft).
		SetText(" [yellow]:[white]command  [yellow]l[white]logs  [yellow]d[white]describe  [yellow]s[white]settings  [yellow]?[white]help  [yellow]ctrl-c[white]quit")
	return bar
}

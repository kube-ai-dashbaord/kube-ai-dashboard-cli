package ui

import (
	"context"
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/kube-ai-dashbaord/kube-ai-dashboard-cli/pkg/ai"
	"github.com/rivo/tview"
)

type Assistant struct {
	App             *tview.Application
	Root            *tview.Flex
	Chat            *tview.TextView
	Input           *tview.InputField
	AI              *ai.Client
	SelectedContext string
}

func NewAssistant(app *tview.Application, aiClient *ai.Client) *Assistant {
	a := &Assistant{
		App: app,
		AI:  aiClient,
		Chat: tview.NewTextView().
			SetDynamicColors(true).
			SetRegions(true).
			SetWordWrap(true),
		Input: tview.NewInputField().
			SetLabel("Ask AI: "),
	}

	a.Chat.SetBorder(true).SetTitle(" AI Assistant ")
	a.Input.SetBorder(true)

	a.Input.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter {
			text := a.Input.GetText()
			if text == "" {
				return
			}
			a.Input.SetText("")
			a.AppendChat("User", text)
			go a.ProcessAI(text)
		}
	})

	flex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(a.Chat, 0, 1, false).
		AddItem(a.Input, 3, 1, true)

	a.Root = flex
	return a
}

func (a *Assistant) AppendChat(sender, text string) {
	fmt.Fprintf(a.Chat, "[yellow]%s: [white]%s\n", sender, text)
	a.Chat.ScrollToEnd()
}

func (a *Assistant) ProcessAI(prompt string) {
	if a.AI == nil {
		a.AppendChat("System", "AI Client not initialized. Please check your settings.")
		return
	}

	a.AppendChat("Assistant", "") // Placeholder for assistant response

	fullPrompt := prompt
	if a.SelectedContext != "" {
		fullPrompt = fmt.Sprintf("[%s]\n%s", a.SelectedContext, prompt)
	}

	err := a.AI.Ask(context.Background(), fullPrompt, func(text string) {
		a.App.QueueUpdateDraw(func() {
			fmt.Fprintf(a.Chat, "%s", text)
			a.Chat.ScrollToEnd()
		})
	})

	if err != nil {
		a.AppendChat("Error", err.Error())
	} else {
		fmt.Fprintf(a.Chat, "\n")
	}
}

func (a *Assistant) SetContext(ctx string) {
	a.SelectedContext = ctx
	a.AppendChat("System", fmt.Sprintf("Context set: %s", ctx))
}

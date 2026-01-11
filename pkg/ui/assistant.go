package ui

import (
	"fmt"
	"time"

	"github.com/GoogleCloudPlatform/kubectl-ai/pkg/agent"
	"github.com/GoogleCloudPlatform/kubectl-ai/pkg/api"
	"github.com/gdamore/tcell/v2"
	"github.com/kube-ai-dashbaord/kube-ai-dashboard-cli/pkg/ai"
	"github.com/kube-ai-dashbaord/kube-ai-dashboard-cli/pkg/db"
	"github.com/kube-ai-dashbaord/kube-ai-dashboard-cli/pkg/i18n"
	"github.com/rivo/tview"
)

type Assistant struct {
	App             *tview.Application
	Reporter        *ai.Reporter
	Root            *tview.Flex
	Chat            *tview.TextView
	Input           *tview.InputField
	ChoiceList      *tview.List
	Agent           *agent.Agent
	SelectedContext string
}

func NewAssistant(app *tview.Application, ag *agent.Agent, reporter *ai.Reporter) *Assistant {
	a := &Assistant{
		App:      app,
		Agent:    ag,
		Reporter: reporter,
		Chat: tview.NewTextView().
			SetDynamicColors(true).
			SetRegions(true).
			SetWordWrap(true).
			SetScrollable(true),
		Input: tview.NewInputField().
			SetLabel(i18n.T("ask_ai")),
		ChoiceList: tview.NewList().
			SetSelectedBackgroundColor(tcell.ColorDarkBlue),
	}

	a.Chat.SetBorder(true).SetTitle(fmt.Sprintf(" %s (Agentic) ", i18n.T("app_title")))
	a.Input.SetBorder(true)
	a.ChoiceList.SetBorder(true).SetTitle(fmt.Sprintf(" %s ", i18n.T("decision_required")))

	a.Input.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter {
			text := a.Input.GetText()
			if text == "" {
				return
			}
			a.Input.SetText("")

			if a.Agent != nil {
				query := text
				if a.SelectedContext != "" {
					query = fmt.Sprintf("[Context: %s] %s", a.SelectedContext, text)
				}
				Infof("User query sent to AI Agent: %s", query)
				a.Agent.Input <- &api.UserInputResponse{Query: query}
			}
		}
	})

	a.Input.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyUp {
			a.App.SetFocus(a.Chat)
			return nil
		}
		return event
	})

	a.Chat.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEnter || event.Key() == tcell.KeyEscape {
			a.App.SetFocus(a.Input)
			return nil
		}
		// Allow scrolling via Up/Down/PgUp/PgDn
		switch event.Key() {
		case tcell.KeyUp:
			row, col := a.Chat.GetScrollOffset()
			if row > 0 {
				a.Chat.ScrollTo(row-1, col)
			}
			return nil
		case tcell.KeyDown:
			row, col := a.Chat.GetScrollOffset()
			a.Chat.ScrollTo(row+1, col)
			return nil
		case tcell.KeyPgUp:
			row, col := a.Chat.GetScrollOffset()
			a.Chat.ScrollTo(row-10, col)
			return nil
		case tcell.KeyPgDn:
			row, col := a.Chat.GetScrollOffset()
			a.Chat.ScrollTo(row+10, col)
			return nil
		}
		return event
	})

	// Listen for agent output
	if a.Agent != nil {
		go func() {
			for msg := range a.Agent.Output {
				a.handleAgentMessage(msg)
			}
		}()
	}

	a.Root = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(a.Chat, 0, 1, false).
		AddItem(a.Input, 3, 1, true)

	return a
}

func (a *Assistant) handleAgentMessage(msg any) {
	a.App.QueueUpdateDraw(func() {
		switch m := msg.(type) {
		case *api.Message:
			switch m.Type {
			case api.MessageTypeText:
				sender := "AI"
				if m.Source == api.MessageSourceUser {
					sender = "User"
				}
				fmt.Fprintf(a.Chat, "\n[yellow]%s: [white]%s\n", sender, m.Payload)
				Infof("AI Chat Message: %s: %s", sender, m.Payload)
				db.RecordAudit(db.AuditEntry{
					User:    sender,
					Action:  "CHAT",
					Details: fmt.Sprintf("%v", m.Payload),
				})
			case api.MessageTypeTextChunk:
				fmt.Fprintf(a.Chat, "%v", m.Payload)
			case api.MessageTypeError:
				fmt.Fprintf(a.Chat, "\n[red]Error: [white]%v\n", m.Payload)
			case api.MessageTypeToolCallRequest:
				desc := fmt.Sprintf("%v", m.Payload)
				fmt.Fprintf(a.Chat, "\n[aqua]Action: [white]Running %s\n", desc)
				Infof("AI Tool Call Request received: %s", desc)
				db.RecordAudit(db.AuditEntry{
					User:    "AI",
					Action:  "TOOL_CALL",
					Details: desc,
				})
				if a.Reporter != nil {
					a.Reporter.Record(ai.Action{
						Timestamp: time.Now(),
						Operation: "TOOL_CALL",
						Details:   desc,
					})
				}
			case api.MessageTypeUserChoiceRequest:
				req := m.Payload.(*api.UserChoiceRequest)
				a.showChoiceUI(req)
			}
		}
		a.Chat.ScrollToEnd()
	})
}

func (a *Assistant) showChoiceUI(req *api.UserChoiceRequest) {
	a.ChoiceList.Clear()
	for i, opt := range req.Options {
		idx := i + 1
		a.ChoiceList.AddItem(opt.Label, opt.Value, 0, func() {
			a.Agent.Input <- &api.UserChoiceResponse{Choice: idx}
			a.hideChoiceUI()
		})
	}
	// Add an option to cancel/decline if not already there or as a standard
	a.ChoiceList.AddItem("Cancel", "Decline this action", 'q', func() {
		// Assuming choice 3 is 'no' as per common Agent logic, but we should be careful.
		// Usually handleChoice handles this.
		a.Agent.Input <- &api.UserChoiceResponse{Choice: 3}
		a.hideChoiceUI()
	})

	// Swap Input with ChoiceList
	a.Root.RemoveItem(a.Input)
	a.Root.AddItem(a.ChoiceList, 10, 1, true)
	a.App.SetFocus(a.ChoiceList)
}

func (a *Assistant) hideChoiceUI() {
	a.Root.RemoveItem(a.ChoiceList)
	a.Root.AddItem(a.Input, 3, 1, true)
	a.App.SetFocus(a.Input)
}

func (a *Assistant) AppendChat(sender, message string) {
	a.App.QueueUpdateDraw(func() {
		fmt.Fprintf(a.Chat, "[yellow]%s: [white]%s\n", sender, message)
		a.Chat.ScrollToEnd()
	})
}

func (a *Assistant) SendMessage(message string) {
	if a.Agent != nil {
		a.Agent.Input <- &api.UserInputResponse{Query: message}
	}
}

func (a *Assistant) SetContext(context string) {
	a.SelectedContext = context
	a.App.QueueUpdateDraw(func() {
		a.Chat.SetTitle(fmt.Sprintf(" AI Assistant (Context: %s) ", context))
	})
}

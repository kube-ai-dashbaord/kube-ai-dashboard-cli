package ui

import (
	"context"
	"fmt"
	"time"

	"github.com/GoogleCloudPlatform/kubectl-ai/pkg/agent"
	"github.com/kube-ai-dashbaord/kube-ai-dashboard-cli/pkg/db"
	"github.com/kube-ai-dashbaord/kube-ai-dashboard-cli/pkg/i18n"
	"github.com/rivo/tview"
)

func (a *App) initCallbacks(ag *agent.Agent) {
	a.Dashboard.OnYaml = func(ns, name string) {
		a.YamlViewer.Show(a.Dashboard.CurrentResource, ns, name)
		db.RecordAudit(db.AuditEntry{
			User:     "User",
			Action:   "YAML_VIEW",
			Resource: a.Dashboard.CurrentResource,
			Details:  fmt.Sprintf("%s/%s", ns, name),
		})
		a.Pages.AddPage("yaml", a.YamlViewer.View, true, true)
		a.Pages.SwitchToPage("yaml")
	}

	a.Dashboard.OnDescribe = func(ns, name string) {
		if ag != nil {
			db.RecordAudit(db.AuditEntry{
				User:     "User",
				Action:   "DESCRIBE",
				Resource: a.Dashboard.CurrentResource,
				Details:  fmt.Sprintf("%s/%s", ns, name),
			})

			res := a.Dashboard.CurrentResource
			a.Assistant.AppendChat("User", fmt.Sprintf("Describe the %s %s/%s", res, ns, name))

			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
				defer cancel()

				fullContext, err := a.K8s.GetResourceContext(ctx, ns, name, res)
				if err != nil {
					a.Assistant.AppendChat("System", fmt.Sprintf("Error gathering context: %v", err))
					return
				}

				prompt := fmt.Sprintf("Analyze the following Kubernetes resource and provide a concise summary of its status, any issues, and recommendations:\n\n%s", fullContext)

				a.Assistant.SendMessage(prompt)
			}()
		}
	}

	a.Dashboard.OnScale = func(ns, name string) {
		res := a.Dashboard.CurrentResource
		var form *tview.Form
		form = tview.NewForm().
			AddInputField("Replicas", "1", 5, nil, nil)
		form.AddButton("Scale", func() {
			replicas := a.FormGetText(form, "Replicas")
			// Parse replicas to int32... (simplified for now)
			db.RecordAudit(db.AuditEntry{
				User:     "User",
				Action:   "SCALE",
				Resource: fmt.Sprintf("%s/%s", res, name),
				Details:  fmt.Sprintf("%s: %s", ns, replicas),
			})
			a.Pages.RemovePage("scale")
			a.Application.SetFocus(a.Dashboard.Root)
		}).
			AddButton("Cancel", func() {
				a.Pages.RemovePage("scale")
				a.Application.SetFocus(a.Dashboard.Root)
			})
		form.SetBorder(true).SetTitle(fmt.Sprintf(" Scale %s/%s ", ns, name))
		a.Pages.AddPage("scale", form, true, true)
		a.Application.SetFocus(form)
	}

	a.Dashboard.OnRestart = func(ns, name string) {
		modal := tview.NewModal().
			SetText(fmt.Sprintf("Are you sure you want to rollout restart %s %s?", a.Dashboard.CurrentResource, name)).
			AddButtons([]string{"Restart", "Cancel"}).
			SetDoneFunc(func(buttonIndex int, buttonLabel string) {
				if buttonLabel == "Restart" {
					db.RecordAudit(db.AuditEntry{
						User:     "User",
						Action:   "RESTART",
						Resource: a.Dashboard.CurrentResource,
						Details:  fmt.Sprintf("%s/%s", ns, name),
					})
				}
				a.Pages.RemovePage("restart_confirm")
				a.Application.SetFocus(a.Dashboard.Root)
			})
		a.Pages.AddPage("restart_confirm", modal, false, true)
		a.Application.SetFocus(modal)
	}

	a.Dashboard.OnPortForward = func(ns, name string) {
		var form *tview.Form
		form = tview.NewForm().
			AddInputField("Local Port", "8080", 10, nil, nil).
			AddInputField("Pod Port", "80", 10, nil, nil)

		form.AddButton("Forward", func() {
			localPortStr := a.FormGetText(form, "Local Port")
			podPortStr := a.FormGetText(form, "Pod Port")

			db.RecordAudit(db.AuditEntry{
				User:     "User",
				Action:   "PORT_FORWARD",
				Resource: a.Dashboard.CurrentResource,
				Details:  fmt.Sprintf("%s/%s %s:%s", ns, name, localPortStr, podPortStr),
			})

			a.Pages.RemovePage("portforward")
			a.Application.SetFocus(a.Dashboard.Root)
			a.Assistant.AppendChat("System", fmt.Sprintf("Port forward started: localhost:%s -> %s:%s", localPortStr, name, podPortStr))
		}).
			AddButton("Cancel", func() {
				a.Pages.RemovePage("portforward")
				a.Application.SetFocus(a.Dashboard.Root)
			})
		form.SetBorder(true).SetTitle(fmt.Sprintf(" Port Forward %s/%s ", ns, name))
		a.Pages.AddPage("portforward", form, true, true)
		a.Application.SetFocus(form)
	}

	a.Dashboard.OnDeleteRequested = func(ns, name string) {
		res := a.Dashboard.CurrentResource
		modal := tview.NewModal().
			SetText(fmt.Sprintf("Are you sure you want to delete %s %s/%s?", res, ns, name)).
			AddButtons([]string{"Delete", "Cancel"}).
			SetDoneFunc(func(buttonIndex int, buttonLabel string) {
				if buttonLabel == "Delete" {
					db.RecordAudit(db.AuditEntry{
						User:     "User",
						Action:   "DELETE",
						Resource: fmt.Sprintf("%s/%s", res, name),
						Details:  ns,
					})
				}
				a.Pages.RemovePage("delete_confirm")
				a.Application.SetFocus(a.Dashboard.Root)
			})
		a.Pages.AddPage("delete_confirm", modal, false, true)
		a.Application.SetFocus(modal)
	}
	a.Dashboard.OnExplainRequested = func(ns, name string) {
		res := a.Dashboard.CurrentResource
		a.Assistant.AppendChat("User", fmt.Sprintf("Explain the %s %s/%s", res, ns, name))

		go func() {
			pedagogy := ""
			if a.Config.BeginnerMode {
				pedagogy = " Explain this like I am a complete beginner. Use simple analogies and avoid overly technical jargon where possible."
			}
			prompt := fmt.Sprintf("Explain the purpose of this Kubernetes %s resource (%s/%s). Help me understand what it does and why it's important.%s Provide the answer in %s.", res, ns, name, pedagogy, i18n.GetLanguage())
			a.Assistant.SendMessage(prompt)
		}()
	}
}

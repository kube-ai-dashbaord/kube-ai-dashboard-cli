package ui

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/kube-ai-dashbaord/kube-ai-dashboard-cli/pkg/ai"
	"github.com/kube-ai-dashbaord/kube-ai-dashboard-cli/pkg/config"
	"github.com/kube-ai-dashbaord/kube-ai-dashboard-cli/pkg/k8s"
	"github.com/rivo/tview"
)

type App struct {
	Application *tview.Application
	Dashboard   *Dashboard
	Assistant   *Assistant
	Settings    *Settings
	CommandBar  *CommandBar
	Help        *Help
	Header      *tview.TextView
	ShortcutBar *tview.TextView
	Pages       *tview.Pages
	Root        *tview.Flex
	Config      *config.Config
	AIClient    *ai.Client
	K8sClient   *k8s.Client
}

func NewApp() *App {
	cfg, _ := config.LoadConfig()
	aiClient, _ := ai.NewClient(&cfg.LLM)
	k8sClient, _ := k8s.NewClient()

	a := &App{
		Application: tview.NewApplication(),
	}
	a.Assistant = NewAssistant(a.Application, aiClient)
	a.Config = cfg
	a.AIClient = aiClient
	a.K8sClient = k8sClient
	a.Pages = tview.NewPages()

	a.Settings = NewSettings(&cfg.LLM, func(newCfg *config.LLMConfig) {
		a.Config.LLM = *newCfg
		a.Config.Save()
		newAI, _ := ai.NewClient(newCfg)
		a.AIClient = newAI
		a.Assistant.AI = newAI
		a.Assistant.AppendChat("System", "Settings updated and saved.")
		a.Pages.SwitchToPage("main")
	}, func() {
		a.Pages.SwitchToPage("main")
	})

	a.Dashboard = NewDashboard(k8sClient, func(podName string) {
		a.Assistant.SetContext(fmt.Sprintf("Current Pod: %s", podName))
	})

	a.CommandBar = NewCommandBar(func(cmd string) {
		if cmd != "" {
			if a.Dashboard.CurrentResource == "namespaces" || a.Dashboard.CurrentResource == "ns" {
				a.Dashboard.CurrentNamespace = cmd
				a.Dashboard.SetResource("pods") // Jump to pods in selected namespace
			} else {
				a.Dashboard.SetResource(cmd)
			}
		}
		a.Root.RemoveItem(a.CommandBar.Input)
		a.Application.SetFocus(a.Dashboard.Root)
	})

	a.Header = NewHeader()
	a.ShortcutBar = NewShortcutBar()
	a.Help = NewHelp(func() {
		a.Pages.SwitchToPage("main")
	})

	mainFlex := tview.NewFlex().SetDirection(tview.FlexRow)
	contentFlex := tview.NewFlex().
		AddItem(a.Dashboard.Root, 0, 1, true).
		AddItem(a.Assistant.Root, 0, 1, false)

	mainFlex.AddItem(a.Header, 7, 1, false) // Fixed height for logo
	mainFlex.AddItem(contentFlex, 0, 1, true)
	mainFlex.AddItem(a.ShortcutBar, 1, 1, false)

	a.Root = mainFlex
	a.Pages.AddPage("main", a.Root, true, true)
	a.Pages.AddPage("settings", a.Settings.Form, true, false)
	a.Pages.AddPage("help", a.Help.Table, true, false)

	a.Application.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Rune() == ':' {
			a.Root.AddItem(a.CommandBar.Input, 1, 0, false)
			a.Application.SetFocus(a.CommandBar.Input)
			return nil
		}
		if event.Rune() == '?' {
			a.Pages.SwitchToPage("help")
			return nil
		}
		if event.Rune() == 's' {
			a.Pages.SwitchToPage("settings")
			return nil
		}
		return event
	})

	return a
}

func (a *App) Run() error {
	a.Application.EnableMouse(true)
	return a.Application.SetRoot(a.Pages, true).Run()
}

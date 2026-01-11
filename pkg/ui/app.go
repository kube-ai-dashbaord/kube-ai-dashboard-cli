package ui

import (
	"fmt"

	"github.com/GoogleCloudPlatform/kubectl-ai/pkg/agent"
	"github.com/kube-ai-dashbaord/kube-ai-dashboard-cli/pkg/ai"
	"github.com/kube-ai-dashbaord/kube-ai-dashboard-cli/pkg/config"
	"github.com/kube-ai-dashbaord/kube-ai-dashboard-cli/pkg/i18n"
	"github.com/kube-ai-dashbaord/kube-ai-dashboard-cli/pkg/k8s"
	"github.com/rivo/tview"
)

type App struct {
	Application     *tview.Application
	Dashboard       *Dashboard
	Assistant       *Assistant
	LogViewer       *LogViewer
	YamlViewer      *YamlViewer
	AuditViewer     *AuditViewer
	Reporter        *ai.Reporter
	Settings        *Settings
	CommandBar      *CommandBar
	K8s             *k8s.Client
	Help            *Help
	Header          *tview.Flex
	HeaderContainer *tview.Flex
	ShortcutBar     *tview.TextView
	Pages           *tview.Pages
	Root            *tview.Flex
	Config          *config.Config
	AIClient        *ai.Client
	K8sClient       *k8s.Client
	DashboardWidth  int
	AssistantWidth  int
	AgentManager    *agent.AgentManager
}

func (a *App) Run() error {
	return a.Application.SetRoot(a.Pages, true).Run()
}

func (a *App) CreateHeader() *tview.Flex {
	logo := tview.NewTextView().
		SetDynamicColors(true).
		SetText(fmt.Sprintf("[green]k[white]13[red]s [white]- [blue]%s", i18n.T("app_title")))

	clusterInfo := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignRight)

	ctx, _ := a.K8s.GetCurrentContext()
	ns := a.K8s.GetCurrentNamespace()
	clusterInfo.SetText(fmt.Sprintf("Context: [blue]%s [white]| Namespace: [blue]%s", ctx, ns))

	header := tview.NewFlex().
		AddItem(logo, 0, 1, false).
		AddItem(clusterInfo, 0, 1, false)
	header.SetBorder(true)
	return header
}

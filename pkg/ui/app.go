package ui

import (
	"fmt"

	"github.com/GoogleCloudPlatform/kubectl-ai/pkg/agent"
	"github.com/kube-ai-dashbaord/kube-ai-dashboard-cli/pkg/ai"
	"github.com/kube-ai-dashbaord/kube-ai-dashboard-cli/pkg/config"
	"github.com/kube-ai-dashbaord/kube-ai-dashboard-cli/pkg/k8s"
	"github.com/rivo/tview"
)

type App struct {
	Application     *tview.Application
	Dashboard       *Dashboard
	Assistant       *Assistant
	LogViewer       *LogViewer
	YamlViewer      *YamlViewer
	DescribeViewer  *DescribeViewer
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
	PulseViewer     *PulseViewer
	DashboardWidth  int
	AssistantWidth  int
	AgentManager    *agent.AgentManager
}

func (a *App) Run() error {
	defer func() {
		if r := recover(); r != nil {
			a.handlePanic(r)
		}
	}()
	return a.Application.SetRoot(a.Pages, true).Run()
}

func (a *App) handlePanic(err interface{}) {
	a.Application.Stop()
	fmt.Printf("\n[FATAL ERROR] k13s encountered a critical problem:\n%v\n", err)
	// In a real app, we might want to restart or show a modal if possible.
	// Since Run() exited, we are back in terminal.
}

func (a *App) CreateHeader() *tview.Flex {
	ctxName, cluster, user, _ := a.K8s.GetContextInfo()
	k8sVersion, _ := a.K8s.GetServerVersion()
	status := "Online"
	if a.AIClient == nil || a.AIClient.LLM == nil {
		status = "Offline"
	}
	ns := a.Dashboard.CurrentNamespace
	if ns == "" {
		ns = "all"
	}
	nsMap := a.Dashboard.GetNamespaceMapping()
	resource := a.Dashboard.CurrentResource
	return NewHeader(ctxName, cluster, user, k8sVersion, ns, status, nsMap, resource)
}

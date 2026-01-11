package ui

import (
	"fmt"

	"github.com/GoogleCloudPlatform/kubectl-ai/pkg/agent"
	"github.com/gdamore/tcell/v2"
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
	ResourceViewer  *ResourceViewer
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
	PulseViewer     *PulseViewer
	ScreenWidth     int
	DashboardWidth  int
	AssistantWidth  int
	MainContent     *tview.Flex
	AgentManager    *agent.AgentManager
}

func (a *App) Run() error {
	defer func() {
		if r := recover(); r != nil {
			a.handlePanic(r)
		}
	}()

	// Trigger initial refresh after the first draw
	firstDraw := true
	a.Application.SetAfterDrawFunc(func(screen tcell.Screen) {
		if firstDraw {
			firstDraw = false
			go a.Dashboard.Refresh()
		}
	})

	return a.Application.SetRoot(a.Pages, true).Run()
}

func (a *App) handlePanic(err interface{}) {
	a.Application.Stop()
	fmt.Printf("\n[FATAL ERROR] k13s encountered a critical problem:\n%v\n", err)
	// In a real app, we might want to restart or show a modal if possible.
	// Since Run() exited, we are back in terminal.
}

func (a *App) CreateHeader() *tview.Flex {
	var ctxName, cluster, user, k8sVersion, ns, nsMap, resource, status string = "N/A", "N/A", "N/A", "N/A", "all", "", "pods", "Offline"

	if a.K8s != nil {
		ctxName, cluster, user, _ = a.K8s.GetContextInfo()
		k8sVersion, _ = a.K8s.GetServerVersion()
	}
	if a.AIClient != nil && a.AIClient.LLM != nil {
		status = "Online"
	}
	if a.Dashboard != nil {
		ns = a.Dashboard.CurrentNamespace
		if ns == "" {
			ns = "all"
		}
		nsMap = a.Dashboard.GetNamespaceMapping()
		resource = a.Dashboard.CurrentResource
	}
	return NewHeader(ctxName, cluster, user, k8sVersion, ns, status, nsMap, resource, a.ScreenWidth)
}

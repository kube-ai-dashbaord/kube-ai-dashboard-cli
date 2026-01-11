package ui

import (
	"context"
	"fmt"

	"github.com/GoogleCloudPlatform/kubectl-ai/pkg/agent"
	"github.com/GoogleCloudPlatform/kubectl-ai/pkg/api"
	"github.com/GoogleCloudPlatform/kubectl-ai/pkg/sessions"
	"github.com/GoogleCloudPlatform/kubectl-ai/pkg/tools"
	"github.com/kube-ai-dashbaord/kube-ai-dashboard-cli/pkg/ai"
	"github.com/kube-ai-dashbaord/kube-ai-dashboard-cli/pkg/config"
	"github.com/kube-ai-dashbaord/kube-ai-dashboard-cli/pkg/i18n"
	"github.com/kube-ai-dashbaord/kube-ai-dashboard-cli/pkg/k8s"
	"github.com/rivo/tview"
)

func NewApp() *App {
	cfg, _ := config.LoadConfig()
	return NewAppWithConfig(cfg)
}

func NewAppWithConfig(cfg *config.Config) *App {
	aiClient, _ := ai.NewClient(&cfg.LLM)
	k8sClient, _ := k8s.NewClient()
	return InitApp(tview.NewApplication(), cfg, aiClient, k8sClient)
}

func InitApp(tviewApp *tview.Application, cfg *config.Config, aiClient *ai.Client, k8sClient *k8s.Client) *App {
	i18n.SetLanguage(cfg.Language)

	sessionManager, _ := sessions.NewSessionManager("memory")
	agentFactory := func(ctx context.Context) (*agent.Agent, error) {
		return &agent.Agent{
			Model:            cfg.LLM.Model,
			Provider:         cfg.LLM.Provider,
			Kubeconfig:       "", // use default
			LLM:              aiClient.LLM,
			MaxIterations:    20, // Match kubectl-ai default
			SkipPermissions:  false,
			MCPClientEnabled: true,
			Tools:            tools.Default(),
		}, nil
	}
	agentManager := agent.NewAgentManager(agentFactory, sessionManager)

	a := &App{
		Application:  tviewApp,
		Pages:        tview.NewPages(),
		K8s:          k8sClient,
		Config:       cfg,
		AgentManager: agentManager,
	}

	ctx := context.Background()
	var ag *agent.Agent
	if aiClient != nil && aiClient.LLM != nil {
		sess, _ := sessionManager.NewSession(sessions.Metadata{
			ModelID:    cfg.LLM.Model,
			ProviderID: cfg.LLM.Provider,
		})
		ag, _ = agentManager.GetAgent(ctx, sess.ID)
	}
	reporter := ai.NewReporter(cfg.ReportPath)

	a.Assistant = NewAssistant(a.Application, ag, reporter)
	a.AIClient = aiClient
	a.K8sClient = k8sClient
	a.Reporter = reporter

	InitLogger("k13s.log", cfg.LogLevel)
	Infof("Starting k13s with log level: %s", cfg.LogLevel)

	a.Settings = NewSettings(cfg, func(newCfg *config.Config) {
		a.Config = newCfg
		a.Config.Save()
		i18n.SetLanguage(newCfg.Language)
		InitLogger("k13s.log", newCfg.LogLevel)
		Infof("Log level updated to: %s", newCfg.LogLevel)
		newAI, _ := ai.NewClient(&newCfg.LLM)
		a.AIClient = newAI
		a.Reporter.OutputPath = newCfg.ReportPath
		if a.Assistant != nil && a.Assistant.Agent != nil {
			a.Assistant.Agent.Model = newCfg.LLM.Model
			a.Assistant.Agent.Provider = newCfg.LLM.Provider
			a.Assistant.Agent.LLM = newAI.LLM
		}
	}, func() {
		a.Pages.SwitchToPage("main")
	})

	a.LogViewer = NewLogViewer(a.Application, k8sClient, func() {
		a.Pages.SwitchToPage("main")
		a.LogViewer.Stop()
	})

	a.YamlViewer = NewYamlViewer(a.Application, k8sClient, func() {
		a.Pages.SwitchToPage("main")
	})

	a.DescribeViewer = NewDescribeViewer(a.Application, k8sClient, func() {
		a.Pages.SwitchToPage("main")
	})

	a.PulseViewer = NewPulseViewer(a.Application, k8sClient, func() {
		a.Pages.SwitchToPage("main")
	})

	a.AuditViewer = NewAuditViewer(a.Application)

	a.Dashboard = NewDashboard(a.Application, k8sClient, func(cmd string) {
		if ag != nil {
			ag.Input <- &api.UserInputResponse{Query: cmd}
		}
	}, func(selected string) {
		if a.Assistant != nil {
			a.Assistant.SetContext(fmt.Sprintf("%s/%s", a.Dashboard.CurrentResource, selected))
		}
	}, func(ns, name string) {
		a.LogViewer.StreamLogs(ns, name)
		a.Pages.AddPage("logs", a.LogViewer.View, true, true)
		a.Pages.SwitchToPage("logs")
	})

	a.Dashboard.OnRefresh = func() {
		a.Application.QueueUpdateDraw(func() {
			newHeader := a.CreateHeader()
			a.HeaderContainer.RemoveItem(a.Header)
			a.HeaderContainer.AddItem(newHeader, 3, 0, false)
			a.Header = newHeader
			a.RefreshShortcuts()
		})
	}

	a.Dashboard.OnExplainRequested = func(ns, name string) {
		Infof("AI Explain requested for %s in %s", name, ns)
		a.Application.SetFocus(a.Assistant.Input)
		res := a.Dashboard.CurrentResource
		prompt := fmt.Sprintf("Explain this %s %s in namespace %s in detail for a beginner.", res, name, ns)
		a.Assistant.Input.SetText(prompt)
		// We could auto-submit, but letting the user see the prompt is better for pedagogical value
	}

	a.Header = a.CreateHeader()
	a.HeaderContainer = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(a.Header, 3, 0, false)

	a.initCallbacks(ag)
	a.initPages()

	return a
}

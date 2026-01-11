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
	i18n.SetLanguage(cfg.Language)
	aiClient, _ := ai.NewClient(&cfg.LLM)
	k8sClient, _ := k8s.NewClient()

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
		Application:  tview.NewApplication(),
		Pages:        tview.NewPages(),
		K8s:          k8sClient,
		Config:       cfg,
		AgentManager: agentManager,
	}

	ctx := context.Background()
	sess, _ := sessionManager.NewSession(sessions.Metadata{
		ModelID:    cfg.LLM.Model,
		ProviderID: cfg.LLM.Provider,
	})

	ag, _ := agentManager.GetAgent(ctx, sess.ID)
	reporter := ai.NewReporter(cfg.ReportPath)

	a.Assistant = NewAssistant(a.Application, ag, reporter)
	a.AIClient = aiClient
	a.K8sClient = k8sClient
	a.Reporter = reporter

	a.Settings = NewSettings(cfg, func(newCfg *config.Config) {
		a.Config = newCfg
		a.Config.Save()
		i18n.SetLanguage(newCfg.Language)
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

	a.Header = a.CreateHeader()
	a.HeaderContainer = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(a.Header, 3, 0, false)

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

	a.initCallbacks(ag)
	a.initPages()

	return a
}

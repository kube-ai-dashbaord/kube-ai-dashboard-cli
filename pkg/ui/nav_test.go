package ui

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/kube-ai-dashbaord/kube-ai-dashboard-cli/pkg/ai"
	"github.com/kube-ai-dashbaord/kube-ai-dashboard-cli/pkg/config"
	"github.com/kube-ai-dashbaord/kube-ai-dashboard-cli/pkg/k8s"
	"k8s.io/client-go/kubernetes/fake"
)

func TestNavigation(t *testing.T) {
	tester, err := NewTUITester()
	if err != nil {
		t.Fatalf("failed to create tester: %v", err)
	}

	cfg := &config.Config{Language: "en"}
	k8sClient := &k8s.Client{Clientset: fake.NewSimpleClientset()}
	aiClient := &ai.Client{}

	app := InitApp(tester.App, cfg, aiClient, k8sClient)

	stop := tester.Run()
	defer stop()

	// 1. Initial State: Main Page (Dashboard focused)
	tester.AssertPage(t, app.Pages, "main")

	// 2. Switch to Settings (s)
	tester.InjectKey(tcell.KeyRune, 's', tcell.ModNone)
	tester.AssertPage(t, app.Pages, "settings")

	// 3. Back to Main (ESC)
	tester.InjectKey(tcell.KeyEscape, 0, tcell.ModNone)
	tester.AssertPage(t, app.Pages, "main")

	// 4. Show Help (?)
	tester.InjectKey(tcell.KeyRune, '?', tcell.ModNone)
	tester.AssertPage(t, app.Pages, "help")

	// 5. Hide Help (ESC)
	tester.InjectKey(tcell.KeyEscape, 0, tcell.ModNone)
	tester.AssertPage(t, app.Pages, "main")
}

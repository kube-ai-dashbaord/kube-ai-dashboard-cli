package ui

import (
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/kubectl-ai/pkg/agent"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// TUITester wraps a tview application with a simulation screen for automated testing.
type TUITester struct {
	App       *tview.Application
	Screen    tcell.SimulationScreen
	Assistant *Assistant
}

// NewTUITester creates a new TUI tester.
func NewTUITester() (*TUITester, error) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		return nil, err
	}
	tviewApp := tview.NewApplication().SetScreen(screen)
	// We need mock dependencies for InitApp
	app := &App{
		Application: tviewApp,
	}
	// Note: In a real test we'd use InitApp with mock K8s/AI clients.
	// For this TUI-only verification, we'll manually set up what we need or call a simplified Init.

	// Create required components
	app.Dashboard = NewDashboard(tviewApp, nil, nil, nil, nil)
	app.Assistant = NewAssistant(tviewApp, &agent.Agent{Input: make(chan any, 10), Output: make(chan any, 10)}, nil)

	// For testing assistant, we set it as the primary root
	tviewApp.SetRoot(app.Assistant.Root, true)

	return &TUITester{
		App:       tviewApp,
		Screen:    screen,
		Assistant: app.Assistant,
	}, nil
}

// InjectKey simulates a key press.
func (t *TUITester) InjectKey(key tcell.Key, r rune, mod tcell.ModMask) {
	t.Screen.InjectKey(key, r, mod)
	time.Sleep(10 * time.Millisecond) // Give tview a moment to process
}

// GetContent returns the text content of the simulation screen.
func (t *TUITester) GetContent() string {
	width, height := t.Screen.Size()
	var content string
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			mainc, _, _, _ := t.Screen.GetContent(x, y)
			content += string(mainc)
		}
		content += "\n"
	}
	return content
}

// Run runs the application in a goroutine and returns a stop function.
func (t *TUITester) Run() func() {
	go func() {
		if err := t.App.Run(); err != nil {
			panic(err)
		}
	}()
	return func() {
		t.App.Stop()
	}
}

// AssertPage verifies that the current front page matches the expected one.
func (t *TUITester) AssertPage(tb testing.TB, pages *tview.Pages, expected string) {
	tb.Helper()
	front, _ := pages.GetFrontPage()
	if front != expected {
		tb.Errorf("expected front page %s, got %s", expected, front)
	}
}

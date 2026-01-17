package ui

import (
	"strings"
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

// WaitForCondition waits for a condition to be true with timeout.
// Returns true if condition was met, false if timeout occurred.
func (t *TUITester) WaitForCondition(timeout time.Duration, condition func() bool) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}

// AssertContentContains checks if screen contains the expected text.
func (t *TUITester) AssertContentContains(tb testing.TB, expected string) {
	tb.Helper()
	content := t.GetContent()
	if !strings.Contains(content, expected) {
		tb.Errorf("Screen does not contain '%s'.\nActual content:\n---\n%s\n---", expected, content)
	}
}

// AssertContentNotContains checks if screen doesn't contain the unexpected text.
func (t *TUITester) AssertContentNotContains(tb testing.TB, unexpected string) {
	tb.Helper()
	content := t.GetContent()
	if strings.Contains(content, unexpected) {
		tb.Errorf("Screen unexpectedly contains '%s'.\nActual content:\n---\n%s\n---", unexpected, content)
	}
}

// TypeString simulates typing a string character by character.
func (t *TUITester) TypeString(s string) {
	for _, r := range s {
		t.InjectKey(tcell.KeyRune, r, tcell.ModNone)
	}
}

// PressEnter simulates pressing the Enter key.
func (t *TUITester) PressEnter() {
	t.InjectKey(tcell.KeyEnter, 0, tcell.ModNone)
}

// PressEscape simulates pressing the Escape key.
func (t *TUITester) PressEscape() {
	t.InjectKey(tcell.KeyEscape, 0, tcell.ModNone)
}

// PressTab simulates pressing the Tab key.
func (t *TUITester) PressTab() {
	t.InjectKey(tcell.KeyTab, 0, tcell.ModNone)
}

// PressCtrl simulates pressing Ctrl+key combination.
func (t *TUITester) PressCtrl(r rune) {
	var key tcell.Key
	switch r {
	case 'c':
		key = tcell.KeyCtrlC
	case 'd':
		key = tcell.KeyCtrlD
	case 'h':
		key = tcell.KeyCtrlH
	case 'l':
		key = tcell.KeyCtrlL
	default:
		// For other keys, use generic approach
		t.InjectKey(tcell.KeyRune, r, tcell.ModCtrl)
		return
	}
	t.InjectKey(key, 0, tcell.ModCtrl)
}

// GetFocusedPrimitive returns the currently focused primitive.
func (t *TUITester) GetFocusedPrimitive() tview.Primitive {
	return t.App.GetFocus()
}

// Wait pauses for the specified duration.
// Useful for allowing async operations to complete.
func (t *TUITester) Wait(d time.Duration) {
	time.Sleep(d)
}

// Resize changes the simulation screen size.
func (t *TUITester) Resize(width, height int) {
	t.Screen.SetSize(width, height)
	t.Wait(20 * time.Millisecond) // Allow redraw
}

// DumpScreen returns the current screen content for debugging.
// This is useful when a test fails and you need to see what's on screen.
func (t *TUITester) DumpScreen() string {
	content := t.GetContent()
	width, height := t.Screen.Size()
	return "=== Screen Dump (" + strings.TrimSpace(strings.Join([]string{
		"size:", string(rune(width)), "x", string(rune(height)),
	}, "")) + ") ===\n" + content + "\n=== End Dump ==="
}

// GetContentLines returns the screen content as a slice of lines.
func (t *TUITester) GetContentLines() []string {
	content := t.GetContent()
	return strings.Split(content, "\n")
}

// AssertLineContains checks if a specific line contains the expected text.
// Line numbers are 0-indexed.
func (t *TUITester) AssertLineContains(tb testing.TB, lineNum int, expected string) {
	tb.Helper()
	lines := t.GetContentLines()
	if lineNum >= len(lines) {
		tb.Errorf("Line %d does not exist (total lines: %d)", lineNum, len(lines))
		return
	}
	if !strings.Contains(lines[lineNum], expected) {
		tb.Errorf("Line %d does not contain '%s'.\nLine content: '%s'", lineNum, expected, lines[lineNum])
	}
}

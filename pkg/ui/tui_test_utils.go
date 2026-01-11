package ui

import (
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// TUITester wraps a tview application with a simulation screen for automated testing.
type TUITester struct {
	App    *tview.Application
	Screen tcell.SimulationScreen
}

// NewTUITester creates a new TUI tester.
func NewTUITester() (*TUITester, error) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		return nil, err
	}
	app := tview.NewApplication().SetScreen(screen)
	return &TUITester{
		App:    app,
		Screen: screen,
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

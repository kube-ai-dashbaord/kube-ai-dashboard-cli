package ui

import (
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// TestAppHarness provides a way to test the TUI with a simulation screen
func TestAppHarness(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("failed to init screen: %v", err)
	}

	app := tview.NewApplication().SetScreen(screen)
	// Add a simple component to test
	box := tview.NewBox().SetBorder(true).SetTitle("Test Box")
	app.SetRoot(box, true)

	go func() {
		time.Sleep(100 * time.Millisecond)
		// Simulate a key press
		screen.InjectKey(tcell.KeyEnter, ' ', tcell.ModNone)
		time.Sleep(100 * time.Millisecond)
		app.Stop()
	}()

	if err := app.Run(); err != nil {
		t.Fatalf("app run failed: %v", err)
	}
}

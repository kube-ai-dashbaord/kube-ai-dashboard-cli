package main

import (
	"fmt"
	"os"
	"runtime/debug"

	"github.com/kube-ai-dashbaord/kube-ai-dashboard-cli/pkg/db"
	"github.com/kube-ai-dashbaord/kube-ai-dashboard-cli/pkg/log"
	"github.com/kube-ai-dashbaord/kube-ai-dashboard-cli/pkg/ui"
)

func main() {
	// Initialize enterprise logger
	if err := log.Init("k13s"); err != nil {
		fmt.Printf("Warning: could not initialize logger: %v\n", err)
	}

	log.Infof("Starting k13s application...")
	app := ui.NewApp()

	// Initialize audit database if enabled in config
	if app.Config.EnableAudit {
		if err := db.Init(""); err != nil {
			log.Errorf("Failed to initialize audit database: %v", err)
		}
		defer db.Close()
	}

	defer func() {
		if r := recover(); r != nil {
			log.Errorf("PANIC RECOVERED: %v\n%s", r, debug.Stack())
			fmt.Fprintf(os.Stderr, "k13s crashed due to a panic. Details have been logged.\n")
			os.Exit(1)
		}
	}()

	if err := app.Run(); err != nil {
		log.Errorf("Application exited with error: %v", err)
		os.Exit(1)
	}
	log.Infof("k13s application exited cleanly.")
}

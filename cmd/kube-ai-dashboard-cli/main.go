package main

import (
	"log"
	"github.com/kube-ai-dashbaord/kube-ai-dashboard-cli/pkg/ui"
)

func main() {
	app := ui.NewApp()
	if err := app.Run(); err != nil {
		log.Fatalf("Error running application: %v", err)
	}
}

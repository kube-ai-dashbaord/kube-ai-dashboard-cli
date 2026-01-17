package main

import (
	"flag"
	"fmt"
	"os"
	"runtime/debug"

	"github.com/kube-ai-dashbaord/kube-ai-dashboard-cli/pkg/config"
	"github.com/kube-ai-dashbaord/kube-ai-dashboard-cli/pkg/db"
	"github.com/kube-ai-dashbaord/kube-ai-dashboard-cli/pkg/log"
	"github.com/kube-ai-dashbaord/kube-ai-dashboard-cli/pkg/ui"
	"github.com/kube-ai-dashbaord/kube-ai-dashboard-cli/pkg/web"
)

// Version info (set by ldflags)
var (
	Version   = "dev"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

func main() {
	// Command line flags (k9s compatible)
	webMode := flag.Bool("web", false, "Start web server mode")
	webPort := flag.Int("port", 8080, "Web server port (used with -web)")
	namespace := flag.String("n", "", "Initial namespace (use 'all' for all namespaces)")
	allNamespaces := flag.Bool("A", false, "Start with all namespaces")
	showVersion := flag.Bool("version", false, "Show version information")
	flag.StringVar(namespace, "namespace", "", "Initial namespace (use 'all' for all namespaces)")
	flag.Parse()

	// Show version
	if *showVersion {
		fmt.Printf("k13s version %s\n", Version)
		fmt.Printf("  Build time: %s\n", BuildTime)
		fmt.Printf("  Git commit: %s\n", GitCommit)
		return
	}

	// Initialize enterprise logger
	if err := log.Init("k13s"); err != nil {
		fmt.Printf("Warning: could not initialize logger: %v\n", err)
	}

	log.Infof("Starting k13s application...")

	// Load config
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Errorf("Failed to load config: %v", err)
		cfg = config.NewDefaultConfig()
	}

	// Web mode
	if *webMode {
		runWebServer(cfg, *webPort)
		return
	}

	// TUI mode with optional namespace
	initialNS := *namespace
	if *allNamespaces {
		initialNS = "" // empty means all namespaces
	}
	runTUI(cfg, initialNS)
}

func runWebServer(cfg *config.Config, port int) {
	server, err := web.NewServer(cfg, port)
	if err != nil {
		log.Errorf("Failed to create web server: %v", err)
		os.Exit(1)
	}

	if err := server.Start(); err != nil {
		log.Errorf("Web server error: %v", err)
		os.Exit(1)
	}
}

func runTUI(cfg *config.Config, initialNamespace string) {
	// Initialize audit database if enabled in config
	if cfg.EnableAudit {
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

	app := ui.NewAppWithNamespace(initialNamespace)
	if err := app.Run(); err != nil {
		log.Errorf("Application exited with error: %v", err)
		os.Exit(1)
	}
	log.Infof("k13s application exited cleanly.")
}

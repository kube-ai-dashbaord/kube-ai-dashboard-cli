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
	genCompletion := flag.String("completion", "", "Generate shell completion (bash, zsh, fish)")
	flag.StringVar(namespace, "namespace", "", "Initial namespace (use 'all' for all namespaces)")
	flag.Parse()

	// Show version
	if *showVersion {
		fmt.Printf("k13s version %s\n", Version)
		fmt.Printf("  Build time: %s\n", BuildTime)
		fmt.Printf("  Git commit: %s\n", GitCommit)
		return
	}

	// Generate shell completion
	if *genCompletion != "" {
		generateCompletion(*genCompletion)
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

// generateCompletion outputs shell completion script
func generateCompletion(shell string) {
	switch shell {
	case "bash":
		fmt.Print(bashCompletion)
	case "zsh":
		fmt.Print(zshCompletion)
	case "fish":
		fmt.Print(fishCompletion)
	default:
		fmt.Fprintf(os.Stderr, "Unknown shell: %s. Supported: bash, zsh, fish\n", shell)
		os.Exit(1)
	}
}

const bashCompletion = `# k13s bash completion
_k13s_completions() {
    local cur prev opts namespaces
    COMPREPLY=()
    cur="${COMP_WORDS[COMP_CWORD]}"
    prev="${COMP_WORDS[COMP_CWORD-1]}"

    # Main options
    opts="-n --namespace -A -web -port --version --completion"

    # Complete namespace after -n or --namespace
    if [[ "${prev}" == "-n" ]] || [[ "${prev}" == "--namespace" ]]; then
        namespaces=$(kubectl get namespaces -o jsonpath='{.items[*].metadata.name}' 2>/dev/null)
        COMPREPLY=( $(compgen -W "${namespaces} all" -- ${cur}) )
        return 0
    fi

    # Complete shell after --completion
    if [[ "${prev}" == "--completion" ]]; then
        COMPREPLY=( $(compgen -W "bash zsh fish" -- ${cur}) )
        return 0
    fi

    # Default to options
    COMPREPLY=( $(compgen -W "${opts}" -- ${cur}) )
    return 0
}
complete -F _k13s_completions k13s

# To enable: source <(k13s --completion bash)
# Or add to ~/.bashrc: eval "$(k13s --completion bash)"
`

const zshCompletion = `#compdef k13s

_k13s() {
    local -a opts namespaces

    opts=(
        '-n[Initial namespace]:namespace:->namespaces'
        '--namespace[Initial namespace]:namespace:->namespaces'
        '-A[Start with all namespaces]'
        '-web[Start web server mode]'
        '-port[Web server port]:port:'
        '--version[Show version information]'
        '--completion[Generate shell completion]:shell:(bash zsh fish)'
    )

    _arguments -s $opts

    case "$state" in
        namespaces)
            namespaces=(${(f)"$(kubectl get namespaces -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}' 2>/dev/null)"})
            namespaces+=("all")
            _describe 'namespace' namespaces
            ;;
    esac
}

_k13s "$@"

# To enable: source <(k13s --completion zsh)
# Or add to ~/.zshrc: eval "$(k13s --completion zsh)"
`

const fishCompletion = `# k13s fish completion
function __k13s_get_namespaces
    kubectl get namespaces -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}' 2>/dev/null
    echo "all"
end

complete -c k13s -f
complete -c k13s -s n -l namespace -d 'Initial namespace' -xa '(__k13s_get_namespaces)'
complete -c k13s -s A -d 'Start with all namespaces'
complete -c k13s -l web -d 'Start web server mode'
complete -c k13s -l port -d 'Web server port'
complete -c k13s -l version -d 'Show version information'
complete -c k13s -l completion -d 'Generate shell completion' -xa 'bash zsh fish'

# To enable: k13s --completion fish | source
# Or add to ~/.config/fish/config.fish: k13s --completion fish | source
`

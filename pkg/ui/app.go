package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"regexp"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/gdamore/tcell/v2"
	"github.com/kube-ai-dashbaord/kube-ai-dashboard-cli/pkg/ai"
	"github.com/kube-ai-dashbaord/kube-ai-dashboard-cli/pkg/config"
	"github.com/kube-ai-dashbaord/kube-ai-dashboard-cli/pkg/i18n"
	"github.com/kube-ai-dashbaord/kube-ai-dashboard-cli/pkg/k8s"
	"github.com/rivo/tview"
	corev1 "k8s.io/api/core/v1"
)

// Command definitions for autocomplete (k9s-style comprehensive list)
var commands = []struct {
	name     string
	alias    string
	desc     string
	category string
}{
	// Core Resources
	{"pods", "po", "List pods", "resource"},
	{"deployments", "deploy", "List deployments", "resource"},
	{"services", "svc", "List services", "resource"},
	{"nodes", "no", "List nodes", "resource"},
	{"namespaces", "ns", "List namespaces", "resource"},
	{"events", "ev", "List events", "resource"},

	// Config & Storage
	{"configmaps", "cm", "List configmaps", "resource"},
	{"secrets", "sec", "List secrets", "resource"},
	{"persistentvolumes", "pv", "List persistent volumes", "resource"},
	{"persistentvolumeclaims", "pvc", "List persistent volume claims", "resource"},
	{"storageclasses", "sc", "List storage classes", "resource"},

	// Workloads
	{"replicasets", "rs", "List replicasets", "resource"},
	{"daemonsets", "ds", "List daemonsets", "resource"},
	{"statefulsets", "sts", "List statefulsets", "resource"},
	{"jobs", "job", "List jobs", "resource"},
	{"cronjobs", "cj", "List cronjobs", "resource"},
	{"replicationcontrollers", "rc", "List replication controllers", "resource"},

	// Networking
	{"ingresses", "ing", "List ingresses", "resource"},
	{"endpoints", "ep", "List endpoints", "resource"},
	{"networkpolicies", "netpol", "List network policies", "resource"},
	{"ingressclasses", "ic", "List ingress classes", "resource"},

	// RBAC
	{"serviceaccounts", "sa", "List service accounts", "resource"},
	{"roles", "role", "List roles", "resource"},
	{"rolebindings", "rb", "List role bindings", "resource"},
	{"clusterroles", "cr", "List cluster roles", "resource"},
	{"clusterrolebindings", "crb", "List cluster role bindings", "resource"},

	// Policy
	{"poddisruptionbudgets", "pdb", "List pod disruption budgets", "resource"},
	{"podsecuritypolicies", "psp", "List pod security policies", "resource"},
	{"limitranges", "limits", "List limit ranges", "resource"},
	{"resourcequotas", "quota", "List resource quotas", "resource"},
	{"horizontalpodautoscalers", "hpa", "List horizontal pod autoscalers", "resource"},

	// CRDs
	{"customresourcedefinitions", "crd", "List custom resource definitions", "resource"},

	// Other
	{"leases", "lease", "List leases", "resource"},
	{"priorityclasses", "pc", "List priority classes", "resource"},
	{"runtimeclasses", "rtc", "List runtime classes", "resource"},
	{"volumeattachments", "va", "List volume attachments", "resource"},
	{"csidrivers", "csidriver", "List CSI drivers", "resource"},
	{"csinodes", "csinode", "List CSI nodes", "resource"},

	// Actions
	{"quit", "q", "Exit application", "action"},
	{"health", "status", "Show cluster health", "action"},
	{"context", "ctx", "Switch context", "action"},
	{"help", "?", "Show help", "action"},
}

// App is the main TUI application with k9s-style stability patterns
type App struct {
	*tview.Application

	// Core
	config   *config.Config
	k8s      *k8s.Client
	aiClient *ai.Client

	// UI components
	pages       *tview.Pages
	header      *tview.TextView
	table       *tview.Table
	statusBar   *tview.TextView
	flash       *tview.TextView
	cmdInput    *tview.InputField
	cmdHint     *tview.TextView // Autocomplete hint (dimmed)
	cmdDropdown *tview.List     // Autocomplete dropdown
	aiPanel     *tview.TextView
	aiInput     *tview.InputField // AI question input

	// State (protected by mutex)
	mx               sync.RWMutex
	currentResource  string
	currentNamespace string
	namespaces       []string
	showAIPanel      bool
	filterText       string     // Current filter text
	filterRegex      bool       // True if filter is regex (e.g., /pattern/)
	tableHeaders     []string   // Original headers
	tableRows        [][]string // Original rows (unfiltered)
	apiResources     []k8s.APIResource // Cached API resources from cluster
	selectedRows     map[int]bool // Multi-select: selected row indices (k9s Space key)

	// Atomic guards (k9s pattern for lock-free update deduplication)
	inUpdate   int32
	running    int32 // 1 after Application.Run() starts
	cancelFn   context.CancelFunc
	cancelLock sync.Mutex

	// Logger
	logger *slog.Logger
}

// NewApp creates a new TUI application with default (all) namespace
func NewApp() *App {
	return NewAppWithNamespace("")
}

// NewAppWithNamespace creates a new TUI application with initial namespace
// Pass "" for all namespaces, or a specific namespace name
func NewAppWithNamespace(initialNamespace string) *App {
	// Setup structured logging (k9s pattern)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	cfg, err := config.LoadConfig()
	if err != nil {
		logger.Warn("Failed to load config, using defaults", "error", err)
		cfg = config.NewDefaultConfig()
	}
	i18n.SetLanguage(cfg.Language)

	// K8s client with graceful degradation (k9s pattern)
	k8sClient, err := k8s.NewClient()
	if err != nil {
		logger.Warn("K8s client initialization failed", "error", err)
	} else {
		logger.Info("K8s connectivity OK")
	}

	// AI client (optional)
	aiClient, _ := ai.NewClient(&cfg.LLM)

	// Handle "all" as empty string (all namespaces)
	if initialNamespace == "all" {
		initialNamespace = ""
	}

	app := &App{
		Application:      tview.NewApplication(),
		config:           cfg,
		k8s:              k8sClient,
		aiClient:         aiClient,
		currentResource:  "pods",
		currentNamespace: initialNamespace,
		namespaces:       []string{""},
		showAIPanel:      true,
		selectedRows:     make(map[int]bool),
		logger:           logger,
	}

	app.setupUI()
	app.setupKeybindings()

	// Load API resources in background (for autocomplete)
	go app.loadAPIResources()

	return app
}

// loadAPIResources fetches available API resources from the cluster
func (a *App) loadAPIResources() {
	if a.k8s == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resources, err := a.k8s.GetAPIResources(ctx)
	if err != nil {
		a.logger.Warn("Failed to load API resources", "error", err)
		// Use common resources as fallback
		resources = a.k8s.GetCommonResources()
	}

	a.mx.Lock()
	a.apiResources = resources
	a.mx.Unlock()

	a.logger.Info("Loaded API resources", "count", len(resources))
}

// setupUI initializes all UI components
func (a *App) setupUI() {
	// Header
	a.header = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	a.header.SetBackgroundColor(tcell.ColorDarkBlue)

	// Main table with fixed header row
	a.table = tview.NewTable().
		SetSelectable(true, false).
		SetFixed(1, 0)
	a.table.SetBorder(true).
		SetBorderColor(tcell.ColorDarkCyan)
	a.table.SetSelectedStyle(tcell.StyleDefault.
		Background(tcell.ColorDarkCyan).
		Foreground(tcell.ColorWhite))

	// AI Panel (output area)
	a.aiPanel = tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetWrap(true)
	a.aiPanel.SetText("[gray]Press [yellow]Tab[gray] to ask AI\n\n" +
		"[white]Examples:\n" +
		"[darkgray]- Why is this pod failing?\n" +
		"- How do I scale this deployment?\n" +
		"- Explain this resource")

	// AI Input field
	a.aiInput = tview.NewInputField().
		SetLabel(" 🤖 ").
		SetFieldWidth(0).
		SetFieldBackgroundColor(tcell.ColorDefault).
		SetPlaceholder("Ask AI a question...")
	a.aiInput.SetPlaceholderStyle(tcell.StyleDefault.Foreground(tcell.ColorDarkGray))
	a.setupAIInput()

	// Flash message area (k9s pattern)
	a.flash = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter)

	// Status bar
	a.statusBar = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter)
	a.statusBar.SetBackgroundColor(tcell.ColorDarkGreen)

	// Command input with autocomplete
	a.cmdInput = tview.NewInputField().
		SetLabel(" : ").
		SetFieldWidth(0).
		SetFieldBackgroundColor(tcell.ColorDefault)

	// Autocomplete hint (dimmed text showing suggestion)
	a.cmdHint = tview.NewTextView().
		SetDynamicColors(true)

	// Autocomplete dropdown
	a.cmdDropdown = tview.NewList().
		ShowSecondaryText(true).
		SetHighlightFullLine(true).
		SetSelectedBackgroundColor(tcell.ColorDarkCyan).
		SetSelectedTextColor(tcell.ColorWhite)
	a.cmdDropdown.SetBorder(true).SetTitle(" Commands ")

	// Setup autocomplete behavior
	a.setupAutocomplete()

	// AI Panel container (output + input)
	aiContainer := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(a.aiPanel, 0, 1, false).
		AddItem(a.aiInput, 1, 0, true)
	aiContainer.SetBorder(true).
		SetTitle(" AI Assistant ").
		SetBorderColor(tcell.ColorDarkMagenta)

	// Content area (table + AI panel)
	contentFlex := tview.NewFlex()
	contentFlex.AddItem(a.table, 0, 3, true)
	if a.showAIPanel {
		contentFlex.AddItem(aiContainer, 45, 0, false)
	}

	// Command bar with hint overlay
	cmdFlex := tview.NewFlex().
		AddItem(a.cmdInput, 0, 1, true).
		AddItem(a.cmdHint, 0, 2, false)

	// Main layout
	mainFlex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(a.header, 3, 0, false).
		AddItem(a.flash, 1, 0, false).
		AddItem(contentFlex, 0, 1, true).
		AddItem(a.statusBar, 1, 0, false).
		AddItem(cmdFlex, 1, 0, false)

	// Pages
	a.pages = tview.NewPages().
		AddPage("main", mainFlex, true, true)

	a.SetRoot(a.pages, true)

	// Initial UI state
	a.updateHeader()
	a.updateStatusBar()
}

// setupAIInput configures the AI input field
func (a *App) setupAIInput() {
	a.aiInput.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter {
			question := a.aiInput.GetText()
			if question != "" {
				a.aiInput.SetText("")
				go a.askAI(question)
			}
		}
	})

	a.aiInput.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEsc, tcell.KeyTab:
			a.SetFocus(a.table)
			return nil
		}
		return event
	})
}

// PendingDecision represents a command awaiting user approval
type PendingDecision struct {
	Command     string
	Description string
	IsDangerous bool
	Warnings    []string
	// For MCP tool execution
	ToolName  string
	ToolArgs  string
	IsToolCall bool
}

// pendingDecisions stores commands awaiting user approval
var pendingDecisions []PendingDecision

// pendingToolApproval channel for synchronous tool approval
var pendingToolApproval = make(chan bool, 1)

// currentToolCallInfo stores info about the tool being approved
var currentToolCallInfo struct {
	Name    string
	Args    string
	Command string
}

// askAI sends a question to the AI and displays the response
func (a *App) askAI(question string) {
	// Show loading state
	a.QueueUpdateDraw(func() {
		a.aiPanel.SetText(fmt.Sprintf("[yellow]Question:[white] %s\n\n[gray]Thinking...", question))
	})

	// Get current context
	a.mx.RLock()
	resource := a.currentResource
	ns := a.currentNamespace
	a.mx.RUnlock()

	// Get selected resource info if available
	var selectedInfo string
	row, _ := a.table.GetSelection()
	if row > 0 {
		var parts []string
		for c := 0; c < a.table.GetColumnCount(); c++ {
			cell := a.table.GetCell(row, c)
			if cell != nil {
				parts = append(parts, cell.Text)
			}
		}
		selectedInfo = strings.Join(parts, " | ")
	}

	// Build context for AI
	ctx := context.Background()
	prompt := fmt.Sprintf(`User is viewing Kubernetes %s`, resource)
	if ns != "" {
		prompt += fmt.Sprintf(` in namespace "%s"`, ns)
	}
	if selectedInfo != "" {
		prompt += fmt.Sprintf(`. Selected: %s`, selectedInfo)
	}
	prompt += fmt.Sprintf(`

User question: %s

Please provide a concise, helpful answer. If you suggest kubectl commands, wrap them in code blocks.`, question)

	// Call AI
	if a.aiClient == nil || !a.aiClient.IsReady() {
		a.QueueUpdateDraw(func() {
			a.aiPanel.SetText(fmt.Sprintf("[yellow]Q:[white] %s\n\n[red]AI is not available.[white]\n\nConfigure LLM in config file:\n[gray]~/.kube-ai-dashboard/config.yaml", question))
		})
		return
	}

	// Check if AI supports tool calling (agentic mode)
	var fullResponse strings.Builder
	var err error

	if a.aiClient.SupportsTools() {
		// Use agentic mode with tool calling
		a.QueueUpdateDraw(func() {
			a.aiPanel.SetText(fmt.Sprintf("[yellow]Q:[white] %s\n\n[cyan]🤖 Agentic Mode[white] - AI can execute kubectl commands\n\n[gray]Thinking...", question))
		})

		err = a.aiClient.AskWithTools(ctx, prompt, func(chunk string) {
			fullResponse.WriteString(chunk)
			response := fullResponse.String()
			a.QueueUpdateDraw(func() {
				a.aiPanel.SetText(fmt.Sprintf("[yellow]Q:[white] %s\n\n[cyan]🤖 Agentic Mode[white]\n\n[green]A:[white] %s", question, response))
			})
		}, func(toolName string, args string) bool {
			// Tool approval callback - kubectl-ai style Decision Required
			a.logger.Info("Tool callback invoked", "tool", toolName, "args", args)

			filter := ai.NewCommandFilter()

			// Parse command from args
			var cmdArgs struct {
				Command   string `json:"command"`
				Namespace string `json:"namespace,omitempty"`
			}
			if err := parseJSON(args, &cmdArgs); err != nil {
				a.logger.Error("Failed to parse tool args", "error", err, "args", args)
			}

			fullCmd := ""
			if toolName == "kubectl" {
				fullCmd = "kubectl " + cmdArgs.Command
				if cmdArgs.Namespace != "" && !strings.Contains(cmdArgs.Command, "-n ") {
					fullCmd = "kubectl -n " + cmdArgs.Namespace + " " + cmdArgs.Command
				}
			} else if toolName == "bash" {
				fullCmd = cmdArgs.Command
			}

			a.logger.Info("Analyzed command", "fullCmd", fullCmd)

			// Analyze command safety
			report := filter.AnalyzeCommand(fullCmd)

			// Read-only commands: auto-approve
			if report.Type == ai.CommandTypeReadOnly {
				return true
			}

			// Store current tool info for approval
			currentToolCallInfo.Name = toolName
			currentToolCallInfo.Args = args
			currentToolCallInfo.Command = fullCmd

			// Show Decision Required UI
			a.QueueUpdateDraw(func() {
				var sb strings.Builder
				sb.WriteString(fmt.Sprintf("[yellow]Q:[white] %s\n\n", question))
				sb.WriteString(fullResponse.String())
				sb.WriteString("\n\n[yellow::b]━━━ DECISION REQUIRED ━━━[white::-]\n\n")

				if report.IsDangerous {
					sb.WriteString("[red]⚠ DANGEROUS COMMAND[white]\n")
				} else if report.Type == ai.CommandTypeWrite {
					sb.WriteString("[yellow]? WRITE OPERATION[white]\n")
				} else {
					sb.WriteString("[gray]? COMMAND APPROVAL[white]\n")
				}

				sb.WriteString(fmt.Sprintf("\n[cyan]%s[white]\n\n", fullCmd))

				for _, w := range report.Warnings {
					sb.WriteString(fmt.Sprintf("[red]• %s[white]\n", w))
				}

				sb.WriteString("\n[gray]Press [green]Y[gray] or [green]Enter[gray] to approve, [red]N[gray] or [red]Esc[gray] to cancel[white]")
				a.aiPanel.SetText(sb.String())

				// Focus AI panel for key input
				a.SetFocus(a.aiPanel)
			})

			// Wait for user decision (blocking)
			// Clear any pending approvals first
			select {
			case <-pendingToolApproval:
			default:
			}

			// Wait for approval with timeout
			select {
			case approved := <-pendingToolApproval:
				if approved {
					a.QueueUpdateDraw(func() {
						currentText := a.aiPanel.GetText(false)
						a.aiPanel.SetText(currentText + "\n\n[green]✓ Approved - Executing...[white]")
					})
				} else {
					a.QueueUpdateDraw(func() {
						currentText := a.aiPanel.GetText(false)
						a.aiPanel.SetText(currentText + "\n\n[red]✗ Cancelled by user[white]")
					})
				}
				return approved
			case <-ctx.Done():
				return false
			}
		})
	} else {
		// Fallback to regular streaming
		err = a.aiClient.Ask(ctx, prompt, func(chunk string) {
			fullResponse.WriteString(chunk)
			response := fullResponse.String()
			a.QueueUpdateDraw(func() {
				a.aiPanel.SetText(fmt.Sprintf("[yellow]Q:[white] %s\n\n[green]A:[white] %s", question, response))
			})
		})
	}

	if err != nil {
		a.QueueUpdateDraw(func() {
			a.aiPanel.SetText(fmt.Sprintf("[yellow]Q:[white] %s\n\n[red]Error:[white] %v", question, err))
		})
		return
	}

	// After response complete, analyze for commands that need approval (fallback mode)
	if !a.aiClient.SupportsTools() {
		finalResponse := fullResponse.String()
		a.analyzeAndShowDecisions(question, finalResponse)
	}
}

// parseJSON is a helper to parse JSON arguments
func parseJSON(jsonStr string, v interface{}) error {
	return jsonUnmarshal([]byte(jsonStr), v)
}

// jsonUnmarshal wraps json.Unmarshal
var jsonUnmarshal = func(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}

// analyzeAndShowDecisions extracts commands from AI response and shows decision UI
func (a *App) analyzeAndShowDecisions(question, response string) {
	// Extract kubectl commands from response
	commands := ai.ExtractKubectlCommands(response)
	if len(commands) == 0 {
		return
	}

	// Analyze commands for safety
	filter := ai.NewCommandFilter()
	pendingDecisions = nil

	var hasDecisions bool
	for _, cmd := range commands {
		report := filter.AnalyzeCommand(cmd)
		if report.RequiresConfirmation || report.IsDangerous {
			hasDecisions = true
			pendingDecisions = append(pendingDecisions, PendingDecision{
				Command:     cmd,
				Description: getCommandDescription(cmd),
				IsDangerous: report.IsDangerous,
				Warnings:    report.Warnings,
			})
		}
	}

	if !hasDecisions {
		return
	}

	// Update AI panel with decision prompt
	a.QueueUpdateDraw(func() {
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("[yellow]Q:[white] %s\n\n", question))
		sb.WriteString(fmt.Sprintf("[green]A:[white] %s\n\n", response))
		sb.WriteString("[yellow::b]━━━ DECISION REQUIRED ━━━[white::-]\n\n")

		for i, decision := range pendingDecisions {
			if decision.IsDangerous {
				sb.WriteString(fmt.Sprintf("[red]⚠ [%d] DANGEROUS:[white] ", i+1))
			} else {
				sb.WriteString(fmt.Sprintf("[yellow]? [%d] Confirm:[white] ", i+1))
			}
			sb.WriteString(fmt.Sprintf("[cyan]%s[white]\n", decision.Command))

			for _, warning := range decision.Warnings {
				sb.WriteString(fmt.Sprintf("   [red]• %s[white]\n", warning))
			}
			sb.WriteString("\n")
		}

		sb.WriteString("[gray]Press [yellow]1-9[gray] to execute, [yellow]A[gray] to execute all, [yellow]Esc[gray] to cancel[white]")
		a.aiPanel.SetText(sb.String())
	})
}

// getCommandDescription returns a brief description of the command
func getCommandDescription(cmd string) string {
	parts := strings.Fields(cmd)
	if len(parts) < 2 {
		return cmd
	}

	// Skip "kubectl" if present
	if parts[0] == "kubectl" {
		parts = parts[1:]
	}

	if len(parts) == 0 {
		return cmd
	}

	switch parts[0] {
	case "delete":
		return "Delete resource"
	case "apply":
		return "Apply configuration"
	case "create":
		return "Create resource"
	case "scale":
		return "Scale resource"
	case "rollout":
		return "Rollout operation"
	case "patch":
		return "Patch resource"
	case "edit":
		return "Edit resource"
	case "drain":
		return "Drain node"
	case "cordon":
		return "Cordon node"
	case "uncordon":
		return "Uncordon node"
	default:
		return parts[0]
	}
}

// executeDecision executes a specific pending decision by index
func (a *App) executeDecision(idx int) {
	if idx < 0 || idx >= len(pendingDecisions) {
		return
	}

	decision := pendingDecisions[idx]
	a.flashMsg(fmt.Sprintf("Executing: %s", decision.Command), false)

	// Execute the command
	cmd := exec.Command("bash", "-c", decision.Command)
	output, err := cmd.CombinedOutput()

	// Update AI panel with result
	a.QueueUpdateDraw(func() {
		var result string
		if err != nil {
			result = fmt.Sprintf("[red]Error:[white] %v\n%s", err, string(output))
		} else {
			result = fmt.Sprintf("[green]Success:[white]\n%s", string(output))
		}

		// Show execution result
		currentText := a.aiPanel.GetText(false)
		a.aiPanel.SetText(currentText + "\n\n[yellow]━━━ EXECUTION RESULT ━━━[white]\n" +
			fmt.Sprintf("[cyan]%s[white]\n%s", decision.Command, result))
	})

	// Remove executed decision
	pendingDecisions = append(pendingDecisions[:idx], pendingDecisions[idx+1:]...)

	// Refresh if it was a modifying command
	go a.refresh()
}

// executeAllDecisions executes all pending decisions
func (a *App) executeAllDecisions() {
	if len(pendingDecisions) == 0 {
		return
	}

	// Show confirmation for dangerous commands
	hasDangerous := false
	for _, d := range pendingDecisions {
		if d.IsDangerous {
			hasDangerous = true
			break
		}
	}

	if hasDangerous {
		modal := tview.NewModal().
			SetText("[red]WARNING:[white] Some commands are dangerous!\n\nAre you sure you want to execute ALL commands?").
			AddButtons([]string{"Cancel", "Execute All"}).
			SetDoneFunc(func(buttonIndex int, buttonLabel string) {
				a.pages.RemovePage("confirm-all")
				if buttonLabel == "Execute All" {
					go a.doExecuteAll()
				}
			})
		modal.SetBackgroundColor(tcell.ColorDarkRed)
		a.pages.AddPage("confirm-all", modal, true, true)
	} else {
		go a.doExecuteAll()
	}
}

// doExecuteAll actually executes all pending decisions
func (a *App) doExecuteAll() {
	decisions := make([]PendingDecision, len(pendingDecisions))
	copy(decisions, pendingDecisions)
	pendingDecisions = nil

	var results strings.Builder
	results.WriteString("\n\n[yellow]━━━ BATCH EXECUTION RESULTS ━━━[white]\n")

	for _, decision := range decisions {
		a.flashMsg(fmt.Sprintf("Executing: %s", decision.Command), false)

		cmd := exec.Command("bash", "-c", decision.Command)
		output, err := cmd.CombinedOutput()

		results.WriteString(fmt.Sprintf("\n[cyan]%s[white]\n", decision.Command))
		if err != nil {
			results.WriteString(fmt.Sprintf("[red]Error:[white] %v\n%s\n", err, string(output)))
		} else {
			results.WriteString(fmt.Sprintf("[green]Success:[white] %s\n", strings.TrimSpace(string(output))))
		}
	}

	a.QueueUpdateDraw(func() {
		currentText := a.aiPanel.GetText(false)
		a.aiPanel.SetText(currentText + results.String())
	})

	a.flashMsg(fmt.Sprintf("Executed %d commands", len(decisions)), false)
	go a.refresh()
}

// setupAutocomplete configures the command input with autocomplete
func (a *App) setupAutocomplete() {
	// Track current suggestions
	var suggestions []string
	var selectedIdx int

	// Update hint as user types
	a.cmdInput.SetChangedFunc(func(text string) {
		suggestions = a.getCompletions(text)
		selectedIdx = 0

		if len(suggestions) > 0 && text != "" {
			// Show dimmed hint for first suggestion
			hint := suggestions[0]
			if strings.HasPrefix(hint, text) {
				remaining := hint[len(text):]
				a.cmdHint.SetText("[gray]" + remaining)
			} else {
				a.cmdHint.SetText("[gray] → " + hint)
			}
		} else {
			a.cmdHint.SetText("")
		}
	})

	// Handle special keys
	a.cmdInput.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		text := a.cmdInput.GetText()

		switch event.Key() {
		case tcell.KeyTab:
			// Accept current suggestion
			if len(suggestions) > 0 {
				selected := suggestions[selectedIdx]
				// If it's a namespace command, add space
				if selected == "ns" || strings.HasPrefix(selected, "ns ") {
					a.cmdInput.SetText(selected + " ")
				} else {
					a.cmdInput.SetText(selected)
				}
				a.cmdHint.SetText("")
			}
			return nil

		case tcell.KeyDown:
			// Cycle through suggestions
			if len(suggestions) > 1 {
				selectedIdx = (selectedIdx + 1) % len(suggestions)
				hint := suggestions[selectedIdx]
				if strings.HasPrefix(hint, text) {
					remaining := hint[len(text):]
					a.cmdHint.SetText("[gray]" + remaining)
				} else {
					a.cmdHint.SetText("[gray] → " + hint)
				}
			}
			return nil

		case tcell.KeyUp:
			// Cycle through suggestions backwards
			if len(suggestions) > 1 {
				selectedIdx--
				if selectedIdx < 0 {
					selectedIdx = len(suggestions) - 1
				}
				hint := suggestions[selectedIdx]
				if strings.HasPrefix(hint, text) {
					remaining := hint[len(text):]
					a.cmdHint.SetText("[gray]" + remaining)
				} else {
					a.cmdHint.SetText("[gray] → " + hint)
				}
			}
			return nil

		case tcell.KeyEnter:
			cmd := text
			// If hint is showing and user didn't type full command, use suggestion
			if len(suggestions) > 0 && cmd != suggestions[selectedIdx] {
				// Check if input matches number for namespace selection
				if num, ok := a.parseNamespaceNumber(cmd); ok {
					a.selectNamespaceByNumber(num)
					a.cmdInput.SetText("")
					a.cmdHint.SetText("")
					a.cmdInput.SetLabel(" : ")
					a.SetFocus(a.table)
					return nil
				}
			}
			a.cmdInput.SetText("")
			a.cmdHint.SetText("")
			a.cmdInput.SetLabel(" : ")
			a.handleCommand(cmd)
			a.SetFocus(a.table)
			return nil

		case tcell.KeyEsc:
			a.cmdInput.SetText("")
			a.cmdHint.SetText("")
			a.cmdInput.SetLabel(" : ")
			a.SetFocus(a.table)
			return nil

		case tcell.KeyRune:
			// Check for number input (1-9) to select namespace
			if event.Rune() >= '0' && event.Rune() <= '9' && text == "" {
				// Show namespace hint
				a.showNamespaceHint()
			}
		}

		return event
	})

	a.cmdInput.SetDoneFunc(func(key tcell.Key) {
		// Already handled in InputCapture
	})
}

// getCompletions returns matching commands for the input
func (a *App) getCompletions(input string) []string {
	if input == "" {
		return nil
	}

	inputLower := strings.ToLower(input)
	var matches []string

	// Check for namespace command (ns <namespace>)
	if strings.HasPrefix(inputLower, "ns ") || strings.HasPrefix(inputLower, "namespace ") {
		prefix := strings.TrimPrefix(inputLower, "ns ")
		prefix = strings.TrimPrefix(prefix, "namespace ")

		a.mx.RLock()
		namespaces := a.namespaces
		a.mx.RUnlock()

		for _, ns := range namespaces {
			if ns == "" {
				continue
			}
			if strings.HasPrefix(ns, prefix) {
				matches = append(matches, "ns "+ns)
			}
		}
		return matches
	}

	// Check for resource command with -n flag (e.g., "pods -n kube")
	if strings.Contains(inputLower, " -n ") {
		parts := strings.Split(input, " -n ")
		if len(parts) == 2 {
			resourcePart := strings.TrimSpace(parts[0])
			nsPrefix := strings.TrimSpace(parts[1])

			a.mx.RLock()
			namespaces := a.namespaces
			a.mx.RUnlock()

			for _, ns := range namespaces {
				if ns == "" {
					continue
				}
				if strings.HasPrefix(ns, nsPrefix) {
					matches = append(matches, resourcePart+" -n "+ns)
				}
			}
			return matches
		}
	}

	// Check if input ends with "-n " - suggest namespaces
	if strings.HasSuffix(inputLower, "-n ") || strings.HasSuffix(inputLower, "-n") {
		basePart := strings.TrimSuffix(input, " ")
		if !strings.HasSuffix(basePart, " ") {
			basePart = strings.TrimSuffix(basePart, "-n") + "-n "
		}

		a.mx.RLock()
		namespaces := a.namespaces
		a.mx.RUnlock()

		for _, ns := range namespaces {
			if ns == "" {
				matches = append(matches, basePart+"all")
			} else {
				matches = append(matches, basePart+ns)
			}
		}
		// Limit suggestions
		if len(matches) > 10 {
			matches = matches[:10]
		}
		return matches
	}

	// Match built-in commands first
	for _, cmd := range commands {
		if strings.HasPrefix(cmd.name, inputLower) || strings.HasPrefix(cmd.alias, inputLower) {
			matches = append(matches, cmd.name)
		}
	}

	// Also match API resources from cluster (including CRDs)
	a.mx.RLock()
	apiResources := a.apiResources
	a.mx.RUnlock()

	seen := make(map[string]bool)
	for _, m := range matches {
		seen[m] = true
	}

	for _, res := range apiResources {
		if seen[res.Name] {
			continue
		}
		if strings.HasPrefix(res.Name, inputLower) {
			matches = append(matches, res.Name)
			seen[res.Name] = true
		}
		// Check short names
		for _, short := range res.ShortNames {
			if strings.HasPrefix(short, inputLower) && !seen[res.Name] {
				matches = append(matches, res.Name)
				seen[res.Name] = true
				break
			}
		}
	}

	return matches
}

// showNamespaceHint shows numbered namespace list in hint
func (a *App) showNamespaceHint() {
	a.mx.RLock()
	namespaces := a.namespaces
	a.mx.RUnlock()

	if len(namespaces) <= 1 {
		return
	}

	var hints []string
	for i, ns := range namespaces {
		if i == 0 {
			hints = append(hints, fmt.Sprintf("[gray]0[darkgray]:all"))
		} else if i <= 9 {
			hints = append(hints, fmt.Sprintf("[gray]%d[darkgray]:%s", i, ns))
		}
	}

	a.cmdHint.SetText(strings.Join(hints, " "))
}

// parseNamespaceNumber parses input as namespace number
func (a *App) parseNamespaceNumber(input string) (int, bool) {
	if len(input) != 1 {
		return 0, false
	}
	if input[0] >= '0' && input[0] <= '9' {
		return int(input[0] - '0'), true
	}
	return 0, false
}

// switchToAllNamespaces switches to all namespaces (k9s style: 0 key)
func (a *App) switchToAllNamespaces() {
	a.mx.Lock()
	a.currentNamespace = ""
	a.mx.Unlock()

	a.flashMsg("Switched to: all namespaces", false)
	a.updateHeader()
	a.refresh()
}

// selectNamespaceByNumber selects namespace by number (for command mode)
func (a *App) selectNamespaceByNumber(num int) {
	a.mx.Lock()

	if num >= len(a.namespaces) {
		a.mx.Unlock()
		a.flashMsg(fmt.Sprintf("Namespace %d not available (max: %d)", num, len(a.namespaces)-1), true)
		return
	}

	a.currentNamespace = a.namespaces[num]
	nsName := a.namespaces[num]
	if nsName == "" {
		nsName = "all"
	}
	a.mx.Unlock()

	a.flashMsg(fmt.Sprintf("Switched to namespace: %s", nsName), false)
	a.updateHeader()
	a.refresh()
}

// startFilter activates filter mode
func (a *App) startFilter() {
	a.cmdInput.SetLabel(" / ")
	a.cmdHint.SetText("[gray]Type to filter (use /regex/ for regex), Enter to confirm, Esc to clear")
	a.cmdInput.SetText(a.filterText)
	a.SetFocus(a.cmdInput)

	// Override input handler for filter mode
	a.cmdInput.SetChangedFunc(func(text string) {
		a.applyFilterText(text)
	})

	a.cmdInput.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEnter:
			text := a.cmdInput.GetText()
			a.mx.Lock()
			// Check for regex pattern /pattern/
			if strings.HasPrefix(text, "/") && strings.HasSuffix(text, "/") && len(text) > 2 {
				a.filterText = text[1 : len(text)-1]
				a.filterRegex = true
			} else {
				a.filterText = text
				a.filterRegex = false
			}
			a.mx.Unlock()
			a.cmdInput.SetLabel(" : ")
			a.cmdHint.SetText("")
			a.restoreAutocompleteHandler()
			a.SetFocus(a.table)
			return nil

		case tcell.KeyEsc:
			a.mx.Lock()
			a.filterText = ""
			a.filterRegex = false
			a.mx.Unlock()
			a.cmdInput.SetText("")
			a.cmdInput.SetLabel(" : ")
			a.cmdHint.SetText("")
			a.applyFilterText("")
			a.restoreAutocompleteHandler()
			a.SetFocus(a.table)
			return nil
		}
		return event
	})
}

// applyFilterText filters the table based on the given text with regex support (k9s style)
func (a *App) applyFilterText(filter string) {
	a.mx.RLock()
	headers := a.tableHeaders
	rows := a.tableRows
	a.mx.RUnlock()

	if len(headers) == 0 || len(rows) == 0 {
		return
	}

	// Check for regex pattern /pattern/
	isRegex := false
	filterPattern := filter
	if strings.HasPrefix(filter, "/") && strings.HasSuffix(filter, "/") && len(filter) > 2 {
		filterPattern = filter[1 : len(filter)-1]
		isRegex = true
	}

	// Compile regex if needed
	var re *regexp.Regexp
	var err error
	if isRegex && filterPattern != "" {
		re, err = regexp.Compile("(?i)" + filterPattern) // Case insensitive
		if err != nil {
			// Invalid regex, treat as plain text
			isRegex = false
			filterPattern = filter
		}
	}

	filterLower := strings.ToLower(filterPattern)

	a.QueueUpdateDraw(func() {
		a.table.Clear()

		// Set headers
		for i, h := range headers {
			cell := tview.NewTableCell(h).
				SetTextColor(tcell.ColorYellow).
				SetAttributes(tcell.AttrBold).
				SetSelectable(false).
				SetExpansion(1)
			a.table.SetCell(0, i, cell)
		}

		// Filter and set rows
		rowIdx := 1
		for _, row := range rows {
			if filterPattern != "" {
				match := false
				for _, cell := range row {
					if isRegex && re != nil {
						if re.MatchString(cell) {
							match = true
							break
						}
					} else {
						if strings.Contains(strings.ToLower(cell), filterLower) {
							match = true
							break
						}
					}
				}
				if !match {
					continue
				}
			}

			for c, text := range row {
				color := tcell.ColorWhite
				if c == 2 { // Usually status column
					color = a.statusColor(text)
				}
				// Highlight matching text
				displayText := text
				if filterPattern != "" {
					if isRegex && re != nil {
						displayText = a.highlightRegexMatch(text, re)
					} else if strings.Contains(strings.ToLower(text), filterLower) {
						displayText = a.highlightMatch(text, filterLower)
					}
				}
				cell := tview.NewTableCell(displayText).
					SetTextColor(color).
					SetExpansion(1)
				a.table.SetCell(rowIdx, c, cell)
			}
			rowIdx++
		}

		a.mx.RLock()
		resource := a.currentResource
		a.mx.RUnlock()

		filterInfo := ""
		if filter != "" {
			if isRegex {
				filterInfo = fmt.Sprintf(" [regex: %s]", filterPattern)
			} else {
				filterInfo = fmt.Sprintf(" [filter: %s]", filter)
			}
		}
		a.table.SetTitle(fmt.Sprintf(" %s (%d/%d)%s ", resource, rowIdx-1, len(rows), filterInfo))

		if rowIdx > 1 {
			a.table.Select(1, 0)
		}
	})
}

// highlightMatch wraps matching text with color tags
func (a *App) highlightMatch(text, filter string) string {
	lower := strings.ToLower(text)
	idx := strings.Index(lower, filter)
	if idx < 0 {
		return text
	}

	before := text[:idx]
	match := text[idx : idx+len(filter)]
	after := text[idx+len(filter):]

	return before + "[yellow]" + match + "[white]" + after
}

// highlightRegexMatch wraps regex-matching text with color tags (k9s style)
func (a *App) highlightRegexMatch(text string, re *regexp.Regexp) string {
	if re == nil {
		return text
	}
	matches := re.FindAllStringIndex(text, -1)
	if len(matches) == 0 {
		return text
	}

	var result strings.Builder
	lastEnd := 0
	for _, match := range matches {
		start, end := match[0], match[1]
		result.WriteString(text[lastEnd:start])
		result.WriteString("[yellow]")
		result.WriteString(text[start:end])
		result.WriteString("[white]")
		lastEnd = end
	}
	result.WriteString(text[lastEnd:])
	return result.String()
}

// restoreAutocompleteHandler restores the default autocomplete behavior
func (a *App) restoreAutocompleteHandler() {
	a.setupAutocomplete()
}

// setupKeybindings configures keyboard shortcuts (k9s compatible)
func (a *App) setupKeybindings() {
	a.table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyRune:
			switch event.Rune() {
			case 'q':
				a.Stop()
				return nil
			case ':':
				a.SetFocus(a.cmdInput)
				return nil
			case '/':
				a.startFilter()
				return nil
			case '?':
				a.showHelp()
				return nil
			case 'r':
				go a.refresh()
				return nil
			case 'n':
				a.cycleNamespace()
				return nil
			// k9s style: 0 = all namespaces
			case '0':
				go a.switchToAllNamespaces()
				return nil
			case 'l':
				a.showLogs()
				return nil
			case 'p':
				a.showLogsPrevious()
				return nil
			case 'd':
				a.showDescribe() // k9s: d = describe
				return nil
			case 'y':
				a.showYAML() // k9s: y = yaml
				return nil
			case 'e':
				a.editResource() // k9s: e = edit
				return nil
			case 's':
				a.execShell() // k9s: s = shell
				return nil
			case 'a':
				a.attachContainer() // k9s: a = attach
				return nil
			case 'c':
				a.showContextSwitcher() // context switcher
				return nil
			case 'g':
				a.table.Select(1, 0) // go to top
				return nil
			case 'G':
				a.table.Select(a.table.GetRowCount()-1, 0) // go to bottom
				return nil
			case 'u':
				a.useNamespace() // k9s: u = use namespace
				return nil
			case 'o':
				a.showNode() // k9s: o = show node (for pods)
				return nil
			case 'k':
				a.killPod() // k9s: k or Ctrl+K = kill pod
				return nil
			case 'b':
				a.showBenchmark() // k9s: b = benchmark (services)
				return nil
			case 't':
				a.triggerCronJob() // k9s: t = trigger (cronjobs)
				return nil
			case 'z':
				a.showRelatedResource() // k9s: z = zoom (show related)
				return nil
			case 'F':
				a.portForward() // k9s: Shift+F = port-forward
				return nil
			case 'S':
				a.scaleResource() // k9s: Shift+S = scale
				return nil
			case 'R':
				a.restartResource() // k9s: Shift+R = restart
				return nil
			case ' ':
				a.toggleSelection() // k9s: Space = toggle selection (multi-select)
				return nil
			}
		case tcell.KeyTab:
			if a.showAIPanel {
				a.SetFocus(a.aiInput)
			}
			return nil
		case tcell.KeyEnter:
			a.drillDown() // k9s: Enter = drill down to related resource
			return nil
		case tcell.KeyEsc:
			a.goBack() // k9s: Esc = go back
			return nil
		case tcell.KeyCtrlD:
			a.confirmDelete() // k9s: Ctrl+D = delete
			return nil
		case tcell.KeyCtrlK:
			a.killPod() // k9s: Ctrl+K = kill pod
			return nil
		case tcell.KeyCtrlU:
			a.pageUp() // k9s: Ctrl+U = page up
			return nil
		case tcell.KeyCtrlF:
			a.pageDown() // k9s: Ctrl+F = page down (vim style)
			return nil
		case tcell.KeyCtrlB:
			a.pageUp() // k9s: Ctrl+B = page up (vim style)
			return nil
		case tcell.KeyCtrlC:
			a.Stop()
			return nil
		}
		return event
	})

	a.aiPanel.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyTab:
			a.SetFocus(a.aiInput)
			return nil
		case tcell.KeyEnter:
			// Approve pending MCP tool call
			if currentToolCallInfo.Command != "" {
				select {
				case pendingToolApproval <- true:
					currentToolCallInfo = struct {
						Name    string
						Args    string
						Command string
					}{}
				default:
				}
				return nil
			}
		case tcell.KeyEsc:
			// Cancel pending MCP tool call
			if currentToolCallInfo.Command != "" {
				select {
				case pendingToolApproval <- false:
					currentToolCallInfo = struct {
						Name    string
						Args    string
						Command string
					}{}
				default:
				}
				a.SetFocus(a.table)
				return nil
			}
			// Clear pending decisions when escaping
			if len(pendingDecisions) > 0 {
				pendingDecisions = nil
				a.flashMsg("Cancelled pending commands", false)
			}
			a.SetFocus(a.table)
			return nil
		case tcell.KeyRune:
			// Handle Y/N for MCP tool approval (kubectl-ai style)
			if currentToolCallInfo.Command != "" {
				switch event.Rune() {
				case 'y', 'Y':
					select {
					case pendingToolApproval <- true:
						currentToolCallInfo = struct {
							Name    string
							Args    string
							Command string
						}{}
					default:
					}
					return nil
				case 'n', 'N':
					select {
					case pendingToolApproval <- false:
						currentToolCallInfo = struct {
							Name    string
							Args    string
							Command string
						}{}
					default:
					}
					a.SetFocus(a.table)
					return nil
				}
			}
			// Handle decision input (1-9 to execute command, A to execute all)
			if len(pendingDecisions) > 0 {
				switch event.Rune() {
				case '1', '2', '3', '4', '5', '6', '7', '8', '9':
					idx := int(event.Rune() - '1')
					if idx < len(pendingDecisions) {
						go a.executeDecision(idx)
					}
					return nil
				case 'a', 'A':
					go a.executeAllDecisions()
					return nil
				}
			}
		}
		return event
	})
}

// flash displays a temporary message (k9s pattern)
func (a *App) flashMsg(msg string, isError bool) {
	color := "[green]"
	if isError {
		color = "[red]"
	}
	a.QueueUpdateDraw(func() {
		a.flash.SetText(color + msg + "[white]")
	})

	// Clear after 3 seconds
	go func() {
		time.Sleep(3 * time.Second)
		a.QueueUpdateDraw(func() {
			a.flash.SetText("")
		})
	}()
}

// updateHeader updates the header text (thread-safe)
func (a *App) updateHeader() {
	ctxName := "N/A"
	cluster := "N/A"
	if a.k8s != nil {
		var err error
		ctxName, cluster, _, err = a.k8s.GetContextInfo()
		if err != nil {
			ctxName = "N/A"
			cluster = "N/A"
		}
	}

	a.mx.RLock()
	ns := a.currentNamespace
	resource := a.currentResource
	a.mx.RUnlock()

	if ns == "" {
		ns = "[green]all[white]"
	} else {
		ns = "[green]" + ns + "[white]"
	}

	aiStatus := "[red]Offline[white]"
	if a.aiClient != nil && a.aiClient.IsReady() {
		aiStatus = "[green]Online[white]"
	}

	header := fmt.Sprintf(
		" [yellow::b]k13s[white::-] - Kubernetes AI Dashboard                                    AI: %s\n"+
			" [gray]Context:[white] %s  [gray]Cluster:[white] %s\n"+
			" [gray]Namespace:[white] %s  [gray]Resource:[white] [cyan]%s[white]",
		aiStatus, ctxName, cluster, ns, resource,
	)

	// Use QueueUpdateDraw only after Application.Run() has started (k9s pattern)
	if atomic.LoadInt32(&a.running) == 1 {
		a.QueueUpdateDraw(func() {
			a.header.SetText(header)
		})
	} else {
		// Direct update during initialization (before Run())
		a.header.SetText(header)
	}
}

// updateStatusBar updates the status bar (k9s style)
func (a *App) updateStatusBar() {
	a.mx.RLock()
	resource := a.currentResource
	a.mx.RUnlock()

	// k9s style status bar: show key shortcuts
	shortcuts := "[yellow]<n>[white]NS [yellow]<0>[white]All [yellow]</>[white]Filter [yellow]<:>[white]Cmd [yellow]<?>[white]Help [yellow]<q>[white]Quit"

	// Add resource-specific shortcuts
	switch resource {
	case "pods", "po":
		shortcuts = "[yellow]<l>[white]Logs [yellow]<s>[white]Shell [yellow]<d>[white]Describe " + shortcuts
	case "deployments", "deploy", "statefulsets", "sts", "daemonsets", "ds":
		shortcuts = "[yellow]<S>[white]Scale [yellow]<R>[white]Restart [yellow]<d>[white]Describe " + shortcuts
	case "namespaces", "ns":
		shortcuts = "[yellow]<u>[white]Use " + shortcuts
	default:
		shortcuts = "[yellow]<d>[white]Describe [yellow]<y>[white]YAML " + shortcuts
	}

	a.statusBar.SetText(shortcuts)
}

// prepareContext cancels previous operations and creates new context (k9s pattern)
func (a *App) prepareContext() context.Context {
	a.cancelLock.Lock()
	defer a.cancelLock.Unlock()

	if a.cancelFn != nil {
		a.cancelFn() // Cancel previous operation
	}

	ctx, cancel := context.WithCancel(context.Background())
	a.cancelFn = cancel
	return ctx
}

// refresh reloads the current resource list with atomic guard (k9s pattern)
func (a *App) refresh() {
	// Atomic guard to prevent concurrent updates (k9s pattern)
	if !atomic.CompareAndSwapInt32(&a.inUpdate, 0, 1) {
		a.logger.Debug("Dropping refresh - update already in progress")
		return
	}
	defer atomic.StoreInt32(&a.inUpdate, 0)

	ctx := a.prepareContext()

	a.mx.RLock()
	resource := a.currentResource
	a.mx.RUnlock()

	// Show loading state
	a.QueueUpdateDraw(func() {
		a.table.Clear()
		a.table.SetTitle(fmt.Sprintf(" %s - Loading... ", resource))
		a.table.SetCell(0, 0, tview.NewTableCell("Loading...").SetTextColor(tcell.ColorYellow))
	})

	// Fetch with exponential backoff (k9s pattern)
	var headers []string
	var rows [][]string
	var fetchErr error

	bf := backoff.NewExponentialBackOff()
	bf.InitialInterval = 300 * time.Millisecond
	bf.MaxElapsedTime = 10 * time.Second

	err := backoff.Retry(func() error {
		select {
		case <-ctx.Done():
			return backoff.Permanent(ctx.Err())
		default:
		}

		headers, rows, fetchErr = a.fetchResources(ctx)
		if fetchErr != nil {
			a.logger.Warn("Fetch failed, retrying", "error", fetchErr, "resource", resource)
			return fetchErr
		}
		return nil
	}, backoff.WithContext(bf, ctx))

	if err != nil {
		a.logger.Error("Fetch failed after retries", "error", err, "resource", resource)
		a.flashMsg(fmt.Sprintf("Error: %v", err), true)
		a.QueueUpdateDraw(func() {
			a.table.Clear()
			a.table.SetTitle(fmt.Sprintf(" %s - Error ", resource))
			a.table.SetCell(0, 0, tview.NewTableCell(fmt.Sprintf("Error: %v", err)).SetTextColor(tcell.ColorRed))
		})
		return
	}

	// Store original data for filtering
	a.mx.Lock()
	a.tableHeaders = headers
	a.tableRows = rows
	currentFilter := a.filterText
	a.mx.Unlock()

	// Apply filter if active, otherwise show all
	if currentFilter != "" {
		a.applyFilterText(currentFilter)
	} else {
		// Update UI (k9s pattern: queue all UI updates)
		a.QueueUpdateDraw(func() {
			a.table.Clear()

			// Set headers
			for i, h := range headers {
				cell := tview.NewTableCell(h).
					SetTextColor(tcell.ColorYellow).
					SetAttributes(tcell.AttrBold).
					SetSelectable(false).
					SetExpansion(1)
				a.table.SetCell(0, i, cell)
			}

			// Set rows
			for r, row := range rows {
				for c, text := range row {
					color := tcell.ColorWhite
					if c == 2 { // Usually status column
						color = a.statusColor(text)
					}
					cell := tview.NewTableCell(text).
						SetTextColor(color).
						SetExpansion(1)
					a.table.SetCell(r+1, c, cell)
				}
			}

			count := len(rows)
			a.table.SetTitle(fmt.Sprintf(" %s (%d) ", resource, count))

			if count > 0 {
				a.table.Select(1, 0)
			}
		})
	}

	// Update status bar for resource-specific shortcuts
	a.QueueUpdateDraw(func() {
		a.updateStatusBar()
	})

	a.logger.Info("Refresh completed", "resource", resource, "count", len(rows))
}

// fetchResources gets resources from K8s API
func (a *App) fetchResources(ctx context.Context) ([]string, [][]string, error) {
	if a.k8s == nil {
		return nil, nil, fmt.Errorf("K8s client not available")
	}

	a.mx.RLock()
	resource := a.currentResource
	ns := a.currentNamespace
	a.mx.RUnlock()

	switch resource {
	case "pods":
		return a.fetchPods(ctx, ns)
	case "deployments":
		return a.fetchDeployments(ctx, ns)
	case "services":
		return a.fetchServices(ctx, ns)
	case "nodes":
		return a.fetchNodes(ctx)
	case "namespaces":
		return a.fetchNamespaces(ctx)
	case "events":
		return a.fetchEvents(ctx, ns)
	case "configmaps":
		return a.fetchConfigMaps(ctx, ns)
	case "secrets":
		return a.fetchSecrets(ctx, ns)
	case "persistentvolumes":
		return a.fetchPersistentVolumes(ctx)
	case "persistentvolumeclaims":
		return a.fetchPersistentVolumeClaims(ctx, ns)
	case "storageclasses":
		return a.fetchStorageClasses(ctx)
	case "replicasets":
		return a.fetchReplicaSets(ctx, ns)
	case "daemonsets":
		return a.fetchDaemonSets(ctx, ns)
	case "statefulsets":
		return a.fetchStatefulSets(ctx, ns)
	case "jobs":
		return a.fetchJobs(ctx, ns)
	case "cronjobs":
		return a.fetchCronJobs(ctx, ns)
	case "replicationcontrollers":
		return a.fetchReplicationControllers(ctx, ns)
	case "ingresses":
		return a.fetchIngresses(ctx, ns)
	case "endpoints":
		return a.fetchEndpoints(ctx, ns)
	case "networkpolicies":
		return a.fetchNetworkPolicies(ctx, ns)
	case "serviceaccounts":
		return a.fetchServiceAccounts(ctx, ns)
	case "roles":
		return a.fetchRoles(ctx, ns)
	case "rolebindings":
		return a.fetchRoleBindings(ctx, ns)
	case "clusterroles":
		return a.fetchClusterRoles(ctx)
	case "clusterrolebindings":
		return a.fetchClusterRoleBindings(ctx)
	case "poddisruptionbudgets":
		return a.fetchPodDisruptionBudgets(ctx, ns)
	case "limitranges":
		return a.fetchLimitRanges(ctx, ns)
	case "resourcequotas":
		return a.fetchResourceQuotas(ctx, ns)
	case "horizontalpodautoscalers":
		return a.fetchHPAs(ctx, ns)
	case "customresourcedefinitions":
		return a.fetchCRDs(ctx)
	default:
		// Try generic fetch for unknown resources
		return a.fetchGenericResource(ctx, resource, ns)
	}
}

func (a *App) fetchPods(ctx context.Context, ns string) ([]string, [][]string, error) {
	headers := []string{"NAMESPACE", "NAME", "STATUS", "READY", "RESTARTS", "AGE"}
	pods, err := a.k8s.ListPods(ctx, ns)
	if err != nil {
		return headers, nil, err
	}

	var rows [][]string
	for _, p := range pods {
		ready := 0
		total := len(p.Status.ContainerStatuses)
		var restarts int32
		for _, cs := range p.Status.ContainerStatuses {
			if cs.Ready {
				ready++
			}
			restarts += cs.RestartCount
		}

		status := string(p.Status.Phase)
		for _, cs := range p.Status.ContainerStatuses {
			if cs.State.Waiting != nil && cs.State.Waiting.Reason != "" {
				status = cs.State.Waiting.Reason
				break
			}
		}

		rows = append(rows, []string{
			p.Namespace,
			p.Name,
			status,
			fmt.Sprintf("%d/%d", ready, total),
			fmt.Sprintf("%d", restarts),
			formatAge(p.CreationTimestamp.Time),
		})
	}
	return headers, rows, nil
}

func (a *App) fetchDeployments(ctx context.Context, ns string) ([]string, [][]string, error) {
	headers := []string{"NAMESPACE", "NAME", "STATUS", "READY", "UP-TO-DATE", "AGE"}
	deps, err := a.k8s.ListDeployments(ctx, ns)
	if err != nil {
		return headers, nil, err
	}

	var rows [][]string
	for _, d := range deps {
		replicas := int32(1)
		if d.Spec.Replicas != nil {
			replicas = *d.Spec.Replicas
		}
		status := "Ready"
		if d.Status.ReadyReplicas < replicas {
			status = "Updating"
		}
		if d.Status.ReadyReplicas == 0 && replicas > 0 {
			status = "NotReady"
		}

		rows = append(rows, []string{
			d.Namespace,
			d.Name,
			status,
			fmt.Sprintf("%d/%d", d.Status.ReadyReplicas, replicas),
			fmt.Sprintf("%d", d.Status.UpdatedReplicas),
			formatAge(d.CreationTimestamp.Time),
		})
	}
	return headers, rows, nil
}

func (a *App) fetchServices(ctx context.Context, ns string) ([]string, [][]string, error) {
	headers := []string{"NAMESPACE", "NAME", "TYPE", "CLUSTER-IP", "PORTS", "AGE"}
	svcs, err := a.k8s.ListServices(ctx, ns)
	if err != nil {
		return headers, nil, err
	}

	var rows [][]string
	for _, s := range svcs {
		var ports []string
		for _, p := range s.Spec.Ports {
			ports = append(ports, fmt.Sprintf("%d/%s", p.Port, p.Protocol))
		}

		rows = append(rows, []string{
			s.Namespace,
			s.Name,
			string(s.Spec.Type),
			s.Spec.ClusterIP,
			strings.Join(ports, ","),
			formatAge(s.CreationTimestamp.Time),
		})
	}
	return headers, rows, nil
}

func (a *App) fetchNodes(ctx context.Context) ([]string, [][]string, error) {
	headers := []string{"NAME", "STATUS", "ROLES", "VERSION", "AGE"}
	nodes, err := a.k8s.ListNodes(ctx)
	if err != nil {
		return headers, nil, err
	}

	var rows [][]string
	for _, n := range nodes {
		status := "NotReady"
		for _, c := range n.Status.Conditions {
			if c.Type == corev1.NodeReady && c.Status == corev1.ConditionTrue {
				status = "Ready"
			}
		}

		roles := []string{}
		for label := range n.Labels {
			if strings.HasPrefix(label, "node-role.kubernetes.io/") {
				role := strings.TrimPrefix(label, "node-role.kubernetes.io/")
				roles = append(roles, role)
			}
		}
		if len(roles) == 0 {
			roles = []string{"<none>"}
		}

		rows = append(rows, []string{
			n.Name,
			status,
			strings.Join(roles, ","),
			n.Status.NodeInfo.KubeletVersion,
			formatAge(n.CreationTimestamp.Time),
		})
	}
	return headers, rows, nil
}

func (a *App) fetchNamespaces(ctx context.Context) ([]string, [][]string, error) {
	headers := []string{"NAME", "STATUS", "AGE"}
	nss, err := a.k8s.ListNamespaces(ctx)
	if err != nil {
		return headers, nil, err
	}

	// Cache namespaces for cycling (protected by mutex)
	a.mx.Lock()
	a.namespaces = []string{""}
	a.mx.Unlock()

	var rows [][]string
	for _, n := range nss {
		a.mx.Lock()
		a.namespaces = append(a.namespaces, n.Name)
		a.mx.Unlock()

		rows = append(rows, []string{
			n.Name,
			string(n.Status.Phase),
			formatAge(n.CreationTimestamp.Time),
		})
	}
	return headers, rows, nil
}

func (a *App) fetchEvents(ctx context.Context, ns string) ([]string, [][]string, error) {
	headers := []string{"NAMESPACE", "TYPE", "REASON", "OBJECT", "MESSAGE"}
	events, err := a.k8s.ListEvents(ctx, ns)
	if err != nil {
		return headers, nil, err
	}

	var rows [][]string
	for _, e := range events {
		msg := e.Message
		if len(msg) > 60 {
			msg = msg[:57] + "..."
		}
		rows = append(rows, []string{
			e.Namespace,
			e.Type,
			e.Reason,
			e.InvolvedObject.Name,
			msg,
		})
	}
	return headers, rows, nil
}

// statusColor returns color based on status
func (a *App) statusColor(status string) tcell.Color {
	switch status {
	case "Running", "Ready", "Active", "Succeeded", "Normal", "Completed":
		return tcell.ColorGreen
	case "Pending", "ContainerCreating", "Warning", "Updating":
		return tcell.ColorYellow
	case "Failed", "Error", "CrashLoopBackOff", "NotReady", "ImagePullBackOff", "ErrImagePull":
		return tcell.ColorRed
	default:
		return tcell.ColorWhite
	}
}

// setResource changes the current resource type (thread-safe)
func (a *App) setResource(resource string) {
	a.mx.Lock()
	a.currentResource = resource
	a.mx.Unlock()

	// Run both updateHeader and refresh in goroutine to avoid deadlock
	// when called from main event loop
	go func() {
		a.updateHeader()
		a.refresh()
	}()
}

// cycleNamespace cycles through namespaces (thread-safe)
func (a *App) cycleNamespace() {
	a.mx.Lock()
	if len(a.namespaces) == 0 {
		a.mx.Unlock()
		return
	}

	current := 0
	for i, n := range a.namespaces {
		if n == a.currentNamespace {
			current = i
			break
		}
	}

	next := (current + 1) % len(a.namespaces)
	a.currentNamespace = a.namespaces[next]
	a.mx.Unlock()

	// Run in goroutine to avoid deadlock when called from main event loop
	go func() {
		a.updateHeader()
		a.refresh()
	}()
}

// handleCommand processes command input
func (a *App) handleCommand(cmd string) {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return
	}

	// Handle namespace filtering
	if strings.HasPrefix(cmd, "ns ") || strings.HasPrefix(cmd, "namespace ") {
		parts := strings.Fields(cmd)
		if len(parts) >= 2 {
			a.mx.Lock()
			a.currentNamespace = parts[1]
			if parts[1] == "all" || parts[1] == "*" {
				a.currentNamespace = ""
			}
			a.mx.Unlock()
			// Run in goroutine to avoid deadlock
			go func() {
				a.updateHeader()
				a.refresh()
			}()
		}
		return
	}

	// Parse command with -n/--namespace flag (kubectl style: pods -n kube-system)
	parts := strings.Fields(cmd)
	resourceCmd := ""
	namespace := ""

	for i := 0; i < len(parts); i++ {
		part := parts[i]
		if part == "-n" || part == "--namespace" {
			if i+1 < len(parts) {
				namespace = parts[i+1]
				i++ // skip namespace value
			}
		} else if part == "-A" || part == "--all-namespaces" {
			namespace = "" // all namespaces
		} else if resourceCmd == "" {
			resourceCmd = part
		}
	}

	// If -n flag was used, switch namespace first
	if namespace != "" || strings.Contains(cmd, "-A") || strings.Contains(cmd, "--all-namespaces") {
		a.mx.Lock()
		if namespace == "all" || namespace == "*" || strings.Contains(cmd, "-A") {
			a.currentNamespace = ""
		} else if namespace != "" {
			a.currentNamespace = namespace
		}
		a.mx.Unlock()
	}

	// Use the resource command if found
	if resourceCmd != "" {
		// Use the commands list to handle all resource types dynamically
		for _, c := range commands {
			if resourceCmd == c.name || resourceCmd == c.alias {
				if c.category == "resource" {
					a.setResource(c.name)
					return
				}
			}
		}
	}

	// Fallback: Use the commands list for simple commands without flags
	for _, c := range commands {
		if cmd == c.name || cmd == c.alias {
			if c.category == "resource" {
				a.setResource(c.name)
				return
			}
		}
	}

	// Handle actions
	switch cmd {
	case "health", "status":
		a.showHealth()
	case "context", "ctx":
		a.showContextSwitcher()
	case "help", "?":
		a.showHelp()
	case "q", "quit", "exit":
		a.Stop()
	}
}

// showLogs shows logs for selected pod
func (a *App) showLogs() {
	a.mx.RLock()
	resource := a.currentResource
	a.mx.RUnlock()

	if resource != "pods" && resource != "po" {
		return
	}

	row, _ := a.table.GetSelection()
	if row <= 0 {
		return
	}

	ns := a.table.GetCell(row, 0).Text
	name := a.table.GetCell(row, 1).Text

	logView := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true)
	logView.SetBorder(true).
		SetTitle(fmt.Sprintf(" Logs: %s/%s (Press Esc to close) ", ns, name))

	a.pages.AddPage("logs", logView, true, true)
	a.SetFocus(logView)

	// Fetch logs
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		logs, err := a.k8s.GetPodLogs(ctx, ns, name, "", 100)
		a.QueueUpdateDraw(func() {
			if err != nil {
				logView.SetText(fmt.Sprintf("[red]Error: %v", err))
			} else if logs == "" {
				logView.SetText("[gray]No logs available")
			} else {
				logView.SetText(logs)
				logView.ScrollToEnd()
			}
		})
	}()

	logView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc {
			a.pages.RemovePage("logs")
			a.SetFocus(a.table)
			return nil
		}
		return event
	})
}

// describeResource shows YAML for selected resource
func (a *App) describeResource() {
	row, _ := a.table.GetSelection()
	if row <= 0 {
		return
	}

	a.mx.RLock()
	resource := a.currentResource
	a.mx.RUnlock()

	var ns, name string
	switch resource {
	case "nodes", "no", "namespaces", "ns":
		name = a.table.GetCell(row, 0).Text
	default:
		ns = a.table.GetCell(row, 0).Text
		name = a.table.GetCell(row, 1).Text
	}

	descView := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true)
	descView.SetBorder(true).
		SetTitle(fmt.Sprintf(" %s: %s (Press Esc to close) ", resource, name))

	a.pages.AddPage("describe", descView, true, true)
	a.SetFocus(descView)

	// Fetch YAML
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		gvr, ok := a.k8s.GetGVR(resource)
		if !ok {
			a.QueueUpdateDraw(func() {
				descView.SetText(fmt.Sprintf("[red]Unknown resource type: %s", resource))
			})
			return
		}

		yaml, err := a.k8s.GetResourceYAML(ctx, ns, name, gvr)
		a.QueueUpdateDraw(func() {
			if err != nil {
				descView.SetText(fmt.Sprintf("[red]Error: %v", err))
			} else {
				descView.SetText(yaml)
			}
		})
	}()

	descView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc {
			a.pages.RemovePage("describe")
			a.SetFocus(a.table)
			return nil
		}
		return event
	})
}

// confirmDelete shows a delete confirmation dialog
func (a *App) confirmDelete() {
	a.mx.RLock()
	resource := a.currentResource
	selectedCount := len(a.selectedRows)
	a.mx.RUnlock()

	// Check for multi-select deletion
	if selectedCount > 0 {
		a.confirmDeleteMultiple()
		return
	}

	row, _ := a.table.GetSelection()
	if row <= 0 {
		return
	}

	// Get resource info
	var ns, name string
	switch resource {
	case "nodes", "no", "namespaces", "ns":
		name = a.table.GetCell(row, 0).Text
	default:
		ns = a.table.GetCell(row, 0).Text
		name = a.table.GetCell(row, 1).Text
	}

	// Create confirmation modal
	modal := tview.NewModal().
		SetText(fmt.Sprintf("[red]Delete %s?[white]\n\n%s/%s\n\nThis action cannot be undone.", resource, ns, name)).
		AddButtons([]string{"Cancel", "Delete"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			a.pages.RemovePage("delete-confirm")
			a.SetFocus(a.table)

			if buttonLabel == "Delete" {
				go a.deleteResource(ns, name, resource)
			}
		})

	modal.SetBackgroundColor(tcell.ColorDarkRed)

	a.pages.AddPage("delete-confirm", modal, true, true)
}

// confirmDeleteMultiple confirms deletion of multiple selected resources (k9s style)
func (a *App) confirmDeleteMultiple() {
	a.mx.RLock()
	resource := a.currentResource
	selectedCount := len(a.selectedRows)
	selectedRowsCopy := make(map[int]bool)
	for k, v := range a.selectedRows {
		selectedRowsCopy[k] = v
	}
	a.mx.RUnlock()

	if selectedCount == 0 {
		return
	}

	// Build list of resources to delete
	var items []struct{ ns, name string }
	for row := range selectedRowsCopy {
		var ns, name string
		switch resource {
		case "nodes", "no", "namespaces", "ns":
			name = strings.TrimSpace(tview.TranslateANSI(a.table.GetCell(row, 0).Text))
		default:
			ns = strings.TrimSpace(tview.TranslateANSI(a.table.GetCell(row, 0).Text))
			name = strings.TrimSpace(tview.TranslateANSI(a.table.GetCell(row, 1).Text))
		}
		if name != "" {
			items = append(items, struct{ ns, name string }{ns, name})
		}
	}

	// Create confirmation modal
	modal := tview.NewModal().
		SetText(fmt.Sprintf("[red]Delete %d %s?[white]\n\nThis action cannot be undone.", len(items), resource)).
		AddButtons([]string{"Cancel", "Delete All"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			a.pages.RemovePage("delete-confirm")
			a.SetFocus(a.table)

			if buttonLabel == "Delete All" {
				go func() {
					for _, item := range items {
						a.deleteResource(item.ns, item.name, resource)
					}
					a.clearSelections()
					a.refresh()
				}()
			}
		})

	modal.SetBackgroundColor(tcell.ColorDarkRed)

	a.pages.AddPage("delete-confirm", modal, true, true)
}

// deleteResource deletes the specified resource
func (a *App) deleteResource(ns, name, resource string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	gvr, ok := a.k8s.GetGVR(resource)
	if !ok {
		a.flashMsg(fmt.Sprintf("Unknown resource type: %s", resource), true)
		return
	}

	a.flashMsg(fmt.Sprintf("Deleting %s/%s...", resource, name), false)

	err := a.k8s.DeleteResource(ctx, gvr, ns, name)
	if err != nil {
		a.flashMsg(fmt.Sprintf("Delete failed: %v", err), true)
		return
	}

	a.flashMsg(fmt.Sprintf("Deleted %s/%s", resource, name), false)
	go a.refresh()
}

// execShell opens an interactive shell in the selected pod
func (a *App) execShell() {
	a.mx.RLock()
	resource := a.currentResource
	a.mx.RUnlock()

	if resource != "pods" && resource != "po" {
		a.flashMsg("Shell only available for pods", true)
		return
	}

	row, _ := a.table.GetSelection()
	if row <= 0 {
		return
	}

	ns := a.table.GetCell(row, 0).Text
	name := a.table.GetCell(row, 1).Text

	// Suspend the TUI and run kubectl exec
	a.Suspend(func() {
		// Try bash first, fall back to sh
		cmd := exec.Command("kubectl", "exec", "-it", "-n", ns, name, "--", "/bin/bash")
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		err := cmd.Run()
		if err != nil {
			// Try sh if bash fails
			cmd = exec.Command("kubectl", "exec", "-it", "-n", ns, name, "--", "/bin/sh")
			cmd.Stdin = os.Stdin
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			cmd.Run()
		}
	})
}

// portForward shows port forwarding dialog
func (a *App) portForward() {
	a.mx.RLock()
	resource := a.currentResource
	a.mx.RUnlock()

	if resource != "pods" && resource != "po" && resource != "services" && resource != "svc" {
		a.flashMsg("Port forward only available for pods and services", true)
		return
	}

	row, _ := a.table.GetSelection()
	if row <= 0 {
		return
	}

	ns := a.table.GetCell(row, 0).Text
	name := a.table.GetCell(row, 1).Text

	// Create port forward dialog
	form := tview.NewForm()
	form.SetBorder(true).SetTitle(fmt.Sprintf(" Port Forward: %s/%s ", ns, name))

	var localPort, remotePort string
	form.AddInputField("Local Port:", "8080", 10, nil, func(text string) {
		localPort = text
	})
	form.AddInputField("Remote Port:", "80", 10, nil, func(text string) {
		remotePort = text
	})
	form.AddButton("Forward", func() {
		a.pages.RemovePage("port-forward")
		a.SetFocus(a.table)

		if localPort == "" || remotePort == "" {
			a.flashMsg("Both ports are required", true)
			return
		}

		go a.startPortForward(ns, name, resource, localPort, remotePort)
	})
	form.AddButton("Cancel", func() {
		a.pages.RemovePage("port-forward")
		a.SetFocus(a.table)
	})

	a.pages.AddPage("port-forward", centered(form, 50, 12), true, true)
}

// startPortForward starts port forwarding in background
func (a *App) startPortForward(ns, name, resource, localPort, remotePort string) {
	resourceType := "pod"
	if resource == "services" || resource == "svc" {
		resourceType = "svc"
	}

	target := fmt.Sprintf("%s/%s", resourceType, name)
	portMap := fmt.Sprintf("%s:%s", localPort, remotePort)

	a.flashMsg(fmt.Sprintf("Starting port forward %s -> %s:%s", localPort, name, remotePort), false)

	cmd := exec.Command("kubectl", "port-forward", "-n", ns, target, portMap)
	err := cmd.Start()
	if err != nil {
		a.flashMsg(fmt.Sprintf("Port forward failed: %v", err), true)
		return
	}

	a.flashMsg(fmt.Sprintf("Port forward active: localhost:%s -> %s:%s (PID: %d)", localPort, name, remotePort, cmd.Process.Pid), false)
}

// showContextSwitcher displays context selection dialog
func (a *App) showContextSwitcher() {
	if a.k8s == nil {
		a.flashMsg("K8s client not available", true)
		return
	}

	contexts, currentCtx, err := a.k8s.ListContexts()
	if err != nil {
		a.flashMsg(fmt.Sprintf("Failed to list contexts: %v", err), true)
		return
	}

	list := tview.NewList()
	list.SetBorder(true).SetTitle(" Switch Context (Enter to select, Esc to cancel) ")

	for i, ctx := range contexts {
		prefix := "  "
		if ctx == currentCtx {
			prefix = "* "
		}
		list.AddItem(prefix+ctx, "", rune('1'+i), nil)
	}

	list.SetSelectedFunc(func(index int, mainText, secondaryText string, shortcut rune) {
		selectedCtx := contexts[index]
		a.pages.RemovePage("context-switcher")
		a.SetFocus(a.table)

		if selectedCtx == currentCtx {
			return
		}

		go func() {
			a.flashMsg(fmt.Sprintf("Switching to context: %s...", selectedCtx), false)
			err := a.k8s.SwitchContext(selectedCtx)
			if err != nil {
				a.flashMsg(fmt.Sprintf("Failed to switch context: %v", err), true)
				return
			}

			a.flashMsg(fmt.Sprintf("Switched to context: %s", selectedCtx), false)
			a.updateHeader()
			a.refresh()
		}()
	})

	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc {
			a.pages.RemovePage("context-switcher")
			a.SetFocus(a.table)
			return nil
		}
		return event
	})

	a.pages.AddPage("context-switcher", centered(list, 60, min(len(contexts)+4, 20)), true, true)
}

// showHealth displays system health status
func (a *App) showHealth() {
	health := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true)
	health.SetBorder(true).SetTitle(" System Health (Press Esc to close) ")

	var sb strings.Builder
	sb.WriteString(" [yellow::b]k13s Health Status[white::-]\n\n")

	// K8s connectivity
	if a.k8s != nil {
		ctxName, cluster, _, err := a.k8s.GetContextInfo()
		if err != nil {
			sb.WriteString(" [red]✗[white] Kubernetes: Not connected\n")
		} else {
			sb.WriteString(fmt.Sprintf(" [green]✓[white] Kubernetes: Connected\n"))
			sb.WriteString(fmt.Sprintf("   Context: %s\n", ctxName))
			sb.WriteString(fmt.Sprintf("   Cluster: %s\n", cluster))
		}
	} else {
		sb.WriteString(" [red]✗[white] Kubernetes: Client not initialized\n")
	}

	sb.WriteString("\n")

	// AI status
	if a.aiClient != nil && a.aiClient.IsReady() {
		sb.WriteString(fmt.Sprintf(" [green]✓[white] AI: Online (%s)\n", a.aiClient.GetModel()))
	} else {
		sb.WriteString(" [red]✗[white] AI: Offline\n")
		sb.WriteString("   Configure in ~/.kube-ai-dashboard/config.yaml\n")
	}

	sb.WriteString("\n")

	// Config
	if a.config != nil {
		sb.WriteString(fmt.Sprintf(" [gray]Language:[white] %s\n", a.config.Language))
	}

	sb.WriteString("\n [gray]Press Esc to close[white]")

	health.SetText(sb.String())

	health.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc {
			a.pages.RemovePage("health")
			a.SetFocus(a.table)
			return nil
		}
		return event
	})

	a.pages.AddPage("health", centered(health, 60, 18), true, true)
}

// showHelp displays help modal
func (a *App) showHelp() {
	help := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetText(`
 [yellow::b]k13s - Kubernetes AI Dashboard[white::-]
 [gray]k9s compatible keybindings with AI assistance[white]

 ┌──────────────────────────────────────────────────────────────────┐
 │ [cyan::b]GENERAL[white::-]                                                      │
 ├──────────────────────────────────────────────────────────────────┤
 │  [yellow]:[white]        Command mode        [yellow]?[white]        Help                   │
 │  [yellow]/[white]        Filter mode         [yellow]Esc[white]      Back/Clear/Cancel      │
 │  [yellow]Tab[white]      AI Panel focus      [yellow]Enter[white]    Select/Drill-down      │
 │  [yellow]Ctrl+E[white]   Toggle AI panel     [yellow]q/Ctrl+C[white] Quit application       │
 └──────────────────────────────────────────────────────────────────┘

 ┌──────────────────────────────────────────────────────────────────┐
 │ [cyan::b]NAVIGATION[white::-]                                                    │
 ├──────────────────────────────────────────────────────────────────┤
 │  [yellow]j/↓[white]      Down               [yellow]k/↑[white]      Up                     │
 │  [yellow]g[white]        Top                [yellow]G[white]        Bottom                 │
 │  [yellow]Ctrl+F[white]   Page down          [yellow]Ctrl+B[white]   Page up                │
 │  [yellow]Ctrl+D[white]   Half page down     [yellow]Ctrl+U[white]   Half page up           │
 └──────────────────────────────────────────────────────────────────┘

 ┌──────────────────────────────────────────────────────────────────┐
 │ [cyan::b]RESOURCE ACTIONS[white::-]                                              │
 ├──────────────────────────────────────────────────────────────────┤
 │  [yellow]d[white]        Describe           [yellow]y[white]        YAML view              │
 │  [yellow]e[white]        Edit ($EDITOR)     [yellow]Ctrl+D[white]   Delete                 │
 │  [yellow]r[white]        Refresh            [yellow]c[white]        Switch context         │
 │  [yellow]n[white]        Cycle namespace    [yellow]Space[white]    Multi-select           │
 └──────────────────────────────────────────────────────────────────┘

 ┌──────────────────────────────────────────────────────────────────┐
 │ [cyan::b]NAMESPACE SHORTCUTS[white::-] (k9s style)                               │
 ├──────────────────────────────────────────────────────────────────┤
 │  [yellow]0[white] All namespaces    [yellow]n[white]   Cycle through namespaces             │
 │  [yellow]u[white] Use namespace (on namespace view)                              │
 │  [yellow]:ns <name>[white]         Switch to specific namespace               │
 └──────────────────────────────────────────────────────────────────┘

 ┌──────────────────────────────────────────────────────────────────┐
 │ [cyan::b]POD ACTIONS[white::-]                                                   │
 ├──────────────────────────────────────────────────────────────────┤
 │  [yellow]l[white]        Logs               [yellow]p[white]        Previous logs          │
 │  [yellow]s[white]        Shell              [yellow]a[white]        Attach                 │
 │  [yellow]o[white]        Show node          [yellow]k/Ctrl+K[white] Kill (force delete)    │
 │  [yellow]Shift+F[white]  Port forward       [yellow]f[white]        Show port-forward      │
 └──────────────────────────────────────────────────────────────────┘

 ┌──────────────────────────────────────────────────────────────────┐
 │ [cyan::b]WORKLOAD ACTIONS[white::-] (Deploy/StatefulSet/DaemonSet/ReplicaSet)   │
 ├──────────────────────────────────────────────────────────────────┤
 │  [yellow]S[white]        Scale              [yellow]R[white]        Restart/Rollout        │
 │  [yellow]z[white]        Show ReplicaSets   [yellow]Enter[white]    Show Pods              │
 └──────────────────────────────────────────────────────────────────┘

 ┌──────────────────────────────────────────────────────────────────┐
 │ [cyan::b]LOG VIEW[white::-]                                                      │
 ├──────────────────────────────────────────────────────────────────┤
 │  [yellow]0-9[white]      Toggle container   [yellow]w[white]        Wrap toggle            │
 │  [yellow]t[white]        Toggle timestamps  [yellow]Ctrl+S[white]   Save to file           │
 │  [yellow]/[white]        Filter logs        [yellow]Esc[white]      Exit log view          │
 └──────────────────────────────────────────────────────────────────┘

 ┌──────────────────────────────────────────────────────────────────┐
 │ [cyan::b]COMMAND EXAMPLES[white::-] (press : to enter command mode)             │
 ├──────────────────────────────────────────────────────────────────┤
 │  [yellow]:pods[white] [yellow]:po[white]              List pods                            │
 │  [yellow]:pods -n kube-system[white]  List pods in specific namespace         │
 │  [yellow]:pods -A[white]              List pods in all namespaces             │
 │  [yellow]:deploy[white] [yellow]:dp[white]            List deployments                     │
 │  [yellow]:svc[white] [yellow]:services[white]         List services                        │
 │  [yellow]:ns kube-system[white]       Switch to namespace                     │
 │  [yellow]:ctx[white] [yellow]:context[white]          Switch context                       │
 └──────────────────────────────────────────────────────────────────┘

 ┌──────────────────────────────────────────────────────────────────┐
 │ [cyan::b]AI ASSISTANT[white::-] (Tab to focus, type and press Enter)            │
 ├──────────────────────────────────────────────────────────────────┤
 │  Ask natural language questions or request kubectl commands:     │
 │  • "Show me all pods in kube-system namespace"                   │
 │  • "Why is my pod crashing?"                                     │
 │  • "Scale deployment nginx to 3 replicas"                        │
 │  • "Show recent events for this deployment"                      │
 │                                                                  │
 │  [gray]AI will suggest commands. Press Y to execute, N to cancel.[white]     │
 └──────────────────────────────────────────────────────────────────┘

 [gray]Press Esc, q, or ? to close this help[white]
`)
	help.SetBorder(true).SetTitle(" Help ")

	a.pages.AddPage("help", centered(help, 75, 55), true, true)
	a.SetFocus(help)

	help.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc || event.Rune() == 'q' || event.Rune() == '?' {
			a.pages.RemovePage("help")
			a.SetFocus(a.table)
			return nil
		}
		return event
	})
}

// Run starts the application with panic recovery (k9s pattern)
func (a *App) Run() error {
	// Top-level panic recovery (k9s pattern)
	defer func() {
		if err := recover(); err != nil {
			a.logger.Error("PANIC RECOVERED", "error", err, "stack", string(debug.Stack()))
			fmt.Fprintf(os.Stderr, "\n[FATAL] k13s crashed: %v\n", err)
		}
	}()

	// Mark as running and trigger initial refresh after first draw
	a.SetAfterDrawFunc(func(screen tcell.Screen) {
		a.SetAfterDrawFunc(nil) // Only run once
		atomic.StoreInt32(&a.running, 1)
		go a.refresh()
	})

	a.logger.Info("Starting k13s TUI")
	return a.Application.Run()
}

// Helper functions

func centered(p tview.Primitive, width, height int) tview.Primitive {
	return tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(p, height, 0, true).
			AddItem(nil, 0, 1, false), width, 0, true).
		AddItem(nil, 0, 1, false)
}

func formatAge(t time.Time) string {
	d := time.Since(t)
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}

// Additional fetch functions for extended resources

func (a *App) fetchConfigMaps(ctx context.Context, ns string) ([]string, [][]string, error) {
	headers := []string{"NAMESPACE", "NAME", "DATA", "AGE"}
	cms, err := a.k8s.ListConfigMaps(ctx, ns)
	if err != nil {
		return headers, nil, err
	}
	var rows [][]string
	for _, cm := range cms {
		rows = append(rows, []string{
			cm.Namespace,
			cm.Name,
			fmt.Sprintf("%d", len(cm.Data)),
			formatAge(cm.CreationTimestamp.Time),
		})
	}
	return headers, rows, nil
}

func (a *App) fetchSecrets(ctx context.Context, ns string) ([]string, [][]string, error) {
	headers := []string{"NAMESPACE", "NAME", "TYPE", "DATA", "AGE"}
	secrets, err := a.k8s.ListSecrets(ctx, ns)
	if err != nil {
		return headers, nil, err
	}
	var rows [][]string
	for _, s := range secrets {
		rows = append(rows, []string{
			s.Namespace,
			s.Name,
			string(s.Type),
			fmt.Sprintf("%d", len(s.Data)),
			formatAge(s.CreationTimestamp.Time),
		})
	}
	return headers, rows, nil
}

func (a *App) fetchPersistentVolumes(ctx context.Context) ([]string, [][]string, error) {
	headers := []string{"NAME", "CAPACITY", "ACCESS MODES", "STATUS", "CLAIM", "AGE"}
	pvs, err := a.k8s.ListPersistentVolumes(ctx)
	if err != nil {
		return headers, nil, err
	}
	var rows [][]string
	for _, pv := range pvs {
		capacity := ""
		if storage, ok := pv.Spec.Capacity[corev1.ResourceStorage]; ok {
			capacity = storage.String()
		}
		claim := ""
		if pv.Spec.ClaimRef != nil {
			claim = fmt.Sprintf("%s/%s", pv.Spec.ClaimRef.Namespace, pv.Spec.ClaimRef.Name)
		}
		rows = append(rows, []string{
			pv.Name,
			capacity,
			strings.Join(accessModesToStrings(pv.Spec.AccessModes), ","),
			string(pv.Status.Phase),
			claim,
			formatAge(pv.CreationTimestamp.Time),
		})
	}
	return headers, rows, nil
}

func (a *App) fetchPersistentVolumeClaims(ctx context.Context, ns string) ([]string, [][]string, error) {
	headers := []string{"NAMESPACE", "NAME", "STATUS", "VOLUME", "CAPACITY", "AGE"}
	pvcs, err := a.k8s.ListPersistentVolumeClaims(ctx, ns)
	if err != nil {
		return headers, nil, err
	}
	var rows [][]string
	for _, pvc := range pvcs {
		capacity := ""
		if pvc.Status.Capacity != nil {
			if storage, ok := pvc.Status.Capacity[corev1.ResourceStorage]; ok {
				capacity = storage.String()
			}
		}
		rows = append(rows, []string{
			pvc.Namespace,
			pvc.Name,
			string(pvc.Status.Phase),
			pvc.Spec.VolumeName,
			capacity,
			formatAge(pvc.CreationTimestamp.Time),
		})
	}
	return headers, rows, nil
}

func (a *App) fetchStorageClasses(ctx context.Context) ([]string, [][]string, error) {
	headers := []string{"NAME", "PROVISIONER", "RECLAIM POLICY", "ALLOW EXPANSION", "AGE"}
	scs, err := a.k8s.ListStorageClasses(ctx)
	if err != nil {
		return headers, nil, err
	}
	var rows [][]string
	for _, sc := range scs {
		reclaim := "<default>"
		if sc.ReclaimPolicy != nil {
			reclaim = string(*sc.ReclaimPolicy)
		}
		expand := "false"
		if sc.AllowVolumeExpansion != nil && *sc.AllowVolumeExpansion {
			expand = "true"
		}
		rows = append(rows, []string{
			sc.Name,
			sc.Provisioner,
			reclaim,
			expand,
			formatAge(sc.CreationTimestamp.Time),
		})
	}
	return headers, rows, nil
}

func (a *App) fetchReplicaSets(ctx context.Context, ns string) ([]string, [][]string, error) {
	headers := []string{"NAMESPACE", "NAME", "DESIRED", "CURRENT", "READY", "AGE"}
	rss, err := a.k8s.ListReplicaSets(ctx, ns)
	if err != nil {
		return headers, nil, err
	}
	var rows [][]string
	for _, rs := range rss {
		desired := int32(0)
		if rs.Spec.Replicas != nil {
			desired = *rs.Spec.Replicas
		}
		rows = append(rows, []string{
			rs.Namespace,
			rs.Name,
			fmt.Sprintf("%d", desired),
			fmt.Sprintf("%d", rs.Status.Replicas),
			fmt.Sprintf("%d", rs.Status.ReadyReplicas),
			formatAge(rs.CreationTimestamp.Time),
		})
	}
	return headers, rows, nil
}

func (a *App) fetchDaemonSets(ctx context.Context, ns string) ([]string, [][]string, error) {
	headers := []string{"NAMESPACE", "NAME", "DESIRED", "CURRENT", "READY", "AGE"}
	dss, err := a.k8s.ListDaemonSets(ctx, ns)
	if err != nil {
		return headers, nil, err
	}
	var rows [][]string
	for _, ds := range dss {
		rows = append(rows, []string{
			ds.Namespace,
			ds.Name,
			fmt.Sprintf("%d", ds.Status.DesiredNumberScheduled),
			fmt.Sprintf("%d", ds.Status.CurrentNumberScheduled),
			fmt.Sprintf("%d", ds.Status.NumberReady),
			formatAge(ds.CreationTimestamp.Time),
		})
	}
	return headers, rows, nil
}

func (a *App) fetchStatefulSets(ctx context.Context, ns string) ([]string, [][]string, error) {
	headers := []string{"NAMESPACE", "NAME", "READY", "AGE"}
	stss, err := a.k8s.ListStatefulSets(ctx, ns)
	if err != nil {
		return headers, nil, err
	}
	var rows [][]string
	for _, sts := range stss {
		replicas := int32(0)
		if sts.Spec.Replicas != nil {
			replicas = *sts.Spec.Replicas
		}
		rows = append(rows, []string{
			sts.Namespace,
			sts.Name,
			fmt.Sprintf("%d/%d", sts.Status.ReadyReplicas, replicas),
			formatAge(sts.CreationTimestamp.Time),
		})
	}
	return headers, rows, nil
}

func (a *App) fetchJobs(ctx context.Context, ns string) ([]string, [][]string, error) {
	headers := []string{"NAMESPACE", "NAME", "COMPLETIONS", "DURATION", "AGE"}
	jobs, err := a.k8s.ListJobs(ctx, ns)
	if err != nil {
		return headers, nil, err
	}
	var rows [][]string
	for _, job := range jobs {
		completions := int32(1)
		if job.Spec.Completions != nil {
			completions = *job.Spec.Completions
		}
		duration := "<running>"
		if job.Status.CompletionTime != nil && job.Status.StartTime != nil {
			d := job.Status.CompletionTime.Sub(job.Status.StartTime.Time)
			duration = d.Round(time.Second).String()
		}
		rows = append(rows, []string{
			job.Namespace,
			job.Name,
			fmt.Sprintf("%d/%d", job.Status.Succeeded, completions),
			duration,
			formatAge(job.CreationTimestamp.Time),
		})
	}
	return headers, rows, nil
}

func (a *App) fetchCronJobs(ctx context.Context, ns string) ([]string, [][]string, error) {
	headers := []string{"NAMESPACE", "NAME", "SCHEDULE", "SUSPEND", "ACTIVE", "LAST SCHEDULE", "AGE"}
	cjs, err := a.k8s.ListCronJobs(ctx, ns)
	if err != nil {
		return headers, nil, err
	}
	var rows [][]string
	for _, cj := range cjs {
		suspend := "False"
		if cj.Spec.Suspend != nil && *cj.Spec.Suspend {
			suspend = "True"
		}
		lastSchedule := "<none>"
		if cj.Status.LastScheduleTime != nil {
			lastSchedule = formatAge(cj.Status.LastScheduleTime.Time)
		}
		rows = append(rows, []string{
			cj.Namespace,
			cj.Name,
			cj.Spec.Schedule,
			suspend,
			fmt.Sprintf("%d", len(cj.Status.Active)),
			lastSchedule,
			formatAge(cj.CreationTimestamp.Time),
		})
	}
	return headers, rows, nil
}

func (a *App) fetchReplicationControllers(ctx context.Context, ns string) ([]string, [][]string, error) {
	headers := []string{"NAMESPACE", "NAME", "DESIRED", "CURRENT", "READY", "AGE"}
	rcs, err := a.k8s.ListReplicationControllers(ctx, ns)
	if err != nil {
		return headers, nil, err
	}
	var rows [][]string
	for _, rc := range rcs {
		desired := int32(0)
		if rc.Spec.Replicas != nil {
			desired = *rc.Spec.Replicas
		}
		rows = append(rows, []string{
			rc.Namespace,
			rc.Name,
			fmt.Sprintf("%d", desired),
			fmt.Sprintf("%d", rc.Status.Replicas),
			fmt.Sprintf("%d", rc.Status.ReadyReplicas),
			formatAge(rc.CreationTimestamp.Time),
		})
	}
	return headers, rows, nil
}

func (a *App) fetchIngresses(ctx context.Context, ns string) ([]string, [][]string, error) {
	headers := []string{"NAMESPACE", "NAME", "CLASS", "HOSTS", "ADDRESS", "AGE"}
	ings, err := a.k8s.ListIngresses(ctx, ns)
	if err != nil {
		return headers, nil, err
	}
	var rows [][]string
	for _, ing := range ings {
		class := "<none>"
		if ing.Spec.IngressClassName != nil {
			class = *ing.Spec.IngressClassName
		}
		var hosts []string
		for _, rule := range ing.Spec.Rules {
			if rule.Host != "" {
				hosts = append(hosts, rule.Host)
			}
		}
		var addresses []string
		for _, lb := range ing.Status.LoadBalancer.Ingress {
			if lb.IP != "" {
				addresses = append(addresses, lb.IP)
			} else if lb.Hostname != "" {
				addresses = append(addresses, lb.Hostname)
			}
		}
		rows = append(rows, []string{
			ing.Namespace,
			ing.Name,
			class,
			strings.Join(hosts, ","),
			strings.Join(addresses, ","),
			formatAge(ing.CreationTimestamp.Time),
		})
	}
	return headers, rows, nil
}

func (a *App) fetchEndpoints(ctx context.Context, ns string) ([]string, [][]string, error) {
	headers := []string{"NAMESPACE", "NAME", "ENDPOINTS", "AGE"}
	eps, err := a.k8s.ListEndpoints(ctx, ns)
	if err != nil {
		return headers, nil, err
	}
	var rows [][]string
	for _, ep := range eps {
		var addrs []string
		for _, subset := range ep.Subsets {
			for _, addr := range subset.Addresses {
				for _, port := range subset.Ports {
					addrs = append(addrs, fmt.Sprintf("%s:%d", addr.IP, port.Port))
				}
			}
		}
		epStr := strings.Join(addrs, ",")
		if len(epStr) > 50 {
			epStr = epStr[:47] + "..."
		}
		rows = append(rows, []string{
			ep.Namespace,
			ep.Name,
			epStr,
			formatAge(ep.CreationTimestamp.Time),
		})
	}
	return headers, rows, nil
}

func (a *App) fetchNetworkPolicies(ctx context.Context, ns string) ([]string, [][]string, error) {
	headers := []string{"NAMESPACE", "NAME", "POD-SELECTOR", "AGE"}
	netpols, err := a.k8s.ListNetworkPolicies(ctx, ns)
	if err != nil {
		return headers, nil, err
	}
	var rows [][]string
	for _, np := range netpols {
		selector := "<all>"
		if len(np.Spec.PodSelector.MatchLabels) > 0 {
			var parts []string
			for k, v := range np.Spec.PodSelector.MatchLabels {
				parts = append(parts, fmt.Sprintf("%s=%s", k, v))
			}
			selector = strings.Join(parts, ",")
		}
		rows = append(rows, []string{
			np.Namespace,
			np.Name,
			selector,
			formatAge(np.CreationTimestamp.Time),
		})
	}
	return headers, rows, nil
}

func (a *App) fetchServiceAccounts(ctx context.Context, ns string) ([]string, [][]string, error) {
	headers := []string{"NAMESPACE", "NAME", "SECRETS", "AGE"}
	sas, err := a.k8s.ListServiceAccounts(ctx, ns)
	if err != nil {
		return headers, nil, err
	}
	var rows [][]string
	for _, sa := range sas {
		rows = append(rows, []string{
			sa.Namespace,
			sa.Name,
			fmt.Sprintf("%d", len(sa.Secrets)),
			formatAge(sa.CreationTimestamp.Time),
		})
	}
	return headers, rows, nil
}

func (a *App) fetchRoles(ctx context.Context, ns string) ([]string, [][]string, error) {
	headers := []string{"NAMESPACE", "NAME", "AGE"}
	roles, err := a.k8s.ListRoles(ctx, ns)
	if err != nil {
		return headers, nil, err
	}
	var rows [][]string
	for _, r := range roles {
		rows = append(rows, []string{
			r.Namespace,
			r.Name,
			formatAge(r.CreationTimestamp.Time),
		})
	}
	return headers, rows, nil
}

func (a *App) fetchRoleBindings(ctx context.Context, ns string) ([]string, [][]string, error) {
	headers := []string{"NAMESPACE", "NAME", "ROLE", "AGE"}
	rbs, err := a.k8s.ListRoleBindings(ctx, ns)
	if err != nil {
		return headers, nil, err
	}
	var rows [][]string
	for _, rb := range rbs {
		roleRef := fmt.Sprintf("%s/%s", rb.RoleRef.Kind, rb.RoleRef.Name)
		rows = append(rows, []string{
			rb.Namespace,
			rb.Name,
			roleRef,
			formatAge(rb.CreationTimestamp.Time),
		})
	}
	return headers, rows, nil
}

func (a *App) fetchClusterRoles(ctx context.Context) ([]string, [][]string, error) {
	headers := []string{"NAME", "AGE"}
	crs, err := a.k8s.ListClusterRoles(ctx)
	if err != nil {
		return headers, nil, err
	}
	var rows [][]string
	for _, cr := range crs {
		rows = append(rows, []string{
			cr.Name,
			formatAge(cr.CreationTimestamp.Time),
		})
	}
	return headers, rows, nil
}

func (a *App) fetchClusterRoleBindings(ctx context.Context) ([]string, [][]string, error) {
	headers := []string{"NAME", "ROLE", "AGE"}
	crbs, err := a.k8s.ListClusterRoleBindings(ctx)
	if err != nil {
		return headers, nil, err
	}
	var rows [][]string
	for _, crb := range crbs {
		roleRef := fmt.Sprintf("%s/%s", crb.RoleRef.Kind, crb.RoleRef.Name)
		rows = append(rows, []string{
			crb.Name,
			roleRef,
			formatAge(crb.CreationTimestamp.Time),
		})
	}
	return headers, rows, nil
}

func (a *App) fetchPodDisruptionBudgets(ctx context.Context, ns string) ([]string, [][]string, error) {
	headers := []string{"NAMESPACE", "NAME", "MIN AVAILABLE", "MAX UNAVAILABLE", "ALLOWED DISRUPTIONS", "AGE"}
	pdbs, err := a.k8s.ListPodDisruptionBudgets(ctx, ns)
	if err != nil {
		return headers, nil, err
	}
	var rows [][]string
	for _, pdb := range pdbs {
		minAvail := "<none>"
		if pdb.Spec.MinAvailable != nil {
			minAvail = pdb.Spec.MinAvailable.String()
		}
		maxUnavail := "<none>"
		if pdb.Spec.MaxUnavailable != nil {
			maxUnavail = pdb.Spec.MaxUnavailable.String()
		}
		rows = append(rows, []string{
			pdb.Namespace,
			pdb.Name,
			minAvail,
			maxUnavail,
			fmt.Sprintf("%d", pdb.Status.DisruptionsAllowed),
			formatAge(pdb.CreationTimestamp.Time),
		})
	}
	return headers, rows, nil
}

func (a *App) fetchLimitRanges(ctx context.Context, ns string) ([]string, [][]string, error) {
	headers := []string{"NAMESPACE", "NAME", "AGE"}
	lrs, err := a.k8s.ListLimitRanges(ctx, ns)
	if err != nil {
		return headers, nil, err
	}
	var rows [][]string
	for _, lr := range lrs {
		rows = append(rows, []string{
			lr.Namespace,
			lr.Name,
			formatAge(lr.CreationTimestamp.Time),
		})
	}
	return headers, rows, nil
}

func (a *App) fetchResourceQuotas(ctx context.Context, ns string) ([]string, [][]string, error) {
	headers := []string{"NAMESPACE", "NAME", "AGE"}
	rqs, err := a.k8s.ListResourceQuotas(ctx, ns)
	if err != nil {
		return headers, nil, err
	}
	var rows [][]string
	for _, rq := range rqs {
		rows = append(rows, []string{
			rq.Namespace,
			rq.Name,
			formatAge(rq.CreationTimestamp.Time),
		})
	}
	return headers, rows, nil
}

func (a *App) fetchHPAs(ctx context.Context, ns string) ([]string, [][]string, error) {
	headers := []string{"NAMESPACE", "NAME", "REFERENCE", "TARGETS", "MINPODS", "MAXPODS", "REPLICAS", "AGE"}
	hpas, err := a.k8s.ListHPAs(ctx, ns)
	if err != nil {
		return headers, nil, err
	}
	var rows [][]string
	for _, hpa := range hpas {
		ref := fmt.Sprintf("%s/%s", hpa.Spec.ScaleTargetRef.Kind, hpa.Spec.ScaleTargetRef.Name)
		minPods := int32(1)
		if hpa.Spec.MinReplicas != nil {
			minPods = *hpa.Spec.MinReplicas
		}
		rows = append(rows, []string{
			hpa.Namespace,
			hpa.Name,
			ref,
			"<complex>",
			fmt.Sprintf("%d", minPods),
			fmt.Sprintf("%d", hpa.Spec.MaxReplicas),
			fmt.Sprintf("%d", hpa.Status.CurrentReplicas),
			formatAge(hpa.CreationTimestamp.Time),
		})
	}
	return headers, rows, nil
}

func (a *App) fetchCRDs(ctx context.Context) ([]string, [][]string, error) {
	headers := []string{"NAME", "CREATED AT"}
	crds, err := a.k8s.ListCRDs(ctx)
	if err != nil {
		return headers, nil, err
	}
	var rows [][]string
	for _, crd := range crds {
		rows = append(rows, []string{
			crd.Name,
			formatAge(crd.CreationTimestamp.Time),
		})
	}
	return headers, rows, nil
}

func (a *App) fetchGenericResource(ctx context.Context, resource, ns string) ([]string, [][]string, error) {
	headers := []string{"NAMESPACE", "NAME", "AGE"}
	return headers, nil, fmt.Errorf("resource type '%s' not yet implemented", resource)
}

// Helper function for PV access modes
func accessModesToStrings(modes []corev1.PersistentVolumeAccessMode) []string {
	var result []string
	for _, m := range modes {
		switch m {
		case corev1.ReadWriteOnce:
			result = append(result, "RWO")
		case corev1.ReadOnlyMany:
			result = append(result, "ROX")
		case corev1.ReadWriteMany:
			result = append(result, "RWX")
		case corev1.ReadWriteOncePod:
			result = append(result, "RWOP")
		}
	}
	return result
}

// Navigation history for back navigation
type navHistory struct {
	resource  string
	namespace string
	filter    string
}

var navigationStack []navHistory

// drillDown navigates to related resources (k9s Enter key behavior)
func (a *App) drillDown() {
	row, _ := a.table.GetSelection()
	if row <= 0 {
		return
	}

	a.mx.RLock()
	resource := a.currentResource
	ns := a.currentNamespace
	filter := a.filterText
	a.mx.RUnlock()

	// Save current state to navigation stack
	navigationStack = append(navigationStack, navHistory{resource, ns, filter})

	// Get selected item info
	var selectedNs, selectedName string
	switch resource {
	case "nodes", "namespaces", "persistentvolumes", "storageclasses",
		"clusterroles", "clusterrolebindings", "customresourcedefinitions":
		selectedName = a.table.GetCell(row, 0).Text
	default:
		selectedNs = a.table.GetCell(row, 0).Text
		selectedName = a.table.GetCell(row, 1).Text
	}

	// Determine drill-down behavior based on resource type
	switch resource {
	case "pods", "po":
		// Pod -> Show logs (container view)
		a.showLogs()
		return

	case "deployments", "deploy":
		// Deployment -> Pods with label selector
		a.mx.Lock()
		a.currentResource = "pods"
		a.currentNamespace = selectedNs
		a.filterText = selectedName // Filter pods by deployment name
		a.mx.Unlock()
		go func() {
			a.updateHeader()
			a.refresh()
		}()

	case "services", "svc":
		// Service -> Pods (show related pods)
		a.mx.Lock()
		a.currentResource = "pods"
		a.currentNamespace = selectedNs
		a.filterText = selectedName
		a.mx.Unlock()
		go func() {
			a.updateHeader()
			a.refresh()
		}()

	case "replicasets", "rs":
		// ReplicaSet -> Pods
		a.mx.Lock()
		a.currentResource = "pods"
		a.currentNamespace = selectedNs
		a.filterText = selectedName
		a.mx.Unlock()
		go func() {
			a.updateHeader()
			a.refresh()
		}()

	case "statefulsets", "sts":
		// StatefulSet -> Pods
		a.mx.Lock()
		a.currentResource = "pods"
		a.currentNamespace = selectedNs
		a.filterText = selectedName
		a.mx.Unlock()
		go func() {
			a.updateHeader()
			a.refresh()
		}()

	case "daemonsets", "ds":
		// DaemonSet -> Pods
		a.mx.Lock()
		a.currentResource = "pods"
		a.currentNamespace = selectedNs
		a.filterText = selectedName
		a.mx.Unlock()
		go func() {
			a.updateHeader()
			a.refresh()
		}()

	case "jobs", "job":
		// Job -> Pods
		a.mx.Lock()
		a.currentResource = "pods"
		a.currentNamespace = selectedNs
		a.filterText = selectedName
		a.mx.Unlock()
		go func() {
			a.updateHeader()
			a.refresh()
		}()

	case "cronjobs", "cj":
		// CronJob -> Jobs
		a.mx.Lock()
		a.currentResource = "jobs"
		a.currentNamespace = selectedNs
		a.filterText = selectedName
		a.mx.Unlock()
		go func() {
			a.updateHeader()
			a.refresh()
		}()

	case "nodes", "no":
		// Node -> Pods on that node
		a.mx.Lock()
		a.currentResource = "pods"
		a.currentNamespace = "" // All namespaces
		a.filterText = selectedName
		a.mx.Unlock()
		go func() {
			a.updateHeader()
			a.refresh()
		}()

	case "namespaces", "ns":
		// Namespace -> Switch to that namespace and show pods
		a.mx.Lock()
		a.currentResource = "pods"
		a.currentNamespace = selectedName
		a.filterText = ""
		a.mx.Unlock()
		go func() {
			a.updateHeader()
			a.refresh()
		}()

	default:
		// Default: show describe
		a.showDescribe()
		return
	}
}

// goBack returns to previous view (k9s Esc key behavior)
func (a *App) goBack() {
	if len(navigationStack) == 0 {
		return
	}

	// Pop from stack
	prev := navigationStack[len(navigationStack)-1]
	navigationStack = navigationStack[:len(navigationStack)-1]

	a.mx.Lock()
	a.currentResource = prev.resource
	a.currentNamespace = prev.namespace
	a.filterText = prev.filter
	a.mx.Unlock()

	go func() {
		a.updateHeader()
		a.refresh()
	}()
}

// pageUp scrolls up by half page
func (a *App) pageUp() {
	row, col := a.table.GetSelection()
	newRow := row - 10
	if newRow < 1 {
		newRow = 1
	}
	a.table.Select(newRow, col)
}

// pageDown scrolls down by half page
func (a *App) pageDown() {
	row, col := a.table.GetSelection()
	maxRow := a.table.GetRowCount() - 1
	newRow := row + 10
	if newRow > maxRow {
		newRow = maxRow
	}
	a.table.Select(newRow, col)
}

// showYAML shows YAML for selected resource (k9s y key)
func (a *App) showYAML() {
	row, _ := a.table.GetSelection()
	if row <= 0 {
		return
	}

	a.mx.RLock()
	resource := a.currentResource
	a.mx.RUnlock()

	var ns, name string
	switch resource {
	case "nodes", "no", "namespaces", "ns", "persistentvolumes", "storageclasses",
		"clusterroles", "clusterrolebindings", "customresourcedefinitions":
		name = a.table.GetCell(row, 0).Text
	default:
		ns = a.table.GetCell(row, 0).Text
		name = a.table.GetCell(row, 1).Text
	}

	yamlView := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true)
	yamlView.SetBorder(true).
		SetTitle(fmt.Sprintf(" YAML: %s/%s (Press Esc or 'q' to close) ", resource, name))

	a.pages.AddPage("yaml", yamlView, true, true)
	a.SetFocus(yamlView)

	// Fetch YAML
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		gvr, ok := a.k8s.GetGVR(resource)
		if !ok {
			a.QueueUpdateDraw(func() {
				yamlView.SetText(fmt.Sprintf("[red]Unknown resource type: %s", resource))
			})
			return
		}

		yaml, err := a.k8s.GetResourceYAML(ctx, ns, name, gvr)
		a.QueueUpdateDraw(func() {
			if err != nil {
				yamlView.SetText(fmt.Sprintf("[red]Error: %v", err))
			} else {
				yamlView.SetText(yaml)
			}
		})
	}()

	yamlView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc || (event.Key() == tcell.KeyRune && event.Rune() == 'q') {
			a.pages.RemovePage("yaml")
			a.SetFocus(a.table)
			return nil
		}
		return event
	})
}

// showLogsPrevious shows logs for previous container (k9s p key)
func (a *App) showLogsPrevious() {
	a.mx.RLock()
	resource := a.currentResource
	a.mx.RUnlock()

	if resource != "pods" && resource != "po" {
		a.flashMsg("Logs only available for pods", true)
		return
	}

	row, _ := a.table.GetSelection()
	if row <= 0 {
		return
	}

	ns := a.table.GetCell(row, 0).Text
	name := a.table.GetCell(row, 1).Text

	logView := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true)
	logView.SetBorder(true).
		SetTitle(fmt.Sprintf(" Previous Logs: %s/%s (Press Esc to close) ", ns, name))

	a.pages.AddPage("logs", logView, true, true)
	a.SetFocus(logView)

	// Fetch previous logs
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		logs, err := a.k8s.GetPodLogsPrevious(ctx, ns, name, "", 100)
		a.QueueUpdateDraw(func() {
			if err != nil {
				logView.SetText(fmt.Sprintf("[red]Error: %v", err))
			} else if logs == "" {
				logView.SetText("[gray]No previous logs available")
			} else {
				logView.SetText(logs)
				logView.ScrollToEnd()
			}
		})
	}()

	logView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc {
			a.pages.RemovePage("logs")
			a.SetFocus(a.table)
			return nil
		}
		return event
	})
}

// editResource opens the resource in $EDITOR (k9s e key)
func (a *App) editResource() {
	row, _ := a.table.GetSelection()
	if row <= 0 {
		return
	}

	a.mx.RLock()
	resource := a.currentResource
	a.mx.RUnlock()

	var ns, name string
	switch resource {
	case "nodes", "no", "namespaces", "ns", "persistentvolumes", "storageclasses",
		"clusterroles", "clusterrolebindings", "customresourcedefinitions":
		name = a.table.GetCell(row, 0).Text
	default:
		ns = a.table.GetCell(row, 0).Text
		name = a.table.GetCell(row, 1).Text
	}

	// Suspend TUI and run kubectl edit
	a.Suspend(func() {
		var cmd *exec.Cmd
		if ns != "" {
			cmd = exec.Command("kubectl", "edit", resource, name, "-n", ns)
		} else {
			cmd = exec.Command("kubectl", "edit", resource, name)
		}
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Run()
	})

	// Refresh after edit
	go a.refresh()
}

// attachContainer attaches to a container (k9s a key)
func (a *App) attachContainer() {
	a.mx.RLock()
	resource := a.currentResource
	a.mx.RUnlock()

	if resource != "pods" && resource != "po" {
		a.flashMsg("Attach only available for pods", true)
		return
	}

	row, _ := a.table.GetSelection()
	if row <= 0 {
		return
	}

	ns := a.table.GetCell(row, 0).Text
	name := a.table.GetCell(row, 1).Text

	// Suspend TUI and run kubectl attach
	a.Suspend(func() {
		cmd := exec.Command("kubectl", "attach", "-it", "-n", ns, name)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Run()
	})
}

// useNamespace switches to the selected namespace (k9s u key)
func (a *App) useNamespace() {
	a.mx.RLock()
	resource := a.currentResource
	a.mx.RUnlock()

	if resource != "namespaces" && resource != "ns" {
		a.flashMsg("Use 'u' only on namespaces view", true)
		return
	}

	row, _ := a.table.GetSelection()
	if row <= 0 {
		return
	}

	nsName := a.table.GetCell(row, 0).Text

	a.mx.Lock()
	a.currentNamespace = nsName
	a.currentResource = "pods"
	a.mx.Unlock()

	a.flashMsg(fmt.Sprintf("Switched to namespace: %s", nsName), false)

	go func() {
		a.updateHeader()
		a.refresh()
	}()
}

// showNode shows the node where the selected pod is running (k9s o key)
func (a *App) showNode() {
	a.mx.RLock()
	resource := a.currentResource
	a.mx.RUnlock()

	if resource != "pods" && resource != "po" {
		a.flashMsg("Show node only available for pods", true)
		return
	}

	row, _ := a.table.GetSelection()
	if row <= 0 {
		return
	}

	ns := a.table.GetCell(row, 0).Text
	name := a.table.GetCell(row, 1).Text

	// Get pod to find node
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pods, err := a.k8s.ListPods(ctx, ns)
	if err != nil {
		a.flashMsg(fmt.Sprintf("Error: %v", err), true)
		return
	}

	var nodeName string
	for _, pod := range pods {
		if pod.Name == name {
			nodeName = pod.Spec.NodeName
			break
		}
	}

	if nodeName == "" {
		a.flashMsg("Pod not scheduled to a node yet", true)
		return
	}

	// Save current state and navigate to nodes with filter
	a.mx.Lock()
	navigationStack = append(navigationStack, navHistory{resource, a.currentNamespace, a.filterText})
	a.currentResource = "nodes"
	a.currentNamespace = ""
	a.filterText = nodeName
	a.mx.Unlock()

	go func() {
		a.updateHeader()
		a.refresh()
	}()
}

// killPod force deletes a pod (k9s k or Ctrl+K key)
func (a *App) killPod() {
	a.mx.RLock()
	resource := a.currentResource
	a.mx.RUnlock()

	if resource != "pods" && resource != "po" {
		a.flashMsg("Kill only available for pods", true)
		return
	}

	row, _ := a.table.GetSelection()
	if row <= 0 {
		return
	}

	ns := a.table.GetCell(row, 0).Text
	name := a.table.GetCell(row, 1).Text

	modal := tview.NewModal().
		SetText(fmt.Sprintf("[red]Kill pod?[white]\n\n%s/%s\n\nThis will force delete the pod.", ns, name)).
		AddButtons([]string{"Cancel", "Kill"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			a.pages.RemovePage("kill-confirm")
			a.SetFocus(a.table)

			if buttonLabel == "Kill" {
				go func() {
					ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
					defer cancel()

					a.flashMsg(fmt.Sprintf("Killing pod %s/%s...", ns, name), false)

					err := a.k8s.DeletePodForce(ctx, ns, name)
					if err != nil {
						a.flashMsg(fmt.Sprintf("Kill failed: %v", err), true)
						return
					}

					a.flashMsg(fmt.Sprintf("Killed pod %s/%s", ns, name), false)
					a.refresh()
				}()
			}
		})

	modal.SetBackgroundColor(tcell.ColorDarkRed)
	a.pages.AddPage("kill-confirm", modal, true, true)
}

// showBenchmark runs benchmark on service (k9s b key) - placeholder
func (a *App) showBenchmark() {
	a.flashMsg("Benchmark feature not yet implemented", true)
}

// triggerCronJob manually triggers a cronjob (k9s t key)
func (a *App) triggerCronJob() {
	a.mx.RLock()
	resource := a.currentResource
	a.mx.RUnlock()

	if resource != "cronjobs" && resource != "cj" {
		a.flashMsg("Trigger only available for cronjobs", true)
		return
	}

	row, _ := a.table.GetSelection()
	if row <= 0 {
		return
	}

	ns := a.table.GetCell(row, 0).Text
	name := a.table.GetCell(row, 1).Text

	modal := tview.NewModal().
		SetText(fmt.Sprintf("Trigger CronJob?\n\n%s/%s\n\nThis will create a new job from this cronjob.", ns, name)).
		AddButtons([]string{"Cancel", "Trigger"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			a.pages.RemovePage("trigger-confirm")
			a.SetFocus(a.table)

			if buttonLabel == "Trigger" {
				go func() {
					a.flashMsg(fmt.Sprintf("Triggering cronjob %s/%s...", ns, name), false)

					// Use kubectl to create job from cronjob
					jobName := fmt.Sprintf("%s-manual-%d", name, time.Now().Unix())
					cmd := exec.Command("kubectl", "create", "job", jobName, "--from=cronjob/"+name, "-n", ns)
					output, err := cmd.CombinedOutput()
					if err != nil {
						a.flashMsg(fmt.Sprintf("Trigger failed: %s", string(output)), true)
						return
					}

					a.flashMsg(fmt.Sprintf("Created job %s from cronjob %s", jobName, name), false)
					a.refresh()
				}()
			}
		})

	a.pages.AddPage("trigger-confirm", modal, true, true)
}

// showRelatedResource shows related resources (k9s z key)
func (a *App) showRelatedResource() {
	a.mx.RLock()
	resource := a.currentResource
	a.mx.RUnlock()

	row, _ := a.table.GetSelection()
	if row <= 0 {
		return
	}

	var ns, name string
	switch resource {
	case "nodes", "namespaces", "persistentvolumes", "storageclasses",
		"clusterroles", "clusterrolebindings", "customresourcedefinitions":
		name = a.table.GetCell(row, 0).Text
	default:
		ns = a.table.GetCell(row, 0).Text
		name = a.table.GetCell(row, 1).Text
	}

	// Different behavior based on resource type
	switch resource {
	case "deployments", "deploy":
		// Show ReplicaSets
		a.mx.Lock()
		navigationStack = append(navigationStack, navHistory{resource, a.currentNamespace, a.filterText})
		a.currentResource = "replicasets"
		a.currentNamespace = ns
		a.filterText = name
		a.mx.Unlock()
		go func() {
			a.updateHeader()
			a.refresh()
		}()

	default:
		a.flashMsg(fmt.Sprintf("No related resources for %s", resource), true)
	}
}

// scaleResource scales a deployment/statefulset (k9s Shift+S key)
func (a *App) scaleResource() {
	a.mx.RLock()
	resource := a.currentResource
	a.mx.RUnlock()

	// Only scalable resources
	scalable := map[string]bool{
		"deployments": true, "deploy": true,
		"statefulsets": true, "sts": true,
		"replicasets": true, "rs": true,
	}

	if !scalable[resource] {
		a.flashMsg("Scale only available for deployments, statefulsets, replicasets", true)
		return
	}

	row, _ := a.table.GetSelection()
	if row <= 0 {
		return
	}

	ns := a.table.GetCell(row, 0).Text
	name := a.table.GetCell(row, 1).Text

	// Create scale dialog
	form := tview.NewForm()
	form.SetBorder(true).SetTitle(fmt.Sprintf(" Scale: %s/%s ", ns, name))

	var replicas string
	form.AddInputField("Replicas:", "1", 10, nil, func(text string) {
		replicas = text
	})
	form.AddButton("Scale", func() {
		a.pages.RemovePage("scale-dialog")
		a.SetFocus(a.table)

		go func() {
			a.flashMsg(fmt.Sprintf("Scaling %s/%s to %s replicas...", ns, name, replicas), false)

			resourceType := resource
			if resourceType == "deploy" {
				resourceType = "deployment"
			} else if resourceType == "sts" {
				resourceType = "statefulset"
			} else if resourceType == "rs" {
				resourceType = "replicaset"
			}

			cmd := exec.Command("kubectl", "scale", resourceType, name, "-n", ns, "--replicas="+replicas)
			output, err := cmd.CombinedOutput()
			if err != nil {
				a.flashMsg(fmt.Sprintf("Scale failed: %s", string(output)), true)
				return
			}

			a.flashMsg(fmt.Sprintf("Scaled %s/%s to %s replicas", ns, name, replicas), false)
			a.refresh()
		}()
	})
	form.AddButton("Cancel", func() {
		a.pages.RemovePage("scale-dialog")
		a.SetFocus(a.table)
	})

	a.pages.AddPage("scale-dialog", centered(form, 50, 10), true, true)
}

// restartResource restarts a deployment/statefulset (k9s Shift+R key)
func (a *App) restartResource() {
	a.mx.RLock()
	resource := a.currentResource
	a.mx.RUnlock()

	restartable := map[string]bool{
		"deployments": true, "deploy": true,
		"statefulsets": true, "sts": true,
		"daemonsets": true, "ds": true,
	}

	if !restartable[resource] {
		a.flashMsg("Restart only available for deployments, statefulsets, daemonsets", true)
		return
	}

	row, _ := a.table.GetSelection()
	if row <= 0 {
		return
	}

	ns := a.table.GetCell(row, 0).Text
	name := a.table.GetCell(row, 1).Text

	modal := tview.NewModal().
		SetText(fmt.Sprintf("Restart %s?\n\n%s/%s\n\nThis will trigger a rolling restart.", resource, ns, name)).
		AddButtons([]string{"Cancel", "Restart"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			a.pages.RemovePage("restart-confirm")
			a.SetFocus(a.table)

			if buttonLabel == "Restart" {
				go func() {
					a.flashMsg(fmt.Sprintf("Restarting %s/%s...", ns, name), false)

					resourceType := resource
					if resourceType == "deploy" {
						resourceType = "deployment"
					} else if resourceType == "sts" {
						resourceType = "statefulset"
					} else if resourceType == "ds" {
						resourceType = "daemonset"
					}

					cmd := exec.Command("kubectl", "rollout", "restart", resourceType, name, "-n", ns)
					output, err := cmd.CombinedOutput()
					if err != nil {
						a.flashMsg(fmt.Sprintf("Restart failed: %s", string(output)), true)
						return
					}

					a.flashMsg(fmt.Sprintf("Restarted %s/%s", ns, name), false)
					a.refresh()
				}()
			}
		})

	a.pages.AddPage("restart-confirm", modal, true, true)
}

// showDescribe shows describe output for selected resource (like kubectl describe)
func (a *App) showDescribe() {
	row, _ := a.table.GetSelection()
	if row <= 0 {
		a.flashMsg("No resource selected", true)
		return
	}

	a.mx.RLock()
	resource := a.currentResource
	a.mx.RUnlock()

	// Get namespace and name from table
	nsCell := a.table.GetCell(row, 0)
	nameCell := a.table.GetCell(row, 1)
	if nsCell == nil || nameCell == nil {
		a.flashMsg("Cannot get resource info", true)
		return
	}

	ns := nsCell.Text
	name := nameCell.Text

	// For cluster-scoped resources, name is in column 0
	if resource == "nodes" || resource == "namespaces" || resource == "persistentvolumes" ||
		resource == "storageclasses" || resource == "clusterroles" || resource == "clusterrolebindings" ||
		resource == "customresourcedefinitions" {
		name = ns
		ns = ""
	}

	// Create modal with loading message
	descView := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetWrap(true)
	descView.SetBorder(true).
		SetTitle(fmt.Sprintf(" Describe: %s/%s (Press Esc or 'q' to close) ", resource, name)).
		SetTitleAlign(tview.AlignLeft)

	descView.SetText("[yellow]Loading...[white]")

	// Add to pages
	a.pages.AddPage("describe", descView, true, true)
	a.SetFocus(descView)

	// Fetch describe output in background
	go func() {
		ctx := a.prepareContext()
		output, err := a.k8s.DescribeResource(ctx, resource, ns, name)
		if err != nil {
			a.QueueUpdateDraw(func() {
				descView.SetText(fmt.Sprintf("[red]Error: %v[white]", err))
			})
			return
		}

		a.QueueUpdateDraw(func() {
			descView.SetText(output)
			descView.ScrollToBeginning()
		})
	}()

	descView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEsc:
			a.pages.RemovePage("describe")
			a.SetFocus(a.table)
			return nil
		case tcell.KeyRune:
			if event.Rune() == 'q' {
				a.pages.RemovePage("describe")
				a.SetFocus(a.table)
				return nil
			}
		}
		return event
	})
}

// toggleSelection toggles selection of the current row (k9s Space key)
func (a *App) toggleSelection() {
	row, _ := a.table.GetSelection()
	if row <= 0 { // Skip header row
		return
	}

	a.mx.Lock()
	if a.selectedRows[row] {
		delete(a.selectedRows, row)
	} else {
		a.selectedRows[row] = true
	}
	selectedCount := len(a.selectedRows)
	a.mx.Unlock()

	// Update row visual
	a.updateRowSelection(row)

	// Move to next row
	rowCount := a.table.GetRowCount()
	if row < rowCount-1 {
		a.table.Select(row+1, 0)
	}

	// Update status bar with selection count
	if selectedCount > 0 {
		a.flashMsg(fmt.Sprintf("%d item(s) selected - Ctrl+D to delete selected", selectedCount), false)
	}
}

// updateRowSelection updates visual styling for a row based on selection state
func (a *App) updateRowSelection(row int) {
	a.mx.RLock()
	isSelected := a.selectedRows[row]
	a.mx.RUnlock()

	colCount := a.table.GetColumnCount()
	for col := 0; col < colCount; col++ {
		cell := a.table.GetCell(row, col)
		if cell != nil {
			if isSelected {
				// Highlight selected rows with cyan background
				cell.SetBackgroundColor(tcell.ColorDarkCyan)
				cell.SetTextColor(tcell.ColorWhite)
			} else {
				// Reset to default
				cell.SetBackgroundColor(tcell.ColorDefault)
				cell.SetTextColor(tcell.ColorWhite)
			}
		}
	}
}

// clearSelections clears all selections
func (a *App) clearSelections() {
	a.mx.Lock()
	for row := range a.selectedRows {
		delete(a.selectedRows, row)
	}
	a.mx.Unlock()

	// Reset all row visuals
	rowCount := a.table.GetRowCount()
	for row := 1; row < rowCount; row++ {
		a.updateRowSelection(row)
	}
}

// getSelectedResources returns names of selected resources (or current if none selected)
func (a *App) getSelectedResources() []string {
	a.mx.RLock()
	selectedCount := len(a.selectedRows)
	a.mx.RUnlock()

	if selectedCount == 0 {
		// No selection, return current row
		row, _ := a.table.GetSelection()
		if row > 0 {
			cell := a.table.GetCell(row, 0)
			if cell != nil {
				name := strings.TrimSpace(tview.TranslateANSI(cell.Text))
				// Handle namespace/name format
				parts := strings.Fields(name)
				if len(parts) > 0 {
					return []string{parts[len(parts)-1]}
				}
			}
		}
		return nil
	}

	// Return all selected resources
	a.mx.RLock()
	defer a.mx.RUnlock()

	var resources []string
	for row := range a.selectedRows {
		cell := a.table.GetCell(row, 0)
		if cell != nil {
			// For namespaced resources, column 0 might be namespace, column 1 is name
			name := strings.TrimSpace(tview.TranslateANSI(cell.Text))
			// Check if there's a second column with name
			if a.table.GetColumnCount() > 1 {
				nameCell := a.table.GetCell(row, 1)
				if nameCell != nil {
					possibleName := strings.TrimSpace(tview.TranslateANSI(nameCell.Text))
					// If first column looks like a namespace, use second column
					if possibleName != "" && !strings.Contains(name, "-") {
						name = possibleName
					}
				}
			}
			if name != "" {
				resources = append(resources, name)
			}
		}
	}
	return resources
}

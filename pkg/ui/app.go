package ui

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
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

// Command definitions for autocomplete
var commands = []struct {
	name     string
	alias    string
	desc     string
	category string
}{
	{"pods", "po", "List pods", "resource"},
	{"deployments", "deploy", "List deployments", "resource"},
	{"services", "svc", "List services", "resource"},
	{"nodes", "no", "List nodes", "resource"},
	{"namespaces", "ns", "List namespaces", "resource"},
	{"events", "ev", "List events", "resource"},
	{"configmaps", "cm", "List configmaps", "resource"},
	{"secrets", "sec", "List secrets", "resource"},
	{"daemonsets", "ds", "List daemonsets", "resource"},
	{"statefulsets", "sts", "List statefulsets", "resource"},
	{"jobs", "job", "List jobs", "resource"},
	{"cronjobs", "cj", "List cronjobs", "resource"},
	{"ingresses", "ing", "List ingresses", "resource"},
	{"quit", "q", "Exit application", "action"},
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
	tableHeaders     []string   // Original headers
	tableRows        [][]string // Original rows (unfiltered)

	// Atomic guards (k9s pattern for lock-free update deduplication)
	inUpdate   int32
	running    int32 // 1 after Application.Run() starts
	cancelFn   context.CancelFunc
	cancelLock sync.Mutex

	// Logger
	logger *slog.Logger
}

// NewApp creates a new TUI application
func NewApp() *App {
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

	app := &App{
		Application:      tview.NewApplication(),
		config:           cfg,
		k8s:              k8sClient,
		aiClient:         aiClient,
		currentResource:  "pods",
		currentNamespace: "",
		namespaces:       []string{""},
		showAIPanel:      true,
		logger:           logger,
	}

	app.setupUI()
	app.setupKeybindings()

	return app
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

Please provide a concise, helpful answer.`, question)

	// Call AI
	if a.aiClient == nil || !a.aiClient.IsReady() {
		a.QueueUpdateDraw(func() {
			a.aiPanel.SetText(fmt.Sprintf("[yellow]Q:[white] %s\n\n[red]AI is not available.[white]\n\nConfigure LLM in config file:\n[gray]~/.kube-ai-dashboard/config.yaml", question))
		})
		return
	}

	// Use streaming API with callback to update UI progressively
	var fullResponse strings.Builder
	err := a.aiClient.Ask(ctx, prompt, func(chunk string) {
		fullResponse.WriteString(chunk)
		response := fullResponse.String()
		a.QueueUpdateDraw(func() {
			a.aiPanel.SetText(fmt.Sprintf("[yellow]Q:[white] %s\n\n[green]A:[white] %s", question, response))
		})
	})

	if err != nil {
		a.QueueUpdateDraw(func() {
			a.aiPanel.SetText(fmt.Sprintf("[yellow]Q:[white] %s\n\n[red]Error:[white] %v", question, err))
		})
	}
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

	input = strings.ToLower(input)
	var matches []string

	// Check for namespace command (ns <namespace>)
	if strings.HasPrefix(input, "ns ") || strings.HasPrefix(input, "namespace ") {
		prefix := strings.TrimPrefix(input, "ns ")
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

	// Match commands
	for _, cmd := range commands {
		if strings.HasPrefix(cmd.name, input) || strings.HasPrefix(cmd.alias, input) {
			matches = append(matches, cmd.name)
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

// selectNamespaceByNumber selects namespace by number
func (a *App) selectNamespaceByNumber(num int) {
	a.mx.Lock()
	defer a.mx.Unlock()

	if num >= len(a.namespaces) {
		return
	}

	a.currentNamespace = a.namespaces[num]
	go func() {
		a.updateHeader()
		a.refresh()
	}()
}

// startFilter activates filter mode
func (a *App) startFilter() {
	a.cmdInput.SetLabel(" / ")
	a.cmdHint.SetText("[gray]Type to filter, Enter to confirm, Esc to clear")
	a.cmdInput.SetText(a.filterText)
	a.SetFocus(a.cmdInput)

	// Override input handler for filter mode
	a.cmdInput.SetChangedFunc(func(text string) {
		a.applyFilter(text)
	})

	a.cmdInput.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEnter:
			a.mx.Lock()
			a.filterText = a.cmdInput.GetText()
			a.mx.Unlock()
			a.cmdInput.SetLabel(" : ")
			a.cmdHint.SetText("")
			a.restoreAutocompleteHandler()
			a.SetFocus(a.table)
			return nil

		case tcell.KeyEsc:
			a.mx.Lock()
			a.filterText = ""
			a.mx.Unlock()
			a.cmdInput.SetText("")
			a.cmdInput.SetLabel(" : ")
			a.cmdHint.SetText("")
			a.applyFilter("")
			a.restoreAutocompleteHandler()
			a.SetFocus(a.table)
			return nil
		}
		return event
	})
}

// applyFilter filters the table based on the given text
func (a *App) applyFilter(filter string) {
	a.mx.RLock()
	headers := a.tableHeaders
	rows := a.tableRows
	a.mx.RUnlock()

	if len(headers) == 0 || len(rows) == 0 {
		return
	}

	filter = strings.ToLower(filter)

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
			if filter != "" {
				match := false
				for _, cell := range row {
					if strings.Contains(strings.ToLower(cell), filter) {
						match = true
						break
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
				if filter != "" && strings.Contains(strings.ToLower(text), filter) {
					displayText = a.highlightMatch(text, filter)
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
			filterInfo = fmt.Sprintf(" [filter: %s]", filter)
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

// restoreAutocompleteHandler restores the default autocomplete behavior
func (a *App) restoreAutocompleteHandler() {
	a.setupAutocomplete()
}

// setupKeybindings configures keyboard shortcuts
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
			case '1':
				a.setResource("pods")
			case '2':
				a.setResource("deployments")
			case '3':
				a.setResource("services")
			case '4':
				a.setResource("nodes")
			case '5':
				a.setResource("namespaces")
			case '6':
				a.setResource("events")
			case 'l':
				a.showLogs()
				return nil
			case 'd':
				a.describeResource()
				return nil
			case 'x':
				a.confirmDelete()
				return nil
			case 's':
				a.execShell()
				return nil
			case 'F':
				a.portForward()
				return nil
			case 'c':
				a.showContextSwitcher()
				return nil
			case 'g':
				a.table.Select(1, 0)
				return nil
			case 'G':
				a.table.Select(a.table.GetRowCount()-1, 0)
				return nil
			}
		case tcell.KeyTab:
			if a.showAIPanel {
				a.SetFocus(a.aiInput)
			}
			return nil
		case tcell.KeyEnter:
			a.describeResource()
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
		case tcell.KeyEsc:
			a.SetFocus(a.table)
			return nil
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

// updateStatusBar updates the status bar
func (a *App) updateStatusBar() {
	shortcuts := " [black]1[white]Pods [black]2[white]Deploy [black]3[white]Svc [black]4[white]Nodes [black]5[white]NS [black]6[white]Events | " +
		"[black]/[white]Filter [black]s[white]Shell [black]x[white]Del [black]l[white]Logs [black]d[white]Desc [black]:[white]Cmd [black]?[white]Help [black]q[white]Quit"
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
		a.applyFilter(currentFilter)
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
	case "pods", "po":
		return a.fetchPods(ctx, ns)
	case "deployments", "deploy":
		return a.fetchDeployments(ctx, ns)
	case "services", "svc":
		return a.fetchServices(ctx, ns)
	case "nodes", "no":
		return a.fetchNodes(ctx)
	case "namespaces", "ns":
		return a.fetchNamespaces(ctx)
	case "events", "ev":
		return a.fetchEvents(ctx, ns)
	default:
		return nil, nil, fmt.Errorf("unknown resource type: %s", resource)
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

	a.updateHeader()
	go a.refresh()
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

	a.updateHeader()
	go a.refresh()
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
			a.updateHeader()
			go a.refresh()
		}
		return
	}

	switch cmd {
	case "pods", "po":
		a.setResource("pods")
	case "deployments", "deploy":
		a.setResource("deployments")
	case "services", "svc":
		a.setResource("services")
	case "nodes", "no":
		a.setResource("nodes")
	case "namespaces", "ns":
		a.setResource("namespaces")
	case "events", "ev":
		a.setResource("events")
	case "configmaps", "cm":
		a.setResource("configmaps")
	case "secrets", "sec":
		a.setResource("secrets")
	case "daemonsets", "ds":
		a.setResource("daemonsets")
	case "statefulsets", "sts":
		a.setResource("statefulsets")
	case "jobs", "job":
		a.setResource("jobs")
	case "cronjobs", "cj":
		a.setResource("cronjobs")
	case "ingresses", "ing":
		a.setResource("ingresses")
	case "health", "status":
		a.showHealth()
	case "context", "ctx":
		a.showContextSwitcher()
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
	row, _ := a.table.GetSelection()
	if row <= 0 {
		return
	}

	a.mx.RLock()
	resource := a.currentResource
	a.mx.RUnlock()

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
	a.refresh()
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
		SetText(`
 [yellow::b]k13s - Kubernetes AI Dashboard[white::-]

 [cyan]Navigation:[white]
   [yellow]j/k[white] or [yellow]Up/Down[white]  Move selection
   [yellow]g[white]                 Go to top
   [yellow]G[white]                 Go to bottom
   [yellow]Enter[white] or [yellow]d[white]       Describe resource (YAML)
   [yellow]l[white]                 View logs (pods only)
   [yellow]s[white]                 Shell into pod
   [yellow]Shift+F[white]           Port forward
   [yellow]x[white]                 Delete resource
   [yellow]c[white]                 Switch context
   [yellow]Tab[white]               Switch to AI panel

 [cyan]Resource Selection:[white]
   [yellow]1[white]  Pods           [yellow]4[white]  Nodes
   [yellow]2[white]  Deployments    [yellow]5[white]  Namespaces
   [yellow]3[white]  Services       [yellow]6[white]  Events

 [cyan]Commands:[white]
   [yellow]:[white]       Command mode
   [yellow]/[white]       Filter mode
   [yellow]n[white]       Cycle namespace
   [yellow]r[white]       Refresh
   [yellow]?[white]       Show this help
   [yellow]q[white]       Quit

 [cyan]Command Examples:[white]
   [yellow]:pods[white], [yellow]:deploy[white], [yellow]:svc[white], [yellow]:cm[white], [yellow]:sec[white]
   [yellow]:ns default[white], [yellow]:ctx[white], [yellow]:health[white]

 [gray]Press Esc to close[white]
`)
	help.SetBorder(true).SetTitle(" Help ")

	a.pages.AddPage("help", centered(help, 60, 32), true, true)
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

//go:build ignore

// Kit Kit — Meta-agent that builds Kit agents
//
// A team of domain-specific research experts operate IN PARALLEL to gather
// documentation and patterns. The primary agent synthesizes their findings
// and WRITES the actual files.
//
// Each expert runs as a separate `kit` subprocess with a domain-specific
// system prompt. Experts are read-only researchers; the primary agent is
// the only writer.
//
// Commands:
//
//	/experts          — list available experts and their status
//	/experts-grid N   — set dashboard column count (default 3)
//
// Usage: kit -e examples/extensions/kit-kit.go
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"kit/ext"
)

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

type expertDef struct {
	Name        string
	Description string
	Tools       string
	System      string // system prompt body
	File        string
}

type expertState struct {
	Def        expertDef
	Status     string // "idle", "researching", "done", "error"
	Question   string
	Elapsed    time.Duration
	LastLine   string
	QueryCount int
	mu         sync.Mutex
}

func (s *expertState) set(status, question, lastLine string, elapsed time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if status != "" {
		s.Status = status
	}
	if question != "" {
		s.Question = question
	}
	if lastLine != "" {
		s.LastLine = lastLine
	}
	if elapsed > 0 {
		s.Elapsed = elapsed
	}
}

func (s *expertState) snapshot() (string, string, string, time.Duration, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.Status, s.Question, s.LastLine, s.Elapsed, s.QueryCount
}

// ---------------------------------------------------------------------------
// Package-level state
// ---------------------------------------------------------------------------

var (
	mu        sync.Mutex
	experts   = map[string]*expertState{}
	gridCols  = 3
	latestCtx ext.Context
	hasCtx    bool
	kitBinary string // resolved path to kit executable
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func displayName(name string) string {
	parts := strings.Split(name, "-")
	for i, w := range parts {
		if len(w) > 0 {
			parts[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(parts, " ")
}

func runeWidth(s string) int {
	return len([]rune(s))
}

func truncate(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	if max < 4 {
		return string(runes[:max])
	}
	return string(runes[:max-3]) + "..."
}

func pad(s string, width int) string {
	w := runeWidth(s)
	if w >= width {
		return string([]rune(s)[:width])
	}
	return s + strings.Repeat(" ", width-w)
}

// parseAgentFile reads a .md file with YAML-like frontmatter.
//
//	---
//	name: ext-expert
//	description: Extensions documentation
//	tools: read,grep,glob
//	---
//	System prompt body here ...
func parseAgentFile(path string) *expertDef {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	text := string(raw)

	// Must start with "---\n"
	if !strings.HasPrefix(text, "---\n") {
		return nil
	}
	rest := text[4:]
	idx := strings.Index(rest, "\n---\n")
	if idx < 0 {
		return nil
	}
	frontmatter := rest[:idx]
	body := strings.TrimSpace(rest[idx+5:])

	fm := map[string]string{}
	for _, line := range strings.Split(frontmatter, "\n") {
		i := strings.Index(line, ":")
		if i > 0 {
			fm[strings.TrimSpace(line[:i])] = strings.TrimSpace(line[i+1:])
		}
	}
	if fm["name"] == "" {
		return nil
	}
	return &expertDef{
		Name:        fm["name"],
		Description: fm["description"],
		Tools:       fm["tools"],
		System:      body,
		File:        path,
	}
}

func loadExperts(cwd string) {
	mu.Lock()
	defer mu.Unlock()

	experts = map[string]*expertState{}
	dir := filepath.Join(cwd, ".kit", "agents", "kit-kit")

	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		if e.Name() == "orchestrator.md" {
			continue
		}
		def := parseAgentFile(filepath.Join(dir, e.Name()))
		if def == nil {
			continue
		}
		key := strings.ToLower(def.Name)
		experts[key] = &expertState{
			Def:    *def,
			Status: "idle",
		}
	}
}

func expertList() []*expertState {
	mu.Lock()
	defer mu.Unlock()
	list := make([]*expertState, 0, len(experts))
	for _, s := range experts {
		list = append(list, s)
	}
	return list
}

func expertNames() string {
	list := expertList()
	names := make([]string, len(list))
	for i, s := range list {
		names[i] = displayName(s.Def.Name)
	}
	return strings.Join(names, ", ")
}

// ---------------------------------------------------------------------------
// Widget grid rendering
// ---------------------------------------------------------------------------

func renderCard(s *expertState, w int) []string {
	status, question, lastLine, elapsed, queryCount := s.snapshot()
	inner := w - 2 // inside the box-drawing borders

	// Name line
	name := truncate(displayName(s.Def.Name), inner-1)

	// Status line
	var icon string
	switch status {
	case "idle":
		icon = "○"
	case "researching":
		icon = "◉"
	case "done":
		icon = "✓"
	default:
		icon = "✗"
	}
	statusText := icon + " " + status
	if status != "idle" {
		statusText += fmt.Sprintf(" %ds", int(elapsed.Seconds()))
	}
	if queryCount > 0 {
		statusText += fmt.Sprintf(" (%d)", queryCount)
	}
	statusText = truncate(statusText, inner-1)

	// Work line (question or description)
	work := question
	if work == "" {
		work = s.Def.Description
	}
	work = truncate(work, inner-1)

	// Last output line
	last := lastLine
	if last == "" {
		last = "—"
	}
	last = truncate(last, inner-1)

	// Build card (use rune width for box-drawing alignment)
	topBar := "─ " + name + " "
	if runeWidth(topBar) < inner {
		topBar += strings.Repeat("─", inner-runeWidth(topBar))
	}

	return []string{
		"┌" + truncate(topBar, inner) + "┐",
		"│ " + pad(statusText, inner-1) + "│",
		"│ " + pad(work, inner-1) + "│",
		"│ " + pad(last, inner-1) + "│",
		"└" + strings.Repeat("─", inner) + "┘",
	}
}

func buildGrid() string {
	list := expertList()
	if len(list) == 0 {
		return "No experts found. Add agent .md files to .kit/agents/kit-kit/"
	}

	cols := gridCols
	if cols > len(list) {
		cols = len(list)
	}

	// Card width: aim for ~28 chars per card
	cardWidth := 28
	gap := 1

	var lines []string
	for i := 0; i < len(list); i += cols {
		end := i + cols
		if end > len(list) {
			end = len(list)
		}
		row := list[i:end]

		// Render each card in this row
		cards := make([][]string, len(row))
		maxHeight := 0
		for j, s := range row {
			cards[j] = renderCard(s, cardWidth)
			if len(cards[j]) > maxHeight {
				maxHeight = len(cards[j])
			}
		}

		// Merge columns line by line
		for line := 0; line < maxHeight; line++ {
			var parts []string
			for _, card := range cards {
				if line < len(card) {
					parts = append(parts, card[line])
				} else {
					parts = append(parts, strings.Repeat(" ", cardWidth))
				}
			}
			lines = append(lines, strings.Join(parts, strings.Repeat(" ", gap)))
		}
	}
	return strings.Join(lines, "\n")
}

func updateWidget() {
	mu.Lock()
	ctx := latestCtx
	ok := hasCtx
	mu.Unlock()
	if !ok {
		return
	}
	ctx.SetWidget(ext.WidgetConfig{
		ID:        "kit-kit:grid",
		Placement: ext.WidgetAbove,
		Content: ext.WidgetContent{
			Text: buildGrid(),
		},
		Style: ext.WidgetStyle{
			NoBorder:    true,
			BorderColor: "",
		},
		Priority: 10,
	})
}

func updateFooter() {
	mu.Lock()
	ctx := latestCtx
	ok := hasCtx
	mu.Unlock()
	if !ok {
		return
	}

	list := expertList()
	active := 0
	done := 0
	for _, s := range list {
		st, _, _, _, _ := s.snapshot()
		switch st {
		case "researching":
			active++
		case "done":
			done++
		}
	}

	var mid string
	if active > 0 {
		mid = fmt.Sprintf("  ◉ %d researching", active)
	} else if done > 0 {
		mid = fmt.Sprintf("  ✓ %d done", done)
	}

	text := fmt.Sprintf("%s  |  Kit Kit%s", ctx.Model, mid)

	ctx.SetFooter(ext.HeaderFooterConfig{
		Content: ext.WidgetContent{Text: text},
		Style:   ext.WidgetStyle{BorderColor: "#89b4fa"},
	})
}

// ---------------------------------------------------------------------------
// Kit binary resolution
// ---------------------------------------------------------------------------

func findKitBinary() string {
	// Try the current process executable first.
	if exe, err := os.Executable(); err == nil {
		if _, err := os.Stat(exe); err == nil {
			return exe
		}
	}
	// Fall back to PATH lookup.
	if p, err := exec.LookPath("kit"); err == nil {
		return p
	}
	return "kit"
}

// ---------------------------------------------------------------------------
// Expert query (subprocess)
// ---------------------------------------------------------------------------

func queryExpert(name, question string) (output string, exitCode int, elapsed time.Duration) {
	mu.Lock()
	state, ok := experts[strings.ToLower(name)]
	mu.Unlock()
	if !ok {
		return fmt.Sprintf("Expert %q not found.", name), 1, 0
	}

	// Mark as researching.
	state.mu.Lock()
	if state.Status == "researching" {
		state.mu.Unlock()
		return fmt.Sprintf("Expert %q is already researching.", displayName(name)), 1, 0
	}
	state.Status = "researching"
	state.Question = question
	state.Elapsed = 0
	state.LastLine = ""
	state.QueryCount++
	state.mu.Unlock()
	updateWidget()

	start := time.Now()

	// Timer goroutine: update widget every second while researching.
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				state.set("", "", "", time.Since(start))
				updateWidget()
				updateFooter()
			}
		}
	}()

	// Write system prompt to temp file.
	tmpFile, err := os.CreateTemp("", "kit-kit-*.txt")
	if err != nil {
		close(done)
		state.set("error", "", "temp file error: "+err.Error(), time.Since(start))
		updateWidget()
		updateFooter()
		return "Error creating temp file: " + err.Error(), 1, time.Since(start)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(state.Def.System); err != nil {
		tmpFile.Close()
		close(done)
		state.set("error", "", "write error: "+err.Error(), time.Since(start))
		updateWidget()
		updateFooter()
		return "Error writing system prompt: " + err.Error(), 1, time.Since(start)
	}
	tmpFile.Close()

	// Build subprocess arguments. Don't pass --model; the subprocess
	// inherits the same config/env and will use the same default.
	args := []string{
		"--prompt", question,
		"--quiet",
		"--no-session",
		"--no-extensions",
		"--system-prompt", tmpFile.Name(),
	}

	cmd := exec.Command(kitBinary, args...)
	cmd.Env = os.Environ()

	outBytes, err := cmd.CombinedOutput()
	close(done)
	elapsed = time.Since(start)
	result := strings.TrimSpace(string(outBytes))

	if err != nil {
		// Extract a single-line summary for the card (no newlines).
		errLine := result
		if idx := strings.Index(errLine, "\n"); idx >= 0 {
			errLine = errLine[:idx]
		}
		state.set("error", "", truncate(strings.TrimSpace(errLine), 80), elapsed)
		updateWidget()
		updateFooter()
		code := 1
		if exitErr, ok := err.(*exec.ExitError); ok {
			code = exitErr.ExitCode()
		}
		return result, code, elapsed
	}

	// Success — extract last non-empty line for the card.
	lines := strings.Split(result, "\n")
	var lastLine string
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.TrimSpace(lines[i]) != "" {
			lastLine = lines[i]
			break
		}
	}
	state.set("done", "", truncate(lastLine, 60), elapsed)
	updateWidget()
	updateFooter()

	return result, 0, elapsed
}

// ---------------------------------------------------------------------------
// Orchestrator system prompt
// ---------------------------------------------------------------------------

func buildOrchestratorPrompt(cwd string) string {
	orchPath := filepath.Join(cwd, ".kit", "agents", "kit-kit", "orchestrator.md")
	raw, err := os.ReadFile(orchPath)
	if err != nil {
		// Fallback: generate a basic orchestrator prompt.
		return buildDefaultOrchestratorPrompt()
	}

	text := string(raw)
	// Strip frontmatter if present.
	if strings.HasPrefix(text, "---\n") {
		if idx := strings.Index(text[4:], "\n---\n"); idx >= 0 {
			text = strings.TrimSpace(text[4+idx+5:])
		}
	}

	list := expertList()
	catalog := buildExpertCatalog(list)
	names := make([]string, len(list))
	for i, s := range list {
		names[i] = displayName(s.Def.Name)
	}

	text = strings.ReplaceAll(text, "{{EXPERT_COUNT}}", fmt.Sprintf("%d", len(list)))
	text = strings.ReplaceAll(text, "{{EXPERT_NAMES}}", strings.Join(names, ", "))
	text = strings.ReplaceAll(text, "{{EXPERT_CATALOG}}", catalog)
	return text
}

func buildExpertCatalog(list []*expertState) string {
	var sb strings.Builder
	for _, s := range list {
		fmt.Fprintf(&sb, "### %s\n", displayName(s.Def.Name))
		fmt.Fprintf(&sb, "**Query as:** `%s`\n", s.Def.Name)
		fmt.Fprintf(&sb, "%s\n\n", s.Def.Description)
	}
	return sb.String()
}

func buildDefaultOrchestratorPrompt() string {
	list := expertList()
	names := make([]string, len(list))
	for i, s := range list {
		names[i] = displayName(s.Def.Name)
	}
	catalog := buildExpertCatalog(list)

	return fmt.Sprintf(`You are Kit Kit, an orchestrator agent with %d domain experts: %s.

Use the query_experts tool to consult experts IN PARALLEL before writing code.
Always query multiple experts at once when the task spans multiple domains.

## Available Experts

%s

## Workflow

1. Analyze the user's request to identify which domains are relevant.
2. Use query_experts to ask specific questions of the relevant experts.
3. Synthesize the expert findings into a coherent implementation.
4. Write the actual code/files — you are the only agent that writes.

## Rules

- ALWAYS query experts before implementing. Never guess.
- Ask SPECIFIC questions. "How does X work?" is better than "Tell me about X".
- Query multiple experts in a single call when possible (they run in parallel).
- If an expert returns insufficient info, query again with a more specific question.
`, len(list), strings.Join(names, ", "), catalog)
}

// ---------------------------------------------------------------------------
// Init
// ---------------------------------------------------------------------------

func Init(api ext.API) {
	kitBinary = findKitBinary()

	// ── Session Start: load experts, show grid ──
	api.OnSessionStart(func(_ ext.SessionStartEvent, ctx ext.Context) {
		mu.Lock()
		latestCtx = ctx
		hasCtx = true
		mu.Unlock()

		loadExperts(ctx.CWD)
		updateWidget()
		updateFooter()

		names := expertNames()
		n := len(expertList())
		if n > 0 {
			ctx.PrintInfo(fmt.Sprintf(
				"Kit Kit loaded — %d experts: %s\n\n"+
					"/experts          List experts and status\n"+
					"/experts-grid N   Set grid columns (1-5)\n\n"+
					"Ask me to build any Kit component!",
				n, names))
		} else {
			ctx.PrintInfo(
				"Kit Kit loaded — no experts found.\n\n" +
					"Add agent .md files to .kit/agents/kit-kit/ to get started.\n" +
					"See examples/extensions/kit-kit-agents/ for samples.")
		}
	})

	// ── Before Agent Start: inject orchestrator system prompt ──
	api.OnBeforeAgentStart(func(_ ext.BeforeAgentStartEvent, ctx ext.Context) *ext.BeforeAgentStartResult {
		mu.Lock()
		latestCtx = ctx
		mu.Unlock()

		prompt := buildOrchestratorPrompt(ctx.CWD)
		return &ext.BeforeAgentStartResult{SystemPrompt: &prompt}
	})

	// ── Agent End: update footer ──
	api.OnAgentEnd(func(_ ext.AgentEndEvent, ctx ext.Context) {
		mu.Lock()
		latestCtx = ctx
		mu.Unlock()
		updateFooter()
	})

	// ── Session Shutdown: cleanup ──
	api.OnSessionShutdown(func(_ ext.SessionShutdownEvent, ctx ext.Context) {
		ctx.RemoveWidget("kit-kit:grid")
		ctx.RemoveFooter()
	})

	// ── Tool: query_experts ──
	api.RegisterTool(ext.ToolDef{
		Name: "query_experts",
		Description: `Query one or more Kit domain experts IN PARALLEL. All experts run simultaneously as concurrent subprocesses.

Pass an array of queries — each with an expert name and a specific question. All experts start at the same time and their results are returned together.

Available experts are loaded from .kit/agents/kit-kit/*.md at session start. The default set includes:
- ext-expert: Kit extensions — tools, events, commands, widgets, editor interceptors
- tui-expert: Kit TUI — Bubble Tea v2 components, rendering, theming, layout
- llm-expert: Kit LLM system — providers, streaming, agent loop, tool execution

Ask specific questions about what you need to BUILD. Each expert will return documentation excerpts, code patterns, and implementation guidance.`,
		Parameters: `{
  "type": "object",
  "properties": {
    "queries": {
      "type": "array",
      "description": "Array of expert queries to run in parallel",
      "items": {
        "type": "object",
        "properties": {
          "expert": {
            "type": "string",
            "description": "Expert name (e.g. ext-expert, tui-expert, llm-expert)"
          },
          "question": {
            "type": "string",
            "description": "Specific question about what you need to build"
          }
        },
        "required": ["expert", "question"]
      }
    }
  },
  "required": ["queries"]
}`,
		Execute: func(input string) (string, error) {
			var params struct {
				Queries []struct {
					Expert   string `json:"expert"`
					Question string `json:"question"`
				} `json:"queries"`
			}
			if err := json.Unmarshal([]byte(input), &params); err != nil {
				return "", fmt.Errorf("invalid parameters: %w", err)
			}
			if len(params.Queries) == 0 {
				return "No queries provided.", nil
			}

			// Launch all experts in parallel.
			type result struct {
				Expert   string
				Question string
				Output   string
				ExitCode int
				Elapsed  time.Duration
			}
			results := make([]result, len(params.Queries))
			var wg sync.WaitGroup

			for i, q := range params.Queries {
				wg.Add(1)
				go func(idx int, expert, question string) {
					defer wg.Done()
					out, code, elapsed := queryExpert(expert, question)
					results[idx] = result{
						Expert:   expert,
						Question: question,
						Output:   out,
						ExitCode: code,
						Elapsed:  elapsed,
					}
				}(i, q.Expert, q.Question)
			}
			wg.Wait()

			// Build combined response.
			var sb strings.Builder
			for _, r := range results {
				icon := "✓"
				if r.ExitCode != 0 {
					icon = "✗"
				}
				fmt.Fprintf(&sb, "## [%s] %s (%ds)\n\n",
					icon, displayName(r.Expert), int(r.Elapsed.Seconds()))

				out := r.Output
				if len(out) > 12000 {
					out = out[:12000] + "\n\n... [truncated — ask follow-up for more]"
				}
				sb.WriteString(out)
				sb.WriteString("\n\n---\n\n")
			}
			return sb.String(), nil
		},
	})

	// ── Tool Renderer: query_experts ──
	api.RegisterToolRenderer(ext.ToolRenderConfig{
		ToolName:    "query_experts",
		DisplayName: "Query Experts",
		BorderColor: "#89b4fa",
		RenderHeader: func(toolArgs string, width int) string {
			var args struct {
				Queries []struct {
					Expert string `json:"expert"`
				} `json:"queries"`
			}
			if err := json.Unmarshal([]byte(toolArgs), &args); err != nil {
				return ""
			}
			names := make([]string, len(args.Queries))
			for i, q := range args.Queries {
				names[i] = displayName(q.Expert)
			}
			header := fmt.Sprintf("%d experts in parallel: %s",
				len(args.Queries), strings.Join(names, ", "))
			return truncate(header, width)
		},
		RenderBody: func(toolResult string, isError bool, width int) string {
			if isError {
				return "" // fall back to default
			}
			// Show compact summary: extract ## headers with status
			var lines []string
			for _, line := range strings.Split(toolResult, "\n") {
				if strings.HasPrefix(line, "## [") {
					lines = append(lines, line[3:]) // strip "## "
				}
			}
			if len(lines) == 0 {
				return ""
			}
			return strings.Join(lines, "  ·  ")
		},
	})

	// ── Command: /experts ──
	api.RegisterCommand(ext.CommandDef{
		Name:        "experts",
		Description: "List available Kit Kit experts and their status",
		Execute: func(args string, ctx ext.Context) (string, error) {
			mu.Lock()
			latestCtx = ctx
			mu.Unlock()

			list := expertList()
			if len(list) == 0 {
				return "No experts loaded. Add agent .md files to .kit/agents/kit-kit/", nil
			}
			var sb strings.Builder
			for _, s := range list {
				status, _, _, _, qc := s.snapshot()
				fmt.Fprintf(&sb, "%s (%s, queries: %d): %s\n",
					displayName(s.Def.Name), status, qc, s.Def.Description)
			}
			return sb.String(), nil
		},
	})

	// ── Command: /experts-grid ──
	api.RegisterCommand(ext.CommandDef{
		Name:        "experts-grid",
		Description: "Set expert grid columns: /experts-grid <1-5>",
		Execute: func(args string, ctx ext.Context) (string, error) {
			mu.Lock()
			latestCtx = ctx
			mu.Unlock()

			args = strings.TrimSpace(args)
			n := 0
			if _, err := fmt.Sscanf(args, "%d", &n); err != nil || n < 1 || n > 5 {
				return "Usage: /experts-grid <1-5>", nil
			}
			mu.Lock()
			gridCols = n
			mu.Unlock()
			updateWidget()
			return fmt.Sprintf("Grid set to %d columns.", n), nil
		},
	})
}

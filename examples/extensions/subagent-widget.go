//go:build ignore

// Subagent Widget — /sub, /subclear, /subrm, /subcont commands with live widgets
//
// Each /sub spawns a background Kit subagent as a subprocess with its own
// live widget showing status, task, elapsed time, and last output line.
// /subcont continues a finished subagent by passing conversation history.
//
// Commands:
//
//	/sub <task>              — spawn a new subagent
//	/subcont <id> <prompt>   — continue subagent #<id>'s conversation
//	/subrm <id>              — remove subagent #<id> widget
//	/subclear                — clear all subagent widgets
//
// The LLM can also use tools: subagent_create, subagent_continue,
// subagent_remove, subagent_list.
//
// Ported from https://github.com/disler/pi-vs-claude-code extensions/subagent-widget.ts
//
// Usage: kit -e examples/extensions/subagent-widget.go
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"kit/ext"
)

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

type subState struct {
	ID        int
	Status    string // "running", "done", "error"
	Task      string
	Chunks    []string // accumulated output chunks
	Elapsed   time.Duration
	TurnCount int
	History   string      // conversation history for /subcont
	Proc      *os.Process // active process for killing
	Removed   bool        // set when /subrm or /subclear removes this agent
	mu        sync.Mutex
}

func (s *subState) appendChunk(chunk string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Chunks = append(s.Chunks, chunk)
}

func (s *subState) setElapsed(d time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Elapsed = d
}

func (s *subState) setProc(p *os.Process) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Proc = p
}

func (s *subState) snapshot() (int, string, string, string, time.Duration, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fullText := strings.Join(s.Chunks, "")
	return s.ID, s.Status, s.Task, fullText, s.Elapsed, s.TurnCount
}

// ---------------------------------------------------------------------------
// Package-level state
// ---------------------------------------------------------------------------

var (
	mu        sync.Mutex
	latestCtx ext.Context
	hasCtx    bool
	agents    = map[int]*subState{}
	nextID    = 1
	kitBinary string
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func findKitBinary() string {
	if exe, err := os.Executable(); err == nil {
		if _, err := os.Stat(exe); err == nil {
			return exe
		}
	}
	if p, err := exec.LookPath("kit"); err == nil {
		return p
	}
	return "kit"
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

func lastNonEmptyLine(text string) string {
	lines := strings.Split(text, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

// ---------------------------------------------------------------------------
// Widget rendering
// ---------------------------------------------------------------------------

func updateWidgets() {
	mu.Lock()
	ctx := latestCtx
	ok := hasCtx
	agentsCopy := make([]*subState, 0, len(agents))
	for _, s := range agents {
		agentsCopy = append(agentsCopy, s)
	}
	mu.Unlock()
	if !ok {
		return
	}

	for _, state := range agentsCopy {
		id, status, task, fullText, elapsed, turnCount := state.snapshot()

		var icon, color string
		switch status {
		case "running":
			icon = "●"
			color = "#89b4fa" // blue
		case "done":
			icon = "✓"
			color = "#a6e3a1" // green
		default:
			icon = "✗"
			color = "#f38ba8" // red
		}

		taskPreview := truncate(task, 40)

		turnLabel := ""
		if turnCount > 1 {
			turnLabel = fmt.Sprintf(" · Turn %d", turnCount)
		}

		header := fmt.Sprintf("%s Subagent #%d%s  %s  (%ds)",
			icon, id, turnLabel, taskPreview, int(elapsed.Seconds()))

		lastLine := truncate(lastNonEmptyLine(fullText), 80)

		text := header
		if lastLine != "" {
			text += "\n  " + lastLine
		}

		ctx.SetWidget(ext.WidgetConfig{
			ID:        fmt.Sprintf("subagent:%d", id),
			Placement: ext.WidgetAbove,
			Content:   ext.WidgetContent{Text: text},
			Style:     ext.WidgetStyle{BorderColor: color},
			Priority:  id,
		})
	}
}

// ---------------------------------------------------------------------------
// Subprocess spawning
// ---------------------------------------------------------------------------

func spawnAgent(state *subState) {
	prompt := state.Task

	state.mu.Lock()
	history := state.History
	state.mu.Unlock()

	if history != "" {
		prompt = "Previous conversation:\n" + history + "\n\nNew instruction: " + state.Task
	}

	args := []string{
		"--prompt", prompt,
		"--quiet",
		"--no-session",
		"--no-extensions",
	}

	cmd := exec.Command(kitBinary, args...)
	cmd.Env = os.Environ()

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		state.mu.Lock()
		state.Status = "error"
		state.Chunks = append(state.Chunks, "Pipe error: "+err.Error())
		state.mu.Unlock()
		updateWidgets()
		return
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		state.mu.Lock()
		state.Status = "error"
		state.Chunks = append(state.Chunks, "Pipe error: "+err.Error())
		state.mu.Unlock()
		updateWidgets()
		return
	}

	start := time.Now()
	if err := cmd.Start(); err != nil {
		state.mu.Lock()
		state.Status = "error"
		state.Chunks = append(state.Chunks, "Start error: "+err.Error())
		state.mu.Unlock()
		updateWidgets()
		return
	}

	state.setProc(cmd.Process)

	// Timer goroutine: update widget every second with elapsed time.
	doneCh := make(chan struct{})
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-doneCh:
				return
			case <-ticker.C:
				state.setElapsed(time.Since(start))
				updateWidgets()
			}
		}
	}()

	// Read stderr in background goroutine.
	var readWg sync.WaitGroup
	readWg.Add(1)
	go func() {
		defer readWg.Done()
		scanner := bufio.NewScanner(stderr)
		scanner.Buffer(make([]byte, 256*1024), 256*1024)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.TrimSpace(line) != "" {
				state.appendChunk(line + "\n")
				updateWidgets()
			}
		}
	}()

	// Read stdout in foreground.
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 256*1024), 256*1024)
	for scanner.Scan() {
		state.appendChunk(scanner.Text() + "\n")
		updateWidgets()
	}

	// Wait for all pipe readers, then the process.
	readWg.Wait()
	waitErr := cmd.Wait()
	close(doneCh) // stop timer

	state.mu.Lock()
	state.Elapsed = time.Since(start)
	state.Proc = nil
	if waitErr != nil {
		state.Status = "error"
	} else {
		state.Status = "done"
	}
	result := strings.Join(state.Chunks, "")

	// Save history for /subcont continuations (cap at 16 KB).
	state.History += fmt.Sprintf("\n--- Turn %d ---\nTask: %s\nResult:\n%s\n",
		state.TurnCount, state.Task, result)
	if len(state.History) > 16000 {
		state.History = state.History[len(state.History)-16000:]
	}

	removed := state.Removed
	id := state.ID
	elapsed := state.Elapsed
	turnCount := state.TurnCount
	task := state.Task
	state.mu.Unlock()

	updateWidgets()

	// Don't deliver follow-up for agents removed via /subrm or /subclear.
	if removed {
		return
	}

	// Deliver result as a follow-up message so the LLM can act on it.
	mu.Lock()
	ctx := latestCtx
	ok := hasCtx
	mu.Unlock()

	if ok {
		resultText := result
		if len(resultText) > 8000 {
			resultText = resultText[:8000] + "\n\n... [truncated]"
		}
		turnSuffix := ""
		if turnCount > 1 {
			turnSuffix = fmt.Sprintf(" (Turn %d)", turnCount)
		}
		ctx.SendMessage(fmt.Sprintf(
			"Subagent #%d%s finished \"%s\" in %ds.\n\nResult:\n%s",
			id, turnSuffix, task, int(elapsed.Seconds()), resultText,
		))
	}
}

// ---------------------------------------------------------------------------
// Init
// ---------------------------------------------------------------------------

func Init(api ext.API) {
	kitBinary = findKitBinary()

	// ── Session Start: reset state, show help ──
	api.OnSessionStart(func(_ ext.SessionStartEvent, ctx ext.Context) {
		mu.Lock()
		latestCtx = ctx
		hasCtx = true
		mu.Unlock()

		// Kill lingering agents from previous session.
		mu.Lock()
		for id, state := range agents {
			state.mu.Lock()
			if state.Proc != nil && state.Status == "running" {
				state.Proc.Kill()
			}
			state.mu.Unlock()
			ctx.RemoveWidget(fmt.Sprintf("subagent:%d", id))
		}
		agents = map[int]*subState{}
		nextID = 1
		mu.Unlock()

		ctx.PrintInfo(
			"Subagent Widget loaded\n\n" +
				"/sub <task>              Spawn a new subagent\n" +
				"/subcont <id> <prompt>   Continue a finished subagent\n" +
				"/subrm <id>             Remove a subagent\n" +
				"/subclear               Clear all subagents\n\n" +
				"The LLM can also spawn subagents with the subagent_create tool.")
	})

	// ── Agent End: keep context fresh ──
	api.OnAgentEnd(func(_ ext.AgentEndEvent, ctx ext.Context) {
		mu.Lock()
		latestCtx = ctx
		mu.Unlock()
	})

	// ── Session Shutdown: cleanup ──
	api.OnSessionShutdown(func(_ ext.SessionShutdownEvent, ctx ext.Context) {
		mu.Lock()
		defer mu.Unlock()
		for id, state := range agents {
			state.mu.Lock()
			if state.Proc != nil && state.Status == "running" {
				state.Proc.Kill()
			}
			state.mu.Unlock()
			ctx.RemoveWidget(fmt.Sprintf("subagent:%d", id))
		}
		agents = map[int]*subState{}
	})

	// ── Tool: subagent_create ──
	api.RegisterTool(ext.ToolDef{
		Name: "subagent_create",
		Description: `Spawn a background subagent to perform a task. Returns the subagent ID immediately while it runs in the background. Results are delivered as a follow-up message when the subagent finishes.

Each subagent runs as a separate Kit subprocess with full tool access. Use this to delegate independent subtasks that can run in parallel with your main work.`,
		Parameters: `{
  "type": "object",
  "properties": {
    "task": {
      "type": "string",
      "description": "The complete task description for the subagent to perform"
    }
  },
  "required": ["task"]
}`,
		Execute: func(input string) (string, error) {
			var params struct {
				Task string `json:"task"`
			}
			if err := json.Unmarshal([]byte(input), &params); err != nil {
				return "", fmt.Errorf("invalid parameters: %w", err)
			}
			if strings.TrimSpace(params.Task) == "" {
				return "", fmt.Errorf("task is required")
			}

			mu.Lock()
			id := nextID
			nextID++
			state := &subState{
				ID:        id,
				Status:    "running",
				Task:      params.Task,
				TurnCount: 1,
			}
			agents[id] = state
			mu.Unlock()

			updateWidgets()
			go spawnAgent(state)

			return fmt.Sprintf("Subagent #%d spawned and running in background.", id), nil
		},
	})

	// ── Tool: subagent_continue ──
	api.RegisterTool(ext.ToolDef{
		Name:        "subagent_continue",
		Description: `Continue an existing subagent's conversation with a follow-up prompt. The subagent receives its previous conversation history as context. Use this to refine or extend a finished subagent's work.`,
		Parameters: `{
  "type": "object",
  "properties": {
    "id": {
      "type": "number",
      "description": "The ID of the subagent to continue"
    },
    "prompt": {
      "type": "string",
      "description": "The follow-up prompt or new instructions"
    }
  },
  "required": ["id", "prompt"]
}`,
		Execute: func(input string) (string, error) {
			var params struct {
				ID     int    `json:"id"`
				Prompt string `json:"prompt"`
			}
			if err := json.Unmarshal([]byte(input), &params); err != nil {
				return "", fmt.Errorf("invalid parameters: %w", err)
			}

			mu.Lock()
			state, ok := agents[params.ID]
			mu.Unlock()
			if !ok {
				return fmt.Sprintf("Error: No subagent #%d found.", params.ID), nil
			}

			state.mu.Lock()
			if state.Status == "running" {
				state.mu.Unlock()
				return fmt.Sprintf("Error: Subagent #%d is still running.", params.ID), nil
			}
			state.Status = "running"
			state.Task = params.Prompt
			state.Chunks = nil
			state.Elapsed = 0
			state.TurnCount++
			turn := state.TurnCount
			state.mu.Unlock()

			updateWidgets()
			go spawnAgent(state)

			return fmt.Sprintf("Subagent #%d continuing conversation in background (Turn %d).", params.ID, turn), nil
		},
	})

	// ── Tool: subagent_remove ──
	api.RegisterTool(ext.ToolDef{
		Name:        "subagent_remove",
		Description: "Remove a specific subagent. Kills it if currently running.",
		Parameters: `{
  "type": "object",
  "properties": {
    "id": {
      "type": "number",
      "description": "The ID of the subagent to remove"
    }
  },
  "required": ["id"]
}`,
		Execute: func(input string) (string, error) {
			var params struct {
				ID int `json:"id"`
			}
			if err := json.Unmarshal([]byte(input), &params); err != nil {
				return "", fmt.Errorf("invalid parameters: %w", err)
			}

			mu.Lock()
			state, ok := agents[params.ID]
			if !ok {
				mu.Unlock()
				return fmt.Sprintf("Error: No subagent #%d found.", params.ID), nil
			}
			delete(agents, params.ID)
			mu.Unlock()

			state.mu.Lock()
			state.Removed = true
			if state.Proc != nil && state.Status == "running" {
				state.Proc.Kill()
			}
			state.mu.Unlock()

			mu.Lock()
			ctx := latestCtx
			ok2 := hasCtx
			mu.Unlock()
			if ok2 {
				ctx.RemoveWidget(fmt.Sprintf("subagent:%d", params.ID))
			}

			return fmt.Sprintf("Subagent #%d removed.", params.ID), nil
		},
	})

	// ── Tool: subagent_list ──
	api.RegisterTool(ext.ToolDef{
		Name:        "subagent_list",
		Description: "List all active and finished subagents with their IDs, tasks, and status.",
		Parameters:  `{"type": "object", "properties": {}}`,
		Execute: func(input string) (string, error) {
			mu.Lock()
			agentsCopy := make([]*subState, 0, len(agents))
			for _, s := range agents {
				agentsCopy = append(agentsCopy, s)
			}
			mu.Unlock()

			if len(agentsCopy) == 0 {
				return "No active subagents.", nil
			}

			var sb strings.Builder
			sb.WriteString("Subagents:\n")
			for _, s := range agentsCopy {
				id, status, task, _, _, turnCount := s.snapshot()
				fmt.Fprintf(&sb, "#%d [%s] (Turn %d) — %s\n",
					id, strings.ToUpper(status), turnCount, task)
			}
			return sb.String(), nil
		},
	})

	// ── Tool Renderers ──
	api.RegisterToolRenderer(ext.ToolRenderConfig{
		ToolName:    "subagent_create",
		DisplayName: "Spawn Subagent",
		BorderColor: "#89b4fa",
		RenderHeader: func(toolArgs string, width int) string {
			var args struct {
				Task string `json:"task"`
			}
			if err := json.Unmarshal([]byte(toolArgs), &args); err != nil {
				return ""
			}
			return truncate(args.Task, width)
		},
		RenderBody: func(toolResult string, isError bool, width int) string {
			return truncate(toolResult, width)
		},
	})

	api.RegisterToolRenderer(ext.ToolRenderConfig{
		ToolName:    "subagent_continue",
		DisplayName: "Continue Subagent",
		BorderColor: "#cba6f7",
		RenderHeader: func(toolArgs string, width int) string {
			var args struct {
				ID     int    `json:"id"`
				Prompt string `json:"prompt"`
			}
			if err := json.Unmarshal([]byte(toolArgs), &args); err != nil {
				return ""
			}
			return truncate(fmt.Sprintf("#%d: %s", args.ID, args.Prompt), width)
		},
		RenderBody: func(toolResult string, isError bool, width int) string {
			return truncate(toolResult, width)
		},
	})

	// ── Command: /sub <task> ──
	api.RegisterCommand(ext.CommandDef{
		Name:        "sub",
		Description: "Spawn a subagent with live widget: /sub <task>",
		Execute: func(args string, ctx ext.Context) (string, error) {
			mu.Lock()
			latestCtx = ctx
			hasCtx = true
			mu.Unlock()

			task := strings.TrimSpace(args)
			if task == "" {
				return "Usage: /sub <task>", nil
			}

			mu.Lock()
			id := nextID
			nextID++
			state := &subState{
				ID:        id,
				Status:    "running",
				Task:      task,
				TurnCount: 1,
			}
			agents[id] = state
			mu.Unlock()

			updateWidgets()
			go spawnAgent(state)

			return fmt.Sprintf("Subagent #%d spawned: %s", id, truncate(task, 60)), nil
		},
	})

	// ── Command: /subcont <id> <prompt> ──
	api.RegisterCommand(ext.CommandDef{
		Name:        "subcont",
		Description: "Continue subagent conversation: /subcont <id> <prompt>",
		Execute: func(args string, ctx ext.Context) (string, error) {
			mu.Lock()
			latestCtx = ctx
			hasCtx = true
			mu.Unlock()

			trimmed := strings.TrimSpace(args)
			spaceIdx := strings.IndexByte(trimmed, ' ')
			if spaceIdx < 0 {
				return "Usage: /subcont <id> <prompt>", nil
			}

			num, err := strconv.Atoi(trimmed[:spaceIdx])
			if err != nil {
				return "Usage: /subcont <id> <prompt>", nil
			}
			prompt := strings.TrimSpace(trimmed[spaceIdx+1:])
			if prompt == "" {
				return "Usage: /subcont <id> <prompt>", nil
			}

			mu.Lock()
			state, ok := agents[num]
			mu.Unlock()
			if !ok {
				return fmt.Sprintf("No subagent #%d found. Use /sub to create one.", num), nil
			}

			state.mu.Lock()
			if state.Status == "running" {
				state.mu.Unlock()
				return fmt.Sprintf("Subagent #%d is still running — wait for it to finish.", num), nil
			}
			state.Status = "running"
			state.Task = prompt
			state.Chunks = nil
			state.Elapsed = 0
			state.TurnCount++
			turn := state.TurnCount
			state.mu.Unlock()

			updateWidgets()
			go spawnAgent(state)

			return fmt.Sprintf("Continuing subagent #%d (Turn %d): %s", num, turn, truncate(prompt, 50)), nil
		},
	})

	// ── Command: /subrm <id> ──
	api.RegisterCommand(ext.CommandDef{
		Name:        "subrm",
		Description: "Remove a subagent widget: /subrm <id>",
		Execute: func(args string, ctx ext.Context) (string, error) {
			mu.Lock()
			latestCtx = ctx
			hasCtx = true
			mu.Unlock()

			num, err := strconv.Atoi(strings.TrimSpace(args))
			if err != nil {
				return "Usage: /subrm <id>", nil
			}

			mu.Lock()
			state, ok := agents[num]
			if !ok {
				mu.Unlock()
				return fmt.Sprintf("No subagent #%d found.", num), nil
			}
			delete(agents, num)
			mu.Unlock()

			state.mu.Lock()
			state.Removed = true
			killed := false
			if state.Proc != nil && state.Status == "running" {
				state.Proc.Kill()
				killed = true
			}
			state.mu.Unlock()

			ctx.RemoveWidget(fmt.Sprintf("subagent:%d", num))

			if killed {
				return fmt.Sprintf("Subagent #%d killed and removed.", num), nil
			}
			return fmt.Sprintf("Subagent #%d removed.", num), nil
		},
	})

	// ── Command: /subclear ──
	api.RegisterCommand(ext.CommandDef{
		Name:        "subclear",
		Description: "Clear all subagent widgets",
		Execute: func(args string, ctx ext.Context) (string, error) {
			mu.Lock()
			latestCtx = ctx
			hasCtx = true
			agentsCopy := make(map[int]*subState, len(agents))
			for k, v := range agents {
				agentsCopy[k] = v
			}
			agents = map[int]*subState{}
			nextID = 1
			mu.Unlock()

			killed := 0
			total := len(agentsCopy)
			for id, state := range agentsCopy {
				state.mu.Lock()
				state.Removed = true
				if state.Proc != nil && state.Status == "running" {
					state.Proc.Kill()
					killed++
				}
				state.mu.Unlock()
				ctx.RemoveWidget(fmt.Sprintf("subagent:%d", id))
			}

			if total == 0 {
				return "No subagents to clear.", nil
			}
			msg := fmt.Sprintf("Cleared %d subagent", total)
			if total != 1 {
				msg += "s"
			}
			if killed > 0 {
				msg += fmt.Sprintf(" (%d killed)", killed)
			}
			msg += "."
			return msg, nil
		},
	})
}

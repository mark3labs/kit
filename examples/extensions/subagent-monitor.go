//go:build ignore

// subagent-monitor — live horizontal widget strip for spawned subagents
//
// Subscribes to subagents spawned by the main Kit agent and displays a
// single widget just above the input box. Each subagent occupies one column
// in a side-by-side horizontal layout. Columns show scrolling real-time
// output as the subagent works. When a subagent finishes its column is
// removed automatically.
//
// Yaegi-safe design notes:
// - No sync.Mutex (Yaegi has reflection issues with sync primitives)
// - No channels in maps (Yaegi panics on range over map[string]chan)
// - All ctx.* calls guarded with nil checks
// - Simple data structures only
package main

import (
	"fmt"
	"strings"
	"time"

	"kit/ext"
)

// ---------------------------------------------------------------------------
// Per-subagent state
// ---------------------------------------------------------------------------

type submonEntry struct {
	id      int
	callID  string
	agent   string // named agent (e.g. "explore"), or "" for the default agent
	task    string
	lines   []string
	started time.Time
	elapsed time.Duration
}

const (
	submonColWidth = 34 // visible character width per column
	submonMaxLines = 5  // scrolling output lines per column
	submonColGap   = 2  // spaces between columns
)

// ---------------------------------------------------------------------------
// Package-level state - all simple types
// ---------------------------------------------------------------------------

var (
	submonCtx     ext.Context
	submonHasCtx  bool
	submonEntries []*submonEntry
	submonNextID  int
	// submonAgentByCall records the named agent for each subagent tool call,
	// captured on OnToolCall (which fires before OnSubagentStart) and keyed
	// by ToolCallID. The subagent lifecycle events don't carry the agent
	// name, so we grab it from the raw tool-call arguments here.
	submonAgentByCall map[string]string
)

func submonInit() {
	submonEntries = nil
	submonNextID = 1
	submonAgentByCall = map[string]string{}
}

// submonAgentLabel returns the display title for a column: the named agent
// if one was requested, otherwise a sensible default.
func submonAgentLabel(agent string) string {
	if strings.TrimSpace(agent) == "" {
		return "default agent"
	}
	return agent
}

// ---------------------------------------------------------------------------
// String helpers
// ---------------------------------------------------------------------------

func submonPad(s string, w int) string {
	r := []rune(s)
	if len(r) >= w {
		return string(r[:w])
	}
	return s + strings.Repeat(" ", w-len(r))
}

func submonTrunc(s string, w int) string {
	r := []rune(s)
	if len(r) <= w {
		return s
	}
	if w <= 1 {
		return "…"
	}
	return string(r[:w-1]) + "…"
}

// ---------------------------------------------------------------------------
// Widget rendering
// ---------------------------------------------------------------------------

func submonRenderColumn(e *submonEntry) []string {
	var rows []string

	// Title row: the type/name of the agent for this pane.
	title := fmt.Sprintf("▸ %s", submonTrunc(submonAgentLabel(e.agent), submonColWidth-2))
	rows = append(rows, submonPad(title, submonColWidth))

	// Calculate elapsed time on-demand to avoid race conditions with ticker
	elapsed := e.elapsed
	if elapsed == 0 && !e.started.IsZero() {
		elapsed = time.Since(e.started)
	}
	secs := int(elapsed.Seconds())
	timeStr := fmt.Sprintf("%ds", secs)
	taskMax := submonColWidth - len(timeStr) - 3
	taskPart := submonTrunc(e.task, taskMax)
	header := fmt.Sprintf("#%d %s  %s", e.id, taskPart, timeStr)
	rows = append(rows, submonPad(header, submonColWidth))

	display := e.lines
	if len(display) > submonMaxLines {
		display = display[len(display)-submonMaxLines:]
	}
	for _, l := range display {
		rows = append(rows, submonPad("  "+submonTrunc(l, submonColWidth-2), submonColWidth))
	}
	for len(rows) < submonMaxLines+2 {
		if len(rows) == 2 && len(e.lines) == 0 {
			rows = append(rows, submonPad("  waiting…", submonColWidth))
		} else {
			rows = append(rows, strings.Repeat(" ", submonColWidth))
		}
	}
	return rows
}

func submonBuildWidget() string {
	if len(submonEntries) == 0 {
		return ""
	}

	numCols := len(submonEntries)
	numRows := submonMaxLines + 2 // +1 title row, +1 header row
	cols := make([][]string, numCols)
	for i, e := range submonEntries {
		rows := submonRenderColumn(e)
		col := make([]string, numRows)
		for j := 0; j < numRows; j++ {
			if j < len(rows) {
				col[j] = rows[j]
			} else {
				col[j] = strings.Repeat(" ", submonColWidth)
			}
		}
		cols[i] = col
	}

	gap := strings.Repeat(" ", submonColGap)
	var sb strings.Builder
	for row := 0; row < numRows; row++ {
		for ci := range cols {
			if ci > 0 {
				sb.WriteString(gap)
			}
			sb.WriteString(cols[ci][row])
		}
		if row < numRows-1 {
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

func submonPushWidget() {
	if !submonHasCtx {
		return
	}
	if submonCtx.SetWidget == nil {
		return
	}

	text := submonBuildWidget()
	if len(submonEntries) == 0 {
		if submonCtx.RemoveWidget != nil {
			submonCtx.RemoveWidget("submon")
		}
		return
	}
	submonCtx.SetWidget(ext.WidgetConfig{
		ID:        "submon",
		Placement: ext.WidgetAbove,
		Content:   ext.WidgetContent{Text: text},
		Style:     ext.WidgetStyle{BorderColor: "#89b4fa"},
		Priority:  0,
	})
}

func submonAppendLine(e *submonEntry, line string) {
	line = strings.TrimRight(line, "\r\n")
	if strings.TrimSpace(line) == "" {
		return
	}
	e.lines = append(e.lines, line)
}

// ---------------------------------------------------------------------------
// Init
// ---------------------------------------------------------------------------

func Init(api ext.API) {
	submonInit()

	api.OnSessionStart(func(_ ext.SessionStartEvent, ctx ext.Context) {
		submonCtx = ctx
		submonHasCtx = true
		submonInit()
		if ctx.RemoveWidget != nil {
			ctx.RemoveWidget("submon")
		}
	})

	api.OnAgentEnd(func(_ ext.AgentEndEvent, ctx ext.Context) {
		submonCtx = ctx
		submonHasCtx = true
	})

	// ── ToolCall: capture the named agent before the subagent starts ────────
	// SubagentStartEvent doesn't carry the agent name, so grab it here from
	// the raw tool-call args. OnToolCall fires before OnSubagentStart.
	api.OnToolCall(func(e ext.ToolCallEvent, ctx ext.Context) *ext.ToolCallResult {
		if e.ToolName == "subagent" && e.ParsedArgs != nil {
			if a, ok := e.ParsedArgs["agent"].(string); ok {
				submonAgentByCall[e.ToolCallID] = a
			}
		}
		return nil
	})

	// ── SubagentStart ────────────────────────────────────────────────────────
	api.OnSubagentStart(func(e ext.SubagentStartEvent, ctx ext.Context) {
		submonCtx = ctx
		submonHasCtx = true

		id := submonNextID
		submonNextID++
		entry := &submonEntry{
			id:      id,
			callID:  e.ToolCallID,
			agent:   submonAgentByCall[e.ToolCallID],
			task:    e.Task,
			started: time.Now(),
		}
		submonEntries = append(submonEntries, entry)

		submonPushWidget()
	})

	// ── SubagentChunk ────────────────────────────────────────────────────────
	api.OnSubagentChunk(func(e ext.SubagentChunkEvent, ctx ext.Context) {
		submonCtx = ctx
		submonHasCtx = true

		var entry *submonEntry
		for _, en := range submonEntries {
			if en.callID == e.ToolCallID {
				entry = en
				break
			}
		}
		if entry == nil {
			return
		}

		switch e.ChunkType {
		case "text":
			for _, line := range strings.Split(e.Content, "\n") {
				submonAppendLine(entry, line)
			}
		case "tool_call":
			submonAppendLine(entry, "→ "+e.ToolName)
		case "tool_execution_start":
			submonAppendLine(entry, "⚙ "+e.ToolName)
		case "tool_result":
			if e.IsError {
				submonAppendLine(entry, "✗ "+e.ToolName)
			} else {
				submonAppendLine(entry, "✓ "+e.ToolName)
			}
		}

		submonPushWidget()
	})

	// ── SubagentEnd ──────────────────────────────────────────────────────────
	api.OnSubagentEnd(func(e ext.SubagentEndEvent, ctx ext.Context) {
		submonCtx = ctx
		submonHasCtx = true

		var entry *submonEntry
		for _, en := range submonEntries {
			if en.callID == e.ToolCallID {
				entry = en
				break
			}
		}
		if entry != nil {
			entry.elapsed = time.Since(entry.started)
			if e.ErrorMsg != "" {
				submonAppendLine(entry, "✗ "+submonTrunc(e.ErrorMsg, submonColWidth-2))
			}
		}

		submonPushWidget()

		// Remove the entry immediately (no goroutine to avoid races)
		delete(submonAgentByCall, e.ToolCallID)
		newEntries := submonEntries[:0]
		for _, en := range submonEntries {
			if en.callID != e.ToolCallID {
				newEntries = append(newEntries, en)
			}
		}
		submonEntries = newEntries
		submonPushWidget()
	})

	// ── SessionShutdown ──────────────────────────────────────────────────────
	api.OnSessionShutdown(func(_ ext.SessionShutdownEvent, ctx ext.Context) {
		submonInit()
		// Guard ctx access - may be nil during shutdown
		if ctx.RemoveWidget != nil {
			ctx.RemoveWidget("submon")
		}
	})
}

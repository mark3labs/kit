package ui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	kit "github.com/mark3labs/kit/pkg/kit"
)

type subagentInspectorController interface {
	SubagentRuns() []kit.SubagentRun
	GetSubagentRun(string) (kit.SubagentRun, bool)
	KillSubagent(string) bool
}

type childScrollPosition struct {
	index, line int
	bottom      bool
}

type subagentInspector struct {
	parentSessionID      string
	runs                 []kit.SubagentRun
	index, width, height int
	childList            *ScrollList
	viewedRunID          string
	renderer             Renderer
	scrollByRun          map[string]childScrollPosition
}
type subagentInspectorTick struct{ generation uint64 }

func (m *AppModel) openSubagentInspector() tea.Cmd {
	c, ok := m.appCtrl.(subagentInspectorController)
	if !ok {
		return nil
	}
	runs := c.SubagentRuns()
	if len(runs) == 0 {
		m.printSystemMessage("No retained subagent runs.")
		return nil
	}
	childWidth := max(1, m.width)
	m.subagentInspectorGeneration++
	parentID := runs[len(runs)-1].ParentSessionID
	m.subagentView = &subagentInspector{parentSessionID: parentID, runs: runs, index: len(runs) - 1, width: childWidth, height: m.height, renderer: newMessageRenderer(childWidth, false), scrollByRun: make(map[string]childScrollPosition)}
	m.refreshSubagentView()
	return subagentInspectorTickCmd(m.subagentInspectorGeneration)
}
func subagentInspectorTickCmd(generation uint64) tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg { return subagentInspectorTick{generation: generation} })
}

func (m *AppModel) refreshSubagentView() {
	s := m.subagentView
	c, ok := m.appCtrl.(subagentInspectorController)
	if s == nil || !ok {
		return
	}
	oldID := ""
	if s.index >= 0 && s.index < len(s.runs) {
		oldID = s.runs[s.index].ID
	}
	fresh := c.SubagentRuns()
	filtered := make([]kit.SubagentRun, 0, len(fresh))
	for _, run := range fresh {
		if run.ParentSessionID == s.parentSessionID {
			filtered = append(filtered, run)
		}
	}
	fresh = filtered
	if len(fresh) == 0 {
		s.runs = nil
		s.index = 0
		s.childList = nil
		s.viewedRunID = ""
		return
	}
	s.runs = fresh
	s.index = 0
	for i, r := range fresh {
		if r.ID == oldID {
			s.index = i
			break
		}
	}
	if oldID == "" {
		s.index = len(fresh) - 1
	}
	r := fresh[s.index]
	if latest, found := c.GetSubagentRun(r.ID); found {
		r = latest
	}
	if s.childList != nil {
		id := s.viewedRunID
		if id != "" {
			s.scrollByRun[id] = childScrollPosition{s.childList.offsetIdx, s.childList.offsetLine, s.childList.AtBottom()}
		}
	}
	s.renderer.SetWidth(s.width)
	items := childRunItems(r, s.renderer)
	list := NewScrollList(s.width, max(1, s.height-4))
	list.SetItems(items)
	pos, saved := s.scrollByRun[r.ID]
	if saved && !pos.bottom {
		list.offsetIdx, list.offsetLine = pos.index, pos.line
		list.clampOffset()
	}
	s.childList = list
	s.viewedRunID = r.ID
}

func childRunItems(r kit.SubagentRun, renderer Renderer) []MessageItem {
	if renderer == nil {
		renderer = newMessageRenderer(80, false)
	}
	var items []MessageItem
	header := fmt.Sprintf("Subagent %s · %s · %s\nPrompt: %s", r.Agent, r.Status, r.Model, r.Prompt)
	if r.DroppedEvents > 0 {
		header += fmt.Sprintf("\nWARNING: Earlier events were truncated (%d dropped).", r.DroppedEvents)
	}
	items = append(items, NewThemedMessageItem("child-header-"+r.ID, "system", header, func() string { return renderer.RenderSystemMessage(header, time.Now()).Content }))
	var text strings.Builder
	msgNum, toolNum := 0, 0
	flush := func(live bool) {
		if text.Len() == 0 {
			return
		}
		v := text.String()
		text.Reset()
		msgNum++
		if live {
			item := NewStreamingMessageItem(fmt.Sprintf("child-%s-message-%d", r.ID, msgNum), "assistant", r.Model)
			item.AppendChunk(v)
			items = append(items, item)
			return
		}
		items = append(items, NewThemedMessageItem(fmt.Sprintf("child-%s-message-%d", r.ID, msgNum), "assistant", v, func() string { return renderer.RenderAssistantMessage(v, time.Now(), r.Model).Content }))
	}
	active := make(map[string]kit.ToolCallEvent)
	for _, event := range r.Events {
		switch e := event.(type) {
		case kit.MessageUpdateEvent:
			text.WriteString(e.Chunk)
		case kit.ToolCallEvent:
			flush(false)
			active[e.ToolCallID] = e
		case kit.ToolExecutionStartEvent:
			if call, ok := active[e.ToolCallID]; ok {
				call.ToolArgs = e.ToolArgs
				active[e.ToolCallID] = call
			}
		case kit.ToolResultEvent:
			flush(false)
			delete(active, e.ToolCallID)
			toolNum++
			raw := toolRawContent(e.ToolName, e.ToolArgs, e.Result, e.IsError)
			id := fmt.Sprintf("child-%s-tool-%d", r.ID, toolNum)
			items = append(items, NewThemedMessageItem(id, "tool", raw, func() string { return renderer.RenderToolMessage(e.ToolName, e.ToolArgs, e.Result, e.IsError).Content }).WithToolCall(ToolCallInfo{Name: e.ToolName, Args: e.ToolArgs, Result: e.Result, IsError: e.IsError}))
		}
	}
	flush(r.Status == "running")
	activeIDs := make([]string, 0, len(active))
	for id := range active {
		activeIDs = append(activeIDs, id)
	}
	sort.Strings(activeIDs)
	for _, id := range activeIDs {
		call := active[id]
		if call.ToolCallID == "" {
			call.ToolCallID = id
		}
		toolNum++
		raw := toolRawContent(call.ToolName, call.ToolArgs, "", false)
		items = append(items, NewThemedMessageItem(fmt.Sprintf("child-%s-active-tool-%s", r.ID, call.ToolCallID), "tool", raw, func() string { return renderer.RenderToolMessage(call.ToolName, call.ToolArgs, "", false).Content }).WithToolCall(ToolCallInfo{Name: call.ToolName, Args: call.ToolArgs}))
	}
	if r.Error != "" {
		items = append(items, NewThemedMessageItem("child-"+r.ID+"-error", "error", r.Error, func() string { return renderer.RenderErrorMessage(r.Error, time.Now()).Content }))
	}
	return items
}

func (m *AppModel) handleSubagentInspectorKey(key string) tea.Cmd {
	s := m.subagentView
	if s == nil {
		return nil
	}
	c, ok := m.appCtrl.(subagentInspectorController)
	switch key {
	case "esc":
		m.subagentView = nil
	case "up":
		if s.childList != nil {
			s.childList.ScrollBy(-1)
		}
	case "down":
		if s.childList != nil {
			s.childList.ScrollBy(1)
		}
	case "pgup":
		if s.childList != nil {
			s.childList.ScrollBy(-max(1, s.height-4))
		}
	case "pgdown":
		if s.childList != nil {
			s.childList.ScrollBy(max(1, s.height-4))
		}
	case "home":
		if s.childList != nil {
			s.childList.GotoTop()
		}
	case "end":
		if s.childList != nil {
			s.childList.GotoBottom()
		}
	case "left", "right":
		if len(s.runs) > 0 {
			d := 1
			if key == "left" {
				d = -1
			}
			s.index = (s.index + d + len(s.runs)) % len(s.runs)
			m.refreshSubagentView()
		}
	case "ctrl+k":
		if ok && len(s.runs) > 0 {
			c.KillSubagent(s.runs[s.index].ID)
			m.refreshSubagentView()
		}
	}
	return nil
}
func (m *AppModel) renderSubagentInspector() string {
	s := m.subagentView
	if s == nil {
		return ""
	}
	if len(s.runs) == 0 {
		return "No retained subagent runs."
	}
	footer := fmt.Sprintf("Child %d/%d · READ ONLY · ←/→ siblings · PgUp/PgDn scroll\nEsc parent · Ctrl+K stop", s.index+1, len(s.runs))
	footer = ansi.Hardwrap(footer, max(1, s.width), true)
	if s.childList == nil {
		return footer
	}
	return s.childList.View() + "\n" + footer
}

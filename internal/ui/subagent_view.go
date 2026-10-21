package ui

import (
	"errors"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/mark3labs/kit/internal/app"
	"github.com/mark3labs/kit/internal/ui/style"
	kit "github.com/mark3labs/kit/pkg/kit"
)

// The subagent view shows a subagent run in place of the parent agent. It is
// not a separate renderer: each run has its own conversationPane, which gets
// the run's events through the same applyPaneEvent function as the parent
// pane. Opening the view swaps the shown pane, so the scrollback, the live
// stream, the shell output and the activity row of the child are drawn by
// the same code and the same layout as the parent's. Only the composer is
// replaced, by a read-only bar with the keys of the view.

type subagentInspectorController interface {
	SubagentRuns() []kit.SubagentRun
	GetSubagentRun(string) (kit.SubagentRun, bool)
	KillSubagent(string) bool
}

// childView is the state of the subagent view while a child pane is shown.
type childView struct {
	// parentSessionID limits the sibling runs (←/→) to the runs of one
	// parent session.
	parentSessionID string
}

// paneRequest is a pane switch that waits for the end of the current
// Update. An empty runID requests the parent pane.
type paneRequest struct {
	runID           string
	parentSessionID string
}

// openSubagentInspector shows the latest subagent run.
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
	last := runs[len(runs)-1]
	m.paneRequest = &paneRequest{runID: last.ID, parentSessionID: last.ParentSessionID}
	return nil
}

// applyPaneRequest performs the pane switch that a handler requested.
func (m *AppModel) applyPaneRequest() {
	req := m.paneRequest
	m.paneRequest = nil
	if req == nil {
		return
	}
	if req.runID == "" {
		m.showParentPane()
		return
	}
	m.showChildPane(req.runID, req.parentSessionID)
}

// showPane puts p on screen in place of the current pane.
func (m *AppModel) showPane(p *conversationPane) {
	if p == m.conversationPane {
		return
	}
	if m.scrollList != nil {
		m.scrollList.SetOffscreen(true)
	}
	m.conversationPane = p
	if m.scrollList != nil && m.scrollList.SetOffscreen(false) {
		// The pane was told to follow its output while it was hidden. Scroll
		// after the next layout pass, which sets its viewport size.
		m.pendingGotoBottom = true
	}
	m.layoutDirty = true
}

// showChildPane shows the pane of the run with the given ID.
func (m *AppModel) showChildPane(runID, parentSessionID string) {
	if m.parentPane == nil {
		m.parentPane = m.conversationPane
	}
	m.showPane(m.ensureChildPane(runID))
	m.childView = &childView{parentSessionID: parentSessionID}
}

// showParentPane shows the parent agent's pane again.
func (m *AppModel) showParentPane() {
	if m.childView == nil || m.parentPane == nil {
		return
	}
	m.showPane(m.parentPane)
	m.childView = nil
}

// ensureChildPane returns the pane of a run and makes it when it does not
// exist. A new pane is filled from the retained snapshot of the run, so a
// run that started before its first live update still shows its history.
func (m *AppModel) ensureChildPane(runID string) *conversationPane {
	if p := m.childPanes[runID]; p != nil {
		return p
	}
	run := kit.SubagentRun{ID: runID}
	c, ok := m.appCtrl.(subagentInspectorController)
	if ok {
		if snapshot, found := c.GetSubagentRun(runID); found {
			run = snapshot
		}
	}
	if m.childPanes == nil {
		m.childPanes = make(map[string]*conversationPane)
	}
	if ok {
		m.pruneChildPanes(c.SubagentRuns())
	}
	p := m.newChildPane(run)
	m.childPanes[runID] = p
	m.withPane(p, func() { m.replayChildRun(run) })
	return p
}

// newChildPane makes an empty pane for a run, with the same components and
// settings as the parent pane.
func (m *AppModel) newChildPane(run kit.SubagentRun) *conversationPane {
	width := max(1, m.width)
	renderer := newMessageRenderer(width, false)
	if parent, ok := m.parentRenderer().(*MessageRenderer); ok && parent != nil {
		renderer.getToolRenderer = parent.getToolRenderer
	}
	stream := NewStreamComponent(width, run.Model)
	stream.SetThinkingVisible(m.thinkingVisible)
	list := NewScrollList(width, max(1, m.height-10))
	list.SetOffscreen(true)
	meta := run
	meta.Events = nil
	return &conversationPane{
		paneID:                run.ID,
		child:                 &childPaneState{run: meta},
		stream:                stream,
		renderer:              renderer,
		scrollList:            list,
		messages:              []MessageItem{},
		streamingBashMaxLines: 50,
	}
}

// parentRenderer returns the renderer of the parent pane, or nil.
func (m *AppModel) parentRenderer() Renderer {
	switch {
	case m.parentPane != nil:
		return m.parentPane.renderer
	case m.conversationPane != nil:
		return m.renderer
	}
	return nil
}

// pruneChildPanes drops the panes of the runs that the registry no longer
// retains. The shown pane stays.
func (m *AppModel) pruneChildPanes(retained []kit.SubagentRun) {
	keep := make(map[string]bool, len(retained))
	for _, r := range retained {
		keep[r.ID] = true
	}
	for id, p := range m.childPanes {
		if !keep[id] && p != m.conversationPane {
			delete(m.childPanes, id)
		}
	}
}

// allPanes returns the parent pane and every child pane.
func (m *AppModel) allPanes() []*conversationPane {
	panes := make([]*conversationPane, 0, 1+len(m.childPanes))
	if parent := m.parentPane; parent != nil {
		panes = append(panes, parent)
	} else if m.conversationPane != nil {
		panes = append(panes, m.conversationPane)
	}
	for _, p := range m.childPanes {
		panes = append(panes, p)
	}
	return panes
}

// replayChildRun fills the shown (new) child pane from a run snapshot: the
// task as the user message, then the retained events, then the status.
func (m *AppModel) replayChildRun(run kit.SubagentRun) {
	if strings.TrimSpace(run.Prompt) != "" {
		m.printUserMessage(run.Prompt)
	}
	if run.DroppedEvents > 0 {
		m.printSystemMessage(fmt.Sprintf("Earlier events of this subagent were not retained (%d dropped).", run.DroppedEvents))
	}
	if run.Status == "" {
		return
	}
	m.setChildStatus("running", "")
	for _, e := range run.Events {
		if ev, ok := app.ConvertSDKEvent(e); ok {
			m.applyChildEvent(ev)
		}
	}
	m.flushPendingStreamChunks()
	m.streamFlushPending = false
	m.child.seq = run.Seq
	m.setChildStatus(run.Status, run.Error)
}

// applySubagentRunEvent applies one live run update to the pane of the run.
// The pane gets the update also when it is not shown, so that it is up to
// date when the user opens it.
func (m *AppModel) applySubagentRunEvent(ev app.SubagentRunEvent) tea.Cmd {
	p := m.ensureChildPane(ev.RunID)
	var cmd tea.Cmd
	m.withPane(p, func() {
		// The snapshot that made the pane can already contain this update.
		if ev.Seq <= m.child.seq {
			return
		}
		m.child.seq = ev.Seq
		var cmds []tea.Cmd
		if ev.Event != nil {
			cmds = append(cmds, m.applyChildEvent(ev.Event))
		}
		cmds = append(cmds, m.setChildStatus(ev.Status, ev.Error))
		cmd = tea.Batch(cmds...)
	})
	return cmd
}

// flushChildPane applies the buffered stream chunks of a child pane.
func (m *AppModel) flushChildPane(runID string) {
	p := m.childPanes[runID]
	if p == nil {
		return
	}
	m.withPane(p, func() {
		m.streamFlushPending = false
		m.flushPendingStreamChunks()
	})
}

// applyChildEvent applies one converted child event to the shown child
// pane. As in the parent's Update, any event other than a stream chunk
// first applies the buffered chunks, so that it sees the full text.
func (m *AppModel) applyChildEvent(ev app.Event) tea.Cmd {
	switch ev.(type) {
	case app.StreamChunkEvent, app.ReasoningChunkEvent:
	default:
		m.flushPendingStreamChunks()
	}
	return m.applyPaneEvent(ev)
}

// setChildStatus records the status of the run of the shown child pane. A
// subagent has no step lifecycle events of its own (the parent's come from
// the app layer), so the start and the end of the run are given to the pane
// as the events that the parent pane gets for a turn.
func (m *AppModel) setChildStatus(status, errText string) tea.Cmd {
	st := m.child
	if st == nil || status == "" {
		return nil
	}
	st.run.Status, st.run.Error = status, errText
	active := status == "starting" || status == "running"
	switch {
	case active && !st.working:
		st.working = true
		m.turnStartedAt = st.run.StartedAt
		if m.turnStartedAt.IsZero() {
			m.turnStartedAt = time.Now()
		}
		m.turnToolCount = 0
		m.flushPreviousStream()
		m.layoutDirty = true
		return m.applyPaneEvent(app.SpinnerEvent{Show: true})
	case !active && st.working:
		st.working = false
		m.flushPendingStreamChunks()
		var end app.Event
		switch status {
		case "completed":
			end = app.StepCompleteEvent{}
		case "stopped":
			end = app.StepCancelledEvent{}
		default:
			text := errText
			if text == "" {
				text = "subagent " + strings.ReplaceAll(status, "_", " ")
			}
			end = app.StepErrorEvent{Err: errors.New(text)}
		}
		cmd := m.applyPaneEvent(end)
		m.turnStartedAt = time.Time{}
		m.layoutDirty = true
		return cmd
	}
	return nil
}

// updateChildView handles the messages that go to the shown child pane.
// It returns false for the messages that the parent pane handles.
func (m *AppModel) updateChildView(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		return m.handleSubagentInspectorKey(msg.String())
	case tea.MouseClickMsg:
		// The composer is not shown, so a click there does nothing.
		if msg.Y >= m.inputYOffset {
			return true, nil
		}
		_, cmd := m.update(msg)
		return true, cmd
	case tea.MouseWheelMsg, tea.MouseMotionMsg, tea.MouseReleaseMsg, frameTickMsg:
		// Scrolling, text selection and animation use the parent's
		// handlers, with the child pane shown.
		_, cmd := m.update(msg)
		return true, cmd
	case tea.PasteMsg:
		return true, nil
	}
	return false, nil
}

// handleSubagentInspectorKey handles a key while a child pane is shown.
// Ctrl+C goes to the parent handlers, so that quit works as usual.
func (m *AppModel) handleSubagentInspectorKey(key string) (bool, tea.Cmd) {
	if m.childView == nil || key == "ctrl+c" {
		return false, nil
	}
	m.flushPendingStreamChunks()
	list := m.scrollList
	switch key {
	case "esc":
		m.paneRequest = &paneRequest{}
	case "left", "right":
		d := 1
		if key == "left" {
			d = -1
		}
		m.stepChildPane(d)
	case "ctrl+k":
		if c, ok := m.appCtrl.(subagentInspectorController); ok && m.child != nil {
			c.KillSubagent(m.paneID)
		}
	case "up":
		list.ScrollBy(-1)
		list.autoScroll = false
	case "down":
		list.ScrollBy(1)
		if list.AtBottom() {
			list.autoScroll = true
		}
	case "pgup":
		list.ScrollBy(-max(1, list.height))
		list.autoScroll = false
	case "pgdown":
		list.ScrollBy(max(1, list.height))
		if list.AtBottom() {
			list.autoScroll = true
		}
	case "home", "ctrl+home":
		list.GotoTop()
		list.autoScroll = false
	case "end", "ctrl+end":
		list.GotoBottom()
		list.autoScroll = true
	}
	return true, nil
}

// childSiblings returns the retained runs of the parent session of the
// child view, oldest first.
func (m *AppModel) childSiblings() []kit.SubagentRun {
	c, ok := m.appCtrl.(subagentInspectorController)
	if !ok || m.childView == nil {
		return nil
	}
	var runs []kit.SubagentRun
	for _, r := range c.SubagentRuns() {
		if r.ParentSessionID == m.childView.parentSessionID {
			runs = append(runs, r)
		}
	}
	return runs
}

// stepChildPane requests the pane of the next (d=1) or previous (d=-1)
// sibling run.
func (m *AppModel) stepChildPane(d int) {
	runs := m.childSiblings()
	if len(runs) == 0 {
		return
	}
	i := -1
	for j, r := range runs {
		if r.ID == m.paneID {
			i = j
			break
		}
	}
	var next int
	switch {
	case i >= 0:
		next = (i + d + len(runs)) % len(runs)
	case d > 0:
		next = 0
	default:
		next = len(runs) - 1
	}
	if runs[next].ID == m.paneID {
		return
	}
	m.paneRequest = &paneRequest{runID: runs[next].ID, parentSessionID: m.childView.parentSessionID}
}

// renderChildBar is the read-only bar that replaces the composer while a
// child pane is shown. It names the run and lists the keys of the view.
func (m *AppModel) renderChildBar() string {
	theme := style.GetTheme()
	st := m.child
	// Every span carries the bar background: a background set only on the
	// outer style tears at each inner ANSI reset (see InputComponent.View).
	base := lipgloss.NewStyle().Background(theme.InputBg)
	dim := base.Foreground(theme.VeryMuted)
	sep := dim.Render(" · ")

	pos := ""
	if runs := m.childSiblings(); len(runs) > 1 {
		for i, r := range runs {
			if r.ID == m.paneID {
				pos = fmt.Sprintf(" %d/%d", i+1, len(runs))
				break
			}
		}
	}
	status := strings.ReplaceAll(st.run.Status, "_", " ")
	if status == "" {
		status = "unknown"
	}
	name := subagentLabel(kit.RunningSubagent{Agent: st.run.Agent})
	parts := []string{base.Bold(true).Render("Subagent" + pos + " · " + name), dim.Render(status)}
	if st.run.Model != "" {
		parts = append(parts, dim.Render(st.run.Model))
	}
	textWidth := max(1, m.width-2)
	line1 := truncateLine(strings.Join(parts, sep), textWidth)

	keys := "esc parent · ←/→ siblings · pgup/pgdn scroll"
	if st.working {
		keys += " · ctrl+k stop"
	}
	line2 := dim.Render(truncateLine(keys+" · read only", textWidth))

	return lipgloss.NewStyle().
		Background(theme.InputBg).
		Padding(0, 1).
		Width(max(1, m.width)).
		Render(line1 + "\n" + line2)
}

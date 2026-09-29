package ui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	kit "github.com/mark3labs/kit/pkg/kit"
)

// KillSubagentSelectedMsg is sent when the user picks a subagent to kill in
// the /kill-subagent selector.
type KillSubagentSelectedMsg struct {
	ID    string // registry ID passed to AppController.KillSubagent
	Label string // display name used in the confirmation message
}

// KillSubagentSelectorCancelledMsg is sent when the user closes the
// /kill-subagent selector without a selection.
type KillSubagentSelectorCancelledMsg struct{}

// killSubagentChoice is the Meta value of each popup row.
type killSubagentChoice struct {
	id    string
	label string
}

// KillSubagentSelectorComponent is a modal popup that lists the running
// subagents so that the user can stop one. Unlike the other selectors it
// does not own an appState: subagents only run while the agent works, so
// the parent keeps stateWorking and routes keys to this popup while it is
// open (see AppModel.killSubagentSelector).
type KillSubagentSelectorComponent struct {
	popup  *PopupList
	width  int
	height int
}

// NewKillSubagentSelector builds the selector for the given running
// subagents. now is the reference time for the elapsed column.
func NewKillSubagentSelector(running []kit.RunningSubagent, now time.Time, width, height int) *KillSubagentSelectorComponent {
	items := make([]PopupItem, len(running))
	for i, sa := range running {
		label := subagentLabel(sa)
		desc := formatSubagentElapsed(now.Sub(sa.StartedAt))
		if sa.Model != "" {
			desc += " · " + sa.Model
		}
		if p := firstLine(sa.Prompt); p != "" {
			desc += " · " + p
		}
		items[i] = PopupItem{
			Label:       label,
			Description: desc,
			Meta:        killSubagentChoice{id: sa.ID, label: label},
		}
	}

	popup := NewPopupList("Kill Subagent", items, width, height)
	popup.Subtitle = "The agent is told that you stopped the subagent"

	return &KillSubagentSelectorComponent{popup: popup, width: width, height: height}
}

// subagentLabel returns the display name of a running subagent.
func subagentLabel(sa kit.RunningSubagent) string {
	if sa.Agent != "" {
		return sa.Agent
	}
	return "subagent"
}

// firstLine returns the first non-empty line of s, trimmed.
func firstLine(s string) string {
	for line := range strings.SplitSeq(s, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			return line
		}
	}
	return ""
}

// formatSubagentElapsed formats a run time as "42s" or "3m05s".
func formatSubagentElapsed(d time.Duration) string {
	d = max(d, 0).Round(time.Second)
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	return fmt.Sprintf("%dm%02ds", int(d.Minutes()), int(d.Seconds())%60)
}

// Init implements tea.Model.
func (k *KillSubagentSelectorComponent) Init() tea.Cmd { return nil }

// Update implements tea.Model.
func (k *KillSubagentSelectorComponent) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		k.width = msg.Width
		k.height = msg.Height
		k.popup.SetSize(msg.Width, msg.Height)
		return k, nil

	case tea.KeyPressMsg:
		result := k.popup.HandleKey(msg.String(), msg.Text)
		if result.Selected != nil {
			choice, _ := result.Selected.Meta.(killSubagentChoice)
			return k, func() tea.Msg {
				return KillSubagentSelectedMsg{ID: choice.id, Label: choice.label}
			}
		}
		if result.Cancelled {
			return k, func() tea.Msg { return KillSubagentSelectorCancelledMsg{} }
		}
	}
	return k, nil
}

// View implements tea.Model. Not used for overlay rendering; see RenderOverlay.
func (k *KillSubagentSelectorComponent) View() tea.View {
	return tea.NewView(k.popup.RenderCentered(k.width, k.height))
}

// RenderOverlay returns the popup as a bare box for compositing on top of the
// conversation.
func (k *KillSubagentSelectorComponent) RenderOverlay() string {
	return k.popup.Render()
}

// --------------------------------------------------------------------------
// AppModel integration
// --------------------------------------------------------------------------

// killSubagentOpen reports whether the /kill-subagent picker is open and
// owns the keyboard. The picker only applies in the plain input and working
// states, so that a modal which opens on top of it (for example an
// extension prompt) keeps its keys.
func (m *AppModel) killSubagentOpen() bool {
	return m.killSubagentSelector != nil && (m.state == stateInput || m.state == stateWorking)
}

// handleKillSubagentCommand opens the /kill-subagent picker with the
// subagents that run now, or tells the user that none run.
func (m *AppModel) handleKillSubagentCommand() {
	if m.appCtrl == nil {
		return
	}
	running := m.appCtrl.RunningSubagents()
	if len(running) == 0 {
		m.printSystemMessage("No subagents are running.")
		return
	}
	m.killSubagentSelector = NewKillSubagentSelector(running, time.Now(), m.width, m.height)
}

// killSubagent stops the subagent the user picked and reports the result.
func (m *AppModel) killSubagent(id, label string) {
	if m.appCtrl == nil {
		return
	}
	if !m.appCtrl.KillSubagent(id) {
		m.printSystemMessage(fmt.Sprintf("Subagent %q is not running. It completed before the kill.", label))
		return
	}
	m.printSystemMessage(fmt.Sprintf("Killed subagent %q. The agent was told that you stopped it.", label))
}

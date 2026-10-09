package ui

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	kit "github.com/mark3labs/kit/pkg/kit"
)

type SubagentRunPicker struct {
	popup         *PopupList
	runs          []kit.SubagentRun
	width, height int
}

func NewSubagentRunPicker(runs []kit.SubagentRun, width, height int) *SubagentRunPicker {
	items := make([]PopupItem, len(runs))
	for i, run := range runs {
		label := strings.TrimSpace(run.Agent)
		if label == "" {
			label = "subagent"
		}
		desc := run.Status
		if run.Model != "" {
			desc += " · " + run.Model
		}
		if prompt := firstLine(run.Prompt); prompt != "" {
			desc += " · " + prompt
		}
		items[i] = PopupItem{Label: label, Description: desc, Meta: i}
	}
	p := NewPopupList("Subagent Runs", items, width, height)
	p.Subtitle = "Active and recent retained runs"
	return &SubagentRunPicker{popup: p, runs: runs, width: width, height: height}
}

func (p *SubagentRunPicker) Init() tea.Cmd { return nil }

func (p *SubagentRunPicker) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		p.width, p.height = msg.Width, msg.Height
		p.popup.SetSize(msg.Width, msg.Height)
	case tea.KeyPressMsg:
		r := p.popup.HandleKey(msg.String(), msg.Text)
		if r.Cancelled {
			return p, func() tea.Msg { return SubagentRunPickerCancelledMsg{} }
		}
		if r.Selected != nil {
			i, _ := r.Selected.Meta.(int)
			return p, func() tea.Msg { return SubagentRunSelectedMsg{Index: i} }
		}
	}
	return p, nil
}

func (p *SubagentRunPicker) View() tea.View {
	return tea.NewView(p.popup.RenderCentered(p.width, p.height))
}
func (p *SubagentRunPicker) RenderOverlay() string { return p.popup.Render() }

type SubagentRunSelectedMsg struct{ Index int }
type SubagentRunPickerCancelledMsg struct{}

func (m *AppModel) subagentRunPickerOpen() bool {
	return m.subagentRunPicker != nil && (m.state == stateInput || m.state == stateWorking)
}

func (m *AppModel) openSubagentRunPicker() {
	c, ok := m.appCtrl.(subagentInspectorController)
	if !ok {
		return
	}
	runs := c.SubagentRuns()
	if len(runs) == 0 {
		m.printSystemMessage("No retained subagent runs.")
		return
	}
	m.subagentRunPicker = NewSubagentRunPicker(runs, m.width, m.height)
}

func (m *AppModel) openSubagentRun(index int) tea.Cmd {
	picker := m.subagentRunPicker
	m.subagentRunPicker = nil
	if picker == nil || index < 0 || index >= len(picker.runs) {
		return nil
	}
	run := picker.runs[index]
	c, ok := m.appCtrl.(subagentInspectorController)
	if !ok {
		return nil
	}
	if _, exists := c.GetSubagentRun(run.ID); !exists {
		m.printSystemMessage("This subagent run is no longer retained.")
		return nil
	}
	runs := c.SubagentRuns()
	selectedIndex := 0
	for i, candidate := range runs {
		if candidate.ID == run.ID {
			selectedIndex = i
			break
		}
	}
	m.subagentInspectorGeneration++
	childWidth := max(1, m.width)
	m.subagentView = &subagentInspector{parentSessionID: run.ParentSessionID, runs: runs, index: selectedIndex, width: childWidth, height: m.height, renderer: newMessageRenderer(childWidth, false), scrollByRun: make(map[string]childScrollPosition)}
	m.refreshSubagentView()
	if m.subagentView != nil {
		for i, r := range m.subagentView.runs {
			if r.ID == run.ID {
				m.subagentView.index = i
				break
			}
		}
		m.refreshSubagentView()
	}
	return subagentInspectorTickCmd(m.subagentInspectorGeneration)
}

func (m *AppModel) updateSubagentRunPickerEvent(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case SubagentRunSelectedMsg:
		return m.openSubagentRun(msg.Index)
	case SubagentRunPickerCancelledMsg:
		m.subagentRunPicker = nil
	}
	return nil
}

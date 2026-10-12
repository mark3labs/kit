package ui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/mark3labs/kit/internal/models"
)

func modelModesFor(provider, name string) []ModelModeOption {
	info := models.LookupModelForSettings(provider + "/" + name)
	if info == nil {
		return nil
	}
	out := make([]ModelModeOption, 0, len(info.Modes))
	modelString := provider + "/" + name
	for _, mode := range info.Modes {
		reason := models.ModeUnsupportedReason(modelString, mode.Name, info)
		cost := info.EffectiveCost(mode.Name)
		out = append(out, ModelModeOption{Name: mode.Name, Supported: reason == "", Reason: reason, InputCost: cost.Input, OutputCost: cost.Output, Experimental: true})
	}
	return out
}

type ModelModeOption struct {
	Name         string
	Supported    bool
	Reason       string
	InputCost    float64
	OutputCost   float64
	Experimental bool
}

type ModelModeSelectedMsg struct{ Mode string }
type ModelModeSelectorCancelledMsg struct{}
type modelModeUpdateMsg struct {
	Mode string
	Err  error
}

type ModelModeSelectorComponent struct {
	popup         *PopupList
	width, height int
}

func NewModelModeSelector(options []ModelModeOption, current string, width, height int) *ModelModeSelectorComponent {
	items := make([]PopupItem, 0, len(options)+1)
	items = append(items, PopupItem{Label: "Default", Description: "Use the model default", Active: current == "", Meta: ""})
	for _, option := range options {
		description := option.Reason
		if option.Experimental {
			if description == "" {
				description = "Experimental"
			} else {
				description = "Experimental · " + description
			}
		}
		description += fmt.Sprintf(" · Input $%.2f/M · Output $%.2f/M", option.InputCost, option.OutputCost)
		items = append(items, PopupItem{Label: option.Name, Description: description, Disabled: !option.Supported, Active: option.Name == current, Meta: option.Name})
	}
	popup := NewPopupList("Model Mode", items, width, height)
	popup.Subtitle = "Select a catalog mode"
	popup.DisabledHint = "This mode is not supported by the active provider"
	return &ModelModeSelectorComponent{popup: popup, width: width, height: height}
}
func (s *ModelModeSelectorComponent) Init() tea.Cmd { return nil }
func (s *ModelModeSelectorComponent) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.width, s.height = msg.Width, msg.Height
		s.popup.SetSize(msg.Width, msg.Height)
	case tea.KeyPressMsg:
		result := s.popup.HandleKey(msg.String(), msg.Text)
		if result.Selected != nil {
			mode, _ := result.Selected.Meta.(string)
			return s, func() tea.Msg { return ModelModeSelectedMsg{Mode: mode} }
		}
		if result.Cancelled {
			return s, func() tea.Msg { return ModelModeSelectorCancelledMsg{} }
		}
	}
	return s, nil
}
func (s *ModelModeSelectorComponent) View() tea.View {
	v := tea.NewView(s.popup.RenderCentered(s.width, s.height))
	v.AltScreen = true
	return v
}
func (s *ModelModeSelectorComponent) RenderOverlay() string { return s.popup.Render() }

package ui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/mark3labs/kit/internal/models"
)

// ThinkingLevelSelectedMsg is sent when the user picks a level from the
// thinking-level selector.
type ThinkingLevelSelectedMsg struct {
	Level string // "off", "none", "minimal", "low", "medium", "high"
}

// ThinkingLevelSelectorCancelledMsg is sent when the user cancels the
// selector without picking a level.
type ThinkingLevelSelectorCancelledMsg struct{}

// ThinkingLevelSelectorComponent is a modal popup for choosing the reasoning
// effort level. It mirrors ModelSelectorComponent — same PopupList overlay,
// same enter/esc behaviour — so a user reaches for /thinking the way they
// reach for /model.
//
// The list is filtered to levels the active model accepts, drawn from the
// catalog's reasoning metadata (see models.SupportedThinkingLevels). This is
// what stops the picker from offering a level the provider would then reject.
type ThinkingLevelSelectorComponent struct {
	popup  *PopupList
	width  int
	height int
	active bool
}

// NewThinkingLevelSelector builds the selector for the given provider/model.
// currentLevel is the level to mark as active in the list. When the model is
// unknown or ungraded, every level Kit understands is offered.
func NewThinkingLevelSelector(provider, modelName, currentLevel string, width, height int) *ThinkingLevelSelectorComponent {
	levels := models.SupportedThinkingLevels(provider, modelName)

	items := make([]PopupItem, len(levels))
	for i, l := range levels {
		s := string(l)
		items[i] = PopupItem{
			Label:       s,
			Description: models.ThinkingLevelDescription(l),
			Active:      s == currentLevel,
			Meta:        s,
		}
	}

	popup := NewPopupList("Thinking Level", items, width, height)
	popup.Subtitle = "Only levels the current model accepts"

	return &ThinkingLevelSelectorComponent{
		popup:  popup,
		width:  width,
		height: height,
		active: true,
	}
}

// Init implements tea.Model.
func (t *ThinkingLevelSelectorComponent) Init() tea.Cmd { return nil }

// Update implements tea.Model.
func (t *ThinkingLevelSelectorComponent) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		t.width = msg.Width
		t.height = msg.Height
		t.popup.SetSize(msg.Width, msg.Height)
		return t, nil

	case tea.KeyPressMsg:
		result := t.popup.HandleKey(msg.String(), msg.Text)

		if result.Selected != nil {
			t.active = false
			level, _ := result.Selected.Meta.(string)
			return t, func() tea.Msg {
				return ThinkingLevelSelectedMsg{Level: level}
			}
		}
		if result.Cancelled {
			t.active = false
			return t, func() tea.Msg {
				return ThinkingLevelSelectorCancelledMsg{}
			}
		}
	}
	return t, nil
}

// View implements tea.Model. Not used for overlay rendering; see RenderOverlay.
func (t *ThinkingLevelSelectorComponent) View() tea.View {
	v := tea.NewView(t.popup.RenderCentered(t.width, t.height))
	v.AltScreen = true
	return v
}

// RenderOverlay returns the popup as a bare box for compositing on top of the
// conversation, matching ModelSelectorComponent's overlay behaviour.
func (t *ThinkingLevelSelectorComponent) RenderOverlay() string {
	return t.popup.Render()
}

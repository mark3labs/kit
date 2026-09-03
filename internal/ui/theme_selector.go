package ui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/mark3labs/kit/internal/ui/style"
)

// ThemeSelectedMsg is sent when the user picks a theme from the selector.
type ThemeSelectedMsg struct {
	Name string
}

// ThemeSelectorCancelledMsg is sent when the user closes the selector
// without picking a theme.
type ThemeSelectorCancelledMsg struct{}

// ThemeSelectorComponent is a modal popup for choosing the active color
// theme. It mirrors ModelSelectorComponent and ThinkingLevelSelectorComponent
// so /theme reaches the same PopupList overlay that /model and /thinking
// already use — Enter selects, Esc cancels, typing filters.
type ThemeSelectorComponent struct {
	popup  *PopupList
	width  int
	height int
	active bool
}

// NewThemeSelector builds the selector with every theme returned by
// style.ListThemes. currentTheme is marked as active in the list.
func NewThemeSelector(currentTheme string, width, height int) *ThemeSelectorComponent {
	names := style.ListThemes()
	items := make([]PopupItem, len(names))
	for i, name := range names {
		items[i] = PopupItem{
			Label:  name,
			Active: name == currentTheme,
			Meta:   name,
		}
	}

	popup := NewPopupList("Theme", items, width, height)
	popup.Subtitle = "Built-in and user themes"

	return &ThemeSelectorComponent{
		popup:  popup,
		width:  width,
		height: height,
		active: true,
	}
}

// Init implements tea.Model.
func (t *ThemeSelectorComponent) Init() tea.Cmd { return nil }

// Update implements tea.Model.
func (t *ThemeSelectorComponent) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
			name, _ := result.Selected.Meta.(string)
			return t, func() tea.Msg {
				return ThemeSelectedMsg{Name: name}
			}
		}
		if result.Cancelled {
			t.active = false
			return t, func() tea.Msg {
				return ThemeSelectorCancelledMsg{}
			}
		}
	}
	return t, nil
}

// View implements tea.Model. Not used for overlay rendering; see RenderOverlay.
func (t *ThemeSelectorComponent) View() tea.View {
	v := tea.NewView(t.popup.RenderCentered(t.width, t.height))
	v.AltScreen = true
	return v
}

// RenderOverlay returns the popup as a bare box for compositing on top of the
// conversation, matching the other selector overlays.
func (t *ThemeSelectorComponent) RenderOverlay() string {
	return t.popup.Render()
}

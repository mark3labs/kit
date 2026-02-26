package ui

import (
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// InputComponent is the interactive text input field for the parent AppModel.
// It wraps the slash command autocomplete popup and delegates slash command
// execution to the AppController. On submit it returns a submitMsg tea.Cmd
// instead of tea.Quit — lifecycle is entirely managed by the parent.
//
// Slash commands handled locally (not forwarded to app layer):
//   - /quit, /q, /exit  → tea.Quit
//   - /clear, /cls, /c  → appCtrl.ClearMessages() then clear the textarea
//
// /clear-queue is forwarded to the parent via submitMsg so the parent can
// update queueCount directly (calling ClearQueue from within Update would
// require prog.Send which deadlocks).
//
// All other input is returned via submitMsg for the parent to forward to
// app.Run().
type InputComponent struct {
	textarea    textarea.Model
	commands    []SlashCommand
	showPopup   bool
	filtered    []FuzzyMatch
	selected    int
	width       int
	height      int
	lastValue   string
	popupHeight int
	title       string
	submitNext  bool // defer submit one tick so popup dismisses cleanly

	// appCtrl is used for slash commands that mutate app state.
	// May be nil in tests; nil-safe.
	appCtrl AppController
}

// NewInputComponent creates a new InputComponent with the given width, title,
// and optional AppController. If appCtrl is nil the component still works but
// /clear and /clear-queue are no-ops.
func NewInputComponent(width int, title string, appCtrl AppController) *InputComponent {
	ta := textarea.New()
	ta.Placeholder = "Type your message..."
	ta.ShowLineNumbers = false
	ta.Prompt = ""
	ta.CharLimit = 5000
	ta.SetWidth(width - 8) // Account for container padding, border and internal padding
	ta.SetHeight(3)        // Default to 3 lines like huh
	ta.Focus()

	// Override InsertNewline so only ctrl+j and alt+enter insert newlines.
	// Enter always submits the input.
	ta.KeyMap.InsertNewline = key.NewBinding(
		key.WithKeys("ctrl+j", "alt+enter"),
		key.WithHelp("ctrl+j", "insert newline"),
	)

	// Style the textarea to match huh theme
	styles := ta.Styles()
	styles.Focused.Base = lipgloss.NewStyle()
	styles.Focused.Placeholder = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	styles.Focused.Text = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	styles.Focused.Prompt = lipgloss.NewStyle()
	styles.Focused.CursorLine = lipgloss.NewStyle()
	ta.SetStyles(styles)

	return &InputComponent{
		textarea:    ta,
		commands:    SlashCommands,
		width:       width,
		popupHeight: 7,
		title:       title,
		appCtrl:     appCtrl,
	}
}

// Init implements tea.Model. Starts the cursor blink animation.
func (s *InputComponent) Init() tea.Cmd {
	return textarea.Blink
}

// Update implements tea.Model. Handles keyboard input, popup navigation, and
// slash command execution. Returns submitMsg via a tea.Cmd when the user
// submits text — it does NOT return tea.Quit (parent owns lifecycle).
func (s *InputComponent) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	// If submitNext is set, the previous update wanted to submit but needed one
	// more frame so the popup dismisses cleanly first.
	if s.submitNext {
		s.submitNext = false
		value := s.textarea.Value()
		s.textarea.SetValue("")
		s.textarea.CursorEnd()
		s.showPopup = false
		s.lastValue = ""
		return s, s.handleSubmit(value)
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.width = msg.Width
		s.textarea.SetWidth(msg.Width - 8)
		return s, nil

	case tea.KeyPressMsg:
		if !s.showPopup {
			switch msg.String() {
			case "ctrl+d", "enter":
				value := s.textarea.Value()
				s.textarea.SetValue("")
				s.textarea.CursorEnd()
				s.lastValue = ""
				return s, s.handleSubmit(value)
			}
		}

		// Handle popup navigation
		if s.showPopup {
			switch {
			case key.Matches(msg, key.NewBinding(key.WithKeys("up"), key.WithHelp("↑", "up"))):
				if s.selected > 0 {
					s.selected--
				}
				return s, nil

			case key.Matches(msg, key.NewBinding(key.WithKeys("down"), key.WithHelp("↓", "down"))):
				if s.selected < len(s.filtered)-1 {
					s.selected++
				}
				return s, nil

			case key.Matches(msg, key.NewBinding(key.WithKeys("tab"))):
				if s.selected < len(s.filtered) {
					s.textarea.SetValue(s.filtered[s.selected].Command.Name)
					s.showPopup = false
					s.selected = 0
					s.textarea.CursorEnd()
				}
				return s, nil

			case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
				if s.selected < len(s.filtered) {
					// Populate textarea with selected command and submit on next tick.
					s.textarea.SetValue(s.filtered[s.selected].Command.Name)
					s.textarea.CursorEnd()
					s.showPopup = false
					s.selected = 0
					s.submitNext = true
					return s, nil
				}
				return s, nil

			case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
				s.showPopup = false
				s.selected = 0
				return s, nil
			}
		}

		// Pass the key to the textarea.
		s.textarea, cmd = s.textarea.Update(msg)

		// Update autocomplete popup state.
		value := s.textarea.Value()
		if value != s.lastValue {
			s.lastValue = value
			lines := strings.Split(value, "\n")
			if len(lines) == 1 && strings.HasPrefix(lines[0], "/") && !strings.Contains(lines[0], " ") {
				s.showPopup = true
				s.filtered = FuzzyMatchCommands(lines[0], s.commands)
				s.selected = 0
			} else {
				s.showPopup = false
			}
		}
		return s, cmd

	default:
		s.textarea, cmd = s.textarea.Update(msg)
		return s, cmd
	}
}

// handleSubmit processes the submitted text. Slash commands that affect app
// state are executed here; /quit returns tea.Quit; everything else returns a
// submitMsg tea.Cmd for the parent to forward to app.Run().
func (s *InputComponent) handleSubmit(value string) tea.Cmd {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}

	// Resolve via canonical command lookup so aliases are handled uniformly.
	// Only /quit and /clear are handled locally — /clear-queue must go
	// through the parent model so it can update queueCount directly
	// (calling ClearQueue here would skip the UI state update since we
	// can't send events from within Update without deadlocking).
	if sc := GetCommandByName(trimmed); sc != nil {
		switch sc.Name {
		case "/quit":
			return tea.Quit

		case "/clear":
			if s.appCtrl != nil {
				s.appCtrl.ClearMessages()
			}
			// Don't forward to app.Run(); just clear silently.
			return nil
		}
	}

	// For all other input (including unrecognised slash commands and regular
	// prompts) hand off to the parent via submitMsg.
	return func() tea.Msg {
		return submitMsg{Text: trimmed}
	}
}

// View implements tea.Model. Renders the title, textarea, autocomplete popup
// (if visible), and help text.
func (s *InputComponent) View() tea.View {
	containerStyle := lipgloss.NewStyle().PaddingLeft(2)

	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252")).
		MarginBottom(1)

	inputBoxStyle := lipgloss.NewStyle().
		Border(lipgloss.ThickBorder()).
		BorderLeft(true).
		BorderRight(false).
		BorderTop(false).
		BorderBottom(false).
		BorderForeground(lipgloss.Color("39")).
		PaddingLeft(1).
		Width(s.width - 2) // Account for container padding

	var view strings.Builder
	view.WriteString(titleStyle.Render(s.title))
	view.WriteString("\n")
	view.WriteString(inputBoxStyle.Render(s.textarea.View()))

	if s.showPopup && len(s.filtered) > 0 {
		view.WriteString("\n")
		view.WriteString(s.renderPopup())
	}

	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		MarginTop(1)

	view.WriteString("\n")
	view.WriteString(helpStyle.Render("enter submit • ctrl+j / alt+enter new line"))

	return tea.NewView(containerStyle.Render(view.String()))
}

// renderPopup renders the autocomplete popup for slash command suggestions.
func (s *InputComponent) renderPopup() string {
	popupStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("236")).
		Padding(1, 2).
		Width(s.width - 4).
		MarginLeft(0)

	var items []string

	visibleItems := min(len(s.filtered), s.popupHeight)
	startIdx := 0
	if s.selected >= s.popupHeight {
		startIdx = s.selected - s.popupHeight + 1
	}
	endIdx := min(startIdx+visibleItems, len(s.filtered))

	for i := startIdx; i < endIdx; i++ {
		match := s.filtered[i]
		sc := match.Command

		var indicator string
		if i == s.selected {
			indicator = lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Render("> ")
		} else {
			indicator = "  "
		}

		nameStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true)
		descStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("243"))
		if i == s.selected {
			nameStyle = nameStyle.Foreground(lipgloss.Color("87"))
			descStyle = descStyle.Foreground(lipgloss.Color("250"))
		}

		nameWidth := 15
		name := nameStyle.Width(nameWidth - 2).Render(sc.Name)

		desc := sc.Description
		maxDescLen := s.width - nameWidth - 14
		if len(desc) > maxDescLen && maxDescLen > 3 {
			desc = desc[:maxDescLen-3] + "..."
		}

		items = append(items, indicator+name+descStyle.Render(desc))
	}

	if startIdx > 0 {
		items = append([]string{lipgloss.NewStyle().Foreground(lipgloss.Color("238")).Render("  ↑ more above")}, items...)
	}
	if endIdx < len(s.filtered) {
		items = append(items, lipgloss.NewStyle().Foreground(lipgloss.Color("238")).Render("  ↓ more below"))
	}

	content := strings.Join(items, "\n")
	footer := lipgloss.NewStyle().Foreground(lipgloss.Color("238")).Italic(true).
		Render("↑↓ navigate • tab complete • ↵ select • esc dismiss")

	return popupStyle.Render(content + "\n\n" + footer)
}

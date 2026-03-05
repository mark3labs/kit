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
	lastValue   string
	popupHeight int
	title       string
	submitNext  bool // defer submit one tick so popup dismisses cleanly

	// Argument completion state. When the user types "/cmd " followed by
	// a partial argument and the command has a Complete function, the popup
	// switches to argument-completion mode showing suggestions from Complete.
	argMode      bool           // true when showing arg completions
	argCommand   string         // command prefix for arg mode (e.g. "/bookmark")
	argSynthCmds []SlashCommand // backing storage for synthetic arg entries

	// File completion state. When the user types @ followed by a partial
	// file path, the popup shows file/directory suggestions from the cwd.
	fileMode        bool             // true when showing @file completions
	filePrefix      string           // current text after @ being matched
	fileAtStartIdx  int              // byte offset of @ in the textarea value
	fileSuggestions []FileSuggestion // backing storage for file entries
	fileSynthCmds   []SlashCommand   // synthetic SlashCommands wrapping file entries

	// cwd is the working directory used for @file path resolution and
	// autocomplete suggestions. Set by the parent via SetCwd.
	cwd string

	// appCtrl is used for slash commands that mutate app state.
	// May be nil in tests; nil-safe.
	appCtrl AppController

	// hideHint suppresses the "enter submit · ctrl+j..." hint text.
	hideHint bool
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

// SetCwd sets the working directory used for @file autocomplete suggestions
// and path resolution. Should be called by the parent after construction.
func (s *InputComponent) SetCwd(cwd string) {
	s.cwd = cwd
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
					if s.fileMode {
						s.applyFileCompletion(s.selected)
					} else if s.argMode {
						s.textarea.SetValue(s.argCommand + " " + s.filtered[s.selected].Command.Name)
						s.showPopup = false
						s.selected = 0
					} else {
						s.textarea.SetValue(s.filtered[s.selected].Command.Name)
						s.showPopup = false
						s.selected = 0
					}
					s.textarea.CursorEnd()
				}
				return s, nil

			case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
				if s.selected < len(s.filtered) {
					if s.fileMode {
						// Apply file completion but don't submit.
						s.applyFileCompletion(s.selected)
						s.textarea.CursorEnd()
						return s, nil
					}
					// Populate textarea with selected item and submit on next tick.
					if s.argMode {
						s.textarea.SetValue(s.argCommand + " " + s.filtered[s.selected].Command.Name)
					} else {
						s.textarea.SetValue(s.filtered[s.selected].Command.Name)
					}
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
			line := lines[len(lines)-1] // current line (last line for multi-line)

			// Check for @file trigger first.
			cursorCol := len(line) // approximate: cursor is at end after typing
			if hasAt, prefix, atIdx := ExtractAtPrefix(line, cursorCol); hasAt && s.cwd != "" {
				suggestions := GetFileSuggestions(prefix, s.cwd)
				if len(suggestions) > 0 {
					s.showPopup = true
					s.fileMode = true
					s.argMode = false
					s.filePrefix = prefix
					s.fileAtStartIdx = atIdx
					s.fileSuggestions = suggestions
					s.fileSynthCmds = make([]SlashCommand, len(suggestions))
					s.filtered = make([]FuzzyMatch, len(suggestions))
					for i, fs := range suggestions {
						name := fs.RelPath
						desc := ""
						if fs.IsDir {
							desc = "directory"
						}
						s.fileSynthCmds[i] = SlashCommand{Name: name, Description: desc}
						s.filtered[i] = FuzzyMatch{Command: &s.fileSynthCmds[i], Score: fs.Score}
					}
					s.selected = 0
				} else {
					s.showPopup = false
					s.fileMode = false
				}
			} else if len(lines) == 1 && strings.HasPrefix(lines[0], "/") {
				s.fileMode = false
				if !strings.Contains(lines[0], " ") {
					// Command name completion.
					s.showPopup = true
					s.argMode = false
					s.filtered = FuzzyMatchCommands(lines[0], s.commands)
					s.selected = 0
				} else if suggestions := s.completeArgs(lines[0]); len(suggestions) > 0 {
					// Argument completion for a command with a Complete function.
					s.showPopup = true
					// s.argMode, s.argCommand, s.argSynthCmds, s.filtered
					// are set by completeArgs.
					s.selected = 0
				} else {
					s.showPopup = false
					s.argMode = false
				}
			} else {
				s.showPopup = false
				s.argMode = false
				s.fileMode = false
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
	containerStyle := lipgloss.NewStyle()

	// PaddingLeft(3) aligns with message content: border(1) + paddingLeft(2).
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252")).
		MarginBottom(1).
		PaddingLeft(3)

	inputBoxStyle := lipgloss.NewStyle().
		Border(lipgloss.ThickBorder()).
		BorderLeft(true).
		BorderRight(false).
		BorderTop(false).
		BorderBottom(false).
		BorderForeground(lipgloss.Color("39")).
		PaddingLeft(2).    // match message block paddingLeft
		Width(s.width - 1) // full width minus left border

	var view strings.Builder
	view.WriteString(titleStyle.Render(s.title))
	view.WriteString("\n")
	view.WriteString(inputBoxStyle.Render(s.textarea.View()))

	if s.showPopup && len(s.filtered) > 0 {
		view.WriteString("\n")
		view.WriteString(s.renderPopup())
	}

	if !s.hideHint {
		helpStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			MarginTop(1).
			PaddingLeft(3)

		view.WriteString("\n")
		view.WriteString(helpStyle.Render("enter submit • ctrl+j / alt+enter new line"))
	}

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

		if s.fileMode {
			// File mode: use full width for the path, show description
			// (e.g. "directory") inline after a gap.
			maxNameLen := s.width - 24
			displayName := sc.Name
			if len(displayName) > maxNameLen && maxNameLen > 3 {
				displayName = displayName[:maxNameLen-3] + "..."
			}
			name := nameStyle.Render(displayName)
			if sc.Description != "" {
				items = append(items, indicator+name+"  "+descStyle.Render(sc.Description))
			} else {
				items = append(items, indicator+name)
			}
		} else {
			nameWidth := 15
			name := nameStyle.Width(nameWidth - 2).Render(sc.Name)

			desc := sc.Description
			maxDescLen := s.width - nameWidth - 14
			if len(desc) > maxDescLen && maxDescLen > 3 {
				desc = desc[:maxDescLen-3] + "..."
			}

			items = append(items, indicator+name+descStyle.Render(desc))
		}
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

// completeArgs checks whether the input line matches a command with a Complete
// function, calls it, and populates the arg-mode state on success. Returns the
// list of suggestions (empty means no completions available).
func (s *InputComponent) completeArgs(line string) []FuzzyMatch {
	parts := strings.SplitN(line, " ", 2)
	cmdName := parts[0]
	argPrefix := ""
	if len(parts) > 1 {
		argPrefix = parts[1]
	}

	cmd := s.findCommandWithComplete(cmdName)
	if cmd == nil {
		return nil
	}

	suggestions := cmd.Complete(argPrefix)
	if len(suggestions) == 0 {
		s.argMode = false
		return nil
	}

	s.argMode = true
	s.argCommand = cmdName
	s.argSynthCmds = make([]SlashCommand, len(suggestions))
	s.filtered = make([]FuzzyMatch, len(suggestions))
	for i, sug := range suggestions {
		s.argSynthCmds[i] = SlashCommand{Name: sug}
		s.filtered[i] = FuzzyMatch{Command: &s.argSynthCmds[i]}
	}
	return s.filtered
}

// findCommandWithComplete looks up a command by name that has a non-nil
// Complete function.
func (s *InputComponent) findCommandWithComplete(name string) *SlashCommand {
	for i := range s.commands {
		if s.commands[i].Name == name && s.commands[i].Complete != nil {
			return &s.commands[i]
		}
	}
	return nil
}

// applyFileCompletion replaces the @prefix in the textarea with the selected
// file suggestion. For directories, it keeps the popup open for further
// drilling. For files, it closes the popup and adds a trailing space.
func (s *InputComponent) applyFileCompletion(idx int) {
	if idx >= len(s.fileSuggestions) {
		return
	}

	suggestion := s.fileSuggestions[idx]
	value := s.textarea.Value()

	// Build the replacement text. The @ and everything after it up to the
	// cursor should be replaced with @<selected path>.
	// Find the current line's contribution.
	lines := strings.Split(value, "\n")
	lastLine := lines[len(lines)-1]

	// Reconstruct: everything before the @ on the last line + @<path>
	beforeAt := lastLine[:s.fileAtStartIdx]
	needsQuote := strings.Contains(suggestion.RelPath, " ")

	var replacement string
	if needsQuote {
		replacement = `@"` + suggestion.RelPath + `"`
	} else {
		replacement = "@" + suggestion.RelPath
	}

	// For files, add a trailing space. For directories, don't — allow
	// continued drilling into the directory.
	if !suggestion.IsDir {
		replacement += " "
	}

	newLastLine := beforeAt + replacement

	// Reconstruct the full value with the updated last line.
	lines[len(lines)-1] = newLastLine
	newValue := strings.Join(lines, "\n")

	s.textarea.SetValue(newValue)
	s.textarea.CursorEnd()

	if suggestion.IsDir {
		// Keep popup open — trigger a refresh for the new directory.
		s.lastValue = "" // force re-evaluation on next update tick
	} else {
		s.showPopup = false
		s.fileMode = false
		s.selected = 0
	}
}

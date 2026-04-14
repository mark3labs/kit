package ui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/mark3labs/kit/internal/clipboard"
	"github.com/mark3labs/kit/internal/ui/commands"
	"github.com/mark3labs/kit/internal/ui/core"
	"github.com/mark3labs/kit/internal/ui/style"
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
	commands    []commands.SlashCommand
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
	argMode      bool                    // true when showing arg completions
	argCommand   string                  // command prefix for arg mode (e.g. "/bookmark")
	argSynthCmds []commands.SlashCommand // backing storage for synthetic arg entries

	// File completion state. When the user types @ followed by a partial
	// file path, the popup shows file/directory suggestions from the cwd.
	fileMode        bool                    // true when showing @file completions
	filePrefix      string                  // current text after @ being matched
	fileAtStartIdx  int                     // byte offset of @ in the textarea value
	fileSuggestions []FileSuggestion        // backing storage for file entries
	fileSynthCmds   []commands.SlashCommand // synthetic commands.SlashCommands wrapping file entries

	// cwd is the working directory used for @file path resolution and
	// autocomplete suggestions. Set by the parent via SetCwd.
	cwd string

	// appCtrl is used for slash commands that mutate app state.
	// May be nil in tests; nil-safe.
	appCtrl AppController

	// hideHint suppresses the "enter submit · ctrl+j..." hint text.
	hideHint bool

	// agentBusy indicates the agent is currently working. When true, the
	// hint text shows steering shortcut (Ctrl+X s) instead of submit.
	agentBusy bool

	// pendingImages holds clipboard images attached to the next submission.
	// Images are added via Ctrl+V and cleared on submit or Ctrl+U.
	pendingImages []core.ImageAttachment

	// history stores previously submitted prompts (most recent last).
	// Limited to maxHistory entries; duplicates of the previous entry are
	// skipped. Empty strings are never stored.
	history []string
	// historyIndex is the current position when browsing history.
	// When not browsing, historyIndex == len(history).
	historyIndex int
	// savedInput holds the user's in-progress text before they started
	// browsing history, so it can be restored when they press down past
	// the end of history.
	savedInput string
	// browsingHistory is true when the user is navigating history with
	// up/down arrows. Set to false when they type a character or submit.
	browsingHistory bool
}

// maxHistory is the maximum number of prompt entries kept in history.
const maxHistory = 100

// clipboardImageMsg is the result of an async clipboard image read.
type clipboardImageMsg struct {
	image *core.ImageAttachment
	err   error
}

// NewInputComponent creates a new InputComponent with the given width, title,
// and optional AppController. If appCtrl is nil the component still works but
// /clear and /clear-queue are no-ops.
func NewInputComponent(width int, title string, appCtrl AppController) *InputComponent {
	ta := textarea.New()
	ta.Placeholder = "Type your message..."
	ta.ShowLineNumbers = false
	ta.Prompt = ""
	ta.CharLimit = 0
	ta.SetWidth(width - 8) // Account for container padding, border and internal padding
	ta.SetHeight(3)        // Default to 3 lines like huh
	ta.Focus()

	// Override InsertNewline so only ctrl+j and shift+enter insert newlines.
	// Enter always submits the input.
	ta.KeyMap.InsertNewline = key.NewBinding(
		key.WithKeys("ctrl+j", "shift+enter"),
		key.WithHelp("ctrl+j", "insert newline"),
	)

	// Style the textarea using theme colors.
	theme := style.GetTheme()
	styles := ta.Styles()
	styles.Focused.Base = lipgloss.NewStyle()
	styles.Focused.Placeholder = lipgloss.NewStyle().Foreground(theme.VeryMuted)
	styles.Focused.Text = lipgloss.NewStyle().Foreground(theme.Text)
	styles.Focused.Prompt = lipgloss.NewStyle()
	styles.Focused.CursorLine = lipgloss.NewStyle()
	ta.SetStyles(styles)

	return &InputComponent{
		textarea:    ta,
		commands:    commands.SlashCommands,
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
		s.pushHistory(value)
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

	case clipboardImageMsg:
		if msg.err != nil {
			// Silently ignore — no image on clipboard or tool unavailable.
			return s, nil
		}
		if msg.image != nil {
			s.pendingImages = append(s.pendingImages, *msg.image)
		}
		return s, nil

	case tea.KeyPressMsg:
		if !s.showPopup {
			switch msg.String() {
			case "ctrl+d", "enter":
				value := s.textarea.Value()
				s.pushHistory(value)
				s.textarea.SetValue("")
				s.textarea.CursorEnd()
				s.lastValue = ""
				return s, s.handleSubmit(value)
			case "up":
				// Navigate prompt history backward (older entries).
				if len(s.history) > 0 {
					if !s.browsingHistory {
						// Start browsing — save current input.
						s.savedInput = s.textarea.Value()
						s.browsingHistory = true
						s.historyIndex = len(s.history)
					}
					if s.historyIndex > 0 {
						s.historyIndex--
						s.textarea.SetValue(s.history[s.historyIndex])
						s.textarea.CursorEnd()
						s.lastValue = s.textarea.Value()
					}
					return s, nil
				}
			case "down":
				// Navigate prompt history forward (newer entries).
				if s.browsingHistory {
					if s.historyIndex < len(s.history)-1 {
						s.historyIndex++
						s.textarea.SetValue(s.history[s.historyIndex])
						s.textarea.CursorEnd()
						s.lastValue = s.textarea.Value()
					} else {
						// Past the end — restore saved input.
						s.historyIndex = len(s.history)
						s.browsingHistory = false
						s.textarea.SetValue(s.savedInput)
						s.textarea.CursorEnd()
						s.lastValue = s.textarea.Value()
						s.savedInput = ""
					}
					return s, nil
				}
			case "ctrl+v":
				// Try to read an image from the clipboard asynchronously.
				return s, readClipboardImageCmd()
			case "ctrl+u":
				// Clear all pending image attachments.
				if len(s.pendingImages) > 0 {
					s.pendingImages = nil
					return s, nil
				}
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
			// User typed something — exit history browsing mode.
			if s.browsingHistory {
				s.browsingHistory = false
				s.savedInput = ""
			}
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
					s.fileSynthCmds = make([]commands.SlashCommand, len(suggestions))
					s.filtered = make([]FuzzyMatch, len(suggestions))
					for i, fs := range suggestions {
						name := fs.RelPath
						desc := ""
						if fs.IsDir {
							desc = "directory"
						}
						s.fileSynthCmds[i] = commands.SlashCommand{Name: name, Description: desc}
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
//
// Shell command prefixes (matching pi's behavior):
//   - !cmd  → execute shell command, output INCLUDED in LLM context
//   - !!cmd → execute shell command, output EXCLUDED from LLM context
func (s *InputComponent) handleSubmit(value string) tea.Cmd {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}

	// Check for shell command prefixes before slash commands. Test !! first
	// (more specific) to avoid matching the single-! case for double-bang.
	if strings.HasPrefix(trimmed, "!!") {
		cmd := strings.TrimSpace(trimmed[2:])
		if cmd != "" {
			return func() tea.Msg {
				return core.ShellCommandMsg{Command: cmd, ExcludeFromContext: true}
			}
		}
	} else if strings.HasPrefix(trimmed, "!") {
		cmd := strings.TrimSpace(trimmed[1:])
		if cmd != "" {
			return func() tea.Msg {
				return core.ShellCommandMsg{Command: cmd, ExcludeFromContext: false}
			}
		}
	}

	// Resolve via canonical command lookup so aliases are handled uniformly.
	// Only /quit is handled locally — all other slash commands (including
	// /clear and /clear-queue) are forwarded to the parent model via
	// submitMsg so the parent can update its own state (ScrollList, queue
	// counts, etc.) in one place.
	if sc := commands.GetCommandByName(trimmed); sc != nil {
		switch sc.Name {
		case "/quit":
			return tea.Quit
		}
	}

	// For all other input (including unrecognised slash commands and regular
	// prompts) hand off to the parent via submitMsg. Attach any pending
	// images and clear them.
	images := s.pendingImages
	s.pendingImages = nil
	return func() tea.Msg {
		return core.SubmitMsg{Text: trimmed, Images: images}
	}
}

// pushHistory adds a prompt to the history ring buffer. Empty strings and
// consecutive duplicates of the last entry are skipped. When the buffer
// exceeds maxHistory, the oldest entry is dropped.
func (s *InputComponent) pushHistory(value string) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return
	}
	// Skip consecutive duplicates.
	if len(s.history) > 0 && s.history[len(s.history)-1] == trimmed {
		s.resetHistoryBrowsing()
		return
	}
	s.history = append(s.history, trimmed)
	if len(s.history) > maxHistory {
		s.history = s.history[len(s.history)-maxHistory:]
	}
	s.resetHistoryBrowsing()
}

// resetHistoryBrowsing resets the history browsing state so the index
// points past the end (ready for new input).
func (s *InputComponent) resetHistoryBrowsing() {
	s.historyIndex = len(s.history)
	s.browsingHistory = false
	s.savedInput = ""
}

// View implements tea.Model. Renders the title, textarea, autocomplete popup
// (if visible), and help text.
func (s *InputComponent) View() tea.View {
	containerStyle := lipgloss.NewStyle()

	theme := style.GetTheme()

	// PaddingLeft(3) aligns with message content: border(1) + paddingLeft(2).
	titleStyle := lipgloss.NewStyle().
		Foreground(theme.Text).
		MarginBottom(1).
		PaddingLeft(3)

	inputBoxStyle := lipgloss.NewStyle().
		Border(lipgloss.ThickBorder()).
		BorderLeft(true).
		BorderRight(false).
		BorderTop(false).
		BorderBottom(false).
		BorderForeground(theme.Primary).
		PaddingLeft(2).    // match message block paddingLeft
		Width(s.width - 1) // full width minus left border

	var view strings.Builder
	view.WriteString(titleStyle.Render(s.title))
	view.WriteString("\n")
	view.WriteString(inputBoxStyle.Render(s.textarea.View()))

	// Popup is now rendered as a centered overlay in AppModel.View()
	// instead of inline here to prevent bottom overflow

	// Show image attachment indicator when images are pending.
	if len(s.pendingImages) > 0 {
		imgStyle := lipgloss.NewStyle().
			Foreground(theme.Secondary).
			PaddingLeft(3)

		label := fmt.Sprintf("[%d image(s) attached] ctrl+u to clear", len(s.pendingImages))
		view.WriteString("\n")
		view.WriteString(imgStyle.Render(label))
	}

	if !s.hideHint {
		helpStyle := lipgloss.NewStyle().
			Foreground(theme.VeryMuted).
			MarginTop(1).
			PaddingLeft(3)

		// Adapt hint text to available width (accounting for left padding of 3).
		var hint string
		availableHintWidth := s.width - 3
		if s.agentBusy {
			// When the agent is working, show steering shortcut.
			if availableHintWidth >= 60 {
				hint = "enter queue • ctrl+x s steer • esc esc cancel"
			} else if availableHintWidth >= 40 {
				hint = "↵ queue • ^X s steer • esc×2 cancel"
			} else {
				hint = "^X s steer"
			}
		} else if availableHintWidth >= 80 {
			hint = "enter submit • ctrl+j / shift+enter new line • ctrl+x e editor • ctrl+v paste image"
		} else if availableHintWidth >= 67 {
			hint = "enter submit • ctrl+j new line • ctrl+x e editor • ctrl+v image"
		} else if availableHintWidth >= 40 {
			hint = "↵ submit • ctrl+j newline • ^X e editor"
		} else if availableHintWidth >= 20 {
			hint = "↵ submit • ^X e editor"
		} else {
			hint = "↵ submit"
		}
		view.WriteString("\n")
		view.WriteString(helpStyle.Render(hint))
	}

	return tea.NewView(containerStyle.Render(view.String()))
}

// renderPopup renders the autocomplete popup for slash command suggestions.
// When rendered inline (not centered), returns the styled popup content.
// RenderPopupCentered renders the popup as a centered overlay.
func (s *InputComponent) RenderPopupCentered(termWidth, termHeight int) string {
	if !s.showPopup || len(s.filtered) == 0 {
		return ""
	}

	popupContent := s.renderPopupWithOptions(true)

	// Center popup using lipgloss.Place
	positioned := lipgloss.Place(
		termWidth,
		termHeight,
		lipgloss.Center,
		lipgloss.Center,
		popupContent,
	)

	return positioned
}

// renderPopupWithOptions renders the popup content with optional center styling.
func (s *InputComponent) renderPopupWithOptions(centered bool) string {
	theme := style.GetTheme()
	popupWidth := max(s.width-4, 20)

	// Use the theme background for the popup - the full-width item backgrounds
	// and primary-colored selection will provide sufficient contrast
	popupBg := theme.Background

	popupStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.Primary).
		Background(popupBg).
		Padding(1, 2).
		Width(popupWidth).
		MarginLeft(0).
		MarginBottom(1) // Visual depth/shadow effect

	// Inner content width: popup minus border (2) and horizontal padding (4).
	innerWidth := max(popupWidth-6, 10)

	// Item background styles for high contrast
	normalItemBg := lipgloss.NewStyle().
		Background(popupBg).
		Foreground(theme.Text).
		Width(innerWidth).
		Padding(0, 1)

	selectedItemBg := lipgloss.NewStyle().
		Background(theme.Primary).
		Foreground(theme.Background).
		Width(innerWidth).
		Padding(0, 1).
		Bold(true)

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

		// Choose the appropriate background style
		itemStyle := normalItemBg
		if i == s.selected {
			itemStyle = selectedItemBg
		}

		// Build indicator with proper coloring
		var indicator string
		if i == s.selected {
			indicator = "> "
		} else {
			indicator = "  "
		}

		// Build content with name and description
		var content string
		if s.fileMode {
			// File mode: use full width for the path, show description inline
			maxNameLen := max(innerWidth-16, 8)
			displayName := sc.Name
			if len(displayName) > maxNameLen && maxNameLen > 3 {
				displayName = displayName[:maxNameLen-3] + "..."
			}

			if sc.Description != "" && innerWidth > 30 {
				content = indicator + displayName + "  " + sc.Description
			} else {
				content = indicator + displayName
			}
		} else {
			// Line layout: indicator(2) + name(nameWidth-2 visual) + desc
			if innerWidth < 20 {
				// Very narrow: show truncated name only
				displayName := sc.Name
				maxName := max(innerWidth-2, 3)
				if len(displayName) > maxName {
					displayName = displayName[:maxName-1] + "…"
				}
				content = indicator + displayName
			} else {
				nameWidth := 15
				if innerWidth < 25 {
					nameWidth = max(innerWidth*2/5+1, 8)
				}
				maxNameChars := nameWidth - 2
				displayName := sc.Name
				if len(displayName) > maxNameChars {
					displayName = displayName[:maxNameChars-1] + "…"
				}

				// Description gets remaining space
				maxDescLen := max(innerWidth-nameWidth, 0)
				desc := sc.Description
				if maxDescLen >= 4 && desc != "" {
					if len(desc) > maxDescLen {
						desc = desc[:maxDescLen-3] + "..."
					}
					content = indicator + lipgloss.NewStyle().Width(maxNameChars).Render(displayName) + desc
				} else {
					content = indicator + displayName
				}
			}
		}

		items = append(items, itemStyle.Render(content))
	}

	// Add scroll indicators with background
	scrollStyle := lipgloss.NewStyle().
		Background(popupBg).
		Foreground(theme.VeryMuted).
		Width(innerWidth).
		Padding(0, 1)

	if startIdx > 0 {
		items = append([]string{scrollStyle.Render("  ↑ more above")}, items...)
	}
	if endIdx < len(s.filtered) {
		items = append(items, scrollStyle.Render("  ↓ more below"))
	}

	content := strings.Join(items, "\n")

	// Adapt footer text to available width with background
	var footerText string
	if innerWidth >= 50 {
		footerText = "↑↓ navigate • tab complete • ↵ select • esc dismiss"
	} else if innerWidth >= 30 {
		footerText = "↑↓ nav • tab • ↵ select • esc"
	} else {
		footerText = "↑↓ tab ↵ esc"
	}
	footer := lipgloss.NewStyle().
		Background(popupBg).
		Foreground(theme.VeryMuted).
		Italic(true).
		Render(footerText)

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
	s.argSynthCmds = make([]commands.SlashCommand, len(suggestions))
	s.filtered = make([]FuzzyMatch, len(suggestions))
	for i, sug := range suggestions {
		s.argSynthCmds[i] = commands.SlashCommand{Name: sug}
		s.filtered[i] = FuzzyMatch{Command: &s.argSynthCmds[i]}
	}
	return s.filtered
}

// findCommandWithComplete looks up a command by name that has a non-nil
// Complete function.
func (s *InputComponent) findCommandWithComplete(name string) *commands.SlashCommand {
	for i := range s.commands {
		if s.commands[i].Name == name && s.commands[i].Complete != nil {
			return &s.commands[i]
		}
	}
	return nil
}

// readClipboardImageCmd returns a tea.Cmd that reads an image from the system
// clipboard. The result is delivered as a clipboardImageMsg.
func readClipboardImageCmd() tea.Cmd {
	return func() tea.Msg {
		img, err := clipboard.ReadImage()
		if err != nil {
			return clipboardImageMsg{err: err}
		}
		return clipboardImageMsg{
			image: &core.ImageAttachment{
				Data:      img.Data,
				MediaType: img.MediaType,
			},
		}
	}
}

// ClearPendingImages removes all pending image attachments and returns them.
// Used by the parent model when consuming images for submission.
func (s *InputComponent) ClearPendingImages() []core.ImageAttachment {
	images := s.pendingImages
	s.pendingImages = nil
	return images
}

// PendingImageCount returns the number of images currently attached.
func (s *InputComponent) PendingImageCount() int {
	return len(s.pendingImages)
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

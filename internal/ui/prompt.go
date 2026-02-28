package ui

import (
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// ---------------------------------------------------------------------------
// Prompt overlay — modal prompt rendered by AppModel when active
// ---------------------------------------------------------------------------

// promptMode indicates the type of interactive prompt being displayed.
type promptMode string

const (
	promptModeSelect  promptMode = "select"
	promptModeConfirm promptMode = "confirm"
	promptModeInput   promptMode = "input"
)

// promptResult carries the synchronous outcome of a prompt overlay update.
// A non-nil value means the prompt is done (completed or cancelled); nil
// means the overlay is still active.
type promptResult struct {
	completed bool
	cancelled bool
	value     string
	index     int
	confirmed bool
}

// promptOverlay holds the state of an active interactive prompt. It is
// created when a PromptRequestEvent arrives and destroyed when the user
// completes or cancels. The AppModel owns the overlay and routes messages
// to it while in statePrompt.
type promptOverlay struct {
	mode      promptMode
	message   string
	options   []string       // select: available choices
	selected  int            // select: currently highlighted index
	confirmed bool           // confirm: current yes/no value
	inputTA   textarea.Model // input: text editor
	width     int
	height    int
}

// newSelectPrompt creates a prompt overlay for a selection list.
func newSelectPrompt(message string, options []string, width, height int) *promptOverlay {
	return &promptOverlay{
		mode:    promptModeSelect,
		message: message,
		options: options,
		width:   width,
		height:  height,
	}
}

// newConfirmPrompt creates a prompt overlay for a yes/no confirmation.
func newConfirmPrompt(message string, defaultValue bool, width, height int) *promptOverlay {
	return &promptOverlay{
		mode:      promptModeConfirm,
		message:   message,
		confirmed: defaultValue,
		width:     width,
		height:    height,
	}
}

// newInputPrompt creates a prompt overlay for free-form text input.
func newInputPrompt(message, placeholder, defaultValue string, width, height int) *promptOverlay {
	ta := textarea.New()
	ta.Placeholder = placeholder
	ta.ShowLineNumbers = false
	ta.Prompt = ""
	ta.CharLimit = 1000
	ta.SetWidth(width - 12) // account for border + padding
	ta.SetHeight(1)
	ta.Focus()

	// Prevent Enter from inserting a newline — we intercept it for submit.
	ta.KeyMap.InsertNewline = key.NewBinding(
		key.WithKeys("ctrl+j", "alt+enter"),
	)

	if defaultValue != "" {
		ta.SetValue(defaultValue)
		ta.CursorEnd()
	}

	return &promptOverlay{
		mode:    promptModeInput,
		message: message,
		inputTA: ta,
		width:   width,
		height:  height,
	}
}

// Init returns the initial command for the prompt overlay. For input mode
// this starts the cursor blink animation.
func (p *promptOverlay) Init() tea.Cmd {
	if p.mode == promptModeInput {
		return textarea.Blink
	}
	return nil
}

// Update handles messages for the prompt overlay. It returns a non-nil
// *promptResult when the user completes or cancels the prompt. The returned
// tea.Cmd is for textarea blink ticks (input mode only).
func (p *promptOverlay) Update(msg tea.Msg) (*promptResult, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		p.width = msg.Width
		p.height = msg.Height
		if p.mode == promptModeInput {
			p.inputTA.SetWidth(p.width - 12)
		}
		return nil, nil

	case tea.KeyPressMsg:
		switch p.mode {
		case promptModeSelect:
			return p.updateSelect(msg)
		case promptModeConfirm:
			return p.updateConfirm(msg)
		case promptModeInput:
			return p.updateInput(msg)
		}
	}

	// Pass non-key messages to textarea for blink animation.
	if p.mode == promptModeInput {
		var cmd tea.Cmd
		p.inputTA, cmd = p.inputTA.Update(msg)
		return nil, cmd
	}
	return nil, nil
}

func (p *promptOverlay) updateSelect(msg tea.KeyPressMsg) (*promptResult, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if p.selected > 0 {
			p.selected--
		}
	case "down", "j":
		if p.selected < len(p.options)-1 {
			p.selected++
		}
	case "home":
		p.selected = 0
	case "end":
		if len(p.options) > 0 {
			p.selected = len(p.options) - 1
		}
	case "enter":
		value := ""
		if p.selected < len(p.options) {
			value = p.options[p.selected]
		}
		return &promptResult{completed: true, value: value, index: p.selected}, nil
	case "esc":
		return &promptResult{cancelled: true}, nil
	}
	return nil, nil
}

func (p *promptOverlay) updateConfirm(msg tea.KeyPressMsg) (*promptResult, tea.Cmd) {
	switch msg.String() {
	case "left", "h", "y", "Y":
		p.confirmed = true
	case "right", "l", "n", "N":
		p.confirmed = false
	case "tab":
		p.confirmed = !p.confirmed
	case "enter":
		return &promptResult{completed: true, confirmed: p.confirmed}, nil
	case "esc":
		return &promptResult{cancelled: true}, nil
	}
	return nil, nil
}

func (p *promptOverlay) updateInput(msg tea.KeyPressMsg) (*promptResult, tea.Cmd) {
	switch msg.String() {
	case "enter":
		return &promptResult{completed: true, value: p.inputTA.Value()}, nil
	case "esc":
		return &promptResult{cancelled: true}, nil
	default:
		// Delegate character input, backspace, cursor movement, etc.
		var cmd tea.Cmd
		p.inputTA, cmd = p.inputTA.Update(msg)
		return nil, cmd
	}
}

// Render returns the prompt as a styled string for inline composition in the
// AppModel layout. The prompt replaces the normal input area (below the
// separator and above the status bar) rather than taking over the full screen.
func (p *promptOverlay) Render() string {
	theme := GetTheme()
	var content string

	switch p.mode {
	case promptModeSelect:
		content = p.viewSelect(theme)
	case promptModeConfirm:
		content = p.viewConfirm(theme)
	case promptModeInput:
		content = p.viewInput(theme)
	}

	return renderContentBlock(content, p.width,
		WithAlign(lipgloss.Left),
		WithBorderColor(theme.Accent),
		WithPaddingTop(0),
		WithPaddingBottom(0),
	)
}

func (p *promptOverlay) viewSelect(theme Theme) string {
	var lines []string
	lines = append(lines, lipgloss.NewStyle().Bold(true).Foreground(theme.Text).Render(p.message))
	lines = append(lines, "")

	for i, opt := range p.options {
		if i == p.selected {
			cursor := lipgloss.NewStyle().Foreground(theme.Accent).Bold(true).Render("> ")
			label := lipgloss.NewStyle().Foreground(theme.Accent).Bold(true).Render(opt)
			lines = append(lines, "  "+cursor+label)
		} else {
			lines = append(lines, "    "+lipgloss.NewStyle().Foreground(theme.Text).Render(opt))
		}
	}

	lines = append(lines, "")
	lines = append(lines, lipgloss.NewStyle().
		Foreground(theme.Muted).
		Render("  up/down navigate  Enter select  Esc cancel"))

	return strings.Join(lines, "\n")
}

func (p *promptOverlay) viewConfirm(theme Theme) string {
	var lines []string
	lines = append(lines, lipgloss.NewStyle().Bold(true).Foreground(theme.Text).Render(p.message))
	lines = append(lines, "")

	yesStyle := lipgloss.NewStyle().Foreground(theme.Text)
	noStyle := lipgloss.NewStyle().Foreground(theme.Text)
	if p.confirmed {
		yesStyle = yesStyle.Bold(true).Foreground(theme.Accent)
	} else {
		noStyle = noStyle.Bold(true).Foreground(theme.Accent)
	}

	yes := yesStyle.Render("[Yes]")
	no := noStyle.Render("[No]")
	lines = append(lines, "  "+yes+"  "+no)

	lines = append(lines, "")
	lines = append(lines, lipgloss.NewStyle().
		Foreground(theme.Muted).
		Render("  left/right switch  y/n  Enter confirm  Esc cancel"))

	return strings.Join(lines, "\n")
}

func (p *promptOverlay) viewInput(theme Theme) string {
	var lines []string
	lines = append(lines, lipgloss.NewStyle().Bold(true).Foreground(theme.Text).Render(p.message))
	lines = append(lines, "")
	lines = append(lines, p.inputTA.View())
	lines = append(lines, "")
	lines = append(lines, lipgloss.NewStyle().
		Foreground(theme.Muted).
		Render("  Enter submit  Esc cancel"))

	return strings.Join(lines, "\n")
}

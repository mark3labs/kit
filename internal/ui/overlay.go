package ui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// ---------------------------------------------------------------------------
// Overlay dialog — modal overlay rendered by AppModel when active
// ---------------------------------------------------------------------------

// overlayResult carries the synchronous outcome of an overlay dialog update.
// A non-nil value means the overlay is done (completed or cancelled); nil
// means the overlay is still active.
type overlayResult struct {
	completed bool
	cancelled bool
	action    string
	index     int
}

// overlayDialog holds the state of an active modal overlay dialog. It is
// created when an OverlayRequestEvent arrives and destroyed when the user
// completes or cancels. The AppModel owns the overlay and routes messages
// to it while in stateOverlay.
type overlayDialog struct {
	title       string
	content     string
	markdown    bool
	borderColor string
	background  string
	actions     []string
	selAction   int // selected action index
	scrollOff   int // scroll offset for content body
	totalLines  int // total body lines (computed on render)
	width       int // terminal width
	height      int // terminal height
	dialogWidth int // configured dialog width (0 = auto)
	maxHeight   int // configured max height (0 = auto)
	anchor      string
}

// newOverlayDialog creates an overlay dialog from an OverlayRequestEvent's
// parameters.
func newOverlayDialog(title, content string, markdown bool, borderColor, background string, width, maxHeight int, anchor string, actions []string, termWidth, termHeight int) *overlayDialog {
	return &overlayDialog{
		title:       title,
		content:     content,
		markdown:    markdown,
		borderColor: borderColor,
		background:  background,
		actions:     actions,
		dialogWidth: width,
		maxHeight:   maxHeight,
		anchor:      anchor,
		width:       termWidth,
		height:      termHeight,
	}
}

// Init returns the initial command for the overlay. Currently no-op.
func (o *overlayDialog) Init() tea.Cmd {
	return nil
}

// Update handles messages for the overlay dialog. It returns a non-nil
// *overlayResult when the user completes or cancels. The returned tea.Cmd
// is always nil (overlays don't produce async commands).
func (o *overlayDialog) Update(msg tea.Msg) (*overlayResult, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		o.width = msg.Width
		o.height = msg.Height
		return nil, nil

	case tea.KeyPressMsg:
		return o.handleKey(msg)
	}
	return nil, nil
}

func (o *overlayDialog) handleKey(msg tea.KeyPressMsg) (*overlayResult, tea.Cmd) {
	switch msg.String() {
	case "esc":
		return &overlayResult{cancelled: true}, nil

	case "enter":
		if len(o.actions) > 0 {
			action := ""
			if o.selAction < len(o.actions) {
				action = o.actions[o.selAction]
			}
			return &overlayResult{completed: true, action: action, index: o.selAction}, nil
		}
		// No actions — Enter dismisses (not cancelled).
		return &overlayResult{completed: true, action: "", index: -1}, nil

	// Content scrolling
	case "up", "k":
		if o.scrollOff > 0 {
			o.scrollOff--
		}
	case "down", "j":
		// Clamped in Render; allow incrementing freely.
		o.scrollOff++
	case "home", "g":
		o.scrollOff = 0
	case "end", "G":
		// Set to a large value; Render will clamp.
		o.scrollOff = o.totalLines

	// Action navigation
	case "left", "h":
		if len(o.actions) > 0 && o.selAction > 0 {
			o.selAction--
		}
	case "right", "l":
		if len(o.actions) > 0 && o.selAction < len(o.actions)-1 {
			o.selAction++
		}
	case "tab":
		if len(o.actions) > 0 {
			o.selAction = (o.selAction + 1) % len(o.actions)
		}
	}
	return nil, nil
}

// Render returns the overlay dialog as a styled string for full-view
// composition. The dialog is a bordered box centered (or anchored)
// horizontally within the terminal width.
func (o *overlayDialog) Render() string {
	theme := GetTheme()

	// Calculate dialog dimensions.
	dw := o.dialogWidth
	if dw == 0 {
		dw = o.width * 60 / 100
	}
	if dw < 30 {
		dw = 30
	}
	if dw > o.width-4 {
		dw = o.width - 4
	}

	mh := o.maxHeight
	if mh == 0 {
		mh = o.height * 80 / 100
	}
	if mh < 8 {
		mh = 8
	}
	if mh > o.height-2 {
		mh = o.height - 2
	}

	// Inner width accounts for border (2) + horizontal padding (2 left + 1 right).
	innerWidth := dw - 5
	if innerWidth < 10 {
		innerWidth = 10
	}

	// Render body text (potentially as markdown).
	bodyText := o.content
	if o.markdown {
		bodyText = toMarkdown(bodyText, innerWidth)
	}
	bodyText = strings.TrimRight(bodyText, "\n")

	bodyLines := strings.Split(bodyText, "\n")
	o.totalLines = len(bodyLines)

	// Calculate available height for the scrollable body.
	// Chrome: border(2) + padTop(1) + padBottom(1) + hintLine(1) = 5
	chromeLines := 5
	if o.title != "" {
		chromeLines += 2 // title line + separator line
	}
	if len(o.actions) > 0 {
		chromeLines += 2 // separator line + action bar
	}

	maxBodyLines := mh - chromeLines
	if maxBodyLines < 1 {
		maxBodyLines = 1
	}

	scrollable := len(bodyLines) > maxBodyLines
	if scrollable {
		// Clamp scroll offset.
		maxOff := len(bodyLines) - maxBodyLines
		if o.scrollOff > maxOff {
			o.scrollOff = maxOff
		}
		if o.scrollOff < 0 {
			o.scrollOff = 0
		}
		bodyLines = bodyLines[o.scrollOff : o.scrollOff+maxBodyLines]
	} else {
		o.scrollOff = 0
	}

	// Build the content to render inside the border.
	var parts []string

	// Title + separator.
	if o.title != "" {
		titleStyle := lipgloss.NewStyle().Bold(true).Foreground(theme.Text)
		parts = append(parts, titleStyle.Render(o.title))
		parts = append(parts, lipgloss.NewStyle().
			Foreground(theme.Muted).
			Render(repeatRune('─', innerWidth)))
	}

	// Body content.
	parts = append(parts, "")
	parts = append(parts, strings.Join(bodyLines, "\n"))

	// Scroll indicator.
	if scrollable {
		indicator := fmt.Sprintf("(%d–%d of %d lines)",
			o.scrollOff+1,
			min(o.scrollOff+maxBodyLines, o.totalLines),
			o.totalLines)
		parts = append(parts, lipgloss.NewStyle().
			Foreground(theme.VeryMuted).
			Render(indicator))
	} else {
		parts = append(parts, "")
	}

	// Action bar.
	if len(o.actions) > 0 {
		parts = append(parts, lipgloss.NewStyle().
			Foreground(theme.Muted).
			Render(repeatRune('─', innerWidth)))

		var actionParts []string
		for i, a := range o.actions {
			if i == o.selAction {
				actionParts = append(actionParts,
					lipgloss.NewStyle().Bold(true).Foreground(theme.Accent).Render("> "+a))
			} else {
				actionParts = append(actionParts,
					lipgloss.NewStyle().Foreground(theme.Text).Render("  "+a))
			}
		}
		parts = append(parts, strings.Join(actionParts, "    "))
	}

	innerContent := strings.Join(parts, "\n")

	// Resolve border color.
	borderClr := lipgloss.Color("#89b4fa") // default blue
	if o.borderColor != "" {
		borderClr = lipgloss.Color(o.borderColor)
	}

	// Build the dialog box style.
	dialogStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderClr).
		Width(dw-2). // -2 for border chars
		Padding(1, 1, 1, 2).
		Foreground(theme.Text)

	if o.background != "" {
		dialogStyle = dialogStyle.Background(lipgloss.Color(o.background))
	}

	dialog := dialogStyle.Render(innerContent)

	// Key hints below the dialog.
	var hints []string
	if scrollable {
		hints = append(hints, "↑/↓ scroll")
	}
	if len(o.actions) > 0 {
		hints = append(hints, "←/→ switch")
		hints = append(hints, "Enter select")
	} else {
		hints = append(hints, "Enter dismiss")
	}
	hints = append(hints, "Esc cancel")
	hintText := lipgloss.NewStyle().
		Foreground(theme.Muted).
		Render("  " + strings.Join(hints, "  "))

	full := lipgloss.JoinVertical(lipgloss.Left, dialog, hintText)

	// Center horizontally within the terminal width.
	centered := lipgloss.PlaceHorizontal(o.width, lipgloss.Center, full)

	// Apply vertical positioning based on anchor.
	// Calculate how many lines we have and how many we need.
	contentHeight := lipgloss.Height(centered)
	if contentHeight < o.height {
		switch o.anchor {
		case "top-center":
			// Add one blank line at top for breathing room.
			centered = "\n" + centered
		case "bottom-center":
			// Pad from the top so the dialog sits near the bottom.
			topPad := o.height - contentHeight - 1
			if topPad > 0 {
				centered = strings.Repeat("\n", topPad) + centered
			}
		default: // "center"
			// Vertically center within available height.
			topPad := (o.height - contentHeight) / 2
			if topPad > 0 {
				centered = strings.Repeat("\n", topPad) + centered
			}
		}
	}

	return centered
}

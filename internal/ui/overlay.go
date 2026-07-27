package ui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	xansi "github.com/charmbracelet/x/ansi"

	"github.com/mark3labs/kit/internal/ui/style"
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

	// dismissOnly marks an overlay that has no consumer for its result —
	// the message inspector, which the UI opens for reading only. Enter and
	// Esc then close the dialog with no observable difference, so the key
	// hint advertises a single "close" action instead of implying that
	// dismiss and cancel lead to different outcomes.
	//
	// Extension overlays leave this false: ShowOverlay reports Enter as
	// completed and Esc as cancelled, and extensions may branch on it.
	dismissOnly bool
}

// Dialog layout constants.
const (
	// dialogTabWidth is the number of spaces a tab expands to inside the
	// dialog body. It must match the box style's tab handling so wrapping
	// measurements agree with what is rendered.
	dialogTabWidth = 4
)

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
	theme := style.GetTheme()

	// Calculate dialog dimensions, clamped to terminal bounds.
	termW := max(o.width, 10)
	termH := max(o.height, 5)

	dw := o.dialogWidth
	if dw == 0 {
		dw = termW * 60 / 100
	}
	dw = clamp(dw, min(24, termW), termW-2)

	mh := o.maxHeight
	if mh == 0 {
		mh = termH * 80 / 100
	}
	mh = clamp(mh, min(6, termH), termH)

	// Inner width accounts for border (2) + horizontal padding (2 left + 1 right).
	// lipgloss's Width() sets the total rendered width including the border,
	// so the dialog style below is given the full dw and the content area is
	// what remains after the frame and padding.
	innerWidth := max(dw-5, 6)

	// Render body text (potentially as markdown).
	bodyText := o.content
	if o.markdown {
		bodyText = style.ToMarkdown(bodyText, innerWidth)
	}
	bodyText = strings.TrimRight(bodyText, "\n")

	// Expand tabs before measuring. Wrapping counts a tab as one cell but
	// the box style renders it as several, so a line wrapped to exactly the
	// content width would overflow and be wrapped a second time by the
	// style — silently doubling the height of tab-indented output such as
	// grep matches. Expanding first makes the two agree.
	bodyText = strings.ReplaceAll(bodyText, "\t", strings.Repeat(" ", dialogTabWidth))

	// Wrap to the content width *before* measuring.
	//
	// The box style would otherwise wrap over-long lines itself, at which
	// point one source line occupies several rows and every downstream
	// number is wrong: totalLines under-counts, the maxBodyLines slice lets
	// through more rows than budgeted (a grep result with long paths grew a
	// 30-row terminal's dialog to 48 rows), and scrolling moves by source
	// line rather than by the row the reader actually sees.
	//
	// Wrapping here makes one element of bodyLines equal exactly one
	// rendered row, so the height budget, the scroll offset and the "of N
	// lines" counter all agree with the display.
	bodyText = xansi.Wrap(bodyText, innerWidth, "")

	bodyLines := strings.Split(bodyText, "\n")
	o.totalLines = len(bodyLines)

	// Calculate available height for the scrollable body.
	// Chrome: border(2) + padTop(1) + padBottom(1) + blank(1) + footer(1) = 6.
	// The footer row carries the scroll indicator and the key hints; both
	// live inside the border so no part of the dialog is a bare strip of
	// padding that would erase the view behind it.
	chromeLines := 6
	if o.title != "" {
		chromeLines += 2 // title line + separator line
	}
	if len(o.actions) > 0 {
		chromeLines += 2 // separator line + action bar
	}

	maxBodyLines := max(mh-chromeLines, 1)

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

	// Footer: scroll position on the left, key hints on the right.
	parts = append(parts, o.renderFooter(scrollable, maxBodyLines, innerWidth, theme))

	innerContent := strings.Join(parts, "\n")

	// Resolve border color.
	borderClr := theme.Info
	if o.borderColor != "" {
		borderClr = lipgloss.Color(o.borderColor)
	}

	// Build the dialog box style. Width() is the total rendered width
	// including the border, so dw is passed through unmodified — subtracting
	// the border here would shrink the content area below innerWidth and
	// wrap the title separator and action bar onto extra lines.
	dialogStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderClr).
		Width(dw).
		Padding(1, 1, 1, 2).
		Foreground(theme.Text)

	if o.background != "" {
		dialogStyle = dialogStyle.Background(lipgloss.Color(o.background))
	}

	dialog := dialogStyle.Render(innerContent)

	// The box is returned unpositioned; the caller composites it over the
	// main view at the anchor point (see compositeAnchored). Positioning
	// here by padding with spaces and newlines would produce an opaque
	// full-screen block that erases whatever is behind it.
	return dialog
}

// renderFooter builds the dialog's bottom row: the scroll position on the
// left and the key hints on the right, padded to exactly innerWidth.
//
// The row lives inside the border. Rendering hints below the box would leave
// a bare band of padding that the compositor draws as opaque cells, cutting a
// blank strip through the content behind the dialog.
//
// Both halves compete for one row, so the content degrades by priority rather
// than being dropped outright: the verbose "(1–12 of 200 lines)" collapses to
// "12/200", and the "↑/↓ scroll" hint goes before any dismiss hint because a
// visible position indicator already implies the dialog scrolls. The keys
// that close the dialog are the last thing surrendered.
func (o *overlayDialog) renderFooter(scrollable bool, maxBodyLines, innerWidth int, theme style.Theme) string {
	mutedStyle := lipgloss.NewStyle().Foreground(theme.Muted)
	veryMutedStyle := lipgloss.NewStyle().Foreground(theme.VeryMuted)

	var indicators []string
	if scrollable {
		last := min(o.scrollOff+maxBodyLines, o.totalLines)
		indicators = append(indicators,
			fmt.Sprintf("(%d–%d of %d lines)", o.scrollOff+1, last, o.totalLines),
			fmt.Sprintf("%d/%d", last, o.totalLines),
		)
	}
	indicators = append(indicators, "")

	// Hint sets, widest first.
	hintSets := [][]string{o.hintLabels(scrollable, true), o.hintLabels(scrollable, false)}

	for _, hints := range hintSets {
		hintText := strings.Join(hints, "  ")
		hintW := lipgloss.Width(hintText)

		for _, indicator := range indicators {
			if indicator == "" {
				continue
			}
			if gap := innerWidth - lipgloss.Width(indicator) - hintW; gap >= 2 {
				return veryMutedStyle.Render(indicator) +
					strings.Repeat(" ", gap) +
					mutedStyle.Render(hintText)
			}
		}

		// Hints alone, right-aligned.
		if pad := innerWidth - hintW; pad >= 0 {
			return strings.Repeat(" ", pad) + mutedStyle.Render(hintText)
		}
	}

	// Narrower than even the terse hints — show them and let the box clip.
	return mutedStyle.Render(strings.Join(o.hintLabels(scrollable, false), "  "))
}

// hintLabels returns the key hints for the dialog's current configuration.
// verbose selects the full wording over the terse forms used when the row is
// too narrow to spell the keys out.
func (o *overlayDialog) hintLabels(scrollable, verbose bool) []string {
	var hints []string

	if verbose {
		if scrollable {
			hints = append(hints, "↑/↓ scroll")
		}
		switch {
		case len(o.actions) > 0:
			hints = append(hints, "←/→ switch", "Enter select", "Esc cancel")
		case o.dismissOnly:
			hints = append(hints, "Enter/Esc close")
		default:
			hints = append(hints, "Enter dismiss", "Esc cancel")
		}
		return hints
	}

	switch {
	case len(o.actions) > 0:
		hints = append(hints, "↵ select", "esc")
	case o.dismissOnly:
		hints = append(hints, "↵/esc close")
	default:
		hints = append(hints, "↵ ok", "esc")
	}
	return hints
}

// Anchor returns the configured vertical anchor for this dialog.
func (o *overlayDialog) Anchor() string {
	return o.anchor
}

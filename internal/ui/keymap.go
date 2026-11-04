package ui

import (
	"charm.land/bubbles/v2/key"
)

// appKeyMap is the single source of truth for Kit's built-in key bindings.
//
// The key handlers match against these bindings (key.Matches), and the /help
// modal renders them, so a key cannot change in one place and drift in the
// other. Add a binding here first, then match it in the handler and list it
// in helpSections.
//
// Leader chords (Ctrl+X, then a key) bind the second key only: the handler
// matches it after the leader has armed. Their help text names the full
// chord.
type appKeyMap struct {
	// General
	Interrupt      key.Binding
	Cancel         key.Binding
	CycleThinking  key.Binding
	SubagentRuns   key.Binding
	PreviewImages  key.Binding
	QueueWhileBusy key.Binding

	// Leader chords (Ctrl+X prefix)
	Leader         key.Binding
	Steer          key.Binding
	ToggleThinking key.Binding
	MessageNav     key.Binding
	ExternalEditor key.Binding

	// Composer
	Submit      key.Binding
	Newline     key.Binding
	HistoryPrev key.Binding
	HistoryNext key.Binding
	PasteImage  key.Binding
	ClearImages key.Binding

	// Completion popup
	PopupUp     key.Binding
	PopupDown   key.Binding
	PopupAccept key.Binding
	PopupRun    key.Binding
	PopupClose  key.Binding

	// Scrollback
	PageUp   key.Binding
	PageDown key.Binding
	Top      key.Binding
	Bottom   key.Binding

	// Message navigation (Ctrl+X m)
	NavUp       key.Binding
	NavDown     key.Binding
	NavFirst    key.Binding
	NavLast     key.Binding
	NavPrevUser key.Binding
	NavNextUser key.Binding
	NavCopy     key.Binding
	NavOpen     key.Binding
	NavExpand   key.Binding
	NavLeft     key.Binding
	NavRight    key.Binding
	NavExit     key.Binding
}

// bind builds a binding whose help key is shown as label.
func bind(label, desc string, keys ...string) key.Binding {
	return key.NewBinding(key.WithKeys(keys...), key.WithHelp(label, desc))
}

// keys holds the active built-in key bindings.
var keys = newAppKeyMap()

func newAppKeyMap() appKeyMap {
	return appKeyMap{
		Interrupt:      bind("ctrl+c", "Clear input, press again to quit", "ctrl+c"),
		Cancel:         bind("esc esc", "Stop the running turn", "esc"),
		CycleThinking:  bind("shift+tab", "Cycle reasoning effort", "shift+tab"),
		SubagentRuns:   bind("ctrl+alt+a", "Inspect a subagent run", "ctrl+alt+a"),
		PreviewImages:  bind("ctrl+o", "Preview attached images", "ctrl+o"),
		QueueWhileBusy: bind("enter", "Queue a message while working", "enter"),

		Leader:         bind("ctrl+x", "Leader key for the chords below", "ctrl+x"),
		Steer:          bind("ctrl+x s", "Steer the agent mid-turn", "s"),
		ToggleThinking: bind("ctrl+x t", "Show or hide thinking blocks", "t"),
		MessageNav:     bind("ctrl+x m", "Browse scrollback messages", "m"),
		ExternalEditor: bind("ctrl+x e", "Write the prompt in $EDITOR", "e"),

		Submit:      bind("enter", "Send the message", "enter"),
		Newline:     bind("shift+enter", "Insert a new line (also ctrl+j)", "shift+enter", "ctrl+j"),
		HistoryPrev: bind("↑", "Previous prompt from history", "up"),
		HistoryNext: bind("↓", "Next prompt from history", "down"),
		PasteImage:  bind("ctrl+v", "Paste an image from the clipboard", "ctrl+v"),
		ClearImages: bind("ctrl+u", "Remove attached images", "ctrl+u"),

		PopupUp:     bind("↑", "Previous suggestion", "up"),
		PopupDown:   bind("↓", "Next suggestion", "down"),
		PopupAccept: bind("tab", "Complete the suggestion", "tab"),
		PopupRun:    bind("enter", "Complete and run", "enter"),
		PopupClose:  bind("esc", "Close the suggestions", "esc"),

		PageUp:   bind("pgup", "Scroll up one page", "pgup"),
		PageDown: bind("pgdn", "Scroll down one page", "pgdown"),
		Top:      bind("ctrl+home", "Go to the first message", "ctrl+home"),
		Bottom:   bind("ctrl+end", "Go to the latest message", "ctrl+end"),

		NavUp:       bind("↑ k", "Previous message", "up", "k"),
		NavDown:     bind("↓ j", "Next message", "down", "j"),
		NavFirst:    bind("home g", "First message", "home", "g"),
		NavLast:     bind("end G", "Last message", "end", "G"),
		NavPrevUser: bind("u", "Previous message you sent", "u"),
		NavNextUser: bind("U", "Next message you sent", "U"),
		NavCopy:     bind("y", "Copy the message", "y"),
		NavOpen:     bind("enter", "Open the message, or expand shell output", "enter"),
		NavExpand:   bind("o space", "Expand or collapse shell output", "o", "space"),
		NavLeft:     bind("H", "Scroll shell output left", "H", "shift+left"),
		NavRight:    bind("L", "Scroll shell output right", "L", "shift+right"),
		NavExit:     bind("esc q", "Stop browsing", "esc", "q"),
	}
}

// helpEntry is one row of the help modal: a key (or command) and what it does.
type helpEntry struct {
	Key  string
	Desc string
	// Note is an optional muted suffix, e.g. command aliases.
	Note string
}

// helpSection is a titled group of help rows.
type helpSection struct {
	Title   string
	Entries []helpEntry
}

// bindingEntries converts key bindings to help rows, skipping disabled ones.
func bindingEntries(bs ...key.Binding) []helpEntry {
	out := make([]helpEntry, 0, len(bs))
	for _, b := range bs {
		if !b.Enabled() {
			continue
		}
		h := b.Help()
		out = append(out, helpEntry{Key: h.Key, Desc: h.Desc})
	}
	return out
}

// keyHelpSections returns the built-in key binding groups for the help modal.
func (k appKeyMap) keyHelpSections() []helpSection {
	return []helpSection{
		{"General", bindingEntries(k.Interrupt, k.Cancel, k.QueueWhileBusy, k.CycleThinking, k.SubagentRuns, k.PreviewImages)},
		{"Leader chords", bindingEntries(k.Steer, k.ToggleThinking, k.MessageNav, k.ExternalEditor)},
		{"Composer", bindingEntries(k.Submit, k.Newline, k.HistoryPrev, k.HistoryNext, k.PasteImage, k.ClearImages)},
		{"Suggestions popup", bindingEntries(k.PopupUp, k.PopupDown, k.PopupAccept, k.PopupRun, k.PopupClose)},
		{"Scrollback", bindingEntries(k.PageUp, k.PageDown, k.Top, k.Bottom)},
		{"Message browsing", bindingEntries(k.NavUp, k.NavDown, k.NavFirst, k.NavLast, k.NavPrevUser, k.NavNextUser, k.NavCopy, k.NavOpen, k.NavExpand, k.NavLeft, k.NavRight, k.NavExit)},
	}
}

// composerSyntaxSection lists the prefixes the composer gives meaning to.
// These are triggers in InputComponent and handleSubmit, not key bindings.
func composerSyntaxSection() helpSection {
	return helpSection{"Input syntax", []helpEntry{
		{Key: "/", Desc: "Run a command"},
		{Key: "@path", Desc: "Attach a file (opens a picker)"},
		{Key: "!cmd", Desc: "Run a shell command, output goes to the model"},
		{Key: "!!cmd", Desc: "Run a shell command, output stays private"},
		{Key: ":name", Desc: "Insert an emoji"},
	}}
}

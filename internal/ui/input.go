package ui

import (
	"fmt"
	"image/color"
	"sort"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/log"

	"github.com/mark3labs/kit/internal/clipboard"
	"github.com/mark3labs/kit/internal/ui/commands"
	"github.com/mark3labs/kit/internal/ui/core"
	"github.com/mark3labs/kit/internal/ui/imagepreview"
	"github.com/mark3labs/kit/internal/ui/style"
	"github.com/mark3labs/kit/internal/ui/termgfx"
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
	submitNext  bool // defer submit one tick so popup dismisses cleanly

	// popup is the shared PopupList used to render the / and @ autocomplete
	// dropdowns. State (items, cursor, visible search-driven filter) is
	// driven externally by InputComponent — we only use PopupList for the
	// rendering chrome so all popups in the app look identical.
	popup *PopupList

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
	fileAtStartIdx  int                     // byte offset of @ (or path start in /edit mode) in the textarea value
	fileSuggestions []FileSuggestion        // backing storage for file entries
	fileSynthCmds   []commands.SlashCommand // synthetic commands.SlashCommands wrapping file entries

	// fileEditMode is true when fileMode was activated by the /edit slash
	// command rather than an @ trigger. Selecting a file submits the line
	// (running $EDITOR on it); selecting a directory drills further like @
	// does. MCP resources are excluded in this mode.
	fileEditMode bool

	// cwd is the working directory used for @file path resolution and
	// autocomplete suggestions. Set by the parent via SetCwd.
	cwd string

	// mcpResources is a callback that returns available MCP resources for
	// the @ autocomplete popup. Set by the parent via SetMCPResourceProvider.
	mcpResources func() []FileSuggestion

	// appCtrl is used for slash commands that mutate app state.
	// May be nil in tests; nil-safe.
	appCtrl AppController

	// pendingImages holds clipboard images attached to the next submission.
	// Images are added via Ctrl+V and cleared on submit or Ctrl+U.
	pendingImages []core.ImageAttachment

	// imageThumbs caches the rendered thumbnail for each entry in
	// pendingImages (1:1 index correspondence). Thumbnails are rendered
	// asynchronously off the Bubble Tea event loop (decode + resample is too
	// slow to run inside Update), so an entry starts as the empty string
	// placeholder and is filled in when the matching thumbnailReadyMsg
	// arrives. An entry stays empty when the terminal can display no preview
	// at all, in which case the text pill is shown alone.
	//
	// The contents are either half-block art or a grid of Unicode placeholders
	// that display a transmitted image; both are ordinary styled text as far as
	// the view is concerned. See internal/ui/imagepreview.
	imageThumbs []string

	// imageIDs holds the terminal image id backing each entry in imageThumbs
	// (1:1 index correspondence), or 0 when the thumbnail is half-block art and
	// owns no terminal resource. Ids are kept so the images can be freed when
	// the pending set is cleared; terminals hold transmitted image data until
	// told to drop it.
	imageIDs []uint32

	// imagePlace holds the placement escape sequence for each entry in
	// imageThumbs (1:1 index correspondence), or "" when the thumbnail draws
	// itself as text and needs no placement.
	//
	// A directly-placed image is painted over the screen rather than being part
	// of the view, so it must be redrawn at its absolute position after every
	// frame. See DirectPlacements.
	imagePlace []string

	// imageGen is a monotonic generation counter incremented whenever the
	// pending image set is cleared. Async thumbnail results carry the
	// generation they were enqueued under and are discarded if it no longer
	// matches, preventing a stale thumbnail from landing on the wrong slot
	// after a clear + re-attach.
	imageGen int

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

// inputChromeWidth is the number of columns the composer frame consumes: one
// column of horizontal padding on each side of the filled bar. The textarea is
// sized to the remainder so text neither wraps early nor overflows.
const inputChromeWidth = 2

// inputMaxRows caps how tall the composer grows before the textarea begins
// scrolling its own content.
const inputMaxRows = 8

// inputMaxContentRows bounds how much text the composer will hold. It exists
// only to keep a runaway paste from exhausting memory; it is far beyond any
// prompt a person would type, so in practice input is unlimited.
const inputMaxContentRows = 10000

// defaultPlaceholder doubles as the submit hint. Folding the hint into the
// placeholder means the composer needs no separate help line, and the hint
// disappears exactly when it stops being useful — as soon as the user types.
const defaultPlaceholder = "Ask kit anything…   ↵ send"

// clipboardImageMsg is the result of an async clipboard image read.
type clipboardImageMsg struct {
	image *core.ImageAttachment
	err   error
}

// thumbnailReadyMsg carries the result of an async thumbnail render back to
// the Update loop. gen and index identify the pendingImages slot the
// thumbnail belongs to; the result is dropped if the generation no longer
// matches (the pending set was cleared) or the index is out of range.
type thumbnailReadyMsg struct {
	gen   int
	index int
	thumb string

	// transmit carries image data that must reach the terminal outside the
	// view text, and is empty for half-block thumbnails. See
	// imagepreview.Thumbnail.
	transmit string

	// imageID identifies the transmitted image so it can be freed later, and is
	// zero for half-block thumbnails.
	imageID uint32

	// place is the escape sequence that draws the image at the cursor, for
	// thumbnails that are painted over the screen instead of drawn as text.
	place string
}

// NewInputComponent creates a new InputComponent with the given width and
// optional AppController. If appCtrl is nil the component still works but
// /clear and /clear-queue are no-ops.
func NewInputComponent(width int, appCtrl AppController) *InputComponent {
	ta := textarea.New()
	ta.Placeholder = defaultPlaceholder
	ta.ShowLineNumbers = false
	ta.Prompt = ""
	ta.CharLimit = 0
	// Clamp to at least one column: on a pathologically narrow terminal
	// width-inputChromeWidth would go non-positive and SetWidth would receive
	// a negative width.
	ta.SetWidth(max(width-inputChromeWidth, 1))

	// The composer starts as a single row and grows with the text, so an empty
	// prompt costs one line instead of four. Beyond inputMaxRows the textarea
	// scrolls internally rather than eating the transcript.
	//
	// MaxContentHeight must be set explicitly: with only MaxHeight the
	// textarea treats it as a content limit and silently refuses newlines past
	// that many logical lines, which would cap a prompt at inputMaxRows.
	// Setting it separately keeps MaxHeight governing the viewport alone.
	ta.DynamicHeight = true
	ta.MinHeight = 1
	ta.MaxHeight = inputMaxRows
	ta.MaxContentHeight = inputMaxContentRows
	ta.SetHeight(1)
	ta.Focus()

	// Override InsertNewline so only ctrl+j and shift+enter insert newlines.
	// Enter always submits the input.
	ta.KeyMap.InsertNewline = key.NewBinding(
		key.WithKeys("ctrl+j", "shift+enter"),
		key.WithHelp("ctrl+j", "insert newline"),
	)

	// Style the textarea using theme colors. Every span the textarea emits
	// carries the composer background explicitly: a Background() set only on
	// the outer container tears at each inner ANSI reset, which renders as
	// patchy off-color blocks across the bar.
	theme := style.GetTheme()
	styles := ta.Styles()
	applyComposerStyles(&styles, theme)
	ta.SetStyles(styles)

	ic := &InputComponent{
		textarea:    ta,
		commands:    commands.SlashCommands,
		width:       width,
		popupHeight: 7,
		appCtrl:     appCtrl,
	}
	ic.popup = NewPopupList("", nil, width, 0)
	ic.popup.ShowSearch = false
	ic.popup.HideCount = true
	ic.popup.MaxVisible = ic.popupHeight
	ic.popup.FooterHint = "↑↓ navigate · tab complete · ↵ select · esc dismiss"
	return ic
}

// SetCwd sets the working directory used for @file autocomplete suggestions
// and path resolution. Should be called by the parent after construction.
func (s *InputComponent) SetCwd(cwd string) {
	s.cwd = cwd
}

// SetMCPResourceProvider sets a callback that returns MCP resource suggestions
// for the @ autocomplete popup. Called by the parent after construction.
func (s *InputComponent) SetMCPResourceProvider(fn func() []FileSuggestion) {
	s.mcpResources = fn
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
		s.textarea.SetWidth(max(msg.Width-inputChromeWidth, 1))
		return s, nil

	case clipboardImageMsg:
		if msg.err != nil {
			// Silently ignore — no image on clipboard or tool unavailable.
			return s, nil
		}
		if msg.image != nil {
			img := *msg.image
			index := len(s.pendingImages)
			s.pendingImages = append(s.pendingImages, img)
			// Reserve a placeholder; the async render fills it in via
			// thumbnailReadyMsg so Update never blocks on decode/resample.
			s.imageThumbs = append(s.imageThumbs, "")
			s.imageIDs = append(s.imageIDs, 0)
			s.imagePlace = append(s.imagePlace, "")
			cols := s.thumbCols()
			if cols < 1 {
				return s, nil
			}
			return s, renderThumbnailCmd(img, cols, thumbMaxRows, style.GetTheme().Background, s.imageGen, index)
		}
		return s, nil

	case thumbnailReadyMsg:
		if msg.gen != s.imageGen || msg.index < 0 || msg.index >= len(s.imageThumbs) {
			// The slot is gone (cleared, or re-attached under a new
			// generation). Free the image rather than leaking it in the
			// terminal, which has already been told to hold the data.
			if seq := imagepreview.DeleteImage(msg.imageID); seq != "" {
				return s, tea.Raw(seq)
			}
			return s, nil
		}
		s.imageThumbs[msg.index] = msg.thumb
		if msg.index < len(s.imageIDs) {
			s.imageIDs[msg.index] = msg.imageID
		}
		if msg.index < len(s.imagePlace) {
			s.imagePlace[msg.index] = msg.place
		}
		if msg.transmit != "" {
			// The image data must be written straight to the terminal; it
			// cannot travel inside the view text. For placeholder thumbnails
			// the cells are already in the view and the terminal fills them in
			// as soon as the data lands.
			cmds := []tea.Cmd{tea.Raw(msg.transmit)}
			if msg.place != "" {
				// A directly-placed image is drawn by AppModel, which can only
				// work out where it goes once this frame has been laid out.
				// Nothing else would arrive to trigger that write, so ask for
				// one more Update after the coming View.
				cmds = append(cmds, gfxNudgeCmd())
			}
			return s, tea.Batch(cmds...)
		}
		return s, nil

	case tea.KeyPressMsg:
		if !s.showPopup {
			switch msg.String() {
			case "enter":
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
					cleanup := s.releaseImages()
					s.pendingImages = nil
					s.imageGen++
					return s, cleanup
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
					selectedCmd := s.filtered[s.selected].Command
					// Populate textarea with selected item and submit on next tick.
					if s.argMode {
						s.textarea.SetValue(s.argCommand + " " + selectedCmd.Name)
					} else {
						s.textarea.SetValue(selectedCmd.Name)
					}
					s.textarea.CursorEnd()
					s.showPopup = false
					s.selected = 0
					// If the selected command expects arguments, populate
					// the input with the command + trailing space so the
					// user can type args, instead of auto-submitting.
					if !s.argMode && selectedCmd.HasArgs {
						s.textarea.SetValue(selectedCmd.Name + " ")
						s.textarea.CursorEnd()
					} else {
						s.submitNext = true
					}
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
			if hasAt, prefix, atIdx := ExtractAtPrefix(line, cursorCol); hasAt {
				var suggestions []FileSuggestion

				// Local file suggestions (only if cwd is set).
				if s.cwd != "" {
					suggestions = GetFileSuggestions(prefix, s.cwd)
				}

				// MCP resource suggestions — merge with file suggestions.
				if s.mcpResources != nil {
					mcpSuggestions := s.mcpResources()
					if prefix != "" {
						// Fuzzy-filter MCP resources against the typed prefix.
						queryLower := strings.ToLower(prefix)
						var filtered []FileSuggestion
						for _, r := range mcpSuggestions {
							score := scoreFilePath(queryLower, r.RelPath)
							if score <= 0 {
								// Also try matching against the resource name without prefix.
								score = scoreFilePath(queryLower, r.MCPServerName+"/"+r.RelPath)
							}
							if score > 0 {
								r.Score = score
								filtered = append(filtered, r)
							}
						}
						mcpSuggestions = filtered
					}
					suggestions = append(suggestions, mcpSuggestions...)
				}

				if len(suggestions) > 0 {
					// Sort by score descending, cap at maxFileSuggestions.
					sort.Slice(suggestions, func(i, j int) bool {
						return suggestions[i].Score > suggestions[j].Score
					})
					if len(suggestions) > maxFileSuggestions {
						suggestions = suggestions[:maxFileSuggestions]
					}

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
						} else if fs.IsMCPResource {
							desc = "mcp:" + fs.MCPServerName
						}
						s.fileSynthCmds[i] = commands.SlashCommand{Name: name, Description: desc}
						s.filtered[i] = FuzzyMatch{Command: &s.fileSynthCmds[i], Score: fs.Score}
					}
					s.selected = 0
				} else {
					s.showPopup = false
					s.fileMode = false
					s.fileEditMode = false
				}
			} else if len(lines) == 1 && strings.HasPrefix(lines[0], "/") {
				s.fileMode = false
				s.fileEditMode = false
				if cmdLen, pathPrefix, isEdit := ExtractEditPrefix(lines[0]); isEdit {
					// /edit fuzzy-file picker. Behaves like @ except
					// MCP resources are excluded and selecting a file
					// submits the line (running $EDITOR).
					s.updateEditFilePopup(cmdLen, pathPrefix)
				} else if !strings.Contains(lines[0], " ") {
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
				s.fileEditMode = false
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
	cleanup := s.releaseImages()
	s.pendingImages = nil
	s.imageGen++
	submit := func() tea.Msg {
		return core.SubmitMsg{Text: trimmed, Images: images}
	}
	if cleanup == nil {
		return submit
	}
	// Free the preview images before handing off. tea.Sequence keeps the order
	// so the terminal drops them while their placeholder cells are still on
	// screen, rather than after the composer has already been redrawn.
	return tea.Sequence(cleanup, submit)
}

// releaseImages returns a command that frees every transmitted preview image
// and resets the thumbnail caches, or nil when nothing was transmitted.
//
// A terminal keeps transmitted image data until it is told to drop it, so a
// session that attaches and clears many images would otherwise leave all of
// them resident.
func (s *InputComponent) releaseImages() tea.Cmd {
	var seq strings.Builder
	for _, id := range s.imageIDs {
		seq.WriteString(imagepreview.DeleteImage(id))
	}
	s.imageThumbs = nil
	s.imageIDs = nil
	s.imagePlace = nil
	if seq.Len() == 0 {
		return nil
	}
	return tea.Raw(seq.String())
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

// thumbMaxCols and thumbMaxRows cap the size, in terminal cells, of pending
// image previews. Kept small for the low-res look and to keep scrollback
// light.
const (
	thumbMaxCols = 40
	thumbMaxRows = 12
)

// thumbCols returns the thumbnail width in terminal cells given the current
// input width, or 0 when there is no room to render a preview.
func (s *InputComponent) thumbCols() int {
	if s.width <= 6 {
		return 0
	}
	cols := min(thumbMaxCols, s.width-6)
	if cols < 1 {
		return 0
	}
	return cols
}

// renderThumbnailCmd returns a tea.Cmd that renders an image preview off the
// Bubble Tea event loop. The decode + resample work runs in the Cmd goroutine,
// and the result is delivered as a thumbnailReadyMsg tagged with the
// generation and slot index it was enqueued for. An empty thumbnail (terminal
// unsupported or render error) leaves the text pill in place.
//
// A terminal that answered the graphics probe gets a real image; anything else
// gets half-block art. The Kitty path falls back to half-block on error rather
// than showing nothing, so a malformed image degrades instead of vanishing.
func renderThumbnailCmd(img core.ImageAttachment, cols, rows int, bg color.Color, gen, index int) tea.Cmd {
	return func() tea.Msg {
		caps := termgfx.Current()
		var (
			kt  imagepreview.Thumbnail
			err error
		)
		switch termgfx.PreviewMode() {
		case termgfx.ModePlaceholder:
			kt, err = imagepreview.RenderKitty(img.Data, cols, rows, caps.CellWidth, caps.CellHeight)
		case termgfx.ModeDirect:
			kt, err = imagepreview.RenderKittyDirect(img.Data, cols, rows, caps.CellWidth, caps.CellHeight)
		}
		if kt.Cells != "" && err == nil {
			return thumbnailReadyMsg{
				gen:      gen,
				index:    index,
				thumb:    kt.Cells,
				transmit: kt.Transmit,
				imageID:  kt.ImageID,
				place:    kt.Place(),
			}
		}
		if err != nil {
			log.Debug("kitty thumbnail failed, using half blocks", "error", err)
		}
		thumb, rerr := imagepreview.Render(img.Data, img.MediaType, cols, rows, bg)
		if rerr != nil {
			thumb = ""
		}
		return thumbnailReadyMsg{gen: gen, index: index, thumb: thumb}
	}
}

// gfxNudgeMsg asks for one more Update/View cycle. It carries no data and is
// handled as a no-op: its only purpose is to give AppModel.Update a chance to
// write a graphics placement that the View before it has just computed.
type gfxNudgeMsg struct{}

// gfxNudgeCmd delivers a gfxNudgeMsg immediately.
func gfxNudgeCmd() tea.Cmd {
	return func() tea.Msg { return gfxNudgeMsg{} }
}

// DirectPlacement locates one directly-placed preview image within the input
// component's view.
type DirectPlacement struct {
	// RowOffset is the image's first row, counted from the top of the input
	// component's own view.
	RowOffset int

	// Col is the image's first column, counted from the left edge.
	Col int

	// Sequence draws the image at the cursor.
	Sequence string
}

// DirectPlacements returns the placement of every preview image that is
// painted over the screen rather than drawn as view text, positioned relative
// to the top-left of this component's view.
//
// It returns nil in the usual case, where previews are either half-block art
// or Unicode placeholders and therefore need no separate placement.
//
// The offsets must match the layout built by View exactly; the two are kept
// in step by thumbRowOffsets.
func (s *InputComponent) DirectPlacements() []DirectPlacement {
	var out []DirectPlacement
	for i, off := range s.thumbRowOffsets() {
		if i < len(s.imagePlace) && s.imagePlace[i] != "" {
			out = append(out, DirectPlacement{
				RowOffset: off,
				Col:       thumbPaddingLeft,
				Sequence:  s.imagePlace[i],
			})
		}
	}
	return out
}

// thumbPaddingLeft is the left padding applied to each thumbnail by View. It
// is a constant so DirectPlacements and View cannot disagree about where a
// thumbnail starts.
const thumbPaddingLeft = 1

// thumbRowOffsets returns, for each pending image, the row its thumbnail
// starts on relative to the top of this component's view, or -1 when that
// image has no thumbnail yet.
//
// This mirrors the stacking order in View: the composer bar, then the "N
// image(s) attached" label, then one block per rendered thumbnail.
func (s *InputComponent) thumbRowOffsets() []int {
	if len(s.pendingImages) == 0 {
		return nil
	}
	// The composer bar, followed by the attachment label.
	row := lipgloss.Height(s.textarea.View()) + 1

	offsets := make([]int, len(s.pendingImages))
	for i := range s.pendingImages {
		if i >= len(s.imageThumbs) || s.imageThumbs[i] == "" {
			offsets[i] = -1
			continue
		}
		offsets[i] = row
		row += lipgloss.Height(s.imageThumbs[i])
	}
	return offsets
}

// View implements tea.Model. Renders the composer bar and any pending image
// attachments. The autocomplete popup is drawn separately as a centered
// overlay by AppModel.View().
func (s *InputComponent) View() tea.View {
	theme := style.GetTheme()

	// The composer is a filled bar rather than a bordered box. A background
	// fill reads as chrome, which is what the composer is; a border reads as
	// content, which is what messages are. Keeping the two distinct means the
	// input can never be mistaken for something the agent said.
	inputBarStyle := lipgloss.NewStyle().
		Background(theme.InputBg).
		Padding(0, 1).
		Width(s.width)

	var view strings.Builder
	view.WriteString(inputBarStyle.Render(s.textarea.View()))

	// Show image attachment previews when images are pending. A cached
	// half-block thumbnail is rendered when the terminal supports it;
	// otherwise the text pill alone is shown.
	if len(s.pendingImages) > 0 {
		imgStyle := lipgloss.NewStyle().
			Foreground(theme.VeryMuted).
			PaddingLeft(1)

		label := fmt.Sprintf("%d image(s) attached · ctrl+u to clear", len(s.pendingImages))
		view.WriteString("\n")
		view.WriteString(imgStyle.Render(label))

		thumbStyle := lipgloss.NewStyle().PaddingLeft(thumbPaddingLeft)
		for i := range s.pendingImages {
			if i < len(s.imageThumbs) && s.imageThumbs[i] != "" {
				view.WriteString("\n")
				view.WriteString(thumbStyle.Render(s.imageThumbs[i]))
			}
		}
	}

	return tea.NewView(view.String())
}

// RenderPopupBox renders the autocomplete popup for / or @ as a bare box.
// Returns "" when the popup is not currently shown. The caller composites it
// over the main view so the transcript stays visible around it.
//
// The actual filtering / selection state lives on InputComponent — this
// method merely converts the filtered FuzzyMatch list into PopupItems
// and asks the shared PopupList to draw it. As a result the / popup, the
// @ popup, the model picker, the tree selector and the session selector
// all share identical chrome.
func (s *InputComponent) RenderPopupBox(termWidth, termHeight int) string {
	if !s.showPopup || len(s.filtered) == 0 {
		return ""
	}

	items := make([]PopupItem, len(s.filtered))
	for i, m := range s.filtered {
		desc := ""
		if m.Command != nil {
			desc = m.Command.Description
		}
		name := ""
		if m.Command != nil {
			name = m.Command.Name
		}
		items[i] = PopupItem{
			Label:       name,
			Description: desc,
		}
	}
	s.popup.SetSize(termWidth, termHeight)
	s.popup.SetItems(items)
	s.popup.SetCursor(s.selected)
	return s.popup.Render()
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

// ClearPendingImages removes all pending image attachments and returns them,
// along with a command that frees any images transmitted to the terminal.
//
// The cleanup command must be run. A terminal holds transmitted image data
// until told to drop it, and these ids are discarded here, so a caller that
// ignores the command leaks every preview image for the rest of the session.
// It is nil when there is nothing to free.
func (s *InputComponent) ClearPendingImages() ([]core.ImageAttachment, tea.Cmd) {
	images := s.pendingImages
	s.pendingImages = nil
	cleanup := s.releaseImages()
	s.imageGen++
	return images, cleanup
}

// PendingImageCount returns the number of images currently attached.
func (s *InputComponent) PendingImageCount() int {
	return len(s.pendingImages)
}

// Clear clears the textarea content and resets related state. Returns true if
// there was content to clear, false if the input was already empty.
func (s *InputComponent) Clear() bool {
	hadContent := s.textarea.Value() != ""
	s.textarea.SetValue("")
	s.textarea.CursorEnd()
	s.lastValue = ""
	s.showPopup = false
	s.argMode = false
	s.fileMode = false
	s.fileEditMode = false
	s.browsingHistory = false
	s.savedInput = ""
	return hadContent
}

// applyFileCompletion replaces the @prefix in the textarea with the selected
// file or MCP resource suggestion. For directories, it keeps the popup open
// for further drilling. For files and resources, it closes the popup and adds
// a trailing space.
//
// When fileEditMode is active the same path-replacement happens against the
// /edit (or alias) command prefix instead of an @ trigger. Selecting a file
// also arms submitNext so the next tick runs $EDITOR on it; selecting a
// directory keeps the popup open for drill-down.
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

	var replacement string
	switch {
	case s.fileEditMode:
		// /edit path mode — no @ prefix; the path is the bare argument.
		// MCP resources are excluded upstream, so only file/dir entries reach here.
		needsQuote := strings.Contains(suggestion.RelPath, " ")
		if needsQuote {
			replacement = `"` + suggestion.RelPath + `"`
		} else {
			replacement = suggestion.RelPath
		}
	case suggestion.IsMCPResource:
		// MCP resources use @mcp:server:uri format.
		// Quote if the URI contains spaces.
		ref := "mcp:" + suggestion.MCPServerName + ":" + suggestion.MCPResourceURI
		if strings.Contains(ref, " ") {
			replacement = `@"` + ref + `"`
		} else {
			replacement = "@" + ref
		}
		replacement += " "
	default:
		needsQuote := strings.Contains(suggestion.RelPath, " ")
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
	}

	newLastLine := beforeAt + replacement

	// Reconstruct the full value with the updated last line.
	lines[len(lines)-1] = newLastLine
	newValue := strings.Join(lines, "\n")

	s.textarea.SetValue(newValue)
	s.textarea.CursorEnd()

	if suggestion.IsDir && !suggestion.IsMCPResource {
		// Keep popup open — trigger a refresh for the new directory.
		s.lastValue = "" // force re-evaluation on next update tick
		return
	}

	s.showPopup = false
	s.fileMode = false
	s.selected = 0

	if s.fileEditMode {
		// A file was selected via /edit — submit on the next tick so the
		// popup dismisses cleanly before $EDITOR takes the terminal.
		s.fileEditMode = false
		s.submitNext = true
	}
}

// updateEditFilePopup queries the file-suggestion engine for the /edit path
// prefix and populates the popup state. cmdLen is the byte offset of the path
// argument within the current line (i.e. length of "/edit " or "/ed ").
// Directories are kept so the user can drill down; MCP resources are skipped.
func (s *InputComponent) updateEditFilePopup(cmdLen int, pathPrefix string) {
	var suggestions []FileSuggestion
	if s.cwd != "" {
		suggestions = GetFileSuggestions(pathPrefix, s.cwd)
	}
	if len(suggestions) == 0 {
		s.showPopup = false
		s.fileMode = false
		s.fileEditMode = false
		return
	}

	sort.Slice(suggestions, func(i, j int) bool {
		return suggestions[i].Score > suggestions[j].Score
	})
	if len(suggestions) > maxFileSuggestions {
		suggestions = suggestions[:maxFileSuggestions]
	}

	s.showPopup = true
	s.fileMode = true
	s.fileEditMode = true
	s.argMode = false
	s.filePrefix = pathPrefix
	s.fileAtStartIdx = cmdLen
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
}

// applyComposerStyles paints every textarea style state with the composer
// background.
//
// lipgloss emits a reset sequence at the end of each styled span, which clears
// the background set by an enclosing container. Filling the bar therefore has
// to happen on the innermost styles rather than on the wrapper, or the result
// is a bar that flickers back to the terminal background wherever the textarea
// changed color (placeholder, text, cursor line).
func applyComposerStyles(styles *textarea.Styles, theme style.Theme) {
	bg := lipgloss.NewStyle().Background(theme.InputBg)

	for _, state := range []*textarea.StyleState{&styles.Focused, &styles.Blurred} {
		state.Base = bg
		state.Text = bg.Foreground(theme.Text)
		state.Placeholder = bg.Foreground(theme.VeryMuted)
		state.Prompt = bg
		state.CursorLine = bg
		state.EndOfBuffer = bg
	}
}

// UpdateTheme repaints the composer with colors from the current theme.
// Called after /theme switches the active palette.
func (s *InputComponent) UpdateTheme() {
	styles := s.textarea.Styles()
	applyComposerStyles(&styles, style.GetTheme())
	s.textarea.SetStyles(styles)
}

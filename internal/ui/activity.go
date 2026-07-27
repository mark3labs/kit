package ui

import (
	"encoding/json"
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/mark3labs/kit/internal/ui/style"
)

// The activity row is the single live-status line rendered directly above the
// composer. It answers three questions at a glance: what is the agent doing
// right now, how long has it been doing it, and how do I stop it.
//
// Two rules keep it readable:
//
//  1. The activity row is the only place present-tense work appears. The
//     transcript records finished work in the past tense, so scrollback never
//     looks as though it is still running.
//  2. Ambient facts (model, token count, cost, working directory) belong in
//     the status bar, not here. They must never compete with live activity for
//     horizontal space.

// activityMaxTarget caps the length of the target fragment in an activity
// phrase (the command, path or pattern) so a long argument cannot push the
// elapsed time and interrupt hint off the row.
const activityMaxTarget = 56

// activityVerb returns a present-tense phrase describing an in-flight tool
// call, e.g. "Running go test ./..." or "Reading internal/ui/model.go".
// toolArgs is the raw JSON argument payload; malformed JSON degrades to the
// bare tool name rather than erroring.
func activityVerb(toolName, toolArgs string) string {
	args := parseToolArgs(toolArgs)

	// verb is the present-tense action; target is the thing being acted on.
	// Keeping them separate lets a missing target degrade to a bare verb
	// instead of leaving a dangling "Reading " with nothing after it.
	var verb, target string

	switch strings.ToLower(toolName) {
	case "bash":
		verb, target = "Running", shortenTarget(oneLine(argString(args, "command")))
	case "read":
		verb, target = "Reading", shortenActivityPath(argString(args, "path"))
	case "write":
		verb, target = "Writing", shortenActivityPath(argString(args, "path"))
	case "edit":
		verb, target = "Editing", shortenActivityPath(argString(args, "path"))
	case "grep":
		verb, target = "Searching", shortenTarget(oneLine(argString(args, "pattern")))
	case "find":
		verb, target = "Matching", shortenTarget(oneLine(argString(args, "pattern")))
	case "ls":
		verb, target = "Listing", shortenActivityPath(argString(args, "path"))
	case "fetch":
		verb, target = "Fetching", shortenTarget(oneLine(argString(args, "url")))
	case "subagent":
		if agent := argString(args, "agent"); agent != "" {
			return "Delegating to " + agent
		}
		verb, target = "Delegating", shortenTarget(oneLine(argString(args, "task")))
	case "todo":
		return "Updating todos"
	default:
		// Unknown tool: title-case the name and attach the first string
		// argument if one is available, so third-party tools still read as
		// activity rather than as a bare identifier.
		verb, target = toolDisplayName(toolName), shortenTarget(oneLine(firstStringArg(args)))
	}

	if target == "" {
		// No usable argument (absent, empty or malformed JSON). Fall back to
		// the tool's display name so the row still says something true.
		return toolDisplayName(toolName)
	}
	return verb + " " + target
}

// parseToolArgs decodes a raw JSON tool-argument payload. A nil map is
// returned for empty or malformed input; callers treat that as "no arguments"
// rather than as an error, because the activity row must never fail to render.
func parseToolArgs(toolArgs string) map[string]any {
	if strings.TrimSpace(toolArgs) == "" {
		return nil
	}
	var args map[string]any
	if err := json.Unmarshal([]byte(toolArgs), &args); err != nil {
		return nil
	}
	return args
}

// argString returns the named argument as a string, or "" when absent or not
// a string.
func argString(args map[string]any, key string) string {
	if args == nil {
		return ""
	}
	if v, ok := args[key].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

// firstStringArg returns any string-valued argument, preferring conventional
// target keys before falling back to an arbitrary one.
func firstStringArg(args map[string]any) string {
	for _, key := range []string{"command", "path", "file_path", "pattern", "query", "url", "task"} {
		if v := argString(args, key); v != "" {
			return v
		}
	}
	// The activity row re-renders on every spinner tick, so the fallback must
	// be stable: Go randomizes map iteration order, and picking a different
	// string each frame would make the row flicker between arguments. Sort the
	// keys so an unknown tool with several string args always shows the same
	// one.
	keys := make([]string, 0, len(args))
	for k := range args {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		if s, ok := args[k].(string); ok && strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

// oneLine collapses a multi-line string into a single line so it cannot break
// the activity row's layout.
func oneLine(s string) string {
	s = strings.ReplaceAll(s, "\r\n", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\t", " ")
	return strings.Join(strings.Fields(s), " ")
}

// shortenTarget truncates a target fragment to activityMaxTarget columns,
// marking elision with a single-column ellipsis.
func shortenTarget(s string) string {
	return truncateLine(s, activityMaxTarget)
}

// shortenActivityPath keeps a path readable inside the activity row by dropping
// leading directories once the full path exceeds the target budget. The last
// two segments are retained because they carry the most identifying detail.
func shortenActivityPath(p string) string {
	if p == "" {
		return ""
	}
	if len([]rune(p)) <= activityMaxTarget {
		return p
	}
	dir, file := filepath.Split(p)
	parent := filepath.Base(strings.TrimSuffix(dir, string(filepath.Separator)))
	if parent == "." || parent == string(filepath.Separator) || parent == "" {
		return "…/" + truncateLine(file, activityMaxTarget-2)
	}
	return "…/" + truncateLine(parent+string(filepath.Separator)+file, activityMaxTarget-2)
}

// formatElapsed renders a duration for the activity row. Sub-second values
// collapse to "<1s" so the row does not flicker through fractional updates.
func formatElapsed(d time.Duration) string {
	switch {
	case d <= 0:
		return ""
	case d < time.Second:
		return "<1s"
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	default:
		return fmt.Sprintf("%dm%02ds", int(d/time.Minute), int(d/time.Second)%60)
	}
}

// renderActivityRow draws the live status line shown above the composer while
// the agent is working. It returns "" when the agent is idle, so the row costs
// no vertical space at rest.
//
// Layout, widest to narrowest:
//
//	● Running go test ./... · 12s                          esc interrupt
//	● Running go test ./... · 12s                                    esc
//	● Running go test ./... · 12s
func (m *AppModel) renderActivityRow() string {
	if m.state != stateWorking || m.stream == nil {
		return ""
	}

	theme := style.GetTheme()

	// The pulsing dot doubles as the liveness indicator: it is always present
	// while working, so a slow provider reads as active rather than stalled.
	dot := m.stream.ActivityDot()
	phrase := m.stream.ActivityPhrase()

	var elapsed string
	if !m.turnStartedAt.IsZero() {
		elapsed = formatElapsed(time.Since(m.turnStartedAt))
	}

	dim := lipgloss.NewStyle().Foreground(theme.VeryMuted)
	// The phrase is bold with no explicit foreground so it inherits the
	// terminal's own text color and stays legible under any color scheme.
	label := lipgloss.NewStyle().Bold(true)

	prefix := " " + dot + " "
	suffix := ""
	if elapsed != "" {
		suffix = dim.Render(" · " + elapsed)
	}

	hint := m.activityHint()
	hintWidth := 0
	if hint != "" {
		hintWidth = lipgloss.Width(hint) + activityHintGap
	}

	// Reserve room for the phrase before spending any on the hint, so the
	// most important text is the last thing to be truncated.
	phraseBudget := m.width - lipgloss.Width(prefix) - lipgloss.Width(suffix) - hintWidth
	if phraseBudget < activityMinPhrase && hint != "" {
		hint = ""
		phraseBudget = m.width - lipgloss.Width(prefix) - lipgloss.Width(suffix)
	}
	if phraseBudget < 1 {
		phraseBudget = 1
	}

	left := prefix + label.Render(truncateLine(phrase, phraseBudget)) + suffix
	if hint == "" {
		return left
	}

	gap := max(m.width-lipgloss.Width(left)-lipgloss.Width(hint), activityHintGap)
	return left + strings.Repeat(" ", gap) + dim.Render(hint)
}

// activityMinPhrase is the smallest phrase width worth preserving. Below this
// the interrupt hint is dropped so the activity text stays meaningful.
const activityMinPhrase = 24

// activityHintGap is the minimum blank gutter between the activity text and
// the right-aligned key hint.
const activityHintGap = 3

// activityHint returns the right-aligned key hint for the current state,
// degrading from a full hint to a compact one as width allows. The caller
// decides whether there is room to show it at all.
func (m *AppModel) activityHint() string {
	// A pending confirmation displaces the ordinary hint: it is the only thing
	// the next keystroke can do, so offering alternatives would be misleading.
	if m.ctrlCPressedOnce {
		return "ctrl+c again to quit"
	}
	if m.canceling {
		return "esc again to cancel"
	}
	full := "↵ queue · esc interrupt"
	compact := "esc interrupt"

	available := m.width - activityMinPhrase - activityHintGap
	if lipgloss.Width(full) <= available {
		return full
	}
	if lipgloss.Width(compact) <= available {
		return compact
	}
	return ""
}

// detectGitBranch returns the current branch name for the repository
// containing dir, or "" when dir is not inside a git worktree.
//
// This reads .git directly rather than shelling out to git: the status bar is
// built during startup and a subprocess spawn is both slower and a needless
// dependency on git being installed. Detached HEADs report the short commit
// hash, matching what a user would see in their prompt.
func detectGitBranch(dir string) string {
	if dir == "" {
		return ""
	}
	for {
		head, err := os.ReadFile(filepath.Join(dir, ".git", "HEAD"))
		if err == nil {
			ref := strings.TrimSpace(string(head))
			if branch, ok := strings.CutPrefix(ref, "ref: refs/heads/"); ok {
				return branch
			}
			// Detached HEAD: HEAD holds a raw commit hash.
			if len(ref) >= 7 {
				return ref[:7]
			}
			return ""
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// turnOutcome describes how an agent turn ended, which decides the receipt's
// glyph and label.
type turnOutcome int

const (
	turnDone turnOutcome = iota
	turnCancelled
	turnFailed
)

// printTurnReceipt appends a one-line summary of the turn that just ended:
//
//	✓ Done · 3 tools · 12s
//
// This is the only place a green check appears. Individual tool calls are
// marked with a dim ·, so the check retains its meaning as "the thing you
// asked for is finished" rather than "another step happened".
//
// The receipt is suppressed for turns that did no measurable work — a bare
// text answer already ends visibly, and a receipt under every reply would be
// noise.
func (m *AppModel) printTurnReceipt(outcome turnOutcome) {
	elapsed := time.Duration(0)
	if !m.turnStartedAt.IsZero() {
		elapsed = time.Since(m.turnStartedAt)
	}

	// Nothing worth reporting: no tools ran and the turn was quick.
	if outcome == turnDone && m.turnToolCount == 0 && elapsed < turnReceiptMinDuration {
		return
	}

	theme := style.GetTheme()

	var glyph, label string
	var glyphColor color.Color
	switch outcome {
	case turnCancelled:
		glyph, label, glyphColor = "×", "Interrupted", theme.Warning
	case turnFailed:
		glyph, label, glyphColor = "×", "Failed", theme.Error
	default:
		glyph, label, glyphColor = "✓", "Done", theme.Success
	}

	dim := lipgloss.NewStyle().Foreground(theme.VeryMuted)
	parts := []string{lipgloss.NewStyle().Bold(true).Render(label)}
	if m.turnToolCount > 0 {
		parts = append(parts, dim.Render(pluralize(m.turnToolCount, "tool")))
	}
	if e := formatElapsed(elapsed); e != "" {
		parts = append(parts, dim.Render(e))
	}

	line := lipgloss.NewStyle().Foreground(glyphColor).Render(glyph) + " " +
		strings.Join(parts, dim.Render(" · "))
	line = styleMarginBottom1.Render(line)

	m.messages = append(m.messages, NewStyledMessageItem(generateMessageID(), "system", line, line))
	m.refreshContent()
}

// turnReceiptMinDuration is the shortest turn worth acknowledging when no
// tools ran. Below this the answer arrived fast enough that a receipt would
// say less than the answer itself.
const turnReceiptMinDuration = 10 * time.Second

// pluralize renders a count with its noun, adding a trailing "s" for any
// count other than one.
func pluralize(n int, noun string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, noun)
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

// syncInputHeight marks the layout dirty when the composer's rendered height
// has changed since it was last measured.
//
// The composer grows and shrinks with its content, so a keystroke that adds or
// removes a wrapped line changes how much room is left for the transcript.
// Without this the transcript keeps its old height and the joined view either
// overflows the terminal (clipping the status bar) or leaves a gap. Layout is
// not recomputed here — that happens once per frame in View() — this only
// records that it needs to be.
func (m *AppModel) syncInputHeight() {
	if m.input == nil || m.layoutDirty {
		return
	}
	rendered := m.renderInput()
	if rendered == "" {
		return
	}
	if lipgloss.Height(rendered) != m.lastInputHeight {
		m.layoutDirty = true
	}
}

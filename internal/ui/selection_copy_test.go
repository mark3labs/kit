package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/mark3labs/kit/internal/ui/clipboard"
)

// selectionModel returns a test model with five two-line items and a mouse
// selection from the first line into the second.
func selectionModel(t *testing.T) *AppModel {
	t.Helper()
	m, _, _ := newTestAppModel(&stubAppController{})
	m.messages = makeItems(5, 2)
	m.refreshContent()
	if !m.scrollList.HandleMouseDown(0, 0) || !m.scrollList.HandleMouseDrag(10, 1) {
		t.Fatal("could not start a selection in the scrollback")
	}
	return m
}

// After a copy, the selection stays highlighted and the status bar says how
// much was copied.
func TestMouseSelectionStaysAfterCopy(t *testing.T) {
	m := selectionModel(t)
	text := m.scrollList.ExtractSelectedText()

	updated, cmd := m.Update(tea.MouseReleaseMsg{Button: tea.MouseLeft})
	m = updated.(*AppModel)
	if cmd == nil {
		t.Fatal("the release returned no command; want the copy")
	}
	if !m.scrollList.HasSelection() {
		t.Error("the release cleared the selection; want it kept after the copy")
	}
	if got, want := m.scrollList.CopiedChars(), len([]rune(text)); got != want {
		t.Errorf("CopiedChars() = %d; want %d", got, want)
	}
	if bar := stripAnsi(m.renderStatusBar()); !strings.Contains(bar, "Copied ") {
		t.Errorf("status bar = %q; want the copy acknowledgement", bar)
	}
}

// The next key, paste or click ends a kept selection, and the status bar
// notice with it.
func TestNextInputEndsKeptSelection(t *testing.T) {
	for _, tc := range []struct {
		name string
		msg  tea.Msg
	}{
		{"key", keyPress("a")},
		{"paste", tea.PasteMsg{Content: "a"}},
		{"click outside the conversation", tea.MouseClickMsg{Y: 1000, Button: tea.MouseLeft}},
		{"right click", tea.MouseClickMsg{Button: tea.MouseRight}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := sendMsg(selectionModel(t), tea.MouseReleaseMsg{Button: tea.MouseLeft})
			if m.scrollList.CopiedChars() == 0 {
				t.Fatal("no kept selection after the copy")
			}
			m = sendMsg(m, tc.msg)
			if m.scrollList.HasSelection() || m.scrollList.CopiedChars() != 0 {
				t.Errorf("selection %v, %d chars copied; want neither",
					m.scrollList.HasSelection(), m.scrollList.CopiedChars())
			}
		})
	}
}

// sentenceError is an error whose text ends with a period, as the one from
// atotto/clipboard does when no clipboard tool is installed.
type sentenceError struct{}

func (sentenceError) Error() string { return "no clipboard tool." }

// noopMsg is a message that no part of the model handles.
type noopMsg struct{}

// A failed write to the system clipboard is reported once, and only when
// the scrollback is quiet: added while a reply streams, the report would
// split the reply.
func TestNativeCopyFailureReportedOnce(t *testing.T) {
	m, _, _ := newTestAppModel(&stubAppController{})
	fail := clipboard.NativeCopyFailedMsg{Err: sentenceError{}}

	m.state = stateWorking
	m = sendMsg(m, fail)
	if len(m.messages) != 0 {
		t.Fatal("the failure was reported while the agent was working")
	}

	// The composer has focus, but the reply at the end still streams.
	reply := NewStreamingMessageItem("reply", "assistant", "test-model")
	m.messages = append(m.messages, reply)
	m.state = stateInput
	m = sendMsg(m, noopMsg{})
	if len(m.messages) != 1 {
		t.Fatal("the failure was reported while the reply was streaming")
	}

	reply.MarkComplete()
	m = sendMsg(m, noopMsg{})
	m = sendMsg(m, fail)
	if len(m.messages) != 2 {
		t.Fatalf("got %d messages; want the reply and one report", len(m.messages))
	}
	item, ok := m.messages[1].(*TextMessageItem)
	if !ok {
		t.Fatalf("the report is a %T; want a system message", m.messages[1])
	}
	if text := item.RawContent(); !strings.Contains(text, "Could not write the system clipboard (no clipboard tool).") {
		t.Errorf("report = %q; want the reason without a doubled period", text)
	}
}

// A mouse copy that missed the system clipboard says so in the status bar,
// where the user looks after the copy.
func TestFailedCopyShownInStatusBar(t *testing.T) {
	m := sendMsg(selectionModel(t), tea.MouseReleaseMsg{Button: tea.MouseLeft})
	m = sendMsg(m, clipboard.NativeCopyFailedMsg{Err: sentenceError{}})
	if bar := stripAnsi(m.renderStatusBar()); !strings.Contains(bar, "system clipboard failed") {
		t.Errorf("status bar = %q; want the failed copy", bar)
	}
}

// On a narrow terminal, the copy notice outlasts the model label: it answers
// something the user just did.
func TestCopyNoticeOutlastsModelLabel(t *testing.T) {
	m := sendMsg(selectionModel(t), tea.MouseReleaseMsg{Button: tea.MouseLeft})
	m.providerName, m.modelName = "anthropic", "claude-opus-5-5"
	m.width = 50
	bar := stripAnsi(m.renderStatusBar())
	if !strings.Contains(bar, "Copied ") || strings.Contains(bar, "claude-opus-5-5") {
		t.Errorf("status bar = %q; want the copy notice without the model label", bar)
	}

	// Where even the notice alone does not fit, the model label stays.
	m.width = 30
	if bar := stripAnsi(m.renderStatusBar()); !strings.Contains(bar, "claude-opus-5-5") {
		t.Errorf("status bar = %q; want the model label", bar)
	}
}

// In message navigation, the copy notice also says when the copy missed the
// system clipboard.
func TestFailedNavCopyShownInStatusBar(t *testing.T) {
	m := navRoleModel(t)
	m.enterMessageNav()
	if cmd := m.copySelectedMessage(); cmd == nil {
		t.Fatal("copy produced no clipboard command")
	}
	updated, _ := m.Update(clipboard.NativeCopyFailedMsg{Err: sentenceError{}})
	m = updated.(*AppModel)
	if bar := stripAnsi(m.renderStatusBar()); !strings.Contains(bar, "system clipboard failed") {
		t.Errorf("status bar = %q; want the failed copy", bar)
	}
}

// textItem is a MessageItem whose text a test can change in place.
type textItem struct{ id, text string }

func (it *textItem) ID() string        { return it.id }
func (it *textItem) Render(int) string { return it.text }
func (it *textItem) Height() int       { return strings.Count(it.text, "\n") + 1 }

// A kept selection marks the copied text by position. It stays while that
// text stays in place, and it goes when the text under it changes.
func TestScrollList_KeptSelectionFollowsText(t *testing.T) {
	a, b := &textItem{"a", "alpha beta"}, &textItem{"b", "gamma"}
	sl := NewScrollList(80, 30)
	sl.SetItems([]MessageItem{a, b})
	keep := func() {
		t.Helper()
		sl.ClearSelection()
		if !sl.HandleMouseDown(0, 0) || !sl.HandleMouseDrag(5, 0) || !sl.HandleMouseUp() {
			t.Fatal("could not select the first word")
		}
		sl.KeepSelection(sl.ExtractSelectedText())
	}
	kept := func(after string, want bool) {
		t.Helper()
		if got := sl.CopiedChars() > 0 && sl.HasSelection(); got != want {
			t.Errorf("after %s: selection kept = %v; want %v", after, got, want)
		}
	}

	keep()
	sl.SetItems([]MessageItem{a, b, &textItem{"c", "delta"}})
	kept("a new item below", true)
	b.text = "epsilon"
	sl.InvalidateItemHeight("b")
	kept("a change below", true)
	a.text = "omega beta"
	sl.InvalidateItemHeight("a")
	kept("a change under the selection", false)

	a.text = "alpha beta"
	keep()
	sl.SetItems([]MessageItem{&textItem{"z", "zeta"}, a, b})
	kept("an item inserted above", false)

	sl.SetItems([]MessageItem{a, b})
	keep()
	sl.SetItems([]MessageItem{b})
	kept("a shorter list", false)
}

// The click that ends a kept selection still counts toward a triple click:
// after a double click copied a word, the third click selects the line.
func TestScrollList_TripleClickAfterKeptWord(t *testing.T) {
	sl := NewScrollList(80, 30)
	sl.SetItems([]MessageItem{&textItem{"a", "alpha beta"}})
	click := func() string {
		t.Helper()
		sl.EndKeptSelection()
		if !sl.HandleMouseDown(1, 0) {
			t.Fatal("the click missed the text")
		}
		if !sl.HandleMouseUp() {
			return ""
		}
		text := sl.ExtractSelectedText()
		sl.KeepSelection(text)
		return text
	}

	click()
	if word := click(); word != "alpha" {
		t.Fatalf("the double click selected %q; want the word", word)
	}
	if line := click(); line != "alpha beta" {
		t.Errorf("the triple click selected %q; want the line", line)
	}
}

// After a drag copy, the next click is a single click even when it comes
// quickly: only a copied double click passes its click history on.
func TestScrollList_ClickAfterKeptDrag(t *testing.T) {
	sl := NewScrollList(80, 30)
	sl.SetItems([]MessageItem{&textItem{"a", "alpha beta"}})
	if !sl.HandleMouseDown(1, 0) || !sl.HandleMouseDrag(8, 0) || !sl.HandleMouseUp() {
		t.Fatal("could not drag a selection")
	}
	sl.KeepSelection(sl.ExtractSelectedText())

	sl.EndKeptSelection()
	sl.HandleMouseDown(1, 0)
	if sl.HandleMouseUp() {
		t.Errorf("the click after the drag copy selected %q; want a single click",
			sl.ExtractSelectedText())
	}
}

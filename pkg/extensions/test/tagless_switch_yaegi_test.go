package test

import (
	"strings"
	"testing"

	"github.com/mark3labs/kit/internal/extensions"
)

// TestTaglessSwitchCommaCaseList documents a live Yaegi miscompilation.
//
// In a tagless switch — `switch { case a, b, c: }` — Yaegi evaluates only the
// FIRST expression of a comma-separated case list and silently ignores the
// rest. There is no load error, no panic, and no warning: the branch simply
// does not fire for inputs that should have matched b or c.
//
// This is a nasty failure mode for extensions, because the idiom is
// idiomatic Go and is commonly used for character-class tests and rune-width
// tables. Two example extensions shipped with Kit were wrong because of it:
// popup-terminal.go dropped every digit and uppercase letter from tmux
// session names, and status-footer.go counted CJK and hangul as one column
// when the whole point of the function was to count them as two.
//
// The workaround is to join the conditions with "||" into a single case
// expression, or to use if/else. Both evaluate correctly.
//
// If this test starts FAILING, Yaegi has been fixed. That is good news:
// delete the workarounds and this test.
func TestTaglessSwitchCommaCaseList(t *testing.T) {
	src := `package main

import (
	"fmt"

	"kit/ext"
)

// commaList uses the buggy comma-separated case list.
func commaList(n int) string {
	switch {
	case n == 1, n == 2, n == 3:
		return "hit"
	default:
		return "miss"
	}
}

// orJoined is the workaround: one expression joined with "||".
func orJoined(n int) string {
	switch {
	case n == 1 || n == 2 || n == 3:
		return "hit"
	default:
		return "miss"
	}
}

// valueSwitch is a switch WITH a tag, which is unaffected.
func valueSwitch(n int) string {
	switch n {
	case 1, 2, 3:
		return "hit"
	default:
		return "miss"
	}
}

func Init(api ext.API) {
	api.OnSessionStart(func(e ext.SessionStartEvent, ctx ext.Context) {
		for _, n := range []int{1, 2, 3, 9} {
			ctx.Print(fmt.Sprintf("comma %d=%s", n, commaList(n)))
			ctx.Print(fmt.Sprintf("or %d=%s", n, orJoined(n)))
			ctx.Print(fmt.Sprintf("value %d=%s", n, valueSwitch(n)))
		}
	})
}
`

	h := New(t)
	h.LoadString(src, "tagless-switch.go")
	if _, err := h.Emit(extensions.SessionStartEvent{SessionID: "s"}); err != nil {
		t.Fatalf("SessionStart: %v", err)
	}
	got := strings.Join(h.Context().Prints, "\n")

	has := func(s string) bool { return strings.Contains(got, s) }

	// The workaround must behave like real Go.
	for _, want := range []string{"or 1=hit", "or 2=hit", "or 3=hit", "or 9=miss"} {
		if !has(want) {
			t.Errorf("|| workaround is broken: missing %q in:\n%s", want, got)
		}
	}

	// A value switch must behave like real Go.
	for _, want := range []string{"value 1=hit", "value 2=hit", "value 3=hit", "value 9=miss"} {
		if !has(want) {
			t.Errorf("value switch is broken: missing %q in:\n%s", want, got)
		}
	}

	// The first expression works even in the buggy form.
	if !has("comma 1=hit") {
		t.Errorf("expected the first case expression to match, got:\n%s", got)
	}
	if !has("comma 9=miss") {
		t.Errorf("expected a non-matching value to fall through, got:\n%s", got)
	}

	// The bug: the second and third expressions are ignored. Assert the
	// broken behaviour so the day Yaegi fixes it, this test tells us.
	if has("comma 2=hit") && has("comma 3=hit") {
		t.Fatal("Yaegi now evaluates every expression in a tagless comma case list.\n" +
			"The bug is fixed — remove the \"||\" workarounds in examples/extensions/ " +
			"(popup-terminal.go, status-footer.go), drop the warning from AGENTS.md " +
			"and the kit-extensions skill, and delete this test.")
	}
	t.Logf("Yaegi tagless-switch bug still present, as expected:\n%s", got)
}

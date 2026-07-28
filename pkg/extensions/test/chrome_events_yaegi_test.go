package test

import (
	"testing"

	"github.com/mark3labs/kit/internal/extensions"
)

// TestNewEventsCrossYaegiBoundary verifies the three chrome-oriented events
// added for status-line extensions — terminal resize, thinking level, and turn
// state — survive the interpreter with their field values intact.
//
// Yaegi silently yields zero values for types missing from symbols.go rather
// than failing to load, so these assert on real field data; a test that only
// checked the extension loads would pass against a broken registration.
func TestNewEventsCrossYaegiBoundary(t *testing.T) {
	src := `package main

import (
	"fmt"

	"kit/ext"
)

func Init(api ext.API) {
	api.OnTerminalResize(func(e ext.TerminalResizeEvent, ctx ext.Context) {
		w, h := ctx.GetTerminalSize()
		ctx.Print(fmt.Sprintf("resize w=%d h=%d ctxW=%d ctxH=%d",
			e.Width, e.Height, w, h))
	})

	api.OnThinkingLevelChange(func(e ext.ThinkingLevelChangeEvent, ctx ext.Context) {
		ctx.Print(fmt.Sprintf("thinking %s->%s src=%s live=%s",
			e.PreviousLevel, e.NewLevel, e.Source, ctx.GetThinkingLevel()))
	})

	api.OnTurnStateChange(func(e ext.TurnStateChangeEvent, ctx ext.Context) {
		ctx.Print(fmt.Sprintf("turn %s->%s", e.Previous, e.State))
	})
}
`

	harness := New(t)
	harness.LoadString(src, "events.go")

	ctx := harness.Context().ToContext()
	ctx.GetTerminalSize = func() (int, int) { return 120, 40 }
	ctx.GetThinkingLevel = func() string { return "high" }
	harness.Runner().SetContext(ctx)

	emit := func(e extensions.Event) {
		t.Helper()
		if _, err := harness.Emit(e); err != nil {
			t.Fatalf("Emit(%T) error = %v", e, err)
		}
	}

	emit(extensions.TerminalResizeEvent{Width: 120, Height: 40})
	emit(extensions.ThinkingLevelChangeEvent{
		NewLevel: "high", PreviousLevel: "medium", Source: "user",
	})
	emit(extensions.TurnStateChangeEvent{State: "working", Previous: "idle"})

	want := []string{
		"resize w=120 h=40 ctxW=120 ctxH=40",
		"thinking medium->high src=user live=high",
		"turn idle->working",
	}

	got := harness.Context().GetPrints()
	if len(got) != len(want) {
		t.Fatalf("got %d prints (%v); want %d", len(got), got, len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("print[%d]:\n got %q\nwant %q", i, got[i], want[i])
		}
	}
}

// TestThinkingLevelDefaultsOffWhenUnwired verifies the runner's no-op stub for
// GetThinkingLevel returns a valid level rather than an empty string. Headless
// and ACP contexts do not wire it, and an extension formatting the result must
// not render a blank.
func TestThinkingLevelDefaultsOffWhenUnwired(t *testing.T) {
	src := `package main

import "kit/ext"

func Init(api ext.API) {
	api.OnSessionStart(func(_ ext.SessionStartEvent, ctx ext.Context) {
		ctx.Print("level=" + ctx.GetThinkingLevel())
	})
}
`

	harness := New(t)
	harness.LoadString(src, "unwired.go")

	// Deliberately leave GetThinkingLevel nil; SetContext normalizes it.
	ctx := harness.Context().ToContext()
	ctx.GetThinkingLevel = nil
	harness.Runner().SetContext(ctx)

	if _, err := harness.Emit(extensions.SessionStartEvent{SessionID: "t"}); err != nil {
		t.Fatalf("Emit error = %v", err)
	}

	got := harness.Context().GetPrints()
	if len(got) != 1 || got[0] != "level=off" {
		t.Errorf("got %v; want [level=off]", got)
	}
}

// TestTerminalSizeIsLiveNotSnapshot guards the reason GetTerminalSize is a
// function rather than a pair of Context fields.
//
// Extensions commonly capture a Context in a long-lived goroutine (a ticking
// clock in a footer, say). With struct fields that goroutine would keep
// reporting the size copied when its handler was first invoked, and any
// width-sensitive output would wrap after the first resize. Reading through a
// function keeps captured contexts correct.
func TestTerminalSizeIsLiveNotSnapshot(t *testing.T) {
	src := `package main

import (
	"fmt"

	"kit/ext"
)

// captured mimics an extension stashing the Context for later use.
var captured ext.Context

func Init(api ext.API) {
	api.OnSessionStart(func(_ ext.SessionStartEvent, ctx ext.Context) {
		captured = ctx
	})

	api.OnTurnStateChange(func(_ ext.TurnStateChangeEvent, ctx ext.Context) {
		w, _ := captured.GetTerminalSize()
		ctx.Print(fmt.Sprintf("captured sees w=%d", w))
	})
}
`

	harness := New(t)
	harness.LoadString(src, "live.go")

	width := 100
	ctx := harness.Context().ToContext()
	ctx.GetTerminalSize = func() (int, int) { return width, 24 }
	harness.Runner().SetContext(ctx)

	if _, err := harness.Emit(extensions.SessionStartEvent{SessionID: "t"}); err != nil {
		t.Fatalf("Emit(SessionStart) error = %v", err)
	}

	// Resize after the Context was captured.
	width = 60

	if _, err := harness.Emit(extensions.TurnStateChangeEvent{State: "working"}); err != nil {
		t.Fatalf("Emit(TurnStateChange) error = %v", err)
	}

	got := harness.Context().GetPrints()
	if len(got) != 1 || got[0] != "captured sees w=60" {
		t.Errorf("got %v; want [captured sees w=60] — a captured Context must observe resizes", got)
	}
}

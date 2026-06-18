package kit

import (
	"context"
	"testing"
)

func TestHaltHolderFirstWins(t *testing.T) {
	h := &haltHolder{}
	if halted, _, _ := h.snapshot(); halted {
		t.Fatal("new holder should not be halted")
	}
	h.set("finish", 42)
	h.set("other", 99) // ignored — first halt wins
	halted, name, val := h.snapshot()
	if !halted {
		t.Fatal("holder should be halted")
	}
	if name != "finish" {
		t.Fatalf("toolName = %q, want finish", name)
	}
	if v, ok := val.(int); !ok || v != 42 {
		t.Fatalf("value = %#v, want 42", val)
	}
}

func TestRecordHalt(t *testing.T) {
	holder := &haltHolder{}
	ctx := context.WithValue(context.Background(), haltHolderKey{}, holder)

	// Non-halting output records nothing.
	recordHalt(ctx, "noop", ToolOutput{Content: "ok"})
	if halted, _, _ := holder.snapshot(); halted {
		t.Fatal("non-halting output should not halt")
	}

	recordHalt(ctx, "finish", ToolOutput{Halt: true, FinalValue: "done"})
	halted, name, val := holder.snapshot()
	if !halted || name != "finish" || val != "done" {
		t.Fatalf("halt not recorded: halted=%v name=%q val=%v", halted, name, val)
	}

	// Missing holder in context is a safe no-op.
	recordHalt(context.Background(), "finish", ToolOutput{Halt: true})
}

func TestStreamCollector(t *testing.T) {
	c := &streamCollector{}
	if c.drain() != nil {
		t.Fatal("empty collector should drain to nil")
	}
	c.add(StreamEvent{Kind: StreamEventTextDelta, Text: "A"})
	c.add(StreamEvent{Kind: StreamEventTextDelta, Text: "B"})
	c.add(StreamEvent{Kind: StreamEventToolCallChunk, ToolName: "x"})

	out := c.drain()
	if len(out) != 3 {
		t.Fatalf("len = %d, want 3", len(out))
	}
	if out[0].Text != "A" || out[1].Text != "B" {
		t.Fatalf("order not preserved: %#v", out)
	}
	if out[2].Kind != StreamEventToolCallChunk || out[2].ToolName != "x" {
		t.Fatalf("tool chunk wrong: %#v", out[2])
	}
}

// nil receiver collector (no per-turn collector attached) must be safe.
func TestStreamCollectorNil(t *testing.T) {
	var c *streamCollector
	c.add(StreamEvent{Kind: StreamEventTextDelta, Text: "x"}) // no panic
	if c.drain() != nil {
		t.Fatal("nil collector should drain to nil")
	}
}

func TestStreamCollectorFromContext(t *testing.T) {
	if streamCollectorFromContext(context.Background()) != nil {
		t.Fatal("expected nil collector for bare context")
	}
	c := &streamCollector{}
	ctx := context.WithValue(context.Background(), streamCollectorKey{}, c)
	if streamCollectorFromContext(ctx) != c {
		t.Fatal("collector not retrieved from context")
	}
}

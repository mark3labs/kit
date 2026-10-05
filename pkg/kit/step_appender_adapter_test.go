package kit

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"

	"charm.land/fantasy"

	"github.com/mark3labs/kit/internal/session"
)

// The default backend must present the StepAppender contract end to end:
// the adapter type-asserts as a StepAppender, and a step reaches the file as
// two lines that the next open can read back (and repair if the process died
// between the tool call and its result).

func TestTreeManagerAdapterIsStepAppender(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	tm, err := session.CreateTreeSession(t.TempDir())
	if err != nil {
		t.Fatalf("CreateTreeSession: %v", err)
	}
	defer func() { _ = tm.Close() }()

	if _, ok := NewTreeManagerAdapter(tm).(StepAppender); !ok {
		t.Fatalf("treeManagerAdapter does not satisfy StepAppender; kit would fall back to per-message appends")
	}
}

func TestTreeManagerAdapterAppendStepPersistsBothLines(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	tm, err := session.CreateTreeSession(t.TempDir())
	if err != nil {
		t.Fatalf("CreateTreeSession: %v", err)
	}
	path := tm.GetFilePath()

	sm := NewTreeManagerAdapter(tm)
	_, err = appendMessages(context.Background(), sm, []LLMMessage{
		{
			Role: fantasy.MessageRoleAssistant,
			Content: []fantasy.MessagePart{
				fantasy.ToolCallPart{ToolCallID: "call-1", ToolName: "bash", Input: "{}"},
			},
		},
		{
			Role: fantasy.MessageRoleTool,
			Content: []fantasy.MessagePart{
				fantasy.ToolResultPart{
					ToolCallID: "call-1",
					Output:     fantasy.ToolResultOutputContentText{Text: "ok"},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("append messages: %v", err)
	}
	if err := tm.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	lines := nonEmptyLines(data)
	if len(lines) != 3 {
		t.Fatalf("lines = %d, want 3 (header + assistant + tool)", len(lines))
	}
	for i, role := range []string{"assistant", "tool"} {
		if !strings.Contains(lines[i+1], `"role":"`+role+`"`) {
			t.Errorf("line %d does not carry role %q: %s", i+1, role, lines[i+1])
		}
	}

	// Reopen: the transcript must build a context the provider accepts —
	// the call is answered.
	reopened, err := session.OpenTreeSession(path)
	if err != nil {
		t.Fatalf("OpenTreeSession: %v", err)
	}
	defer func() { _ = reopened.Close() }()
	msgs := reopened.GetLLMMessages()
	if len(msgs) != 2 {
		t.Fatalf("len(messages) = %d, want 2", len(msgs))
	}
	if msgs[1].Role != fantasy.MessageRoleTool {
		t.Fatalf("second message role = %q, want tool", msgs[1].Role)
	}
}

func TestTreeManagerAdapterRecoversInterruptedTurn(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	tm, err := session.CreateTreeSession(t.TempDir())
	if err != nil {
		t.Fatalf("CreateTreeSession: %v", err)
	}
	path := tm.GetFilePath()

	// Simulate the crash window: the assistant message (tool calls) was
	// persisted, the tool message was not. This is the per-message state
	// older kit versions could leave behind between the two appends.
	sm := NewTreeManagerAdapter(tm)
	_, err = appendMessages(context.Background(), sm, []LLMMessage{
		{
			Role: fantasy.MessageRoleAssistant,
			Content: []fantasy.MessagePart{
				fantasy.ToolCallPart{ToolCallID: "call-9", ToolName: "bash", Input: "{}"},
			},
		},
	})
	if err != nil {
		t.Fatalf("append messages: %v", err)
	}
	if err := tm.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Reopen: the repair pass must make the transcript provider-valid by
	// appending a synthetic interrupted result.
	reopened, err := session.OpenTreeSession(path)
	if err != nil {
		t.Fatalf("OpenTreeSession: %v", err)
	}
	defer func() { _ = reopened.Close() }()

	msgs := reopened.GetLLMMessages()
	if len(msgs) != 2 {
		t.Fatalf("len(messages) = %d, want 2 (assistant + repaired tool)", len(msgs))
	}
	if msgs[1].Role != fantasy.MessageRoleTool {
		t.Fatalf("last message role = %q, want tool", msgs[1].Role)
	}
	for _, p := range msgs[1].Content {
		if rp, ok := p.(fantasy.ToolResultPart); ok {
			if rp.ToolCallID != "call-9" {
				t.Errorf("repaired ToolCallID = %q, want call-9", rp.ToolCallID)
			}
			if _, isErr := rp.Output.(fantasy.ToolResultOutputContentError); !isErr {
				t.Errorf("repaired output = %T, want error content", rp.Output)
			}
			return
		}
	}
	t.Fatalf("last message carries no tool result")
}

// nonEmptyLines splits file content into non-blank lines.
func nonEmptyLines(data []byte) []string {
	var lines []string
	for line := range bytes.SplitSeq(data, []byte("\n")) {
		if len(bytes.TrimSpace(line)) > 0 {
			lines = append(lines, string(line))
		}
	}
	return lines
}

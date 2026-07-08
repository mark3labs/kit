package kit

import (
	"context"
	"strings"
	"testing"
)

// Tests for issue #87: resumable subagent sessions and parent-child session
// linking. These validate the fast-fail paths in Kit.Subagent, which run
// before any provider/child initialization.

func TestSubagent_SessionIDConflictsWithNoSession(t *testing.T) {
	m := &Kit{}
	_, err := m.Subagent(context.Background(), SubagentConfig{
		Prompt:    "task",
		SessionID: "some-session",
		NoSession: true,
	})
	if err == nil {
		t.Fatal("expected error when both SessionID and NoSession are set")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("error = %v, want mention of mutual exclusivity", err)
	}
}

func TestSubagent_UnknownSessionIDFailsFast(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // isolate from real ~/.kit/sessions

	m := &Kit{}
	_, err := m.Subagent(context.Background(), SubagentConfig{
		Prompt:    "task",
		SessionID: "does-not-exist",
	})
	if err == nil {
		t.Fatal("expected error for unknown subagent session ID")
	}
	if !strings.Contains(err.Error(), "cannot resume subagent session") {
		t.Errorf("error = %v, want resume-failure context", err)
	}
}

package kit

import (
	"context"
	"errors"
	"testing"
	"time"
)

// stubSessionManager is a minimal SessionManager that records how messages
// arrive. It implements the interface but NOT StepAppender, so it exercises
// the fallback path.
type stubSessionManager struct {
	// groups records one entry per append call group. A StepAppender receives
	// one group per step; a plain SessionManager receives one group per
	// message.
	groups  [][]string
	appends int
}

func (s *stubSessionManager) AppendMessage(msg LLMMessage) (string, error) {
	s.appends++
	s.groups = append(s.groups, []string{string(msg.Role)})
	return "e", nil
}

func (s *stubSessionManager) GetMessages() []LLMMessage                    { return nil }
func (s *stubSessionManager) BuildContext() ([]LLMMessage, string, string) { return nil, "", "" }
func (s *stubSessionManager) Branch(string) error                          { return nil }
func (s *stubSessionManager) GetCurrentBranch() []BranchEntry              { return nil }
func (s *stubSessionManager) GetChildren(string) []string                  { return nil }
func (s *stubSessionManager) GetEntry(string) *BranchEntry                 { return nil }
func (s *stubSessionManager) GetSessionID() string                         { return "stub" }
func (s *stubSessionManager) GetSessionName() string                       { return "" }
func (s *stubSessionManager) SetSessionName(string) error                  { return nil }
func (s *stubSessionManager) GetCreatedAt() time.Time                      { return time.Time{} }
func (s *stubSessionManager) IsPersisted() bool                            { return false }
func (s *stubSessionManager) AppendCompaction(string, string, int, int, int, []string, []string) (string, error) {
	return "", nil
}
func (s *stubSessionManager) GetLastCompaction() *CompactionEntry { return nil }
func (s *stubSessionManager) AppendExtensionData(string, string) (string, error) {
	return "", nil
}
func (s *stubSessionManager) GetExtensionData(string) []ExtensionDataEntry { return nil }
func (s *stubSessionManager) AppendModelChange(string, string) (string, error) {
	return "", nil
}
func (s *stubSessionManager) GetContextEntryIDs() []string { return nil }
func (s *stubSessionManager) AppendBranchSummary(string, string) (string, error) {
	return "", nil
}
func (s *stubSessionManager) Close() error { return nil }

// batchSessionManager additionally implements StepAppender.
type batchSessionManager struct {
	stubSessionManager
	steps     [][]string
	stepCalls int
	err       error
	// gotCtx records the context handed to the most recent AppendStep call.
	gotCtx     context.Context
	contextErr error
}

func (b *batchSessionManager) AppendStep(ctx context.Context, msgs []LLMMessage) ([]string, error) {
	b.stepCalls++
	b.gotCtx = ctx
	b.contextErr = ctx.Err()
	if b.err != nil {
		return nil, b.err
	}
	roles := make([]string, 0, len(msgs))
	ids := make([]string, 0, len(msgs))
	for i, m := range msgs {
		roles = append(roles, string(m.Role))
		ids = append(ids, string(rune('a'+i)))
	}
	b.steps = append(b.steps, roles)
	return ids, nil
}

// Compile-time proof that the optional interface is satisfiable from outside
// the package's own implementations.
var _ StepAppender = (*batchSessionManager)(nil)

func toolStep() []LLMMessage {
	return []LLMMessage{
		{Role: LLMMessageRole("assistant")},
		{Role: LLMMessageRole("tool")},
	}
}

// TestAppendMessagesUsesStepAppender is the core guarantee behind StepAppender:
// a session manager that implements it receives a tool-calling step as ONE
// call, so it can commit the assistant message and its tool result atomically.
func TestAppendMessagesUsesStepAppender(t *testing.T) {
	t.Parallel()

	sm := &batchSessionManager{}
	if n, err := appendMessages(context.Background(), sm, toolStep()); err != nil || n != 2 {
		t.Fatalf("append messages: confirmed=%d error=%v", n, err)
	}

	if sm.stepCalls != 1 {
		t.Fatalf("AppendStep called %d times, want exactly 1", sm.stepCalls)
	}
	if sm.appends != 0 {
		t.Fatalf("AppendMessage called %d times, want 0 when StepAppender exists", sm.appends)
	}
	if len(sm.steps) != 1 || len(sm.steps[0]) != 2 {
		t.Fatalf("step groups = %v, want one group of two messages", sm.steps)
	}
	if sm.steps[0][0] != "assistant" || sm.steps[0][1] != "tool" {
		t.Fatalf("step order = %v, want [assistant tool]", sm.steps[0])
	}
}

// TestAppendMessagesFallsBackToPerMessage proves the change is not breaking:
// an existing SessionManager that knows nothing about StepAppender still gets
// its messages, one per call, exactly as before.
func TestAppendMessagesFallsBackToPerMessage(t *testing.T) {
	t.Parallel()

	sm := &stubSessionManager{}
	if n, err := appendMessages(context.Background(), sm, toolStep()); err != nil || n != 2 {
		t.Fatalf("append messages: confirmed=%d error=%v", n, err)
	}

	if sm.appends != 2 {
		t.Fatalf("AppendMessage called %d times, want 2", sm.appends)
	}
	if len(sm.groups) != 2 {
		t.Fatalf("groups = %v, want one per message", sm.groups)
	}
}

// Failed atomic writes must return an error without a per-message fallback.
func TestAppendMessagesReportsStepAppenderError(t *testing.T) {
	t.Parallel()

	sm := &batchSessionManager{err: errors.New("disk full")}
	n, err := appendMessages(context.Background(), sm, toolStep())
	if n != 0 || !errors.Is(err, sm.err) {
		t.Fatalf("count=%d error=%v", n, err)
	}

	if sm.stepCalls != 1 {
		t.Fatalf("AppendStep called %d times, want 1", sm.stepCalls)
	}
	if sm.appends != 0 {
		t.Fatal("a failed AppendStep must not silently fall back to AppendMessage: " +
			"that would write a partially-committed step twice")
	}
}

func TestAppendMessagesIgnoresEmptyAndNil(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	sm := &batchSessionManager{}
	for _, msgs := range [][]LLMMessage{nil, {}} {
		if n, err := appendMessages(ctx, sm, msgs); err != nil || n != 0 {
			t.Fatalf("empty append: confirmed=%d error=%v", n, err)
		}
	}
	if sm.stepCalls != 0 || sm.appends != 0 {
		t.Fatalf("empty input caused %d step calls and %d appends, want none",
			sm.stepCalls, sm.appends)
	}

	if n, err := appendMessages(ctx, nil, toolStep()); err != nil || n != 0 {
		t.Fatalf("nil session append: confirmed=%d error=%v", n, err)
	}
}

// TestAppendMessagesForwardsContext proves the turn's context reaches a
// StepAppender, so a durable implementation can carry tracing values and pick
// its own write deadline.
func TestAppendMessagesForwardsContext(t *testing.T) {
	t.Parallel()

	type ctxKey struct{}
	ctx := context.WithValue(context.Background(), ctxKey{}, "trace-42")

	sm := &batchSessionManager{}
	if n, err := appendMessages(ctx, sm, toolStep()); err != nil || n != 2 {
		t.Fatalf("append messages: confirmed=%d error=%v", n, err)
	}

	if sm.gotCtx == nil {
		t.Fatal("AppendStep received a nil context")
	}
	if got := sm.gotCtx.Value(ctxKey{}); got != "trace-42" {
		t.Fatalf("context value = %v, want the caller's value to survive", got)
	}
}

// TestAppendMessagesStillWritesWhenContextCancelled pins deliberate behaviour.
// Kit persists a completed step BEFORE it checks for cancellation (see
// OnStepFinish in internal/agent), so that finished work survives an
// interrupted turn. appendMessages must therefore hand the step over even when
// ctx is already done, and must not short-circuit on ctx.Err() as a
// well-meaning optimisation: doing so would silently drop exactly the progress
// this path exists to save.
func TestAppendMessagesStillWritesWhenContextCancelled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	sm := &batchSessionManager{}
	if n, err := appendMessages(ctx, sm, toolStep()); err != nil || n != 2 {
		t.Fatalf("append messages: confirmed=%d error=%v", n, err)
	}

	if sm.stepCalls != 1 {
		t.Fatalf("AppendStep called %d times on a cancelled context, want 1: "+
			"completed work must still be persisted", sm.stepCalls)
	}
	if sm.gotCtx == nil || sm.contextErr != nil {
		t.Fatal("turn cancellation must be detached for completed writes")
	}
	if _, ok := sm.gotCtx.Deadline(); !ok {
		t.Fatal("write deadline missing")
	}

	// The fallback path must behave the same way.
	plain := &stubSessionManager{}
	if n, err := appendMessages(ctx, plain, toolStep()); err != nil || n != 2 {
		t.Fatalf("append messages: confirmed=%d error=%v", n, err)
	}
	if plain.appends != 2 {
		t.Fatalf("AppendMessage called %d times on a cancelled context, want 2", plain.appends)
	}
}

// TestSessionManagerInterfaceIsFrozen guards the stability promise in the
// SessionManager godoc. If a method is added, every external implementer
// breaks at compile time — so this stub breaking is the intended early
// warning, and the promise must be revisited before the change lands.
func TestSessionManagerInterfaceIsFrozen(t *testing.T) {
	t.Parallel()

	var sm SessionManager = &stubSessionManager{}
	if sm.GetSessionID() != "stub" {
		t.Fatalf("GetSessionID = %q", sm.GetSessionID())
	}
}

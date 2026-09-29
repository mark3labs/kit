package kit

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"strings"
	"sync"
	"testing"
	"time"

	"charm.land/fantasy"
)

func TestSubagentRegistry_AddListKill(t *testing.T) {
	var r subagentRegistry
	now := time.Now()

	ctxA, killA := context.WithCancelCause(context.Background())
	idA, removeA := r.add(RunningSubagent{ID: "a", StartedAt: now.Add(time.Second)}, killA)
	_, killB := context.WithCancelCause(context.Background())
	idB, removeB := r.add(RunningSubagent{StartedAt: now}, killB)

	if idA != "a" {
		t.Fatalf("idA = %q, want a", idA)
	}
	if idB == "" || idB == idA {
		t.Fatalf("idB = %q, want a new random ID", idB)
	}
	list := r.list()
	if len(list) != 2 || list[0].ID != idB || list[1].ID != idA {
		t.Fatalf("list = %+v, want oldest first [%s %s]", list, idB, idA)
	}

	if !r.kill("a") {
		t.Fatal("kill(a) = false, want true")
	}
	if !errors.Is(context.Cause(ctxA), ErrSubagentKilled) {
		t.Fatalf("cause = %v, want ErrSubagentKilled", context.Cause(ctxA))
	}
	if r.kill("missing") {
		t.Fatal("kill(missing) = true, want false")
	}

	removeA()
	removeB()
	if n := len(r.list()); n != 0 {
		t.Fatalf("list has %d entries after remove, want 0", n)
	}
}

func TestSubagentRegistry_DuplicateIDGetsNewID(t *testing.T) {
	var r subagentRegistry
	_, kill := context.WithCancelCause(context.Background())
	id1, remove1 := r.add(RunningSubagent{ID: "x"}, kill)
	id2, remove2 := r.add(RunningSubagent{ID: "x"}, kill)
	defer remove2()
	if id1 != "x" || id2 == "x" {
		t.Fatalf("ids = %q, %q; want x and a new ID", id1, id2)
	}
	// Removing the first run must not remove the second one.
	remove1()
	if list := r.list(); len(list) != 1 || list[0].ID != id2 {
		t.Fatalf("list = %+v, want only %s", list, id2)
	}
}

// killTestModel acts as the parent and the child model. The parent calls the
// subagent tool once and then answers with text. The child blocks until its
// context is cancelled.
type killTestModel struct {
	echoModel
	childStarted chan struct{}
	startOnce    sync.Once

	mu          sync.Mutex
	parentCalls int
	finalPrompt string
}

const killTestChildTask = "child task for kill test"

func (m *killTestModel) Generate(context.Context, fantasy.Call) (*fantasy.Response, error) {
	return nil, errors.New("killTestModel: streaming only")
}

func (m *killTestModel) Stream(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	prompt := fmt.Sprintf("%+v", call.Prompt)
	isChild := !strings.Contains(prompt, "PARENT")

	if isChild {
		m.startOnce.Do(func() { close(m.childStarted) })
		return iter.Seq[fantasy.StreamPart](func(yield func(fantasy.StreamPart) bool) {
			<-ctx.Done()
			yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeError, Error: ctx.Err()})
		}), nil
	}

	m.mu.Lock()
	m.parentCalls++
	first := m.parentCalls == 1
	if !first {
		m.finalPrompt = prompt
	}
	m.mu.Unlock()

	var parts []fantasy.StreamPart
	if first {
		input := fmt.Sprintf(`{"task":%q}`, killTestChildTask)
		parts = []fantasy.StreamPart{
			{Type: fantasy.StreamPartTypeToolInputStart, ID: "call-sub", ToolCallName: "subagent"},
			{Type: fantasy.StreamPartTypeToolInputDelta, ID: "call-sub", Delta: input},
			{Type: fantasy.StreamPartTypeToolInputEnd, ID: "call-sub"},
			{Type: fantasy.StreamPartTypeToolCall, ID: "call-sub", ToolCallName: "subagent", ToolCallInput: input},
			{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonToolCalls},
		}
	} else {
		parts = []fantasy.StreamPart{
			{Type: fantasy.StreamPartTypeTextStart, ID: "t"},
			{Type: fantasy.StreamPartTypeTextDelta, ID: "t", Delta: "ok"},
			{Type: fantasy.StreamPartTypeTextEnd, ID: "t"},
			{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonStop},
		}
	}
	return iter.Seq[fantasy.StreamPart](func(yield func(fantasy.StreamPart) bool) {
		for _, p := range parts {
			if !yield(p) {
				return
			}
		}
	}), nil
}

// TestKillSubagent_ToolReportsKillToParent starts a subagent through the
// subagent tool, kills it, and checks that the parent LLM gets a tool result
// that says the user killed it.
func TestKillSubagent_ToolReportsKillToParent(t *testing.T) {
	model := &killTestModel{childStarted: make(chan struct{})}
	model.provider, model.model = "killtest", "m"
	factory := func(context.Context, *ProviderConfig, string) (*ProviderResult, error) {
		return &ProviderResult{Model: model}, nil
	}

	var subagentTool []Tool
	for _, tool := range AllTools() {
		if tool.Info().Name == "subagent" {
			subagentTool = append(subagentTool, tool)
		}
	}
	if len(subagentTool) != 1 {
		t.Fatalf("subagent tool not found in AllTools()")
	}

	k := newProviderTestKit(t, &Options{
		Model:        "killtest/m",
		SystemPrompt: "PARENT",
		Providers:    map[string]ProviderFactory{"killtest": factory},
		Tools:        subagentTool,
	})

	type turn struct {
		res *TurnResult
		err error
	}
	done := make(chan turn, 1)
	go func() {
		res, err := k.PromptResult(context.Background(), "delegate")
		done <- turn{res, err}
	}()

	select {
	case <-model.childStarted:
	case <-time.After(10 * time.Second):
		t.Fatal("child model was not called")
	}

	running := k.RunningSubagents()
	if len(running) != 1 {
		t.Fatalf("RunningSubagents = %+v, want one run", running)
	}
	if running[0].ID != "call-sub" || running[0].Prompt != killTestChildTask {
		t.Fatalf("RunningSubagents[0] = %+v, want ID call-sub and the task prompt", running[0])
	}
	if !k.KillSubagent("call-sub") {
		t.Fatal("KillSubagent returned false for a running subagent")
	}

	var got turn
	select {
	case got = <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("parent turn did not complete after the kill")
	}
	if got.err != nil {
		t.Fatalf("PromptResult: %v", got.err)
	}

	model.mu.Lock()
	final := model.finalPrompt
	model.mu.Unlock()
	if !strings.Contains(final, "Subagent was killed by the user") {
		t.Fatalf("parent did not get the kill message; final prompt:\n%s", final)
	}
	if n := len(k.RunningSubagents()); n != 0 {
		t.Fatalf("RunningSubagents has %d entries after the kill, want 0", n)
	}
	if k.KillSubagent("call-sub") {
		t.Fatal("KillSubagent returned true for a completed subagent")
	}
}

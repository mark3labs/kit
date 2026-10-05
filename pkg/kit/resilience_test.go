package kit

import (
	"context"
	"errors"
	"iter"
	"strings"
	"testing"

	"charm.land/fantasy"
)

type unfinishedModel struct {
	echoModel
	failure error
}

func (m *unfinishedModel) Stream(context.Context, fantasy.Call) (fantasy.StreamResponse, error) {
	return iter.Seq[fantasy.StreamPart](func(yield func(fantasy.StreamPart) bool) {
		for _, part := range []fantasy.StreamPart{
			{Type: fantasy.StreamPartTypeTextStart, ID: "text"},
			{Type: fantasy.StreamPartTypeTextDelta, ID: "text", Delta: "unfinished reply"},
			{Type: fantasy.StreamPartTypeError, Error: m.failure},
		} {
			if !yield(part) {
				return
			}
		}
	}), nil
}

func TestPromptResultRetainsIncompleteOutput(t *testing.T) {
	failure := errors.New("stream failed")
	model := &unfinishedModel{failure: failure}
	model.provider, model.model = "partial", "m"
	k := newProviderTestKit(t, &Options{Model: "partial/m", Providers: map[string]ProviderFactory{
		"partial": func(context.Context, *ProviderConfig, string) (*ProviderResult, error) {
			return &ProviderResult{Model: model}, nil
		},
	}})
	result, err := k.PromptResult(context.Background(), "go")
	if !errors.Is(err, failure) {
		t.Fatalf("error = %v", err)
	}
	if result == nil || !result.Incomplete || result.PartialText != "unfinished reply" || result.AttemptID == "" || len(result.Stream) == 0 {
		t.Fatalf("result = %#v", result)
	}
	for _, msg := range k.session.GetMessages() {
		for _, part := range msg.Content {
			if text, ok := part.(fantasy.TextPart); ok && strings.Contains(text.Text, "unfinished reply") {
				t.Fatal("unfinished text entered model history")
			}
		}
	}
	branch := k.session.GetCurrentBranch()
	last := branch[len(branch)-1]
	if last.Type != EntryTypeIncompleteOutput || last.AttemptID != result.AttemptID || !last.Incomplete {
		t.Fatalf("entry = %#v", last)
	}
}

type failingWrites struct {
	messages []LLMMessage
	stubSessionManager
	failAt  int
	failure error
}

func (s *failingWrites) AppendMessage(msg LLMMessage) (string, error) {
	if s.appends == s.failAt {
		return "", s.failure
	}
	s.appends++
	s.messages = append(s.messages, msg)
	return "confirmed", nil
}
func (s *failingWrites) GetMessages() []LLMMessage                    { return s.messages }
func (s *failingWrites) BuildContext() ([]LLMMessage, string, string) { return s.messages, "", "" }

func TestAppendMessagesCountsConfirmedWrites(t *testing.T) {
	failure := errors.New("disk full")
	sm := &failingWrites{failAt: 1, failure: failure}
	n, err := appendMessages(context.Background(), sm, toolStep())
	if n != 1 || !errors.Is(err, failure) || sm.appends != 1 {
		t.Fatalf("count=%d error=%v appends=%d", n, err, sm.appends)
	}
}

func TestPromptStopsWhenPromptWriteFails(t *testing.T) {
	failure := errors.New("disk full")
	model := &scriptedToolModel{}
	model.provider, model.model = "persist", "m"
	k := newProviderTestKit(t, &Options{Model: "persist/m", Providers: map[string]ProviderFactory{
		"persist": func(context.Context, *ProviderConfig, string) (*ProviderResult, error) {
			return &ProviderResult{Model: model}, nil
		},
	}})
	k.session = &failingWrites{failure: failure}
	result, err := k.PromptResult(context.Background(), "go")
	if !errors.Is(err, failure) || result == nil || !result.Incomplete || model.callCount() != 0 {
		t.Fatalf("result=%#v error=%v calls=%d", result, err, model.callCount())
	}
}

func TestPromptReturnsResultWhenCompletedStepWriteFails(t *testing.T) {
	failure := errors.New("step write failed")
	model := &scriptedToolModel{}
	model.provider, model.model = "persist", "m"
	k := newProviderTestKit(t, &Options{Model: "persist/m", Providers: map[string]ProviderFactory{
		"persist": func(context.Context, *ProviderConfig, string) (*ProviderResult, error) {
			return &ProviderResult{Model: model}, nil
		},
	}})
	sm := &failingWrites{failAt: 1, failure: failure}
	k.session = sm
	result, err := k.PromptResult(context.Background(), "go")
	if !errors.Is(err, failure) || result == nil || !result.Incomplete || model.callCount() != 1 {
		t.Fatalf("result=%#v error=%v calls=%d", result, err, model.callCount())
	}
	if sm.appends != 1 {
		t.Fatalf("confirmed writes=%d", sm.appends)
	}
	if len(result.Messages) != 2 {
		t.Fatalf("completed messages=%v", result.Messages)
	}
}

func TestPromptResultRetainsTextOnCancellation(t *testing.T) {
	model := &unfinishedModel{failure: context.Canceled}
	model.provider, model.model = "partial", "m"
	k := newProviderTestKit(t, &Options{Model: "partial/m", Providers: map[string]ProviderFactory{
		"partial": func(context.Context, *ProviderConfig, string) (*ProviderResult, error) {
			return &ProviderResult{Model: model}, nil
		},
	}})
	result, err := k.PromptResult(context.Background(), "go")
	if !errors.Is(err, context.Canceled) || result == nil || result.PartialText != "unfinished reply" || !result.Incomplete {
		t.Fatalf("result=%#v error=%v", result, err)
	}
}

type incompleteWriteFailure struct {
	SessionManager
	failure error
}

func (s *incompleteWriteFailure) AppendIncompleteOutput(context.Context, IncompleteOutput) (string, error) {
	return "", s.failure
}

func TestIncompleteWriteFailurePreservesBothErrorsAndText(t *testing.T) {
	streamFailure, writeFailure := errors.New("stream failed"), errors.New("transcript write failed")
	model := &unfinishedModel{failure: streamFailure}
	model.provider, model.model = "partial", "m"
	k := newProviderTestKit(t, &Options{Model: "partial/m", Providers: map[string]ProviderFactory{
		"partial": func(context.Context, *ProviderConfig, string) (*ProviderResult, error) {
			return &ProviderResult{Model: model}, nil
		},
	}})
	k.session = &incompleteWriteFailure{SessionManager: k.session, failure: writeFailure}
	result, err := k.PromptResult(context.Background(), "go")
	if !errors.Is(err, streamFailure) || !errors.Is(err, writeFailure) || result == nil || result.PartialText != "unfinished reply" {
		t.Fatalf("result=%#v error=%v", result, err)
	}
}

type completedThenUnfinishedModel struct {
	scriptedToolModel
	failure error
}

func (m *completedThenUnfinishedModel) Stream(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	if m.callCount() == 0 {
		return m.scriptedToolModel.Stream(ctx, call)
	}
	return (&unfinishedModel{failure: m.failure}).Stream(ctx, call)
}

func TestPartialOutputDoesNotReplaceCompletedToolStep(t *testing.T) {
	failure := errors.New("second step failed")
	model := &completedThenUnfinishedModel{failure: failure}
	model.script = []string{"test_tool"}
	model.provider, model.model = "partial", "m"
	tool := NewTool("test_tool", "Test tool.", func(context.Context, struct{}) (ToolOutput, error) { return ToolOutput{Content: "tool completed"}, nil })
	k := newProviderTestKit(t, &Options{Model: "partial/m", ExtraTools: []Tool{tool}, Providers: map[string]ProviderFactory{
		"partial": func(context.Context, *ProviderConfig, string) (*ProviderResult, error) {
			return &ProviderResult{Model: model}, nil
		},
	}})
	result, err := k.PromptResult(context.Background(), "go")
	if !errors.Is(err, failure) || result == nil || result.PartialText != "unfinished reply" {
		t.Fatalf("result=%#v error=%v", result, err)
	}
	messages := k.session.GetMessages()
	if len(messages) != 3 || messages[1].Role != LLMMessageRole("assistant") || messages[2].Role != LLMMessageRole("tool") {
		t.Fatalf("history=%v", messages)
	}
	if len(result.Messages) != 3 {
		t.Fatalf("result history=%v", result.Messages)
	}
	branch := k.session.GetCurrentBranch()
	if len(branch) != 4 || branch[3].Type != EntryTypeIncompleteOutput {
		t.Fatalf("branch=%v", branch)
	}
}

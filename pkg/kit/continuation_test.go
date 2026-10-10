package kit

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"

	"charm.land/fantasy"
)

type continuationModel struct {
	echoModel
	requests  [][]LLMMessage
	toolModel *scriptedToolModel
	failure   error
}

func (m *continuationModel) Stream(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	m.requests = append(m.requests, slices.Clone(call.Prompt))
	if m.toolModel != nil && m.toolModel.callCount() == 0 {
		return m.toolModel.Stream(ctx, call)
	}
	if m.failure != nil {
		return (&unfinishedModel{failure: m.failure}).Stream(ctx, call)
	}
	return m.echoModel.Stream(ctx, call)
}

func TestContinueResultRestoredConversation(t *testing.T) {
	ctx := context.Background()
	const prompt = "original task"
	var toolRuns int
	tool := NewTool("saved_tool", "Complete a step.", func(context.Context, struct{}) (ToolOutput, error) {
		toolRuns++
		return ToolOutput{Content: "saved result"}, nil
	})
	newHost := func(model *continuationModel, sm SessionManager) *Kit {
		t.Helper()
		return newProviderTestKit(t, &Options{
			Model: "offline/m", SessionManager: sm, ExtraTools: []Tool{tool},
			NoContextFiles: true, NoSkills: true, NoAgents: true,
			Providers: map[string]ProviderFactory{"offline": func(context.Context, *ProviderConfig, string) (*ProviderResult, error) {
				return &ProviderResult{Model: model}, nil
			}},
		})
	}
	// Persist a completed tool step, then interrupt the original turn.
	first := &continuationModel{provider: "offline", model: "m",
		toolModel: &scriptedToolModel{script: []string{"saved_tool"}}, failure: context.Canceled}
	sm := &failingWrites{failAt: 100}
	k := newHost(first, sm)
	partial, err := k.PromptResult(ctx, prompt)
	if !errors.Is(err, context.Canceled) || partial == nil || !partial.Incomplete {
		t.Fatalf("initial turn: result=%+v error=%v", partial, err)
	}
	saved := slices.Clone(sm.GetMessages())
	if len(saved) != 3 || saved[1].Role != fantasy.MessageRoleAssistant || saved[2].Role != fantasy.MessageRoleTool {
		t.Fatalf("saved tool step: %+v", saved)
	}
	// Each fresh session manager represents a restore from durable storage.
	second := &continuationModel{echoModel: first.echoModel, failure: context.Canceled}
	third := &continuationModel{echoModel: first.echoModel}
	for i, model := range []*continuationModel{second, third} {
		restored := &failingWrites{messages: slices.Clone(saved), failAt: 100}
		host := newHost(model, restored)
		before, after, prepared := 0, 0, 0
		host.OnBeforeTurn(HookPriorityNormal, func(h BeforeTurnHook) *BeforeTurnResult {
			before++
			if h.Prompt != "" {
				t.Errorf("continuation hook prompt = %q", h.Prompt)
			}
			replacement := "must not become a user message"
			return &BeforeTurnResult{Prompt: &replacement}
		})
		host.OnContextPrepare(HookPriorityNormal, func(h ContextPrepareHook) *ContextPrepareResult {
			prepared++
			return nil
		})
		host.OnAfterTurn(HookPriorityNormal, func(h AfterTurnHook) { after++ })
		result, err := host.ContinueResult(ctx)
		if i == 0 {
			if !errors.Is(err, context.Canceled) || result == nil || !result.Incomplete || result.PartialText != "unfinished reply" {
				t.Fatalf("interrupted continuation: result=%+v error=%v", result, err)
			}
		} else if err != nil || result == nil || result.Incomplete || result.Response != third.reply() || result.TotalUsage == nil || result.FinalUsage == nil {
			t.Fatalf("completed continuation: result=%+v error=%v", result, err)
		}
		if before != 1 || after != 1 || prepared != 1 || len(result.Stream) == 0 {
			t.Fatalf("lifecycle: before=%d after=%d context=%d result=%+v", before, after, prepared, result)
		}
		if !reflect.DeepEqual(restored.GetMessages()[:len(saved)], saved) {
			t.Fatal("continuation changed saved messages")
		}
		saved = slices.Clone(restored.GetMessages())
	}
	if toolRuns != 1 {
		t.Fatalf("saved tool ran %d times, want 1", toolRuns)
	}
	check := func(messages []LLMMessage) {
		t.Helper()
		users := 0
		for _, msg := range messages {
			if msg.Role == fantasy.MessageRoleUser {
				users++
				if !reflect.DeepEqual(msg, fantasy.NewUserMessage(prompt)) {
					t.Fatalf("unexpected user message: %+v", msg)
				}
			}
		}
		if users != 1 {
			t.Fatalf("user messages=%d, want 1", users)
		}
	}
	check(saved)
	for _, model := range []*continuationModel{first, second, third} {
		for _, request := range model.requests {
			check(request)
		}
	}
	for _, model := range []*continuationModel{second, third} {
		if len(model.requests) != 1 {
			t.Fatalf("continuation requests=%d", len(model.requests))
		}
		var history []LLMMessage
		for _, msg := range model.requests[0] {
			if msg.Role == fantasy.MessageRoleAssistant || msg.Role == fantasy.MessageRoleTool {
				history = append(history, msg)
			}
		}
		if !reflect.DeepEqual(history, saved[1:3]) {
			t.Fatalf("provider did not receive restored tool calls and results: got %#v want %#v", history, saved[1:3])
		}
	}
}

func TestContinueResultRequiresConversation(t *testing.T) {
	for _, messages := range [][]LLMMessage{nil, {fantasy.NewSystemMessage("system only")}, {{Role: fantasy.MessageRoleUser}}, {fantasy.NewUserMessage("")}, {fantasy.NewUserMessage("  ")}, {{Role: fantasy.MessageRoleUser, Content: []fantasy.MessagePart{&fantasy.TextPart{Text: " "}}}}} {
		model := &continuationModel{provider: "offline", model: "m"}
		sm := &failingWrites{messages: messages, failAt: 100}
		k := newProviderTestKit(t, &Options{Model: "offline/m", SessionManager: sm,
			Providers: map[string]ProviderFactory{"offline": func(context.Context, *ProviderConfig, string) (*ProviderResult, error) {
				return &ProviderResult{Model: model}, nil
			}},
		})
		result, err := k.ContinueResult(context.Background())
		if err == nil || !strings.Contains(err.Error(), "no usable saved conversation") || result != nil || len(model.requests) != 0 || sm.appends != 0 {
			t.Fatalf("result=%+v error=%v requests=%d appends=%d", result, err, len(model.requests), sm.appends)
		}
	}
}

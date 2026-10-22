package kit

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync/atomic"
	"testing"
	"time"
)

func extensionProviderKit(t *testing.T, example, model string, extra []Tool) *Kit {
	t.Helper()
	source, err := filepath.Abs(filepath.Join("..", "..", "examples", "extensions", example))
	if err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(t.TempDir(), "config.yml")
	if err := os.WriteFile(config, []byte(fmt.Sprintf("extension: [%q]\nno-skills: true\nno-agents: true\n", source)), 0600); err != nil {
		t.Fatal(err)
	}
	k, err := New(context.Background(), &Options{Bare: true, ConfigFile: config, Model: model, Quiet: true, NoSession: true, DisableCoreTools: true, ExtraTools: extra})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := k.Close(); err != nil {
			t.Error(err)
		}
	})
	return k
}

func TestExtensionProviderGreetingThroughKit(t *testing.T) {
	before := RegisteredProviders()
	k := extensionProviderKit(t, "fake-provider.go", "fake/echo", nil)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	got, err := k.Prompt(ctx, "hello")
	if err != nil || got != "Hello from a fake provider." {
		t.Fatalf("got=%q err=%v", got, err)
	}
	for _, model := range []string{"", "fake/echo"} {
		response, err := k.ExecuteCompletion(ctx, CompleteRequest{Model: model, Prompt: "hello"})
		if err != nil || response.Text != "Hello from a fake provider." {
			t.Fatalf("completion=%+v err=%v", response, err)
		}
	}
	child, err := k.Subagent(ctx, SubagentConfig{Prompt: "hello", Model: "fake/echo", Tools: []Tool{}, NoSession: true})
	if err != nil || child.Response != "Hello from a fake provider." {
		t.Fatalf("child=%+v err=%v", child, err)
	}
	got, err = k.Prompt(ctx, "parent still works")
	if err != nil || got != "Hello from a fake provider." {
		t.Fatalf("parent after child=%q err=%v", got, err)
	}
	if !slices.Equal(before, RegisteredProviders()) {
		t.Fatal("extension changed global provider factories")
	}
	if LookupModel("fake", "echo") != nil {
		t.Fatal("extension changed global catalog")
	}
}

func TestExtensionProviderToolRoundTrip(t *testing.T) {
	var executions atomic.Int32
	var beforeCalls, afterCalls atomic.Int32
	type input struct {
		A int `json:"a"`
		B int `json:"b"`
	}
	tool := NewTool("add", "add numbers", func(_ context.Context, args input) (ToolOutput, error) {
		executions.Add(1)
		return ToolOutput{Content: fmt.Sprint(args.A + args.B)}, nil
	})
	k := extensionProviderKit(t, "fake-tool-provider.go", "fake-tool/add", []Tool{tool})
	k.OnBeforeToolCall(HookPriorityNormal, func(BeforeToolCallHook) *BeforeToolCallResult { beforeCalls.Add(1); return nil })
	k.OnAfterToolResult(HookPriorityNormal, func(AfterToolResultHook) *AfterToolResultResult { afterCalls.Add(1); return nil })
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	got, err := k.Prompt(ctx, "add 2 and 3")
	if err != nil || got != "5" {
		t.Fatalf("got=%q err=%v", got, err)
	}
	if beforeCalls.Load() != 1 || afterCalls.Load() != 1 {
		t.Fatalf("hook calls before=%d after=%d", beforeCalls.Load(), afterCalls.Load())
	}
	if executions.Load() != 1 {
		t.Fatalf("executions=%d", executions.Load())
	}
	calls, results := 0, 0
	for _, msg := range k.GetStructuredMessages() {
		for _, part := range msg.Parts {
			switch item := part.(type) {
			case ToolCall:
				if item.ID != "add-call" || item.Input != `{"a":2,"b":3}` {
					t.Fatalf("call=%+v", item)
				}
				calls++
			case ToolResult:
				if calls != 1 || item.ToolCallID != "add-call" || item.Content != "5" {
					t.Fatalf("result=%+v calls=%d", item, calls)
				}
				results++
			}
		}
	}
	if calls != 1 || results != 1 {
		t.Fatalf("stored calls=%d results=%d", calls, results)
	}
}

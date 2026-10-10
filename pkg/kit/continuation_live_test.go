package kit_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	kit "github.com/mark3labs/kit/pkg/kit"
)

// Run explicitly with KIT_CONTINUATION_LIVE=1. This test makes a paid live
// provider request and uses the SDK's normal provider authentication.
func TestContinueResultLive(t *testing.T) {
	if os.Getenv("KIT_CONTINUATION_LIVE") != "1" {
		t.Skip("set KIT_CONTINUATION_LIVE=1 to test opencode/kimi-k3 live")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	const prompt = "Reply with exactly this text: continuation-live-ok"
	const sessionID = "continuation-live-regression"
	path := filepath.Join(t.TempDir(), "continuation.jsonl")
	// Start a version-3 JSONL session in the test directory. Do not change
	// HOME: the live provider must use the normal authentication location.
	header, err := json.Marshal(kit.SessionHeader{
		Type: "session", Version: 3, ID: sessionID,
		Timestamp: time.Now().UTC(), Cwd: filepath.Dir(path),
	})
	if err != nil {
		t.Fatalf("encode session header: %v", err)
	}
	if err := os.WriteFile(path, append(header, '\n'), 0600); err != nil {
		t.Fatalf("create session file: %v", err)
	}
	seed, err := kit.OpenTreeSession(path)
	if err != nil {
		t.Fatalf("open seed session: %v", err)
	}
	seedManager := kit.NewTreeManagerAdapter(seed)
	if _, err := seedManager.AppendMessage(kit.NewLLMUserMessage(prompt)); err != nil {
		if closeErr := seedManager.Close(); closeErr != nil {
			t.Errorf("close seed session: %v", closeErr)
		}
		t.Fatalf("save original prompt: %v", err)
	}
	if err := seedManager.Close(); err != nil {
		t.Fatalf("close saved session: %v", err)
	}

	saved, err := kit.OpenTreeSession(path)
	if err != nil {
		t.Fatalf("reopen saved session: %v", err)
	}
	manager := kit.NewTreeManagerAdapter(saved)
	t.Cleanup(func() {
		if err := manager.Close(); err != nil {
			t.Errorf("close installed session: %v", err)
		}
	})
	host, err := kit.New(ctx, &kit.Options{
		Model: "opencode/kimi-k3", SessionManager: manager,
		SkipConfig: true, MCPConfig: &kit.Config{}, Tools: []kit.Tool{},
		DisableCoreTools: true, NoExtensions: true, NoContextFiles: true,
		NoSkills: true, NoAgents: true, Quiet: true, MaxSteps: 1,
		MaxTokens: 1024, SystemPrompt: "Follow the user's instruction. Reply briefly.",
	})
	if err != nil {
		t.Fatalf("create fresh Kit: %v", err)
	}
	t.Cleanup(func() {
		if err := host.Close(); err != nil {
			t.Errorf("close Kit: %v", err)
		}
	})
	if len(host.GetToolNames()) != 0 {
		t.Fatal("live continuation must have no tools")
	}
	checkMessages := func(label string, messages []kit.LLMMessage, wantAssistant bool) {
		t.Helper()
		users, assistants := 0, 0
		for _, msg := range messages {
			switch msg.Role {
			case kit.LLMRoleUser:
				users++
				if len(msg.Content) != 1 {
					t.Fatalf("%s: original prompt content changed", label)
				}
				text, ok := msg.Content[0].(kit.LLMTextPart)
				if !ok || text.Text != prompt {
					t.Fatalf("%s: user message is not the original prompt", label)
				}
			case kit.LLMRoleAssistant:
				assistants++
			}
		}
		if users != 1 {
			t.Errorf("%s: user messages = %d, want 1", label, users)
		}
		if wantAssistant && assistants == 0 {
			t.Errorf("%s: no assistant message", label)
		}
	}
	checkMessages("saved session", manager.GetMessages(), false)
	// No Prompt or FollowUp call: resume only the saved original input.
	result, err := host.ContinueResult(ctx)
	if err != nil {
		t.Fatalf("ContinueResult live: %v", err)
	}
	if result == nil {
		t.Fatal("ContinueResult returned no result")
	}
	if result.Incomplete || strings.TrimSpace(result.Response) == "" {
		t.Fatal("ContinueResult did not return a complete, non-empty response")
	}
	if !strings.Contains(result.Response, "continuation-live-ok") {
		t.Errorf("response does not answer the original prompt: %q", result.Response)
	}
	if result.SessionID != sessionID {
		t.Errorf("session ID = %q, want %q", result.SessionID, sessionID)
	}
	checkMessages("result", result.Messages, true)
	checkMessages("installed session", manager.GetMessages(), true)
	for label, usage := range map[string]*kit.LLMUsage{"total": result.TotalUsage, "final": result.FinalUsage} {
		if usage == nil || usage.InputTokens+usage.CacheReadTokens+usage.CacheCreationTokens <= 0 || usage.OutputTokens <= 0 || usage.TotalTokens <= 0 {
			t.Errorf("%s usage: want positive input, output, and total tokens; got %+v", label, usage)
		}
	}
	t.Logf("model=opencode/kimi-k3 user_messages=1 response=%q stop_reason=%q total_usage=%+v final_usage=%+v",
		result.Response, result.StopReason, result.TotalUsage, result.FinalUsage)
}

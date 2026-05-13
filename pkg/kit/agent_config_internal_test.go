package kit

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mark3labs/kit/internal/agent"
)

// TestAgentConfigToInternal verifies that the SDK-side AgentConfig converts
// faithfully to the internal agent.AgentConfig representation, preserving
// every field consumed by the internal agent layer.
//
// Regression test for https://github.com/mark3labs/kit/issues/30.
func TestAgentConfigToInternal(t *testing.T) {
	t.Run("nil receiver returns nil", func(t *testing.T) {
		var c *AgentConfig
		if got := c.toInternal(); got != nil {
			t.Errorf("nil.toInternal() = %v, want nil", got)
		}
	})

	t.Run("scalar fields round-trip", func(t *testing.T) {
		c := &AgentConfig{
			SystemPrompt:     "sys",
			MaxSteps:         7,
			StreamingEnabled: true,
			DisableCoreTools: true,
		}
		got := c.toInternal()
		if got == nil {
			t.Fatal("toInternal() = nil")
		}
		if got.SystemPrompt != "sys" {
			t.Errorf("SystemPrompt = %q, want %q", got.SystemPrompt, "sys")
		}
		if got.MaxSteps != 7 {
			t.Errorf("MaxSteps = %d, want 7", got.MaxSteps)
		}
		if !got.StreamingEnabled {
			t.Error("StreamingEnabled = false, want true")
		}
		if !got.DisableCoreTools {
			t.Error("DisableCoreTools = false, want true")
		}
	})

	t.Run("tool slices propagate without conversion", func(t *testing.T) {
		// Tool is a type alias for the underlying LLM-tool type, so the
		// SDK []Tool and internal []fantasy.AgentTool slices share the
		// same backing array after conversion.
		tool := NewTool[struct{}]("noop", "noop", nil)
		c := &AgentConfig{
			CoreTools:  []Tool{tool},
			ExtraTools: []Tool{tool, tool},
		}
		got := c.toInternal()
		if len(got.CoreTools) != 1 {
			t.Errorf("CoreTools len = %d, want 1", len(got.CoreTools))
		}
		if len(got.ExtraTools) != 2 {
			t.Errorf("ExtraTools len = %d, want 2", len(got.ExtraTools))
		}
	})

	t.Run("tool wrapper is invoked through internal config", func(t *testing.T) {
		called := false
		c := &AgentConfig{
			ToolWrapper: func(in []Tool) []Tool {
				called = true
				return in
			},
		}
		got := c.toInternal()
		if got.ToolWrapper == nil {
			t.Fatal("internal ToolWrapper is nil")
		}
		_ = got.ToolWrapper(nil)
		if !called {
			t.Error("SDK ToolWrapper was not invoked through the internal config")
		}
	})

	t.Run("OnMCPServerLoaded propagates", func(t *testing.T) {
		var captured string
		wantErr := errors.New("boom")
		c := &AgentConfig{
			OnMCPServerLoaded: func(name string, _ int, _ error) {
				captured = name
			},
		}
		got := c.toInternal()
		got.OnMCPServerLoaded("svr", 3, wantErr)
		if captured != "svr" {
			t.Errorf("OnMCPServerLoaded captured = %q, want %q", captured, "svr")
		}
	})

	t.Run("DebugLogger propagates", func(t *testing.T) {
		dl := &fakeDebugLogger{enabled: true}
		c := &AgentConfig{DebugLogger: dl}
		got := c.toInternal()
		if got.DebugLogger == nil {
			t.Fatal("internal DebugLogger is nil")
		}
		if !got.DebugLogger.IsDebugEnabled() {
			t.Error("IsDebugEnabled = false, want true")
		}
		got.DebugLogger.LogDebug("hello")
		if len(dl.messages) != 1 || dl.messages[0] != "hello" {
			t.Errorf("messages = %v, want [hello]", dl.messages)
		}
	})

	t.Run("MCPTaskConfig propagates with mode + progress", func(t *testing.T) {
		c := &AgentConfig{
			MCPTaskConfig: MCPTaskConfig{
				PerServerMode: map[string]MCPTaskMode{
					"build-svr": MCPTaskModeAlways,
				},
				DefaultTTL:      30 * time.Second,
				PollInterval:    250 * time.Millisecond,
				MaxPollInterval: 2 * time.Second,
				Timeout:         5 * time.Minute,
				Progress:        func(_ MCPTaskProgress) {},
			},
		}
		got := c.toInternal()
		if got.MCPTaskConfig.DefaultTTL != 30*time.Second {
			t.Errorf("DefaultTTL = %v, want 30s", got.MCPTaskConfig.DefaultTTL)
		}
		if got.MCPTaskConfig.PollInterval != 250*time.Millisecond {
			t.Errorf("PollInterval = %v, want 250ms", got.MCPTaskConfig.PollInterval)
		}
		if got.MCPTaskConfig.MaxPollInterval != 2*time.Second {
			t.Errorf("MaxPollInterval = %v, want 2s", got.MCPTaskConfig.MaxPollInterval)
		}
		if got.MCPTaskConfig.Timeout != 5*time.Minute {
			t.Errorf("Timeout = %v, want 5m", got.MCPTaskConfig.Timeout)
		}
		mode, ok := got.MCPTaskConfig.PerServerMode["build-svr"]
		if !ok {
			t.Fatal("PerServerMode missing 'build-svr'")
		}
		if string(mode) != string(MCPTaskModeAlways) {
			t.Errorf("mode = %q, want %q", mode, MCPTaskModeAlways)
		}
		if got.MCPTaskConfig.Progress == nil {
			t.Fatal("internal Progress handler is nil")
		}
	})

	t.Run("auth and token store factories are wired", func(t *testing.T) {
		auth := &fakeAuthHandler{}
		tokenCalls := 0
		var tokenServer string
		factory := MCPTokenStoreFactory(func(server string) (MCPTokenStore, error) {
			tokenCalls++
			tokenServer = server
			return nil, nil
		})
		c := &AgentConfig{
			AuthHandler:       auth,
			TokenStoreFactory: factory,
		}
		got := c.toInternal()
		if got.AuthHandler == nil {
			t.Fatal("internal AuthHandler is nil")
		}
		if got.TokenStoreFactory == nil {
			t.Fatal("internal TokenStoreFactory is nil")
		}
		_, _ = got.TokenStoreFactory("https://example.test")
		if tokenCalls != 1 {
			t.Errorf("token factory call count = %d, want 1", tokenCalls)
		}
		if tokenServer != "https://example.test" {
			t.Errorf("token factory server arg = %q", tokenServer)
		}
		if got.AuthHandler.RedirectURI() != "redirect" {
			t.Errorf("RedirectURI = %q, want %q", got.AuthHandler.RedirectURI(), "redirect")
		}
	})

	// Compile-time check that the internal type is what we expect.
	//nolint:staticcheck // QF1011: explicit type asserts the conversion target.
	var _ *agent.AgentConfig = (&AgentConfig{}).toInternal()
}

// fakeAuthHandler implements both kit.MCPAuthHandler and the structurally
// identical tools.MCPAuthHandler used by the internal layer.
type fakeAuthHandler struct{}

func (f *fakeAuthHandler) RedirectURI() string { return "redirect" }
func (f *fakeAuthHandler) HandleAuth(_ context.Context, _ string, _ string) (string, error) {
	return "", nil
}

// fakeDebugLogger implements kit.DebugLogger for tests.
type fakeDebugLogger struct {
	enabled  bool
	messages []string
}

func (f *fakeDebugLogger) LogDebug(m string)    { f.messages = append(f.messages, m) }
func (f *fakeDebugLogger) IsDebugEnabled() bool { return f.enabled }

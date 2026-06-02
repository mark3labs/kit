package kit

import (
	"testing"
	"time"

	"github.com/spf13/viper"

	"github.com/mark3labs/kit/internal/tools"
)

func TestMCPTaskStatusIsTerminal(t *testing.T) {
	cases := []struct {
		s    MCPTaskStatus
		want bool
	}{
		{MCPTaskStatusWorking, false},
		{MCPTaskStatusInputRequired, false},
		{MCPTaskStatusCompleted, true},
		{MCPTaskStatusFailed, true},
		{MCPTaskStatusCancelled, true},
		{MCPTaskStatus("unknown"), false},
	}
	for _, tc := range cases {
		if got := tc.s.IsTerminal(); got != tc.want {
			t.Errorf("MCPTaskStatus(%q).IsTerminal() = %v, want %v", tc.s, got, tc.want)
		}
	}
}

func TestMCPTaskOptionsToToolsConfig(t *testing.T) {
	called := 0
	o := mcpTaskOptions{
		perServer: map[string]MCPTaskMode{
			"alpha": MCPTaskModeAlways,
			"beta":  MCPTaskModeNever,
		},
		defaultTTL:      30 * time.Second,
		pollInterval:    250 * time.Millisecond,
		maxPollInterval: 2 * time.Second,
		timeout:         5 * time.Minute,
		progress:        func(p MCPTaskProgress) { called++ },
	}
	cfg := o.toToolsConfig()

	if cfg.DefaultTTL != 30*time.Second {
		t.Errorf("DefaultTTL = %v, want 30s", cfg.DefaultTTL)
	}
	if cfg.PollInterval != 250*time.Millisecond {
		t.Errorf("PollInterval = %v, want 250ms", cfg.PollInterval)
	}
	if cfg.MaxPollInterval != 2*time.Second {
		t.Errorf("MaxPollInterval = %v, want 2s", cfg.MaxPollInterval)
	}
	if cfg.Timeout != 5*time.Minute {
		t.Errorf("Timeout = %v, want 5m", cfg.Timeout)
	}
	if cfg.PerServerMode["alpha"] != tools.MCPTaskModeAlways {
		t.Errorf("PerServerMode[alpha] = %q, want always", cfg.PerServerMode["alpha"])
	}
	if cfg.PerServerMode["beta"] != tools.MCPTaskModeNever {
		t.Errorf("PerServerMode[beta] = %q, want never", cfg.PerServerMode["beta"])
	}

	// Progress conversion: invoking the internal handler must call our
	// SDK-level callback with the converted struct.
	if cfg.Progress == nil {
		t.Fatal("Progress callback was lost in conversion")
	}
	cfg.Progress(tools.MCPTaskProgress{
		Server: "alpha",
		TaskID: "t1",
		Status: "working",
	})
	if called != 1 {
		t.Errorf("expected SDK progress handler to be invoked once, got %d", called)
	}
}

func TestMCPTaskFromInternal(t *testing.T) {
	in := tools.MCPTaskInfo{
		Server:        "srv",
		TaskID:        "t-1",
		Status:        "working",
		StatusMessage: "phase 1",
		CreatedAt:     time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC),
		UpdatedAt:     time.Date(2026, 5, 4, 12, 0, 1, 0, time.UTC),
		TTL:           5 * time.Minute,
		PollInterval:  500 * time.Millisecond,
	}
	out := mcpTaskFromInternal(in)

	if out.Server != "srv" || out.TaskID != "t-1" {
		t.Errorf("identity fields not copied: %+v", out)
	}
	if out.Status != MCPTaskStatusWorking {
		t.Errorf("Status = %q, want working", out.Status)
	}
	if out.StatusMessage != "phase 1" {
		t.Errorf("StatusMessage = %q, want phase 1", out.StatusMessage)
	}
	if out.TTL != 5*time.Minute || out.PollInterval != 500*time.Millisecond {
		t.Errorf("durations not copied: %+v", out)
	}
}

func TestKitMCPTasksWithoutAgentReturnsError(t *testing.T) {
	// A nil/zero Kit must not panic — task RPCs should surface a clear
	// error instead. Useful for SDK consumers that try task ops on a Kit
	// constructed without MCP servers.
	var k *Kit
	ctx := t.Context()
	if _, err := k.ListMCPTasks(ctx, "any"); err == nil {
		t.Error("ListMCPTasks on nil Kit should error")
	}
	if _, err := k.GetMCPTask(ctx, "any", "id"); err == nil {
		t.Error("GetMCPTask on nil Kit should error")
	}
	if _, err := k.CancelMCPTask(ctx, "any", "id"); err == nil {
		t.Error("CancelMCPTask on nil Kit should error")
	}
}

func TestSubagentPropagatesMCPTaskOptions(t *testing.T) {
	// Exercises the helper Kit.Subagent uses to copy MCP task options
	// onto child Options. Calling the real helper (rather than
	// duplicating its body in the test) means any new field added to
	// the propagation list is picked up automatically by the
	// equivalence assertion below.
	parent := &Options{
		MCPTaskMode: map[string]MCPTaskMode{
			"build": MCPTaskModeAlways,
			"chat":  MCPTaskModeNever,
		},
		MCPTaskTimeout:         30 * time.Minute,
		MCPTaskTTL:             45 * time.Minute,
		MCPTaskPollInterval:    750 * time.Millisecond,
		MCPTaskMaxPollInterval: 4 * time.Second,
		MCPTaskProgress:        func(MCPTaskProgress) {},
	}

	child := &Options{}
	inheritMCPTaskOptions(child, parent)

	if child.MCPTaskMode["build"] != MCPTaskModeAlways || child.MCPTaskMode["chat"] != MCPTaskModeNever {
		t.Errorf("MCPTaskMode not propagated: got %+v", child.MCPTaskMode)
	}
	if child.MCPTaskTimeout != 30*time.Minute {
		t.Errorf("MCPTaskTimeout = %v, want 30m", child.MCPTaskTimeout)
	}
	if child.MCPTaskTTL != 45*time.Minute {
		t.Errorf("MCPTaskTTL = %v, want 45m", child.MCPTaskTTL)
	}
	if child.MCPTaskPollInterval != 750*time.Millisecond {
		t.Errorf("MCPTaskPollInterval = %v, want 750ms", child.MCPTaskPollInterval)
	}
	if child.MCPTaskMaxPollInterval != 4*time.Second {
		t.Errorf("MCPTaskMaxPollInterval = %v, want 4s", child.MCPTaskMaxPollInterval)
	}
	if child.MCPTaskProgress == nil {
		t.Error("MCPTaskProgress not propagated")
	}

	// Nil parent is a no-op rather than a panic.
	inheritMCPTaskOptions(&Options{}, nil)
	inheritMCPTaskOptions(nil, parent)
}

// TestInheritProviderConfig verifies that Kit.Subagent's provider/runtime
// config inheritance copies the parent's effective settings onto child
// Options, and that the tri-state (IsSet) keys are only propagated when the
// parent explicitly set them. Regression test for config loss after the
// per-instance viper store isolation (#40).
func TestInheritProviderConfig(t *testing.T) {
	t.Run("explicit values propagate", func(t *testing.T) {
		v := viper.New()
		v.Set("provider-api-key", "sk-parent")
		v.Set("provider-url", "https://proxy.internal/v1")
		v.Set("tls-skip-verify", true)
		v.Set("thinking-level", "high")
		v.Set("max-tokens", 4321)
		v.Set("temperature", 0.25)
		v.Set("top-p", 0.9)
		v.Set("top-k", 40)
		v.Set("frequency-penalty", 0.1)
		v.Set("presence-penalty", 0.2)

		child := &Options{}
		inheritProviderConfig(child, v)

		if child.ProviderAPIKey != "sk-parent" {
			t.Errorf("ProviderAPIKey = %q, want sk-parent", child.ProviderAPIKey)
		}
		if child.ProviderURL != "https://proxy.internal/v1" {
			t.Errorf("ProviderURL = %q", child.ProviderURL)
		}
		if !child.TLSSkipVerify {
			t.Error("TLSSkipVerify not propagated")
		}
		if child.ThinkingLevel != "high" {
			t.Errorf("ThinkingLevel = %q, want high", child.ThinkingLevel)
		}
		if child.MaxTokens != 4321 {
			t.Errorf("MaxTokens = %d, want 4321", child.MaxTokens)
		}
		if child.Temperature == nil || *child.Temperature != 0.25 {
			t.Errorf("Temperature = %v, want 0.25", child.Temperature)
		}
		if child.TopP == nil || *child.TopP != 0.9 {
			t.Errorf("TopP = %v, want 0.9", child.TopP)
		}
		if child.TopK == nil || *child.TopK != 40 {
			t.Errorf("TopK = %v, want 40", child.TopK)
		}
		if child.FrequencyPenalty == nil || *child.FrequencyPenalty != 0.1 {
			t.Errorf("FrequencyPenalty = %v, want 0.1", child.FrequencyPenalty)
		}
		if child.PresencePenalty == nil || *child.PresencePenalty != 0.2 {
			t.Errorf("PresencePenalty = %v, want 0.2", child.PresencePenalty)
		}
	})

	t.Run("unset tri-state keys stay unset", func(t *testing.T) {
		// A store with no sampler / max-tokens keys must leave the child's
		// pointers nil and MaxTokens zero so per-model defaults still apply.
		v := viper.New()
		child := &Options{}
		inheritProviderConfig(child, v)

		if child.MaxTokens != 0 {
			t.Errorf("MaxTokens = %d, want 0 (unset)", child.MaxTokens)
		}
		if child.Temperature != nil || child.TopP != nil || child.TopK != nil ||
			child.FrequencyPenalty != nil || child.PresencePenalty != nil {
			t.Error("sampler pointers must stay nil when the parent did not set them")
		}
		if child.ThinkingLevel != "" {
			t.Errorf("ThinkingLevel = %q, want empty", child.ThinkingLevel)
		}
	})

	t.Run("nil child or store is a no-op", func(t *testing.T) {
		inheritProviderConfig(nil, viper.New())
		inheritProviderConfig(&Options{}, nil)
	})
}

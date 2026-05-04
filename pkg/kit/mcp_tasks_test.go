package kit

import (
	"testing"
	"time"

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

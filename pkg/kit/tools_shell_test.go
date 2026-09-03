package kit

import (
	"slices"
	"testing"
	"time"

	"github.com/mark3labs/kit/internal/core"
)

// The exported surface of the shell tool: the current spelling, the earlier
// spelling kept as an alias, and the resolution order between them.

func TestFirstNonZero_CurrentSpellingWins(t *testing.T) {
	cases := []struct {
		name string
		in   []int
		want int
	}{
		{"shell only", []int{30, 0, 0, 0}, 30},
		{"bash only", []int{0, 45, 0, 0}, 45},
		{"shell beats bash at the SDK layer", []int{30, 45, 0, 0}, 30},
		{"config shell beats config bash", []int{0, 0, 60, 90}, 60},
		{"SDK beats config", []int{0, 45, 60, 90}, 45},
		{"none set", []int{0, 0, 0, 0}, 0},
	}
	for _, c := range cases {
		if got := firstNonZero(c.in...); got != c.want {
			t.Errorf("%s: firstNonZero(%v) = %d, want %d", c.name, c.in, got, c.want)
		}
	}
}

func TestToolOptionAliases_SetTheSameField(t *testing.T) {
	cfg := core.ApplyOptions([]core.ToolOption{WithShellTimeout(30 * time.Second)})
	if cfg.ShellTimeout != 30*time.Second {
		t.Errorf("WithShellTimeout: got %v, want 30s", cfg.ShellTimeout)
	}
	cfg = core.ApplyOptions([]core.ToolOption{WithBashTimeout(30 * time.Second)})
	if cfg.ShellTimeout != 30*time.Second {
		t.Errorf("WithBashTimeout: got %v, want 30s", cfg.ShellTimeout)
	}

	cfg = core.ApplyOptions([]core.ToolOption{WithShellMaxTimeout(90 * time.Second)})
	if cfg.ShellMaxTimeout != 90*time.Second {
		t.Errorf("WithShellMaxTimeout: got %v, want 90s", cfg.ShellMaxTimeout)
	}
	cfg = core.ApplyOptions([]core.ToolOption{WithBashMaxTimeout(90 * time.Second)})
	if cfg.ShellMaxTimeout != 90*time.Second {
		t.Errorf("WithBashMaxTimeout: got %v, want 90s", cfg.ShellMaxTimeout)
	}

	// Both are functional options, so the later one in the list wins. That is
	// the existing convention for every option in this package.
	cfg = core.ApplyOptions([]core.ToolOption{
		WithBashTimeout(30 * time.Second),
		WithShellTimeout(45 * time.Second),
	})
	if cfg.ShellTimeout != 45*time.Second {
		t.Errorf("later option should win: got %v, want 45s", cfg.ShellTimeout)
	}
}

func TestConstructorAlias_ProducesTheSameTool(t *testing.T) {
	if got := NewShellTool().Info().Name; got != "shell" {
		t.Errorf("NewShellTool name = %q, want \"shell\"", got)
	}
	if got := NewBashTool().Info().Name; got != "shell" {
		t.Errorf("NewBashTool name = %q, want \"shell\"", got)
	}
}

func TestWithShell_ReachesTheToolConfig(t *testing.T) {
	cfg := core.ApplyOptions([]core.ToolOption{WithShell([]string{"busybox", "ash"})})
	if !slices.Equal(cfg.Shell, []string{"busybox", "ash"}) {
		t.Errorf("Shell = %#v", cfg.Shell)
	}
}

func TestNormalizeCoreToolNames_MapsAndDeduplicates(t *testing.T) {
	got := normalizeCoreToolNames([]string{"bash", "shell", "read", "bash"})
	if !slices.Equal(got, []string{"shell", "read"}) {
		t.Errorf("normalizeCoreToolNames = %#v, want [shell read]", got)
	}
	if got := normalizeCoreToolNames(nil); got != nil {
		t.Errorf("nil input = %#v, want nil", got)
	}
}

func TestFilterCoreToolNames_AcceptsTheEarlierName(t *testing.T) {
	got, err := FilterCoreToolNames([]string{"bash"}, nil)
	if err != nil {
		t.Fatalf("include [bash]: %v", err)
	}
	if !slices.Equal(got, []string{"shell"}) {
		t.Errorf("include [bash] = %#v, want [shell]", got)
	}

	got, err = FilterCoreToolNames(nil, []string{"bash"})
	if err != nil {
		t.Fatalf("exclude [bash]: %v", err)
	}
	if slices.Contains(got, "shell") {
		t.Errorf("exclude [bash] still contains the shell tool: %#v", got)
	}
	if len(got) != len(ListAllCoreToolNames())-1 {
		t.Errorf("exclude [bash] produced %d tools, want %d", len(got), len(ListAllCoreToolNames())-1)
	}
}

func TestHandleCoreToolList_AcceptsTheEarlierName(t *testing.T) {
	// An SDK caller can pass CoreToolList directly, so this path has to accept
	// the earlier name too.
	got := handleCoreToolList([]string{"bash", "read"}, false)
	if !slices.Contains(got, "shell") {
		t.Errorf("handleCoreToolList([bash read]) = %#v, want it to contain shell", got)
	}
	if slices.Contains(got, "bash") {
		t.Errorf("handleCoreToolList returned the earlier name: %#v", got)
	}
}

func TestFilterToolsByName_AllowlistAcceptsTheEarlierName(t *testing.T) {
	// A named agent definition written before the rename says "bash". The
	// allowlist admits the mapped name beside the written one, so the shell
	// tool is selected; a custom tool genuinely named "bash" would match its
	// own entry as well.
	got := filterToolsByName(SubagentTools(), []string{"bash", "read"})
	names := make([]string, 0, len(got))
	for _, tool := range got {
		names = append(names, tool.Info().Name)
	}
	if !slices.Contains(names, "shell") {
		t.Errorf("allowlist [bash read] selected %v, want it to contain shell", names)
	}
	if !slices.Contains(names, "read") {
		t.Errorf("allowlist [bash read] selected %v, want it to contain read", names)
	}
	if len(got) != 2 {
		t.Errorf("allowlist [bash read] selected %d tools, want 2", len(got))
	}
}

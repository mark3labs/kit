package kit

import (
	"context"
	"strings"
	"testing"

	"github.com/mark3labs/kit/internal/core"
)

func TestSubagent_MaxSubagentsLimit(t *testing.T) {
	factory := func(_ context.Context, _ *ProviderConfig, name string) (*ProviderResult, error) {
		return &ProviderResult{Model: &echoModel{provider: "mock", model: name}}, nil
	}
	maxSubs := 2
	k := newProviderTestKit(t, &Options{
		Model:        "mock/test",
		Providers:    map[string]ProviderFactory{"mock": factory},
		MaxSubagents: &maxSubs,
	})

	ctx := context.Background()
	// Since maxSubs is 2, running 3 concurrently should not fail but simply queue.
	// We verify they all finish without the "limit reached" error.
	errCh := make(chan error, 3)
	for i := range 3 {
		go func(id int) {
			_, err := k.Subagent(ctx, SubagentConfig{Prompt: "test", NoSession: true, Tools: []Tool{}})
			errCh <- err
		}(i)
	}

	for range 3 {
		if err := <-errCh; err != nil {
			t.Fatalf("subagent spawn failed (queueing failed): %v", err)
		}
	}
}

func TestSubagent_MaxSubagentsDisabled(t *testing.T) {
	factory := func(_ context.Context, _ *ProviderConfig, name string) (*ProviderResult, error) {
		return &ProviderResult{Model: &echoModel{provider: "mock", model: name}}, nil
	}
	maxSubs := 0
	k := newProviderTestKit(t, &Options{
		Model:        "mock/test",
		Providers:    map[string]ProviderFactory{"mock": factory},
		MaxSubagents: &maxSubs,
	})

	ctx := context.Background()
	_, err := k.Subagent(ctx, SubagentConfig{Prompt: "test 1", NoSession: true, Tools: []Tool{}})
	if err == nil {
		t.Fatal("expected error when subagents are disabled, got nil")
	}
	if !strings.Contains(err.Error(), "subagents are disabled") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestSubagent_MaxNestedSubagentsLimit(t *testing.T) {
	factory := func(_ context.Context, _ *ProviderConfig, name string) (*ProviderResult, error) {
		return &ProviderResult{Model: &echoModel{provider: "mock", model: name}}, nil
	}
	maxNested := 2
	k := newProviderTestKit(t, &Options{
		Model:              "mock/test",
		Providers:          map[string]ProviderFactory{"mock": factory},
		SubagentDepth:      1,
		MaxNestedSubagents: &maxNested,
	})

	ctx := context.Background()
	// Since maxNested is 2, running 3 concurrently should not fail but simply queue.
	errCh := make(chan error, 3)
	for i := range 3 {
		go func(id int) {
			_, err := k.Subagent(ctx, SubagentConfig{Prompt: "nested", NoSession: true, Tools: []Tool{}})
			errCh <- err
		}(i)
	}

	for range 3 {
		if err := <-errCh; err != nil {
			t.Fatalf("nested subagent spawn failed (queueing failed): %v", err)
		}
	}
}

func TestSubagent_NestedSubagentsDisabled(t *testing.T) {
	factory := func(_ context.Context, _ *ProviderConfig, name string) (*ProviderResult, error) {
		return &ProviderResult{Model: &echoModel{provider: "mock", model: name}}, nil
	}
	zeroNested := 0
	k := newProviderTestKit(t, &Options{
		Model:              "mock/test",
		Providers:          map[string]ProviderFactory{"mock": factory},
		SubagentDepth:      1,
		MaxNestedSubagents: &zeroNested,
	})

	ctx := context.Background()
	_, err := k.Subagent(ctx, SubagentConfig{Prompt: "nested 1", NoSession: true, Tools: []Tool{}})
	if err == nil {
		t.Fatal("expected error when nested subagents are disabled, got nil")
	}
	if !strings.Contains(err.Error(), "nested subagents are disabled") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestSubagent_Depth2CannotSpawn(t *testing.T) {
	factory := func(_ context.Context, _ *ProviderConfig, name string) (*ProviderResult, error) {
		return &ProviderResult{Model: &echoModel{provider: "mock", model: name}}, nil
	}
	k := newProviderTestKit(t, &Options{
		Model:         "mock/test",
		Providers:     map[string]ProviderFactory{"mock": factory},
		SubagentDepth: 2,
	})

	ctx := context.Background()
	_, err := k.Subagent(ctx, SubagentConfig{Prompt: "sub-nested", NoSession: true, Tools: []Tool{}})
	if err == nil {
		t.Fatal("expected error for depth >= 2 subagent spawn, got nil")
	}
	if !strings.Contains(err.Error(), "cannot spawn further subagents") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestSubagent_UnlimitedSubagents(t *testing.T) {
	factory := func(_ context.Context, _ *ProviderConfig, name string) (*ProviderResult, error) {
		return &ProviderResult{Model: &echoModel{provider: "mock", model: name}}, nil
	}
	unlimited := -1
	k := newProviderTestKit(t, &Options{
		Model:        "mock/test",
		Providers:    map[string]ProviderFactory{"mock": factory},
		MaxSubagents: &unlimited,
	})

	ctx := context.Background()
	for i := range 15 {
		_, err := k.Subagent(ctx, SubagentConfig{Prompt: "unlimited", NoSession: true, Tools: []Tool{}})
		if err != nil {
			t.Fatalf("subagent spawn %d failed: %v", i+1, err)
		}
	}
}

func TestGetToolsForSubagent_Limits(t *testing.T) {
	factory := func(_ context.Context, _ *ProviderConfig, name string) (*ProviderResult, error) {
		return &ProviderResult{Model: &echoModel{provider: "mock", model: name}}, nil
	}

	subTool := core.NewSubagentTool()
	dummyTool := NewTool("dummy", "A dummy tool", func(ctx context.Context, input struct{}) (ToolOutput, error) {
		return TextResult("dummy"), nil
	})

	hasTool := func(tools []Tool, name string) bool {
		for _, tool := range tools {
			if tool.Info().Name == name {
				return true
			}
		}
		return false
	}

	// 1. By default, nested subagents are enabled (default 3), so depth 0 includes subagent.
	kDefault := newProviderTestKit(t, &Options{
		Model:     "mock/test",
		Providers: map[string]ProviderFactory{"mock": factory},
		Tools:     []Tool{subTool, dummyTool},
	})
	toolsDefault := kDefault.GetToolsForSubagent()
	if !hasTool(toolsDefault, "subagent") {
		t.Errorf("expected GetToolsForSubagent() to include subagent by default")
	}
	if !hasTool(toolsDefault, "dummy") {
		t.Errorf("expected GetToolsForSubagent() to include dummy tool")
	}

	// 2. When max-nested-subagents is 0 (disabled), subagent is excluded.
	zeroNested := 0
	kNoNested := newProviderTestKit(t, &Options{
		Model:              "mock/test",
		Providers:          map[string]ProviderFactory{"mock": factory},
		Tools:              []Tool{subTool, dummyTool},
		MaxNestedSubagents: &zeroNested,
	})
	toolsNoNested := kNoNested.GetToolsForSubagent()
	if hasTool(toolsNoNested, "subagent") {
		t.Errorf("expected GetToolsForSubagent() to exclude subagent when max-nested-subagents=0")
	}
	if !hasTool(toolsNoNested, "dummy") {
		t.Errorf("expected GetToolsForSubagent() to include dummy tool")
	}

	// 3. When depth >= 1, subagent is excluded to prevent recursion beyond depth 2.
	kDepth1 := newProviderTestKit(t, &Options{
		Model:         "mock/test",
		Providers:     map[string]ProviderFactory{"mock": factory},
		Tools:         []Tool{subTool, dummyTool},
		SubagentDepth: 1,
	})
	toolsDepth1 := kDepth1.GetToolsForSubagent()
	if hasTool(toolsDepth1, "subagent") {
		t.Errorf("expected GetToolsForSubagent() to exclude subagent for depth >= 1")
	}
	if !hasTool(toolsDepth1, "dummy") {
		t.Errorf("expected GetToolsForSubagent() to include dummy tool")
	}
}

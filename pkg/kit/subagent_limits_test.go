package kit

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mark3labs/kit/internal/core"
)

type blockingModel struct {
	*echoModel
	started chan struct{}
	release chan struct{}
}

func (m *blockingModel) Generate(ctx context.Context, call LLMCall) (*LLMResponse, error) {
	select {
	case m.started <- struct{}{}:
	default:
	}
	select {
	case <-m.release:
		return m.echoModel.Generate(ctx, call)
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (m *blockingModel) Stream(ctx context.Context, call LLMCall) (LLMStreamResponse, error) {
	select {
	case m.started <- struct{}{}:
	default:
	}
	select {
	case <-m.release:
		return m.echoModel.Stream(ctx, call)
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func TestSubagent_MaxSubagentsLimit(t *testing.T) {
	started := make(chan struct{}, 10)
	release := make(chan struct{})
	slotWaiting := make(chan struct{}, 10)

	factory := func(_ context.Context, _ *ProviderConfig, name string) (*ProviderResult, error) {
		return &ProviderResult{Model: &blockingModel{echoModel: &echoModel{provider: "mock", model: name}, started: started, release: release}}, nil
	}
	maxSubs := 2
	otherNested := 3
	k := newProviderTestKit(t, &Options{
		Model:              "mock/test",
		Providers:          map[string]ProviderFactory{"mock": factory},
		MaxSubagents:       &maxSubs,
		MaxNestedSubagents: &otherNested,
		onSubagentSlotWait: func() { slotWaiting <- struct{}{} },
	})

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	var releaseOnce sync.Once
	t.Cleanup(func() {
		releaseOnce.Do(func() { close(release) })
		cancel()
		wg.Wait()
	})

	errCh := make(chan error, 3)

	for i := range 3 {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			_, err := k.Subagent(ctx, SubagentConfig{Prompt: "test", NoSession: true, Tools: []Tool{}})
			errCh <- err
		}(i)
	}

	// 1. Verify that exactly maxSubs (2) subagents become active.
	for i := range maxSubs {
		select {
		case <-started:
		case <-time.After(1 * time.Second):
			t.Fatalf("expected subagent %d to become active, but it blocked", i+1)
		}
	}

	// 2. Verify the 3rd request reaches the limiter's wait point while both model calls remain blocked.
	select {
	case <-slotWaiting:
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for 3rd subagent to reach the limiter's wait point")
	}

	finished := 0
	sendRelease := func() {
		t.Helper()
		for {
			select {
			case release <- struct{}{}:
				return
			case err := <-errCh:
				finished++
				if err != nil {
					t.Fatalf("worker failed: %v", err)
				}
			case <-time.After(1 * time.Second):
				t.Fatal("timed out sending to release channel")
			}
		}
	}

	// 3. Release one active subagent.
	sendRelease()

	// 4. Verify the 3rd subagent now unblocks and becomes active.
	select {
	case <-started:
	case <-time.After(1 * time.Second):
		t.Fatalf("3rd subagent did not become active after a slot was released")
	}

	// Release the remaining two.
	sendRelease()
	sendRelease()

	// Ensure all finished without error.
	for finished < 3 {
		select {
		case err := <-errCh:
			finished++
			if err != nil {
				t.Fatalf("subagent spawn failed (queueing failed): %v", err)
			}
		case <-time.After(1 * time.Second):
			t.Fatalf("subagent did not return")
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
	started := make(chan struct{}, 10)
	release := make(chan struct{})
	slotWaiting := make(chan struct{}, 10)

	factory := func(_ context.Context, _ *ProviderConfig, name string) (*ProviderResult, error) {
		return &ProviderResult{Model: &blockingModel{echoModel: &echoModel{provider: "mock", model: name}, started: started, release: release}}, nil
	}
	otherSubs := 3
	maxNested := 2
	k := newProviderTestKit(t, &Options{
		Model:              "mock/test",
		Providers:          map[string]ProviderFactory{"mock": factory},
		SubagentDepth:      1,
		MaxSubagents:       &otherSubs,
		MaxNestedSubagents: &maxNested,
		onSubagentSlotWait: func() { slotWaiting <- struct{}{} },
	})

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	var releaseOnce sync.Once
	t.Cleanup(func() {
		releaseOnce.Do(func() { close(release) })
		cancel()
		wg.Wait()
	})

	errCh := make(chan error, 3)

	for i := range 3 {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			_, err := k.Subagent(ctx, SubagentConfig{Prompt: "nested", NoSession: true, Tools: []Tool{}})
			errCh <- err
		}(i)
	}

	for i := range maxNested {
		select {
		case <-started:
		case <-time.After(1 * time.Second):
			t.Fatalf("expected nested subagent %d to become active, but it blocked", i+1)
		}
	}

	// Verify the 3rd request reaches the limiter's wait point while both model calls remain blocked.
	select {
	case <-slotWaiting:
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for 3rd nested subagent to reach the limiter's wait point")
	}

	finished := 0
	sendRelease := func() {
		t.Helper()
		for {
			select {
			case release <- struct{}{}:
				return
			case err := <-errCh:
				finished++
				if err != nil {
					t.Fatalf("worker failed: %v", err)
				}
			case <-time.After(1 * time.Second):
				t.Fatal("timed out sending to release channel")
			}
		}
	}

	sendRelease()

	select {
	case <-started:
	case <-time.After(1 * time.Second):
		t.Fatalf("3rd nested subagent did not become active after a slot was released")
	}

	sendRelease()
	sendRelease()

	for finished < 3 {
		select {
		case err := <-errCh:
			finished++
			if err != nil {
				t.Fatalf("nested subagent spawn failed (queueing failed): %v", err)
			}
		case <-time.After(1 * time.Second):
			t.Fatalf("nested subagent did not return")
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
	started := make(chan struct{}, 15)
	release := make(chan struct{})

	factory := func(_ context.Context, _ *ProviderConfig, name string) (*ProviderResult, error) {
		return &ProviderResult{Model: &blockingModel{echoModel: &echoModel{provider: "mock", model: name}, started: started, release: release}}, nil
	}
	unlimited := -1
	k := newProviderTestKit(t, &Options{
		Model:        "mock/test",
		Providers:    map[string]ProviderFactory{"mock": factory},
		MaxSubagents: &unlimited,
	})

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	var releaseOnce sync.Once
	openRelease := func() {
		releaseOnce.Do(func() { close(release) })
	}
	t.Cleanup(func() {
		openRelease()
		cancel()
		wg.Wait()
	})

	errCh := make(chan error, 15)

	for i := range 15 {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			_, err := k.Subagent(ctx, SubagentConfig{Prompt: "unlimited", NoSession: true, Tools: []Tool{}})
			errCh <- err
		}(i)
	}

	// Verify all 15 become active immediately because limits are unbounded
	for i := range 15 {
		select {
		case <-started:
		case <-time.After(1 * time.Second):
			t.Fatalf("expected subagent %d to become active, but it blocked", i+1)
		}
	}

	openRelease()

	for range 15 {
		select {
		case err := <-errCh:
			if err != nil {
				t.Fatalf("subagent spawn failed: %v", err)
			}
		case <-time.After(1 * time.Second):
			t.Fatalf("subagent did not return")
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

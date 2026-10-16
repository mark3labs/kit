package skilltool

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"charm.land/fantasy"
	"github.com/mark3labs/kit/internal/skills"
)

func TestSnapshotVersionAndEligibility(t *testing.T) {
	var mu sync.RWMutex
	current := []*skills.Skill{{Name: "demo", Content: "v1"}}
	provider := func() []*skills.Skill { mu.RLock(); defer mu.RUnlock(); return skills.CloneAll(current) }
	root := New(nil, provider).(*activateSkillTool)
	selected := root.Snapshot()
	call := fantasy.ToolCall{Input: `{"name":"demo"}`}
	run := func(tool fantasy.AgentTool) string {
		t.Helper()
		r, e := tool.Run(context.Background(), call)
		if e != nil {
			t.Fatal(e)
		}
		return r.Content
	}
	if !strings.Contains(run(selected), "v1") {
		t.Fatal("missing v1")
	}
	mu.Lock()
	current[0] = &skills.Skill{Name: "demo", Content: "v2"}
	mu.Unlock()
	if out := run(selected); !strings.Contains(out, "fresh request") || strings.Contains(out, "v2") {
		t.Fatalf("stale output=%s", out)
	}
	next := root.Snapshot()
	if !strings.Contains(run(next), "v2") || !strings.Contains(run(next), "already loaded") {
		t.Fatal("version dedup failed")
	}
	for _, list := range [][]*skills.Skill{{{Name: "demo", Content: "v2", DisableModelInvocation: true}}, nil} {
		mu.Lock()
		current = list
		mu.Unlock()
		if root.Snapshot() != nil {
			t.Fatal("ineligible catalogue exposed")
		}
		if out := run(next); !strings.Contains(out, "unknown skill") {
			t.Fatalf("membership checked after dedup: %s", out)
		}
	}
	mu.Lock()
	current = []*skills.Skill{{Name: "demo", Content: "v3"}}
	mu.Unlock()
	if root.Snapshot() == nil {
		t.Fatal("empty-to-nonempty did not restore tool")
	}
}

func TestSnapshotConcurrentActivation(t *testing.T) {
	root := New(nil, func() []*skills.Skill { return []*skills.Skill{{Name: "demo", Content: "unique body"}} }).(*activateSkillTool)
	tool := root.Snapshot()
	var bodies atomic.Int32
	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() {
			r, e := tool.Run(context.Background(), fantasy.ToolCall{Input: `{"name":"demo"}`})
			if e != nil {
				t.Error(e)
			}
			if strings.Contains(r.Content, "unique body") {
				bodies.Add(1)
			}
		})
	}
	wg.Wait()
	if bodies.Load() != 1 {
		t.Fatalf("bodies=%d", bodies.Load())
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := tool.Run(ctx, fantasy.ToolCall{Input: `{"name":"demo"}`}); err != context.Canceled {
		t.Fatalf("error=%v", err)
	}
}

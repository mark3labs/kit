package kit

import (
	"context"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mark3labs/kit/internal/session"
)

// Tests for issue #87: resumable subagent sessions and parent-child session
// linking. These validate the fast-fail paths in Kit.Subagent, which run
// before any provider/child initialization.

func TestSubagent_SessionIDConflictsWithNoSession(t *testing.T) {
	m := &Kit{}
	_, err := m.Subagent(context.Background(), SubagentConfig{
		Prompt:    "task",
		SessionID: "some-session",
		NoSession: true,
	})
	if err == nil {
		t.Fatal("expected error when both SessionID and NoSession are set")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("error = %v, want mention of mutual exclusivity", err)
	}
}

func TestSubagent_UnknownSessionIDFailsFast(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // isolate from real ~/.kit/sessions

	m := &Kit{}
	_, err := m.Subagent(context.Background(), SubagentConfig{
		Prompt:    "task",
		SessionID: "does-not-exist",
	})
	if err == nil {
		t.Fatal("expected error for unknown subagent session ID")
	}
	if !strings.Contains(err.Error(), "cannot resume subagent session") {
		t.Errorf("error = %v, want resume-failure context", err)
	}
}

func TestSubagent_ParentSessionLinkCapturesBeforeSlotWait(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	started := make(chan struct{}, 10)
	release := make(chan struct{})
	slotWaiting := make(chan struct{}, 10)

	factory := func(_ context.Context, _ *ProviderConfig, name string) (*ProviderResult, error) {
		return &ProviderResult{Model: &blockingModel{echoModel: &echoModel{provider: "mock", model: name}, started: started, release: release}}, nil
	}

	maxSubs := 1
	otherNested := 1
	sessionDir := t.TempDir()
	k, err := New(context.Background(), &Options{
		Model:              "mock/test",
		Providers:          map[string]ProviderFactory{"mock": factory},
		SessionDir:         sessionDir,
		NoSession:          false,
		Quiet:              true,
		NoExtensions:       true,
		DisableCoreTools:   true,
		SkipConfig:         true,
		MaxSubagents:       &maxSubs,
		MaxNestedSubagents: &otherNested,
		onSubagentSlotWait: func() { slotWaiting <- struct{}{} },
	})
	if err != nil {
		t.Fatalf("kit.New: %v", err)
	}
	defer func() { _ = k.Close() }()

	parentPath1 := k.GetSessionPath()
	parentID1 := k.GetSessionID()
	if parentPath1 == "" || parentID1 == "" {
		t.Fatalf("expected initial session to be persisted, got path=%q id=%q", parentPath1, parentID1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup

	// 1. Start subagent 1 to occupy the slot.
	wg.Go(func() {
		_, _ = k.Subagent(ctx, SubagentConfig{Prompt: "task 1", Tools: []Tool{}})
	})

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("subagent 1 did not start")
	}

	// 2. Start subagent 2 which will wait for slot.
	var res2 *SubagentResult
	var err2 error
	wg.Go(func() {
		res2, err2 = k.Subagent(ctx, SubagentConfig{Prompt: "task 2", Tools: []Tool{}})
	})

	select {
	case <-slotWaiting:
	case <-time.After(2 * time.Second):
		t.Fatal("subagent 2 did not reach slot wait")
	}

	// 3. While subagent 2 is waiting in acquireSubagentSlot, switch the parent session.
	tm2, err := session.CreateTreeSession(t.TempDir())
	if err != nil {
		t.Fatalf("CreateTreeSession: %v", err)
	}
	defer func() { _ = tm2.Close() }()
	k.SetTreeSession(tm2)

	parentPath2 := k.GetSessionPath()
	parentID2 := k.GetSessionID()
	if parentID2 == parentID1 {
		t.Fatal("session switch did not change parent session ID")
	}

	// 4. Unblock subagent 1 so subagent 2 can acquire slot.
	release <- struct{}{}

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("subagent 2 did not start")
	}

	// Unblock subagent 2.
	release <- struct{}{}

	wg.Wait()

	if err2 != nil {
		t.Fatalf("subagent 2 failed: %v", err2)
	}
	if res2 == nil || res2.SessionID == "" {
		t.Fatalf("subagent 2 returned invalid result: %+v", res2)
	}

	// 5. Verify registry entry has parentID1, not parentID2.
	var run2 *SubagentRun
	for _, r := range k.SubagentRuns() {
		if r.Prompt == "task 2" {
			runCopy := r
			run2 = &runCopy
			break
		}
	}
	if run2 == nil {
		t.Fatal("subagent 2 run not found in registry")
	}
	if run2.ParentSessionID != parentID1 {
		t.Errorf("registry run.ParentSessionID = %q, want %q (got switched ID %q)", run2.ParentSessionID, parentID1, parentID2)
	}

	// 6. Verify child session file has parentID1 and parentPath1, not parentID2/parentPath2.
	cwd, _ := os.Getwd()
	childPath, err := session.FindSessionPathByID(cwd, res2.SessionID)
	if err != nil {
		t.Fatalf("FindSessionPathByID for child: %v", err)
	}
	childSession, err := session.OpenTreeSession(childPath)
	if err != nil {
		t.Fatalf("OpenTreeSession for child: %v", err)
	}
	defer func() { _ = childSession.Close() }()

	childHeader := childSession.GetHeader()
	if childHeader.ParentSessionID != parentID1 {
		t.Errorf("child header ParentSessionID = %q, want %q (got switched ID %q)", childHeader.ParentSessionID, parentID1, parentID2)
	}
	if childHeader.ParentSession != parentPath1 {
		t.Errorf("child header ParentSession = %q, want %q (got switched path %q)", childHeader.ParentSession, parentPath1, parentPath2)
	}
}

func TestSubagent_ParentSessionIDOverride(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	factory := func(_ context.Context, _ *ProviderConfig, name string) (*ProviderResult, error) {
		return &ProviderResult{Model: &echoModel{provider: "mock", model: name}}, nil
	}

	sessionDir := t.TempDir()
	k, err := New(context.Background(), &Options{
		Model:            "mock/test",
		Providers:        map[string]ProviderFactory{"mock": factory},
		SessionDir:       sessionDir,
		NoSession:        false,
		Quiet:            true,
		NoExtensions:     true,
		DisableCoreTools: true,
		SkipConfig:       true,
	})
	if err != nil {
		t.Fatalf("kit.New: %v", err)
	}
	defer func() { _ = k.Close() }()

	const overrideID = "custom-parent-session-id"
	res, err := k.Subagent(context.Background(), SubagentConfig{
		Prompt:          "override task",
		ParentSessionID: overrideID,
		Tools:           []Tool{},
	})
	if err != nil {
		t.Fatalf("Subagent: %v", err)
	}

	// 1. Verify registry entry has the override ID.
	var run *SubagentRun
	for _, r := range k.SubagentRuns() {
		if r.Prompt == "override task" {
			runCopy := r
			run = &runCopy
			break
		}
	}
	if run == nil {
		t.Fatal("subagent run not found in registry")
	}
	if run.ParentSessionID != overrideID {
		t.Errorf("registry run.ParentSessionID = %q, want %q", run.ParentSessionID, overrideID)
	}

	// 2. Verify child session file header has the override ID and parent path.
	cwd, _ := os.Getwd()
	childPath, err := session.FindSessionPathByID(cwd, res.SessionID)
	if err != nil {
		t.Fatalf("FindSessionPathByID for child: %v", err)
	}
	childSession, err := session.OpenTreeSession(childPath)
	if err != nil {
		t.Fatalf("OpenTreeSession for child: %v", err)
	}
	defer func() { _ = childSession.Close() }()

	childHeader := childSession.GetHeader()
	if childHeader.ParentSessionID != overrideID {
		t.Errorf("child header ParentSessionID = %q, want %q", childHeader.ParentSessionID, overrideID)
	}
	if childHeader.ParentSession != k.GetSessionPath() {
		t.Errorf("child header ParentSession = %q, want %q", childHeader.ParentSession, k.GetSessionPath())
	}
}

func TestSubagent_EphemeralParentAutomaticLinkingSkipped(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	factory := func(_ context.Context, _ *ProviderConfig, name string) (*ProviderResult, error) {
		return &ProviderResult{Model: &echoModel{provider: "mock", model: name}}, nil
	}

	k, err := New(context.Background(), &Options{
		Model:            "mock/test",
		Providers:        map[string]ProviderFactory{"mock": factory},
		NoSession:        false,
		Quiet:            true,
		NoExtensions:     true,
		DisableCoreTools: true,
		SkipConfig:       true,
	})
	if err != nil {
		t.Fatalf("kit.New: %v", err)
	}
	defer func() { _ = k.Close() }()

	// Set an in-memory session (no file path) on the parent kit.
	memSession := session.InMemoryTreeSession("ephemeral-test")
	k.SetTreeSession(memSession)

	if k.GetSessionPath() != "" {
		t.Fatalf("expected in-memory session path to be empty, got %q", k.GetSessionPath())
	}
	parentID := k.GetSessionID()
	if parentID == "" {
		t.Fatal("expected in-memory session ID to be non-empty")
	}

	res, err := k.Subagent(context.Background(), SubagentConfig{
		Prompt: "ephemeral parent task",
		Tools:  []Tool{},
	})
	if err != nil {
		t.Fatalf("Subagent: %v", err)
	}

	// 1. Verify registry entry captured the in-memory parent session ID.
	var run *SubagentRun
	for _, r := range k.SubagentRuns() {
		if r.Prompt == "ephemeral parent task" {
			runCopy := r
			run = &runCopy
			break
		}
	}
	if run == nil {
		t.Fatal("subagent run not found in registry")
	}
	if run.ParentSessionID != parentID {
		t.Errorf("registry run.ParentSessionID = %q, want %q", run.ParentSessionID, parentID)
	}

	// 2. Verify automatic linking is skipped in the child session file header.
	cwd, _ := os.Getwd()
	childPath, err := session.FindSessionPathByID(cwd, res.SessionID)
	if err != nil {
		t.Fatalf("FindSessionPathByID for child: %v", err)
	}
	childSession, err := session.OpenTreeSession(childPath)
	if err != nil {
		t.Fatalf("OpenTreeSession for child: %v", err)
	}
	defer func() { _ = childSession.Close() }()

	childHeader := childSession.GetHeader()
	if childHeader.ParentSessionID != "" {
		t.Errorf("child header ParentSessionID = %q, want empty (linking should be skipped for ephemeral parent)", childHeader.ParentSessionID)
	}
	if childHeader.ParentSession != "" {
		t.Errorf("child header ParentSession = %q, want empty (linking should be skipped for ephemeral parent)", childHeader.ParentSession)
	}
}

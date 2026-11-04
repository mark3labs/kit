package core

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"charm.land/fantasy"

	"github.com/mark3labs/kit/internal/jobs"
)

type fakeJobService struct {
	*jobs.Manager
	starts   int
	yieldIDs []string
	yieldAll bool
	yieldErr error
	startErr error
}

// Start counts admission attempts and either returns the configured error or
// starts work with context values preserved and turn cancellation removed.
func (s *fakeJobService) Start(ctx context.Context, kind string, work func(context.Context) (string, error)) (jobs.Job, error) {
	s.starts++
	if s.startErr != nil {
		return jobs.Job{}, s.startErr
	}
	return s.Manager.Start(context.WithoutCancel(ctx), kind, "session", "branch", jobs.Work(work))
}

// Yield copies the requested wait IDs and mode and returns the configured scheduler error.
func (s *fakeJobService) Yield(_ context.Context, ids []string, all bool) error {
	s.yieldIDs = slices.Clone(ids)
	s.yieldAll = all
	return s.yieldErr
}

// testJobService creates a fake service and closes its manager during test cleanup.
func testJobService(t *testing.T) *fakeJobService {
	t.Helper()
	s := &fakeJobService{Manager: jobs.New()}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Error(err)
		}
	})
	return s
}

// runJobTool invokes a tool with the supplied JSON input and fails the test on a Go error.
func runJobTool(t *testing.T, ctx context.Context, tool fantasy.AgentTool, input string) fantasy.ToolResponse {
	t.Helper()
	response, err := tool.Run(ctx, fantasy.ToolCall{ID: "job-call", Input: input})
	if err != nil {
		t.Fatal(err)
	}
	return response
}

// startedJob decodes a successful tool response and requires a nonempty job ID.
func startedJob(t *testing.T, response fantasy.ToolResponse) jobs.Job {
	t.Helper()
	if response.IsError {
		t.Fatal(response.Content)
	}
	var job jobs.Job
	if err := json.Unmarshal([]byte(response.Content), &job); err != nil {
		t.Fatal(err)
	}
	if job.ID == "" {
		t.Fatal("missing job ID")
	}
	return job
}

// waitJob waits up to three seconds for a terminal job snapshot or fails the test.
func waitJob(t *testing.T, s *fakeJobService, id string) jobs.Job {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	job, err := s.Wait(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	return job
}

// TestBackgroundShellUsesForegroundOptions checks that background shell work
// keeps runner options and timeout limits, survives turn cancellation, and does
// not use the turn output callback.
func TestBackgroundShellUsesForegroundOptions(t *testing.T) {
	s := testJobService(t)
	r := &fakeRunner{res: CommandResult{Output: "result"}}
	tool := NewShellTool(WithCommandRunner(r), WithWorkDir("/work"), WithShell([]string{"sh"}), WithShellTimeout(7*time.Second), WithShellMaxTimeout(9*time.Second))
	ctx, cancel := context.WithCancel(WithJobService(context.Background(), s))
	ctx = ContextWithToolOutputCallback(ctx, func(_, _, _ string, _ bool) { t.Error("stale output callback") })
	job := startedJob(t, runJobTool(t, ctx, tool, `{"command":"echo result","background":true,"timeout":99}`))
	cancel()
	done := waitJob(t, s, job.ID)
	if done.Status != jobs.Completed || done.Output != "result" {
		t.Fatalf("job = %+v", done)
	}
	if r.req.WorkDir != "/work" || r.req.Timeout != 9*time.Second || !reflect.DeepEqual(r.req.Argv, []string{"sh", "-c", "echo result"}) {
		t.Fatalf("request = %+v", r.req)
	}
}

// TestBackgroundShellBufferedAndFailure checks that shell output is buffered
// without turn streaming and that a nonzero exit records a failed job.
func TestBackgroundShellBufferedAndFailure(t *testing.T) {
	for _, tc := range []struct {
		command string
		status  jobs.Status
		output  string
	}{
		{"printf hello", jobs.Completed, "hello"},
		{"printf bad; exit 2", jobs.Failed, "Exit code: 2"},
	} {
		t.Run(tc.command, func(t *testing.T) {
			s := testJobService(t)
			ctx := ContextWithToolOutputCallback(WithJobService(context.Background(), s), func(_, _, _ string, _ bool) { t.Error("stale stream") })
			input, err := json.Marshal(shellArgs{Command: tc.command, Background: true})
			if err != nil {
				t.Fatal(err)
			}
			job := startedJob(t, runJobTool(t, ctx, NewShellTool(), string(input)))
			done := waitJob(t, s, job.ID)
			if done.Status != tc.status || !strings.Contains(done.Output, tc.output) {
				t.Fatalf("job = %+v", done)
			}
		})
	}
}

// TestBackgroundSubagentCancellationAndValues checks that background subagents
// keep request fields and context values, ignore turn cancellation, and stop
// when their job is cancelled.
func TestBackgroundSubagentCancellationAndValues(t *testing.T) {
	s := testJobService(t)
	entered := make(chan context.Context, 1)
	exited := make(chan struct{})
	type valueKey struct{}
	ctx, cancel := context.WithCancel(WithJobService(context.WithValue(context.Background(), valueKey{}, "kept"), s))
	defer cancel()
	ctx = ContextWithToolOutputCallback(ctx, func(_, _, _ string, _ bool) { t.Error("stale stream") })
	ctx = WithSubagentSpawner(ctx, func(ctx context.Context, req SubagentSpawnRequest) (*SubagentSpawnResult, error) {
		defer close(exited)
		if req.Prompt != "task" || req.Agent != "agent" || req.Timeout != 12*time.Second || req.SessionID != "resume" {
			t.Errorf("request = %+v", req)
		}
		entered <- ctx
		<-ctx.Done()
		return nil, ctx.Err()
	})
	job := startedJob(t, runJobTool(t, ctx, NewSubagentTool(), `{"task":"task","agent":"agent","session_id":"resume","timeout_seconds":12,"background":true}`))
	cancel()
	var spawnCtx context.Context
	select {
	case spawnCtx = <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("spawner did not start")
	}
	if spawnCtx.Err() != nil || spawnCtx.Value(valueKey{}) != "kept" || getSubagentSpawner(spawnCtx) == nil || toolOutputCallbackFromContext(spawnCtx) != nil {
		t.Fatal("incorrect background context")
	}
	if err := s.Cancel(job.ID); err != nil {
		t.Fatal(err)
	}
	select {
	case <-exited:
	case <-time.After(3 * time.Second):
		t.Fatal("job cancellation did not reach spawner")
	}
	if done := waitJob(t, s, job.ID); done.Status != jobs.Cancelled {
		t.Fatalf("job = %+v", done)
	}
}

// TestBackgroundValidationBeforeStart checks that invalid shell and subagent
// requests are rejected before the service admits work.
func TestBackgroundValidationBeforeStart(t *testing.T) {
	s := testJobService(t)
	ctx := WithSubagentSpawner(WithJobService(context.Background(), s), func(context.Context, SubagentSpawnRequest) (*SubagentSpawnResult, error) {
		t.Fatal("unexpected spawn")
		return nil, nil
	})
	for _, tc := range []struct {
		tool  fantasy.AgentTool
		input string
	}{
		{NewShellTool(), `{"background":true}`},
		{NewShellTool(), `{"command":12,"background":true}`},
		{NewShellTool(), `{"command":"echo ok","background":"true"}`},
		{NewShellTool(), `{"command":"echo ok","background":true`},
		{NewShellTool(), `{"command":"alias x=y","background":true}`},
		{NewShellTool(WithShell([]string{""})), `{"command":"echo ok","background":true}`},
		{NewSubagentTool(), `{"background":true}`},
		{NewSubagentTool(), `{"task":12,"background":true}`},
		{NewSubagentTool(), `{"task":"task","background":true,"timeout_seconds":"bad"}`},
	} {
		if response := runJobTool(t, ctx, tc.tool, tc.input); !response.IsError {
			t.Errorf("accepted %s", tc.input)
		}
	}
	if s.starts != 0 {
		t.Fatalf("started %d invalid jobs", s.starts)
	}
}

// TestJobToolsAndYield checks job listing, lookup, cancellation, and argument
// errors, and verifies that jobs_wait registers a scheduler yield without
// waiting for the running worker.
func TestJobToolsAndYield(t *testing.T) {
	s := testJobService(t)
	ctx := WithJobService(context.Background(), s)
	job, err := s.Start(ctx, "test", func(ctx context.Context) (string, error) { <-ctx.Done(); return "", ctx.Err() })
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range []fantasy.AgentTool{NewJobsListTool(), NewJobsGetTool()} {
		response := runJobTool(t, ctx, tool, `{"id":"`+job.ID+`"}`)
		if response.IsError || !strings.Contains(response.Content, job.ID) {
			t.Fatalf("response = %+v", response)
		}
	}
	response := runJobTool(t, ctx, NewJobsWaitTool(), `{"ids":["`+job.ID+`"],"all":true}`)
	if response.IsError || !reflect.DeepEqual(s.yieldIDs, []string{job.ID}) || !s.yieldAll {
		t.Fatalf("yield = %+v, %+v", response, s)
	}
	// The worker is still running: jobs_wait must not block on WaitAny.
	if snapshot, err := s.Get(job.ID); err != nil || snapshot.Status != jobs.Running {
		t.Fatalf("get = %+v, %v", snapshot, err)
	}
	response = runJobTool(t, ctx, NewJobsCancelTool(), `{"id":"`+job.ID+`"}`)
	if response.IsError {
		t.Fatal(response.Content)
	}
	if done := waitJob(t, s, job.ID); done.Status != jobs.Cancelled {
		t.Fatalf("job = %+v", done)
	}
	s.yieldErr = errors.New("scheduler failure")
	if response := runJobTool(t, ctx, NewJobsWaitTool(), `{"ids":["1"]}`); !response.IsError || !strings.Contains(response.Content, "scheduler failure") {
		t.Fatalf("response = %+v", response)
	}
	for _, tool := range []fantasy.AgentTool{NewJobsGetTool(), NewJobsCancelTool(), NewJobsWaitTool()} {
		for _, input := range []string{`{}`, `{"id":"missing","ids":["missing"]}`, `{"id":1}`, `{`} {
			if response := runJobTool(t, ctx, tool, input); !response.IsError {
				t.Errorf("%s accepted %s", tool.Info().Name, input)
			}
		}
	}
}

// TestJobServiceUnavailableAndStartError checks tool errors for a missing job
// service and for a service that rejects work at capacity.
func TestJobServiceUnavailableAndStartError(t *testing.T) {
	for _, tool := range []fantasy.AgentTool{NewJobsListTool(), NewJobsGetTool(), NewJobsCancelTool(), NewJobsWaitTool(), NewShellTool()} {
		if response := runJobTool(t, context.Background(), tool, `{"command":"true","background":true}`); !response.IsError {
			t.Errorf("%s needs a service", tool.Info().Name)
		}
	}
	s := testJobService(t)
	s.startErr = jobs.ErrCapacity
	response := runJobTool(t, WithJobService(context.Background(), s), NewShellTool(), `{"command":"true","background":true}`)
	if !response.IsError || !strings.Contains(response.Content, jobs.ErrCapacity.Error()) {
		t.Fatalf("response = %+v", response)
	}
}

// TestJobToolsIncludedByDefault checks that all four job tools are registered
// and included in the default core tool names.
func TestJobToolsIncludedByDefault(t *testing.T) {
	for _, name := range []string{"jobs_list", "jobs_get", "jobs_cancel", "jobs_wait"} {
		if !slices.Contains(DefaultCoreToolNames(), name) {
			t.Errorf("missing default %s", name)
		}
		if tools := ListedTools([]string{name}); len(tools) != 1 || tools[0].Info().Name != name {
			t.Errorf("tool not registered: %s", name)
		}
	}
}

// TestForegroundSubagentKeepsDetachedContext checks that foreground subagents
// still ignore turn cancellation and return session metadata without starting a job.
func TestForegroundSubagentKeepsDetachedContext(t *testing.T) {
	s := testJobService(t)
	ctx, cancel := context.WithCancel(WithJobService(context.Background(), s))
	cancel()
	called := false
	ctx = WithSubagentSpawner(ctx, func(ctx context.Context, req SubagentSpawnRequest) (*SubagentSpawnResult, error) {
		called = true
		if ctx.Err() != nil || ctx.Done() != nil {
			t.Error("foreground subagent inherited turn cancellation")
		}
		if req.Prompt != "foreground" {
			t.Errorf("request = %+v", req)
		}
		return &SubagentSpawnResult{Response: "done", SessionID: "resume-id"}, nil
	})
	response := runJobTool(t, ctx, NewSubagentTool(), `{"task":"foreground","background":false}`)
	if !called || response.IsError || !strings.Contains(response.Content, "resume-id") || s.starts != 0 {
		t.Fatalf("response = %+v, starts = %d", response, s.starts)
	}
}

// TestBackgroundSubagentReturnsForegroundResult checks that completed background
// subagent output includes both the answer and the resumable session ID.
func TestBackgroundSubagentReturnsForegroundResult(t *testing.T) {
	s := testJobService(t)
	ctx := WithSubagentSpawner(WithJobService(context.Background(), s), func(context.Context, SubagentSpawnRequest) (*SubagentSpawnResult, error) {
		return &SubagentSpawnResult{Response: "answer", SessionID: "resume-id"}, nil
	})
	job := startedJob(t, runJobTool(t, ctx, NewSubagentTool(), `{"task":"task","background":true}`))
	done := waitJob(t, s, job.ID)
	if done.Status != jobs.Completed || !strings.Contains(done.Output, "answer") || !strings.Contains(done.Output, "subagent_session_id: resume-id") {
		t.Fatalf("job = %+v", done)
	}
}

// TestBackgroundSudoPreflightRejectsPasswordPrompt checks that sudo work which
// needs authorization is rejected before job admission without prompting for a password.
func TestBackgroundSudoPreflightRejectsPasswordPrompt(t *testing.T) {
	dir := t.TempDir()
	// Use a fake sudo so this test does not depend on host credentials.
	if err := os.WriteFile(filepath.Join(dir, "sudo"), []byte("#!/bin/sh\nexit 1\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	t.Setenv("SUDO_PASSWORD", "")
	s := testJobService(t)
	ctx := ContextWithPasswordPrompt(WithJobService(context.Background(), s), func(string) (string, bool) { t.Error("background password prompt"); return "", true })
	response := runJobTool(t, ctx, NewShellTool(), `{"command":"sudo id","background":true}`)
	if !response.IsError || !strings.Contains(response.Content, "sudo -v") || s.starts != 0 {
		t.Fatalf("response = %+v, starts = %d", response, s.starts)
	}
}

// TestBackgroundSudoPreflightAcceptsCachedAuthorization checks that background
// sudo work can complete when a fake sudo reports existing authorization.
func TestBackgroundSudoPreflightAcceptsCachedAuthorization(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "sudo"), []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	t.Setenv("SUDO_PASSWORD", "")
	s := testJobService(t)
	ctx := WithJobService(context.Background(), s)
	// The foreground path uses the same shell and performs its own sudo check.
	tool := NewShellTool(WithShell([]string{"/bin/sh"}))
	job := startedJob(t, runJobTool(t, ctx, tool, `{"command":"sudo id","background":true}`))
	if done := waitJob(t, s, job.ID); done.Status != jobs.Completed {
		t.Fatalf("job = %+v", done)
	}
}

// TestJobToolsRequiredNeverNull guards against "required": null in the tool
// schema, which OpenAI rejects with "null is not of type array".
func TestJobToolsRequiredNeverNull(t *testing.T) {
	for _, tool := range []fantasy.AgentTool{NewJobsListTool(), NewJobsGetTool(), NewJobsCancelTool(), NewJobsWaitTool()} {
		info := tool.Info()
		if info.Required == nil {
			t.Errorf("%s: Required is nil, want empty slice", info.Name)
		}
		data, err := json.Marshal(info.Required)
		if err != nil {
			t.Fatalf("%s: marshal: %v", info.Name, err)
		}
		if string(data) == "null" {
			t.Errorf("%s: required serializes as null", info.Name)
		}
	}
}

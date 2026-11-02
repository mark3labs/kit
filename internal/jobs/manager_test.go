package jobs

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"
)

func testContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	return ctx
}

func testManager(t *testing.T) *Manager {
	t.Helper()
	m := New()
	t.Cleanup(func() {
		if err := m.Close(); err != nil {
			t.Error(err)
		}
	})
	return m
}

func startJob(t *testing.T, m *Manager, ctx context.Context, work Work) Job {
	t.Helper()
	job, err := m.Start(ctx, "test", "session", "branch", work)
	if err != nil {
		t.Fatal(err)
	}
	return job
}

func waitJob(t *testing.T, m *Manager, id string) Job {
	t.Helper()
	job, err := m.Wait(testContext(t), id)
	if err != nil {
		t.Fatal(err)
	}
	return job
}

func TestResults(t *testing.T) {
	for _, tc := range []struct {
		name            string
		work            Work
		status          Status
		output, message string
	}{
		{"success", func(context.Context) (string, error) { return "output", nil }, Completed, "output", ""},
		{"failure", func(context.Context) (string, error) { return "partial", errors.New("failed work") }, Failed, "partial", "failed work"},
		{"panic", func(context.Context) (string, error) { panic("broken") }, Failed, "", "job panicked: broken"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := testManager(t)
			initial := startJob(t, m, context.Background(), tc.work)
			if initial.Status != Running || initial.StartedAt.IsZero() || !initial.FinishedAt.IsZero() {
				t.Fatalf("bad initial snapshot: %+v", initial)
			}
			job := waitJob(t, m, initial.ID)
			if job.Status != tc.status || job.Output != tc.output || job.Error != tc.message || job.FinishedAt.Before(job.StartedAt) {
				t.Fatalf("bad result: %+v", job)
			}
			if job.Kind != "test" || job.SessionID != "session" || job.BranchID != "branch" {
				t.Fatalf("metadata lost: %+v", job)
			}
			if err := m.Cancel(job.ID); err != nil {
				t.Fatal(err)
			}
			got, err := m.Get(job.ID)
			if err != nil || got != job {
				t.Fatalf("terminal snapshot changed: %+v, %v", got, err)
			}
		})
	}
}

func TestOutputLimit(t *testing.T) {
	m := testManager(t)
	text := strings.Repeat("a", MaxOutputBytes-1) + "界" + strings.Repeat("b", 100)
	job := startJob(t, m, context.Background(), func(context.Context) (string, error) {
		return text, errors.New(strings.Repeat("e", MaxOutputBytes+100))
	})
	result := waitJob(t, m, job.ID)
	if len(result.Output) != MaxOutputBytes-1 || !utf8.ValidString(result.Output) || len(result.Error) != MaxOutputBytes {
		t.Fatalf("invalid bounds: output=%d error=%d", len(result.Output), len(result.Error))
	}
}

func TestCapacityAndCancellation(t *testing.T) {
	m := testManager(t)
	release := make(chan struct{})
	defer close(release)
	var running, peak atomic.Int32
	ids := make([]string, 0, MaxConcurrency)
	entered := make(chan struct{}, MaxConcurrency)
	for range MaxConcurrency {
		job := startJob(t, m, context.Background(), func(context.Context) (string, error) {
			n := running.Add(1)
			for p := peak.Load(); n > p && !peak.CompareAndSwap(p, n); p = peak.Load() {
			}
			defer running.Add(-1)
			entered <- struct{}{}
			<-release
			return "late output", nil
		})
		ids = append(ids, job.ID)
	}
	for range MaxConcurrency {
		select {
		case <-entered:
		case <-testContext(t).Done():
			t.Fatal("workers did not start")
		}
	}
	if _, err := m.Start(context.Background(), "", "", "", func(context.Context) (string, error) { return "", nil }); !errors.Is(err, ErrCapacity) {
		t.Fatalf("capacity: %v", err)
	}
	if err := m.Cancel(ids[0]); err != nil {
		t.Fatal(err)
	}
	if job := waitJob(t, m, ids[0]); job.Status != Cancelled {
		t.Fatalf("cancel: %+v", job)
	}
	if _, err := m.Start(context.Background(), "", "", "", func(context.Context) (string, error) { return "", nil }); !errors.Is(err, ErrCapacity) {
		t.Fatalf("cancelled worker released its slot: %v", err)
	}
	if peak.Load() != MaxConcurrency {
		t.Fatalf("peak=%d", peak.Load())
	}
}

func TestContextLifetimeAndWaitCancellation(t *testing.T) {
	m := testManager(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	observed := make(chan struct{})
	job := startJob(t, m, ctx, func(ctx context.Context) (string, error) { <-ctx.Done(); close(observed); return "ignored", nil })
	waitCtx, stopWait := context.WithCancel(context.Background())
	stopWait()
	if _, err := m.Wait(waitCtx, job.ID); !errors.Is(err, context.Canceled) {
		t.Fatalf("wait cancellation: %v", err)
	}
	if got, err := m.Get(job.ID); err != nil || got.Status != Running {
		t.Fatalf("wait cancelled work: %+v %v", got, err)
	}
	cancel()
	if got := waitJob(t, m, job.ID); got.Status != Cancelled {
		t.Fatalf("parent cancellation: %+v", got)
	}
	select {
	case <-observed:
	case <-testContext(t).Done():
		t.Fatal("worker did not observe cancellation")
	}
	if err := m.Close(); err != nil {
		t.Fatal(err)
	}
	got, err := m.Get(job.ID)
	if err != nil || got.Status != Cancelled || got.Output != "" {
		t.Fatalf("late result overwrote cancellation: %+v %v", got, err)
	}
}

func TestCloseWaitsAndRejectsStart(t *testing.T) {
	m := New()
	release := make(chan struct{})
	observed := make(chan struct{})
	job := startJob(t, m, context.Background(), func(ctx context.Context) (string, error) { <-ctx.Done(); close(observed); <-release; return "", nil })
	closed := make(chan error, 1)
	go func() { closed <- m.Close() }()
	select {
	case <-observed:
	case <-testContext(t).Done():
		t.Fatal("close did not cancel worker")
	}
	select {
	case err := <-closed:
		t.Fatalf("close returned before worker exit: %v", err)
	default:
	}
	if _, err := m.Start(context.Background(), "", "", "", func(context.Context) (string, error) { return "", nil }); !errors.Is(err, ErrClosed) {
		t.Fatalf("start after close: %v", err)
	}
	if got := waitJob(t, m, job.ID); got.Status != Cancelled {
		t.Fatalf("close status: %+v", got)
	}
	close(release)
	select {
	case err := <-closed:
		if err != nil {
			t.Fatal(err)
		}
	case <-testContext(t).Done():
		t.Fatal("close did not return")
	}
	if err := m.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestRetention(t *testing.T) {
	m := testManager(t)
	active := startJob(t, m, context.Background(), func(ctx context.Context) (string, error) { <-ctx.Done(); return "", ctx.Err() })
	var first, last Job
	for i := range MaxFinished + 20 {
		job := startJob(t, m, context.Background(), func(context.Context) (string, error) { return "done", nil })
		waitJob(t, m, job.ID)
		if i == 0 {
			first = job
		}
		last = job
	}
	if _, err := m.Get(first.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("old job retained: %v", err)
	}
	if _, err := m.Get(last.ID); err != nil {
		t.Fatal(err)
	}
	if got, err := m.Get(active.ID); err != nil || got.Status != Running {
		t.Fatalf("active job evicted: %+v %v", got, err)
	}
	snapshot := m.Snapshot()
	if len(snapshot) != MaxFinished+1 {
		t.Fatalf("retained %d jobs", len(snapshot))
	}
	snapshot[0].Kind = "changed"
	if got, _ := m.Get(active.ID); got.Kind == "changed" {
		t.Fatal("snapshot aliases manager state")
	}
}

func TestWaitAny(t *testing.T) {
	m := testManager(t)
	release := make(chan struct{})
	defer close(release)
	a := startJob(t, m, context.Background(), func(context.Context) (string, error) { <-release; return "a", nil })
	b := startJob(t, m, context.Background(), func(ctx context.Context) (string, error) { <-ctx.Done(); return "", ctx.Err() })
	if err := m.Cancel(b.ID); err != nil {
		t.Fatal(err)
	}
	result, err := m.WaitAny(testContext(t), []string{a.ID, b.ID, b.ID}, false)
	if err != nil || len(result) != 1 || result[0].ID != b.ID {
		t.Fatalf("any: %+v %v", result, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := m.WaitAny(ctx, []string{a.ID, b.ID}, true); !errors.Is(err, context.Canceled) {
		t.Fatalf("all: %v", err)
	}
	if result, err := m.WaitAny(testContext(t), nil, true); err != nil || len(result) != 0 {
		t.Fatalf("empty: %+v %v", result, err)
	}
	// Repeated immediate completions exercise registration versus completion.
	for range 50 {
		j := startJob(t, m, context.Background(), func(context.Context) (string, error) { return "", nil })
		result, err := m.WaitAny(testContext(t), []string{j.ID, b.ID, j.ID}, true)
		if err != nil || len(result) != 2 || result[0].ID != j.ID || result[1].ID != b.ID {
			t.Fatalf("all results: %+v %v", result, err)
		}
	}
}

func TestSubscribe(t *testing.T) {
	m := testManager(t)
	entered := make(chan struct{})
	release := make(chan struct{})
	defer close(release)
	var once sync.Once
	unsubscribe := m.Subscribe(func(Job) {
		_ = m.List() // Callback must not hold the manager lock.
		once.Do(func() { close(entered) })
		<-release
	})
	defer unsubscribe()
	job := startJob(t, m, context.Background(), func(context.Context) (string, error) { return "done", nil })
	select {
	case <-entered:
	case <-testContext(t).Done():
		t.Fatal("no callback")
	}
	waitJob(t, m, job.ID)
	// A blocked callback must not block callers, work, or Close.
	for range 20 {
		j := startJob(t, m, context.Background(), func(context.Context) (string, error) { return "", nil })
		waitJob(t, m, j.ID)
	}
	unsubscribe()
	unsubscribe()
	if err := m.Close(); err != nil {
		t.Fatal(err)
	}
	m.Subscribe(func(Job) { t.Error("subscription after close") })()
}

func TestSubscriberPanic(t *testing.T) {
	m := testManager(t)
	called := make(chan struct{}, 2)
	unsubscribe := m.Subscribe(func(Job) { called <- struct{}{}; panic("subscriber") })
	defer unsubscribe()
	job := startJob(t, m, context.Background(), func(ctx context.Context) (string, error) { <-ctx.Done(); return "", nil })
	select {
	case <-called:
	case <-testContext(t).Done():
		t.Fatal("no initial notification")
	}
	if err := m.Cancel(job.ID); err != nil {
		t.Fatal(err)
	}
	select {
	case <-called:
	case <-testContext(t).Done():
		t.Fatal("panic stopped notifications")
	}
}

func TestErrors(t *testing.T) {
	m := testManager(t)
	if _, err := m.Start(context.Background(), "", "", "", nil); !errors.Is(err, ErrInvalidWork) {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := m.Start(ctx, "", "", "", func(context.Context) (string, error) { t.Error("cancelled work ran"); return "", nil }); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := m.Get("missing"); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if err := m.Cancel("missing"); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if _, err := m.Wait(testContext(t), "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
}

func TestConcurrentOperations(t *testing.T) {
	m := testManager(t)
	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() {
			for range 30 {
				unsubscribe := m.Subscribe(func(Job) { _ = m.Snapshot() })
				job, err := m.Start(context.Background(), "test", "", "", func(context.Context) (string, error) { return "ok", nil })
				if err == nil {
					_ = m.Cancel(job.ID)
					_, err = m.Wait(testContext(t), job.ID)
					if err != nil && !errors.Is(err, ErrNotFound) {
						t.Error(err)
					}
				} else if !errors.Is(err, ErrCapacity) {
					t.Error(fmt.Errorf("concurrent start: %w", err))
				}
				_ = m.List()
				unsubscribe()
			}
		})
	}
	wg.Wait()
	var closeWG sync.WaitGroup
	for range 10 {
		closeWG.Go(func() {
			if err := m.Close(); err != nil {
				t.Error(err)
			}
		})
	}
	closeWG.Wait()
}

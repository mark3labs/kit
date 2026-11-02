// Package jobs manages bounded, in-memory background work.
package jobs

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

const (
	MaxConcurrency = 8
	MaxOutputBytes = 50 * 1024
	MaxFinished    = 100
)

// Status is the current job state.
type Status string

const (
	Running   Status = "running"
	Completed Status = "completed"
	Failed    Status = "failed"
	Cancelled Status = "cancelled"
)

var (
	ErrClosed      = errors.New("job manager is closed")
	ErrCapacity    = errors.New("job worker limit reached")
	ErrNotFound    = errors.New("job not found")
	ErrInvalidWork = errors.New("job work is nil")
)

// Job is an independent snapshot. Output is limited to MaxOutputBytes.
type Job struct {
	ID         string    `json:"id"`
	Kind       string    `json:"kind"`
	SessionID  string    `json:"session_id,omitempty"`
	BranchID   string    `json:"branch_id,omitempty"`
	Output     string    `json:"output,omitempty"`
	Error      string    `json:"error,omitempty"`
	Status     Status    `json:"status"`
	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at"`
}

// Work runs until it returns or observes context cancellation. Work must honor
// cancellation so that Close can return. A panic is recorded as a failed job.
type Work func(context.Context) (string, error)

type entry struct {
	job    Job
	cancel context.CancelFunc
	stop   func() bool
}

type subscription struct {
	updates chan Job
	done    chan struct{}
}

// Manager is safe for concurrent use. Construct it with New.
// Finished jobs are retained in completion order, up to MaxFinished.
// Active workers, including cancelled workers that have not returned, retain
// their concurrency slots. No work is queued.
type Manager struct {
	mu          sync.Mutex
	jobs        map[string]*entry
	finished    []string
	subscribers map[*subscription]struct{}
	changed     chan struct{}
	next        uint64
	active      int
	closed      bool
	workers     sync.WaitGroup
}

// New constructs a manager with eight worker slots.
func New() *Manager {
	return &Manager{jobs: make(map[string]*entry), subscribers: make(map[*subscription]struct{}), changed: make(chan struct{})}
}

// Start starts work with a child of ctx. The context controls the job lifetime,
// not just admission. Start returns ErrCapacity rather than waiting for a slot.
func (m *Manager) Start(ctx context.Context, kind, sessionID, branchID string, work Work) (Job, error) {
	if work == nil {
		return Job{}, ErrInvalidWork
	}
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return Job{}, ErrClosed
	}
	if err := ctx.Err(); err != nil {
		m.mu.Unlock()
		return Job{}, fmt.Errorf("start job: %w", err)
	}
	if m.active >= MaxConcurrency {
		m.mu.Unlock()
		return Job{}, ErrCapacity
	}
	m.next++
	jobCtx, cancel := context.WithCancel(ctx)
	e := &entry{job: Job{ID: strconv.FormatUint(m.next, 10), Kind: kind, SessionID: sessionID, BranchID: branchID, Status: Running, StartedAt: time.Now()}, cancel: cancel}
	m.jobs[e.job.ID] = e
	m.active++
	m.workers.Add(1)
	e.stop = context.AfterFunc(jobCtx, func() {
		m.mu.Lock()
		m.finishLocked(e, Cancelled, "", jobCtx.Err().Error())
		m.mu.Unlock()
	})
	m.notifyLocked(e.job)
	snapshot := e.job
	m.mu.Unlock()
	go m.run(jobCtx, e, work)
	return snapshot, nil
}

// run records the worker result and releases its concurrency slot.
// Cancellation takes precedence over output or errors returned by the worker.
func (m *Manager) run(ctx context.Context, e *entry, work Work) {
	defer m.workers.Done()
	output, err := invoke(ctx, work)
	m.mu.Lock()
	status, message := Completed, ""
	if err != nil {
		status, message = Failed, err.Error()
	}
	if ctx.Err() != nil {
		status, message = Cancelled, ctx.Err().Error()
		output = ""
	}
	m.finishLocked(e, status, output, message)
	m.active--
	e.stop()
	e.cancel()
	m.mu.Unlock()
}

// invoke runs work and converts a panic into a job error.
func invoke(ctx context.Context, work Work) (output string, err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("job panicked: %v", p)
		}
	}()
	return work(ctx)
}

// bounded limits text to MaxOutputBytes without splitting a UTF-8 character
// at the cutoff, and copies truncated text to release the original allocation.
func bounded(s string) string {
	if len(s) <= MaxOutputBytes {
		return s
	}
	end := MaxOutputBytes
	// Do not split a UTF-8 character at the size boundary. Preserve arbitrary
	// bytes elsewhere; output need not be valid UTF-8.
	for end > MaxOutputBytes-utf8.UTFMax && !utf8.RuneStart(s[end]) {
		end--
	}
	return strings.Clone(s[:end])
}

// finishLocked records the first terminal state, evicts excess finished jobs,
// and notifies observers. The caller must hold m.mu.
func (m *Manager) finishLocked(e *entry, status Status, output, message string) {
	if e.job.Status != Running {
		return
	}
	e.job.Status, e.job.Output, e.job.Error = status, bounded(output), bounded(message)
	e.job.FinishedAt = time.Now()
	m.finished = append(m.finished, e.job.ID)
	if len(m.finished) > MaxFinished {
		delete(m.jobs, m.finished[0])
		m.finished = m.finished[1:]
	}
	m.notifyLocked(e.job)
}

// Get returns a snapshot, or ErrNotFound for an unknown or evicted job.
func (m *Manager) Get(id string) (Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.jobs[id]
	if !ok {
		return Job{}, fmt.Errorf("get job %q: %w", id, ErrNotFound)
	}
	return e.job, nil
}

// List returns all retained jobs, oldest start first. The slice is independent.
func (m *Manager) List() []Job {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]Job, 0, len(m.jobs))
	for _, e := range m.jobs {
		result = append(result, e.job)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].StartedAt.Equal(result[j].StartedAt) {
			return result[i].ID < result[j].ID
		}
		return result[i].StartedAt.Before(result[j].StartedAt)
	})
	return result
}

// Snapshot is an alias for List. Subscribe before taking a snapshot to avoid
// missing changes between the initial read and notification registration.
func (m *Manager) Snapshot() []Job { return m.List() }

// Cancel marks a running job cancelled and cancels its context. It is idempotent
// for retained terminal jobs. A worker cannot overwrite a terminal status.
func (m *Manager) Cancel(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.jobs[id]
	if !ok {
		return fmt.Errorf("cancel job %q: %w", id, ErrNotFound)
	}
	if e.job.Status == Running {
		e.cancel()
		m.finishLocked(e, Cancelled, "", context.Canceled.Error())
	}
	return nil
}

// Wait waits for a terminal status, not necessarily worker exit. Cancelling ctx
// only cancels this wait. An admitted wait survives retention eviction.
// Job failures are returned in Job.Error, not as a wait error.
func (m *Manager) Wait(ctx context.Context, id string) (Job, error) {
	result, err := m.WaitAny(ctx, []string{id}, true)
	if err != nil {
		return Job{}, err
	}
	return result[0], nil
}

// WaitAny waits until at least one requested job is terminal, or until all are
// terminal when all is true. It returns terminal snapshots in input order,
// without duplicates. Empty input returns an empty slice. Registration and
// state checks are atomic; admitted waits survive retention eviction.
func (m *Manager) WaitAny(ctx context.Context, ids []string, all bool) ([]Job, error) {
	m.mu.Lock()
	entries := make([]*entry, 0, len(ids))
	seen := make(map[string]bool, len(ids))
	for _, id := range ids {
		e, ok := m.jobs[id]
		if !ok {
			m.mu.Unlock()
			return nil, fmt.Errorf("wait for job %q: %w", id, ErrNotFound)
		}
		if !seen[id] {
			entries = append(entries, e)
			seen[id] = true
		}
	}
	for {
		result := make([]Job, 0, len(entries))
		for _, e := range entries {
			if e.job.Status != Running {
				result = append(result, e.job)
			}
		}
		if len(result) == len(entries) || (!all && len(result) > 0) {
			m.mu.Unlock()
			return result, nil
		}
		changed := m.changed
		m.mu.Unlock()
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("wait for jobs: %w", ctx.Err())
		case <-changed:
		}
		m.mu.Lock()
	}
}

// Subscribe registers asynchronous change notifications. Callbacks run outside
// the manager lock and must not block. Each subscriber has a one-item mailbox:
// slow subscribers receive the latest change, not every transition. Read List
// or Snapshot after a notification. Callback panics are isolated. Unsubscribe
// is idempotent; a callback already in progress may finish after it returns.
// Close removes all subscriptions. Subscribe after Close is a no-op.
func (m *Manager) Subscribe(callback func(Job)) func() {
	if callback == nil {
		return func() {}
	}
	s := &subscription{updates: make(chan Job, 1), done: make(chan struct{})}
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return func() {}
	}
	m.subscribers[s] = struct{}{}
	m.mu.Unlock()
	go func() {
		for {
			select {
			case <-s.done:
				return
			case job := <-s.updates:
				select {
				case <-s.done:
					return
				default:
				}
				func() { defer func() { _ = recover() }(); callback(job) }()
			}
		}
	}()
	return func() {
		m.mu.Lock()
		if _, ok := m.subscribers[s]; ok {
			delete(m.subscribers, s)
			close(s.done)
		}
		m.mu.Unlock()
	}
}

// notifyLocked wakes waiters and replaces each subscriber's pending update
// with the latest job snapshot. The caller must hold m.mu.
func (m *Manager) notifyLocked(job Job) {
	close(m.changed)
	m.changed = make(chan struct{})
	for s := range m.subscribers {
		select {
		case s.updates <- job:
		default:
			select {
			case <-s.updates:
			default:
			}
			s.updates <- job
		}
	}
}

// Close rejects new jobs, cancels running jobs, and waits for all workers.
// Close is idempotent and safe for concurrent calls. Work must not call Close
// on its own manager. Close does not wait for subscriber callbacks.
func (m *Manager) Close() error {
	m.mu.Lock()
	m.closed = true
	for _, e := range m.jobs {
		if e.job.Status == Running {
			e.cancel()
			m.finishLocked(e, Cancelled, "", context.Canceled.Error())
		}
	}
	for s := range m.subscribers {
		delete(m.subscribers, s)
		close(s.done)
	}
	m.mu.Unlock()
	m.workers.Wait()
	return nil
}

package app

import (
	"context"
	"fmt"
	"sync/atomic"

	kit "github.com/mark3labs/kit/pkg/kit"
)

// Jobs returns retained jobs owned by the current session and branch.
func (a *App) Jobs() []kit.Job {
	if a.opts.Kit == nil {
		return nil
	}
	return a.opts.Kit.OwnedJobs()
}

// GetJob returns the output and status of an owned retained job.
func (a *App) GetJob(id string) (kit.Job, error) {
	if a.opts.Kit == nil {
		return kit.Job{}, fmt.Errorf("background jobs are not available")
	}
	return a.opts.Kit.GetOwnedJob(id)
}

// CancelJob cancels owned work without cancelling the current model turn.
func (a *App) CancelJob(id string) error {
	if a.opts.Kit == nil {
		return fmt.Errorf("background jobs are not available")
	}
	if err := a.opts.Kit.CancelOwnedJob(id); err != nil {
		return fmt.Errorf("cancel background job: %w", err)
	}
	return nil
}

// jobWake coalesces status updates and remembers a terminal update even when a
// running update arrives before the worker reads the signal. Running updates
// alone must never schedule an app step.
type jobWake struct {
	signal chan struct{}
	ready  atomic.Bool
}

func newJobWake() *jobWake {
	return &jobWake{signal: make(chan struct{}, 1)}
}

// shouldDrain also checks snapshots because subscriptions can combine updates
// and replace a terminal notification with a later running notification.
func (w *jobWake) shouldDrain(snapshot []kit.Job) bool {
	ready := w.ready.Swap(false)
	for _, job := range snapshot {
		if job.Status != kit.JobRunning {
			return true
		}
	}
	return ready
}

func (w *jobWake) notify(job kit.Job) {
	if job.Status != kit.JobRunning {
		w.ready.Store(true)
	}
	select {
	case w.signal <- struct{}{}:
	default:
	}
}

// startJobForwarding enables idle job handling when the interactive program is
// attached. Subscribe before the initial wake so pre-existing jobs are included.
// Notifications are coalesced; callbacks never call the model or block on Send.
func (a *App) startJobForwarding() {
	if a.opts.Kit == nil {
		return
	}
	a.jobForwardOnce.Do(func() {
		wake := newJobWake()
		unsub := a.opts.Kit.SubscribeJobs(wake.notify)
		a.mu.Lock()
		if a.closed {
			a.mu.Unlock()
			unsub()
			return
		}
		a.wg.Add(1)
		a.mu.Unlock()
		for _, job := range a.Jobs() {
			wake.notify(job)
		}
		go func() {
			defer a.wg.Done()
			defer unsub()
			for {
				select {
				case <-a.rootCtx.Done():
					return
				case <-wake.signal:
					// Read snapshots as well: a combined notification may hide
					// a completion. Ready-only drains never wait for active work.
					if wake.shouldDrain(a.Jobs()) {
						a.scheduleJobs()
					}
					a.sendEvent(JobsUpdatedEvent{})
				}
			}
		}()
	})
}

// scheduleJobs hands idle work to the app queue. If a turn or compaction is
// active, its queue tail takes the notification before releasing busy.
func (a *App) scheduleJobs() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return
	}
	if a.busy {
		a.jobsPending = true
		return
	}
	a.setBusyLocked(true)
	a.wg.Add(1)
	go a.drainQueue(queueItem{JobsOnly: true})
}

// beginOnce reserves the same busy window used by interactive queue execution.
// Headless calls retain it until RunJobs has drained all owned work.
func (a *App) beginOnce(ctx context.Context) (context.Context, func(), error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return nil, nil, fmt.Errorf("app is closed")
	}
	if a.busy {
		return nil, nil, ErrAgentBusy
	}
	stepCtx, cancel := context.WithCancel(ctx)
	stop := context.AfterFunc(a.rootCtx, cancel)
	a.cancelStep = cancel
	a.setBusyLocked(true)
	a.wg.Add(1)
	return stepCtx, func() {
		stop()
		cancel()
		a.releaseBusyAfterCompact()
		a.wg.Done()
	}, nil
}

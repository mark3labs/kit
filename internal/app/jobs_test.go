package app

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"charm.land/fantasy"

	kit "github.com/mark3labs/kit/pkg/kit"
)

func TestJobDrainReturnsLastResult(t *testing.T) {
	final := &kit.TurnResult{Response: "job output", Messages: []kit.LLMMessage{fantasy.NewUserMessage("completed")}}
	var order []string
	a := New(Options{
		PromptFunc: func(context.Context, string) (*kit.TurnResult, error) {
			order = append(order, "prompt")
			return &kit.TurnResult{Response: "started", HaltedByTool: "jobs_wait"}, nil
		},
		RunJobsFunc: func(context.Context) (*kit.TurnResult, error) {
			order = append(order, "jobs")
			return final, nil
		},
		RunReadyJobsFunc: func(context.Context) (*kit.TurnResult, error) {
			t.Error("headless path used ready-only drain")
			return nil, nil
		},
	}, nil)
	defer a.Close()
	result, err := a.RunOnceResultWithFiles(context.Background(), "run", nil)
	if err != nil || result != final {
		t.Fatalf("result = %v, err = %v", result, err)
	}
	if len(order) != 2 || order[0] != "prompt" || order[1] != "jobs" {
		t.Fatalf("execution order = %v", order)
	}
	if got := a.store.GetAll(); len(got) != 1 {
		t.Fatalf("store did not receive continuation: %v", got)
	}
}

func TestJobDrainPromptPaths(t *testing.T) {
	for _, path := range []string{"quiet", "display", "batch", "files"} {
		t.Run(path, func(t *testing.T) {
			drains := 0
			a := New(Options{
				PromptFunc: func(context.Context, string) (*kit.TurnResult, error) {
					return &kit.TurnResult{Response: "initial"}, nil
				},
				RunJobsFunc: func(context.Context) (*kit.TurnResult, error) {
					drains++
					return &kit.TurnResult{Response: "final"}, nil
				},
			}, nil)
			a.opts.RunReadyJobsFunc = a.opts.RunJobsFunc
			defer a.Close()
			var err error
			switch path {
			case "quiet":
				err = a.RunOnceWithFiles(context.Background(), "run", nil)
			case "display":
				var completed string
				err = a.RunOnceWithDisplay(context.Background(), "run", func(e Event) {
					if e, ok := e.(StepCompleteEvent); ok {
						completed = e.ResponseText
					}
				})
				if completed != "final" {
					t.Fatalf("completion = %q", completed)
				}
			case "batch":
				_, err = a.executeBatch(context.Background(), []queueItem{{Prompt: "one"}, {Prompt: "two"}}, nil)
			case "files":
				_, err = a.RunOnceResultWithFiles(context.Background(), "run", []kit.LLMFilePart{{MediaType: "image/png", Data: []byte("image")}})
			}
			if err != nil || drains != 1 {
				t.Fatalf("drains = %d, err = %v", drains, err)
			}
		})
	}
}

// Interactive drains must return while long jobs (and explicit jobs_wait
// targets) remain active. A new user prompt must not wait for their completion.
func TestInteractivePromptRunsWhileJobPending(t *testing.T) {
	for _, halted := range []string{"", "jobs_wait"} {
		t.Run("halt_"+halted, func(t *testing.T) {
			var completed atomic.Bool
			var prompts atomic.Int32
			var readyCalls atomic.Int32
			var blockingCalls atomic.Int32
			a := New(Options{
				PromptFunc: func(context.Context, string) (*kit.TurnResult, error) {
					prompts.Add(1)
					return &kit.TurnResult{Response: "started", HaltedByTool: halted}, nil
				},
				RunJobsFunc: func(context.Context) (*kit.TurnResult, error) {
					blockingCalls.Add(1)
					return nil, errors.New("interactive path called blocking drain")
				},
				RunReadyJobsFunc: func(context.Context) (*kit.TurnResult, error) {
					readyCalls.Add(1)
					if completed.Load() {
						return &kit.TurnResult{Response: "job completed"}, nil
					}
					return nil, nil
				},
			}, nil)
			defer a.Close()
			a.Run("start a long job")
			if err := a.WaitForIdle(time.Second); err != nil {
				t.Fatal(err)
			}
			// A jobs-only wake must also release busy with the job still pending.
			a.scheduleJobs()
			if err := a.WaitForIdle(time.Second); err != nil {
				t.Fatal(err)
			}
			if depth := a.Run("new user prompt"); depth != 0 {
				t.Fatalf("queued while only job was pending: %d", depth)
			}
			if err := a.WaitForIdle(time.Second); err != nil {
				t.Fatal(err)
			}
			if completed.Load() || prompts.Load() != 2 || readyCalls.Load() != 3 || blockingCalls.Load() != 0 {
				t.Fatalf("prompts=%d ready=%d blocking=%d", prompts.Load(), readyCalls.Load(), blockingCalls.Load())
			}
			// No notification means no repeated drain while the job runs.
			a.mu.Lock()
			if a.jobsPending || len(a.queue) != 0 {
				t.Error("job wake remained queued")
			}
			a.mu.Unlock()
			completed.Store(true)
			a.scheduleJobs()
			if err := a.WaitForIdle(time.Second); err != nil {
				t.Fatal(err)
			}
			if readyCalls.Load() != 4 || prompts.Load() != 2 {
				t.Fatal("completion did not use jobs-only ready drain")
			}
		})
	}
}

func TestJobDrainCancellationAndErrors(t *testing.T) {
	for _, cancel := range []bool{false, true} {
		t.Run(map[bool]string{false: "error", true: "cancel"}[cancel], func(t *testing.T) {
			waiting := make(chan struct{})
			failure := errors.New("job continuation failed")
			a := New(Options{
				PromptFunc: func(context.Context, string) (*kit.TurnResult, error) { return &kit.TurnResult{}, nil },
				RunJobsFunc: func(ctx context.Context) (*kit.TurnResult, error) {
					close(waiting)
					if cancel {
						<-ctx.Done()
						return nil, ctx.Err()
					}
					return nil, failure
				},
			}, nil)
			defer a.Close()
			done := make(chan error, 1)
			go func() {
				_, err := a.RunOnceResultWithFiles(context.Background(), "run", nil)
				done <- err
			}()
			<-waiting
			want := failure
			if cancel {
				a.CancelCurrentStep()
				want = context.Canceled
			}
			select {
			case err := <-done:
				if !errors.Is(err, want) {
					t.Fatalf("err = %v, want %v", err, want)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("job drain did not stop")
			}
			if a.IsBusy() {
				t.Fatal("busy after failed drain")
			}
		})
	}
}

func TestIdleJobNotificationUsesAppQueue(t *testing.T) {
	started := make(chan struct{})
	var once sync.Once
	a := New(Options{
		PromptFunc: func(context.Context, string) (*kit.TurnResult, error) {
			t.Error("idle drain must not submit a user prompt")
			return nil, nil
		},
		RunReadyJobsFunc: func(context.Context) (*kit.TurnResult, error) {
			once.Do(func() { close(started) })
			return &kit.TurnResult{Response: "idle completion"}, nil
		},
	}, nil)
	a.scheduleJobs()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("idle notification was not scheduled")
	}
	if err := a.WaitForIdle(3 * time.Second); err != nil {
		t.Fatal(err)
	}
	a.Close()
	a.scheduleJobs() // Closed apps must not start another drain.
	if a.IsBusy() {
		t.Fatal("closed app accepted job notification")
	}
}

func TestNoJobContinuationKeepsInitialResult(t *testing.T) {
	initial := &kit.TurnResult{Response: "no background work"}
	a := New(Options{
		PromptFunc:  func(context.Context, string) (*kit.TurnResult, error) { return initial, nil },
		RunJobsFunc: func(context.Context) (*kit.TurnResult, error) { return nil, nil },
	}, nil)
	defer a.Close()
	result, err := a.RunOnceResultWithFiles(context.Background(), "run", nil)
	if err != nil || result != initial {
		t.Fatalf("result = %v, err = %v", result, err)
	}
	// A redundant idle notification must not manufacture an empty completion.
	result, err = a.executeBatch(context.Background(), []queueItem{{JobsOnly: true}}, nil)
	if err != nil || result != nil {
		t.Fatalf("idle result = %v, err = %v", result, err)
	}
}

func TestCloseStopsHeadlessJobWait(t *testing.T) {
	waiting := make(chan struct{})
	a := New(Options{
		PromptFunc: func(context.Context, string) (*kit.TurnResult, error) { return &kit.TurnResult{}, nil },
		RunJobsFunc: func(ctx context.Context) (*kit.TurnResult, error) {
			close(waiting)
			<-ctx.Done()
			return nil, ctx.Err()
		},
	}, nil)
	done := make(chan error, 1)
	go func() {
		_, err := a.RunOnceResultWithFiles(context.Background(), "run", nil)
		done <- err
	}()
	<-waiting
	a.Close()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Close left the headless drain running")
	}
}

func TestRunningJobNotificationsDoNotScheduleDrain(t *testing.T) {
	wake := newJobWake()
	for range 100 {
		wake.notify(kit.Job{Status: kit.JobRunning})
	}
	<-wake.signal
	if wake.ready.Swap(false) {
		t.Fatal("running status scheduled a drain")
	}
	select {
	case <-wake.signal:
		t.Fatal("status notifications were not coalesced")
	default:
	}
	wake.notify(kit.Job{Status: kit.JobCompleted})
	wake.notify(kit.Job{Status: kit.JobRunning})
	<-wake.signal
	if !wake.ready.Swap(false) {
		t.Fatal("running update hid the completion notification")
	}
	if wake.ready.Swap(false) {
		t.Fatal("completion was not consumed")
	}
}

func TestInteractiveBatchNeverUsesBlockingDrain(t *testing.T) {
	a := New(Options{
		PromptFunc: func(context.Context, string) (*kit.TurnResult, error) { return &kit.TurnResult{}, nil },
		RunJobsFunc: func(context.Context) (*kit.TurnResult, error) {
			t.Error("batch used blocking job drain")
			return nil, nil
		},
		RunReadyJobsFunc: func(context.Context) (*kit.TurnResult, error) { return nil, nil },
	}, nil)
	defer a.Close()
	for _, items := range [][]queueItem{
		{{Prompt: "one"}, {Prompt: "two"}},
		{{Prompt: "one"}, {JobsOnly: true}},
		{{JobsOnly: true}},
		{{Prompt: "image", Files: []kit.LLMFilePart{{MediaType: "image/png"}}}},
	} {
		if _, err := a.executeBatch(context.Background(), items, nil); err != nil {
			t.Fatal(err)
		}
	}
}

func TestJobWakeRecoversCompletionFromSnapshot(t *testing.T) {
	wake := newJobWake()
	// The mailbox delivered only a later running update.
	wake.notify(kit.Job{ID: "running", Status: kit.JobRunning})
	<-wake.signal
	if wake.shouldDrain([]kit.Job{{ID: "running", Status: kit.JobRunning}}) {
		t.Fatal("running jobs alone must not schedule a drain")
	}
	if !wake.shouldDrain([]kit.Job{
		{ID: "finished", Status: kit.JobCompleted},
		{ID: "running", Status: kit.JobRunning},
	}) {
		t.Fatal("snapshot completion must schedule a drain even without a terminal notification")
	}
}

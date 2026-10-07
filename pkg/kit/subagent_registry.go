package kit

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"slices"
	"sync"
	"time"
)

// ErrSubagentKilled is the error a subagent run returns when it was stopped
// with [Kit.KillSubagent]. Test for it with errors.Is.
var ErrSubagentKilled = errors.New("subagent killed by the user")

// RunningSubagent describes an in-process subagent that is currently running.
// Values are snapshots; they do not change after they are returned.
type RunningSubagent struct {
	// ID identifies the run. Pass it to [Kit.KillSubagent]. For subagents
	// started by the LLM through the subagent tool, this is the tool call ID
	// (the same ID that [Kit.SubscribeSubagent] uses).
	ID string
	// Prompt is the task given to the subagent.
	Prompt string
	// Agent is the named agent definition, or empty for none.
	Agent string
	// Model is the resolved "provider/model" string the subagent uses.
	Model string
	// StartedAt is the time the run started.
	StartedAt time.Time
}

// SubagentRun is a snapshot of a subagent run, including its retained events.
type SubagentRun struct {
	ID              string
	Prompt          string
	Agent           string
	Model           string
	StartedAt       time.Time
	FinishedAt      time.Time
	SessionID       string
	ParentSessionID string
	Status          string
	Error           string
	Events          []Event
	DroppedEvents   int
}

const subagentHistoryLimit = 100
const subagentEventLimit = 500

type subagentRun struct {
	info     RunningSubagent
	snapshot SubagentRun
	kill     context.CancelCauseFunc
}

// subagentRegistry tracks the running subagents of one Kit instance.
type subagentRegistry struct {
	mu    sync.Mutex
	runs  map[string]*subagentRun
	order []string
}

// add registers a run and returns the function that removes it. An empty
// or duplicate ID is replaced with a new random ID.
func (r *subagentRegistry) add(info RunningSubagent, kill context.CancelCauseFunc) (string, func()) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.runs == nil {
		r.runs = make(map[string]*subagentRun)
	}
	if _, taken := r.runs[info.ID]; info.ID == "" || taken {
		info.ID = newRunID()
	}
	run := &subagentRun{info: info, kill: kill, snapshot: SubagentRun{ID: info.ID, Prompt: info.Prompt, Agent: info.Agent, Model: info.Model, StartedAt: info.StartedAt, Status: "starting"}}
	r.runs[info.ID] = run
	r.order = append(r.order, info.ID)
	r.pruneLocked()
	return info.ID, func() {
		r.update(info.ID, func(s *SubagentRun) {
			if s.FinishedAt.IsZero() {
				s.Status = "stopped"
				s.FinishedAt = time.Now()
			}
		})
		r.mu.Lock()
		if r.runs[info.ID] == run {
			run.kill = nil
			r.pruneLocked()
		}
		r.mu.Unlock()
	}
}

// list returns the running subagents, oldest first.
func (r *subagentRegistry) list() []RunningSubagent {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]RunningSubagent, 0, len(r.runs))
	for _, run := range r.runs {
		if run.snapshot.Status == "starting" || run.snapshot.Status == "running" {
			out = append(out, run.info)
		}
	}
	slices.SortFunc(out, func(a, b RunningSubagent) int {
		return a.StartedAt.Compare(b.StartedAt)
	})
	return out
}

func (r *subagentRegistry) pruneLocked() {
	completed := 0
	for _, id := range r.order {
		if run := r.runs[id]; run != nil && run.snapshot.Status != "starting" && run.snapshot.Status != "running" {
			completed++
		}
	}
	for i := 0; completed > subagentHistoryLimit && i < len(r.order); {
		id := r.order[i]
		run := r.runs[id]
		if run != nil && run.snapshot.Status != "starting" && run.snapshot.Status != "running" {
			delete(r.runs, id)
			r.order = append(r.order[:i], r.order[i+1:]...)
			completed--
			continue
		}
		i++
	}
}

func (r *subagentRegistry) update(id string, fn func(*SubagentRun)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if run := r.runs[id]; run != nil {
		fn(&run.snapshot)
		run.info.ID = run.snapshot.ID
		r.pruneLocked()
	}
}
func (r *subagentRegistry) addEvent(id string, e Event) {
	r.update(id, func(s *SubagentRun) {
		if chunk, ok := e.(MessageUpdateEvent); ok && len(s.Events) > 0 {
			if previous, ok := s.Events[len(s.Events)-1].(MessageUpdateEvent); ok && len(previous.Chunk)+len(chunk.Chunk) <= 64*1024 {
				s.Events[len(s.Events)-1] = MessageUpdateEvent{Chunk: previous.Chunk + chunk.Chunk}
				return
			}
		}
		s.Events = append(s.Events, e)
		if len(s.Events) > subagentEventLimit {
			dropped := len(s.Events) - subagentEventLimit
			s.DroppedEvents += dropped
			s.Events = append([]Event(nil), s.Events[dropped:]...)
		}
	})
}
func (r *subagentRegistry) snapshots() []SubagentRun {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]SubagentRun, 0, len(r.order))
	for _, id := range r.order {
		if run := r.runs[id]; run != nil {
			s := run.snapshot
			s.Events = append([]Event(nil), s.Events...)
			out = append(out, s)
		}
	}
	return out
}
func (r *subagentRegistry) get(id string) (SubagentRun, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	run := r.runs[id]
	if run == nil {
		return SubagentRun{}, false
	}
	s := run.snapshot
	s.Events = append([]Event(nil), s.Events...)
	return s, true
}

// kill cancels the run with the given ID. It returns false when no run
// with that ID is running.
func (r *subagentRegistry) kill(id string) bool {
	r.mu.Lock()
	run, ok := r.runs[id]
	if !ok || run.snapshot.Status != "starting" && run.snapshot.Status != "running" || run.kill == nil {
		r.mu.Unlock()
		return false
	}
	kill := run.kill
	r.mu.Unlock()
	if kill == nil {
		return false
	}
	kill(ErrSubagentKilled)
	return true
}

// newRunID returns a random ID for a subagent run.
func newRunID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "subagent-" + time.Now().Format("150405.000000000")
	}
	return "subagent-" + hex.EncodeToString(b[:])
}

// SubagentRuns returns retained run snapshots, oldest first.
func (m *Kit) SubagentRuns() []SubagentRun { return m.subagents.snapshots() }

// GetSubagentRun returns a retained snapshot for a run ID.
func (m *Kit) GetSubagentRun(id string) (SubagentRun, bool) { return m.subagents.get(id) }

// RunningSubagents returns the in-process subagents of this Kit instance
// that are running now, oldest first. This includes subagents started by
// the LLM through the subagent tool, by extensions, and by direct calls to
// [Kit.Subagent].
func (m *Kit) RunningSubagents() []RunningSubagent {
	return m.subagents.list()
}

// KillSubagent stops the running subagent with the given ID (see
// [RunningSubagent].ID). The run stops as soon as possible and its
// [Kit.Subagent] call returns [ErrSubagentKilled]. When the LLM started the
// subagent, the subagent tool tells the LLM that the user killed it.
// KillSubagent returns false when no subagent with that ID is running.
func (m *Kit) KillSubagent(id string) bool {
	return m.subagents.kill(id)
}

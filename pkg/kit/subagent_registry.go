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

// SubagentRun is a snapshot of an active or finished in-process subagent run.
// Snapshots are retained in memory by the host Kit instance, including for
// NoSession runs. They are not restored from saved child sessions.
// See [Kit.SubagentRuns] and [Kit.GetSubagentRun].
type SubagentRun struct {
	// ID identifies this execution, not the child session. Pass it to KillSubagent.
	ID string
	// Prompt is the task submitted for this run.
	Prompt string
	// Agent is the named agent definition, or empty when none was selected.
	Agent string
	// Model is the resolved provider/model identifier.
	Model string
	// StartedAt is the time execution started, including child setup.
	StartedAt time.Time
	// FinishedAt is zero until execution ends.
	FinishedAt time.Time
	// SessionID identifies the child session and is empty until setup succeeds.
	SessionID string
	// ParentSessionID identifies the parent session associated with this run.
	ParentSessionID string
	// Status is starting, running, completed, failed, stopped, or timed_out.
	// Starting includes setup; stopped includes explicit user cancellation.
	Status string
	// Error describes a failed, stopped, or timed-out execution, if available.
	Error string
	// Events contains retained child events in emission order. Adjacent text
	// chunks can be combined. Up to 500 event records are retained per run.
	// The slice is copied when a snapshot is returned; event payloads are not
	// deep-copied.
	Events []Event
	// DroppedEvents counts event records removed by the per-run event limit.
	DroppedEvents int
	// Seq counts the changes of the run: its registration, each child event
	// (also the events that were combined or dropped from Events) and each
	// status change. It matches [SubagentRunUpdate].Seq, so a consumer that
	// starts from a snapshot can skip the updates that it already contains.
	Seq uint64
}

// SubagentRunUpdate describes one change to a retained subagent run. See
// [Kit.SubscribeSubagentRuns].
type SubagentRunUpdate struct {
	// RunID identifies the run (see [SubagentRun].ID).
	RunID string
	// Seq is the run's [SubagentRun].Seq after the change. Each update of
	// a run has a Seq one larger than the previous update of that run.
	Seq uint64
	// Event is the child event, or nil when only the status changed.
	Event Event
	// Status and Error are the run's status and error after the change.
	Status string
	Error  string
}

type subagentRunListener struct {
	id uint64
	fn func(SubagentRunUpdate)
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

	// notifyMu serializes changes together with their notifications, so
	// that listeners see the updates of a run in the order they happened.
	// It is taken before mu and is never held by the read methods.
	notifyMu     sync.Mutex
	listeners    []subagentRunListener
	nextListener uint64
}

// add registers a run and returns the function that removes it. An empty
// or duplicate ID is replaced with a new random ID.
func (r *subagentRegistry) add(info RunningSubagent, kill context.CancelCauseFunc) (string, func()) {
	r.notifyMu.Lock()
	defer r.notifyMu.Unlock()
	r.mu.Lock()
	if r.runs == nil {
		r.runs = make(map[string]*subagentRun)
	}
	if _, taken := r.runs[info.ID]; info.ID == "" || taken {
		info.ID = newRunID()
	}
	run := &subagentRun{info: info, kill: kill, snapshot: SubagentRun{ID: info.ID, Prompt: info.Prompt, Agent: info.Agent, Model: info.Model, StartedAt: info.StartedAt, Status: "starting", Seq: 1}}
	r.runs[info.ID] = run
	r.order = append(r.order, info.ID)
	r.pruneLocked()
	listeners := slices.Clone(r.listeners)
	r.mu.Unlock()
	notifySubagentRunListeners(listeners, SubagentRunUpdate{RunID: info.ID, Seq: 1, Status: "starting"})
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
	r.change(id, nil, fn)
}

func (r *subagentRegistry) addEvent(id string, e Event) {
	r.change(id, e, func(s *SubagentRun) {
		if len(s.Events) > 0 {
			if combined, ok := combineSubagentEvents(s.Events[len(s.Events)-1], e); ok {
				s.Events[len(s.Events)-1] = combined
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

// change applies fn to the run and notifies the listeners. A change without
// an event notifies only when the status or the error changed.
func (r *subagentRegistry) change(id string, e Event, fn func(*SubagentRun)) {
	r.notifyMu.Lock()
	defer r.notifyMu.Unlock()
	r.mu.Lock()
	run := r.runs[id]
	if run == nil {
		r.mu.Unlock()
		return
	}
	before := run.snapshot
	fn(&run.snapshot)
	notify := e != nil || before.Status != run.snapshot.Status || before.Error != run.snapshot.Error
	if notify {
		run.snapshot.Seq++
	}
	after := run.snapshot
	wasActive := before.Status == "starting" || before.Status == "running"
	isActive := after.Status == "starting" || after.Status == "running"
	if wasActive && !isActive {
		r.pruneLocked()
	}
	listeners := slices.Clone(r.listeners)
	r.mu.Unlock()
	if !notify {
		return
	}
	notifySubagentRunListeners(listeners, SubagentRunUpdate{RunID: id, Seq: after.Seq, Event: e, Status: after.Status, Error: after.Error})
}

// combineSubagentEvents merges adjacent streaming chunks so that a long
// response, reasoning block or command output uses one event record and
// does not push older events out of the per-run limit.
func combineSubagentEvents(previous, next Event) (Event, bool) {
	const maxCombined = 64 * 1024
	switch n := next.(type) {
	case MessageUpdateEvent:
		if p, ok := previous.(MessageUpdateEvent); ok && len(p.Chunk)+len(n.Chunk) <= maxCombined {
			return MessageUpdateEvent{Chunk: p.Chunk + n.Chunk}, true
		}
	case ReasoningDeltaEvent:
		if p, ok := previous.(ReasoningDeltaEvent); ok && len(p.Delta)+len(n.Delta) <= maxCombined {
			return ReasoningDeltaEvent{Delta: p.Delta + n.Delta}, true
		}
	case ToolOutputEvent:
		if p, ok := previous.(ToolOutputEvent); ok && p.ToolCallID == n.ToolCallID && p.IsStderr == n.IsStderr && len(p.Chunk)+len(n.Chunk) <= maxCombined {
			p.Chunk += n.Chunk
			return p, true
		}
	}
	return nil, false
}

func (r *subagentRegistry) subscribe(fn func(SubagentRunUpdate)) func() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.nextListener++
	id := r.nextListener
	r.listeners = append(r.listeners, subagentRunListener{id: id, fn: fn})
	return func() {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.listeners = slices.DeleteFunc(r.listeners, func(l subagentRunListener) bool { return l.id == id })
	}
}

func notifySubagentRunListeners(listeners []subagentRunListener, u SubagentRunUpdate) {
	for _, l := range listeners {
		l.fn(u)
	}
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

// SubagentRuns returns active and retained finished run snapshots, oldest first
// by registration order. Runs started by tools, extensions, and direct calls to
// [Kit.Subagent] are included. The host retains up to 100 finished runs plus all
// active runs. Each snapshot has a copied Events slice.
// Use [Kit.RunningSubagents] when only active runs are needed.
func (m *Kit) SubagentRuns() []SubagentRun { return m.subagents.snapshots() }

// GetSubagentRun returns a retained snapshot for an execution ID, not a child
// session ID. It returns false if the ID is unknown or has been evicted from
// history. The returned snapshot has a copied Events slice.
// See [Kit.SubagentRuns] for retention limits.
func (m *Kit) GetSubagentRun(id string) (SubagentRun, bool) { return m.subagents.get(id) }

// SubscribeSubagentRuns registers a listener for the changes to every
// subagent run of this Kit instance: registration, each child event, and
// each status change. Updates of one run arrive in order. The listener is
// called synchronously on the goroutine that made the change, so it must
// return quickly and must not block. Returns a function that removes the
// listener.
func (m *Kit) SubscribeSubagentRuns(listener func(SubagentRunUpdate)) func() {
	return m.subagents.subscribe(listener)
}

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

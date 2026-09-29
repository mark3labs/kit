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

// subagentRun is the registry entry for one running subagent.
type subagentRun struct {
	info RunningSubagent
	kill context.CancelCauseFunc
}

// subagentRegistry tracks the running subagents of one Kit instance.
type subagentRegistry struct {
	mu   sync.Mutex
	runs map[string]*subagentRun
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
	run := &subagentRun{info: info, kill: kill}
	r.runs[info.ID] = run
	return info.ID, func() {
		r.mu.Lock()
		defer r.mu.Unlock()
		if r.runs[info.ID] == run {
			delete(r.runs, info.ID)
		}
	}
}

// list returns the running subagents, oldest first.
func (r *subagentRegistry) list() []RunningSubagent {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]RunningSubagent, 0, len(r.runs))
	for _, run := range r.runs {
		out = append(out, run.info)
	}
	slices.SortFunc(out, func(a, b RunningSubagent) int {
		return a.StartedAt.Compare(b.StartedAt)
	})
	return out
}

// kill cancels the run with the given ID. It returns false when no run
// with that ID is running.
func (r *subagentRegistry) kill(id string) bool {
	r.mu.Lock()
	run, ok := r.runs[id]
	r.mu.Unlock()
	if !ok {
		return false
	}
	run.kill(ErrSubagentKilled)
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

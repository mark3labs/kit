package app

import (
	"sync"

	kit "github.com/mark3labs/kit/pkg/kit"
)

// subagentForwarder sends subagent run updates to the TUI program.
//
// The registry calls its listeners synchronously and they must not block,
// but tea.Program.Send blocks until the event loop reads the message. The
// event loop in turn reads the registry (for example to list the runs), so
// a direct Send from the listener could deadlock. The forwarder therefore
// queues each update and sends it from its own goroutine, in order.
type subagentForwarder struct {
	mu     sync.Mutex
	queue  []SubagentRunEvent
	signal chan struct{}
}

func newSubagentForwarder() *subagentForwarder {
	return &subagentForwarder{signal: make(chan struct{}, 1)}
}

// push converts and queues one update. It never blocks.
func (f *subagentForwarder) push(u kit.SubagentRunUpdate) {
	ev := SubagentRunEvent{RunID: u.RunID, Seq: u.Seq, Status: u.Status, Error: u.Error}
	if u.Event != nil {
		ev.Event, _ = ConvertSDKEvent(u.Event)
	}
	f.mu.Lock()
	f.queue = append(f.queue, ev)
	f.mu.Unlock()
	select {
	case f.signal <- struct{}{}:
	default:
	}
}

// run sends the queued updates with send until done is closed.
func (f *subagentForwarder) run(done <-chan struct{}, send func(Event)) {
	for {
		select {
		case <-done:
			return
		case <-f.signal:
		}
		f.mu.Lock()
		batch := f.queue
		f.queue = nil
		f.mu.Unlock()
		for _, ev := range batch {
			select {
			case <-done:
				return
			default:
			}
			send(ev)
		}
	}
}

// startSubagentForwarding subscribes to the subagent runs of the Kit
// instance and sends each change to the program. It runs once, when the
// first program is registered. Close stops it.
func (a *App) startSubagentForwarding() {
	if a.opts.Kit == nil {
		return
	}
	a.subagentForwardOnce.Do(func() {
		f := newSubagentForwarder()
		unsub := a.opts.Kit.SubscribeSubagentRuns(f.push)
		a.wg.Go(func() {
			defer unsub()
			f.run(a.rootCtx.Done(), a.sendEvent)
		})
	})
}

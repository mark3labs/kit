package app

import (
	"testing"
	"time"

	kit "github.com/mark3labs/kit/pkg/kit"
)

func TestSubagentForwarderKeepsOrderAndConverts(t *testing.T) {
	f := newSubagentForwarder()
	done := make(chan struct{})
	got := make(chan Event, 8)
	go f.run(done, func(e Event) { got <- e })
	defer close(done)

	f.push(kit.SubagentRunUpdate{RunID: "r", Seq: 1, Status: "starting"})
	f.push(kit.SubagentRunUpdate{RunID: "r", Seq: 2, Status: "running", Event: kit.MessageUpdateEvent{Chunk: "hi"}})
	f.push(kit.SubagentRunUpdate{RunID: "r", Seq: 3, Status: "running", Event: kit.TurnStartEvent{}})

	for want := uint64(1); want <= 3; want++ {
		select {
		case e := <-got:
			ev := e.(SubagentRunEvent)
			if ev.Seq != want || ev.RunID != "r" {
				t.Fatalf("event %d = %+v", want, ev)
			}
			switch want {
			case 2:
				if chunk, ok := ev.Event.(StreamChunkEvent); !ok || chunk.Content != "hi" {
					t.Fatalf("converted event = %#v", ev.Event)
				}
			default:
				if ev.Event != nil {
					t.Fatalf("event %d has %#v, want nil", want, ev.Event)
				}
			}
		case <-time.After(time.Second):
			t.Fatalf("event %d not forwarded", want)
		}
	}
}

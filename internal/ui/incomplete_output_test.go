package ui

import (
	"errors"
	"strings"
	"testing"

	"github.com/mark3labs/kit/internal/app"
)

func TestFailedTurnRetainsVisiblyIncompleteText(t *testing.T) {
	for _, event := range []app.Event{app.StepErrorEvent{Err: errors.New("stream failed")}, app.StepCancelledEvent{}} {
		m, _, _ := newTestAppModel(&stubAppController{})
		m.state = stateWorking
		m.bufferStreamChunk("assistant", "unfinished reply")
		updated, _ := m.Update(event)
		m = updated.(*AppModel)
		var output *StreamingMessageItem
		for _, item := range m.messages {
			if stream, ok := item.(*StreamingMessageItem); ok {
				output = stream
				break
			}
		}
		if output == nil || output.RawContent() != "unfinished reply" || !output.incomplete || output.streaming {
			t.Fatalf("output=%#v", output)
		}
		if !strings.Contains(output.Render(80), "Incomplete output") {
			t.Fatal("incomplete label missing")
		}
		m.flushStreamAndPendingUserMessages()
		count := 0
		for _, item := range m.messages {
			if stream, ok := item.(*StreamingMessageItem); ok && stream.RawContent() == "unfinished reply" {
				count++
			}
		}
		if count != 1 {
			t.Fatalf("unfinished output copies=%d", count)
		}
	}
}

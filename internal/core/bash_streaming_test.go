package core

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"

	"charm.land/fantasy"
)

// runStreaming drives executeBashStreaming directly for a shell command and
// returns the tool response plus every chunk pushed to the output callback.
//
// It fails the test rather than blocking forever if the call does not return,
// because the defect it guards against is a deadlock.
func runStreaming(t *testing.T, command string) (fantasy.ToolResponse, []string) {
	t.Helper()

	type result struct {
		resp   fantasy.ToolResponse
		chunks []string
		err    error
	}
	done := make(chan result, 1)

	go func() {
		ctx := context.Background()
		cmd := exec.CommandContext(ctx, "bash", "-c", command)
		var chunks []string
		cb := func(_, _, chunk string, _ bool) {
			chunks = append(chunks, chunk)
		}
		resp, err := executeBashStreaming(ctx, bashCall(command, 0), cmd, cb, "")
		done <- result{resp, chunks, err}
	}()

	select {
	case r := <-done:
		if r.err != nil {
			t.Fatalf("executeBashStreaming: %v", r.err)
		}
		return r.resp, r.chunks
	case <-time.After(30 * time.Second):
		t.Fatal("executeBashStreaming did not return within 30s (deadlocked on an undrained pipe?)")
		return fantasy.ToolResponse{}, nil
	}
}

// TestBashStreaming_ReportsOversizedLine is a regression test for a deadlock.
// The streaming scanner caps a single line at 1 MB; a longer one ends the scan
// early. The loop used to ignore scanner.Err() and leave the rest of the pipe
// unread, so the child process blocked writing to a full pipe and cmd.Wait()
// never returned. The call hung until the command timeout fired, and every
// byte of output was lost.
//
// A single line over the limit is not exotic — minified JSON, a packed bundle
// or `cat` of a binary all produce one.
func TestBashStreaming_ReportsOversizedLine(t *testing.T) {
	// One line of 2 MB, comfortably over the 1 MB scanner limit.
	resp, chunks := runStreaming(t, `head -c 2000000 /dev/zero | tr '\0' 'a'`)

	if !strings.Contains(resp.Content, "output truncated") {
		t.Errorf("oversized line should be reported in the result, got %d bytes: %.120q",
			len(resp.Content), resp.Content)
	}

	var sawNotice bool
	for _, c := range chunks {
		if strings.Contains(c, "output truncated") {
			sawNotice = true
			break
		}
	}
	if !sawNotice {
		t.Error("oversized line should be reported to the streaming callback too")
	}
}

// TestBashStreaming_NormalOutputUnaffected is the control: ordinary output
// must stream through unchanged, with no truncation notice.
func TestBashStreaming_NormalOutputUnaffected(t *testing.T) {
	resp, chunks := runStreaming(t, "printf 'alpha\\nbeta\\ngamma\\n'")

	if strings.Contains(resp.Content, "output truncated") {
		t.Errorf("normal output must not be flagged as truncated: %q", resp.Content)
	}
	for _, want := range []string{"alpha", "beta", "gamma"} {
		if !strings.Contains(resp.Content, want) {
			t.Errorf("result missing %q, got %q", want, resp.Content)
		}
	}
	if len(chunks) != 3 {
		t.Errorf("want 3 streamed chunks, got %d: %v", len(chunks), chunks)
	}
}

// TestBashStreaming_LongButUnderLimit guards the boundary: a line near but
// below the 1 MB scanner cap must stream without a truncation notice, so the
// fix does not over-report.
//
// The response content is still shorter than the 500 KB produced, because
// buildBashResponse applies its own deliberate display truncation
// (defaultMaxLineLen caps a line at 2000 characters). That is a separate,
// intended mechanism; what matters here is that the scanner did not error.
func TestBashStreaming_LongButUnderLimit(t *testing.T) {
	resp, chunks := runStreaming(t, `head -c 500000 /dev/zero | tr '\0' 'b'`)

	if strings.Contains(resp.Content, "output truncated") {
		t.Error("a 500 KB line is under the 1 MB limit and must not be flagged")
	}

	// The scanner should have delivered the line whole, before any
	// display-level truncation is applied downstream.
	if len(chunks) != 1 {
		t.Fatalf("want 1 streamed chunk, got %d", len(chunks))
	}
	if got := len(chunks[0]); got != 500000 {
		t.Errorf("scanner should deliver the full 500000-byte line, got %d", got)
	}
}

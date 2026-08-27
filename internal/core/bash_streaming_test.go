package core

import (
	"context"
	"os/exec"
	"strings"
	"sync"
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

		// executeBashStreaming drains stdout and stderr in two separate
		// goroutines, so the callback is invoked concurrently. The production
		// code guards its own chunk slices with a mutex; the callback we pass
		// must do the same or the append races.
		var mu sync.Mutex
		var chunks []string
		cb := func(_, _, chunk string, _ bool) {
			mu.Lock()
			chunks = append(chunks, chunk)
			mu.Unlock()
		}

		resp, err := executeBashStreaming(ctx, bashCall(command, 0), cmd, cb, "")

		// executeBashStreaming has joined both stream goroutines by the time it
		// returns, so no further callback can fire. Take the lock anyway to
		// publish the final slice under the same mutex that guarded the writes.
		mu.Lock()
		snapshot := append([]string(nil), chunks...)
		mu.Unlock()

		done <- result{resp, snapshot, err}
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

// TestBashStreaming_NoTruncationOnFastExit is a regression test for output
// being silently truncated when the child exits while a scanner is still
// draining the kernel buffer.
//
// The cause was Cmd.Wait closing the pipes it owns as soon as it observes the
// child exit, without waiting for the reader. The fix gives the parent its own
// os.Pipe read ends, which Wait cannot touch.
//
// The race needs the child to exit immediately after writing a large payload,
// and it is scheduling-dependent, so this runs many iterations. On the
// unfixed code it reported a spurious "output truncated" notice on the first
// iteration under -cpu=1,2,4.
func TestBashStreaming_NoTruncationOnFastExit(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping repeated-iteration race check in short mode")
	}

	const wantBytes = 300000
	for i := range 40 {
		resp, chunks := runStreaming(t, `head -c 300000 /dev/zero | tr '\0' 'x'`)

		if strings.Contains(resp.Content, "output truncated") {
			t.Fatalf("iteration %d: output truncated even though the line is under the scanner limit", i)
		}
		total := 0
		for _, c := range chunks {
			total += len(c)
		}
		if len(chunks) != 1 || total != wantBytes {
			t.Fatalf("iteration %d: got %d chunk(s) totalling %d bytes, want 1 chunk of %d",
				i, len(chunks), total, wantBytes)
		}
	}
}

// TestBashStreaming_BackgroundProcessNoSpuriousNotice guards the interaction
// between the drain watchdog and the truncation notice.
//
// When a backgrounded grandchild keeps a write end open, the parent
// force-closes the pipes to avoid hanging. That makes the pending read fail,
// which looks like a scan error but is not truncation: the foreground command
// already completed and nothing it wrote was lost. Reporting it would put a
// bogus "[output truncated: file already closed]" line on the output of every
// `cmd &` invocation.
func TestBashStreaming_BackgroundProcessNoSpuriousNotice(t *testing.T) {
	resp, chunks := runStreaming(t, "echo started; sleep 300 &")

	if strings.Contains(resp.Content, "output truncated") {
		t.Errorf("backgrounded process produced a spurious truncation notice: %q", resp.Content)
	}
	if !strings.Contains(resp.Content, "started") {
		t.Errorf("foreground output missing, got %q", resp.Content)
	}
	for _, c := range chunks {
		if strings.Contains(c, "output truncated") {
			t.Errorf("spurious truncation notice streamed to callback: %q", c)
		}
	}
}

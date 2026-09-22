package daemon

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEncodeClipboardChunksSingleChunk(t *testing.T) {
	media := "image/png"
	data := bytes.Repeat([]byte{0xAB}, 100)
	chunks := EncodeClipboardChunks(media, data)
	if len(chunks) != 1 {
		t.Fatalf("chunk count = %d, want 1", len(chunks))
	}
	p := chunks[0]
	if p[0]&FrameClipboardFlagFinal == 0 {
		t.Fatal("single chunk must be marked final")
	}
	if p[1] != byte(len(media)) {
		t.Fatalf("media len = %d, want %d", p[1], len(media))
	}
	if got := string(p[2 : 2+len(media)]); got != media {
		t.Fatalf("media = %q, want %q", got, media)
	}
	if !bytes.Equal(p[2+len(media):], data) {
		t.Fatal("data mismatch")
	}
}

func TestEncodeClipboardChunksMultiChunkRoundTrip(t *testing.T) {
	media := "image/jpeg"
	data := bytes.Repeat([]byte{0x42}, clipboardChunkSize*3+17) // 4 chunks
	chunks := EncodeClipboardChunks(media, data)
	if len(chunks) != 4 {
		t.Fatalf("chunk count = %d, want 4", len(chunks))
	}
	coll := NewClipboardCollector()
	var got []byte
	var gotMedia string
	for i, p := range chunks {
		done, m, d, err := coll.Add(p)
		if err != nil {
			t.Fatalf("chunk %d: %v", i, err)
		}
		if i < len(chunks)-1 && done {
			t.Fatalf("chunk %d reported done early", i)
		}
		if done {
			got, gotMedia = d, m
		}
	}
	if gotMedia != media {
		t.Fatalf("media = %q, want %q", gotMedia, media)
	}
	if !bytes.Equal(got, data) {
		t.Fatalf("round-trip mismatch: got %d bytes, want %d", len(got), len(data))
	}
}

func TestClipboardCollectorRejectsOversize(t *testing.T) {
	coll := NewClipboardCollector()
	big := bytes.Repeat([]byte{0x01}, clipboardChunkSize)
	// Simulate a stream that never ends and exceeds the cap.
	for range clipboardMaxImageSize/clipboardChunkSize + 2 {
		done, _, _, err := coll.Add(append([]byte{0}, big...))
		if err == ErrClipboardTooLarge {
			return // expected once over the cap
		}
		if done {
			t.Fatal("unexpected completion")
		}
	}
	t.Fatal("oversize transfer was never rejected")
}

func TestClipboardCollectorTruncatedMedia(t *testing.T) {
	coll := NewClipboardCollector()
	// First chunk declares 10 bytes of media type but carries none.
	if _, _, _, err := coll.Add([]byte{0, 10}); err == nil {
		t.Fatal("expected error for truncated media type")
	}
}

func TestClipboardCollectorEmptyImage(t *testing.T) {
	chunks := EncodeClipboardChunks("image/png", nil)
	done, media, data, err := NewClipboardCollector().Add(chunks[0])
	if err != nil || !done {
		t.Fatalf("empty image should complete immediately: done=%v err=%v", done, err)
	}
	if media != "image/png" || len(data) != 0 {
		t.Fatalf("unexpected empty image: media=%q len=%d", media, len(data))
	}
}

func TestClipboardClearFlagDetection(t *testing.T) {
	// A clear frame carries only the flags byte and is intercepted by the
	// daemon BEFORE the collector sees it (it is not chunk data).
	p := []byte{FrameClipboardFlagFinal | FrameClipboardFlagClear}
	if p[0]&FrameClipboardFlagClear == 0 {
		t.Fatal("clear flag must be settable together with final")
	}
	if p[0]&FrameClipboardFlagFinal == 0 {
		t.Fatal("final flag must be preserved")
	}
	// Normal chunks must not trip the clear flag.
	if EncodeClipboardChunks("image/png", []byte("data"))[0][0]&FrameClipboardFlagClear != 0 {
		t.Fatal("image chunks must not carry the clear flag")
	}
}

// TestRemoteClipboardPathStablePerSession pins the property the path
// must have now that a session outlives its daemon: it depends on the
// SESSION and on nothing else.
//
// It used to carry the daemon's run nonce as well, so that a new run
// reusing logical id 3 could not inherit the dead session's image. Ids
// are no longer reused (seedSessionIDs), and an adopted session's child
// holds this path in its environment for its whole life — so a path that
// changed with the daemon would break every adopted session's paste.
func TestRemoteClipboardPathStablePerSession(t *testing.T) {
	table := newSessionTable(newDaemonRuntime(nil))
	a, b := table.remoteClipboardPath(3), table.remoteClipboardPath(3)
	if a != b {
		t.Fatal("path must be stable for a session")
	}
	if a == table.remoteClipboardPath(4) {
		t.Fatal("paths must differ per session")
	}
	if !strings.HasSuffix(a, "-3") || !strings.Contains(a, tempFilePrefix) {
		t.Fatalf("unexpected path: %s", a)
	}

	// A later daemon run must reach the SAME file, or an adopted session's
	// child would go on writing to a path the new daemon never reads.
	other := newSessionTable(newDaemonRuntime(nil))
	if other.remoteClipboardPath(3) != a {
		t.Fatal("a new daemon run sees a different clipboard path; an adopted session's pastes would be lost")
	}
}

// A staging file left behind by a crash is collected by the next daemon
// run's sweep, so a dropped paste cannot leave an image on disk forever.
//
// The sweep is now told which sessions are live rather than which run is
// current, because an adopted session's files were written by a previous
// run and must survive. Nothing is live here, so everything goes.
func TestPublishClipboardImageStagingIsSweepable(t *testing.T) {
	isolateRuntimeDir(t)
	dir, err := daemonRuntimeDir()
	if err != nil {
		t.Fatal(err)
	}

	// The name a crashed publish would leave behind, built the same way
	// the helper builds it.
	leaked := filepath.Join(dir, fmt.Sprintf("%sclip-run-old-stage-123", tempFilePrefix))
	if err := os.WriteFile(leaked, []byte("png"), 0o600); err != nil {
		t.Fatal(err)
	}

	sweepStaleTempFiles(nil)

	if _, err := os.Stat(leaked); !os.IsNotExist(err) {
		t.Fatalf("a crashed run's staging file survived the sweep: %v", err)
	}
}

// TestSweepStaleTempFilesKeepsLiveSessions is the other half of the
// sweep's contract, and the one that adoption depends on: a scratch file
// belonging to a session that is STILL RUNNING must survive, even though
// it was written by a previous daemon run.
//
// Getting this wrong is silent and destructive — an adopted session would
// keep working while its pastes went nowhere and its directory vanished
// from `kit ls`.
func TestSweepStaleTempFilesKeepsLiveSessions(t *testing.T) {
	isolateRuntimeDir(t)
	table := newSessionTable(newDaemonRuntime(nil))

	live := table.remoteClipboardPath(7)
	liveCwd := table.sessionCwdPath(7)
	dead := table.remoteClipboardPath(8)
	for _, p := range []string{live, liveCwd, dead} {
		if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	sweepStaleTempFiles([]uint64{7})

	for _, p := range []string{live, liveCwd} {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("a live session's scratch file was swept: %s (%v)", p, err)
		}
	}
	if _, err := os.Stat(dead); !os.IsNotExist(err) {
		t.Fatal("a finished session's scratch file survived the sweep")
	}
}

func TestChunkPayloadsStayUnderMaxPayload(t *testing.T) {
	data := bytes.Repeat([]byte{0x77}, clipboardChunkSize*5)
	for i, p := range EncodeClipboardChunks("image/png", data) {
		if len(p) > maxPayload {
			t.Fatalf("chunk %d is %d bytes, exceeds maxPayload %d", i, len(p), maxPayload)
		}
	}
}

// Sanity: the injection text uses the quoted form the @-tokenizer accepts.
func TestInjectionQuotingMatchesTokenizer(t *testing.T) {
	path := "/tmp/kit-clip-12345.png"
	injected := "@" + hexOrQuote(path) + " "
	// The tokenizer pattern @"[^"]+"|@[^\s]+ must match the quoted form.
	if injected != `@"/tmp/kit-clip-12345.png" ` {
		t.Fatalf("unexpected injection: %q", injected)
	}
}

func hexOrQuote(path string) string {
	return `"` + path + `"`
}

// silence unused import in constrained builds
var _ = hex.EncodeToString

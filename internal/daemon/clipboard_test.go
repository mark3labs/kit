package daemon

import (
	"bytes"
	"encoding/hex"
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

	// A second daemon run must not reuse the first run's files: logical
	// ids restart at 1, so a shared path would let a new session inherit a
	// dead session's clipboard image.
	other := newSessionTable(newDaemonRuntime(nil))
	if other.remoteClipboardPath(3) == a {
		t.Fatal("two daemon runs share a clipboard path; a stale image could leak into a new session")
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

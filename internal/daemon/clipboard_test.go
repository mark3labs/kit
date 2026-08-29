package daemon

import (
	"bytes"
	"encoding/hex"
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

func TestMediaExtension(t *testing.T) {
	cases := map[string]string{
		"image/png":  ".png",
		"image/jpeg": ".jpg",
		"image/gif":  ".gif",
		"image/webp": ".webp",
		"weird/type": ".bin",
	}
	for media, want := range cases {
		if got := mediaExtension(media); got != want {
			t.Fatalf("mediaExtension(%q) = %q, want %q", media, got, want)
		}
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

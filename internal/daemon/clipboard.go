package daemon

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Clipboard image transfer over the session wire.
//
// Terminals cannot paste binary image data through bracketed paste, and the
// host TUI reads the HOST's clipboard on ctrl+v — which is the wrong one
// when the session is driven via `kit remote`. Instead the CLIENT
// intercepts a bare ctrl+v, reads the client machine's clipboard
// (internal/clipboard), and streams the image to the daemon as chunked
// FrameClipboard frames. The daemon reassembles the bytes into a tempfile
// and injects @"<tempfile>" into the session child's input, where the
// normal @-attachment pipeline takes over (MIME detection, preview,
// multimodal submission).
//
// The sidecar relays FrameClipboard verbatim like DATA/RESIZE — no sidecar
// changes. Frames are tied to the session id like every other frame.
//
// FrameClipboard payload layout (client -> daemon):
//
//	byte 0      flags: bit 0 (0x01) = final chunk
//	first chunk only:
//	  byte 1    media type length (n)
//	  bytes 2..2+n  media type, e.g. "image/png"
//	  bytes 2+n..   first image bytes
//	continuation chunks: bytes 1.. are image bytes
//
// Chunk data uses the same 16 KiB budget as PTY DATA frames, keeping every
// frame under maxPayload.

const (
	// FrameClipboardFlagFinal marks the last chunk of a clipboard transfer.
	FrameClipboardFlagFinal byte = 0x01
	// FrameClipboardFlagClear marks a "clipboard has no image" signal: the
	// client sends it when Ctrl-V found no local image. The daemon clears
	// the session clipboard file so the child's Ctrl-V sees nothing.
	FrameClipboardFlagClear byte = 0x02

	// clipboardMaxImageSize caps reassembly so a hostile or buggy client
	// cannot exhaust daemon memory. Far beyond any real screenshot.
	clipboardMaxImageSize = 32 << 20
)

// clipboardChunkSize is the per-frame image byte budget (16 KiB).
const clipboardChunkSize = 16 * 1024

var (
	// ErrClipboardTooLarge is returned by the collector when a transfer
	// exceeds clipboardMaxImageSize.
	ErrClipboardTooLarge = errors.New("clipboard image too large")
)

// EncodeClipboardChunks splits an image into FrameClipboard payloads. The
// first chunk carries the media type; each payload's low bit marks the
// final chunk.
func EncodeClipboardChunks(mediaType string, data []byte) [][]byte {
	// First-chunk budget: flags(1) + mediaLen(1) + mediaType + data.
	const maxMedia = 255
	media := mediaType
	if len(media) > maxMedia {
		media = media[:maxMedia]
	}
	first := 1 + 1 + len(media)
	n := len(data)
	// Data split across chunks: the first chunk carries (clipboardChunkSize - first).
	var chunks [][]byte
	if n == 0 {
		// Empty image: single frame, no data.
		p := []byte{FrameClipboardFlagFinal, byte(len(media))}
		p = append(p, media...)
		return [][]byte{p}
	}
	firstData := clipboardChunkSize - first
	if firstData <= 0 {
		firstData = 1 // pathological media type; still make progress
	}
	count := 1
	remaining := n - firstData
	if remaining > 0 {
		count += (remaining + clipboardChunkSize - 1) / clipboardChunkSize
	}
	for i := 0; i < count; i++ {
		last := i == count-1
		var p []byte
		flags := byte(0)
		if last {
			flags |= FrameClipboardFlagFinal
		}
		if i == 0 {
			p = append(p, flags, byte(len(media)))
			p = append(p, media...)
			end := min(firstData, n)
			p = append(p, data[:end]...)
		} else {
			p = append(p, flags)
			start := firstData + (i-1)*clipboardChunkSize
			end := min(start+clipboardChunkSize, n)
			p = append(p, data[start:end]...)
		}
		chunks = append(chunks, p)
	}
	return chunks
}

// publishClipboardImage writes data to dest atomically: a child reading
// the file while a paste lands sees the old image or the new one, never a
// torn one.
//
// The staging file is created BESIDE dest rather than in the system temp
// directory. dest lives in the daemon's runtime directory, which is
// normally on a different filesystem from /tmp, and os.Rename across
// filesystems fails with EXDEV — which silently dropped every remote
// paste. Its name carries the session-scratch prefix and the run nonce, so
// a staging file left behind by a crash is collected by the same sweep as
// the published one.
func publishClipboardImage(dest, run string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(dest),
		fmt.Sprintf("%sclip-%s-stage-*", tempFilePrefix, run))
	if err != nil {
		return fmt.Errorf("stage clipboard image: %w", err)
	}
	path := tmp.Name()
	_, werr := tmp.Write(data)
	cerr := tmp.Close()
	if werr != nil || cerr != nil {
		_ = os.Remove(path)
		return fmt.Errorf("write clipboard image: %w", errors.Join(werr, cerr))
	}
	if err := os.Rename(path, dest); err != nil {
		_ = os.Remove(path)
		return fmt.Errorf("publish clipboard image: %w", err)
	}
	return nil
}

// ClipboardCollector reassembles a chunked clipboard transfer.
type ClipboardCollector struct {
	media   string
	buf     []byte
	started bool
}

// NewClipboardCollector returns a collector for one image transfer.
func NewClipboardCollector() *ClipboardCollector {
	return &ClipboardCollector{}
}

// Add consumes one FrameClipboard payload. On the final chunk it returns
// the complete image; the collector must then be discarded.
func (c *ClipboardCollector) Add(payload []byte) (done bool, mediaType string, data []byte, err error) {
	if len(payload) < 1 {
		return false, "", nil, fmt.Errorf("empty clipboard chunk")
	}
	final := payload[0]&FrameClipboardFlagFinal != 0
	body := payload[1:]
	if !c.started {
		if len(body) < 1 {
			return false, "", nil, fmt.Errorf("clipboard chunk missing media type")
		}
		n := int(body[0])
		if len(body) < 1+n {
			return false, "", nil, fmt.Errorf("truncated clipboard media type")
		}
		c.media = string(body[1 : 1+n])
		body = body[1+n:]
		c.started = true
	}
	c.buf = append(c.buf, body...)
	if len(c.buf) > clipboardMaxImageSize {
		return false, "", nil, ErrClipboardTooLarge
	}
	if final {
		return true, c.media, c.buf, nil
	}
	return false, "", nil, nil
}

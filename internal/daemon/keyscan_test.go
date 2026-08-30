package daemon

import (
	"bytes"
	"testing"
	"time"
)

// The wire bytes captured from a real kitty window with the protocol
// enabled the way the host TUI does (CSI = 3 ; 1 u).
var (
	kittyCtrlVPress   = []byte{0x1b, '[', '1', '1', '8', ';', '5', 'u'}
	kittyCtrlVRelease = []byte{0x1b, '[', '1', '1', '8', ';', '5', ':', '3', 'u'}
	kittyCtrlVRepeat  = []byte{0x1b, '[', '1', '1', '8', ';', '5', ':', '2', 'u'}
	kittyAPressRel    = []byte{0x1b, '[', '9', '7', ';', '1', ':', '3', 'u'}
)

func TestKeyScannerKittyCtrlVPress(t *testing.T) {
	k := &keyScanner{}
	evs := k.Feed(kittyCtrlVPress)
	if len(evs) != 1 || !evs[0].Paste || !bytes.Equal(evs[0].Data, kittyCtrlVPress) {
		t.Fatalf("expected paste event with original bytes, got %+v", evs)
	}
}

func TestKeyScannerKittyCtrlVReleaseCarriesData(t *testing.T) {
	k := &keyScanner{}
	_ = k.Feed(kittyCtrlVPress)
	evs := k.Feed(kittyCtrlVRelease)
	// The scanner never decides suppression: the release is reported with
	// its bytes and the CLIENT drops it after a successful interception.
	if len(evs) != 1 || !evs[0].Release || !bytes.Equal(evs[0].Data, kittyCtrlVRelease) {
		t.Fatalf("release should be reported with bytes, got %+v", evs)
	}
}

func TestKeyScannerKittyCtrlVReleaseForwardedWithoutPress(t *testing.T) {
	k := &keyScanner{}
	evs := k.Feed(kittyCtrlVRelease)
	if len(evs) != 1 || !evs[0].Release || !bytes.Equal(evs[0].Data, kittyCtrlVRelease) {
		t.Fatalf("release without press should forward, got %+v", evs)
	}
}

func TestKeyScannerLegacyCtrlV(t *testing.T) {
	k := &keyScanner{}
	evs := k.Feed([]byte{pasteKey})
	if len(evs) != 1 || !evs[0].Paste || !bytes.Equal(evs[0].Data, []byte{pasteKey}) {
		t.Fatalf("legacy ctrl+v should be a paste event with bytes, got %+v", evs)
	}
}

func TestKeyScannerPlainKeyPassesThrough(t *testing.T) {
	k := &keyScanner{}
	evs := k.Feed([]byte{'a'})
	if len(evs) != 1 || evs[0].Paste || !bytes.Equal(evs[0].Data, []byte{'a'}) {
		t.Fatalf("plain key should pass through, got %+v", evs)
	}
	k = &keyScanner{}
	evs = k.Feed(kittyAPressRel)
	if len(evs) != 1 || evs[0].Paste || !bytes.Equal(evs[0].Data, kittyAPressRel) {
		t.Fatalf("kitty plain key should pass through, got %+v", evs)
	}
}

func TestKeyScannerSequenceSplitAcrossChunks(t *testing.T) {
	k := &keyScanner{}
	evs := k.Feed(kittyCtrlVPress[:3])
	if len(evs) != 0 {
		t.Fatalf("partial sequence should emit nothing, got %+v", evs)
	}
	evs = k.Feed(kittyCtrlVPress[3:])
	if len(evs) != 1 || !evs[0].Paste {
		t.Fatalf("expected paste event after split feed, got %+v", evs)
	}
}

func TestKeyScannerRepeatIsPaste(t *testing.T) {
	k := &keyScanner{}
	evs := k.Feed(kittyCtrlVRepeat)
	if len(evs) != 1 || !evs[0].Paste {
		t.Fatalf("repeat should be a paste event, got %+v", evs)
	}
}

func TestKeyScannerMouseAndNonVUSequencesPassThrough(t *testing.T) {
	k := &keyScanner{}
	seqs := [][]byte{
		{0x1b, '[', '<', '0', ';', '5', ';', '1', '0', 'M'},
		{0x1b, '[', '2', '0', '0', '~'},
		{0x1b, '[', '9', '7', ';', '5', 'u'},
		{0x1b, '[', '1', '1', '8', ';', '1', 'u'},
	}
	for i, seq := range seqs {
		evs := k.Feed(seq)
		if len(evs) != 1 || evs[0].Paste || !bytes.Equal(evs[0].Data, seq) {
			t.Fatalf("seq %d should pass through unchanged, got %+v", i, evs)
		}
	}
}

func TestKeyScannerMixedBatch(t *testing.T) {
	k := &keyScanner{}
	chunk := append(append([]byte("abc"), kittyCtrlVPress...), 'x', 'y')
	evs := k.Feed(chunk)
	if len(evs) != 3 {
		t.Fatalf("expected data+paste+data, got %+v", evs)
	}
	if !bytes.Equal(evs[0].Data, []byte("abc")) || evs[0].Paste {
		t.Fatalf("first event mismatch: %+v", evs[0])
	}
	if !evs[1].Paste {
		t.Fatalf("second event should be paste: %+v", evs[1])
	}
	if !bytes.Equal(evs[2].Data, []byte("xy")) {
		t.Fatalf("third event mismatch: %+v", evs[2])
	}
}

func TestKeyScannerLoneEscapeFlushedAfterIdle(t *testing.T) {
	k := &keyScanner{}
	if evs := k.Feed([]byte{0x1b}); len(evs) != 0 {
		t.Fatalf("lone ESC should stay pending in the same chunk, got %+v", evs)
	}
	// An idle past escIdleFlush means a standalone Esc key press.
	time.Sleep(escIdleFlush + 20*time.Millisecond)
	evs := k.Feed([]byte{'x'})
	joined := []byte{}
	for _, ev := range evs {
		joined = append(joined, ev.Data...)
	}
	if !bytes.Equal(joined, []byte{0x1b, 'x'}) {
		t.Fatalf("idle ESC should flush with the following input, got %+v", evs)
	}
}

func TestKeyScannerCSISplitRightAfterEscapeStillDetected(t *testing.T) {
	k := &keyScanner{}
	// The read boundary falls between ESC and the rest of the sequence.
	if evs := k.Feed([]byte{0x1b}); len(evs) != 0 {
		t.Fatalf("ESC chunk should emit nothing, got %+v", evs)
	}
	evs := k.Feed(kittyCtrlVPress[1:])
	if len(evs) != 1 || !evs[0].Paste {
		t.Fatalf("CSI split after ESC should still be a paste event, got %+v", evs)
	}
}

func TestKeyScannerOversizeCSIFlushed(t *testing.T) {
	k := &keyScanner{}
	// A CSI that exceeds maxCSILen without a final byte is malformed.
	evs := k.Feed(append([]byte{0x1b, '['}, bytes.Repeat([]byte{0x31}, maxCSILen+10)...))
	if len(evs) != 1 {
		t.Fatalf("oversize CSI should flush as one data event, got %+v", evs)
	}
	// The scanner must recover and keep working.
	evs = k.Feed(kittyCtrlVPress)
	if len(evs) != 1 || !evs[0].Paste {
		t.Fatalf("scanner should work after flushing, got %+v", evs)
	}
}

func TestKeyScannerUnhandledCSIDoesNotDropFollowingBytes(t *testing.T) {
	k := &keyScanner{}
	chunk := append(append([]byte{}, []byte{0x1b, '[', 'A'}...), 'x')
	evs := k.Feed(chunk)
	joined := []byte{}
	for _, ev := range evs {
		joined = append(joined, ev.Data...)
		if ev.Paste || ev.Release {
			t.Fatalf("ctrl+a up-arrow must not be a paste/release: %+v", ev)
		}
	}
	if !bytes.Equal(joined, chunk) {
		t.Fatalf("bytes lost: sent %d, got %d", len(chunk), len(joined))
	}
}

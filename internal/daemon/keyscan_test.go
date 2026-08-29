package daemon

import (
	"bytes"
	"testing"
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
	if len(evs) != 1 || !evs[0].Paste {
		t.Fatalf("expected paste event, got %+v", evs)
	}
}

func TestKeyScannerKittyCtrlVReleaseSwallowedAfterPress(t *testing.T) {
	k := &keyScanner{}
	_ = k.Feed(kittyCtrlVPress)
	evs := k.Feed(kittyCtrlVRelease)
	if len(evs) != 0 {
		t.Fatalf("release after press should be swallowed, got %+v", evs)
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
	if len(evs) != 1 || !evs[0].Paste {
		t.Fatalf("legacy ctrl+v should be a paste event, got %+v", evs)
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

package daemon

import (
	"bytes"
	"testing"
)

func TestFrameRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	payload := []byte("hello \x00 world")
	if err := WriteFrame(&buf, FrameData, 42, payload); err != nil {
		t.Fatalf("WriteFrame: %v", err)
	}
	frame, err := ReadFrame(&buf)
	if err != nil {
		t.Fatalf("ReadFrame: %v", err)
	}
	if frame.Type != FrameData || frame.Session != 42 || !bytes.Equal(frame.Payload, payload) {
		t.Fatalf("round trip mismatch: %+v", frame)
	}
}

func TestFrameRoundTripEmptyPayload(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteFrame(&buf, FrameSessionClosed, 7, nil); err != nil {
		t.Fatalf("WriteFrame: %v", err)
	}
	frame, err := ReadFrame(&buf)
	if err != nil {
		t.Fatalf("ReadFrame: %v", err)
	}
	if frame.Type != FrameSessionClosed || frame.Session != 7 || len(frame.Payload) != 0 {
		t.Fatalf("round trip mismatch: %+v", frame)
	}
}

func TestFrameRejectsOversizedPayload(t *testing.T) {
	if err := WriteFrame(&bytes.Buffer{}, FrameData, 0, make([]byte, maxPayload+1)); err == nil {
		t.Fatal("expected error for oversized payload")
	}
}

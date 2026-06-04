package ui

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"

	uicore "github.com/mark3labs/kit/internal/ui/core"
)

// makeTestPNG builds a small solid-color PNG for transcript preview tests.
func makeTestPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{R: 200, G: 40, B: 90, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

func TestTranscriptPreviewCmdNoImages(t *testing.T) {
	m, _, _ := newTestAppModel(nil)
	if cmd := m.transcriptPreviewCmd(nil, ""); cmd != nil {
		t.Error("expected nil cmd when there are no images")
	}
}

func TestTranscriptPreviewCmdRendersBlock(t *testing.T) {
	m, _, _ := newTestAppModel(nil)
	images := []uicore.ImageAttachment{
		{Data: makeTestPNG(t, 16, 16), MediaType: "image/png"},
	}
	cmd := m.transcriptPreviewCmd(images, "anchor-1")
	if cmd == nil {
		t.Fatal("expected a non-nil cmd for a valid image")
	}
	msg := cmd()
	// The result depends on the test process color profile. When the
	// terminal supports color the cmd yields a preview block; otherwise it
	// yields nil (caller keeps the text badge). Both are valid — assert the
	// shape only when a block is produced.
	if msg == nil {
		t.Skip("color profile below ANSI256 in test env; preview correctly skipped")
	}
	ready, ok := msg.(imagePreviewReadyMsg)
	if !ok {
		t.Fatalf("expected imagePreviewReadyMsg, got %T", msg)
	}
	if !strings.Contains(ready.block, "▀") {
		t.Errorf("preview block should contain half-block glyphs, got %q", ready.block)
	}
	if ready.anchorID != "anchor-1" {
		t.Errorf("preview should carry the originating anchorID, got %q", ready.anchorID)
	}
}

func TestImagePreviewReadyMsgAppendsItem(t *testing.T) {
	m, _, _ := newTestAppModel(nil)
	before := len(m.messages)
	m = sendMsg(m, imagePreviewReadyMsg{block: "\x1b[38;2;1;2;3;48;2;4;5;6m▀\x1b[0m"})
	if len(m.messages) != before+1 {
		t.Fatalf("expected one appended message item, got %d (was %d)", len(m.messages), before)
	}
	last, ok := m.messages[len(m.messages)-1].(*TextMessageItem)
	if !ok {
		t.Fatalf("expected last item to be *TextMessageItem, got %T", m.messages[len(m.messages)-1])
	}
	if !strings.Contains(last.Render(0), "▀") {
		t.Error("appended preview item should render the half-block block verbatim")
	}
}

// TestImagePreviewReadyMsgInsertsAfterAnchor verifies the preview is placed
// directly after its originating user message even when a later message (e.g.
// a streamed assistant reply) was already appended while the thumbnail was
// being decoded asynchronously.
func TestImagePreviewReadyMsgInsertsAfterAnchor(t *testing.T) {
	m, _, _ := newTestAppModel(nil)
	userItem := NewStyledMessageItem("user-anchor", "user", "hi", "hi")
	assistantItem := NewStyledMessageItem("assistant-1", "assistant", "reply", "reply")
	m.messages = append(m.messages, userItem, assistantItem)

	m = sendMsg(m, imagePreviewReadyMsg{
		block:    "\x1b[38;2;1;2;3;48;2;4;5;6m▀\x1b[0m",
		anchorID: "user-anchor",
	})

	// Expect order: user, preview, assistant.
	if len(m.messages) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(m.messages))
	}
	if m.messages[0].ID() != "user-anchor" {
		t.Errorf("messages[0] should be the user message, got %q", m.messages[0].ID())
	}
	if m.messages[2].ID() != "assistant-1" {
		t.Errorf("messages[2] should be the assistant message, got %q", m.messages[2].ID())
	}
	if !strings.Contains(m.messages[1].Render(0), "▀") {
		t.Errorf("messages[1] should be the inserted preview, got %q", m.messages[1].Render(0))
	}
}

// TestImagePreviewReadyMsgUnknownAnchorAppends verifies that when the anchor
// is missing (e.g. the message was cleared), the preview falls back to append.
func TestImagePreviewReadyMsgUnknownAnchorAppends(t *testing.T) {
	m, _, _ := newTestAppModel(nil)
	m.messages = append(m.messages, NewStyledMessageItem("only", "user", "hi", "hi"))
	m = sendMsg(m, imagePreviewReadyMsg{
		block:    "\x1b[38;2;1;2;3;48;2;4;5;6m▀\x1b[0m",
		anchorID: "does-not-exist",
	})
	if len(m.messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(m.messages))
	}
	if !strings.Contains(m.messages[1].Render(0), "▀") {
		t.Error("preview should be appended as the last item when anchor is unknown")
	}
}

func TestImagePreviewReadyMsgEmptyBlockIgnored(t *testing.T) {
	m, _, _ := newTestAppModel(nil)
	before := len(m.messages)
	m = sendMsg(m, imagePreviewReadyMsg{block: ""})
	if len(m.messages) != before {
		t.Errorf("empty preview block should not append an item; got %d (was %d)", len(m.messages), before)
	}
}

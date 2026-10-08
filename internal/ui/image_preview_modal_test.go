package ui

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/mark3labs/kit/internal/ui/core"
)

func TestImageTilesWrapAndHit(t *testing.T) {
	in := NewInputComponent(52, nil)
	in.pendingImages = make([]core.ImageAttachment, 3)
	in.imageThumbs = []string{"one", "two", "three"}
	rects := in.imageTileRects()
	if rects[0].row != rects[1].row || rects[2].row <= rects[1].row {
		t.Fatalf("tile layout: %+v", rects)
	}
	for i, r := range rects {
		index, remove := in.ImageTileAt(r.col, r.row)
		if index != i || remove {
			t.Fatalf("tile %d click: %d %v", i, index, remove)
		}
		index, remove = in.ImageTileAt(r.col+r.width-1, r.row+r.height-1)
		if index != i || !remove {
			t.Fatalf("tile %d remove click: %d %v", i, index, remove)
		}
	}
	if lipgloss.Width(in.View().Content) > 52 {
		t.Fatal("tiles exceed input width")
	}
}

func TestImageTilePreservesANSI(t *testing.T) {
	in := NewInputComponent(80, nil)
	in.pendingImages = make([]core.ImageAttachment, 2)
	in.imageThumbs = []string{"\x1b[31m█\x1b[0m", "\x1b[32m█\x1b[0m"}
	view := in.View().Content
	for _, thumb := range in.imageThumbs {
		if !strings.Contains(view, thumb) {
			t.Fatalf("lost image colors: %q", view)
		}
	}
}

func TestRemoveImageInvalidatesAllPendingRenders(t *testing.T) {
	in := NewInputComponent(80, nil)
	in.pendingImages = []core.ImageAttachment{{MediaType: "first"}, {MediaType: "second"}, {MediaType: "third"}}
	in.imageThumbs = []string{"", "", ""}
	in.RemoveImage(1)
	if len(in.pendingImages) != 2 || in.pendingImages[1].MediaType != "third" {
		t.Fatal("wrong attachment removed")
	}
	if in.imageGen != 1 || len(in.imageThumbs) != 2 || len(in.imageIDs) != 2 || len(in.imagePlace) != 2 {
		t.Fatal("render caches not reset")
	}
}

func TestImagePreviewFitsAndCloses(t *testing.T) {
	var data bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 600, 100))
	img.Set(0, 0, color.White)
	if err := png.Encode(&data, img); err != nil {
		t.Fatal(err)
	}
	in := NewInputComponent(80, nil)
	in.pendingImages = []core.ImageAttachment{{Data: data.Bytes(), MediaType: "image/png"}}
	m := &AppModel{input: in, width: 80, height: 24, state: stateInput}
	m.openImagePreview(0)
	ready := m.renderImagePreviewCmd()().(attachmentPreviewReadyMsg)
	if ready.thumb == "" {
		t.Fatal("no preview rendered")
	}
	m.updateImagePreview(ready)
	box := m.renderImagePreview()
	if lipgloss.Width(box) > m.width || lipgloss.Height(box) > m.height {
		t.Fatal("modal exceeds viewport")
	}
	if lipgloss.Height(ready.thumb) >= 16 {
		t.Fatal("wide image was stretched")
	}
	m.updateImagePreview(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.imagePreview != nil || m.state != stateInput {
		t.Fatal("close did not restore input")
	}
	m.openImagePreview(0)
	m.updateImagePreview(ready)
	if m.imagePreview.content != "" {
		t.Fatal("accepted stale preview result")
	}
}

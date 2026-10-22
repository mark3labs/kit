package test

import (
	"testing"

	"github.com/mark3labs/kit/pkg/extensions"
)

// TestRenderSizedCrossesYaegiBoundary checks dimensions and a later helper call.
func TestRenderSizedCrossesYaegiBoundary(t *testing.T) {
	h := New(t)
	h.LoadString(`package main
import (
 "fmt"
 "kit/ext"
)
func Init(api ext.API) {
 api.OnSessionStart(func(_ ext.SessionStartEvent, ctx ext.Context) {
  ctx.SetWidget(ext.WidgetConfig{ID:"fill", Placement:ext.WidgetAbove, Content:ext.WidgetContent{
   FillHeight:true,
   RenderSized:func(w,h int) string { return panel(w,h) },
  }})
 })
}
// The closure above must call this later helper, not use a bare reference.
func panel(w,h int) string { return fmt.Sprintf("%dx%d",w,h) }
`, "fill.go")
	if _, err := h.Emit(extensions.SessionStartEvent{}); err != nil {
		t.Fatal(err)
	}
	w := h.Context().Widgets["fill"].Content
	if !w.FillHeight || w.RenderSized == nil {
		t.Fatal("fill fields lost")
	}
	if got := w.RenderSized(72, 19); got != "72x19" {
		t.Fatalf("RenderSized=%q", got)
	}
}

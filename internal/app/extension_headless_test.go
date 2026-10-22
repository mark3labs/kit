package app

import (
	"testing"

	"github.com/mark3labs/kit/internal/extensions"
)

func TestExtensionHeadlessNotifications(t *testing.T) {
	for _, quiet := range []bool{false, true} {
		a := New(Options{Quiet: quiet}, nil)
		stdout, stderr := captureStdStreams(t, func() {
			a.SetEditorTextFromExtension("ignored")
			a.NotifyModelChanged("ignored", "ignored")
			a.PrintBlockFromExtension(extensions.PrintBlockOpts{Text: "body", Subtitle: "title", Markdown: true})
			a.PrintBlockFromExtension(extensions.PrintBlockOpts{Text: "plain"})
		})
		if stdout != "" || stderr != "--- title ---\nbody\n---\nplain\n---\n" {
			t.Fatalf("output = %q, %q", stdout, stderr)
		}
		a.QuitFromExtension()
		if a.Context().Err() == nil {
			t.Error("headless exit did not cancel context")
		}
		a.Close()
	}
}

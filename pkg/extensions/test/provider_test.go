package test

import (
	"testing"

	ext "github.com/mark3labs/kit/pkg/extensions"
)

func TestProviderCallbacksThroughPublicHarness(t *testing.T) {
	h := New(t)
	loaded := h.LoadFile("../../../examples/extensions/fake-provider.go")
	if len(loaded.Providers) != 1 {
		t.Fatalf("providers=%d", len(loaded.Providers))
	}
	created := loaded.Providers[0].Create(ext.ProviderCreateRequest{Model: "echo"})
	if created.Error != nil {
		t.Fatal(created.Error)
	}
	text := ""
	finished := false
	result := created.Runtime.Stream(ext.ProviderRequest{}, func(event ext.ProviderEvent) bool {
		text += event.Delta
		if event.Type == ext.ProviderFinish {
			finished = true
		}
		return true
	})
	if result.Error != nil || text != "Hello from a fake provider." || !finished {
		t.Fatalf("text=%q finish=%v error=%v", text, finished, result.Error)
	}
	if err := created.Runtime.Close(); err != nil {
		t.Fatal(err)
	}
}

package zen

import (
	"slices"
	"testing"
)

func TestPickReturnsKnownWisdom(t *testing.T) {
	for range 128 {
		got := Pick()
		if !slices.Contains(Aphorisms, got) {
			t.Fatalf("Pick() returned unknown wisdom %q", got)
		}
	}
}

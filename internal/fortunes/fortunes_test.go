package fortunes

import (
	"strings"
	"testing"
)

func TestAllReturnsFullDeck(t *testing.T) {
	got := All()
	if len(got) != len(cookies) {
		t.Fatalf("All() returned %d fortunes, want %d", len(got), len(cookies))
	}
	for i, f := range got {
		if strings.TrimSpace(f) == "" {
			t.Errorf("fortune %d is empty", i)
		}
		// The copy must not alias the deck: callers may mutate their slice.
		if len(cookies) > 0 && f != cookies[i] {
			t.Errorf("fortune %d = %q, want %q", i, f, cookies[i])
		}
	}
}

func TestAllDoesNotAliasDeck(t *testing.T) {
	got := All()
	if len(got) > 0 {
		got[0] = "mutated by a careless caller"
		if cookies[0] == got[0] {
			t.Fatal("All() aliases the internal deck; mutations leak through")
		}
	}
}

func TestRandomReturnsKnownFortune(t *testing.T) {
	deck := make(map[string]bool, len(cookies))
	for _, f := range cookies {
		deck[f] = true
	}
	for range 64 {
		f := Random()
		if !deck[f] {
			t.Fatalf("Random() returned unknown fortune %q", f)
		}
	}
}

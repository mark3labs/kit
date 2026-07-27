package commands

import (
	"slices"
	"testing"
)

func TestFooterCommandAcceptsArguments(t *testing.T) {
	cmd := GetCommandByName("/footer")
	if cmd == nil {
		t.Fatal("GetCommandByName(/footer) returned nil")
	}
	if !cmd.HasArgs {
		t.Fatal("/footer HasArgs = false; want true for /footer off and /footer fields ...")
	}

	for _, prefix := range []string{"off", "fields"} {
		if got := cmd.Complete(prefix); !slices.Contains(got, prefix) {
			t.Errorf("/footer completion for %q = %v; want %q", prefix, got, prefix)
		}
	}
}

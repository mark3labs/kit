package cmd

import (
	"errors"
	"strings"
	"testing"
	"testing/iotest"
)

func TestSessionCommandGroup(t *testing.T) {
	for _, name := range []string{"attach", "ls", "rename", "kill"} {
		command, _, err := rootCmd.Find([]string{"session", name})
		if err != nil || command.Name() != name || command.Parent() != sessionCmd {
			t.Fatalf("missing session %s: %v", name, err)
		}
		if command.Deprecated != "" {
			t.Fatalf("session %s must not be deprecated", name)
		}
	}
	for _, name := range []string{"attach", "ls"} {
		command, _, err := rootCmd.Find([]string{name})
		if err != nil || command.Parent() != rootCmd || command.Deprecated == "" {
			t.Fatalf("missing deprecated alias %s", name)
		}
	}
}

func TestReadSessionConfirmation(t *testing.T) {
	for _, tc := range []struct {
		input string
		want  bool
	}{{"", false}, {"n", false}, {"\n", false}, {"y", true}, {"YES", true}, {"yes\n", true}} {
		got, err := readSessionConfirmation(strings.NewReader(tc.input))
		if err != nil || got != tc.want {
			t.Fatalf("input %q: %v, %v", tc.input, got, err)
		}
	}
	wantErr := errors.New("read failed")
	if _, err := readSessionConfirmation(iotest.ErrReader(wantErr)); !errors.Is(err, wantErr) {
		t.Fatalf("error not preserved: %v", err)
	}
}

func TestParseSessionID(t *testing.T) {
	for _, value := range []string{"0", "-1", "x", "18446744073709551616"} {
		if _, err := parseSessionID(value); err == nil {
			t.Fatalf("accepted %q", value)
		}
	}
	if id, err := parseSessionID("28"); err != nil || id != 28 {
		t.Fatalf("valid ID: %d %v", id, err)
	}
}

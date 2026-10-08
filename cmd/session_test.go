package cmd

import "testing"

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

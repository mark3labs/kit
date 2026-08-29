package cmd

import "testing"

func TestRemoteSubcommandSelected(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want bool
	}{
		{"plain", []string{"remote", "--host", "zora"}, true},
		{"global flag with value before remote", []string{"--config", "/tmp/broken.yml", "remote", "--list"}, true},
		{"attached value", []string{"--config=/tmp/x", "remote", "--pair", "ABCD2345"}, true},
		{"bool flag before remote", []string{"--debug", "remote", "--list"}, true},
		{"no remote", []string{"--config", "/tmp/x"}, false},
		{"other subcommand", []string{"daemon", "pair"}, false},
		{"empty", nil, false},
		{"remote as flag value", []string{"--model", "remote"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := remoteSubcommandSelected(tc.args); got != tc.want {
				t.Fatalf("remoteSubcommandSelected(%v) = %v, want %v", tc.args, got, tc.want)
			}
		})
	}
}

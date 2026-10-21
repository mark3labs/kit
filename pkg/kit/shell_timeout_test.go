package kit

import (
	"strings"
	"testing"

	"github.com/spf13/viper"
)

// fileStore returns a configuration store holding only the given YAML, so a
// test can stand in for a configuration file without touching the disk.
func fileStore(t *testing.T, yml string) *viper.Viper {
	t.Helper()
	v := viper.New()
	v.SetConfigType("yaml")
	if err := v.ReadConfig(strings.NewReader(yml)); err != nil {
		t.Fatalf("ReadConfig: %v", err)
	}
	return v
}

// Precedence follows the source, not the spelling: SDK option, environment,
// configuration file; within one source the shell-named spelling wins.
func TestResolveShellTimeouts_SourceBeatsSpelling(t *testing.T) {
	for _, key := range []string{"KIT_SHELL_TIMEOUT", "KIT_BASH_TIMEOUT", "KIT_SHELL_MAX_TIMEOUT", "KIT_BASH_MAX_TIMEOUT"} {
		t.Setenv(key, "")
	}

	// Within one source the shell-named spelling wins.
	if got, _ := resolveShellTimeouts(&Options{}, fileStore(t, "shell-timeout: 30\nbash-timeout: 90\n")); got != 30 {
		t.Errorf("both spellings in the file: timeout = %d, want 30", got)
	}

	// The earlier spelling in the environment beats the current spelling in
	// the file, because the environment is the higher source.
	t.Setenv("KIT_BASH_TIMEOUT", "90")
	if got, _ := resolveShellTimeouts(&Options{}, fileStore(t, "shell-timeout: 60\n")); got != 90 {
		t.Errorf("earlier env over current file: timeout = %d, want 90", got)
	}
	t.Setenv("KIT_BASH_TIMEOUT", "")

	// The current spelling in the environment beats the earlier spelling in
	// the file.
	t.Setenv("KIT_SHELL_TIMEOUT", "45")
	if got, _ := resolveShellTimeouts(&Options{}, fileStore(t, "bash-timeout: 90\n")); got != 45 {
		t.Errorf("current env over earlier file: timeout = %d, want 45", got)
	}
	t.Setenv("KIT_SHELL_TIMEOUT", "")

	// SDK options beat both environment spellings and file values.
	t.Setenv("KIT_SHELL_TIMEOUT", "45")
	t.Setenv("KIT_BASH_TIMEOUT", "90")
	t.Setenv("KIT_SHELL_MAX_TIMEOUT", "500")
	t.Setenv("KIT_BASH_MAX_TIMEOUT", "700")
	gotTimeout, gotMax := resolveShellTimeouts(
		&Options{ShellTimeout: 15, ShellMaxTimeout: 300},
		fileStore(t, "shell-timeout: 60\nbash-timeout: 90\nshell-max-timeout: 600\nbash-max-timeout: 900\n"),
	)
	if gotTimeout != 15 || gotMax != 300 {
		t.Errorf("SDK over env and file: timeouts = (%d, %d), want (15, 300)", gotTimeout, gotMax)
	}
	t.Setenv("KIT_SHELL_TIMEOUT", "")
	t.Setenv("KIT_BASH_TIMEOUT", "")
	t.Setenv("KIT_SHELL_MAX_TIMEOUT", "")
	t.Setenv("KIT_BASH_MAX_TIMEOUT", "")

	// Keep support for both earlier config-file keys.
	gotTimeout, gotMax = resolveShellTimeouts(&Options{}, fileStore(t, "bash-timeout: 90\nbash-max-timeout: 700\n"))
	if gotTimeout != 90 || gotMax != 700 {
		t.Errorf("earlier file keys: timeouts = (%d, %d), want (90, 700)", gotTimeout, gotMax)
	}

	// The maximum timeout follows the same rule.
	t.Setenv("KIT_BASH_MAX_TIMEOUT", "700")
	if _, got := resolveShellTimeouts(&Options{}, fileStore(t, "shell-max-timeout: 500\n")); got != 700 {
		t.Errorf("earlier env over current file: max timeout = %d, want 700", got)
	}
	t.Setenv("KIT_BASH_MAX_TIMEOUT", "")
}

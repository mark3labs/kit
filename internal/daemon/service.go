package daemon

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// systemd user-service management for `kit daemon`. The unit keeps the
// daemon attached to the user session (no root, inherits HOME so the lock,
// state, and session storage resolve to the same paths as interactive use).

const (
	serviceUnitName = "kit.service"
	serviceDesc     = "kit daemon — remote kit sessions over iroh"
)

func systemdUnitPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("daemon: resolve home: %w", err)
	}
	return filepath.Join(home, ".config", "systemd", "user", serviceUnitName), nil
}

func systemctlUser(ctx context.Context, args ...string) error {
	if os.Getenv("XDG_RUNTIME_DIR") == "" {
		return errors.New("systemd user session not available (XDG_RUNTIME_DIR is unset)")
	}
	if _, err := exec.LookPath("systemctl"); err != nil {
		return errors.New("systemctl not found — this machine does not use systemd")
	}
	cmd := exec.CommandContext(ctx, "systemctl", append([]string{"--user"}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		detail := strings.TrimSpace(string(out))
		if detail != "" {
			return fmt.Errorf("systemctl %s: %s", strings.Join(args, " "), detail)
		}
		return fmt.Errorf("systemctl %s: %w", strings.Join(args, " "), err)
	}
	return nil
}

// envFileVars returns the caller's environment variables a headless daemon
// needs: provider credentials and endpoint overrides (systemd starts the
// service with a minimal environment, so API keys set in the shell would
// be missing and remote sessions would fail at agent creation).
func envFileVars() []string {
	prefixes := []string{
		"PROVIDER_", "OPENCODE_", "ANTHROPIC_", "OPENAI_", "GEMINI_",
		"GOOGLE_", "AZURE_", "GROQ_", "MISTRAL_", "DEEPSEEK_",
		"OPENROUTER_", "XAI_", "KIT_",
	}
	suffixes := []string{"_API_KEY", "_TOKEN", "_URL", "_CREDENTIALS"}
	var out []string
	for _, kv := range os.Environ() {
		k, v, ok := strings.Cut(kv, "=")
		if !ok || v == "" {
			continue
		}
		match := k == "PATH"
		for _, p := range prefixes {
			if strings.HasPrefix(k, p) {
				match = true
				break
			}
		}
		if !match {
			for _, sfx := range suffixes {
				if strings.HasSuffix(k, sfx) {
					match = true
					break
				}
			}
		}
		if match {
			out = append(out, kv)
		}
	}
	return out
}

// writeServiceEnvFile seeds ~/.config/kit/daemon.env with the provider
// environment captured from the caller. An existing file is left untouched
// so user edits survive reinstalls. Returns whether a new file was written.
func writeServiceEnvFile() (string, bool, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", false, err
	}
	path := filepath.Join(home, ".config", "kit", "daemon.env")
	if _, err := os.Stat(path); err == nil {
		return path, false, nil
	}
	vars := envFileVars()
	var b strings.Builder
	b.WriteString("# Environment for the kit daemon systemd service.\n")
	b.WriteString("# Written by 'kit daemon service install' from the installing shell;\n# edit freely and restart with: systemctl --user restart kit\n\n")
	for _, kv := range vars {
		b.WriteString(kv)
		b.WriteString("\n")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return path, false, err
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
		return path, false, err
	}
	return path, true, nil
}

// InstallSystemService writes the user unit and enables + starts it. When a
// daemon is already running interactively, the service's own lock attempt
// would fail forever, so refuse here instead of creating a restart loop.
func InstallSystemService(ctx context.Context) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("daemon: resolve kit binary: %w", err)
	}
	if st := ReadStatus(); st.Running {
		// State may legitimately be nil: the lock is held before the first
		// state snapshot is persisted.
		pid := "unknown"
		if st.State != nil {
			pid = fmt.Sprint(st.State.PID)
		}
		return fmt.Errorf("daemon: an instance is already running (pid %s) — stop it before installing the service", pid)
	}

	envPath, envCreated, err := writeServiceEnvFile()
	if err != nil {
		return fmt.Errorf("daemon: environment file: %w", err)
	}

	unitPath, err := systemdUnitPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(unitPath), 0o755); err != nil {
		return fmt.Errorf("daemon: unit dir: %w", err)
	}
	unit := fmt.Sprintf(`[Unit]
Description=%s
Documentation=https://github.com/mark3labs/kit
After=network-online.target
StartLimitIntervalSec=30
StartLimitBurst=5

[Service]
Type=simple
ExecStart=%s daemon
EnvironmentFile=-%s
Restart=on-failure
RestartSec=5
# The daemon owns its sessions' terminals and shuts them down itself on
# SIGTERM. KillMode=mixed sends the signal to the daemon only, so it gets
# to end each session cleanly; the default (control-group) would signal
# every child at once and cut sessions off mid-turn. TimeoutStopSec is the
# backstop if the daemon fails to finish.
KillMode=mixed
TimeoutStopSec=15

[Install]
WantedBy=default.target
`, serviceDesc, exe, envPath)
	if err := os.WriteFile(unitPath, []byte(unit), 0o644); err != nil {
		return fmt.Errorf("daemon: write unit: %w", err)
	}

	if err := systemctlUser(ctx, "daemon-reload"); err != nil {
		return err
	}
	if err := systemctlUser(ctx, "enable", "--now", serviceUnitName); err != nil {
		return err
	}
	fmt.Printf("  Installed and started %s\n", unitPath)
	if envCreated {
		fmt.Printf("  Provider environment captured to %s (0600).\n", envPath)
		fmt.Println("  Add or change keys there and run: systemctl --user restart kit")
	}
	fmt.Println("  Manage it with: systemctl --user status kit")
	fmt.Printf("  Show the pairing code with: kit daemon status\n")
	return nil
}

// RemoveSystemService stops, disables, and deletes the user unit.
func RemoveSystemService(ctx context.Context) error {
	unitPath, err := systemdUnitPath()
	if err != nil {
		return err
	}
	if _, err := os.Stat(unitPath); err != nil {
		return fmt.Errorf("daemon: %s is not installed (%s not found)", serviceUnitName, unitPath)
	}
	if err := systemctlUser(ctx, "disable", "--now", serviceUnitName); err != nil {
		return err
	}
	if err := os.Remove(unitPath); err != nil {
		return fmt.Errorf("daemon: remove unit: %w", err)
	}
	_ = systemctlUser(ctx, "daemon-reload")
	fmt.Printf("  Stopped and removed %s\n", unitPath)
	return nil
}

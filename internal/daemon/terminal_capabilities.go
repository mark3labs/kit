package daemon

import (
	"fmt"
	"os"
	"path/filepath"
)

// RemoteTerminalFileEnv names the capability file for a daemon session.
// The session host replaces this file before sending SIGWINCH to the child.
// Read it at startup and on each window-size event. A missing file means
// that live capability updates are not available; keep the current settings.
const RemoteTerminalFileEnv = "KIT_REMOTE_TERMINAL_FILE"

// ReadTerminalCapabilities reads the latest attached terminal's capabilities.
// ColorProfile is optional for compatibility with older clients. Its numeric
// values are 1 (no terminal), 2 (no color), 3 (16 colors), 4 (256 colors),
// and 5 (true color). An empty background means it was not reported;
// BackgroundUnknown means the client queried it but received no answer.
func ReadTerminalCapabilities(path string) (TerminalInfo, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return TerminalInfo{}, fmt.Errorf("daemon: read terminal capabilities: %w", err)
	}
	info, err := DecodeTerminalInfo(data)
	if err != nil {
		return TerminalInfo{}, fmt.Errorf("daemon: read terminal capabilities: %w", err)
	}
	return info, nil
}

// writeTerminalCapabilities publishes a complete snapshot. Atomic replacement
// lets the child read without locks and never exposes partial JSON.
func writeTerminalCapabilities(path string, info TerminalInfo) error {
	info.Background = backgroundEnvValue(info.Background)
	data, err := EncodeTerminalInfo(info)
	if err != nil {
		return fmt.Errorf("daemon: encode terminal capabilities: %w", err)
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".terminal-*")
	if err != nil {
		return fmt.Errorf("daemon: create terminal capabilities: %w", err)
	}
	defer func() { _ = os.Remove(f.Name()) }()
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return fmt.Errorf("daemon: write terminal capabilities: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("daemon: close terminal capabilities: %w", err)
	}
	if err := os.Rename(f.Name(), path); err != nil {
		return fmt.Errorf("daemon: publish terminal capabilities: %w", err)
	}
	return nil
}

// terminalUpdater is optional so transports from older implementations and
// test transports can continue to use sessionIO without live updates.
type terminalUpdater interface {
	UpdateTerminal(TerminalInfo) error
}

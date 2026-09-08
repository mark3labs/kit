package extensions

import (
	"encoding/json"
	"strings"
)

// ParseToolArgs decodes a raw JSON tool-argument payload into a map.
//
// It returns nil for empty, whitespace-only, or malformed input. Callers treat
// nil as "no arguments" rather than as an error: this is non-fatal convenience
// parsing shared by the extension wrapper, the SDK event bridge, and the TUI
// activity row, none of which may fail because a payload did not decode.
func ParseToolArgs(raw string) map[string]any {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil
	}
	return parsed
}

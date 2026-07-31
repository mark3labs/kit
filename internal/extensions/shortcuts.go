package extensions

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/log"
)

// ---------------------------------------------------------------------------
// Shortcut key normalization
// ---------------------------------------------------------------------------
//
// Kit matches extension shortcuts against two different renderings of a key
// press produced by the terminal layer:
//
//   - the key's *text* (what the key types), e.g. "A" for Shift+A, "?" for
//     Shift+/
//   - the key's *keystroke*, e.g. "shift+a", "ctrl+g", "f1"
//
// Extension authors naturally write the keystroke form ("shift+a"), so the
// dispatcher tries both. Normalizing at registration time means an author can
// write "Ctrl+Shift+S", "control+shift+s" or "shift+ctrl+s" and still land on
// the single canonical spelling the terminal will produce.

// modifierOrder is the canonical emission order. It matches the order used by
// the terminal layer's Keystroke() rendering, so a normalized key compares
// equal to a real key press by plain string equality.
var modifierOrder = []string{"ctrl", "alt", "shift", "meta", "hyper", "super"}

// modifierAliases maps accepted modifier spellings to their canonical form.
var modifierAliases = map[string]string{
	"ctrl":    "ctrl",
	"control": "ctrl",
	"alt":     "alt",
	"opt":     "alt",
	"option":  "alt",
	"shift":   "shift",
	"meta":    "meta",
	"cmd":     "meta",
	"command": "meta",
	"hyper":   "hyper",
	"super":   "super",
	"win":     "super",
}

// keyAliases maps accepted base-key spellings to the name the terminal layer
// emits.
var keyAliases = map[string]string{
	"escape":   "esc",
	"return":   "enter",
	"del":      "delete",
	"ins":      "insert",
	"pageup":   "pgup",
	"page_up":  "pgup",
	"pagedn":   "pgdown",
	"pagedown": "pgdown",
	"pgdn":     "pgdown",
	"spacebar": "space",
	" ":        "space",
}

// reservedShortcutKeys can never reach an extension: Kit consumes them and
// returns before the shortcut dispatcher runs. Registering one is an error
// rather than a warning, because the handler would silently never fire.
var reservedShortcutKeys = map[string]string{
	"ctrl+c": "Kit handles Ctrl+C for cancel/quit before extensions are consulted",
}

// builtinShortcutKeys are bound by Kit's own TUI. An extension *can* claim
// them — the shortcut dispatcher runs first — but doing so shadows built-in
// behaviour, so registration emits a warning naming what was overridden.
var builtinShortcutKeys = map[string]string{
	"esc":       "cancel the running turn",
	"ctrl+x":    "leader-chord prefix (Ctrl+X s/t/m/e)",
	"pgup":      "scroll the transcript up",
	"pgdown":    "scroll the transcript down",
	"ctrl+home": "jump to the start of the transcript",
	"ctrl+end":  "jump to the end of the transcript",
	"shift+tab": "cycle thinking level on reasoning models",
	"enter":     "submit the composer",
	"tab":       "accept the completion popup",
	"up":        "history / popup navigation",
	"down":      "history / popup navigation",
}

// splitShortcutKey splits on "+" while preserving a literal "+" base key, so
// "ctrl++" yields ["ctrl", "+"] rather than ["ctrl", "", ""].
func splitShortcutKey(k string) []string {
	parts := strings.Split(k, "+")
	if n := len(parts); n >= 2 && parts[n-1] == "" && parts[n-2] == "" {
		parts = append(parts[:n-2], "+")
	}
	return parts
}

// canonicalBaseKey normalizes the non-modifier portion of a binding.
//
// When the binding carries modifiers the terminal always renders the base in
// lower case ("shift+a"), so we can fold case freely. Without modifiers the
// binding is compared against the key's *text*, where case is meaningful:
// "A" and "a" are different key presses. Multi-rune names ("Enter", "F1") are
// always key names rather than text, so those still fold.
func canonicalBaseKey(s string, hasModifiers bool) string {
	lower := strings.ToLower(s)
	if alias, ok := keyAliases[lower]; ok {
		return alias
	}
	if hasModifiers {
		return lower
	}
	if len([]rune(s)) > 1 {
		return lower
	}
	return s
}

// NormalizeShortcutKey converts a shortcut binding into the canonical spelling
// Kit matches against. Modifier aliases are folded, modifiers are reordered
// into canonical order, duplicates are dropped, and known key names are
// normalized. Bindings containing an unrecognized modifier are returned
// trimmed but otherwise untouched, so an unusual-but-valid terminal key name
// still has a chance to match.
func NormalizeShortcutKey(key string) string {
	// A lone space is the space bar, not blank input, so it must be resolved
	// before trimming would erase it.
	if key == " " {
		return "space"
	}

	k := strings.TrimSpace(key)
	if k == "" || k == "+" {
		return k
	}

	parts := splitShortcutKey(k)
	if len(parts) == 1 {
		return canonicalBaseKey(parts[0], false)
	}

	seen := make(map[string]bool, len(parts))
	for _, raw := range parts[:len(parts)-1] {
		mod, ok := modifierAliases[strings.ToLower(strings.TrimSpace(raw))]
		if !ok {
			// Unknown modifier — normalizing further risks mangling a key
			// name we simply don't know about. Leave it alone.
			return k
		}
		seen[mod] = true
	}

	out := make([]string, 0, len(seen)+1)
	for _, mod := range modifierOrder {
		if seen[mod] {
			out = append(out, mod)
		}
	}
	out = append(out, canonicalBaseKey(parts[len(parts)-1], true))
	return strings.Join(out, "+")
}

// ValidateShortcutKey normalizes a binding and reports whether Kit can deliver
// it. A non-nil error means the shortcut must be rejected: the handler could
// never fire. A non-empty warning means the shortcut will fire but shadows
// built-in behaviour.
func ValidateShortcutKey(key string) (normalized string, warning string, err error) {
	normalized = NormalizeShortcutKey(key)
	if normalized == "" {
		return "", "", fmt.Errorf("shortcut key is empty")
	}
	if reason, bad := reservedShortcutKeys[normalized]; bad {
		return normalized, "", fmt.Errorf("%q is reserved: %s", normalized, reason)
	}
	if reason, clash := builtinShortcutKeys[normalized]; clash {
		warning = fmt.Sprintf("shortcut %q shadows Kit's built-in binding (%s)", normalized, reason)
	}
	return normalized, warning, nil
}

// prepareShortcut validates and normalizes a shortcut registration. It reports
// false when the shortcut must be dropped. Diagnostics are attributed to the
// extension's file name so an author can tell which file to fix.
func prepareShortcut(def ShortcutDef, handler func(Context), path string) (ShortcutEntry, bool) {
	source := filepath.Base(path)

	if handler == nil {
		log.Warn("Extension shortcut has no handler", "extension", source, "key", def.Key)
		return ShortcutEntry{}, false
	}

	normalized, warning, err := ValidateShortcutKey(def.Key)
	if err != nil {
		log.Warn("Ignoring extension shortcut", "extension", source, "key", def.Key, "error", err)
		return ShortcutEntry{}, false
	}
	if warning != "" {
		log.Warn(warning, "extension", source)
	}
	if normalized != def.Key {
		log.Debug("Normalized extension shortcut", "extension", source, "from", def.Key, "to", normalized)
	}

	def.Key = normalized
	return ShortcutEntry{Def: def, Handler: handler, Source: source}, true
}

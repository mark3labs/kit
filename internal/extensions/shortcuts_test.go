package extensions

import (
	"sync"
	"testing"
)

func TestNormalizeShortcutKey(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		// Modifier ordering is canonicalized to match the terminal's own
		// Keystroke() rendering, so authors can write them in any order.
		{"reorders modifiers", "shift+ctrl+s", "ctrl+shift+s"},
		{"already canonical", "ctrl+shift+s", "ctrl+shift+s"},
		{"folds modifier case", "Ctrl+Shift+S", "ctrl+shift+s"},
		{"control alias", "control+p", "ctrl+p"},
		{"option alias", "option+t", "alt+t"},
		{"cmd alias", "cmd+k", "meta+k"},
		{"dedupes modifiers", "ctrl+ctrl+a", "ctrl+a"},
		{"trims surrounding space", "  ctrl+p  ", "ctrl+p"},

		// Base keys.
		{"named key folds case", "F1", "f1"},
		{"escape alias", "escape", "esc"},
		{"return alias", "Return", "enter"},
		{"pgdn alias", "pgdn", "pgdown"},
		{"page up alias", "PageUp", "pgup"},
		{"literal space", " ", "space"},

		// A bare single character is compared against the key's *text*, where
		// case is meaningful: "A" and "a" are genuinely different presses.
		{"preserves bare uppercase", "A", "A"},
		{"preserves bare lowercase", "a", "a"},
		{"preserves punctuation", "?", "?"},

		// With modifiers the terminal always lowercases the base.
		{"lowercases modified base", "shift+A", "shift+a"},

		// Literal plus must survive the split.
		{"bare plus", "+", "+"},
		{"modified plus", "ctrl++", "ctrl++"},

		// An unknown modifier is left alone rather than mangled.
		{"unknown modifier untouched", "hyper9+x", "hyper9+x"},

		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NormalizeShortcutKey(tt.in); got != tt.want {
				t.Errorf("NormalizeShortcutKey(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestNormalizeShortcutKeyIsIdempotent(t *testing.T) {
	inputs := []string{"shift+ctrl+s", "Ctrl+P", "escape", "A", "ctrl++", "f1", "+"}
	for _, in := range inputs {
		once := NormalizeShortcutKey(in)
		twice := NormalizeShortcutKey(once)
		if once != twice {
			t.Errorf("not idempotent for %q: %q then %q", in, once, twice)
		}
	}
}

func TestValidateShortcutKeyRejectsReserved(t *testing.T) {
	// Ctrl+C is consumed by Kit before the shortcut dispatcher runs, so a
	// handler bound to it could never fire. Registration must fail loudly
	// rather than silently doing nothing.
	for _, key := range []string{"ctrl+c", "Ctrl+C", "control+c"} {
		if _, _, err := ValidateShortcutKey(key); err == nil {
			t.Errorf("ValidateShortcutKey(%q) succeeded, want reserved-key error", key)
		}
	}
}

func TestValidateShortcutKeyRejectsEmpty(t *testing.T) {
	if _, _, err := ValidateShortcutKey("   "); err == nil {
		t.Error("ValidateShortcutKey(blank) succeeded, want error")
	}
}

func TestValidateShortcutKeyWarnsOnBuiltinShadow(t *testing.T) {
	// These are bindable — the dispatcher runs first — but shadowing a
	// built-in silently is exactly the failure mode we want surfaced.
	for _, key := range []string{"esc", "ctrl+x", "pgup", "enter"} {
		normalized, warning, err := ValidateShortcutKey(key)
		if err != nil {
			t.Fatalf("ValidateShortcutKey(%q) errored: %v", key, err)
		}
		if warning == "" {
			t.Errorf("ValidateShortcutKey(%q) gave no warning, want builtin-shadow warning", key)
		}
		if normalized != key {
			t.Errorf("ValidateShortcutKey(%q) normalized to %q", key, normalized)
		}
	}
}

func TestValidateShortcutKeyQuietForOrdinaryBindings(t *testing.T) {
	for _, key := range []string{"ctrl+p", "alt+t", "f5", "ctrl+shift+k"} {
		_, warning, err := ValidateShortcutKey(key)
		if err != nil {
			t.Errorf("ValidateShortcutKey(%q) errored: %v", key, err)
		}
		if warning != "" {
			t.Errorf("ValidateShortcutKey(%q) warned unexpectedly: %s", key, warning)
		}
	}
}

func TestPrepareShortcutNormalizesAndAttributes(t *testing.T) {
	entry, ok := prepareShortcut(
		ShortcutDef{Key: "Shift+Ctrl+S", Description: "save"},
		func(Context) {},
		"/home/user/.config/kit/extensions/saver.go",
	)
	if !ok {
		t.Fatal("prepareShortcut rejected a valid shortcut")
	}
	if entry.Def.Key != "ctrl+shift+s" {
		t.Errorf("key = %q, want %q", entry.Def.Key, "ctrl+shift+s")
	}
	if entry.Source != "saver.go" {
		t.Errorf("source = %q, want %q", entry.Source, "saver.go")
	}
}

func TestPrepareShortcutDropsUnusable(t *testing.T) {
	if _, ok := prepareShortcut(ShortcutDef{Key: "ctrl+c"}, func(Context) {}, "x.go"); ok {
		t.Error("prepareShortcut accepted the reserved ctrl+c binding")
	}
	if _, ok := prepareShortcut(ShortcutDef{Key: "ctrl+p"}, nil, "x.go"); ok {
		t.Error("prepareShortcut accepted a shortcut with a nil handler")
	}
	if _, ok := prepareShortcut(ShortcutDef{Key: ""}, func(Context) {}, "x.go"); ok {
		t.Error("prepareShortcut accepted an empty key")
	}
}

func TestRunnerShortcutCacheIsStable(t *testing.T) {
	entry, ok := prepareShortcut(ShortcutDef{Key: "ctrl+p"}, func(Context) {}, "a.go")
	if !ok {
		t.Fatal("prepareShortcut rejected a valid shortcut")
	}
	r := NewRunner([]LoadedExtension{{Path: "a.go", Shortcuts: []ShortcutEntry{entry}}})

	// The TUI calls this on every key press, so repeated calls must hand back
	// the same map rather than rebuilding it. Comparing map identity via a
	// sentinel is the only way to observe that from outside the package.
	first := r.GetShortcutHandlers()
	if len(first) != 1 {
		t.Fatalf("got %d handlers, want 1", len(first))
	}
	first["__sentinel__"] = func() {}

	second := r.GetShortcutHandlers()
	if _, cached := second["__sentinel__"]; !cached {
		t.Error("GetShortcutHandlers rebuilt the map instead of serving the cache")
	}

	entries := r.GetShortcuts()
	if _, present := entries["ctrl+p"]; !present {
		t.Error(`cached entry map lost the "ctrl+p" binding`)
	}
}

func TestRunnerShortcutsConcurrentAccess(t *testing.T) {
	entry, _ := prepareShortcut(ShortcutDef{Key: "ctrl+p"}, func(Context) {}, "a.go")
	r := NewRunner([]LoadedExtension{{Path: "a.go", Shortcuts: []ShortcutEntry{entry}}})

	// Hot-reload used to race against the per-key-press lookup, which read the
	// extension slice with no lock. Run under -race to catch a regression.
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			for range 200 {
				_ = r.GetShortcutHandlers()
				_ = r.RegisteredShortcuts()
			}
		})
	}
	wg.Go(func() {
		for range 50 {
			swap, _ := prepareShortcut(ShortcutDef{Key: "alt+t"}, func(Context) {}, "b.go")
			r.Reload([]LoadedExtension{{Path: "b.go", Shortcuts: []ShortcutEntry{swap}}})
		}
	})
	wg.Wait()
}

func TestRunnerShortcutCacheInvalidatedOnReload(t *testing.T) {
	first, _ := prepareShortcut(ShortcutDef{Key: "ctrl+p"}, func(Context) {}, "a.go")
	r := NewRunner([]LoadedExtension{{Path: "a.go", Shortcuts: []ShortcutEntry{first}}})
	if _, present := r.GetShortcutHandlers()["ctrl+p"]; !present {
		t.Fatal("initial binding missing")
	}

	second, _ := prepareShortcut(ShortcutDef{Key: "alt+t"}, func(Context) {}, "b.go")
	r.Reload([]LoadedExtension{{Path: "b.go", Shortcuts: []ShortcutEntry{second}}})

	handlers := r.GetShortcutHandlers()
	if _, present := handlers["ctrl+p"]; present {
		t.Error("stale binding survived reload")
	}
	if _, present := handlers["alt+t"]; !present {
		t.Error("new binding missing after reload")
	}
}

func TestRunnerRegisteredShortcutsSorted(t *testing.T) {
	b, _ := prepareShortcut(ShortcutDef{Key: "alt+t", Description: "t"}, func(Context) {}, "b.go")
	a2, _ := prepareShortcut(ShortcutDef{Key: "ctrl+z", Description: "z"}, func(Context) {}, "a.go")
	a1, _ := prepareShortcut(ShortcutDef{Key: "ctrl+a", Description: "a"}, func(Context) {}, "a.go")
	r := NewRunner([]LoadedExtension{
		{Path: "b.go", Shortcuts: []ShortcutEntry{b}},
		{Path: "a.go", Shortcuts: []ShortcutEntry{a2, a1}},
	})

	got := r.RegisteredShortcuts()
	want := []string{"a.go/ctrl+a", "a.go/ctrl+z", "b.go/alt+t"}
	if len(got) != len(want) {
		t.Fatalf("got %d entries, want %d", len(got), len(want))
	}
	for i, entry := range got {
		if key := entry.Source + "/" + entry.Def.Key; key != want[i] {
			t.Errorf("entry %d = %s, want %s", i, key, want[i])
		}
	}
}

func TestRunnerNoShortcutsReturnsNil(t *testing.T) {
	r := NewRunner([]LoadedExtension{{Path: "a.go"}})
	if r.GetShortcuts() != nil {
		t.Error("GetShortcuts should be nil when nothing is registered")
	}
	if r.GetShortcutHandlers() != nil {
		t.Error("GetShortcutHandlers should be nil when nothing is registered")
	}
}

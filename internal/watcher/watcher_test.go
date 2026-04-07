package watcher

import (
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestContentWatcher_ReloadsOnMatchingFile(t *testing.T) {
	dir := t.TempDir()

	// Write an initial file so the directory isn't empty.
	initial := filepath.Join(dir, "existing.md")
	if err := os.WriteFile(initial, []byte("# Hello"), 0644); err != nil {
		t.Fatal(err)
	}

	var reloadCount atomic.Int32
	w, err := New(Options{
		Dirs:       []string{dir},
		Extensions: []string{".md"},
		OnReload:   func() { reloadCount.Add(1) },
		Label:      "test",
		Debounce:   50 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}

	go w.Start(t.Context())

	// Wait for watcher to be ready.
	time.Sleep(100 * time.Millisecond)

	// Modify the file.
	if err := os.WriteFile(initial, []byte("# Updated"), 0644); err != nil {
		t.Fatal(err)
	}

	// Wait for debounce + processing.
	time.Sleep(200 * time.Millisecond)

	if got := reloadCount.Load(); got != 1 {
		t.Errorf("expected 1 reload, got %d", got)
	}

	_ = w.Close()
}

func TestContentWatcher_IgnoresNonMatchingFiles(t *testing.T) {
	dir := t.TempDir()

	var reloadCount atomic.Int32
	w, err := New(Options{
		Dirs:       []string{dir},
		Extensions: []string{".md"},
		OnReload:   func() { reloadCount.Add(1) },
		Label:      "test",
		Debounce:   50 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}

	go w.Start(t.Context())

	time.Sleep(100 * time.Millisecond)

	// Write a non-matching file.
	if err := os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}

	time.Sleep(200 * time.Millisecond)

	if got := reloadCount.Load(); got != 0 {
		t.Errorf("expected 0 reloads for non-matching file, got %d", got)
	}

	_ = w.Close()
}

func TestContentWatcher_MultipleExtensions(t *testing.T) {
	dir := t.TempDir()

	var reloadCount atomic.Int32
	w, err := New(Options{
		Dirs:       []string{dir},
		Extensions: []string{".md", ".txt"},
		OnReload:   func() { reloadCount.Add(1) },
		Label:      "test",
		Debounce:   50 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}

	go w.Start(t.Context())

	time.Sleep(100 * time.Millisecond)

	// Write a .txt file — should trigger.
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("notes"), 0644); err != nil {
		t.Fatal(err)
	}

	time.Sleep(200 * time.Millisecond)

	if got := reloadCount.Load(); got != 1 {
		t.Errorf("expected 1 reload for .txt file, got %d", got)
	}

	_ = w.Close()
}

func TestContentWatcher_Debounces(t *testing.T) {
	dir := t.TempDir()

	var reloadCount atomic.Int32
	w, err := New(Options{
		Dirs:       []string{dir},
		Extensions: []string{".md"},
		OnReload:   func() { reloadCount.Add(1) },
		Label:      "test",
		Debounce:   100 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}

	go w.Start(t.Context())

	time.Sleep(100 * time.Millisecond)

	// Rapid-fire writes — should debounce into 1 reload.
	for i := range 5 {
		if err := os.WriteFile(filepath.Join(dir, "test.md"), []byte("v"+string(rune('0'+i))), 0644); err != nil {
			t.Fatal(err)
		}
		time.Sleep(30 * time.Millisecond)
	}

	time.Sleep(300 * time.Millisecond)

	if got := reloadCount.Load(); got != 1 {
		t.Errorf("expected 1 debounced reload, got %d", got)
	}

	_ = w.Close()
}

func TestContentWatcher_WatchesSubdirectories(t *testing.T) {
	dir := t.TempDir()

	// Create a subdirectory (simulates skill-name/SKILL.md pattern).
	subdir := filepath.Join(dir, "my-skill")
	if err := os.MkdirAll(subdir, 0755); err != nil {
		t.Fatal(err)
	}

	var reloadCount atomic.Int32
	w, err := New(Options{
		Dirs:       []string{dir},
		Extensions: []string{".md"},
		OnReload:   func() { reloadCount.Add(1) },
		Label:      "test",
		Debounce:   50 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}

	go w.Start(t.Context())

	time.Sleep(100 * time.Millisecond)

	// Write to subdirectory.
	if err := os.WriteFile(filepath.Join(subdir, "SKILL.md"), []byte("# Skill"), 0644); err != nil {
		t.Fatal(err)
	}

	time.Sleep(200 * time.Millisecond)

	if got := reloadCount.Load(); got != 1 {
		t.Errorf("expected 1 reload for subdirectory file, got %d", got)
	}

	_ = w.Close()
}

func TestContentWatcher_WatchesNewSubdirectory(t *testing.T) {
	dir := t.TempDir()

	var reloadCount atomic.Int32
	w, err := New(Options{
		Dirs:       []string{dir},
		Extensions: []string{".md"},
		OnReload:   func() { reloadCount.Add(1) },
		Label:      "test",
		Debounce:   50 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}

	go w.Start(t.Context())

	// Wait for watcher to be ready.
	time.Sleep(100 * time.Millisecond)

	// Create a NEW subdirectory after the watcher started (the bug scenario).
	subdir := filepath.Join(dir, "new-skill")
	if err := os.MkdirAll(subdir, 0755); err != nil {
		t.Fatal(err)
	}

	// Give fsnotify time to pick up the new directory.
	time.Sleep(100 * time.Millisecond)

	// Write a matching file inside the new subdirectory.
	if err := os.WriteFile(filepath.Join(subdir, "SKILL.md"), []byte("# New Skill"), 0644); err != nil {
		t.Fatal(err)
	}

	// Wait for debounce + processing.
	time.Sleep(200 * time.Millisecond)

	if got := reloadCount.Load(); got < 1 {
		t.Errorf("expected at least 1 reload for file in new subdirectory, got %d", got)
	}

	_ = w.Close()
}

func TestContentWatcher_WatchesNewSubdirectoryWithExistingFiles(t *testing.T) {
	dir := t.TempDir()

	var reloadCount atomic.Int32
	w, err := New(Options{
		Dirs:       []string{dir},
		Extensions: []string{".md"},
		OnReload:   func() { reloadCount.Add(1) },
		Label:      "test",
		Debounce:   50 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}

	go w.Start(t.Context())

	time.Sleep(100 * time.Millisecond)

	// Create a subdirectory with a matching file already inside (simulates cp -r).
	subdir := filepath.Join(dir, "copied-skill")
	if err := os.MkdirAll(subdir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subdir, "SKILL.md"), []byte("# Copied"), 0644); err != nil {
		t.Fatal(err)
	}

	// Wait for debounce + processing.
	time.Sleep(300 * time.Millisecond)

	if got := reloadCount.Load(); got < 1 {
		t.Errorf("expected at least 1 reload for copied subdirectory with files, got %d", got)
	}

	_ = w.Close()
}

func TestCollectDirs_Deduplicates(t *testing.T) {
	dir := t.TempDir()

	dirs := CollectDirs([]string{dir, dir}, nil)
	if len(dirs) != 1 {
		t.Errorf("expected 1 deduplicated dir, got %d", len(dirs))
	}
}

func TestCollectDirs_FileParent(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test.md")
	if err := os.WriteFile(file, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	dirs := CollectDirs(nil, []string{file})
	if len(dirs) != 1 {
		t.Fatalf("expected 1 dir, got %d", len(dirs))
	}

	abs, _ := filepath.Abs(dir)
	if dirs[0] != abs {
		t.Errorf("expected %s, got %s", abs, dirs[0])
	}
}

func TestCollectDirs_SkipsNonexistent(t *testing.T) {
	dirs := CollectDirs([]string{"/nonexistent/dir"}, nil)
	if len(dirs) != 0 {
		t.Errorf("expected 0 dirs for nonexistent path, got %d", len(dirs))
	}
}

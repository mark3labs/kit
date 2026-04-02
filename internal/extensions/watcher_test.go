package extensions

import (
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestWatcher_ReloadsOnGoFileChange(t *testing.T) {
	dir := t.TempDir()

	// Write an initial extension file.
	extFile := filepath.Join(dir, "test.go")
	if err := os.WriteFile(extFile, []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var reloadCount atomic.Int32

	w, err := NewWatcher([]string{dir}, func() {
		reloadCount.Add(1)
	})
	if err != nil {
		t.Fatal(err)
	}

	go w.Start(t.Context())

	// Modify the file.
	time.Sleep(50 * time.Millisecond) // let watcher settle
	if err := os.WriteFile(extFile, []byte("package main\n// changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Wait for debounce (300ms) + margin.
	time.Sleep(600 * time.Millisecond)

	if got := reloadCount.Load(); got != 1 {
		t.Errorf("expected 1 reload, got %d", got)
	}

	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestWatcher_IgnoresNonGoFiles(t *testing.T) {
	dir := t.TempDir()

	var reloadCount atomic.Int32

	w, err := NewWatcher([]string{dir}, func() {
		reloadCount.Add(1)
	})
	if err != nil {
		t.Fatal(err)
	}

	go w.Start(t.Context())

	// Write a non-.go file.
	time.Sleep(50 * time.Millisecond)
	txtFile := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(txtFile, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Wait past the debounce window.
	time.Sleep(600 * time.Millisecond)

	if got := reloadCount.Load(); got != 0 {
		t.Errorf("expected 0 reloads for .txt file, got %d", got)
	}

	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestWatcher_Debounces(t *testing.T) {
	dir := t.TempDir()

	extFile := filepath.Join(dir, "ext.go")
	if err := os.WriteFile(extFile, []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var reloadCount atomic.Int32

	w, err := NewWatcher([]string{dir}, func() {
		reloadCount.Add(1)
	})
	if err != nil {
		t.Fatal(err)
	}

	go w.Start(t.Context())

	time.Sleep(50 * time.Millisecond)

	// Rapid-fire writes (simulating editor save: write temp, rename, etc.).
	for range 5 {
		if err := os.WriteFile(extFile, []byte("package main\n// changed\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		time.Sleep(50 * time.Millisecond)
	}

	// Wait for debounce to fire.
	time.Sleep(600 * time.Millisecond)

	if got := reloadCount.Load(); got != 1 {
		t.Errorf("expected 1 debounced reload, got %d", got)
	}

	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestWatchedDirs_Deduplicates(t *testing.T) {
	dir := t.TempDir()
	dirs := WatchedDirs([]string{dir, dir})

	count := 0
	for _, d := range dirs {
		abs, _ := filepath.Abs(dir)
		if d == abs {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected directory to appear once, got %d", count)
	}
}

func TestWatchedDirs_FileParent(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "ext.go")
	if err := os.WriteFile(file, []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	dirs := WatchedDirs([]string{file})

	abs, _ := filepath.Abs(dir)
	found := false
	for _, d := range dirs {
		if d == abs {
			found = true
		}
	}
	if !found {
		t.Errorf("expected parent dir %s in watched dirs %v", abs, dirs)
	}
}

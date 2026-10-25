package models

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
)

type failingCacheFile struct {
	*os.File
	stage string
	err   error
}

func (f failingCacheFile) Write(data []byte) (int, error) {
	if f.stage == "write" {
		_, _ = f.File.Write(data[:len(data)/2])
		return len(data) / 2, f.err
	}
	return f.File.Write(data)
}
func (f failingCacheFile) Sync() error {
	if f.stage == "sync" {
		return f.err
	}
	return f.File.Sync()
}
func (f failingCacheFile) Close() error {
	err := f.File.Close()
	if f.stage == "close" {
		return f.err
	}
	return err
}
func (f failingCacheFile) Chmod(mode os.FileMode) error {
	if f.stage == "chmod" {
		return f.err
	}
	return f.File.Chmod(mode)
}

func TestCacheRenameRetry(t *testing.T) {
	failure := errors.New("sharing violation")
	for _, tc := range []struct {
		name          string
		failures      int
		retry, cancel bool
		wantCalls     int
	}{
		{"transient", 2, true, false, 3},
		{"bounded", 100, true, false, 20},
		{"permanent", 1, false, false, 1},
		{"cancelled", 1, true, true, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			calls := 0
			err := renameCache(ctx, "old", "new", func(from, to string) error {
				calls++
				if tc.cancel {
					cancel()
				}
				if calls <= tc.failures {
					return failure
				}
				return nil
			}, func(err error) bool { return tc.retry && errors.Is(err, failure) })
			if calls != tc.wantCalls {
				t.Fatalf("calls = %d", calls)
			}
			switch {
			case tc.cancel:
				if !errors.Is(err, context.Canceled) {
					t.Fatal(err)
				}
			case tc.name == "transient":
				if err != nil {
					t.Fatal(err)
				}
			default:
				if !errors.Is(err, failure) {
					t.Fatal(err)
				}
			}
		})
	}
}

func TestAtomicCacheFailures(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	for _, stage := range []string{"create", "write", "chmod", "sync", "close", "rename", "cancel"} {
		t.Run(stage, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), cacheFile)
			before := []byte(`{"old":true}`)
			if err := os.WriteFile(path, before, 0o644); err != nil {
				t.Fatal(err)
			}
			failure := errors.New("injected " + stage)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			ops := cacheFileOps{
				create: func(dir string) (cacheTempFile, error) {
					if stage == "create" {
						return nil, failure
					}
					f, err := os.CreateTemp(dir, ".providers-*")
					if err != nil {
						return nil, err
					}
					if stage == "cancel" {
						cancel()
					}
					return failingCacheFile{f, stage, failure}, nil
				},
				rename: func(from, to string) error {
					if stage == "rename" {
						return failure
					}
					return os.Rename(from, to)
				},
			}
			err := replaceCache(ctx, path, []byte(`{"new":true}`), ops)
			if stage == "cancel" {
				failure = context.Canceled
			}
			if !errors.Is(err, failure) {
				t.Fatalf("error = %v", err)
			}
			got, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(got, before) {
				t.Fatalf("old cache changed: %s %v", got, err)
			}
			entries, err := os.ReadDir(filepath.Dir(path))
			if err != nil || len(entries) != 1 {
				t.Fatalf("temporary file leaked: %v %v", entries, err)
			}
		})
	}
}

func TestAtomicCacheReaders(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	providers, err := decodeModels([]byte(updateFixture))
	if err != nil {
		t.Fatal(err)
	}
	if err := StoreCachedProviders(providers, "old"); err != nil {
		t.Fatal(err)
	}
	path, err := cachePath()
	if err != nil {
		t.Fatal(err)
	}
	old := cacheBytes(t)
	if err := StoreCachedProviders(providers, "new"); err != nil {
		t.Fatal(err)
	}
	newData := cacheBytes(t)
	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Go(func() {
		for {
			select {
			case <-stop:
				return
			default:
			}
			data, err := os.ReadFile(path)
			if err != nil || !json.Valid(data) || (!bytes.Equal(data, old) && !bytes.Equal(data, newData)) {
				t.Errorf("partial snapshot: %s %v", data, err)
				return
			}
		}
	})
	for i := range 50 {
		etag := "old"
		if i%2 == 0 {
			etag = "new"
		}
		if err := StoreCachedProviders(providers, etag); err != nil {
			t.Error(err)
			break
		}
	}
	close(stop)
	wg.Wait()
	info, err := os.Stat(path)
	// Windows exposes only the writable bit through Chmod, not Unix permissions.
	if err != nil || (runtime.GOOS != "windows" && info.Mode().Perm() != 0o644) {
		t.Fatalf("permissions: %v %v", info, err)
	}
}

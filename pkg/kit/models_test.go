package kit_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/mark3labs/kit/pkg/kit"
)

func TestStandaloneUpdateModels(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	const catalog = `{"standalone":{"models":{"chat":{},"decision":{"type":"decision"}}}}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("If-None-Match") == `"sdk"` {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", `"sdk"`)
		_, _ = w.Write([]byte(catalog))
	}))
	defer server.Close()
	t.Setenv("KIT_MODELS_URL", server.URL)
	file := filepath.Join(t.TempDir(), "models.json")
	if err := os.WriteFile(file, []byte(catalog), 0o600); err != nil {
		t.Fatal(err)
	}
	// Capture both process streams: the standalone API must be silent.
	stdout, stderr := os.Stdout, os.Stderr
	capture, err := os.CreateTemp(t.TempDir(), "output")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { os.Stdout, os.Stderr = stdout, stderr; _ = capture.Close() }()
	os.Stdout, os.Stderr = capture, capture
	for _, tc := range []struct {
		source            string
		status            kit.UpdateModelsStatus
		providers, models int
	}{
		{"", kit.UpdateModelsStatusUpdated, 1, 2},
		{"", kit.UpdateModelsStatusNotModified, 1, 2},
		{file, kit.UpdateModelsStatusUpdated, 1, 2},
		{"embedded", kit.UpdateModelsStatusReset, 0, 0},
	} {
		got, err := kit.UpdateModels(context.Background(), kit.UpdateModelsOptions{Source: tc.source})
		if err != nil || got.Status != tc.status || got.ProviderCount != tc.providers || got.ModelCount != tc.models {
			t.Fatalf("result=%+v err=%v", got, err)
		}
	}
	if _, err := capture.Seek(0, 0); err != nil {
		t.Fatal(err)
	}
	output, err := io.ReadAll(capture)
	if err != nil || len(output) != 0 {
		t.Fatalf("API output=%q err=%v", output, err)
	}
}

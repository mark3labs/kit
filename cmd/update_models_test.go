package cmd

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"net/http/httptest"
	"testing"

	"github.com/mark3labs/kit/internal/models"
)

func captureUpdateOutput(t *testing.T, run func() error) (string, string, error) {
	t.Helper()
	out, err := os.CreateTemp(t.TempDir(), "stdout")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = out.Close() }()
	errOut, err := os.CreateTemp(t.TempDir(), "stderr")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = errOut.Close() }()
	stdout, stderr := os.Stdout, os.Stderr
	defer func() { os.Stdout, os.Stderr = stdout, stderr }()
	os.Stdout, os.Stderr = out, errOut
	runErr := run()
	if _, err := out.Seek(0, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := errOut.Seek(0, 0); err != nil {
		t.Fatal(err)
	}
	a, err := io.ReadAll(out)
	if err != nil {
		t.Fatal(err)
	}
	b, err := io.ReadAll(errOut)
	if err != nil {
		t.Fatal(err)
	}
	return string(a), string(b), runErr
}

func TestUpdateModelsCommand(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	file := filepath.Join(t.TempDir(), "models.json")
	if err := os.WriteFile(file, []byte(`{"p":{"models":{"m":{}}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("If-None-Match") != "" {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", `"cmd"`)
		_, _ = w.Write([]byte(`{"p":{"models":{"m":{}}}}`))
	}))
	defer server.Close()
	t.Setenv("KIT_MODELS_URL", server.URL)
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	for _, tc := range []struct {
		args   []string
		output string
	}{
		{nil, "Fetching models from " + server.URL + "...\nModel database updated: 1 providers, 1 models.\n"},
		{[]string{server.URL}, "Fetching models from " + server.URL + "...\nModel database is already up to date.\n"},
		{[]string{file}, "Model database updated from file: 1 providers, 1 models.\n"},
		{[]string{"embedded"}, "Model database reset to embedded version.\n"},
	} {
		stdout, stderr, err := captureUpdateOutput(t, func() error { return runUpdateModels(cmd, tc.args) })
		if err != nil || stdout != "" || stderr != tc.output {
			t.Fatalf("stdout=%q stderr=%q err=%v", stdout, stderr, err)
		}
	}
	if err := updateModelsCmd.ValidateArgs([]string{"a", "b"}); err == nil {
		t.Fatal("accepted two arguments")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cmd.SetContext(ctx)
	_, stderr, err := captureUpdateOutput(t, func() error { return runUpdateModels(cmd, []string{file}) })
	if !errors.Is(err, context.Canceled) || strings.Contains(stderr, "updated") {
		t.Fatalf("cancellation: %q %v", stderr, err)
	}
}

func TestCatalogDownloadUserAgentAndETag(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("User-Agent") != "KIT" {
			t.Errorf("identity = %q", r.Header.Get("User-Agent"))
		}
		if calls == 1 {
			w.Header().Set("ETag", `"fixture"`)
			_, _ = w.Write([]byte(`{"test":{"id":"test","name":"Test","models":{"m":{}}}}`))
			return
		}
		if r.Header.Get("If-None-Match") != `"fixture"` {
			t.Errorf("ETag = %q", r.Header.Get("If-None-Match"))
		}
		w.WriteHeader(http.StatusNotModified)
	}))
	defer srv.Close()
	for range 2 {
		cmd := &cobra.Command{}
		cmd.SetContext(context.Background())
		if err := runUpdateModels(cmd, []string{srv.URL}); err != nil {
			t.Fatal(err)
		}
	}
	_, etag := models.LoadCachedProviders()
	if etag != `"fixture"` {
		t.Errorf("stored ETag = %q", etag)
	}
}

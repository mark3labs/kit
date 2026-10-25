package models

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

const updateFixture = `{"fixture-update":{"models":{"m":{"id":"m","name":"Updated","type":"decision","reasoning":true,"reasoning_options":[{"type":"effort","values":["high"]}],"cost":{"input":1,"output":2,"tiers":[{"input":3,"tier":{"type":"context","size":200}}]},"modalities":{"input":["text"],"output":["text"]},"experimental":{"modes":{"fast":{"provider":{"body":{"fast":true}},"cost":{"input":0.5}}}}}}},"empty":{"models":{}}}`

func cacheBytes(t *testing.T) []byte {
	t.Helper()
	path, err := cachePath()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func mustUpdate(t *testing.T, source string, status UpdateModelsStatus) UpdateModelsResult {
	t.Helper()
	result, err := UpdateModels(context.Background(), UpdateModelsOptions{Source: source})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != status {
		t.Fatalf("result = %+v", result)
	}
	return result
}

func TestResolveModelsUpdateSource(t *testing.T) {
	for _, tc := range []struct {
		source, env, location   string
		network, reset, invalid bool
	}{
		{"", "", defaultModelsURL, true, false, false},
		{"", "https://example.test/db?q=1", "https://example.test/db?q=1", true, false, false},
		{"http://example.test/x", "bad", "http://example.test/x", true, false, false},
		{"db.json", "bad", "db.json", false, false, false},
		{"embedded", "bad", "", false, true, false},
		{"", "embedded", "", false, false, true},
		{"", "/tmp/db.json", "", false, false, true},
		{"", "ftp://example.test", "", false, false, true},
		{"https://", "", "", false, false, true},
		{"http://%", "", "", false, false, true},
	} {
		t.Run(fmt.Sprintf("%s/%s", tc.source, tc.env), func(t *testing.T) {
			t.Setenv("KIT_MODELS_URL", tc.env)
			got, err := ResolveModelsUpdateSource(tc.source)
			if (err != nil) != tc.invalid {
				t.Fatalf("source=%+v err=%v", got, err)
			}
			if err == nil && (got.Location != tc.location || got.HTTP != tc.network || got.Reset != tc.reset) {
				t.Fatalf("source=%+v", got)
			}
		})
	}
}

func TestDecodeModels(t *testing.T) {
	for _, data := range []string{
		``, `null`, `{}`, `[]`, `1`, `{"p":null}`, `{"p":{}}`, `{"p":{"models":null}}`,
		`{"p":{"models":{}}}`, `{" ":{"models":{"m":{}}}}`, `{"p":{"models":{" ":{}}}}`,
		`{"p":{"models":{"m":null}}}`, `{"p":{"models":[]}}`, `{"p":{"models":{"m":1}}}`,
		`{"providers":{"p":{"models":{"m":{}}}}}`, `{"version":4,"providers":{}}`,
		`{"version":3}`, updateFixture + ` {}`, `{"p":{"models":{"m":{"cost":"bad"}}}}`,
	} {
		t.Run(data, func(t *testing.T) {
			if _, err := decodeModels([]byte(data)); err == nil {
				t.Fatal("accepted invalid catalog")
			}
		})
	}
	providers, err := decodeModels([]byte(updateFixture))
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(cacheEnvelope{Version: cacheSchemaVersion, Providers: providers, ETag: "untrusted", SourceFingerprint: "untrusted"})
	if err != nil {
		t.Fatal(err)
	}
	got, err := decodeModels(data)
	if err != nil || !reflect.DeepEqual(got, providers) {
		t.Fatalf("envelope roundtrip: %v", err)
	}
	for _, version := range []int{0, cacheSchemaVersion - 1, cacheSchemaVersion + 1} {
		if _, err := decodeModels([]byte(fmt.Sprintf(`{"version":%d,"providers":%s}`, version, updateFixture))); err == nil {
			t.Fatal("accepted incompatible schema")
		}
	}
}

func TestModelsHTTPValidators(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	var wantValidator string
	status := http.StatusOK
	etag := `"same"`
	body := updateFixture
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("User-Agent"); got != "KIT" {
			t.Errorf("user agent=%q", got)
		}
		if got := r.Header.Get("If-None-Match"); got != wantValidator {
			t.Errorf("validator=%q want=%q", got, wantValidator)
		}
		w.Header().Set("ETag", etag)
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()
	result := mustUpdate(t, server.URL+"?q=1", UpdateModelsStatusUpdated)
	if result.ProviderCount != 2 || result.ModelCount != 1 {
		t.Fatal(result)
	}
	env := loadCachedEnvelope()
	if env.SourceFingerprint != sourceFingerprint(server.URL+"?q=1") || env.ETag != etag {
		t.Fatal(env)
	}
	wantValidator = etag
	status = http.StatusNotModified
	before := cacheBytes(t)
	result = mustUpdate(t, server.URL+"?q=1", UpdateModelsStatusNotModified)
	if result.ProviderCount != 2 || result.ModelCount != 1 || !bytes.Equal(before, cacheBytes(t)) {
		t.Fatal("304 changed snapshot")
	}

	wantValidator = ""
	status = http.StatusOK
	mustUpdate(t, server.URL+"?q=2", UpdateModelsStatusUpdated)
	mustUpdate(t, server.URL+"/another", UpdateModelsStatusUpdated)
	// No server validator means the next request must also be unconditional.
	etag = ""
	mustUpdate(t, server.URL, UpdateModelsStatusUpdated)
	mustUpdate(t, server.URL, UpdateModelsStatusUpdated)
	before = cacheBytes(t)
	status = http.StatusNotModified
	if got, err := UpdateModels(context.Background(), UpdateModelsOptions{Source: server.URL}); err == nil || got != (UpdateModelsResult{}) {
		t.Fatalf("unsolicited 304: %+v %v", got, err)
	}
	if !bytes.Equal(before, cacheBytes(t)) {
		t.Fatal("unsolicited 304 changed disk")
	}

	status = http.StatusOK
	for _, invalid := range []string{`null`, `{"p":{"models":{}}}`, updateFixture + `null`} {
		body = invalid
		if _, err := UpdateModels(context.Background(), UpdateModelsOptions{Source: server.URL}); err == nil {
			t.Fatal("accepted invalid HTTP data")
		}
		if !bytes.Equal(before, cacheBytes(t)) {
			t.Fatal("invalid HTTP data changed disk")
		}
	}
	status = http.StatusInternalServerError
	if _, err := UpdateModels(context.Background(), UpdateModelsOptions{Source: server.URL}); err == nil {
		t.Fatal("accepted server failure")
	}
	if !bytes.Equal(before, cacheBytes(t)) {
		t.Fatal("HTTP error changed disk")
	}
}

func TestModelsLegacyAndFileValidators(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("If-None-Match") != "" {
			t.Error("sent untrusted validator")
		}
		_, _ = w.Write([]byte(updateFixture))
	}))
	defer server.Close()
	providers, err := decodeModels([]byte(updateFixture))
	if err != nil {
		t.Fatal(err)
	}
	for _, version := range []int{0, cacheSchemaVersion - 1, cacheSchemaVersion, cacheSchemaVersion + 1} {
		data, err := json.Marshal(cacheEnvelope{Version: version, ETag: `"same"`, Providers: providers})
		if err != nil {
			t.Fatal(err)
		}
		path, err := cachePath()
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
		mustUpdate(t, server.URL, UpdateModelsStatusUpdated)
	}
	file := filepath.Join(t.TempDir(), "import.json")
	data, err := json.Marshal(cacheEnvelope{Version: cacheSchemaVersion, ETag: `"same"`, SourceFingerprint: sourceFingerprint(server.URL), Providers: providers})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, data, 0o600); err != nil {
		t.Fatal(err)
	}
	mustUpdate(t, file, UpdateModelsStatusUpdated)
	env := loadCachedEnvelope()
	if env.ETag != "" || env.SourceFingerprint != "" || !reflect.DeepEqual(providers, env.Providers) {
		t.Fatal("file lost metadata or retained validator")
	}
	before := cacheBytes(t)
	if err := os.WriteFile(file, []byte(`null`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := UpdateModels(context.Background(), UpdateModelsOptions{Source: file}); err == nil {
		t.Fatal("accepted invalid file")
	}
	if !bytes.Equal(before, cacheBytes(t)) {
		t.Fatal("invalid file changed cache")
	}
	mustUpdate(t, server.URL, UpdateModelsStatusUpdated)
}

func TestModelsRedirect(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	redirect := false
	targetStatus := http.StatusOK
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/target" {
			if r.Header.Get("If-None-Match") != "" {
				t.Error("forwarded source validator")
			}
			w.Header().Set("ETag", `"target"`)
			w.WriteHeader(targetStatus)
			_, _ = w.Write([]byte(updateFixture))
			return
		}
		if redirect {
			http.Redirect(w, r, "/target", http.StatusFound)
			return
		}
		w.Header().Set("ETag", `"source"`)
		_, _ = w.Write([]byte(updateFixture))
	}))
	defer server.Close()
	mustUpdate(t, server.URL, UpdateModelsStatusUpdated)
	before := cacheBytes(t)
	redirect = true
	targetStatus = http.StatusNotModified
	if _, err := UpdateModels(context.Background(), UpdateModelsOptions{Source: server.URL}); err == nil {
		t.Fatal("accepted redirected 304")
	}
	if !bytes.Equal(before, cacheBytes(t)) {
		t.Fatal("redirected 304 changed cache")
	}
	targetStatus = http.StatusOK
	mustUpdate(t, server.URL, UpdateModelsStatusUpdated)
	if loadCachedEnvelope().ETag != "" {
		t.Fatal("persisted target validator for original source")
	}
}

func TestModelsCancellation(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	file := filepath.Join(t.TempDir(), "models.json")
	if err := os.WriteFile(file, []byte(updateFixture), 0o600); err != nil {
		t.Fatal(err)
	}
	mustUpdate(t, file, UpdateModelsStatusUpdated)
	before := cacheBytes(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, source := range []string{file, "embedded", "http://localhost:1"} {
		got, err := UpdateModels(ctx, UpdateModelsOptions{Source: source})
		if !errors.Is(err, context.Canceled) || got != (UpdateModelsResult{}) {
			t.Fatalf("%s: %+v %v", source, got, err)
		}
		if !bytes.Equal(before, cacheBytes(t)) {
			t.Fatal("cancellation changed cache")
		}
	}
	//nolint:staticcheck // Exercise the required nil-context rejection contract.
	if _, err := UpdateModels(nil, UpdateModelsOptions{}); err == nil {
		t.Fatal("accepted nil context")
	}
	for _, body := range []bool{false, true} {
		t.Run(fmt.Sprint(body), func(t *testing.T) {
			started := make(chan struct{})
			closed := make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer close(closed)
				if body {
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`{"p":`))
					w.(http.Flusher).Flush()
				}
				close(started)
				<-r.Context().Done()
			}))
			defer server.Close()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan error, 1)
			go func() { _, err := UpdateModels(ctx, UpdateModelsOptions{Source: server.URL}); done <- err }()
			select {
			case <-started:
			case <-time.After(3 * time.Second):
				t.Fatal("request did not start")
			}
			cancel()
			select {
			case err := <-done:
				if !errors.Is(err, context.Canceled) {
					t.Fatal(err)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("cancellation hung")
			}
			select {
			case <-closed:
			case <-time.After(3 * time.Second):
				t.Fatal("request body not closed")
			}
			if !bytes.Equal(before, cacheBytes(t)) {
				t.Fatal("cancelled request changed cache")
			}
		})
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer server.Close()
	ctx, cancel = context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := UpdateModels(ctx, UpdateModelsOptions{Source: server.URL}); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
	if !bytes.Equal(before, cacheBytes(t)) {
		t.Fatal("deadline changed cache")
	}
	bounded, stop := modelsUpdateContext(context.Background())
	defer stop()
	deadline, ok := bounded.Deadline()
	if !ok || time.Until(deadline) > 30*time.Second || time.Until(deadline) < 29*time.Second {
		t.Fatal("missing 30 second bound")
	}
	bounded, stop = modelsUpdateContext(ctx)
	defer stop()
	if got, _ := bounded.Deadline(); !got.Equal(func() time.Time { d, _ := ctx.Deadline(); return d }()) {
		t.Fatal("caller deadline changed")
	}
}

func TestModelsResetAndRegistrySnapshots(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	file := filepath.Join(t.TempDir(), "models.json")
	// Chat fixture permits checking the normal registry merge path.
	if err := os.WriteFile(file, []byte(`{"fixture-update":{"id":"fixture-update","models":{"m":{"id":"m","name":"Updated"}}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	old := NewModelsRegistry()
	mustUpdate(t, file, UpdateModelsStatusUpdated)
	newRegistry := NewModelsRegistry()
	if old.LookupModel("fixture-update", "m") != nil || newRegistry.LookupModel("fixture-update", "m") == nil {
		t.Fatal("registry snapshot semantics changed")
	}
	for range 2 {
		got := mustUpdate(t, "embedded", UpdateModelsStatusReset)
		if got.ProviderCount != 0 || got.ModelCount != 0 {
			t.Fatal(got)
		}
	}
	if newRegistry.LookupModel("fixture-update", "m") == nil || NewModelsRegistry().LookupModel("fixture-update", "m") != nil {
		t.Fatal("reset reloaded an existing registry")
	}
	path, err := cachePath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "block"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := UpdateModels(context.Background(), UpdateModelsOptions{Source: "embedded"}); err == nil {
		t.Fatal("ignored remove error")
	}
}

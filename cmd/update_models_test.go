package cmd

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mark3labs/kit/internal/models"
)

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
			_, _ = w.Write([]byte(`{"test":{"id":"test","name":"Test","models":{}}}`))
			return
		}
		if r.Header.Get("If-None-Match") != `"fixture"` {
			t.Errorf("ETag = %q", r.Header.Get("If-None-Match"))
		}
		w.WriteHeader(http.StatusNotModified)
	}))
	defer srv.Close()
	for range 2 {
		if err := fetchFromURL(srv.URL); err != nil {
			t.Fatal(err)
		}
	}
	_, etag := models.LoadCachedProviders()
	if etag != `"fixture"` {
		t.Errorf("stored ETag = %q", etag)
	}
}

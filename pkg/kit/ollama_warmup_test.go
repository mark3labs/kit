package kit

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

// TestOllamaWarmupGPUOptions checks the options of the request that loads an
// Ollama model. Kit sends no GPU options with the chat requests. Since
// Ollama 0.30.0, a main_gpu value in the load request alone makes Ollama
// load the model again for the chat requests (issue #265). Kit must
// therefore send the GPU options only when the user sets them.
func TestOllamaWarmupGPUOptions(t *testing.T) {
	for _, tc := range []struct {
		name, numGPU, mainGPU string
		want                  map[string]any
	}{
		{name: "not set", want: map[string]any{}},
		{name: "set", numGPU: "20", mainGPU: "0", want: map[string]any{"num_gpu": float64(20), "main_gpu": float64(0)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			// Viper treats an empty variable as unset.
			t.Setenv("KIT_NUM_GPU_LAYERS", tc.numGPU)
			t.Setenv("KIT_MAIN_GPU", tc.mainGPU)

			var mu sync.Mutex
			var warmups []map[string]any
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.URL.Path == "/api/generate" {
					var body struct {
						Options map[string]any `json:"options"`
					}
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Errorf("decode /api/generate body: %v", err)
					}
					mu.Lock()
					warmups = append(warmups, body.Options)
					mu.Unlock()
					_, _ = w.Write([]byte(`{"done":true}`))
					return
				}
				_, _ = w.Write([]byte(`{}`))
			}))
			defer server.Close()

			newProviderTestKit(t, &Options{Model: "ollama/test", ProviderURL: server.URL})

			mu.Lock()
			defer mu.Unlock()
			if len(warmups) != 1 {
				t.Fatalf("got %d /api/generate requests, want 1", len(warmups))
			}
			for _, key := range []string{"num_gpu", "main_gpu"} {
				got, gotOK := warmups[0][key]
				want, wantOK := tc.want[key]
				if gotOK != wantOK || got != want {
					t.Errorf("option %s = %v (sent: %v), want %v (sent: %v)", key, got, gotOK, want, wantOK)
				}
			}
		})
	}
}

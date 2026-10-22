package extensions

import (
	"strings"
	"sync"
	"testing"
)

const providerFixture = `package main
import ext "kit/ext"
func create(req ext.ProviderCreateRequest) ext.ProviderCreateResult {
 return ext.ProviderCreateResult{Runtime: ext.ProviderRuntime{
  Stream: func(req ext.ProviderRequest, emit func(ext.ProviderEvent) bool) ext.ProviderResult {
   select { case <-req.Done: return ext.ProviderResult{Error: &ext.ProviderError{Code: "canceled"}}; default: }
   if !emit(ext.ProviderEvent{Type: ext.ProviderTextStart, ID: "text"}) { return ext.ProviderResult{} }
   if !emit(ext.ProviderEvent{Type: ext.ProviderTextDelta, ID: "text", Delta: "Hello from a fake provider."}) { return ext.ProviderResult{} }
   if !emit(ext.ProviderEvent{Type: ext.ProviderTextEnd, ID: "text"}) { return ext.ProviderResult{} }
   emit(ext.ProviderEvent{Type: ext.ProviderFinish, FinishReason: ext.ProviderFinishStop})
   return ext.ProviderResult{}
  },
  Close: func() error { return nil },
 }}
}
func Init(api ext.API) {
 api.RegisterProvider(ext.ProviderDef{ID: "FAKE", Name: "Fake", Create: create,
  Models: []ext.ProviderModel{{ID:"echo",Name:"Echo",Capabilities: ext.ProviderCapabilities{Streaming:true}}},
 })
}`

func TestProviderProductionLoaderCallbacks(t *testing.T) {
	loaded, err := LoadExtensionsScoped([]string{writeExt(t, providerFixture)}, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 1 || len(loaded[0].Providers) != 1 {
		t.Fatalf("registrations = %#v", loaded)
	}
	def := loaded[0].Providers[0]
	if def.ID != "fake" {
		t.Fatalf("ID = %q", def.ID)
	}
	var wg sync.WaitGroup
	for range 8 {
		runtime := def.Create(ProviderCreateRequest{}).Runtime
		wg.Go(func() {
			for range 100 {
				text := ""
				result := runtime.Stream(ProviderRequest{}, func(event ProviderEvent) bool { text += event.Delta; return true })
				if result.Error != nil || text != "Hello from a fake provider." {
					t.Errorf("text=%q error=%v", text, result.Error)
				}
			}
			if err := runtime.Close(); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
}

func TestProviderRegistrationFreezesMetadata(t *testing.T) {
	def := ProviderDef{ID: "Fake", Name: "Fake", Create: func(ProviderCreateRequest) ProviderCreateResult { return ProviderCreateResult{} }, Models: []ProviderModel{{ID: "a/b", Name: "AB", Capabilities: ProviderCapabilities{InputModalities: []string{"text"}}}}}
	frozen, err := freezeProvider(def)
	if err != nil {
		t.Fatal(err)
	}
	def.Models[0].Capabilities.InputModalities[0] = "image"
	def.Models[0].ID = "changed"
	if frozen.Models[0].ID != "a/b" || frozen.Models[0].Capabilities.InputModalities[0] != "text" {
		t.Fatal("registration shares caller metadata")
	}
}

func TestInvalidProviderRegistrationFailsBulkLoad(t *testing.T) {
	path := writeExt(t, strings.Replace(providerFixture, `ID: "FAKE"`, `ID: "not/a/provider"`, 1))
	loaded, err := LoadExtensionsScoped([]string{path}, true)
	if err == nil || len(loaded) != 0 {
		t.Fatalf("loaded=%d err=%v", len(loaded), err)
	}
	if !strings.Contains(err.Error(), path) {
		t.Fatalf("missing owner: %v", err)
	}
}

func TestStrictProviderCandidateRetainsCompileFailures(t *testing.T) {
	bad := writeExt(t, "package main\nfunc invalid syntax")
	good := writeExt(t, providerFixture)
	result := LoadExtensionsWithDiagnostics([]string{bad, good}, true)
	if len(result.Diagnostics) != 1 || len(result.Extensions) != 1 {
		t.Fatalf("diagnostics=%v count=%d", result.Diagnostics, len(result.Extensions))
	}
	if loaded, err := LoadExtensionsStrict([]string{bad, good}, true); err == nil || loaded != nil {
		t.Fatalf("strict loaded=%v err=%v", loaded, err)
	}
}

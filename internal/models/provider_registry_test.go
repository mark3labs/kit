package models

import (
	"context"
	"errors"
	"iter"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"charm.land/fantasy"
)

// stubModel is a minimal LanguageModel for provider factory tests.
type stubModel struct {
	provider string
	model    string
}

func (m *stubModel) Generate(context.Context, fantasy.Call) (*fantasy.Response, error) {
	return &fantasy.Response{Content: fantasy.ResponseContent{fantasy.TextContent{Text: "ok"}}}, nil
}

func (m *stubModel) Stream(context.Context, fantasy.Call) (fantasy.StreamResponse, error) {
	return iter.Seq[fantasy.StreamPart](func(func(fantasy.StreamPart) bool) {}), nil
}

func (m *stubModel) GenerateObject(context.Context, fantasy.ObjectCall) (*fantasy.ObjectResponse, error) {
	return nil, errors.New("not implemented")
}

func (m *stubModel) StreamObject(context.Context, fantasy.ObjectCall) (fantasy.ObjectStreamResponse, error) {
	return nil, errors.New("not implemented")
}

func (m *stubModel) Provider() string { return m.provider }
func (m *stubModel) Model() string    { return m.model }

type closerFunc func() error

func (f closerFunc) Close() error { return f() }

// registerForTest registers a process-wide factory and removes it when the
// test ends.
func registerForTest(t *testing.T, name string, f ProviderFactory) {
	t.Helper()
	if err := RegisterProviderFactory(name, f); err != nil {
		t.Fatalf("RegisterProviderFactory(%q): %v", name, err)
	}
	t.Cleanup(func() { UnregisterProviderFactory(name) })
}

func stubFactory(provider string) ProviderFactory {
	return func(_ context.Context, _ *ProviderConfig, modelName string) (*ProviderResult, error) {
		return &ProviderResult{Model: &stubModel{provider: provider, model: modelName}}, nil
	}
}

func TestRegisterProviderFactory_Validation(t *testing.T) {
	if err := RegisterProviderFactory("", stubFactory("x")); err == nil {
		t.Error("empty name must be rejected")
	}
	if err := RegisterProviderFactory("  ", stubFactory("x")); err == nil {
		t.Error("blank name must be rejected")
	}
	if err := RegisterProviderFactory("a/b", stubFactory("x")); err == nil {
		t.Error("name with '/' must be rejected")
	}
}

func TestRegisterProviderFactory_Lifecycle(t *testing.T) {
	registerForTest(t, "Zeta-Test", stubFactory("zeta"))
	registerForTest(t, "alpha-test", stubFactory("alpha"))

	names := RegisteredProviderFactories()
	if !slices.Contains(names, "zeta-test") || !slices.Contains(names, "alpha-test") {
		t.Fatalf("RegisteredProviderFactories = %v, want normalized names", names)
	}
	if !slices.IsSorted(names) {
		t.Errorf("RegisteredProviderFactories = %v, want sorted", names)
	}
	if !HasProviderFactory(nil, "ZETA-TEST") {
		t.Error("lookup must be case-insensitive")
	}

	// A nil factory removes the registration.
	if err := RegisterProviderFactory("zeta-test", nil); err != nil {
		t.Fatalf("nil factory: %v", err)
	}
	if HasProviderFactory(nil, "zeta-test") {
		t.Error("nil factory must remove the registration")
	}

	if !UnregisterProviderFactory("alpha-test") {
		t.Error("UnregisterProviderFactory must report an existing registration")
	}
	if UnregisterProviderFactory("alpha-test") {
		t.Error("UnregisterProviderFactory must report false for a missing registration")
	}
}

func TestValidateProviderFactories(t *testing.T) {
	if err := ValidateProviderFactories(nil); err != nil {
		t.Errorf("nil map: %v", err)
	}
	if err := ValidateProviderFactories(map[string]ProviderFactory{"ok": stubFactory("ok")}); err != nil {
		t.Errorf("valid map: %v", err)
	}
	if err := ValidateProviderFactories(map[string]ProviderFactory{"a/b": stubFactory("x")}); err == nil {
		t.Error("name with '/' must be rejected")
	}
	if err := ValidateProviderFactories(map[string]ProviderFactory{"ok": nil}); err == nil {
		t.Error("nil factory must be rejected")
	}
}

func TestCreateProvider_UsesGlobalFactory(t *testing.T) {
	var gotModel string
	var gotMaxTokens int
	registerForTest(t, "embedded-test", func(_ context.Context, cfg *ProviderConfig, modelName string) (*ProviderResult, error) {
		gotModel = modelName
		gotMaxTokens = cfg.MaxTokens
		return &ProviderResult{Model: &stubModel{provider: "embedded-test", model: modelName}}, nil
	})

	result, err := CreateProvider(context.Background(), &ProviderConfig{
		ModelString: "Embedded-Test/org/model-q8.gguf",
		MaxTokens:   1234,
	})
	if err != nil {
		t.Fatalf("CreateProvider: %v", err)
	}
	if gotModel != "org/model-q8.gguf" {
		t.Errorf("factory got model %q, want everything after the first '/'", gotModel)
	}
	if gotMaxTokens != 1234 {
		t.Errorf("factory got MaxTokens %d, want 1234", gotMaxTokens)
	}
	if result.Model.Model() != "org/model-q8.gguf" {
		t.Errorf("result model = %q", result.Model.Model())
	}
}

func TestCreateProvider_InstanceFactoryWinsOverGlobal(t *testing.T) {
	registerForTest(t, "shared-test", stubFactory("global"))

	result, err := CreateProvider(context.Background(), &ProviderConfig{
		ModelString: "shared-test/m",
		ProviderFactories: map[string]ProviderFactory{
			"SHARED-TEST": stubFactory("instance"),
		},
	})
	if err != nil {
		t.Fatalf("CreateProvider: %v", err)
	}
	if got := result.Model.Provider(); got != "instance" {
		t.Errorf("provider = %q, want instance factory", got)
	}
}

func TestCreateProvider_FactoryOverridesBuiltin(t *testing.T) {
	// No credentials are configured: without the factory this would fail
	// or reach the network.
	t.Setenv("OPENAI_API_KEY", "")
	result, err := CreateProvider(context.Background(), &ProviderConfig{
		ModelString:       "openai/gpt-4o",
		ProviderFactories: map[string]ProviderFactory{"openai": stubFactory("proxy")},
	})
	if err != nil {
		t.Fatalf("CreateProvider: %v", err)
	}
	if got := result.Model.Provider(); got != "proxy" {
		t.Errorf("provider = %q, want proxy", got)
	}
	if result.ProviderOptions != nil {
		t.Errorf("factory results must not get automatic provider options, got %v", result.ProviderOptions)
	}
}

func TestCreateProvider_FactoryErrors(t *testing.T) {
	var closed atomic.Int32
	closer := closerFunc(func() error { closed.Add(1); return nil })

	tests := []struct {
		name    string
		factory ProviderFactory
		wantErr string
	}{
		{
			name: "error is wrapped",
			factory: func(context.Context, *ProviderConfig, string) (*ProviderResult, error) {
				return &ProviderResult{Closer: closer}, errors.New("boom")
			},
			wantErr: "provider broken-test: boom",
		},
		{
			name: "nil result",
			factory: func(context.Context, *ProviderConfig, string) (*ProviderResult, error) {
				return nil, nil
			},
			wantErr: "factory returned no model",
		},
		{
			name: "nil model",
			factory: func(context.Context, *ProviderConfig, string) (*ProviderResult, error) {
				return &ProviderResult{Closer: closer}, nil
			},
			wantErr: "factory returned no model",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := CreateProvider(context.Background(), &ProviderConfig{
				ModelString:       "broken-test/m",
				ProviderFactories: map[string]ProviderFactory{"broken-test": tt.factory},
			})
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("err = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
	if got := closed.Load(); got != 2 {
		t.Errorf("closer called %d times, want 2 (once per failed result with a closer)", got)
	}
}

func TestValidateModelString_AcceptsRegisteredProvider(t *testing.T) {
	r := GetGlobalRegistry()
	if err := r.ValidateModelString("unknown-factory-test/m"); err == nil {
		t.Fatal("unregistered unknown provider must be rejected")
	}
	registerForTest(t, "unknown-factory-test", stubFactory("x"))
	if err := r.ValidateModelString("unknown-factory-test/m"); err != nil {
		t.Errorf("registered provider must be accepted: %v", err)
	}
}

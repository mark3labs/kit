package kit

import (
	"context"
	"errors"
	"iter"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"charm.land/fantasy"
)

// echoModel is an in-process language model that answers every call with a
// fixed text that includes its model name. It stands in for an application
// bundled inference backend.
type echoModel struct {
	provider string
	model    string
}

func (m *echoModel) reply() string { return "reply from " + m.model }

func (m *echoModel) Generate(context.Context, fantasy.Call) (*fantasy.Response, error) {
	return &fantasy.Response{
		Content:      fantasy.ResponseContent{fantasy.TextContent{Text: m.reply()}},
		FinishReason: fantasy.FinishReasonStop,
		Usage:        fantasy.Usage{InputTokens: 3, OutputTokens: 2},
	}, nil
}

func (m *echoModel) Stream(context.Context, fantasy.Call) (fantasy.StreamResponse, error) {
	text := m.reply()
	return iter.Seq[fantasy.StreamPart](func(yield func(fantasy.StreamPart) bool) {
		parts := []fantasy.StreamPart{
			{Type: fantasy.StreamPartTypeTextStart, ID: "t1"},
			{Type: fantasy.StreamPartTypeTextDelta, ID: "t1", Delta: text},
			{Type: fantasy.StreamPartTypeTextEnd, ID: "t1"},
			{
				Type:         fantasy.StreamPartTypeFinish,
				FinishReason: fantasy.FinishReasonStop,
				Usage:        fantasy.Usage{InputTokens: 3, OutputTokens: 2},
			},
		}
		for _, p := range parts {
			if !yield(p) {
				return
			}
		}
	}), nil
}

func (m *echoModel) GenerateObject(context.Context, fantasy.ObjectCall) (*fantasy.ObjectResponse, error) {
	return nil, errors.New("not implemented")
}

func (m *echoModel) StreamObject(context.Context, fantasy.ObjectCall) (fantasy.ObjectStreamResponse, error) {
	return nil, errors.New("not implemented")
}

func (m *echoModel) Provider() string { return m.provider }
func (m *echoModel) Model() string    { return m.model }

type closerFunc func() error

func (f closerFunc) Close() error { return f() }

// recordingFactory builds echo models and records which models it built and
// which of them were closed.
type recordingFactory struct {
	mu      sync.Mutex
	created []string
	closed  []string
}

func (r *recordingFactory) factory(provider string) ProviderFactory {
	return func(_ context.Context, _ *ProviderConfig, modelName string) (*ProviderResult, error) {
		r.mu.Lock()
		r.created = append(r.created, modelName)
		r.mu.Unlock()
		return &ProviderResult{
			Model: LLMLanguageModel(&echoModel{provider: provider, model: modelName}),
			Closer: closerFunc(func() error {
				r.mu.Lock()
				r.closed = append(r.closed, modelName)
				r.mu.Unlock()
				return nil
			}),
		}, nil
	}
}

func (r *recordingFactory) snapshot() (created, closed []string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.created), slices.Clone(r.closed)
}

func newProviderTestKit(t *testing.T, opts *Options) *Kit {
	t.Helper()
	opts.Quiet = true
	opts.NoSession = true
	opts.NoExtensions = true
	opts.DisableCoreTools = true
	opts.SkipConfig = true
	k, err := New(context.Background(), opts)
	if err != nil {
		t.Fatalf("kit.New: %v", err)
	}
	t.Cleanup(func() { _ = k.Close() })
	return k
}

func TestProviders_InstanceFactoryEndToEnd(t *testing.T) {
	rec := &recordingFactory{}
	k := newProviderTestKit(t, &Options{
		Model:     "local/tiny-1b",
		Providers: map[string]ProviderFactory{"local": rec.factory("local")},
	})

	got, err := k.Prompt(context.Background(), "hello")
	if err != nil {
		t.Fatalf("Prompt: %v", err)
	}
	if !strings.Contains(got, "reply from tiny-1b") {
		t.Errorf("Prompt = %q, want the bundled model's reply", got)
	}

	// A model switch builds the new model through the factory and closes
	// the old one.
	if err := k.SetModel(context.Background(), "local/tiny-3b"); err != nil {
		t.Fatalf("SetModel: %v", err)
	}
	got, err = k.Prompt(context.Background(), "again")
	if err != nil {
		t.Fatalf("Prompt after SetModel: %v", err)
	}
	if !strings.Contains(got, "reply from tiny-3b") {
		t.Errorf("Prompt after SetModel = %q", got)
	}
	created, closed := rec.snapshot()
	if !slices.Contains(created, "tiny-3b") {
		t.Errorf("created = %v, want tiny-3b", created)
	}
	if !slices.Contains(closed, "tiny-1b") {
		t.Errorf("closed = %v, want tiny-1b closed on model switch", closed)
	}

	// Close releases the active model.
	if err := k.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	_, closed = rec.snapshot()
	if !slices.Contains(closed, "tiny-3b") {
		t.Errorf("closed = %v, want tiny-3b closed on Kit.Close", closed)
	}
}

func TestProviders_GlobalRegistration(t *testing.T) {
	rec := &recordingFactory{}
	if err := RegisterProvider("sdk-global-test", rec.factory("sdk-global-test")); err != nil {
		t.Fatalf("RegisterProvider: %v", err)
	}
	t.Cleanup(func() { UnregisterProvider("sdk-global-test") })

	if !slices.Contains(RegisteredProviders(), "sdk-global-test") {
		t.Errorf("RegisteredProviders = %v", RegisteredProviders())
	}

	k := newProviderTestKit(t, &Options{Model: "sdk-global-test/m1"})
	got, err := k.Prompt(context.Background(), "hi")
	if err != nil {
		t.Fatalf("Prompt: %v", err)
	}
	if !strings.Contains(got, "reply from m1") {
		t.Errorf("Prompt = %q", got)
	}
}

func TestProviders_WithProviderOption(t *testing.T) {
	var calls atomic.Int32
	o := &Options{}
	WithModel("opt/m")(o)
	WithProvider("opt", func(_ context.Context, _ *ProviderConfig, modelName string) (*ProviderResult, error) {
		calls.Add(1)
		return &ProviderResult{Model: &echoModel{provider: "opt", model: modelName}}, nil
	})(o)

	k := newProviderTestKit(t, o)
	if _, err := k.Prompt(context.Background(), "hi"); err != nil {
		t.Fatalf("Prompt: %v", err)
	}
	if calls.Load() == 0 {
		t.Error("WithProvider factory was not used")
	}
}

func TestProviders_ExecuteCompletionWithFactoryModel(t *testing.T) {
	rec := &recordingFactory{}
	k := newProviderTestKit(t, &Options{
		Model:     "local/main",
		Providers: map[string]ProviderFactory{"local": rec.factory("local")},
	})

	resp, err := k.ExecuteCompletion(context.Background(), CompleteRequest{
		Model:  "local/side",
		Prompt: "summarize",
	})
	if err != nil {
		t.Fatalf("ExecuteCompletion: %v", err)
	}
	if !strings.Contains(resp.Text, "reply from side") {
		t.Errorf("completion = %q", resp.Text)
	}
	_, closed := rec.snapshot()
	if !slices.Contains(closed, "side") {
		t.Errorf("closed = %v, want the temporary model closed", closed)
	}
}

func TestProviders_SubagentInheritsInstanceFactories(t *testing.T) {
	rec := &recordingFactory{}
	k := newProviderTestKit(t, &Options{
		Model:     "local/parent",
		Providers: map[string]ProviderFactory{"local": rec.factory("local")},
	})

	res, err := k.Subagent(context.Background(), SubagentConfig{
		Prompt:    "do it",
		Model:     "local/child",
		Tools:     []Tool{},
		NoSession: true,
	})
	if err != nil {
		t.Fatalf("Subagent: %v", err)
	}
	if !strings.Contains(res.Response, "reply from child") {
		t.Errorf("subagent response = %q", res.Response)
	}
}

func TestProviders_InvalidInstanceFactories(t *testing.T) {
	for name, providers := range map[string]map[string]ProviderFactory{
		"slash in name": {"a/b": func(context.Context, *ProviderConfig, string) (*ProviderResult, error) { return nil, nil }},
		"nil factory":   {"local": nil},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := New(context.Background(), &Options{
				Model:        "local/m",
				Providers:    providers,
				Quiet:        true,
				NoSession:    true,
				NoExtensions: true,
				SkipConfig:   true,
			})
			if err == nil || !strings.Contains(err.Error(), "Options.Providers") {
				t.Fatalf("err = %v, want Options.Providers validation error", err)
			}
		})
	}
}

// configCapture records the ProviderConfig each factory call receives.
type configCapture struct {
	mu   sync.Mutex
	cfgs map[string]ProviderConfig // keyed by model name
}

func (c *configCapture) factory(provider string) ProviderFactory {
	return func(_ context.Context, cfg *ProviderConfig, modelName string) (*ProviderResult, error) {
		c.mu.Lock()
		if c.cfgs == nil {
			c.cfgs = make(map[string]ProviderConfig)
		}
		c.cfgs[modelName] = *cfg
		c.mu.Unlock()
		return &ProviderResult{Model: &echoModel{provider: provider, model: modelName}}, nil
	}
}

func (c *configCapture) get(model string) (ProviderConfig, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	cfg, ok := c.cfgs[model]
	return cfg, ok
}

// Regression test for the CodeRabbit finding on PR #143: a temporary
// factory model in ExecuteCompletion must get the Kit's effective settings,
// and endpoint overrides only when they belong to the factory's provider.
func TestProviders_ExecuteCompletionPassesEffectiveSettings(t *testing.T) {
	local := &configCapture{}
	other := &configCapture{}
	temp := float32(0.3)
	k := newProviderTestKit(t, &Options{
		Model:          "local/main",
		Temperature:    &temp,
		ProviderURL:    "http://127.0.0.1:9/v1",
		ProviderAPIKey: "local-key",
		Providers: map[string]ProviderFactory{
			"local": local.factory("local"),
			"other": other.factory("other"),
		},
	})

	if _, err := k.ExecuteCompletion(context.Background(), CompleteRequest{
		Model: "local/side", Prompt: "x", MaxTokens: 77,
	}); err != nil {
		t.Fatalf("ExecuteCompletion local: %v", err)
	}
	cfg, ok := local.get("side")
	if !ok {
		t.Fatal("local factory was not called for local/side")
	}
	if cfg.ProviderURL != "http://127.0.0.1:9/v1" || cfg.ProviderAPIKey != "local-key" {
		t.Errorf("local cfg endpoint = (%q, %q), want the Kit's overrides", cfg.ProviderURL, cfg.ProviderAPIKey)
	}
	if cfg.Temperature == nil || *cfg.Temperature != temp {
		t.Errorf("local cfg Temperature = %v, want %v", cfg.Temperature, temp)
	}
	if cfg.MaxTokens != 77 {
		t.Errorf("local cfg MaxTokens = %d, want the request's 77", cfg.MaxTokens)
	}

	// The overrides are bound to "local": another provider must not get them.
	if _, err := k.ExecuteCompletion(context.Background(), CompleteRequest{
		Model: "other/m", Prompt: "x",
	}); err != nil {
		t.Fatalf("ExecuteCompletion other: %v", err)
	}
	cfg, ok = other.get("m")
	if !ok {
		t.Fatal("other factory was not called for other/m")
	}
	if cfg.ProviderURL != "" || cfg.ProviderAPIKey != "" || cfg.ProviderWire != "" {
		t.Errorf("other cfg endpoint = (%q, %q, %q), want empty", cfg.ProviderURL, cfg.ProviderAPIKey, cfg.ProviderWire)
	}
	if cfg.Temperature == nil || *cfg.Temperature != temp {
		t.Errorf("other cfg Temperature = %v, want %v", cfg.Temperature, temp)
	}
}

// Regression test for the CodeRabbit finding on PR #143: New must snapshot
// Options.Providers so later changes to the caller's map do not reach an
// existing Kit.
func TestProviders_NewSnapshotsProvidersMap(t *testing.T) {
	first := &recordingFactory{}
	second := &recordingFactory{}
	opts := &Options{
		Model:     "local/a",
		Providers: map[string]ProviderFactory{"local": first.factory("local")},
	}
	k := newProviderTestKit(t, opts)

	// Replace the factory in the caller's map after construction.
	opts.Providers["local"] = second.factory("local")

	if err := k.SetModel(context.Background(), "local/b"); err != nil {
		t.Fatalf("SetModel: %v", err)
	}
	if created, _ := first.snapshot(); !slices.Contains(created, "b") {
		t.Errorf("first factory created %v, want b (snapshot must be used)", created)
	}
	if created, _ := second.snapshot(); len(created) != 0 {
		t.Errorf("second factory created %v, want none", created)
	}
}

func TestProviders_RejectsCaseVariantNames(t *testing.T) {
	f := (&recordingFactory{}).factory("local")
	_, err := New(context.Background(), &Options{
		Model:        "local/m",
		Providers:    map[string]ProviderFactory{"Local": f, "local": f},
		Quiet:        true,
		NoSession:    true,
		NoExtensions: true,
		SkipConfig:   true,
	})
	if err == nil || !strings.Contains(err.Error(), "same provider") {
		t.Fatalf("err = %v, want a 'same provider' error", err)
	}
}

// Regression test for the CodeRabbit finding on PR #143: CreateProvider may
// raise MaxTokens (right-sizing to the model's known output limit) before it
// calls the factory. ExecuteCompletion must give the factory the request's
// MaxTokens, which is the limit the completion agent uses.
func TestProviders_ExecuteCompletionKeepsRequestMaxTokens(t *testing.T) {
	capture := &configCapture{}
	// openai/gpt-4o is in the model database with a known output limit, so
	// right-sizing would raise an unpinned MaxTokens.
	k := newProviderTestKit(t, &Options{
		Model:     "openai/gpt-4o",
		Providers: map[string]ProviderFactory{"openai": capture.factory("openai")},
	})

	if _, err := k.ExecuteCompletion(context.Background(), CompleteRequest{
		Model: "openai/gpt-4o", Prompt: "x", MaxTokens: 77,
	}); err != nil {
		t.Fatalf("ExecuteCompletion: %v", err)
	}
	cfg, ok := capture.get("gpt-4o")
	if !ok {
		t.Fatal("factory was not called")
	}
	if cfg.MaxTokens != 77 {
		t.Errorf("factory cfg MaxTokens = %d, want the request's 77", cfg.MaxTokens)
	}
}

// testProviderOption is a minimal provider option value.
type testProviderOption struct{}

func (testProviderOption) Options()                     {}
func (testProviderOption) MarshalJSON() ([]byte, error) { return []byte(`{}`), nil }
func (*testProviderOption) UnmarshalJSON([]byte) error  { return nil }

// optionsModel records the provider options of each call.
type optionsModel struct {
	echoModel
	mu     sync.Mutex
	seen   []LLMProviderOptions
	maxOut []*int64 // MaxOutputTokens of each call
}

func (m *optionsModel) record(call fantasy.Call) {
	m.mu.Lock()
	m.seen = append(m.seen, call.ProviderOptions)
	m.maxOut = append(m.maxOut, call.MaxOutputTokens)
	m.mu.Unlock()
}

func (m *optionsModel) Generate(ctx context.Context, call fantasy.Call) (*fantasy.Response, error) {
	m.record(call)
	return m.echoModel.Generate(ctx, call)
}

func (m *optionsModel) Stream(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	m.record(call)
	return m.echoModel.Stream(ctx, call)
}

func (m *optionsModel) lastMaxOut() (*int64, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.maxOut) == 0 {
		return nil, false
	}
	return m.maxOut[len(m.maxOut)-1], true
}

func (m *optionsModel) last() LLMProviderOptions {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.seen) == 0 {
		return nil
	}
	return m.seen[len(m.seen)-1]
}

// Regression test for the CodeRabbit finding on PR #143: when
// ExecuteCompletion reuses an active factory model, the completion call must
// carry the options that the factory returned.
func TestProviders_ExecuteCompletionReusesFactoryProviderOptions(t *testing.T) {
	model := &optionsModel{}
	model.provider, model.model = "local", "main"
	k := newProviderTestKit(t, &Options{
		Model: "local/main",
		Providers: map[string]ProviderFactory{
			"local": func(context.Context, *ProviderConfig, string) (*ProviderResult, error) {
				return &ProviderResult{
					Model:           model,
					ProviderOptions: LLMProviderOptions{"local": &testProviderOption{}},
				}, nil
			},
		},
	})

	if _, err := k.ExecuteCompletion(context.Background(), CompleteRequest{Prompt: "x"}); err != nil {
		t.Fatalf("ExecuteCompletion: %v", err)
	}
	if _, ok := model.last()["local"]; !ok {
		t.Errorf("completion provider options = %v, want the factory's options", model.last())
	}
}

// The factory-backed flag follows the active model, so built-in models keep
// nil provider options in ExecuteCompletion.
func TestProviders_ActiveModelFromFactoryTracksModel(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "sk-test")
	k := newProviderTestKit(t, &Options{
		Model:     "openai/gpt-4o-mini",
		Providers: map[string]ProviderFactory{"local": (&recordingFactory{}).factory("local")},
	})
	if k.activeModelFromFactory {
		t.Fatal("built-in model must not be marked as factory-backed")
	}
	if err := k.SetModel(context.Background(), "local/m"); err != nil {
		t.Fatalf("SetModel local: %v", err)
	}
	if !k.activeModelFromFactory {
		t.Error("factory model must be marked as factory-backed")
	}
	if err := k.SetModel(context.Background(), "openai/gpt-4o-mini"); err != nil {
		t.Fatalf("SetModel openai: %v", err)
	}
	if k.activeModelFromFactory {
		t.Error("flag must clear after switching back to a built-in model")
	}
}

// Regression test for the CodeRabbit finding on PR #143: ExecuteCompletion
// must not send max_output_tokens when the provider result sets
// SkipMaxOutputTokens, both when it reuses the active model and when it
// builds a temporary one.
func TestProviders_ExecuteCompletionHonorsSkipMaxOutputTokens(t *testing.T) {
	models := map[string]*optionsModel{}
	var mu sync.Mutex
	factory := func(_ context.Context, _ *ProviderConfig, modelName string) (*ProviderResult, error) {
		mu.Lock()
		defer mu.Unlock()
		m, ok := models[modelName]
		if !ok {
			m = &optionsModel{}
			m.provider, m.model = "local", modelName
			models[modelName] = m
		}
		return &ProviderResult{Model: m, SkipMaxOutputTokens: true}, nil
	}
	k := newProviderTestKit(t, &Options{
		Model:     "local/main",
		Providers: map[string]ProviderFactory{"local": factory},
	})

	for _, tc := range []struct{ name, reqModel, modelName string }{
		{"reuse active model", "", "main"},
		{"temporary model", "local/side", "side"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := k.ExecuteCompletion(context.Background(), CompleteRequest{
				Model: tc.reqModel, Prompt: "x", MaxTokens: 50,
			}); err != nil {
				t.Fatalf("ExecuteCompletion: %v", err)
			}
			mu.Lock()
			m := models[tc.modelName]
			mu.Unlock()
			if m == nil {
				t.Fatalf("model %q was not built", tc.modelName)
			}
			got, called := m.lastMaxOut()
			if !called {
				t.Fatalf("model %q was not called", tc.modelName)
			}
			if got != nil {
				t.Errorf("MaxOutputTokens = %d, want unset", *got)
			}
		})
	}
}

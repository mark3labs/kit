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

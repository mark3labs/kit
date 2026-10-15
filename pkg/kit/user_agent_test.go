package kit

import (
	"context"
	"testing"
)

func TestUserAgentPropagation(t *testing.T) {
	t.Setenv("KIT_INTERNAL_USER_AGENT", "environment-must-not-win")
	for _, identity := range []string{"", "App/1.0"} {
		t.Run(identity, func(t *testing.T) {
			want := identity
			if want == "" {
				want = "KIT"
			}
			calls := 0
			factory := func(_ context.Context, cfg *ProviderConfig, name string) (*ProviderResult, error) {
				calls++
				if cfg.UserAgent != want {
					t.Errorf("identity = %q, want %q", cfg.UserAgent, want)
				}
				return &ProviderResult{Model: &echoModel{provider: "local", model: name}}, nil
			}
			opts := &Options{Model: "local/main", UserAgent: identity, Providers: map[string]ProviderFactory{"local": factory, "other": factory}}
			k := newProviderTestKit(t, opts)
			opts.UserAgent = "mutated"
			if err := k.SetModel(t.Context(), "other/next"); err != nil {
				t.Fatal(err)
			}
			if _, err := k.ExecuteCompletion(t.Context(), CompleteRequest{Model: "local/temp", Prompt: "hi"}); err != nil {
				t.Fatal(err)
			}
			child := &Options{}
			inheritProviderConfig(child, k.v)
			if child.UserAgent != want {
				t.Fatalf("child identity = %q", child.UserAgent)
			}
			if _, err := k.Subagent(t.Context(), SubagentConfig{Prompt: "hi", Model: "other/child", Tools: []Tool{}, NoSession: true}); err != nil {
				t.Fatal(err)
			}
			if calls != 4 {
				t.Fatalf("factory calls = %d", calls)
			}
		})
	}
}

func TestUserAgentValidationAndOptionOrder(t *testing.T) {
	opts := &Options{}
	WithUserAgent("first")(opts)
	WithUserAgent("second")(opts)
	if opts.UserAgent != "second" {
		t.Fatal("last option must win")
	}
	for _, value := range []string{"bad\r\nvalue", "bad\x00", "bad\x7f"} {
		if k, err := New(t.Context(), &Options{UserAgent: value, Model: "not-a-model"}); err == nil {
			_ = k.Close()
			t.Fatalf("accepted %q", value)
		}
	}
}

func TestConcurrentInstanceIdentity(t *testing.T) {
	for _, identity := range []string{"InstanceA/1", "InstanceB/1"} {
		t.Run(identity, func(t *testing.T) {
			t.Parallel()
			factory := func(_ context.Context, cfg *ProviderConfig, name string) (*ProviderResult, error) {
				if cfg.UserAgent != identity {
					t.Errorf("identity = %q, want %q", cfg.UserAgent, identity)
				}
				return &ProviderResult{Model: &echoModel{provider: "local", model: name}}, nil
			}
			k, err := NewIsolatedAgent(t.Context(), WithModel("local/test"), WithProvider("local", factory), WithUserAgent("overridden"), WithUserAgent(identity))
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = k.Close() }()
			if _, err := k.ExecuteCompletion(t.Context(), CompleteRequest{Model: "local/temp", Prompt: "hi"}); err != nil {
				t.Fatal(err)
			}
			if _, err := k.Subagent(t.Context(), SubagentConfig{Prompt: "hi", NoSession: true, Tools: []Tool{}}); err != nil {
				t.Fatal(err)
			}
		})
	}
}

package kit

import (
	"context"
	"fmt"
	"testing"

	"charm.land/fantasy"
)

func TestStepContextChain(t *testing.T) {
	k := &Kit{stepContext: newHookRegistry[StepContextHook, context.Context]()}
	type key int
	var order []int
	remove := k.OnStepContext(HookPriorityLow, func(h StepContextHook) context.Context {
		order = append(order, 3)
		if h.Context.Value(key(1)) != "first" {
			t.Fatal("context did not chain")
		}
		return context.WithValue(h.Context, key(2), "last")
	})
	k.OnStepContext(HookPriorityHigh, func(h StepContextHook) context.Context {
		order = append(order, 1)
		return context.WithValue(h.Context, key(1), "first")
	})
	k.OnStepContext(HookPriorityNormal, func(h StepContextHook) context.Context {
		order = append(order, 2)
		if h.Context.Value(key(1)) != "first" {
			t.Fatal("nil hook lost context")
		}
		return nil
	})
	run := func() context.Context {
		return k.stepContext.chain(StepContextHook{Context: t.Context()}, func(h StepContextHook, ctx context.Context) StepContextHook { h.Context = ctx; return h }).Context
	}
	if got := run().Value(key(2)); got != "last" {
		t.Fatalf("last = %v", got)
	}
	if fmt.Sprint(order) != "[1 2 3]" {
		t.Fatalf("order = %v", order)
	}
	remove()
	if run().Value(key(2)) != nil {
		t.Fatal("unregister failed")
	}
}

type contextRecordingModel struct {
	echoModel
	check func(context.Context)
}

func (m *contextRecordingModel) Stream(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	m.check(ctx)
	return m.echoModel.Stream(ctx, call)
}

func TestStepContextAndEventModel(t *testing.T) {
	type key struct{}
	seen := false
	factory := func(_ context.Context, _ *ProviderConfig, name string) (*ProviderResult, error) {
		base := echoModel{provider: "local", model: name}
		return &ProviderResult{Model: &contextRecordingModel{echoModel: base, check: func(ctx context.Context) {
			seen = true
			if ctx.Value(key{}) != "step" {
				t.Errorf("provider context = %v", ctx.Value(key{}))
			}
		}}}, nil
	}
	k := newProviderTestKit(t, &Options{Model: "local/old", Providers: map[string]ProviderFactory{"local": factory}})
	k.OnStepContext(HookPriorityNormal, func(h StepContextHook) context.Context {
		if h.Model != "local/old" || h.StepNumber != 0 {
			t.Errorf("hook = %+v", h)
		}
		return context.WithValue(h.Context, key{}, "step")
	})
	var starts, finishes int
	k.Subscribe(func(e Event) {
		switch e := e.(type) {
		case StepStartEvent:
			starts++
			if e.Model != "local/old" {
				t.Errorf("start model = %s", e.Model)
			}
			if err := k.SetModel(t.Context(), "local/new"); err != nil {
				t.Error(err)
			}
		case StepFinishEvent:
			finishes++
			if e.Model != "local/old" {
				t.Errorf("finish model = %s", e.Model)
			}
		}
	})
	if _, err := k.Prompt(t.Context(), "hello"); err != nil {
		t.Fatal(err)
	}
	if !seen || starts != 1 || finishes != 1 {
		t.Fatalf("seen=%v starts=%d finishes=%d", seen, starts, finishes)
	}
}

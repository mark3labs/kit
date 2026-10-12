package kit

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/kit/internal/agent"
	"github.com/mark3labs/kit/internal/kitsetup"
	"github.com/mark3labs/kit/internal/models"
)

const modelModesTestModel = "openai/gpt-4o"

func modelModesTestKit(t *testing.T, mode string) *Kit {
	t.Helper()
	if models.LookupModelForSettings(modelModesTestModel) == nil {
		t.Fatalf("model %q is not in the embedded catalog", modelModesTestModel)
	}
	factory := (&recordingFactory{}).factory("openai")
	k := newProviderTestKit(t, &Options{
		Model: modelModesTestModel, ModelMode: mode,
		Providers: map[string]ProviderFactory{"openai": factory},
	})
	return k
}

func TestWithModelModeOverridesStartupConfig(t *testing.T) {
	v := newConfigStore(&Options{SkipConfig: true})
	applyOptionOverrides(v, &Options{Model: modelModesTestModel, ModelMode: "fast"})
	cfg, _, err := kitsetup.BuildProviderConfig(v)
	if err != nil {
		t.Fatalf("BuildProviderConfig: %v", err)
	}
	if got := cfg.ModelMode; got != "fast" {
		t.Fatalf("provider config model_mode = %q, want fast", got)
	}
}

func TestWithModelModeRejectsUnsupportedAtStartup(t *testing.T) {
	_, err := New(context.Background(), &Options{
		Model: modelModesTestModel, ModelMode: "not-a-real-mode", Quiet: true,
		NoSession: true, NoExtensions: true, DisableCoreTools: true, SkipConfig: true,
		Providers: map[string]ProviderFactory{"openai": (&recordingFactory{}).factory("openai")},
	})
	if !errors.Is(err, ErrUnsupportedModelMode) {
		t.Fatalf("New error = %v, want ErrUnsupportedModelMode", err)
	}
}

func TestSetModelModeRejectsActiveTurn(t *testing.T) {
	k := modelModesTestKit(t, "")
	k.steerMu.Lock()
	k.steerCh = make(chan agent.SteerMessage, 1)
	k.steerMu.Unlock()
	err := k.SetModelMode(context.Background(), "fast")
	if err == nil || !strings.Contains(err.Error(), "active turn") {
		t.Fatalf("SetModelMode error = %v, want active-turn error", err)
	}
	if got := k.CurrentModelMode(); got != "" {
		t.Fatalf("mode changed to %q after rejection", got)
	}
	k.steerMu.Lock()
	k.steerCh = nil
	k.steerMu.Unlock()
}

func TestSetModelModeUnknownDoesNotChangeCurrent(t *testing.T) {
	k := modelModesTestKit(t, "")
	k.modelMode = "previous"
	err := k.SetModelMode(context.Background(), "not-a-real-mode")
	if !errors.Is(err, ErrUnsupportedModelMode) {
		t.Fatalf("SetModelMode error = %v, want ErrUnsupportedModelMode", err)
	}
	if got := k.CurrentModelMode(); got != "previous" {
		t.Fatalf("CurrentModelMode() = %q after failure, want previous", got)
	}
}

func TestModelModesSupportedFilter(t *testing.T) {
	k := modelModesTestKit(t, "")
	all := k.ModelModes()
	filtered := k.SupportedModelModes()
	var want []ModelMode
	for _, mode := range all {
		if mode.Supported {
			want = append(want, mode)
		}
	}
	if len(filtered) != len(want) {
		t.Fatalf("SupportedModelModes length = %d, want %d", len(filtered), len(want))
	}
	for i := range want {
		if filtered[i] != want[i] {
			t.Errorf("supported mode[%d] = %#v, want %#v", i, filtered[i], want[i])
		}
	}
}

func TestSetModelModeFailsWithoutDeadlockForCustomFactory(t *testing.T) {
	k := modelModesTestKit(t, "")
	done := make(chan error, 1)
	go func() { done <- k.SetModelMode(context.Background(), "fast") }()
	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "custom provider factory") {
			t.Fatalf("SetModelMode error = %v, want custom-factory error", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("SetModelMode deadlocked")
	}
	if got := k.CurrentModelMode(); got != "" {
		t.Fatalf("CurrentModelMode() = %q after failed rebuild, want empty", got)
	}
}

func TestSetModelModeRebuildsSameModelToDefault(t *testing.T) {
	factory := &recordingFactory{}
	k := newProviderTestKit(t, &Options{Model: modelModesTestModel, Providers: map[string]ProviderFactory{"openai": factory.factory("openai")}})
	before, _ := factory.snapshot()
	k.modelMode = "previous"
	if err := k.SetModelMode(context.Background(), ""); err != nil {
		t.Fatalf("SetModelMode: %v", err)
	}
	if got := k.CurrentModelMode(); got != "" {
		t.Fatalf("CurrentModelMode() = %q, want default empty mode", got)
	}
	after, _ := factory.snapshot()
	if len(after) <= len(before) {
		t.Fatalf("provider factory calls = %d, want rebuild after %d initial calls", len(after), len(before))
	}
}

package kit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"charm.land/fantasy"
)

// activationModel exercises the real public provider/agent boundary, including
// the actual tool result delivered in the next model request.
type activationModel struct {
	echoModel
	calls     []fantasy.Call
	script    []string
	skillName string
}

func (m *activationModel) Stream(_ context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	i := len(m.calls)
	m.calls = append(m.calls, call)
	return iter.Seq[fantasy.StreamPart](func(yield func(fantasy.StreamPart) bool) {
		var parts []fantasy.StreamPart
		if i < len(m.script) {
			name := m.script[i]
			input := "{}"
			if name == "activate_skill" {
				name := m.skillName
				if name == "" {
					name = "workspace/root/demo--0123456789abcdef"
				}
				input = fmt.Sprintf(`{"name":%q}`, name)
			}
			id := fmt.Sprint(i)
			parts = []fantasy.StreamPart{
				{Type: fantasy.StreamPartTypeToolInputStart, ID: id, ToolCallName: name},
				{Type: fantasy.StreamPartTypeToolInputDelta, ID: id, Delta: input},
				{Type: fantasy.StreamPartTypeToolInputEnd, ID: id},
				{Type: fantasy.StreamPartTypeToolCall, ID: id, ToolCallName: name, ToolCallInput: input},
				{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonToolCalls},
			}
		} else {
			parts = []fantasy.StreamPart{{Type: fantasy.StreamPartTypeTextStart, ID: "t"}, {Type: fantasy.StreamPartTypeTextDelta, ID: "t", Delta: "done"}, {Type: fantasy.StreamPartTypeTextEnd, ID: "t"}, {Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonStop}}
		}
		for _, p := range parts {
			if !yield(p) {
				return
			}
		}
	}), nil
}

func activationHost(t *testing.T, model *activationModel, paths ...string) *Kit {
	t.Helper()
	advance := NewTool("advance", "Advance.", func(context.Context, struct{}) (ToolOutput, error) { return ToolOutput{Content: "advanced"}, nil })
	return newProviderTestKit(t, &Options{Model: "offline/m", NoSkills: len(paths) == 0, Skills: paths, NoAgents: true, NoContextFiles: true, ExtraTools: []Tool{advance}, Providers: map[string]ProviderFactory{"offline": func(context.Context, *ProviderConfig, string) (*ProviderResult, error) {
		return &ProviderResult{Model: model}, nil
	}}})
}

func callJSON(t *testing.T, call fantasy.Call) string {
	t.Helper()
	data, err := json.Marshal(call.Prompt)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestRuntimeActivationPublicLoop(t *testing.T) {
	for _, startup := range []bool{false, true} {
		for _, location := range []string{"absent", "missing", "conflicting"} {
			t.Run(fmt.Sprintf("startup=%v/%s", startup, location), func(t *testing.T) {
				dir := t.TempDir()
				var paths []string
				if startup {
					p := filepath.Join(dir, "seed.md")
					if err := os.WriteFile(p, []byte("---\nname: seed\ndescription: seed metadata\n---\nseed body"), 0600); err != nil {
						t.Fatal(err)
					}
					paths = []string{p}
				}
				path := ""
				if location != "absent" {
					path = filepath.Join(dir, "saved.md")
				}
				if location == "conflicting" {
					if err := os.WriteFile(path, []byte("WRONG DISK BODY"), 0600); err != nil {
						t.Fatal(err)
					}
				}
				model := &activationModel{script: []string{"advance", "activate_skill", "activate_skill", "activate_skill"}}
				host := activationHost(t, model, paths...)
				const name = "workspace/root/demo--0123456789abcdef"
				host.OnPrepareStep(HookPriorityNormal, func(h PrepareStepHook) *PrepareStepResult {
					if h.Context == nil {
						t.Error("missing preparation context")
					}
					if h.StepNumber == 1 || h.StepNumber == 2 {
						body := "saved v1"
						if h.StepNumber == 2 {
							body = "saved v2"
						}
						if err := host.AddSkill(&Skill{Name: name, Description: "selected metadata", Content: body, Path: path, NoHostIO: true, Metadata: map[string]string{"source": "saved"}}); err != nil {
							t.Fatal(err)
						}
					}
					return nil
				})
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				if _, err := host.PromptResult(ctx, "start"); err != nil {
					t.Fatal(err)
				}
				if len(model.calls) != 5 {
					t.Fatalf("calls=%d", len(model.calls))
				}
				for _, step := range []int{1, 2, 3} {
					count := 0
					for _, tool := range model.calls[step].Tools {
						if tool.GetName() == "activate_skill" {
							count++
							b, _ := json.Marshal(tool)
							if !strings.Contains(string(b), name) {
								t.Errorf("schema=%s", b)
							}
						}
					}
					if count != 1 {
						t.Errorf("step %d activation count=%d", step, count)
					}
					if !strings.Contains(callJSON(t, model.calls[step]), "selected metadata") {
						t.Errorf("missing metadata at step %d", step)
					}
				}
				if !strings.Contains(callJSON(t, model.calls[2]), "saved v1") || !strings.Contains(callJSON(t, model.calls[3]), "saved v2") || !strings.Contains(callJSON(t, model.calls[4]), "already loaded") {
					t.Fatal("missing selected content or dedup acknowledgement")
				}
				if strings.Contains(callJSON(t, model.calls[4]), "WRONG DISK BODY") {
					t.Fatal("activation read disk")
				}
			})
		}
	}
}

func TestPrepareFailurePublicLoop(t *testing.T) {
	sentinel := errors.New("durable preparation failed: HTTP 503 image too large context length exceeded")
	for _, step := range []int{0, 1} {
		t.Run(fmt.Sprint(step), func(t *testing.T) {
			model := &activationModel{script: []string{"advance"}}
			host := activationHost(t, model)
			later := false
			host.OnPrepareStep(HookPriorityHigh, func(h PrepareStepHook) *PrepareStepResult {
				if h.StepNumber == step {
					return &PrepareStepResult{Error: sentinel, Messages: []LLMMessage{}, Tools: []Tool{}}
				}
				return nil
			})
			host.OnPrepareStep(HookPriorityNormal, func(h PrepareStepHook) *PrepareStepResult {
				if h.StepNumber == step {
					later = true
				}
				return nil
			})
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			result, err := host.PromptResult(ctx, "start")
			if !errors.Is(err, sentinel) {
				t.Fatalf("error=%v", err)
			}
			if len(model.calls) != step || later {
				t.Fatalf("calls=%d later hook=%v", len(model.calls), later)
			}
			if step == 1 && (result == nil || !strings.Contains(fmt.Sprint(result.Messages), "advanced")) {
				t.Fatalf("missing completed tool result: %+v", result)
			}
		})
	}
}

func TestPrepareCancellationPublicLoop(t *testing.T) {
	model := &activationModel{}
	host := activationHost(t, model)
	entered := make(chan struct{})
	exited := make(chan struct{})
	host.OnPrepareStep(HookPriorityNormal, func(h PrepareStepHook) *PrepareStepResult {
		close(entered)
		<-h.Context.Done()
		close(exited)
		return nil
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := host.PromptResult(ctx, "start"); done <- err }()
	select {
	case <-entered:
		cancel()
	case <-ctx.Done():
		t.Fatal("hook did not start")
	}
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error=%v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("prompt did not stop")
	}
	<-exited
	if len(model.calls) != 0 {
		t.Fatal("provider called after cancellation")
	}
}

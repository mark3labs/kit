package kit

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkspaceSourceIsMetadata(t *testing.T) {
	for _, state := range []string{"changed", "deleted"} {
		t.Run(state, func(t *testing.T) {
			source := filepath.Join(t.TempDir(), "SKILL.md")
			if err := os.WriteFile(source, []byte("original disk bytes"), 0600); err != nil {
				t.Fatal(err)
			}
			model := &activationModel{skillName: workspaceName, script: []string{"activate_skill"}}
			host := workspaceHost(t, model)
			skill := &Skill{Name: workspaceName, Description: "workspace metadata", Content: "REGISTERED BODY", Path: source, NoHostIO: true}
			if err := host.AddSkill(skill); err != nil {
				t.Fatal(err)
			}
			skill.Content = "CALLER MUTATION"
			if state == "deleted" {
				if err := os.Remove(source); err != nil {
					t.Fatal(err)
				}
			} else if err := os.WriteFile(source, []byte("CHANGED DISK BODY"), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := host.PromptResult(context.Background(), "start"); err != nil {
				t.Fatal(err)
			}
			text := callJSON(t, model.calls[1])
			if !strings.Contains(text, "REGISTERED BODY") || strings.Contains(text, "CHANGED DISK BODY") || strings.Contains(text, "CALLER MUTATION") {
				t.Fatalf("not immutable: %s", text)
			}
		})
	}
}

func TestWorkspacePreparationAbort(t *testing.T) {
	for _, cancellation := range []bool{false, true} {
		t.Run(map[bool]string{false: "error", true: "cancellation"}[cancellation], func(t *testing.T) {
			model := &activationModel{}
			host := workspaceHost(t, model)
			type requestKey struct{}
			ctx, cancel := context.WithCancel(context.WithValue(context.Background(), requestKey{}, "request"))
			defer cancel()
			sentinel := errors.New("durable append failed: context length exceeded")
			expected := sentinel
			if cancellation {
				expected = context.Canceled
			}
			host.OnPrepareStep(HookPriorityNormal, func(h PrepareStepHook) *PrepareStepResult {
				if h.Context == nil || h.Context.Value(requestKey{}) != "request" {
					t.Error("request context not preserved")
				}
				if cancellation {
					cancel()
					return nil
				}
				return &PrepareStepResult{Error: sentinel}
			})
			_, err := host.PromptResult(ctx, "start")
			if !errors.Is(err, expected) || len(model.calls) != 0 {
				t.Fatalf("error=%v requests=%d", err, len(model.calls))
			}
		})
	}
}

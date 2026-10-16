package kit

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"charm.land/fantasy"
)

const workspaceName = "workspace/root/demo--0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func workspaceHost(t *testing.T, model *activationModel, extra ...Tool) *Kit {
	t.Helper()
	k, err := NewAgent(context.Background(), Isolated(), WithSkillActivation(), WithModel("offline/m"), WithExtraTools(extra...), func(o *Options) {
		o.Quiet = true
		o.Providers = map[string]ProviderFactory{"offline": func(context.Context, *ProviderConfig, string) (*ProviderResult, error) {
			return &ProviderResult{Model: model}, nil
		}}
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = k.Close() })
	return k
}

func TestWorkspaceActivationPublicLoop(t *testing.T) {
	for _, symlink := range []bool{false, true} {
		t.Run(map[bool]string{false: "directories", true: "symlinks"}[symlink], func(t *testing.T) {
			base := t.TempDir()
			for _, sub := range []string{"assets", "scripts", "references"} {
				target := filepath.Join(base, sub)
				if symlink {
					target = t.TempDir()
					if err := os.Symlink(target, filepath.Join(base, sub)); err != nil {
						t.Fatal(err)
					}
				} else if err := os.Mkdir(target, 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(target, "HOST_ONLY_RESOURCE"), []byte("host"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			// The source never exists. Resource directories alone must not authorize I/O.
			source := filepath.Join(base, "SKILL.md")
			model := &activationModel{skillName: workspaceName, script: []string{"activate_skill", "activate_skill", "activate_skill", "activate_skill", "activate_skill"}}
			host := workspaceHost(t, model)
			if len(host.GetSkills()) != 0 {
				t.Fatal("discovery enabled")
			}
			host.OnPrepareStep(HookPriorityNormal, func(h PrepareStepHook) *PrepareStepResult {
				switch h.StepNumber {
				case 1, 2:
					version := "v1"
					if h.StepNumber == 2 {
						version = "v2"
					}
					if err := host.SetSkills([]*Skill{{Name: workspaceName, Description: "metadata-" + version, Content: "BODY-" + version, Path: source, NoHostIO: true}}); err != nil {
						t.Fatal(err)
					}
				case 3:
					if !host.RemoveSkill(workspaceName) {
						t.Fatal("remove failed")
					}
				}
				return nil
			})
			if _, err := host.PromptResult(context.Background(), "preserve user input"); err != nil {
				t.Fatal(err)
			}
			if len(model.calls) != 6 {
				t.Fatalf("calls=%d", len(model.calls))
			}
			for i, call := range model.calls {
				count := 0
				for _, tool := range call.Tools {
					if tool.GetName() != "activate_skill" {
						continue
					}
					count++
					raw, err := json.Marshal(tool)
					if err != nil {
						t.Fatal(err)
					}
					schema := string(raw)
					if i == 1 || i == 2 {
						if !strings.Contains(schema, workspaceName) {
							t.Fatalf("missing qualified enum: %s", schema)
						}
					} else if strings.Contains(schema, `"enum"`) || strings.Contains(schema, workspaceName) {
						t.Fatalf("stale/empty enum: %s", schema)
					}
				}
				if count != 1 {
					t.Fatalf("step %d activation count=%d", i, count)
				}
				text := callJSON(t, call)
				if strings.Contains(text, "HOST_ONLY_RESOURCE") {
					t.Fatal("host resources leaked")
				}
				if !strings.Contains(text, "preserve user input") {
					t.Fatal("messages lost")
				}
			}
			if strings.Contains(callJSON(t, model.calls[1]), "BODY-") {
				t.Fatal("eager body exposure")
			}
			if !strings.Contains(callJSON(t, model.calls[1]), "unknown skill") {
				t.Fatal("empty activation succeeded")
			}
			if !strings.Contains(callJSON(t, model.calls[2]), "BODY-v1") || !strings.Contains(callJSON(t, model.calls[3]), "BODY-v2") {
				t.Fatal("version activation failed")
			}
			if !strings.Contains(callJSON(t, model.calls[2]), "metadata-v2") || strings.Contains(callJSON(t, model.calls[3]), "metadata-v2") {
				t.Fatal("metadata not refreshed")
			}
			for _, step := range []int{4, 5} {
				if strings.Contains(callJSON(t, model.calls[step]), "already loaded") {
					t.Fatal("removed activation deduplicated")
				}
			}
			final := callJSON(t, model.calls[5])
			if !strings.Contains(final, source) || !strings.Contains(final, base) {
				t.Fatal("source/base lost")
			}
		})
	}
}

func TestWorkspaceSavedContentAndCancellation(t *testing.T) {
	// JSON is a durable public representation; no original files or Kit state.
	saved, err := json.Marshal(&Skill{Name: workspaceName, Path: "remote/skills/demo/SKILL.md", Content: "IMMUTABLE BODY\nversion: durable-version\nscope: root", Description: "saved metadata", NoHostIO: true, Metadata: map[string]string{"sha256": "durable-version", "scope": "root"}})
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		var skill Skill
		if err := json.Unmarshal(saved, &skill); err != nil {
			t.Fatal(err)
		}
		model := &activationModel{skillName: workspaceName, script: []string{"activate_skill"}}
		host := workspaceHost(t, model)
		if err := host.AddSkill(&skill); err != nil {
			t.Fatal(err)
		}
		if _, err := host.PromptResult(context.Background(), "start"); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(callJSON(t, model.calls[0]), "IMMUTABLE BODY") || !strings.Contains(callJSON(t, model.calls[1]), "IMMUTABLE BODY") {
			t.Fatal("saved activation failed")
		}
		if !strings.Contains(callJSON(t, model.calls[1]), "version: durable-version") || !strings.Contains(callJSON(t, model.calls[1]), "scope: root") {
			t.Fatal("composed durable annotations missing")
		}
		if host.GetSkills()[0].Metadata["sha256"] != "durable-version" {
			t.Fatal("metadata lost")
		}
	}
	host := workspaceHost(t, &activationModel{})
	if err := host.AddSkill(&Skill{Name: workspaceName, Content: "cancel body", NoHostIO: true}); err != nil {
		t.Fatal(err)
	}
	// Use public GetTools to capture a version, then invalidate it.
	var tool Tool
	for _, candidate := range host.GetTools() {
		if candidate.Info().Name == "activate_skill" {
			tool = candidate
		}
	}
	if tool == nil {
		t.Fatal("missing activation")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	input := fantasy.ToolCall{Input: `{"name":"` + workspaceName + `"}`}
	if _, err := tool.Run(ctx, input); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel error=%v", err)
	}
	response, err := tool.Run(context.Background(), input)
	if err != nil || !strings.Contains(response.Content, "cancel body") {
		t.Fatalf("aborted activation recorded: %+v %v", response, err)
	}
	if err := host.AddSkill(&Skill{Name: workspaceName, Content: "new body", NoHostIO: true}); err != nil {
		t.Fatal(err)
	}
	response, err = tool.Run(context.Background(), input)
	if err != nil || !strings.Contains(response.Content, "changed") || strings.Contains(response.Content, "new body") {
		t.Fatalf("stale tool selected newer body: %+v %v", response, err)
	}
}

func TestWorkspaceActivationCallerConflict(t *testing.T) {
	custom := NewTool("activate_skill", "caller owned", func(context.Context, struct{}) (ToolOutput, error) {
		return ToolOutput{Content: "caller response"}, nil
	})
	model := &activationModel{script: []string{"activate_skill"}}
	host := workspaceHost(t, model, custom)
	if _, err := host.PromptResult(context.Background(), "start"); err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, tool := range model.calls[0].Tools {
		if tool.GetName() == "activate_skill" {
			count++
		}
	}
	if count != 1 || !strings.Contains(callJSON(t, model.calls[1]), "caller response") {
		t.Fatal("caller tool replaced or duplicated")
	}
}

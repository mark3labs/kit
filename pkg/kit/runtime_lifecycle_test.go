package kit

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"charm.land/fantasy"
)

func TestRuntimeCatalogueLifecyclePublicLoop(t *testing.T) {
	path := filepath.Join(t.TempDir(), "disk.md")
	write := func(body string) {
		t.Helper()
		if err := os.WriteFile(path, []byte("---\nname: disk\ndescription: disk metadata\n---\n"+body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write("disk v1")
	model := &activationModel{script: []string{"advance", "advance", "advance", "advance", "advance", "advance", "advance"}}
	host := activationHost(t, model, path)
	host.OnPrepareStep(HookPriorityNormal, func(h PrepareStepHook) *PrepareStepResult {
		switch h.StepNumber {
		case 1:
			if !host.DisableSkill("disk") {
				t.Error("disable failed")
			}
		case 2:
			if !host.EnableSkill("disk") {
				t.Error("enable failed")
			}
		case 3:
			if !host.RemoveSkill("disk") {
				t.Error("remove failed")
			}
		case 4:
			if err := host.SetSkills([]*Skill{{Name: "runtime", Description: "runtime metadata", Content: "saved"}}); err != nil {
				t.Fatal(err)
			}
		case 5:
			if err := host.SetSkills(nil); err != nil {
				t.Fatal(err)
			}
		case 6:
			write("disk v2")
			if err := host.ReloadSkills(); err != nil {
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
	want := []string{"disk", "", "disk", "", "runtime", "", "disk", "disk"}
	for i, name := range want {
		count := 0
		for _, tool := range model.calls[i].Tools {
			if tool.GetName() == "activate_skill" {
				count++
			}
		}
		if (name == "" && count != 0) || (name != "" && count != 1) {
			t.Errorf("step %d tool count=%d", i, count)
		}
		prompt := callJSON(t, model.calls[i])
		if name != "" && !strings.Contains(prompt, name+" metadata") {
			t.Errorf("step %d missing metadata", i)
		}
		if name == "" && (strings.Contains(prompt, "disk metadata") || strings.Contains(prompt, "runtime metadata")) {
			t.Errorf("step %d stale metadata", i)
		}
	}
	if host.GetSkills()[0].Content != "disk v2" {
		t.Fatal("reload did not select disk version")
	}
}

func TestPrepareOverridesWinOverLiveSnapshot(t *testing.T) {
	model := &activationModel{}
	host := activationHost(t, model)
	host.OnPrepareStep(HookPriorityNormal, func(h PrepareStepHook) *PrepareStepResult {
		if err := host.AddSkill(&Skill{Name: "demo", Description: "live metadata", Content: "body"}); err != nil {
			t.Fatal(err)
		}
		choice := LLMToolChoiceNone
		return &PrepareStepResult{Messages: []LLMMessage{fantasy.NewSystemMessage("explicit host system"), fantasy.NewUserMessage("explicit input")}, Tools: []Tool{}, ToolChoice: &choice}
	})
	if _, err := host.PromptResult(context.Background(), "start"); err != nil {
		t.Fatal(err)
	}
	if len(model.calls) != 1 || len(model.calls[0].Tools) != 0 {
		t.Fatal("empty override lost")
	}
	prompt := callJSON(t, model.calls[0])
	if !strings.Contains(prompt, "explicit host system") || strings.Contains(prompt, "live metadata") {
		t.Fatalf("override lost: %s", prompt)
	}
	if model.calls[0].ToolChoice == nil || *model.calls[0].ToolChoice != fantasy.ToolChoiceNone {
		t.Fatalf("choice=%v", model.calls[0].ToolChoice)
	}
}

func TestPrepareCancellationAtBoundary(t *testing.T) {
	for _, before := range []bool{true, false} {
		t.Run(fmt.Sprint(before), func(t *testing.T) {
			model := &activationModel{}
			host := activationHost(t, model)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			hooks := 0
			host.OnPrepareStep(HookPriorityNormal, func(PrepareStepHook) *PrepareStepResult { hooks++; cancel(); return nil })
			if before {
				cancel()
			}
			_, err := host.PromptResult(ctx, "start")
			if err == nil || len(model.calls) != 0 {
				t.Fatalf("err=%v calls=%d", err, len(model.calls))
			}
			if before && hooks != 0 {
				t.Fatal("hook called with cancelled context")
			}
		})
	}
}

package kit

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"

	"charm.land/fantasy"
)

func TestRegisteredSkillOwnership(t *testing.T) {
	host := activationHost(t, &activationModel{})
	input := &Skill{Name: "demo", Content: "saved", Description: "metadata", Metadata: map[string]string{"key": "saved"}, Tags: []string{"saved"}}
	if err := host.SetSkills([]*Skill{{Name: "demo", Content: "old"}, input}); err != nil {
		t.Fatal(err)
	}
	input.Content = "changed"
	input.Metadata["key"] = "changed"
	input.Tags[0] = "changed"
	snapshot := host.GetSkills()
	if len(snapshot) != 1 || snapshot[0].Content != "saved" || snapshot[0].Metadata["key"] != "saved" || snapshot[0].Tags[0] != "saved" {
		t.Fatalf("snapshot=%+v", snapshot)
	}
	snapshot[0].Content = "changed"
	snapshot[0].Metadata["key"] = "changed"
	snapshot[0].Tags[0] = "changed"
	if host.GetSkills()[0].Content != "saved" || host.GetSkills()[0].Metadata["key"] != "saved" || host.GetSkills()[0].Tags[0] != "saved" {
		t.Fatal("public read changed registry")
	}
	if err := host.SetSkills([]*Skill{{Name: "valid"}, nil}); err == nil || host.GetSkills()[0].Name != "demo" {
		t.Fatal("invalid batch changed registry")
	}
}

func TestCataloguePublicationConcurrent(t *testing.T) {
	host := activationHost(t, &activationModel{})
	var wg sync.WaitGroup
	for worker := range 4 {
		wg.Go(func() {
			for i := range 30 {
				name := fmt.Sprintf("catalogue-%d-%d", worker, i)
				if err := host.SetSkills([]*Skill{{Name: name, Description: name, Content: name}}); err != nil {
					t.Error(err)
				}
				prompt, tools := host.agentSnapshotForTest()
				for _, tool := range tools {
					if tool.Info().Name == "activate_skill" {
						params := tool.Info().Parameters["name"].(map[string]any)
						for _, entry := range params["enum"].([]any) {
							if !strings.Contains(prompt, entry.(string)) {
								t.Errorf("prompt/schema disagree: %s", entry)
							}
						}
						b, _ := json.Marshal(map[string]string{"name": name})
						if _, err := tool.Run(context.Background(), fantasy.ToolCall{Input: string(b)}); err != nil {
							t.Error(err)
						}
					}
				}
				_ = host.GetSkills()
			}
		})
	}
	wg.Wait()
}

func (m *Kit) agentSnapshotForTest() (string, []Tool) {
	m.catalogueMu.Lock()
	defer m.catalogueMu.Unlock()
	return m.agent.RequestSnapshot()
}

func TestManagedSkillToolNativeControl(t *testing.T) {
	for _, action := range []string{"remove", "replace", "set"} {
		t.Run(action, func(t *testing.T) {
			host := activationHost(t, &activationModel{})
			custom := NewTool("activate_skill", "custom", func(context.Context, struct{}) (ToolOutput, error) { return ToolOutput{Content: "custom"}, nil })
			switch action {
			case "remove":
				if err := host.RemoveTools("activate_skill"); err != nil {
					t.Fatal(err)
				}
			case "replace":
				host.AddTools(custom)
			case "set":
				host.SetExtraTools(custom)
			}
			if err := host.AddSkill(&Skill{Name: "demo", Content: "body", Description: "metadata"}); err != nil {
				t.Fatal(err)
			}
			_, tools := host.agentSnapshotForTest()
			count := 0
			for _, tool := range tools {
				if tool.Info().Name == "activate_skill" {
					count++
					if tool.Info().Description != "custom" {
						t.Fatal("managed tool resurrected")
					}
				}
			}
			expected := 1
			if action == "remove" {
				expected = 0
			}
			if count != expected {
				t.Fatalf("count=%d want=%d", count, expected)
			}
		})
	}
}

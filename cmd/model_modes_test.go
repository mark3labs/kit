package cmd

import "testing"

func TestModelModeOptionsAreModelSpecific(t *testing.T) {
	sol := modelModeOptions(nil, "openai/gpt-6.1-sol")
	if len(sol) != 3 {
		t.Fatalf("Sol modes = %#v, want 3 catalog modes", sol)
	}
	for _, mode := range sol {
		if mode.Supported != (mode.Name == "ultrafast") {
			t.Fatalf("unexpected Sol availability: %#v", mode)
		}
	}
	mini := modelModeOptions(nil, "openai/gpt-4.1-mini")
	if len(mini) != 1 || mini[0].Name != "fast" || !mini[0].Supported {
		t.Fatalf("mini modes = %#v", mini)
	}
	for _, model := range []string{"openai/gpt-4-turbo", "openai/unknown", "gpt-6.1-sol"} {
		if modes := modelModeOptions(nil, model); len(modes) != 0 {
			t.Fatalf("%s modes = %#v, want none", model, modes)
		}
	}
}

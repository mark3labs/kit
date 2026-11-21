package cmd

import "testing"

func TestModelModeOptionsAreModelSpecific(t *testing.T) {
	// The catalog can add a mode to a model, so the test does not count the
	// modes. It makes sure that the known modes are there.
	sol := modelModeOptions(nil, "openai/gpt-6.1-sol")
	found := map[string]bool{}
	for _, mode := range sol {
		found[mode.Name] = true
		if mode.Supported != (mode.Name == "ultrafast") {
			t.Fatalf("unexpected Sol availability: %#v", mode)
		}
	}
	for _, name := range []string{"fast", "pro", "ultrafast"} {
		if !found[name] {
			t.Fatalf("Sol modes = %#v, want the catalog mode %q", sol, name)
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

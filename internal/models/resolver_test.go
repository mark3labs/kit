package models

import "testing"

func TestResolverIsolatedAndUnknownLookupDoesNotDeclare(t *testing.T) {
	input := map[string]map[string]ModelInfo{"fake": {"echo": {ID: "echo", Name: "Echo", InputModalities: []string{"text"}}}}
	resolver := NewResolver(input, map[string]bool{"fake": true})
	input["fake"]["echo"] = ModelInfo{Name: "changed"}
	if got := resolver.Lookup("fake", "echo"); got.Name != "Echo" {
		t.Fatalf("lookup=%+v", got)
	}
	unknown := resolver.Lookup("fake", "unlisted")
	if unknown == nil || unknown.Limit.Context != 0 || unknown.Cost.Published || len(resolver.List()["fake"]) != 1 {
		t.Fatalf("unknown=%+v", unknown)
	}
	known := resolver.Lookup("fake", "echo")
	known.InputModalities[0] = "image"
	if resolver.Lookup("fake", "echo").InputModalities[0] != "text" {
		t.Fatal("lookup aliases resolver")
	}
	if GetGlobalRegistry().LookupModel("fake", "echo") != nil {
		t.Fatal("global catalog changed")
	}
}

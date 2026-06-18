package kit_test

import (
	"context"
	"testing"

	"charm.land/fantasy"

	"github.com/mark3labs/kit/pkg/kit"
)

func TestNewRawTool(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"city": map[string]any{"type": "string", "description": "City name"},
		},
		"required": []any{"city"},
	}

	var gotArgs map[string]any
	tool := kit.NewRawTool("get_weather", "Get weather", schema,
		func(ctx context.Context, args map[string]any) (kit.ToolOutput, error) {
			gotArgs = args
			return kit.TextResult("72F in " + args["city"].(string)), nil
		},
	)

	info := tool.Info()
	if info.Name != "get_weather" {
		t.Fatalf("name = %q", info.Name)
	}
	if info.Parameters["type"] != "object" {
		t.Fatalf("schema not propagated: %#v", info.Parameters)
	}
	if len(info.Required) != 1 || info.Required[0] != "city" {
		t.Fatalf("required not propagated: %#v", info.Required)
	}

	resp, err := tool.Run(context.Background(), fantasy.ToolCall{
		ID:    "call_1",
		Input: `{"city":"Boston"}`,
	})
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if resp.IsError {
		t.Fatalf("unexpected error response: %q", resp.Content)
	}
	if resp.Content != "72F in Boston" {
		t.Fatalf("content = %q", resp.Content)
	}
	if gotArgs["city"] != "Boston" {
		t.Fatalf("args not decoded: %#v", gotArgs)
	}
}

func TestNewRawToolInvalidArgs(t *testing.T) {
	tool := kit.NewRawTool("t", "d", nil,
		func(ctx context.Context, args map[string]any) (kit.ToolOutput, error) {
			t.Fatal("handler should not be called for invalid args")
			return kit.ToolOutput{}, nil
		},
	)
	resp, err := tool.Run(context.Background(), fantasy.ToolCall{ID: "x", Input: `not json`})
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if !resp.IsError {
		t.Fatalf("expected error response for invalid args")
	}
}

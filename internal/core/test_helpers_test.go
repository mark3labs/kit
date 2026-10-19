package core

import (
	"context"

	"charm.land/fantasy"

	"github.com/mark3labs/kit/internal/media"
)

// executeEdit is a local-filesystem test helper.
func executeEdit(ctx context.Context, call fantasy.ToolCall, workDir string) (fantasy.ToolResponse, error) {
	return executeEditFS(ctx, call, workDir, localFS{})
}

// executeRead is a local-filesystem test helper.
func executeRead(ctx context.Context, call fantasy.ToolCall, workDir string, imageLimits media.Limits) (fantasy.ToolResponse, error) {
	return executeReadFS(ctx, call, workDir, imageLimits, nil)
}

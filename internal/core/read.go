package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"charm.land/fantasy"

	"github.com/mark3labs/kit/internal/media"
)

type readArgs struct {
	Path   string `json:"path"`
	Offset int    `json:"offset,omitempty"`
	Limit  int    `json:"limit,omitempty"`
}

// NewReadTool creates the read core tool.
func NewReadTool(opts ...ToolOption) fantasy.AgentTool {
	cfg := ApplyOptions(opts)
	return &coreTool{
		info: fantasy.ToolInfo{
			Name:        "read",
			Description: "Read the contents of a file. Output is truncated to 2000 lines or 50KB. Use offset/limit for large files. Use offset to continue reading until complete. Image files (PNG, JPEG, GIF, WebP) are returned as viewable images; offset and limit do not apply to them.",
			Parameters: map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Path to the file to read (relative or absolute)",
				},
				"offset": map[string]any{
					"type":        "number",
					"description": "Line number to start reading from (1-indexed)",
				},
				"limit": map[string]any{
					"type":        "number",
					"description": "Maximum number of lines to read",
				},
			},
			Required: []string{"path"},
			Parallel: true,
		},
		handler: func(ctx context.Context, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return executeRead(ctx, call, cfg.WorkDir, cfg.ImageLimits)
		},
	}
}

func executeRead(ctx context.Context, call fantasy.ToolCall, workDir string, imageLimits media.Limits) (fantasy.ToolResponse, error) {
	if err := ctx.Err(); err != nil {
		return fantasy.ToolResponse{}, err
	}
	var args readArgs
	if err := parseArgs(call.Input, &args); err != nil {
		return fantasy.NewTextErrorResponse("path parameter is required"), nil
	}
	if args.Path == "" {
		return fantasy.NewTextErrorResponse("path parameter is required"), nil
	}

	absPath, err := resolvePathWithWorkDir(args.Path, workDir)
	if err != nil {
		return fantasy.NewTextErrorResponse(fmt.Sprintf("invalid path: %v", err)), nil
	}

	// Check if path is a directory
	info, err := os.Stat(absPath)
	if err != nil {
		return fantasy.NewTextErrorResponse(fmt.Sprintf("cannot access '%s': %v", args.Path, err)), nil
	}

	if info.IsDir() {
		return fantasy.NewTextErrorResponse(fmt.Sprintf("'%s' is a directory, not a file. Use the ls tool to list directory contents.", args.Path)), nil
	}

	// Reject an oversized file before it is loaded into memory. Text files
	// are truncated after the read, but a huge binary would be read whole.
	if media.IsImageExt(absPath) && info.Size() > media.IngestLimitBytes {
		return fantasy.NewTextErrorResponse(fmt.Sprintf(
			"'%s' is %d bytes, above the %d byte image limit",
			args.Path, info.Size(), media.IngestLimitBytes)), nil
	}

	content, err := os.ReadFile(absPath)
	if err != nil {
		return fantasy.NewTextErrorResponse(fmt.Sprintf("failed to read file: %v", err)), nil
	}

	if resp, ok := readImage(content, absPath, args.Path, imageLimits); ok {
		return resp, nil
	}

	lines := strings.Split(string(content), "\n")
	totalLines := len(lines)

	// Apply offset (1-indexed)
	offset := 0
	if args.Offset > 0 {
		offset = args.Offset - 1
		if offset >= totalLines {
			return fantasy.NewTextResponse(fmt.Sprintf("offset %d exceeds file length (%d lines)", args.Offset, totalLines)), nil
		}
		lines = lines[offset:]
	}

	// Apply limit
	maxLines := defaultMaxLines
	if args.Limit > 0 {
		maxLines = args.Limit
	}
	if len(lines) > maxLines {
		lines = lines[:maxLines]
	}

	// Number lines
	var result strings.Builder
	for i, line := range lines {
		lineNum := offset + i + 1
		fmt.Fprintf(&result, "%d: %s\n", lineNum, line)
	}

	output := result.String()
	tr := truncateHead(output, 0, defaultMaxBytes)

	// Add truncation notice
	if len(lines) < totalLines-offset {
		tr.Content += fmt.Sprintf("\n[showing lines %d-%d of %d total. Use offset=%d to continue reading]",
			offset+1, offset+len(lines), totalLines, offset+len(lines)+1)
	}

	return fantasy.NewTextResponse(tr.Content), nil
}

// readImage returns an image tool response for content when content is a
// supported image. The second return value reports whether content was an
// image at all; when it is false the caller should fall back to text.
//
// A decode or budget failure still returns ok=true, with an error response,
// because falling through to the text path would emit binary noise.
func readImage(content []byte, absPath, displayPath string, limits media.Limits) (fantasy.ToolResponse, bool) {
	if media.DetectMediaType(content, absPath) == "" {
		return fantasy.ToolResponse{}, false
	}
	res, err := media.Normalize(content, absPath, limits)
	if err != nil {
		return fantasy.NewTextErrorResponse(fmt.Sprintf("cannot read image '%s': %v", displayPath, err)), true
	}

	summary := fmt.Sprintf("Read image %s (%s, %dx%d, %d bytes)",
		displayPath, res.MediaType, res.Width, res.Height, len(res.Data))
	if res.Resized {
		summary += fmt.Sprintf(" [resized from %dx%d, %d bytes]",
			res.OriginalWidth, res.OriginalHeight, res.OriginalBytes)
	}

	resp := fantasy.NewImageResponse(res.Data, res.MediaType)
	resp.Content = summary
	return resp, true
}

// resolvePathWithWorkDir resolves a path to an absolute path relative to the
// given workDir. If workDir is empty, os.Getwd() is used.
func resolvePathWithWorkDir(path, workDir string) (string, error) {
	if filepath.IsAbs(path) {
		return filepath.Clean(path), nil
	}
	baseDir := workDir
	if baseDir == "" {
		var err error
		baseDir, err = os.Getwd()
		if err != nil {
			return "", fmt.Errorf("failed to get working directory: %w", err)
		}
	}
	return filepath.Clean(filepath.Join(baseDir, path)), nil
}

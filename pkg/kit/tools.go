package kit

import (
	"charm.land/fantasy"

	"github.com/mark3labs/kit/internal/core"
)

// Tool is the interface that all Kit tools implement.
type Tool = fantasy.AgentTool

// ToolOption configures tool behavior.
type ToolOption = core.ToolOption

// WithWorkDir sets the working directory for file-based tools.
// If empty, os.Getwd() is used at execution time.
var WithWorkDir = core.WithWorkDir

// --- Individual tool constructors ---

// NewReadTool creates a file-reading tool.
func NewReadTool(opts ...ToolOption) Tool { return core.NewReadTool(opts...) }

// NewWriteTool creates a file-writing tool.
func NewWriteTool(opts ...ToolOption) Tool { return core.NewWriteTool(opts...) }

// NewEditTool creates a surgical text-editing tool.
func NewEditTool(opts ...ToolOption) Tool { return core.NewEditTool(opts...) }

// NewBashTool creates a bash command execution tool.
func NewBashTool(opts ...ToolOption) Tool { return core.NewBashTool(opts...) }

// NewGrepTool creates a content search tool (uses ripgrep when available).
func NewGrepTool(opts ...ToolOption) Tool { return core.NewGrepTool(opts...) }

// NewFindTool creates a file search tool (uses fd when available).
func NewFindTool(opts ...ToolOption) Tool { return core.NewFindTool(opts...) }

// NewLsTool creates a directory listing tool.
func NewLsTool(opts ...ToolOption) Tool { return core.NewLsTool(opts...) }

// --- Tool bundles ---

// AllTools returns all available core tools.
func AllTools(opts ...ToolOption) []Tool { return core.AllTools(opts...) }

// CodingTools returns the default set of core tools for a coding agent:
// bash, read, write, edit.
func CodingTools(opts ...ToolOption) []Tool { return core.CodingTools(opts...) }

// ReadOnlyTools returns tools for read-only exploration:
// read, grep, find, ls.
func ReadOnlyTools(opts ...ToolOption) []Tool { return core.ReadOnlyTools(opts...) }

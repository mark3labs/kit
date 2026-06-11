package extensions

// ToolKind constants classify what a tool does, enabling UIs to render
// appropriate visualizations (e.g. diff view for edit tools, command+output
// for execute tools) and file trackers to identify which results contain
// modifications.
//
// This is the single source of truth for tool-kind classification; the
// pkg/kit SDK re-exports these constants.
const (
	ToolKindExecute  = "execute" // Shell execution (bash)
	ToolKindEdit     = "edit"    // File modification (edit, write)
	ToolKindRead     = "read"    // File reading (read, ls)
	ToolKindSearch   = "search"  // Content/file search (grep, find)
	ToolKindSubagent = "agent"   // Subagent spawning (subagent)
)

// coreToolKinds maps built-in tool names to their kind classification.
// MCP and extension tools without an entry default to ToolKindExecute.
var coreToolKinds = map[string]string{
	"bash":     ToolKindExecute,
	"edit":     ToolKindEdit,
	"write":    ToolKindEdit,
	"read":     ToolKindRead,
	"ls":       ToolKindRead,
	"grep":     ToolKindSearch,
	"find":     ToolKindSearch,
	"subagent": ToolKindSubagent,
}

// ToolKindFor returns the ToolKind for a given tool name, defaulting to
// ToolKindExecute for unknown tools (including MCP tools).
func ToolKindFor(toolName string) string {
	if kind, ok := coreToolKinds[toolName]; ok {
		return kind
	}
	return ToolKindExecute
}

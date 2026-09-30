package acpserver

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	acp "github.com/coder/acp-go-sdk"

	kit "github.com/mark3labs/kit/pkg/kit"
)

// acpToolKind maps a Kit tool name to an ACP tool kind. Clients use the kind
// to choose icons and renderers. MCP and extension tools map to "other"
// because Kit cannot know what they do.
func acpToolKind(toolName string) acp.ToolKind {
	switch toolName {
	case "shell", "bash":
		return acp.ToolKindExecute
	case "edit", "write":
		return acp.ToolKindEdit
	case "read", "ls":
		return acp.ToolKindRead
	case "grep", "find":
		return acp.ToolKindSearch
	}
	return acp.ToolKindOther
}

// toolTitle returns a short human-readable title for a tool call, such as
// "read main.go" or "shell: go test ./...". It falls back to the tool name.
func toolTitle(toolName string, args map[string]any) string {
	str := func(key string) string {
		v, _ := args[key].(string)
		return strings.TrimSpace(v)
	}
	var detail string
	switch toolName {
	case "shell", "bash":
		detail = str("command")
		if i := strings.IndexByte(detail, '\n'); i >= 0 {
			detail = detail[:i] + " …"
		}
	case "read", "write", "edit", "ls":
		detail = str("path")
	case "grep", "find":
		detail = str("pattern")
		if p := str("path"); p != "" && detail != "" {
			detail += " in " + p
		}
	case "subagent":
		detail = str("agent")
		if detail == "" {
			detail = str("task")
		}
		if i := strings.IndexByte(detail, '\n'); i >= 0 {
			detail = detail[:i] + " …"
		}
	}
	if detail == "" {
		return toolName
	}
	const maxLen = 120
	if r := []rune(detail); len(r) > maxLen {
		detail = string(r[:maxLen]) + "…"
	}
	if toolName == "shell" || toolName == "bash" {
		return toolName + ": " + detail
	}
	return toolName + " " + detail
}

// toolLocations returns the file the tool call touches, so clients can
// follow the agent. Relative paths are resolved against the session cwd.
func toolLocations(toolName string, args map[string]any, cwd string) []acp.ToolCallLocation {
	switch toolName {
	case "read", "write", "edit", "ls":
	default:
		return nil
	}
	p, _ := args["path"].(string)
	if p == "" {
		return nil
	}
	if !filepath.IsAbs(p) && cwd != "" {
		p = filepath.Join(cwd, p)
	}
	return []acp.ToolCallLocation{{Path: p}}
}

// toolResultContent builds the content of a finished tool call: the text
// result, plus one diff per replaced block for file edits.
func toolResultContent(result string, meta *kit.ToolResultMetadata) []acp.ToolCallContent {
	var content []acp.ToolCallContent
	if meta != nil {
		for _, fd := range meta.FileDiffs {
			for _, b := range fd.DiffBlocks {
				if fd.IsNew || b.OldText == "" {
					content = append(content, acp.ToolDiffContent(fd.Path, b.NewText))
				} else {
					content = append(content, acp.ToolDiffContent(fd.Path, b.NewText, b.OldText))
				}
			}
		}
	}
	if result != "" {
		content = append(content, acp.ToolContent(acp.TextBlock(result)))
	}
	return content
}

// parseToolArgs parses a JSON tool args string into a map for structured
// display. Invalid JSON is wrapped as {"input": args}.
func parseToolArgs(args string) map[string]any {
	if args == "" {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(args), &m); err == nil {
		return m
	}
	return map[string]any{"input": args}
}

// toolIDMapper makes tool call IDs unique within a session. ACP identifies a
// tool call by its toolCallId for the whole session, but some providers reuse
// IDs such as "ls_0" in every response. A repeated ID gets a "~N" suffix; the
// updates of a call use the ID its tool_call got. Calls without an ID get a
// generated one.
type toolIDMapper struct {
	mu      sync.Mutex
	seen    map[string]int    // provider ID -> times seen
	current map[string]string // provider ID -> ACP ID of its latest call
	anon    int               // generated IDs so far
}

func newToolIDMapper() *toolIDMapper {
	return &toolIDMapper{seen: map[string]int{}, current: map[string]string{}}
}

// start returns the ACP ID for a new tool call.
func (m *toolIDMapper) start(raw string) acp.ToolCallId {
	m.mu.Lock()
	defer m.mu.Unlock()
	if raw == "" {
		m.anon++
		raw = fmt.Sprintf("tool_%d", m.anon)
	}
	n := m.seen[raw]
	m.seen[raw] = n + 1
	id := raw
	if n > 0 {
		id = fmt.Sprintf("%s~%d", raw, n)
	}
	m.current[raw] = id
	return acp.ToolCallId(id)
}

// find returns the ACP ID of the latest tool call with the given provider
// ID, and whether such a call was started in this session.
func (m *toolIDMapper) find(raw string) (acp.ToolCallId, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id, ok := m.current[raw]
	return acp.ToolCallId(id), ok
}

// lookup returns the ACP ID of the latest tool call with the given provider
// ID.
func (m *toolIDMapper) lookup(raw string) acp.ToolCallId {
	m.mu.Lock()
	defer m.mu.Unlock()
	if raw == "" {
		raw = fmt.Sprintf("tool_%d", m.anon)
	}
	if id, ok := m.current[raw]; ok {
		return acp.ToolCallId(id)
	}
	return acp.ToolCallId(raw)
}

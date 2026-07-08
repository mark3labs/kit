package kit

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"charm.land/fantasy"

	"github.com/mark3labs/kit/internal/agent"
	"github.com/mark3labs/kit/internal/config"
	"github.com/mark3labs/kit/internal/core"
	"github.com/mark3labs/kit/internal/extensions"
	"github.com/mark3labs/kit/internal/kitsetup"
	"github.com/mark3labs/kit/internal/message"
	"github.com/mark3labs/kit/internal/models"
	"github.com/mark3labs/kit/internal/session"
	"github.com/mark3labs/kit/internal/skills"
	"github.com/mark3labs/kit/internal/skilltool"
	"github.com/mark3labs/kit/internal/tools"

	"github.com/spf13/viper"
)

// ContextFile represents a project context file (e.g. AGENTS.md) that was
// loaded during initialization and injected into the system prompt.
type ContextFile struct {
	Path    string // Absolute filesystem path.
	Content string // Full file content.
}

// Kit provides programmatic access to kit functionality, allowing
// integration of MCP tools and LLM interactions into Go applications. It manages
// agents, sessions, and model configurations.
type Kit struct {
	agent          *agent.Agent
	session        SessionManager
	modelString    string
	events         *eventBus
	autoCompact    bool
	compactionOpts *CompactionOptions
	contextFiles   []*ContextFile
	skills         []*skills.Skill
	namedAgents    []*AgentDefinition // named agent definitions discovered at construction
	extRunner      *extensions.Runner
	bufferedLogger *tools.BufferedDebugLogger
	authHandler    MCPAuthHandler // OAuth handler for remote MCP servers (may need Close)
	opts           *Options       // stored for reload operations (skills, etc.)
	mcpConfig      *config.Config // loaded MCP/server config, shared with subagents

	// v is this Kit instance's isolated configuration store. Each Kit owns its
	// own *viper.Viper (constructed via viper.New) so that runtime config
	// mutators (SetModel, SetThinkingLevel) and config reads do not clobber or
	// observe state from other Kit instances in the same process. When the CLI
	// constructs a Kit (Options.CLI != nil) this points at the process-global
	// store so cobra flag bindings remain in effect.
	v *viper.Viper

	// hasCustomSystemPrompt is true when the user explicitly configured a
	// system prompt (via --system-prompt flag, config file, or SDK option).
	// When false, per-model system prompts from modelSettings/customModels
	// can replace the default prompt on model switch.
	hasCustomSystemPrompt bool
	// systemPromptSource holds the raw configured value (file path or text)
	// when hasCustomSystemPrompt is true; empty when the built-in default is in use.
	systemPromptSource string
	// basePrompt holds the resolved base system prompt text (post file-load,
	// pre runtime-context composition) captured during New. Used by
	// RefreshSystemPrompt to recompose after skills/context-file mutations.
	// Protected by runtimeMu.
	basePrompt string

	// Hook registries — interception layer (see hooks.go).
	beforeToolCall  *hookRegistry[BeforeToolCallHook, BeforeToolCallResult]
	afterToolResult *hookRegistry[AfterToolResultHook, AfterToolResultResult]
	beforeTurn      *hookRegistry[BeforeTurnHook, BeforeTurnResult]
	afterTurn       *hookRegistry[AfterTurnHook, AfterTurnResult]
	contextPrepare  *hookRegistry[ContextPrepareHook, ContextPrepareResult]
	beforeCompact   *hookRegistry[BeforeCompactHook, BeforeCompactResult]
	prepareStep     *hookRegistry[PrepareStepHook, PrepareStepResult]

	// lastInputTokens stores the API-reported input token count from the
	// most recent turn. Used by GetContextStats() to return accurate usage
	// instead of the text-based heuristic which misses system prompts,
	// tool definitions, etc.
	lastInputTokensMu sync.RWMutex
	lastInputTokens   int

	// subagentListeners holds per-tool-call event listeners registered via
	// SubscribeSubagent(). Keyed by toolCallID → *subagentListenerSet.
	subagentListeners sync.Map

	// skillCache holds skills discovered for this Kit instance.
	// Using a per-instance cache avoids cross-contamination when multiple
	// Kit instances exist in the same process.
	skillCache struct {
		skills []*skills.Skill
		mu     sync.RWMutex
	}

	// runtimeMu protects contextFiles and skills against concurrent runtime
	// mutations via AddSkill / RemoveSkill / AddContextFile etc. The fields
	// are read by composeSystemPrompt and several other accessors, so all
	// reads and writes after Kit construction must take this lock.
	runtimeMu sync.RWMutex

	// steerCh is a buffered channel used to inject steering messages into
	// the running agent turn via the LLM library's PrepareStep. Created fresh for
	// each generate() call and set to nil when idle. Protected by steerMu.
	steerMu       sync.Mutex
	steerCh       chan agent.SteerMessage
	leftoverSteer []agent.SteerMessage // unconsumed steer messages from the last turn

	// promptOptsMu protects shared agent state that can be mutated at runtime
	// (model, thinking level, provider creds, extra tools). It serializes
	// writers so the apply/restore window of one call never races another, while
	// allowing concurrent readers of the extra-tool set.
	promptOptsMu sync.RWMutex

	// runtimeExtraTools holds native tools added via AddTools / SetExtraTools /
	// RemoveTools and via Options.ExtraTools at construction. Extension tools
	// are kept separately on the extension runner and recomposed with this
	// slice when either side changes.
	runtimeExtraTools []Tool
}

// Subscribe registers an EventListener that will be called for every lifecycle
// event emitted during Prompt(). Returns an unsubscribe function that removes
// the listener.
func (m *Kit) Subscribe(listener EventListener) func() {
	return m.events.subscribe(listener)
}

// --------------------------------------------------------------------------
// Narrow accessors
// --------------------------------------------------------------------------

// GetToolNames returns the names of all tools available to the agent.
func (m *Kit) GetToolNames() []string {
	agentTools := m.agent.GetTools()
	names := make([]string, len(agentTools))
	for i, t := range agentTools {
		names[i] = t.Info().Name
	}
	return names
}

// GetToolsForSubagent like GetTools but eliminates subagent tool
// to avoid infinite recursion.
func (m *Kit) GetToolsForSubagent() []Tool {
	var tools []Tool
	for _, t := range m.agent.GetTools() {
		if t.Info().Name == "subagent" {
			continue
		}
		tools = append(tools, t)
	}
	return tools
}

// AddTools additively registers native Go tools on the live host. Added tools
// persist for the session and become visible to the model at the next LLM
// step (see below).
// If a provided tool shares a name with a tool that is already in the
// extra-tool set, the new tool replaces the previous one (last-write-wins).
// Core tools and MCP tools are not affected.
//
// AddTools is safe to call while the agent is idle. If a turn is in progress
// ([Kit.IsGenerating] returns true), the change takes effect starting from the
// next LLM step of the current turn — a tool added from within a tool handler
// is callable by the model on its next step without waiting for a new turn.
func (m *Kit) AddTools(tools ...Tool) {
	m.promptOptsMu.Lock()
	defer m.promptOptsMu.Unlock()

	cur := m.runtimeExtraTools
	if len(cur) == 0 && len(tools) == 0 {
		return
	}

	replacements := make(map[string]Tool, len(tools))
	for _, t := range tools {
		replacements[t.Info().Name] = t
	}

	seen := make(map[string]struct{}, len(cur)+len(tools))
	merged := make([]Tool, 0, len(cur)+len(tools))
	for _, t := range cur {
		if r, ok := replacements[t.Info().Name]; ok {
			merged = append(merged, r)
		} else {
			merged = append(merged, t)
		}
		seen[t.Info().Name] = struct{}{}
	}
	for _, t := range tools {
		if _, ok := seen[t.Info().Name]; !ok {
			merged = append(merged, replacements[t.Info().Name])
			seen[t.Info().Name] = struct{}{}
		}
	}

	m.runtimeExtraTools = merged
	m.recomposeExtraTools()
}

// RemoveTools removes previously-added native Go tools by name. Core tools and
// MCP tools are unaffected. If any of the supplied names is not currently in
// the extra-tool set, an error is returned listing the missing names and no
// tools are removed.
//
// RemoveTools is safe to call while the agent is idle. If a turn is in
// progress, the tools are removed at the next LLM step.
func (m *Kit) RemoveTools(names ...string) error {
	m.promptOptsMu.Lock()
	defer m.promptOptsMu.Unlock()

	cur := m.runtimeExtraTools

	drop := make(map[string]struct{}, len(names))
	for _, n := range names {
		drop[n] = struct{}{}
	}

	missing := make(map[string]struct{}, len(names))
	for n := range drop {
		missing[n] = struct{}{}
	}

	kept := make([]Tool, 0, len(cur))
	for _, t := range cur {
		if _, ok := drop[t.Info().Name]; ok {
			delete(missing, t.Info().Name)
			continue
		}
		kept = append(kept, t)
	}

	if len(missing) > 0 {
		list := make([]string, 0, len(missing))
		for n := range missing {
			list = append(list, n)
		}
		sort.Strings(list)
		return fmt.Errorf("tool(s) not found: %s", strings.Join(list, ", "))
	}

	m.runtimeExtraTools = kept
	m.recomposeExtraTools()
	return nil
}

// SetExtraTools replaces the entire native extra-tool set in one call. Core
// tools and MCP tools are unaffected. Pass an empty slice to clear all
// extra tools.
//
// SetExtraTools is safe to call while the agent is idle. If a turn is in
// progress, the change takes effect starting from the next LLM step.
func (m *Kit) SetExtraTools(tools ...Tool) {
	m.promptOptsMu.Lock()
	defer m.promptOptsMu.Unlock()
	m.runtimeExtraTools = append([]Tool(nil), tools...)
	m.recomposeExtraTools()
}

// GetExtraTools returns a snapshot of the native extra tools that were added
// via AddTools / SetExtraTools / RemoveTools or Options.ExtraTools. Extension
// tools are not included. The returned slice is a copy; modifying it does not
// affect the tools registered on the host.
func (m *Kit) GetExtraTools() []Tool {
	m.promptOptsMu.RLock()
	defer m.promptOptsMu.RUnlock()
	if len(m.runtimeExtraTools) == 0 {
		return nil
	}
	out := make([]Tool, len(m.runtimeExtraTools))
	copy(out, m.runtimeExtraTools)
	return out
}

// recomposeExtraTools rebuilds the agent's extra-tool list from extension
// tools plus runtime native tools. Callers must hold promptOptsMu.
func (m *Kit) recomposeExtraTools() {
	var combined []Tool
	if m.extRunner != nil {
		extTools := extensions.ExtensionToolsAsLLMTools(m.extRunner.RegisteredTools(), m.extRunner)
		if len(extTools) > 0 {
			combined = make([]Tool, 0, len(extTools)+len(m.runtimeExtraTools))
			combined = append(combined, extTools...)
		}
	}
	combined = append(combined, m.runtimeExtraTools...)
	m.agent.SetExtraTools(combined)
}

// GetLoadingMessage returns the agent's startup info message (e.g. GPU
// fallback info), or empty string if none.
func (m *Kit) GetLoadingMessage() string {
	return m.agent.GetLoadingMessage()
}

// GetLoadedServerNames returns the names of successfully loaded MCP servers.
// If MCP servers are still loading in the background, this returns only the
// servers that have completed loading so far.
func (m *Kit) GetLoadedServerNames() []string {
	return m.agent.GetLoadedServerNames()
}

// GetMCPToolCount returns the number of tools loaded from external MCP servers.
// If MCP servers are still loading in the background, this returns the count
// of tools loaded so far (may be 0).
func (m *Kit) GetMCPToolCount() int {
	return m.agent.GetMCPToolCount()
}

// GetMCPToolNames returns the prefixed names (serverName__toolName) of all
// tools currently loaded from external MCP servers. Returns nil when no MCP
// servers are configured or none have finished loading yet.
func (m *Kit) GetMCPToolNames() []string {
	return m.agent.GetMCPToolNames()
}

// WaitForMCPTools blocks until background MCP tool loading completes.
// Returns nil if no MCP servers are configured or if loading succeeded.
// Returns the loading error if all servers failed. Safe to call multiple times.
func (m *Kit) WaitForMCPTools() error {
	return m.agent.WaitForMCPTools()
}

// MCPToolsReady returns true if MCP tool loading has completed (or was never
// started). This is a non-blocking check useful for UI status display.
func (m *Kit) MCPToolsReady() bool {
	return m.agent.MCPToolsReady()
}

// MCPServerStatus describes the runtime state of a loaded MCP server.
type MCPServerStatus struct {
	// Name is the configured server name.
	Name string
	// ToolCount is the number of tools loaded from this server.
	ToolCount int
}

// AddMCPServer connects to a new MCP server at runtime and makes its tools
// available to the agent immediately. The server's tools are prefixed with the
// server name (e.g. "myserver__tool_name") to avoid naming conflicts, matching
// the behaviour of servers loaded at initialization.
//
// Returns the number of tools loaded from the server.
//
// AddMCPServer is safe to call while the agent is idle. If a turn is in
// progress ([Kit.IsGenerating] returns true), the new tools will be visible
// starting from the next LLM step.
//
// Example:
//
//	n, err := k.AddMCPServer(ctx, "github", kit.MCPServerConfig{
//	    Command: []string{"npx", "-y", "@modelcontextprotocol/server-github"},
//	    Environment: map[string]string{"GITHUB_TOKEN": os.Getenv("GITHUB_TOKEN")},
//	})
func (m *Kit) AddMCPServer(ctx context.Context, name string, cfg MCPServerConfig) (int, error) {
	return m.agent.AddMCPServer(ctx, name, cfg)
}

// AddInProcessMCPServer connects an in-process mcp-go server and makes its
// tools available to the agent immediately. Unlike [AddMCPServer] with a
// command/URL config, this uses mcp-go's in-process transport — no subprocess
// is spawned and no network I/O occurs.
//
// The server must be a *[server.MCPServer] from github.com/mark3labs/mcp-go/server.
// Kit does not take ownership of the server's lifecycle; the caller is responsible
// for any cleanup when the server is no longer needed.
//
// Returns the number of tools loaded from the server.
//
// Example:
//
//	import (
//	    "github.com/mark3labs/mcp-go/mcp"
//	    "github.com/mark3labs/mcp-go/server"
//	)
//
//	mcpSrv := server.NewMCPServer("my-tools", "1.0.0",
//	    server.WithToolCapabilities(true),
//	)
//	mcpSrv.AddTool(mcp.NewTool("search_docs",
//	    mcp.WithDescription("Search documentation"),
//	    mcp.WithString("query", mcp.Required()),
//	), searchHandler)
//
//	n, err := k.AddInProcessMCPServer(ctx, "docs", mcpSrv)
func (m *Kit) AddInProcessMCPServer(ctx context.Context, name string, srv *MCPServer) (int, error) {
	cfg := MCPServerConfig{
		Type:            "inprocess",
		InProcessServer: srv,
	}
	return m.agent.AddMCPServer(ctx, name, cfg)
}

// RemoveMCPServer disconnects an MCP server and removes all its tools from
// the agent. After this call the agent will no longer see or be able to call
// tools from the named server.
//
// RemoveMCPServer is safe to call while the agent is idle. If a turn is in
// progress, the tools are removed at the next LLM step. Any in-flight tool
// calls to the removed server will fail gracefully.
//
// Returns an error if the named server is not currently loaded.
func (m *Kit) RemoveMCPServer(name string) error {
	return m.agent.RemoveMCPServer(name)
}

// ListMCPServers returns the status of all currently loaded MCP servers.
// The returned slice is a snapshot; it is safe to read concurrently.
func (m *Kit) ListMCPServers() []MCPServerStatus {
	names := m.agent.GetLoadedServerNames()
	if len(names) == 0 {
		return nil
	}

	// Build a tool count per server by scanning tool names for the prefix.
	toolNames := m.GetToolNames()
	countByServer := make(map[string]int, len(names))
	for _, tn := range toolNames {
		for _, sn := range names {
			prefix := sn + "__"
			if len(tn) > len(prefix) && tn[:len(prefix)] == prefix {
				countByServer[sn]++
				break
			}
		}
	}

	result := make([]MCPServerStatus, 0, len(names))
	for _, n := range names {
		result = append(result, MCPServerStatus{
			Name:      n,
			ToolCount: countByServer[n],
		})
	}
	return result
}

// GetExtensionToolCount returns the number of tools registered by extensions.
func (m *Kit) GetExtensionToolCount() int {
	return m.agent.GetExtensionToolCount()
}

// --------------------------------------------------------------------------
// MCP Prompts
// --------------------------------------------------------------------------

// MCPPrompt describes a prompt exposed by an MCP server.
type MCPPrompt struct {
	// Name is the prompt name on the MCP server.
	Name string
	// Description is a human-readable description.
	Description string
	// Arguments lists the prompt's expected arguments.
	Arguments []MCPPromptArgument
	// ServerName is the MCP server that provides this prompt.
	ServerName string
}

// MCPPromptArgument describes a single argument for an MCP prompt.
type MCPPromptArgument struct {
	// Name is the argument name.
	Name string
	// Description is a human-readable description.
	Description string
	// Required indicates whether this argument must be provided.
	Required bool
}

// MCPPromptMessage is a single message returned by a prompt expansion.
type MCPPromptMessage struct {
	// Role is "user" or "assistant".
	Role string
	// Content is the text content of the message.
	Content string
	// FileParts contains binary attachments extracted from embedded resources,
	// images, or audio content blocks within the prompt message. Empty for
	// text-only messages.
	FileParts []LLMFilePart
}

// MCPPromptResult is the result of expanding an MCP prompt.
type MCPPromptResult struct {
	// Description is an optional description returned by the server.
	Description string
	// Messages contains the expanded prompt messages.
	Messages []MCPPromptMessage
}

// ListMCPPrompts returns all prompts discovered from connected MCP servers.
// If MCP servers are still loading in the background, this returns only the
// prompts discovered so far. Returns nil if no prompts are available.
func (m *Kit) ListMCPPrompts() []MCPPrompt {
	internal := m.agent.GetMCPPrompts()
	if len(internal) == 0 {
		return nil
	}
	result := make([]MCPPrompt, len(internal))
	for i, p := range internal {
		args := make([]MCPPromptArgument, len(p.Arguments))
		for j, a := range p.Arguments {
			args[j] = MCPPromptArgument{
				Name:        a.Name,
				Description: a.Description,
				Required:    a.Required,
			}
		}
		result[i] = MCPPrompt{
			Name:        p.Name,
			Description: p.Description,
			Arguments:   args,
			ServerName:  p.ServerName,
		}
	}
	return result
}

// GetMCPPrompt retrieves and expands a specific prompt from an MCP server.
// This is a lazy call — the server is contacted each time to get the latest
// prompt content. Arguments are passed as key=value pairs to the server for
// template substitution.
//
// Returns an error if the server is not found or the prompt expansion fails.
func (m *Kit) GetMCPPrompt(ctx context.Context, serverName, promptName string, args map[string]string) (*MCPPromptResult, error) {
	internal, err := m.agent.GetMCPPrompt(ctx, serverName, promptName, args)
	if err != nil {
		return nil, err
	}
	msgs := make([]MCPPromptMessage, len(internal.Messages))
	for i, msg := range internal.Messages {
		var fileParts []LLMFilePart
		for _, fp := range msg.FileParts {
			fileParts = append(fileParts, LLMFilePart{
				Filename:  fp.Filename,
				Data:      fp.Data,
				MediaType: fp.MediaType,
			})
		}
		msgs[i] = MCPPromptMessage{
			Role:      msg.Role,
			Content:   msg.Content,
			FileParts: fileParts,
		}
	}
	return &MCPPromptResult{
		Description: internal.Description,
		Messages:    msgs,
	}, nil
}

// --------------------------------------------------------------------------
// MCP Resources
// --------------------------------------------------------------------------

// MCPResource describes a resource exposed by an MCP server.
type MCPResource struct {
	// URI is the unique resource identifier (e.g. "file:///path" or custom scheme).
	URI string
	// Name is a human-readable name for the resource.
	Name string
	// Description is an optional description of the resource.
	Description string
	// MIMEType is the MIME type of the resource, if known.
	MIMEType string
	// ServerName is the MCP server that provides this resource.
	ServerName string
}

// MCPResourceContent is the result of reading an MCP resource.
type MCPResourceContent struct {
	// URI is the resource URI that was read.
	URI string
	// MIMEType is the MIME type of the content.
	MIMEType string
	// Text is the text content (non-empty for text resources).
	Text string
	// BlobData is the decoded binary content (non-empty for blob resources).
	BlobData []byte
	// IsBlob is true when the content is binary (BlobData is set).
	IsBlob bool
}

// ListMCPResources returns all resources discovered from connected MCP servers.
// If MCP servers are still loading in the background, this returns only the
// resources discovered so far. Returns nil if no resources are available.
func (m *Kit) ListMCPResources() []MCPResource {
	internal := m.agent.GetMCPResources()
	if len(internal) == 0 {
		return nil
	}
	result := make([]MCPResource, len(internal))
	for i, r := range internal {
		result[i] = MCPResource{
			URI:         r.URI,
			Name:        r.Name,
			Description: r.Description,
			MIMEType:    r.MIMEType,
			ServerName:  r.ServerName,
		}
	}
	return result
}

// ReadMCPResource reads a specific resource from an MCP server by URI.
// Returns the resource content (text or binary blob).
func (m *Kit) ReadMCPResource(ctx context.Context, serverName, uri string) (*MCPResourceContent, error) {
	internal, err := m.agent.ReadMCPResource(ctx, serverName, uri)
	if err != nil {
		return nil, err
	}
	return &MCPResourceContent{
		URI:      internal.URI,
		MIMEType: internal.MIMEType,
		Text:     internal.Text,
		BlobData: internal.BlobData,
		IsBlob:   internal.IsBlob,
	}, nil
}

// SubscribeMCPResource subscribes to change notifications for a resource.
// When the resource changes on the server, the resource list is refreshed.
func (m *Kit) SubscribeMCPResource(ctx context.Context, serverName, uri string) error {
	return m.agent.SubscribeMCPResource(ctx, serverName, uri)
}

// UnsubscribeMCPResource cancels change notifications for a resource.
func (m *Kit) UnsubscribeMCPResource(ctx context.Context, serverName, uri string) error {
	return m.agent.UnsubscribeMCPResource(ctx, serverName, uri)
}

// GetBufferedDebugMessages returns any debug messages that were buffered
// during initialization, then clears the buffer. Returns nil if no messages
// were buffered or if buffered logging was not configured.
func (m *Kit) GetBufferedDebugMessages() []string {
	if m.bufferedLogger == nil {
		return nil
	}
	return m.bufferedLogger.GetMessages()
}

// StructuredMessage represents a conversation message with typed content parts
// (tool calls, reasoning, finish markers, etc.) instead of flattened text.
type StructuredMessage struct {
	ID        string
	ParentID  string
	Role      MessageRole
	Parts     []ContentPart
	Model     string
	Provider  string
	Timestamp string // RFC3339 format
}

// GetStructuredMessages returns the conversation messages on the current
// branch with full typed content parts. Unlike GetSessionMessages() which
// flattens all content to a single text string, this preserves tool calls,
// tool results, reasoning blocks, and finish markers as distinct typed parts.
func (m *Kit) GetStructuredMessages() []StructuredMessage {
	if m.session == nil {
		return nil
	}

	branch := m.session.GetCurrentBranch()
	var results []StructuredMessage
	for _, entry := range branch {
		if entry.Type != EntryTypeMessage {
			continue
		}
		results = append(results, StructuredMessage{
			ID:        entry.ID,
			ParentID:  entry.ParentID,
			Role:      MessageRole(entry.Role),
			Parts:     entry.RawParts,
			Model:     entry.Model,
			Provider:  entry.Provider,
			Timestamp: entry.Timestamp.Format("2006-01-02T15:04:05Z07:00"),
		})
	}
	return results
}

// iterBranchMessages iterates over the current branch's MessageEntry items,
// converting each to a message.Message and calling fn to build the result.
// Returns nil if there is no session. Skips entries that are not
// MessageEntry or that fail conversion.
// Deprecated: Use SessionManager.GetCurrentBranch() directly.
func iterBranchMessages[T any](tm *session.TreeManager, fn func(*session.MessageEntry, message.Message) T) []T {
	if tm == nil {
		return nil
	}

	branch := tm.GetBranch("")
	var results []T
	for _, entry := range branch {
		me, ok := entry.(*session.MessageEntry)
		if !ok {
			continue
		}
		msg, err := me.ToMessage()
		if err != nil {
			continue
		}
		results = append(results, fn(me, msg))
	}
	return results
}

// SetModel changes the active model at runtime. The existing tools and
// session are preserved. When the new model has a per-model system prompt
// (from modelSettings or customModels params), it is composed with the
// current AGENTS.md context and skills before being applied.
// The model string should be in "provider/model" format
// (e.g. "anthropic/claude-sonnet-4-5-20250929").
// Returns an error if the model string is invalid or the provider cannot
// be created.
func (m *Kit) SetModel(ctx context.Context, modelString string) error {
	// Validate the model string first.
	if _, _, err := ParseModelString(modelString); err != nil {
		return err
	}

	// Build a provider config from current settings, overriding the model.
	// Load system prompt properly (handles both file paths and inline content).
	systemPrompt, _ := config.LoadSystemPrompt(m.v.GetString("system-prompt"))
	thinkingLevel := models.ParseThinkingLevel(m.v.GetString("thinking-level"))

	// Validate and adjust thinking level for the target model.
	// Some models (e.g., OpenAI gpt-5.4) don't support "minimal" and require "none".
	if thinkingLevel != models.ThinkingOff {
		parts := strings.SplitN(modelString, "/", 2)
		if len(parts) == 2 {
			modelName := parts[1]
			if !models.IsValidThinkingLevelForModel(thinkingLevel, modelName) {
				fallback := models.SuggestThinkingLevelFallback(thinkingLevel, modelName)
				if fallback != models.ThinkingOff {
					// Adjust the thinking level in the instance store so the change persists.
					m.v.Set("thinking-level", string(fallback))
					thinkingLevel = fallback
				}
			}
		}
	}

	// With message-level caching, thinking and caching can work together.
	// No need to disable caching when thinking is enabled.
	cfg := &models.ProviderConfig{
		ModelString:    modelString,
		SystemPrompt:   systemPrompt,
		ProviderAPIKey: m.v.GetString("provider-api-key"),
		ProviderURL:    m.v.GetString("provider-url"),
		MaxTokens:      m.v.GetInt("max-tokens"),
		TLSSkipVerify:  m.v.GetBool("tls-skip-verify"),
		ThinkingLevel:  thinkingLevel,
		DisableCaching: false, // Caching enabled by default, works with thinking
		ConfigStore:    m.v,
	}

	// Only set generation parameter pointers when the user has explicitly
	// provided a value. This leaves nil pointers for unset params, allowing
	// per-model defaults (modelSettings / customModels params) to apply.
	if m.v.IsSet("temperature") {
		v := float32(m.v.GetFloat64("temperature"))
		cfg.Temperature = &v
	}
	if m.v.IsSet("top-p") {
		v := float32(m.v.GetFloat64("top-p"))
		cfg.TopP = &v
	}
	if m.v.IsSet("top-k") {
		v := int32(m.v.GetInt("top-k"))
		cfg.TopK = &v
	}
	if m.v.IsSet("frequency-penalty") {
		v := float32(m.v.GetFloat64("frequency-penalty"))
		cfg.FrequencyPenalty = &v
	}
	if m.v.IsSet("presence-penalty") {
		v := float32(m.v.GetFloat64("presence-penalty"))
		cfg.PresencePenalty = &v
	}

	// When the user hasn't set a custom global system prompt, check for a
	// per-model system prompt. Pre-apply model settings to discover it,
	// then compose with AGENTS.md context and skills if found.
	if !m.hasCustomSystemPrompt {
		// Temporarily clear the system prompt so ApplyModelSettings can
		// detect that no explicit prompt is set and apply the per-model one.
		cfg.SystemPrompt = ""
		models.ApplyModelSettings(cfg, models.LookupModelForSettings(modelString))

		if cfg.SystemPrompt != "" {
			// Per-model system prompt found — compose with runtime context.
			cfg.SystemPrompt = m.composeSystemPrompt(cfg.SystemPrompt)
		} else {
			// No per-model prompt — restore the global composed prompt.
			cfg.SystemPrompt = systemPrompt
		}
	}

	if err := m.agent.SetModel(ctx, cfg); err != nil {
		return err
	}

	m.modelString = modelString

	// Update extension context's Model field.
	if m.extRunner != nil {
		extCtx := m.extRunner.GetContext()
		extCtx.Model = modelString
		m.extRunner.SetContext(extCtx)
	}

	return nil
}

// HasCustomSystemPrompt reports whether the user explicitly configured a system
// prompt via --system-prompt, a config file entry, or SDK Options.SystemPrompt.
// When false, the built-in default (or a per-model override) is in use and can
// be replaced transparently on model switch.
func (m *Kit) HasCustomSystemPrompt() bool {
	return m.hasCustomSystemPrompt
}

// GetSystemPromptSource returns the raw configured value — a file path or
// inline text — when HasCustomSystemPrompt is true; returns an empty string
// when the built-in default prompt is active.
func (m *Kit) GetSystemPromptSource() string {
	return m.systemPromptSource
}

// composeSystemPrompt takes a base system prompt and composes it with the
// current runtime context: AGENTS.md content, skills metadata, and date/cwd.
// This mirrors the composition done during Kit.New() initialization.
// It acquires a read lock on runtimeMu while snapshotting contextFiles and
// skills, so callers must not hold the write lock.
func (m *Kit) composeSystemPrompt(basePrompt string) string {
	cwd, _ := os.Getwd()
	pb := skills.NewPromptBuilder(basePrompt)

	m.runtimeMu.RLock()
	contextFiles := append([]*ContextFile(nil), m.contextFiles...)
	loadedSkills := append([]*skills.Skill(nil), m.skills...)
	m.runtimeMu.RUnlock()

	// Inject AGENTS.md content as project context.
	for _, cf := range contextFiles {
		pb.WithSection("", fmt.Sprintf("Instructions from: %s\n\n%s", cf.Path, cf.Content))
	}

	// Inject skills metadata.
	if len(loadedSkills) > 0 {
		pb.WithSkills(loadedSkills)
	}

	// Append current date/time and working directory.
	pb.WithSection("", fmt.Sprintf(
		"Current date and time: %s\nCurrent working directory: %s",
		time.Now().Format("Monday, January 2, 2006, 3:04:05 PM MST"), cwd,
	))

	return pb.Build()
}

// GetAvailableModels returns a list of known models from the registry. Each
// entry includes provider, model ID, context limit, and whether the model
// supports reasoning. This is an advisory list — models not in the registry
// can still be used by specifying their provider/model string.
func (m *Kit) GetAvailableModels() []extensions.ModelInfoEntry {
	registry := models.GetGlobalRegistry()
	var result []extensions.ModelInfoEntry
	for _, providerID := range registry.GetLLMProviders() {
		modelsMap, err := registry.GetModelsForProvider(providerID)
		if err != nil {
			continue
		}
		for modelID, info := range modelsMap {
			result = append(result, extensions.ModelInfoEntry{
				Provider:     providerID,
				ModelID:      modelID,
				Name:         info.Name,
				ContextLimit: info.Limit.Context,
				OutputLimit:  info.Limit.Output,
				Reasoning:    info.Reasoning,
			})
		}
	}
	return result
}

// ReloadExtensions hot-reloads all extensions from disk. Event handlers,
// commands, renderers, shortcuts, and extension-defined tools all update
// immediately.
func (m *Kit) ReloadExtensions() error {
	if m.extRunner == nil {
		return fmt.Errorf("no extensions loaded")
	}

	// Emit shutdown to old extensions.
	if m.extRunner.HasHandlers(extensions.SessionShutdown) {
		_, _ = m.extRunner.Emit(extensions.SessionShutdownEvent{})
	}

	// Re-load from disk.
	extraPaths := m.v.GetStringSlice("extension")
	loaded, err := extensions.LoadExtensions(extraPaths)
	if err != nil {
		return fmt.Errorf("reloading extensions: %w", err)
	}

	// Swap extensions on the runner (clears dynamic state).
	m.extRunner.Reload(loaded)
	m.extRunner.SetConfigStore(m.v)

	// Update extension tools on the agent so the LLM sees changes.
	if m.agent != nil {
		m.promptOptsMu.Lock()
		m.recomposeExtraTools()
		m.promptOptsMu.Unlock()
	}

	// Re-set context and emit SessionStart.
	ctx := m.extRunner.GetContext()
	m.extRunner.SetContext(ctx)
	if m.extRunner.HasHandlers(extensions.SessionStart) {
		_, _ = m.extRunner.Emit(extensions.SessionStartEvent{SessionID: ctx.SessionID})
	}

	return nil
}

// ExecuteCompletion makes a standalone LLM completion call for extensions.
// When req.Model is empty the current agent model is reused (no provider
// creation overhead). When req.Model is set a temporary provider is created,
// used, and closed.
func (m *Kit) ExecuteCompletion(ctx context.Context, req extensions.CompleteRequest) (extensions.CompleteResponse, error) {
	var (
		llmModel    fantasy.LanguageModel
		closer      func()
		usedModel   string
		providerOps LLMProviderOptions
	)

	if req.Model == "" {
		// Reuse the active agent's model.
		llmModel = m.agent.GetModel()
		usedModel = m.modelString
		closer = func() {} // nothing to clean up
	} else {
		// Create a temporary provider for the requested model.
		config := &models.ProviderConfig{
			ModelString:   req.Model,
			TLSSkipVerify: m.v.GetBool("tls-skip-verify"),
			ConfigStore:   m.v,
		}
		if req.MaxTokens > 0 {
			config.MaxTokens = req.MaxTokens
		}
		providerResult, err := models.CreateProvider(ctx, config)
		if err != nil {
			return extensions.CompleteResponse{}, fmt.Errorf("create provider for %q: %w", req.Model, err)
		}
		llmModel = providerResult.Model
		usedModel = req.Model
		providerOps = providerResult.ProviderOptions
		closer = func() {
			if providerResult.Closer != nil {
				_ = providerResult.Closer.Close()
			}
		}
	}
	defer closer()

	// Build agent options (no tools — just a simple completion).
	var agentOpts []fantasy.AgentOption
	if req.System != "" {
		agentOpts = append(agentOpts, fantasy.WithSystemPrompt(req.System))
	}
	if req.MaxTokens > 0 {
		agentOpts = append(agentOpts, fantasy.WithMaxOutputTokens(int64(req.MaxTokens)))
	}
	if providerOps != nil {
		agentOpts = append(agentOpts, fantasy.WithProviderOptions(providerOps))
	}

	completionAgent := fantasy.NewAgent(llmModel, agentOpts...)

	// Convert extension SessionMessage history to LLM message slice.
	var messages []fantasy.Message
	for _, sm := range req.Messages {
		messages = append(messages, fantasy.Message{
			Role: fantasy.MessageRole(sm.Role),
			Content: []fantasy.MessagePart{
				fantasy.TextPart{Text: sm.Content},
			},
		})
	}

	// Streaming path.
	if req.OnChunk != nil {
		result, err := completionAgent.Stream(ctx, fantasy.AgentStreamCall{
			Prompt:   req.Prompt,
			Messages: messages,
			OnTextDelta: func(_, text string) error {
				req.OnChunk(text)
				return nil
			},
		})
		if err != nil {
			return extensions.CompleteResponse{}, fmt.Errorf("streaming completion: %w", err)
		}
		return extensions.CompleteResponse{
			Text:         result.Response.Content.Text(),
			InputTokens:  int(result.Response.Usage.InputTokens),
			OutputTokens: int(result.Response.Usage.OutputTokens),
			Model:        usedModel,
		}, nil
	}

	// Non-streaming path.
	result, err := completionAgent.Generate(ctx, fantasy.AgentCall{
		Prompt:   req.Prompt,
		Messages: messages,
	})
	if err != nil {
		return extensions.CompleteResponse{}, fmt.Errorf("completion: %w", err)
	}
	return extensions.CompleteResponse{
		Text:         result.Response.Content.Text(),
		InputTokens:  int(result.Response.Usage.InputTokens),
		OutputTokens: int(result.Response.Usage.OutputTokens),
		Model:        usedModel,
	}, nil
}

// Options configures Kit creation with optional overrides for model,
// prompts, configuration, and behavior settings. All fields are optional
// and will use CLI defaults if not specified.
//
// Config isolation: each [New] / [NewAgent] call constructs its own isolated
// configuration store (via viper.New internally). Options are applied to that
// per-instance store, so two Kits constructed in the same process do NOT share
// or clobber each other's configuration. Runtime mutators ([Kit.SetModel],
// [Kit.SetThinkingLevel]) and config readers ([Kit.GetThinkingLevel]) operate
// only on the owning instance. Fields left at their zero value are simply not
// applied; they fall through to the precedence chain (env → .kit.yml →
// per-model defaults) resolved within the instance's own store.
type Options struct {
	Model        string // Override model (e.g., "anthropic/claude-sonnet-4-5-20250929")
	SystemPrompt string // Override system prompt
	ConfigFile   string // Override config file path
	MaxSteps     int    // Override max steps (0 = use default)

	// Streaming enables or disables streaming output. It is a pointer so the
	// SDK can distinguish "unset" (nil) from an explicit choice, mirroring the
	// sampling-parameter fields below. nil leaves streaming to the precedence
	// chain (env → .kit.yml → default true); a non-nil value forces it. Prefer
	// [WithStreaming] for the functional-options API.
	Streaming *bool

	Quiet      bool   // Suppress debug output
	Tools      []Tool // Custom tool set. If empty, AllTools() is used.
	ExtraTools []Tool // Additional tools added alongside core/MCP/extension tools.

	// Generation parameters. These override the corresponding values from
	// .kit.yml / KIT_* environment variables. Leaving a field at its
	// zero/nil value means "use the configured default", which in turn
	// falls back to per-model defaults (modelSettings / customModels) and
	// finally to a last-resort SDK floor of 8192 for MaxTokens (matching
	// the CLI --max-tokens default; sampling params fall through to
	// provider-level defaults).
	//
	// Pointer types are used for sampling parameters so the SDK can
	// distinguish "explicitly set to 0" from "leave alone".

	// MaxTokens overrides the maximum output tokens per LLM response.
	// 0 = let the precedence chain resolve a value (env → config →
	// per-model → 8192 SDK floor, matching the CLI default). Setting a
	// non-zero value here suppresses automatic right-sizing, matching
	// the CLI's --max-tokens flag semantics. Bump this when generating
	// long outputs (HTML artifacts, large refactors, etc.) to avoid
	// silent truncation mid-tool-call. The cap also applies after
	// model switches via [Kit.SetModel].
	MaxTokens int

	// ThinkingLevel sets the reasoning effort for models that support
	// extended thinking. Valid values: "off", "none", "minimal", "low",
	// "medium", "high". "" = let the precedence chain resolve a level
	// (env → config → per-model → "off"). Use [Kit.SetThinkingLevel]
	// to change at runtime.
	ThinkingLevel string

	// Temperature controls sampling randomness (typically 0.0–2.0).
	// nil = leave provider/per-model default in place. Pointer type
	// so explicit 0.0 (deterministic) is distinguishable from "unset".
	Temperature *float32

	// TopP is the nucleus-sampling cutoff (0.0–1.0).
	// nil = leave provider/per-model default in place.
	TopP *float32

	// TopK limits sampling to the top K tokens.
	// nil = leave provider/per-model default in place.
	TopK *int32

	// FrequencyPenalty discourages repeated tokens (OpenAI-family models).
	// nil = leave provider/per-model default in place.
	FrequencyPenalty *float32

	// PresencePenalty discourages repeating topics (OpenAI-family models).
	// nil = leave provider/per-model default in place.
	PresencePenalty *float32

	// Provider configuration. These override values normally read from
	// .kit.yml or provider-specific environment variables. Useful when
	// loading credentials from a secrets manager, pointing at custom
	// OpenAI-compatible endpoints (LiteLLM, vLLM, Azure OpenAI, internal
	// proxies), or running against self-hosted infrastructure.

	// ProviderAPIKey overrides the API key used to authenticate with the
	// model provider. "" = use the value from config or the
	// provider-specific environment variable.
	ProviderAPIKey string

	// ProviderURL overrides the provider endpoint. "" = use the provider's
	// default URL.
	ProviderURL string

	// TLSSkipVerify disables TLS certificate verification on provider
	// HTTP clients. Only set this for self-signed certificates in
	// development. Once enabled here it cannot be disabled via Options
	// (use the config file or env var to opt back out).
	TLSSkipVerify bool

	// SkipConfig, when true, skips loading .kit.yml configuration files.
	// Viper defaults (setSDKDefaults) and environment variables (KIT_*)
	// are still applied. Use this for fully programmatic configuration.
	SkipConfig bool

	// List of tools to include, when empty, include all available core
	// tools.
	CoreToolList []string

	// DisableCoreTools, when true, prevents loading any core tools.
	// Use with Tools or ExtraTools to provide only custom tools.
	// If both DisableCoreTools is true and Tools is empty, the agent
	// will have no tools (useful for simple chat completions).
	DisableCoreTools bool

	// Session configuration
	SessionDir  string // Base directory for session discovery (default: cwd)
	SessionPath string // Open a specific session file by path
	Continue    bool   // Continue the most recent session for SessionDir
	NoSession   bool   // Ephemeral mode — in-memory session, no persistence

	// Skills
	Skills    []string // Explicit skill files/dirs to load (empty = auto-discover)
	SkillsDir string   // Direct skills directory to scan (overrides auto-discovery; scanned as-is)
	NoSkills  bool     // Disable skill loading entirely (auto-discovery and explicit)

	// SkillsDisable names skills (by Name) to exclude from the model-facing
	// catalog. Disabled skills remain available via the /skill: slash command.
	SkillsDisable []string

	// SkillTrustPrompt is an optional callback invoked the first time Kit
	// auto-discovers project-local skills (under <project>/.agents/skills or
	// <project>/.kit/skills) in a directory that is not yet on the trust
	// allowlist. It receives the project directory and the number of skills
	// found, and returns a TrustDecision controlling whether the skills load.
	//
	// When nil, project-local skills are loaded without prompting (historical
	// behaviour). Directories trusted via TrustProject are persisted to
	// ~/.config/kit/trusted-projects.json and not prompted again.
	SkillTrustPrompt func(projectDir string, skillCount int) TrustDecision

	// NoExtensions disables Yaegi extension loading entirely.
	NoExtensions bool

	// NoContextFiles disables automatic loading of project context files
	// (e.g. AGENTS.md) from the working directory.
	NoContextFiles bool

	// NoAgents disables discovery of named agent definitions (built-ins and
	// .agents/agents/ / .kit/agents/ / ~/.config/kit/agents/ files). When
	// set, the subagent tool advertises no named agents and
	// [SubagentConfig].Agent cannot resolve.
	NoAgents bool

	// MCPConfig provides a pre-loaded MCP configuration. When set,
	// LoadAndValidateConfig is skipped during Kit creation — avoiding
	// viper access entirely. This is set automatically for in-process
	// subagents (inheriting the parent's loaded config) and can be used
	// by SDK consumers who build config programmatically.
	MCPConfig *config.Config

	// InProcessMCPServers registers mcp-go servers that run in the same
	// process. Each key is the server name (used to prefix tool names, e.g.
	// "docs__search"). The value must be a *[server.MCPServer].
	//
	// In-process servers bypass subprocess spawning and network I/O entirely.
	// Kit does not take ownership of the servers — the caller is responsible
	// for any cleanup after [Kit.Close].
	//
	// Example:
	//
	//	mcpSrv := server.NewMCPServer("my-tools", "1.0.0",
	//	    server.WithToolCapabilities(true),
	//	)
	//	mcpSrv.AddTool(mcp.NewTool("search", ...), handler)
	//
	//	host, _ := kit.New(ctx, &kit.Options{
	//	    InProcessMCPServers: map[string]*kit.MCPServer{
	//	        "docs": mcpSrv,
	//	    },
	//	})
	InProcessMCPServers map[string]*MCPServer

	// Compaction
	AutoCompact       bool               // Auto-compact when near context limit
	CompactionOptions *CompactionOptions // Config for auto-compaction (nil = defaults)

	// Debug enables debug logging for the SDK. When DebugLogger is nil this
	// flag selects between the default no-op SimpleDebugLogger (Debug=false)
	// and the built-in console/buffered logger (Debug=true). When DebugLogger
	// is non-nil this flag is ignored — the supplied logger's
	// IsDebugEnabled() controls whether downstream code emits messages.
	Debug bool

	// DebugLogger, if non-nil, routes low-level debug output from the engine
	// and the MCP tool plumbing to a caller-supplied implementation. This is
	// the SDK escape hatch for embedders that want to forward debug output
	// into their own logging system (zap, slog, log/charm, an in-app TUI
	// panel, etc.) instead of the built-in console logger.
	//
	// When nil (default) the Debug bool controls whether the built-in logger
	// is installed. When non-nil this logger is used unconditionally and the
	// Debug bool is ignored; the supplied logger's IsDebugEnabled() reports
	// whether downstream code should bother formatting messages.
	DebugLogger DebugLogger

	// MCPAuthHandler handles OAuth authorization for remote MCP servers.
	// When set, remote transports (streamable HTTP, SSE) are configured
	// with OAuth support. If the server returns a 401, the handler is
	// invoked to let the user authorize.
	//
	// If nil, OAuth is disabled: remote MCP servers requiring authorization
	// will fail to connect and the underlying authorization-required error
	// is surfaced to the caller. The SDK deliberately does not construct a
	// default handler — doing so would bind a local TCP port and trigger
	// presentation I/O (browser open, stderr writes) without the consumer
	// opting in, which is wrong for library, daemon, or web-app embedders.
	//
	// CLI consumers: pass [NewCLIMCPAuthHandler] to get the standard
	// "open browser + print status" behavior.
	//
	// Custom UX: implement [MCPAuthHandler] directly, or use
	// [DefaultMCPAuthHandler] and set its OnAuthURL hook to plug in your
	// own presentation (TUI modal, QR code, web redirect, etc.).
	MCPAuthHandler MCPAuthHandler

	// MCPTokenStoreFactory, if non-nil, is called to create a token store for
	// each remote MCP server that requires OAuth. The factory receives the
	// server's URL and returns a [MCPTokenStore] implementation.
	//
	// When nil (default), tokens are persisted to a JSON file at
	// $XDG_CONFIG_HOME/.kit/mcp_tokens.json (or ~/.config/.kit/mcp_tokens.json).
	//
	// Use this to store tokens in a database, encrypt them, keep them
	// in-memory, or write them to a custom file path.
	MCPTokenStoreFactory MCPTokenStoreFactory

	// OnMCPServerLoaded, if non-nil, is called when each MCP server finishes
	// loading during Kit initialization. The callback receives the server name,
	// tool count, and any error. Called from a background goroutine; safe to
	// call app.NotifyMCPServerLoaded() from within the callback to display
	// real-time progress in the TUI.
	OnMCPServerLoaded func(serverName string, toolCount int, err error)

	// MCPTaskMode overrides the per-server [MCPTaskMode] for task-augmented
	// tools/call execution. Keys are MCP server names. Servers not present
	// in the map fall back to the TasksMode field of MCPServerConfig (or
	// MCPTaskModeAuto when that is empty). See the MCP Tasks spec for the
	// underlying semantics:
	// https://modelcontextprotocol.io/specification/2025-11-25/basic/utilities/tasks
	MCPTaskMode map[string]MCPTaskMode

	// MCPTaskTimeout is the maximum wall-clock duration to wait for a
	// task-augmented tool call to reach a terminal state. Independent of
	// any per-call context deadline; whichever fires first wins. Zero
	// means use the default (15 minutes).
	MCPTaskTimeout time.Duration

	// MCPTaskTTL is the TTL hint sent in TaskParams for every
	// task-augmented tools/call. Zero omits the TTL and lets the server
	// pick its own retention policy.
	MCPTaskTTL time.Duration

	// MCPTaskPollInterval is the fallback interval between tasks/get
	// requests when the server does not suggest one. Zero means use the
	// default (1 second).
	MCPTaskPollInterval time.Duration

	// MCPTaskMaxPollInterval caps the polling interval (a server-supplied
	// pollInterval can otherwise grow without bound). Zero means use the
	// default (5 seconds).
	MCPTaskMaxPollInterval time.Duration

	// MCPTaskProgress, if non-nil, is invoked once when a task is accepted
	// and on every status transition observed by the polling loop. The
	// final invocation always carries a terminal status. Implementations
	// must not block; long work should run on a goroutine.
	MCPTaskProgress MCPTaskProgressHandler

	// CLI is optional CLI-specific configuration. SDK users leave this nil.
	CLI *CLIOptions

	// SessionManager allows custom session storage backends.
	// If nil (default), Kit uses the built-in file-based TreeManager.
	// When provided, SessionPath, Continue, and NoSession options are ignored.
	SessionManager SessionManager
}

// CLIOptions holds fields only relevant to the CLI binary. SDK users should
// not need these; they are separated to keep the main Options struct clean.
type CLIOptions struct {
	// MCPConfig is a pre-loaded MCP config. When set, LoadAndValidateConfig
	// is skipped during Kit creation.
	MCPConfig *config.Config
	// ShowSpinner shows a loading spinner for Ollama models.
	ShowSpinner bool
	// SpinnerFunc provides the spinner implementation (nil = no spinner).
	SpinnerFunc SpinnerFunc
	// UseBufferedLogger buffers debug messages for later display.
	UseBufferedLogger bool
	// ProgressReaderFunc wraps an io.Reader with a progress display for
	// long-running operations such as Ollama model pulls. The returned
	// io.ReadCloser must be closed when done. When nil, progress is not
	// displayed.
	ProgressReaderFunc func(io.Reader) io.ReadCloser
}

// InitTreeSession creates or opens a tree session based on the given options.
// Both kit.New() and the CLI use this function so session initialisation
// logic lives in one place.
//
// Behaviour based on Options:
//   - NoSession:   in-memory tree session (no persistence)
//   - Continue:    resume most recent session for SessionDir (or cwd)
//   - SessionPath: open a specific JSONL session file
//   - default:     create a new tree session for SessionDir (or cwd)
func InitTreeSession(opts *Options) (*TreeManager, error) {
	if opts == nil {
		opts = &Options{}
	}

	sessionDir := opts.SessionDir
	if sessionDir == "" {
		sessionDir, _ = os.Getwd()
	}

	if opts.NoSession {
		return session.InMemoryTreeSession(sessionDir), nil
	}

	if opts.Continue {
		return session.ContinueRecent(sessionDir)
	}

	if opts.SessionPath != "" {
		return session.OpenTreeSession(opts.SessionPath)
	}

	// Default: create a new tree session for the working directory.
	return session.CreateTreeSession(sessionDir)
}

// New creates a Kit instance using the same initialization as the CLI.
// It loads configuration, initializes MCP servers, creates the LLM model, and
// sets up the agent for interaction. Returns an error if initialization fails.
//
// Config isolation: New constructs a per-instance configuration store (via
// viper.New internally) and applies [Options] to it. Two Kits constructed in
// the same process are therefore fully isolated — neither overwrites the
// other's model, thinking level, or generation parameters, and runtime
// mutators ([Kit.SetModel], [Kit.SetThinkingLevel]) only affect the owning
// instance. This makes subagent spawning and multi-Kit embedding safe without
// any external synchronization.
//
// CLI integration: when Options.CLI is non-nil the Kit shares the
// process-global viper store instead of allocating a fresh one, so cobra flag
// bindings established by the CLI remain in effect. SDK callers leave
// Options.CLI nil and always get an isolated store.
//
// For an ergonomic functional-options front door, see [NewAgent].
func New(ctx context.Context, opts *Options) (*Kit, error) {
	if opts == nil {
		opts = &Options{}
	}

	// Construct this Kit's configuration store. SDK callers get a fresh,
	// isolated *viper.Viper so concurrent constructions never clobber each
	// other. The CLI (Options.CLI != nil) shares the process-global store so
	// its cobra flag bindings and pre-loaded config remain visible.
	var v *viper.Viper
	if opts.CLI != nil {
		v = viper.GetViper()
	} else {
		v = viper.New()
	}

	var (
		providerConfig        *models.ProviderConfig
		modelString           string
		cwd                   string
		contextFiles          []*ContextFile
		loadedSkills          []*Skill
		namedAgents           []*AgentDefinition
		mcpConfig             *config.Config
		debug                 bool
		noExtensions          bool
		toolList              []string
		maxSteps              int
		streaming             bool
		hasCustomSystemPrompt bool
		systemPromptSource    string
		capturedBasePrompt    string
	)

	if err := func() error {
		// Set CLI-equivalent defaults on the instance store. When used as an
		// SDK (without cobra), these defaults are not registered via flag bindings.
		setSDKDefaults(v)

		// Initialize config (loads config files and env vars) into the instance
		// store. The CLI shares the process-global store, which cobra.OnInitialize
		// has already populated, so re-running initConfig there is unnecessary;
		// SDK callers get a fresh isolated store that must be loaded here.
		// We key off opts.CLI (not a config value) because setSDKDefaults always
		// seeds "model", which would otherwise mask an empty store.
		// SkipConfig bypasses .kit.yml file loading (viper defaults and env vars still apply).
		if !opts.SkipConfig && opts.CLI == nil {
			if err := initConfig(v, opts.ConfigFile, false); err != nil {
				return fmt.Errorf("failed to initialize config: %w", err)
			}
		}

		// Handle CLI debug mode.
		if opts.Debug {
			v.Set("debug", true)
		}

		// Override instance settings with options.
		if opts.Model != "" {
			v.Set("model", opts.Model)
		}
		if opts.SystemPrompt != "" {
			v.Set("system-prompt", opts.SystemPrompt)
		}
		if opts.MaxSteps > 0 {
			v.Set("max-steps", opts.MaxSteps)
		}
		// Only override streaming when the caller explicitly set it. Otherwise
		// leave the precedence chain (env → config → default true) untouched so a
		// zero-valued Options does not silently force stream=false.
		if opts.Streaming != nil {
			v.Set("stream", *opts.Streaming)
		}

		// Generation parameter overrides. Each Options field, when set,
		// is pushed into the instance store here so the existing downstream
		// code (BuildProviderConfig, SetModel, modelSettings lookups) picks
		// it up uniformly. Pointer-typed sampling params use Set only when
		// non-nil so that nil means "leave provider/per-model default in
		// place" (BuildProviderConfig keys off IsSet).
		if opts.MaxTokens > 0 {
			v.Set("max-tokens", opts.MaxTokens)
		}
		if opts.ThinkingLevel != "" {
			v.Set("thinking-level", opts.ThinkingLevel)
		}
		if opts.Temperature != nil {
			v.Set("temperature", *opts.Temperature)
		}
		if opts.TopP != nil {
			v.Set("top-p", *opts.TopP)
		}
		if opts.TopK != nil {
			v.Set("top-k", *opts.TopK)
		}
		if opts.FrequencyPenalty != nil {
			v.Set("frequency-penalty", *opts.FrequencyPenalty)
		}
		if opts.PresencePenalty != nil {
			v.Set("presence-penalty", *opts.PresencePenalty)
		}

		// Provider overrides. TLSSkipVerify only takes effect when true —
		// callers wanting to force-disable should use the config file or
		// env var instead.
		if opts.ProviderAPIKey != "" {
			v.Set("provider-api-key", opts.ProviderAPIKey)
		}
		if opts.ProviderURL != "" {
			v.Set("provider-url", opts.ProviderURL)
		}
		if opts.TLSSkipVerify {
			v.Set("tls-skip-verify", true)
		}

		// Resolve working directory for context/skill discovery.
		cwd = opts.SessionDir
		if cwd == "" {
			cwd, _ = os.Getwd()
		}

		// Load context files (AGENTS.md) from the project root.
		if !opts.NoContextFiles {
			contextFiles = loadContextFiles(cwd)
		}

		// Load skills — either from explicit paths or via auto-discovery.
		// Merge viper config with opts: CLI flag / config file values are
		// already bound to viper by cmd/root.go, so v.GetBool("no-skills"),
		// v.GetStringSlice("skill"), and v.GetString("skills-dir") capture
		// both --flag and .kit.yml keys transparently.
		noSkills := opts.NoSkills || v.GetBool("no-skills")
		skillPaths := opts.Skills
		if len(skillPaths) == 0 {
			skillPaths = v.GetStringSlice("skill")
		}
		skillsDir := opts.SkillsDir
		if skillsDir == "" {
			skillsDir = v.GetString("skills-dir")
		}
		if !noSkills {
			mergedOpts := *opts
			mergedOpts.Skills = skillPaths
			mergedOpts.SkillsDir = skillsDir
			var err error
			loadedSkills, err = loadSkills(&mergedOpts)
			if err != nil {
				return fmt.Errorf("failed to load skills: %w", err)
			}

			// Apply per-skill disable list (--skill-disable / skill-disable
			// config key). Disabled skills stay loaded (so /skill: still
			// works) but are hidden from the model-facing catalog.
			disable := opts.SkillsDisable
			if len(disable) == 0 {
				disable = v.GetStringSlice("skill-disable")
			}
			applySkillDisableList(loadedSkills, disable)
		}

		// Discover named agent definitions (built-ins + .agents/agents/,
		// .kit/agents/, ~/.config/kit/agents/). They are advertised in the
		// subagent tool description and resolvable via SubagentConfig.Agent.
		// Per-file parse failures are non-fatal: usable agents still load and
		// a warning is printed unless quiet.
		if !opts.NoAgents && !v.GetBool("no-agents") {
			var agErr error
			namedAgents, agErr = LoadAgentDefinitions(cwd)
			if agErr != nil && !opts.Quiet {
				fmt.Fprintf(os.Stderr, "Warning: failed to load some agent definitions: %v\n", agErr)
			}
		}

		// Always compose the system prompt with runtime context: base prompt +
		// AGENTS.md context + skills metadata + date/cwd.
		//
		// If the configured model has a per-model system prompt (via
		// modelSettings or customModels params) and the user hasn't
		// explicitly set system-prompt, use the per-model prompt as the
		// base instead of the global default.
		{
			rawPromptInput := v.GetString("system-prompt")

			// Resolve a file path to its content so PromptBuilder receives the
			// actual prompt text rather than a literal path string. Without this,
			// when system-prompt is set to a file path in the config file or via
			// --system-prompt, the path itself becomes the effective system prompt
			// sent to the model (LoadSystemPrompt only ran later, after viper had
			// been overwritten with the augmented base text).
			basePrompt, _ := config.LoadSystemPrompt(rawPromptInput)
			if basePrompt == "" {
				basePrompt = rawPromptInput
			}

			// Track whether the user explicitly configured a custom system
			// prompt. When they haven't (basePrompt is the built-in default
			// or empty), per-model system prompts can replace it on switch.
			userSetSystemPrompt := basePrompt != "" && basePrompt != defaultSystemPrompt
			hasCustomSystemPrompt = userSetSystemPrompt
			if hasCustomSystemPrompt {
				systemPromptSource = rawPromptInput
			}

			// Check for per-model system prompt override when no explicit
			// global system-prompt was configured by the user.
			if !userSetSystemPrompt {
				modelStr := v.GetString("model")
				if modelStr != "" {
					if mi := models.LookupModelForSettings(modelStr); mi != nil {
						var perModelParams *models.GenerationParams
						// modelSettings takes priority over custom model params.
						if ms := models.LoadModelSettingsFrom(v); ms != nil {
							perModelParams = ms[modelStr]
						}
						if perModelParams == nil && mi.Params != nil {
							perModelParams = mi.Params
						}
						if perModelParams != nil && perModelParams.SystemPrompt != "" {
							basePrompt = models.LoadSystemPromptValue(perModelParams.SystemPrompt)
						}
					}
				}
			}

			pb := skills.NewPromptBuilder(basePrompt)

			// Capture the resolved base prompt so RefreshSystemPrompt can
			// recompose later after runtime skill/context-file mutations.
			capturedBasePrompt = basePrompt

			// Inject AGENTS.md content as project context.
			for _, cf := range contextFiles {
				pb.WithSection("", fmt.Sprintf("Instructions from: %s\n\n%s", cf.Path, cf.Content))
			}

			// Inject skills metadata (name + description + location).
			if len(loadedSkills) > 0 {
				pb.WithSkills(loadedSkills)
			}

			// Append current date/time and working directory.
			pb.WithSection("", fmt.Sprintf(
				"Current date and time: %s\nCurrent working directory: %s",
				time.Now().Format("Monday, January 2, 2006, 3:04:05 PM MST"), cwd,
			))

			v.Set("system-prompt", pb.Build())
		}

		// Snapshot all instance-derived values now.
		// BuildProviderConfig is fast (pure reads).
		var pcErr error
		providerConfig, _, pcErr = kitsetup.BuildProviderConfig(v)
		if pcErr != nil {
			return fmt.Errorf("failed to build provider config: %w", pcErr)
		}

		// SDK last-resort max-tokens floor. When nothing — Options, env,
		// config, nor a per-model default — supplied a value, we land on
		// zero here (GetInt returns 0 for unset keys). Apply the
		// SDK default directly on the struct rather than via the store so
		// IsSet("max-tokens") stays false: downstream right-sizing
		// can still raise this toward the model's known output ceiling,
		// and per-model modelSettings[...].maxTokens can still win.
		if providerConfig.MaxTokens == 0 && opts.MaxTokens == 0 {
			providerConfig.MaxTokens = sdkDefaultMaxTokens
		}
		modelString = v.GetString("model")
		debug = v.GetBool("debug")
		noExtensions = opts.NoExtensions || v.GetBool("no-extensions")
		toolList = opts.CoreToolList
		if toolList == nil {
			var err error
			toolList, err = CoreToolFilterHelper(v)
			if err != nil {
				return err
			}
		}
		toolList = handleCoreToolList(toolList, opts.DisableCoreTools || v.GetBool("no-core-tools"))
		maxSteps = v.GetInt("max-steps")
		streaming = v.GetBool("stream")

		return nil
	}(); err != nil {
		return nil, err
	}
	// ---- config snapshot complete — heavy I/O below ----

	// Load MCP configuration. Use pre-loaded config if provided directly,
	// via CLI options, or load from the instance store as a last resort.
	if opts.MCPConfig != nil {
		mcpConfig = opts.MCPConfig
	} else if opts.CLI != nil && opts.CLI.MCPConfig != nil {
		mcpConfig = opts.CLI.MCPConfig
	}
	if mcpConfig == nil {
		var err error
		mcpConfig, err = config.LoadAndValidateConfigFrom(v)
		if err != nil {
			return nil, fmt.Errorf("failed to load MCP config: %w", err)
		}
	}

	// Merge in-process MCP servers from Options into the MCP config.
	// These are programmatically-provided *server.MCPServer instances that
	// bypass subprocess spawning and network I/O.
	if len(opts.InProcessMCPServers) > 0 {
		if mcpConfig.MCPServers == nil {
			mcpConfig.MCPServers = make(map[string]config.MCPServerConfig, len(opts.InProcessMCPServers))
		}
		for name, srv := range opts.InProcessMCPServers {
			mcpConfig.MCPServers[name] = config.MCPServerConfig{
				Type:            "inprocess",
				InProcessServer: srv,
			}
		}
	}

	// Pre-create hook registries so the tool wrapper can reference them.
	// Hooks registered after New() returns are still invoked because the
	// wrapper captures the registries by pointer.
	beforeToolCall := newHookRegistry[BeforeToolCallHook, BeforeToolCallResult]()
	afterToolResult := newHookRegistry[AfterToolResultHook, AfterToolResultResult]()
	beforeTurn := newHookRegistry[BeforeTurnHook, BeforeTurnResult]()
	afterTurn := newHookRegistry[AfterTurnHook, AfterTurnResult]()
	contextPrepare := newHookRegistry[ContextPrepareHook, ContextPrepareResult]()
	beforeCompact := newHookRegistry[BeforeCompactHook, BeforeCompactResult]()
	prepareStep := newHookRegistry[PrepareStepHook, PrepareStepResult]()

	// Build agent setup options, pulling CLI-specific fields when available.
	// Pass the pre-built ProviderConfig and scalar viper snapshots so
	// SetupAgent doesn't need to re-read viper (which would require the lock).

	// Register the dedicated activate_skill tool when at least one skill is
	// loaded (issue #65, gaps #13/#14). The provider closure reads the live
	// skill set from the Kit instance once it exists so runtime additions
	// resolve; skillToolKit is assigned after construction below.
	var skillToolKit *Kit
	extraTools := opts.ExtraTools
	if len(loadedSkills) > 0 {
		names := make([]string, 0, len(loadedSkills))
		for _, s := range loadedSkills {
			if !s.DisableModelInvocation {
				names = append(names, s.Name)
			}
		}
		provider := func() []*skills.Skill {
			if skillToolKit == nil {
				return loadedSkills
			}
			return skillToolKit.GetSkills()
		}
		if t := skilltool.New(names, provider); t != nil {
			extraTools = append(extraTools, t)
		}
	}

	setupOpts := kitsetup.AgentSetupOptions{
		MCPConfig:         mcpConfig,
		Quiet:             opts.Quiet,
		CoreTools:         opts.Tools,
		CoreToolList:      toolList,
		ExtraTools:        extraTools,
		NamedAgents:       namedAgentSpecs(namedAgents),
		ToolWrapper:       hookToolWrapper(beforeToolCall, afterToolResult),
		ProviderConfig:    providerConfig,
		Debug:             debug,
		DebugLogger:       opts.DebugLogger,
		NoExtensions:      noExtensions,
		MaxSteps:          maxSteps,
		StreamingEnabled:  streaming,
		OnMCPServerLoaded: opts.OnMCPServerLoaded,
		MCPTaskConfig: mcpTaskOptions{
			perServer:       opts.MCPTaskMode,
			defaultTTL:      opts.MCPTaskTTL,
			pollInterval:    opts.MCPTaskPollInterval,
			maxPollInterval: opts.MCPTaskMaxPollInterval,
			timeout:         opts.MCPTaskTimeout,
			progress:        opts.MCPTaskProgress,
		}.toToolsConfig(),
		Viper: v,
	}

	// Set up OAuth handler for remote MCP servers. The SDK does not create
	// a default handler: auto-construction would bind a local TCP port and
	// (historically) shell out to a browser without the consumer asking,
	// which is a surprise for library/daemon/web-app embedders. Consumers
	// that want CLI behavior pass a [CLIMCPAuthHandler] explicitly; other
	// consumers implement [MCPAuthHandler] themselves. If nil, remote MCP
	// servers requiring OAuth will fail to connect with the underlying
	// authorization-required error surfaced to the caller.
	//
	// The SDK MCPAuthHandler interface is structurally identical to
	// tools.MCPAuthHandler, so any implementation satisfies both.
	if opts.MCPAuthHandler != nil {
		setupOpts.AuthHandler = opts.MCPAuthHandler
	}

	// Set up custom token store factory for MCP OAuth tokens.
	// The SDK MCPTokenStoreFactory is structurally identical to
	// tools.TokenStoreFactory, so it can be assigned directly.
	if opts.MCPTokenStoreFactory != nil {
		setupOpts.TokenStoreFactory = tools.TokenStoreFactory(opts.MCPTokenStoreFactory)
	}

	if opts.CLI != nil {
		setupOpts.ShowSpinner = opts.CLI.ShowSpinner
		setupOpts.SpinnerFunc = agent.SpinnerFunc(opts.CLI.SpinnerFunc)
		setupOpts.UseBufferedLogger = opts.CLI.UseBufferedLogger
		if opts.CLI.ProgressReaderFunc != nil {
			providerConfig.ProgressReaderFunc = opts.CLI.ProgressReaderFunc
		}
	}

	// Create agent using shared setup with the hook tool wrapper.
	agentResult, err := kitsetup.SetupAgent(ctx, setupOpts)
	if err != nil {
		return nil, err
	}

	// Initialize session manager.
	var sessionManager SessionManager
	if opts.SessionManager != nil {
		// Use custom session manager provided by user.
		sessionManager = opts.SessionManager
	} else {
		// DEFAULT: Use built-in TreeManager (existing behavior).
		treeSession, err := InitTreeSession(opts)
		if err != nil {
			_ = agentResult.Agent.Close()
			return nil, fmt.Errorf("failed to initialize session: %w", err)
		}
		// Wrap TreeManager in adapter to satisfy SessionManager interface.
		sessionManager = NewTreeManagerAdapter(treeSession)
	}

	k := &Kit{
		agent:                 agentResult.Agent,
		session:               sessionManager,
		modelString:           modelString,
		events:                newEventBus(),
		autoCompact:           opts.AutoCompact,
		compactionOpts:        opts.CompactionOptions,
		contextFiles:          contextFiles,
		skills:                loadedSkills,
		namedAgents:           namedAgents,
		extRunner:             agentResult.ExtRunner,
		bufferedLogger:        agentResult.BufferedLogger,
		authHandler:           setupOpts.AuthHandler,
		opts:                  opts,
		mcpConfig:             mcpConfig,
		v:                     v,
		hasCustomSystemPrompt: hasCustomSystemPrompt,
		systemPromptSource:    systemPromptSource,
		basePrompt:            capturedBasePrompt,
		beforeToolCall:        beforeToolCall,
		afterToolResult:       afterToolResult,
		beforeTurn:            beforeTurn,
		afterTurn:             afterTurn,
		contextPrepare:        contextPrepare,
		beforeCompact:         beforeCompact,
		prepareStep:           prepareStep,
		runtimeExtraTools:     append([]Tool(nil), extraTools...),
	}

	// Ensure the agent's extra-tool list reflects the current extension tools
	// plus the runtime native tools captured above.
	k.recomposeExtraTools()

	// Point the activate_skill provider closure at the live Kit instance so it
	// resolves skills mutated after construction.
	skillToolKit = k

	// Bridge extension events to SDK hooks.
	if agentResult.ExtRunner != nil {
		k.bridgeExtensions(agentResult.ExtRunner)

		// Initialize extension context with minimal defaults. SDK users can call
		// Extensions().SetContext to override with richer implementations (TUI callbacks,
		// prompts, etc.). This ensures extensions never crash on nil function fields.
		k.Extensions().SetContext(extensions.Context{
			CWD:         cwd,
			Model:       k.modelString,
			Interactive: false, // SDK mode defaults to non-interactive
		})
	}

	return k, nil
}

// GetContextFiles returns the context files (e.g. AGENTS.md) currently active
// on this Kit instance. The returned slice is a snapshot — mutating it does
// not affect Kit state. Returns nil when no context files are loaded.
func (m *Kit) GetContextFiles() []*ContextFile {
	m.runtimeMu.RLock()
	defer m.runtimeMu.RUnlock()
	if len(m.contextFiles) == 0 {
		return nil
	}
	out := make([]*ContextFile, len(m.contextFiles))
	copy(out, m.contextFiles)
	return out
}

// GetSkills returns the skills currently active on this Kit instance. The
// returned slice is a snapshot — mutating it does not affect Kit state.
// Returns nil when no skills are loaded.
func (m *Kit) GetSkills() []*Skill {
	m.runtimeMu.RLock()
	defer m.runtimeMu.RUnlock()
	if len(m.skills) == 0 {
		return nil
	}
	out := make([]*Skill, len(m.skills))
	copy(out, m.skills)
	return out
}

// ---------------------------------------------------------------------------
// Context file loading
// ---------------------------------------------------------------------------

// loadContextFiles discovers and loads project context files (AGENTS.md) from
// the working directory. Returns nil if no context file is found.
func loadContextFiles(cwd string) []*ContextFile {
	path := filepath.Join(cwd, "AGENTS.md")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return []*ContextFile{{
		Path:    path,
		Content: strings.TrimSpace(string(data)),
	}}
}

// ---------------------------------------------------------------------------
// Skill command expansion
// ---------------------------------------------------------------------------

// expandSkillCommand checks whether prompt starts with "/skill:<name>" and, if
// so, re-reads the skill file, strips its YAML frontmatter, wraps the body in
// a <skill> block with baseDir metadata, and appends any trailing user args.
// Returns the original text unchanged when the prefix is absent or the skill is
// not found.
func (m *Kit) expandSkillCommand(prompt string) string {
	if !strings.HasPrefix(prompt, "/skill:") {
		return prompt
	}

	// Parse: /skill:name [args]
	rest := prompt[len("/skill:"):]
	name, args, _ := strings.Cut(rest, " ")
	name = strings.TrimSpace(name)
	if name == "" {
		return prompt
	}

	// Find the skill by name.
	var skillPath string
	m.runtimeMu.RLock()
	for _, s := range m.skills {
		if s.Name == name {
			skillPath = s.Path
			break
		}
	}
	m.runtimeMu.RUnlock()
	if skillPath == "" {
		return prompt
	}

	// Re-read the file for freshness (user may have edited it since startup).
	loaded, err := skills.LoadSkill(skillPath)
	if err != nil {
		return prompt
	}

	baseDir := filepath.Dir(loaded.Path)
	var buf strings.Builder
	fmt.Fprintf(&buf, "<skill name=%q location=%q>\n", loaded.Name, loaded.Path)
	fmt.Fprintf(&buf, "References are relative to %s.\n\n", baseDir)
	buf.WriteString(loaded.Content)

	// Enumerate bundled resources (scripts/, references/, assets/) so the model
	// knows what it can read without listing the directory itself.
	if res := skills.FormatResources(loaded.Resources()); res != "" {
		buf.WriteString("\n\n")
		buf.WriteString(res)
	}

	buf.WriteString("\n</skill>")

	args = strings.TrimSpace(args)
	if args != "" {
		buf.WriteString("\n\n")
		buf.WriteString(args)
	}

	return buf.String()
}

// ---------------------------------------------------------------------------
// Skills loading
// ---------------------------------------------------------------------------

// loadSkills loads skills based on Options. If explicit paths are provided
// they are loaded directly. If SkillsDir is set it is treated as a direct
// skills directory (scanned as-is, not as a parent of .agents/.kit). Otherwise
// auto-discovery runs against the standard scopes rooted at SessionDir.
func loadSkills(opts *Options) ([]*skills.Skill, error) {
	if len(opts.Skills) > 0 {
		return loadExplicitSkills(opts.Skills)
	}

	// An explicit --skills-dir is a direct skills directory: scan it as-is
	// rather than appending .agents/skills and .kit/skills beneath it.
	if opts.SkillsDir != "" {
		return skills.LoadSkillsFromDir(opts.SkillsDir)
	}

	// Auto-discover from the standard scopes rooted at the session directory.
	// Project-local skills are injected into the system prompt, so they are
	// gated on a trust decision when a SkillTrustPrompt is configured.
	cwd := opts.SessionDir
	if cwd == "" {
		cwd, _ = os.Getwd()
	}
	user := skills.LoadUserSkills()
	project := skills.LoadProjectSkills(cwd)
	if len(project) > 0 && !projectSkillsTrusted(opts, cwd, len(project)) {
		project = nil
	}
	return skills.Combine(user, project), nil
}

// loadExplicitSkills loads skills from a list of explicit paths. Each path
// can be a file or a directory.
func loadExplicitSkills(paths []string) ([]*skills.Skill, error) {
	seen := make(map[string]bool)
	var all []*skills.Skill

	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			return nil, fmt.Errorf("skill path %s: %w", p, err)
		}

		if info.IsDir() {
			dirSkills, err := skills.LoadSkillsFromDir(p)
			if err != nil {
				return nil, err
			}
			for _, s := range dirSkills {
				if !seen[s.Path] {
					seen[s.Path] = true
					all = append(all, s)
				}
			}
		} else {
			abs, _ := filepath.Abs(p)
			if !seen[abs] {
				seen[abs] = true
				s, err := skills.LoadSkill(p)
				if err != nil {
					return nil, err
				}
				all = append(all, s)
			}
		}
	}

	return all, nil
}

// ---------------------------------------------------------------------------
// TurnResult
// ---------------------------------------------------------------------------

// TurnResult contains the full result of a prompt turn, including usage
// statistics and the updated conversation. Use PromptResult() instead of
// Prompt() when you need access to this data.
type TurnResult struct {
	// Response is the assistant's final text response.
	Response string

	// StopReason indicates why the turn ended. Derived from the LLM
	// provider's finish reason: FinishReasonStop, FinishReasonLength (max
	// output tokens reached), FinishReasonToolCalls, FinishReasonContentFilter,
	// FinishReasonError, FinishReasonOther, FinishReasonUnknown.
	StopReason string

	// SessionID is the UUID of the session this turn belongs to.
	SessionID string

	// TotalUsage is the aggregate token usage across all steps in the turn
	// (includes tool-calling loop iterations). Nil if the provider didn't
	// report usage.
	TotalUsage *LLMUsage

	// FinalUsage is the token usage from the last API call only. For context
	// window fill, sum all categories: InputTokens + CacheReadTokens +
	// CacheCreationTokens + OutputTokens. With prompt caching, InputTokens
	// alone understates the context (cached tokens are reported separately).
	// Nil if unavailable.
	FinalUsage *LLMUsage

	// Messages is the full updated conversation after the turn, including
	// any tool call/result messages added during the agent loop.
	// Each message carries role and plain-text content.
	Messages []LLMMessage

	// FinalValue is set when a tool returned a [ToolOutput] with Halt=true
	// during the turn. The dynamic type is whatever the tool handler placed
	// in [ToolOutput.FinalValue]. Nil when no tool halted the turn.
	FinalValue any

	// HaltedByTool is the name of the tool that returned Halt=true, or empty
	// if the turn ended for any other reason.
	HaltedByTool string

	// Stream contains every delta event observed during the turn in emit
	// order. It is populated regardless of streaming mode (in non-streaming
	// mode it carries the coarse-grained events the provider reported).
	// PromptResult and the other turn-returning entry points always block
	// until end-of-turn, so Stream is complete when they return.
	Stream []StreamEvent
}

// StreamEventKind classifies a [StreamEvent] captured during a turn.
type StreamEventKind string

// Stream event kinds captured in [TurnResult.Stream].
const (
	StreamEventTextDelta      StreamEventKind = "text_delta"
	StreamEventReasoningStart StreamEventKind = "reasoning_start"
	StreamEventReasoningDelta StreamEventKind = "reasoning_delta"
	StreamEventReasoningEnd   StreamEventKind = "reasoning_end"
	StreamEventToolCallChunk  StreamEventKind = "tool_call_chunk"
)

// StreamEvent is a single delta observed during a turn, captured in
// [TurnResult.Stream]. It lets embedders assert streamed ordering
// deterministically without re-implementing an OnMessageUpdate collector.
type StreamEvent struct {
	// Kind classifies the event.
	Kind StreamEventKind

	// Text carries the assistant text for StreamEventTextDelta.
	Text string

	// Reasoning carries the reasoning text for StreamEventReasoningDelta.
	Reasoning string

	// ToolName is the tool name for StreamEventToolCallChunk.
	ToolName string

	// ToolID is the tool call ID for StreamEventToolCallChunk.
	ToolID string

	// Args carries the (accumulating) tool-call argument JSON for
	// StreamEventToolCallChunk.
	Args string
}

// ---------------------------------------------------------------------------
// In-process subagent
// ---------------------------------------------------------------------------

// SubagentConfig configures an in-process subagent spawned via Kit.Subagent().
type SubagentConfig struct {
	// Prompt is the task/instruction for the subagent (required).
	Prompt string

	// Agent optionally names a discovered agent definition (see
	// [Kit.GetAgents]) whose presets — model, system prompt, tool
	// allowlist, temperature, and timeout — apply as defaults. Explicitly
	// set scalar fields on this struct (Model, SystemPrompt, Timeout,
	// Temperature) override the definition's values. Tools is the
	// exception: when the definition declares a tool allowlist, Tools acts
	// as the base set and is intersected with the allowlist — it can narrow
	// the agent's tool access but never widen it. An unknown name is an
	// error.
	Agent string

	// Model overrides the parent's model (e.g. "anthropic/claude-haiku-3-5-20241022").
	// Empty string uses the parent's current model.
	Model string

	// SystemPrompt provides domain-specific instructions for the subagent.
	// Empty string uses a minimal default prompt.
	SystemPrompt string

	// Tools overrides the tool set available to the subagent.
	// If nil and the subagent is created via the SDK (Kit.Subagent()), the
	// static SubagentTools() set (all core tools except "subagent") is used.
	// When spawned internally by the agent loop, the parent's active tools
	// minus "subagent" are used instead (see GetToolsForSubagent()).
	// Pass m.GetToolsForSubagent() explicitly to opt into inheritance from
	// SDK call sites.
	// (The subagent tool is dropped to prevent infinite recursion.)
	Tools []Tool

	// NoSession, when true, uses an in-memory ephemeral session. When false
	// (default), the subagent's session is persisted and can be loaded for
	// replay/inspection.
	NoSession bool

	// Timeout limits execution time. Zero means 5 minute default.
	Timeout time.Duration

	// Temperature overrides the sampling temperature for the subagent.
	// Nil inherits the parent's effective setting.
	Temperature *float32

	// OnEvent, when set, receives all events from the subagent's event bus.
	// This enables the parent to stream subagent tool calls, text chunks,
	// etc. in real time.
	OnEvent func(Event)
}

// SubagentResult contains the outcome of an in-process subagent execution.
// Errors are returned as the error return value of Subagent(), not in this struct.
type SubagentResult struct {
	// Response is the subagent's final text response.
	Response string
	// SessionID is the subagent's session identifier (for replay).
	SessionID string
	// StopReason is the LLM's finish reason for the subagent's final turn.
	StopReason string
	// Usage contains token usage from the subagent's run.
	Usage *LLMUsage
	// Elapsed is the total execution time.
	Elapsed time.Duration
}

// inheritProviderConfig copies the parent's effective provider/runtime
// configuration from its isolated config store onto child Options. Used by
// Kit.Subagent so the child — which owns a separate store and re-loads only
// .kit.yml / KIT_* on its own — still observes provider credentials, the
// thinking level, and sampler/token overrides the parent acquired via
// programmatic Options or runtime setters (e.g. SetThinkingLevel).
//
// max-tokens and the sampling parameters are only propagated when the parent
// explicitly set them (IsSet), preserving the tri-state precedence so per-model
// defaults still apply on the child when the parent left them unset. A nil
// child or store is a no-op.
func inheritProviderConfig(child *Options, v *viper.Viper) {
	if child == nil || v == nil {
		return
	}
	child.ProviderAPIKey = v.GetString("provider-api-key")
	child.ProviderURL = v.GetString("provider-url")
	child.TLSSkipVerify = v.GetBool("tls-skip-verify")
	child.ThinkingLevel = v.GetString("thinking-level")
	if v.IsSet("max-tokens") {
		child.MaxTokens = v.GetInt("max-tokens")
	}
	if v.IsSet("temperature") {
		t := float32(v.GetFloat64("temperature"))
		child.Temperature = &t
	}
	if v.IsSet("top-p") {
		p := float32(v.GetFloat64("top-p"))
		child.TopP = &p
	}
	if v.IsSet("top-k") {
		k := int32(v.GetInt("top-k"))
		child.TopK = &k
	}
	if v.IsSet("frequency-penalty") {
		fp := float32(v.GetFloat64("frequency-penalty"))
		child.FrequencyPenalty = &fp
	}
	if v.IsSet("presence-penalty") {
		pp := float32(v.GetFloat64("presence-penalty"))
		child.PresencePenalty = &pp
	}
}

// toolsIncludeMCP reports whether the provided tool set already contains any
// of the parent's loaded MCP tools (matched by prefixed name). Used to decide
// whether a spawned subagent needs to re-load MCP servers or can rely on the
// inherited tools. Returns false when there are no MCP tool names to match.
func toolsIncludeMCP(tools []Tool, mcpNames []string) bool {
	if len(mcpNames) == 0 || len(tools) == 0 {
		return false
	}
	mcpSet := make(map[string]struct{}, len(mcpNames))
	for _, n := range mcpNames {
		mcpSet[n] = struct{}{}
	}
	for _, t := range tools {
		if _, ok := mcpSet[t.Info().Name]; ok {
			return true
		}
	}
	return false
}

// Subagent spawns an in-process child Kit instance to perform a task. The
// child gets its own session, event bus, and agent loop but shares the
// parent's config (API keys, provider settings) and defaults to the parent's
// model when SubagentConfig.Model is empty.
//
// This is the recommended way to run subagents in the SDK — no subprocess,
// no kit binary dependency, native Go types for results.
func (m *Kit) Subagent(ctx context.Context, cfg SubagentConfig) (*SubagentResult, error) {
	if cfg.Prompt == "" {
		return nil, fmt.Errorf("subagent prompt is required")
	}

	start := time.Now()

	// Resolve a named agent definition: its model, system prompt, tool
	// allowlist, temperature, and timeout act as defaults that explicitly
	// set cfg fields override. agentRestricted records whether an allowlist
	// was applied — the child must then not re-load MCP servers, which
	// would add tools beyond the allowlist.
	agentRestricted := false
	if cfg.Agent != "" {
		var err error
		agentRestricted, err = m.resolveAgentDefinition(&cfg)
		if err != nil {
			return nil, err
		}
	}

	// Default timeout.
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 5 * time.Minute
	}

	// Pre-flight check: if the incoming context is already dead, don't
	// waste time attempting init. This catches the case where the parent
	// generation loop's context was cancelled (e.g. user ESC, step cancel)
	// between when the LLM requested the subagent tool and when this code
	// runs. We replace it with a fresh context carrying only the timeout,
	// since the subagent should be independently bounded.
	if ctx.Err() != nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Resolve model: fall back to parent's model, and inherit the parent's
	// provider when only a bare model name is given (e.g. "claude-haiku"
	// instead of "anthropic/claude-haiku"). This avoids provider guessing.
	model := cfg.Model
	if model == "" {
		model = m.modelString
	} else if !strings.Contains(model, "/") {
		// Bare model name — prepend parent's provider.
		if parts := strings.SplitN(m.modelString, "/", 2); len(parts) == 2 {
			model = parts[0] + "/" + model
		}
	}

	// Early validation: check model format and provider before doing any
	// expensive work (MCP init, system prompt composition, etc.). This
	// gives the calling agent immediate feedback it can act on — e.g.
	// correcting a typo — instead of waiting for a full Kit.New() cycle
	// that silently falls back to the parent model.
	if model != m.modelString {
		if err := models.GetGlobalRegistry().ValidateModelString(model); err != nil {
			return nil, fmt.Errorf("invalid subagent model %q: %w", model, err)
		}
	}

	// Default system prompt.
	systemPrompt := cfg.SystemPrompt
	if systemPrompt == "" {
		systemPrompt = "You are a helpful coding assistant. Complete the task efficiently and thoroughly."
	}

	// Default tools: everything except subagent.
	tools := cfg.Tools
	if tools == nil {
		tools = SubagentTools()
	}

	// Decide whether the child should re-load MCP servers. When the caller
	// passes an explicit tool set that ALREADY contains the parent's loaded
	// MCP tools (the internal agent-loop spawner does this via
	// GetToolsForSubagent), re-loading MCP would spin up a second set of MCP
	// server connections in the child. The inherited MCP AgentTools are
	// closures bound to the PARENT's live tool manager, so the child can call
	// them directly through the parent's existing connections — no re-load
	// needed. Detect that case and suppress the child's MCP loading.
	//
	// Note: we must pass a non-nil config with no MCPServers rather than nil.
	// A nil MCPConfig makes New() fall back to loading .kit.yml from disk,
	// which would re-spawn exactly the servers we are trying to avoid. An
	// explicit empty config takes the "pre-loaded" branch in New() and loads
	// zero servers.
	childMCPConfig := m.mcpConfig
	if agentRestricted || (cfg.Tools != nil && toolsIncludeMCP(tools, m.GetMCPToolNames())) {
		if m.mcpConfig != nil {
			cp := *m.mcpConfig
			cp.MCPServers = nil
			childMCPConfig = &cp
		} else {
			childMCPConfig = &config.Config{}
		}
	}

	// Create child Kit instance. Pass the parent's loaded MCP config to
	// avoid re-loading and re-validating config for the child.
	// Streaming is enabled explicitly — without it, non-streaming can hit
	// provider-level differences (e.g. Anthropic non-streaming timeouts with
	// extended thinking). The child gets its own config store, so this does not
	// affect any other concurrent caller.
	streamOn := true
	childOpts := &Options{
		Model:        model,
		SystemPrompt: systemPrompt,
		Tools:        tools,
		NoSession:    cfg.NoSession,
		Quiet:        true,
		Streaming:    &streamOn,
		MCPConfig:    childMCPConfig,
	}

	// Inherit the parent's effective provider/runtime configuration. Since #40
	// each Kit owns an isolated config store, so the child's New() only re-loads
	// .kit.yml / KIT_* on its own — values the parent picked up from
	// programmatic Options or runtime setters (e.g. SetThinkingLevel) would
	// otherwise be lost.
	inheritProviderConfig(childOpts, m.v)
	// A per-subagent temperature (explicit or from a named agent definition)
	// overrides whatever the parent's config store provided.
	if cfg.Temperature != nil {
		childOpts.Temperature = cfg.Temperature
	}
	// Propagate the parent's MCP task configuration so a child subagent
	// invoking long-running MCP tools observes the same per-server modes,
	// timeouts, and progress callback as the parent. Without this, child
	// agents would silently fall back to MCPTaskModeAuto with default
	// polling and no progress feedback even when the parent had configured
	// custom values.
	inheritMCPTaskOptions(childOpts, m.opts)
	child, err := New(ctx, childOpts)
	if err != nil {
		return &SubagentResult{Elapsed: time.Since(start)}, fmt.Errorf("failed to create subagent: %w", err)
	}
	defer func() { _ = child.Close() }()

	// Forward events to parent if requested.
	if cfg.OnEvent != nil {
		child.Subscribe(cfg.OnEvent)
	}

	// Run the prompt.
	result, err := child.PromptResult(ctx, cfg.Prompt)
	elapsed := time.Since(start)

	if err != nil {
		return &SubagentResult{Elapsed: elapsed}, err
	}

	subResult := &SubagentResult{
		Response:   result.Response,
		SessionID:  child.GetSessionID(),
		StopReason: result.StopReason,
		Elapsed:    elapsed,
	}
	if result.TotalUsage != nil {
		subResult.Usage = result.TotalUsage
	}

	return subResult, nil
}

// ---------------------------------------------------------------------------
// Shared generation helpers
// ---------------------------------------------------------------------------

// generate calls the agent's generation loop with event-emitting handlers.
// All prompt modes (Prompt, Steer, FollowUp, PromptWithOptions) share this
// single code path so callback wiring is never duplicated.
func (m *Kit) generate(ctx context.Context, messages []fantasy.Message) (*agent.GenerateWithLoopResult, error) {
	// Capture the per-turn stream collector (set by runTurn) so streamed
	// deltas are recorded into TurnResult.Stream in emit order.
	collector := streamCollectorFromContext(ctx)
	// Create a per-turn steer channel and attach it to the context so the
	// agent's PrepareStep can inject steering messages between steps.
	steerCh := make(chan agent.SteerMessage, 16)
	m.steerMu.Lock()
	m.steerCh = steerCh
	m.steerMu.Unlock()
	defer func() {
		// Drain any unconsumed steer messages before nilling the channel.
		// These are stored in leftoverSteer so DrainSteer() can return them.
		var leftover []agent.SteerMessage
		for {
			select {
			case msg := <-steerCh:
				leftover = append(leftover, msg)
			default:
				m.steerMu.Lock()
				m.steerCh = nil
				m.leftoverSteer = leftover
				m.steerMu.Unlock()
				return
			}
		}
	}()
	ctx = agent.ContextWithSteerCh(ctx, steerCh)
	ctx = agent.ContextWithSteerConsumed(ctx, func(count int) {
		m.events.emit(SteerConsumedEvent{Count: count})
	})

	// Inject the in-process subagent spawner into the context so the
	// subagent core tool can create child Kit instances without
	// importing pkg/kit (which would create an import cycle).
	ctx = core.WithSubagentSpawner(ctx, func(
		spawnCtx context.Context, req core.SubagentSpawnRequest,
	) (*core.SubagentSpawnResult, error) {
		// Build OnEvent: dispatch to per-tool-call listeners if any are
		// registered via SubscribeSubagent(). Listeners are cleaned up
		// after the subagent completes.
		var onEvent func(Event)
		if listeners := m.getSubagentListenerSet(req.ToolCallID); listeners != nil {
			onEvent = listeners.emit
		}
		result, err := m.Subagent(spawnCtx, SubagentConfig{
			Prompt:       req.Prompt,
			Agent:        req.Agent,
			Model:        req.Model,
			SystemPrompt: req.SystemPrompt,
			Timeout:      req.Timeout,
			OnEvent:      onEvent,
			Tools:        m.GetToolsForSubagent(),
		})
		m.cleanupSubagentListeners(req.ToolCallID)
		if result == nil {
			return &core.SubagentSpawnResult{Error: err}, err
		}
		sr := &core.SubagentSpawnResult{
			Response:  result.Response,
			Error:     err,
			SessionID: result.SessionID,
			Elapsed:   result.Elapsed,
		}
		if result.Usage != nil {
			sr.InputTokens = result.Usage.InputTokens
			sr.OutputTokens = result.Usage.OutputTokens
		}
		return sr, err
	})

	return m.agent.GenerateWithCallbacks(ctx, messages, agent.GenerateCallbacks{
		OnToolCall: func(toolCallID, toolName, toolArgs string) {
			m.events.emit(ToolCallEvent{
				ToolCallID: toolCallID, ToolName: toolName, ToolKind: toolKindFor(toolName),
				ToolArgs: toolArgs, ParsedArgs: parseToolArgs(toolArgs),
			})
		},
		OnToolExecution: func(toolCallID, toolName, toolArgs string, isStarting bool) {
			if isStarting {
				m.events.emit(ToolExecutionStartEvent{ToolCallID: toolCallID, ToolName: toolName, ToolKind: toolKindFor(toolName), ToolArgs: toolArgs})
			} else {
				m.events.emit(ToolExecutionEndEvent{ToolCallID: toolCallID, ToolName: toolName, ToolKind: toolKindFor(toolName)})
			}
		},
		OnToolResult: func(toolCallID, toolName, toolArgs, resultText, metadata string, isError bool) {
			evt := ToolResultEvent{
				ToolCallID: toolCallID, ToolName: toolName, ToolKind: toolKindFor(toolName),
				ToolArgs: toolArgs, ParsedArgs: parseToolArgs(toolArgs),
				Result: resultText, IsError: isError,
			}
			if metadata != "" {
				var meta ToolResultMetadata
				if err := json.Unmarshal([]byte(metadata), &meta); err == nil {
					evt.Metadata = &meta
				}
			}
			m.events.emit(evt)
		},
		OnResponse: func(content string) {
			m.events.emit(ResponseEvent{Content: content})
		},
		OnToolCallContent: func(content string) {
			m.events.emit(ToolCallContentEvent{Content: content})
		},
		// <think> tag filtering: models like Qwen/DeepSeek wrap reasoning inside
		// <think>...</think> tags in the regular text stream. We intercept those
		// spans here and re-route them as ReasoningDeltaEvent/ReasoningCompleteEvent
		// so callers always receive clean, tag-free text and structured reasoning.
		OnStreamingResponse: func() func(chunk string) {
			const (
				thinkOpen  = "<think>"
				thinkClose = "</think>"
			)
			var inThinkTag bool
			return func(chunk string) {
				remaining := chunk
				for remaining != "" {
					if inThinkTag {
						i := strings.Index(remaining, thinkClose)
						if i == -1 {
							m.events.emit(ReasoningDeltaEvent{Delta: remaining})
							collector.add(StreamEvent{Kind: StreamEventReasoningDelta, Reasoning: remaining})
							return
						}
						if i > 0 {
							m.events.emit(ReasoningDeltaEvent{Delta: remaining[:i]})
							collector.add(StreamEvent{Kind: StreamEventReasoningDelta, Reasoning: remaining[:i]})
						}
						inThinkTag = false
						m.events.emit(ReasoningCompleteEvent{})
						collector.add(StreamEvent{Kind: StreamEventReasoningEnd})
						remaining = remaining[i+len(thinkClose):]
					} else {
						i := strings.Index(remaining, thinkOpen)
						if i == -1 {
							m.events.emit(MessageUpdateEvent{Chunk: remaining})
							collector.add(StreamEvent{Kind: StreamEventTextDelta, Text: remaining})
							return
						}
						if i > 0 {
							m.events.emit(MessageUpdateEvent{Chunk: remaining[:i]})
							collector.add(StreamEvent{Kind: StreamEventTextDelta, Text: remaining[:i]})
						}
						inThinkTag = true
						collector.add(StreamEvent{Kind: StreamEventReasoningStart})
						remaining = remaining[i+len(thinkOpen):]
					}
				}
			}
		}(),
		OnReasoningDelta: func(delta string) {
			m.events.emit(ReasoningDeltaEvent{Delta: delta})
			collector.add(StreamEvent{Kind: StreamEventReasoningDelta, Reasoning: delta})
		},
		OnReasoningComplete: func() {
			m.events.emit(ReasoningCompleteEvent{})
			collector.add(StreamEvent{Kind: StreamEventReasoningEnd})
		},
		OnToolOutput: func(toolCallID, toolName, chunk string, isStderr bool) {
			m.events.emit(ToolOutputEvent{
				ToolCallID: toolCallID,
				ToolName:   toolName,
				Chunk:      chunk,
				IsStderr:   isStderr,
			})
		},
		// Persist step messages incrementally so that progress survives
		// crashes and long-running turns don't lose work.
		OnStepMessages: func(stepMessages []fantasy.Message) {
			for _, msg := range stepMessages {
				_, _ = m.session.AppendMessage(msg)
			}
		},
		OnStepUsage: func(inputTokens, outputTokens, cacheReadTokens, cacheCreationTokens int64) {
			if m.v.GetBool("debug") {
				log.Printf("DEBUG Kit.generate emitting StepUsageEvent: input=%d output=%d cacheRead=%d cacheCreate=%d",
					inputTokens, outputTokens, cacheReadTokens, cacheCreationTokens,
				)
			}
			m.events.emit(StepUsageEvent{
				InputTokens:      uint64(inputTokens),
				OutputTokens:     uint64(outputTokens),
				CacheReadTokens:  uint64(cacheReadTokens),
				CacheWriteTokens: uint64(cacheCreationTokens),
			})
		},
		// Password prompt handler for sudo commands
		OnPasswordPrompt: func(prompt string) (string, bool) {
			responseCh := make(chan PasswordPromptResponse, 1)
			m.events.emit(PasswordPromptEvent{
				Prompt:     prompt,
				ResponseCh: responseCh,
			})
			resp := <-responseCh
			return resp.Password, resp.Cancelled
		},
		// Tool call argument streaming
		OnToolCallStart: func(toolCallID, toolName string) {
			m.events.emit(ToolCallStartEvent{
				ToolCallID: toolCallID,
				ToolName:   toolName,
				ToolKind:   toolKindFor(toolName),
			})
			collector.add(StreamEvent{Kind: StreamEventToolCallChunk, ToolID: toolCallID, ToolName: toolName})
		},
		OnToolCallDelta: func(toolCallID, delta string) {
			m.events.emit(ToolCallDeltaEvent{
				ToolCallID: toolCallID,
				Delta:      delta,
			})
			collector.add(StreamEvent{Kind: StreamEventToolCallChunk, ToolID: toolCallID, Args: delta})
		},
		OnToolCallEnd: func(toolCallID string) {
			m.events.emit(ToolCallEndEvent{
				ToolCallID: toolCallID,
			})
		},

		// New callbacks for previously unwired agent lifecycle events.
		OnStepStart: func(stepNumber int) {
			m.events.emit(StepStartEvent{StepNumber: stepNumber})
		},
		OnStepFinish: func(stepNumber int, hasToolCalls bool, finishReason string, usage fantasy.Usage) {
			m.events.emit(StepFinishEvent{
				StepNumber:   stepNumber,
				HasToolCalls: hasToolCalls,
				FinishReason: finishReason,
				Usage:        usage,
			})
		},
		OnTextStart: func(id string) {
			m.events.emit(TextStartEvent{ID: id})
		},
		OnTextEnd: func(id string) {
			m.events.emit(TextEndEvent{ID: id})
		},
		OnReasoningStart: func(id string) {
			m.events.emit(ReasoningStartEvent{ID: id})
		},
		OnWarnings: func(warnings []string) {
			m.events.emit(WarningsEvent{Warnings: warnings})
		},
		OnSource: func(sourceType, id, url, title string) {
			m.events.emit(SourceEvent{
				SourceType: sourceType,
				ID:         id,
				URL:        url,
				Title:      title,
			})
		},
		OnStreamFinish: func(usage fantasy.Usage, finishReason string) {
			m.events.emit(StreamFinishEvent{
				Usage:        usage,
				FinishReason: finishReason,
			})
		},
		OnError: func(err error) {
			m.events.emit(ErrorEvent{Error: err})
		},
		OnRetry: func(attempt int, err error) {
			m.events.emit(RetryEvent{Attempt: attempt, Error: err})
		},
		// PrepareStep hook — compose with steering (handled in agent layer)
		// and then run SDK consumer hooks.
		OnPrepareStep: func() agent.PrepareStepHandler {
			if !m.prepareStep.hasHooks() {
				return nil
			}
			return func(stepNumber int, messages []fantasy.Message) *agent.PrepareStepUpdate {
				hookResult := m.prepareStep.run(PrepareStepHook{
					StepNumber: stepNumber,
					Messages:   messages,
				})
				if hookResult == nil || (hookResult.Messages == nil && hookResult.ToolChoice == nil) {
					return nil
				}
				return &agent.PrepareStepUpdate{
					Messages:   hookResult.Messages,
					ToolChoice: hookResult.ToolChoice,
				}
			}
		}(),
	})
}

// runTurn is the shared lifecycle for every prompt mode:
//  1. Run BeforeTurn hooks (can modify prompt, inject messages).
//  2. Persist pre-generation messages to the tree session.
//  3. Build context from the tree (walks leaf-to-root for current branch).
//  4. Emit turn/message start events.
//  5. Run generation (messages are persisted incrementally per step).
//  6. Persist any remaining messages not covered by incremental persistence.
//  7. Emit turn/message end events.
//  8. Run AfterTurn hooks.
//
// During generation, each completed step's messages are persisted immediately
// via the onStepMessages callback. Tool calls are always persisted as
// call/response pairs (assistant + tool messages together). Reasoning and
// text-only assistant messages are persisted as soon as their step completes.
// This ensures long-running turns don't lose progress on crash or cancellation.
//
// promptLabel is the human-readable label emitted in TurnStartEvent.Prompt.
// prompt is the raw user text passed to BeforeTurn hooks.
func (m *Kit) runTurn(ctx context.Context, promptLabel string, prompt string, preMessages []fantasy.Message) (*TurnResult, error) {
	// Expand /skill:name commands — reads the skill file, wraps it in a
	// <skill> block, and appends any trailing user args.
	if expanded := m.expandSkillCommand(prompt); expanded != prompt {
		prompt = expanded
		// Replace the last user message in preMessages with the expanded text,
		// preserving any file parts (e.g. clipboard images).
		for i := len(preMessages) - 1; i >= 0; i-- {
			if preMessages[i].Role == fantasy.MessageRoleUser {
				files := extractFileParts(preMessages[i])
				preMessages[i] = fantasy.NewUserMessage(expanded, files...)
				break
			}
		}
	}

	// Run BeforeTurn hooks — can modify the prompt, inject system/context messages.
	if hookResult := m.beforeTurn.run(BeforeTurnHook{Prompt: prompt}); hookResult != nil {
		// Override prompt text in the last user message, preserving
		// any file parts (e.g. clipboard images).
		if hookResult.Prompt != nil {
			for i := len(preMessages) - 1; i >= 0; i-- {
				if preMessages[i].Role == fantasy.MessageRoleUser {
					files := extractFileParts(preMessages[i])
					preMessages[i] = fantasy.NewUserMessage(*hookResult.Prompt, files...)
					break
				}
			}
		}
		// Inject messages before the original preMessages.
		var injected []fantasy.Message
		if hookResult.SystemPrompt != nil {
			injected = append(injected, fantasy.NewSystemMessage(*hookResult.SystemPrompt))
		}
		if hookResult.InjectText != nil {
			injected = append(injected, fantasy.NewUserMessage(*hookResult.InjectText))
		}
		if len(injected) > 0 {
			preMessages = append(injected, preMessages...)
		}
	}

	// Persist pre-generation messages to session.
	for _, msg := range preMessages {
		_, _ = m.session.AppendMessage(msg)
	}

	// Auto-compact if enabled and conversation is near the context limit.
	if m.autoCompact && m.ShouldCompact() {
		_, _ = m.compactInternal(ctx, m.compactionOpts, "", true) // best-effort, automatic
	}

	// Build context from the session so only the current branch is sent.
	messages, _, _ := m.session.BuildContext()

	// Run ContextPrepare hooks — extensions can filter, reorder, or inject messages.
	if hookResult := m.contextPrepare.run(ContextPrepareHook{Messages: messages}); hookResult != nil && hookResult.Messages != nil {
		messages = hookResult.Messages
	}

	sentCount := len(messages)

	// Attach a per-turn stream collector and halt holder so generate's
	// callbacks can capture delta events (TurnResult.Stream) and tools can
	// signal loop termination (TurnResult.FinalValue / HaltedByTool).
	collector := &streamCollector{}
	holder := &haltHolder{}
	ctx = context.WithValue(ctx, streamCollectorKey{}, collector)
	ctx = context.WithValue(ctx, haltHolderKey{}, holder)

	m.events.emit(TurnStartEvent{Prompt: promptLabel})
	m.events.emit(MessageStartEvent{})

	result, err := m.generate(ctx, messages)
	if err != nil {
		// Persist any messages from completed steps that were NOT already
		// persisted incrementally by the onStepMessages callback. The agent
		// layer only includes fully-paired tool_use + tool_result messages
		// in completedStepMessages, so there are no orphaned entries that
		// would break subsequent API requests.
		if result != nil {
			newMessages := result.ConversationMessages[sentCount:]
			alreadyPersisted := result.PersistedMessageCount
			if alreadyPersisted < len(newMessages) {
				for _, msg := range newMessages[alreadyPersisted:] {
					_, _ = m.session.AppendMessage(msg)
				}
			}
		}
		m.events.emit(TurnEndEvent{Error: err})
		// Run AfterTurn hooks even on error.
		m.afterTurn.run(AfterTurnHook{Error: err})
		return nil, ClassifyProviderError(err)
	}

	responseText := result.FinalResponse.Content.Text()

	// Persist any new messages that were NOT already persisted incrementally
	// by the onStepMessages callback during generation. This handles the
	// non-streaming path (where onStepMessages is not called) and any edge
	// cases where the final response messages weren't covered by step callbacks.
	if len(result.ConversationMessages) > sentCount {
		newMessages := result.ConversationMessages[sentCount:]
		alreadyPersisted := result.PersistedMessageCount
		if alreadyPersisted < len(newMessages) {
			for _, msg := range newMessages[alreadyPersisted:] {
				_, _ = m.session.AppendMessage(msg)
			}
		}
	}

	// Store the API-reported token count so GetContextStats() matches the
	// built-in status bar. The context window is filled by all token
	// categories: non-cached input, cache reads, cache writes, and output.
	// With Anthropic prompt caching, InputTokens can be near-zero while
	// CacheReadTokens/CacheCreationTokens hold the bulk of the context.
	if result.FinalResponse != nil {
		u := result.FinalResponse.Usage
		m.lastInputTokensMu.Lock()
		m.lastInputTokens = int(u.InputTokens) + int(u.CacheReadTokens) + int(u.CacheCreationTokens) + int(u.OutputTokens)
		m.lastInputTokensMu.Unlock()
	}

	stopReason := result.StopReason

	m.events.emit(MessageEndEvent{Content: responseText})
	m.events.emit(TurnEndEvent{Response: responseText, StopReason: stopReason})

	// Run AfterTurn hooks.
	m.afterTurn.run(AfterTurnHook{Response: responseText})

	// Build TurnResult with usage stats.
	turnResult := &TurnResult{
		Response:   responseText,
		StopReason: stopReason,
		SessionID:  m.GetSessionID(),
		Messages:   result.ConversationMessages,
	}
	totalUsage := result.TotalUsage
	turnResult.TotalUsage = &totalUsage
	if result.FinalResponse != nil {
		finalUsage := result.FinalResponse.Usage
		turnResult.FinalUsage = &finalUsage
	}

	// Surface captured stream deltas and any tool-driven halt signal.
	turnResult.Stream = collector.drain()
	if halted, toolName, value := holder.snapshot(); halted {
		turnResult.HaltedByTool = toolName
		turnResult.FinalValue = value
	}

	return turnResult, nil
}

// ---------------------------------------------------------------------------
// Prompt modes
// ---------------------------------------------------------------------------

// Prompt sends a message to the agent and returns the response. The agent may
// use tools as needed to generate the response. The conversation history is
// automatically maintained in the tree session. Lifecycle events are emitted
// to all registered subscribers. Returns an error if generation fails.
func (m *Kit) Prompt(ctx context.Context, message string) (string, error) {
	result, err := m.runTurn(ctx, message, message, []fantasy.Message{
		fantasy.NewUserMessage(message),
	})
	if err != nil {
		return "", err
	}
	return result.Response, nil
}

// Steer injects a system-level instruction and triggers a new agent turn.
// Use Steer to dynamically adjust agent behavior mid-conversation without a
// visible user message — for example, changing tone, focus, or constraints.
//
// Under the hood, Steer appends a system message (the instruction) followed by
// a synthetic user message so the agent acknowledges and follows the directive.
// Both messages are persisted to the session.
func (m *Kit) Steer(ctx context.Context, instruction string) (string, error) {
	result, err := m.runTurn(ctx, "[steer] "+instruction, instruction, []fantasy.Message{
		fantasy.NewSystemMessage(instruction),
		fantasy.NewUserMessage("Please acknowledge and follow the above instruction."),
	})
	if err != nil {
		return "", err
	}
	return result.Response, nil
}

// FollowUp continues the conversation without explicit new user input.
// If text is empty, "Continue." is used as the prompt. Use FollowUp when the
// agent's previous response was truncated or you want the agent to elaborate.
//
// Returns an error if there are no previous messages in the session.
func (m *Kit) FollowUp(ctx context.Context, text string) (string, error) {
	// Verify there is conversation history to follow up on.
	if len(m.session.GetMessages()) == 0 {
		return "", fmt.Errorf("cannot follow up: no previous messages")
	}

	if text == "" {
		text = "Continue."
	}

	result, err := m.runTurn(ctx, "[follow-up]", text, []fantasy.Message{
		fantasy.NewUserMessage(text),
	})
	if err != nil {
		return "", err
	}
	return result.Response, nil
}

// InjectSteer sends a steering message into the currently active agent turn.
// The message will be injected as a user message between steps (after the
// current tool execution finishes, before the next LLM call). If no turn is
// active the message is silently dropped — callers should check IsGenerating()
// or use Prompt()/Steer() for idle-state messaging.
//
// InjectSteer is safe to call from any goroutine. Multiple calls queue
// messages in order; all pending steer messages are drained and injected
// together at the next step boundary.
//
// This is the preferred way to redirect an agent mid-turn without cancelling
// in-progress tool execution.
func (m *Kit) InjectSteer(message string) {
	m.InjectSteerWithFiles(message, nil)
}

// InjectSteerWithFiles sends a steering message with optional file attachments
// (e.g. pasted images) into the currently active agent turn. Behaves like
// InjectSteer but includes file parts in the injected user message.
func (m *Kit) InjectSteerWithFiles(message string, files []LLMFilePart) {
	m.steerMu.Lock()
	ch := m.steerCh
	m.steerMu.Unlock()
	if ch == nil {
		return
	}
	select {
	case ch <- agent.SteerMessage{Text: message, Files: files}:
	default:
		// Channel full — extremely unlikely with buffer of 16, but don't block.
	}
}

// IsGenerating returns true if an agent turn is currently in progress.
// Use this to decide between InjectSteer (mid-turn) and Prompt (new turn).
func (m *Kit) IsGenerating() bool {
	m.steerMu.Lock()
	defer m.steerMu.Unlock()
	return m.steerCh != nil
}

// DrainSteer removes and returns all unconsumed steer messages. Called after
// a turn completes so the app layer can process any steer messages that
// arrived after the last PrepareStep fired (e.g. during a text-only response
// with no tool calls, or after the agent finished its last step).
func (m *Kit) DrainSteer() []agent.SteerMessage {
	m.steerMu.Lock()
	defer m.steerMu.Unlock()

	// First check leftover messages saved when generate() returned.
	if len(m.leftoverSteer) > 0 {
		msgs := m.leftoverSteer
		m.leftoverSteer = nil
		return msgs
	}

	// If a turn is still active, drain from the live channel.
	if m.steerCh != nil {
		var msgs []agent.SteerMessage
		for {
			select {
			case msg := <-m.steerCh:
				msgs = append(msgs, msg)
			default:
				return msgs
			}
		}
	}

	return nil
}

// PromptOptions configures a single PromptWithOptions call.
type PromptOptions struct {
	// SystemMessage is prepended as a system message before the user prompt.
	// Use it to inject per-call instructions or context without permanently
	// modifying the agent's system prompt.
	SystemMessage string

	// Model overrides the agent's configured model for this call only. Empty
	// string means "use the agent's default". The previous model is restored
	// after the call returns.
	Model string

	// ThinkingLevel overrides the agent's reasoning level for this call only
	// (e.g. "off", "low", "medium", "high"). Empty string means "use the
	// agent's default". The previous level is restored after the call.
	ThinkingLevel string

	// ExtraTools are added to the effective tool set for this call only and
	// removed afterwards.
	ExtraTools []Tool

	// ProviderURL overrides the provider base URL for this call only. Useful
	// for multi-tenant embedders that resolve endpoints per request. The
	// previous value is restored after the call.
	ProviderURL string

	// ProviderAPIKey overrides the provider credential for this call only.
	// The previous value is restored after the call.
	ProviderAPIKey string
}

// applyPromptOptions applies the per-call overrides in opts to the shared
// agent state and returns a restore function that reverts every change. It
// holds promptOptsMu for the lifetime of the override window (the returned
// restore releases it), so concurrent option-driven prompts are serialized.
// On error nothing is changed and the lock is released.
func (m *Kit) applyPromptOptions(ctx context.Context, opts PromptOptions) (func(), error) {
	needsModelRebuild := opts.Model != "" || opts.ThinkingLevel != "" ||
		opts.ProviderURL != "" || opts.ProviderAPIKey != ""
	if !needsModelRebuild && len(opts.ExtraTools) == 0 {
		return func() {}, nil
	}

	m.promptOptsMu.Lock()
	var restores []func()
	restore := func() {
		for i := len(restores) - 1; i >= 0; i-- {
			restores[i]()
		}
		m.promptOptsMu.Unlock()
	}

	// Extra tools (additive) — restored by re-setting the prior slice.
	if len(opts.ExtraTools) > 0 {
		prev := m.agent.GetExtraTools()
		merged := make([]Tool, 0, len(prev)+len(opts.ExtraTools))
		merged = append(merged, prev...)
		merged = append(merged, opts.ExtraTools...)
		m.agent.SetExtraTools(merged)
		restores = append(restores, func() { m.agent.SetExtraTools(prev) })
	}

	if needsModelRebuild {
		prevModel := m.modelString
		prevThinkingSet := m.v.IsSet("thinking-level")
		prevThinking := m.v.GetString("thinking-level")
		prevURLSet := m.v.IsSet("provider-url")
		prevURL := m.v.GetString("provider-url")
		prevKeySet := m.v.IsSet("provider-api-key")
		prevKey := m.v.GetString("provider-api-key")

		if opts.ThinkingLevel != "" {
			m.v.Set("thinking-level", opts.ThinkingLevel)
		}
		if opts.ProviderURL != "" {
			m.v.Set("provider-url", opts.ProviderURL)
		}
		if opts.ProviderAPIKey != "" {
			m.v.Set("provider-api-key", opts.ProviderAPIKey)
		}

		targetModel := opts.Model
		if targetModel == "" {
			targetModel = prevModel
		}
		if err := m.SetModel(ctx, targetModel); err != nil {
			// Revert config keys we may have set, then unwind prior restores.
			restoreViperString(m.v, "thinking-level", prevThinking, prevThinkingSet)
			restoreViperString(m.v, "provider-url", prevURL, prevURLSet)
			restoreViperString(m.v, "provider-api-key", prevKey, prevKeySet)
			restore()
			return nil, err
		}
		restores = append(restores, func() {
			restoreViperString(m.v, "thinking-level", prevThinking, prevThinkingSet)
			restoreViperString(m.v, "provider-url", prevURL, prevURLSet)
			restoreViperString(m.v, "provider-api-key", prevKey, prevKeySet)
			// Use a fresh context: the rollback must complete even if the
			// caller's ctx was canceled or expired during the call, otherwise
			// the per-call model override would leak into subsequent calls.
			_ = m.SetModel(context.Background(), prevModel)
		})
	}

	return restore, nil
}

// restoreViperString restores a config key to its prior value, clearing it
// back to the unset state when it was not explicitly set before.
func restoreViperString(v *viper.Viper, key, prev string, wasSet bool) {
	if wasSet {
		v.Set(key, prev)
		return
	}
	v.Set(key, "")
}

// PromptWithOptions sends a message with per-call configuration. It behaves
// like Prompt but applies the overrides in opts (system message, model,
// thinking level, provider credentials, extra tools) for this call only,
// restoring the agent's prior state afterwards.
func (m *Kit) PromptWithOptions(ctx context.Context, msg string, opts PromptOptions) (string, error) {
	result, err := m.PromptResultWithOptions(ctx, msg, opts)
	if err != nil {
		return "", err
	}
	return result.Response, nil
}

// PromptResultWithOptions is the [TurnResult]-returning counterpart of
// PromptWithOptions. Like all turn-returning entry points it blocks until
// end-of-turn, so the returned TurnResult (including TurnResult.Stream) is
// complete when it returns. Per-call overrides in opts are applied for this
// call only and the agent's prior state is restored before returning.
func (m *Kit) PromptResultWithOptions(ctx context.Context, msg string, opts PromptOptions) (*TurnResult, error) {
	restore, err := m.applyPromptOptions(ctx, opts)
	if err != nil {
		return nil, err
	}
	defer restore()

	var preMessages []fantasy.Message
	if opts.SystemMessage != "" {
		preMessages = append(preMessages, fantasy.NewSystemMessage(opts.SystemMessage))
	}
	preMessages = append(preMessages, fantasy.NewUserMessage(msg))

	return m.runTurn(ctx, msg, msg, preMessages)
}

// PromptResult sends a message and returns the full turn result including
// usage statistics and conversation messages. Use this instead of Prompt()
// when you need more than just the response text.
//
// PromptResult blocks until end-of-turn regardless of whether streaming is
// enabled. When streaming is enabled, every delta observed during the turn is
// also captured in order in [TurnResult.Stream], so callers can assert
// streamed ordering deterministically without an OnMessageUpdate collector.
func (m *Kit) PromptResult(ctx context.Context, message string) (*TurnResult, error) {
	return m.runTurn(ctx, message, message, []fantasy.Message{
		fantasy.NewUserMessage(message),
	})
}

// PromptResultWithFiles sends a multimodal message (text + images) and returns
// the full turn result. The files parameter carries binary file data (e.g.
// clipboard images) that are included alongside the text in the user message.
func (m *Kit) PromptResultWithFiles(ctx context.Context, message string, files []LLMFilePart) (*TurnResult, error) {
	return m.runTurn(ctx, message, message, []fantasy.Message{
		fantasy.NewUserMessage(message, files...),
	})
}

// PromptResultWithMessages submits multiple user messages in a single turn.
// All messages are persisted to the session and sent to the agent together.
// The agent will respond once to the combined context of all messages.
// Returns the full turn result including usage statistics and conversation messages.
func (m *Kit) PromptResultWithMessages(ctx context.Context, messages []string) (*TurnResult, error) {
	if len(messages) == 0 {
		return nil, fmt.Errorf("no messages provided")
	}

	// Build prompt label from all messages
	promptLabel := strings.Join(messages, " | ")
	if len(promptLabel) > 100 {
		promptLabel = promptLabel[:100] + "..."
	}

	// Build LLM messages from all strings
	var preMessages []fantasy.Message
	for _, msg := range messages {
		preMessages = append(preMessages, fantasy.NewUserMessage(msg))
	}

	return m.runTurn(ctx, promptLabel, messages[len(messages)-1], preMessages)
}

// ClearSession resets the session's leaf pointer to the root, starting
// a fresh conversation branch.
func (m *Kit) ClearSession() {
	if m.session != nil {
		_ = m.session.Branch("")
	}
}

// GetModelString returns the current model string identifier (e.g.,
// "anthropic/claude-sonnet-4-5-20250929" or "openai/gpt-4") being used by the agent.
func (m *Kit) GetModelString() string {
	return m.modelString
}

// GetModelInfo returns detailed information about the current model
// (capabilities, pricing, limits). Returns nil if the model is not in the
// registry — this is expected for new models or custom fine-tunes.
func (m *Kit) GetModelInfo() *ModelInfo {
	provider, modelID, err := ParseModelString(m.modelString)
	if err != nil {
		return nil
	}
	return LookupModel(provider, modelID)
}

// IsReasoningModel returns true if the current model supports extended thinking / reasoning.
func (m *Kit) IsReasoningModel() bool {
	info := m.GetModelInfo()
	return info != nil && info.Reasoning
}

// GetThinkingLevel returns the current thinking level.
func (m *Kit) GetThinkingLevel() string {
	return m.v.GetString("thinking-level")
}

// SetThinkingLevel changes the thinking level and recreates the agent with
// the new thinking budget. Returns an error if provider recreation fails.
//
// With message-level caching, both thinking and caching work together.
// Caching reduces costs by 60-90% for repeated context.
func (m *Kit) SetThinkingLevel(ctx context.Context, level string) error {
	m.v.Set("thinking-level", level)
	// Recreate agent with new thinking config by re-running SetModel
	// with the same model string. SetModel rebuilds the provider and
	// passes the updated viper config (including thinking-level).
	return m.SetModel(ctx, m.modelString)
}

// GetTools returns all tools available to the agent (core + MCP + extensions).
func (m *Kit) GetTools() []Tool {
	return m.agent.GetTools()
}

// MaxTokens returns the effective max output tokens currently configured for
// the agent. This is the value actually sent to the LLM provider on each
// request, after CLI/env/config resolution, per-model overrides, model-aware
// right-sizing, and any Anthropic thinking-budget adjustments.
//
// Returns 0 when the active provider suppresses the max_output_tokens
// parameter (e.g. OpenAI Codex OAuth) or when no model is configured yet.
// A non-zero value is the number that will cause a FinishReasonLength
// truncation if the model tries to generate beyond it.
func (m *Kit) MaxTokens() int {
	if m.agent == nil {
		return 0
	}
	return m.agent.GetMaxTokens()
}

// MaxOutputLimit returns the catalog-reported output ceiling for the current
// model in tokens, or 0 when the model isn't in the registry (custom models,
// new releases, Ollama, etc.). Pair with MaxTokens() to detect when the agent
// is configured well below what the model supports and surface a hint to the
// user.
func (m *Kit) MaxOutputLimit() int {
	info := m.GetModelInfo()
	if info == nil {
		return 0
	}
	return info.Limit.Output
}

// extractFileParts returns all FilePart entries from a message's Content.
// Used to preserve image attachments when replacing user message text.
func extractFileParts(msg fantasy.Message) []fantasy.FilePart {
	var files []fantasy.FilePart
	for _, part := range msg.Content {
		if fp, ok := part.(fantasy.FilePart); ok {
			files = append(files, fp)
		}
	}
	return files
}

// Close cleans up resources including MCP server connections, model resources,
// and the tree session file handle. Should be called when the Kit instance is
// no longer needed. Returns an error if cleanup fails.
//
// Close is equivalent to CloseContext(context.Background()). Use
// [Kit.CloseContext] when shutdown must be bounded by a deadline.
func (m *Kit) Close() error {
	return m.CloseContext(context.Background())
}

// CloseContext is like [Kit.Close] but accepts a context so graceful shutdown
// can be bounded by a deadline or cancellation. The context is honored on a
// best-effort basis: if it is already done when CloseContext is called, the
// context error is returned after a best-effort cleanup pass.
func (m *Kit) CloseContext(ctx context.Context) error {
	// Emit SessionShutdown for extensions.
	if m.extRunner != nil && m.extRunner.HasHandlers(extensions.SessionShutdown) {
		_, _ = m.extRunner.Emit(extensions.SessionShutdownEvent{})
	}
	if m.session != nil {
		_ = m.session.Close()
	}
	// Release the OAuth callback port if we own the handler.
	if closer, ok := m.authHandler.(interface{ Close() error }); ok {
		_ = closer.Close()
	}
	err := m.agent.Close()
	if ctxErr := ctx.Err(); ctxErr != nil && err == nil {
		return ctxErr
	}
	return err
}

// Conversion helpers are defined in adapter.go.

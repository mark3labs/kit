# Kit SDK: Re-exported Types

> Part of the `kit-sdk` skill. Read `SKILL.md` first for the overview and critical constraints.

## Re-exported Types

The SDK re-exports internal types so you don't need direct internal imports:

```go
// Message types
kit.Message, kit.MessageRole, kit.ContentPart
kit.TextContent, kit.ReasoningContent, kit.ToolCall, kit.ToolResult, kit.Finish
kit.RoleUser, kit.RoleAssistant, kit.RoleTool, kit.RoleSystem

// Session types
kit.SessionInfo, kit.TreeManager, kit.SessionHeader, kit.MessageEntry

// Config types
kit.Config, kit.MCPServerConfig

// Provider types
kit.ProviderConfig, kit.ProviderResult, kit.ModelInfo, kit.ModelCost, kit.ModelLimit

// LLM types — clean aliases (no external library dependency in consumer code)
kit.LLMMessage      // {Role LLMMessageRole, Content string}
kit.LLMMessagePart  // interface for message content parts
kit.LLMMessageRole  // "user" | "assistant" | "system" | "tool"
kit.LLMUsage        // {InputTokens, OutputTokens, TotalTokens, ReasoningTokens,
                     //  CacheCreationTokens, CacheReadTokens}
kit.LLMResponse     // {Content, FinishReason, Usage}
kit.LLMFilePart     // {Filename, Data []byte, MediaType}
kit.LLMTextPart     // plain-text content part
kit.LLMReasoningPart // reasoning/chain-of-thought content part
kit.LLMToolCall     // {ID, Name, Input string} — execution-layer tool call (for Tool.Run)
kit.LLMToolResponse // {Type, Content, Data, MediaType, IsError, ...} — raw tool response
kit.LLMToolCallPart    // LLM-initiated tool invocation within a message
kit.LLMToolResultPart  // tool result within a message
kit.LLMToolResultOutputContent      // interface for tool result output
kit.LLMToolResultOutputContentText  // text tool result
kit.LLMToolResultOutputContentError // error tool result
kit.LLMToolResultOutputContentMedia // media tool result {Data, MediaType, Text}
kit.LLMToolResultContentType        // "text" | "error" | "media"
kit.LLMToolInfo          // {Name, Description, Parameters, Required, Parallel}
kit.LLMProviderOptions   // provider-specific option maps (keyed by provider name)
kit.LLMProviderMetadata  // provider-specific response metadata
kit.LLMPrompt            // []LLMMessage — ordered prompt sequence
kit.LLMFinishReason      // "stop" | "length" | "tool-calls" | ...

// Compaction types
kit.CompactionResult, kit.CompactionOptions

// MCP OAuth types
kit.MCPAuthHandler         // interface: RedirectURI() + HandleAuth(ctx, server, authURL) for OAuth UX
kit.DefaultMCPAuthHandler  // SDK-provided transport mechanics (port + callback server); set OnAuthURL hook
kit.CLIMCPAuthHandler      // CLI wrapper around DefaultMCPAuthHandler: opens browser, prints status
kit.NewDefaultMCPAuthHandler()         // random port, no UX side effects
kit.NewDefaultMCPAuthHandlerWithPort(p) // fixed port (stable, pre-registered redirect URI)
kit.NewCLIMCPAuthHandler()             // CLI handler: browser + stderr + localhost callback
kit.MCPTokenStore        // interface for custom OAuth token storage
kit.MCPToken             // OAuth token struct (access, refresh, expiry)
kit.MCPTokenStoreFactory // func(serverURL string) (MCPTokenStore, error)
kit.ErrMCPNoToken        // sentinel error for "no token stored"
kit.MCPServer            // *server.MCPServer for in-process MCP transport
kit.MCPServerStatus      // {Name string, ToolCount int}
kit.MCPPrompt            // {Name, Description, Arguments []MCPPromptArgument, ServerName}
kit.MCPPromptArgument    // {Name, Description string, Required bool}
kit.MCPPromptMessage     // {Role, Content string, FileParts []LLMFilePart}
kit.MCPPromptResult      // {Description string, Messages []MCPPromptMessage}
kit.MCPResource          // {URI, Name, Description, MIMEType, ServerName}
kit.MCPResourceContent   // {URI, MIMEType, Text string, BlobData []byte, IsBlob bool}

// Conversion helpers
msgs := kit.ConvertToLLMMessages(&msg)   // SDK Message  → []LLMMessage
msg  := kit.ConvertFromLLMMessage(lMsg)  // LLMMessage   → SDK Message
```


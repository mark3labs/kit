package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"charm.land/fantasy"

	"github.com/mark3labs/kit/internal/config"
	"github.com/mark3labs/kit/internal/core"
	"github.com/mark3labs/kit/internal/message"
	"github.com/mark3labs/kit/internal/models"
	"github.com/mark3labs/kit/internal/tools"
)

// AgentConfig holds configuration options for creating a new Agent.
type AgentConfig struct {
	ModelConfig      *models.ProviderConfig
	MCPConfig        *config.Config
	SystemPrompt     string
	MaxSteps         int
	StreamingEnabled bool
	DebugLogger      tools.DebugLogger

	// AuthHandler handles OAuth authorization for remote MCP servers.
	// When set, remote transports are configured with OAuth support.
	// If nil, remote MCP servers that require OAuth will fail to connect.
	AuthHandler tools.MCPAuthHandler

	// CoreTools overrides the default core tool set. If empty, core.AllTools()
	// is used. This allows SDK users to provide a custom tool set (e.g.
	// CodingTools or tools with a custom WorkDir).
	CoreTools []fantasy.AgentTool

	// ToolWrapper is an optional function that wraps the combined tool list
	// before it is passed to the LLM agent. Used by the extensions system
	// to intercept tool calls/results.
	ToolWrapper func([]fantasy.AgentTool) []fantasy.AgentTool

	// ExtraTools are additional tools to include alongside core and MCP tools.
	// Used by extensions to register custom tools.
	ExtraTools []fantasy.AgentTool
}

// ToolCallHandler is a function type for handling tool calls as they happen.
type ToolCallHandler func(toolCallID, toolName, toolArgs string)

// ToolExecutionHandler is a function type for handling tool execution start/end events.
type ToolExecutionHandler func(toolCallID, toolName, toolArgs string, isStarting bool)

// ToolResultHandler is a function type for handling tool results.
// The metadata parameter carries optional structured data (e.g. file diff
// info) from the tool execution, JSON-encoded. It may be empty.
type ToolResultHandler func(toolCallID, toolName, toolArgs, result, metadata string, isError bool)

// ResponseHandler is a function type for handling LLM responses.
type ResponseHandler func(content string)

// StreamingResponseHandler is a function type for handling streaming LLM responses.
type StreamingResponseHandler func(content string)

// ToolCallContentHandler is a function type for handling content that accompanies tool calls.
type ToolCallContentHandler func(content string)

// ReasoningDeltaHandler is a function type for handling streaming reasoning/thinking deltas.
type ReasoningDeltaHandler func(delta string)

// ReasoningCompleteHandler is a function type for handling reasoning/thinking completion.
// Called when the last reasoning token has been processed, before text streaming starts.
type ReasoningCompleteHandler func()

// ToolOutputHandler is a function type for handling streaming tool output chunks.
// Used by tools like bash to stream output as it arrives rather than waiting
// for the command to complete. The isStderr flag indicates if the chunk
// contains stderr output.
// Note: This is an alias for core.ToolOutputCallback to avoid import cycles.
type ToolOutputHandler = core.ToolOutputCallback

// StepUsageHandler is a function type for handling token usage after each
// complete step in a multi-step agent turn. This enables real-time cost
// tracking during long-running tool-calling conversations.
type StepUsageHandler func(inputTokens, outputTokens, cacheReadTokens, cacheCreationTokens int64)

// Agent represents an AI agent with core tool integration using the LLM library.
// Core tools (bash, read, write, edit, grep, find, ls) are registered as direct
// AgentTool implementations — no MCP layer, no serialization overhead.
// Additional tools from external MCP servers can be loaded alongside core tools.
//
// When MCP servers are configured, tool loading happens in the background so the
// agent (and UI) can start immediately. The first LLM call automatically waits
// for MCP tools to finish loading before proceeding.
type Agent struct {
	toolManager      *tools.MCPToolManager
	fantasyAgent     fantasy.Agent
	model            fantasy.LanguageModel
	providerCloser   io.Closer // optional cleanup for providers like kronk
	maxSteps         int
	systemPrompt     string
	loadingMessage   string
	providerType     string
	streamingEnabled bool
	coreTools        []fantasy.AgentTool
	extraTools       []fantasy.AgentTool
	toolWrapper      func([]fantasy.AgentTool) []fantasy.AgentTool // stored for SetModel rebuild

	// providerOptions and modelConfig are stored for rebuilding the fantasy
	// agent when MCP tools arrive asynchronously or on SetModel.
	providerOptions     fantasy.ProviderOptions
	skipMaxOutputTokens bool
	modelConfig         *models.ProviderConfig

	// mcpReady is closed when background MCP tool loading completes (success
	// or failure). nil when no MCP servers are configured.
	mcpReady chan struct{}
	// mcpErr holds any error from background MCP loading.
	mcpErr error
}

// GenerateWithLoopResult contains the result and conversation history from an agent interaction.
type GenerateWithLoopResult struct {
	// FinalResponse is the last message generated by the model
	FinalResponse *fantasy.Response
	// ConversationMessages contains all messages in the conversation including tool calls and results
	ConversationMessages []fantasy.Message
	// Messages contains the conversation as custom content blocks
	Messages []message.Message
	// TotalUsage contains aggregate token usage across all steps
	TotalUsage fantasy.Usage
	// StopReason is the LLM provider's finish reason for the final response.
	StopReason string
}

// NewAgent creates a new Agent with core tools and optional MCP tool integration.
// Core tools (bash, read, write, edit, grep, find, ls) are always registered.
// If MCP servers are configured, their tools are loaded in the background —
// the agent returns immediately and is usable with core tools only. The first
// LLM call (GenerateWithLoop) automatically waits for MCP tools to finish
// loading and rebuilds the agent with the full tool set.
func NewAgent(ctx context.Context, agentConfig *AgentConfig) (*Agent, error) {
	// Create the LLM provider
	providerResult, err := models.CreateProvider(ctx, agentConfig.ModelConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create model provider: %v", err)
	}

	// Register core tools (direct AgentTool implementations, no MCP overhead).
	// Use caller-provided tools if set, otherwise default to all core tools.
	coreTools := agentConfig.CoreTools
	if len(coreTools) == 0 {
		coreTools = core.AllTools()
	}

	// Build the initial tool list: core tools + extension tools (no MCP yet).
	allTools := make([]fantasy.AgentTool, len(coreTools))
	copy(allTools, coreTools)
	// Append any extra tools provided by extensions.
	if len(agentConfig.ExtraTools) > 0 {
		allTools = append(allTools, agentConfig.ExtraTools...)
	}

	// Apply tool wrapper (extension interception layer) if configured.
	if agentConfig.ToolWrapper != nil {
		allTools = agentConfig.ToolWrapper(allTools)
	}

	// Build agent options
	agentOpts := buildAgentOptions(agentConfig, providerResult, allTools)

	// Create the agent
	fantasyAgent := fantasy.NewAgent(providerResult.Model, agentOpts...)

	// Determine provider type from model string
	providerType := "default"
	if agentConfig.ModelConfig != nil && agentConfig.ModelConfig.ModelString != "" {
		if p, _, err := models.ParseModelString(agentConfig.ModelConfig.ModelString); err == nil {
			providerType = p
		}
	}

	a := &Agent{
		fantasyAgent:        fantasyAgent,
		model:               providerResult.Model,
		providerCloser:      providerResult.Closer,
		maxSteps:            agentConfig.MaxSteps,
		systemPrompt:        agentConfig.SystemPrompt,
		loadingMessage:      providerResult.Message,
		providerType:        providerType,
		streamingEnabled:    agentConfig.StreamingEnabled,
		coreTools:           coreTools,
		extraTools:          agentConfig.ExtraTools,
		toolWrapper:         agentConfig.ToolWrapper,
		providerOptions:     providerResult.ProviderOptions,
		skipMaxOutputTokens: providerResult.SkipMaxOutputTokens,
		modelConfig:         agentConfig.ModelConfig,
	}

	// Start MCP tool loading in the background if servers are configured.
	// The mcpReady channel is closed when loading completes (success or failure).
	if agentConfig.MCPConfig != nil && len(agentConfig.MCPConfig.MCPServers) > 0 {
		toolManager := tools.NewMCPToolManager()
		toolManager.SetModel(providerResult.Model)
		if agentConfig.AuthHandler != nil {
			toolManager.SetAuthHandler(agentConfig.AuthHandler)
		}
		if agentConfig.DebugLogger != nil {
			toolManager.SetDebugLogger(agentConfig.DebugLogger)
		}
		a.toolManager = toolManager
		a.mcpReady = make(chan struct{})

		go func() {
			defer close(a.mcpReady)
			if err := toolManager.LoadTools(ctx, agentConfig.MCPConfig); err != nil {
				a.mcpErr = err
				fmt.Printf("Warning: Failed to load MCP tools: %v\n", err)
			}
		}()
	}

	return a, nil
}

// WaitForMCPTools blocks until background MCP tool loading completes.
// Returns nil if no MCP servers are configured or if loading succeeded.
// Returns the loading error if all servers failed. Safe to call multiple times.
func (a *Agent) WaitForMCPTools() error {
	if a.mcpReady == nil {
		return nil
	}
	<-a.mcpReady
	return a.mcpErr
}

// MCPToolsReady returns true if MCP tool loading has completed (or was never
// started). This is a non-blocking check useful for UI status display.
func (a *Agent) MCPToolsReady() bool {
	if a.mcpReady == nil {
		return true
	}
	select {
	case <-a.mcpReady:
		return true
	default:
		return false
	}
}

// ensureMCPTools waits for MCP tools to load and rebuilds the fantasy agent
// with the full tool set. Called lazily before the first LLM call.
// This is idempotent — subsequent calls after the first rebuild are no-ops.
func (a *Agent) ensureMCPTools() {
	if a.mcpReady == nil {
		return
	}
	<-a.mcpReady

	// If there are MCP tools, rebuild the fantasy agent to include them.
	if a.toolManager != nil && len(a.toolManager.GetTools()) > 0 {
		a.rebuildFantasyAgent()
	}

	// Nil out the channel so future calls are instant no-ops and we
	// don't rebuild again.
	a.mcpReady = nil
}

// rebuildFantasyAgent reconstructs the fantasy agent with the current full
// tool set (core + MCP + extension tools). Used after MCP tools arrive
// asynchronously and by SetModel.
func (a *Agent) rebuildFantasyAgent() {
	allTools := make([]fantasy.AgentTool, len(a.coreTools))
	copy(allTools, a.coreTools)
	if a.toolManager != nil {
		allTools = append(allTools, a.toolManager.GetTools()...)
	}
	if len(a.extraTools) > 0 {
		allTools = append(allTools, a.extraTools...)
	}
	if a.toolWrapper != nil {
		allTools = a.toolWrapper(allTools)
	}

	providerResult := &models.ProviderResult{
		Model:               a.model,
		ProviderOptions:     a.providerOptions,
		SkipMaxOutputTokens: a.skipMaxOutputTokens,
	}
	agentOpts := buildAgentOptions(&AgentConfig{
		ModelConfig:  a.modelConfig,
		SystemPrompt: a.systemPrompt,
		MaxSteps:     a.maxSteps,
	}, providerResult, allTools)

	a.fantasyAgent = fantasy.NewAgent(a.model, agentOpts...)
}

// buildAgentOptions constructs the fantasy.AgentOption slice from config,
// provider result, and the combined tool list. Shared by NewAgent,
// rebuildFantasyAgent, and SetModel.
func buildAgentOptions(agentConfig *AgentConfig, providerResult *models.ProviderResult, allTools []fantasy.AgentTool) []fantasy.AgentOption {
	var agentOpts []fantasy.AgentOption

	if agentConfig.SystemPrompt != "" {
		agentOpts = append(agentOpts, fantasy.WithSystemPrompt(agentConfig.SystemPrompt))
	}

	if len(allTools) > 0 {
		agentOpts = append(agentOpts, fantasy.WithTools(allTools...))
	}

	// Set max steps as stop condition
	if agentConfig.MaxSteps > 0 {
		agentOpts = append(agentOpts, fantasy.WithStopConditions(
			fantasy.StepCountIs(agentConfig.MaxSteps),
		))
	}

	// Pass provider-specific options (e.g. OpenAI Responses API reasoning settings).
	if providerResult.ProviderOptions != nil {
		agentOpts = append(agentOpts, fantasy.WithProviderOptions(providerResult.ProviderOptions))
	}

	// Pass generation parameters when available.
	if agentConfig.ModelConfig != nil {
		// Skip max_output_tokens for providers that don't support it (e.g., Codex OAuth)
		if agentConfig.ModelConfig.MaxTokens > 0 && !providerResult.SkipMaxOutputTokens {
			agentOpts = append(agentOpts, fantasy.WithMaxOutputTokens(int64(agentConfig.ModelConfig.MaxTokens)))
		}
		if agentConfig.ModelConfig.Temperature != nil {
			agentOpts = append(agentOpts, fantasy.WithTemperature(float64(*agentConfig.ModelConfig.Temperature)))
		}
		if agentConfig.ModelConfig.TopP != nil {
			agentOpts = append(agentOpts, fantasy.WithTopP(float64(*agentConfig.ModelConfig.TopP)))
		}
		if agentConfig.ModelConfig.TopK != nil {
			agentOpts = append(agentOpts, fantasy.WithTopK(int64(*agentConfig.ModelConfig.TopK)))
		}
		if agentConfig.ModelConfig.FrequencyPenalty != nil {
			agentOpts = append(agentOpts, fantasy.WithFrequencyPenalty(float64(*agentConfig.ModelConfig.FrequencyPenalty)))
		}
		if agentConfig.ModelConfig.PresencePenalty != nil {
			agentOpts = append(agentOpts, fantasy.WithPresencePenalty(float64(*agentConfig.ModelConfig.PresencePenalty)))
		}
	}

	return agentOpts
}

// GenerateWithLoop processes messages with a custom loop that displays tool calls in real-time.
func (a *Agent) GenerateWithLoop(ctx context.Context, messages []fantasy.Message,
	onToolCall ToolCallHandler, onToolExecution ToolExecutionHandler, onToolResult ToolResultHandler,
	onResponse ResponseHandler, onToolCallContent ToolCallContentHandler,
) (*GenerateWithLoopResult, error) {
	return a.GenerateWithLoopAndStreaming(ctx, messages, onToolCall, onToolExecution, onToolResult,
		onResponse, onToolCallContent, nil, nil, nil, nil, nil)
}

// GenerateWithLoopAndStreaming processes messages using the agent with streaming and callbacks.
// The agent handles the tool call loop internally. We map the rich callback system
// to kit's existing callback interface for UI integration.
func (a *Agent) GenerateWithLoopAndStreaming(ctx context.Context, messages []fantasy.Message,
	onToolCall ToolCallHandler, onToolExecution ToolExecutionHandler, onToolResult ToolResultHandler,
	onResponse ResponseHandler, onToolCallContent ToolCallContentHandler,
	onStreamingResponse StreamingResponseHandler,
	onReasoningDelta ReasoningDeltaHandler,
	onReasoningComplete ReasoningCompleteHandler,
	onToolOutput ToolOutputHandler,
	onStepUsage StepUsageHandler,
) (*GenerateWithLoopResult, error) {

	// Wait for background MCP tool loading to complete and rebuild the
	// fantasy agent with the full tool set. This is a no-op when no MCP
	// servers are configured or tools have already been integrated.
	a.ensureMCPTools()

	// Inject tool output handler into context for use by core tools (e.g., bash).
	if onToolOutput != nil {
		ctx = core.ContextWithToolOutputCallback(ctx, onToolOutput)
	}

	// The agent requires the current user input as Prompt, with prior messages as history.
	// Extract the last user message text and files as the prompt, and pass everything
	// before it as Messages. Files (e.g. clipboard images) are passed via the Files
	// field so the agent includes them in the API request.
	prompt, files, history := splitPromptAndHistory(messages)

	// Apply message-level cache control for Anthropic models.
	// This avoids type conflicts with provider-level options.
	history = applyCacheControlToMessages(history)

	// Track current tool call args for callbacks
	var currentToolArgs string

	// Use the streaming path when streaming is enabled OR when any callbacks are
	// provided. The agent only exposes tool/step callbacks on AgentStreamCall, so
	// Stream is required to observe tool execution in real time. The non-streaming
	// Generate path is reserved for the simple case with no callbacks at all.
	hasCallbacks := onToolCall != nil || onToolExecution != nil || onToolResult != nil ||
		onToolCallContent != nil || onStreamingResponse != nil || onReasoningDelta != nil

	if a.streamingEnabled || hasCallbacks {
		// Track completed step messages so we can return partial results
		// on cancellation. The agent's Stream() discards accumulated steps
		// when it returns an error, but the OnStepFinish callback fires
		// for every step that completed before the error occurred.
		var completedStepMessages []fantasy.Message

		// Use the streaming agent
		streamCall := fantasy.AgentStreamCall{
			Prompt:   prompt,
			Files:    files,
			Messages: history,

			// Reasoning/thinking streaming callback
			OnReasoningDelta: func(id, delta string) error {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				if onReasoningDelta != nil {
					onReasoningDelta(delta)
				}
				return nil
			},

			// Reasoning/thinking complete callback
			OnReasoningEnd: func(id string, _ fantasy.ReasoningContent) error {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				if onReasoningComplete != nil {
					onReasoningComplete()
				}
				return nil
			},

			// Text streaming callback
			OnTextDelta: func(id, text string) error {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				if onStreamingResponse != nil {
					onStreamingResponse(text)
				}
				return nil
			},

			// Tool call complete - the tool has been parsed and is about to execute
			OnToolCall: func(tc fantasy.ToolCallContent) error {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				currentToolArgs = tc.Input

				// Notify about the tool call
				if onToolCall != nil {
					onToolCall(tc.ToolCallID, tc.ToolName, tc.Input)
				}

				// Notify tool execution starting
				if onToolExecution != nil {
					onToolExecution(tc.ToolCallID, tc.ToolName, tc.Input, true)
				}

				return nil
			},

			// Tool result - tool execution completed
			OnToolResult: func(tr fantasy.ToolResultContent) error {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				// Notify tool execution finished
				if onToolExecution != nil {
					onToolExecution(tr.ToolCallID, tr.ToolName, currentToolArgs, false)
				}

				if onToolResult != nil {
					// Extract result text and error status
					resultText, isError := extractToolResultText(tr)
					onToolResult(tr.ToolCallID, tr.ToolName, currentToolArgs, resultText, tr.ClientMetadata, isError)
				}

				return nil
			},

			// Step callbacks for content that accompanies tool calls
			OnStepFinish: func(step fantasy.StepResult) error {
				// Accumulate messages from completed steps so they can be
				// persisted even if a later step is cancelled.
				completedStepMessages = append(completedStepMessages, step.Messages...)

				if ctx.Err() != nil {
					return ctx.Err()
				}
				// Check if step has text content alongside tool calls
				text := step.Content.Text()
				toolCalls := step.Content.ToolCalls()
				if text != "" && len(toolCalls) > 0 && onToolCallContent != nil {
					onToolCallContent(text)
				}
				// Emit step usage for real-time cost tracking
				if onStepUsage != nil {
					onStepUsage(step.Usage.InputTokens, step.Usage.OutputTokens,
						step.Usage.CacheReadTokens, step.Usage.CacheCreationTokens)
				}
				return nil
			},
		}

		// If a steer channel is attached to the context, wire up a
		// PrepareStep function that drains the channel between steps
		// and injects pending steer messages as user messages before
		// the next LLM call. This enables graceful mid-turn steering
		// without cancelling in-progress tool execution.
		if steerCh := steerChFromContext(ctx); steerCh != nil {
			onConsumed := steerConsumedFromContext(ctx)
			streamCall.PrepareStep = func(
				stepCtx context.Context,
				opts fantasy.PrepareStepFunctionOptions,
			) (context.Context, fantasy.PrepareStepResult, error) {
				// Drain all pending steer messages (non-blocking).
				var steered []SteerMessage
				for {
					select {
					case msg := <-steerCh:
						steered = append(steered, msg)
					default:
						goto done
					}
				}
			done:
				result := fantasy.PrepareStepResult{
					Model:    opts.Model,
					Messages: opts.Messages,
				}
				if len(steered) > 0 {
					// Inject each steer message as a user message so the
					// LLM sees the redirection on the next step.
					for _, sm := range steered {
						result.Messages = append(result.Messages,
							fantasy.NewUserMessage(sm.Text, sm.Files...))
					}
					// Notify that steer messages were consumed.
					if onConsumed != nil {
						onConsumed(len(steered))
					}
				}

				// Apply message-level cache control for Anthropic models.
				// This avoids type conflicts with provider-level options.
				result.Messages = applyCacheControlToMessages(result.Messages)

				return stepCtx, result, nil
			}
		}

		result, err := a.fantasyAgent.Stream(ctx, streamCall)
		if err != nil {
			// On cancellation (or any error), return a partial result
			// containing messages from completed steps so the caller can
			// persist tool calls and results that finished before the
			// cancellation. The original input messages are included so
			// the caller sees the full conversation up to the point of
			// cancellation.
			if len(completedStepMessages) > 0 {
				partialMessages := make([]fantasy.Message, 0, len(messages)+len(completedStepMessages))
				partialMessages = append(partialMessages, messages...)
				partialMessages = append(partialMessages, completedStepMessages...)
				return &GenerateWithLoopResult{
					ConversationMessages: partialMessages,
				}, err
			}
			return nil, err
		}

		// Fire the response callback so callers (e.g. the TUI) can reset
		// streaming state. This must fire even when the response text is
		// empty (e.g. reasoning-only responses) so the UI properly resets
		// the stream component and avoids duplicate content on the next
		// flush.
		if onResponse != nil {
			onResponse(result.Response.Content.Text())
		}

		return convertAgentResult(result, messages), nil
	}

	// Non-streaming path with no callbacks — use the simpler Generate call.
	result, err := a.fantasyAgent.Generate(ctx, fantasy.AgentCall{
		Prompt:   prompt,
		Files:    files,
		Messages: history,
	})
	if err != nil {
		return nil, err
	}

	// For non-streaming, fire the response callback so callers can reset
	// streaming state (see streaming path comment above).
	if onResponse != nil {
		onResponse(result.Response.Content.Text())
	}

	return convertAgentResult(result, messages), nil
}

// splitPromptAndHistory extracts the last user message as the prompt string,
// and returns everything before it as conversation history. The agent's
// requires the current turn's input as Prompt (string), with prior messages
// passed separately as Messages (history).
func splitPromptAndHistory(messages []fantasy.Message) (string, []fantasy.FilePart, []fantasy.Message) {
	if len(messages) == 0 {
		return "", nil, nil
	}

	// Walk backwards to find the last user message
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == fantasy.MessageRoleUser {
			// Extract text and file parts from the user message
			var prompt string
			var files []fantasy.FilePart
			for _, part := range messages[i].Content {
				switch p := part.(type) {
				case fantasy.TextPart:
					if prompt == "" {
						prompt = p.Text
					}
				case fantasy.FilePart:
					files = append(files, p)
				}
			}
			// History is everything except this last user message
			history := make([]fantasy.Message, 0, len(messages)-1)
			history = append(history, messages[:i]...)
			history = append(history, messages[i+1:]...)
			return prompt, files, history
		}
	}

	// No user message found — use the last message's text as prompt
	last := messages[len(messages)-1]
	for _, part := range last.Content {
		if tp, ok := part.(fantasy.TextPart); ok {
			return tp.Text, nil, messages[:len(messages)-1]
		}
	}

	return "", nil, messages
}

// convertAgentResult converts an AgentResult to our GenerateWithLoopResult.
// It builds both the message slice and the new custom content blocks.
func convertAgentResult(result *fantasy.AgentResult, originalMessages []fantasy.Message) *GenerateWithLoopResult {
	// Collect all conversation messages: original + all step messages
	var allFantasyMessages []fantasy.Message
	allFantasyMessages = append(allFantasyMessages, originalMessages...)

	for _, step := range result.Steps {
		allFantasyMessages = append(allFantasyMessages, step.Messages...)
	}

	// Convert to custom content blocks
	var allMessages []message.Message
	for _, fm := range allFantasyMessages {
		allMessages = append(allMessages, message.FromLLMMessage(fm))
	}

	return &GenerateWithLoopResult{
		FinalResponse:        &result.Response,
		ConversationMessages: allFantasyMessages,
		Messages:             allMessages,
		TotalUsage:           result.TotalUsage,
		StopReason:           string(result.Response.FinishReason),
	}
}

// extractToolResultText extracts the text and error status from a ToolResultContent.
// For core tools, the result is already clean text (no MCP JSON wrapping).
// For MCP tools, it unwraps the MCP content structure.
func extractToolResultText(tr fantasy.ToolResultContent) (string, bool) {
	if tr.Result == nil {
		return "", false
	}

	// Check if this is an error result by examining the type.
	if errResult, ok := tr.Result.(fantasy.ToolResultOutputContentError); ok {
		return errResult.Error.Error(), true
	}

	// Get text directly from the result type.
	if textResult, ok := tr.Result.(fantasy.ToolResultOutputContentText); ok {
		// Try to unwrap MCP JSON structure (for external MCP tools).
		// Core tools return plain text, so this is a no-op for them.
		return extractMCPContentText(textResult.Text), false
	}

	// Fallback: stringify for display.
	return fmt.Sprintf("%v", tr.Result), false
}

// extractMCPContentText attempts to parse an MCP tool result JSON string
// and extract the human-readable text from its content array. The expected
// format is: {"content":[{"type":"text","text":"..."}], "_meta":{...}}
// If parsing fails the original string is returned unchanged.
func extractMCPContentText(result string) string {
	// Quick check: if it doesn't look like MCP JSON, return as-is
	if !strings.HasPrefix(strings.TrimSpace(result), "{") {
		return result
	}

	// Try to parse as MCP result structure
	type mcpContent struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	type mcpResult struct {
		Content []mcpContent `json:"content"`
	}

	var parsed mcpResult
	if err := json.Unmarshal([]byte(result), &parsed); err == nil && len(parsed.Content) > 0 {
		var texts []string
		for _, c := range parsed.Content {
			if c.Type == "text" && c.Text != "" {
				texts = append(texts, c.Text)
			}
		}
		if len(texts) > 0 {
			return strings.Join(texts, "\n")
		}
	}

	return result
}

// GetTools returns the list of available tools loaded in the agent,
// including core tools, MCP tools, and extension-registered tools.
func (a *Agent) GetTools() []fantasy.AgentTool {
	allTools := make([]fantasy.AgentTool, len(a.coreTools))
	copy(allTools, a.coreTools)
	if a.toolManager != nil {
		allTools = append(allTools, a.toolManager.GetTools()...)
	}
	if len(a.extraTools) > 0 {
		allTools = append(allTools, a.extraTools...)
	}
	return allTools
}

// GetCoreToolCount returns the number of core tools.
func (a *Agent) GetCoreToolCount() int {
	return len(a.coreTools)
}

// GetMCPToolCount returns the number of tools loaded from external MCP servers.
func (a *Agent) GetMCPToolCount() int {
	if a.toolManager == nil {
		return 0
	}
	return len(a.toolManager.GetTools())
}

// GetExtensionToolCount returns the number of tools registered by extensions.
func (a *Agent) GetExtensionToolCount() int {
	return len(a.extraTools)
}

// SetExtraTools replaces the agent's extra tools (e.g. extension-registered
// tools) and rebuilds the internal agent with the updated tool list. The
// model, system prompt, and all other configuration are preserved.
func (a *Agent) SetExtraTools(extraTools []fantasy.AgentTool) {
	a.extraTools = extraTools
	a.rebuildFantasyAgent()
}

// GetLoadingMessage returns the loading message from provider creation.
func (a *Agent) GetLoadingMessage() string {
	return a.loadingMessage
}

// GetLoadedServerNames returns the names of successfully loaded MCP servers.
func (a *Agent) GetLoadedServerNames() []string {
	if a.toolManager == nil {
		return nil
	}
	return a.toolManager.GetLoadedServerNames()
}

// SetModel swaps the agent's LLM provider to a new model. The existing tools,
// system prompt, and configuration are preserved. The old provider is closed
// if it has a closer. Returns the previous model string for notification.
func (a *Agent) SetModel(ctx context.Context, config *models.ProviderConfig) error {
	// Ensure MCP tools are loaded before rebuilding (SetModel may be called
	// before the first LLM call).
	a.ensureMCPTools()

	providerResult, err := models.CreateProvider(ctx, config)
	if err != nil {
		return fmt.Errorf("failed to create model provider: %v", err)
	}
	// Close old provider.
	if a.providerCloser != nil {
		_ = a.providerCloser.Close()
	}

	// Update model info on MCP tool manager.
	if a.toolManager != nil {
		a.toolManager.SetModel(providerResult.Model)
	}

	// Swap fields.
	a.model = providerResult.Model
	a.providerCloser = providerResult.Closer
	a.providerOptions = providerResult.ProviderOptions
	a.skipMaxOutputTokens = providerResult.SkipMaxOutputTokens
	a.modelConfig = config

	// Update provider type.
	if config.ModelString != "" {
		if p, _, err := models.ParseModelString(config.ModelString); err == nil {
			a.providerType = p
		}
	}

	// Rebuild the fantasy agent with the new model and current tool set.
	a.rebuildFantasyAgent()

	return nil
}

// GetModel returns the underlying LanguageModel.
func (a *Agent) GetModel() fantasy.LanguageModel {
	return a.model
}

// Close closes the agent and cleans up resources.
// If MCP tools are still loading in the background, Close waits for them
// to finish before closing connections to avoid resource leaks.
func (a *Agent) Close() error {
	// Wait for background MCP loading to finish before closing connections.
	if a.mcpReady != nil {
		<-a.mcpReady
	}
	var toolErr error
	if a.toolManager != nil {
		toolErr = a.toolManager.Close()
	}
	if a.providerCloser != nil {
		if err := a.providerCloser.Close(); err != nil && toolErr == nil {
			toolErr = err
		}
	}
	return toolErr
}

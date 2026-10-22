package extensions

// ProviderDef declares an instance-owned model provider. Create must not execute
// tools. Only Kit executes tools, using the definitions exposed in each request.
// Callbacks are trusted cooperative code: observe Done and join all emitters
// before returning. A successful Create transfers runtime ownership to Kit.
type ProviderDef struct {
	owner                *ProviderOwner
	authCache            *ProviderAuthCache
	ID                   string
	Name                 string
	Models               []ProviderModel
	Auth                 ProviderAuth
	AllowUnknownModels   bool
	UnknownModelDefaults ProviderCapabilities
	SettingsJSON         string
	ValidateSettings     func(string) *ProviderError
	Create               func(ProviderCreateRequest) ProviderCreateResult
}

// ProviderCapabilities is the authoritative capability declaration for a model.
type ProviderCapabilities struct {
	InputModalities []string
	Tools           bool
	Streaming       bool
	Reasoning       bool
	ReasoningLevels []string
}

// ProviderModel describes a selectable model. Limits of zero are unknown.
// Prices are USD per million tokens; Known distinguishes absent and free.
type ProviderModel struct {
	ID              string
	Name            string
	ContextLimit    int64
	OutputLimit     int64
	Capabilities    ProviderCapabilities
	InputPrice      ProviderPrice
	OutputPrice     ProviderPrice
	CacheReadPrice  ProviderPrice
	CacheWritePrice ProviderPrice
	SettingsJSON    string
}

// ProviderPrice distinguishes an unknown price from an explicit zero.
type ProviderPrice struct {
	Known bool
	Value float64
}

// ProviderAuth declares credential resolution, not an OAuth login flow.
type ProviderAuth struct {
	Mode             ProviderAuthMode
	Environment      []string
	SetupInstruction string
	Status           func(ProviderAuthRequest) ProviderAuthResult
}

// ProviderAuthMode identifies how the extension obtains credentials.
type ProviderAuthMode string

const (
	ProviderAuthNone     ProviderAuthMode = "none"
	ProviderAuthAPIKey   ProviderAuthMode = "api_key"
	ProviderAuthExternal ProviderAuthMode = "external"
)

// ProviderAuthRequest bounds an optional safe status check.
type ProviderAuthRequest struct{ Done <-chan struct{} }

// ProviderAuthResult never contains a credential.
type ProviderAuthResult struct {
	Status      ProviderAuthStatus
	Instruction string
	Error       *ProviderError
}

// ProviderAuthStatus identifies safe picker availability.
type ProviderAuthStatus string

const (
	ProviderAuthConfigured   ProviderAuthStatus = "configured"
	ProviderAuthUnconfigured ProviderAuthStatus = "unconfigured"
	ProviderAuthUnknown      ProviderAuthStatus = "unknown"
)

// ProviderCreateRequest contains transient credentials. Never log or persist it.
type ProviderCreateRequest struct {
	Provider   string
	Model      string
	Parameters ProviderParameters
	Endpoint   string
	APIKey     string
	UserAgent  string
	Done       <-chan struct{}
}

// ProviderCreateResult has exactly one terminal error path. On failure, the
// extension owns cleanup of partial resources; on success Kit owns Runtime.
type ProviderCreateResult struct {
	Runtime ProviderRuntime
	Error   *ProviderError
}

// ProviderRuntime is one model runtime. Calls on it are serialized by Kit;
// separate runtimes can be called concurrently. Close is called at most once.
type ProviderRuntime struct {
	Stream func(ProviderRequest, func(ProviderEvent) bool) ProviderResult
	Close  func() error
}

// ProviderResult is returned after Stream has stopped all emitters.
type ProviderResult struct{ Error *ProviderError }

// ProviderError carries safe diagnostic text, an optional code and retry policy.
// Never include credentials, full requests, or sensitive configuration in Message.
type ProviderError struct {
	Code      string
	Message   string
	Retryable bool
}

// ProviderParameters preserves explicit zeros with pointers.
type ProviderParameters struct {
	MaxOutputTokens  *int64
	Temperature      *float64
	TopP             *float64
	TopK             *int64
	PresencePenalty  *float64
	FrequencyPenalty *float64
	StopSequences    []string
	ReasoningLevel   string
	SettingsJSON     string
}

// ProviderRequest contains the selected branch's history and exposed schemas,
// never executable tools or a runtime Context. Emit is synchronous; after false
// it must not be called again. Usage snapshots are cumulative per request.
type ProviderRequest struct {
	Provider   string
	Model      string
	Messages   []ProviderMessage
	Tools      []ProviderTool
	ToolChoice ProviderToolChoice
	Parameters ProviderParameters
	Done       <-chan struct{}
}

// ProviderTool is a JSON object schema, not an executable callback.
type ProviderTool struct {
	Name        string
	Description string
	SchemaJSON  string
}

// ProviderToolChoice selects auto, none, required, or a named exposed tool.
type ProviderToolChoice struct {
	Type ProviderToolChoiceType
	Name string
}

// ProviderToolChoiceType is the tool selection policy.
type ProviderToolChoiceType string

const (
	ProviderToolAuto     ProviderToolChoiceType = "auto"
	ProviderToolNone     ProviderToolChoiceType = "none"
	ProviderToolRequired ProviderToolChoiceType = "required"
	ProviderToolNamed    ProviderToolChoiceType = "named"
)

// ProviderReplayData preserves opaque valid JSON under its originating provider.
// Version 1 is supported. Do not put credentials in replay data.
type ProviderReplayData struct {
	Version  int
	Provider string
	JSON     string
}

// ProviderMessage preserves ordered concrete parts.
type ProviderMessage struct {
	Role   string
	Parts  []ProviderPart
	Replay []ProviderReplayData
}

// ProviderPart is a tagged record. CallID is exact and must not be normalized.
type ProviderPart struct {
	Type          ProviderPartType
	Text          string
	Signature     string
	CallID        string
	ToolName      string
	ArgumentsJSON string
	IsError       bool
	MIMEType      string
	Filename      string
	Data          []byte
	Replay        []ProviderReplayData
}

// ProviderPartType identifies a concrete history part.
type ProviderPartType string

const (
	ProviderPartText       ProviderPartType = "text"
	ProviderPartReasoning  ProviderPartType = "reasoning"
	ProviderPartFile       ProviderPartType = "file"
	ProviderPartToolCall   ProviderPartType = "tool_call"
	ProviderPartToolResult ProviderPartType = "tool_result"
)

// ProviderEvent is one protocol event. Tool-end has no full argument payload:
// Kit assembles argument deltas. Finish and executable calls are withheld until
// Stream returns successfully, so failed protocols cannot execute tools.
type ProviderEvent struct {
	Type         ProviderEventType
	ID           string
	Delta        string
	ToolName     string
	Signature    string
	Replay       []ProviderReplayData
	Usage        ProviderUsage
	FinishReason ProviderFinishReason
}

// ProviderEventType identifies a stream state transition.
type ProviderEventType string

const (
	ProviderTextStart      ProviderEventType = "text_start"
	ProviderTextDelta      ProviderEventType = "text_delta"
	ProviderTextEnd        ProviderEventType = "text_end"
	ProviderReasoningStart ProviderEventType = "reasoning_start"
	ProviderReasoningDelta ProviderEventType = "reasoning_delta"
	ProviderReasoningEnd   ProviderEventType = "reasoning_end"
	ProviderToolStart      ProviderEventType = "tool_start"
	ProviderToolDelta      ProviderEventType = "tool_delta"
	ProviderToolEnd        ProviderEventType = "tool_end"
	ProviderUsageEvent     ProviderEventType = "usage"
	ProviderFinish         ProviderEventType = "finish"
)

// ProviderFinishReason identifies successful termination.
type ProviderFinishReason string

const (
	ProviderFinishStop      ProviderFinishReason = "stop"
	ProviderFinishLength    ProviderFinishReason = "length"
	ProviderFinishToolCalls ProviderFinishReason = "tool_calls"
)

// ProviderUsage is a nonnegative, nondecreasing cumulative snapshot. Kit counts
// the final snapshot once. Cache tokens use the same units as other token counts.
type ProviderUsage struct {
	InputTokens         int64
	OutputTokens        int64
	TotalTokens         int64
	ReasoningTokens     int64
	CacheCreationTokens int64
	CacheReadTokens     int64
}

// RegisterProvider registers metadata and creation callbacks with this load.
// Invalid registrations invalidate the candidate even if Init ignores them.
func (a *API) RegisterProvider(def ProviderDef) { a.registerProviderFn(def) }

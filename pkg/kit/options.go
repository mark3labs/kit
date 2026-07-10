package kit

import "context"

// Option configures a [Kit] created via [NewAgent]. Options are applied in
// order to an [Options] value, so later options override earlier ones. The
// type is a plain func(*Options), so callers can define their own options
// without depending on any internal type.
type Option func(*Options)

// NewAgent creates a Kit using an ergonomic functional-options API. It is a
// thin, additive front door over [New]: the supplied options are applied to a
// fresh [Options] value which is then passed to [New]. For advanced
// configuration not covered by the With* helpers (MCPConfig,
// InProcessMCPServers, session backends, MCP task tuning, etc.) construct an
// [Options] explicitly and call [New].
//
// Streaming defaults to enabled. Pass WithStreaming(false) to disable it.
//
// Example:
//
//	k, err := kit.NewAgent(ctx,
//	    kit.WithModel("anthropic/claude-sonnet-4-5-20250929"),
//	    kit.WithSystemPrompt("You are a helpful assistant."),
//	    kit.WithMaxTokens(8192),
//	    kit.Ephemeral(),
//	)
func NewAgent(ctx context.Context, opts ...Option) (*Kit, error) {
	// Streaming defaults to true for the ergonomic constructor — this is the
	// natural expectation for interactive agents. WithStreaming(false) overrides it.
	streamOn := true
	o := &Options{Streaming: &streamOn}
	for _, fn := range opts {
		fn(o)
	}
	return New(ctx, o)
}

// WithModel sets the model in "provider/model" format
// (e.g. "anthropic/claude-sonnet-4-5-20250929").
func WithModel(m string) Option { return func(o *Options) { o.Model = m } }

// WithSystemPrompt sets the system prompt. The value may be inline text or a
// path to a file whose contents are loaded as the prompt.
func WithSystemPrompt(p string) Option { return func(o *Options) { o.SystemPrompt = p } }

// WithStreaming enables or disables streaming responses. [NewAgent] enables
// streaming by default, so pass WithStreaming(false) to opt out.
func WithStreaming(b bool) Option {
	return func(o *Options) { o.Streaming = &b }
}

// WithMaxTokens sets the maximum output tokens per LLM response. A value of 0
// lets the precedence chain (env → config → per-model → SDK floor) resolve a
// value; a non-zero value pins it and suppresses automatic right-sizing.
func WithMaxTokens(n int) Option { return func(o *Options) { o.MaxTokens = n } }

// WithThinkingLevel sets the reasoning effort for models that support extended
// thinking. Valid values: "off", "none", "minimal", "low", "medium", "high".
// An empty string lets the precedence chain resolve a level.
func WithThinkingLevel(level string) Option { return func(o *Options) { o.ThinkingLevel = level } }

// WithTools sets the agent's tool set, replacing the default core tools. When
// no tools are provided the default set is used.
func WithTools(t ...Tool) Option { return func(o *Options) { o.Tools = t } }

// WithExtraTools adds tools alongside the core/MCP/extension tools rather than
// replacing them.
func WithExtraTools(t ...Tool) Option { return func(o *Options) { o.ExtraTools = t } }

// WithProviderAPIKey overrides the API key used to authenticate with the model
// provider.
func WithProviderAPIKey(key string) Option { return func(o *Options) { o.ProviderAPIKey = key } }

// WithProviderURL overrides the provider endpoint URL. Useful for
// OpenAI-compatible proxies (LiteLLM, vLLM, Azure OpenAI, etc.).
func WithProviderURL(url string) Option { return func(o *Options) { o.ProviderURL = url } }

// WithProviderWire overrides the wire protocol used for auto-routed
// providers: "openai" (Responses API), "openai-compat" (chat completions),
// "anthropic", or "google". Combine with WithProviderURL to point Kit at a
// non-OpenAI-flavored proxy or a provider not in the model database.
func WithProviderWire(wire string) Option { return func(o *Options) { o.ProviderWire = wire } }

// WithConfigFile sets an explicit config file path, overriding the default
// .kit.yml search.
func WithConfigFile(path string) Option { return func(o *Options) { o.ConfigFile = path } }

// WithDebug enables SDK debug logging.
func WithDebug() Option { return func(o *Options) { o.Debug = true } }

// WithDebugLogger installs a caller-supplied [DebugLogger] for low-level
// engine and MCP tool plumbing output. When set this overrides the built-in
// logger selected by [WithDebug] — messages flow into the supplied logger
// unconditionally, and the logger's IsDebugEnabled reports whether downstream
// code should bother formatting them. Use this to forward Kit's debug output
// into your application's logging system (slog, zap, charm/log, an in-app
// panel, etc.).
func WithDebugLogger(l DebugLogger) Option {
	return func(o *Options) { o.DebugLogger = l }
}

// Ephemeral configures an in-memory session with no persistence (equivalent to
// Options.NoSession = true).
func Ephemeral() Option { return func(o *Options) { o.NoSession = true } }

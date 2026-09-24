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

// WithAllowMissingCredentials lets New succeed when the configured provider
// has no credentials. See Options.AllowMissingCredentials and
// Kit.ProviderError.
func WithAllowMissingCredentials() Option {
	return func(o *Options) { o.AllowMissingCredentials = true }
}

// WithProviderURL overrides the provider endpoint URL. Useful for
// OpenAI-compatible proxies (LiteLLM, vLLM, Azure OpenAI, etc.).
func WithProviderURL(url string) Option { return func(o *Options) { o.ProviderURL = url } }

// WithProviderWire overrides the wire protocol used for auto-routed
// providers: "openai" (Responses API), "openai-compat" (chat completions),
// "anthropic", or "google". Combine with WithProviderURL to point Kit at a
// non-OpenAI-flavored proxy or a provider not in the model database.
func WithProviderWire(wire string) Option { return func(o *Options) { o.ProviderWire = wire } }

// WithConfigFile sets an explicit config file path, overriding the default
// .kit.yml search. Only that file is loaded. An explicit file always loads,
// so WithConfigFile also cancels the config part of [Isolated].
func WithConfigFile(path string) Option {
	return func(o *Options) {
		o.ConfigFile = path
		o.SkipConfig = false
	}
}

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

// Isolated turns off everything that Kit otherwise discovers or writes on
// the host by itself. Use it when you embed Kit in an application and want
// the agent to depend only on what the application passes in:
//
//   - no .kit.yml loading, which also means no MCP servers from the user's
//     config (Options.SkipConfig; KIT_* environment variables still apply)
//   - no project context files such as AGENTS.md (Options.NoContextFiles)
//   - no skills (Options.NoSkills)
//   - no extensions (Options.NoExtensions)
//   - no named agent definitions (Options.NoAgents)
//   - an in-memory session only (Options.NoSession)
//   - no core tools, so no file system or shell access
//     (Options.DisableCoreTools)
//
// Options apply in order. Put Isolated first and then turn features back on
// one at a time with [WithConfig], [WithConfigFile], [WithContextFiles],
// [WithSkills], [WithExtensions], [WithAgents], [WithSessions] and
// [WithCoreTools]. [WithTools] and [WithExtraTools] add your own tools.
//
// Subagents that an isolated Kit starts are isolated in the same way.
//
// Isolated does not set Options.Bare. Bare is one switch for all discovery;
// Isolated uses the separate fields so that you can enable each one again.
//
// Example:
//
//	k, err := kit.NewAgent(ctx,
//	    kit.Isolated(),
//	    kit.WithModel("anthropic/claude-sonnet-4-5-20250929"),
//	    kit.WithCoreTools("read", "grep"),
//	)
func Isolated() Option {
	return func(o *Options) {
		o.SkipConfig = true
		o.NoContextFiles = true
		o.NoSkills = true
		o.NoExtensions = true
		o.NoAgents = true
		o.NoSession = true
		o.DisableCoreTools = true
	}
}

// NewIsolatedAgent is [NewAgent] with [Isolated] applied before opts. The
// options in opts can turn the isolated features back on, for example:
//
//	k, err := kit.NewIsolatedAgent(ctx,
//	    kit.WithModel("openai/gpt-4o-mini"),
//	    kit.WithSessions(),
//	)
func NewIsolatedAgent(ctx context.Context, opts ...Option) (*Kit, error) {
	return NewAgent(ctx, append([]Option{Isolated()}, opts...)...)
}

// WithConfig enables .kit.yml discovery again (the home and project config
// files). It cancels the config part of [Isolated]. To load one specific
// file, use [WithConfigFile].
func WithConfig() Option { return func(o *Options) { o.SkipConfig = false } }

// WithContextFiles enables loading of project context files (AGENTS.md)
// from the working directory again.
func WithContextFiles() Option { return func(o *Options) { o.NoContextFiles = false } }

// WithSkills enables skills again. With no paths, Kit discovers skills in
// the user and project skill directories. With paths, Kit loads only those
// skill files or directories.
func WithSkills(paths ...string) Option {
	return func(o *Options) {
		o.NoSkills = false
		if len(paths) > 0 {
			o.Skills = paths
		}
	}
}

// WithExtensions enables loading of extensions again.
func WithExtensions() Option { return func(o *Options) { o.NoExtensions = false } }

// WithAgents enables discovery of named agent definitions again.
func WithAgents() Option { return func(o *Options) { o.NoAgents = false } }

// WithSessions enables persistent sessions again. Session files are written
// under the working directory's session bucket. It cancels [Ephemeral] and
// the session part of [Isolated].
func WithSessions() Option { return func(o *Options) { o.NoSession = false } }

// WithCoreTools enables the core tools again. With no names, all core tools
// are enabled. With names, only those tools are enabled (for example
// "read", "grep"); see [ListAllCoreToolNames] for the valid names.
func WithCoreTools(names ...string) Option {
	return func(o *Options) {
		o.DisableCoreTools = false
		if len(names) > 0 {
			o.CoreToolList = names
		}
	}
}

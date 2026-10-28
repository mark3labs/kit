var e={frontmatter:{title:`SDK Options`,description:`Configuration options for the Kit Go SDK.`,hidden:!1,toc:!0,ogImage:`/og-image.png`,draft:!1},html:`<h1 id="sdk-options"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#sdk-options"><span class="icon icon-link"></span></a>SDK Options</h1>
<p>Pass an <code>Options</code> struct to <code>kit.New()</code> to configure the Kit instance.</p>
<blockquote>
<p><strong>Tip:</strong> for simple setups, <code>kit.NewAgent(ctx, ...Option)</code> provides
functional-options helpers (<code>WithModel</code>, <code>WithStreaming</code>, <code>Ephemeral</code>, ...)
over the same <code>Options</code> struct. See
<a href="/sdk/overview#functional-options-newagent">Functional options</a>.</p>
<p>To embed Kit with no config, context files, skills, extensions, named
agents, session files or core tools from the host, use
<code>kit.NewIsolatedAgent</code>. It sets the fields below for you. See
<a href="/sdk/overview#isolated-agents">Isolated agents</a>.</p>
</blockquote>
<p>Each <code>kit.New</code> / <code>kit.NewAgent</code> call owns an isolated configuration store, so
these options never leak between Kit instances in the same process. See
<a href="/sdk/overview#per-instance-config-isolation">Per-instance config isolation</a>.</p>
<h2 id="http-request-identity"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#http-request-identity"><span class="icon icon-link"></span></a>HTTP request identity</h2>
<p>Kit-controlled HTTP requests use <code>User-Agent: KIT</code> by default. Set a per-instance
identity with either API:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">host, err </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> kit.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">New</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(ctx, </span><span style="color:#D73A49;--shiki-dark:#F97583">&amp;</span><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Options</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{UserAgent: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"MyApp/1.0"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// Or use functional options:</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">host, err </span><span style="color:#D73A49;--shiki-dark:#F97583">=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> kit.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">NewAgent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(ctx, kit.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">WithUserAgent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"MyApp/1.0"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">))</span></span></code></pre>
<p>Empty means <code>KIT</code>, not an opt-out. Valid values are preserved without trimming,
including spaces. HTTP control characters (including CR, LF, NUL and DEL) are
rejected before construction or login does network work. This setting has no
CLI flag, environment variable, or config-file key.</p>
<p>Each instance keeps its own identity. It passes to model changes, temporary
completions and subagents, including isolated/bare children and provider changes.
It covers built-in model requests (including streams), Ollama setup, remote MCP
and their OAuth requests, and provider token refresh. The identity replaces
conflicting user-agent headers from model defaults, catalog headers, per-call
headers and MCP configuration, case-insensitively. Other headers stay unchanged.
Custom provider factories receive <code>ProviderConfig.UserAgent</code> and must enforce it
on their own requests; Kit does not intercept arbitrary factory traffic.</p>
<p>Two inference paths keep their fixed compatibility identities, even with an
SDK override. Their token endpoints still use the configured Kit identity.
These exceptions follow existing source requirements; they were not live-tested.</p>
<table>
<thead>
<tr>
<th>Inference path</th>
<th>Fixed user agent</th>
</tr>
</thead>
<tbody>
<tr>
<td>Codex</td>
<td><code>Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36</code></td>
</tr>
<tr>
<td>Copilot</td>
<td><code>GitHubCopilotChat/0.31.0</code></td>
</tr>
</tbody>
</table>
<p>Standalone <code>kit.LoginProvider</code> has a separate <code>kit.LoginOptions.UserAgent</code> field
(default <code>KIT</code>), independent of any Kit instance or supplied credential manager.
Standalone CLI OAuth and model-catalog downloads use <code>KIT</code>.</p>
<p>This policy does not change global HTTP defaults. It does not cover subprocesses
(shell, git/gh/curl, installers, npm or MCP server processes), extension/application
code, browser navigation, or remote services acting for Kit. Third-party cloud
credential discovery (metadata services or default credential chains) inherits
the identity only when it uses the HTTP client Kit supplies; model endpoint
requests use the configured identity. A custom transport can rewrite headers
after Kit's boundary and is responsible for its own policy.</p>
<h2 id="full-options-reference"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#full-options-reference"><span class="icon icon-link"></span></a>Full options reference</h2>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">host, err </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> kit.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">New</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(ctx, </span><span style="color:#D73A49;--shiki-dark:#F97583">&amp;</span><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Options</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // Model</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Model:        </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"ollama/llama3"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    SystemPrompt: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"You are a helpful bot"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    ConfigFile:   </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"/path/to/config.yml"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // Behavior</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    MaxSteps:     </span><span style="color:#005CC5;--shiki-dark:#79B8FF">10</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Streaming:    </span><span style="color:#6F42C1;--shiki-dark:#B392F0">ptrBool</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#005CC5;--shiki-dark:#79B8FF">true</span><span style="color:#24292E;--shiki-dark:#E1E4E8">), </span><span style="color:#6A737D;--shiki-dark:#6A737D">// *bool: nil = unset (default true), &amp;false = off</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Quiet:        </span><span style="color:#005CC5;--shiki-dark:#79B8FF">true</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Debug:        </span><span style="color:#005CC5;--shiki-dark:#79B8FF">true</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    DebugLogger:  myLogger,       </span><span style="color:#6A737D;--shiki-dark:#6A737D">// optional; overrides Debug + built-in logger when non-nil</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // Generation parameters (override env/config/per-model defaults)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    MaxTokens:        </span><span style="color:#005CC5;--shiki-dark:#79B8FF">16384</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,              </span><span style="color:#6A737D;--shiki-dark:#6A737D">// 0 = auto-resolve; non-zero suppresses right-sizing</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    ThinkingLevel:    </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"medium"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,           </span><span style="color:#6A737D;--shiki-dark:#6A737D">// model-specific; see SupportedThinkingLevels</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Temperature:      </span><span style="color:#6F42C1;--shiki-dark:#B392F0">ptrFloat32</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#005CC5;--shiki-dark:#79B8FF">0.2</span><span style="color:#24292E;--shiki-dark:#E1E4E8">),    </span><span style="color:#6A737D;--shiki-dark:#6A737D">// pointer so explicit 0.0 != unset</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    TopP:             </span><span style="color:#005CC5;--shiki-dark:#79B8FF">nil</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,                 </span><span style="color:#6A737D;--shiki-dark:#6A737D">// nil = provider/per-model default</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    TopK:             </span><span style="color:#005CC5;--shiki-dark:#79B8FF">nil</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    FrequencyPenalty: </span><span style="color:#005CC5;--shiki-dark:#79B8FF">nil</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    PresencePenalty:  </span><span style="color:#005CC5;--shiki-dark:#79B8FF">nil</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // Provider configuration</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    ProviderAPIKey: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"sk-..."</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,                      </span><span style="color:#6A737D;--shiki-dark:#6A737D">// "" = use config / provider env var</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    ProviderURL:    </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"https://proxy.internal/v1"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,  </span><span style="color:#6A737D;--shiki-dark:#6A737D">// "" = provider default endpoint</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    ProviderWire:   </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"anthropic"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,                  </span><span style="color:#6A737D;--shiki-dark:#6A737D">// "" = infer wire from model database</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    TLSSkipVerify:  </span><span style="color:#005CC5;--shiki-dark:#79B8FF">false</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,                         </span><span style="color:#6A737D;--shiki-dark:#6A737D">// only effective when true</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Providers: </span><span style="color:#D73A49;--shiki-dark:#F97583">map</span><span style="color:#24292E;--shiki-dark:#E1E4E8">[</span><span style="color:#D73A49;--shiki-dark:#F97583">string</span><span style="color:#24292E;--shiki-dark:#E1E4E8">]</span><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ProviderFactory</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{    </span><span style="color:#6A737D;--shiki-dark:#6A737D">// app-supplied backends for "local/&lt;model&gt;"</span></span>
<span class="line"><span style="color:#032F62;--shiki-dark:#9ECBFF">        "local"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: localFactory,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    },</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // Session</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    SessionPath:  </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"./session.jsonl"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    SessionDir:   </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"/home/user/project"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#6A737D;--shiki-dark:#6A737D">// project identity for discovery, not storage root</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Continue:     </span><span style="color:#005CC5;--shiki-dark:#79B8FF">true</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    NoSession:    </span><span style="color:#005CC5;--shiki-dark:#79B8FF">true</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // Tools</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Tools:            []</span><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Tool</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span><span style="color:#D73A49;--shiki-dark:#F97583">...</span><span style="color:#24292E;--shiki-dark:#E1E4E8">},     </span><span style="color:#6A737D;--shiki-dark:#6A737D">// Replace default core tools; MCP and extension tools remain</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    ExtraTools:       []</span><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Tool</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span><span style="color:#D73A49;--shiki-dark:#F97583">...</span><span style="color:#24292E;--shiki-dark:#E1E4E8">},     </span><span style="color:#6A737D;--shiki-dark:#6A737D">// Add tools alongside defaults (mutate at runtime via host.AddTools/RemoveTools)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    DisableCoreTools: </span><span style="color:#005CC5;--shiki-dark:#79B8FF">true</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,                </span><span style="color:#6A737D;--shiki-dark:#6A737D">// Use no core tools (0 tools, for chat-only)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    CoreToolList:     []</span><span style="color:#D73A49;--shiki-dark:#F97583">string</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,            </span><span style="color:#6A737D;--shiki-dark:#6A737D">// List of core tools to include, if empty (default) include all</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    WorkDir:          </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"/path/to/project"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,  </span><span style="color:#6A737D;--shiki-dark:#6A737D">// Base directory of the file and shell tools (default: process cwd)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    CoreToolOptions:  []</span><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ToolOption</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span><span style="color:#D73A49;--shiki-dark:#F97583">...</span><span style="color:#24292E;--shiki-dark:#E1E4E8">}, </span><span style="color:#6A737D;--shiki-dark:#6A737D">// Extra options for the core tools, e.g. kit.WithFileSystem(fs)</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // Configuration</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    SkipConfig:   </span><span style="color:#005CC5;--shiki-dark:#79B8FF">true</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,                   </span><span style="color:#6A737D;--shiki-dark:#6A737D">// Skip .kit.yml files (viper defaults + env vars still apply)</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // Compaction</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    AutoCompact:  </span><span style="color:#005CC5;--shiki-dark:#79B8FF">true</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // Skills</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Skills:        []</span><span style="color:#D73A49;--shiki-dark:#F97583">string</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"/path/to/skill.md"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">},</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    SkillsDir:     </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"/path/to/skills/"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    SkillsDisable: []</span><span style="color:#D73A49;--shiki-dark:#F97583">string</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"noisy-skill"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">},</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    NoSkills:      </span><span style="color:#005CC5;--shiki-dark:#79B8FF">true</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // Feature toggles</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    NoExtensions:   </span><span style="color:#005CC5;--shiki-dark:#79B8FF">true</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,               </span><span style="color:#6A737D;--shiki-dark:#6A737D">// disable Yaegi extension loading</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    NoContextFiles: </span><span style="color:#005CC5;--shiki-dark:#79B8FF">true</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,               </span><span style="color:#6A737D;--shiki-dark:#6A737D">// disable automatic AGENTS.md loading</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    NoAgents:       </span><span style="color:#005CC5;--shiki-dark:#79B8FF">true</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,               </span><span style="color:#6A737D;--shiki-dark:#6A737D">// disable named agent discovery (.agents/agents/, .kit/agents/, ~/.config/kit/agents/, and built-ins)</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // Session (advanced)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    SessionManager: myCustomSession,    </span><span style="color:#6A737D;--shiki-dark:#6A737D">// custom SessionManager implementation</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // MCP OAuth — both opt-in. Leave MCPAuthHandler nil to disable</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // OAuth entirely (remote MCP 401s bubble up as errors). CLI apps</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // pass kit.NewCLIMCPAuthHandler(); custom UX embedders implement</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // MCPAuthHandler or configure DefaultMCPAuthHandler + OnAuthURL.</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    MCPAuthHandler: authHandler,                  </span><span style="color:#6A737D;--shiki-dark:#6A737D">// nil = OAuth disabled</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    MCPTokenStoreFactory: </span><span style="color:#D73A49;--shiki-dark:#F97583">func</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#E36209;--shiki-dark:#FFAB70">serverURL</span><span style="color:#D73A49;--shiki-dark:#F97583"> string</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) (</span><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">MCPTokenStore</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#D73A49;--shiki-dark:#F97583">error</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) {</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">        return</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> myStore</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(serverURL), </span><span style="color:#005CC5;--shiki-dark:#79B8FF">nil</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    },</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // In-Process MCP Servers</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    InProcessMCPServers: </span><span style="color:#D73A49;--shiki-dark:#F97583">map</span><span style="color:#24292E;--shiki-dark:#E1E4E8">[</span><span style="color:#D73A49;--shiki-dark:#F97583">string</span><span style="color:#24292E;--shiki-dark:#E1E4E8">]</span><span style="color:#D73A49;--shiki-dark:#F97583">*</span><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">MCPServer</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span></span>
<span class="line"><span style="color:#032F62;--shiki-dark:#9ECBFF">        "docs"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: mcpSrv,  </span><span style="color:#6A737D;--shiki-dark:#6A737D">// *server.MCPServer from mcp-go</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    },</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span></code></pre>
<h2 id="options-fields"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#options-fields"><span class="icon icon-link"></span></a>Options fields</h2>
<h3 id="core"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#core"><span class="icon icon-link"></span></a>Core</h3>
<table>
<thead>
<tr>
<th>Field</th>
<th>Type</th>
<th>Default</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td><code>Model</code></td>
<td><code>string</code></td>
<td>config default</td>
<td>Model string (provider/model format)</td>
</tr>
<tr>
<td><code>DecisionModel</code></td>
<td><code>string</code></td>
<td>config default</td>
<td>Default <a href="/sdk/decisions">decision model</a> for <code>Kit.Decide</code> and extension <code>ctx.Decide</code> when a request names no model</td>
</tr>
<tr>
<td><code>SystemPrompt</code></td>
<td><code>string</code></td>
<td>—</td>
<td>System prompt text or file path</td>
</tr>
<tr>
<td><code>ConfigFile</code></td>
<td><code>string</code></td>
<td>empty</td>
<td>Explicit path; when unset, discover project config before home config</td>
</tr>
<tr>
<td><code>MaxSteps</code></td>
<td><code>int</code></td>
<td><code>0</code></td>
<td>Max agent steps (0 = use the configured default)</td>
</tr>
<tr>
<td><code>Streaming</code></td>
<td><code>*bool</code></td>
<td><code>nil</code></td>
<td>Enable streaming output. <code>nil</code> leaves it to the precedence chain (env → config → default <code>true</code>); <code>&amp;true</code>/<code>&amp;false</code> forces it. Pointer so unset is distinct from explicit <code>false</code>.</td>
</tr>
<tr>
<td><code>Quiet</code></td>
<td><code>bool</code></td>
<td><code>false</code></td>
<td>Suppress output</td>
</tr>
<tr>
<td><code>Debug</code></td>
<td><code>bool</code></td>
<td><code>false</code></td>
<td>Enable debug logging via the built-in console / buffered logger. Ignored when <code>DebugLogger</code> is non-nil.</td>
</tr>
<tr>
<td><code>DebugLogger</code></td>
<td><code>DebugLogger</code></td>
<td><code>nil</code></td>
<td>Caller-supplied logger that receives low-level engine + MCP tool plumbing debug output. When non-nil this overrides <code>Debug</code> — the supplied logger's <code>IsDebugEnabled()</code> controls downstream emission. See <a href="#custom-debug-logger">Custom debug logger</a>.</td>
</tr>
</tbody>
</table>
<h3 id="generation-parameters"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#generation-parameters"><span class="icon icon-link"></span></a>Generation parameters</h3>
<p>These fields override the corresponding values from <code>.kit.yml</code> / <code>KIT_*</code>
environment variables. Leaving a field at its zero/nil value lets the
precedence chain resolve a value (<code>KIT_*</code> env → config file → per-model
defaults from <code>modelSettings</code>/<code>customModels</code> → an 8192 SDK floor for
<code>MaxTokens</code> (matching the CLI <code>--max-tokens</code> default) and provider-level
defaults for samplers).</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Type</th>
<th>Default</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td><code>MaxTokens</code></td>
<td><code>int</code></td>
<td>auto-resolved</td>
<td>Max output tokens per response. <code>0</code> = auto-resolve; non-zero suppresses automatic right-sizing (same semantics as <code>--max-tokens</code>).</td>
</tr>
<tr>
<td><code>ThinkingLevel</code></td>
<td><code>string</code></td>
<td>auto-resolved</td>
<td>Case-sensitive, model-specific effort. <code>off</code> omits the field; <code>none</code> is literal. <code>""</code> falls through to config/env/per-model/<code>"off"</code>. Explicit unsupported selections return an error; use <a href="/sdk/overview#reasoning-support-per-model"><code>SupportedThinkingLevels</code></a> for catalog choices (which may include future names).</td>
</tr>
<tr>
<td><code>Temperature</code></td>
<td><code>*float32</code></td>
<td>—</td>
<td>Sampling randomness. Pointer type so explicit <code>0.0</code> is distinguishable from "unset".</td>
</tr>
<tr>
<td><code>TopP</code></td>
<td><code>*float32</code></td>
<td>—</td>
<td>Nucleus sampling cutoff. <code>nil</code> leaves provider/per-model default.</td>
</tr>
<tr>
<td><code>TopK</code></td>
<td><code>*int32</code></td>
<td>—</td>
<td>Top-K sampling limit. <code>nil</code> leaves provider/per-model default.</td>
</tr>
<tr>
<td><code>FrequencyPenalty</code></td>
<td><code>*float32</code></td>
<td>—</td>
<td>OpenAI-family frequency penalty. <code>nil</code> leaves provider default.</td>
</tr>
<tr>
<td><code>PresencePenalty</code></td>
<td><code>*float32</code></td>
<td>—</td>
<td>OpenAI-family presence penalty. <code>nil</code> leaves provider default.</td>
</tr>
</tbody>
</table>
<p>Pointer-typed fields (<code>Streaming</code> and the samplers) are populated via tiny helpers:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">func</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> ptrBool</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#E36209;--shiki-dark:#FFAB70">v</span><span style="color:#D73A49;--shiki-dark:#F97583"> bool</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) </span><span style="color:#D73A49;--shiki-dark:#F97583">*bool</span><span style="color:#24292E;--shiki-dark:#E1E4E8">          { </span><span style="color:#D73A49;--shiki-dark:#F97583">return</span><span style="color:#D73A49;--shiki-dark:#F97583"> &amp;</span><span style="color:#24292E;--shiki-dark:#E1E4E8">v }</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">func</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> ptrFloat32</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#E36209;--shiki-dark:#FFAB70">v</span><span style="color:#D73A49;--shiki-dark:#F97583"> float32</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) </span><span style="color:#D73A49;--shiki-dark:#F97583">*float32</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> { </span><span style="color:#D73A49;--shiki-dark:#F97583">return</span><span style="color:#D73A49;--shiki-dark:#F97583"> &amp;</span><span style="color:#24292E;--shiki-dark:#E1E4E8">v }</span></span></code></pre>
<p>These fields eliminate the need for <code>viper.Set()</code> calls before <code>kit.New()</code>
when embedding Kit as a library.</p>
<h3 id="provider-configuration"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#provider-configuration"><span class="icon icon-link"></span></a>Provider configuration</h3>
<table>
<thead>
<tr>
<th>Field</th>
<th>Type</th>
<th>Default</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td><code>ProviderAPIKey</code></td>
<td><code>string</code></td>
<td>—</td>
<td>API key used to authenticate with the provider. <code>""</code> falls back to config / provider-specific env var (e.g. <code>ANTHROPIC_API_KEY</code>). When set, it takes precedence over config and env values on this instance's store.</td>
</tr>
<tr>
<td><code>ProviderURL</code></td>
<td><code>string</code></td>
<td>—</td>
<td>Override the provider endpoint (e.g. LiteLLM, vLLM, Azure OpenAI, internal proxy). <code>""</code> = provider default.</td>
</tr>
<tr>
<td><code>ProviderWire</code></td>
<td><code>string</code></td>
<td>—</td>
<td>Override the wire protocol for auto-routed providers: <code>openai</code> (Responses API), <code>openai-compat</code> (chat completions), <code>anthropic</code>, or <code>google</code>. <code>""</code> = infer from the model database. Takes precedence over per-provider <code>wire</code> declarations in the <a href="/configuration#provider-overrides"><code>providers</code> config section</a>. Combine with <code>ProviderURL</code> to target providers not in the database.</td>
</tr>
<tr>
<td><code>TLSSkipVerify</code></td>
<td><code>bool</code></td>
<td><code>false</code></td>
<td>Disable TLS certificate verification on the provider HTTP client. Only effective when <code>true</code>; to force-disable, use config file or env var instead. For self-signed dev certs only.</td>
</tr>
<tr>
<td><code>Providers</code></td>
<td><code>map[string]ProviderFactory</code></td>
<td>—</td>
<td>Provider factories for this instance, keyed by provider name (case-insensitive, no <code>/</code>). A model string <code>name/&lt;model&gt;</code> is built by the factory. Wins over <code>kit.RegisterProvider</code> and the built-in providers; subagents inherit it. <code>kit.New</code> copies the map and rejects nil factories and names that differ only in case. See <a href="/sdk/overview#custom-provider-backends">Custom provider backends</a>.</td>
</tr>
<tr>
<td><code>DecisionProviders</code></td>
<td><code>map[string]DecisionProviderFactory</code></td>
<td>—</td>
<td>Decision provider factories for this instance, with the same name rules as <code>Providers</code>. <code>Kit.Decide</code> (and extension <code>ctx.Decide</code>) sends requests for <code>name/&lt;model&gt;</code> to the factory model instead of the model database. Wins over <code>kit.RegisterDecisionProvider</code>; subagents inherit it. See <a href="/sdk/decisions#custom-decision-providers">custom decision providers</a>.</td>
</tr>
</tbody>
</table>
<h3 id="session"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#session"><span class="icon icon-link"></span></a>Session</h3>
<table>
<thead>
<tr>
<th>Field</th>
<th>Type</th>
<th>Default</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td><code>SessionPath</code></td>
<td><code>string</code></td>
<td>—</td>
<td>Open a specific session file</td>
</tr>
<tr>
<td><code>SessionDir</code></td>
<td><code>string</code></td>
<td>—</td>
<td>Base directory for session discovery</td>
</tr>
<tr>
<td><code>Continue</code></td>
<td><code>bool</code></td>
<td><code>false</code></td>
<td>Resume most recent session</td>
</tr>
<tr>
<td><code>NoSession</code></td>
<td><code>bool</code></td>
<td><code>false</code></td>
<td>Ephemeral mode (no persistence)</td>
</tr>
<tr>
<td><code>SessionManager</code></td>
<td><code>SessionManager</code></td>
<td>—</td>
<td>Custom session backend (advanced)</td>
</tr>
</tbody>
</table>
<h3 id="tools--extensions"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#tools--extensions"><span class="icon icon-link"></span></a>Tools &amp; extensions</h3>
<table>
<thead>
<tr>
<th>Field</th>
<th>Type</th>
<th>Default</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td><code>Tools</code></td>
<td><code>[]Tool</code></td>
<td>—</td>
<td>Replace default core tools; MCP and extension tools remain</td>
</tr>
<tr>
<td><code>ExtraTools</code></td>
<td><code>[]Tool</code></td>
<td>—</td>
<td>Additional tools alongside core/MCP/extension tools</td>
</tr>
<tr>
<td><code>DisableCoreTools</code></td>
<td><code>bool</code></td>
<td><code>false</code></td>
<td>Use no core tools (0 tools, for chat-only)</td>
</tr>
<tr>
<td><code>CoreToolList</code></td>
<td><code>[]string</code></td>
<td>—</td>
<td>Allow-list of core tool names; empty/nil means all. Build with <a href="/sdk/overview#filtering-core-tools"><code>FilterCoreToolNames</code></a> from include/exclude filters.</td>
</tr>
<tr>
<td><code>WorkDir</code></td>
<td><code>string</code></td>
<td>process cwd</td>
<td>Directory the core file and shell tools resolve relative paths against and run commands in. Subagents inherit it.</td>
</tr>
<tr>
<td><code>CoreToolOptions</code></td>
<td><code>[]ToolOption</code></td>
<td>—</td>
<td>Extra options for the core tools, applied last. Use <a href="#routing-file-access-and-commands"><code>WithFileSystem</code></a> and <a href="#routing-file-access-and-commands"><code>WithCommandRunner</code></a> here. Subagents inherit them.</td>
</tr>
<tr>
<td><code>NoExtensions</code></td>
<td><code>bool</code></td>
<td><code>false</code></td>
<td>Disable Yaegi extension loading</td>
</tr>
<tr>
<td><code>NoContextFiles</code></td>
<td><code>bool</code></td>
<td><code>false</code></td>
<td>Disable automatic AGENTS.md loading</td>
</tr>
<tr>
<td><code>NoAgents</code></td>
<td><code>bool</code></td>
<td><code>false</code></td>
<td>Disable named agent discovery (built-ins and <code>.agents/agents/</code> / <code>.kit/agents/</code> / <code>~/.config/kit/agents/</code> files); see <a href="/advanced/subagents#named-agents">Subagents</a></td>
</tr>
</tbody>
</table>
<p>Unless <code>NoExtensions</code> is set, an embedded Kit uses the same
<a href="/extensions/loading#auto-discovery">auto-discovery paths</a> as the CLI, which
include the system-wide <code>/usr/share/kit/extensions</code> directory. Set
<code>NoExtensions: true</code> for a hermetic embed that must not pick up extensions
from the host machine. <a href="/sdk/overview#isolated-agents"><code>kit.Isolated()</code></a>
sets it together with the other discovery switches.</p>
<h3 id="skills--configuration"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#skills--configuration"><span class="icon icon-link"></span></a>Skills &amp; configuration</h3>
<table>
<thead>
<tr>
<th>Field</th>
<th>Type</th>
<th>Default</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td><code>SkipConfig</code></td>
<td><code>bool</code></td>
<td><code>false</code></td>
<td>Skip <code>.kit.yml</code> file loading (viper defaults + env vars still apply)</td>
</tr>
<tr>
<td><code>Skills</code></td>
<td><code>[]string</code></td>
<td>—</td>
<td>Explicit skill files/dirs to load</td>
</tr>
<tr>
<td><code>SkillsDir</code></td>
<td><code>string</code></td>
<td>—</td>
<td>Scan this directory directly for skills (overrides auto-discovery; scanned as-is)</td>
</tr>
<tr>
<td><code>SkillsDisable</code></td>
<td><code>[]string</code></td>
<td>—</td>
<td>Skill names to hide from the model catalog (still usable via <code>/&lt;name&gt;</code>)</td>
</tr>
<tr>
<td><code>SkillTrustPrompt</code></td>
<td><code>func(projectDir string, skillCount int) TrustDecision</code></td>
<td><code>nil</code></td>
<td>Callback gating project-local skill loading on a trust decision (see below)</td>
</tr>
<tr>
<td><code>NoSkills</code></td>
<td><code>bool</code></td>
<td><code>false</code></td>
<td>Disable skill loading entirely</td>
</tr>
</tbody>
</table>
<h4 id="project-skill-trust-gate"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#project-skill-trust-gate"><span class="icon icon-link"></span></a>Project-skill trust gate</h4>
<p>Project-local skills (under <code>&lt;project&gt;/.agents/skills/</code> or <code>&lt;project&gt;/.kit/skills/</code>)
are injected into the system prompt, so loading them from an untrusted, freshly
cloned repository is a prompt-injection vector. Set <code>SkillTrustPrompt</code> to gate
that first load on an explicit decision. When <code>nil</code> (the default), project
skills load without prompting — preserving historical behaviour.</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">opts </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#D73A49;--shiki-dark:#F97583"> &amp;</span><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Options</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    SkillTrustPrompt: </span><span style="color:#D73A49;--shiki-dark:#F97583">func</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#E36209;--shiki-dark:#FFAB70">projectDir</span><span style="color:#D73A49;--shiki-dark:#F97583"> string</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#E36209;--shiki-dark:#FFAB70">skillCount</span><span style="color:#D73A49;--shiki-dark:#F97583"> int</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) </span><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">TrustDecision</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> {</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">        // Consult your own UI / policy here.</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">        if</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> userApproves</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(projectDir, skillCount) {</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">            return</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> kit.TrustProject     </span><span style="color:#6A737D;--shiki-dark:#6A737D">// load and persist the directory as trusted</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">        }</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">        return</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> kit.SkipProjectSkills    </span><span style="color:#6A737D;--shiki-dark:#6A737D">// do not load project skills</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    },</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">}</span></span></code></pre>
<p>The callback returns one of three <code>TrustDecision</code> values:</p>
<table>
<thead>
<tr>
<th>Decision</th>
<th>Effect</th>
</tr>
</thead>
<tbody>
<tr>
<td><code>kit.TrustProject</code></td>
<td>Load project skills and persist <code>projectDir</code> to <code>~/.config/kit/trusted-projects.json</code> (not prompted again)</td>
</tr>
<tr>
<td><code>kit.TrustProjectOnce</code></td>
<td>Load project skills for this run only, without persisting</td>
</tr>
<tr>
<td><code>kit.SkipProjectSkills</code></td>
<td>Do not load project skills</td>
</tr>
</tbody>
</table>
<p>A directory already on the persisted allowlist is trusted without invoking the
callback. The Kit CLI wires this to an interactive terminal prompt automatically
for TTY sessions.</p>
<p>These fields only control the <strong>initial</strong> skill and context-file set picked
up by <code>New()</code>. To add, remove, or replace skills and <code>AGENTS.md</code>-style
context files at runtime (e.g. per user or per session), use the
<code>AddSkill</code> / <code>LoadAndAddSkill</code> / <code>RemoveSkill</code> / <code>SetSkills</code> /
<code>DisableSkill</code> / <code>EnableSkill</code> and
<code>AddContextFile</code> / <code>AddContextFileContent</code> / <code>RemoveContextFile</code> /
<code>SetContextFiles</code> methods on <code>*kit.Kit</code>. See
<a href="/sdk/overview#runtime-skills-and-context-files">Runtime skills and context files</a>.</p>
<h3 id="compaction--mcp"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#compaction--mcp"><span class="icon icon-link"></span></a>Compaction &amp; MCP</h3>
<table>
<thead>
<tr>
<th>Field</th>
<th>Type</th>
<th>Default</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td><code>AutoCompact</code></td>
<td><code>bool</code></td>
<td><code>false</code></td>
<td>Compact proactively before turns that near the context limit. Independent of this setting, Kit always compacts <strong>reactively</strong> and replays the turn once when a provider call fails with a context-overflow error.</td>
</tr>
<tr>
<td><code>CompactionOptions</code></td>
<td><code>*CompactionOptions</code></td>
<td>—</td>
<td>Configuration for compaction; zero-value budget fields adapt to the model's limits. See <a href="#compactionoptions">CompactionOptions</a> below.</td>
</tr>
<tr>
<td><code>MCPConfig</code></td>
<td><code>*Config</code></td>
<td>—</td>
<td>Pre-loaded MCP configuration. When set, <code>LoadAndValidateConfig</code> is skipped during Kit creation — useful for in-process subagents (inheriting a parent's loaded config) and programmatic setups that build config without reading <code>.kit.yml</code>.</td>
</tr>
<tr>
<td><code>MCPAuthHandler</code></td>
<td><code>MCPAuthHandler</code></td>
<td>—</td>
<td>OAuth handler for remote MCP servers. <code>nil</code> disables OAuth (servers returning 401 fail with the authorization-required error). See <a href="#mcp-oauth-authorization">MCP OAuth</a> below.</td>
</tr>
<tr>
<td><code>MCPTokenStoreFactory</code></td>
<td><code>func</code></td>
<td>—</td>
<td>Custom OAuth token storage for MCP servers (default: JSON file in <code>$XDG_CONFIG_HOME/.kit/mcp_tokens.json</code>).</td>
</tr>
<tr>
<td><code>InProcessMCPServers</code></td>
<td><code>map[string]*MCPServer</code></td>
<td>—</td>
<td>In-process mcp-go servers (no subprocess)</td>
</tr>
<tr>
<td><code>MCPTaskMode</code></td>
<td><code>map[string]MCPTaskMode</code></td>
<td>—</td>
<td>Per-server override for task-augmented <code>tools/call</code>. Keys are server names; missing entries fall back to the <code>tasksMode</code> field of the matching <code>MCPServerConfig</code>. See <a href="#mcp-tasks">MCP Tasks</a>.</td>
</tr>
<tr>
<td><code>MCPTaskTimeout</code></td>
<td><code>time.Duration</code></td>
<td><code>15m</code></td>
<td>Maximum wall-clock to wait for a task to reach a terminal state. Independent of any per-call context deadline.</td>
</tr>
<tr>
<td><code>MCPTaskTTL</code></td>
<td><code>time.Duration</code></td>
<td>—</td>
<td>TTL hint sent in <code>TaskParams</code> for every task-augmented call. Zero omits the field and lets the server pick.</td>
</tr>
<tr>
<td><code>MCPTaskPollInterval</code></td>
<td><code>time.Duration</code></td>
<td><code>1s</code></td>
<td>Fallback interval between <code>tasks/get</code> requests when the server does not suggest one.</td>
</tr>
<tr>
<td><code>MCPTaskMaxPollInterval</code></td>
<td><code>time.Duration</code></td>
<td><code>5s</code></td>
<td>Cap on the polling interval (a server-supplied <code>pollInterval</code> can otherwise grow without bound).</td>
</tr>
<tr>
<td><code>MCPTaskProgress</code></td>
<td><code>MCPTaskProgressHandler</code></td>
<td>—</td>
<td>Optional callback invoked once when a task is accepted and on every observed status transition. The final invocation always carries a terminal status.</td>
</tr>
</tbody>
</table>
<h3 id="compactionoptions"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#compactionoptions"><span class="icon icon-link"></span></a>CompactionOptions</h3>
<p><code>CompactionOptions</code> controls how conversations are summarized. The token
budgets are adaptive: leave a field at zero and it scales to the model's
limits, or set an explicit value to override.</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Type</th>
<th>Zero-value behaviour</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td><code>ContextWindow</code></td>
<td><code>int</code></td>
<td>auto-populated from the model registry</td>
<td>Model's context window size in tokens</td>
</tr>
<tr>
<td><code>MaxOutputTokens</code></td>
<td><code>int</code></td>
<td>auto-populated from the model registry</td>
<td>Model's max output tokens; scales the adaptive <code>ReserveTokens</code></td>
</tr>
<tr>
<td><code>ReserveTokens</code></td>
<td><code>int</code></td>
<td><code>min(16384, MaxOutputTokens)</code></td>
<td>Tokens reserved for the LLM response</td>
</tr>
<tr>
<td><code>KeepRecentTokens</code></td>
<td><code>int</code></td>
<td>a quarter of the usable context (<code>ContextWindow − ReserveTokens</code>), floored at 2000</td>
<td>Recent tokens preserved verbatim (not summarized)</td>
</tr>
<tr>
<td><code>SummaryPrompt</code></td>
<td><code>string</code></td>
<td>built-in structured checkpoint prompt</td>
<td>Custom summarization prompt</td>
</tr>
</tbody>
</table>
<p>When a session compacts more than once, the previous summary is fed back into
the summarization prompt and updated incrementally (anchored summaries), so
detail is preserved across compaction generations.</p>
<h4 id="reactive-compaction-on-context-overflow"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#reactive-compaction-on-context-overflow"><span class="icon icon-link"></span></a>Reactive compaction on context overflow</h4>
<p>Token estimates inevitably drift from real tokenizer counts, and a single
huge mid-turn tool result can overflow the context even when the turn started
well under the limit. When the provider rejects a request with a
context-overflow error, Kit automatically:</p>
<ol>
<li>Compacts the conversation (persisting any completed steps first, so the
replay resumes rather than restarts).</li>
<li>Replays the turn once with the rebuilt context, replacing media
attachments with text placeholders (media cannot be shrunk by
summarization).</li>
<li>If the replay still overflows, fails the turn with <code>kit.ErrContextOverflow</code>
("conversation too large to compact").</li>
</ol>
<p>This safety net is always on — it does not require <code>AutoCompact</code>, which only
controls the <em>proactive</em> check before each turn.</p>
<h2 id="mcp-oauth-authorization"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#mcp-oauth-authorization"><span class="icon icon-link"></span></a>MCP OAuth Authorization</h2>
<p>When a remote MCP server (SSE or Streamable HTTP) requires OAuth, Kit runs
the full authorization flow (dynamic client registration → PKCE → user
consent → token exchange → token persistence) but delegates the <strong>user-facing
step</strong> — displaying the authorization URL and receiving the callback — to
an <code>MCPAuthHandler</code>.</p>
<p>The SDK is deliberately inert when <code>MCPAuthHandler</code> is <code>nil</code>: it does <strong>not</strong>
auto-construct a default handler, bind a local TCP port, or open a browser.
This keeps library, daemon, and web-app embedders free of surprise I/O.
Consumers opt in by passing a handler explicitly.</p>
<table>
<thead>
<tr>
<th>Building block</th>
<th>When to use</th>
</tr>
</thead>
<tbody>
<tr>
<td><code>MCPAuthHandler = nil</code> (default)</td>
<td>OAuth disabled. Remote MCP servers requiring auth fail with a clear error. Correct for libraries, daemons, and web apps.</td>
</tr>
<tr>
<td><code>kit.NewCLIMCPAuthHandler()</code></td>
<td>CLI/TUI apps. Opens the system browser, prints status to stderr (or via <code>NotifyFunc</code>), runs a localhost callback server. Used by the <code>kit</code> binary.</td>
</tr>
<tr>
<td><code>kit.NewDefaultMCPAuthHandler()</code> + <code>OnAuthURL</code></td>
<td>Custom UX. Use the SDK's port reservation and callback server; plug in your own presentation via the <code>OnAuthURL(serverName, authURL)</code> closure.</td>
</tr>
<tr>
<td>Implement <code>kit.MCPAuthHandler</code> directly</td>
<td>Full control. No localhost binding — e.g. return the URL from an HTTP endpoint and have the consumer POST the callback URL back.</td>
</tr>
</tbody>
</table>
<p><strong>CLI-style embedder:</strong></p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">authHandler, err </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> kit.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">NewCLIMCPAuthHandler</span><span style="color:#24292E;--shiki-dark:#E1E4E8">()</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">if</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> err </span><span style="color:#D73A49;--shiki-dark:#F97583">!=</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> nil</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> {</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    log.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Fatal</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(err)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">}</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">defer</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> authHandler.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Close</span><span style="color:#24292E;--shiki-dark:#E1E4E8">() </span><span style="color:#6A737D;--shiki-dark:#6A737D">// release the reserved port</span></span>
<span class="line"></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">host, _ </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> kit.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">New</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(ctx, </span><span style="color:#D73A49;--shiki-dark:#F97583">&amp;</span><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Options</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    MCPAuthHandler: authHandler,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span></code></pre>
<p><strong>Custom UX embedder (TUI modal, QR code, web redirect, etc.):</strong></p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">authHandler, _ </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> kit.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">NewDefaultMCPAuthHandler</span><span style="color:#24292E;--shiki-dark:#E1E4E8">()</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">authHandler.OnAuthURL </span><span style="color:#D73A49;--shiki-dark:#F97583">=</span><span style="color:#D73A49;--shiki-dark:#F97583"> func</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#E36209;--shiki-dark:#FFAB70">serverName</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#E36209;--shiki-dark:#FFAB70">authURL</span><span style="color:#D73A49;--shiki-dark:#F97583"> string</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) {</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // No browser or terminal assumptions — render however you like.</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    myUI.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ShowAuthPrompt</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(serverName, authURL)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">}</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">defer</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> authHandler.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Close</span><span style="color:#24292E;--shiki-dark:#E1E4E8">()</span></span>
<span class="line"></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">host, _ </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> kit.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">New</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(ctx, </span><span style="color:#D73A49;--shiki-dark:#F97583">&amp;</span><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Options</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    MCPAuthHandler: authHandler,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span></code></pre>
<p><strong>Fully custom handler (no local port binding at all):</strong></p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">type</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> WebAuthHandler</span><span style="color:#D73A49;--shiki-dark:#F97583"> struct</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> {</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    redirectURI </span><span style="color:#D73A49;--shiki-dark:#F97583">string</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    callbacks   </span><span style="color:#D73A49;--shiki-dark:#F97583">chan</span><span style="color:#D73A49;--shiki-dark:#F97583"> string</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">}</span></span>
<span class="line"></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">func</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> (</span><span style="color:#E36209;--shiki-dark:#FFAB70">h </span><span style="color:#D73A49;--shiki-dark:#F97583">*</span><span style="color:#6F42C1;--shiki-dark:#B392F0">WebAuthHandler</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) </span><span style="color:#6F42C1;--shiki-dark:#B392F0">RedirectURI</span><span style="color:#24292E;--shiki-dark:#E1E4E8">() </span><span style="color:#D73A49;--shiki-dark:#F97583">string</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> { </span><span style="color:#D73A49;--shiki-dark:#F97583">return</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> h.redirectURI }</span></span>
<span class="line"></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">func</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> (</span><span style="color:#E36209;--shiki-dark:#FFAB70">h </span><span style="color:#D73A49;--shiki-dark:#F97583">*</span><span style="color:#6F42C1;--shiki-dark:#B392F0">WebAuthHandler</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) </span><span style="color:#6F42C1;--shiki-dark:#B392F0">HandleAuth</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#E36209;--shiki-dark:#FFAB70">ctx</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> context</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Context</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#E36209;--shiki-dark:#FFAB70">serverName</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#E36209;--shiki-dark:#FFAB70">authURL</span><span style="color:#D73A49;--shiki-dark:#F97583"> string</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) (</span><span style="color:#D73A49;--shiki-dark:#F97583">string</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#D73A49;--shiki-dark:#F97583">error</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) {</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // Push the URL to the user's existing browser session via your web app,</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // then block on the callback that your HTTP handler pushes onto the channel.</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    h.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">pushToUserSession</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(serverName, authURL)</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">    select</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> {</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">    case</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> callbackURL </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#D73A49;--shiki-dark:#F97583"> &lt;-</span><span style="color:#24292E;--shiki-dark:#E1E4E8">h.callbacks:</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">        return</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> callbackURL, </span><span style="color:#005CC5;--shiki-dark:#79B8FF">nil</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">    case</span><span style="color:#D73A49;--shiki-dark:#F97583"> &lt;-</span><span style="color:#24292E;--shiki-dark:#E1E4E8">ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Done</span><span style="color:#24292E;--shiki-dark:#E1E4E8">():</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">        return</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> ""</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Err</span><span style="color:#24292E;--shiki-dark:#E1E4E8">()</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    }</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">}</span></span></code></pre>
<blockquote>
<p><strong>Warning:</strong> <code>DefaultMCPAuthHandler</code> with no <code>OnAuthURL</code> set will silently
drop the authorization URL and hang until the 2-minute callback timeout
fires. Always set <code>OnAuthURL</code>, or use a higher-level wrapper like
<code>CLIMCPAuthHandler</code>.</p>
</blockquote>
<h2 id="mcp-tasks"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#mcp-tasks"><span class="icon icon-link"></span></a>MCP Tasks</h2>
<p>The <a href="https://modelcontextprotocol.io/specification/2025-11-25/basic/utilities/tasks">MCP Tasks utility</a>
turns a synchronous <code>tools/call</code> into a pollable async job: the server
returns a <code>taskId</code> with status <code>working</code> immediately, and the client polls
<code>tasks/get</code> / <code>tasks/result</code> until the task reaches a terminal state.</p>
<p>Kit advertises task support during <code>initialize</code> and, by default, augments
<code>tools/call</code> with task metadata only when the server advertises
<code>tasks/toolCalls</code> capability — so any existing MCP server keeps its previous
synchronous behaviour bit-for-bit. Long-running tools (builds, deployments,
batch jobs, sub-agent runs) get HTTP/SSE timeout-resistance and clean
cancellation "for free" once both sides opt in.</p>
<h3 id="per-server-mode"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#per-server-mode"><span class="icon icon-link"></span></a>Per-server mode</h3>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">import</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> "</span><span style="color:#6F42C1;--shiki-dark:#B392F0">time</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"</span></span>
<span class="line"></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">host, _ </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> kit.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">New</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(ctx, </span><span style="color:#D73A49;--shiki-dark:#F97583">&amp;</span><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Options</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    MCPTaskMode: </span><span style="color:#D73A49;--shiki-dark:#F97583">map</span><span style="color:#24292E;--shiki-dark:#E1E4E8">[</span><span style="color:#D73A49;--shiki-dark:#F97583">string</span><span style="color:#24292E;--shiki-dark:#E1E4E8">]</span><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">MCPTaskMode</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span></span>
<span class="line"><span style="color:#032F62;--shiki-dark:#9ECBFF">        "build-server"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: kit.MCPTaskModeAlways, </span><span style="color:#6A737D;--shiki-dark:#6A737D">// force task-augmented calls</span></span>
<span class="line"><span style="color:#032F62;--shiki-dark:#9ECBFF">        "chat-server"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">:  kit.MCPTaskModeNever,  </span><span style="color:#6A737D;--shiki-dark:#6A737D">// force synchronous calls</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">        // any server not in the map honours its \`tasksMode\` config field</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">        // (default "auto")</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    },</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span></code></pre>
<table>
<thead>
<tr>
<th>Mode</th>
<th>Behaviour</th>
</tr>
</thead>
<tbody>
<tr>
<td><code>MCPTaskModeAuto</code> (default)</td>
<td>Augment <code>tools/call</code> with <code>TaskParams</code> only when the server advertised <code>tasks/toolCalls</code>.</td>
</tr>
<tr>
<td><code>MCPTaskModeNever</code></td>
<td>Always issue <code>tools/call</code> synchronously, ignoring server capability.</td>
</tr>
<tr>
<td><code>MCPTaskModeAlways</code></td>
<td>Always opt in, even when the server didn't advertise the capability. The server may still respond synchronously.</td>
</tr>
</tbody>
</table>
<h3 id="progress-callbacks"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#progress-callbacks"><span class="icon icon-link"></span></a>Progress callbacks</h3>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">host, _ </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> kit.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">New</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(ctx, </span><span style="color:#D73A49;--shiki-dark:#F97583">&amp;</span><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Options</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    MCPTaskTimeout:  </span><span style="color:#005CC5;--shiki-dark:#79B8FF">15</span><span style="color:#D73A49;--shiki-dark:#F97583"> *</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> time.Minute,        </span><span style="color:#6A737D;--shiki-dark:#6A737D">// total wall-clock cap</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    MCPTaskTTL:      </span><span style="color:#005CC5;--shiki-dark:#79B8FF">30</span><span style="color:#D73A49;--shiki-dark:#F97583"> *</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> time.Minute,        </span><span style="color:#6A737D;--shiki-dark:#6A737D">// server retention hint</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    MCPTaskProgress: </span><span style="color:#D73A49;--shiki-dark:#F97583">func</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#E36209;--shiki-dark:#FFAB70">p</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">MCPTaskProgress</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) {</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">        log.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Printf</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"</span><span style="color:#005CC5;--shiki-dark:#79B8FF">%s</span><span style="color:#032F62;--shiki-dark:#9ECBFF">/</span><span style="color:#005CC5;--shiki-dark:#79B8FF">%s</span><span style="color:#032F62;--shiki-dark:#9ECBFF">: </span><span style="color:#005CC5;--shiki-dark:#79B8FF">%s</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> %s</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, p.Server, p.TaskID, p.Status, p.Message)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    },</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span></code></pre>
<p>The handler fires once when a task is accepted and again on every observed
status transition. The final call always carries a terminal status
(<code>MCPTaskStatusCompleted</code>, <code>MCPTaskStatusFailed</code>, or <code>MCPTaskStatusCancelled</code>).
Do not block in the handler — dispatch long work on a goroutine.</p>
<h3 id="inspecting-and-cancelling-tasks"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#inspecting-and-cancelling-tasks"><span class="icon icon-link"></span></a>Inspecting and cancelling tasks</h3>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">tasks, _ </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ListMCPTasks</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(ctx, </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"build-server"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">for</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> _, t </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#D73A49;--shiki-dark:#F97583"> range</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> tasks {</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    fmt.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Printf</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"</span><span style="color:#005CC5;--shiki-dark:#79B8FF">%s</span><span style="color:#032F62;--shiki-dark:#9ECBFF">: </span><span style="color:#005CC5;--shiki-dark:#79B8FF">%s</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> (</span><span style="color:#005CC5;--shiki-dark:#79B8FF">%s</span><span style="color:#032F62;--shiki-dark:#9ECBFF">)</span><span style="color:#005CC5;--shiki-dark:#79B8FF">\\n</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, t.TaskID, t.Status, t.StatusMessage)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">}</span></span>
<span class="line"></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">t, _ </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">GetMCPTask</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(ctx, </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"build-server"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, taskID)</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">if</span><span style="color:#D73A49;--shiki-dark:#F97583"> !</span><span style="color:#24292E;--shiki-dark:#E1E4E8">t.Status.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">IsTerminal</span><span style="color:#24292E;--shiki-dark:#E1E4E8">() {</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    _, _ </span><span style="color:#D73A49;--shiki-dark:#F97583">=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">CancelMCPTask</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(ctx, </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"build-server"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, taskID)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">}</span></span></code></pre>
<p><code>Kit.ListMCPTasks</code>, <code>Kit.GetMCPTask</code>, and <code>Kit.CancelMCPTask</code> work against any
loaded MCP server that advertises the corresponding capability.
<code>MCPTaskStatus.IsTerminal()</code> is the canonical check for completion.</p>
<p>Context cancellation also works end-to-end: cancelling the <code>ctx</code> passed to a
tool execution triggers a best-effort <code>tasks/cancel</code> before the call returns.</p>
<h2 id="custom-debug-logger"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#custom-debug-logger"><span class="icon icon-link"></span></a>Custom debug logger</h2>
<p>Kit's engine and MCP tool plumbing emit low-level debug output through a
<code>DebugLogger</code> interface. By default, setting <code>Debug: true</code> (or calling
<code>WithDebug()</code>) installs the built-in console logger. To route the same output
into your application's logging system instead, provide a custom
implementation via <code>Options.DebugLogger</code> or <code>WithDebugLogger</code>.</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">type</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> DebugLogger</span><span style="color:#D73A49;--shiki-dark:#F97583"> interface</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> {</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">    LogDebug</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#E36209;--shiki-dark:#FFAB70">message</span><span style="color:#D73A49;--shiki-dark:#F97583"> string</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">    IsDebugEnabled</span><span style="color:#24292E;--shiki-dark:#E1E4E8">() </span><span style="color:#D73A49;--shiki-dark:#F97583">bool</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">}</span></span></code></pre>
<p>When <code>DebugLogger</code> is non-nil it takes precedence over <code>Debug</code> — the
supplied logger's <code>IsDebugEnabled()</code> reports whether downstream code should
bother formatting messages.</p>
<p><strong>Example: forward to <code>log/slog</code>:</strong></p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">import</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> "</span><span style="color:#6F42C1;--shiki-dark:#B392F0">log/slog</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"</span></span>
<span class="line"></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">type</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> slogDebugLogger</span><span style="color:#D73A49;--shiki-dark:#F97583"> struct</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{ l </span><span style="color:#D73A49;--shiki-dark:#F97583">*</span><span style="color:#6F42C1;--shiki-dark:#B392F0">slog</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Logger</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> }</span></span>
<span class="line"></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">func</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> (</span><span style="color:#E36209;--shiki-dark:#FFAB70">s </span><span style="color:#D73A49;--shiki-dark:#F97583">*</span><span style="color:#6F42C1;--shiki-dark:#B392F0">slogDebugLogger</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) </span><span style="color:#6F42C1;--shiki-dark:#B392F0">LogDebug</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#E36209;--shiki-dark:#FFAB70">m</span><span style="color:#D73A49;--shiki-dark:#F97583"> string</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)    { s.l.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Debug</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(m) }</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">func</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> (</span><span style="color:#E36209;--shiki-dark:#FFAB70">s </span><span style="color:#D73A49;--shiki-dark:#F97583">*</span><span style="color:#6F42C1;--shiki-dark:#B392F0">slogDebugLogger</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) </span><span style="color:#6F42C1;--shiki-dark:#B392F0">IsDebugEnabled</span><span style="color:#24292E;--shiki-dark:#E1E4E8">() </span><span style="color:#D73A49;--shiki-dark:#F97583">bool</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> { </span><span style="color:#D73A49;--shiki-dark:#F97583">return</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> true</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> }</span></span>
<span class="line"></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">host, _ </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> kit.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">NewAgent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(ctx,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    kit.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">WithModel</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"anthropic/claude-sonnet-4-5-20250929"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">),</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    kit.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">WithDebugLogger</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#D73A49;--shiki-dark:#F97583">&amp;</span><span style="color:#6F42C1;--shiki-dark:#B392F0">slogDebugLogger</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{l: slog.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Default</span><span style="color:#24292E;--shiki-dark:#E1E4E8">()}),</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span></code></pre>
<p>Implementations must be safe for concurrent use — messages can arrive
from the engine goroutine, MCP connection pool, and tool execution paths
simultaneously.</p>
<h2 id="precedence"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#precedence"><span class="icon icon-link"></span></a>Precedence</h2>
<p>For any given generation or provider field, the effective value is resolved
in this order (highest priority first):</p>
<ol>
<li><code>Options.X</code> (SDK caller)</li>
<li><code>KIT_X</code> environment variable</li>
<li><code>.kit.yml</code> (project-local then <code>~/.kit.yml</code>)</li>
<li>Per-model defaults (<code>modelSettings[provider/model]</code> or <code>customModels[...].params</code>)</li>
<li>Provider-level defaults (e.g. Anthropic's own temperature default)</li>
<li>SDK last-resort floor (currently: <code>MaxTokens = 8192</code>, matching the CLI <code>--max-tokens</code> default)</li>
</ol>
<p>Sampling params that remain <code>nil</code> after the SDK resolution step are left out
of the provider call entirely, so the LLM library applies its own default.</p>
<h2 id="tool-configuration"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#tool-configuration"><span class="icon icon-link"></span></a>Tool configuration</h2>
<p><strong><code>Tools</code></strong> replaces default core tools, not MCP or extension tools. <strong><code>ExtraTools</code></strong> adds tools alongside the defaults. Use <code>Tools</code> to replace core tools; use <code>ExtraTools</code> to extend them.</p>
<p>To keep the default tool set but narrow the built-in core tools, set
<strong><code>CoreToolList</code></strong> (an allow-list of names such as <code>"bash"</code>, <code>"read"</code>,
<code>"write"</code>, <code>"edit"</code>, <code>"grep"</code>, <code>"find"</code>, <code>"ls"</code>, <code>"subagent"</code>). Build the list
from include/exclude filters with
<a href="/sdk/overview#filtering-core-tools"><code>kit.FilterCoreToolNames</code></a>:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">list, _ </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> kit.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">FilterCoreToolNames</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#005CC5;--shiki-dark:#79B8FF">nil</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, []</span><span style="color:#D73A49;--shiki-dark:#F97583">string</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"bash"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"write"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">}) </span><span style="color:#6A737D;--shiki-dark:#6A737D">// drop two</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">host, _ </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> kit.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">New</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(ctx, </span><span style="color:#D73A49;--shiki-dark:#F97583">&amp;</span><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Options</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{CoreToolList: list})</span></span></code></pre>
<p><code>DisableCoreTools: true</code> is the chat-only shortcut for an empty core set.</p>
<p>Create custom tools with <code>kit.NewTool</code> — no external dependencies needed:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">type</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> LookupInput</span><span style="color:#D73A49;--shiki-dark:#F97583"> struct</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> {</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    ID </span><span style="color:#D73A49;--shiki-dark:#F97583">string</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> \`json:"id" description:"Record ID to look up"\`</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">}</span></span>
<span class="line"></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">lookupTool </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> kit.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">NewTool</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"lookup"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"Look up a record by ID"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">    func</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#E36209;--shiki-dark:#FFAB70">ctx</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> context</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Context</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#E36209;--shiki-dark:#FFAB70">input</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> LookupInput</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) (</span><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ToolOutput</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#D73A49;--shiki-dark:#F97583">error</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) {</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">        record </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> db.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Find</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(input.ID)</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">        return</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> kit.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">TextResult</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(record.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">String</span><span style="color:#24292E;--shiki-dark:#E1E4E8">()), </span><span style="color:#005CC5;--shiki-dark:#79B8FF">nil</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    },</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span>
<span class="line"></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">host, _ </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> kit.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">New</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(ctx, </span><span style="color:#D73A49;--shiki-dark:#F97583">&amp;</span><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Options</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    ExtraTools: []</span><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Tool</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{lookupTool},</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span></code></pre>
<p>See <a href="/sdk/overview#custom-tools">Overview</a> for full custom tool documentation.</p>
<h3 id="routing-file-access-and-commands"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#routing-file-access-and-commands"><span class="icon icon-link"></span></a>Routing file access and commands</h3>
<p>By default the core tools read and write the local disk and run commands as
local child processes. Two tool options change this, for example to let an
editor provide unsaved buffers and run commands in its own terminal:</p>
<ul>
<li><strong><code>kit.WithFileSystem(fs)</code></strong>: the <code>read</code>, <code>write</code> and <code>edit</code> tools read and
write text through <code>fs</code> (a <code>kit.FileSystem</code>). Images are still read from
the local disk.</li>
<li><strong><code>kit.WithCommandRunner(r)</code></strong>: the <code>shell</code> tool runs commands through <code>r</code>
(a <code>kit.CommandRunner</code>). The tool still checks the command and the timeout
first. Interactive sudo prompts are not available through a runner.</li>
</ul>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">host, _ </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> kit.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">New</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(ctx, </span><span style="color:#D73A49;--shiki-dark:#F97583">&amp;</span><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Options</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    WorkDir:         </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"/path/to/project"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    CoreToolOptions: []</span><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ToolOption</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{kit.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">WithFileSystem</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(myFS), kit.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">WithCommandRunner</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(myRunner)},</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span></code></pre>
<p><code>kit acp</code> uses these options to implement the ACP <code>fs/*</code> and <code>terminal/*</code>
client methods.</p>
<h2 id="options-and-functional-helper-inventory"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#options-and-functional-helper-inventory"><span class="icon icon-link"></span></a>Options and functional helper inventory</h2>
<p>This inventory links configuration to the <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options">authoritative Go contract</a>. Fields retain their documented zero/unset semantics. Functional options apply in order; later values win. Use <code>New</code> with <code>Options</code> for fields without a helper.</p>
<table>
<thead>
<tr>
<th>Name</th>
<th>Contract</th>
</tr>
</thead>
<tbody>
<tr>
<td><code>WithProvider</code></td>
<td>Registers a provider factory for this Kit instance only; see <code>Providers</code>. <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#WithProvider">Go reference</a></td>
</tr>
<tr>
<td><code>WithDecisionProvider</code></td>
<td>Registers a decision provider factory for this Kit instance only; see <code>DecisionProviders</code>. <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#WithDecisionProvider">Go reference</a></td>
</tr>
<tr>
<td><code>AllowMissingCredentials</code></td>
<td>AllowMissingCredentials lets New succeed when the configured provider cannot be created — no API key or OAuth token, a failed OAuth token refresh, an unsupported provider, and so on. <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options">Go reference</a></td>
</tr>
<tr>
<td><code>AlwaysOfferSkillActivation</code></td>
<td>AlwaysOfferSkillActivation keeps the built-in activate_skill tool offered with an empty catalogue. <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options">Go reference</a></td>
</tr>
<tr>
<td><code>AutoCompact</code></td>
<td>Compaction AutoCompact enables proactive compaction before turns that near the context limit. <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options">Go reference</a></td>
</tr>
<tr>
<td><code>Bare</code></td>
<td>Bare disables every form of automatic context discovery, so starting Kit inside a directory does not implicitly load that directory's instructions, tooling or configuration. <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options">Go reference</a></td>
</tr>
<tr>
<td><code>CLI</code></td>
<td>CLI is optional CLI-specific configuration. <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options">Go reference</a></td>
</tr>
<tr>
<td><code>CodeMode</code></td>
<td>CodeMode configures code mode: one tool that runs a JavaScript program which calls the other tools and returns a small result. <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options">Go reference</a></td>
</tr>
<tr>
<td><code>CompactionOptions</code></td>
<td>Config for auto-compaction (nil = defaults). <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options">Go reference</a></td>
</tr>
<tr>
<td><code>ConfigFile</code></td>
<td>Override config file path. <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options">Go reference</a></td>
</tr>
<tr>
<td><code>Continue</code></td>
<td>Continue the newest session for SessionDir. <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options">Go reference</a></td>
</tr>
<tr>
<td><code>CoreToolList</code></td>
<td>List of tools to include, when empty, include all available core tools. <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options">Go reference</a></td>
</tr>
<tr>
<td><code>CoreToolOptions</code></td>
<td>CoreToolOptions are applied to the built-in core tools after the options Kit derives from its own settings, so they win on conflict. <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options">Go reference</a></td>
</tr>
<tr>
<td><code>DebugLogger</code></td>
<td>DebugLogger, if non-nil, routes low-level debug output from the engine and the MCP tool plumbing to a caller-supplied implementation. <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options">Go reference</a></td>
</tr>
<tr>
<td><code>Debug</code></td>
<td>Debug enables debug logging for the SDK. <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options">Go reference</a></td>
</tr>
<tr>
<td><code>DisableCoreTools</code></td>
<td>DisableCoreTools, when true, prevents loading any core tools. <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options">Go reference</a></td>
</tr>
<tr>
<td><code>Ephemeral</code></td>
<td>Ephemeral configures an in-memory session with no persistence (equivalent to Options.NoSession = true). <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Ephemeral">Go reference</a></td>
</tr>
<tr>
<td><code>ExtraTools</code></td>
<td>Additional tools added alongside core/MCP/extension tools. <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options">Go reference</a></td>
</tr>
<tr>
<td><code>FrequencyPenalty</code></td>
<td>FrequencyPenalty discourages repeated tokens (OpenAI-family models). <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options">Go reference</a></td>
</tr>
<tr>
<td><code>HTTPTransport</code></td>
<td>HTTPTransport wraps each built-in provider's base HTTP transport. <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options">Go reference</a></td>
</tr>
<tr>
<td><code>ImageMaxBytes</code></td>
<td>ImageMaxBytes caps the encoded size, in bytes, of an image the read tool attaches to a message. <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options">Go reference</a></td>
</tr>
<tr>
<td><code>ImageMaxEdge</code></td>
<td>ImageMaxEdge caps the width and height, in pixels, of an image the read tool attaches to a message. <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options">Go reference</a></td>
</tr>
<tr>
<td><code>ImageNoResize</code></td>
<td>ImageNoResize makes the read tool reject an oversized image instead of scaling it down. <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options">Go reference</a></td>
</tr>
<tr>
<td><code>InProcessMCPServers</code></td>
<td>InProcessMCPServers registers mcp-go servers that run in the same process. <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options">Go reference</a></td>
</tr>
<tr>
<td><code>Isolated</code></td>
<td>Isolated turns off everything that Kit otherwise discovers or writes on the host by itself. <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Isolated">Go reference</a></td>
</tr>
<tr>
<td><code>MCPAuthHandler</code></td>
<td>MCPAuthHandler handles OAuth authorization for remote MCP servers. <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options">Go reference</a></td>
</tr>
<tr>
<td><code>MCPConfig</code></td>
<td>MCPConfig provides a pre-loaded MCP configuration. <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options">Go reference</a></td>
</tr>
<tr>
<td><code>MCPTaskMaxPollInterval</code></td>
<td>MCPTaskMaxPollInterval caps the polling interval (a server-supplied pollInterval can otherwise grow without bound). <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options">Go reference</a></td>
</tr>
<tr>
<td><code>MCPTaskMode</code></td>
<td>MCPTaskMode overrides the per-server [MCPTaskMode] for task-augmented tools/call execution. <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options">Go reference</a></td>
</tr>
<tr>
<td><code>MCPTaskPollInterval</code></td>
<td>MCPTaskPollInterval is the fallback interval between tasks/get requests when the server does not suggest one. <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options">Go reference</a></td>
</tr>
<tr>
<td><code>MCPTaskProgress</code></td>
<td>MCPTaskProgress, if non-nil, is invoked once when a task is accepted and on every status transition observed by the polling loop. <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options">Go reference</a></td>
</tr>
<tr>
<td><code>MCPTaskTTL</code></td>
<td>MCPTaskTTL is the TTL hint sent in TaskParams for every task-augmented tools/call. <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options">Go reference</a></td>
</tr>
<tr>
<td><code>MCPTaskTimeout</code></td>
<td>MCPTaskTimeout is the maximum wall-clock duration to wait for a task-augmented tool call to reach a terminal state. <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options">Go reference</a></td>
</tr>
<tr>
<td><code>MCPTokenStoreFactory</code></td>
<td>MCPTokenStoreFactory, if non-nil, is called to create a token store for each remote MCP server that requires OAuth. <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options">Go reference</a></td>
</tr>
<tr>
<td><code>MaxSteps</code></td>
<td>Override max steps (0 = use default). <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options">Go reference</a></td>
</tr>
<tr>
<td><code>MaxTokens</code></td>
<td>MaxTokens overrides the maximum output tokens per LLM response. <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options">Go reference</a></td>
</tr>
<tr>
<td><code>ModelMode</code></td>
<td>Model mode to select (empty selects the model default). <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options">Go reference</a></td>
</tr>
<tr>
<td><code>Model</code></td>
<td>Override model (e.g., "anthropic/claude-sonnet-4-5-20250929"). <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options">Go reference</a></td>
</tr>
<tr>
<td><code>DecisionModel</code></td>
<td>Default decision model in "provider/model" format, used by Kit.Decide and extensions when a request names no model. <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options">Go reference</a></td>
</tr>
<tr>
<td><code>NewAgent</code></td>
<td>NewAgent creates a Kit using an ergonomic functional-options API. <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#NewAgent">Go reference</a></td>
</tr>
<tr>
<td><code>NewIsolatedAgent</code></td>
<td>NewIsolatedAgent is [NewAgent] with [Isolated] applied before opts. <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#NewIsolatedAgent">Go reference</a></td>
</tr>
<tr>
<td><code>NoAgents</code></td>
<td>NoAgents disables discovery of named agent definitions (built-ins and .agents/agents/ / .kit/agents/ / ~/.config/kit/agents/ files). <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options">Go reference</a></td>
</tr>
<tr>
<td><code>NoContextFiles</code></td>
<td>NoContextFiles disables automatic loading of project context files (for example AGENTS.md). <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options">Go reference</a></td>
</tr>
<tr>
<td><code>NoExtensions</code></td>
<td>NoExtensions disables Yaegi extension loading entirely. <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options">Go reference</a></td>
</tr>
<tr>
<td><code>NoSession</code></td>
<td>In-memory session without file persistence. <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options">Go reference</a></td>
</tr>
<tr>
<td><code>NoSkills</code></td>
<td>Disable skill loading entirely (auto-discovery and explicit). <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options">Go reference</a></td>
</tr>
<tr>
<td><code>OnMCPServerLoaded</code></td>
<td>OnMCPServerLoaded, if non-nil, is called when each MCP server finishes loading during Kit initialization. <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options">Go reference</a></td>
</tr>
<tr>
<td><code>PresencePenalty</code></td>
<td>PresencePenalty discourages repeating topics (OpenAI-family models). <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options">Go reference</a></td>
</tr>
<tr>
<td><code>ProviderAPIKey</code></td>
<td>ProviderAPIKey overrides the API key used to authenticate with the model provider. <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options">Go reference</a></td>
</tr>
<tr>
<td><code>ProviderURL</code></td>
<td>ProviderURL overrides the provider endpoint. <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options">Go reference</a></td>
</tr>
<tr>
<td><code>ProviderWire</code></td>
<td>ProviderWire overrides the wire protocol used for auto-routed providers: "openai" (Responses API), "openai-compat" (chat completions), "anthropic", or "google". <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options">Go reference</a></td>
</tr>
<tr>
<td><code>Providers</code></td>
<td>Providers registers provider factories for this Kit instance, keyed by provider name (case-insensitive, must not contain "/"). <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options">Go reference</a></td>
</tr>
<tr>
<td><code>DecisionProviders</code></td>
<td>DecisionProviders registers decision provider factories for this Kit instance, keyed by provider name with the same rules as Providers. <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options">Go reference</a></td>
</tr>
<tr>
<td><code>Quiet</code></td>
<td>Suppress debug output. <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options">Go reference</a></td>
</tr>
<tr>
<td><code>RetryPolicy</code></td>
<td>RetryPolicy limits retries per model request. <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options">Go reference</a></td>
</tr>
<tr>
<td><code>SessionDir</code></td>
<td>Project directory identity for session discovery, not a storage root. <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options">Go reference</a></td>
</tr>
<tr>
<td><code>SessionManager</code></td>
<td>SessionManager allows custom session storage backends. <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options">Go reference</a></td>
</tr>
<tr>
<td><code>SessionPath</code></td>
<td>Open a specific session file. <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options">Go reference</a></td>
</tr>
<tr>
<td><code>ShellMaxTimeout</code></td>
<td>ShellMaxTimeout caps the maximum timeout (in seconds) a shell tool call may request via its timeout argument. <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options">Go reference</a></td>
</tr>
<tr>
<td><code>ShellTimeout</code></td>
<td>ShellTimeout sets the default per-call timeout (in seconds) for the shell tool, applied when the model does not specify one. <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options">Go reference</a></td>
</tr>
<tr>
<td><code>Shell</code></td>
<td>Shell is the shell the shell tool runs a command string through, plus its own leading arguments (for example bash or busybox ash). <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options">Go reference</a></td>
</tr>
<tr>
<td><code>SkillTrustPrompt</code></td>
<td>SkillTrustPrompt is an optional callback invoked the first time Kit auto-discovers project-local skills (under /.agents/skills or /.kit/skills) in a directory that is not yet on the trust allowlist. <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options">Go reference</a></td>
</tr>
<tr>
<td><code>SkillsDir</code></td>
<td>Scan this directory directly for skills. <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options">Go reference</a></td>
</tr>
<tr>
<td><code>SkillsDisable</code></td>
<td>SkillsDisable names skills (by Name) to exclude from the model-facing catalog. <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options">Go reference</a></td>
</tr>
<tr>
<td><code>Skills</code></td>
<td>Explicit skill files or directories; empty enables discovery. <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options">Go reference</a></td>
</tr>
<tr>
<td><code>SkipConfig</code></td>
<td>SkipConfig, when true, skips loading .kit.yml configuration files. <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options">Go reference</a></td>
</tr>
<tr>
<td><code>Streaming</code></td>
<td>Streaming enables or disables streaming output. <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options">Go reference</a></td>
</tr>
<tr>
<td><code>SystemPrompt</code></td>
<td>Override system prompt. <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options">Go reference</a></td>
</tr>
<tr>
<td><code>TLSSkipVerify</code></td>
<td>TLSSkipVerify disables TLS certificate verification on provider HTTP clients. <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options">Go reference</a></td>
</tr>
<tr>
<td><code>Temperature</code></td>
<td>Temperature controls sampling randomness (typically 0.0–2.0). <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options">Go reference</a></td>
</tr>
<tr>
<td><code>ThinkingLevel</code></td>
<td>ThinkingLevel sets the reasoning effort for models that support extended thinking. <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options">Go reference</a></td>
</tr>
<tr>
<td><code>Tools</code></td>
<td>Custom tool set. <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options">Go reference</a></td>
</tr>
<tr>
<td><code>TopK</code></td>
<td>TopK limits sampling to the top K tokens. <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options">Go reference</a></td>
</tr>
<tr>
<td><code>TopP</code></td>
<td>TopP is the nucleus-sampling cutoff (0.0–1.0). <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options">Go reference</a></td>
</tr>
<tr>
<td><code>UserAgent</code></td>
<td>UserAgent identifies Kit-controlled HTTP requests. <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options">Go reference</a></td>
</tr>
<tr>
<td><code>WithAgents</code></td>
<td>WithAgents enables discovery of named agent definitions again. <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#WithAgents">Go reference</a></td>
</tr>
<tr>
<td><code>WithAllowMissingCredentials</code></td>
<td>WithAllowMissingCredentials lets New succeed when the configured provider cannot be created (no credentials, a failed OAuth token refresh, an unsupported provider, and so on). <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#WithAllowMissingCredentials">Go reference</a></td>
</tr>
<tr>
<td><code>WithConfigFile</code></td>
<td>WithConfigFile sets an explicit config file path, overriding the default .kit.yml search. <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#WithConfigFile">Go reference</a></td>
</tr>
<tr>
<td><code>WithConfig</code></td>
<td>WithConfig enables .kit.yml discovery again (the home and project config files). <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#WithConfig">Go reference</a></td>
</tr>
<tr>
<td><code>WithContextFiles</code></td>
<td>WithContextFiles enables loading of project context files (AGENTS.md) from the working directory again. <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#WithContextFiles">Go reference</a></td>
</tr>
<tr>
<td><code>WithCoreTools</code></td>
<td>WithCoreTools enables the core tools again. <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#WithCoreTools">Go reference</a></td>
</tr>
<tr>
<td><code>WithDebugLogger</code></td>
<td>WithDebugLogger installs a caller-supplied [DebugLogger] for low-level engine and MCP tool plumbing output. <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#WithDebugLogger">Go reference</a></td>
</tr>
<tr>
<td><code>WithDebug</code></td>
<td>WithDebug enables SDK debug logging. <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#WithDebug">Go reference</a></td>
</tr>
<tr>
<td><code>WithExtensions</code></td>
<td>WithExtensions enables loading of extensions again. <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#WithExtensions">Go reference</a></td>
</tr>
<tr>
<td><code>WithExtraTools</code></td>
<td>WithExtraTools adds tools alongside the core/MCP/extension tools rather than replacing them. <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#WithExtraTools">Go reference</a></td>
</tr>
<tr>
<td><code>WithHTTPTransport</code></td>
<td>WithHTTPTransport wraps the base HTTP transport of built-in providers. <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#WithHTTPTransport">Go reference</a></td>
</tr>
<tr>
<td><code>WithMaxTokens</code></td>
<td>WithMaxTokens sets the maximum output tokens per LLM response. <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#WithMaxTokens">Go reference</a></td>
</tr>
<tr>
<td><code>WithModelMode</code></td>
<td>WithModelMode selects a named mode for the selected model. <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#WithModelMode">Go reference</a></td>
</tr>
<tr>
<td><code>WithModel</code></td>
<td>WithModel sets the model in "provider/model" format (for example openai/gpt-4.1). <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#WithModel">Go reference</a></td>
</tr>
<tr>
<td><code>WithDecisionModel</code></td>
<td>WithDecisionModel sets the default decision model in "provider/model" format (e.g. openrouter/typesafe/jev-1.14). <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#WithDecisionModel">Go reference</a></td>
</tr>
<tr>
<td><code>WithProviderAPIKey</code></td>
<td>WithProviderAPIKey overrides the API key used to authenticate with the model provider. <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#WithProviderAPIKey">Go reference</a></td>
</tr>
<tr>
<td><code>WithProviderURL</code></td>
<td>WithProviderURL overrides the provider endpoint URL. <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#WithProviderURL">Go reference</a></td>
</tr>
<tr>
<td><code>WithProviderWire</code></td>
<td>WithProviderWire overrides the wire protocol used for auto-routed providers: "openai" (Responses API), "openai-compat" (chat completions), "anthropic", or "google". <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#WithProviderWire">Go reference</a></td>
</tr>
<tr>
<td><code>WithRetryPolicy</code></td>
<td>WithRetryPolicy sets the retry limits for model requests. <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#WithRetryPolicy">Go reference</a></td>
</tr>
<tr>
<td><code>WithSessions</code></td>
<td>WithSessions enables persistent sessions again. <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#WithSessions">Go reference</a></td>
</tr>
<tr>
<td><code>WithSkillActivation</code></td>
<td>WithSkillActivation keeps activate_skill available even before any skills are registered. <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#WithSkillActivation">Go reference</a></td>
</tr>
<tr>
<td><code>WithSkills</code></td>
<td>WithSkills enables skills again. <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#WithSkills">Go reference</a></td>
</tr>
<tr>
<td><code>WithStreaming</code></td>
<td>WithStreaming enables or disables streaming responses. <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#WithStreaming">Go reference</a></td>
</tr>
<tr>
<td><code>WithSystemPrompt</code></td>
<td>WithSystemPrompt sets the system prompt. <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#WithSystemPrompt">Go reference</a></td>
</tr>
<tr>
<td><code>WithThinkingLevel</code></td>
<td>WithThinkingLevel sets the reasoning effort for models that support extended thinking. <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#WithThinkingLevel">Go reference</a></td>
</tr>
<tr>
<td><code>WithTools</code></td>
<td>WithTools sets the agent's tool set, replacing the default core tools. <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#WithTools">Go reference</a></td>
</tr>
<tr>
<td><code>WithUserAgent</code></td>
<td>WithUserAgent sets the per-instance HTTP identity. <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#WithUserAgent">Go reference</a></td>
</tr>
<tr>
<td><code>WorkDir</code></td>
<td>WorkDir is the directory the built-in file and shell tools resolve relative paths against and run commands in. <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options">Go reference</a></td>
</tr>
</tbody>
</table>
<h2 id="model-modes"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#model-modes"><span class="icon icon-link"></span></a>Model modes</h2>
<p><code>ModelMode</code> describes catalog modes, including <code>Experimental</code>, <code>Supported</code>, <code>UnsupportedReason</code>, and estimated input/output USD per million tokens. <code>host.ModelModes()</code> includes unavailable modes; <code>host.SupportedModelModes()</code> filters them. Query these methods rather than assuming every model offers a fixed mode.</p>
<p>Select with <code>Options.ModelMode</code> / <code>WithModelMode</code>, or <code>host.SetModelMode(ctx, name)</code>. Check errors with <code>errors.Is(err, kit.ErrUnsupportedModelMode)</code> for unsupported modes. <code>CurrentModelMode</code> reports the selection; an empty string selects the model default. Changes during an active turn fail, including reset. Finish or cancel the turn first.</p>
<h2 id="code-mode"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#code-mode"><span class="icon icon-link"></span></a>Code mode</h2>
<p><code>Options.CodeMode</code> points to <code>CodeModeOptions</code>. Code mode is default-on, but default enablement respects the resolved tool list and disabled core tools. Explicit <code>Enabled: true</code> can offer code mode with MCP-only tools; naming <code>codemode</code> in the core tool list selects it. <code>Disabled: true</code> takes precedence over enablement and config. For a custom <code>Tools</code> or <code>ExtraTools</code> set, create the tool with <code>NewCodeModeTool</code>, check its error, and add the returned tool. Kit supplies the current tool set with hooks applied, so nested calls still pass through hooks.</p>
<p>Zero option fields fall back to the <code>codemode</code> config section, then defaults:</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Default and unit</th>
</tr>
</thead>
<tbody>
<tr>
<td><code>Timeout</code></td>
<td>30 seconds (<code>time.Duration</code> in Go, seconds in YAML)</td>
</tr>
<tr>
<td><code>MaxToolCalls</code></td>
<td>50 tool calls per script</td>
</tr>
<tr>
<td><code>MaxOutputBytes</code></td>
<td>100000 bytes; larger captured output may be saved to a temporary file, but combined script output and logs are capped at 4 MiB and excess is truncated</td>
</tr>
<tr>
<td><code>MaxConcurrency</code></td>
<td>8 parallel tool calls per script</td>
</tr>
<tr>
<td><code>MemoryLimitMB</code></td>
<td>256 MiB approximate heap growth (value shifted by 20 bits); negative disables check</td>
</tr>
<tr>
<td><code>CatalogBudget</code></td>
<td>12000 bytes in the tool catalog description</td>
</tr>
<tr>
<td><code>MCPExposure</code></td>
<td><code>CodeModeExposureDirect</code> for unmatched MCP tools</td>
</tr>
<tr>
<td><code>Exposure</code></td>
<td>Tool-name glob rules; exact names win over globs, then longer patterns win</td>
</tr>
</tbody>
</table>
<p>Exposure constants and their config values:</p>
<ul>
<li><code>CodeModeExposureDirect</code> (<code>direct</code>): model sees the tool; scripts can call it.</li>
<li><code>CodeModeExposureScriptOnly</code> (<code>codemode</code>): scripts only; full signature in catalog.</li>
<li><code>CodeModeExposureDeferred</code> (<code>deferred</code>): scripts only; discover with <code>searchTools()</code>.</li>
<li><code>CodeModeExposureModelOnly</code> (<code>model-only</code>): model only; scripts cannot call it.</li>
</ul>
<p><code>CodeModeToolName</code> is <code>codemode</code>. See <a href="https://go-kit.dev/advanced/code-mode">code mode</a> for JavaScript APIs. JavaScript limits and discovery/isolation switches are not OS sandboxing. Tool access has the host process permissions; restrict the runtime when inputs are untrusted.</p>
<h2 id="managed-skill-activation"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#managed-skill-activation"><span class="icon icon-link"></span></a>Managed skill activation</h2>
<p><code>AlwaysOfferSkillActivation</code> / <code>WithSkillActivation</code> keeps <code>activate_skill</code> offered even with an empty catalog. It does not enable discovery or override <code>NoSkills</code>. Combine with <code>Isolated</code> and register application-owned skills. Use <code>Skill.NoHostIO</code> for content that must not be reopened from host paths. Caller-owned activation tools and explicit preparation tool overrides retain precedence. See the <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#WithSkillActivation">Go contract</a> for per-step snapshot and cancellation rules.</p>`,headings:[{depth:2,text:`HTTP request identity`,id:`http-request-identity`},{depth:2,text:`Full options reference`,id:`full-options-reference`},{depth:2,text:`Options fields`,id:`options-fields`},{depth:3,text:`Core`,id:`core`},{depth:3,text:`Generation parameters`,id:`generation-parameters`},{depth:3,text:`Provider configuration`,id:`provider-configuration`},{depth:3,text:`Session`,id:`session`},{depth:3,text:`Tools &amp; extensions`,id:`tools--extensions`},{depth:3,text:`Skills &amp; configuration`,id:`skills--configuration`},{depth:4,text:`Project-skill trust gate`,id:`project-skill-trust-gate`},{depth:3,text:`Compaction &amp; MCP`,id:`compaction--mcp`},{depth:3,text:`CompactionOptions`,id:`compactionoptions`},{depth:4,text:`Reactive compaction on context overflow`,id:`reactive-compaction-on-context-overflow`},{depth:2,text:`MCP OAuth Authorization`,id:`mcp-oauth-authorization`},{depth:2,text:`MCP Tasks`,id:`mcp-tasks`},{depth:3,text:`Per-server mode`,id:`per-server-mode`},{depth:3,text:`Progress callbacks`,id:`progress-callbacks`},{depth:3,text:`Inspecting and cancelling tasks`,id:`inspecting-and-cancelling-tasks`},{depth:2,text:`Custom debug logger`,id:`custom-debug-logger`},{depth:2,text:`Precedence`,id:`precedence`},{depth:2,text:`Tool configuration`,id:`tool-configuration`},{depth:3,text:`Routing file access and commands`,id:`routing-file-access-and-commands`},{depth:2,text:`Options and functional helper inventory`,id:`options-and-functional-helper-inventory`},{depth:2,text:`Model modes`,id:`model-modes`},{depth:2,text:`Code mode`,id:`code-mode`},{depth:2,text:`Managed skill activation`,id:`managed-skill-activation`}],raw:'\n# SDK Options\n\nPass an `Options` struct to `kit.New()` to configure the Kit instance.\n\n> **Tip:** for simple setups, `kit.NewAgent(ctx, ...Option)` provides\n> functional-options helpers (`WithModel`, `WithStreaming`, `Ephemeral`, ...)\n> over the same `Options` struct. See\n> [Functional options](/sdk/overview#functional-options-newagent).\n>\n> To embed Kit with no config, context files, skills, extensions, named\n> agents, session files or core tools from the host, use\n> `kit.NewIsolatedAgent`. It sets the fields below for you. See\n> [Isolated agents](/sdk/overview#isolated-agents).\n\nEach `kit.New` / `kit.NewAgent` call owns an isolated configuration store, so\nthese options never leak between Kit instances in the same process. See\n[Per-instance config isolation](/sdk/overview#per-instance-config-isolation).\n\n\n## HTTP request identity\n\nKit-controlled HTTP requests use `User-Agent: KIT` by default. Set a per-instance\nidentity with either API:\n\n```go\nhost, err := kit.New(ctx, &kit.Options{UserAgent: "MyApp/1.0"})\n// Or use functional options:\nhost, err = kit.NewAgent(ctx, kit.WithUserAgent("MyApp/1.0"))\n```\n\nEmpty means `KIT`, not an opt-out. Valid values are preserved without trimming,\nincluding spaces. HTTP control characters (including CR, LF, NUL and DEL) are\nrejected before construction or login does network work. This setting has no\nCLI flag, environment variable, or config-file key.\n\nEach instance keeps its own identity. It passes to model changes, temporary\ncompletions and subagents, including isolated/bare children and provider changes.\nIt covers built-in model requests (including streams), Ollama setup, remote MCP\nand their OAuth requests, and provider token refresh. The identity replaces\nconflicting user-agent headers from model defaults, catalog headers, per-call\nheaders and MCP configuration, case-insensitively. Other headers stay unchanged.\nCustom provider factories receive `ProviderConfig.UserAgent` and must enforce it\non their own requests; Kit does not intercept arbitrary factory traffic.\n\nTwo inference paths keep their fixed compatibility identities, even with an\nSDK override. Their token endpoints still use the configured Kit identity.\nThese exceptions follow existing source requirements; they were not live-tested.\n\n| Inference path | Fixed user agent |\n| --- | --- |\n| Codex | `Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36` |\n| Copilot | `GitHubCopilotChat/0.31.0` |\n\nStandalone `kit.LoginProvider` has a separate `kit.LoginOptions.UserAgent` field\n(default `KIT`), independent of any Kit instance or supplied credential manager.\nStandalone CLI OAuth and model-catalog downloads use `KIT`.\n\nThis policy does not change global HTTP defaults. It does not cover subprocesses\n(shell, git/gh/curl, installers, npm or MCP server processes), extension/application\ncode, browser navigation, or remote services acting for Kit. Third-party cloud\ncredential discovery (metadata services or default credential chains) inherits\nthe identity only when it uses the HTTP client Kit supplies; model endpoint\nrequests use the configured identity. A custom transport can rewrite headers\nafter Kit\'s boundary and is responsible for its own policy.\n\n## Full options reference\n\n```go\nhost, err := kit.New(ctx, &kit.Options{\n    // Model\n    Model:        "ollama/llama3",\n    SystemPrompt: "You are a helpful bot",\n    ConfigFile:   "/path/to/config.yml",\n\n    // Behavior\n    MaxSteps:     10,\n    Streaming:    ptrBool(true), // *bool: nil = unset (default true), &false = off\n    Quiet:        true,\n    Debug:        true,\n    DebugLogger:  myLogger,       // optional; overrides Debug + built-in logger when non-nil\n\n    // Generation parameters (override env/config/per-model defaults)\n    MaxTokens:        16384,              // 0 = auto-resolve; non-zero suppresses right-sizing\n    ThinkingLevel:    "medium",           // model-specific; see SupportedThinkingLevels\n    Temperature:      ptrFloat32(0.2),    // pointer so explicit 0.0 != unset\n    TopP:             nil,                 // nil = provider/per-model default\n    TopK:             nil,\n    FrequencyPenalty: nil,\n    PresencePenalty:  nil,\n\n    // Provider configuration\n    ProviderAPIKey: "sk-...",                      // "" = use config / provider env var\n    ProviderURL:    "https://proxy.internal/v1",  // "" = provider default endpoint\n    ProviderWire:   "anthropic",                  // "" = infer wire from model database\n    TLSSkipVerify:  false,                         // only effective when true\n    Providers: map[string]kit.ProviderFactory{    // app-supplied backends for "local/<model>"\n        "local": localFactory,\n    },\n\n    // Session\n    SessionPath:  "./session.jsonl",\n    SessionDir:   "/home/user/project", // project identity for discovery, not storage root\n    Continue:     true,\n    NoSession:    true,\n\n    // Tools\n    Tools:            []kit.Tool{...},     // Replace default core tools; MCP and extension tools remain\n    ExtraTools:       []kit.Tool{...},     // Add tools alongside defaults (mutate at runtime via host.AddTools/RemoveTools)\n    DisableCoreTools: true,                // Use no core tools (0 tools, for chat-only)\n    CoreToolList:     []string,            // List of core tools to include, if empty (default) include all\n    WorkDir:          "/path/to/project",  // Base directory of the file and shell tools (default: process cwd)\n    CoreToolOptions:  []kit.ToolOption{...}, // Extra options for the core tools, e.g. kit.WithFileSystem(fs)\n\n    // Configuration\n    SkipConfig:   true,                   // Skip .kit.yml files (viper defaults + env vars still apply)\n\n    // Compaction\n    AutoCompact:  true,\n\n    // Skills\n    Skills:        []string{"/path/to/skill.md"},\n    SkillsDir:     "/path/to/skills/",\n    SkillsDisable: []string{"noisy-skill"},\n    NoSkills:      true,\n\n    // Feature toggles\n    NoExtensions:   true,               // disable Yaegi extension loading\n    NoContextFiles: true,               // disable automatic AGENTS.md loading\n    NoAgents:       true,               // disable named agent discovery (.agents/agents/, .kit/agents/, ~/.config/kit/agents/, and built-ins)\n\n    // Session (advanced)\n    SessionManager: myCustomSession,    // custom SessionManager implementation\n\n    // MCP OAuth — both opt-in. Leave MCPAuthHandler nil to disable\n    // OAuth entirely (remote MCP 401s bubble up as errors). CLI apps\n    // pass kit.NewCLIMCPAuthHandler(); custom UX embedders implement\n    // MCPAuthHandler or configure DefaultMCPAuthHandler + OnAuthURL.\n    MCPAuthHandler: authHandler,                  // nil = OAuth disabled\n    MCPTokenStoreFactory: func(serverURL string) (kit.MCPTokenStore, error) {\n        return myStore(serverURL), nil\n    },\n\n    // In-Process MCP Servers\n    InProcessMCPServers: map[string]*kit.MCPServer{\n        "docs": mcpSrv,  // *server.MCPServer from mcp-go\n    },\n})\n```\n\n## Options fields\n\n### Core\n\n| Field | Type | Default | Description |\n|-------|------|---------|-------------|\n| `Model` | `string` | config default | Model string (provider/model format) |\n| `DecisionModel` | `string` | config default | Default [decision model](/sdk/decisions) for `Kit.Decide` and extension `ctx.Decide` when a request names no model |\n| `SystemPrompt` | `string` | — | System prompt text or file path |\n| `ConfigFile` | `string` | empty | Explicit path; when unset, discover project config before home config |\n| `MaxSteps` | `int` | `0` | Max agent steps (0 = use the configured default) |\n| `Streaming` | `*bool` | `nil` | Enable streaming output. `nil` leaves it to the precedence chain (env → config → default `true`); `&true`/`&false` forces it. Pointer so unset is distinct from explicit `false`. |\n| `Quiet` | `bool` | `false` | Suppress output |\n| `Debug` | `bool` | `false` | Enable debug logging via the built-in console / buffered logger. Ignored when `DebugLogger` is non-nil. |\n| `DebugLogger` | `DebugLogger` | `nil` | Caller-supplied logger that receives low-level engine + MCP tool plumbing debug output. When non-nil this overrides `Debug` — the supplied logger\'s `IsDebugEnabled()` controls downstream emission. See [Custom debug logger](#custom-debug-logger). |\n\n### Generation parameters\n\nThese fields override the corresponding values from `.kit.yml` / `KIT_*`\nenvironment variables. Leaving a field at its zero/nil value lets the\nprecedence chain resolve a value (`KIT_*` env → config file → per-model\ndefaults from `modelSettings`/`customModels` → an 8192 SDK floor for\n`MaxTokens` (matching the CLI `--max-tokens` default) and provider-level\ndefaults for samplers).\n\n| Field | Type | Default | Description |\n|-------|------|---------|-------------|\n| `MaxTokens` | `int` | auto-resolved | Max output tokens per response. `0` = auto-resolve; non-zero suppresses automatic right-sizing (same semantics as `--max-tokens`). |\n| `ThinkingLevel` | `string` | auto-resolved | Case-sensitive, model-specific effort. `off` omits the field; `none` is literal. `""` falls through to config/env/per-model/`"off"`. Explicit unsupported selections return an error; use [`SupportedThinkingLevels`](/sdk/overview#reasoning-support-per-model) for catalog choices (which may include future names). |\n| `Temperature` | `*float32` | — | Sampling randomness. Pointer type so explicit `0.0` is distinguishable from "unset". |\n| `TopP` | `*float32` | — | Nucleus sampling cutoff. `nil` leaves provider/per-model default. |\n| `TopK` | `*int32` | — | Top-K sampling limit. `nil` leaves provider/per-model default. |\n| `FrequencyPenalty` | `*float32` | — | OpenAI-family frequency penalty. `nil` leaves provider default. |\n| `PresencePenalty` | `*float32` | — | OpenAI-family presence penalty. `nil` leaves provider default. |\n\nPointer-typed fields (`Streaming` and the samplers) are populated via tiny helpers:\n\n```go\nfunc ptrBool(v bool) *bool          { return &v }\nfunc ptrFloat32(v float32) *float32 { return &v }\n```\n\nThese fields eliminate the need for `viper.Set()` calls before `kit.New()`\nwhen embedding Kit as a library.\n\n### Provider configuration\n\n| Field | Type | Default | Description |\n|-------|------|---------|-------------|\n| `ProviderAPIKey` | `string` | — | API key used to authenticate with the provider. `""` falls back to config / provider-specific env var (e.g. `ANTHROPIC_API_KEY`). When set, it takes precedence over config and env values on this instance\'s store. |\n| `ProviderURL` | `string` | — | Override the provider endpoint (e.g. LiteLLM, vLLM, Azure OpenAI, internal proxy). `""` = provider default. |\n| `ProviderWire` | `string` | — | Override the wire protocol for auto-routed providers: `openai` (Responses API), `openai-compat` (chat completions), `anthropic`, or `google`. `""` = infer from the model database. Takes precedence over per-provider `wire` declarations in the [`providers` config section](/configuration#provider-overrides). Combine with `ProviderURL` to target providers not in the database. |\n| `TLSSkipVerify` | `bool` | `false` | Disable TLS certificate verification on the provider HTTP client. Only effective when `true`; to force-disable, use config file or env var instead. For self-signed dev certs only. |\n| `Providers` | `map[string]ProviderFactory` | — | Provider factories for this instance, keyed by provider name (case-insensitive, no `/`). A model string `name/<model>` is built by the factory. Wins over `kit.RegisterProvider` and the built-in providers; subagents inherit it. `kit.New` copies the map and rejects nil factories and names that differ only in case. See [Custom provider backends](/sdk/overview#custom-provider-backends). |\n| `DecisionProviders` | `map[string]DecisionProviderFactory` | — | Decision provider factories for this instance, with the same name rules as `Providers`. `Kit.Decide` (and extension `ctx.Decide`) sends requests for `name/<model>` to the factory model instead of the model database. Wins over `kit.RegisterDecisionProvider`; subagents inherit it. See [custom decision providers](/sdk/decisions#custom-decision-providers). |\n\n### Session\n\n| Field | Type | Default | Description |\n|-------|------|---------|-------------|\n| `SessionPath` | `string` | — | Open a specific session file |\n| `SessionDir` | `string` | — | Base directory for session discovery |\n| `Continue` | `bool` | `false` | Resume most recent session |\n| `NoSession` | `bool` | `false` | Ephemeral mode (no persistence) |\n| `SessionManager` | `SessionManager` | — | Custom session backend (advanced) |\n\n### Tools & extensions\n\n| Field | Type | Default | Description |\n|-------|------|---------|-------------|\n| `Tools` | `[]Tool` | — | Replace default core tools; MCP and extension tools remain |\n| `ExtraTools` | `[]Tool` | — | Additional tools alongside core/MCP/extension tools |\n| `DisableCoreTools` | `bool` | `false` | Use no core tools (0 tools, for chat-only) |\n| `CoreToolList` | `[]string` | — | Allow-list of core tool names; empty/nil means all. Build with [`FilterCoreToolNames`](/sdk/overview#filtering-core-tools) from include/exclude filters. |\n| `WorkDir` | `string` | process cwd | Directory the core file and shell tools resolve relative paths against and run commands in. Subagents inherit it. |\n| `CoreToolOptions` | `[]ToolOption` | — | Extra options for the core tools, applied last. Use [`WithFileSystem`](#routing-file-access-and-commands) and [`WithCommandRunner`](#routing-file-access-and-commands) here. Subagents inherit them. |\n| `NoExtensions` | `bool` | `false` | Disable Yaegi extension loading |\n| `NoContextFiles` | `bool` | `false` | Disable automatic AGENTS.md loading |\n| `NoAgents` | `bool` | `false` | Disable named agent discovery (built-ins and `.agents/agents/` / `.kit/agents/` / `~/.config/kit/agents/` files); see [Subagents](/advanced/subagents#named-agents) |\n\nUnless `NoExtensions` is set, an embedded Kit uses the same\n[auto-discovery paths](/extensions/loading#auto-discovery) as the CLI, which\ninclude the system-wide `/usr/share/kit/extensions` directory. Set\n`NoExtensions: true` for a hermetic embed that must not pick up extensions\nfrom the host machine. [`kit.Isolated()`](/sdk/overview#isolated-agents)\nsets it together with the other discovery switches.\n\n### Skills & configuration\n\n| Field | Type | Default | Description |\n|-------|------|---------|-------------|\n| `SkipConfig` | `bool` | `false` | Skip `.kit.yml` file loading (viper defaults + env vars still apply) |\n| `Skills` | `[]string` | — | Explicit skill files/dirs to load |\n| `SkillsDir` | `string` | — | Scan this directory directly for skills (overrides auto-discovery; scanned as-is) |\n| `SkillsDisable` | `[]string` | — | Skill names to hide from the model catalog (still usable via `/<name>`) |\n| `SkillTrustPrompt` | `func(projectDir string, skillCount int) TrustDecision` | `nil` | Callback gating project-local skill loading on a trust decision (see below) |\n| `NoSkills` | `bool` | `false` | Disable skill loading entirely |\n\n#### Project-skill trust gate\n\nProject-local skills (under `<project>/.agents/skills/` or `<project>/.kit/skills/`)\nare injected into the system prompt, so loading them from an untrusted, freshly\ncloned repository is a prompt-injection vector. Set `SkillTrustPrompt` to gate\nthat first load on an explicit decision. When `nil` (the default), project\nskills load without prompting — preserving historical behaviour.\n\n```go\nopts := &kit.Options{\n    SkillTrustPrompt: func(projectDir string, skillCount int) kit.TrustDecision {\n        // Consult your own UI / policy here.\n        if userApproves(projectDir, skillCount) {\n            return kit.TrustProject     // load and persist the directory as trusted\n        }\n        return kit.SkipProjectSkills    // do not load project skills\n    },\n}\n```\n\nThe callback returns one of three `TrustDecision` values:\n\n| Decision | Effect |\n|----------|--------|\n| `kit.TrustProject` | Load project skills and persist `projectDir` to `~/.config/kit/trusted-projects.json` (not prompted again) |\n| `kit.TrustProjectOnce` | Load project skills for this run only, without persisting |\n| `kit.SkipProjectSkills` | Do not load project skills |\n\nA directory already on the persisted allowlist is trusted without invoking the\ncallback. The Kit CLI wires this to an interactive terminal prompt automatically\nfor TTY sessions.\n\nThese fields only control the **initial** skill and context-file set picked\nup by `New()`. To add, remove, or replace skills and `AGENTS.md`-style\ncontext files at runtime (e.g. per user or per session), use the\n`AddSkill` / `LoadAndAddSkill` / `RemoveSkill` / `SetSkills` /\n`DisableSkill` / `EnableSkill` and\n`AddContextFile` / `AddContextFileContent` / `RemoveContextFile` /\n`SetContextFiles` methods on `*kit.Kit`. See\n[Runtime skills and context files](/sdk/overview#runtime-skills-and-context-files).\n\n### Compaction & MCP\n\n| Field | Type | Default | Description |\n|-------|------|---------|-------------|\n| `AutoCompact` | `bool` | `false` | Compact proactively before turns that near the context limit. Independent of this setting, Kit always compacts **reactively** and replays the turn once when a provider call fails with a context-overflow error. |\n| `CompactionOptions` | `*CompactionOptions` | — | Configuration for compaction; zero-value budget fields adapt to the model\'s limits. See [CompactionOptions](#compactionoptions) below. |\n| `MCPConfig` | `*Config` | — | Pre-loaded MCP configuration. When set, `LoadAndValidateConfig` is skipped during Kit creation — useful for in-process subagents (inheriting a parent\'s loaded config) and programmatic setups that build config without reading `.kit.yml`. |\n| `MCPAuthHandler` | `MCPAuthHandler` | — | OAuth handler for remote MCP servers. `nil` disables OAuth (servers returning 401 fail with the authorization-required error). See [MCP OAuth](#mcp-oauth-authorization) below. |\n| `MCPTokenStoreFactory` | `func` | — | Custom OAuth token storage for MCP servers (default: JSON file in `$XDG_CONFIG_HOME/.kit/mcp_tokens.json`). |\n| `InProcessMCPServers` | `map[string]*MCPServer` | — | In-process mcp-go servers (no subprocess) |\n| `MCPTaskMode` | `map[string]MCPTaskMode` | — | Per-server override for task-augmented `tools/call`. Keys are server names; missing entries fall back to the `tasksMode` field of the matching `MCPServerConfig`. See [MCP Tasks](#mcp-tasks). |\n| `MCPTaskTimeout` | `time.Duration` | `15m` | Maximum wall-clock to wait for a task to reach a terminal state. Independent of any per-call context deadline. |\n| `MCPTaskTTL` | `time.Duration` | — | TTL hint sent in `TaskParams` for every task-augmented call. Zero omits the field and lets the server pick. |\n| `MCPTaskPollInterval` | `time.Duration` | `1s` | Fallback interval between `tasks/get` requests when the server does not suggest one. |\n| `MCPTaskMaxPollInterval` | `time.Duration` | `5s` | Cap on the polling interval (a server-supplied `pollInterval` can otherwise grow without bound). |\n| `MCPTaskProgress` | `MCPTaskProgressHandler` | — | Optional callback invoked once when a task is accepted and on every observed status transition. The final invocation always carries a terminal status. |\n\n### CompactionOptions\n\n`CompactionOptions` controls how conversations are summarized. The token\nbudgets are adaptive: leave a field at zero and it scales to the model\'s\nlimits, or set an explicit value to override.\n\n| Field | Type | Zero-value behaviour | Description |\n|-------|------|----------------------|-------------|\n| `ContextWindow` | `int` | auto-populated from the model registry | Model\'s context window size in tokens |\n| `MaxOutputTokens` | `int` | auto-populated from the model registry | Model\'s max output tokens; scales the adaptive `ReserveTokens` |\n| `ReserveTokens` | `int` | `min(16384, MaxOutputTokens)` | Tokens reserved for the LLM response |\n| `KeepRecentTokens` | `int` | a quarter of the usable context (`ContextWindow − ReserveTokens`), floored at 2000 | Recent tokens preserved verbatim (not summarized) |\n| `SummaryPrompt` | `string` | built-in structured checkpoint prompt | Custom summarization prompt |\n\nWhen a session compacts more than once, the previous summary is fed back into\nthe summarization prompt and updated incrementally (anchored summaries), so\ndetail is preserved across compaction generations.\n\n#### Reactive compaction on context overflow\n\nToken estimates inevitably drift from real tokenizer counts, and a single\nhuge mid-turn tool result can overflow the context even when the turn started\nwell under the limit. When the provider rejects a request with a\ncontext-overflow error, Kit automatically:\n\n1. Compacts the conversation (persisting any completed steps first, so the\n   replay resumes rather than restarts).\n2. Replays the turn once with the rebuilt context, replacing media\n   attachments with text placeholders (media cannot be shrunk by\n   summarization).\n3. If the replay still overflows, fails the turn with `kit.ErrContextOverflow`\n   ("conversation too large to compact").\n\nThis safety net is always on — it does not require `AutoCompact`, which only\ncontrols the *proactive* check before each turn.\n\n## MCP OAuth Authorization\n\nWhen a remote MCP server (SSE or Streamable HTTP) requires OAuth, Kit runs\nthe full authorization flow (dynamic client registration → PKCE → user\nconsent → token exchange → token persistence) but delegates the **user-facing\nstep** — displaying the authorization URL and receiving the callback — to\nan `MCPAuthHandler`.\n\nThe SDK is deliberately inert when `MCPAuthHandler` is `nil`: it does **not**\nauto-construct a default handler, bind a local TCP port, or open a browser.\nThis keeps library, daemon, and web-app embedders free of surprise I/O.\nConsumers opt in by passing a handler explicitly.\n\n| Building block | When to use |\n|---|---|\n| `MCPAuthHandler = nil` (default) | OAuth disabled. Remote MCP servers requiring auth fail with a clear error. Correct for libraries, daemons, and web apps. |\n| `kit.NewCLIMCPAuthHandler()` | CLI/TUI apps. Opens the system browser, prints status to stderr (or via `NotifyFunc`), runs a localhost callback server. Used by the `kit` binary. |\n| `kit.NewDefaultMCPAuthHandler()` + `OnAuthURL` | Custom UX. Use the SDK\'s port reservation and callback server; plug in your own presentation via the `OnAuthURL(serverName, authURL)` closure. |\n| Implement `kit.MCPAuthHandler` directly | Full control. No localhost binding — e.g. return the URL from an HTTP endpoint and have the consumer POST the callback URL back. |\n\n**CLI-style embedder:**\n\n```go\nauthHandler, err := kit.NewCLIMCPAuthHandler()\nif err != nil {\n    log.Fatal(err)\n}\ndefer authHandler.Close() // release the reserved port\n\nhost, _ := kit.New(ctx, &kit.Options{\n    MCPAuthHandler: authHandler,\n})\n```\n\n**Custom UX embedder (TUI modal, QR code, web redirect, etc.):**\n\n```go\nauthHandler, _ := kit.NewDefaultMCPAuthHandler()\nauthHandler.OnAuthURL = func(serverName, authURL string) {\n    // No browser or terminal assumptions — render however you like.\n    myUI.ShowAuthPrompt(serverName, authURL)\n}\ndefer authHandler.Close()\n\nhost, _ := kit.New(ctx, &kit.Options{\n    MCPAuthHandler: authHandler,\n})\n```\n\n**Fully custom handler (no local port binding at all):**\n\n```go\ntype WebAuthHandler struct {\n    redirectURI string\n    callbacks   chan string\n}\n\nfunc (h *WebAuthHandler) RedirectURI() string { return h.redirectURI }\n\nfunc (h *WebAuthHandler) HandleAuth(ctx context.Context, serverName, authURL string) (string, error) {\n    // Push the URL to the user\'s existing browser session via your web app,\n    // then block on the callback that your HTTP handler pushes onto the channel.\n    h.pushToUserSession(serverName, authURL)\n    select {\n    case callbackURL := <-h.callbacks:\n        return callbackURL, nil\n    case <-ctx.Done():\n        return "", ctx.Err()\n    }\n}\n```\n\n> **Warning:** `DefaultMCPAuthHandler` with no `OnAuthURL` set will silently\n> drop the authorization URL and hang until the 2-minute callback timeout\n> fires. Always set `OnAuthURL`, or use a higher-level wrapper like\n> `CLIMCPAuthHandler`.\n\n## MCP Tasks\n\nThe [MCP Tasks utility](https://modelcontextprotocol.io/specification/2025-11-25/basic/utilities/tasks)\nturns a synchronous `tools/call` into a pollable async job: the server\nreturns a `taskId` with status `working` immediately, and the client polls\n`tasks/get` / `tasks/result` until the task reaches a terminal state.\n\nKit advertises task support during `initialize` and, by default, augments\n`tools/call` with task metadata only when the server advertises\n`tasks/toolCalls` capability — so any existing MCP server keeps its previous\nsynchronous behaviour bit-for-bit. Long-running tools (builds, deployments,\nbatch jobs, sub-agent runs) get HTTP/SSE timeout-resistance and clean\ncancellation "for free" once both sides opt in.\n\n### Per-server mode\n\n```go\nimport "time"\n\nhost, _ := kit.New(ctx, &kit.Options{\n    MCPTaskMode: map[string]kit.MCPTaskMode{\n        "build-server": kit.MCPTaskModeAlways, // force task-augmented calls\n        "chat-server":  kit.MCPTaskModeNever,  // force synchronous calls\n        // any server not in the map honours its `tasksMode` config field\n        // (default "auto")\n    },\n})\n```\n\n| Mode | Behaviour |\n|---|---|\n| `MCPTaskModeAuto` (default) | Augment `tools/call` with `TaskParams` only when the server advertised `tasks/toolCalls`. |\n| `MCPTaskModeNever` | Always issue `tools/call` synchronously, ignoring server capability. |\n| `MCPTaskModeAlways` | Always opt in, even when the server didn\'t advertise the capability. The server may still respond synchronously. |\n\n### Progress callbacks\n\n```go\nhost, _ := kit.New(ctx, &kit.Options{\n    MCPTaskTimeout:  15 * time.Minute,        // total wall-clock cap\n    MCPTaskTTL:      30 * time.Minute,        // server retention hint\n    MCPTaskProgress: func(p kit.MCPTaskProgress) {\n        log.Printf("%s/%s: %s %s", p.Server, p.TaskID, p.Status, p.Message)\n    },\n})\n```\n\nThe handler fires once when a task is accepted and again on every observed\nstatus transition. The final call always carries a terminal status\n(`MCPTaskStatusCompleted`, `MCPTaskStatusFailed`, or `MCPTaskStatusCancelled`).\nDo not block in the handler — dispatch long work on a goroutine.\n\n### Inspecting and cancelling tasks\n\n```go\ntasks, _ := host.ListMCPTasks(ctx, "build-server")\nfor _, t := range tasks {\n    fmt.Printf("%s: %s (%s)\\n", t.TaskID, t.Status, t.StatusMessage)\n}\n\nt, _ := host.GetMCPTask(ctx, "build-server", taskID)\nif !t.Status.IsTerminal() {\n    _, _ = host.CancelMCPTask(ctx, "build-server", taskID)\n}\n```\n\n`Kit.ListMCPTasks`, `Kit.GetMCPTask`, and `Kit.CancelMCPTask` work against any\nloaded MCP server that advertises the corresponding capability.\n`MCPTaskStatus.IsTerminal()` is the canonical check for completion.\n\nContext cancellation also works end-to-end: cancelling the `ctx` passed to a\ntool execution triggers a best-effort `tasks/cancel` before the call returns.\n\n## Custom debug logger\n\nKit\'s engine and MCP tool plumbing emit low-level debug output through a\n`DebugLogger` interface. By default, setting `Debug: true` (or calling\n`WithDebug()`) installs the built-in console logger. To route the same output\ninto your application\'s logging system instead, provide a custom\nimplementation via `Options.DebugLogger` or `WithDebugLogger`.\n\n```go\ntype DebugLogger interface {\n    LogDebug(message string)\n    IsDebugEnabled() bool\n}\n```\n\nWhen `DebugLogger` is non-nil it takes precedence over `Debug` — the\nsupplied logger\'s `IsDebugEnabled()` reports whether downstream code should\nbother formatting messages.\n\n**Example: forward to `log/slog`:**\n\n```go\nimport "log/slog"\n\ntype slogDebugLogger struct{ l *slog.Logger }\n\nfunc (s *slogDebugLogger) LogDebug(m string)    { s.l.Debug(m) }\nfunc (s *slogDebugLogger) IsDebugEnabled() bool { return true }\n\nhost, _ := kit.NewAgent(ctx,\n    kit.WithModel("anthropic/claude-sonnet-4-5-20250929"),\n    kit.WithDebugLogger(&slogDebugLogger{l: slog.Default()}),\n)\n```\n\nImplementations must be safe for concurrent use — messages can arrive\nfrom the engine goroutine, MCP connection pool, and tool execution paths\nsimultaneously.\n\n## Precedence\n\nFor any given generation or provider field, the effective value is resolved\nin this order (highest priority first):\n\n1. `Options.X` (SDK caller)\n2. `KIT_X` environment variable\n3. `.kit.yml` (project-local then `~/.kit.yml`)\n4. Per-model defaults (`modelSettings[provider/model]` or `customModels[...].params`)\n5. Provider-level defaults (e.g. Anthropic\'s own temperature default)\n6. SDK last-resort floor (currently: `MaxTokens = 8192`, matching the CLI `--max-tokens` default)\n\nSampling params that remain `nil` after the SDK resolution step are left out\nof the provider call entirely, so the LLM library applies its own default.\n\n## Tool configuration\n\n**`Tools`** replaces default core tools, not MCP or extension tools. **`ExtraTools`** adds tools alongside the defaults. Use `Tools` to replace core tools; use `ExtraTools` to extend them.\n\nTo keep the default tool set but narrow the built-in core tools, set\n**`CoreToolList`** (an allow-list of names such as `"bash"`, `"read"`,\n`"write"`, `"edit"`, `"grep"`, `"find"`, `"ls"`, `"subagent"`). Build the list\nfrom include/exclude filters with\n[`kit.FilterCoreToolNames`](/sdk/overview#filtering-core-tools):\n\n```go\nlist, _ := kit.FilterCoreToolNames(nil, []string{"bash", "write"}) // drop two\nhost, _ := kit.New(ctx, &kit.Options{CoreToolList: list})\n```\n\n`DisableCoreTools: true` is the chat-only shortcut for an empty core set.\n\nCreate custom tools with `kit.NewTool` — no external dependencies needed:\n\n```go\ntype LookupInput struct {\n    ID string `json:"id" description:"Record ID to look up"`\n}\n\nlookupTool := kit.NewTool("lookup", "Look up a record by ID",\n    func(ctx context.Context, input LookupInput) (kit.ToolOutput, error) {\n        record := db.Find(input.ID)\n        return kit.TextResult(record.String()), nil\n    },\n)\n\nhost, _ := kit.New(ctx, &kit.Options{\n    ExtraTools: []kit.Tool{lookupTool},\n})\n```\n\nSee [Overview](/sdk/overview#custom-tools) for full custom tool documentation.\n\n### Routing file access and commands\n\nBy default the core tools read and write the local disk and run commands as\nlocal child processes. Two tool options change this, for example to let an\neditor provide unsaved buffers and run commands in its own terminal:\n\n- **`kit.WithFileSystem(fs)`**: the `read`, `write` and `edit` tools read and\n  write text through `fs` (a `kit.FileSystem`). Images are still read from\n  the local disk.\n- **`kit.WithCommandRunner(r)`**: the `shell` tool runs commands through `r`\n  (a `kit.CommandRunner`). The tool still checks the command and the timeout\n  first. Interactive sudo prompts are not available through a runner.\n\n```go\nhost, _ := kit.New(ctx, &kit.Options{\n    WorkDir:         "/path/to/project",\n    CoreToolOptions: []kit.ToolOption{kit.WithFileSystem(myFS), kit.WithCommandRunner(myRunner)},\n})\n```\n\n`kit acp` uses these options to implement the ACP `fs/*` and `terminal/*`\nclient methods.\n\n## Options and functional helper inventory\n\nThis inventory links configuration to the [authoritative Go contract](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options). Fields retain their documented zero/unset semantics. Functional options apply in order; later values win. Use `New` with `Options` for fields without a helper.\n\n| Name | Contract |\n| --- | --- |\n| `WithProvider` | Registers a provider factory for this Kit instance only; see `Providers`. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#WithProvider) |\n| `WithDecisionProvider` | Registers a decision provider factory for this Kit instance only; see `DecisionProviders`. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#WithDecisionProvider) |\n| `AllowMissingCredentials` | AllowMissingCredentials lets New succeed when the configured provider cannot be created — no API key or OAuth token, a failed OAuth token refresh, an unsupported provider, and so on. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |\n| `AlwaysOfferSkillActivation` | AlwaysOfferSkillActivation keeps the built-in activate_skill tool offered with an empty catalogue. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |\n| `AutoCompact` | Compaction AutoCompact enables proactive compaction before turns that near the context limit. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |\n| `Bare` | Bare disables every form of automatic context discovery, so starting Kit inside a directory does not implicitly load that directory\'s instructions, tooling or configuration. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |\n| `CLI` | CLI is optional CLI-specific configuration. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |\n| `CodeMode` | CodeMode configures code mode: one tool that runs a JavaScript program which calls the other tools and returns a small result. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |\n| `CompactionOptions` | Config for auto-compaction (nil = defaults). [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |\n| `ConfigFile` | Override config file path. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |\n| `Continue` | Continue the newest session for SessionDir. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |\n| `CoreToolList` | List of tools to include, when empty, include all available core tools. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |\n| `CoreToolOptions` | CoreToolOptions are applied to the built-in core tools after the options Kit derives from its own settings, so they win on conflict. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |\n| `DebugLogger` | DebugLogger, if non-nil, routes low-level debug output from the engine and the MCP tool plumbing to a caller-supplied implementation. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |\n| `Debug` | Debug enables debug logging for the SDK. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |\n| `DisableCoreTools` | DisableCoreTools, when true, prevents loading any core tools. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |\n| `Ephemeral` | Ephemeral configures an in-memory session with no persistence (equivalent to Options.NoSession = true). [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Ephemeral) |\n| `ExtraTools` | Additional tools added alongside core/MCP/extension tools. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |\n| `FrequencyPenalty` | FrequencyPenalty discourages repeated tokens (OpenAI-family models). [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |\n| `HTTPTransport` | HTTPTransport wraps each built-in provider\'s base HTTP transport. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |\n| `ImageMaxBytes` | ImageMaxBytes caps the encoded size, in bytes, of an image the read tool attaches to a message. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |\n| `ImageMaxEdge` | ImageMaxEdge caps the width and height, in pixels, of an image the read tool attaches to a message. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |\n| `ImageNoResize` | ImageNoResize makes the read tool reject an oversized image instead of scaling it down. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |\n| `InProcessMCPServers` | InProcessMCPServers registers mcp-go servers that run in the same process. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |\n| `Isolated` | Isolated turns off everything that Kit otherwise discovers or writes on the host by itself. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Isolated) |\n| `MCPAuthHandler` | MCPAuthHandler handles OAuth authorization for remote MCP servers. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |\n| `MCPConfig` | MCPConfig provides a pre-loaded MCP configuration. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |\n| `MCPTaskMaxPollInterval` | MCPTaskMaxPollInterval caps the polling interval (a server-supplied pollInterval can otherwise grow without bound). [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |\n| `MCPTaskMode` | MCPTaskMode overrides the per-server [MCPTaskMode] for task-augmented tools/call execution. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |\n| `MCPTaskPollInterval` | MCPTaskPollInterval is the fallback interval between tasks/get requests when the server does not suggest one. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |\n| `MCPTaskProgress` | MCPTaskProgress, if non-nil, is invoked once when a task is accepted and on every status transition observed by the polling loop. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |\n| `MCPTaskTTL` | MCPTaskTTL is the TTL hint sent in TaskParams for every task-augmented tools/call. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |\n| `MCPTaskTimeout` | MCPTaskTimeout is the maximum wall-clock duration to wait for a task-augmented tool call to reach a terminal state. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |\n| `MCPTokenStoreFactory` | MCPTokenStoreFactory, if non-nil, is called to create a token store for each remote MCP server that requires OAuth. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |\n| `MaxSteps` | Override max steps (0 = use default). [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |\n| `MaxTokens` | MaxTokens overrides the maximum output tokens per LLM response. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |\n| `ModelMode` | Model mode to select (empty selects the model default). [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |\n| `Model` | Override model (e.g., "anthropic/claude-sonnet-4-5-20250929"). [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |\n| `DecisionModel` | Default decision model in "provider/model" format, used by Kit.Decide and extensions when a request names no model. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |\n| `NewAgent` | NewAgent creates a Kit using an ergonomic functional-options API. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#NewAgent) |\n| `NewIsolatedAgent` | NewIsolatedAgent is [NewAgent] with [Isolated] applied before opts. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#NewIsolatedAgent) |\n| `NoAgents` | NoAgents disables discovery of named agent definitions (built-ins and .agents/agents/ / .kit/agents/ / ~/.config/kit/agents/ files). [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |\n| `NoContextFiles` | NoContextFiles disables automatic loading of project context files (for example AGENTS.md). [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |\n| `NoExtensions` | NoExtensions disables Yaegi extension loading entirely. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |\n| `NoSession` | In-memory session without file persistence. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |\n| `NoSkills` | Disable skill loading entirely (auto-discovery and explicit). [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |\n| `OnMCPServerLoaded` | OnMCPServerLoaded, if non-nil, is called when each MCP server finishes loading during Kit initialization. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |\n| `PresencePenalty` | PresencePenalty discourages repeating topics (OpenAI-family models). [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |\n| `ProviderAPIKey` | ProviderAPIKey overrides the API key used to authenticate with the model provider. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |\n| `ProviderURL` | ProviderURL overrides the provider endpoint. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |\n| `ProviderWire` | ProviderWire overrides the wire protocol used for auto-routed providers: "openai" (Responses API), "openai-compat" (chat completions), "anthropic", or "google". [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |\n| `Providers` | Providers registers provider factories for this Kit instance, keyed by provider name (case-insensitive, must not contain "/"). [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |\n| `DecisionProviders` | DecisionProviders registers decision provider factories for this Kit instance, keyed by provider name with the same rules as Providers. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |\n| `Quiet` | Suppress debug output. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |\n| `RetryPolicy` | RetryPolicy limits retries per model request. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |\n| `SessionDir` | Project directory identity for session discovery, not a storage root. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |\n| `SessionManager` | SessionManager allows custom session storage backends. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |\n| `SessionPath` | Open a specific session file. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |\n| `ShellMaxTimeout` | ShellMaxTimeout caps the maximum timeout (in seconds) a shell tool call may request via its timeout argument. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |\n| `ShellTimeout` | ShellTimeout sets the default per-call timeout (in seconds) for the shell tool, applied when the model does not specify one. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |\n| `Shell` | Shell is the shell the shell tool runs a command string through, plus its own leading arguments (for example bash or busybox ash). [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |\n| `SkillTrustPrompt` | SkillTrustPrompt is an optional callback invoked the first time Kit auto-discovers project-local skills (under <project>/.agents/skills or <project>/.kit/skills) in a directory that is not yet on the trust allowlist. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |\n| `SkillsDir` | Scan this directory directly for skills. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |\n| `SkillsDisable` | SkillsDisable names skills (by Name) to exclude from the model-facing catalog. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |\n| `Skills` | Explicit skill files or directories; empty enables discovery. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |\n| `SkipConfig` | SkipConfig, when true, skips loading .kit.yml configuration files. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |\n| `Streaming` | Streaming enables or disables streaming output. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |\n| `SystemPrompt` | Override system prompt. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |\n| `TLSSkipVerify` | TLSSkipVerify disables TLS certificate verification on provider HTTP clients. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |\n| `Temperature` | Temperature controls sampling randomness (typically 0.0–2.0). [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |\n| `ThinkingLevel` | ThinkingLevel sets the reasoning effort for models that support extended thinking. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |\n| `Tools` | Custom tool set. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |\n| `TopK` | TopK limits sampling to the top K tokens. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |\n| `TopP` | TopP is the nucleus-sampling cutoff (0.0–1.0). [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |\n| `UserAgent` | UserAgent identifies Kit-controlled HTTP requests. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |\n| `WithAgents` | WithAgents enables discovery of named agent definitions again. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#WithAgents) |\n| `WithAllowMissingCredentials` | WithAllowMissingCredentials lets New succeed when the configured provider cannot be created (no credentials, a failed OAuth token refresh, an unsupported provider, and so on). [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#WithAllowMissingCredentials) |\n| `WithConfigFile` | WithConfigFile sets an explicit config file path, overriding the default .kit.yml search. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#WithConfigFile) |\n| `WithConfig` | WithConfig enables .kit.yml discovery again (the home and project config files). [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#WithConfig) |\n| `WithContextFiles` | WithContextFiles enables loading of project context files (AGENTS.md) from the working directory again. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#WithContextFiles) |\n| `WithCoreTools` | WithCoreTools enables the core tools again. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#WithCoreTools) |\n| `WithDebugLogger` | WithDebugLogger installs a caller-supplied [DebugLogger] for low-level engine and MCP tool plumbing output. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#WithDebugLogger) |\n| `WithDebug` | WithDebug enables SDK debug logging. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#WithDebug) |\n| `WithExtensions` | WithExtensions enables loading of extensions again. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#WithExtensions) |\n| `WithExtraTools` | WithExtraTools adds tools alongside the core/MCP/extension tools rather than replacing them. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#WithExtraTools) |\n| `WithHTTPTransport` | WithHTTPTransport wraps the base HTTP transport of built-in providers. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#WithHTTPTransport) |\n| `WithMaxTokens` | WithMaxTokens sets the maximum output tokens per LLM response. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#WithMaxTokens) |\n| `WithModelMode` | WithModelMode selects a named mode for the selected model. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#WithModelMode) |\n| `WithModel` | WithModel sets the model in "provider/model" format (for example openai/gpt-4.1). [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#WithModel) |\n| `WithDecisionModel` | WithDecisionModel sets the default decision model in "provider/model" format (e.g. openrouter/typesafe/jev-1.14). [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#WithDecisionModel) |\n| `WithProviderAPIKey` | WithProviderAPIKey overrides the API key used to authenticate with the model provider. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#WithProviderAPIKey) |\n| `WithProviderURL` | WithProviderURL overrides the provider endpoint URL. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#WithProviderURL) |\n| `WithProviderWire` | WithProviderWire overrides the wire protocol used for auto-routed providers: "openai" (Responses API), "openai-compat" (chat completions), "anthropic", or "google". [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#WithProviderWire) |\n| `WithRetryPolicy` | WithRetryPolicy sets the retry limits for model requests. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#WithRetryPolicy) |\n| `WithSessions` | WithSessions enables persistent sessions again. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#WithSessions) |\n| `WithSkillActivation` | WithSkillActivation keeps activate_skill available even before any skills are registered. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#WithSkillActivation) |\n| `WithSkills` | WithSkills enables skills again. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#WithSkills) |\n| `WithStreaming` | WithStreaming enables or disables streaming responses. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#WithStreaming) |\n| `WithSystemPrompt` | WithSystemPrompt sets the system prompt. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#WithSystemPrompt) |\n| `WithThinkingLevel` | WithThinkingLevel sets the reasoning effort for models that support extended thinking. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#WithThinkingLevel) |\n| `WithTools` | WithTools sets the agent\'s tool set, replacing the default core tools. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#WithTools) |\n| `WithUserAgent` | WithUserAgent sets the per-instance HTTP identity. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#WithUserAgent) |\n| `WorkDir` | WorkDir is the directory the built-in file and shell tools resolve relative paths against and run commands in. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |\n\n## Model modes\n\n`ModelMode` describes catalog modes, including `Experimental`, `Supported`, `UnsupportedReason`, and estimated input/output USD per million tokens. `host.ModelModes()` includes unavailable modes; `host.SupportedModelModes()` filters them. Query these methods rather than assuming every model offers a fixed mode.\n\nSelect with `Options.ModelMode` / `WithModelMode`, or `host.SetModelMode(ctx, name)`. Check errors with `errors.Is(err, kit.ErrUnsupportedModelMode)` for unsupported modes. `CurrentModelMode` reports the selection; an empty string selects the model default. Changes during an active turn fail, including reset. Finish or cancel the turn first.\n\n## Code mode\n\n`Options.CodeMode` points to `CodeModeOptions`. Code mode is default-on, but default enablement respects the resolved tool list and disabled core tools. Explicit `Enabled: true` can offer code mode with MCP-only tools; naming `codemode` in the core tool list selects it. `Disabled: true` takes precedence over enablement and config. For a custom `Tools` or `ExtraTools` set, create the tool with `NewCodeModeTool`, check its error, and add the returned tool. Kit supplies the current tool set with hooks applied, so nested calls still pass through hooks.\n\nZero option fields fall back to the `codemode` config section, then defaults:\n\n| Field | Default and unit |\n| --- | --- |\n| `Timeout` | 30 seconds (`time.Duration` in Go, seconds in YAML) |\n| `MaxToolCalls` | 50 tool calls per script |\n| `MaxOutputBytes` | 100000 bytes; larger captured output may be saved to a temporary file, but combined script output and logs are capped at 4 MiB and excess is truncated |\n| `MaxConcurrency` | 8 parallel tool calls per script |\n| `MemoryLimitMB` | 256 MiB approximate heap growth (value shifted by 20 bits); negative disables check |\n| `CatalogBudget` | 12000 bytes in the tool catalog description |\n| `MCPExposure` | `CodeModeExposureDirect` for unmatched MCP tools |\n| `Exposure` | Tool-name glob rules; exact names win over globs, then longer patterns win |\n\nExposure constants and their config values:\n\n- `CodeModeExposureDirect` (`direct`): model sees the tool; scripts can call it.\n- `CodeModeExposureScriptOnly` (`codemode`): scripts only; full signature in catalog.\n- `CodeModeExposureDeferred` (`deferred`): scripts only; discover with `searchTools()`.\n- `CodeModeExposureModelOnly` (`model-only`): model only; scripts cannot call it.\n\n`CodeModeToolName` is `codemode`. See [code mode](https://go-kit.dev/advanced/code-mode) for JavaScript APIs. JavaScript limits and discovery/isolation switches are not OS sandboxing. Tool access has the host process permissions; restrict the runtime when inputs are untrusted.\n\n## Managed skill activation\n\n`AlwaysOfferSkillActivation` / `WithSkillActivation` keeps `activate_skill` offered even with an empty catalog. It does not enable discovery or override `NoSkills`. Combine with `Isolated` and register application-owned skills. Use `Skill.NoHostIO` for content that must not be reopened from host paths. Caller-owned activation tools and explicit preparation tool overrides retain precedence. See the [Go contract](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#WithSkillActivation) for per-step snapshot and cancellation rules.\n'};export{e as default};
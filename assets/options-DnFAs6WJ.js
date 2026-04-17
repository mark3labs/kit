const s={frontmatter:{title:"SDK Options",description:"Configuration options for the Kit Go SDK.",hidden:!1,toc:!0,draft:!1},html:`<h1 id="sdk-options"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#sdk-options"><span class="icon icon-link"></span></a>SDK Options</h1>
<p>Pass an <code>Options</code> struct to <code>kit.New()</code> to configure the Kit instance.</p>
<h2 id="full-options-reference"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#full-options-reference"><span class="icon icon-link"></span></a>Full options reference</h2>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">host, err </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> kit.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">New</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(ctx, </span><span style="color:#D73A49;--shiki-dark:#F97583">&amp;</span><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Options</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // Model</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Model:        </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"ollama/llama3"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    SystemPrompt: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"You are a helpful bot"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    ConfigFile:   </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"/path/to/config.yml"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // Behavior</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    MaxSteps:     </span><span style="color:#005CC5;--shiki-dark:#79B8FF">10</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Streaming:    </span><span style="color:#005CC5;--shiki-dark:#79B8FF">true</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Quiet:        </span><span style="color:#005CC5;--shiki-dark:#79B8FF">true</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Debug:        </span><span style="color:#005CC5;--shiki-dark:#79B8FF">true</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // Generation parameters (override env/config/per-model defaults)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    MaxTokens:        </span><span style="color:#005CC5;--shiki-dark:#79B8FF">16384</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,              </span><span style="color:#6A737D;--shiki-dark:#6A737D">// 0 = auto-resolve; non-zero suppresses right-sizing</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    ThinkingLevel:    </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"medium"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,           </span><span style="color:#6A737D;--shiki-dark:#6A737D">// "off", "low", "medium", "high"</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Temperature:      </span><span style="color:#6F42C1;--shiki-dark:#B392F0">ptrFloat32</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#005CC5;--shiki-dark:#79B8FF">0.2</span><span style="color:#24292E;--shiki-dark:#E1E4E8">),    </span><span style="color:#6A737D;--shiki-dark:#6A737D">// pointer so explicit 0.0 != unset</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    TopP:             </span><span style="color:#005CC5;--shiki-dark:#79B8FF">nil</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,                 </span><span style="color:#6A737D;--shiki-dark:#6A737D">// nil = provider/per-model default</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    TopK:             </span><span style="color:#005CC5;--shiki-dark:#79B8FF">nil</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    FrequencyPenalty: </span><span style="color:#005CC5;--shiki-dark:#79B8FF">nil</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    PresencePenalty:  </span><span style="color:#005CC5;--shiki-dark:#79B8FF">nil</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // Provider configuration</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    ProviderAPIKey: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"sk-..."</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,                      </span><span style="color:#6A737D;--shiki-dark:#6A737D">// "" = use config / provider env var</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    ProviderURL:    </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"https://proxy.internal/v1"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,  </span><span style="color:#6A737D;--shiki-dark:#6A737D">// "" = provider default endpoint</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    TLSSkipVerify:  </span><span style="color:#005CC5;--shiki-dark:#79B8FF">false</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,                         </span><span style="color:#6A737D;--shiki-dark:#6A737D">// only effective when true</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // Session</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    SessionPath:  </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"./session.jsonl"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    SessionDir:   </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"/custom/sessions/"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Continue:     </span><span style="color:#005CC5;--shiki-dark:#79B8FF">true</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    NoSession:    </span><span style="color:#005CC5;--shiki-dark:#79B8FF">true</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // Tools</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Tools:            []</span><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Tool</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span><span style="color:#D73A49;--shiki-dark:#F97583">...</span><span style="color:#24292E;--shiki-dark:#E1E4E8">},     </span><span style="color:#6A737D;--shiki-dark:#6A737D">// Replace default tool set entirely</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    ExtraTools:       []</span><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Tool</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span><span style="color:#D73A49;--shiki-dark:#F97583">...</span><span style="color:#24292E;--shiki-dark:#E1E4E8">},     </span><span style="color:#6A737D;--shiki-dark:#6A737D">// Add tools alongside defaults</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    DisableCoreTools: </span><span style="color:#005CC5;--shiki-dark:#79B8FF">true</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,                </span><span style="color:#6A737D;--shiki-dark:#6A737D">// Use no core tools (0 tools, for chat-only)</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // Configuration</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    SkipConfig:   </span><span style="color:#005CC5;--shiki-dark:#79B8FF">true</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,                   </span><span style="color:#6A737D;--shiki-dark:#6A737D">// Skip .kit.yml files (viper defaults + env vars still apply)</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // Compaction</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    AutoCompact:  </span><span style="color:#005CC5;--shiki-dark:#79B8FF">true</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // Skills</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Skills:       []</span><span style="color:#D73A49;--shiki-dark:#F97583">string</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"/path/to/skill.md"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">},</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    SkillsDir:    </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"/path/to/skills/"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    NoSkills:     </span><span style="color:#005CC5;--shiki-dark:#79B8FF">true</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // Feature toggles</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    NoExtensions:   </span><span style="color:#005CC5;--shiki-dark:#79B8FF">true</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,               </span><span style="color:#6A737D;--shiki-dark:#6A737D">// disable Yaegi extension loading</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    NoContextFiles: </span><span style="color:#005CC5;--shiki-dark:#79B8FF">true</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,               </span><span style="color:#6A737D;--shiki-dark:#6A737D">// disable automatic AGENTS.md loading</span></span>
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
<td><code>SystemPrompt</code></td>
<td><code>string</code></td>
<td>—</td>
<td>System prompt text or file path</td>
</tr>
<tr>
<td><code>ConfigFile</code></td>
<td><code>string</code></td>
<td><code>~/.kit.yml</code></td>
<td>Path to config file</td>
</tr>
<tr>
<td><code>MaxSteps</code></td>
<td><code>int</code></td>
<td><code>0</code></td>
<td>Max agent steps (0 = unlimited)</td>
</tr>
<tr>
<td><code>Streaming</code></td>
<td><code>bool</code></td>
<td><code>true</code></td>
<td>Enable streaming output</td>
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
<td>Enable debug logging</td>
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
<td>Reasoning effort: <code>"off"</code>, <code>"low"</code>, <code>"medium"</code>, <code>"high"</code> (some providers also accept <code>"minimal"</code>). <code>""</code> falls through to config/env/per-model/<code>"off"</code>.</td>
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
<p>Pointer-typed samplers are populated via a tiny helper:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">func</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> ptrFloat32</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#E36209;--shiki-dark:#FFAB70">v</span><span style="color:#D73A49;--shiki-dark:#F97583"> float32</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) </span><span style="color:#D73A49;--shiki-dark:#F97583">*float32</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> { </span><span style="color:#D73A49;--shiki-dark:#F97583">return</span><span style="color:#D73A49;--shiki-dark:#F97583"> &amp;</span><span style="color:#24292E;--shiki-dark:#E1E4E8">v }</span></span></code></pre>
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
<td>API key used to authenticate with the provider. <code>""</code> falls back to config / provider-specific env var (e.g. <code>ANTHROPIC_API_KEY</code>). When set, overrides any pre-existing viper state.</td>
</tr>
<tr>
<td><code>ProviderURL</code></td>
<td><code>string</code></td>
<td>—</td>
<td>Override the provider endpoint (e.g. LiteLLM, vLLM, Azure OpenAI, internal proxy). <code>""</code> = provider default.</td>
</tr>
<tr>
<td><code>TLSSkipVerify</code></td>
<td><code>bool</code></td>
<td><code>false</code></td>
<td>Disable TLS certificate verification on the provider HTTP client. Only effective when <code>true</code>; to force-disable, use config file or env var instead. For self-signed dev certs only.</td>
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
<td>Replace the entire default tool set</td>
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
</tbody>
</table>
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
<td>Override default skills directory</td>
</tr>
<tr>
<td><code>NoSkills</code></td>
<td><code>bool</code></td>
<td><code>false</code></td>
<td>Disable skill loading entirely</td>
</tr>
</tbody>
</table>
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
<td>Auto-compact when near context limit</td>
</tr>
<tr>
<td><code>CompactionOptions</code></td>
<td><code>*CompactionOptions</code></td>
<td>—</td>
<td>Configuration for auto-compaction</td>
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
</tbody>
</table>
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
<p>::: warning
<code>DefaultMCPAuthHandler</code> with no <code>OnAuthURL</code> set will silently drop the
authorization URL and hang until the 2-minute callback timeout fires. Always
set <code>OnAuthURL</code>, or use a higher-level wrapper like <code>CLIMCPAuthHandler</code>.
:::</p>
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
<p><strong><code>Tools</code></strong> replaces ALL default tools (core + MCP + extension). <strong><code>ExtraTools</code></strong> adds tools alongside the defaults. Use <code>Tools</code> to restrict capabilities; use <code>ExtraTools</code> to extend them.</p>
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
<p>See <a href="/sdk/overview#custom-tools">Overview</a> for full custom tool documentation.</p>`,headings:[{depth:2,text:"Full options reference",id:"full-options-reference"},{depth:2,text:"Options fields",id:"options-fields"},{depth:3,text:"Core",id:"core"},{depth:3,text:"Generation parameters",id:"generation-parameters"},{depth:3,text:"Provider configuration",id:"provider-configuration"},{depth:3,text:"Session",id:"session"},{depth:3,text:"Tools &amp; extensions",id:"tools--extensions"},{depth:3,text:"Skills &amp; configuration",id:"skills--configuration"},{depth:3,text:"Compaction &amp; MCP",id:"compaction--mcp"},{depth:2,text:"MCP OAuth Authorization",id:"mcp-oauth-authorization"},{depth:2,text:"Precedence",id:"precedence"},{depth:2,text:"Tool configuration",id:"tool-configuration"}],raw:'\n# SDK Options\n\nPass an `Options` struct to `kit.New()` to configure the Kit instance.\n\n## Full options reference\n\n```go\nhost, err := kit.New(ctx, &kit.Options{\n    // Model\n    Model:        "ollama/llama3",\n    SystemPrompt: "You are a helpful bot",\n    ConfigFile:   "/path/to/config.yml",\n\n    // Behavior\n    MaxSteps:     10,\n    Streaming:    true,\n    Quiet:        true,\n    Debug:        true,\n\n    // Generation parameters (override env/config/per-model defaults)\n    MaxTokens:        16384,              // 0 = auto-resolve; non-zero suppresses right-sizing\n    ThinkingLevel:    "medium",           // "off", "low", "medium", "high"\n    Temperature:      ptrFloat32(0.2),    // pointer so explicit 0.0 != unset\n    TopP:             nil,                 // nil = provider/per-model default\n    TopK:             nil,\n    FrequencyPenalty: nil,\n    PresencePenalty:  nil,\n\n    // Provider configuration\n    ProviderAPIKey: "sk-...",                      // "" = use config / provider env var\n    ProviderURL:    "https://proxy.internal/v1",  // "" = provider default endpoint\n    TLSSkipVerify:  false,                         // only effective when true\n\n    // Session\n    SessionPath:  "./session.jsonl",\n    SessionDir:   "/custom/sessions/",\n    Continue:     true,\n    NoSession:    true,\n\n    // Tools\n    Tools:            []kit.Tool{...},     // Replace default tool set entirely\n    ExtraTools:       []kit.Tool{...},     // Add tools alongside defaults\n    DisableCoreTools: true,                // Use no core tools (0 tools, for chat-only)\n\n    // Configuration\n    SkipConfig:   true,                   // Skip .kit.yml files (viper defaults + env vars still apply)\n\n    // Compaction\n    AutoCompact:  true,\n\n    // Skills\n    Skills:       []string{"/path/to/skill.md"},\n    SkillsDir:    "/path/to/skills/",\n    NoSkills:     true,\n\n    // Feature toggles\n    NoExtensions:   true,               // disable Yaegi extension loading\n    NoContextFiles: true,               // disable automatic AGENTS.md loading\n\n    // Session (advanced)\n    SessionManager: myCustomSession,    // custom SessionManager implementation\n\n    // MCP OAuth — both opt-in. Leave MCPAuthHandler nil to disable\n    // OAuth entirely (remote MCP 401s bubble up as errors). CLI apps\n    // pass kit.NewCLIMCPAuthHandler(); custom UX embedders implement\n    // MCPAuthHandler or configure DefaultMCPAuthHandler + OnAuthURL.\n    MCPAuthHandler: authHandler,                  // nil = OAuth disabled\n    MCPTokenStoreFactory: func(serverURL string) (kit.MCPTokenStore, error) {\n        return myStore(serverURL), nil\n    },\n\n    // In-Process MCP Servers\n    InProcessMCPServers: map[string]*kit.MCPServer{\n        "docs": mcpSrv,  // *server.MCPServer from mcp-go\n    },\n})\n```\n\n## Options fields\n\n### Core\n\n| Field | Type | Default | Description |\n|-------|------|---------|-------------|\n| `Model` | `string` | config default | Model string (provider/model format) |\n| `SystemPrompt` | `string` | — | System prompt text or file path |\n| `ConfigFile` | `string` | `~/.kit.yml` | Path to config file |\n| `MaxSteps` | `int` | `0` | Max agent steps (0 = unlimited) |\n| `Streaming` | `bool` | `true` | Enable streaming output |\n| `Quiet` | `bool` | `false` | Suppress output |\n| `Debug` | `bool` | `false` | Enable debug logging |\n\n### Generation parameters\n\nThese fields override the corresponding values from `.kit.yml` / `KIT_*`\nenvironment variables. Leaving a field at its zero/nil value lets the\nprecedence chain resolve a value (`KIT_*` env → config file → per-model\ndefaults from `modelSettings`/`customModels` → an 8192 SDK floor for\n`MaxTokens` (matching the CLI `--max-tokens` default) and provider-level\ndefaults for samplers).\n\n| Field | Type | Default | Description |\n|-------|------|---------|-------------|\n| `MaxTokens` | `int` | auto-resolved | Max output tokens per response. `0` = auto-resolve; non-zero suppresses automatic right-sizing (same semantics as `--max-tokens`). |\n| `ThinkingLevel` | `string` | auto-resolved | Reasoning effort: `"off"`, `"low"`, `"medium"`, `"high"` (some providers also accept `"minimal"`). `""` falls through to config/env/per-model/`"off"`. |\n| `Temperature` | `*float32` | — | Sampling randomness. Pointer type so explicit `0.0` is distinguishable from "unset". |\n| `TopP` | `*float32` | — | Nucleus sampling cutoff. `nil` leaves provider/per-model default. |\n| `TopK` | `*int32` | — | Top-K sampling limit. `nil` leaves provider/per-model default. |\n| `FrequencyPenalty` | `*float32` | — | OpenAI-family frequency penalty. `nil` leaves provider default. |\n| `PresencePenalty` | `*float32` | — | OpenAI-family presence penalty. `nil` leaves provider default. |\n\nPointer-typed samplers are populated via a tiny helper:\n\n```go\nfunc ptrFloat32(v float32) *float32 { return &v }\n```\n\nThese fields eliminate the need for `viper.Set()` calls before `kit.New()`\nwhen embedding Kit as a library.\n\n### Provider configuration\n\n| Field | Type | Default | Description |\n|-------|------|---------|-------------|\n| `ProviderAPIKey` | `string` | — | API key used to authenticate with the provider. `""` falls back to config / provider-specific env var (e.g. `ANTHROPIC_API_KEY`). When set, overrides any pre-existing viper state. |\n| `ProviderURL` | `string` | — | Override the provider endpoint (e.g. LiteLLM, vLLM, Azure OpenAI, internal proxy). `""` = provider default. |\n| `TLSSkipVerify` | `bool` | `false` | Disable TLS certificate verification on the provider HTTP client. Only effective when `true`; to force-disable, use config file or env var instead. For self-signed dev certs only. |\n\n### Session\n\n| Field | Type | Default | Description |\n|-------|------|---------|-------------|\n| `SessionPath` | `string` | — | Open a specific session file |\n| `SessionDir` | `string` | — | Base directory for session discovery |\n| `Continue` | `bool` | `false` | Resume most recent session |\n| `NoSession` | `bool` | `false` | Ephemeral mode (no persistence) |\n| `SessionManager` | `SessionManager` | — | Custom session backend (advanced) |\n\n### Tools & extensions\n\n| Field | Type | Default | Description |\n|-------|------|---------|-------------|\n| `Tools` | `[]Tool` | — | Replace the entire default tool set |\n| `ExtraTools` | `[]Tool` | — | Additional tools alongside core/MCP/extension tools |\n| `DisableCoreTools` | `bool` | `false` | Use no core tools (0 tools, for chat-only) |\n| `NoExtensions` | `bool` | `false` | Disable Yaegi extension loading |\n| `NoContextFiles` | `bool` | `false` | Disable automatic AGENTS.md loading |\n\n### Skills & configuration\n\n| Field | Type | Default | Description |\n|-------|------|---------|-------------|\n| `SkipConfig` | `bool` | `false` | Skip `.kit.yml` file loading (viper defaults + env vars still apply) |\n| `Skills` | `[]string` | — | Explicit skill files/dirs to load |\n| `SkillsDir` | `string` | — | Override default skills directory |\n| `NoSkills` | `bool` | `false` | Disable skill loading entirely |\n\n### Compaction & MCP\n\n| Field | Type | Default | Description |\n|-------|------|---------|-------------|\n| `AutoCompact` | `bool` | `false` | Auto-compact when near context limit |\n| `CompactionOptions` | `*CompactionOptions` | — | Configuration for auto-compaction |\n| `MCPAuthHandler` | `MCPAuthHandler` | — | OAuth handler for remote MCP servers. `nil` disables OAuth (servers returning 401 fail with the authorization-required error). See [MCP OAuth](#mcp-oauth-authorization) below. |\n| `MCPTokenStoreFactory` | `func` | — | Custom OAuth token storage for MCP servers (default: JSON file in `$XDG_CONFIG_HOME/.kit/mcp_tokens.json`). |\n| `InProcessMCPServers` | `map[string]*MCPServer` | — | In-process mcp-go servers (no subprocess) |\n\n## MCP OAuth Authorization\n\nWhen a remote MCP server (SSE or Streamable HTTP) requires OAuth, Kit runs\nthe full authorization flow (dynamic client registration → PKCE → user\nconsent → token exchange → token persistence) but delegates the **user-facing\nstep** — displaying the authorization URL and receiving the callback — to\nan `MCPAuthHandler`.\n\nThe SDK is deliberately inert when `MCPAuthHandler` is `nil`: it does **not**\nauto-construct a default handler, bind a local TCP port, or open a browser.\nThis keeps library, daemon, and web-app embedders free of surprise I/O.\nConsumers opt in by passing a handler explicitly.\n\n| Building block | When to use |\n|---|---|\n| `MCPAuthHandler = nil` (default) | OAuth disabled. Remote MCP servers requiring auth fail with a clear error. Correct for libraries, daemons, and web apps. |\n| `kit.NewCLIMCPAuthHandler()` | CLI/TUI apps. Opens the system browser, prints status to stderr (or via `NotifyFunc`), runs a localhost callback server. Used by the `kit` binary. |\n| `kit.NewDefaultMCPAuthHandler()` + `OnAuthURL` | Custom UX. Use the SDK\'s port reservation and callback server; plug in your own presentation via the `OnAuthURL(serverName, authURL)` closure. |\n| Implement `kit.MCPAuthHandler` directly | Full control. No localhost binding — e.g. return the URL from an HTTP endpoint and have the consumer POST the callback URL back. |\n\n**CLI-style embedder:**\n\n```go\nauthHandler, err := kit.NewCLIMCPAuthHandler()\nif err != nil {\n    log.Fatal(err)\n}\ndefer authHandler.Close() // release the reserved port\n\nhost, _ := kit.New(ctx, &kit.Options{\n    MCPAuthHandler: authHandler,\n})\n```\n\n**Custom UX embedder (TUI modal, QR code, web redirect, etc.):**\n\n```go\nauthHandler, _ := kit.NewDefaultMCPAuthHandler()\nauthHandler.OnAuthURL = func(serverName, authURL string) {\n    // No browser or terminal assumptions — render however you like.\n    myUI.ShowAuthPrompt(serverName, authURL)\n}\ndefer authHandler.Close()\n\nhost, _ := kit.New(ctx, &kit.Options{\n    MCPAuthHandler: authHandler,\n})\n```\n\n**Fully custom handler (no local port binding at all):**\n\n```go\ntype WebAuthHandler struct {\n    redirectURI string\n    callbacks   chan string\n}\n\nfunc (h *WebAuthHandler) RedirectURI() string { return h.redirectURI }\n\nfunc (h *WebAuthHandler) HandleAuth(ctx context.Context, serverName, authURL string) (string, error) {\n    // Push the URL to the user\'s existing browser session via your web app,\n    // then block on the callback that your HTTP handler pushes onto the channel.\n    h.pushToUserSession(serverName, authURL)\n    select {\n    case callbackURL := <-h.callbacks:\n        return callbackURL, nil\n    case <-ctx.Done():\n        return "", ctx.Err()\n    }\n}\n```\n\n::: warning\n`DefaultMCPAuthHandler` with no `OnAuthURL` set will silently drop the\nauthorization URL and hang until the 2-minute callback timeout fires. Always\nset `OnAuthURL`, or use a higher-level wrapper like `CLIMCPAuthHandler`.\n:::\n\n## Precedence\n\nFor any given generation or provider field, the effective value is resolved\nin this order (highest priority first):\n\n1. `Options.X` (SDK caller)\n2. `KIT_X` environment variable\n3. `.kit.yml` (project-local then `~/.kit.yml`)\n4. Per-model defaults (`modelSettings[provider/model]` or `customModels[...].params`)\n5. Provider-level defaults (e.g. Anthropic\'s own temperature default)\n6. SDK last-resort floor (currently: `MaxTokens = 8192`, matching the CLI `--max-tokens` default)\n\nSampling params that remain `nil` after the SDK resolution step are left out\nof the provider call entirely, so the LLM library applies its own default.\n\n## Tool configuration\n\n**`Tools`** replaces ALL default tools (core + MCP + extension). **`ExtraTools`** adds tools alongside the defaults. Use `Tools` to restrict capabilities; use `ExtraTools` to extend them.\n\nCreate custom tools with `kit.NewTool` — no external dependencies needed:\n\n```go\ntype LookupInput struct {\n    ID string `json:"id" description:"Record ID to look up"`\n}\n\nlookupTool := kit.NewTool("lookup", "Look up a record by ID",\n    func(ctx context.Context, input LookupInput) (kit.ToolOutput, error) {\n        record := db.Find(input.ID)\n        return kit.TextResult(record.String()), nil\n    },\n)\n\nhost, _ := kit.New(ctx, &kit.Options{\n    ExtraTools: []kit.Tool{lookupTool},\n})\n```\n\nSee [Overview](/sdk/overview#custom-tools) for full custom tool documentation.\n'};export{s as default};

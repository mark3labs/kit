const s={frontmatter:{title:"Go SDK",description:"Embed Kit in your Go applications.",hidden:!1,toc:!0,draft:!1},html:`<h1 id="go-sdk"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#go-sdk"><span class="icon icon-link"></span></a>Go SDK</h1>
<p>The <code>pkg/kit</code> package lets you embed Kit as a library in your Go applications.</p>
<h2 id="installation"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#installation"><span class="icon icon-link"></span></a>Installation</h2>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">go</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> get</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> github.com/mark3labs/kit/pkg/kit</span></span></code></pre>
<h2 id="basic-usage"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#basic-usage"><span class="icon icon-link"></span></a>Basic usage</h2>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">package</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> main</span></span>
<span class="line"></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">import</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> (</span></span>
<span class="line"><span style="color:#032F62;--shiki-dark:#9ECBFF">    "</span><span style="color:#6F42C1;--shiki-dark:#B392F0">context</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"</span></span>
<span class="line"><span style="color:#032F62;--shiki-dark:#9ECBFF">    "</span><span style="color:#6F42C1;--shiki-dark:#B392F0">log</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"</span></span>
<span class="line"></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    kit </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"</span><span style="color:#6F42C1;--shiki-dark:#B392F0">github.com/mark3labs/kit/pkg/kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span>
<span class="line"></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">func</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> main</span><span style="color:#24292E;--shiki-dark:#E1E4E8">() {</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    ctx </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> context.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Background</span><span style="color:#24292E;--shiki-dark:#E1E4E8">()</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // Create Kit instance with default configuration</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    host, err </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> kit.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">New</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(ctx, </span><span style="color:#005CC5;--shiki-dark:#79B8FF">nil</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">    if</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> err </span><span style="color:#D73A49;--shiki-dark:#F97583">!=</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> nil</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> {</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">        log.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Fatal</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(err)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    }</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">    defer</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Close</span><span style="color:#24292E;--shiki-dark:#E1E4E8">()</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // Send a prompt</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    response, err </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Prompt</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(ctx, </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"What is 2+2?"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">    if</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> err </span><span style="color:#D73A49;--shiki-dark:#F97583">!=</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> nil</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> {</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">        log.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Fatal</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(err)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    }</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">    println</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(response)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">}</span></span></code></pre>
<h2 id="functional-options-newagent"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#functional-options-newagent"><span class="icon icon-link"></span></a>Functional options (<code>NewAgent</code>)</h2>
<p>For simple programmatic setups, <code>kit.NewAgent</code> offers an ergonomic
functional-options front door over <code>kit.New</code>. Streaming is <strong>enabled by
default</strong>; pass <code>kit.WithStreaming(false)</code> to opt out.</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">host, err </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> kit.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">NewAgent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(ctx,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    kit.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">WithModel</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"anthropic/claude-sonnet-4-5-20250929"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">),</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    kit.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">WithSystemPrompt</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"You are a helpful assistant."</span><span style="color:#24292E;--shiki-dark:#E1E4E8">),</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    kit.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">WithMaxTokens</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#005CC5;--shiki-dark:#79B8FF">8192</span><span style="color:#24292E;--shiki-dark:#E1E4E8">),</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    kit.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">WithThinkingLevel</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"medium"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">),</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    kit.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Ephemeral</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(), </span><span style="color:#6A737D;--shiki-dark:#6A737D">// in-memory session, no persistence</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">if</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> err </span><span style="color:#D73A49;--shiki-dark:#F97583">!=</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> nil</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> {</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    log.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Fatal</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(err)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">}</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">defer</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Close</span><span style="color:#24292E;--shiki-dark:#E1E4E8">()</span></span></code></pre>
<p>Available options:</p>
<table>
<thead>
<tr>
<th>Option</th>
<th>Sets</th>
</tr>
</thead>
<tbody>
<tr>
<td><code>WithModel(string)</code></td>
<td><code>Options.Model</code> (provider/model format)</td>
</tr>
<tr>
<td><code>WithSystemPrompt(string)</code></td>
<td><code>Options.SystemPrompt</code> (inline text or file path)</td>
</tr>
<tr>
<td><code>WithStreaming(bool)</code></td>
<td><code>Options.Streaming</code> (default <code>true</code> under <code>NewAgent</code>)</td>
</tr>
<tr>
<td><code>WithMaxTokens(int)</code></td>
<td><code>Options.MaxTokens</code></td>
</tr>
<tr>
<td><code>WithThinkingLevel(string)</code></td>
<td><code>Options.ThinkingLevel</code></td>
</tr>
<tr>
<td><code>WithTools(...Tool)</code></td>
<td><code>Options.Tools</code> (replaces the default set)</td>
</tr>
<tr>
<td><code>WithExtraTools(...Tool)</code></td>
<td><code>Options.ExtraTools</code> (adds alongside defaults)</td>
</tr>
<tr>
<td><code>WithProviderAPIKey(string)</code></td>
<td><code>Options.ProviderAPIKey</code></td>
</tr>
<tr>
<td><code>WithProviderURL(string)</code></td>
<td><code>Options.ProviderURL</code></td>
</tr>
<tr>
<td><code>WithProviderWire(string)</code></td>
<td><code>Options.ProviderWire</code> (wire protocol for auto-routed providers: <code>openai</code>, <code>openai-compat</code>, <code>anthropic</code>, <code>google</code>)</td>
</tr>
<tr>
<td><code>WithConfigFile(string)</code></td>
<td><code>Options.ConfigFile</code></td>
</tr>
<tr>
<td><code>WithDebug()</code></td>
<td><code>Options.Debug = true</code></td>
</tr>
<tr>
<td><code>WithDebugLogger(DebugLogger)</code></td>
<td><code>Options.DebugLogger</code> (route engine + MCP debug output into a custom logger; overrides <code>WithDebug</code> when set)</td>
</tr>
<tr>
<td><code>Ephemeral()</code></td>
<td><code>Options.NoSession = true</code></td>
</tr>
</tbody>
</table>
<p>Options are applied in order, so later options override earlier ones. <code>Option</code>
is a plain <code>func(*Options)</code>, so you can define your own. For advanced
configuration not covered by the helpers (custom MCP config, in-process MCP
servers, session backends, MCP task tuning) construct an <code>Options</code> value
explicitly and call <code>kit.New</code>.</p>
<h3 id="when-to-use-which"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#when-to-use-which"><span class="icon icon-link"></span></a>When to use which</h3>
<table>
<thead>
<tr>
<th>Constructor</th>
<th>Use when</th>
</tr>
</thead>
<tbody>
<tr>
<td><code>kit.NewAgent(ctx, ...Option)</code></td>
<td>Quick programmatic setups; you only need the common fields. Streaming defaults on.</td>
</tr>
<tr>
<td><code>kit.New(ctx, *Options)</code></td>
<td>You need fields without a <code>With*</code> helper (<code>MCPConfig</code>, <code>InProcessMCPServers</code>, <code>SessionManager</code>, MCP task tuning, etc.), or you already hold an <code>Options</code> value.</td>
</tr>
</tbody>
</table>
<h2 id="per-instance-config-isolation"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#per-instance-config-isolation"><span class="icon icon-link"></span></a>Per-instance config isolation</h2>
<p>Each <code>kit.New</code> / <code>kit.NewAgent</code> call owns an <strong>isolated configuration store</strong>,
so constructing multiple Kit instances in the same process is safe: setting the
model, thinking level, or generation parameters on one never affects another,
and runtime mutators (<code>SetModel</code>, <code>SetThinkingLevel</code>) only touch the owning
instance. This makes subagent spawning and multi-Kit embedding race-free with
no external synchronization required.</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">a, _ </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> kit.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">NewAgent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(ctx, kit.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">WithThinkingLevel</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"low"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">))</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">b, _ </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> kit.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">NewAgent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(ctx, kit.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">WithThinkingLevel</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"high"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">))</span></span>
<span class="line"></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">a.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">SetThinkingLevel</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(ctx, </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"medium"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// a.GetThinkingLevel() == "medium"; b.GetThinkingLevel() is still "high"</span></span></code></pre>
<h2 id="multi-turn-conversations"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#multi-turn-conversations"><span class="icon icon-link"></span></a>Multi-turn conversations</h2>
<p>Conversations retain context automatically across calls:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Prompt</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(ctx, </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"My name is Alice"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">response, _ </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Prompt</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(ctx, </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"What's my name?"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// response: "Your name is Alice"</span></span></code></pre>
<h2 id="additional-prompt-methods"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#additional-prompt-methods"><span class="icon icon-link"></span></a>Additional prompt methods</h2>
<p>The SDK provides several prompt variants:</p>
<table>
<thead>
<tr>
<th>Method</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td><code>Prompt(ctx, message)</code></td>
<td>Simple prompt, returns response string</td>
</tr>
<tr>
<td><code>PromptWithOptions(ctx, message, opts)</code></td>
<td>With per-call options (model, tools, thinking level, provider creds)</td>
</tr>
<tr>
<td><code>PromptResult(ctx, message)</code></td>
<td>Returns full <code>TurnResult</code> with usage stats</td>
</tr>
<tr>
<td><code>PromptResultWithOptions(ctx, message, opts)</code></td>
<td>Per-call options variant that returns the full <code>TurnResult</code></td>
</tr>
<tr>
<td><code>PromptResultWithFiles(ctx, message, files)</code></td>
<td>Multimodal with file attachments</td>
</tr>
<tr>
<td><code>Steer(ctx, instruction)</code></td>
<td>System-level steering without user message</td>
</tr>
<tr>
<td><code>FollowUp(ctx, text)</code></td>
<td>Continue without new user input</td>
</tr>
</tbody>
</table>
<h3 id="per-call-overrides"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#per-call-overrides"><span class="icon icon-link"></span></a>Per-call overrides</h3>
<p><code>PromptOptions</code> scopes configuration to a <strong>single call</strong> and restores the
agent's prior state afterwards — no need to rebuild a <code>*Kit</code> per request. This
suits multi-tenant hosts that resolve the model, credentials, or tool set per
request:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">result, err </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">PromptResultWithOptions</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(ctx, </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"Summarise this ticket"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">PromptOptions</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    SystemMessage:  </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"You are a concise triage assistant."</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#6A737D;--shiki-dark:#6A737D">// prepended for this call</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Model:          </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"anthropic/claude-haiku-3-5-20241022"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#6A737D;--shiki-dark:#6A737D">// overrides the default model</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    ThinkingLevel:  </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"low"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,                                 </span><span style="color:#6A737D;--shiki-dark:#6A737D">// "off" | "low" | "medium" | "high"</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    ExtraTools:     []</span><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Tool</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{lookupTool},                </span><span style="color:#6A737D;--shiki-dark:#6A737D">// added on top of the core set</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    ProviderURL:    </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"https://proxy.tenant-a/v1"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,           </span><span style="color:#6A737D;--shiki-dark:#6A737D">// per-tenant endpoint</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    ProviderAPIKey: tenantKey,                             </span><span style="color:#6A737D;--shiki-dark:#6A737D">// per-tenant credential</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span></code></pre>
<p>Every field is optional; a zero value means "use the agent's default." The
prior model, thinking level, provider credentials, and tool set are all
restored before the call returns, and concurrent option-driven prompts are
serialized so the apply/restore window of one call never races another.</p>
<h2 id="custom-tools"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#custom-tools"><span class="icon icon-link"></span></a>Custom tools</h2>
<p>Create custom tools with <code>kit.NewTool</code>. The JSON schema is auto-generated from the input struct — no external dependencies required:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">type</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> WeatherInput</span><span style="color:#D73A49;--shiki-dark:#F97583"> struct</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> {</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    City </span><span style="color:#D73A49;--shiki-dark:#F97583">string</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> \`json:"city" description:"City name"\`</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">}</span></span>
<span class="line"></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">weatherTool </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> kit.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">NewTool</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"get_weather"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"Get current weather for a city"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">    func</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#E36209;--shiki-dark:#FFAB70">ctx</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> context</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Context</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#E36209;--shiki-dark:#FFAB70">input</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> WeatherInput</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) (</span><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ToolOutput</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#D73A49;--shiki-dark:#F97583">error</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) {</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">        return</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> kit.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">TextResult</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"72°F, sunny in "</span><span style="color:#D73A49;--shiki-dark:#F97583"> +</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> input.City), </span><span style="color:#005CC5;--shiki-dark:#79B8FF">nil</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    },</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span>
<span class="line"></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">host, _ </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> kit.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">New</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(ctx, </span><span style="color:#D73A49;--shiki-dark:#F97583">&amp;</span><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Options</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    ExtraTools: []</span><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Tool</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{weatherTool},</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span></code></pre>
<p>Struct tags control the schema:</p>
<ul>
<li><code>json:"name"</code> — parameter name</li>
<li><code>description:"..."</code> — description shown to the LLM</li>
<li><code>enum:"a,b,c"</code> — restrict valid values</li>
<li><code>omitempty</code> — marks the parameter as optional</li>
</ul>
<p>Return values:</p>
<table>
<thead>
<tr>
<th>Helper</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td><code>kit.TextResult(s)</code></td>
<td>Successful text result</td>
</tr>
<tr>
<td><code>kit.ErrorResult(s)</code></td>
<td>Error result (LLM sees it as a tool error)</td>
</tr>
<tr>
<td><code>kit.ImageResult(s, data, mediaType)</code></td>
<td>Image result with binary data (e.g. <code>"image/png"</code>)</td>
</tr>
<tr>
<td><code>kit.MediaResult(s, data, mediaType)</code></td>
<td>Non-image media result (e.g. <code>"audio/mpeg"</code>)</td>
</tr>
</tbody>
</table>
<p>Binary data (images, audio, etc.) in <code>ToolOutput.Data</code> is automatically forwarded to the LLM when <code>MediaType</code> is set. For advanced use, return a <code>kit.ToolOutput</code> struct directly with <code>Data</code>, <code>MediaType</code>, and <code>Metadata</code> fields.</p>
<p>Use <code>kit.NewParallelTool</code> for tools that are safe to run concurrently. Use <code>kit.ToolCallIDFromContext(ctx)</code> to retrieve the LLM-assigned call ID for logging or tracing.</p>
<p><code>Options.ExtraTools</code> fixes the native tool set at construction time. To add or remove native tools on a live host, see <a href="#runtime-native-tools">Runtime native tools</a>.</p>
<h3 id="schema-driven-tools"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#schema-driven-tools"><span class="icon icon-link"></span></a>Schema-driven tools</h3>
<p>When the tool's input shape isn't known at compile time — tools sourced from
JSON Schema definitions in skill files, MCP server catalogs, or user-supplied
definitions — use <code>kit.NewRawTool</code>. It takes a JSON Schema and a handler that
receives the decoded arguments as a <code>map[string]any</code>, so no Go input type is
required:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">schema </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#D73A49;--shiki-dark:#F97583"> map</span><span style="color:#24292E;--shiki-dark:#E1E4E8">[</span><span style="color:#D73A49;--shiki-dark:#F97583">string</span><span style="color:#24292E;--shiki-dark:#E1E4E8">]</span><span style="color:#6F42C1;--shiki-dark:#B392F0">any</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span></span>
<span class="line"><span style="color:#032F62;--shiki-dark:#9ECBFF">    "type"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"object"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#032F62;--shiki-dark:#9ECBFF">    "properties"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#D73A49;--shiki-dark:#F97583">map</span><span style="color:#24292E;--shiki-dark:#E1E4E8">[</span><span style="color:#D73A49;--shiki-dark:#F97583">string</span><span style="color:#24292E;--shiki-dark:#E1E4E8">]</span><span style="color:#6F42C1;--shiki-dark:#B392F0">any</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span></span>
<span class="line"><span style="color:#032F62;--shiki-dark:#9ECBFF">        "city"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#D73A49;--shiki-dark:#F97583">map</span><span style="color:#24292E;--shiki-dark:#E1E4E8">[</span><span style="color:#D73A49;--shiki-dark:#F97583">string</span><span style="color:#24292E;--shiki-dark:#E1E4E8">]</span><span style="color:#6F42C1;--shiki-dark:#B392F0">any</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"type"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"string"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"description"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"City name"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">},</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    },</span></span>
<span class="line"><span style="color:#032F62;--shiki-dark:#9ECBFF">    "required"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: []</span><span style="color:#6F42C1;--shiki-dark:#B392F0">any</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"city"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">},</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">}</span></span>
<span class="line"></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">weatherTool </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> kit.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">NewRawTool</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"get_weather"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"Get current weather for a city"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, schema,</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">    func</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#E36209;--shiki-dark:#FFAB70">ctx</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> context</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Context</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#E36209;--shiki-dark:#FFAB70">args</span><span style="color:#D73A49;--shiki-dark:#F97583"> map</span><span style="color:#24292E;--shiki-dark:#E1E4E8">[</span><span style="color:#D73A49;--shiki-dark:#F97583">string</span><span style="color:#24292E;--shiki-dark:#E1E4E8">]</span><span style="color:#6F42C1;--shiki-dark:#B392F0">any</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) (</span><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ToolOutput</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#D73A49;--shiki-dark:#F97583">error</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) {</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">        return</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> kit.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">TextResult</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"72°F, sunny in "</span><span style="color:#D73A49;--shiki-dark:#F97583"> +</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> args[</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"city"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">].(</span><span style="color:#D73A49;--shiki-dark:#F97583">string</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)), </span><span style="color:#005CC5;--shiki-dark:#79B8FF">nil</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    },</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span></code></pre>
<p>The <code>schema</code> is advertised to the model as the tool's parameter schema. If the
model sends arguments that aren't a valid JSON object, the call short-circuits
with an error result before your handler runs.</p>
<h3 id="halting-the-agent-loop"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#halting-the-agent-loop"><span class="icon icon-link"></span></a>Halting the agent loop</h3>
<p>For structured-result patterns — the model calls a <code>finish(...)</code> tool with a
typed argument and the loop should terminate, returning that value to the
caller — set <code>Halt</code> and <code>FinalValue</code> on the returned <code>ToolOutput</code> instead of
smuggling the value out through a side-channel:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">finishTool </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> kit.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">NewTool</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"finish"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"Return the final structured answer"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">    func</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#E36209;--shiki-dark:#FFAB70">ctx</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> context</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Context</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#E36209;--shiki-dark:#FFAB70">input</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> AnswerInput</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) (</span><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ToolOutput</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#D73A49;--shiki-dark:#F97583">error</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) {</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">        return</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ToolOutput</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">            Content:    </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"done"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">            Halt:       </span><span style="color:#005CC5;--shiki-dark:#79B8FF">true</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,       </span><span style="color:#6A737D;--shiki-dark:#6A737D">// terminate the agent loop after this call</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">            FinalValue: input,      </span><span style="color:#6A737D;--shiki-dark:#6A737D">// surfaced to the caller</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">        }, </span><span style="color:#005CC5;--shiki-dark:#79B8FF">nil</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    },</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span>
<span class="line"></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">result, _ </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">PromptResult</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(ctx, </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"Extract the order details"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">if</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> result.HaltedByTool </span><span style="color:#D73A49;--shiki-dark:#F97583">==</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> "finish"</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> {</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    answer </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> result.FinalValue.(</span><span style="color:#6F42C1;--shiki-dark:#B392F0">AnswerInput</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) </span><span style="color:#6A737D;--shiki-dark:#6A737D">// the typed value your handler stored</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    _ </span><span style="color:#D73A49;--shiki-dark:#F97583">=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> answer</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">}</span></span></code></pre>
<p><code>TurnResult.HaltedByTool</code> names the tool that halted the turn (empty if the
turn ended for any other reason), and <code>TurnResult.FinalValue</code> carries whatever
your handler placed in <code>ToolOutput.FinalValue</code>. <code>Halt</code>/<code>FinalValue</code> work with
<code>NewTool</code>, <code>NewParallelTool</code>, and <code>NewRawTool</code> alike.</p>
<h2 id="generation--provider-overrides"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#generation--provider-overrides"><span class="icon icon-link"></span></a>Generation &amp; provider overrides</h2>
<p>SDK consumers can configure generation parameters and provider endpoints
entirely in-code via <code>Options</code>, without touching <code>.kit.yml</code> or <code>viper.Set()</code>:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">host, _ </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> kit.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">New</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(ctx, </span><span style="color:#D73A49;--shiki-dark:#F97583">&amp;</span><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Options</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Model:          </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"anthropic/claude-sonnet-4-5-20250929"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    MaxTokens:      </span><span style="color:#005CC5;--shiki-dark:#79B8FF">16384</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,             </span><span style="color:#6A737D;--shiki-dark:#6A737D">// 0 = auto-resolve (env → config → per-model → floor)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    ThinkingLevel:  </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"high"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,            </span><span style="color:#6A737D;--shiki-dark:#6A737D">// "off" | "none" | "minimal" | "low" | "medium" | "high"</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Temperature:    </span><span style="color:#6F42C1;--shiki-dark:#B392F0">ptrFloat32</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#005CC5;--shiki-dark:#79B8FF">0.2</span><span style="color:#24292E;--shiki-dark:#E1E4E8">),   </span><span style="color:#6A737D;--shiki-dark:#6A737D">// nil = provider/per-model default</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    ProviderAPIKey: os.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Getenv</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"MY_SECRET"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">), </span><span style="color:#6A737D;--shiki-dark:#6A737D">// overrides pre-existing viper state</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    ProviderURL:    </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"https://proxy.internal/v1"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span>
<span class="line"></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">func</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> ptrFloat32</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#E36209;--shiki-dark:#FFAB70">v</span><span style="color:#D73A49;--shiki-dark:#F97583"> float32</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) </span><span style="color:#D73A49;--shiki-dark:#F97583">*float32</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> { </span><span style="color:#D73A49;--shiki-dark:#F97583">return</span><span style="color:#D73A49;--shiki-dark:#F97583"> &amp;</span><span style="color:#24292E;--shiki-dark:#E1E4E8">v }</span></span></code></pre>
<p>See <a href="/sdk/options#generation-parameters">Options</a> for the full field reference,
including <code>TopP</code>, <code>TopK</code>, <code>FrequencyPenalty</code>, <code>PresencePenalty</code>, and <code>TLSSkipVerify</code>.</p>
<h2 id="event-system"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#event-system"><span class="icon icon-link"></span></a>Event system</h2>
<p>Subscribe to events for monitoring:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">unsubscribe </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">OnToolCall</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#D73A49;--shiki-dark:#F97583">func</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#E36209;--shiki-dark:#FFAB70">event</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ToolCallEvent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) {</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    fmt.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Println</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"Tool called:"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, event.Name)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">defer</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> unsubscribe</span><span style="color:#24292E;--shiki-dark:#E1E4E8">()</span></span>
<span class="line"></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">OnToolResult</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#D73A49;--shiki-dark:#F97583">func</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#E36209;--shiki-dark:#FFAB70">event</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ToolResultEvent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) {</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    fmt.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Println</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"Tool result:"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, event.Name)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span>
<span class="line"></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">OnMessageUpdate</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#D73A49;--shiki-dark:#F97583">func</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#E36209;--shiki-dark:#FFAB70">event</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">MessageUpdateEvent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) {</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    fmt.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Print</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(event.Chunk)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span></code></pre>
<h2 id="model-management"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#model-management"><span class="icon icon-link"></span></a>Model management</h2>
<p>Switch models at runtime:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">SetModel</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(ctx, </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"openai/gpt-4o"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">info </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">GetModelInfo</span><span style="color:#24292E;--shiki-dark:#E1E4E8">()</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">models </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">GetAvailableModels</span><span style="color:#24292E;--shiki-dark:#E1E4E8">()</span></span></code></pre>
<h2 id="dynamic-mcp-servers"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#dynamic-mcp-servers"><span class="icon icon-link"></span></a>Dynamic MCP servers</h2>
<p>Add and remove MCP servers at runtime:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">n, err </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">AddMCPServer</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(ctx, </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"github"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">MCPServerConfig</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Command: []</span><span style="color:#D73A49;--shiki-dark:#F97583">string</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"npx"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"-y"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"@modelcontextprotocol/server-github"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">},</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">fmt.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Printf</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"Loaded </span><span style="color:#005CC5;--shiki-dark:#79B8FF">%d</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> tools</span><span style="color:#005CC5;--shiki-dark:#79B8FF">\\n</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, n)</span></span>
<span class="line"></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">err </span><span style="color:#D73A49;--shiki-dark:#F97583">=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">RemoveMCPServer</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"github"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">servers </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ListMCPServers</span><span style="color:#24292E;--shiki-dark:#E1E4E8">() </span><span style="color:#6A737D;--shiki-dark:#6A737D">// []kit.MCPServerStatus</span></span></code></pre>
<h3 id="in-process-mcp-servers"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#in-process-mcp-servers"><span class="icon icon-link"></span></a>In-process MCP servers</h3>
<p>Register mcp-go servers running in the same process — zero subprocess overhead:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">import</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> (</span></span>
<span class="line"><span style="color:#032F62;--shiki-dark:#9ECBFF">    "</span><span style="color:#6F42C1;--shiki-dark:#B392F0">github.com/mark3labs/mcp-go/mcp</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"</span></span>
<span class="line"><span style="color:#032F62;--shiki-dark:#9ECBFF">    "</span><span style="color:#6F42C1;--shiki-dark:#B392F0">github.com/mark3labs/mcp-go/server</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span>
<span class="line"></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">mcpSrv </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> server.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">NewMCPServer</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"my-tools"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"1.0.0"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    server.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">WithToolCapabilities</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#005CC5;--shiki-dark:#79B8FF">true</span><span style="color:#24292E;--shiki-dark:#E1E4E8">),</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">mcpSrv.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">AddTool</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(mcp.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">NewTool</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"search_docs"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    mcp.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">WithDescription</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"Search documentation"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">),</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    mcp.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">WithString</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"query"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, mcp.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Required</span><span style="color:#24292E;--shiki-dark:#E1E4E8">()),</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">), searchHandler)</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// At init time</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">host, _ </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> kit.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">New</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(ctx, </span><span style="color:#D73A49;--shiki-dark:#F97583">&amp;</span><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Options</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    InProcessMCPServers: </span><span style="color:#D73A49;--shiki-dark:#F97583">map</span><span style="color:#24292E;--shiki-dark:#E1E4E8">[</span><span style="color:#D73A49;--shiki-dark:#F97583">string</span><span style="color:#24292E;--shiki-dark:#E1E4E8">]</span><span style="color:#D73A49;--shiki-dark:#F97583">*</span><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">MCPServer</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span></span>
<span class="line"><span style="color:#032F62;--shiki-dark:#9ECBFF">        "docs"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: mcpSrv,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    },</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// Or at runtime</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">n, _ </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">AddInProcessMCPServer</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(ctx, </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"docs"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, mcpSrv)</span></span></code></pre>
<h2 id="runtime-native-tools"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#runtime-native-tools"><span class="icon icon-link"></span></a>Runtime native tools</h2>
<p><code>Options.Tools</code> / <code>Options.ExtraTools</code> freeze the native Go tool set at
construction time. For progressive disclosure (loading a domain's tools only
when the model asks for them) or multi-tenant hosts that swap tool catalogs per
request, mutate the native tool set on a live host — mirroring the runtime MCP
and skill APIs. No host rebuild, so session history, MCP connections, and the
system-prompt snapshot all survive.</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">weatherTool </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> kit.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">NewTool</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"get_weather"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"Get current weather for a city"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">    func</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#E36209;--shiki-dark:#FFAB70">ctx</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> context</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Context</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#E36209;--shiki-dark:#FFAB70">input</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> WeatherInput</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) (</span><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ToolOutput</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#D73A49;--shiki-dark:#F97583">error</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) {</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">        return</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> kit.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">TextResult</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"72°F, sunny in "</span><span style="color:#D73A49;--shiki-dark:#F97583"> +</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> input.City), </span><span style="color:#005CC5;--shiki-dark:#79B8FF">nil</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    },</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// Add tools that persist for the session (visible on the next turn).</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">AddTools</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(weatherTool)</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// Drop tools by name when a domain is no longer needed.</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">if</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> err </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">RemoveTools</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"get_weather"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">); err </span><span style="color:#D73A49;--shiki-dark:#F97583">!=</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> nil</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> {</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    log.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Printf</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"remove tools: </span><span style="color:#005CC5;--shiki-dark:#79B8FF">%v</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, err)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">}</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// Replace the entire native extra-tool set in one call.</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">SetExtraTools</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(activeToolsForUser</span><span style="color:#D73A49;--shiki-dark:#F97583">...</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// Inspect the current set (snapshot copy — safe to mutate).</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">extra </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">GetExtraTools</span><span style="color:#24292E;--shiki-dark:#E1E4E8">()</span></span></code></pre>
<p>Key points:</p>
<ul>
<li><strong>Scope is <code>extraTools</code> only.</strong> These methods manage the same slice as
<code>Options.ExtraTools</code>. Core tools, MCP tools, and extension-registered tools are
never touched, and <code>GetExtraTools</code> excludes extension tools.</li>
<li><strong>Last-write-wins on name.</strong> <code>AddTools</code> replaces any existing extra tool that
shares a <code>Info().Name</code>, then appends the rest. Duplicate names within a single
call also resolve to the last one provided.</li>
<li><strong><code>RemoveTools</code> is atomic.</strong> If any supplied name is not currently registered,
it returns an error listing the missing names (deduped and sorted) and leaves
the tool set unchanged.</li>
<li><strong>Next-step visibility.</strong> Mutations apply from the next LLM step. If a turn is
in progress, the running step finishes with its existing tool set.</li>
<li><strong>Composes with per-call tools.</strong> <a href="#per-call-overrides"><code>PromptOptions.ExtraTools</code></a>
still layers on top for a single call and is reverted afterwards, snapshotting
around whatever persistent set is active.</li>
<li><strong>Thread safety.</strong> All four methods are safe to call concurrently; the
extra-tool state is guarded by an internal <code>RWMutex</code>.</li>
<li><strong>Not session-persisted.</strong> Native tool <em>definitions</em> are not serialized into
session state. Re-add them on session resume, just as with <code>Options.ExtraTools</code>.</li>
</ul>
<h2 id="runtime-skills-and-context-files"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#runtime-skills-and-context-files"><span class="icon icon-link"></span></a>Runtime skills and context files</h2>
<p>Kit auto-discovers skills and <code>AGENTS.md</code>-style context files during <code>New()</code>,
but multi-tenant hosts (chatbots, web services, per-user agents) often need
to swap these <strong>after</strong> construction. The runtime mutators below recompose
the system prompt and apply it to the agent so the next turn picks up the
updated instructions — no restart, no file shuffling.</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// Add a programmatic skill — no file on disk required.</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">AddSkill</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#D73A49;--shiki-dark:#F97583">&amp;</span><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Skill</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Name:        </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"polite-french"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Description: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"Respond in French and always greet the user."</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Content:     </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"Always reply in French. Open every response with 'Bonjour'."</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// Or load one from disk.</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">LoadAndAddSkill</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"/var/skills/refund-policy.md"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// Project context (AGENTS.md equivalents): inline content from a DB...</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">AddContextFileContent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    fmt.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Sprintf</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"session://</span><span style="color:#005CC5;--shiki-dark:#79B8FF">%s</span><span style="color:#032F62;--shiki-dark:#9ECBFF">/AGENTS.md"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, userID),</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    rulesFromDB,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// ...or load from disk.</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">LoadAndAddContextFile</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"/etc/agents/tenant-acme.md"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// Remove individually when a session ends.</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">RemoveSkill</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"polite-french"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">RemoveContextFile</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(fmt.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Sprintf</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"session://</span><span style="color:#005CC5;--shiki-dark:#79B8FF">%s</span><span style="color:#032F62;--shiki-dark:#9ECBFF">/AGENTS.md"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, userID))</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// Hide a skill from the model-facing catalog without unloading it — it stays</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// available for explicit /skill: activation. EnableSkill reverses this.</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">DisableSkill</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"refund-policy"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">EnableSkill</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"refund-policy"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// Or replace the whole set in one call.</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">SetSkills</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(activeSkillsForUser)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">SetContextFiles</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(activeContextForUser)</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// Inspect current state (snapshot copies — safe to mutate).</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">skills </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">GetSkills</span><span style="color:#24292E;--shiki-dark:#E1E4E8">()</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">ctxFiles </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">GetContextFiles</span><span style="color:#24292E;--shiki-dark:#E1E4E8">()</span></span></code></pre>
<p>Key points:</p>
<ul>
<li>
<p><strong>Auto-refresh.</strong> Every <code>Add*</code> / <code>Remove*</code> / <code>Set*</code> call recomposes the system
prompt against the captured base prompt (preserving per-model overrides and
<code>--system-prompt</code> resolution) and pushes the result onto the agent. Call
<code>host.RefreshSystemPrompt()</code> only if you mutate state through a different
path and need to force a re-render.</p>
</li>
<li>
<p><strong>Dedup keys.</strong> Skills dedupe by <code>Name</code>; context files dedupe by <code>Path</code>.
Re-adding the same key replaces the entry instead of appending a duplicate.</p>
</li>
<li>
<p><strong>Path is opaque.</strong> <code>ContextFile.Path</code> does not have to point at a real file
— it's only used for dedup and for the <code>Instructions from: &lt;Path&gt;</code> header
injected into the prompt. URIs like <code>session://user-123/AGENTS.md</code> work fine.</p>
</li>
<li>
<p><strong>Thread safety.</strong> All readers and mutators are safe to call concurrently
from multiple goroutines; the underlying state is guarded by an internal
<code>RWMutex</code>.</p>
</li>
<li>
<p><strong>Init-time options still apply.</strong> <code>Options.Skills</code>, <code>Options.SkillsDir</code>,
<code>Options.SkillsDisable</code>, <code>Options.SkillTrustPrompt</code>, <code>Options.NoSkills</code>, and
<code>Options.NoContextFiles</code> continue to control the startup set; the runtime API
mutates from whatever state <code>New()</code> produced.
See <a href="/sdk/options#skills--configuration">SDK options</a>.</p>
</li>
<li>
<p><strong>Auto-discovery scopes.</strong> When no explicit <code>Skills</code>/<code>SkillsDir</code> are given,
<code>New()</code> scans four <a href="https://agentskills.io/specification">agentskills.io</a>
locations: <code>~/.agents/skills/</code>, <code>~/.config/kit/skills/</code>,
<code>&lt;project&gt;/.agents/skills/</code>, and <code>&lt;project&gt;/.kit/skills/</code>. Project-level
skills override user-level skills of the same <code>name</code>. Skills missing a
<code>description</code> are skipped with a logged warning, and a skill's
<code>disable-model-invocation: true</code> (or <code>Options.SkillsDisable</code>) hides it from
the catalog while keeping it available for explicit activation.</p>
</li>
<li>
<p><strong>Skill helpers.</strong> A <code>kit.Skill</code> exposes <code>BaseDir()</code> (its directory) and
<code>Resources()</code> (the files bundled under <code>scripts/</code>, <code>references/</code>, and
<code>assets/</code>), which power the <code>&lt;skill_resources&gt;</code> enumeration shown when a skill
is activated.</p>
</li>
<li>
<p><strong><code>fs.FS</code>-backed discovery.</strong> The package-level loaders <code>kit.LoadSkill</code>,
<code>kit.LoadSkillsFromDir</code>, and <code>kit.LoadSkills</code> are path-string based;
<code>kit.LoadSkillsFromFS(fsys, root)</code> is the <code>fs.FS</code>-typed counterpart for
<code>embed.FS</code> distribution, <code>fstest.MapFS</code> tests, or per-tenant virtual
filesystems. Feed the result into <code>host.SetSkills(...)</code>:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">//go:embed skills</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">var</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> skillsFS </span><span style="color:#6F42C1;--shiki-dark:#B392F0">embed</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">FS</span></span>
<span class="line"></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">loaded, _ </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> kit.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">LoadSkillsFromFS</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(skillsFS, </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"skills"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">SetSkills</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(loaded)</span></span></code></pre>
</li>
</ul>
<h2 id="mcp-prompts-and-resources"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#mcp-prompts-and-resources"><span class="icon icon-link"></span></a>MCP prompts and resources</h2>
<p>Query prompts and resources exposed by connected MCP servers:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// List and expand prompts</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">prompts </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ListMCPPrompts</span><span style="color:#24292E;--shiki-dark:#E1E4E8">()</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">result, _ </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">GetMCPPrompt</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(ctx, </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"server"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"prompt-name"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#D73A49;--shiki-dark:#F97583">map</span><span style="color:#24292E;--shiki-dark:#E1E4E8">[</span><span style="color:#D73A49;--shiki-dark:#F97583">string</span><span style="color:#24292E;--shiki-dark:#E1E4E8">]</span><span style="color:#D73A49;--shiki-dark:#F97583">string</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"key"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"value"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// List and read resources</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">resources </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ListMCPResources</span><span style="color:#24292E;--shiki-dark:#E1E4E8">()</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">content, _ </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ReadMCPResource</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(ctx, </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"server"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"file:///path"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span></code></pre>
<h2 id="mcp-tasks-long-running-tools"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#mcp-tasks-long-running-tools"><span class="icon icon-link"></span></a>MCP tasks (long-running tools)</h2>
<p>Kit advertises <a href="https://modelcontextprotocol.io/specification/2025-11-25/basic/utilities/tasks">MCP task support</a>
during <code>initialize</code>, so cooperating servers can return a <code>taskId</code> immediately
and let Kit poll <code>tasks/get</code> / <code>tasks/result</code> until the operation completes.
This avoids HTTP/SSE proxy timeouts on long tools and gives you clean
cancellation via context.</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">host, _ </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> kit.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">New</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(ctx, </span><span style="color:#D73A49;--shiki-dark:#F97583">&amp;</span><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Options</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    MCPTaskMode: </span><span style="color:#D73A49;--shiki-dark:#F97583">map</span><span style="color:#24292E;--shiki-dark:#E1E4E8">[</span><span style="color:#D73A49;--shiki-dark:#F97583">string</span><span style="color:#24292E;--shiki-dark:#E1E4E8">]</span><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">MCPTaskMode</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span></span>
<span class="line"><span style="color:#032F62;--shiki-dark:#9ECBFF">        "build-server"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: kit.MCPTaskModeAlways,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    },</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    MCPTaskProgress: </span><span style="color:#D73A49;--shiki-dark:#F97583">func</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#E36209;--shiki-dark:#FFAB70">p</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">MCPTaskProgress</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) {</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">        log.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Printf</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"</span><span style="color:#005CC5;--shiki-dark:#79B8FF">%s</span><span style="color:#032F62;--shiki-dark:#9ECBFF">: </span><span style="color:#005CC5;--shiki-dark:#79B8FF">%s</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, p.TaskID, p.Status)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    },</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// Inspect / cancel in-flight tasks</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">tasks, _ </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ListMCPTasks</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(ctx, </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"build-server"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">_, _    </span><span style="color:#D73A49;--shiki-dark:#F97583">=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">CancelMCPTask</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(ctx, </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"build-server"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, tasks[</span><span style="color:#005CC5;--shiki-dark:#79B8FF">0</span><span style="color:#24292E;--shiki-dark:#E1E4E8">].TaskID)</span></span></code></pre>
<p>Defaults to <code>MCPTaskModeAuto</code> per server, so any existing MCP server keeps
its previous synchronous behaviour. See <a href="/sdk/options#mcp-tasks">SDK options → MCP Tasks</a>
for the full surface.</p>
<h2 id="context-and-compaction"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#context-and-compaction"><span class="icon icon-link"></span></a>Context and compaction</h2>
<p>Monitor and manage context usage:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">tokens </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">EstimateContextTokens</span><span style="color:#24292E;--shiki-dark:#E1E4E8">()</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">stats </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">GetContextStats</span><span style="color:#24292E;--shiki-dark:#E1E4E8">()</span></span>
<span class="line"></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">if</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ShouldCompact</span><span style="color:#24292E;--shiki-dark:#E1E4E8">() {</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    result, err </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Compact</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(ctx, </span><span style="color:#005CC5;--shiki-dark:#79B8FF">nil</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#032F62;--shiki-dark:#9ECBFF">""</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">}</span></span></code></pre>
<p>Token estimates count every message part (tool-call arguments, tool results,
reasoning, file attachments), and after the first turn the real API-reported
token count is preferred over the heuristic. Compaction budgets adapt to the
model's context and output limits when left at their zero values; pass a
<code>*kit.CompactionOptions</code> as the second argument to <code>Compact</code> to override
them — see <a href="/sdk/options#compactionoptions">SDK options → CompactionOptions</a>.</p>
<p>Estimates can still undercount. When a provider call fails with a
context-overflow error, the turn loop automatically compacts and replays the
turn once before surfacing <code>kit.ErrContextOverflow</code> — see
<a href="/sdk/options#reactive-compaction-on-context-overflow">Reactive compaction</a>.</p>
<h2 id="provider-error-classification"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#provider-error-classification"><span class="icon icon-link"></span></a>Provider error classification</h2>
<p>Provider failures are wrapped with exported sentinels so you can branch on the
failure category with <code>errors.Is</code> instead of string-matching the underlying
HTTP error. <code>PromptResult</code> / <code>Prompt</code> already return classified errors; you can
also classify any provider error yourself with <code>kit.ClassifyProviderError</code>:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">_, err </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">PromptResult</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(ctx, prompt)</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">switch</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> {</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">case</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> errors.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Is</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(err, kit.ErrContextOverflow):</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // Kit already compacted and replayed the turn once before surfacing</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // this — reaching here means the conversation cannot fit even after</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // compaction (e.g. start a new session or trim input).</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">    handleUnrecoverableOverflow</span><span style="color:#24292E;--shiki-dark:#E1E4E8">()</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">case</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> errors.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Is</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(err, kit.ErrRateLimit):</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">    backoffAndRetry</span><span style="color:#24292E;--shiki-dark:#E1E4E8">()</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">case</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> errors.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Is</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(err, kit.ErrAuth):</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">    rePromptForKey</span><span style="color:#24292E;--shiki-dark:#E1E4E8">()</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">case</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> errors.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Is</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(err, kit.ErrProviderUnavailable):</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">    retryLater</span><span style="color:#24292E;--shiki-dark:#E1E4E8">()</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">case</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> errors.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Is</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(err, kit.ErrInvalidRequest):</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    log.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Printf</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"non-retryable: </span><span style="color:#005CC5;--shiki-dark:#79B8FF">%v</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, err)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">}</span></span></code></pre>
<table>
<thead>
<tr>
<th>Sentinel</th>
<th>Meaning</th>
</tr>
</thead>
<tbody>
<tr>
<td><code>kit.ErrContextOverflow</code></td>
<td>Request exceeded the model's context window — surfaced only after Kit's automatic compact-and-replay recovery also failed</td>
</tr>
<tr>
<td><code>kit.ErrRateLimit</code></td>
<td>Provider throttled the request</td>
</tr>
<tr>
<td><code>kit.ErrAuth</code></td>
<td>Credential / authorization failure</td>
</tr>
<tr>
<td><code>kit.ErrProviderUnavailable</code></td>
<td>Transient upstream failure (5xx, network, timeout)</td>
</tr>
<tr>
<td><code>kit.ErrInvalidRequest</code></td>
<td>Structurally invalid request — retrying won't help</td>
</tr>
</tbody>
</table>
<p>The original error stays reachable via <code>errors.Is</code>, so you never lose the
provider's detail message.</p>
<h2 id="graceful-shutdown"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#graceful-shutdown"><span class="icon icon-link"></span></a>Graceful shutdown</h2>
<p><code>Close()</code> releases MCP connections, model resources, and the session file
handle. When shutdown must be bounded by a deadline, use <code>CloseContext</code>:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">shutdownCtx, cancel </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> context.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">WithTimeout</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(context.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Background</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(), </span><span style="color:#005CC5;--shiki-dark:#79B8FF">5</span><span style="color:#D73A49;--shiki-dark:#F97583">*</span><span style="color:#24292E;--shiki-dark:#E1E4E8">time.Second)</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">defer</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> cancel</span><span style="color:#24292E;--shiki-dark:#E1E4E8">()</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">if</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> err </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">CloseContext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(shutdownCtx); err </span><span style="color:#D73A49;--shiki-dark:#F97583">!=</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> nil</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> {</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    log.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Printf</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"shutdown: </span><span style="color:#005CC5;--shiki-dark:#79B8FF">%v</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, err)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">}</span></span></code></pre>
<p><code>Close()</code> is equivalent to <code>CloseContext(context.Background())</code>.</p>
<h2 id="in-process-subagents"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#in-process-subagents"><span class="icon icon-link"></span></a>In-process subagents</h2>
<p>Spawn child Kit instances without subprocess overhead:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">result, err </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Subagent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(ctx, </span><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">SubagentConfig</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Prompt:    </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"Analyze the test files"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Model:     </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"anthropic/claude-haiku-3-5-20241022"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    NoSession: </span><span style="color:#005CC5;--shiki-dark:#79B8FF">true</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Timeout:   </span><span style="color:#005CC5;--shiki-dark:#79B8FF">2</span><span style="color:#D73A49;--shiki-dark:#F97583"> *</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> time.Minute,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span></code></pre>
<p>Set <code>Agent</code> to a named agent definition (discovered from <code>.agents/agents/*.md</code>,
<code>.kit/agents/*.md</code>, <code>~/.config/kit/agents/*.md</code>, or the built-ins <code>general</code> /
<code>explore</code>) to apply its preset system prompt, model, tool allowlist, and
timeout:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">result, err </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Subagent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(ctx, </span><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">SubagentConfig</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Prompt: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"Map out the session persistence flow"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Agent:  </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"explore"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#6A737D;--shiki-dark:#6A737D">// read-only preset</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span></code></pre>
<p>Session-backed runs are linked to the parent: new child sessions record the
parent's session ID in their header, and <code>result.SessionID</code> can be passed back
as <code>SubagentConfig.SessionID</code> to resume the child session for follow-up
prompts that reuse its accumulated context.</p>
<p>See <a href="/advanced/subagents#named-agents">Subagents</a> for definition file format
and discovery precedence.</p>
<p>See <a href="/sdk/options">Options</a>, <a href="/sdk/callbacks">Callbacks</a>, and <a href="/sdk/sessions">Sessions</a> for more details.</p>`,headings:[{depth:2,text:"Installation",id:"installation"},{depth:2,text:"Basic usage",id:"basic-usage"},{depth:2,text:"Functional options (NewAgent)",id:"functional-options-newagent"},{depth:3,text:"When to use which",id:"when-to-use-which"},{depth:2,text:"Per-instance config isolation",id:"per-instance-config-isolation"},{depth:2,text:"Multi-turn conversations",id:"multi-turn-conversations"},{depth:2,text:"Additional prompt methods",id:"additional-prompt-methods"},{depth:3,text:"Per-call overrides",id:"per-call-overrides"},{depth:2,text:"Custom tools",id:"custom-tools"},{depth:3,text:"Schema-driven tools",id:"schema-driven-tools"},{depth:3,text:"Halting the agent loop",id:"halting-the-agent-loop"},{depth:2,text:"Generation &amp; provider overrides",id:"generation--provider-overrides"},{depth:2,text:"Event system",id:"event-system"},{depth:2,text:"Model management",id:"model-management"},{depth:2,text:"Dynamic MCP servers",id:"dynamic-mcp-servers"},{depth:3,text:"In-process MCP servers",id:"in-process-mcp-servers"},{depth:2,text:"Runtime native tools",id:"runtime-native-tools"},{depth:2,text:"Runtime skills and context files",id:"runtime-skills-and-context-files"},{depth:2,text:"MCP prompts and resources",id:"mcp-prompts-and-resources"},{depth:2,text:"MCP tasks (long-running tools)",id:"mcp-tasks-long-running-tools"},{depth:2,text:"Context and compaction",id:"context-and-compaction"},{depth:2,text:"Provider error classification",id:"provider-error-classification"},{depth:2,text:"Graceful shutdown",id:"graceful-shutdown"},{depth:2,text:"In-process subagents",id:"in-process-subagents"}],raw:`
# Go SDK

The \`pkg/kit\` package lets you embed Kit as a library in your Go applications.

## Installation

\`\`\`bash
go get github.com/mark3labs/kit/pkg/kit
\`\`\`

## Basic usage

\`\`\`go
package main

import (
    "context"
    "log"

    kit "github.com/mark3labs/kit/pkg/kit"
)

func main() {
    ctx := context.Background()

    // Create Kit instance with default configuration
    host, err := kit.New(ctx, nil)
    if err != nil {
        log.Fatal(err)
    }
    defer host.Close()

    // Send a prompt
    response, err := host.Prompt(ctx, "What is 2+2?")
    if err != nil {
        log.Fatal(err)
    }

    println(response)
}
\`\`\`

## Functional options (\`NewAgent\`)

For simple programmatic setups, \`kit.NewAgent\` offers an ergonomic
functional-options front door over \`kit.New\`. Streaming is **enabled by
default**; pass \`kit.WithStreaming(false)\` to opt out.

\`\`\`go
host, err := kit.NewAgent(ctx,
    kit.WithModel("anthropic/claude-sonnet-4-5-20250929"),
    kit.WithSystemPrompt("You are a helpful assistant."),
    kit.WithMaxTokens(8192),
    kit.WithThinkingLevel("medium"),
    kit.Ephemeral(), // in-memory session, no persistence
)
if err != nil {
    log.Fatal(err)
}
defer host.Close()
\`\`\`

Available options:

| Option | Sets |
|--------|------|
| \`WithModel(string)\` | \`Options.Model\` (provider/model format) |
| \`WithSystemPrompt(string)\` | \`Options.SystemPrompt\` (inline text or file path) |
| \`WithStreaming(bool)\` | \`Options.Streaming\` (default \`true\` under \`NewAgent\`) |
| \`WithMaxTokens(int)\` | \`Options.MaxTokens\` |
| \`WithThinkingLevel(string)\` | \`Options.ThinkingLevel\` |
| \`WithTools(...Tool)\` | \`Options.Tools\` (replaces the default set) |
| \`WithExtraTools(...Tool)\` | \`Options.ExtraTools\` (adds alongside defaults) |
| \`WithProviderAPIKey(string)\` | \`Options.ProviderAPIKey\` |
| \`WithProviderURL(string)\` | \`Options.ProviderURL\` |
| \`WithProviderWire(string)\` | \`Options.ProviderWire\` (wire protocol for auto-routed providers: \`openai\`, \`openai-compat\`, \`anthropic\`, \`google\`) |
| \`WithConfigFile(string)\` | \`Options.ConfigFile\` |
| \`WithDebug()\` | \`Options.Debug = true\` |
| \`WithDebugLogger(DebugLogger)\` | \`Options.DebugLogger\` (route engine + MCP debug output into a custom logger; overrides \`WithDebug\` when set) |
| \`Ephemeral()\` | \`Options.NoSession = true\` |

Options are applied in order, so later options override earlier ones. \`Option\`
is a plain \`func(*Options)\`, so you can define your own. For advanced
configuration not covered by the helpers (custom MCP config, in-process MCP
servers, session backends, MCP task tuning) construct an \`Options\` value
explicitly and call \`kit.New\`.

### When to use which

| Constructor | Use when |
|-------------|----------|
| \`kit.NewAgent(ctx, ...Option)\` | Quick programmatic setups; you only need the common fields. Streaming defaults on. |
| \`kit.New(ctx, *Options)\` | You need fields without a \`With*\` helper (\`MCPConfig\`, \`InProcessMCPServers\`, \`SessionManager\`, MCP task tuning, etc.), or you already hold an \`Options\` value. |

## Per-instance config isolation

Each \`kit.New\` / \`kit.NewAgent\` call owns an **isolated configuration store**,
so constructing multiple Kit instances in the same process is safe: setting the
model, thinking level, or generation parameters on one never affects another,
and runtime mutators (\`SetModel\`, \`SetThinkingLevel\`) only touch the owning
instance. This makes subagent spawning and multi-Kit embedding race-free with
no external synchronization required.

\`\`\`go
a, _ := kit.NewAgent(ctx, kit.WithThinkingLevel("low"))
b, _ := kit.NewAgent(ctx, kit.WithThinkingLevel("high"))

a.SetThinkingLevel(ctx, "medium")
// a.GetThinkingLevel() == "medium"; b.GetThinkingLevel() is still "high"
\`\`\`

## Multi-turn conversations

Conversations retain context automatically across calls:

\`\`\`go
host.Prompt(ctx, "My name is Alice")
response, _ := host.Prompt(ctx, "What's my name?")
// response: "Your name is Alice"
\`\`\`

## Additional prompt methods

The SDK provides several prompt variants:

| Method | Description |
|--------|-------------|
| \`Prompt(ctx, message)\` | Simple prompt, returns response string |
| \`PromptWithOptions(ctx, message, opts)\` | With per-call options (model, tools, thinking level, provider creds) |
| \`PromptResult(ctx, message)\` | Returns full \`TurnResult\` with usage stats |
| \`PromptResultWithOptions(ctx, message, opts)\` | Per-call options variant that returns the full \`TurnResult\` |
| \`PromptResultWithFiles(ctx, message, files)\` | Multimodal with file attachments |
| \`Steer(ctx, instruction)\` | System-level steering without user message |
| \`FollowUp(ctx, text)\` | Continue without new user input |

### Per-call overrides

\`PromptOptions\` scopes configuration to a **single call** and restores the
agent's prior state afterwards — no need to rebuild a \`*Kit\` per request. This
suits multi-tenant hosts that resolve the model, credentials, or tool set per
request:

\`\`\`go
result, err := host.PromptResultWithOptions(ctx, "Summarise this ticket", kit.PromptOptions{
    SystemMessage:  "You are a concise triage assistant.", // prepended for this call
    Model:          "anthropic/claude-haiku-3-5-20241022", // overrides the default model
    ThinkingLevel:  "low",                                 // "off" | "low" | "medium" | "high"
    ExtraTools:     []kit.Tool{lookupTool},                // added on top of the core set
    ProviderURL:    "https://proxy.tenant-a/v1",           // per-tenant endpoint
    ProviderAPIKey: tenantKey,                             // per-tenant credential
})
\`\`\`

Every field is optional; a zero value means "use the agent's default." The
prior model, thinking level, provider credentials, and tool set are all
restored before the call returns, and concurrent option-driven prompts are
serialized so the apply/restore window of one call never races another.

## Custom tools

Create custom tools with \`kit.NewTool\`. The JSON schema is auto-generated from the input struct — no external dependencies required:

\`\`\`go
type WeatherInput struct {
    City string \`json:"city" description:"City name"\`
}

weatherTool := kit.NewTool("get_weather", "Get current weather for a city",
    func(ctx context.Context, input WeatherInput) (kit.ToolOutput, error) {
        return kit.TextResult("72°F, sunny in " + input.City), nil
    },
)

host, _ := kit.New(ctx, &kit.Options{
    ExtraTools: []kit.Tool{weatherTool},
})
\`\`\`

Struct tags control the schema:

- \`json:"name"\` — parameter name
- \`description:"..."\` — description shown to the LLM
- \`enum:"a,b,c"\` — restrict valid values
- \`omitempty\` — marks the parameter as optional

Return values:

| Helper | Description |
|--------|-------------|
| \`kit.TextResult(s)\` | Successful text result |
| \`kit.ErrorResult(s)\` | Error result (LLM sees it as a tool error) |
| \`kit.ImageResult(s, data, mediaType)\` | Image result with binary data (e.g. \`"image/png"\`) |
| \`kit.MediaResult(s, data, mediaType)\` | Non-image media result (e.g. \`"audio/mpeg"\`) |

Binary data (images, audio, etc.) in \`ToolOutput.Data\` is automatically forwarded to the LLM when \`MediaType\` is set. For advanced use, return a \`kit.ToolOutput\` struct directly with \`Data\`, \`MediaType\`, and \`Metadata\` fields.

Use \`kit.NewParallelTool\` for tools that are safe to run concurrently. Use \`kit.ToolCallIDFromContext(ctx)\` to retrieve the LLM-assigned call ID for logging or tracing.

\`Options.ExtraTools\` fixes the native tool set at construction time. To add or remove native tools on a live host, see [Runtime native tools](#runtime-native-tools).

### Schema-driven tools

When the tool's input shape isn't known at compile time — tools sourced from
JSON Schema definitions in skill files, MCP server catalogs, or user-supplied
definitions — use \`kit.NewRawTool\`. It takes a JSON Schema and a handler that
receives the decoded arguments as a \`map[string]any\`, so no Go input type is
required:

\`\`\`go
schema := map[string]any{
    "type": "object",
    "properties": map[string]any{
        "city": map[string]any{"type": "string", "description": "City name"},
    },
    "required": []any{"city"},
}

weatherTool := kit.NewRawTool("get_weather", "Get current weather for a city", schema,
    func(ctx context.Context, args map[string]any) (kit.ToolOutput, error) {
        return kit.TextResult("72°F, sunny in " + args["city"].(string)), nil
    },
)
\`\`\`

The \`schema\` is advertised to the model as the tool's parameter schema. If the
model sends arguments that aren't a valid JSON object, the call short-circuits
with an error result before your handler runs.

### Halting the agent loop

For structured-result patterns — the model calls a \`finish(...)\` tool with a
typed argument and the loop should terminate, returning that value to the
caller — set \`Halt\` and \`FinalValue\` on the returned \`ToolOutput\` instead of
smuggling the value out through a side-channel:

\`\`\`go
finishTool := kit.NewTool("finish", "Return the final structured answer",
    func(ctx context.Context, input AnswerInput) (kit.ToolOutput, error) {
        return kit.ToolOutput{
            Content:    "done",
            Halt:       true,       // terminate the agent loop after this call
            FinalValue: input,      // surfaced to the caller
        }, nil
    },
)

result, _ := host.PromptResult(ctx, "Extract the order details")
if result.HaltedByTool == "finish" {
    answer := result.FinalValue.(AnswerInput) // the typed value your handler stored
    _ = answer
}
\`\`\`

\`TurnResult.HaltedByTool\` names the tool that halted the turn (empty if the
turn ended for any other reason), and \`TurnResult.FinalValue\` carries whatever
your handler placed in \`ToolOutput.FinalValue\`. \`Halt\`/\`FinalValue\` work with
\`NewTool\`, \`NewParallelTool\`, and \`NewRawTool\` alike.

## Generation & provider overrides

SDK consumers can configure generation parameters and provider endpoints
entirely in-code via \`Options\`, without touching \`.kit.yml\` or \`viper.Set()\`:

\`\`\`go
host, _ := kit.New(ctx, &kit.Options{
    Model:          "anthropic/claude-sonnet-4-5-20250929",
    MaxTokens:      16384,             // 0 = auto-resolve (env → config → per-model → floor)
    ThinkingLevel:  "high",            // "off" | "none" | "minimal" | "low" | "medium" | "high"
    Temperature:    ptrFloat32(0.2),   // nil = provider/per-model default
    ProviderAPIKey: os.Getenv("MY_SECRET"), // overrides pre-existing viper state
    ProviderURL:    "https://proxy.internal/v1",
})

func ptrFloat32(v float32) *float32 { return &v }
\`\`\`

See [Options](/sdk/options#generation-parameters) for the full field reference,
including \`TopP\`, \`TopK\`, \`FrequencyPenalty\`, \`PresencePenalty\`, and \`TLSSkipVerify\`.

## Event system

Subscribe to events for monitoring:

\`\`\`go
unsubscribe := host.OnToolCall(func(event kit.ToolCallEvent) {
    fmt.Println("Tool called:", event.Name)
})
defer unsubscribe()

host.OnToolResult(func(event kit.ToolResultEvent) {
    fmt.Println("Tool result:", event.Name)
})

host.OnMessageUpdate(func(event kit.MessageUpdateEvent) {
    fmt.Print(event.Chunk)
})
\`\`\`

## Model management

Switch models at runtime:

\`\`\`go
host.SetModel(ctx, "openai/gpt-4o")
info := host.GetModelInfo()
models := host.GetAvailableModels()
\`\`\`

## Dynamic MCP servers

Add and remove MCP servers at runtime:

\`\`\`go
n, err := host.AddMCPServer(ctx, "github", kit.MCPServerConfig{
    Command: []string{"npx", "-y", "@modelcontextprotocol/server-github"},
})
fmt.Printf("Loaded %d tools\\n", n)

err = host.RemoveMCPServer("github")
servers := host.ListMCPServers() // []kit.MCPServerStatus
\`\`\`

### In-process MCP servers

Register mcp-go servers running in the same process — zero subprocess overhead:

\`\`\`go
import (
    "github.com/mark3labs/mcp-go/mcp"
    "github.com/mark3labs/mcp-go/server"
)

mcpSrv := server.NewMCPServer("my-tools", "1.0.0",
    server.WithToolCapabilities(true),
)
mcpSrv.AddTool(mcp.NewTool("search_docs",
    mcp.WithDescription("Search documentation"),
    mcp.WithString("query", mcp.Required()),
), searchHandler)

// At init time
host, _ := kit.New(ctx, &kit.Options{
    InProcessMCPServers: map[string]*kit.MCPServer{
        "docs": mcpSrv,
    },
})

// Or at runtime
n, _ := host.AddInProcessMCPServer(ctx, "docs", mcpSrv)
\`\`\`

## Runtime native tools

\`Options.Tools\` / \`Options.ExtraTools\` freeze the native Go tool set at
construction time. For progressive disclosure (loading a domain's tools only
when the model asks for them) or multi-tenant hosts that swap tool catalogs per
request, mutate the native tool set on a live host — mirroring the runtime MCP
and skill APIs. No host rebuild, so session history, MCP connections, and the
system-prompt snapshot all survive.

\`\`\`go
weatherTool := kit.NewTool("get_weather", "Get current weather for a city",
    func(ctx context.Context, input WeatherInput) (kit.ToolOutput, error) {
        return kit.TextResult("72°F, sunny in " + input.City), nil
    },
)

// Add tools that persist for the session (visible on the next turn).
host.AddTools(weatherTool)

// Drop tools by name when a domain is no longer needed.
if err := host.RemoveTools("get_weather"); err != nil {
    log.Printf("remove tools: %v", err)
}

// Replace the entire native extra-tool set in one call.
host.SetExtraTools(activeToolsForUser...)

// Inspect the current set (snapshot copy — safe to mutate).
extra := host.GetExtraTools()
\`\`\`

Key points:

- **Scope is \`extraTools\` only.** These methods manage the same slice as
  \`Options.ExtraTools\`. Core tools, MCP tools, and extension-registered tools are
  never touched, and \`GetExtraTools\` excludes extension tools.
- **Last-write-wins on name.** \`AddTools\` replaces any existing extra tool that
  shares a \`Info().Name\`, then appends the rest. Duplicate names within a single
  call also resolve to the last one provided.
- **\`RemoveTools\` is atomic.** If any supplied name is not currently registered,
  it returns an error listing the missing names (deduped and sorted) and leaves
  the tool set unchanged.
- **Next-step visibility.** Mutations apply from the next LLM step. If a turn is
  in progress, the running step finishes with its existing tool set.
- **Composes with per-call tools.** [\`PromptOptions.ExtraTools\`](#per-call-overrides)
  still layers on top for a single call and is reverted afterwards, snapshotting
  around whatever persistent set is active.
- **Thread safety.** All four methods are safe to call concurrently; the
  extra-tool state is guarded by an internal \`RWMutex\`.
- **Not session-persisted.** Native tool *definitions* are not serialized into
  session state. Re-add them on session resume, just as with \`Options.ExtraTools\`.

## Runtime skills and context files

Kit auto-discovers skills and \`AGENTS.md\`-style context files during \`New()\`,
but multi-tenant hosts (chatbots, web services, per-user agents) often need
to swap these **after** construction. The runtime mutators below recompose
the system prompt and apply it to the agent so the next turn picks up the
updated instructions — no restart, no file shuffling.

\`\`\`go
// Add a programmatic skill — no file on disk required.
host.AddSkill(&kit.Skill{
    Name:        "polite-french",
    Description: "Respond in French and always greet the user.",
    Content:     "Always reply in French. Open every response with 'Bonjour'.",
})

// Or load one from disk.
host.LoadAndAddSkill("/var/skills/refund-policy.md")

// Project context (AGENTS.md equivalents): inline content from a DB...
host.AddContextFileContent(
    fmt.Sprintf("session://%s/AGENTS.md", userID),
    rulesFromDB,
)
// ...or load from disk.
host.LoadAndAddContextFile("/etc/agents/tenant-acme.md")

// Remove individually when a session ends.
host.RemoveSkill("polite-french")
host.RemoveContextFile(fmt.Sprintf("session://%s/AGENTS.md", userID))

// Hide a skill from the model-facing catalog without unloading it — it stays
// available for explicit /skill: activation. EnableSkill reverses this.
host.DisableSkill("refund-policy")
host.EnableSkill("refund-policy")

// Or replace the whole set in one call.
host.SetSkills(activeSkillsForUser)
host.SetContextFiles(activeContextForUser)

// Inspect current state (snapshot copies — safe to mutate).
skills := host.GetSkills()
ctxFiles := host.GetContextFiles()
\`\`\`

Key points:

- **Auto-refresh.** Every \`Add*\` / \`Remove*\` / \`Set*\` call recomposes the system
  prompt against the captured base prompt (preserving per-model overrides and
  \`--system-prompt\` resolution) and pushes the result onto the agent. Call
  \`host.RefreshSystemPrompt()\` only if you mutate state through a different
  path and need to force a re-render.
- **Dedup keys.** Skills dedupe by \`Name\`; context files dedupe by \`Path\`.
  Re-adding the same key replaces the entry instead of appending a duplicate.
- **Path is opaque.** \`ContextFile.Path\` does not have to point at a real file
  — it's only used for dedup and for the \`Instructions from: <Path>\` header
  injected into the prompt. URIs like \`session://user-123/AGENTS.md\` work fine.
- **Thread safety.** All readers and mutators are safe to call concurrently
  from multiple goroutines; the underlying state is guarded by an internal
  \`RWMutex\`.
- **Init-time options still apply.** \`Options.Skills\`, \`Options.SkillsDir\`,
  \`Options.SkillsDisable\`, \`Options.SkillTrustPrompt\`, \`Options.NoSkills\`, and
  \`Options.NoContextFiles\` continue to control the startup set; the runtime API
  mutates from whatever state \`New()\` produced.
  See [SDK options](/sdk/options#skills--configuration).
- **Auto-discovery scopes.** When no explicit \`Skills\`/\`SkillsDir\` are given,
  \`New()\` scans four [agentskills.io](https://agentskills.io/specification)
  locations: \`~/.agents/skills/\`, \`~/.config/kit/skills/\`,
  \`<project>/.agents/skills/\`, and \`<project>/.kit/skills/\`. Project-level
  skills override user-level skills of the same \`name\`. Skills missing a
  \`description\` are skipped with a logged warning, and a skill's
  \`disable-model-invocation: true\` (or \`Options.SkillsDisable\`) hides it from
  the catalog while keeping it available for explicit activation.
- **Skill helpers.** A \`kit.Skill\` exposes \`BaseDir()\` (its directory) and
  \`Resources()\` (the files bundled under \`scripts/\`, \`references/\`, and
  \`assets/\`), which power the \`<skill_resources>\` enumeration shown when a skill
  is activated.
- **\`fs.FS\`-backed discovery.** The package-level loaders \`kit.LoadSkill\`,
  \`kit.LoadSkillsFromDir\`, and \`kit.LoadSkills\` are path-string based;
  \`kit.LoadSkillsFromFS(fsys, root)\` is the \`fs.FS\`-typed counterpart for
  \`embed.FS\` distribution, \`fstest.MapFS\` tests, or per-tenant virtual
  filesystems. Feed the result into \`host.SetSkills(...)\`:

  \`\`\`go
  //go:embed skills
  var skillsFS embed.FS

  loaded, _ := kit.LoadSkillsFromFS(skillsFS, "skills")
  host.SetSkills(loaded)
  \`\`\`

## MCP prompts and resources

Query prompts and resources exposed by connected MCP servers:

\`\`\`go
// List and expand prompts
prompts := host.ListMCPPrompts()
result, _ := host.GetMCPPrompt(ctx, "server", "prompt-name", map[string]string{"key": "value"})

// List and read resources
resources := host.ListMCPResources()
content, _ := host.ReadMCPResource(ctx, "server", "file:///path")
\`\`\`

## MCP tasks (long-running tools)

Kit advertises [MCP task support](https://modelcontextprotocol.io/specification/2025-11-25/basic/utilities/tasks)
during \`initialize\`, so cooperating servers can return a \`taskId\` immediately
and let Kit poll \`tasks/get\` / \`tasks/result\` until the operation completes.
This avoids HTTP/SSE proxy timeouts on long tools and gives you clean
cancellation via context.

\`\`\`go
host, _ := kit.New(ctx, &kit.Options{
    MCPTaskMode: map[string]kit.MCPTaskMode{
        "build-server": kit.MCPTaskModeAlways,
    },
    MCPTaskProgress: func(p kit.MCPTaskProgress) {
        log.Printf("%s: %s", p.TaskID, p.Status)
    },
})

// Inspect / cancel in-flight tasks
tasks, _ := host.ListMCPTasks(ctx, "build-server")
_, _    = host.CancelMCPTask(ctx, "build-server", tasks[0].TaskID)
\`\`\`

Defaults to \`MCPTaskModeAuto\` per server, so any existing MCP server keeps
its previous synchronous behaviour. See [SDK options → MCP Tasks](/sdk/options#mcp-tasks)
for the full surface.

## Context and compaction

Monitor and manage context usage:

\`\`\`go
tokens := host.EstimateContextTokens()
stats := host.GetContextStats()

if host.ShouldCompact() {
    result, err := host.Compact(ctx, nil, "")
}
\`\`\`

Token estimates count every message part (tool-call arguments, tool results,
reasoning, file attachments), and after the first turn the real API-reported
token count is preferred over the heuristic. Compaction budgets adapt to the
model's context and output limits when left at their zero values; pass a
\`*kit.CompactionOptions\` as the second argument to \`Compact\` to override
them — see [SDK options → CompactionOptions](/sdk/options#compactionoptions).

Estimates can still undercount. When a provider call fails with a
context-overflow error, the turn loop automatically compacts and replays the
turn once before surfacing \`kit.ErrContextOverflow\` — see
[Reactive compaction](/sdk/options#reactive-compaction-on-context-overflow).

## Provider error classification

Provider failures are wrapped with exported sentinels so you can branch on the
failure category with \`errors.Is\` instead of string-matching the underlying
HTTP error. \`PromptResult\` / \`Prompt\` already return classified errors; you can
also classify any provider error yourself with \`kit.ClassifyProviderError\`:

\`\`\`go
_, err := host.PromptResult(ctx, prompt)
switch {
case errors.Is(err, kit.ErrContextOverflow):
    // Kit already compacted and replayed the turn once before surfacing
    // this — reaching here means the conversation cannot fit even after
    // compaction (e.g. start a new session or trim input).
    handleUnrecoverableOverflow()
case errors.Is(err, kit.ErrRateLimit):
    backoffAndRetry()
case errors.Is(err, kit.ErrAuth):
    rePromptForKey()
case errors.Is(err, kit.ErrProviderUnavailable):
    retryLater()
case errors.Is(err, kit.ErrInvalidRequest):
    log.Printf("non-retryable: %v", err)
}
\`\`\`

| Sentinel | Meaning |
|----------|---------|
| \`kit.ErrContextOverflow\` | Request exceeded the model's context window — surfaced only after Kit's automatic compact-and-replay recovery also failed |
| \`kit.ErrRateLimit\` | Provider throttled the request |
| \`kit.ErrAuth\` | Credential / authorization failure |
| \`kit.ErrProviderUnavailable\` | Transient upstream failure (5xx, network, timeout) |
| \`kit.ErrInvalidRequest\` | Structurally invalid request — retrying won't help |

The original error stays reachable via \`errors.Is\`, so you never lose the
provider's detail message.

## Graceful shutdown

\`Close()\` releases MCP connections, model resources, and the session file
handle. When shutdown must be bounded by a deadline, use \`CloseContext\`:

\`\`\`go
shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()
if err := host.CloseContext(shutdownCtx); err != nil {
    log.Printf("shutdown: %v", err)
}
\`\`\`

\`Close()\` is equivalent to \`CloseContext(context.Background())\`.

## In-process subagents

Spawn child Kit instances without subprocess overhead:

\`\`\`go
result, err := host.Subagent(ctx, kit.SubagentConfig{
    Prompt:    "Analyze the test files",
    Model:     "anthropic/claude-haiku-3-5-20241022",
    NoSession: true,
    Timeout:   2 * time.Minute,
})
\`\`\`

Set \`Agent\` to a named agent definition (discovered from \`.agents/agents/*.md\`,
\`.kit/agents/*.md\`, \`~/.config/kit/agents/*.md\`, or the built-ins \`general\` /
\`explore\`) to apply its preset system prompt, model, tool allowlist, and
timeout:

\`\`\`go
result, err := host.Subagent(ctx, kit.SubagentConfig{
    Prompt: "Map out the session persistence flow",
    Agent:  "explore", // read-only preset
})
\`\`\`

Session-backed runs are linked to the parent: new child sessions record the
parent's session ID in their header, and \`result.SessionID\` can be passed back
as \`SubagentConfig.SessionID\` to resume the child session for follow-up
prompts that reuse its accumulated context.

See [Subagents](/advanced/subagents#named-agents) for definition file format
and discovery precedence.

See [Options](/sdk/options), [Callbacks](/sdk/callbacks), and [Sessions](/sdk/sessions) for more details.
`};export{s as default};

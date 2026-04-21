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
<td>With per-call options</td>
</tr>
<tr>
<td><code>PromptResult(ctx, message)</code></td>
<td>Returns full <code>TurnResult</code> with usage stats</td>
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
</tbody>
</table>
<p>For advanced use, return a <code>kit.ToolOutput</code> struct directly with <code>Data</code>, <code>MediaType</code>, and <code>Metadata</code> fields.</p>
<p>Use <code>kit.NewParallelTool</code> for tools that are safe to run concurrently. Use <code>kit.ToolCallIDFromContext(ctx)</code> to retrieve the LLM-assigned call ID for logging or tracing.</p>
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
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">OnStreaming</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#D73A49;--shiki-dark:#F97583">func</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#E36209;--shiki-dark:#FFAB70">event</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">MessageUpdateEvent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) {</span></span>
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
<h2 id="mcp-prompts-and-resources"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#mcp-prompts-and-resources"><span class="icon icon-link"></span></a>MCP prompts and resources</h2>
<p>Query prompts and resources exposed by connected MCP servers:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// List and expand prompts</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">prompts </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ListMCPPrompts</span><span style="color:#24292E;--shiki-dark:#E1E4E8">()</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">result, _ </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">GetMCPPrompt</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(ctx, </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"server"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"prompt-name"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#D73A49;--shiki-dark:#F97583">map</span><span style="color:#24292E;--shiki-dark:#E1E4E8">[</span><span style="color:#D73A49;--shiki-dark:#F97583">string</span><span style="color:#24292E;--shiki-dark:#E1E4E8">]</span><span style="color:#D73A49;--shiki-dark:#F97583">string</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"key"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"value"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// List and read resources</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">resources </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ListMCPResources</span><span style="color:#24292E;--shiki-dark:#E1E4E8">()</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">content, _ </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ReadMCPResource</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(ctx, </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"server"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"file:///path"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span></code></pre>
<h2 id="context-and-compaction"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#context-and-compaction"><span class="icon icon-link"></span></a>Context and compaction</h2>
<p>Monitor and manage context usage:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">tokens </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">EstimateContextTokens</span><span style="color:#24292E;--shiki-dark:#E1E4E8">()</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">stats </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">GetContextStats</span><span style="color:#24292E;--shiki-dark:#E1E4E8">()</span></span>
<span class="line"></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">if</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ShouldCompact</span><span style="color:#24292E;--shiki-dark:#E1E4E8">() {</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    result, err </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Compact</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(ctx, </span><span style="color:#005CC5;--shiki-dark:#79B8FF">nil</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#032F62;--shiki-dark:#9ECBFF">""</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">}</span></span></code></pre>
<h2 id="in-process-subagents"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#in-process-subagents"><span class="icon icon-link"></span></a>In-process subagents</h2>
<p>Spawn child Kit instances without subprocess overhead:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">result, err </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Subagent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(ctx, </span><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">SubagentConfig</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Prompt:    </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"Analyze the test files"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Model:     </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"anthropic/claude-haiku-3-5-20241022"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    NoSession: </span><span style="color:#005CC5;--shiki-dark:#79B8FF">true</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Timeout:   </span><span style="color:#005CC5;--shiki-dark:#79B8FF">2</span><span style="color:#D73A49;--shiki-dark:#F97583"> *</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> time.Minute,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span></code></pre>
<p>See <a href="/sdk/options">Options</a>, <a href="/sdk/callbacks">Callbacks</a>, and <a href="/sdk/sessions">Sessions</a> for more details.</p>`,headings:[{depth:2,text:"Installation",id:"installation"},{depth:2,text:"Basic usage",id:"basic-usage"},{depth:2,text:"Multi-turn conversations",id:"multi-turn-conversations"},{depth:2,text:"Additional prompt methods",id:"additional-prompt-methods"},{depth:2,text:"Custom tools",id:"custom-tools"},{depth:2,text:"Generation &amp; provider overrides",id:"generation--provider-overrides"},{depth:2,text:"Event system",id:"event-system"},{depth:2,text:"Model management",id:"model-management"},{depth:2,text:"Dynamic MCP servers",id:"dynamic-mcp-servers"},{depth:3,text:"In-process MCP servers",id:"in-process-mcp-servers"},{depth:2,text:"MCP prompts and resources",id:"mcp-prompts-and-resources"},{depth:2,text:"Context and compaction",id:"context-and-compaction"},{depth:2,text:"In-process subagents",id:"in-process-subagents"}],raw:`
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
| \`PromptWithOptions(ctx, message, opts)\` | With per-call options |
| \`PromptResult(ctx, message)\` | Returns full \`TurnResult\` with usage stats |
| \`PromptResultWithFiles(ctx, message, files)\` | Multimodal with file attachments |
| \`Steer(ctx, instruction)\` | System-level steering without user message |
| \`FollowUp(ctx, text)\` | Continue without new user input |

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

For advanced use, return a \`kit.ToolOutput\` struct directly with \`Data\`, \`MediaType\`, and \`Metadata\` fields.

Use \`kit.NewParallelTool\` for tools that are safe to run concurrently. Use \`kit.ToolCallIDFromContext(ctx)\` to retrieve the LLM-assigned call ID for logging or tracing.

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

host.OnStreaming(func(event kit.MessageUpdateEvent) {
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

## Context and compaction

Monitor and manage context usage:

\`\`\`go
tokens := host.EstimateContextTokens()
stats := host.GetContextStats()

if host.ShouldCompact() {
    result, err := host.Compact(ctx, nil, "")
}
\`\`\`

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

See [Options](/sdk/options), [Callbacks](/sdk/callbacks), and [Sessions](/sdk/sessions) for more details.
`};export{s as default};

const s={frontmatter:{title:"Capabilities",description:"All extension capabilities — lifecycle events, tools, commands, widgets, and more.",hidden:!1,toc:!0,draft:!1},html:`<h1 id="extension-capabilities"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#extension-capabilities"><span class="icon icon-link"></span></a>Extension Capabilities</h1>
<h2 id="lifecycle-events"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#lifecycle-events"><span class="icon icon-link"></span></a>Lifecycle events</h2>
<p>Extensions can hook into 30 lifecycle events:</p>
<table>
<thead>
<tr>
<th>Event</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td><code>OnSessionStart</code></td>
<td>Session initialized</td>
</tr>
<tr>
<td><code>OnSessionShutdown</code></td>
<td>Session ending</td>
</tr>
<tr>
<td><code>OnBeforeAgentStart</code></td>
<td>Before the agent loop begins</td>
</tr>
<tr>
<td><code>OnAgentStart</code></td>
<td>Agent loop started</td>
</tr>
<tr>
<td><code>OnAgentEnd</code></td>
<td>Agent loop completed (carries per-turn aggregates: tool counts, token deltas, cost, duration)</td>
</tr>
<tr>
<td><code>OnLLMUsage</code></td>
<td>Per-LLM-call token + cost delta (fires once per provider round-trip)</td>
</tr>
<tr>
<td><code>OnToolCall</code></td>
<td>Tool call requested by the model</td>
</tr>
<tr>
<td><code>OnToolCallInputStart</code></td>
<td>LLM began generating tool call arguments (tool name known, args streaming)</td>
</tr>
<tr>
<td><code>OnToolCallInputDelta</code></td>
<td>Streamed JSON fragment of tool call arguments</td>
</tr>
<tr>
<td><code>OnToolCallInputEnd</code></td>
<td>Tool argument streaming complete, before execution begins</td>
</tr>
<tr>
<td><code>OnToolExecutionStart</code></td>
<td>Tool execution beginning</td>
</tr>
<tr>
<td><code>OnToolOutput</code></td>
<td>Streaming tool output chunk (for long-running tools)</td>
</tr>
<tr>
<td><code>OnToolExecutionEnd</code></td>
<td>Tool execution completed</td>
</tr>
<tr>
<td><code>OnToolResult</code></td>
<td>Tool result returned</td>
</tr>
<tr>
<td><code>OnInput</code></td>
<td>User input received</td>
</tr>
<tr>
<td><code>OnMessageStart</code></td>
<td>Assistant message started</td>
</tr>
<tr>
<td><code>OnMessageUpdate</code></td>
<td>Streaming text chunk received</td>
</tr>
<tr>
<td><code>OnMessageEnd</code></td>
<td>Assistant message completed</td>
</tr>
<tr>
<td><code>OnModelChange</code></td>
<td>Model switched</td>
</tr>
<tr>
<td><code>OnThinkingLevelChange</code></td>
<td>Extended-thinking effort level changed</td>
</tr>
<tr>
<td><code>OnTerminalResize</code></td>
<td>Terminal resized (also fires once at startup)</td>
</tr>
<tr>
<td><code>OnTurnStateChange</code></td>
<td>UI entered or left the working state</td>
</tr>
<tr>
<td><code>OnContextPrepare</code></td>
<td>Context being assembled for the model</td>
</tr>
<tr>
<td><code>OnBeforeFork</code></td>
<td>Before forking a conversation branch</td>
</tr>
<tr>
<td><code>OnBeforeSessionSwitch</code></td>
<td>Before switching sessions</td>
</tr>
<tr>
<td><code>OnBeforeCompact</code></td>
<td>Before conversation compaction</td>
</tr>
<tr>
<td><code>OnCustomEvent</code></td>
<td>Custom inter-extension event received</td>
</tr>
<tr>
<td><code>OnSubagentStart</code></td>
<td>Subagent spawned by the main agent</td>
</tr>
<tr>
<td><code>OnSubagentChunk</code></td>
<td>Real-time output from subagent (text, tool calls, results)</td>
</tr>
<tr>
<td><code>OnSubagentEnd</code></td>
<td>Subagent completed with final response/error</td>
</tr>
</tbody>
</table>
<h3 id="example"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#example"><span class="icon icon-link"></span></a>Example</h3>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">api.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">OnToolCall</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#D73A49;--shiki-dark:#F97583">func</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#E36209;--shiki-dark:#FFAB70">event</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ToolCallEvent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#E36209;--shiki-dark:#FFAB70">ctx</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Context</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) {</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">PrintInfo</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"Calling tool: "</span><span style="color:#D73A49;--shiki-dark:#F97583"> +</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> event.Name)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span>
<span class="line"></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">api.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">OnAgentEnd</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#D73A49;--shiki-dark:#F97583">func</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#E36209;--shiki-dark:#FFAB70">e</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">AgentEndEvent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#E36209;--shiki-dark:#FFAB70">ctx</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Context</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) {</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // Per-turn aggregates populated by Kit's runtime — no parallel</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // bookkeeping required in the handler.</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">PrintInfo</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(fmt.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Sprintf</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span></span>
<span class="line"><span style="color:#032F62;--shiki-dark:#9ECBFF">        "Turn finished: </span><span style="color:#005CC5;--shiki-dark:#79B8FF">%d</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> tool calls (</span><span style="color:#005CC5;--shiki-dark:#79B8FF">%v</span><span style="color:#032F62;--shiki-dark:#9ECBFF">), </span><span style="color:#005CC5;--shiki-dark:#79B8FF">%d</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> LLM round-trips, $</span><span style="color:#005CC5;--shiki-dark:#79B8FF">%.4f</span><span style="color:#032F62;--shiki-dark:#9ECBFF">, </span><span style="color:#005CC5;--shiki-dark:#79B8FF">%d</span><span style="color:#032F62;--shiki-dark:#9ECBFF">ms"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">        e.ToolCallCount, e.ToolNames, e.LLMCallCount, e.CostDelta, e.DurationMs,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    ))</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// Per-LLM-call usage — fires multiple times per turn (once per round-trip).</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// Use for accurate budget enforcement between calls.</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">api.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">OnLLMUsage</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#D73A49;--shiki-dark:#F97583">func</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#E36209;--shiki-dark:#FFAB70">e</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">LLMUsageEvent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#E36209;--shiki-dark:#FFAB70">ctx</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Context</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) {</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">PrintInfo</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(fmt.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Sprintf</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span></span>
<span class="line"><span style="color:#032F62;--shiki-dark:#9ECBFF">        "</span><span style="color:#005CC5;--shiki-dark:#79B8FF">%s</span><span style="color:#032F62;--shiki-dark:#9ECBFF">/</span><span style="color:#005CC5;--shiki-dark:#79B8FF">%s</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> step=</span><span style="color:#005CC5;--shiki-dark:#79B8FF">%d</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> tokens=↑</span><span style="color:#005CC5;--shiki-dark:#79B8FF">%d</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> ↓</span><span style="color:#005CC5;--shiki-dark:#79B8FF">%d</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> cost=$</span><span style="color:#005CC5;--shiki-dark:#79B8FF">%.4f</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> (</span><span style="color:#005CC5;--shiki-dark:#79B8FF">%s</span><span style="color:#032F62;--shiki-dark:#9ECBFF">)"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">        e.Provider, e.Model, e.StepNumber,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">        e.InputTokens, e.OutputTokens, e.Cost, e.FinishReason,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    ))</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span></code></pre>
<p><strong><code>AgentEndEvent</code> fields</strong> (in addition to <code>Response</code> and <code>StopReason</code>):</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Type</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td><code>ToolCallCount</code></td>
<td><code>int</code></td>
<td>Total tool invocations during the turn</td>
</tr>
<tr>
<td><code>ToolNames</code></td>
<td><code>[]string</code></td>
<td>Tool names in call order (duplicates preserved)</td>
</tr>
<tr>
<td><code>LLMCallCount</code></td>
<td><code>int</code></td>
<td>LLM round-trips / tool-loop iterations</td>
</tr>
<tr>
<td><code>InputTokensDelta</code></td>
<td><code>int</code></td>
<td>Sum of input tokens across all LLM calls this turn</td>
</tr>
<tr>
<td><code>OutputTokensDelta</code></td>
<td><code>int</code></td>
<td>Sum of output tokens across all LLM calls this turn</td>
</tr>
<tr>
<td><code>CacheReadTokensDelta</code></td>
<td><code>int</code></td>
<td>Sum of cache-read tokens this turn</td>
</tr>
<tr>
<td><code>CacheWriteTokensDelta</code></td>
<td><code>int</code></td>
<td>Sum of cache-write tokens this turn</td>
</tr>
<tr>
<td><code>CostDelta</code></td>
<td><code>float64</code></td>
<td>Cost in USD (zero when pricing is unknown or OAuth credentials)</td>
</tr>
<tr>
<td><code>DurationMs</code></td>
<td><code>int64</code></td>
<td>Wall-clock time from <code>AgentStart</code> to <code>AgentEnd</code></td>
</tr>
</tbody>
</table>
<p><strong><code>LLMUsageEvent</code> fields</strong>:</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Type</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td><code>InputTokens</code> / <code>OutputTokens</code></td>
<td><code>int</code></td>
<td>Per-call token deltas</td>
</tr>
<tr>
<td><code>CacheReadTokens</code> / <code>CacheWriteTokens</code></td>
<td><code>int</code></td>
<td>Per-call cache token deltas</td>
</tr>
<tr>
<td><code>Cost</code></td>
<td><code>float64</code></td>
<td>Per-call USD cost (zero when pricing unknown)</td>
</tr>
<tr>
<td><code>Model</code> / <code>Provider</code></td>
<td><code>string</code></td>
<td>Model used for this specific call — may differ from earlier calls if <code>ctx.SetModel</code> was called mid-turn</td>
</tr>
<tr>
<td><code>StepNumber</code></td>
<td><code>int</code></td>
<td>Zero-based step index within the turn</td>
</tr>
<tr>
<td><code>FinishReason</code></td>
<td><code>string</code></td>
<td>Provider finish reason for this call (<code>"stop"</code>, <code>"tool_calls"</code>, <code>"length"</code>, ...)</td>
</tr>
<tr>
<td><code>RequestID</code></td>
<td><code>string</code></td>
<td>Optional provider correlation id (may be empty)</td>
</tr>
</tbody>
</table>
<h2 id="tools"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#tools"><span class="icon icon-link"></span></a>Tools</h2>
<p>Register custom tools that the LLM can invoke:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">api.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">RegisterTool</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ToolDef</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Name:        </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"weather"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Description: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"Get current weather for a location"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Parameters: </span><span style="color:#D73A49;--shiki-dark:#F97583">map</span><span style="color:#24292E;--shiki-dark:#E1E4E8">[</span><span style="color:#D73A49;--shiki-dark:#F97583">string</span><span style="color:#24292E;--shiki-dark:#E1E4E8">]</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ParameterDef</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span></span>
<span class="line"><span style="color:#032F62;--shiki-dark:#9ECBFF">        "city"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: {Type: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"string"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, Description: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"City name"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, Required: </span><span style="color:#005CC5;--shiki-dark:#79B8FF">true</span><span style="color:#24292E;--shiki-dark:#E1E4E8">},</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    },</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Handler: </span><span style="color:#D73A49;--shiki-dark:#F97583">func</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#E36209;--shiki-dark:#FFAB70">ctx</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Context</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#E36209;--shiki-dark:#FFAB70">params</span><span style="color:#D73A49;--shiki-dark:#F97583"> map</span><span style="color:#24292E;--shiki-dark:#E1E4E8">[</span><span style="color:#D73A49;--shiki-dark:#F97583">string</span><span style="color:#24292E;--shiki-dark:#E1E4E8">]</span><span style="color:#6F42C1;--shiki-dark:#B392F0">any</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) (</span><span style="color:#D73A49;--shiki-dark:#F97583">string</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#D73A49;--shiki-dark:#F97583">error</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) {</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">        city </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> params[</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"city"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">].(</span><span style="color:#D73A49;--shiki-dark:#F97583">string</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">        return</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> "Sunny, 72°F in "</span><span style="color:#D73A49;--shiki-dark:#F97583"> +</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> city, </span><span style="color:#005CC5;--shiki-dark:#79B8FF">nil</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    },</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span></code></pre>
<h2 id="commands"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#commands"><span class="icon icon-link"></span></a>Commands</h2>
<p>Register slash commands that users can invoke directly:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">api.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">RegisterCommand</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">CommandDef</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Name:        </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"stats"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Description: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"Show context statistics"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Handler: </span><span style="color:#D73A49;--shiki-dark:#F97583">func</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#E36209;--shiki-dark:#FFAB70">ctx</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Context</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#E36209;--shiki-dark:#FFAB70">args</span><span style="color:#D73A49;--shiki-dark:#F97583"> string</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) {</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">        stats </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">GetContextStats</span><span style="color:#24292E;--shiki-dark:#E1E4E8">()</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">        ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">PrintInfo</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(fmt.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Sprintf</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"Tokens: </span><span style="color:#005CC5;--shiki-dark:#79B8FF">%d</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, stats.TotalTokens))</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    },</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span></code></pre>
<h2 id="widgets"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#widgets"><span class="icon icon-link"></span></a>Widgets</h2>
<p>Add persistent status displays above or below the input area:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">SetWidget</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">WidgetConfig</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    ID:        </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"token-count"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Placement: ext.WidgetBelow,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Content:   </span><span style="color:#6F42C1;--shiki-dark:#B392F0">ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">WidgetContent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{Text: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"Tokens: 1,234"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">},</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// Update later — same ID replaces the previous widget</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">SetWidget</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">WidgetConfig</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    ID:        </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"token-count"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Placement: ext.WidgetBelow,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Content:   </span><span style="color:#6F42C1;--shiki-dark:#B392F0">ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">WidgetContent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{Text: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"Tokens: 2,456"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">},</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// Remove</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">RemoveWidget</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"token-count"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span></code></pre>
<p><code>Placement</code> is <code>ext.WidgetAbove</code> or <code>ext.WidgetBelow</code>. <code>Priority</code> orders
multiple widgets within the same slot (lower renders first).</p>
<h3 id="markdown-content"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#markdown-content"><span class="icon icon-link"></span></a>Markdown content</h3>
<p>Set <code>Markdown: true</code> to render <code>Text</code> as styled markdown — headings, bold,
inline code and lists are formatted and sized to the widget's content column:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">SetWidget</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">WidgetConfig</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    ID:        </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"notes"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Placement: ext.WidgetAbove,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Content: </span><span style="color:#6F42C1;--shiki-dark:#B392F0">ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">WidgetContent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">        Markdown: </span><span style="color:#005CC5;--shiki-dark:#79B8FF">true</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">        Text:     </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"## Build</span><span style="color:#005CC5;--shiki-dark:#79B8FF">\\n\\n</span><span style="color:#032F62;--shiki-dark:#9ECBFF">**passing** — \`go test ./...\`"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    },</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span></code></pre>
<h3 id="custom-rendering"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#custom-rendering"><span class="icon icon-link"></span></a>Custom rendering</h3>
<p><code>Text</code> covers static content. For anything Kit has no vocabulary for — sparklines,
gauges, box drawing, sprites — supply a <code>Render</code> function instead. It receives the
width in columns available for content (the gutter and padding are already
subtracted) and Kit uses the returned string <strong>verbatim</strong>:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">SetWidget</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">WidgetConfig</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    ID:        </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"cpu-gauge"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Placement: ext.WidgetAbove,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Style:     </span><span style="color:#6F42C1;--shiki-dark:#B392F0">ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">WidgetStyle</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{NoBorder: </span><span style="color:#005CC5;--shiki-dark:#79B8FF">true</span><span style="color:#24292E;--shiki-dark:#E1E4E8">},</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Content: </span><span style="color:#6F42C1;--shiki-dark:#B392F0">ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">WidgetContent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">        Render: </span><span style="color:#D73A49;--shiki-dark:#F97583">func</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#E36209;--shiki-dark:#FFAB70">width</span><span style="color:#D73A49;--shiki-dark:#F97583"> int</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) </span><span style="color:#D73A49;--shiki-dark:#F97583">string</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> {</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">            filled </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#D73A49;--shiki-dark:#F97583"> int</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(load </span><span style="color:#D73A49;--shiki-dark:#F97583">*</span><span style="color:#D73A49;--shiki-dark:#F97583"> float64</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(width))</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">            return</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> "</span><span style="color:#005CC5;--shiki-dark:#79B8FF">\\033</span><span style="color:#032F62;--shiki-dark:#9ECBFF">[38;5;82m"</span><span style="color:#D73A49;--shiki-dark:#F97583"> +</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> strings.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Repeat</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"━"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, filled) </span><span style="color:#D73A49;--shiki-dark:#F97583">+</span></span>
<span class="line"><span style="color:#032F62;--shiki-dark:#9ECBFF">                "</span><span style="color:#005CC5;--shiki-dark:#79B8FF">\\033</span><span style="color:#032F62;--shiki-dark:#9ECBFF">[0m"</span><span style="color:#D73A49;--shiki-dark:#F97583"> +</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> strings.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Repeat</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"─"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, width</span><span style="color:#D73A49;--shiki-dark:#F97583">-</span><span style="color:#24292E;--shiki-dark:#E1E4E8">filled)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">        },</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    },</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span></code></pre>
<p><code>Render</code> takes priority over <code>Text</code>, and <code>Markdown</code> is ignored when it is set —
a render function is expected to do its own styling. Returning an empty string
hides the widget. A panic inside <code>Render</code> is contained: the widget is hidden and
the error logged, rather than taking down the TUI.</p>
<p><code>Render</code> also works on headers and footers via <code>HeaderFooterConfig</code>.</p>
<h3 id="animated-widgets"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#animated-widgets"><span class="icon icon-link"></span></a>Animated widgets</h3>
<p>Kit's animation clock is demand-driven — it runs while the startup logo or the
activity spinner needs it and stops otherwise, so an idle session costs nothing.
A widget that only reads state repaints whenever something <em>else</em> causes a
render, which when idle means roughly twice a second (the input cursor blink).
That is fine for a counter and visibly choppy for a spinner.</p>
<p>Set <code>RefreshHz</code> to hold the clock open and repaint at a chosen rate:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">Content: </span><span style="color:#6F42C1;--shiki-dark:#B392F0">ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">WidgetContent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    RefreshHz: </span><span style="color:#005CC5;--shiki-dark:#79B8FF">15</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Render:    </span><span style="color:#D73A49;--shiki-dark:#F97583">func</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#E36209;--shiki-dark:#FFAB70">width</span><span style="color:#D73A49;--shiki-dark:#F97583"> int</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) </span><span style="color:#D73A49;--shiki-dark:#F97583">string</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> { </span><span style="color:#D73A49;--shiki-dark:#F97583">return</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> spinnerFrame</span><span style="color:#24292E;--shiki-dark:#E1E4E8">() </span><span style="color:#D73A49;--shiki-dark:#F97583">+</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> " working"</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> },</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">},</span></span></code></pre>
<table>
<thead>
<tr>
<th><code>RefreshHz</code></th>
<th>Behaviour</th>
<th>Use for</th>
</tr>
</thead>
<tbody>
<tr>
<td><code>0</code> (default)</td>
<td>Static. Repaints only when something else renders.</td>
<td>Counters, status text</td>
</tr>
<tr>
<td><code>4</code>–<code>8</code></td>
<td>Gentle pulse</td>
<td>Slow progress, breathing indicators</td>
</tr>
<tr>
<td><code>10</code>–<code>15</code></td>
<td>Smooth</td>
<td>Spinners, meters</td>
</tr>
<tr>
<td><code>30</code></td>
<td>Kit's ceiling</td>
<td>Continuous motion</td>
</tr>
</tbody>
</table>
<p>This is a real cost: a non-zero value means the app never idles. Ask for the
lowest rate that looks right. Kit calls <code>Render</code> at approximately the requested
rate rather than on every frame, so a 5Hz widget does not pay 30Hz of
interpreter crossings.</p>
<p>::: warning
Because <code>Render</code> runs on every frame it must be cheap and must not block — no
network calls, no locks held across the call. Compute in an event handler or
goroutine, store the result, and format it here.
:::</p>
<p>See <a href="https://github.com/mark3labs/kit/blob/master/examples/extensions/arbitrary-ui.go"><code>arbitrary-ui.go</code></a>
for a live dashboard and <a href="https://github.com/mark3labs/kit/blob/master/examples/extensions/bad-apple.go"><code>bad-apple.go</code></a>
for 30fps playback.</p>
<h2 id="headers-and-footers"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#headers-and-footers"><span class="icon icon-link"></span></a>Headers and footers</h2>
<p>Persistent content above and below the conversation:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">SetHeader</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">HeaderFooterConfig</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Content: </span><span style="color:#6F42C1;--shiki-dark:#B392F0">ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">WidgetContent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{Text: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"Project: my-app | Branch: main"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">},</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span>
<span class="line"></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">SetFooter</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">HeaderFooterConfig</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Content: </span><span style="color:#6F42C1;--shiki-dark:#B392F0">ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">WidgetContent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{Text: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"Plan Mode (read-only)"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">},</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span></code></pre>
<p>Headers and footers accept the same <code>WidgetContent</code> as widgets, so <code>Markdown</code>,
<code>Render</code> and <code>RefreshHz</code> all apply.</p>
<p>Plain <code>Text</code> is rendered at <strong>full terminal width with no truncation</strong> — a longer
line wraps and silently consumes a row of scrollback. Measure against
<code>ctx.GetTerminalSize()</code> and truncate before calling <code>SetHeader</code>/<code>SetFooter</code>, or
use <code>Render</code>, which is handed the exact width to draw into.</p>
<h2 id="terminal-size"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#terminal-size"><span class="icon icon-link"></span></a>Terminal size</h2>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">width, height </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">GetTerminalSize</span><span style="color:#24292E;--shiki-dark:#E1E4E8">()  </span><span style="color:#6A737D;--shiki-dark:#6A737D">// 0, 0 outside the interactive TUI</span></span>
<span class="line"></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">api.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">OnTerminalResize</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#D73A49;--shiki-dark:#F97583">func</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#E36209;--shiki-dark:#FFAB70">e</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">TerminalResizeEvent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#E36209;--shiki-dark:#FFAB70">ctx</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Context</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) {</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // e.Width, e.Height — re-render chrome at the new size</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span></code></pre>
<p><code>OnTerminalResize</code> also fires once at startup, so a handler can lay out
immediately instead of waiting for the user to resize.</p>
<p>This is a <strong>function, not a field</strong>, so it reports the live size. A long-lived
goroutine (a ticking clock in a footer, say) that captured a <code>Context</code> still
observes resizes; a struct field would freeze at the value copied when the
handler was invoked.</p>
<p>Note that multi-byte characters occupy more than one column — count display
width, not bytes or runes, when fitting text to <code>width</code>.</p>
<h2 id="status-bar"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#status-bar"><span class="icon icon-link"></span></a>Status bar</h2>
<p>Custom status bar entries:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">SetStatus</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"mode"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"Planning"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">RemoveStatus</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"mode"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span></code></pre>
<h2 id="thinking-level"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#thinking-level"><span class="icon icon-link"></span></a>Thinking level</h2>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">level </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">GetThinkingLevel</span><span style="color:#24292E;--shiki-dark:#E1E4E8">()  </span><span style="color:#6A737D;--shiki-dark:#6A737D">// "off", "none", "minimal", "low", "medium", "high"</span></span>
<span class="line"></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">api.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">OnThinkingLevelChange</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#D73A49;--shiki-dark:#F97583">func</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#E36209;--shiki-dark:#FFAB70">e</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ThinkingLevelChangeEvent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#E36209;--shiki-dark:#FFAB70">ctx</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Context</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) {</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // e.NewLevel, e.PreviousLevel string</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // e.Source string — "user" (/thinking or shift+tab) or "model_fallback"</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span></code></pre>
<p>Models without reasoning support report <code>"off"</code>. To distinguish "reasoning is
switched off" from "this model cannot reason at all", pair it with
<code>ctx.GetModelCapabilities("").Reasoning</code>.</p>
<p><code>Source</code> is <code>"model_fallback"</code> when Kit downgrades the level automatically
because the newly selected model does not support the previous one.</p>
<h2 id="turn-state"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#turn-state"><span class="icon icon-link"></span></a>Turn state</h2>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">api.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">OnTurnStateChange</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#D73A49;--shiki-dark:#F97583">func</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#E36209;--shiki-dark:#FFAB70">e</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">TurnStateChangeEvent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#E36209;--shiki-dark:#FFAB70">ctx</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Context</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) {</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // e.State, e.Previous string — "working" or "idle"</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span></code></pre>
<p>This is a <strong>superset of <code>OnAgentStart</code>/<code>OnAgentEnd</code></strong>: it also covers work that
never reaches the agent loop (shell commands run with <code>!</code>) and fires on every
path back to idle, including cancellation and error.</p>
<table>
<thead>
<tr>
<th>Use</th>
<th>For</th>
</tr>
</thead>
<tbody>
<tr>
<td><code>OnTurnStateChange</code></td>
<td>UI that tracks whether Kit is busy — a spinner, a turn timer</td>
</tr>
<tr>
<td><code>OnAgentStart</code> / <code>OnAgentEnd</code></td>
<td>Agent turns specifically, plus their token usage and cost</td>
</tr>
</tbody>
</table>
<p>Interactive TUI only — like <code>OnTerminalResize</code>, this does not fire in headless,
ACP, or script mode.</p>
<h2 id="shortcuts"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#shortcuts"><span class="icon icon-link"></span></a>Shortcuts</h2>
<p>Global keyboard shortcuts:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">api.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">RegisterShortcut</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ShortcutDef</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Key:         </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"ctrl+t"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Description: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"Toggle plan mode"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">}, </span><span style="color:#D73A49;--shiki-dark:#F97583">func</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#E36209;--shiki-dark:#FFAB70">ctx</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Context</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) {</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // handle shortcut</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span></code></pre>
<h2 id="overlays"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#overlays"><span class="icon icon-link"></span></a>Overlays</h2>
<p>Modal dialogs with markdown content:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ShowOverlay</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">OverlayConfig</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Title:   </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"Help"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Content: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"# Keyboard Shortcuts</span><span style="color:#005CC5;--shiki-dark:#79B8FF">\\n\\n</span><span style="color:#032F62;--shiki-dark:#9ECBFF">- **ctrl+t** — Toggle plan mode</span><span style="color:#005CC5;--shiki-dark:#79B8FF">\\n</span><span style="color:#032F62;--shiki-dark:#9ECBFF">- **ctrl+s** — Save session"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span></code></pre>
<h2 id="tool-renderers"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#tool-renderers"><span class="icon icon-link"></span></a>Tool renderers</h2>
<p>Customize how specific tool calls are displayed in the TUI. <code>RenderHeader</code>
replaces the parameter summary on the header line; <code>RenderBody</code> replaces the
result body. Both receive the width they may draw into, and returning an empty
string falls back to Kit's default rendering:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">api.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">RegisterToolRenderer</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ToolRenderConfig</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    ToolName:    </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"bash"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    DisplayName: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"Shell"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    RenderHeader: </span><span style="color:#D73A49;--shiki-dark:#F97583">func</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#E36209;--shiki-dark:#FFAB70">toolArgs</span><span style="color:#D73A49;--shiki-dark:#F97583"> string</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#E36209;--shiki-dark:#FFAB70">width</span><span style="color:#D73A49;--shiki-dark:#F97583"> int</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) </span><span style="color:#D73A49;--shiki-dark:#F97583">string</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> {</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">        return</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> "$ "</span><span style="color:#D73A49;--shiki-dark:#F97583"> +</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> toolArgs</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    },</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    RenderBody: </span><span style="color:#D73A49;--shiki-dark:#F97583">func</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#E36209;--shiki-dark:#FFAB70">toolResult</span><span style="color:#D73A49;--shiki-dark:#F97583"> string</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#E36209;--shiki-dark:#FFAB70">isError</span><span style="color:#D73A49;--shiki-dark:#F97583"> bool</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#E36209;--shiki-dark:#FFAB70">width</span><span style="color:#D73A49;--shiki-dark:#F97583"> int</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) </span><span style="color:#D73A49;--shiki-dark:#F97583">string</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> {</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">        return</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> toolResult</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    },</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span></code></pre>
<p>Set <code>BorderColor</code> and/or <code>Background</code> (hex strings) to give the tool block its
own stripe and backdrop. Tool blocks are otherwise unattributed, so the stripe
appears only when asked for — it marks a tool as special rather than restyling
every call:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">api.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">RegisterToolRenderer</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ToolRenderConfig</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    ToolName:    </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"deploy"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    BorderColor: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#c678dd"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Background:  </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#1b1b2b"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    RenderBody: </span><span style="color:#D73A49;--shiki-dark:#F97583">func</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#E36209;--shiki-dark:#FFAB70">result</span><span style="color:#D73A49;--shiki-dark:#F97583"> string</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#E36209;--shiki-dark:#FFAB70">isError</span><span style="color:#D73A49;--shiki-dark:#F97583"> bool</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#E36209;--shiki-dark:#FFAB70">width</span><span style="color:#D73A49;--shiki-dark:#F97583"> int</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) </span><span style="color:#D73A49;--shiki-dark:#F97583">string</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> {</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">        return</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> result</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    },</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span></code></pre>
<p>Set <code>BodyMarkdown: true</code> to pass <code>RenderBody</code>'s output through the markdown
renderer.</p>
<h2 id="message-renderers"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#message-renderers"><span class="icon icon-link"></span></a>Message renderers</h2>
<p>Named renderers invoked explicitly from extension code via
<code>ctx.RenderMessage(name, content)</code>:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">api.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">RegisterMessageRenderer</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">MessageRendererConfig</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Name: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"build-status"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Render: </span><span style="color:#D73A49;--shiki-dark:#F97583">func</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#E36209;--shiki-dark:#FFAB70">content</span><span style="color:#D73A49;--shiki-dark:#F97583"> string</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#E36209;--shiki-dark:#FFAB70">width</span><span style="color:#D73A49;--shiki-dark:#F97583"> int</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) </span><span style="color:#D73A49;--shiki-dark:#F97583">string</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> {</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">        return</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> "▸ "</span><span style="color:#D73A49;--shiki-dark:#F97583"> +</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> content</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    },</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span>
<span class="line"></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">RenderMessage</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"build-status"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"all tests passed"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span></code></pre>
<p>::: info
The returned string is <strong>not</strong> emitted verbatim. In interactive mode Kit
re-wraps it to the content width and nests it inside a system message block
(gutter glyph plus indent), so box drawing that assumes full terminal width is
wrapped a second time. Size output to roughly <code>width-4</code> and prefer inline
styling over full-width frames. For output Kit uses as-is, use a
<a href="#custom-rendering">widget with a <code>Render</code> callback</a>.
:::</p>
<h2 id="editor-interceptors"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#editor-interceptors"><span class="icon icon-link"></span></a>Editor interceptors</h2>
<p>Handle key events and wrap the editor's rendering:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">SetEditor</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">EditorConfig</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    HandleKey: </span><span style="color:#D73A49;--shiki-dark:#F97583">func</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#E36209;--shiki-dark:#FFAB70">key</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#E36209;--shiki-dark:#FFAB70">text</span><span style="color:#D73A49;--shiki-dark:#F97583"> string</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) </span><span style="color:#6F42C1;--shiki-dark:#B392F0">ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">EditorKeyAction</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> {</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">        if</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> key </span><span style="color:#D73A49;--shiki-dark:#F97583">==</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> "escape"</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> {</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">            return</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">EditorKeyAction</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{Handled: </span><span style="color:#005CC5;--shiki-dark:#79B8FF">true</span><span style="color:#24292E;--shiki-dark:#E1E4E8">}</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">        }</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">        return</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">EditorKeyAction</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{Handled: </span><span style="color:#005CC5;--shiki-dark:#79B8FF">false</span><span style="color:#24292E;--shiki-dark:#E1E4E8">}</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    },</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span></code></pre>
<h2 id="interactive-prompts"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#interactive-prompts"><span class="icon icon-link"></span></a>Interactive prompts</h2>
<p>Select, multi-select, confirm, and text input dialogs. Each blocks the calling
goroutine until the user answers, and each returns a result struct whose
<code>Cancelled</code> field is true if the user pressed ESC or the prompt was unavailable
(non-interactive mode):</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// Single select</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">res </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">PromptSelect</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">PromptSelectConfig</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Message: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"Choose a model"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Options: []</span><span style="color:#D73A49;--shiki-dark:#F97583">string</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"claude-sonnet"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"gpt-4o"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"llama3"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">},</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">if</span><span style="color:#D73A49;--shiki-dark:#F97583"> !</span><span style="color:#24292E;--shiki-dark:#E1E4E8">res.Cancelled {</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">PrintInfo</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"picked "</span><span style="color:#D73A49;--shiki-dark:#F97583"> +</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> res.Value)  </span><span style="color:#6A737D;--shiki-dark:#6A737D">// also res.Index</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">}</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// Multi-select — Space toggles, a selects all, n clears, Enter confirms</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">pick </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">PromptMultiSelect</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">PromptMultiSelectConfig</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Message:         </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"Which checks should run?"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Options:         []</span><span style="color:#D73A49;--shiki-dark:#F97583">string</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"vet"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"test"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"lint"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">},</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    DefaultSelected: []</span><span style="color:#D73A49;--shiki-dark:#F97583">int</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span><span style="color:#005CC5;--shiki-dark:#79B8FF">0</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#005CC5;--shiki-dark:#79B8FF">1</span><span style="color:#24292E;--shiki-dark:#E1E4E8">},  </span><span style="color:#6A737D;--shiki-dark:#6A737D">// nil selects everything</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">if</span><span style="color:#D73A49;--shiki-dark:#F97583"> !</span><span style="color:#24292E;--shiki-dark:#E1E4E8">pick.Cancelled {</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">PrintInfo</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(strings.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Join</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(pick.Values, </span><span style="color:#032F62;--shiki-dark:#9ECBFF">", "</span><span style="color:#24292E;--shiki-dark:#E1E4E8">))  </span><span style="color:#6A737D;--shiki-dark:#6A737D">// also pick.Indices</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">}</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// Confirm</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">yes </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">PromptConfirm</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">PromptConfirmConfig</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Message: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"Delete this file?"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// Text input</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">name </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">PromptInput</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">PromptInputConfig</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Message:     </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"Enter project name"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Placeholder: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"my-project"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span></code></pre>
<p><code>PromptMultiSelect</code> returns both <code>Values</code> (the selected option text) and
<code>Indices</code> (their zero-based positions), so you can map back to your own data
without string matching.</p>
<h2 id="options"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#options"><span class="icon icon-link"></span></a>Options</h2>
<p>Register configurable extension options:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">api.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">RegisterOption</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">OptionDef</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Name:         </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"auto-commit"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Description:  </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"Automatically commit on shutdown"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    DefaultValue: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"false"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span></code></pre>
<h2 id="subagents"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#subagents"><span class="icon icon-link"></span></a>Subagents</h2>
<p>Spawn in-process child Kit instances:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">_, result, err </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">SpawnSubagent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">SubagentConfig</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Prompt:       </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"Analyze the test files and summarize coverage"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Model:        </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"anthropic/claude-haiku-latest"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    SystemPrompt: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"You are a test analysis expert."</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Blocking:     </span><span style="color:#005CC5;--shiki-dark:#79B8FF">true</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span></code></pre>
<p>With <code>Blocking: false</code> (the default), the subagent runs in a background goroutine and <code>SpawnSubagent</code> returns immediately with a non-nil handle (<code>handle.Wait()</code>, <code>handle.Done()</code>, <code>handle.Kill()</code>); use <code>OnComplete</code>/<code>OnEvent</code> callbacks for results. See <a href="/advanced/subagents">Subagents</a> for a full background-mode example.</p>
<p>Subagent sessions are persisted and linked to the host session by default. Set <code>SessionID</code> to a previous run's <code>SubagentResult.SessionID</code> to resume that subagent for follow-up prompts; see <a href="/advanced/subagents#session-linking-and-resuming">Session linking and resuming</a>.</p>
<h3 id="monitoring-subagents-spawned-by-the-main-agent"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#monitoring-subagents-spawned-by-the-main-agent"><span class="icon icon-link"></span></a>Monitoring subagents spawned by the main agent</h3>
<p>When the LLM uses the built-in <code>subagent</code> tool, extensions can monitor the subagent's activity in real-time using three lifecycle events:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// Subagent started</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">api.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">OnSubagentStart</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#D73A49;--shiki-dark:#F97583">func</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#E36209;--shiki-dark:#FFAB70">e</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">SubagentStartEvent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#E36209;--shiki-dark:#FFAB70">ctx</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Context</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) {</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // e.ToolCallID — unique ID for this subagent invocation</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // e.Task — the task/prompt sent to the subagent</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">PrintInfo</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(fmt.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Sprintf</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"Subagent started: </span><span style="color:#005CC5;--shiki-dark:#79B8FF">%s</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, e.Task))</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// Real-time streaming output from subagent</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">api.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">OnSubagentChunk</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#D73A49;--shiki-dark:#F97583">func</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#E36209;--shiki-dark:#FFAB70">e</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">SubagentChunkEvent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#E36209;--shiki-dark:#FFAB70">ctx</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Context</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) {</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // e.ToolCallID — matches the start event</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // e.Task — task description</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // e.ChunkType — "text", "tool_call", "tool_execution_start", "tool_result"</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // e.Content — text content (for text chunks)</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // e.ToolName — tool name (for tool-related chunks)</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // e.IsError — true if tool result is an error</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">    switch</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> e.ChunkType {</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">    case</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> "text"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">:</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">        // Streaming text output</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">    case</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> "tool_call"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">:</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">        // Subagent is calling a tool</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">    case</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> "tool_execution_start"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">:</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">        // Tool execution started</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">    case</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> "tool_result"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">:</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">        // Tool execution completed (check e.IsError)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    }</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// Subagent completed</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">api.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">OnSubagentEnd</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#D73A49;--shiki-dark:#F97583">func</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#E36209;--shiki-dark:#FFAB70">e</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">SubagentEndEvent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#E36209;--shiki-dark:#FFAB70">ctx</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Context</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) {</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // e.ToolCallID — matches start event</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // e.Task — task description</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // e.Response — final response from subagent</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // e.ErrorMsg — error message if subagent failed</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">    if</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> e.ErrorMsg </span><span style="color:#D73A49;--shiki-dark:#F97583">!=</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> ""</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> {</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">        ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">PrintError</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(fmt.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Sprintf</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"Subagent failed: </span><span style="color:#005CC5;--shiki-dark:#79B8FF">%s</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, e.ErrorMsg))</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    } </span><span style="color:#D73A49;--shiki-dark:#F97583">else</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> {</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">        ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">PrintInfo</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(fmt.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Sprintf</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"Subagent completed: </span><span style="color:#005CC5;--shiki-dark:#79B8FF">%s</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, e.Response))</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    }</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span></code></pre>
<p>This enables building widgets that display real-time subagent activity.</p>
<h2 id="llm-completion"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#llm-completion"><span class="icon icon-link"></span></a>LLM completion</h2>
<p>Make direct model calls without going through the agent loop:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">response </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Complete</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">CompleteRequest</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Prompt: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"Summarize this in one sentence: "</span><span style="color:#D73A49;--shiki-dark:#F97583"> +</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> content,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span></code></pre>
<h2 id="themes"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#themes"><span class="icon icon-link"></span></a>Themes</h2>
<p>Register and switch color themes at runtime:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// Register a custom theme</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">RegisterTheme</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"neon"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#6F42C1;--shiki-dark:#B392F0">ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ThemeColorConfig</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Primary:    </span><span style="color:#6F42C1;--shiki-dark:#B392F0">ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ThemeColor</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{Light: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#CC00FF"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, Dark: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#FF00FF"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">},</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Secondary:  </span><span style="color:#6F42C1;--shiki-dark:#B392F0">ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ThemeColor</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{Light: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#0088CC"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, Dark: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#00FFFF"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">},</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Success:    </span><span style="color:#6F42C1;--shiki-dark:#B392F0">ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ThemeColor</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{Light: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#00CC44"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, Dark: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#00FF66"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">},</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Warning:    </span><span style="color:#6F42C1;--shiki-dark:#B392F0">ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ThemeColor</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{Light: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#CCAA00"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, Dark: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#FFFF00"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">},</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Error:      </span><span style="color:#6F42C1;--shiki-dark:#B392F0">ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ThemeColor</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{Light: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#CC0033"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, Dark: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#FF0055"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">},</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Info:       </span><span style="color:#6F42C1;--shiki-dark:#B392F0">ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ThemeColor</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{Light: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#0088CC"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, Dark: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#00CCFF"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">},</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Text:       </span><span style="color:#6F42C1;--shiki-dark:#B392F0">ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ThemeColor</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{Light: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#111111"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, Dark: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#F0F0F0"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">},</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Background: </span><span style="color:#6F42C1;--shiki-dark:#B392F0">ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ThemeColor</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{Light: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#F0F0F0"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, Dark: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#0A0A14"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">},</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// Switch to it</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">SetTheme</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"neon"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// List all available themes</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">names </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ListThemes</span><span style="color:#24292E;--shiki-dark:#E1E4E8">()</span></span></code></pre>
<p>See <a href="/themes">Themes</a> for the full theme file format, built-in themes, and color reference.</p>
<h2 id="custom-events"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#custom-events"><span class="icon icon-link"></span></a>Custom events</h2>
<p>Inter-extension communication:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// Emit</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">EmitCustomEvent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"my-extension:data-ready"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, payload)</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// Listen</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">api.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">OnCustomEvent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"my-extension:data-ready"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#D73A49;--shiki-dark:#F97583">func</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#E36209;--shiki-dark:#FFAB70">data</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> any</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#E36209;--shiki-dark:#FFAB70">ctx</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Context</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) {</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // handle event</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span></code></pre>
<h2 id="session-state"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#session-state"><span class="icon icon-link"></span></a>Session state</h2>
<p>Last-write-wins key-value store, scoped to the current session and persisted to a sidecar file (<code>&lt;session&gt;.ext-state.json</code>) outside the conversation tree:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">SetState</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"myext:budget-cap"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"10.00"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span>
<span class="line"></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">if</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> cap, ok </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">GetState</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"myext:budget-cap"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">); ok {</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // ...</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">}</span></span>
<span class="line"></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">DeleteState</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"myext:budget-cap"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">keys </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ListState</span><span style="color:#24292E;--shiki-dark:#E1E4E8">()  </span><span style="color:#6A737D;--shiki-dark:#6A737D">// []string, unspecified order</span></span></code></pre>
<p>Reads are O(1) (no branch walk), writes don't grow the session JSONL, and the store is not duplicated when the conversation forks. State is invisible to the LLM and survives session resume.</p>
<h3 id="when-to-use-which-persistence-primitive"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#when-to-use-which-persistence-primitive"><span class="icon icon-link"></span></a>When to use which persistence primitive</h3>
<table>
<thead>
<tr>
<th>Need</th>
<th>Use</th>
<th>Why</th>
</tr>
</thead>
<tbody>
<tr>
<td>Snapshot state ("current value of X")</td>
<td><code>SetState</code> / <code>GetState</code></td>
<td>O(1) reads, sidecar file, last-write-wins</td>
</tr>
<tr>
<td>Audit log / event history</td>
<td><code>AppendEntry</code> / <code>GetEntries</code></td>
<td>Append-only, lives in conversation tree, fork-aware</td>
</tr>
<tr>
<td>One-shot per-turn signal</td>
<td>Enriched <code>AgentEndEvent</code> fields</td>
<td>No persistence needed; runtime tracks it for you</td>
</tr>
<tr>
<td>Per-LLM-call observation</td>
<td><code>OnLLMUsage</code> event</td>
<td>Already attributed to model/provider/step</td>
</tr>
</tbody>
</table>
<p>Using <code>AppendEntry</code> for snapshot state has a cost: it's O(branch_length) to read, fsyncs into the JSONL on every write, and the entry list duplicates on every fork. Prefer <code>SetState</code> for "what's the current value of X?"-style data.</p>
<p>For ephemeral / in-memory sessions (no JSONL path) the state lives only in memory for the lifetime of the runner.</p>
<h2 id="bridged-sdk-apis"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#bridged-sdk-apis"><span class="icon icon-link"></span></a>Bridged SDK APIs</h2>
<p>Extensions can access powerful internal SDK capabilities that enable advanced features like conversation tree navigation, dynamic skill loading, template parsing, and model resolution.</p>
<h3 id="tree-navigation"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#tree-navigation"><span class="icon icon-link"></span></a>Tree Navigation</h3>
<p>Navigate the conversation tree, summarize branches, and implement "fresh context" loops:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// Get a specific node by ID with full metadata and children</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">node </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">GetTreeNode</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"entry-id"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// node.ID, node.ParentID, node.Type ("message"/"branch_summary"/etc)</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// node.Role, node.Content, node.Model, node.Children ([]string)</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// Get the current branch from root to leaf</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">branch </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">GetCurrentBranch</span><span style="color:#24292E;--shiki-dark:#E1E4E8">()  </span><span style="color:#6A737D;--shiki-dark:#6A737D">// []ext.TreeNode</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// Get child entry IDs of a node</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">children </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">GetChildren</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"entry-id"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)  </span><span style="color:#6A737D;--shiki-dark:#6A737D">// []string</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// Navigate/fork to a different entry in the tree</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">result </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">NavigateTo</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"entry-id"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)  </span><span style="color:#6A737D;--shiki-dark:#6A737D">// ext.TreeNavigationResult{Success, Error}</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// Summarize a range of the branch using LLM</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">summary </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">SummarizeBranch</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"from-id"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"to-id"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)  </span><span style="color:#6A737D;--shiki-dark:#6A737D">// string</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// Collapse a branch range into a summary entry (fresh context primitive)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">result </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">CollapseBranch</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"from-id"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"to-id"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"summary text"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span></code></pre>
<h3 id="skill-loading"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#skill-loading"><span class="icon icon-link"></span></a>Skill Loading</h3>
<p>Load and inject skills dynamically at runtime:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// Discover skills from standard locations</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">result </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">DiscoverSkills</span><span style="color:#24292E;--shiki-dark:#E1E4E8">()  </span><span style="color:#6A737D;--shiki-dark:#6A737D">// ext.SkillLoadResult{Skills, Error}</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// Standard locations: ~/.agents/skills/, ~/.config/kit/skills/,</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">//                     &lt;project&gt;/.agents/skills/, &lt;project&gt;/.kit/skills/</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// Load a specific skill file</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">skill, err </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">LoadSkill</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"/path/to/skill.md"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)  </span><span style="color:#6A737D;--shiki-dark:#6A737D">// (*ext.Skill, error string)</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// Spec fields: skill.Name, skill.Description, skill.License, skill.Compatibility,</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">//              skill.Metadata, skill.AllowedTools, skill.DisableModelInvocation</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// Plus content/path and Kit extensions: skill.Content, skill.Path, skill.Tags, skill.When</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// Load all skills from a directory</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">result </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">LoadSkillsFromDir</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"/path/to/skills"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)  </span><span style="color:#6A737D;--shiki-dark:#6A737D">// ext.SkillLoadResult</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// Inject a skill as context (pre-loads for next turn)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">err </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">InjectSkillAsContext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"skill-name"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)  </span><span style="color:#6A737D;--shiki-dark:#6A737D">// error string</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// Inject a skill file directly</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">err </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">InjectRawSkillAsContext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"/path/to/skill.md"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)  </span><span style="color:#6A737D;--shiki-dark:#6A737D">// error string</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// Get all discovered skills</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">skills </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">GetAvailableSkills</span><span style="color:#24292E;--shiki-dark:#E1E4E8">()  </span><span style="color:#6A737D;--shiki-dark:#6A737D">// []ext.Skill</span></span></code></pre>
<h3 id="template-parsing"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#template-parsing"><span class="icon icon-link"></span></a>Template Parsing</h3>
<p>Parse and render templates with variable substitution:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// Parse a template to extract {{variables}}</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">tpl </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ParseTemplate</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"name"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"Hello {{name}}, welcome to {{place}}!"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// tpl.Name, tpl.Content, tpl.Variables ([]string)</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// Render a template with variable values</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">vars </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#D73A49;--shiki-dark:#F97583"> map</span><span style="color:#24292E;--shiki-dark:#E1E4E8">[</span><span style="color:#D73A49;--shiki-dark:#F97583">string</span><span style="color:#24292E;--shiki-dark:#E1E4E8">]</span><span style="color:#D73A49;--shiki-dark:#F97583">string</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"name"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"Alice"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"place"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"Kit"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">}</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">rendered </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">RenderTemplate</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(tpl, vars)  </span><span style="color:#6A737D;--shiki-dark:#6A737D">// "Hello Alice, welcome to Kit!"</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// Parse command-line style arguments</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">pattern </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ArgumentPattern</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Positional: []</span><span style="color:#D73A49;--shiki-dark:#F97583">string</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"command"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"target"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">},  </span><span style="color:#6A737D;--shiki-dark:#6A737D">// $1, $2</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Rest:       </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"args"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,                         </span><span style="color:#6A737D;--shiki-dark:#6A737D">// $@</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Flags:      </span><span style="color:#D73A49;--shiki-dark:#F97583">map</span><span style="color:#24292E;--shiki-dark:#E1E4E8">[</span><span style="color:#D73A49;--shiki-dark:#F97583">string</span><span style="color:#24292E;--shiki-dark:#E1E4E8">]</span><span style="color:#D73A49;--shiki-dark:#F97583">string</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"--loop"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"loop"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"-f"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"force"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">},</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">}</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">result </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ParseArguments</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"deploy staging --loop 5"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, pattern)</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// result.Vars["command"] = "deploy"</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// result.Vars["target"] = "staging"</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// result.Flags["--loop"] = "5"</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// Simple positional argument parsing ($1, $2, $@)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">args </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">SimpleParseArguments</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"deploy staging --force"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#005CC5;--shiki-dark:#79B8FF">2</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// args[0] = "deploy staging --force" (full input)</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// args[1] = "deploy" ($1)</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// args[2] = "staging" ($2)</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// args[3] = "--force" ($@)</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// Evaluate model conditionals with wildcards</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">matches </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">EvaluateModelConditional</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"claude-*"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)  </span><span style="color:#6A737D;--shiki-dark:#6A737D">// bool</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// Patterns: * matches any, ? matches single char, comma = OR</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// Render content with &lt;if-model&gt; conditionals</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">content </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> \`&lt;if-model is="claude-*"&gt;Hi Claude&lt;else&gt;Hi there&lt;/if-model&gt;\`</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">rendered </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">RenderWithModelConditionals</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(content)  </span><span style="color:#6A737D;--shiki-dark:#6A737D">// based on current model</span></span></code></pre>
<h3 id="model-resolution"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#model-resolution"><span class="icon icon-link"></span></a>Model Resolution</h3>
<p>Resolve model fallback chains and query capabilities:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// Resolve a chain of model preferences (tries each until available)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">result </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ResolveModelChain</span><span style="color:#24292E;--shiki-dark:#E1E4E8">([]</span><span style="color:#D73A49;--shiki-dark:#F97583">string</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span></span>
<span class="line"><span style="color:#032F62;--shiki-dark:#9ECBFF">    "anthropic/claude-opus-4"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#032F62;--shiki-dark:#9ECBFF">    "anthropic/claude-sonnet-4"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#032F62;--shiki-dark:#9ECBFF">    "openai/gpt-4o"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// result.Model (selected), result.Capabilities, result.Attempted, result.Error</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// Get capabilities for a specific model</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">caps, err </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">GetModelCapabilities</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"anthropic/claude-sonnet-4"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// caps.Provider, caps.ModelID, caps.ContextLimit, caps.Reasoning, caps.Streaming</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// caps.Pricing (see Model pricing below)</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// Pass an empty string for the model currently in use</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">caps, err </span><span style="color:#D73A49;--shiki-dark:#F97583">=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">GetModelCapabilities</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">""</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// Check if a model is available (provider exists)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">available </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">CheckModelAvailable</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"anthropic/claude-sonnet-4"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)  </span><span style="color:#6A737D;--shiki-dark:#6A737D">// bool</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// Get current provider/model ID</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">provider </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">GetCurrentProvider</span><span style="color:#24292E;--shiki-dark:#E1E4E8">()  </span><span style="color:#6A737D;--shiki-dark:#6A737D">// "anthropic"</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">modelID </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">GetCurrentModelID</span><span style="color:#24292E;--shiki-dark:#E1E4E8">()    </span><span style="color:#6A737D;--shiki-dark:#6A737D">// "claude-sonnet-4"</span></span></code></pre>
<h3 id="model-pricing"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#model-pricing"><span class="icon icon-link"></span></a>Model pricing</h3>
<p><code>ModelCapabilities.Pricing</code> reports registry token costs in <strong>US dollars per
million tokens</strong>, so a cost is <code>tokens * rate / 1_000_000</code>:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">caps, _ </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">GetModelCapabilities</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">""</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">p </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> caps.Pricing</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// p.Input, p.Output           float64 — $ per 1M tokens</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// p.CacheRead, p.CacheWrite   float64 — valid only when the Has* flag is true</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// p.HasCacheRead, p.HasCacheWrite bool</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// p.Known                     bool</span></span></code></pre>
<p>Always check <code>Known</code> before rendering a cost. It is <code>false</code> for local models and
custom OpenAI-compatible endpoints, where every rate is zero — without the flag
an unpriced model is indistinguishable from a free one. Likewise check
<code>HasCacheRead</code> rather than assuming a zero <code>CacheRead</code> means cache reads are
free.</p>
<p>Computing prompt-cache savings:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">usage </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">GetSessionUsage</span><span style="color:#24292E;--shiki-dark:#E1E4E8">()</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">if</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> p.Known </span><span style="color:#D73A49;--shiki-dark:#F97583">&amp;&amp;</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> p.HasCacheRead {</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    saved </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#D73A49;--shiki-dark:#F97583"> float64</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(usage.TotalCacheReadTokens) </span><span style="color:#D73A49;--shiki-dark:#F97583">*</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> (p.Input </span><span style="color:#D73A49;--shiki-dark:#F97583">-</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> p.CacheRead) </span><span style="color:#D73A49;--shiki-dark:#F97583">/</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> 1000000</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">}</span></span></code></pre>
<p>The same <code>Pricing</code> field is present on each entry from
<code>ctx.GetAvailableModels()</code>.</p>`,headings:[{depth:2,text:"Lifecycle events",id:"lifecycle-events"},{depth:3,text:"Example",id:"example"},{depth:2,text:"Tools",id:"tools"},{depth:2,text:"Commands",id:"commands"},{depth:2,text:"Widgets",id:"widgets"},{depth:3,text:"Markdown content",id:"markdown-content"},{depth:3,text:"Custom rendering",id:"custom-rendering"},{depth:3,text:"Animated widgets",id:"animated-widgets"},{depth:2,text:"Headers and footers",id:"headers-and-footers"},{depth:2,text:"Terminal size",id:"terminal-size"},{depth:2,text:"Status bar",id:"status-bar"},{depth:2,text:"Thinking level",id:"thinking-level"},{depth:2,text:"Turn state",id:"turn-state"},{depth:2,text:"Shortcuts",id:"shortcuts"},{depth:2,text:"Overlays",id:"overlays"},{depth:2,text:"Tool renderers",id:"tool-renderers"},{depth:2,text:"Message renderers",id:"message-renderers"},{depth:2,text:"Editor interceptors",id:"editor-interceptors"},{depth:2,text:"Interactive prompts",id:"interactive-prompts"},{depth:2,text:"Options",id:"options"},{depth:2,text:"Subagents",id:"subagents"},{depth:3,text:"Monitoring subagents spawned by the main agent",id:"monitoring-subagents-spawned-by-the-main-agent"},{depth:2,text:"LLM completion",id:"llm-completion"},{depth:2,text:"Themes",id:"themes"},{depth:2,text:"Custom events",id:"custom-events"},{depth:2,text:"Session state",id:"session-state"},{depth:3,text:"When to use which persistence primitive",id:"when-to-use-which-persistence-primitive"},{depth:2,text:"Bridged SDK APIs",id:"bridged-sdk-apis"},{depth:3,text:"Tree Navigation",id:"tree-navigation"},{depth:3,text:"Skill Loading",id:"skill-loading"},{depth:3,text:"Template Parsing",id:"template-parsing"},{depth:3,text:"Model Resolution",id:"model-resolution"},{depth:3,text:"Model pricing",id:"model-pricing"}],raw:`
# Extension Capabilities

## Lifecycle events

Extensions can hook into 30 lifecycle events:

| Event | Description |
|-------|-------------|
| \`OnSessionStart\` | Session initialized |
| \`OnSessionShutdown\` | Session ending |
| \`OnBeforeAgentStart\` | Before the agent loop begins |
| \`OnAgentStart\` | Agent loop started |
| \`OnAgentEnd\` | Agent loop completed (carries per-turn aggregates: tool counts, token deltas, cost, duration) |
| \`OnLLMUsage\` | Per-LLM-call token + cost delta (fires once per provider round-trip) |
| \`OnToolCall\` | Tool call requested by the model |
| \`OnToolCallInputStart\` | LLM began generating tool call arguments (tool name known, args streaming) |
| \`OnToolCallInputDelta\` | Streamed JSON fragment of tool call arguments |
| \`OnToolCallInputEnd\` | Tool argument streaming complete, before execution begins |
| \`OnToolExecutionStart\` | Tool execution beginning |
| \`OnToolOutput\` | Streaming tool output chunk (for long-running tools) |
| \`OnToolExecutionEnd\` | Tool execution completed |
| \`OnToolResult\` | Tool result returned |
| \`OnInput\` | User input received |
| \`OnMessageStart\` | Assistant message started |
| \`OnMessageUpdate\` | Streaming text chunk received |
| \`OnMessageEnd\` | Assistant message completed |
| \`OnModelChange\` | Model switched |
| \`OnThinkingLevelChange\` | Extended-thinking effort level changed |
| \`OnTerminalResize\` | Terminal resized (also fires once at startup) |
| \`OnTurnStateChange\` | UI entered or left the working state |
| \`OnContextPrepare\` | Context being assembled for the model |
| \`OnBeforeFork\` | Before forking a conversation branch |
| \`OnBeforeSessionSwitch\` | Before switching sessions |
| \`OnBeforeCompact\` | Before conversation compaction |
| \`OnCustomEvent\` | Custom inter-extension event received |
| \`OnSubagentStart\` | Subagent spawned by the main agent |
| \`OnSubagentChunk\` | Real-time output from subagent (text, tool calls, results) |
| \`OnSubagentEnd\` | Subagent completed with final response/error |

### Example

\`\`\`go
api.OnToolCall(func(event ext.ToolCallEvent, ctx ext.Context) {
    ctx.PrintInfo("Calling tool: " + event.Name)
})

api.OnAgentEnd(func(e ext.AgentEndEvent, ctx ext.Context) {
    // Per-turn aggregates populated by Kit's runtime — no parallel
    // bookkeeping required in the handler.
    ctx.PrintInfo(fmt.Sprintf(
        "Turn finished: %d tool calls (%v), %d LLM round-trips, $%.4f, %dms",
        e.ToolCallCount, e.ToolNames, e.LLMCallCount, e.CostDelta, e.DurationMs,
    ))
})

// Per-LLM-call usage — fires multiple times per turn (once per round-trip).
// Use for accurate budget enforcement between calls.
api.OnLLMUsage(func(e ext.LLMUsageEvent, ctx ext.Context) {
    ctx.PrintInfo(fmt.Sprintf(
        "%s/%s step=%d tokens=↑%d ↓%d cost=$%.4f (%s)",
        e.Provider, e.Model, e.StepNumber,
        e.InputTokens, e.OutputTokens, e.Cost, e.FinishReason,
    ))
})
\`\`\`

**\`AgentEndEvent\` fields** (in addition to \`Response\` and \`StopReason\`):

| Field | Type | Description |
|-------|------|-------------|
| \`ToolCallCount\` | \`int\` | Total tool invocations during the turn |
| \`ToolNames\` | \`[]string\` | Tool names in call order (duplicates preserved) |
| \`LLMCallCount\` | \`int\` | LLM round-trips / tool-loop iterations |
| \`InputTokensDelta\` | \`int\` | Sum of input tokens across all LLM calls this turn |
| \`OutputTokensDelta\` | \`int\` | Sum of output tokens across all LLM calls this turn |
| \`CacheReadTokensDelta\` | \`int\` | Sum of cache-read tokens this turn |
| \`CacheWriteTokensDelta\` | \`int\` | Sum of cache-write tokens this turn |
| \`CostDelta\` | \`float64\` | Cost in USD (zero when pricing is unknown or OAuth credentials) |
| \`DurationMs\` | \`int64\` | Wall-clock time from \`AgentStart\` to \`AgentEnd\` |

**\`LLMUsageEvent\` fields**:

| Field | Type | Description |
|-------|------|-------------|
| \`InputTokens\` / \`OutputTokens\` | \`int\` | Per-call token deltas |
| \`CacheReadTokens\` / \`CacheWriteTokens\` | \`int\` | Per-call cache token deltas |
| \`Cost\` | \`float64\` | Per-call USD cost (zero when pricing unknown) |
| \`Model\` / \`Provider\` | \`string\` | Model used for this specific call — may differ from earlier calls if \`ctx.SetModel\` was called mid-turn |
| \`StepNumber\` | \`int\` | Zero-based step index within the turn |
| \`FinishReason\` | \`string\` | Provider finish reason for this call (\`"stop"\`, \`"tool_calls"\`, \`"length"\`, ...) |
| \`RequestID\` | \`string\` | Optional provider correlation id (may be empty) |

## Tools

Register custom tools that the LLM can invoke:

\`\`\`go
api.RegisterTool(ext.ToolDef{
    Name:        "weather",
    Description: "Get current weather for a location",
    Parameters: map[string]ext.ParameterDef{
        "city": {Type: "string", Description: "City name", Required: true},
    },
    Handler: func(ctx ext.Context, params map[string]any) (string, error) {
        city := params["city"].(string)
        return "Sunny, 72°F in " + city, nil
    },
})
\`\`\`

## Commands

Register slash commands that users can invoke directly:

\`\`\`go
api.RegisterCommand(ext.CommandDef{
    Name:        "stats",
    Description: "Show context statistics",
    Handler: func(ctx ext.Context, args string) {
        stats := ctx.GetContextStats()
        ctx.PrintInfo(fmt.Sprintf("Tokens: %d", stats.TotalTokens))
    },
})
\`\`\`

## Widgets

Add persistent status displays above or below the input area:

\`\`\`go
ctx.SetWidget(ext.WidgetConfig{
    ID:        "token-count",
    Placement: ext.WidgetBelow,
    Content:   ext.WidgetContent{Text: "Tokens: 1,234"},
})

// Update later — same ID replaces the previous widget
ctx.SetWidget(ext.WidgetConfig{
    ID:        "token-count",
    Placement: ext.WidgetBelow,
    Content:   ext.WidgetContent{Text: "Tokens: 2,456"},
})

// Remove
ctx.RemoveWidget("token-count")
\`\`\`

\`Placement\` is \`ext.WidgetAbove\` or \`ext.WidgetBelow\`. \`Priority\` orders
multiple widgets within the same slot (lower renders first).

### Markdown content

Set \`Markdown: true\` to render \`Text\` as styled markdown — headings, bold,
inline code and lists are formatted and sized to the widget's content column:

\`\`\`go
ctx.SetWidget(ext.WidgetConfig{
    ID:        "notes",
    Placement: ext.WidgetAbove,
    Content: ext.WidgetContent{
        Markdown: true,
        Text:     "## Build\\n\\n**passing** — \`go test ./...\`",
    },
})
\`\`\`

### Custom rendering

\`Text\` covers static content. For anything Kit has no vocabulary for — sparklines,
gauges, box drawing, sprites — supply a \`Render\` function instead. It receives the
width in columns available for content (the gutter and padding are already
subtracted) and Kit uses the returned string **verbatim**:

\`\`\`go
ctx.SetWidget(ext.WidgetConfig{
    ID:        "cpu-gauge",
    Placement: ext.WidgetAbove,
    Style:     ext.WidgetStyle{NoBorder: true},
    Content: ext.WidgetContent{
        Render: func(width int) string {
            filled := int(load * float64(width))
            return "\\033[38;5;82m" + strings.Repeat("━", filled) +
                "\\033[0m" + strings.Repeat("─", width-filled)
        },
    },
})
\`\`\`

\`Render\` takes priority over \`Text\`, and \`Markdown\` is ignored when it is set —
a render function is expected to do its own styling. Returning an empty string
hides the widget. A panic inside \`Render\` is contained: the widget is hidden and
the error logged, rather than taking down the TUI.

\`Render\` also works on headers and footers via \`HeaderFooterConfig\`.

### Animated widgets

Kit's animation clock is demand-driven — it runs while the startup logo or the
activity spinner needs it and stops otherwise, so an idle session costs nothing.
A widget that only reads state repaints whenever something *else* causes a
render, which when idle means roughly twice a second (the input cursor blink).
That is fine for a counter and visibly choppy for a spinner.

Set \`RefreshHz\` to hold the clock open and repaint at a chosen rate:

\`\`\`go
Content: ext.WidgetContent{
    RefreshHz: 15,
    Render:    func(width int) string { return spinnerFrame() + " working" },
},
\`\`\`

| \`RefreshHz\` | Behaviour | Use for |
|---|---|---|
| \`0\` (default) | Static. Repaints only when something else renders. | Counters, status text |
| \`4\`–\`8\` | Gentle pulse | Slow progress, breathing indicators |
| \`10\`–\`15\` | Smooth | Spinners, meters |
| \`30\` | Kit's ceiling | Continuous motion |

This is a real cost: a non-zero value means the app never idles. Ask for the
lowest rate that looks right. Kit calls \`Render\` at approximately the requested
rate rather than on every frame, so a 5Hz widget does not pay 30Hz of
interpreter crossings.

::: warning
Because \`Render\` runs on every frame it must be cheap and must not block — no
network calls, no locks held across the call. Compute in an event handler or
goroutine, store the result, and format it here.
:::

See [\`arbitrary-ui.go\`](https://github.com/mark3labs/kit/blob/master/examples/extensions/arbitrary-ui.go)
for a live dashboard and [\`bad-apple.go\`](https://github.com/mark3labs/kit/blob/master/examples/extensions/bad-apple.go)
for 30fps playback.

## Headers and footers

Persistent content above and below the conversation:

\`\`\`go
ctx.SetHeader(ext.HeaderFooterConfig{
    Content: ext.WidgetContent{Text: "Project: my-app | Branch: main"},
})

ctx.SetFooter(ext.HeaderFooterConfig{
    Content: ext.WidgetContent{Text: "Plan Mode (read-only)"},
})
\`\`\`

Headers and footers accept the same \`WidgetContent\` as widgets, so \`Markdown\`,
\`Render\` and \`RefreshHz\` all apply.

Plain \`Text\` is rendered at **full terminal width with no truncation** — a longer
line wraps and silently consumes a row of scrollback. Measure against
\`ctx.GetTerminalSize()\` and truncate before calling \`SetHeader\`/\`SetFooter\`, or
use \`Render\`, which is handed the exact width to draw into.

## Terminal size

\`\`\`go
width, height := ctx.GetTerminalSize()  // 0, 0 outside the interactive TUI

api.OnTerminalResize(func(e ext.TerminalResizeEvent, ctx ext.Context) {
    // e.Width, e.Height — re-render chrome at the new size
})
\`\`\`

\`OnTerminalResize\` also fires once at startup, so a handler can lay out
immediately instead of waiting for the user to resize.

This is a **function, not a field**, so it reports the live size. A long-lived
goroutine (a ticking clock in a footer, say) that captured a \`Context\` still
observes resizes; a struct field would freeze at the value copied when the
handler was invoked.

Note that multi-byte characters occupy more than one column — count display
width, not bytes or runes, when fitting text to \`width\`.

## Status bar

Custom status bar entries:

\`\`\`go
ctx.SetStatus("mode", "Planning")
ctx.RemoveStatus("mode")
\`\`\`

## Thinking level

\`\`\`go
level := ctx.GetThinkingLevel()  // "off", "none", "minimal", "low", "medium", "high"

api.OnThinkingLevelChange(func(e ext.ThinkingLevelChangeEvent, ctx ext.Context) {
    // e.NewLevel, e.PreviousLevel string
    // e.Source string — "user" (/thinking or shift+tab) or "model_fallback"
})
\`\`\`

Models without reasoning support report \`"off"\`. To distinguish "reasoning is
switched off" from "this model cannot reason at all", pair it with
\`ctx.GetModelCapabilities("").Reasoning\`.

\`Source\` is \`"model_fallback"\` when Kit downgrades the level automatically
because the newly selected model does not support the previous one.

## Turn state

\`\`\`go
api.OnTurnStateChange(func(e ext.TurnStateChangeEvent, ctx ext.Context) {
    // e.State, e.Previous string — "working" or "idle"
})
\`\`\`

This is a **superset of \`OnAgentStart\`/\`OnAgentEnd\`**: it also covers work that
never reaches the agent loop (shell commands run with \`!\`) and fires on every
path back to idle, including cancellation and error.

| Use | For |
|-----|-----|
| \`OnTurnStateChange\` | UI that tracks whether Kit is busy — a spinner, a turn timer |
| \`OnAgentStart\` / \`OnAgentEnd\` | Agent turns specifically, plus their token usage and cost |

Interactive TUI only — like \`OnTerminalResize\`, this does not fire in headless,
ACP, or script mode.

## Shortcuts

Global keyboard shortcuts:

\`\`\`go
api.RegisterShortcut(ext.ShortcutDef{
    Key:         "ctrl+t",
    Description: "Toggle plan mode",
}, func(ctx ext.Context) {
    // handle shortcut
})
\`\`\`

## Overlays

Modal dialogs with markdown content:

\`\`\`go
ctx.ShowOverlay(ext.OverlayConfig{
    Title:   "Help",
    Content: "# Keyboard Shortcuts\\n\\n- **ctrl+t** — Toggle plan mode\\n- **ctrl+s** — Save session",
})
\`\`\`

## Tool renderers

Customize how specific tool calls are displayed in the TUI. \`RenderHeader\`
replaces the parameter summary on the header line; \`RenderBody\` replaces the
result body. Both receive the width they may draw into, and returning an empty
string falls back to Kit's default rendering:

\`\`\`go
api.RegisterToolRenderer(ext.ToolRenderConfig{
    ToolName:    "bash",
    DisplayName: "Shell",
    RenderHeader: func(toolArgs string, width int) string {
        return "$ " + toolArgs
    },
    RenderBody: func(toolResult string, isError bool, width int) string {
        return toolResult
    },
})
\`\`\`

Set \`BorderColor\` and/or \`Background\` (hex strings) to give the tool block its
own stripe and backdrop. Tool blocks are otherwise unattributed, so the stripe
appears only when asked for — it marks a tool as special rather than restyling
every call:

\`\`\`go
api.RegisterToolRenderer(ext.ToolRenderConfig{
    ToolName:    "deploy",
    BorderColor: "#c678dd",
    Background:  "#1b1b2b",
    RenderBody: func(result string, isError bool, width int) string {
        return result
    },
})
\`\`\`

Set \`BodyMarkdown: true\` to pass \`RenderBody\`'s output through the markdown
renderer.

## Message renderers

Named renderers invoked explicitly from extension code via
\`ctx.RenderMessage(name, content)\`:

\`\`\`go
api.RegisterMessageRenderer(ext.MessageRendererConfig{
    Name: "build-status",
    Render: func(content string, width int) string {
        return "▸ " + content
    },
})

ctx.RenderMessage("build-status", "all tests passed")
\`\`\`

::: info
The returned string is **not** emitted verbatim. In interactive mode Kit
re-wraps it to the content width and nests it inside a system message block
(gutter glyph plus indent), so box drawing that assumes full terminal width is
wrapped a second time. Size output to roughly \`width-4\` and prefer inline
styling over full-width frames. For output Kit uses as-is, use a
[widget with a \`Render\` callback](#custom-rendering).
:::

## Editor interceptors

Handle key events and wrap the editor's rendering:

\`\`\`go
ctx.SetEditor(ext.EditorConfig{
    HandleKey: func(key, text string) ext.EditorKeyAction {
        if key == "escape" {
            return ext.EditorKeyAction{Handled: true}
        }
        return ext.EditorKeyAction{Handled: false}
    },
})
\`\`\`

## Interactive prompts

Select, multi-select, confirm, and text input dialogs. Each blocks the calling
goroutine until the user answers, and each returns a result struct whose
\`Cancelled\` field is true if the user pressed ESC or the prompt was unavailable
(non-interactive mode):

\`\`\`go
// Single select
res := ctx.PromptSelect(ext.PromptSelectConfig{
    Message: "Choose a model",
    Options: []string{"claude-sonnet", "gpt-4o", "llama3"},
})
if !res.Cancelled {
    ctx.PrintInfo("picked " + res.Value)  // also res.Index
}

// Multi-select — Space toggles, a selects all, n clears, Enter confirms
pick := ctx.PromptMultiSelect(ext.PromptMultiSelectConfig{
    Message:         "Which checks should run?",
    Options:         []string{"vet", "test", "lint"},
    DefaultSelected: []int{0, 1},  // nil selects everything
})
if !pick.Cancelled {
    ctx.PrintInfo(strings.Join(pick.Values, ", "))  // also pick.Indices
}

// Confirm
yes := ctx.PromptConfirm(ext.PromptConfirmConfig{
    Message: "Delete this file?",
})

// Text input
name := ctx.PromptInput(ext.PromptInputConfig{
    Message:     "Enter project name",
    Placeholder: "my-project",
})
\`\`\`

\`PromptMultiSelect\` returns both \`Values\` (the selected option text) and
\`Indices\` (their zero-based positions), so you can map back to your own data
without string matching.

## Options

Register configurable extension options:

\`\`\`go
api.RegisterOption(ext.OptionDef{
    Name:         "auto-commit",
    Description:  "Automatically commit on shutdown",
    DefaultValue: "false",
})
\`\`\`

## Subagents

Spawn in-process child Kit instances:

\`\`\`go
_, result, err := ctx.SpawnSubagent(ext.SubagentConfig{
    Prompt:       "Analyze the test files and summarize coverage",
    Model:        "anthropic/claude-haiku-latest",
    SystemPrompt: "You are a test analysis expert.",
    Blocking:     true,
})
\`\`\`

With \`Blocking: false\` (the default), the subagent runs in a background goroutine and \`SpawnSubagent\` returns immediately with a non-nil handle (\`handle.Wait()\`, \`handle.Done()\`, \`handle.Kill()\`); use \`OnComplete\`/\`OnEvent\` callbacks for results. See [Subagents](/advanced/subagents) for a full background-mode example.

Subagent sessions are persisted and linked to the host session by default. Set \`SessionID\` to a previous run's \`SubagentResult.SessionID\` to resume that subagent for follow-up prompts; see [Session linking and resuming](/advanced/subagents#session-linking-and-resuming).

### Monitoring subagents spawned by the main agent

When the LLM uses the built-in \`subagent\` tool, extensions can monitor the subagent's activity in real-time using three lifecycle events:

\`\`\`go
// Subagent started
api.OnSubagentStart(func(e ext.SubagentStartEvent, ctx ext.Context) {
    // e.ToolCallID — unique ID for this subagent invocation
    // e.Task — the task/prompt sent to the subagent
    ctx.PrintInfo(fmt.Sprintf("Subagent started: %s", e.Task))
})

// Real-time streaming output from subagent
api.OnSubagentChunk(func(e ext.SubagentChunkEvent, ctx ext.Context) {
    // e.ToolCallID — matches the start event
    // e.Task — task description
    // e.ChunkType — "text", "tool_call", "tool_execution_start", "tool_result"
    // e.Content — text content (for text chunks)
    // e.ToolName — tool name (for tool-related chunks)
    // e.IsError — true if tool result is an error
    switch e.ChunkType {
    case "text":
        // Streaming text output
    case "tool_call":
        // Subagent is calling a tool
    case "tool_execution_start":
        // Tool execution started
    case "tool_result":
        // Tool execution completed (check e.IsError)
    }
})

// Subagent completed
api.OnSubagentEnd(func(e ext.SubagentEndEvent, ctx ext.Context) {
    // e.ToolCallID — matches start event
    // e.Task — task description
    // e.Response — final response from subagent
    // e.ErrorMsg — error message if subagent failed
    if e.ErrorMsg != "" {
        ctx.PrintError(fmt.Sprintf("Subagent failed: %s", e.ErrorMsg))
    } else {
        ctx.PrintInfo(fmt.Sprintf("Subagent completed: %s", e.Response))
    }
})
\`\`\`

This enables building widgets that display real-time subagent activity.

## LLM completion

Make direct model calls without going through the agent loop:

\`\`\`go
response := ctx.Complete(ext.CompleteRequest{
    Prompt: "Summarize this in one sentence: " + content,
})
\`\`\`

## Themes

Register and switch color themes at runtime:

\`\`\`go
// Register a custom theme
ctx.RegisterTheme("neon", ext.ThemeColorConfig{
    Primary:    ext.ThemeColor{Light: "#CC00FF", Dark: "#FF00FF"},
    Secondary:  ext.ThemeColor{Light: "#0088CC", Dark: "#00FFFF"},
    Success:    ext.ThemeColor{Light: "#00CC44", Dark: "#00FF66"},
    Warning:    ext.ThemeColor{Light: "#CCAA00", Dark: "#FFFF00"},
    Error:      ext.ThemeColor{Light: "#CC0033", Dark: "#FF0055"},
    Info:       ext.ThemeColor{Light: "#0088CC", Dark: "#00CCFF"},
    Text:       ext.ThemeColor{Light: "#111111", Dark: "#F0F0F0"},
    Background: ext.ThemeColor{Light: "#F0F0F0", Dark: "#0A0A14"},
})

// Switch to it
ctx.SetTheme("neon")

// List all available themes
names := ctx.ListThemes()
\`\`\`

See [Themes](/themes) for the full theme file format, built-in themes, and color reference.

## Custom events

Inter-extension communication:

\`\`\`go
// Emit
ctx.EmitCustomEvent("my-extension:data-ready", payload)

// Listen
api.OnCustomEvent("my-extension:data-ready", func(data any, ctx ext.Context) {
    // handle event
})
\`\`\`

## Session state

Last-write-wins key-value store, scoped to the current session and persisted to a sidecar file (\`<session>.ext-state.json\`) outside the conversation tree:

\`\`\`go
ctx.SetState("myext:budget-cap", "10.00")

if cap, ok := ctx.GetState("myext:budget-cap"); ok {
    // ...
}

ctx.DeleteState("myext:budget-cap")
keys := ctx.ListState()  // []string, unspecified order
\`\`\`

Reads are O(1) (no branch walk), writes don't grow the session JSONL, and the store is not duplicated when the conversation forks. State is invisible to the LLM and survives session resume.

### When to use which persistence primitive

| Need | Use | Why |
|------|-----|-----|
| Snapshot state ("current value of X") | \`SetState\` / \`GetState\` | O(1) reads, sidecar file, last-write-wins |
| Audit log / event history | \`AppendEntry\` / \`GetEntries\` | Append-only, lives in conversation tree, fork-aware |
| One-shot per-turn signal | Enriched \`AgentEndEvent\` fields | No persistence needed; runtime tracks it for you |
| Per-LLM-call observation | \`OnLLMUsage\` event | Already attributed to model/provider/step |

Using \`AppendEntry\` for snapshot state has a cost: it's O(branch_length) to read, fsyncs into the JSONL on every write, and the entry list duplicates on every fork. Prefer \`SetState\` for "what's the current value of X?"-style data.

For ephemeral / in-memory sessions (no JSONL path) the state lives only in memory for the lifetime of the runner.

## Bridged SDK APIs

Extensions can access powerful internal SDK capabilities that enable advanced features like conversation tree navigation, dynamic skill loading, template parsing, and model resolution.

### Tree Navigation

Navigate the conversation tree, summarize branches, and implement "fresh context" loops:

\`\`\`go
// Get a specific node by ID with full metadata and children
node := ctx.GetTreeNode("entry-id")
// node.ID, node.ParentID, node.Type ("message"/"branch_summary"/etc)
// node.Role, node.Content, node.Model, node.Children ([]string)

// Get the current branch from root to leaf
branch := ctx.GetCurrentBranch()  // []ext.TreeNode

// Get child entry IDs of a node
children := ctx.GetChildren("entry-id")  // []string

// Navigate/fork to a different entry in the tree
result := ctx.NavigateTo("entry-id")  // ext.TreeNavigationResult{Success, Error}

// Summarize a range of the branch using LLM
summary := ctx.SummarizeBranch("from-id", "to-id")  // string

// Collapse a branch range into a summary entry (fresh context primitive)
result := ctx.CollapseBranch("from-id", "to-id", "summary text")
\`\`\`

### Skill Loading

Load and inject skills dynamically at runtime:

\`\`\`go
// Discover skills from standard locations
result := ctx.DiscoverSkills()  // ext.SkillLoadResult{Skills, Error}
// Standard locations: ~/.agents/skills/, ~/.config/kit/skills/,
//                     <project>/.agents/skills/, <project>/.kit/skills/

// Load a specific skill file
skill, err := ctx.LoadSkill("/path/to/skill.md")  // (*ext.Skill, error string)
// Spec fields: skill.Name, skill.Description, skill.License, skill.Compatibility,
//              skill.Metadata, skill.AllowedTools, skill.DisableModelInvocation
// Plus content/path and Kit extensions: skill.Content, skill.Path, skill.Tags, skill.When

// Load all skills from a directory
result := ctx.LoadSkillsFromDir("/path/to/skills")  // ext.SkillLoadResult

// Inject a skill as context (pre-loads for next turn)
err := ctx.InjectSkillAsContext("skill-name")  // error string

// Inject a skill file directly
err := ctx.InjectRawSkillAsContext("/path/to/skill.md")  // error string

// Get all discovered skills
skills := ctx.GetAvailableSkills()  // []ext.Skill
\`\`\`

### Template Parsing

Parse and render templates with variable substitution:

\`\`\`go
// Parse a template to extract {{variables}}
tpl := ctx.ParseTemplate("name", "Hello {{name}}, welcome to {{place}}!")
// tpl.Name, tpl.Content, tpl.Variables ([]string)

// Render a template with variable values
vars := map[string]string{"name": "Alice", "place": "Kit"}
rendered := ctx.RenderTemplate(tpl, vars)  // "Hello Alice, welcome to Kit!"

// Parse command-line style arguments
pattern := ext.ArgumentPattern{
    Positional: []string{"command", "target"},  // $1, $2
    Rest:       "args",                         // $@
    Flags:      map[string]string{"--loop": "loop", "-f": "force"},
}
result := ctx.ParseArguments("deploy staging --loop 5", pattern)
// result.Vars["command"] = "deploy"
// result.Vars["target"] = "staging"
// result.Flags["--loop"] = "5"

// Simple positional argument parsing ($1, $2, $@)
args := ctx.SimpleParseArguments("deploy staging --force", 2)
// args[0] = "deploy staging --force" (full input)
// args[1] = "deploy" ($1)
// args[2] = "staging" ($2)
// args[3] = "--force" ($@)

// Evaluate model conditionals with wildcards
matches := ctx.EvaluateModelConditional("claude-*")  // bool
// Patterns: * matches any, ? matches single char, comma = OR

// Render content with <if-model> conditionals
content := \`<if-model is="claude-*">Hi Claude<else>Hi there</if-model>\`
rendered := ctx.RenderWithModelConditionals(content)  // based on current model
\`\`\`

### Model Resolution

Resolve model fallback chains and query capabilities:

\`\`\`go
// Resolve a chain of model preferences (tries each until available)
result := ctx.ResolveModelChain([]string{
    "anthropic/claude-opus-4",
    "anthropic/claude-sonnet-4",
    "openai/gpt-4o",
})
// result.Model (selected), result.Capabilities, result.Attempted, result.Error

// Get capabilities for a specific model
caps, err := ctx.GetModelCapabilities("anthropic/claude-sonnet-4")
// caps.Provider, caps.ModelID, caps.ContextLimit, caps.Reasoning, caps.Streaming
// caps.Pricing (see Model pricing below)

// Pass an empty string for the model currently in use
caps, err = ctx.GetModelCapabilities("")

// Check if a model is available (provider exists)
available := ctx.CheckModelAvailable("anthropic/claude-sonnet-4")  // bool

// Get current provider/model ID
provider := ctx.GetCurrentProvider()  // "anthropic"
modelID := ctx.GetCurrentModelID()    // "claude-sonnet-4"
\`\`\`

### Model pricing

\`ModelCapabilities.Pricing\` reports registry token costs in **US dollars per
million tokens**, so a cost is \`tokens * rate / 1_000_000\`:

\`\`\`go
caps, _ := ctx.GetModelCapabilities("")
p := caps.Pricing
// p.Input, p.Output           float64 — $ per 1M tokens
// p.CacheRead, p.CacheWrite   float64 — valid only when the Has* flag is true
// p.HasCacheRead, p.HasCacheWrite bool
// p.Known                     bool
\`\`\`

Always check \`Known\` before rendering a cost. It is \`false\` for local models and
custom OpenAI-compatible endpoints, where every rate is zero — without the flag
an unpriced model is indistinguishable from a free one. Likewise check
\`HasCacheRead\` rather than assuming a zero \`CacheRead\` means cache reads are
free.

Computing prompt-cache savings:

\`\`\`go
usage := ctx.GetSessionUsage()
if p.Known && p.HasCacheRead {
    saved := float64(usage.TotalCacheReadTokens) * (p.Input - p.CacheRead) / 1000000
}
\`\`\`

The same \`Pricing\` field is present on each entry from
\`ctx.GetAvailableModels()\`.
`};export{s as default};

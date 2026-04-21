const s={frontmatter:{title:"Capabilities",description:"All extension capabilities — lifecycle events, tools, commands, widgets, and more.",hidden:!1,toc:!0,draft:!1},html:`<h1 id="extension-capabilities"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#extension-capabilities"><span class="icon icon-link"></span></a>Extension Capabilities</h1>
<h2 id="lifecycle-events"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#lifecycle-events"><span class="icon icon-link"></span></a>Lifecycle events</h2>
<p>Extensions can hook into 26 lifecycle events:</p>
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
<td>Agent loop completed</td>
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
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">api.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">OnAgentEnd</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#D73A49;--shiki-dark:#F97583">func</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#E36209;--shiki-dark:#FFAB70">_</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">AgentEndEvent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#E36209;--shiki-dark:#FFAB70">ctx</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Context</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) {</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">PrintInfo</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"Agent finished"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span></code></pre>
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
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    ID:       </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"token-count"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Position: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"bottom"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Content:  </span><span style="color:#6F42C1;--shiki-dark:#B392F0">ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">WidgetContent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{Text: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"Tokens: 1,234"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">},</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// Update later</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">SetWidget</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">WidgetConfig</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    ID:       </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"token-count"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Position: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"bottom"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Content:  </span><span style="color:#6F42C1;--shiki-dark:#B392F0">ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">WidgetContent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{Text: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"Tokens: 2,456"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">},</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// Remove</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">RemoveWidget</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"token-count"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span></code></pre>
<h2 id="headers-and-footers"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#headers-and-footers"><span class="icon icon-link"></span></a>Headers and footers</h2>
<p>Persistent content above and below the conversation:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">SetHeader</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">HeaderFooterConfig</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Content: </span><span style="color:#6F42C1;--shiki-dark:#B392F0">ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">WidgetContent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{Text: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"Project: my-app | Branch: main"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">},</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span>
<span class="line"></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">SetFooter</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">HeaderFooterConfig</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Content: </span><span style="color:#6F42C1;--shiki-dark:#B392F0">ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">WidgetContent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{Text: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"Plan Mode (read-only)"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">},</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span></code></pre>
<h2 id="status-bar"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#status-bar"><span class="icon icon-link"></span></a>Status bar</h2>
<p>Custom status bar entries:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">SetStatus</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"mode"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"Planning"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">RemoveStatus</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"mode"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span></code></pre>
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
<p>Customize how specific tool calls are displayed in the TUI:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">api.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">RegisterToolRenderer</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ToolRenderConfig</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    ToolName: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"bash"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Render: </span><span style="color:#D73A49;--shiki-dark:#F97583">func</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#E36209;--shiki-dark:#FFAB70">name</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#E36209;--shiki-dark:#FFAB70">args</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#E36209;--shiki-dark:#FFAB70">result</span><span style="color:#D73A49;--shiki-dark:#F97583"> string</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#E36209;--shiki-dark:#FFAB70">isError</span><span style="color:#D73A49;--shiki-dark:#F97583"> bool</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) </span><span style="color:#D73A49;--shiki-dark:#F97583">string</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> {</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">        return</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> "$ "</span><span style="color:#D73A49;--shiki-dark:#F97583"> +</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> args </span><span style="color:#D73A49;--shiki-dark:#F97583">+</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> "</span><span style="color:#005CC5;--shiki-dark:#79B8FF">\\n</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"</span><span style="color:#D73A49;--shiki-dark:#F97583"> +</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> result</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    },</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span></code></pre>
<h2 id="message-renderers"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#message-renderers"><span class="icon icon-link"></span></a>Message renderers</h2>
<p>Custom rendering for assistant messages:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">api.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">RegisterMessageRenderer</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">MessageRendererConfig</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Name: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"custom"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Render: </span><span style="color:#D73A49;--shiki-dark:#F97583">func</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#E36209;--shiki-dark:#FFAB70">content</span><span style="color:#D73A49;--shiki-dark:#F97583"> string</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) </span><span style="color:#D73A49;--shiki-dark:#F97583">string</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> {</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">        return</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> "&gt;&gt; "</span><span style="color:#D73A49;--shiki-dark:#F97583"> +</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> content</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    },</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span></code></pre>
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
<p>Select, confirm, input, and multi-select dialogs:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// Single select</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">response </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">PromptSelect</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">PromptSelectConfig</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Title:   </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"Choose a model"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Options: []</span><span style="color:#D73A49;--shiki-dark:#F97583">string</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"claude-sonnet"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"gpt-4o"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"llama3"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">},</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// Confirm</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">confirmed </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">PromptConfirm</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">PromptConfirmConfig</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Title: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"Delete this file?"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// Text input</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">name </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">PromptInput</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">PromptInputConfig</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Title:       </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"Enter project name"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Placeholder: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"my-project"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span></code></pre>
<h2 id="options"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#options"><span class="icon icon-link"></span></a>Options</h2>
<p>Register configurable extension options:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">api.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">RegisterOption</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">OptionDef</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Name:         </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"auto-commit"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Description:  </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"Automatically commit on shutdown"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    DefaultValue: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"false"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span></code></pre>
<h2 id="subagents"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#subagents"><span class="icon icon-link"></span></a>Subagents</h2>
<p>Spawn in-process child Kit instances:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">result </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">SpawnSubagent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">SubagentConfig</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Task:         </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"Analyze the test files and summarize coverage"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Model:        </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"anthropic/claude-haiku-latest"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    SystemPrompt: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"You are a test analysis expert."</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span></code></pre>
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
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// Standard locations: ~/.config/kit/skills/, .kit/skills/, .agents/skills/</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// Load a specific skill file</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">skill, err </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">LoadSkill</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"/path/to/skill.md"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)  </span><span style="color:#6A737D;--shiki-dark:#6A737D">// (*ext.Skill, error string)</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// skill.Name, skill.Description, skill.Content, skill.Tags, skill.When</span></span>
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
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// Check if a model is available (provider exists)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">available </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">CheckModelAvailable</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"anthropic/claude-sonnet-4"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)  </span><span style="color:#6A737D;--shiki-dark:#6A737D">// bool</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// Get current provider/model ID</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">provider </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">GetCurrentProvider</span><span style="color:#24292E;--shiki-dark:#E1E4E8">()  </span><span style="color:#6A737D;--shiki-dark:#6A737D">// "anthropic"</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">modelID </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">GetCurrentModelID</span><span style="color:#24292E;--shiki-dark:#E1E4E8">()    </span><span style="color:#6A737D;--shiki-dark:#6A737D">// "claude-sonnet-4"</span></span></code></pre>`,headings:[{depth:2,text:"Lifecycle events",id:"lifecycle-events"},{depth:3,text:"Example",id:"example"},{depth:2,text:"Tools",id:"tools"},{depth:2,text:"Commands",id:"commands"},{depth:2,text:"Widgets",id:"widgets"},{depth:2,text:"Headers and footers",id:"headers-and-footers"},{depth:2,text:"Status bar",id:"status-bar"},{depth:2,text:"Shortcuts",id:"shortcuts"},{depth:2,text:"Overlays",id:"overlays"},{depth:2,text:"Tool renderers",id:"tool-renderers"},{depth:2,text:"Message renderers",id:"message-renderers"},{depth:2,text:"Editor interceptors",id:"editor-interceptors"},{depth:2,text:"Interactive prompts",id:"interactive-prompts"},{depth:2,text:"Options",id:"options"},{depth:2,text:"Subagents",id:"subagents"},{depth:3,text:"Monitoring subagents spawned by the main agent",id:"monitoring-subagents-spawned-by-the-main-agent"},{depth:2,text:"LLM completion",id:"llm-completion"},{depth:2,text:"Themes",id:"themes"},{depth:2,text:"Custom events",id:"custom-events"},{depth:2,text:"Bridged SDK APIs",id:"bridged-sdk-apis"},{depth:3,text:"Tree Navigation",id:"tree-navigation"},{depth:3,text:"Skill Loading",id:"skill-loading"},{depth:3,text:"Template Parsing",id:"template-parsing"},{depth:3,text:"Model Resolution",id:"model-resolution"}],raw:`
# Extension Capabilities

## Lifecycle events

Extensions can hook into 26 lifecycle events:

| Event | Description |
|-------|-------------|
| \`OnSessionStart\` | Session initialized |
| \`OnSessionShutdown\` | Session ending |
| \`OnBeforeAgentStart\` | Before the agent loop begins |
| \`OnAgentStart\` | Agent loop started |
| \`OnAgentEnd\` | Agent loop completed |
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

api.OnAgentEnd(func(_ ext.AgentEndEvent, ctx ext.Context) {
    ctx.PrintInfo("Agent finished")
})
\`\`\`

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
    ID:       "token-count",
    Position: "bottom",
    Content:  ext.WidgetContent{Text: "Tokens: 1,234"},
})

// Update later
ctx.SetWidget(ext.WidgetConfig{
    ID:       "token-count",
    Position: "bottom",
    Content:  ext.WidgetContent{Text: "Tokens: 2,456"},
})

// Remove
ctx.RemoveWidget("token-count")
\`\`\`

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

## Status bar

Custom status bar entries:

\`\`\`go
ctx.SetStatus("mode", "Planning")
ctx.RemoveStatus("mode")
\`\`\`

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

Customize how specific tool calls are displayed in the TUI:

\`\`\`go
api.RegisterToolRenderer(ext.ToolRenderConfig{
    ToolName: "bash",
    Render: func(name, args, result string, isError bool) string {
        return "$ " + args + "\\n" + result
    },
})
\`\`\`

## Message renderers

Custom rendering for assistant messages:

\`\`\`go
api.RegisterMessageRenderer(ext.MessageRendererConfig{
    Name: "custom",
    Render: func(content string) string {
        return ">> " + content
    },
})
\`\`\`

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

Select, confirm, input, and multi-select dialogs:

\`\`\`go
// Single select
response := ctx.PromptSelect(ext.PromptSelectConfig{
    Title:   "Choose a model",
    Options: []string{"claude-sonnet", "gpt-4o", "llama3"},
})

// Confirm
confirmed := ctx.PromptConfirm(ext.PromptConfirmConfig{
    Title: "Delete this file?",
})

// Text input
name := ctx.PromptInput(ext.PromptInputConfig{
    Title:       "Enter project name",
    Placeholder: "my-project",
})
\`\`\`

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
result := ctx.SpawnSubagent(ext.SubagentConfig{
    Task:         "Analyze the test files and summarize coverage",
    Model:        "anthropic/claude-haiku-latest",
    SystemPrompt: "You are a test analysis expert.",
})
\`\`\`

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
// Standard locations: ~/.config/kit/skills/, .kit/skills/, .agents/skills/

// Load a specific skill file
skill, err := ctx.LoadSkill("/path/to/skill.md")  // (*ext.Skill, error string)
// skill.Name, skill.Description, skill.Content, skill.Tags, skill.When

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

// Check if a model is available (provider exists)
available := ctx.CheckModelAvailable("anthropic/claude-sonnet-4")  // bool

// Get current provider/model ID
provider := ctx.GetCurrentProvider()  // "anthropic"
modelID := ctx.GetCurrentModelID()    // "claude-sonnet-4"
\`\`\`
`};export{s as default};

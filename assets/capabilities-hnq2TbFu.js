const s={frontmatter:{title:"Capabilities",description:"All extension capabilities — lifecycle events, tools, commands, widgets, and more.",hidden:!1,toc:!0,draft:!1},html:`<h1 id="extension-capabilities"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#extension-capabilities"><span class="icon icon-link"></span></a>Extension Capabilities</h1>
<h2 id="lifecycle-events"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#lifecycle-events"><span class="icon icon-link"></span></a>Lifecycle events</h2>
<p>Extensions can hook into 18 lifecycle events:</p>
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
<td><code>OnToolExecutionStart</code></td>
<td>Tool execution beginning</td>
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
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Model:        </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"anthropic/claude-haiku-3-5-20241022"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    SystemPrompt: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"You are a test analysis expert."</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span></code></pre>
<h2 id="llm-completion"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#llm-completion"><span class="icon icon-link"></span></a>LLM completion</h2>
<p>Make direct model calls without going through the agent loop:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">response </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Complete</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">CompleteRequest</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Prompt: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"Summarize this in one sentence: "</span><span style="color:#D73A49;--shiki-dark:#F97583"> +</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> content,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span></code></pre>
<h2 id="custom-events"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#custom-events"><span class="icon icon-link"></span></a>Custom events</h2>
<p>Inter-extension communication:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// Emit</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">EmitCustomEvent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"my-extension:data-ready"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, payload)</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// Listen</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">api.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">OnCustomEvent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"my-extension:data-ready"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#D73A49;--shiki-dark:#F97583">func</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#E36209;--shiki-dark:#FFAB70">data</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> any</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#E36209;--shiki-dark:#FFAB70">ctx</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Context</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) {</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // handle event</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span></code></pre>`,headings:[{depth:2,text:"Lifecycle events",id:"lifecycle-events"},{depth:3,text:"Example",id:"example"},{depth:2,text:"Tools",id:"tools"},{depth:2,text:"Commands",id:"commands"},{depth:2,text:"Widgets",id:"widgets"},{depth:2,text:"Headers and footers",id:"headers-and-footers"},{depth:2,text:"Status bar",id:"status-bar"},{depth:2,text:"Shortcuts",id:"shortcuts"},{depth:2,text:"Overlays",id:"overlays"},{depth:2,text:"Tool renderers",id:"tool-renderers"},{depth:2,text:"Message renderers",id:"message-renderers"},{depth:2,text:"Editor interceptors",id:"editor-interceptors"},{depth:2,text:"Interactive prompts",id:"interactive-prompts"},{depth:2,text:"Options",id:"options"},{depth:2,text:"Subagents",id:"subagents"},{depth:2,text:"LLM completion",id:"llm-completion"},{depth:2,text:"Custom events",id:"custom-events"}],raw:`
# Extension Capabilities

## Lifecycle events

Extensions can hook into 18 lifecycle events:

| Event | Description |
|-------|-------------|
| \`OnSessionStart\` | Session initialized |
| \`OnSessionShutdown\` | Session ending |
| \`OnBeforeAgentStart\` | Before the agent loop begins |
| \`OnAgentStart\` | Agent loop started |
| \`OnAgentEnd\` | Agent loop completed |
| \`OnToolCall\` | Tool call requested by the model |
| \`OnToolExecutionStart\` | Tool execution beginning |
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
    Model:        "anthropic/claude-haiku-3-5-20241022",
    SystemPrompt: "You are a test analysis expert.",
})
\`\`\`

## LLM completion

Make direct model calls without going through the agent loop:

\`\`\`go
response := ctx.Complete(ext.CompleteRequest{
    Prompt: "Summarize this in one sentence: " + content,
})
\`\`\`

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
`};export{s as default};

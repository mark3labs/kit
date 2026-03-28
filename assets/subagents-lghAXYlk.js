const s={frontmatter:{title:"Subagents",description:"Multi-agent orchestration with Kit subagents.",hidden:!1,toc:!0,draft:!1},html:`<h1 id="subagents"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#subagents"><span class="icon icon-link"></span></a>Subagents</h1>
<p>Kit supports multi-agent orchestration through both subprocess spawning and in-process subagents.</p>
<h2 id="subprocess-pattern"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#subprocess-pattern"><span class="icon icon-link"></span></a>Subprocess pattern</h2>
<p>Spawn Kit as a subprocess for isolated agent execution:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> "Analyze codebase"</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> \\</span></span>
<span class="line"><span style="color:#005CC5;--shiki-dark:#79B8FF">    --json</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> \\</span></span>
<span class="line"><span style="color:#005CC5;--shiki-dark:#79B8FF">    --no-session</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> \\</span></span>
<span class="line"><span style="color:#005CC5;--shiki-dark:#79B8FF">    --no-extensions</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> \\</span></span>
<span class="line"><span style="color:#005CC5;--shiki-dark:#79B8FF">    --quiet</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> \\</span></span>
<span class="line"><span style="color:#005CC5;--shiki-dark:#79B8FF">    --model</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> anthropic/claude-haiku-latest</span></span></code></pre>
<p>Key flags for subprocess usage:</p>
<table>
<thead>
<tr>
<th>Flag</th>
<th>Purpose</th>
</tr>
</thead>
<tbody>
<tr>
<td><code>--quiet</code></td>
<td>Stdout only, no TUI</td>
</tr>
<tr>
<td><code>--no-session</code></td>
<td>Ephemeral, no persistence</td>
</tr>
<tr>
<td><code>--no-extensions</code></td>
<td>Prevent recursive extension loading</td>
</tr>
<tr>
<td><code>--json</code></td>
<td>Machine-readable output</td>
</tr>
<tr>
<td><code>--system-prompt</code></td>
<td>Custom system prompt (string or file path)</td>
</tr>
</tbody>
</table>
<p>Positional arguments are the prompt. <code>@file</code> arguments attach file content as context.</p>
<h2 id="built-in-subagent-tool"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#built-in-subagent-tool"><span class="icon icon-link"></span></a>Built-in subagent tool</h2>
<p>Kit includes a built-in <code>subagent</code> tool that the LLM can use to delegate tasks to independent child agents:</p>
<pre><code>subagent(
    task: "Analyze the test files and summarize coverage",
    model: "anthropic/claude-haiku-latest",   // optional
    system_prompt: "You are a test analysis expert.",  // optional
    timeout_seconds: 300                               // optional, max 1800
)
</code></pre>
<p>Subagents run as separate in-process Kit instances with full tool access (except spawning further subagents, to prevent infinite recursion). They can run in parallel.</p>
<h2 id="extension-subagents"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#extension-subagents"><span class="icon icon-link"></span></a>Extension subagents</h2>
<p>Extensions can spawn subagents programmatically:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">result </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">SpawnSubagent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">SubagentConfig</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Task:         </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"Review this code for security issues"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Model:        </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"anthropic/claude-sonnet-latest"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    SystemPrompt: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"You are a security auditor."</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span></code></pre>
<h3 id="monitoring-subagents-from-extensions"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#monitoring-subagents-from-extensions"><span class="icon icon-link"></span></a>Monitoring subagents from extensions</h3>
<p>When the LLM (not the extension itself) spawns a subagent using the <code>subagent</code> tool, extensions can monitor its activity in real-time using three lifecycle event handlers:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// Track active subagents and display their output</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">var</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> subagentWidgets </span><span style="color:#D73A49;--shiki-dark:#F97583">map</span><span style="color:#24292E;--shiki-dark:#E1E4E8">[</span><span style="color:#D73A49;--shiki-dark:#F97583">string</span><span style="color:#24292E;--shiki-dark:#E1E4E8">]</span><span style="color:#D73A49;--shiki-dark:#F97583">*</span><span style="color:#6F42C1;--shiki-dark:#B392F0">SubagentWidget</span></span>
<span class="line"></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">func</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> Init</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#E36209;--shiki-dark:#FFAB70">api</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">API</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) {</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // Subagent started by the main agent</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    api.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">OnSubagentStart</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#D73A49;--shiki-dark:#F97583">func</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#E36209;--shiki-dark:#FFAB70">e</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">SubagentStartEvent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#E36209;--shiki-dark:#FFAB70">ctx</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Context</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) {</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">        // e.ToolCallID — unique ID for this subagent invocation</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">        // e.Task — the task/prompt sent to the subagent</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">        widget </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> NewWidget</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(e.ToolCallID, e.Task)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">        subagentWidgets[e.ToolCallID] </span><span style="color:#D73A49;--shiki-dark:#F97583">=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> widget</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">        ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">SetWidget</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(widget.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Config</span><span style="color:#24292E;--shiki-dark:#E1E4E8">())</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    })</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // Real-time streaming from subagent</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    api.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">OnSubagentChunk</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#D73A49;--shiki-dark:#F97583">func</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#E36209;--shiki-dark:#FFAB70">e</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">SubagentChunkEvent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#E36209;--shiki-dark:#FFAB70">ctx</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Context</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) {</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">        // e.ToolCallID — matches the start event</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">        // e.ChunkType — "text", "tool_call", "tool_execution_start", "tool_result"</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">        // e.Content — text content</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">        // e.ToolName — tool name (for tool chunks)</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">        // e.IsError — true if tool result failed</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">        widget </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> subagentWidgets[e.ToolCallID]</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">        if</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> widget </span><span style="color:#D73A49;--shiki-dark:#F97583">!=</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> nil</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> {</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">            widget.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">AddOutput</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(e)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">            ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">SetWidget</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(widget.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Config</span><span style="color:#24292E;--shiki-dark:#E1E4E8">())</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">        }</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    })</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // Subagent completed</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    api.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">OnSubagentEnd</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#D73A49;--shiki-dark:#F97583">func</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#E36209;--shiki-dark:#FFAB70">e</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">SubagentEndEvent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#E36209;--shiki-dark:#FFAB70">ctx</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Context</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) {</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">        // e.Response — final response from subagent</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">        // e.ErrorMsg — error message if subagent failed</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">        widget </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> subagentWidgets[e.ToolCallID]</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">        if</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> widget </span><span style="color:#D73A49;--shiki-dark:#F97583">!=</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> nil</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> {</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">            widget.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">MarkComplete</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(e.Response, e.ErrorMsg)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">            ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">SetWidget</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(widget.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Config</span><span style="color:#24292E;--shiki-dark:#E1E4E8">())</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">            delete</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(subagentWidgets, e.ToolCallID)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">        }</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    })</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">}</span></span></code></pre>
<p><strong>Event structs:</strong></p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">type</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> SubagentStartEvent</span><span style="color:#D73A49;--shiki-dark:#F97583"> struct</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> {</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    ToolCallID </span><span style="color:#D73A49;--shiki-dark:#F97583">string</span><span style="color:#6A737D;--shiki-dark:#6A737D">  // Unique ID for this subagent invocation</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Task       </span><span style="color:#D73A49;--shiki-dark:#F97583">string</span><span style="color:#6A737D;--shiki-dark:#6A737D">  // The task/prompt sent to subagent</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">}</span></span>
<span class="line"></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">type</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> SubagentChunkEvent</span><span style="color:#D73A49;--shiki-dark:#F97583"> struct</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> {</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    ToolCallID </span><span style="color:#D73A49;--shiki-dark:#F97583">string</span><span style="color:#6A737D;--shiki-dark:#6A737D">  // Matches SubagentStartEvent.ToolCallID</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Task       </span><span style="color:#D73A49;--shiki-dark:#F97583">string</span><span style="color:#6A737D;--shiki-dark:#6A737D">  // Task description</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    ChunkType  </span><span style="color:#D73A49;--shiki-dark:#F97583">string</span><span style="color:#6A737D;--shiki-dark:#6A737D">  // "text", "tool_call", "tool_execution_start", "tool_result"</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Content    </span><span style="color:#D73A49;--shiki-dark:#F97583">string</span><span style="color:#6A737D;--shiki-dark:#6A737D">  // For text chunks</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    ToolName   </span><span style="color:#D73A49;--shiki-dark:#F97583">string</span><span style="color:#6A737D;--shiki-dark:#6A737D">  // For tool-related chunks</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    IsError    </span><span style="color:#D73A49;--shiki-dark:#F97583">bool</span><span style="color:#6A737D;--shiki-dark:#6A737D">    // For tool_result chunks</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">}</span></span>
<span class="line"></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">type</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> SubagentEndEvent</span><span style="color:#D73A49;--shiki-dark:#F97583"> struct</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> {</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    ToolCallID </span><span style="color:#D73A49;--shiki-dark:#F97583">string</span><span style="color:#6A737D;--shiki-dark:#6A737D">  // Matches start event</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Task       </span><span style="color:#D73A49;--shiki-dark:#F97583">string</span><span style="color:#6A737D;--shiki-dark:#6A737D">  // Task description</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Response   </span><span style="color:#D73A49;--shiki-dark:#F97583">string</span><span style="color:#6A737D;--shiki-dark:#6A737D">  // Final response from subagent</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    ErrorMsg   </span><span style="color:#D73A49;--shiki-dark:#F97583">string</span><span style="color:#6A737D;--shiki-dark:#6A737D">  // Error message if failed</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">}</span></span></code></pre>
<p>This enables building monitoring widgets that display real-time activity from all subagents spawned by the main agent.</p>
<h2 id="go-sdk-subagents"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#go-sdk-subagents"><span class="icon icon-link"></span></a>Go SDK subagents</h2>
<p>The SDK provides in-process subagent spawning:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">result, err </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Subagent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(ctx, </span><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">SubagentConfig</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Task:         </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"Summarize the changes in this PR"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Model:        </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"anthropic/claude-haiku-latest"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    SystemPrompt: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"You are a code reviewer."</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Timeout:      </span><span style="color:#005CC5;--shiki-dark:#79B8FF">5</span><span style="color:#D73A49;--shiki-dark:#F97583"> *</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> time.Minute,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span></code></pre>
<h3 id="real-time-subagent-events"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#real-time-subagent-events"><span class="icon icon-link"></span></a>Real-time subagent events</h3>
<p>Use <code>SubscribeSubagent</code> to receive real-time events from LLM-initiated subagents (i.e., when the model uses the <code>subagent</code> tool). Register inside an <code>OnToolCall</code> handler using the tool call ID:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">OnToolCall</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#D73A49;--shiki-dark:#F97583">func</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#E36209;--shiki-dark:#FFAB70">e</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ToolCallEvent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) {</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">    if</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> e.ToolName </span><span style="color:#D73A49;--shiki-dark:#F97583">==</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> "subagent"</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> {</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">        host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">SubscribeSubagent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(e.ToolCallID, </span><span style="color:#D73A49;--shiki-dark:#F97583">func</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#E36209;--shiki-dark:#FFAB70">event</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Event</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) {</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">            switch</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> ev </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> event.(</span><span style="color:#D73A49;--shiki-dark:#F97583">type</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) {</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">            case</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">MessageUpdateEvent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">:</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">                fmt.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Print</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(ev.Chunk) </span><span style="color:#6A737D;--shiki-dark:#6A737D">// streaming text from child</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">            case</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ToolCallEvent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">:</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">                fmt.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Printf</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"Child calling: </span><span style="color:#005CC5;--shiki-dark:#79B8FF">%s\\n</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, ev.ToolName)</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">            case</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ToolResultEvent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">:</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">                fmt.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Printf</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"Child result: </span><span style="color:#005CC5;--shiki-dark:#79B8FF">%s\\n</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, ev.ToolName)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">            }</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">        })</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    }</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span></code></pre>
<p>The listener receives the same event types as <code>Subscribe()</code> (<code>ToolCallEvent</code>, <code>MessageUpdateEvent</code>, <code>ReasoningDeltaEvent</code>, etc.) but scoped to the child agent's activity. Listeners are cleaned up automatically when the subagent completes.</p>
<p>If no listeners are registered for a tool call, no event dispatching overhead is incurred.</p>`,headings:[{depth:2,text:"Subprocess pattern",id:"subprocess-pattern"},{depth:2,text:"Built-in subagent tool",id:"built-in-subagent-tool"},{depth:2,text:"Extension subagents",id:"extension-subagents"},{depth:3,text:"Monitoring subagents from extensions",id:"monitoring-subagents-from-extensions"},{depth:2,text:"Go SDK subagents",id:"go-sdk-subagents"},{depth:3,text:"Real-time subagent events",id:"real-time-subagent-events"}],raw:`
# Subagents

Kit supports multi-agent orchestration through both subprocess spawning and in-process subagents.

## Subprocess pattern

Spawn Kit as a subprocess for isolated agent execution:

\`\`\`bash
kit "Analyze codebase" \\
    --json \\
    --no-session \\
    --no-extensions \\
    --quiet \\
    --model anthropic/claude-haiku-latest
\`\`\`

Key flags for subprocess usage:

| Flag | Purpose |
|------|---------|
| \`--quiet\` | Stdout only, no TUI |
| \`--no-session\` | Ephemeral, no persistence |
| \`--no-extensions\` | Prevent recursive extension loading |
| \`--json\` | Machine-readable output |
| \`--system-prompt\` | Custom system prompt (string or file path) |

Positional arguments are the prompt. \`@file\` arguments attach file content as context.

## Built-in subagent tool

Kit includes a built-in \`subagent\` tool that the LLM can use to delegate tasks to independent child agents:

\`\`\`
subagent(
    task: "Analyze the test files and summarize coverage",
    model: "anthropic/claude-haiku-latest",   // optional
    system_prompt: "You are a test analysis expert.",  // optional
    timeout_seconds: 300                               // optional, max 1800
)
\`\`\`

Subagents run as separate in-process Kit instances with full tool access (except spawning further subagents, to prevent infinite recursion). They can run in parallel.

## Extension subagents

Extensions can spawn subagents programmatically:

\`\`\`go
result := ctx.SpawnSubagent(ext.SubagentConfig{
    Task:         "Review this code for security issues",
    Model:        "anthropic/claude-sonnet-latest",
    SystemPrompt: "You are a security auditor.",
})
\`\`\`

### Monitoring subagents from extensions

When the LLM (not the extension itself) spawns a subagent using the \`subagent\` tool, extensions can monitor its activity in real-time using three lifecycle event handlers:

\`\`\`go
// Track active subagents and display their output
var subagentWidgets map[string]*SubagentWidget

func Init(api ext.API) {
    // Subagent started by the main agent
    api.OnSubagentStart(func(e ext.SubagentStartEvent, ctx ext.Context) {
        // e.ToolCallID — unique ID for this subagent invocation
        // e.Task — the task/prompt sent to the subagent
        widget := NewWidget(e.ToolCallID, e.Task)
        subagentWidgets[e.ToolCallID] = widget
        ctx.SetWidget(widget.Config())
    })

    // Real-time streaming from subagent
    api.OnSubagentChunk(func(e ext.SubagentChunkEvent, ctx ext.Context) {
        // e.ToolCallID — matches the start event
        // e.ChunkType — "text", "tool_call", "tool_execution_start", "tool_result"
        // e.Content — text content
        // e.ToolName — tool name (for tool chunks)
        // e.IsError — true if tool result failed
        widget := subagentWidgets[e.ToolCallID]
        if widget != nil {
            widget.AddOutput(e)
            ctx.SetWidget(widget.Config())
        }
    })

    // Subagent completed
    api.OnSubagentEnd(func(e ext.SubagentEndEvent, ctx ext.Context) {
        // e.Response — final response from subagent
        // e.ErrorMsg — error message if subagent failed
        widget := subagentWidgets[e.ToolCallID]
        if widget != nil {
            widget.MarkComplete(e.Response, e.ErrorMsg)
            ctx.SetWidget(widget.Config())
            delete(subagentWidgets, e.ToolCallID)
        }
    })
}
\`\`\`

**Event structs:**

\`\`\`go
type SubagentStartEvent struct {
    ToolCallID string  // Unique ID for this subagent invocation
    Task       string  // The task/prompt sent to subagent
}

type SubagentChunkEvent struct {
    ToolCallID string  // Matches SubagentStartEvent.ToolCallID
    Task       string  // Task description
    ChunkType  string  // "text", "tool_call", "tool_execution_start", "tool_result"
    Content    string  // For text chunks
    ToolName   string  // For tool-related chunks
    IsError    bool    // For tool_result chunks
}

type SubagentEndEvent struct {
    ToolCallID string  // Matches start event
    Task       string  // Task description
    Response   string  // Final response from subagent
    ErrorMsg   string  // Error message if failed
}
\`\`\`

This enables building monitoring widgets that display real-time activity from all subagents spawned by the main agent.

## Go SDK subagents

The SDK provides in-process subagent spawning:

\`\`\`go
result, err := host.Subagent(ctx, kit.SubagentConfig{
    Task:         "Summarize the changes in this PR",
    Model:        "anthropic/claude-haiku-latest",
    SystemPrompt: "You are a code reviewer.",
    Timeout:      5 * time.Minute,
})
\`\`\`

### Real-time subagent events

Use \`SubscribeSubagent\` to receive real-time events from LLM-initiated subagents (i.e., when the model uses the \`subagent\` tool). Register inside an \`OnToolCall\` handler using the tool call ID:

\`\`\`go
host.OnToolCall(func(e kit.ToolCallEvent) {
    if e.ToolName == "subagent" {
        host.SubscribeSubagent(e.ToolCallID, func(event kit.Event) {
            switch ev := event.(type) {
            case kit.MessageUpdateEvent:
                fmt.Print(ev.Chunk) // streaming text from child
            case kit.ToolCallEvent:
                fmt.Printf("Child calling: %s\\n", ev.ToolName)
            case kit.ToolResultEvent:
                fmt.Printf("Child result: %s\\n", ev.ToolName)
            }
        })
    }
})
\`\`\`

The listener receives the same event types as \`Subscribe()\` (\`ToolCallEvent\`, \`MessageUpdateEvent\`, \`ReasoningDeltaEvent\`, etc.) but scoped to the child agent's activity. Listeners are cleaned up automatically when the subagent completes.

If no listeners are registered for a tool call, no event dispatching overhead is incurred.
`};export{s as default};

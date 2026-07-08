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
    agent: "explore",                                  // optional named agent
    model: "anthropic/claude-haiku-latest",   // optional
    system_prompt: "You are a test analysis expert.",  // optional
    timeout_seconds: 300,                              // optional, max 1800
    session_id: "..."                                  // optional, resume a previous subagent
)
</code></pre>
<p>Subagents run as separate in-process Kit instances and inherit the parent's active tools minus <code>subagent</code> (to prevent recursion); named-agent presets and tool allowlists can narrow that set further. They can run in parallel.</p>
<h2 id="session-linking-and-resuming"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#session-linking-and-resuming"><span class="icon icon-link"></span></a>Session linking and resuming</h2>
<p>Subagent runs are session-backed by default, and their sessions are linked to the parent in both directions:</p>
<ul>
<li><strong>Parent → child</strong>: every successful <code>subagent</code> tool call returns the child's session ID as <code>subagent_session_id</code> in the tool-response metadata (also <code>SubagentResult.SessionID</code> in the SDK).</li>
<li><strong>Child → parent</strong>: when the parent is running with a persisted session, the child session's header records <code>parent_session_id</code> (the parent's session UUID), <code>parent_session</code> (the parent's file path), and <code>subagent_task</code> (the original task prompt), so viewers can navigate delegated work as a session tree.</li>
</ul>
<p>Passing a previous run's <code>subagent_session_id</code> back via the <code>session_id</code> parameter resumes that child session instead of starting fresh — the subagent keeps its accumulated context (files read, findings, state), making iterative delegation cheap:</p>
<pre><code>subagent(task: "Research how session persistence works")
→ "Subagent completed successfully..." (subagent_session_id: "abc123...")

subagent(task: "Now check how it handles errors", session_id: "abc123...")
→ follow-up runs in the same child session, reusing its context
</code></pre>
<p>Resumed sessions keep their original parent link; an unknown <code>session_id</code> is an error. Resuming is incompatible with ephemeral (<code>NoSession</code>) runs.</p>
<h2 id="named-agents"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#named-agents"><span class="icon icon-link"></span></a>Named agents</h2>
<p>Named agents are reusable subagent presets defined in markdown files. They are advertised in the <code>subagent</code> tool description, so the LLM can delegate to the right specialist by name — with a preset system prompt, model, tool allowlist, temperature, and timeout.</p>
<h3 id="definition-files"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#definition-files"><span class="icon icon-link"></span></a>Definition files</h3>
<p>The filename (minus <code>.md</code>) is the agent name; YAML frontmatter configures it; the markdown body is the system prompt:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">---</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">description</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">Reviews code for quality and best practices</span><span style="color:#6A737D;--shiki-dark:#6A737D">   # required</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">model</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">anthropic/claude-sonnet-4</span><span style="color:#6A737D;--shiki-dark:#6A737D">                           # optional model override</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">tools</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: [</span><span style="color:#032F62;--shiki-dark:#9ECBFF">read</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#032F62;--shiki-dark:#9ECBFF">grep</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#032F62;--shiki-dark:#9ECBFF">find</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#032F62;--shiki-dark:#9ECBFF">ls</span><span style="color:#24292E;--shiki-dark:#E1E4E8">]                              </span><span style="color:#6A737D;--shiki-dark:#6A737D"># optional tool allowlist</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">temperature</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#005CC5;--shiki-dark:#79B8FF">0.1</span><span style="color:#6A737D;--shiki-dark:#6A737D">                                           # optional</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">timeout</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#005CC5;--shiki-dark:#79B8FF">300</span><span style="color:#6A737D;--shiki-dark:#6A737D">                                               # optional, seconds</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">hidden</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#005CC5;--shiki-dark:#79B8FF">false</span><span style="color:#6A737D;--shiki-dark:#6A737D">                                              # optional: resolvable but not advertised</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">disabled</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#005CC5;--shiki-dark:#79B8FF">false</span><span style="color:#6A737D;--shiki-dark:#6A737D">                                            # optional: remove this agent (and anything it shadows)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">---</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">You are in code review mode. Focus on correctness, security, and</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">maintainability. Report findings with file paths and line references.</span></span></code></pre>
<h3 id="discovery-and-precedence"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#discovery-and-precedence"><span class="icon icon-link"></span></a>Discovery and precedence</h3>
<p>Definitions are discovered from (highest to lowest precedence):</p>
<table>
<thead>
<tr>
<th>Location</th>
<th>Scope</th>
</tr>
</thead>
<tbody>
<tr>
<td><code>&lt;project&gt;/.agents/agents/*.md</code></td>
<td>Project-local (cross-client convention)</td>
</tr>
<tr>
<td><code>&lt;project&gt;/.kit/agents/*.md</code></td>
<td>Project-local (Kit-specific)</td>
</tr>
<tr>
<td><code>~/.config/kit/agents/*.md</code></td>
<td>User-level (<code>$XDG_CONFIG_HOME</code> aware)</td>
</tr>
<tr>
<td>Built-in</td>
<td>Ships with Kit</td>
</tr>
</tbody>
</table>
<p>Higher-precedence definitions override lower ones by name, so a project can replace — or disable via <code>disabled: true</code> — a built-in or user-level agent.</p>
<p>Two built-in agents ship with Kit:</p>
<table>
<thead>
<tr>
<th>Agent</th>
<th>Tools</th>
<th>Purpose</th>
</tr>
</thead>
<tbody>
<tr>
<td><code>general</code></td>
<td>all tools</td>
<td>General-purpose research and multi-step task execution</td>
</tr>
<tr>
<td><code>explore</code></td>
<td><code>read</code>, <code>grep</code>, <code>find</code>, <code>ls</code></td>
<td>Read-only codebase exploration</td>
</tr>
</tbody>
</table>
<h3 id="tool-allowlists"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#tool-allowlists"><span class="icon icon-link"></span></a>Tool allowlists</h3>
<p>An agent without a <code>tools:</code> list gets the default subagent tool set (everything except <code>subagent</code>, preventing recursion). With a <code>tools:</code> allowlist, the subagent is restricted to exactly those tools — a read-only <code>explore</code>-style agent cannot edit files or run commands. Explicit <code>model</code> / <code>system_prompt</code> / <code>timeout_seconds</code> arguments in the tool call override the agent's presets.</p>
<p>Disable named-agent discovery entirely with <code>--no-agents</code>, the <code>no-agents</code> config key, or <code>KIT_NO_AGENTS=true</code>.</p>
<h2 id="extension-subagents"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#extension-subagents"><span class="icon icon-link"></span></a>Extension subagents</h2>
<p>Extensions can spawn subagents programmatically:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">_, result, err </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">SpawnSubagent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">SubagentConfig</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Prompt:       </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"Review this code for security issues"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Model:        </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"anthropic/claude-sonnet-latest"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    SystemPrompt: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"You are a security auditor."</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Blocking:     </span><span style="color:#005CC5;--shiki-dark:#79B8FF">true</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span></code></pre>
<p>With <code>Blocking: false</code> (the default), the subagent runs in a background goroutine and <code>SpawnSubagent</code> returns immediately with a non-nil handle (result is nil); use <code>OnComplete</code>/<code>OnEvent</code> callbacks or the handle to observe the run:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">handle, _, err </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">SpawnSubagent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">SubagentConfig</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Prompt: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"Write unit tests for UserService"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    OnOutput: </span><span style="color:#D73A49;--shiki-dark:#F97583">func</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#E36209;--shiki-dark:#FFAB70">chunk</span><span style="color:#D73A49;--shiki-dark:#F97583"> string</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) {</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">        // Live assistant text chunks (e.g. update a widget)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    },</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    OnComplete: </span><span style="color:#D73A49;--shiki-dark:#F97583">func</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#E36209;--shiki-dark:#FFAB70">result</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">SubagentResult</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) {</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">        ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">SendMessage</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"Subagent finished:</span><span style="color:#005CC5;--shiki-dark:#79B8FF">\\n</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"</span><span style="color:#D73A49;--shiki-dark:#F97583"> +</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> result.Response)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    },</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// handle.Kill()   — cancel the running subagent</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// handle.Wait()   — block until completion, returns SubagentResult</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// &lt;-handle.Done() — channel that closes on completion</span></span></code></pre>
<p>Background subagents run in-process (no subprocess): they get their own session, event bus, and agent loop, inherit the parent's active tools minus the <code>subagent</code> tool, and do not load extensions. Sessions are persisted by default; set <code>NoSession: true</code> for ephemeral runs.</p>
<p>Set <code>SessionID</code> to a previous run's <code>SubagentResult.SessionID</code> to resume that subagent's session for follow-up prompts, and <code>ParentSessionID</code> to override the parent link recorded in the child session's header (it defaults to the host's active persisted session — see <a href="#session-linking-and-resuming">Session linking and resuming</a>).</p>
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
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Prompt:       </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"Summarize the changes in this PR"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Model:        </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"anthropic/claude-haiku-latest"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    SystemPrompt: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"You are a code reviewer."</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Timeout:      </span><span style="color:#005CC5;--shiki-dark:#79B8FF">5</span><span style="color:#D73A49;--shiki-dark:#F97583"> *</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> time.Minute,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span></code></pre>
<p>Set <code>Agent</code> to run the task with a <a href="#named-agents">named agent</a>'s presets; explicitly set fields still win:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">result, err </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Subagent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(ctx, </span><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">SubagentConfig</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Prompt: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"Map out the session persistence flow"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Agent:  </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"explore"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#6A737D;--shiki-dark:#6A737D">// preset prompt + read-only tool allowlist</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span></code></pre>
<p>Set <code>SessionID</code> to a previous run's <code>SubagentResult.SessionID</code> to resume that subagent's session, so follow-ups reuse its accumulated context instead of re-establishing it from scratch:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">first, err </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Subagent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(ctx, </span><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">SubagentConfig</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Prompt: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"Research how session persistence works"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span>
<span class="line"></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">followUp, err </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Subagent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(ctx, </span><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">SubagentConfig</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Prompt:    </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"Now check how it handles errors"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    SessionID: first.SessionID, </span><span style="color:#6A737D;--shiki-dark:#6A737D">// resume the same child session</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span></code></pre>
<p>New child sessions automatically record the parent's session ID in their header when the parent is session-backed (see <a href="#session-linking-and-resuming">Session linking and resuming</a>); set <code>ParentSessionID</code> to override the recorded link.</p>
<p>Inspect the discovered definitions:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">defs </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">GetAgents</span><span style="color:#24292E;--shiki-dark:#E1E4E8">()             </span><span style="color:#6A737D;--shiki-dark:#6A737D">// snapshot of discovered definitions</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">def, ok </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">GetAgent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"explore"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)  </span><span style="color:#6A737D;--shiki-dark:#6A737D">// lookup by name</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// Standalone discovery without a Kit instance:</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">defs, err </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> kit.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">LoadAgentDefinitions</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">""</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) </span><span style="color:#6A737D;--shiki-dark:#6A737D">// "" = current working directory</span></span></code></pre>
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
<p>If no listeners are registered for a tool call, no event dispatching overhead is incurred.</p>`,headings:[{depth:2,text:"Subprocess pattern",id:"subprocess-pattern"},{depth:2,text:"Built-in subagent tool",id:"built-in-subagent-tool"},{depth:2,text:"Session linking and resuming",id:"session-linking-and-resuming"},{depth:2,text:"Named agents",id:"named-agents"},{depth:3,text:"Definition files",id:"definition-files"},{depth:3,text:"Discovery and precedence",id:"discovery-and-precedence"},{depth:3,text:"Tool allowlists",id:"tool-allowlists"},{depth:2,text:"Extension subagents",id:"extension-subagents"},{depth:3,text:"Monitoring subagents from extensions",id:"monitoring-subagents-from-extensions"},{depth:2,text:"Go SDK subagents",id:"go-sdk-subagents"},{depth:3,text:"Real-time subagent events",id:"real-time-subagent-events"}],raw:`
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
    agent: "explore",                                  // optional named agent
    model: "anthropic/claude-haiku-latest",   // optional
    system_prompt: "You are a test analysis expert.",  // optional
    timeout_seconds: 300,                              // optional, max 1800
    session_id: "..."                                  // optional, resume a previous subagent
)
\`\`\`

Subagents run as separate in-process Kit instances and inherit the parent's active tools minus \`subagent\` (to prevent recursion); named-agent presets and tool allowlists can narrow that set further. They can run in parallel.

## Session linking and resuming

Subagent runs are session-backed by default, and their sessions are linked to the parent in both directions:

- **Parent → child**: every successful \`subagent\` tool call returns the child's session ID as \`subagent_session_id\` in the tool-response metadata (also \`SubagentResult.SessionID\` in the SDK).
- **Child → parent**: when the parent is running with a persisted session, the child session's header records \`parent_session_id\` (the parent's session UUID), \`parent_session\` (the parent's file path), and \`subagent_task\` (the original task prompt), so viewers can navigate delegated work as a session tree.

Passing a previous run's \`subagent_session_id\` back via the \`session_id\` parameter resumes that child session instead of starting fresh — the subagent keeps its accumulated context (files read, findings, state), making iterative delegation cheap:

\`\`\`
subagent(task: "Research how session persistence works")
→ "Subagent completed successfully..." (subagent_session_id: "abc123...")

subagent(task: "Now check how it handles errors", session_id: "abc123...")
→ follow-up runs in the same child session, reusing its context
\`\`\`

Resumed sessions keep their original parent link; an unknown \`session_id\` is an error. Resuming is incompatible with ephemeral (\`NoSession\`) runs.

## Named agents

Named agents are reusable subagent presets defined in markdown files. They are advertised in the \`subagent\` tool description, so the LLM can delegate to the right specialist by name — with a preset system prompt, model, tool allowlist, temperature, and timeout.

### Definition files

The filename (minus \`.md\`) is the agent name; YAML frontmatter configures it; the markdown body is the system prompt:

\`\`\`markdown
---
description: Reviews code for quality and best practices   # required
model: anthropic/claude-sonnet-4                           # optional model override
tools: [read, grep, find, ls]                              # optional tool allowlist
temperature: 0.1                                           # optional
timeout: 300                                               # optional, seconds
hidden: false                                              # optional: resolvable but not advertised
disabled: false                                            # optional: remove this agent (and anything it shadows)
---
You are in code review mode. Focus on correctness, security, and
maintainability. Report findings with file paths and line references.
\`\`\`

### Discovery and precedence

Definitions are discovered from (highest to lowest precedence):

| Location | Scope |
|----------|-------|
| \`<project>/.agents/agents/*.md\` | Project-local (cross-client convention) |
| \`<project>/.kit/agents/*.md\` | Project-local (Kit-specific) |
| \`~/.config/kit/agents/*.md\` | User-level (\`$XDG_CONFIG_HOME\` aware) |
| Built-in | Ships with Kit |

Higher-precedence definitions override lower ones by name, so a project can replace — or disable via \`disabled: true\` — a built-in or user-level agent.

Two built-in agents ship with Kit:

| Agent | Tools | Purpose |
|-------|-------|---------|
| \`general\` | all tools | General-purpose research and multi-step task execution |
| \`explore\` | \`read\`, \`grep\`, \`find\`, \`ls\` | Read-only codebase exploration |

### Tool allowlists

An agent without a \`tools:\` list gets the default subagent tool set (everything except \`subagent\`, preventing recursion). With a \`tools:\` allowlist, the subagent is restricted to exactly those tools — a read-only \`explore\`-style agent cannot edit files or run commands. Explicit \`model\` / \`system_prompt\` / \`timeout_seconds\` arguments in the tool call override the agent's presets.

Disable named-agent discovery entirely with \`--no-agents\`, the \`no-agents\` config key, or \`KIT_NO_AGENTS=true\`.

## Extension subagents

Extensions can spawn subagents programmatically:

\`\`\`go
_, result, err := ctx.SpawnSubagent(ext.SubagentConfig{
    Prompt:       "Review this code for security issues",
    Model:        "anthropic/claude-sonnet-latest",
    SystemPrompt: "You are a security auditor.",
    Blocking:     true,
})
\`\`\`

With \`Blocking: false\` (the default), the subagent runs in a background goroutine and \`SpawnSubagent\` returns immediately with a non-nil handle (result is nil); use \`OnComplete\`/\`OnEvent\` callbacks or the handle to observe the run:

\`\`\`go
handle, _, err := ctx.SpawnSubagent(ext.SubagentConfig{
    Prompt: "Write unit tests for UserService",
    OnOutput: func(chunk string) {
        // Live assistant text chunks (e.g. update a widget)
    },
    OnComplete: func(result ext.SubagentResult) {
        ctx.SendMessage("Subagent finished:\\n" + result.Response)
    },
})
// handle.Kill()   — cancel the running subagent
// handle.Wait()   — block until completion, returns SubagentResult
// <-handle.Done() — channel that closes on completion
\`\`\`

Background subagents run in-process (no subprocess): they get their own session, event bus, and agent loop, inherit the parent's active tools minus the \`subagent\` tool, and do not load extensions. Sessions are persisted by default; set \`NoSession: true\` for ephemeral runs.

Set \`SessionID\` to a previous run's \`SubagentResult.SessionID\` to resume that subagent's session for follow-up prompts, and \`ParentSessionID\` to override the parent link recorded in the child session's header (it defaults to the host's active persisted session — see [Session linking and resuming](#session-linking-and-resuming)).

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
    Prompt:       "Summarize the changes in this PR",
    Model:        "anthropic/claude-haiku-latest",
    SystemPrompt: "You are a code reviewer.",
    Timeout:      5 * time.Minute,
})
\`\`\`

Set \`Agent\` to run the task with a [named agent](#named-agents)'s presets; explicitly set fields still win:

\`\`\`go
result, err := host.Subagent(ctx, kit.SubagentConfig{
    Prompt: "Map out the session persistence flow",
    Agent:  "explore", // preset prompt + read-only tool allowlist
})
\`\`\`

Set \`SessionID\` to a previous run's \`SubagentResult.SessionID\` to resume that subagent's session, so follow-ups reuse its accumulated context instead of re-establishing it from scratch:

\`\`\`go
first, err := host.Subagent(ctx, kit.SubagentConfig{
    Prompt: "Research how session persistence works",
})

followUp, err := host.Subagent(ctx, kit.SubagentConfig{
    Prompt:    "Now check how it handles errors",
    SessionID: first.SessionID, // resume the same child session
})
\`\`\`

New child sessions automatically record the parent's session ID in their header when the parent is session-backed (see [Session linking and resuming](#session-linking-and-resuming)); set \`ParentSessionID\` to override the recorded link.

Inspect the discovered definitions:

\`\`\`go
defs := host.GetAgents()             // snapshot of discovered definitions
def, ok := host.GetAgent("explore")  // lookup by name

// Standalone discovery without a Kit instance:
defs, err := kit.LoadAgentDefinitions("") // "" = current working directory
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

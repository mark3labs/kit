const s={frontmatter:{title:"Subagents",description:"Multi-agent orchestration with Kit subagents.",hidden:!1,toc:!0,draft:!1},html:`<h1 id="subagents"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#subagents"><span class="icon icon-link"></span></a>Subagents</h1>
<p>Kit supports multi-agent orchestration through both subprocess spawning and in-process subagents.</p>
<h2 id="subprocess-pattern"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#subprocess-pattern"><span class="icon icon-link"></span></a>Subprocess pattern</h2>
<p>Spawn Kit as a subprocess for isolated agent execution:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> "Analyze codebase"</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> \\</span></span>
<span class="line"><span style="color:#005CC5;--shiki-dark:#79B8FF">    --json</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> \\</span></span>
<span class="line"><span style="color:#005CC5;--shiki-dark:#79B8FF">    --no-session</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> \\</span></span>
<span class="line"><span style="color:#005CC5;--shiki-dark:#79B8FF">    --no-extensions</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> \\</span></span>
<span class="line"><span style="color:#005CC5;--shiki-dark:#79B8FF">    --quiet</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> \\</span></span>
<span class="line"><span style="color:#005CC5;--shiki-dark:#79B8FF">    --model</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> anthropic/claude-haiku-3-5-20241022</span></span></code></pre>
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
<h2 id="built-in-spawn_subagent-tool"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#built-in-spawn_subagent-tool"><span class="icon icon-link"></span></a>Built-in spawn_subagent tool</h2>
<p>Kit includes a built-in <code>spawn_subagent</code> tool that the LLM can use to delegate tasks to independent child agents:</p>
<pre><code>spawn_subagent(
    task: "Analyze the test files and summarize coverage",
    model: "anthropic/claude-haiku-3-5-20241022",   // optional
    system_prompt: "You are a test analysis expert.",  // optional
    timeout_seconds: 300                               // optional, max 1800
)
</code></pre>
<p>Subagents run as separate in-process Kit instances with full tool access (except spawning further subagents, to prevent infinite recursion). They can run in parallel.</p>
<h2 id="extension-subagents"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#extension-subagents"><span class="icon icon-link"></span></a>Extension subagents</h2>
<p>Extensions can spawn subagents programmatically:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">result </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">SpawnSubagent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">SubagentConfig</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Task:         </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"Review this code for security issues"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Model:        </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"anthropic/claude-sonnet-4-5-20250929"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    SystemPrompt: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"You are a security auditor."</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span></code></pre>
<h2 id="go-sdk-subagents"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#go-sdk-subagents"><span class="icon icon-link"></span></a>Go SDK subagents</h2>
<p>The SDK provides in-process subagent spawning:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">result, err </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Subagent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(ctx, </span><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">SubagentConfig</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Task:         </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"Summarize the changes in this PR"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Model:        </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"anthropic/claude-haiku-3-5-20241022"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    SystemPrompt: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"You are a code reviewer."</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Timeout:      </span><span style="color:#005CC5;--shiki-dark:#79B8FF">5</span><span style="color:#D73A49;--shiki-dark:#F97583"> *</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> time.Minute,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span></code></pre>`,headings:[{depth:2,text:"Subprocess pattern",id:"subprocess-pattern"},{depth:2,text:"Built-in spawn_subagent tool",id:"built-in-spawn_subagent-tool"},{depth:2,text:"Extension subagents",id:"extension-subagents"},{depth:2,text:"Go SDK subagents",id:"go-sdk-subagents"}],raw:`
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
    --model anthropic/claude-haiku-3-5-20241022
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

## Built-in spawn_subagent tool

Kit includes a built-in \`spawn_subagent\` tool that the LLM can use to delegate tasks to independent child agents:

\`\`\`
spawn_subagent(
    task: "Analyze the test files and summarize coverage",
    model: "anthropic/claude-haiku-3-5-20241022",   // optional
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
    Model:        "anthropic/claude-sonnet-4-5-20250929",
    SystemPrompt: "You are a security auditor.",
})
\`\`\`

## Go SDK subagents

The SDK provides in-process subagent spawning:

\`\`\`go
result, err := host.Subagent(ctx, kit.SubagentConfig{
    Task:         "Summarize the changes in this PR",
    Model:        "anthropic/claude-haiku-3-5-20241022",
    SystemPrompt: "You are a code reviewer.",
    Timeout:      5 * time.Minute,
})
\`\`\`
`};export{s as default};

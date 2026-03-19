const s={frontmatter:{title:"Quick Start",description:"Get up and running with Kit in minutes.",hidden:!1,toc:!0,draft:!1},html:`<h1 id="quick-start"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#quick-start"><span class="icon icon-link"></span></a>Quick Start</h1>
<h2 id="basic-usage"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#basic-usage"><span class="icon icon-link"></span></a>Basic usage</h2>
<p>Start an interactive session:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span></span></code></pre>
<p>Run a one-off prompt:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> "List files in src/"</span></span></code></pre>
<p>Attach files as context using the <code>@</code> prefix:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> @main.go</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> @test.go</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> "Review these files"</span></span></code></pre>
<p>Use a specific model:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> --model</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> anthropic/claude-sonnet-4-5-20250929</span></span></code></pre>
<h2 id="non-interactive-mode"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#non-interactive-mode"><span class="icon icon-link"></span></a>Non-interactive mode</h2>
<p>Kit can run as a non-interactive tool for scripting and automation.</p>
<p>Get JSON output:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> "Explain main.go"</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> --json</span></span></code></pre>
<p>Quiet mode (final response only, no TUI):</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> "Run tests"</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> --quiet</span></span></code></pre>
<p>Ephemeral mode (no session file created):</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> "Quick question"</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> --no-session</span></span></code></pre>
<h2 id="resuming-sessions"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#resuming-sessions"><span class="icon icon-link"></span></a>Resuming sessions</h2>
<p>Continue the most recent session for the current directory:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> --continue</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D"># or</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> -c</span></span></code></pre>
<p>Pick from previous sessions interactively:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> --resume</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D"># or</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> -r</span></span></code></pre>
<h2 id="acp-server-mode"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#acp-server-mode"><span class="icon icon-link"></span></a>ACP server mode</h2>
<p>Kit can run as an <a href="https://agentclientprotocol.com">ACP (Agent Client Protocol)</a> agent server, enabling ACP-compatible clients (such as <a href="https://github.com/sst/opencode">OpenCode</a>) to drive Kit as a remote coding agent over stdio:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D"># Start Kit as an ACP server (JSON-RPC 2.0 on stdin/stdout)</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> acp</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D"># With debug logging to stderr</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> acp</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> --debug</span></span></code></pre>
<p>The ACP server exposes Kit's full capabilities — LLM execution, tool calls (bash, read, write, edit, grep, etc.), and session persistence — over the standard ACP protocol.</p>`,headings:[{depth:2,text:"Basic usage",id:"basic-usage"},{depth:2,text:"Non-interactive mode",id:"non-interactive-mode"},{depth:2,text:"Resuming sessions",id:"resuming-sessions"},{depth:2,text:"ACP server mode",id:"acp-server-mode"}],raw:`
# Quick Start

## Basic usage

Start an interactive session:

\`\`\`bash
kit
\`\`\`

Run a one-off prompt:

\`\`\`bash
kit "List files in src/"
\`\`\`

Attach files as context using the \`@\` prefix:

\`\`\`bash
kit @main.go @test.go "Review these files"
\`\`\`

Use a specific model:

\`\`\`bash
kit --model anthropic/claude-sonnet-4-5-20250929
\`\`\`

## Non-interactive mode

Kit can run as a non-interactive tool for scripting and automation.

Get JSON output:

\`\`\`bash
kit "Explain main.go" --json
\`\`\`

Quiet mode (final response only, no TUI):

\`\`\`bash
kit "Run tests" --quiet
\`\`\`

Ephemeral mode (no session file created):

\`\`\`bash
kit "Quick question" --no-session
\`\`\`

## Resuming sessions

Continue the most recent session for the current directory:

\`\`\`bash
kit --continue
# or
kit -c
\`\`\`

Pick from previous sessions interactively:

\`\`\`bash
kit --resume
# or
kit -r
\`\`\`

## ACP server mode

Kit can run as an [ACP (Agent Client Protocol)](https://agentclientprotocol.com) agent server, enabling ACP-compatible clients (such as [OpenCode](https://github.com/sst/opencode)) to drive Kit as a remote coding agent over stdio:

\`\`\`bash
# Start Kit as an ACP server (JSON-RPC 2.0 on stdin/stdout)
kit acp

# With debug logging to stderr
kit acp --debug
\`\`\`

The ACP server exposes Kit's full capabilities — LLM execution, tool calls (bash, read, write, edit, grep, etc.), and session persistence — over the standard ACP protocol.
`};export{s as default};

const e={frontmatter:{title:"Commands",description:"Complete reference for all Kit CLI subcommands.",hidden:!1,toc:!0,draft:!1},html:`<h1 id="commands"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#commands"><span class="icon icon-link"></span></a>Commands</h1>
<h2 id="authentication"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#authentication"><span class="icon icon-link"></span></a>Authentication</h2>
<p>For OAuth-enabled providers like Anthropic.</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> auth</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> login</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> [provider]    </span><span style="color:#6A737D;--shiki-dark:#6A737D"># Start OAuth flow (e.g., anthropic)</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> auth</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> logout</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> [provider]   </span><span style="color:#6A737D;--shiki-dark:#6A737D"># Remove credentials for provider</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> auth</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> status</span><span style="color:#6A737D;--shiki-dark:#6A737D">              # Check authentication status</span></span></code></pre>
<h2 id="model-database"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#model-database"><span class="icon icon-link"></span></a>Model database</h2>
<p>Manage the local model database that maps provider names to API configurations.</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> models</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> [provider]        </span><span style="color:#6A737D;--shiki-dark:#6A737D"># List available models (optionally filter by provider)</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> models</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> --all</span><span style="color:#6A737D;--shiki-dark:#6A737D">             # Show all providers (not just LLM-compatible)</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> update-models</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> [source]   </span><span style="color:#6A737D;--shiki-dark:#6A737D"># Update model database</span></span></code></pre>
<p>The <code>update-models</code> command accepts an optional source argument:</p>
<ul>
<li><em>(none)</em> — update from <a href="https://models.dev">models.dev</a></li>
<li>A URL — fetch from a custom endpoint</li>
<li>A file path — load from a local file</li>
<li><code>embedded</code> — reset to the bundled database</li>
</ul>
<h2 id="extension-management"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#extension-management"><span class="icon icon-link"></span></a>Extension management</h2>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> extensions</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> list</span><span style="color:#6A737D;--shiki-dark:#6A737D">          # List discovered extensions</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> extensions</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> validate</span><span style="color:#6A737D;--shiki-dark:#6A737D">      # Validate extension files</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> extensions</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> init</span><span style="color:#6A737D;--shiki-dark:#6A737D">          # Generate example extension template</span></span></code></pre>
<h3 id="installing-extensions-from-git"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#installing-extensions-from-git"><span class="icon icon-link"></span></a>Installing extensions from git</h3>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> install</span><span style="color:#D73A49;--shiki-dark:#F97583"> &lt;</span><span style="color:#032F62;--shiki-dark:#9ECBFF">git-ur</span><span style="color:#24292E;--shiki-dark:#E1E4E8">l</span><span style="color:#D73A49;--shiki-dark:#F97583">&gt;</span><span style="color:#6A737D;--shiki-dark:#6A737D">        # Install extensions from git repositories</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> install</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> -l</span><span style="color:#D73A49;--shiki-dark:#F97583"> &lt;</span><span style="color:#032F62;--shiki-dark:#9ECBFF">git-ur</span><span style="color:#24292E;--shiki-dark:#E1E4E8">l</span><span style="color:#D73A49;--shiki-dark:#F97583">&gt;</span><span style="color:#6A737D;--shiki-dark:#6A737D">     # Install to project-local .kit/git/ directory</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> install</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> -u</span><span style="color:#D73A49;--shiki-dark:#F97583"> &lt;</span><span style="color:#032F62;--shiki-dark:#9ECBFF">git-ur</span><span style="color:#24292E;--shiki-dark:#E1E4E8">l</span><span style="color:#D73A49;--shiki-dark:#F97583">&gt;</span><span style="color:#6A737D;--shiki-dark:#6A737D">     # Update an already-installed package</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> install</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> --uninstall</span><span style="color:#D73A49;--shiki-dark:#F97583"> &lt;</span><span style="color:#032F62;--shiki-dark:#9ECBFF">pk</span><span style="color:#24292E;--shiki-dark:#E1E4E8">g</span><span style="color:#D73A49;--shiki-dark:#F97583">&gt;</span><span style="color:#6A737D;--shiki-dark:#6A737D"> # Remove an installed package</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> install</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> --all</span><span style="color:#6A737D;--shiki-dark:#6A737D">            # Install all extensions without prompting</span></span></code></pre>
<h2 id="skills"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#skills"><span class="icon icon-link"></span></a>Skills</h2>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> skill</span><span style="color:#6A737D;--shiki-dark:#6A737D">                    # Install the Kit extensions skill via skills.sh</span></span></code></pre>
<h2 id="interactive-slash-commands"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#interactive-slash-commands"><span class="icon icon-link"></span></a>Interactive slash commands</h2>
<p>These commands are available inside the Kit TUI during an interactive session:</p>
<table>
<thead>
<tr>
<th>Command</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td><code>/help</code></td>
<td>Show available commands</td>
</tr>
<tr>
<td><code>/tools</code></td>
<td>List available MCP tools</td>
</tr>
<tr>
<td><code>/servers</code></td>
<td>Show connected MCP servers</td>
</tr>
<tr>
<td><code>/model [name]</code></td>
<td>Switch model or open model selector</td>
</tr>
<tr>
<td><code>/theme [name]</code></td>
<td>Switch color theme or list available themes</td>
</tr>
<tr>
<td><code>/thinking [level]</code></td>
<td>Set thinking level (off, none, minimal, low, medium, high)</td>
</tr>
<tr>
<td><code>/compact [focus]</code></td>
<td>Summarize older messages to free context</td>
</tr>
<tr>
<td><code>/clear</code></td>
<td>Clear conversation</td>
</tr>
<tr>
<td><code>/clear-queue</code></td>
<td>Clear queued messages</td>
</tr>
<tr>
<td><code>/usage</code></td>
<td>Show token usage</td>
</tr>
<tr>
<td><code>/reset-usage</code></td>
<td>Reset usage statistics</td>
</tr>
<tr>
<td><code>/tree</code></td>
<td>Navigate session tree</td>
</tr>
<tr>
<td><code>/fork</code></td>
<td>Fork to new session from an earlier message</td>
</tr>
<tr>
<td><code>/new</code></td>
<td>Start a new session (creates new session file)</td>
</tr>
<tr>
<td><code>/name [name]</code></td>
<td>Set or show session display name</td>
</tr>
<tr>
<td><code>/resume</code></td>
<td>Open session picker to switch sessions (alias: <code>/r</code>)</td>
</tr>
<tr>
<td><code>/session</code></td>
<td>Show session info</td>
</tr>
<tr>
<td><code>/export [path]</code></td>
<td>Export session as JSONL (default: auto-generated path)</td>
</tr>
<tr>
<td><code>/import &lt;path&gt;</code></td>
<td>Import a session from a JSONL file</td>
</tr>
<tr>
<td><code>/share</code></td>
<td>Upload session to GitHub Gist and get a shareable viewer URL</td>
</tr>
<tr>
<td><code>/quit</code></td>
<td>Exit Kit</td>
</tr>
</tbody>
</table>
<h3 id="prompt-history"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#prompt-history"><span class="icon icon-link"></span></a>Prompt history</h3>
<p>Use <strong>↑</strong> and <strong>↓</strong> arrow keys to navigate through previously submitted prompts. Kit keeps the last 100 entries. Consecutive duplicates are skipped.</p>
<h3 id="cancelling-operations"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#cancelling-operations"><span class="icon icon-link"></span></a>Cancelling operations</h3>
<p>Press <strong>ESC twice</strong> to cancel the current operation:</p>
<ul>
<li>During a tool call: rolls back the entire turn to maintain API message pairing</li>
<li>During streaming: stops the response generation</li>
</ul>
<p>This ensures that <code>tool_use</code> and <code>tool_result</code> messages are always sent to the API as matched pairs, avoiding errors from orphaned tool calls.</p>
<h3 id="external-editor"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#external-editor"><span class="icon icon-link"></span></a>External editor</h3>
<p>Press <strong>Ctrl+X e</strong> to open your <code>$VISUAL</code> or <code>$EDITOR</code> in a temporary file pre-populated with the current input text. On save and quit, the edited content replaces the input textarea. On error exit (e.g., <code>:cq</code> in Vim), the original input is preserved.</p>
<h3 id="mid-turn-steering"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#mid-turn-steering"><span class="icon icon-link"></span></a>Mid-turn steering</h3>
<p>Press <strong>Ctrl+X s</strong> during streaming to inject a system-level instruction mid-turn. This allows you to steer the conversation direction without waiting for the model to finish:</p>
<ul>
<li>Works during streaming output</li>
<li>Sends a steering instruction as a system message</li>
<li>Model continues from the interruption point with the new guidance</li>
</ul>
<p>Example: While the model is writing code, press Ctrl+X s and type "Use async/await instead" to change the implementation approach.</p>
<h2 id="prompt-templates"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#prompt-templates"><span class="icon icon-link"></span></a>Prompt templates</h2>
<h3 id="creating-templates"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#creating-templates"><span class="icon icon-link"></span></a>Creating templates</h3>
<p>Templates use YAML frontmatter for metadata and support argument placeholders:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">---</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">description</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">Review code for issues</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">---</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">Review the following code for bugs and security issues.</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">Focus on $1 specifically.</span></span></code></pre>
<p>Save to <code>~/.kit/prompts/review.md</code> or <code>.kit/prompts/review.md</code>.</p>
<h3 id="using-templates"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#using-templates"><span class="icon icon-link"></span></a>Using templates</h3>
<p>Templates appear as slash commands:</p>
<pre><code>/review error handling
</code></pre>
<h3 id="argument-placeholders"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#argument-placeholders"><span class="icon icon-link"></span></a>Argument placeholders</h3>
<table>
<thead>
<tr>
<th>Placeholder</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td><code>$1</code>, <code>$2</code>, etc.</td>
<td>Individual arguments by position</td>
</tr>
<tr>
<td><code>$@</code>, <code>$ARGUMENTS</code></td>
<td>All arguments joined with spaces (zero or more)</td>
</tr>
<tr>
<td><code>$+</code></td>
<td>All arguments joined with spaces (one or more required)</td>
</tr>
<tr>
<td><code>\${@:N}</code></td>
<td>Arguments from position N onwards</td>
</tr>
<tr>
<td><code>\${@:N:L}</code></td>
<td>L arguments starting at position N</td>
</tr>
</tbody>
</table>
<p>Placeholders inside fenced code blocks (<code>\`\`\`</code>) and inline code spans are ignored, so documentation examples won't be substituted.</p>
<h3 id="cli-flags"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#cli-flags"><span class="icon icon-link"></span></a>CLI flags</h3>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D"># Load a specific template by name</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> --prompt-template</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> review</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D"># Disable template loading</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> --no-prompt-templates</span></span></code></pre>
<h2 id="acp-server"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#acp-server"><span class="icon icon-link"></span></a>ACP server</h2>
<p>Run Kit as an <a href="https://agentclientprotocol.com">ACP (Agent Client Protocol)</a> agent server. ACP-compatible clients communicate with Kit over JSON-RPC 2.0 on stdin/stdout.</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> acp</span><span style="color:#6A737D;--shiki-dark:#6A737D">                      # Start as ACP agent</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> acp</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> --debug</span><span style="color:#6A737D;--shiki-dark:#6A737D">              # With debug logging to stderr</span></span></code></pre>`,headings:[{depth:2,text:"Authentication",id:"authentication"},{depth:2,text:"Model database",id:"model-database"},{depth:2,text:"Extension management",id:"extension-management"},{depth:3,text:"Installing extensions from git",id:"installing-extensions-from-git"},{depth:2,text:"Skills",id:"skills"},{depth:2,text:"Interactive slash commands",id:"interactive-slash-commands"},{depth:3,text:"Prompt history",id:"prompt-history"},{depth:3,text:"Cancelling operations",id:"cancelling-operations"},{depth:3,text:"External editor",id:"external-editor"},{depth:3,text:"Mid-turn steering",id:"mid-turn-steering"},{depth:2,text:"Prompt templates",id:"prompt-templates"},{depth:3,text:"Creating templates",id:"creating-templates"},{depth:3,text:"Using templates",id:"using-templates"},{depth:3,text:"Argument placeholders",id:"argument-placeholders"},{depth:3,text:"CLI flags",id:"cli-flags"},{depth:2,text:"ACP server",id:"acp-server"}],raw:`
# Commands

## Authentication

For OAuth-enabled providers like Anthropic.

\`\`\`bash
kit auth login [provider]    # Start OAuth flow (e.g., anthropic)
kit auth logout [provider]   # Remove credentials for provider
kit auth status              # Check authentication status
\`\`\`

## Model database

Manage the local model database that maps provider names to API configurations.

\`\`\`bash
kit models [provider]        # List available models (optionally filter by provider)
kit models --all             # Show all providers (not just LLM-compatible)
kit update-models [source]   # Update model database
\`\`\`

The \`update-models\` command accepts an optional source argument:
- *(none)* — update from [models.dev](https://models.dev)
- A URL — fetch from a custom endpoint
- A file path — load from a local file
- \`embedded\` — reset to the bundled database

## Extension management

\`\`\`bash
kit extensions list          # List discovered extensions
kit extensions validate      # Validate extension files
kit extensions init          # Generate example extension template
\`\`\`

### Installing extensions from git

\`\`\`bash
kit install <git-url>        # Install extensions from git repositories
kit install -l <git-url>     # Install to project-local .kit/git/ directory
kit install -u <git-url>     # Update an already-installed package
kit install --uninstall <pkg> # Remove an installed package
kit install --all            # Install all extensions without prompting
\`\`\`

## Skills

\`\`\`bash
kit skill                    # Install the Kit extensions skill via skills.sh
\`\`\`

## Interactive slash commands

These commands are available inside the Kit TUI during an interactive session:

| Command | Description |
|---------|-------------|
| \`/help\` | Show available commands |
| \`/tools\` | List available MCP tools |
| \`/servers\` | Show connected MCP servers |
| \`/model [name]\` | Switch model or open model selector |
| \`/theme [name]\` | Switch color theme or list available themes |
| \`/thinking [level]\` | Set thinking level (off, none, minimal, low, medium, high) |
| \`/compact [focus]\` | Summarize older messages to free context |
| \`/clear\` | Clear conversation |
| \`/clear-queue\` | Clear queued messages |
| \`/usage\` | Show token usage |
| \`/reset-usage\` | Reset usage statistics |
| \`/tree\` | Navigate session tree |
| \`/fork\` | Fork to new session from an earlier message |
| \`/new\` | Start a new session (creates new session file) |
| \`/name [name]\` | Set or show session display name |
| \`/resume\` | Open session picker to switch sessions (alias: \`/r\`) |
| \`/session\` | Show session info |
| \`/export [path]\` | Export session as JSONL (default: auto-generated path) |
| \`/import <path>\` | Import a session from a JSONL file |
| \`/share\` | Upload session to GitHub Gist and get a shareable viewer URL |
| \`/quit\` | Exit Kit |

### Prompt history

Use **↑** and **↓** arrow keys to navigate through previously submitted prompts. Kit keeps the last 100 entries. Consecutive duplicates are skipped.

### Cancelling operations

Press **ESC twice** to cancel the current operation:
- During a tool call: rolls back the entire turn to maintain API message pairing
- During streaming: stops the response generation

This ensures that \`tool_use\` and \`tool_result\` messages are always sent to the API as matched pairs, avoiding errors from orphaned tool calls.

### External editor

Press **Ctrl+X e** to open your \`$VISUAL\` or \`$EDITOR\` in a temporary file pre-populated with the current input text. On save and quit, the edited content replaces the input textarea. On error exit (e.g., \`:cq\` in Vim), the original input is preserved.

### Mid-turn steering

Press **Ctrl+X s** during streaming to inject a system-level instruction mid-turn. This allows you to steer the conversation direction without waiting for the model to finish:

- Works during streaming output
- Sends a steering instruction as a system message
- Model continues from the interruption point with the new guidance

Example: While the model is writing code, press Ctrl+X s and type "Use async/await instead" to change the implementation approach.

## Prompt templates

### Creating templates

Templates use YAML frontmatter for metadata and support argument placeholders:

\`\`\`markdown
---
description: Review code for issues
---
Review the following code for bugs and security issues.
Focus on $1 specifically.
\`\`\`

Save to \`~/.kit/prompts/review.md\` or \`.kit/prompts/review.md\`.

### Using templates

Templates appear as slash commands:

\`\`\`
/review error handling
\`\`\`

### Argument placeholders

| Placeholder | Description |
|-------------|-------------|
| \`$1\`, \`$2\`, etc. | Individual arguments by position |
| \`$@\`, \`$ARGUMENTS\` | All arguments joined with spaces (zero or more) |
| \`$+\` | All arguments joined with spaces (one or more required) |
| \`\${@:N}\` | Arguments from position N onwards |
| \`\${@:N:L}\` | L arguments starting at position N |

Placeholders inside fenced code blocks (\`\` \`\`\` \`\`) and inline code spans are ignored, so documentation examples won't be substituted.

### CLI flags

\`\`\`bash
# Load a specific template by name
kit --prompt-template review

# Disable template loading
kit --no-prompt-templates
\`\`\`

## ACP server

Run Kit as an [ACP (Agent Client Protocol)](https://agentclientprotocol.com) agent server. ACP-compatible clients communicate with Kit over JSON-RPC 2.0 on stdin/stdout.

\`\`\`bash
kit acp                      # Start as ACP agent
kit acp --debug              # With debug logging to stderr
\`\`\`
`};export{e as default};

const s={frontmatter:{title:"Commands",description:"Complete reference for all Kit CLI subcommands.",hidden:!1,toc:!0,draft:!1},html:`<h1 id="commands"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#commands"><span class="icon icon-link"></span></a>Commands</h1>
<h2 id="authentication"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#authentication"><span class="icon icon-link"></span></a>Authentication</h2>
<p>For OAuth-enabled providers like Anthropic.</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> auth</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> login</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> [provider]    </span><span style="color:#6A737D;--shiki-dark:#6A737D"># Start OAuth flow (e.g., anthropic)</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> auth</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> logout</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> [provider]   </span><span style="color:#6A737D;--shiki-dark:#6A737D"># Remove credentials for provider</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> auth</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> status</span><span style="color:#6A737D;--shiki-dark:#6A737D">              # Check authentication status</span></span></code></pre>
<h2 id="model-database"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#model-database"><span class="icon icon-link"></span></a>Model database</h2>
<p>Manage the local model database that maps provider names to API configurations.</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> models</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> [provider]        </span><span style="color:#6A737D;--shiki-dark:#6A737D"># List available models (optionally filter by provider)</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> models</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> --all</span><span style="color:#6A737D;--shiki-dark:#6A737D">             # Show all providers (not just Fantasy-compatible)</span></span>
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
<td>Set thinking level (off, minimal, low, medium, high)</td>
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
<td>Branch from an earlier message</td>
</tr>
<tr>
<td><code>/new</code></td>
<td>Start a new session</td>
</tr>
<tr>
<td><code>/name</code></td>
<td>Set session display name</td>
</tr>
<tr>
<td><code>/session</code></td>
<td>Show session info</td>
</tr>
<tr>
<td><code>/quit</code></td>
<td>Exit Kit</td>
</tr>
</tbody>
</table>
<h2 id="acp-server"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#acp-server"><span class="icon icon-link"></span></a>ACP server</h2>
<p>Run Kit as an <a href="https://agentclientprotocol.com">ACP (Agent Client Protocol)</a> agent server. ACP-compatible clients communicate with Kit over JSON-RPC 2.0 on stdin/stdout.</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> acp</span><span style="color:#6A737D;--shiki-dark:#6A737D">                      # Start as ACP agent</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> acp</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> --debug</span><span style="color:#6A737D;--shiki-dark:#6A737D">              # With debug logging to stderr</span></span></code></pre>`,headings:[{depth:2,text:"Authentication",id:"authentication"},{depth:2,text:"Model database",id:"model-database"},{depth:2,text:"Extension management",id:"extension-management"},{depth:3,text:"Installing extensions from git",id:"installing-extensions-from-git"},{depth:2,text:"Skills",id:"skills"},{depth:2,text:"Interactive slash commands",id:"interactive-slash-commands"},{depth:2,text:"ACP server",id:"acp-server"}],raw:`
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
kit models --all             # Show all providers (not just Fantasy-compatible)
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
| \`/thinking [level]\` | Set thinking level (off, minimal, low, medium, high) |
| \`/compact [focus]\` | Summarize older messages to free context |
| \`/clear\` | Clear conversation |
| \`/clear-queue\` | Clear queued messages |
| \`/usage\` | Show token usage |
| \`/reset-usage\` | Reset usage statistics |
| \`/tree\` | Navigate session tree |
| \`/fork\` | Branch from an earlier message |
| \`/new\` | Start a new session |
| \`/name\` | Set session display name |
| \`/session\` | Show session info |
| \`/quit\` | Exit Kit |

## ACP server

Run Kit as an [ACP (Agent Client Protocol)](https://agentclientprotocol.com) agent server. ACP-compatible clients communicate with Kit over JSON-RPC 2.0 on stdin/stdout.

\`\`\`bash
kit acp                      # Start as ACP agent
kit acp --debug              # With debug logging to stderr
\`\`\`
`};export{s as default};

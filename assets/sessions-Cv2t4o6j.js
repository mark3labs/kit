const e={frontmatter:{title:"Session Management",description:"How Kit persists and manages conversation sessions.",hidden:!1,toc:!0,draft:!1},html:`<h1 id="session-management"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#session-management"><span class="icon icon-link"></span></a>Session Management</h1>
<p>Kit uses a tree-based session model that supports branching and forking conversations.</p>
<h2 id="session-storage"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#session-storage"><span class="icon icon-link"></span></a>Session storage</h2>
<p>Sessions are stored as JSONL (JSON Lines) files:</p>
<pre><code>~/.kit/sessions/&lt;cwd-path&gt;/&lt;timestamp&gt;_&lt;id&gt;.jsonl
</code></pre>
<p>Path separators in the working directory are replaced with <code>--</code>. For example, <code>/home/user/project</code> becomes <code>home--user--project</code>.</p>
<p>Each line in the session file is a JSON entry representing a message, tool call, model change, or extension data. The tree structure allows branching from any message to explore alternate paths.</p>
<h2 id="compaction"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#compaction"><span class="icon icon-link"></span></a>Compaction</h2>
<p>When conversations grow long, Kit can compact them to free up context window space. The compaction system:</p>
<ul>
<li><strong>Non-destructive</strong>: Old messages remain on disk for history; only the LLM context is summarized</li>
<li><strong>File tracking</strong>: Tracks which files were read and modified across compactions</li>
<li><strong>Split-turn handling</strong>: Can summarize large single turns by splitting them</li>
<li><strong>Tool result truncation</strong>: Caps tool output during serialization to stay within token budgets</li>
</ul>
<p>Use <code>/compact [focus]</code> to manually compact, or enable <code>--auto-compact</code> to compact automatically near the context limit.</p>
<h2 id="auto-cleanup"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#auto-cleanup"><span class="icon icon-link"></span></a>Auto-cleanup</h2>
<p>Kit automatically cleans up empty sessions on shutdown and when using <code>/resume</code>. A session is considered empty if it has no messages beyond the initial system prompt. This prevents cluttering your sessions directory with unused files.</p>
<p>To start fresh without creating a session file at all, use ephemeral mode:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> --no-session</span></span></code></pre>
<h2 id="resuming-sessions"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#resuming-sessions"><span class="icon icon-link"></span></a>Resuming sessions</h2>
<h3 id="continue-most-recent"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#continue-most-recent"><span class="icon icon-link"></span></a>Continue most recent</h3>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> --continue</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> -c</span></span></code></pre>
<h3 id="interactive-picker"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#interactive-picker"><span class="icon icon-link"></span></a>Interactive picker</h3>
<p>Choose from previous sessions interactively:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> --resume</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> -r</span></span></code></pre>
<p>The session picker supports search, scope/filter toggles (all sessions vs. current directory), and session deletion. You can also open it during a session with the <code>/resume</code> slash command.</p>
<h3 id="open-a-specific-session"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#open-a-specific-session"><span class="icon icon-link"></span></a>Open a specific session</h3>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> --session</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> path/to/session.jsonl</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> -s</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> path/to/session.jsonl</span></span></code></pre>
<h2 id="session-commands"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#session-commands"><span class="icon icon-link"></span></a>Session commands</h2>
<p>These slash commands are available during an interactive session:</p>
<table>
<thead>
<tr>
<th>Command</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td><code>/name [name]</code></td>
<td>Set or display the session's display name</td>
</tr>
<tr>
<td><code>/session</code></td>
<td>Show session info (path, ID, message count)</td>
</tr>
<tr>
<td><code>/resume</code></td>
<td>Open the session picker to switch sessions</td>
</tr>
<tr>
<td><code>/export [path]</code></td>
<td>Export session as JSONL (auto-generates path if omitted)</td>
</tr>
<tr>
<td><code>/import &lt;path&gt;</code></td>
<td>Import and switch to a session from a JSONL file</td>
</tr>
<tr>
<td><code>/share</code></td>
<td>Upload session to GitHub Gist and get a shareable viewer URL</td>
</tr>
<tr>
<td><code>/tree</code></td>
<td>Navigate the session tree</td>
</tr>
<tr>
<td><code>/fork</code></td>
<td>Fork to new session from an earlier message (creates new session file)</td>
</tr>
<tr>
<td><code>/new</code></td>
<td>Start a new session (creates new session file)</td>
</tr>
</tbody>
</table>
<h2 id="ephemeral-mode"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#ephemeral-mode"><span class="icon icon-link"></span></a>Ephemeral mode</h2>
<p>Run without creating a session file:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> --no-session</span></span></code></pre>
<p>This is useful for one-off prompts, scripting, and subagent patterns where persistence isn't needed.</p>
<h2 id="sharing-sessions"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#sharing-sessions"><span class="icon icon-link"></span></a>Sharing sessions</h2>
<p>The <code>/share</code> command uploads your session JSONL to GitHub Gist (via the <code>gh</code> CLI) and prints a shareable viewer URL:</p>
<pre><code>/share
</code></pre>
<p>The viewer is available at <code>https://go-kit.dev/session/#GIST_ID</code> and supports all message types including text, reasoning blocks, tool calls, images, and model changes.</p>
<p>You can also load any JSONL session via URL parameter: <code>https://go-kit.dev/session/?url=https://example.com/session.jsonl</code></p>
<h2 id="preferences-persistence"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#preferences-persistence"><span class="icon icon-link"></span></a>Preferences persistence</h2>
<p>Kit automatically saves your preferences across sessions to <code>~/.config/kit/preferences.yml</code>:</p>
<ul>
<li><strong>Theme</strong> — Set via <code>/theme &lt;name&gt;</code></li>
<li><strong>Model</strong> — Set via <code>/model &lt;name&gt;</code> or the model selector</li>
<li><strong>Thinking level</strong> — Set via <code>/thinking &lt;level&gt;</code> or Shift+Tab cycling</li>
</ul>
<p>These preferences are restored on next launch. Precedence: CLI flag &gt; config file &gt; saved preference &gt; default.</p>`,headings:[{depth:2,text:"Session storage",id:"session-storage"},{depth:2,text:"Compaction",id:"compaction"},{depth:2,text:"Auto-cleanup",id:"auto-cleanup"},{depth:2,text:"Resuming sessions",id:"resuming-sessions"},{depth:3,text:"Continue most recent",id:"continue-most-recent"},{depth:3,text:"Interactive picker",id:"interactive-picker"},{depth:3,text:"Open a specific session",id:"open-a-specific-session"},{depth:2,text:"Session commands",id:"session-commands"},{depth:2,text:"Ephemeral mode",id:"ephemeral-mode"},{depth:2,text:"Sharing sessions",id:"sharing-sessions"},{depth:2,text:"Preferences persistence",id:"preferences-persistence"}],raw:`
# Session Management

Kit uses a tree-based session model that supports branching and forking conversations.

## Session storage

Sessions are stored as JSONL (JSON Lines) files:

\`\`\`
~/.kit/sessions/<cwd-path>/<timestamp>_<id>.jsonl
\`\`\`

Path separators in the working directory are replaced with \`--\`. For example, \`/home/user/project\` becomes \`home--user--project\`.

Each line in the session file is a JSON entry representing a message, tool call, model change, or extension data. The tree structure allows branching from any message to explore alternate paths.

## Compaction

When conversations grow long, Kit can compact them to free up context window space. The compaction system:

- **Non-destructive**: Old messages remain on disk for history; only the LLM context is summarized
- **File tracking**: Tracks which files were read and modified across compactions
- **Split-turn handling**: Can summarize large single turns by splitting them
- **Tool result truncation**: Caps tool output during serialization to stay within token budgets

Use \`/compact [focus]\` to manually compact, or enable \`--auto-compact\` to compact automatically near the context limit.

## Auto-cleanup

Kit automatically cleans up empty sessions on shutdown and when using \`/resume\`. A session is considered empty if it has no messages beyond the initial system prompt. This prevents cluttering your sessions directory with unused files.

To start fresh without creating a session file at all, use ephemeral mode:

\`\`\`bash
kit --no-session
\`\`\`

## Resuming sessions

### Continue most recent

\`\`\`bash
kit --continue
kit -c
\`\`\`

### Interactive picker

Choose from previous sessions interactively:

\`\`\`bash
kit --resume
kit -r
\`\`\`

The session picker supports search, scope/filter toggles (all sessions vs. current directory), and session deletion. You can also open it during a session with the \`/resume\` slash command.

### Open a specific session

\`\`\`bash
kit --session path/to/session.jsonl
kit -s path/to/session.jsonl
\`\`\`

## Session commands

These slash commands are available during an interactive session:

| Command | Description |
|---------|-------------|
| \`/name [name]\` | Set or display the session's display name |
| \`/session\` | Show session info (path, ID, message count) |
| \`/resume\` | Open the session picker to switch sessions |
| \`/export [path]\` | Export session as JSONL (auto-generates path if omitted) |
| \`/import <path>\` | Import and switch to a session from a JSONL file |
| \`/share\` | Upload session to GitHub Gist and get a shareable viewer URL |
| \`/tree\` | Navigate the session tree |
| \`/fork\` | Fork to new session from an earlier message (creates new session file) |
| \`/new\` | Start a new session (creates new session file) |

## Ephemeral mode

Run without creating a session file:

\`\`\`bash
kit --no-session
\`\`\`

This is useful for one-off prompts, scripting, and subagent patterns where persistence isn't needed.

## Sharing sessions

The \`/share\` command uploads your session JSONL to GitHub Gist (via the \`gh\` CLI) and prints a shareable viewer URL:

\`\`\`
/share
\`\`\`

The viewer is available at \`https://go-kit.dev/session/#GIST_ID\` and supports all message types including text, reasoning blocks, tool calls, images, and model changes.

You can also load any JSONL session via URL parameter: \`https://go-kit.dev/session/?url=https://example.com/session.jsonl\`

## Preferences persistence

Kit automatically saves your preferences across sessions to \`~/.config/kit/preferences.yml\`:

- **Theme** — Set via \`/theme <name>\`
- **Model** — Set via \`/model <name>\` or the model selector
- **Thinking level** — Set via \`/thinking <level>\` or Shift+Tab cycling

These preferences are restored on next launch. Precedence: CLI flag > config file > saved preference > default.
`};export{e as default};

const s={frontmatter:{title:"Session Management",description:"How Kit persists and manages conversation sessions.",hidden:!1,toc:!0,draft:!1},html:`<h1 id="session-management"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#session-management"><span class="icon icon-link"></span></a>Session Management</h1>
<p>Kit uses a tree-based session model that supports branching and forking conversations.</p>
<h2 id="session-storage"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#session-storage"><span class="icon icon-link"></span></a>Session storage</h2>
<p>Sessions are stored as JSONL (JSON Lines) files:</p>
<pre><code>~/.kit/sessions/&lt;cwd-path&gt;/&lt;timestamp&gt;_&lt;id&gt;.jsonl
</code></pre>
<p>Path separators in the working directory are replaced with <code>--</code>. For example, <code>/home/user/project</code> becomes <code>home--user--project</code>.</p>
<p>Each line in the session file is a JSON entry representing a message, tool call, model change, or extension data. The tree structure allows branching from any message to explore alternate paths.</p>
<h2 id="resuming-sessions"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#resuming-sessions"><span class="icon icon-link"></span></a>Resuming sessions</h2>
<h3 id="continue-most-recent"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#continue-most-recent"><span class="icon icon-link"></span></a>Continue most recent</h3>
<p>Resume the most recent session for the current directory:</p>
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
<td><code>/tree</code></td>
<td>Navigate the session tree</td>
</tr>
<tr>
<td><code>/fork</code></td>
<td>Branch from an earlier message</td>
</tr>
<tr>
<td><code>/new</code></td>
<td>Start a fresh session</td>
</tr>
</tbody>
</table>
<h2 id="ephemeral-mode"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#ephemeral-mode"><span class="icon icon-link"></span></a>Ephemeral mode</h2>
<p>Run without creating a session file:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> --no-session</span></span></code></pre>
<p>This is useful for one-off prompts, scripting, and subagent patterns where persistence isn't needed.</p>`,headings:[{depth:2,text:"Session storage",id:"session-storage"},{depth:2,text:"Resuming sessions",id:"resuming-sessions"},{depth:3,text:"Continue most recent",id:"continue-most-recent"},{depth:3,text:"Interactive picker",id:"interactive-picker"},{depth:3,text:"Open a specific session",id:"open-a-specific-session"},{depth:2,text:"Session commands",id:"session-commands"},{depth:2,text:"Ephemeral mode",id:"ephemeral-mode"}],raw:`
# Session Management

Kit uses a tree-based session model that supports branching and forking conversations.

## Session storage

Sessions are stored as JSONL (JSON Lines) files:

\`\`\`
~/.kit/sessions/<cwd-path>/<timestamp>_<id>.jsonl
\`\`\`

Path separators in the working directory are replaced with \`--\`. For example, \`/home/user/project\` becomes \`home--user--project\`.

Each line in the session file is a JSON entry representing a message, tool call, model change, or extension data. The tree structure allows branching from any message to explore alternate paths.

## Resuming sessions

### Continue most recent

Resume the most recent session for the current directory:

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
| \`/tree\` | Navigate the session tree |
| \`/fork\` | Branch from an earlier message |
| \`/new\` | Start a fresh session |

## Ephemeral mode

Run without creating a session file:

\`\`\`bash
kit --no-session
\`\`\`

This is useful for one-off prompts, scripting, and subagent patterns where persistence isn't needed.
`};export{s as default};

var e={frontmatter:{title:`Commands`,description:`Complete reference for all Kit CLI subcommands.`,hidden:!1,toc:!0,draft:!1},html:`<h1 id="commands"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#commands"><span class="icon icon-link"></span></a>Commands</h1>
<h2 id="authentication"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#authentication"><span class="icon icon-link"></span></a>Authentication</h2>
<p>Anthropic, OpenAI and GitHub Copilot use OAuth flows. Every other provider
in the model database takes an API key.</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> auth</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> login</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> [provider]          </span><span style="color:#6A737D;--shiki-dark:#6A737D"># Start OAuth flow (e.g., anthropic) or prompt for an API key</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> auth</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> login</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> groq</span><span style="color:#6A737D;--shiki-dark:#6A737D">                # Prompt for a Groq API key (hidden input)</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> auth</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> login</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> openrouter</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> --api-key</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> sk-or-...</span><span style="color:#6A737D;--shiki-dark:#6A737D">   # Store a key without prompting</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> auth</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> login</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> [provider] --set-default  </span><span style="color:#6A737D;--shiki-dark:#6A737D"># Set provider's default model as system default</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> auth</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> logout</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> [provider]         </span><span style="color:#6A737D;--shiki-dark:#6A737D"># Remove credentials for provider</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> auth</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> status</span><span style="color:#6A737D;--shiki-dark:#6A737D">                    # Check authentication status</span></span></code></pre>
<p>Stored keys live in <code>$XDG_CONFIG_HOME/.kit/credentials.json</code> (defaults to
<code>~/.config/.kit/credentials.json</code>, mode <code>0600</code>) and take precedence over the
provider's environment variable (for example <code>GROQ_API_KEY</code>). The
environment variable is still used when no key is stored.</p>
<p>Kit starts even when the configured model has no key. The TUI shows a notice
and refuses to send prompts until you add a key with <code>/connect</code> (or switch
to a model whose provider has one with <code>/model</code>). One-shot prompts
(<code>kit "question"</code>) still fail fast with the same error.</p>
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
<p>Kit records the schema version of the on-disk cache. When Kit's binary
understands catalog fields the cache was written without — reasoning options,
long-context pricing tiers, deprecation status — the older cache is ignored
in favour of the embedded snapshot, so a stale cache never masks metadata the
current binary can use. Run <code>kit update-models</code> once to rewrite the cache in
the current schema.</p>
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
<p>Kit implements the <a href="https://agentskills.io/specification">Agent Skills</a> format. A skill is a directory with a <code>SKILL.md</code> file (YAML frontmatter + Markdown instructions) and optional <code>scripts/</code>, <code>references/</code>, and <code>assets/</code> subdirectories.</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> skill</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> list</span><span style="color:#6A737D;--shiki-dark:#6A737D">               # List discovered skills with scope, path and spec warnings</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> skill</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> validate</span><span style="color:#D73A49;--shiki-dark:#F97583"> &lt;</span><span style="color:#032F62;--shiki-dark:#9ECBFF">pat</span><span style="color:#24292E;--shiki-dark:#E1E4E8">h</span><span style="color:#D73A49;--shiki-dark:#F97583">&gt;</span><span style="color:#6A737D;--shiki-dark:#6A737D">    # Validate a skill directory / SKILL.md against the spec</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> skill</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> install</span><span style="color:#6A737D;--shiki-dark:#6A737D">            # Install the Kit skills (kit-extensions, kit-sdk) via skills.sh</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> skill</span><span style="color:#6A737D;--shiki-dark:#6A737D">                    # Same as "kit skill install"</span></span></code></pre>
<p><code>kit skill list</code> honors <code>--skill</code>, <code>--skills-dir</code>, <code>--no-skills</code> and <code>--bare</code>, and never prompts for project trust (it is read-only). <code>kit skill validate</code> accepts a <code>SKILL.md</code> file, a skill directory, or a directory of skills; errors (missing <code>name</code>/<code>description</code>) make the exit code non-zero, while warnings (name format, length limits, name/directory mismatch, body over 500 lines) are informational.</p>
<h3 id="activating-skills"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#activating-skills"><span class="icon icon-link"></span></a>Activating skills</h3>
<p>Skills load in three tiers, following the spec's progressive-disclosure model:</p>
<ol>
<li><strong>Catalog</strong> — at startup the model sees every skill's <code>name</code>, <code>description</code> and location in an <code>&lt;available_skills&gt;</code> block in the system prompt.</li>
<li><strong>Instructions</strong> — the full <code>SKILL.md</code> body is loaded when a skill is activated, either by the model (the <code>activate_skill</code> tool, or a <code>read</code> of the listed location) or by you.</li>
<li><strong>Resources</strong> — bundled files are listed in a <code>&lt;skill_resources&gt;</code> block and read on demand.</li>
</ol>
<p>To activate a skill yourself, type its name as a slash command, optionally followed by a request:</p>
<pre><code>/pdf-processing extract the tables from report.pdf
</code></pre>
<p>The <code>/</code> autocomplete popup lists every loaded skill that is not shadowed by another slash command (see precedence below) with a <code>[skill]</code> badge next to it — hidden-from-model skills included, since hiding only affects the model-facing catalog (prompt templates show <code>[prompt]</code>, extension commands <code>[ext]</code>, MCP prompts <code>[mcp]</code>). Kit wraps the skill body in a <code>&lt;skill_content&gt;</code> block, appends your text, and sends it as the turn. Activated skill content is protected from <code>/compact</code> pruning.</p>
<p>Slash-name precedence is: built-in commands, extension commands, MCP prompts (<code>/server:prompt</code>), prompt templates, then skills. A skill whose name is already taken is left out of the popup and a warning is logged, so rename one side if that happens.</p>
<h3 id="skills-cli-flags"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#skills-cli-flags"><span class="icon icon-link"></span></a>Skills CLI flags</h3>
<p>Control which skills are loaded at startup:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D"># Load a specific skill file</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> --skill</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> path/to/skill.md</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> "prompt"</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D"># Load multiple skill files or directories (flag is repeatable)</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> --skill</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> ./skill1.md</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> --skill</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> ./skill2.md</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> "prompt"</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D"># Scan a directory directly for skills (overrides auto-discovery)</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> --skills-dir</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> /path/to/skills</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> "prompt"</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D"># Hide a skill from the model catalog by name (still usable via /&lt;name&gt;)</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> --skill-disable</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> noisy-skill</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> "prompt"</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D"># Disable all skill loading (auto-discovery and explicit)</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> --no-skills</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> "prompt"</span></span></code></pre>
<p>Skills follow the <a href="https://agentskills.io/specification">agentskills.io</a> convention. They are auto-discovered from four canonical scopes:</p>
<table>
<thead>
<tr>
<th>Scope</th>
<th>Location</th>
</tr>
</thead>
<tbody>
<tr>
<td>User-level (cross-client)</td>
<td><code>~/.agents/skills/</code></td>
</tr>
<tr>
<td>User-level (Kit)</td>
<td><code>~/.config/kit/skills/</code> (honors <code>$XDG_CONFIG_HOME</code>)</td>
</tr>
<tr>
<td>Project-local (cross-client)</td>
<td><code>&lt;project&gt;/.agents/skills/</code></td>
</tr>
<tr>
<td>Project-local (Kit)</td>
<td><code>&lt;project&gt;/.kit/skills/</code></td>
</tr>
</tbody>
</table>
<p>When two skills share the same <code>name</code>, the project-level one takes precedence over the user-level one. Use <code>--skills-dir</code> to scan one directory directly instead (it is <strong>not</strong> treated as a parent of <code>.agents</code>/<code>.kit</code> — the directory itself is scanned). <code>--skill</code> loads files explicitly (which disables auto-discovery), and <code>--no-skills</code> suppresses all skill loading regardless of other flags.</p>
<p>Disabled skills (<code>--skill-disable</code>, the <code>skill-disable</code> config key, or <code>disable-model-invocation: true</code> in a skill's frontmatter) are hidden from the model-facing <code>&lt;available_skills&gt;</code> catalog but remain available for explicit activation via the <code>/&lt;name&gt;</code> slash command.</p>
<h3 id="skill-frontmatter"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#skill-frontmatter"><span class="icon icon-link"></span></a>Skill frontmatter</h3>
<p>A skill is a markdown file (<code>SKILL.md</code> in a directory, or a standalone <code>.md</code>/<code>.txt</code> file) with optional YAML frontmatter. Kit reads the full <a href="https://agentskills.io/specification">agentskills.io</a> field set plus two Kit-specific extensions:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">---</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">name</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">pdf-extractor</span><span style="color:#6A737D;--shiki-dark:#6A737D">                 # required; lowercase, digits, single hyphens; must match the directory</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">description</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">Use when extracting tables from PDFs</span><span style="color:#6A737D;--shiki-dark:#6A737D">   # required (drives model discovery)</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">license</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">MIT</span><span style="color:#6A737D;--shiki-dark:#6A737D">                        # optional, SPDX identifier</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">compatibility</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">Requires python3 and pdftotext</span><span style="color:#6A737D;--shiki-dark:#6A737D">  # optional, environment requirements (max 500 chars)</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">allowed-tools</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">Read Bash(pdftotext:*)</span><span style="color:#6A737D;--shiki-dark:#6A737D">  # optional (experimental); parsed, not enforced by Kit</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">disable-model-invocation</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#005CC5;--shiki-dark:#79B8FF">false</span><span style="color:#6A737D;--shiki-dark:#6A737D">     # optional; true hides from the catalog</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">metadata</span><span style="color:#24292E;--shiki-dark:#E1E4E8">:                           </span><span style="color:#6A737D;--shiki-dark:#6A737D"># optional arbitrary string key/value pairs</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">  author</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">you</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">tags</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: [</span><span style="color:#032F62;--shiki-dark:#9ECBFF">pdf</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#032F62;--shiki-dark:#9ECBFF">data</span><span style="color:#24292E;--shiki-dark:#E1E4E8">]                   </span><span style="color:#6A737D;--shiki-dark:#6A737D"># Kit extension</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">when</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">on-demand</span><span style="color:#6A737D;--shiki-dark:#6A737D">                     # Kit extension</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">---</span></span></code></pre>
<p><code>name</code> and <code>description</code> are required — a skill missing its description is skipped with a logged warning, since the description is the sole basis on which the model decides relevance. Other spec constraints (name is 1–64 chars of <code>a-z0-9-</code> with no leading/trailing/double hyphen and matches the parent directory; description ≤ 1024 chars; <code>compatibility</code> ≤ 500 chars; body under 500 lines) are checked leniently: the skill still loads, and the deviation is reported by <code>kit skill list</code> / <code>kit skill validate</code>. Descriptions are XML-escaped before they enter the catalog, so characters like <code>&lt;</code>, <code>&gt;</code>, and <code>&amp;</code> are safe. A skill directory may bundle <code>scripts/</code>, <code>references/</code>, and <code>assets/</code> subdirectories; when a skill is activated those files are enumerated in a <code>&lt;skill_resources&gt;</code> block so the model knows what it can read.</p>
<h3 id="project-trust-prompt"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#project-trust-prompt"><span class="icon icon-link"></span></a>Project trust prompt</h3>
<p>Because project-local skills are injected into the system prompt, entering a repository that ships <code>.agents/skills/</code> or <code>.kit/skills/</code> for the first time prompts you to trust it before any project skill loads — a safeguard against a freshly cloned, untrusted repo smuggling instructions into the agent:</p>
<pre><code>This project provides 2 skills under .agents/skills or .kit/skills:
  /path/to/repo
Load them into the agent? [t]rust always / [o]nce / [s]kip (default skip):
</code></pre>
<p>Choosing <strong>trust always</strong> persists the directory to <code>~/.config/kit/trusted-projects.json</code> so you are not asked again. The prompt is skipped (skills load silently) in non-interactive runs — when a prompt is passed positionally, <code>--quiet</code> is set, or stdin is not a TTY.</p>
<h2 id="github-integration"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#github-integration"><span class="icon icon-link"></span></a>GitHub integration</h2>
<p>Scaffold a GitHub Actions workflow that runs Kit as an automated collaborator/reviewer. The workflow triggers when someone comments <code>/kit ...</code> on an issue or pull request review, runs the agent non-interactively in the runner, and lets it respond.</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> github</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> install</span><span style="color:#6A737D;--shiki-dark:#6A737D">           # Scaffold .github/workflows/kit.yml</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> github</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> install</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> --model</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> anthropic/claude-sonnet-4-5-20250929</span><span style="color:#6A737D;--shiki-dark:#6A737D">  # Skip the model prompt</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> github</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> install</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> --force</span><span style="color:#6A737D;--shiki-dark:#6A737D">   # Overwrite an existing workflow file</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> github</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> install</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> --no-secret</span><span style="color:#6A737D;--shiki-dark:#6A737D"> # Skip the offer to set the provider secret via the gh CLI</span></span></code></pre>
<p>By default the command prompts for the model (pre-filled with a sensible default). If the <a href="https://cli.github.com/"><code>gh</code> CLI</a> is detected on your <code>PATH</code> and the provider API key is present in your environment, you'll be offered the option to store it as a repository secret automatically.</p>
<p>The generated workflow:</p>
<ul>
<li>Triggers only on <code>issue_comment</code> and <code>pull_request_review_comment</code> (<code>types: [created]</code>).</li>
<li>Runs only when the comment begins with the <code>/kit</code> command token.</li>
<li>Restricts triggers to repository owners, members, and collaborators (via <code>author_association</code>).</li>
<li>Uses least-privilege <code>permissions</code> and <code>persist-credentials: false</code>.</li>
<li>Authenticates git/PR operations with the built-in <code>secrets.GITHUB_TOKEN</code> and the provider via a repository secret (e.g. <code>ANTHROPIC_API_KEY</code>).</li>
</ul>
<p>After committing the workflow and setting the provider secret, comment <code>/kit &lt;your request&gt;</code> on any issue or pull request to trigger Kit.</p>
<p>The generated workflow uses the bundled <a href="https://github.com/mark3labs/kit/blob/master/action.yml"><code>mark3labs/kit</code></a> composite action, which installs the Kit binary and runs <code>kit github run</code>. That command reads the triggering event, enforces permissions, reacts with an emoji, runs the agent against the issue thread or PR, posts the response as a comment, and — if the agent changed files — pushes a <code>kit-agent[bot]</code> branch and opens a pull request.</p>
<table>
<thead>
<tr>
<th>Flag</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td><code>--model</code></td>
<td>Provider/model to write into the workflow</td>
</tr>
<tr>
<td><code>--force</code></td>
<td>Overwrite an existing workflow file</td>
</tr>
<tr>
<td><code>--no-secret</code></td>
<td>Skip the offer to set the provider secret via the <code>gh</code> CLI</td>
</tr>
</tbody>
</table>
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
<td><code>/connect [provider]</code></td>
<td>Add an API key for a provider. Opens a searchable provider list, then a masked key input. The key is saved to the credentials file and the active model is reconnected when it belongs to that provider. <code>/connect groq</code> skips the list. Alias: <code>/login</code>.</td>
</tr>
<tr>
<td><code>/theme [name]</code></td>
<td>Switch color theme. Running with no argument opens a modal picker showing every built-in and user theme.</td>
</tr>
<tr>
<td><code>/thinking [level]</code></td>
<td>Set thinking level. Running with no argument opens a modal picker showing only the levels the current model accepts; passing a level (<code>off</code>, <code>none</code>, <code>minimal</code>, <code>low</code>, <code>medium</code>, <code>high</code>) switches directly, substituting with the nearest supported level when needed.</td>
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
<td><code>/shortcuts</code></td>
<td>List keyboard shortcuts registered by extensions</td>
</tr>
<tr>
<td><code>/reload-ext</code></td>
<td>Hot-reload all extensions from disk (alias: <code>/re</code>)</td>
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
<h3 id="image-attachments"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#image-attachments"><span class="icon icon-link"></span></a>Image attachments</h3>
<p>Attach images to your next prompt straight from the clipboard:</p>
<ul>
<li>Copy an image (e.g. a screenshot) to the system clipboard, then press <strong>Ctrl+V</strong> in the input to attach it.</li>
<li>Press <strong>Ctrl+U</strong> to clear all pending image attachments.</li>
<li>Attachments are sent alongside your text when you submit, and cleared afterward.</li>
</ul>
<p>When a terminal supports color, Kit renders a small low-resolution <strong>thumbnail preview</strong> of each pending image directly in the input, below the <code>[N image(s) attached]</code> indicator, so you can confirm the right image was attached before sending.</p>
<p>Kit probes the terminal at startup and draws the preview the best way it actually supports. A terminal that answers the <strong>Kitty graphics protocol</strong> gets a real image; anything else gets a thumbnail built from Unicode half-block characters and ordinary terminal colors, which renders correctly everywhere. Thumbnails are capped to a small cell box for a glanceable, low-res look.</p>
<p>Detection is a live query rather than a guess from <code>$TERM</code>, because a multiplexer can forward the query to the terminal behind it and appear to support graphics it cannot draw:</p>
<ul>
<li><strong>kitty</strong> and similar: real images, in both the input preview and the transcript.</li>
<li><strong>zellij</strong>: real images in the input preview. Submitted images fall back to half blocks in the transcript, because a scrolling transcript needs the image to move with its message and zellij cannot yet do that.</li>
<li><strong>tmux</strong>: half blocks. tmux answers the query itself while forwarding it onward, so the terminal's late reply would be printed into the UI as stray escape codes.</li>
<li>Best fidelity needs a <strong>truecolor</strong> terminal (<code>COLORTERM=truecolor</code>); Kit degrades to 256-color where truecolor is unavailable.</li>
<li>On terminals with neither, the preview is skipped and the <code>[N image(s) attached]</code> text indicator is shown alone.</li>
</ul>
<p>Set <code>KIT_IMAGE_PROTOCOL=kitty</code> to force the graphics protocol, or <code>KIT_IMAGE_PROTOCOL=halfblock</code> to force half blocks, when the probe gets it wrong.</p>
<p>You can also attach image files by referencing them with <code>@path/to/image.png</code> — binary files are auto-detected by MIME type. See <a href="/quick-start">Quick Start</a> for the <code>@</code> attachment syntax.</p>
<p><strong>In a remote session</strong> (<code>kit remote --host &lt;name&gt;</code>), <code>Ctrl+V</code> attaches an image from the <strong>local machine's</strong> clipboard: it is streamed to the host over the tunnel and attached to the session with the same thumbnail preview. The clipboard tools only need to exist on the machine you are typing on. See <a href="/advanced/remote-sessions">Remote sessions</a>.</p>
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
<p>Templates appear as slash commands, tagged <code>[prompt]</code> in the autocomplete popup:</p>
<pre><code>/review error handling
</code></pre>
<p>A template and a <a href="#activating-skills">skill</a> with the same name cannot both be reached: the template wins and the skill is left out of the popup with a logged warning.</p>
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
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> acp</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> --debug</span><span style="color:#6A737D;--shiki-dark:#6A737D">              # With debug logging to stderr</span></span></code></pre>
<h2 id="detachable-sessions"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#detachable-sessions"><span class="icon icon-link"></span></a>Detachable sessions</h2>
<p>Run Kit inside a session that survives closing your terminal, and switch
between several of them like tmux or zellij. Sessions are hosted by a
background daemon; <code>kit attach</code> starts one automatically if none is
running. See <a href="/advanced/remote-sessions">Remote sessions</a> for the full
picture.</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> attach</span><span style="color:#6A737D;--shiki-dark:#6A737D">                        # Pick a live session (local or paired host), or start one</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> attach</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> 3</span><span style="color:#6A737D;--shiki-dark:#6A737D">                      # Attach straight to session 3</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> attach</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> --new</span><span style="color:#6A737D;--shiki-dark:#6A737D">                  # Skip the picker, start a new session</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> attach</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> --host</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> homelab</span><span style="color:#6A737D;--shiki-dark:#6A737D">         # Attach on a paired remote host</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> attach</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> --all</span><span style="color:#6A737D;--shiki-dark:#6A737D">                  # Pick across paired hosts without starting a local daemon</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> ls</span><span style="color:#6A737D;--shiki-dark:#6A737D">                            # List live sessions on this machine</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> ls</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> --all</span><span style="color:#6A737D;--shiki-dark:#6A737D">                      # Include sessions on every paired host</span></span></code></pre>
<p>The <code>kit attach</code> picker lists this machine's sessions <strong>and</strong> those on
every paired host, grouped by host name, so it is the one command that
shows everything you can attach to. Querying those hosts costs up to eight
seconds when one is asleep (in parallel, so eight seconds total); <code>--new</code>
and <code>kit attach &lt;id&gt;</code> skip it, because neither opens a picker.</p>
<p>Every new session starts with the working-directory picker, whichever way
you create it — <code>kit attach</code>, <code>kit attach --new</code>, or <code>Ctrl+] c</code>. <code>--new</code>
skips the <em>session</em> picker, not the directory one. Once a directory is
chosen the session behaves exactly like a local <code>kit</code>.</p>
<p>You can also run the directory picker outside a session with
<a href="/cli/flags"><code>kit --pick-dir</code></a>; the daemon uses that same flag to start
each session.</p>
<p>Inside a session, <code>Ctrl+]</code> is the multiplexer prefix:</p>
<table>
<thead>
<tr>
<th>Chord</th>
<th>Action</th>
</tr>
</thead>
<tbody>
<tr>
<td><code>Ctrl+] d</code></td>
<td>Detach; the session keeps running</td>
</tr>
<tr>
<td><code>Ctrl+] s</code></td>
<td>Switch to another session</td>
</tr>
<tr>
<td><code>Ctrl+] c</code></td>
<td>Start a new session</td>
</tr>
<tr>
<td><code>Ctrl+] n</code> / <code>Ctrl+] p</code></td>
<td>Next / previous session</td>
</tr>
<tr>
<td><code>Ctrl+] w</code></td>
<td>Switch across paired hosts</td>
</tr>
<tr>
<td><code>Ctrl+] Ctrl+]</code></td>
<td>Send a literal <code>Ctrl+]</code> to the session</td>
</tr>
</tbody>
</table>
<p>The prefix is deliberately not <code>Ctrl+X</code>: that belongs to the session itself
(<a href="#mid-turn-steering">steering</a>, <a href="#external-editor">external editor</a>), so
the keymap is identical whether Kit runs locally or through a session.
<code>Ctrl+X d</code> still detaches too, kept for compatibility with earlier
releases.</p>
<p>Sessions survive a client disconnect, but not a restart of the hosting
daemon — see <a href="/advanced/remote-sessions#sessions-and-daemon-restarts">Sessions and daemon
restarts</a>.</p>
<p>A session renders into your terminal but is spawned by the daemon, so the
client forwards its <code>TERM</code>, <code>COLORTERM</code> and background color when it
attaches — otherwise a session would inherit the daemon's environment and
render a truecolor theme in the wrong palette. See <a href="/advanced/remote-sessions#terminal-and-colors">Terminal and
colors</a>.</p>
<h2 id="remote-sessions"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#remote-sessions"><span class="icon icon-link"></span></a>Remote sessions</h2>
<p>Run Kit as a daemon on one machine and attach to it from another over an
end-to-end encrypted iroh connection. All work runs on the daemon host; see
<a href="/advanced/remote-sessions">Remote sessions</a> for the full picture.</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D"># On the host</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> daemon</span><span style="color:#6A737D;--shiki-dark:#6A737D">                        # Host sessions for paired clients (Ctrl+C stops)</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> daemon</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> pair</span><span style="color:#6A737D;--shiki-dark:#6A737D">                   # One-time pairing: show code, confirm on this terminal</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> daemon</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> pair</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> --list</span><span style="color:#6A737D;--shiki-dark:#6A737D">            # List paired clients (fingerprints)</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> daemon</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> pair</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> --revoke</span><span style="color:#D73A49;--shiki-dark:#F97583"> &lt;</span><span style="color:#032F62;--shiki-dark:#9ECBFF">f</span><span style="color:#24292E;--shiki-dark:#E1E4E8">p</span><span style="color:#D73A49;--shiki-dark:#F97583">&gt;</span><span style="color:#6A737D;--shiki-dark:#6A737D">     # Revoke a client</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> daemon</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> status</span><span style="color:#6A737D;--shiki-dark:#6A737D">                 # Endpoint, paired clients, active sessions</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> daemon</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> service</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> install</span><span style="color:#6A737D;--shiki-dark:#6A737D">        # Install + start the systemd user service</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> daemon</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> service</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> remove</span><span style="color:#6A737D;--shiki-dark:#6A737D">         # Stop and uninstall the service</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D"># On the client (first time — the host terminal asks you to accept)</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> remote</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> --pair</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> A1B2C3D4</span><span style="color:#6A737D;--shiki-dark:#6A737D">        # Pair and save the host under a name</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D"># On the client (any time after)</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> remote</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> --host</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> homelab</span><span style="color:#6A737D;--shiki-dark:#6A737D">         # Attach to the paired host</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> attach</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> --host</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> homelab</span><span style="color:#6A737D;--shiki-dark:#6A737D">         # Same, with session switching (Ctrl+] s)</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> remote</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> --list</span><span style="color:#6A737D;--shiki-dark:#6A737D">                 # List paired hosts</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> remote</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> --forget</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> homelab</span><span style="color:#6A737D;--shiki-dark:#6A737D">       # Forget a saved host</span></span></code></pre>
<p>Pairing is one-time and human-approved: the code only works while the
<code>kit daemon pair</code> window is open, and the host must accept the request on
its terminal. After pairing, the client authenticates with its own signing
key — no code involved — and the host can revoke it at any time. Each
client picks a working directory and gets a private session; multiple
clients can hold sessions at the same time and <code>/quit</code> closes only that
client's connection. <code>Ctrl+] d</code> detaches (<code>Ctrl+X d</code> still works, for
compatibility) — the session keeps running on the
host and can be reattached later (or shared by several clients at once, with
every attached terminal mirroring the same screen). Only one daemon may run
per user.</p>`,headings:[{depth:2,text:`Authentication`,id:`authentication`},{depth:2,text:`Model database`,id:`model-database`},{depth:2,text:`Extension management`,id:`extension-management`},{depth:3,text:`Installing extensions from git`,id:`installing-extensions-from-git`},{depth:2,text:`Skills`,id:`skills`},{depth:3,text:`Activating skills`,id:`activating-skills`},{depth:3,text:`Skills CLI flags`,id:`skills-cli-flags`},{depth:3,text:`Skill frontmatter`,id:`skill-frontmatter`},{depth:3,text:`Project trust prompt`,id:`project-trust-prompt`},{depth:2,text:`GitHub integration`,id:`github-integration`},{depth:2,text:`Interactive slash commands`,id:`interactive-slash-commands`},{depth:3,text:`Prompt history`,id:`prompt-history`},{depth:3,text:`Cancelling operations`,id:`cancelling-operations`},{depth:3,text:`External editor`,id:`external-editor`},{depth:3,text:`Mid-turn steering`,id:`mid-turn-steering`},{depth:3,text:`Image attachments`,id:`image-attachments`},{depth:2,text:`Prompt templates`,id:`prompt-templates`},{depth:3,text:`Creating templates`,id:`creating-templates`},{depth:3,text:`Using templates`,id:`using-templates`},{depth:3,text:`Argument placeholders`,id:`argument-placeholders`},{depth:3,text:`CLI flags`,id:`cli-flags`},{depth:2,text:`ACP server`,id:`acp-server`},{depth:2,text:`Detachable sessions`,id:`detachable-sessions`},{depth:2,text:`Remote sessions`,id:`remote-sessions`}],raw:'\n# Commands\n\n## Authentication\n\nAnthropic, OpenAI and GitHub Copilot use OAuth flows. Every other provider\nin the model database takes an API key.\n\n```bash\nkit auth login [provider]          # Start OAuth flow (e.g., anthropic) or prompt for an API key\nkit auth login groq                # Prompt for a Groq API key (hidden input)\nkit auth login openrouter --api-key sk-or-...   # Store a key without prompting\nkit auth login [provider] --set-default  # Set provider\'s default model as system default\nkit auth logout [provider]         # Remove credentials for provider\nkit auth status                    # Check authentication status\n```\n\nStored keys live in `$XDG_CONFIG_HOME/.kit/credentials.json` (defaults to\n`~/.config/.kit/credentials.json`, mode `0600`) and take precedence over the\nprovider\'s environment variable (for example `GROQ_API_KEY`). The\nenvironment variable is still used when no key is stored.\n\nKit starts even when the configured model has no key. The TUI shows a notice\nand refuses to send prompts until you add a key with `/connect` (or switch\nto a model whose provider has one with `/model`). One-shot prompts\n(`kit "question"`) still fail fast with the same error.\n\n## Model database\n\nManage the local model database that maps provider names to API configurations.\n\n```bash\nkit models [provider]        # List available models (optionally filter by provider)\nkit models --all             # Show all providers (not just LLM-compatible)\nkit update-models [source]   # Update model database\n```\n\nThe `update-models` command accepts an optional source argument:\n- *(none)* — update from [models.dev](https://models.dev)\n- A URL — fetch from a custom endpoint\n- A file path — load from a local file\n- `embedded` — reset to the bundled database\n\nKit records the schema version of the on-disk cache. When Kit\'s binary\nunderstands catalog fields the cache was written without — reasoning options,\nlong-context pricing tiers, deprecation status — the older cache is ignored\nin favour of the embedded snapshot, so a stale cache never masks metadata the\ncurrent binary can use. Run `kit update-models` once to rewrite the cache in\nthe current schema.\n\n## Extension management\n\n```bash\nkit extensions list          # List discovered extensions\nkit extensions validate      # Validate extension files\nkit extensions init          # Generate example extension template\n```\n\n### Installing extensions from git\n\n```bash\nkit install <git-url>        # Install extensions from git repositories\nkit install -l <git-url>     # Install to project-local .kit/git/ directory\nkit install -u <git-url>     # Update an already-installed package\nkit install --uninstall <pkg> # Remove an installed package\nkit install --all            # Install all extensions without prompting\n```\n\n## Skills\n\nKit implements the [Agent Skills](https://agentskills.io/specification) format. A skill is a directory with a `SKILL.md` file (YAML frontmatter + Markdown instructions) and optional `scripts/`, `references/`, and `assets/` subdirectories.\n\n```bash\nkit skill list               # List discovered skills with scope, path and spec warnings\nkit skill validate <path>    # Validate a skill directory / SKILL.md against the spec\nkit skill install            # Install the Kit skills (kit-extensions, kit-sdk) via skills.sh\nkit skill                    # Same as "kit skill install"\n```\n\n`kit skill list` honors `--skill`, `--skills-dir`, `--no-skills` and `--bare`, and never prompts for project trust (it is read-only). `kit skill validate` accepts a `SKILL.md` file, a skill directory, or a directory of skills; errors (missing `name`/`description`) make the exit code non-zero, while warnings (name format, length limits, name/directory mismatch, body over 500 lines) are informational.\n\n### Activating skills\n\nSkills load in three tiers, following the spec\'s progressive-disclosure model:\n\n1. **Catalog** — at startup the model sees every skill\'s `name`, `description` and location in an `<available_skills>` block in the system prompt.\n2. **Instructions** — the full `SKILL.md` body is loaded when a skill is activated, either by the model (the `activate_skill` tool, or a `read` of the listed location) or by you.\n3. **Resources** — bundled files are listed in a `<skill_resources>` block and read on demand.\n\nTo activate a skill yourself, type its name as a slash command, optionally followed by a request:\n\n```\n/pdf-processing extract the tables from report.pdf\n```\n\nThe `/` autocomplete popup lists every loaded skill that is not shadowed by another slash command (see precedence below) with a `[skill]` badge next to it — hidden-from-model skills included, since hiding only affects the model-facing catalog (prompt templates show `[prompt]`, extension commands `[ext]`, MCP prompts `[mcp]`). Kit wraps the skill body in a `<skill_content>` block, appends your text, and sends it as the turn. Activated skill content is protected from `/compact` pruning.\n\nSlash-name precedence is: built-in commands, extension commands, MCP prompts (`/server:prompt`), prompt templates, then skills. A skill whose name is already taken is left out of the popup and a warning is logged, so rename one side if that happens.\n\n### Skills CLI flags\n\nControl which skills are loaded at startup:\n\n```bash\n# Load a specific skill file\nkit --skill path/to/skill.md "prompt"\n\n# Load multiple skill files or directories (flag is repeatable)\nkit --skill ./skill1.md --skill ./skill2.md "prompt"\n\n# Scan a directory directly for skills (overrides auto-discovery)\nkit --skills-dir /path/to/skills "prompt"\n\n# Hide a skill from the model catalog by name (still usable via /<name>)\nkit --skill-disable noisy-skill "prompt"\n\n# Disable all skill loading (auto-discovery and explicit)\nkit --no-skills "prompt"\n```\n\nSkills follow the [agentskills.io](https://agentskills.io/specification) convention. They are auto-discovered from four canonical scopes:\n\n| Scope | Location |\n|-------|----------|\n| User-level (cross-client) | `~/.agents/skills/` |\n| User-level (Kit) | `~/.config/kit/skills/` (honors `$XDG_CONFIG_HOME`) |\n| Project-local (cross-client) | `<project>/.agents/skills/` |\n| Project-local (Kit) | `<project>/.kit/skills/` |\n\nWhen two skills share the same `name`, the project-level one takes precedence over the user-level one. Use `--skills-dir` to scan one directory directly instead (it is **not** treated as a parent of `.agents`/`.kit` — the directory itself is scanned). `--skill` loads files explicitly (which disables auto-discovery), and `--no-skills` suppresses all skill loading regardless of other flags.\n\nDisabled skills (`--skill-disable`, the `skill-disable` config key, or `disable-model-invocation: true` in a skill\'s frontmatter) are hidden from the model-facing `<available_skills>` catalog but remain available for explicit activation via the `/<name>` slash command.\n\n### Skill frontmatter\n\nA skill is a markdown file (`SKILL.md` in a directory, or a standalone `.md`/`.txt` file) with optional YAML frontmatter. Kit reads the full [agentskills.io](https://agentskills.io/specification) field set plus two Kit-specific extensions:\n\n```yaml\n---\nname: pdf-extractor                 # required; lowercase, digits, single hyphens; must match the directory\ndescription: Use when extracting tables from PDFs   # required (drives model discovery)\nlicense: MIT                        # optional, SPDX identifier\ncompatibility: Requires python3 and pdftotext  # optional, environment requirements (max 500 chars)\nallowed-tools: Read Bash(pdftotext:*)  # optional (experimental); parsed, not enforced by Kit\ndisable-model-invocation: false     # optional; true hides from the catalog\nmetadata:                           # optional arbitrary string key/value pairs\n  author: you\ntags: [pdf, data]                   # Kit extension\nwhen: on-demand                     # Kit extension\n---\n```\n\n`name` and `description` are required — a skill missing its description is skipped with a logged warning, since the description is the sole basis on which the model decides relevance. Other spec constraints (name is 1–64 chars of `a-z0-9-` with no leading/trailing/double hyphen and matches the parent directory; description ≤ 1024 chars; `compatibility` ≤ 500 chars; body under 500 lines) are checked leniently: the skill still loads, and the deviation is reported by `kit skill list` / `kit skill validate`. Descriptions are XML-escaped before they enter the catalog, so characters like `<`, `>`, and `&` are safe. A skill directory may bundle `scripts/`, `references/`, and `assets/` subdirectories; when a skill is activated those files are enumerated in a `<skill_resources>` block so the model knows what it can read.\n\n### Project trust prompt\n\nBecause project-local skills are injected into the system prompt, entering a repository that ships `.agents/skills/` or `.kit/skills/` for the first time prompts you to trust it before any project skill loads — a safeguard against a freshly cloned, untrusted repo smuggling instructions into the agent:\n\n```\nThis project provides 2 skills under .agents/skills or .kit/skills:\n  /path/to/repo\nLoad them into the agent? [t]rust always / [o]nce / [s]kip (default skip):\n```\n\nChoosing **trust always** persists the directory to `~/.config/kit/trusted-projects.json` so you are not asked again. The prompt is skipped (skills load silently) in non-interactive runs — when a prompt is passed positionally, `--quiet` is set, or stdin is not a TTY.\n\n## GitHub integration\n\nScaffold a GitHub Actions workflow that runs Kit as an automated collaborator/reviewer. The workflow triggers when someone comments `/kit ...` on an issue or pull request review, runs the agent non-interactively in the runner, and lets it respond.\n\n```bash\nkit github install           # Scaffold .github/workflows/kit.yml\nkit github install --model anthropic/claude-sonnet-4-5-20250929  # Skip the model prompt\nkit github install --force   # Overwrite an existing workflow file\nkit github install --no-secret # Skip the offer to set the provider secret via the gh CLI\n```\n\nBy default the command prompts for the model (pre-filled with a sensible default). If the [`gh` CLI](https://cli.github.com/) is detected on your `PATH` and the provider API key is present in your environment, you\'ll be offered the option to store it as a repository secret automatically.\n\nThe generated workflow:\n\n- Triggers only on `issue_comment` and `pull_request_review_comment` (`types: [created]`).\n- Runs only when the comment begins with the `/kit` command token.\n- Restricts triggers to repository owners, members, and collaborators (via `author_association`).\n- Uses least-privilege `permissions` and `persist-credentials: false`.\n- Authenticates git/PR operations with the built-in `secrets.GITHUB_TOKEN` and the provider via a repository secret (e.g. `ANTHROPIC_API_KEY`).\n\nAfter committing the workflow and setting the provider secret, comment `/kit <your request>` on any issue or pull request to trigger Kit.\n\nThe generated workflow uses the bundled [`mark3labs/kit`](https://github.com/mark3labs/kit/blob/master/action.yml) composite action, which installs the Kit binary and runs `kit github run`. That command reads the triggering event, enforces permissions, reacts with an emoji, runs the agent against the issue thread or PR, posts the response as a comment, and — if the agent changed files — pushes a `kit-agent[bot]` branch and opens a pull request.\n\n| Flag | Description |\n|------|-------------|\n| `--model` | Provider/model to write into the workflow |\n| `--force` | Overwrite an existing workflow file |\n| `--no-secret` | Skip the offer to set the provider secret via the `gh` CLI |\n\n## Interactive slash commands\n\nThese commands are available inside the Kit TUI during an interactive session:\n\n| Command | Description |\n|---------|-------------|\n| `/help` | Show available commands |\n| `/tools` | List available MCP tools |\n| `/servers` | Show connected MCP servers |\n| `/model [name]` | Switch model or open model selector |\n| `/connect [provider]` | Add an API key for a provider. Opens a searchable provider list, then a masked key input. The key is saved to the credentials file and the active model is reconnected when it belongs to that provider. `/connect groq` skips the list. Alias: `/login`. |\n| `/theme [name]` | Switch color theme. Running with no argument opens a modal picker showing every built-in and user theme. |\n| `/thinking [level]` | Set thinking level. Running with no argument opens a modal picker showing only the levels the current model accepts; passing a level (`off`, `none`, `minimal`, `low`, `medium`, `high`) switches directly, substituting with the nearest supported level when needed. |\n| `/compact [focus]` | Summarize older messages to free context |\n| `/clear` | Clear conversation |\n| `/clear-queue` | Clear queued messages |\n| `/usage` | Show token usage |\n| `/reset-usage` | Reset usage statistics |\n| `/shortcuts` | List keyboard shortcuts registered by extensions |\n| `/reload-ext` | Hot-reload all extensions from disk (alias: `/re`) |\n| `/tree` | Navigate session tree |\n| `/fork` | Fork to new session from an earlier message |\n| `/new` | Start a new session (creates new session file) |\n| `/name [name]` | Set or show session display name |\n| `/resume` | Open session picker to switch sessions (alias: `/r`) |\n| `/session` | Show session info |\n| `/export [path]` | Export session as JSONL (default: auto-generated path) |\n| `/import <path>` | Import a session from a JSONL file |\n| `/share` | Upload session to GitHub Gist and get a shareable viewer URL |\n| `/quit` | Exit Kit |\n\n### Prompt history\n\nUse **↑** and **↓** arrow keys to navigate through previously submitted prompts. Kit keeps the last 100 entries. Consecutive duplicates are skipped.\n\n### Cancelling operations\n\nPress **ESC twice** to cancel the current operation:\n- During a tool call: rolls back the entire turn to maintain API message pairing\n- During streaming: stops the response generation\n\nThis ensures that `tool_use` and `tool_result` messages are always sent to the API as matched pairs, avoiding errors from orphaned tool calls.\n\n### External editor\n\nPress **Ctrl+X e** to open your `$VISUAL` or `$EDITOR` in a temporary file pre-populated with the current input text. On save and quit, the edited content replaces the input textarea. On error exit (e.g., `:cq` in Vim), the original input is preserved.\n\n### Mid-turn steering\n\nPress **Ctrl+X s** during streaming to inject a system-level instruction mid-turn. This allows you to steer the conversation direction without waiting for the model to finish:\n\n- Works during streaming output\n- Sends a steering instruction as a system message\n- Model continues from the interruption point with the new guidance\n\nExample: While the model is writing code, press Ctrl+X s and type "Use async/await instead" to change the implementation approach.\n\n### Image attachments\n\nAttach images to your next prompt straight from the clipboard:\n\n- Copy an image (e.g. a screenshot) to the system clipboard, then press **Ctrl+V** in the input to attach it.\n- Press **Ctrl+U** to clear all pending image attachments.\n- Attachments are sent alongside your text when you submit, and cleared afterward.\n\nWhen a terminal supports color, Kit renders a small low-resolution **thumbnail preview** of each pending image directly in the input, below the `[N image(s) attached]` indicator, so you can confirm the right image was attached before sending.\n\nKit probes the terminal at startup and draws the preview the best way it actually supports. A terminal that answers the **Kitty graphics protocol** gets a real image; anything else gets a thumbnail built from Unicode half-block characters and ordinary terminal colors, which renders correctly everywhere. Thumbnails are capped to a small cell box for a glanceable, low-res look.\n\nDetection is a live query rather than a guess from `$TERM`, because a multiplexer can forward the query to the terminal behind it and appear to support graphics it cannot draw:\n\n- **kitty** and similar: real images, in both the input preview and the transcript.\n- **zellij**: real images in the input preview. Submitted images fall back to half blocks in the transcript, because a scrolling transcript needs the image to move with its message and zellij cannot yet do that.\n- **tmux**: half blocks. tmux answers the query itself while forwarding it onward, so the terminal\'s late reply would be printed into the UI as stray escape codes.\n- Best fidelity needs a **truecolor** terminal (`COLORTERM=truecolor`); Kit degrades to 256-color where truecolor is unavailable.\n- On terminals with neither, the preview is skipped and the `[N image(s) attached]` text indicator is shown alone.\n\nSet `KIT_IMAGE_PROTOCOL=kitty` to force the graphics protocol, or `KIT_IMAGE_PROTOCOL=halfblock` to force half blocks, when the probe gets it wrong.\n\nYou can also attach image files by referencing them with `@path/to/image.png` — binary files are auto-detected by MIME type. See [Quick Start](/quick-start) for the `@` attachment syntax.\n\n**In a remote session** (`kit remote --host <name>`), `Ctrl+V` attaches an image from the **local machine\'s** clipboard: it is streamed to the host over the tunnel and attached to the session with the same thumbnail preview. The clipboard tools only need to exist on the machine you are typing on. See [Remote sessions](/advanced/remote-sessions).\n\n## Prompt templates\n\n### Creating templates\n\nTemplates use YAML frontmatter for metadata and support argument placeholders:\n\n```markdown\n---\ndescription: Review code for issues\n---\nReview the following code for bugs and security issues.\nFocus on $1 specifically.\n```\n\nSave to `~/.kit/prompts/review.md` or `.kit/prompts/review.md`.\n\n### Using templates\n\nTemplates appear as slash commands, tagged `[prompt]` in the autocomplete popup:\n\n```\n/review error handling\n```\n\nA template and a [skill](#activating-skills) with the same name cannot both be reached: the template wins and the skill is left out of the popup with a logged warning.\n\n### Argument placeholders\n\n| Placeholder | Description |\n|-------------|-------------|\n| `$1`, `$2`, etc. | Individual arguments by position |\n| `$@`, `$ARGUMENTS` | All arguments joined with spaces (zero or more) |\n| `$+` | All arguments joined with spaces (one or more required) |\n| `${@:N}` | Arguments from position N onwards |\n| `${@:N:L}` | L arguments starting at position N |\n\nPlaceholders inside fenced code blocks (`` ``` ``) and inline code spans are ignored, so documentation examples won\'t be substituted.\n\n### CLI flags\n\n```bash\n# Load a specific template by name\nkit --prompt-template review\n\n# Disable template loading\nkit --no-prompt-templates\n```\n\n## ACP server\n\nRun Kit as an [ACP (Agent Client Protocol)](https://agentclientprotocol.com) agent server. ACP-compatible clients communicate with Kit over JSON-RPC 2.0 on stdin/stdout.\n\n```bash\nkit acp                      # Start as ACP agent\nkit acp --debug              # With debug logging to stderr\n```\n\n## Detachable sessions\n\nRun Kit inside a session that survives closing your terminal, and switch\nbetween several of them like tmux or zellij. Sessions are hosted by a\nbackground daemon; `kit attach` starts one automatically if none is\nrunning. See [Remote sessions](/advanced/remote-sessions) for the full\npicture.\n\n```bash\nkit attach                        # Pick a live session (local or paired host), or start one\nkit attach 3                      # Attach straight to session 3\nkit attach --new                  # Skip the picker, start a new session\nkit attach --host homelab         # Attach on a paired remote host\nkit attach --all                  # Pick across paired hosts without starting a local daemon\n\nkit ls                            # List live sessions on this machine\nkit ls --all                      # Include sessions on every paired host\n```\n\nThe `kit attach` picker lists this machine\'s sessions **and** those on\nevery paired host, grouped by host name, so it is the one command that\nshows everything you can attach to. Querying those hosts costs up to eight\nseconds when one is asleep (in parallel, so eight seconds total); `--new`\nand `kit attach <id>` skip it, because neither opens a picker.\n\nEvery new session starts with the working-directory picker, whichever way\nyou create it — `kit attach`, `kit attach --new`, or `Ctrl+] c`. `--new`\nskips the *session* picker, not the directory one. Once a directory is\nchosen the session behaves exactly like a local `kit`.\n\nYou can also run the directory picker outside a session with\n[`kit --pick-dir`](/cli/flags); the daemon uses that same flag to start\neach session.\n\nInside a session, `Ctrl+]` is the multiplexer prefix:\n\n| Chord | Action |\n|-------|--------|\n| `Ctrl+] d` | Detach; the session keeps running |\n| `Ctrl+] s` | Switch to another session |\n| `Ctrl+] c` | Start a new session |\n| `Ctrl+] n` / `Ctrl+] p` | Next / previous session |\n| `Ctrl+] w` | Switch across paired hosts |\n| `Ctrl+] Ctrl+]` | Send a literal `Ctrl+]` to the session |\n\nThe prefix is deliberately not `Ctrl+X`: that belongs to the session itself\n([steering](#mid-turn-steering), [external editor](#external-editor)), so\nthe keymap is identical whether Kit runs locally or through a session.\n`Ctrl+X d` still detaches too, kept for compatibility with earlier\nreleases.\n\nSessions survive a client disconnect, but not a restart of the hosting\ndaemon — see [Sessions and daemon\nrestarts](/advanced/remote-sessions#sessions-and-daemon-restarts).\n\nA session renders into your terminal but is spawned by the daemon, so the\nclient forwards its `TERM`, `COLORTERM` and background color when it\nattaches — otherwise a session would inherit the daemon\'s environment and\nrender a truecolor theme in the wrong palette. See [Terminal and\ncolors](/advanced/remote-sessions#terminal-and-colors).\n\n## Remote sessions\n\nRun Kit as a daemon on one machine and attach to it from another over an\nend-to-end encrypted iroh connection. All work runs on the daemon host; see\n[Remote sessions](/advanced/remote-sessions) for the full picture.\n\n```bash\n# On the host\nkit daemon                        # Host sessions for paired clients (Ctrl+C stops)\nkit daemon pair                   # One-time pairing: show code, confirm on this terminal\nkit daemon pair --list            # List paired clients (fingerprints)\nkit daemon pair --revoke <fp>     # Revoke a client\nkit daemon status                 # Endpoint, paired clients, active sessions\nkit daemon service install        # Install + start the systemd user service\nkit daemon service remove         # Stop and uninstall the service\n\n# On the client (first time — the host terminal asks you to accept)\nkit remote --pair A1B2C3D4        # Pair and save the host under a name\n\n# On the client (any time after)\nkit remote --host homelab         # Attach to the paired host\nkit attach --host homelab         # Same, with session switching (Ctrl+] s)\nkit remote --list                 # List paired hosts\nkit remote --forget homelab       # Forget a saved host\n```\n\nPairing is one-time and human-approved: the code only works while the\n`kit daemon pair` window is open, and the host must accept the request on\nits terminal. After pairing, the client authenticates with its own signing\nkey — no code involved — and the host can revoke it at any time. Each\nclient picks a working directory and gets a private session; multiple\nclients can hold sessions at the same time and `/quit` closes only that\nclient\'s connection. `Ctrl+] d` detaches (`Ctrl+X d` still works, for\ncompatibility) — the session keeps running on the\nhost and can be reattached later (or shared by several clients at once, with\nevery attached terminal mirroring the same screen). Only one daemon may run\nper user.\n'};export{e as default};
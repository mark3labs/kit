var e={frontmatter:{title:`Global Flags`,description:`Complete reference for all Kit CLI flags.`,hidden:!1,toc:!0,draft:!1},html:`<h1 id="global-flags"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#global-flags"><span class="icon icon-link"></span></a>Global Flags</h1>
<p>All flags can be passed to the root <code>kit</code> command.</p>
<h2 id="model-and-provider"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#model-and-provider"><span class="icon icon-link"></span></a>Model and provider</h2>
<table>
<thead>
<tr>
<th>Flag</th>
<th>Short</th>
<th>Default</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td><code>--model</code></td>
<td><code>-m</code></td>
<td><code>anthropic/claude-sonnet-latest</code></td>
<td>Model to use (provider/model format)</td>
</tr>
<tr>
<td><code>--provider-api-key</code></td>
<td>—</td>
<td>—</td>
<td>API key for the provider</td>
</tr>
<tr>
<td><code>--provider-url</code></td>
<td>—</td>
<td>—</td>
<td>Base URL for provider API</td>
</tr>
<tr>
<td><code>--provider-wire</code></td>
<td>—</td>
<td>—</td>
<td>Wire protocol for auto-routed providers: <code>openai</code>, <code>openai-compat</code>, <code>anthropic</code>, <code>google</code> (<a href="/providers#provider-overrides">overrides the model database</a>)</td>
</tr>
<tr>
<td><code>--tls-skip-verify</code></td>
<td>—</td>
<td><code>false</code></td>
<td>Skip TLS certificate verification</td>
</tr>
</tbody>
</table>
<h2 id="session-management"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#session-management"><span class="icon icon-link"></span></a>Session management</h2>
<table>
<thead>
<tr>
<th>Flag</th>
<th>Short</th>
<th>Default</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td><code>--session</code></td>
<td><code>-s</code></td>
<td>—</td>
<td>Open specific JSONL session file</td>
</tr>
<tr>
<td><code>--continue</code></td>
<td><code>-c</code></td>
<td><code>false</code></td>
<td>Resume most recent session for current directory</td>
</tr>
<tr>
<td><code>--resume</code></td>
<td><code>-r</code></td>
<td><code>false</code></td>
<td>Interactive session picker</td>
</tr>
<tr>
<td><code>--no-session</code></td>
<td>—</td>
<td><code>false</code></td>
<td>Ephemeral mode, no persistence</td>
</tr>
</tbody>
</table>
<h2 id="behavior"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#behavior"><span class="icon icon-link"></span></a>Behavior</h2>
<p>These flags control Kit's behavior. When a prompt is passed as a positional argument, Kit runs in non-interactive mode.</p>
<table>
<thead>
<tr>
<th>Flag</th>
<th>Short</th>
<th>Default</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td><code>--quiet</code></td>
<td>—</td>
<td><code>false</code></td>
<td>Suppress all output (non-interactive only)</td>
</tr>
<tr>
<td><code>--json</code></td>
<td>—</td>
<td><code>false</code></td>
<td>Output response as JSON (non-interactive only)</td>
</tr>
<tr>
<td><code>--no-exit</code></td>
<td>—</td>
<td><code>false</code></td>
<td>Enter interactive mode after prompt completes</td>
</tr>
<tr>
<td><code>--max-steps</code></td>
<td>—</td>
<td><code>0</code></td>
<td>Maximum agent steps (0 for unlimited)</td>
</tr>
<tr>
<td><code>--stream</code></td>
<td>—</td>
<td><code>true</code></td>
<td>Enable streaming output</td>
</tr>
<tr>
<td><code>--compact</code></td>
<td>—</td>
<td><code>false</code></td>
<td>Enable compact output mode</td>
</tr>
<tr>
<td><code>--pick-dir</code></td>
<td>—</td>
<td><code>false</code></td>
<td>Choose a working directory with a picker before starting. <a href="/cli/commands#detachable-sessions">Detachable sessions</a> always start this way, including with <code>kit attach --new</code>, so the flag is only needed when running <code>kit</code> directly</td>
</tr>
<tr>
<td><code>--auto-compact</code></td>
<td>—</td>
<td><code>false</code></td>
<td>Compact proactively when near the context limit (reactive compact-and-retry on provider overflow errors is <a href="/sessions#reactive-compaction-on-overflow">always on</a>)</td>
</tr>
</tbody>
</table>
<h2 id="context"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#context"><span class="icon icon-link"></span></a>Context</h2>
<table>
<thead>
<tr>
<th>Flag</th>
<th>Short</th>
<th>Default</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td><code>--bare</code></td>
<td>—</td>
<td><code>false</code></td>
<td>No project context — skip all automatic discovery</td>
</tr>
</tbody>
</table>
<p><code>--bare</code> starts Kit without reading anything from the directory it runs in, so
you can ask a question without first loading whatever project you happen to be
standing in. It disables:</p>
<ul>
<li>project context files (<code>AGENTS.md</code>)</li>
<li>skills, from both project and user directories</li>
<li>extensions, from every source — project, user and system directories</li>
<li>named agent definitions</li>
<li>prompt template directories</li>
<li>project-local <code>.kit.yml</code></li>
</ul>
<p>Your home <code>~/.kit.yml</code> still loads, so API keys and your default model keep
working, as do <code>KIT_*</code> environment variables and an explicit <code>--config</code> file.</p>
<p>Anything you name on the command line still applies — <code>--extension</code>,
<code>--skill</code>, <code>--prompt-template</code>, <code>--system-prompt</code> and <code>@file</code> attachments are
unaffected. Bare mode suppresses what Kit finds on its own, never what you
asked for.</p>
<p>Core tools stay enabled and the working directory is unchanged, so the agent
can still read files when you ask it to. Combine with <code>--no-core-tools</code> for a
pure question-answering session:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> --bare</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> "why does a TLS handshake fail with an SNI mismatch"</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> --bare</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> --no-core-tools</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> "explain the CAP theorem"</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> --bare</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> -e</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> ~/my-ext.go</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> "..."</span><span style="color:#6A737D;--shiki-dark:#6A737D">   # explicit extension still loads</span></span></code></pre>
<p>Because a bare session is not tied to a directory, bare sessions share a
single store rather than the usual per-directory one. <code>kit --bare -c</code> resumes
your last bare conversation from anywhere on the filesystem.</p>
<p><code>--bare</code> cannot be set from a config file. It exists to ignore project
configuration, so allowing a project config to enable it would be
self-defeating.</p>
<h2 id="extensions"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#extensions"><span class="icon icon-link"></span></a>Extensions</h2>
<table>
<thead>
<tr>
<th>Flag</th>
<th>Short</th>
<th>Default</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td><code>--extension</code></td>
<td><code>-e</code></td>
<td>—</td>
<td>Load additional extension file(s) (repeatable)</td>
</tr>
<tr>
<td><code>--no-extensions</code></td>
<td>—</td>
<td><code>false</code></td>
<td>Disable all extensions</td>
</tr>
<tr>
<td><code>--prompt-template</code></td>
<td>—</td>
<td>—</td>
<td>Load a specific prompt template by name</td>
</tr>
<tr>
<td><code>--no-prompt-templates</code></td>
<td>—</td>
<td><code>false</code></td>
<td>Disable prompt template loading</td>
</tr>
</tbody>
</table>
<h2 id="skills"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#skills"><span class="icon icon-link"></span></a>Skills</h2>
<table>
<thead>
<tr>
<th>Flag</th>
<th>Short</th>
<th>Default</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td><code>--skill</code></td>
<td>—</td>
<td>—</td>
<td>Load skill file or directory (repeatable)</td>
</tr>
<tr>
<td><code>--skills-dir</code></td>
<td>—</td>
<td>—</td>
<td>Scan this directory directly for skills (overrides auto-discovery)</td>
</tr>
<tr>
<td><code>--skill-disable</code></td>
<td>—</td>
<td>—</td>
<td>Hide a skill from the model catalog by name (repeatable); still usable via <code>/skill:</code></td>
</tr>
<tr>
<td><code>--no-skills</code></td>
<td>—</td>
<td><code>false</code></td>
<td>Disable skill loading (auto-discovery and explicit)</td>
</tr>
<tr>
<td><code>--no-agents</code></td>
<td>—</td>
<td><code>false</code></td>
<td>Disable named agent discovery (built-ins and <a href="/advanced/subagents#named-agents">definition files</a>)</td>
</tr>
</tbody>
</table>
<h2 id="generation-parameters"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#generation-parameters"><span class="icon icon-link"></span></a>Generation parameters</h2>
<table>
<thead>
<tr>
<th>Flag</th>
<th>Short</th>
<th>Default</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td><code>--max-tokens</code></td>
<td>—</td>
<td><code>8192</code></td>
<td>Base cap for output tokens. Auto-raised per-model up to 32768 when the model's catalog ceiling is higher and no explicit value is set.</td>
</tr>
<tr>
<td><code>--temperature</code></td>
<td>—</td>
<td><code>0.7</code></td>
<td>Randomness 0.0–1.0</td>
</tr>
<tr>
<td><code>--top-p</code></td>
<td>—</td>
<td><code>0.95</code></td>
<td>Nucleus sampling 0.0–1.0</td>
</tr>
<tr>
<td><code>--top-k</code></td>
<td>—</td>
<td><code>40</code></td>
<td>Limit top K tokens</td>
</tr>
<tr>
<td><code>--stop-sequences</code></td>
<td>—</td>
<td>—</td>
<td>Custom stop sequences (comma-separated)</td>
</tr>
<tr>
<td><code>--frequency-penalty</code></td>
<td>—</td>
<td><code>0.0</code></td>
<td>Penalize frequent tokens (0.0–2.0)</td>
</tr>
<tr>
<td><code>--presence-penalty</code></td>
<td>—</td>
<td><code>0.0</code></td>
<td>Penalize present tokens (0.0–2.0)</td>
</tr>
<tr>
<td><code>--thinking-level</code></td>
<td>—</td>
<td><code>off</code></td>
<td>Extended thinking level: off, none, minimal, low, medium, high</td>
</tr>
</tbody>
</table>
<h2 id="system"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#system"><span class="icon icon-link"></span></a>System</h2>
<table>
<thead>
<tr>
<th>Flag</th>
<th>Short</th>
<th>Default</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td><code>--config</code></td>
<td>—</td>
<td><code>~/.kit.yml</code></td>
<td>Config file path</td>
</tr>
<tr>
<td><code>--system-prompt</code></td>
<td>—</td>
<td>—</td>
<td>System prompt text or file path</td>
</tr>
<tr>
<td><code>--debug</code></td>
<td>—</td>
<td><code>false</code></td>
<td>Enable debug logging</td>
</tr>
</tbody>
</table>`,headings:[{depth:2,text:`Model and provider`,id:`model-and-provider`},{depth:2,text:`Session management`,id:`session-management`},{depth:2,text:`Behavior`,id:`behavior`},{depth:2,text:`Context`,id:`context`},{depth:2,text:`Extensions`,id:`extensions`},{depth:2,text:`Skills`,id:`skills`},{depth:2,text:`Generation parameters`,id:`generation-parameters`},{depth:2,text:`System`,id:`system`}],raw:'\n# Global Flags\n\nAll flags can be passed to the root `kit` command.\n\n## Model and provider\n\n| Flag | Short | Default | Description |\n|------|-------|---------|-------------|\n| `--model` | `-m` | `anthropic/claude-sonnet-latest` | Model to use (provider/model format) |\n| `--provider-api-key` | — | — | API key for the provider |\n| `--provider-url` | — | — | Base URL for provider API |\n| `--provider-wire` | — | — | Wire protocol for auto-routed providers: `openai`, `openai-compat`, `anthropic`, `google` ([overrides the model database](/providers#provider-overrides)) |\n| `--tls-skip-verify` | — | `false` | Skip TLS certificate verification |\n\n## Session management\n\n| Flag | Short | Default | Description |\n|------|-------|---------|-------------|\n| `--session` | `-s` | — | Open specific JSONL session file |\n| `--continue` | `-c` | `false` | Resume most recent session for current directory |\n| `--resume` | `-r` | `false` | Interactive session picker |\n| `--no-session` | — | `false` | Ephemeral mode, no persistence |\n\n## Behavior\n\nThese flags control Kit\'s behavior. When a prompt is passed as a positional argument, Kit runs in non-interactive mode.\n\n| Flag | Short | Default | Description |\n|------|-------|---------|-------------|\n| `--quiet` | — | `false` | Suppress all output (non-interactive only) |\n| `--json` | — | `false` | Output response as JSON (non-interactive only) |\n| `--no-exit` | — | `false` | Enter interactive mode after prompt completes |\n| `--max-steps` | — | `0` | Maximum agent steps (0 for unlimited) |\n| `--stream` | — | `true` | Enable streaming output |\n| `--compact` | — | `false` | Enable compact output mode |\n| `--pick-dir` | — | `false` | Choose a working directory with a picker before starting. [Detachable sessions](/cli/commands#detachable-sessions) always start this way, including with `kit attach --new`, so the flag is only needed when running `kit` directly |\n| `--auto-compact` | — | `false` | Compact proactively when near the context limit (reactive compact-and-retry on provider overflow errors is [always on](/sessions#reactive-compaction-on-overflow)) |\n\n## Context\n\n| Flag | Short | Default | Description |\n|------|-------|---------|-------------|\n| `--bare` | — | `false` | No project context — skip all automatic discovery |\n\n`--bare` starts Kit without reading anything from the directory it runs in, so\nyou can ask a question without first loading whatever project you happen to be\nstanding in. It disables:\n\n- project context files (`AGENTS.md`)\n- skills, from both project and user directories\n- extensions, from every source — project, user and system directories\n- named agent definitions\n- prompt template directories\n- project-local `.kit.yml`\n\nYour home `~/.kit.yml` still loads, so API keys and your default model keep\nworking, as do `KIT_*` environment variables and an explicit `--config` file.\n\nAnything you name on the command line still applies — `--extension`,\n`--skill`, `--prompt-template`, `--system-prompt` and `@file` attachments are\nunaffected. Bare mode suppresses what Kit finds on its own, never what you\nasked for.\n\nCore tools stay enabled and the working directory is unchanged, so the agent\ncan still read files when you ask it to. Combine with `--no-core-tools` for a\npure question-answering session:\n\n```bash\nkit --bare "why does a TLS handshake fail with an SNI mismatch"\nkit --bare --no-core-tools "explain the CAP theorem"\nkit --bare -e ~/my-ext.go "..."   # explicit extension still loads\n```\n\nBecause a bare session is not tied to a directory, bare sessions share a\nsingle store rather than the usual per-directory one. `kit --bare -c` resumes\nyour last bare conversation from anywhere on the filesystem.\n\n`--bare` cannot be set from a config file. It exists to ignore project\nconfiguration, so allowing a project config to enable it would be\nself-defeating.\n\n## Extensions\n\n| Flag | Short | Default | Description |\n|------|-------|---------|-------------|\n| `--extension` | `-e` | — | Load additional extension file(s) (repeatable) |\n| `--no-extensions` | — | `false` | Disable all extensions |\n| `--prompt-template` | — | — | Load a specific prompt template by name |\n| `--no-prompt-templates` | — | `false` | Disable prompt template loading |\n\n## Skills\n\n| Flag | Short | Default | Description |\n|------|-------|---------|-------------|\n| `--skill` | — | — | Load skill file or directory (repeatable) |\n| `--skills-dir` | — | — | Scan this directory directly for skills (overrides auto-discovery) |\n| `--skill-disable` | — | — | Hide a skill from the model catalog by name (repeatable); still usable via `/skill:` |\n| `--no-skills` | — | `false` | Disable skill loading (auto-discovery and explicit) |\n| `--no-agents` | — | `false` | Disable named agent discovery (built-ins and [definition files](/advanced/subagents#named-agents)) |\n\n## Generation parameters\n\n| Flag | Short | Default | Description |\n|------|-------|---------|-------------|\n| `--max-tokens` | — | `8192` | Base cap for output tokens. Auto-raised per-model up to 32768 when the model\'s catalog ceiling is higher and no explicit value is set. |\n| `--temperature` | — | `0.7` | Randomness 0.0–1.0 |\n| `--top-p` | — | `0.95` | Nucleus sampling 0.0–1.0 |\n| `--top-k` | — | `40` | Limit top K tokens |\n| `--stop-sequences` | — | — | Custom stop sequences (comma-separated) |\n| `--frequency-penalty` | — | `0.0` | Penalize frequent tokens (0.0–2.0) |\n| `--presence-penalty` | — | `0.0` | Penalize present tokens (0.0–2.0) |\n| `--thinking-level` | — | `off` | Extended thinking level: off, none, minimal, low, medium, high |\n\n## System\n\n| Flag | Short | Default | Description |\n|------|-------|---------|-------------|\n| `--config` | — | `~/.kit.yml` | Config file path |\n| `--system-prompt` | — | — | System prompt text or file path |\n| `--debug` | — | `false` | Enable debug logging |\n'};export{e as default};
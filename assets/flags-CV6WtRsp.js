const t={frontmatter:{title:"Global Flags",description:"Complete reference for all Kit CLI flags.",hidden:!1,toc:!0,draft:!1},html:`<h1 id="global-flags"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#global-flags"><span class="icon icon-link"></span></a>Global Flags</h1>
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
<td><code>--auto-compact</code></td>
<td>—</td>
<td><code>false</code></td>
<td>Auto-compact conversation near context limit</td>
</tr>
</tbody>
</table>
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
<td>Extended thinking level: off, minimal, low, medium, high</td>
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
</table>`,headings:[{depth:2,text:"Model and provider",id:"model-and-provider"},{depth:2,text:"Session management",id:"session-management"},{depth:2,text:"Behavior",id:"behavior"},{depth:2,text:"Extensions",id:"extensions"},{depth:2,text:"Generation parameters",id:"generation-parameters"},{depth:2,text:"System",id:"system"}],raw:"\n# Global Flags\n\nAll flags can be passed to the root `kit` command.\n\n## Model and provider\n\n| Flag | Short | Default | Description |\n|------|-------|---------|-------------|\n| `--model` | `-m` | `anthropic/claude-sonnet-latest` | Model to use (provider/model format) |\n| `--provider-api-key` | — | — | API key for the provider |\n| `--provider-url` | — | — | Base URL for provider API |\n| `--tls-skip-verify` | — | `false` | Skip TLS certificate verification |\n\n## Session management\n\n| Flag | Short | Default | Description |\n|------|-------|---------|-------------|\n| `--session` | `-s` | — | Open specific JSONL session file |\n| `--continue` | `-c` | `false` | Resume most recent session for current directory |\n| `--resume` | `-r` | `false` | Interactive session picker |\n| `--no-session` | — | `false` | Ephemeral mode, no persistence |\n\n## Behavior\n\nThese flags control Kit's behavior. When a prompt is passed as a positional argument, Kit runs in non-interactive mode.\n\n| Flag | Short | Default | Description |\n|------|-------|---------|-------------|\n| `--quiet` | — | `false` | Suppress all output (non-interactive only) |\n| `--json` | — | `false` | Output response as JSON (non-interactive only) |\n| `--no-exit` | — | `false` | Enter interactive mode after prompt completes |\n| `--max-steps` | — | `0` | Maximum agent steps (0 for unlimited) |\n| `--stream` | — | `true` | Enable streaming output |\n| `--compact` | — | `false` | Enable compact output mode |\n| `--auto-compact` | — | `false` | Auto-compact conversation near context limit |\n\n## Extensions\n\n| Flag | Short | Default | Description |\n|------|-------|---------|-------------|\n| `--extension` | `-e` | — | Load additional extension file(s) (repeatable) |\n| `--no-extensions` | — | `false` | Disable all extensions |\n| `--prompt-template` | — | — | Load a specific prompt template by name |\n| `--no-prompt-templates` | — | `false` | Disable prompt template loading |\n\n## Generation parameters\n\n| Flag | Short | Default | Description |\n|------|-------|---------|-------------|\n| `--max-tokens` | — | `8192` | Base cap for output tokens. Auto-raised per-model up to 32768 when the model's catalog ceiling is higher and no explicit value is set. |\n| `--temperature` | — | `0.7` | Randomness 0.0–1.0 |\n| `--top-p` | — | `0.95` | Nucleus sampling 0.0–1.0 |\n| `--top-k` | — | `40` | Limit top K tokens |\n| `--stop-sequences` | — | — | Custom stop sequences (comma-separated) |\n| `--frequency-penalty` | — | `0.0` | Penalize frequent tokens (0.0–2.0) |\n| `--presence-penalty` | — | `0.0` | Penalize present tokens (0.0–2.0) |\n| `--thinking-level` | — | `off` | Extended thinking level: off, minimal, low, medium, high |\n\n## System\n\n| Flag | Short | Default | Description |\n|------|-------|---------|-------------|\n| `--config` | — | `~/.kit.yml` | Config file path |\n| `--system-prompt` | — | — | System prompt text or file path |\n| `--debug` | — | `false` | Enable debug logging |\n"};export{t as default};

const n={frontmatter:{title:"Configuration",description:"Configure Kit using config files, environment variables, and CLI flags.",hidden:!1,toc:!0,draft:!1},html:`<h1 id="configuration"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#configuration"><span class="icon icon-link"></span></a>Configuration</h1>
<p>Kit looks for configuration in the following locations, in order of priority:</p>
<ol>
<li>CLI flags</li>
<li>Environment variables (with <code>KIT_</code> prefix)</li>
<li><code>./.kit.yml</code> / <code>./.kit.yaml</code> / <code>./.kit.json</code> (project-local)</li>
<li><code>~/.kit.yml</code> / <code>~/.kit.yaml</code> / <code>~/.kit.json</code> (global)</li>
</ol>
<h2 id="basic-configuration"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#basic-configuration"><span class="icon icon-link"></span></a>Basic configuration</h2>
<p>Create <code>~/.kit.yml</code>:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">model</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">anthropic/claude-sonnet-4-5-20250929</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">max-tokens</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#005CC5;--shiki-dark:#79B8FF">4096</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">temperature</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#005CC5;--shiki-dark:#79B8FF">0.7</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">stream</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#005CC5;--shiki-dark:#79B8FF">true</span></span></code></pre>
<h2 id="all-configuration-keys"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#all-configuration-keys"><span class="icon icon-link"></span></a>All configuration keys</h2>
<table>
<thead>
<tr>
<th>Key</th>
<th>Type</th>
<th>Default</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td><code>model</code></td>
<td>string</td>
<td><code>anthropic/claude-sonnet-4-5-20250929</code></td>
<td>Model to use (provider/model format)</td>
</tr>
<tr>
<td><code>max-tokens</code></td>
<td>int</td>
<td><code>4096</code></td>
<td>Maximum tokens in response</td>
</tr>
<tr>
<td><code>temperature</code></td>
<td>float</td>
<td><code>0.7</code></td>
<td>Randomness 0.0–1.0</td>
</tr>
<tr>
<td><code>top-p</code></td>
<td>float</td>
<td><code>0.95</code></td>
<td>Nucleus sampling 0.0–1.0</td>
</tr>
<tr>
<td><code>top-k</code></td>
<td>int</td>
<td><code>40</code></td>
<td>Limit top K tokens</td>
</tr>
<tr>
<td><code>stream</code></td>
<td>bool</td>
<td><code>true</code></td>
<td>Enable streaming output</td>
</tr>
<tr>
<td><code>debug</code></td>
<td>bool</td>
<td><code>false</code></td>
<td>Enable debug logging</td>
</tr>
<tr>
<td><code>compact</code></td>
<td>bool</td>
<td><code>false</code></td>
<td>Enable compact output mode</td>
</tr>
<tr>
<td><code>system-prompt</code></td>
<td>string</td>
<td>—</td>
<td>System prompt text or file path</td>
</tr>
<tr>
<td><code>max-steps</code></td>
<td>int</td>
<td><code>0</code></td>
<td>Maximum agent steps (0 = unlimited)</td>
</tr>
<tr>
<td><code>thinking-level</code></td>
<td>string</td>
<td><code>off</code></td>
<td>Extended thinking: off, minimal, low, medium, high</td>
</tr>
<tr>
<td><code>provider-api-key</code></td>
<td>string</td>
<td>—</td>
<td>API key for the provider</td>
</tr>
<tr>
<td><code>provider-url</code></td>
<td>string</td>
<td>—</td>
<td>Base URL for provider API</td>
</tr>
<tr>
<td><code>tls-skip-verify</code></td>
<td>bool</td>
<td><code>false</code></td>
<td>Skip TLS certificate verification</td>
</tr>
<tr>
<td><code>stop-sequences</code></td>
<td>list</td>
<td>—</td>
<td>Custom stop sequences</td>
</tr>
<tr>
<td><code>theme</code></td>
<td>object or string</td>
<td>—</td>
<td>UI theme (<a href="/themes">inline overrides or file path</a>)</td>
</tr>
</tbody>
</table>
<h2 id="environment-variables"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#environment-variables"><span class="icon icon-link"></span></a>Environment variables</h2>
<p>Any configuration key can be set via environment variable with the <code>KIT_</code> prefix. Hyphens become underscores:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">export</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> KIT_MODEL</span><span style="color:#D73A49;--shiki-dark:#F97583">=</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"openai/gpt-4o"</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">export</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> KIT_MAX_TOKENS</span><span style="color:#D73A49;--shiki-dark:#F97583">=</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"8192"</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">export</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> KIT_TEMPERATURE</span><span style="color:#D73A49;--shiki-dark:#F97583">=</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"0.5"</span></span></code></pre>
<p>Provider API keys use their own environment variables:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">export</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> ANTHROPIC_API_KEY</span><span style="color:#D73A49;--shiki-dark:#F97583">=</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"sk-..."</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">export</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> OPENAI_API_KEY</span><span style="color:#D73A49;--shiki-dark:#F97583">=</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"sk-..."</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">export</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> GOOGLE_API_KEY</span><span style="color:#D73A49;--shiki-dark:#F97583">=</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"..."</span></span></code></pre>
<h2 id="mcp-server-configuration"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#mcp-server-configuration"><span class="icon icon-link"></span></a>MCP server configuration</h2>
<p>Add external MCP servers to your <code>.kit.yml</code>:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">mcpServers</span><span style="color:#24292E;--shiki-dark:#E1E4E8">:</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">  filesystem</span><span style="color:#24292E;--shiki-dark:#E1E4E8">:</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">    type</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">local</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">    command</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: [</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"npx"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"-y"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"@modelcontextprotocol/server-filesystem"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"/path/to/allowed"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">]</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">    environment</span><span style="color:#24292E;--shiki-dark:#E1E4E8">:</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">      LOG_LEVEL</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"info"</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">    allowedTools</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: [</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"read_file"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"write_file"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">]</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">    excludedTools</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: [</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"delete_file"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">]</span></span>
<span class="line"></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">  search</span><span style="color:#24292E;--shiki-dark:#E1E4E8">:</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">    type</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">remote</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">    url</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"https://mcp.example.com/search"</span></span></code></pre>
<h3 id="mcp-server-fields"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#mcp-server-fields"><span class="icon icon-link"></span></a>MCP server fields</h3>
<table>
<thead>
<tr>
<th>Field</th>
<th>Type</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td><code>type</code></td>
<td>string</td>
<td><code>local</code> (stdio) or <code>remote</code> (streamable HTTP)</td>
</tr>
<tr>
<td><code>command</code></td>
<td>list</td>
<td>Command and args for local servers</td>
</tr>
<tr>
<td><code>environment</code></td>
<td>map</td>
<td>Environment variables for the server process</td>
</tr>
<tr>
<td><code>url</code></td>
<td>string</td>
<td>URL for remote servers</td>
</tr>
<tr>
<td><code>allowedTools</code></td>
<td>list</td>
<td>Whitelist of tool names to expose</td>
</tr>
<tr>
<td><code>excludedTools</code></td>
<td>list</td>
<td>Blacklist of tool names to hide</td>
</tr>
</tbody>
</table>
<p>A legacy format with <code>transport</code>, <code>args</code>, <code>env</code>, and <code>headers</code> fields is also supported.</p>
<h2 id="theme-configuration"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#theme-configuration"><span class="icon icon-link"></span></a>Theme configuration</h2>
<p>Set theme colors inline or reference an external file:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D"># Inline partial overrides (unspecified fields inherit from default)</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">theme</span><span style="color:#24292E;--shiki-dark:#E1E4E8">:</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">  primary</span><span style="color:#24292E;--shiki-dark:#E1E4E8">:</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">    light</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#8839ef"</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">    dark</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#cba6f7"</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">  error</span><span style="color:#24292E;--shiki-dark:#E1E4E8">:</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">    dark</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#FF0000"</span></span></code></pre>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D"># Reference external theme file</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">theme</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"./themes/my-custom-theme.yml"</span></span></code></pre>
<p>See <a href="/themes">Themes</a> for the full theme file format, built-in themes, and the extension theme API.</p>`,headings:[{depth:2,text:"Basic configuration",id:"basic-configuration"},{depth:2,text:"All configuration keys",id:"all-configuration-keys"},{depth:2,text:"Environment variables",id:"environment-variables"},{depth:2,text:"MCP server configuration",id:"mcp-server-configuration"},{depth:3,text:"MCP server fields",id:"mcp-server-fields"},{depth:2,text:"Theme configuration",id:"theme-configuration"}],raw:'\n# Configuration\n\nKit looks for configuration in the following locations, in order of priority:\n\n1. CLI flags\n2. Environment variables (with `KIT_` prefix)\n3. `./.kit.yml` / `./.kit.yaml` / `./.kit.json` (project-local)\n4. `~/.kit.yml` / `~/.kit.yaml` / `~/.kit.json` (global)\n\n## Basic configuration\n\nCreate `~/.kit.yml`:\n\n```yaml\nmodel: anthropic/claude-sonnet-4-5-20250929\nmax-tokens: 4096\ntemperature: 0.7\nstream: true\n```\n\n## All configuration keys\n\n| Key | Type | Default | Description |\n|-----|------|---------|-------------|\n| `model` | string | `anthropic/claude-sonnet-4-5-20250929` | Model to use (provider/model format) |\n| `max-tokens` | int | `4096` | Maximum tokens in response |\n| `temperature` | float | `0.7` | Randomness 0.0–1.0 |\n| `top-p` | float | `0.95` | Nucleus sampling 0.0–1.0 |\n| `top-k` | int | `40` | Limit top K tokens |\n| `stream` | bool | `true` | Enable streaming output |\n| `debug` | bool | `false` | Enable debug logging |\n| `compact` | bool | `false` | Enable compact output mode |\n| `system-prompt` | string | — | System prompt text or file path |\n| `max-steps` | int | `0` | Maximum agent steps (0 = unlimited) |\n| `thinking-level` | string | `off` | Extended thinking: off, minimal, low, medium, high |\n| `provider-api-key` | string | — | API key for the provider |\n| `provider-url` | string | — | Base URL for provider API |\n| `tls-skip-verify` | bool | `false` | Skip TLS certificate verification |\n| `stop-sequences` | list | — | Custom stop sequences |\n| `theme` | object or string | — | UI theme ([inline overrides or file path](/themes)) |\n\n## Environment variables\n\nAny configuration key can be set via environment variable with the `KIT_` prefix. Hyphens become underscores:\n\n```bash\nexport KIT_MODEL="openai/gpt-4o"\nexport KIT_MAX_TOKENS="8192"\nexport KIT_TEMPERATURE="0.5"\n```\n\nProvider API keys use their own environment variables:\n\n```bash\nexport ANTHROPIC_API_KEY="sk-..."\nexport OPENAI_API_KEY="sk-..."\nexport GOOGLE_API_KEY="..."\n```\n\n## MCP server configuration\n\nAdd external MCP servers to your `.kit.yml`:\n\n```yaml\nmcpServers:\n  filesystem:\n    type: local\n    command: ["npx", "-y", "@modelcontextprotocol/server-filesystem", "/path/to/allowed"]\n    environment:\n      LOG_LEVEL: "info"\n    allowedTools: ["read_file", "write_file"]\n    excludedTools: ["delete_file"]\n\n  search:\n    type: remote\n    url: "https://mcp.example.com/search"\n```\n\n### MCP server fields\n\n| Field | Type | Description |\n|-------|------|-------------|\n| `type` | string | `local` (stdio) or `remote` (streamable HTTP) |\n| `command` | list | Command and args for local servers |\n| `environment` | map | Environment variables for the server process |\n| `url` | string | URL for remote servers |\n| `allowedTools` | list | Whitelist of tool names to expose |\n| `excludedTools` | list | Blacklist of tool names to hide |\n\nA legacy format with `transport`, `args`, `env`, and `headers` fields is also supported.\n\n## Theme configuration\n\nSet theme colors inline or reference an external file:\n\n```yaml\n# Inline partial overrides (unspecified fields inherit from default)\ntheme:\n  primary:\n    light: "#8839ef"\n    dark: "#cba6f7"\n  error:\n    dark: "#FF0000"\n```\n\n```yaml\n# Reference external theme file\ntheme: "./themes/my-custom-theme.yml"\n```\n\nSee [Themes](/themes) for the full theme file format, built-in themes, and the extension theme API.\n'};export{n as default};

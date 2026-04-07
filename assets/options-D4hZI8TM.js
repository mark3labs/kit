const s={frontmatter:{title:"SDK Options",description:"Configuration options for the Kit Go SDK.",hidden:!1,toc:!0,draft:!1},html:`<h1 id="sdk-options"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#sdk-options"><span class="icon icon-link"></span></a>SDK Options</h1>
<p>Pass an <code>Options</code> struct to <code>kit.New()</code> to configure the Kit instance.</p>
<h2 id="full-options-reference"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#full-options-reference"><span class="icon icon-link"></span></a>Full options reference</h2>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">host, err </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> kit.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">New</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(ctx, </span><span style="color:#D73A49;--shiki-dark:#F97583">&amp;</span><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Options</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // Model</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Model:        </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"ollama/llama3"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    SystemPrompt: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"You are a helpful bot"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    ConfigFile:   </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"/path/to/config.yml"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // Behavior</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    MaxSteps:     </span><span style="color:#005CC5;--shiki-dark:#79B8FF">10</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Streaming:    </span><span style="color:#005CC5;--shiki-dark:#79B8FF">true</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Quiet:        </span><span style="color:#005CC5;--shiki-dark:#79B8FF">true</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Debug:        </span><span style="color:#005CC5;--shiki-dark:#79B8FF">true</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // Session</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    SessionPath:  </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"./session.jsonl"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    SessionDir:   </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"/custom/sessions/"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Continue:     </span><span style="color:#005CC5;--shiki-dark:#79B8FF">true</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    NoSession:    </span><span style="color:#005CC5;--shiki-dark:#79B8FF">true</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // Tools</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Tools:            []</span><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Tool</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span><span style="color:#D73A49;--shiki-dark:#F97583">...</span><span style="color:#24292E;--shiki-dark:#E1E4E8">},     </span><span style="color:#6A737D;--shiki-dark:#6A737D">// Replace default tool set entirely</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    ExtraTools:       []</span><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Tool</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span><span style="color:#D73A49;--shiki-dark:#F97583">...</span><span style="color:#24292E;--shiki-dark:#E1E4E8">},     </span><span style="color:#6A737D;--shiki-dark:#6A737D">// Add tools alongside defaults</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    DisableCoreTools: </span><span style="color:#005CC5;--shiki-dark:#79B8FF">true</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,                </span><span style="color:#6A737D;--shiki-dark:#6A737D">// Use no core tools (0 tools, for chat-only)</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // Configuration</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    SkipConfig:   </span><span style="color:#005CC5;--shiki-dark:#79B8FF">true</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,                   </span><span style="color:#6A737D;--shiki-dark:#6A737D">// Skip .kit.yml files (viper defaults + env vars still apply)</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // Compaction</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    AutoCompact:  </span><span style="color:#005CC5;--shiki-dark:#79B8FF">true</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // Skills</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Skills:       []</span><span style="color:#D73A49;--shiki-dark:#F97583">string</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"/path/to/skill.md"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">},</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    SkillsDir:    </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"/path/to/skills/"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span></code></pre>
<h2 id="options-fields"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#options-fields"><span class="icon icon-link"></span></a>Options fields</h2>
<table>
<thead>
<tr>
<th>Field</th>
<th>Type</th>
<th>Default</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td><code>Model</code></td>
<td><code>string</code></td>
<td>config default</td>
<td>Model string (provider/model format)</td>
</tr>
<tr>
<td><code>SystemPrompt</code></td>
<td><code>string</code></td>
<td>—</td>
<td>System prompt text or file path</td>
</tr>
<tr>
<td><code>ConfigFile</code></td>
<td><code>string</code></td>
<td><code>~/.kit.yml</code></td>
<td>Path to config file</td>
</tr>
<tr>
<td><code>MaxSteps</code></td>
<td><code>int</code></td>
<td><code>0</code></td>
<td>Max agent steps (0 = unlimited)</td>
</tr>
<tr>
<td><code>Streaming</code></td>
<td><code>bool</code></td>
<td><code>true</code></td>
<td>Enable streaming output</td>
</tr>
<tr>
<td><code>Quiet</code></td>
<td><code>bool</code></td>
<td><code>false</code></td>
<td>Suppress output</td>
</tr>
<tr>
<td><code>Debug</code></td>
<td><code>bool</code></td>
<td><code>false</code></td>
<td>Enable debug logging</td>
</tr>
<tr>
<td><code>SessionPath</code></td>
<td><code>string</code></td>
<td>—</td>
<td>Open a specific session file</td>
</tr>
<tr>
<td><code>SessionDir</code></td>
<td><code>string</code></td>
<td>—</td>
<td>Base directory for session discovery</td>
</tr>
<tr>
<td><code>Continue</code></td>
<td><code>bool</code></td>
<td><code>false</code></td>
<td>Resume most recent session</td>
</tr>
<tr>
<td><code>NoSession</code></td>
<td><code>bool</code></td>
<td><code>false</code></td>
<td>Ephemeral mode (no persistence)</td>
</tr>
<tr>
<td><code>Tools</code></td>
<td><code>[]Tool</code></td>
<td>—</td>
<td>Replace the entire default tool set</td>
</tr>
<tr>
<td><code>ExtraTools</code></td>
<td><code>[]Tool</code></td>
<td>—</td>
<td>Additional tools alongside core/MCP/extension tools</td>
</tr>
<tr>
<td><code>DisableCoreTools</code></td>
<td><code>bool</code></td>
<td><code>false</code></td>
<td>Use no core tools (0 tools, for chat-only)</td>
</tr>
<tr>
<td><code>SkipConfig</code></td>
<td><code>bool</code></td>
<td><code>false</code></td>
<td>Skip .kit.yml file loading</td>
</tr>
<tr>
<td><code>AutoCompact</code></td>
<td><code>bool</code></td>
<td><code>false</code></td>
<td>Auto-compact when near context limit</td>
</tr>
<tr>
<td><code>CompactionOptions</code></td>
<td><code>*CompactionOptions</code></td>
<td>—</td>
<td>Configuration for auto-compaction</td>
</tr>
<tr>
<td><code>Skills</code></td>
<td><code>[]string</code></td>
<td>—</td>
<td>Explicit skill files/dirs to load</td>
</tr>
<tr>
<td><code>SkillsDir</code></td>
<td><code>string</code></td>
<td>—</td>
<td>Override default skills directory</td>
</tr>
</tbody>
</table>`,headings:[{depth:2,text:"Full options reference",id:"full-options-reference"},{depth:2,text:"Options fields",id:"options-fields"}],raw:'\n# SDK Options\n\nPass an `Options` struct to `kit.New()` to configure the Kit instance.\n\n## Full options reference\n\n```go\nhost, err := kit.New(ctx, &kit.Options{\n    // Model\n    Model:        "ollama/llama3",\n    SystemPrompt: "You are a helpful bot",\n    ConfigFile:   "/path/to/config.yml",\n\n    // Behavior\n    MaxSteps:     10,\n    Streaming:    true,\n    Quiet:        true,\n    Debug:        true,\n\n    // Session\n    SessionPath:  "./session.jsonl",\n    SessionDir:   "/custom/sessions/",\n    Continue:     true,\n    NoSession:    true,\n\n    // Tools\n    Tools:            []kit.Tool{...},     // Replace default tool set entirely\n    ExtraTools:       []kit.Tool{...},     // Add tools alongside defaults\n    DisableCoreTools: true,                // Use no core tools (0 tools, for chat-only)\n\n    // Configuration\n    SkipConfig:   true,                   // Skip .kit.yml files (viper defaults + env vars still apply)\n\n    // Compaction\n    AutoCompact:  true,\n\n    // Skills\n    Skills:       []string{"/path/to/skill.md"},\n    SkillsDir:    "/path/to/skills/",\n})\n```\n\n## Options fields\n\n| Field | Type | Default | Description |\n|-------|------|---------|-------------|\n| `Model` | `string` | config default | Model string (provider/model format) |\n| `SystemPrompt` | `string` | — | System prompt text or file path |\n| `ConfigFile` | `string` | `~/.kit.yml` | Path to config file |\n| `MaxSteps` | `int` | `0` | Max agent steps (0 = unlimited) |\n| `Streaming` | `bool` | `true` | Enable streaming output |\n| `Quiet` | `bool` | `false` | Suppress output |\n| `Debug` | `bool` | `false` | Enable debug logging |\n| `SessionPath` | `string` | — | Open a specific session file |\n| `SessionDir` | `string` | — | Base directory for session discovery |\n| `Continue` | `bool` | `false` | Resume most recent session |\n| `NoSession` | `bool` | `false` | Ephemeral mode (no persistence) |\n| `Tools` | `[]Tool` | — | Replace the entire default tool set |\n| `ExtraTools` | `[]Tool` | — | Additional tools alongside core/MCP/extension tools |\n| `DisableCoreTools` | `bool` | `false` | Use no core tools (0 tools, for chat-only) |\n| `SkipConfig` | `bool` | `false` | Skip .kit.yml file loading |\n| `AutoCompact` | `bool` | `false` | Auto-compact when near context limit |\n| `CompactionOptions` | `*CompactionOptions` | — | Configuration for auto-compaction |\n| `Skills` | `[]string` | — | Explicit skill files/dirs to load |\n| `SkillsDir` | `string` | — | Override default skills directory |\n'};export{s as default};

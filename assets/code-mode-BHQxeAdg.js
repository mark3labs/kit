var e={frontmatter:{title:`Code Mode`,description:`Let the model write a JavaScript program that calls Kit's tools, so intermediate results do not fill the context.`,hidden:!1,toc:!0,ogImage:`/og-image.png`,draft:!1},html:`<h1 id="code-mode"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#code-mode"><span class="icon icon-link"></span></a>Code Mode</h1>
<p>In code mode the model gets one more tool, <code>codemode</code>. Its input is a
JavaScript program. The program calls Kit's other tools, combines and filters
their results, and returns a small answer. <strong>Only the program's output goes
back to the model</strong> — the results of the tool calls inside the program do not.</p>
<p>Use it to:</p>
<ul>
<li>chain dependent calls without a round trip to the model for each one,</li>
<li>run independent calls in parallel,</li>
<li>loop over many items (files, issues, records),</li>
<li>read large outputs and keep only the facts you need.</li>
</ul>
<p>Code mode is off by default.</p>
<h2 id="enable-it"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#enable-it"><span class="icon icon-link"></span></a>Enable it</h2>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> --codemode</span></span></code></pre>
<p>Or in <code>.kit.yml</code>:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">codemode</span><span style="color:#24292E;--shiki-dark:#E1E4E8">:</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">  enabled</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#005CC5;--shiki-dark:#79B8FF">true</span></span></code></pre>
<p>Naming it in <code>include-core-tools</code> also enables it. <code>--codemode</code> works together
with <code>--no-core-tools</code>: the model then reaches MCP tools only through scripts.</p>
<h2 id="what-a-script-looks-like"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#what-a-script-looks-like"><span class="icon icon-link"></span></a>What a script looks like</h2>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">const</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> files</span><span style="color:#D73A49;--shiki-dark:#F97583"> =</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> (</span><span style="color:#D73A49;--shiki-dark:#F97583">await</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> tools.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ls</span><span style="color:#24292E;--shiki-dark:#E1E4E8">({})).</span><span style="color:#6F42C1;--shiki-dark:#B392F0">split</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"</span><span style="color:#005CC5;--shiki-dark:#79B8FF">\\n</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">).</span><span style="color:#6F42C1;--shiki-dark:#B392F0">filter</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#E36209;--shiki-dark:#FFAB70">f</span><span style="color:#D73A49;--shiki-dark:#F97583"> =&gt;</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> f.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">endsWith</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">".go"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">))</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">const</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> contents</span><span style="color:#D73A49;--shiki-dark:#F97583"> =</span><span style="color:#D73A49;--shiki-dark:#F97583"> await</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> Promise</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">all</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(files.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">map</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#E36209;--shiki-dark:#FFAB70">f</span><span style="color:#D73A49;--shiki-dark:#F97583"> =&gt;</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> tools.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">read</span><span style="color:#24292E;--shiki-dark:#E1E4E8">({ path: f })))</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">return</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> files.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">map</span><span style="color:#24292E;--shiki-dark:#E1E4E8">((</span><span style="color:#E36209;--shiki-dark:#FFAB70">file</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#E36209;--shiki-dark:#FFAB70">i</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) </span><span style="color:#D73A49;--shiki-dark:#F97583">=&gt;</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> ({ file, todo: (contents[i].</span><span style="color:#6F42C1;--shiki-dark:#B392F0">match</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">/</span><span style="color:#032F62;--shiki-dark:#DBEDFF">TODO: (</span><span style="color:#005CC5;--shiki-dark:#79B8FF">.</span><span style="color:#D73A49;--shiki-dark:#F97583">*</span><span style="color:#032F62;--shiki-dark:#DBEDFF">)</span><span style="color:#032F62;--shiki-dark:#9ECBFF">/</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) </span><span style="color:#D73A49;--shiki-dark:#F97583">||</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> [])[</span><span style="color:#005CC5;--shiki-dark:#79B8FF">1</span><span style="color:#24292E;--shiki-dark:#E1E4E8">] }))</span></span></code></pre>
<p>The model receives:</p>
<pre><code>Script completed in 1ms.
Tool calls: 7 — ls ×1, read ×6

Output:
[ { "file": "f1.go", "todo": "item 1" }, ... ]
</code></pre>
<h3 id="rules-for-scripts"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#rules-for-scripts"><span class="icon icon-link"></span></a>Rules for scripts</h3>
<ul>
<li>The code runs inside an <code>async</code> function. Use <code>await</code>, and use <code>return</code> to
send the result. Strings go back as-is; other values go back as JSON.</li>
<li>Built-in tools are <code>tools.&lt;name&gt;(args)</code>. MCP tools are grouped by server:
the tool <code>github__list_issues</code> is <code>tools.github.list_issues(args)</code>.</li>
<li>Each call takes one object argument and resolves to the tool's text output.
Use <code>JSON.parse</code> when a tool returns JSON.</li>
<li>A failed call throws an <code>Error</code> whose <code>name</code> is <code>"ToolError"</code>, with the
properties <code>tool</code> and <code>output</code>. Catch it with <code>try</code>/<code>catch</code>, or use
<code>Promise.allSettled</code>.</li>
<li>The sandbox has no filesystem, network, process, timer or module access.
The only way out is through tools.</li>
</ul>
<h3 id="helpers"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#helpers"><span class="icon icon-link"></span></a>Helpers</h3>
<table>
<thead>
<tr>
<th>Helper</th>
<th>Purpose</th>
</tr>
</thead>
<tbody>
<tr>
<td><code>text(value)</code></td>
<td>Add to the output</td>
</tr>
<tr>
<td><code>console.log/info/warn/error(...)</code></td>
<td>Add log lines (shown to the model and streamed to the TUI)</td>
</tr>
<tr>
<td><code>call(name, args)</code></td>
<td>Call a tool by its full name, e.g. <code>call("github__list_issues", {...})</code></td>
</tr>
<tr>
<td><code>searchTools(query, {limit, namespace})</code></td>
<td>Find tools by keyword</td>
</tr>
<tr>
<td><code>describeTool(name)</code></td>
<td>Show a tool's full signature and input schema</td>
</tr>
<tr>
<td><code>ALL_TOOLS</code></td>
<td>Every tool a script can call</td>
</tr>
<tr>
<td><code>store(key, value)</code> / <code>load(key)</code></td>
<td>Keep JSON values across scripts for the life of the Kit instance. <code>store(key, undefined)</code> deletes</td>
</tr>
<tr>
<td><code>sleep(ms)</code></td>
<td>Wait</td>
</tr>
</tbody>
</table>
<h2 id="hooks-and-extensions-see-every-call"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#hooks-and-extensions-see-every-call"><span class="icon icon-link"></span></a>Hooks and extensions see every call</h2>
<p>Calls made inside a script go through the same tool wrappers as direct calls.
Extension <code>OnToolCall</code> / <code>OnToolResult</code> handlers and SDK
<code>OnBeforeToolCall</code> / <code>OnAfterToolResult</code> hooks fire for each one, so a hook
that blocks a tool blocks it inside scripts too. The <code>codemode</code> tool itself
cannot be called from a script.</p>
<h2 id="exposure-hide-tools-from-the-model"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#exposure-hide-tools-from-the-model"><span class="icon icon-link"></span></a>Exposure: hide tools from the model</h2>
<p>By default the model still sees every tool directly, and scripts can call
them too. Large MCP servers can cost thousands of tokens of tool schemas on
every request. Exposure rules move tools out of the model's tool list and into
the code mode catalog:</p>
<table>
<thead>
<tr>
<th>Exposure</th>
<th>Model sees it as a tool</th>
<th>Scripts can call it</th>
<th>Listed in the code mode catalog</th>
</tr>
</thead>
<tbody>
<tr>
<td><code>direct</code> (default)</td>
<td>yes</td>
<td>yes</td>
<td>short form</td>
</tr>
<tr>
<td><code>codemode</code></td>
<td>no</td>
<td>yes</td>
<td>full signature</td>
</tr>
<tr>
<td><code>deferred</code></td>
<td>no</td>
<td>yes</td>
<td>no — found with <code>searchTools()</code></td>
</tr>
<tr>
<td><code>model-only</code></td>
<td>yes</td>
<td>no</td>
<td>no</td>
</tr>
</tbody>
</table>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">codemode</span><span style="color:#24292E;--shiki-dark:#E1E4E8">:</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">  enabled</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#005CC5;--shiki-dark:#79B8FF">true</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">  mcp-exposure</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">codemode</span><span style="color:#6A737D;--shiki-dark:#6A737D">       # default for every MCP tool</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">  exposure</span><span style="color:#24292E;--shiki-dark:#E1E4E8">:                    </span><span style="color:#6A737D;--shiki-dark:#6A737D"># glob rules; the most specific pattern wins</span></span>
<span class="line"><span style="color:#032F62;--shiki-dark:#9ECBFF">    "github__*"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">codemode</span></span>
<span class="line"><span style="color:#032F62;--shiki-dark:#9ECBFF">    "github__create_issue"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">direct</span></span>
<span class="line"><span style="color:#032F62;--shiki-dark:#9ECBFF">    "linear__*"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">deferred</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">    subagent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">model-only</span></span></code></pre>
<p>Exact names win over globs, and longer patterns win over shorter ones.
Matching ignores case.</p>
<p>The catalog in the tool description is limited by <code>catalog-budget</code>. Namespaces
share the budget round-robin, so one big server cannot crowd out the others.
Tools left out are still callable; the description tells the model to use
<code>searchTools()</code>.</p>
<h2 id="limits"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#limits"><span class="icon icon-link"></span></a>Limits</h2>
<table>
<thead>
<tr>
<th>Setting</th>
<th>Default</th>
<th>Meaning</th>
</tr>
</thead>
<tbody>
<tr>
<td><code>timeout</code></td>
<td><code>30</code></td>
<td>Seconds per script</td>
</tr>
<tr>
<td><code>max-tool-calls</code></td>
<td><code>50</code></td>
<td>Tool calls per script</td>
</tr>
<tr>
<td><code>max-concurrency</code></td>
<td><code>8</code></td>
<td>Parallel tool calls per script</td>
</tr>
<tr>
<td><code>max-output-bytes</code></td>
<td><code>100000</code></td>
<td>Result size the model receives. The full output goes to a temporary file, and the result gives its path</td>
</tr>
<tr>
<td><code>memory-limit-mb</code></td>
<td><code>256</code></td>
<td>Approximate heap growth limit; <code>-1</code> disables it</td>
</tr>
<tr>
<td><code>catalog-budget</code></td>
<td><code>12000</code></td>
<td>Bytes of tool list in the tool description</td>
</tr>
</tbody>
</table>
<p>A script that passes a limit stops with a typed error the model can act on:
<code>TimeoutExceeded</code>, <code>ToolCallLimitExceeded</code>, <code>MemoryLimitExceeded</code>. Other
kinds are <code>ParseError</code>, <code>UnknownTool</code> (with suggestions), <code>InvalidToolInput</code>
(a required argument is missing), <code>ToolFailure</code>, <code>ExecutionFailure</code>,
<code>UnsettledPromise</code> and <code>Cancelled</code>. Errors carry the line and column in the
submitted code.</p>
<p>Press <kbd>Esc</kbd> twice within two seconds to stop a running script. The
first press arms cancellation; the second cancels the step and its tool calls
in flight. Whether a call stops immediately also depends on the tool honoring
its context.</p>
<p>::: info
The JavaScript engine has no per-script heap limit. The memory limit is a
watchdog on process heap growth while the script runs. It stops runaway
scripts; it is not a precise byte budget.
:::</p>
<h2 id="in-the-tui"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#in-the-tui"><span class="icon icon-link"></span></a>In the TUI</h2>
<p>While a script runs, the transcript streams its call log:</p>
<pre><code>· Running script
   ▸ ls
   ✓ ls (45µs)
   ▸ read {"path":"f1.go"}
   ✓ read (210µs)
</code></pre>
<p>When it finishes, the block shows the script with line numbers and syntax
highlighting, followed by the result.</p>
<h2 id="sdk"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#sdk"><span class="icon icon-link"></span></a>SDK</h2>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">host, err </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> kit.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">New</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(ctx, </span><span style="color:#D73A49;--shiki-dark:#F97583">&amp;</span><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Options</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    CodeMode: </span><span style="color:#D73A49;--shiki-dark:#F97583">&amp;</span><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">CodeModeOptions</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">        Enabled:     </span><span style="color:#005CC5;--shiki-dark:#79B8FF">true</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">        MCPExposure: kit.CodeModeExposureScriptOnly,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">        Exposure:    </span><span style="color:#D73A49;--shiki-dark:#F97583">map</span><span style="color:#24292E;--shiki-dark:#E1E4E8">[</span><span style="color:#D73A49;--shiki-dark:#F97583">string</span><span style="color:#24292E;--shiki-dark:#E1E4E8">]</span><span style="color:#D73A49;--shiki-dark:#F97583">string</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"subagent"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: kit.CodeModeExposureModelOnly},</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">        Timeout:     time.Minute,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    },</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span></code></pre>
<p>Zero fields fall back to the <code>codemode</code> config section. To use code mode with
a custom tool set, add <code>kit.NewCodeModeTool(opts)</code> to <code>Options.Tools</code> or
<code>Options.ExtraTools</code>; Kit gives it the live tool set.</p>
<p>Each code mode <code>ToolResultEvent</code> carries <code>Metadata.CodeMode</code>, with the nested
calls (name, arguments, status, duration, error), the wall time, the error
kind, and the path of the full output file when the result was truncated.</p>`,headings:[{depth:2,text:`Enable it`,id:`enable-it`},{depth:2,text:`What a script looks like`,id:`what-a-script-looks-like`},{depth:3,text:`Rules for scripts`,id:`rules-for-scripts`},{depth:3,text:`Helpers`,id:`helpers`},{depth:2,text:`Hooks and extensions see every call`,id:`hooks-and-extensions-see-every-call`},{depth:2,text:`Exposure: hide tools from the model`,id:`exposure-hide-tools-from-the-model`},{depth:2,text:`Limits`,id:`limits`},{depth:2,text:`In the TUI`,id:`in-the-tui`},{depth:2,text:`SDK`,id:`sdk`}],raw:`
# Code Mode

In code mode the model gets one more tool, \`codemode\`. Its input is a
JavaScript program. The program calls Kit's other tools, combines and filters
their results, and returns a small answer. **Only the program's output goes
back to the model** — the results of the tool calls inside the program do not.

Use it to:

- chain dependent calls without a round trip to the model for each one,
- run independent calls in parallel,
- loop over many items (files, issues, records),
- read large outputs and keep only the facts you need.

Code mode is off by default.

## Enable it

\`\`\`bash
kit --codemode
\`\`\`

Or in \`.kit.yml\`:

\`\`\`yaml
codemode:
  enabled: true
\`\`\`

Naming it in \`include-core-tools\` also enables it. \`--codemode\` works together
with \`--no-core-tools\`: the model then reaches MCP tools only through scripts.

## What a script looks like

\`\`\`js
const files = (await tools.ls({})).split("\\n").filter(f => f.endsWith(".go"))
const contents = await Promise.all(files.map(f => tools.read({ path: f })))
return files.map((file, i) => ({ file, todo: (contents[i].match(/TODO: (.*)/) || [])[1] }))
\`\`\`

The model receives:

\`\`\`
Script completed in 1ms.
Tool calls: 7 — ls ×1, read ×6

Output:
[ { "file": "f1.go", "todo": "item 1" }, ... ]
\`\`\`

### Rules for scripts

- The code runs inside an \`async\` function. Use \`await\`, and use \`return\` to
  send the result. Strings go back as-is; other values go back as JSON.
- Built-in tools are \`tools.<name>(args)\`. MCP tools are grouped by server:
  the tool \`github__list_issues\` is \`tools.github.list_issues(args)\`.
- Each call takes one object argument and resolves to the tool's text output.
  Use \`JSON.parse\` when a tool returns JSON.
- A failed call throws an \`Error\` whose \`name\` is \`"ToolError"\`, with the
  properties \`tool\` and \`output\`. Catch it with \`try\`/\`catch\`, or use
  \`Promise.allSettled\`.
- The sandbox has no filesystem, network, process, timer or module access.
  The only way out is through tools.

### Helpers

| Helper | Purpose |
|--------|---------|
| \`text(value)\` | Add to the output |
| \`console.log/info/warn/error(...)\` | Add log lines (shown to the model and streamed to the TUI) |
| \`call(name, args)\` | Call a tool by its full name, e.g. \`call("github__list_issues", {...})\` |
| \`searchTools(query, {limit, namespace})\` | Find tools by keyword |
| \`describeTool(name)\` | Show a tool's full signature and input schema |
| \`ALL_TOOLS\` | Every tool a script can call |
| \`store(key, value)\` / \`load(key)\` | Keep JSON values across scripts for the life of the Kit instance. \`store(key, undefined)\` deletes |
| \`sleep(ms)\` | Wait |

## Hooks and extensions see every call

Calls made inside a script go through the same tool wrappers as direct calls.
Extension \`OnToolCall\` / \`OnToolResult\` handlers and SDK
\`OnBeforeToolCall\` / \`OnAfterToolResult\` hooks fire for each one, so a hook
that blocks a tool blocks it inside scripts too. The \`codemode\` tool itself
cannot be called from a script.

## Exposure: hide tools from the model

By default the model still sees every tool directly, and scripts can call
them too. Large MCP servers can cost thousands of tokens of tool schemas on
every request. Exposure rules move tools out of the model's tool list and into
the code mode catalog:

| Exposure | Model sees it as a tool | Scripts can call it | Listed in the code mode catalog |
|----------|:-:|:-:|:-:|
| \`direct\` (default) | yes | yes | short form |
| \`codemode\` | no | yes | full signature |
| \`deferred\` | no | yes | no — found with \`searchTools()\` |
| \`model-only\` | yes | no | no |

\`\`\`yaml
codemode:
  enabled: true
  mcp-exposure: codemode       # default for every MCP tool
  exposure:                    # glob rules; the most specific pattern wins
    "github__*": codemode
    "github__create_issue": direct
    "linear__*": deferred
    subagent: model-only
\`\`\`

Exact names win over globs, and longer patterns win over shorter ones.
Matching ignores case.

The catalog in the tool description is limited by \`catalog-budget\`. Namespaces
share the budget round-robin, so one big server cannot crowd out the others.
Tools left out are still callable; the description tells the model to use
\`searchTools()\`.

## Limits

| Setting | Default | Meaning |
|---------|---------|---------|
| \`timeout\` | \`30\` | Seconds per script |
| \`max-tool-calls\` | \`50\` | Tool calls per script |
| \`max-concurrency\` | \`8\` | Parallel tool calls per script |
| \`max-output-bytes\` | \`100000\` | Result size the model receives. The full output goes to a temporary file, and the result gives its path |
| \`memory-limit-mb\` | \`256\` | Approximate heap growth limit; \`-1\` disables it |
| \`catalog-budget\` | \`12000\` | Bytes of tool list in the tool description |

A script that passes a limit stops with a typed error the model can act on:
\`TimeoutExceeded\`, \`ToolCallLimitExceeded\`, \`MemoryLimitExceeded\`. Other
kinds are \`ParseError\`, \`UnknownTool\` (with suggestions), \`InvalidToolInput\`
(a required argument is missing), \`ToolFailure\`, \`ExecutionFailure\`,
\`UnsettledPromise\` and \`Cancelled\`. Errors carry the line and column in the
submitted code.

Press <kbd>Esc</kbd> twice within two seconds to stop a running script. The
first press arms cancellation; the second cancels the step and its tool calls
in flight. Whether a call stops immediately also depends on the tool honoring
its context.

::: info
The JavaScript engine has no per-script heap limit. The memory limit is a
watchdog on process heap growth while the script runs. It stops runaway
scripts; it is not a precise byte budget.
:::

## In the TUI

While a script runs, the transcript streams its call log:

\`\`\`
· Running script
   ▸ ls
   ✓ ls (45µs)
   ▸ read {"path":"f1.go"}
   ✓ read (210µs)
\`\`\`

When it finishes, the block shows the script with line numbers and syntax
highlighting, followed by the result.

## SDK

\`\`\`go
host, err := kit.New(ctx, &kit.Options{
    CodeMode: &kit.CodeModeOptions{
        Enabled:     true,
        MCPExposure: kit.CodeModeExposureScriptOnly,
        Exposure:    map[string]string{"subagent": kit.CodeModeExposureModelOnly},
        Timeout:     time.Minute,
    },
})
\`\`\`

Zero fields fall back to the \`codemode\` config section. To use code mode with
a custom tool set, add \`kit.NewCodeModeTool(opts)\` to \`Options.Tools\` or
\`Options.ExtraTools\`; Kit gives it the live tool set.

Each code mode \`ToolResultEvent\` carries \`Metadata.CodeMode\`, with the nested
calls (name, arguments, status, duration, error), the wall time, the error
kind, and the path of the full output file when the result was truncated.
`};export{e as default};
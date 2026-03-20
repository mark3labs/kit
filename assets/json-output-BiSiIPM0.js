const s={frontmatter:{title:"JSON Output",description:"Machine-readable JSON output for scripting and automation.",hidden:!1,toc:!0,draft:!1},html:`<h1 id="json-output"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#json-output"><span class="icon icon-link"></span></a>JSON Output</h1>
<p>Use the <code>--json</code> flag to get structured output for scripting and automation:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> "Explain main.go"</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> --json</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> --quiet</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> --no-session</span></span></code></pre>
<h2 id="response-format"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#response-format"><span class="icon icon-link"></span></a>Response format</h2>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span></span>
<span class="line"><span style="color:#005CC5;--shiki-dark:#79B8FF">  "response"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"Final assistant response text"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#005CC5;--shiki-dark:#79B8FF">  "model"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"anthropic/claude-haiku-latest"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#005CC5;--shiki-dark:#79B8FF">  "stop_reason"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"end_turn"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#005CC5;--shiki-dark:#79B8FF">  "session_id"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"a1b2c3d4e5f6"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#005CC5;--shiki-dark:#79B8FF">  "usage"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: {</span></span>
<span class="line"><span style="color:#005CC5;--shiki-dark:#79B8FF">    "input_tokens"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#005CC5;--shiki-dark:#79B8FF">1024</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#005CC5;--shiki-dark:#79B8FF">    "output_tokens"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#005CC5;--shiki-dark:#79B8FF">512</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#005CC5;--shiki-dark:#79B8FF">    "total_tokens"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#005CC5;--shiki-dark:#79B8FF">1536</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#005CC5;--shiki-dark:#79B8FF">    "cache_read_tokens"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#005CC5;--shiki-dark:#79B8FF">0</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#005CC5;--shiki-dark:#79B8FF">    "cache_creation_tokens"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#005CC5;--shiki-dark:#79B8FF">0</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">  },</span></span>
<span class="line"><span style="color:#005CC5;--shiki-dark:#79B8FF">  "messages"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: [</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    {</span></span>
<span class="line"><span style="color:#005CC5;--shiki-dark:#79B8FF">      "role"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"assistant"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#005CC5;--shiki-dark:#79B8FF">      "parts"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: [</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">        {</span><span style="color:#005CC5;--shiki-dark:#79B8FF">"type"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"text"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#005CC5;--shiki-dark:#79B8FF">"data"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"..."</span><span style="color:#24292E;--shiki-dark:#E1E4E8">},</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">        {</span><span style="color:#005CC5;--shiki-dark:#79B8FF">"type"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"tool_call"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#005CC5;--shiki-dark:#79B8FF">"data"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: {</span><span style="color:#005CC5;--shiki-dark:#79B8FF">"name"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"..."</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#005CC5;--shiki-dark:#79B8FF">"args"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"..."</span><span style="color:#24292E;--shiki-dark:#E1E4E8">}},</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">        {</span><span style="color:#005CC5;--shiki-dark:#79B8FF">"type"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"tool_result"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#005CC5;--shiki-dark:#79B8FF">"data"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: {</span><span style="color:#005CC5;--shiki-dark:#79B8FF">"name"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"..."</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#005CC5;--shiki-dark:#79B8FF">"result"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"..."</span><span style="color:#24292E;--shiki-dark:#E1E4E8">}}</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">      ]</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    }</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">  ]</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">}</span></span></code></pre>
<h2 id="fields"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#fields"><span class="icon icon-link"></span></a>Fields</h2>
<h3 id="top-level"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#top-level"><span class="icon icon-link"></span></a>Top-level</h3>
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
<td><code>response</code></td>
<td>string</td>
<td>The final assistant response text</td>
</tr>
<tr>
<td><code>model</code></td>
<td>string</td>
<td>The model that was used</td>
</tr>
<tr>
<td><code>stop_reason</code></td>
<td>string</td>
<td>Why the model stopped (e.g., <code>end_turn</code>)</td>
</tr>
<tr>
<td><code>session_id</code></td>
<td>string</td>
<td>Session identifier (omitted in <code>--no-session</code> mode)</td>
</tr>
<tr>
<td><code>usage</code></td>
<td>object</td>
<td>Token usage statistics</td>
</tr>
<tr>
<td><code>messages</code></td>
<td>array</td>
<td>Full conversation history</td>
</tr>
</tbody>
</table>
<h3 id="usage"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#usage"><span class="icon icon-link"></span></a>Usage</h3>
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
<td><code>input_tokens</code></td>
<td>int</td>
<td>Tokens sent to the model</td>
</tr>
<tr>
<td><code>output_tokens</code></td>
<td>int</td>
<td>Tokens generated by the model</td>
</tr>
<tr>
<td><code>total_tokens</code></td>
<td>int</td>
<td>Sum of input and output tokens</td>
</tr>
<tr>
<td><code>cache_read_tokens</code></td>
<td>int</td>
<td>Tokens read from prompt cache</td>
</tr>
<tr>
<td><code>cache_creation_tokens</code></td>
<td>int</td>
<td>Tokens written to prompt cache</td>
</tr>
</tbody>
</table>
<h3 id="message-parts"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#message-parts"><span class="icon icon-link"></span></a>Message parts</h3>
<p>Each message contains a <code>parts</code> array with typed entries:</p>
<table>
<thead>
<tr>
<th>Type</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td><code>text</code></td>
<td>Assistant text content</td>
</tr>
<tr>
<td><code>tool_call</code></td>
<td>Tool invocation with name and args</td>
</tr>
<tr>
<td><code>tool_result</code></td>
<td>Tool execution result</td>
</tr>
<tr>
<td><code>reasoning</code></td>
<td>Extended thinking content</td>
</tr>
<tr>
<td><code>finish</code></td>
<td>End-of-turn marker</td>
</tr>
</tbody>
</table>
<h2 id="parsing-in-scripts"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#parsing-in-scripts"><span class="icon icon-link"></span></a>Parsing in scripts</h2>
<h3 id="bash--jq"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#bash--jq"><span class="icon icon-link"></span></a>bash + jq</h3>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">result</span><span style="color:#D73A49;--shiki-dark:#F97583">=</span><span style="color:#24292E;--shiki-dark:#E1E4E8">$(</span><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> "Count files"</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> --json</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> --quiet</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> --no-session</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">response</span><span style="color:#D73A49;--shiki-dark:#F97583">=</span><span style="color:#24292E;--shiki-dark:#E1E4E8">$(</span><span style="color:#005CC5;--shiki-dark:#79B8FF">echo</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> "</span><span style="color:#24292E;--shiki-dark:#E1E4E8">$result</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"</span><span style="color:#D73A49;--shiki-dark:#F97583"> |</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> jq</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> -r</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> '.response'</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">tokens</span><span style="color:#D73A49;--shiki-dark:#F97583">=</span><span style="color:#24292E;--shiki-dark:#E1E4E8">$(</span><span style="color:#005CC5;--shiki-dark:#79B8FF">echo</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> "</span><span style="color:#24292E;--shiki-dark:#E1E4E8">$result</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"</span><span style="color:#D73A49;--shiki-dark:#F97583"> |</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> jq</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> '.usage.total_tokens'</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span></code></pre>
<h3 id="go-sdk"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#go-sdk"><span class="icon icon-link"></span></a>Go SDK</h3>
<p>For Go programs, use the SDK's <code>PromptResult</code> method instead of parsing JSON:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">result, err </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">PromptResult</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(ctx, </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"Count files"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">fmt.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Println</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(result.Response)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">fmt.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Println</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(result.Usage.TotalTokens)</span></span></code></pre>`,headings:[{depth:2,text:"Response format",id:"response-format"},{depth:2,text:"Fields",id:"fields"},{depth:3,text:"Top-level",id:"top-level"},{depth:3,text:"Usage",id:"usage"},{depth:3,text:"Message parts",id:"message-parts"},{depth:2,text:"Parsing in scripts",id:"parsing-in-scripts"},{depth:3,text:"bash + jq",id:"bash--jq"},{depth:3,text:"Go SDK",id:"go-sdk"}],raw:`
# JSON Output

Use the \`--json\` flag to get structured output for scripting and automation:

\`\`\`bash
kit "Explain main.go" --json --quiet --no-session
\`\`\`

## Response format

\`\`\`json
{
  "response": "Final assistant response text",
  "model": "anthropic/claude-haiku-latest",
  "stop_reason": "end_turn",
  "session_id": "a1b2c3d4e5f6",
  "usage": {
    "input_tokens": 1024,
    "output_tokens": 512,
    "total_tokens": 1536,
    "cache_read_tokens": 0,
    "cache_creation_tokens": 0
  },
  "messages": [
    {
      "role": "assistant",
      "parts": [
        {"type": "text", "data": "..."},
        {"type": "tool_call", "data": {"name": "...", "args": "..."}},
        {"type": "tool_result", "data": {"name": "...", "result": "..."}}
      ]
    }
  ]
}
\`\`\`

## Fields

### Top-level

| Field | Type | Description |
|-------|------|-------------|
| \`response\` | string | The final assistant response text |
| \`model\` | string | The model that was used |
| \`stop_reason\` | string | Why the model stopped (e.g., \`end_turn\`) |
| \`session_id\` | string | Session identifier (omitted in \`--no-session\` mode) |
| \`usage\` | object | Token usage statistics |
| \`messages\` | array | Full conversation history |

### Usage

| Field | Type | Description |
|-------|------|-------------|
| \`input_tokens\` | int | Tokens sent to the model |
| \`output_tokens\` | int | Tokens generated by the model |
| \`total_tokens\` | int | Sum of input and output tokens |
| \`cache_read_tokens\` | int | Tokens read from prompt cache |
| \`cache_creation_tokens\` | int | Tokens written to prompt cache |

### Message parts

Each message contains a \`parts\` array with typed entries:

| Type | Description |
|------|-------------|
| \`text\` | Assistant text content |
| \`tool_call\` | Tool invocation with name and args |
| \`tool_result\` | Tool execution result |
| \`reasoning\` | Extended thinking content |
| \`finish\` | End-of-turn marker |

## Parsing in scripts

### bash + jq

\`\`\`bash
result=$(kit "Count files" --json --quiet --no-session)
response=$(echo "$result" | jq -r '.response')
tokens=$(echo "$result" | jq '.usage.total_tokens')
\`\`\`

### Go SDK

For Go programs, use the SDK's \`PromptResult\` method instead of parsing JSON:

\`\`\`go
result, err := host.PromptResult(ctx, "Count files")
fmt.Println(result.Response)
fmt.Println(result.Usage.TotalTokens)
\`\`\`
`};export{s as default};

const s={frontmatter:{title:"Go SDK",description:"Embed Kit in your Go applications.",hidden:!1,toc:!0,draft:!1},html:`<h1 id="go-sdk"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#go-sdk"><span class="icon icon-link"></span></a>Go SDK</h1>
<p>The <code>pkg/kit</code> package lets you embed Kit as a library in your Go applications.</p>
<h2 id="installation"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#installation"><span class="icon icon-link"></span></a>Installation</h2>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">go</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> get</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> github.com/mark3labs/kit/pkg/kit</span></span></code></pre>
<h2 id="basic-usage"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#basic-usage"><span class="icon icon-link"></span></a>Basic usage</h2>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">package</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> main</span></span>
<span class="line"></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">import</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> (</span></span>
<span class="line"><span style="color:#032F62;--shiki-dark:#9ECBFF">    "</span><span style="color:#6F42C1;--shiki-dark:#B392F0">context</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"</span></span>
<span class="line"><span style="color:#032F62;--shiki-dark:#9ECBFF">    "</span><span style="color:#6F42C1;--shiki-dark:#B392F0">log</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"</span></span>
<span class="line"></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    kit </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"</span><span style="color:#6F42C1;--shiki-dark:#B392F0">github.com/mark3labs/kit/pkg/kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span>
<span class="line"></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">func</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> main</span><span style="color:#24292E;--shiki-dark:#E1E4E8">() {</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    ctx </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> context.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Background</span><span style="color:#24292E;--shiki-dark:#E1E4E8">()</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // Create Kit instance with default configuration</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    host, err </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> kit.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">New</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(ctx, </span><span style="color:#005CC5;--shiki-dark:#79B8FF">nil</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">    if</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> err </span><span style="color:#D73A49;--shiki-dark:#F97583">!=</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> nil</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> {</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">        log.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Fatal</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(err)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    }</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">    defer</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Close</span><span style="color:#24292E;--shiki-dark:#E1E4E8">()</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // Send a prompt</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    response, err </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Prompt</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(ctx, </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"What is 2+2?"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">    if</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> err </span><span style="color:#D73A49;--shiki-dark:#F97583">!=</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> nil</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> {</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">        log.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Fatal</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(err)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    }</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">    println</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(response)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">}</span></span></code></pre>
<h2 id="multi-turn-conversations"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#multi-turn-conversations"><span class="icon icon-link"></span></a>Multi-turn conversations</h2>
<p>Conversations retain context automatically across calls:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Prompt</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(ctx, </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"My name is Alice"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">response, _ </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Prompt</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(ctx, </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"What's my name?"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// response: "Your name is Alice"</span></span></code></pre>
<h2 id="additional-prompt-methods"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#additional-prompt-methods"><span class="icon icon-link"></span></a>Additional prompt methods</h2>
<p>The SDK provides several prompt variants:</p>
<table>
<thead>
<tr>
<th>Method</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td><code>Prompt(ctx, message)</code></td>
<td>Simple prompt, returns response string</td>
</tr>
<tr>
<td><code>PromptWithOptions(ctx, message, opts)</code></td>
<td>With per-call options</td>
</tr>
<tr>
<td><code>PromptResult(ctx, message)</code></td>
<td>Returns full <code>TurnResult</code> with usage stats</td>
</tr>
<tr>
<td><code>PromptResultWithFiles(ctx, message, files)</code></td>
<td>Multimodal with file attachments</td>
</tr>
<tr>
<td><code>Steer(ctx, instruction)</code></td>
<td>System-level steering without user message</td>
</tr>
<tr>
<td><code>FollowUp(ctx, text)</code></td>
<td>Continue without new user input</td>
</tr>
</tbody>
</table>
<h2 id="event-system"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#event-system"><span class="icon icon-link"></span></a>Event system</h2>
<p>Subscribe to events for monitoring:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">unsubscribe </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">OnToolCall</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#D73A49;--shiki-dark:#F97583">func</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#E36209;--shiki-dark:#FFAB70">event</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ToolCallEvent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) {</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    fmt.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Println</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"Tool called:"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, event.Name)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">defer</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> unsubscribe</span><span style="color:#24292E;--shiki-dark:#E1E4E8">()</span></span>
<span class="line"></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">OnToolResult</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#D73A49;--shiki-dark:#F97583">func</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#E36209;--shiki-dark:#FFAB70">event</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ToolResultEvent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) {</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    fmt.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Println</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"Tool result:"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, event.Name)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span>
<span class="line"></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">OnStreaming</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#D73A49;--shiki-dark:#F97583">func</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#E36209;--shiki-dark:#FFAB70">event</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">MessageUpdateEvent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) {</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    fmt.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Print</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(event.Chunk)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span></code></pre>
<h2 id="model-management"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#model-management"><span class="icon icon-link"></span></a>Model management</h2>
<p>Switch models at runtime:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">SetModel</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(ctx, </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"openai/gpt-4o"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">info </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">GetModelInfo</span><span style="color:#24292E;--shiki-dark:#E1E4E8">()</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">models </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">GetAvailableModels</span><span style="color:#24292E;--shiki-dark:#E1E4E8">()</span></span></code></pre>
<h2 id="context-and-compaction"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#context-and-compaction"><span class="icon icon-link"></span></a>Context and compaction</h2>
<p>Monitor and manage context usage:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">tokens </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">EstimateContextTokens</span><span style="color:#24292E;--shiki-dark:#E1E4E8">()</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">stats </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">GetContextStats</span><span style="color:#24292E;--shiki-dark:#E1E4E8">()</span></span>
<span class="line"></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">if</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ShouldCompact</span><span style="color:#24292E;--shiki-dark:#E1E4E8">() {</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    result, err </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Compact</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(ctx, </span><span style="color:#005CC5;--shiki-dark:#79B8FF">nil</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#032F62;--shiki-dark:#9ECBFF">""</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">}</span></span></code></pre>
<p>See <a href="/sdk/options">Options</a>, <a href="/sdk/callbacks">Callbacks</a>, and <a href="/sdk/sessions">Sessions</a> for more details.</p>`,headings:[{depth:2,text:"Installation",id:"installation"},{depth:2,text:"Basic usage",id:"basic-usage"},{depth:2,text:"Multi-turn conversations",id:"multi-turn-conversations"},{depth:2,text:"Additional prompt methods",id:"additional-prompt-methods"},{depth:2,text:"Event system",id:"event-system"},{depth:2,text:"Model management",id:"model-management"},{depth:2,text:"Context and compaction",id:"context-and-compaction"}],raw:`
# Go SDK

The \`pkg/kit\` package lets you embed Kit as a library in your Go applications.

## Installation

\`\`\`bash
go get github.com/mark3labs/kit/pkg/kit
\`\`\`

## Basic usage

\`\`\`go
package main

import (
    "context"
    "log"

    kit "github.com/mark3labs/kit/pkg/kit"
)

func main() {
    ctx := context.Background()

    // Create Kit instance with default configuration
    host, err := kit.New(ctx, nil)
    if err != nil {
        log.Fatal(err)
    }
    defer host.Close()

    // Send a prompt
    response, err := host.Prompt(ctx, "What is 2+2?")
    if err != nil {
        log.Fatal(err)
    }

    println(response)
}
\`\`\`

## Multi-turn conversations

Conversations retain context automatically across calls:

\`\`\`go
host.Prompt(ctx, "My name is Alice")
response, _ := host.Prompt(ctx, "What's my name?")
// response: "Your name is Alice"
\`\`\`

## Additional prompt methods

The SDK provides several prompt variants:

| Method | Description |
|--------|-------------|
| \`Prompt(ctx, message)\` | Simple prompt, returns response string |
| \`PromptWithOptions(ctx, message, opts)\` | With per-call options |
| \`PromptResult(ctx, message)\` | Returns full \`TurnResult\` with usage stats |
| \`PromptResultWithFiles(ctx, message, files)\` | Multimodal with file attachments |
| \`Steer(ctx, instruction)\` | System-level steering without user message |
| \`FollowUp(ctx, text)\` | Continue without new user input |

## Event system

Subscribe to events for monitoring:

\`\`\`go
unsubscribe := host.OnToolCall(func(event kit.ToolCallEvent) {
    fmt.Println("Tool called:", event.Name)
})
defer unsubscribe()

host.OnToolResult(func(event kit.ToolResultEvent) {
    fmt.Println("Tool result:", event.Name)
})

host.OnStreaming(func(event kit.MessageUpdateEvent) {
    fmt.Print(event.Chunk)
})
\`\`\`

## Model management

Switch models at runtime:

\`\`\`go
host.SetModel(ctx, "openai/gpt-4o")
info := host.GetModelInfo()
models := host.GetAvailableModels()
\`\`\`

## Context and compaction

Monitor and manage context usage:

\`\`\`go
tokens := host.EstimateContextTokens()
stats := host.GetContextStats()

if host.ShouldCompact() {
    result, err := host.Compact(ctx, nil, "")
}
\`\`\`

See [Options](/sdk/options), [Callbacks](/sdk/callbacks), and [Sessions](/sdk/sessions) for more details.
`};export{s as default};

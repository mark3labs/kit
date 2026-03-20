const s={frontmatter:{title:"Testing Extensions",description:"Write unit tests for your Kit extensions using the test package.",hidden:!1,toc:!0,draft:!1},html:`<h1 id="testing-extensions"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#testing-extensions"><span class="icon icon-link"></span></a>Testing Extensions</h1>
<p>Kit provides a testing package (<code>github.com/mark3labs/kit/pkg/extensions/test</code>) that enables you to write unit tests for your extensions. Tests run outside the Yaegi interpreter but load your extension code into an isolated interpreter instance, allowing you to verify behavior without running the full Kit TUI.</p>
<h2 id="overview"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#overview"><span class="icon icon-link"></span></a>Overview</h2>
<p>Extension tests allow you to:</p>
<ul>
<li>Test event handlers without running the interactive TUI</li>
<li>Verify tool/command registration</li>
<li>Assert that context methods (Print, SetWidget, etc.) are called correctly</li>
<li>Test blocking and non-blocking event handling</li>
<li>Simulate user input and tool calls</li>
<li>Verify widget, header, footer, and status bar updates</li>
</ul>
<h2 id="installation"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#installation"><span class="icon icon-link"></span></a>Installation</h2>
<p>The test package is part of the Kit codebase. Import it in your extension tests:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">import</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> (</span></span>
<span class="line"><span style="color:#032F62;--shiki-dark:#9ECBFF">    "</span><span style="color:#6F42C1;--shiki-dark:#B392F0">testing</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"</span></span>
<span class="line"><span style="color:#032F62;--shiki-dark:#9ECBFF">    "</span><span style="color:#6F42C1;--shiki-dark:#B392F0">github.com/mark3labs/kit/pkg/extensions/test</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"</span></span>
<span class="line"><span style="color:#032F62;--shiki-dark:#9ECBFF">    "</span><span style="color:#6F42C1;--shiki-dark:#B392F0">github.com/mark3labs/kit/internal/extensions</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span></code></pre>
<h2 id="basic-usage"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#basic-usage"><span class="icon icon-link"></span></a>Basic Usage</h2>
<h3 id="testing-an-extension-file"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#testing-an-extension-file"><span class="icon icon-link"></span></a>Testing an Extension File</h3>
<p>Create a test file alongside your extension (e.g., <code>my-ext_test.go</code>):</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">package</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> main</span></span>
<span class="line"></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">import</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> (</span></span>
<span class="line"><span style="color:#032F62;--shiki-dark:#9ECBFF">    "</span><span style="color:#6F42C1;--shiki-dark:#B392F0">testing</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"</span></span>
<span class="line"><span style="color:#032F62;--shiki-dark:#9ECBFF">    "</span><span style="color:#6F42C1;--shiki-dark:#B392F0">github.com/mark3labs/kit/pkg/extensions/test</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"</span></span>
<span class="line"><span style="color:#032F62;--shiki-dark:#9ECBFF">    "</span><span style="color:#6F42C1;--shiki-dark:#B392F0">github.com/mark3labs/kit/internal/extensions</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span>
<span class="line"></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">func</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> TestMyExtension</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#E36209;--shiki-dark:#FFAB70">t</span><span style="color:#D73A49;--shiki-dark:#F97583"> *</span><span style="color:#6F42C1;--shiki-dark:#B392F0">testing</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">T</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) {</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // Create a test harness</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    harness </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> test.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">New</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(t)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    </span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // Load your extension</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    harness.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">LoadFile</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"my-ext.go"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    </span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // Emit events and check results</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    result, err </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> harness.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Emit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#6F42C1;--shiki-dark:#B392F0">extensions</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ToolCallEvent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">        ToolName: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"my_tool"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">        Input:    </span><span style="color:#032F62;--shiki-dark:#9ECBFF">\`{"key": "value"}\`</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    })</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">    if</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> err </span><span style="color:#D73A49;--shiki-dark:#F97583">!=</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> nil</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> {</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">        t.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Fatalf</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"unexpected error: </span><span style="color:#005CC5;--shiki-dark:#79B8FF">%v</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, err)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    }</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    </span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // Use assertion helpers</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    test.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">AssertNotBlocked</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(t, result)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    test.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">AssertPrinted</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(t, harness, </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"expected output"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">}</span></span></code></pre>
<h3 id="testing-inline-extension-code"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#testing-inline-extension-code"><span class="icon icon-link"></span></a>Testing Inline Extension Code</h3>
<p>For quick tests or edge cases, you can load extension source directly:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">func</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> TestToolBlocking</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#E36209;--shiki-dark:#FFAB70">t</span><span style="color:#D73A49;--shiki-dark:#F97583"> *</span><span style="color:#6F42C1;--shiki-dark:#B392F0">testing</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">T</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) {</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    src </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> \`package main</span></span>
<span class="line"></span>
<span class="line"><span style="color:#032F62;--shiki-dark:#9ECBFF">import "kit/ext"</span></span>
<span class="line"></span>
<span class="line"><span style="color:#032F62;--shiki-dark:#9ECBFF">func Init(api ext.API) {</span></span>
<span class="line"><span style="color:#032F62;--shiki-dark:#9ECBFF">    api.OnToolCall(func(tc ext.ToolCallEvent, ctx ext.Context) *ext.ToolCallResult {</span></span>
<span class="line"><span style="color:#032F62;--shiki-dark:#9ECBFF">        if tc.ToolName == "dangerous" {</span></span>
<span class="line"><span style="color:#032F62;--shiki-dark:#9ECBFF">            return &amp;ext.ToolCallResult{Block: true, Reason: "not allowed"}</span></span>
<span class="line"><span style="color:#032F62;--shiki-dark:#9ECBFF">        }</span></span>
<span class="line"><span style="color:#032F62;--shiki-dark:#9ECBFF">        return nil</span></span>
<span class="line"><span style="color:#032F62;--shiki-dark:#9ECBFF">    })</span></span>
<span class="line"><span style="color:#032F62;--shiki-dark:#9ECBFF">}</span></span>
<span class="line"><span style="color:#032F62;--shiki-dark:#9ECBFF">\`</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    harness </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> test.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">New</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(t)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    harness.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">LoadString</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(src, </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"test-ext.go"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    </span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // Test the tool is blocked</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    result, _ </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> harness.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Emit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#6F42C1;--shiki-dark:#B392F0">extensions</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ToolCallEvent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">        ToolName: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"dangerous"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">        Input:    </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"{}"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    })</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    </span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    test.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">AssertBlocked</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(t, result, </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"not allowed"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">}</span></span></code></pre>
<h2 id="common-testing-patterns"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#common-testing-patterns"><span class="icon icon-link"></span></a>Common Testing Patterns</h2>
<h3 id="testing-handler-registration"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#testing-handler-registration"><span class="icon icon-link"></span></a>Testing Handler Registration</h3>
<p>Verify your extension registers the expected handlers:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">func</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> TestHandlers</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#E36209;--shiki-dark:#FFAB70">t</span><span style="color:#D73A49;--shiki-dark:#F97583"> *</span><span style="color:#6F42C1;--shiki-dark:#B392F0">testing</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">T</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) {</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    harness </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> test.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">New</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(t)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    harness.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">LoadFile</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"my-ext.go"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    </span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    test.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">AssertHasHandlers</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(t, harness, extensions.ToolCall)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    test.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">AssertHasHandlers</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(t, harness, extensions.SessionStart)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    test.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">AssertNoHandlers</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(t, harness, extensions.AgentEnd) </span><span style="color:#6A737D;--shiki-dark:#6A737D">// Verify no unexpected handlers</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">}</span></span></code></pre>
<h3 id="testing-tool-registration"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#testing-tool-registration"><span class="icon icon-link"></span></a>Testing Tool Registration</h3>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">func</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> TestTools</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#E36209;--shiki-dark:#FFAB70">t</span><span style="color:#D73A49;--shiki-dark:#F97583"> *</span><span style="color:#6F42C1;--shiki-dark:#B392F0">testing</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">T</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) {</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    harness </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> test.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">New</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(t)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    harness.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">LoadFile</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"my-ext.go"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    </span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // Verify a specific tool is registered</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    test.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">AssertToolRegistered</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(t, harness, </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"my_tool"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    </span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // Or inspect all tools</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    tools </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> harness.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">RegisteredTools</span><span style="color:#24292E;--shiki-dark:#E1E4E8">()</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">    for</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> _, tool </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#D73A49;--shiki-dark:#F97583"> range</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> tools {</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">        t.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Logf</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"Tool: </span><span style="color:#005CC5;--shiki-dark:#79B8FF">%s</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> - </span><span style="color:#005CC5;--shiki-dark:#79B8FF">%s</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, tool.Name, tool.Description)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    }</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">}</span></span></code></pre>
<h3 id="testing-commands"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#testing-commands"><span class="icon icon-link"></span></a>Testing Commands</h3>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">func</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> TestCommands</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#E36209;--shiki-dark:#FFAB70">t</span><span style="color:#D73A49;--shiki-dark:#F97583"> *</span><span style="color:#6F42C1;--shiki-dark:#B392F0">testing</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">T</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) {</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    harness </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> test.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">New</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(t)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    harness.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">LoadFile</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"my-ext.go"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    </span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    test.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">AssertCommandRegistered</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(t, harness, </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"mycommand"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">}</span></span></code></pre>
<h3 id="testing-widgets"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#testing-widgets"><span class="icon icon-link"></span></a>Testing Widgets</h3>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">func</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> TestWidgets</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#E36209;--shiki-dark:#FFAB70">t</span><span style="color:#D73A49;--shiki-dark:#F97583"> *</span><span style="color:#6F42C1;--shiki-dark:#B392F0">testing</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">T</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) {</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    harness </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> test.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">New</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(t)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    harness.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">LoadFile</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"my-ext.go"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    </span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // Trigger event that creates the widget</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    _, _ </span><span style="color:#D73A49;--shiki-dark:#F97583">=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> harness.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Emit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#6F42C1;--shiki-dark:#B392F0">extensions</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">SessionStartEvent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{SessionID: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"test"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    </span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // Verify widget was set</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    test.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">AssertWidgetSet</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(t, harness, </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"my-widget"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    test.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">AssertWidgetText</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(t, harness, </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"my-widget"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"Expected Text"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    test.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">AssertWidgetTextContains</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(t, harness, </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"my-widget"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"partial"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    </span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // Check widget properties directly</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    widget, ok </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> harness.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Context</span><span style="color:#24292E;--shiki-dark:#E1E4E8">().</span><span style="color:#6F42C1;--shiki-dark:#B392F0">GetWidget</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"my-widget"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">    if</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> ok {</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">        t.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Logf</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"Border color: </span><span style="color:#005CC5;--shiki-dark:#79B8FF">%s</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, widget.Style.BorderColor)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    }</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">}</span></span></code></pre>
<h3 id="testing-input-handling"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#testing-input-handling"><span class="icon icon-link"></span></a>Testing Input Handling</h3>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">func</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> TestInput</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#E36209;--shiki-dark:#FFAB70">t</span><span style="color:#D73A49;--shiki-dark:#F97583"> *</span><span style="color:#6F42C1;--shiki-dark:#B392F0">testing</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">T</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) {</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    harness </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> test.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">New</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(t)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    harness.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">LoadFile</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"my-ext.go"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    </span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    result, _ </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> harness.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Emit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#6F42C1;--shiki-dark:#B392F0">extensions</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">InputEvent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">        Text:   </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"!mycommand"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">        Source: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"cli"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    })</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    </span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    test.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">AssertInputHandled</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(t, result, </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"handled"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">}</span></span></code></pre>
<h3 id="testing-headers-and-footers"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#testing-headers-and-footers"><span class="icon icon-link"></span></a>Testing Headers and Footers</h3>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">func</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> TestHeaderFooter</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#E36209;--shiki-dark:#FFAB70">t</span><span style="color:#D73A49;--shiki-dark:#F97583"> *</span><span style="color:#6F42C1;--shiki-dark:#B392F0">testing</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">T</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) {</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    harness </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> test.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">New</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(t)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    harness.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">LoadFile</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"my-ext.go"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    </span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    _, _ </span><span style="color:#D73A49;--shiki-dark:#F97583">=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> harness.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Emit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#6F42C1;--shiki-dark:#B392F0">extensions</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">SessionStartEvent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{SessionID: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"test"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    </span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    test.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">AssertHeaderSet</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(t, harness)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    test.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">AssertFooterSet</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(t, harness)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    </span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // Inspect content</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    header </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> harness.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Context</span><span style="color:#24292E;--shiki-dark:#E1E4E8">().</span><span style="color:#6F42C1;--shiki-dark:#B392F0">GetHeader</span><span style="color:#24292E;--shiki-dark:#E1E4E8">()</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">    if</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> header </span><span style="color:#D73A49;--shiki-dark:#F97583">!=</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> nil</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> {</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">        t.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Logf</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"Header text: </span><span style="color:#005CC5;--shiki-dark:#79B8FF">%s</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, header.Content.Text)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    }</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">}</span></span></code></pre>
<h3 id="testing-status-bar"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#testing-status-bar"><span class="icon icon-link"></span></a>Testing Status Bar</h3>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">func</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> TestStatus</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#E36209;--shiki-dark:#FFAB70">t</span><span style="color:#D73A49;--shiki-dark:#F97583"> *</span><span style="color:#6F42C1;--shiki-dark:#B392F0">testing</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">T</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) {</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    harness </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> test.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">New</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(t)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    harness.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">LoadFile</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"my-ext.go"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    </span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    _, _ </span><span style="color:#D73A49;--shiki-dark:#F97583">=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> harness.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Emit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#6F42C1;--shiki-dark:#B392F0">extensions</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">AgentEndEvent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{})</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    </span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    test.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">AssertStatusSet</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(t, harness, </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"myext:status"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    test.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">AssertStatusText</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(t, harness, </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"myext:status"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"Ready"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">}</span></span></code></pre>
<h3 id="testing-print-output"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#testing-print-output"><span class="icon icon-link"></span></a>Testing Print Output</h3>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">func</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> TestOutput</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#E36209;--shiki-dark:#FFAB70">t</span><span style="color:#D73A49;--shiki-dark:#F97583"> *</span><span style="color:#6F42C1;--shiki-dark:#B392F0">testing</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">T</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) {</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    harness </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> test.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">New</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(t)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    harness.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">LoadFile</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"my-ext.go"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    </span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    _, _ </span><span style="color:#D73A49;--shiki-dark:#F97583">=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> harness.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Emit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#6F42C1;--shiki-dark:#B392F0">extensions</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ToolCallEvent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{ToolName: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"test"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    </span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // Exact match</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    test.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">AssertPrinted</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(t, harness, </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"exact output"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    </span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // Partial match</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    test.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">AssertPrintedContains</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(t, harness, </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"partial"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    </span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // Styled output</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    test.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">AssertPrintInfo</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(t, harness, </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"info message"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    test.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">AssertPrintError</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(t, harness, </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"error message"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">}</span></span></code></pre>
<h3 id="testing-with-prompts"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#testing-with-prompts"><span class="icon icon-link"></span></a>Testing with Prompts</h3>
<p>Configure mock prompt results for testing interactive behavior:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">func</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> TestWithPrompts</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#E36209;--shiki-dark:#FFAB70">t</span><span style="color:#D73A49;--shiki-dark:#F97583"> *</span><span style="color:#6F42C1;--shiki-dark:#B392F0">testing</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">T</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) {</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    harness </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> test.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">New</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(t)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    harness.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">LoadFile</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"my-ext.go"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    </span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // Configure what prompts should return</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    harness.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Context</span><span style="color:#24292E;--shiki-dark:#E1E4E8">().</span><span style="color:#6F42C1;--shiki-dark:#B392F0">SetPromptSelectResult</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#6F42C1;--shiki-dark:#B392F0">extensions</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">PromptSelectResult</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">        Value:     </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"option1"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">        Index:     </span><span style="color:#005CC5;--shiki-dark:#79B8FF">0</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">        Cancelled: </span><span style="color:#005CC5;--shiki-dark:#79B8FF">false</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    })</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    </span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    harness.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Context</span><span style="color:#24292E;--shiki-dark:#E1E4E8">().</span><span style="color:#6F42C1;--shiki-dark:#B392F0">SetPromptConfirmResult</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#6F42C1;--shiki-dark:#B392F0">extensions</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">PromptConfirmResult</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">        Value:     </span><span style="color:#005CC5;--shiki-dark:#79B8FF">true</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">        Cancelled: </span><span style="color:#005CC5;--shiki-dark:#79B8FF">false</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    })</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    </span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // Now when your extension calls ctx.PromptSelect(), it gets this result</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    _, _ </span><span style="color:#D73A49;--shiki-dark:#F97583">=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> harness.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Emit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#6F42C1;--shiki-dark:#B392F0">extensions</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">SessionStartEvent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{SessionID: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"test"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">}</span></span></code></pre>
<h3 id="testing-complete-session-flow"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#testing-complete-session-flow"><span class="icon icon-link"></span></a>Testing Complete Session Flow</h3>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">func</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> TestFullSession</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#E36209;--shiki-dark:#FFAB70">t</span><span style="color:#D73A49;--shiki-dark:#F97583"> *</span><span style="color:#6F42C1;--shiki-dark:#B392F0">testing</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">T</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) {</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    harness </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> test.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">New</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(t)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    harness.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">LoadFile</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"my-ext.go"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    </span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // Simulate a complete session</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    _, _ </span><span style="color:#D73A49;--shiki-dark:#F97583">=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> harness.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Emit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#6F42C1;--shiki-dark:#B392F0">extensions</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">SessionStartEvent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{SessionID: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"test"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    _, _ </span><span style="color:#D73A49;--shiki-dark:#F97583">=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> harness.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Emit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#6F42C1;--shiki-dark:#B392F0">extensions</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">BeforeAgentStartEvent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{})</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    _, _ </span><span style="color:#D73A49;--shiki-dark:#F97583">=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> harness.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Emit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#6F42C1;--shiki-dark:#B392F0">extensions</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">AgentStartEvent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{})</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    </span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // Multiple tool calls</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    tools </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> []</span><span style="color:#D73A49;--shiki-dark:#F97583">string</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"Read"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"Grep"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"Bash"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">}</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">    for</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> _, tool </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#D73A49;--shiki-dark:#F97583"> range</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> tools {</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">        _, _ </span><span style="color:#D73A49;--shiki-dark:#F97583">=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> harness.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Emit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#6F42C1;--shiki-dark:#B392F0">extensions</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ToolCallEvent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{ToolName: tool})</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">        _, _ </span><span style="color:#D73A49;--shiki-dark:#F97583">=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> harness.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Emit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#6F42C1;--shiki-dark:#B392F0">extensions</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ToolResultEvent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{ToolName: tool})</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    }</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    </span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    _, _ </span><span style="color:#D73A49;--shiki-dark:#F97583">=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> harness.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Emit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#6F42C1;--shiki-dark:#B392F0">extensions</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">AgentEndEvent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{})</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    _, _ </span><span style="color:#D73A49;--shiki-dark:#F97583">=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> harness.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Emit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#6F42C1;--shiki-dark:#B392F0">extensions</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">SessionShutdownEvent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{})</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    </span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // Verify final state</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    test.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">AssertWidgetTextContains</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(t, harness, </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"status"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"Complete"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">}</span></span></code></pre>
<h2 id="available-assertions"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#available-assertions"><span class="icon icon-link"></span></a>Available Assertions</h2>
<p>The test package provides these assertion helpers:</p>
<h3 id="event-results"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#event-results"><span class="icon icon-link"></span></a>Event Results</h3>
<table>
<thead>
<tr>
<th>Function</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td><code>AssertNotBlocked(t, result)</code></td>
<td>Verify tool was not blocked</td>
</tr>
<tr>
<td><code>AssertBlocked(t, result, reason)</code></td>
<td>Verify tool was blocked with reason</td>
</tr>
<tr>
<td><code>AssertInputHandled(t, result, action)</code></td>
<td>Verify input was handled</td>
</tr>
<tr>
<td><code>AssertInputTransformed(t, result, text)</code></td>
<td>Verify input was transformed</td>
</tr>
</tbody>
</table>
<h3 id="context-interactions"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#context-interactions"><span class="icon icon-link"></span></a>Context Interactions</h3>
<table>
<thead>
<tr>
<th>Function</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td><code>AssertPrinted(t, harness, text)</code></td>
<td>Verify exact print output</td>
</tr>
<tr>
<td><code>AssertPrintedContains(t, harness, substring)</code></td>
<td>Verify partial print output</td>
</tr>
<tr>
<td><code>AssertPrintInfo(t, harness, text)</code></td>
<td>Verify PrintInfo was called</td>
</tr>
<tr>
<td><code>AssertPrintError(t, harness, text)</code></td>
<td>Verify PrintError was called</td>
</tr>
<tr>
<td><code>AssertWidgetSet(t, harness, id)</code></td>
<td>Verify widget was set</td>
</tr>
<tr>
<td><code>AssertWidgetNotSet(t, harness, id)</code></td>
<td>Verify widget was not set</td>
</tr>
<tr>
<td><code>AssertWidgetText(t, harness, id, text)</code></td>
<td>Verify widget content</td>
</tr>
<tr>
<td><code>AssertWidgetTextContains(t, harness, id, substring)</code></td>
<td>Verify widget contains text</td>
</tr>
<tr>
<td><code>AssertHeaderSet(t, harness)</code></td>
<td>Verify header was set</td>
</tr>
<tr>
<td><code>AssertFooterSet(t, harness)</code></td>
<td>Verify footer was set</td>
</tr>
<tr>
<td><code>AssertStatusSet(t, harness, key)</code></td>
<td>Verify status was set</td>
</tr>
<tr>
<td><code>AssertStatusText(t, harness, key, text)</code></td>
<td>Verify status text</td>
</tr>
</tbody>
</table>
<h3 id="registration"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#registration"><span class="icon icon-link"></span></a>Registration</h3>
<table>
<thead>
<tr>
<th>Function</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td><code>AssertToolRegistered(t, harness, name)</code></td>
<td>Verify tool registration</td>
</tr>
<tr>
<td><code>AssertCommandRegistered(t, harness, name)</code></td>
<td>Verify command registration</td>
</tr>
<tr>
<td><code>AssertHasHandlers(t, harness, eventType)</code></td>
<td>Verify handlers exist</td>
</tr>
<tr>
<td><code>AssertNoHandlers(t, harness, eventType)</code></td>
<td>Verify no handlers</td>
</tr>
</tbody>
</table>
<h3 id="messaging"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#messaging"><span class="icon icon-link"></span></a>Messaging</h3>
<table>
<thead>
<tr>
<th>Function</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td><code>AssertMessageSent(t, harness, text)</code></td>
<td>Verify SendMessage was called</td>
</tr>
<tr>
<td><code>AssertCancelAndSend(t, harness, text)</code></td>
<td>Verify CancelAndSend was called</td>
</tr>
</tbody>
</table>
<h2 id="helper-functions"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#helper-functions"><span class="icon icon-link"></span></a>Helper Functions</h2>
<p>For custom assertions, extract result details:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">result, _ </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> harness.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Emit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#6F42C1;--shiki-dark:#B392F0">extensions</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ToolCallEvent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span><span style="color:#D73A49;--shiki-dark:#F97583">...</span><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">tcr </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> test.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">GetToolCallResult</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(result)</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">if</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> tcr </span><span style="color:#D73A49;--shiki-dark:#F97583">!=</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> nil</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> {</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    t.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Logf</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"Block: </span><span style="color:#005CC5;--shiki-dark:#79B8FF">%v</span><span style="color:#032F62;--shiki-dark:#9ECBFF">, Reason: </span><span style="color:#005CC5;--shiki-dark:#79B8FF">%s</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, tcr.Block, tcr.Reason)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">}</span></span>
<span class="line"></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">ir </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> test.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">GetInputResult</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(result)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">trr </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> test.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">GetToolResultResult</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(result)</span></span></code></pre>
<h2 id="advanced-usage"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#advanced-usage"><span class="icon icon-link"></span></a>Advanced Usage</h2>
<h3 id="accessing-the-mock-context"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#accessing-the-mock-context"><span class="icon icon-link"></span></a>Accessing the Mock Context</h3>
<p>For custom verification:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">ctx </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> harness.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Context</span><span style="color:#24292E;--shiki-dark:#E1E4E8">()</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// Get all recorded prints</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">prints </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">GetPrints</span><span style="color:#24292E;--shiki-dark:#E1E4E8">()</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// Check options</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">value </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">GetOption</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"my-option"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// Verify widget properties</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">widget, ok </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">GetWidget</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"my-widget"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">if</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> ok </span><span style="color:#D73A49;--shiki-dark:#F97583">&amp;&amp;</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> widget.Style.BorderColor </span><span style="color:#D73A49;--shiki-dark:#F97583">==</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> "#ff0000"</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> {</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    t.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Log</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"Widget has red border"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">}</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// Check status entries</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">status, ok </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">GetStatus</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"myext:status"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span></code></pre>
<h3 id="testing-multiple-extensions"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#testing-multiple-extensions"><span class="icon icon-link"></span></a>Testing Multiple Extensions</h3>
<p>Each harness is isolated:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">harness1 </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> test.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">New</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(t)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">harness1.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">LoadFile</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"ext1.go"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span>
<span class="line"></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">harness2 </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> test.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">New</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(t)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">harness2.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">LoadFile</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"ext2.go"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// Events to one don't affect the other</span></span></code></pre>
<h3 id="running-tests"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#running-tests"><span class="icon icon-link"></span></a>Running Tests</h3>
<p>Run all tests in your extension directory:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#005CC5;--shiki-dark:#79B8FF">cd</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> examples/extensions</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">go</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> test</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> -v</span></span></code></pre>
<p>Run with race detector:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">go</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> test</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> -race</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> -v</span></span></code></pre>
<p>Run a specific test:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">go</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> test</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> -v</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> -run</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> TestMyExtension</span></span></code></pre>
<h2 id="best-practices"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#best-practices"><span class="icon icon-link"></span></a>Best Practices</h2>
<ol>
<li><strong>Test one behavior per test</strong> — Keep tests focused and readable</li>
<li><strong>Use inline source for edge cases</strong> — <code>LoadString()</code> is great for testing specific scenarios</li>
<li><strong>Use <code>LoadFile()</code> for integration tests</strong> — Tests the actual extension file</li>
<li><strong>Assert on context calls</strong> — Verify your extension interacts with the context correctly</li>
<li><strong>Test both positive and negative cases</strong> — Verify tools are blocked AND allowed appropriately</li>
<li><strong>Test all event handlers</strong> — Make sure all registered handlers work correctly</li>
<li><strong>Use descriptive test names</strong> — <code>TestExtension_BlocksDangerousTools</code> is clearer than <code>Test1</code></li>
</ol>
<h2 id="limitations"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#limitations"><span class="icon icon-link"></span></a>Limitations</h2>
<p>The test harness has these intentional limitations:</p>
<ul>
<li><strong>No TUI rendering</strong> — Widgets are recorded but not rendered visually</li>
<li><strong>Prompts return configured values</strong> — Pre-configure prompt results in tests</li>
<li><strong>Subagents don't spawn real processes</strong> — <code>SpawnSubagent()</code> returns nil/empty results</li>
<li><strong>LLM completions are mocked</strong> — <code>Complete()</code> returns empty responses</li>
<li><strong>Some context methods are no-ops</strong> — <code>Exit()</code>, <code>SetActiveTools()</code>, etc. don't have side effects</li>
</ul>
<p>These limitations focus testing on extension logic rather than the full Kit runtime.</p>
<h2 id="complete-example"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#complete-example"><span class="icon icon-link"></span></a>Complete Example</h2>
<p>See <code>examples/extensions/tool-logger_test.go</code> for a complete example with 14 tests covering:</p>
<ul>
<li>Handler registration</li>
<li>Tool call and result handling</li>
<li>Session lifecycle events</li>
<li>Input commands (<code>!time</code>, <code>!status</code>)</li>
<li>Unknown command handling</li>
<li>Concurrent operations (race condition check)</li>
<li>Real file logging verification</li>
</ul>`,headings:[{depth:2,text:"Overview",id:"overview"},{depth:2,text:"Installation",id:"installation"},{depth:2,text:"Basic Usage",id:"basic-usage"},{depth:3,text:"Testing an Extension File",id:"testing-an-extension-file"},{depth:3,text:"Testing Inline Extension Code",id:"testing-inline-extension-code"},{depth:2,text:"Common Testing Patterns",id:"common-testing-patterns"},{depth:3,text:"Testing Handler Registration",id:"testing-handler-registration"},{depth:3,text:"Testing Tool Registration",id:"testing-tool-registration"},{depth:3,text:"Testing Commands",id:"testing-commands"},{depth:3,text:"Testing Widgets",id:"testing-widgets"},{depth:3,text:"Testing Input Handling",id:"testing-input-handling"},{depth:3,text:"Testing Headers and Footers",id:"testing-headers-and-footers"},{depth:3,text:"Testing Status Bar",id:"testing-status-bar"},{depth:3,text:"Testing Print Output",id:"testing-print-output"},{depth:3,text:"Testing with Prompts",id:"testing-with-prompts"},{depth:3,text:"Testing Complete Session Flow",id:"testing-complete-session-flow"},{depth:2,text:"Available Assertions",id:"available-assertions"},{depth:3,text:"Event Results",id:"event-results"},{depth:3,text:"Context Interactions",id:"context-interactions"},{depth:3,text:"Registration",id:"registration"},{depth:3,text:"Messaging",id:"messaging"},{depth:2,text:"Helper Functions",id:"helper-functions"},{depth:2,text:"Advanced Usage",id:"advanced-usage"},{depth:3,text:"Accessing the Mock Context",id:"accessing-the-mock-context"},{depth:3,text:"Testing Multiple Extensions",id:"testing-multiple-extensions"},{depth:3,text:"Running Tests",id:"running-tests"},{depth:2,text:"Best Practices",id:"best-practices"},{depth:2,text:"Limitations",id:"limitations"},{depth:2,text:"Complete Example",id:"complete-example"}],raw:`
# Testing Extensions

Kit provides a testing package (\`github.com/mark3labs/kit/pkg/extensions/test\`) that enables you to write unit tests for your extensions. Tests run outside the Yaegi interpreter but load your extension code into an isolated interpreter instance, allowing you to verify behavior without running the full Kit TUI.

## Overview

Extension tests allow you to:

- Test event handlers without running the interactive TUI
- Verify tool/command registration
- Assert that context methods (Print, SetWidget, etc.) are called correctly
- Test blocking and non-blocking event handling
- Simulate user input and tool calls
- Verify widget, header, footer, and status bar updates

## Installation

The test package is part of the Kit codebase. Import it in your extension tests:

\`\`\`go
import (
    "testing"
    "github.com/mark3labs/kit/pkg/extensions/test"
    "github.com/mark3labs/kit/internal/extensions"
)
\`\`\`

## Basic Usage

### Testing an Extension File

Create a test file alongside your extension (e.g., \`my-ext_test.go\`):

\`\`\`go
package main

import (
    "testing"
    "github.com/mark3labs/kit/pkg/extensions/test"
    "github.com/mark3labs/kit/internal/extensions"
)

func TestMyExtension(t *testing.T) {
    // Create a test harness
    harness := test.New(t)
    
    // Load your extension
    harness.LoadFile("my-ext.go")
    
    // Emit events and check results
    result, err := harness.Emit(extensions.ToolCallEvent{
        ToolName: "my_tool",
        Input:    \`{"key": "value"}\`,
    })
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    
    // Use assertion helpers
    test.AssertNotBlocked(t, result)
    test.AssertPrinted(t, harness, "expected output")
}
\`\`\`

### Testing Inline Extension Code

For quick tests or edge cases, you can load extension source directly:

\`\`\`go
func TestToolBlocking(t *testing.T) {
    src := \`package main

import "kit/ext"

func Init(api ext.API) {
    api.OnToolCall(func(tc ext.ToolCallEvent, ctx ext.Context) *ext.ToolCallResult {
        if tc.ToolName == "dangerous" {
            return &ext.ToolCallResult{Block: true, Reason: "not allowed"}
        }
        return nil
    })
}
\`
    harness := test.New(t)
    harness.LoadString(src, "test-ext.go")
    
    // Test the tool is blocked
    result, _ := harness.Emit(extensions.ToolCallEvent{
        ToolName: "dangerous",
        Input:    "{}",
    })
    
    test.AssertBlocked(t, result, "not allowed")
}
\`\`\`

## Common Testing Patterns

### Testing Handler Registration

Verify your extension registers the expected handlers:

\`\`\`go
func TestHandlers(t *testing.T) {
    harness := test.New(t)
    harness.LoadFile("my-ext.go")
    
    test.AssertHasHandlers(t, harness, extensions.ToolCall)
    test.AssertHasHandlers(t, harness, extensions.SessionStart)
    test.AssertNoHandlers(t, harness, extensions.AgentEnd) // Verify no unexpected handlers
}
\`\`\`

### Testing Tool Registration

\`\`\`go
func TestTools(t *testing.T) {
    harness := test.New(t)
    harness.LoadFile("my-ext.go")
    
    // Verify a specific tool is registered
    test.AssertToolRegistered(t, harness, "my_tool")
    
    // Or inspect all tools
    tools := harness.RegisteredTools()
    for _, tool := range tools {
        t.Logf("Tool: %s - %s", tool.Name, tool.Description)
    }
}
\`\`\`

### Testing Commands

\`\`\`go
func TestCommands(t *testing.T) {
    harness := test.New(t)
    harness.LoadFile("my-ext.go")
    
    test.AssertCommandRegistered(t, harness, "mycommand")
}
\`\`\`

### Testing Widgets

\`\`\`go
func TestWidgets(t *testing.T) {
    harness := test.New(t)
    harness.LoadFile("my-ext.go")
    
    // Trigger event that creates the widget
    _, _ = harness.Emit(extensions.SessionStartEvent{SessionID: "test"})
    
    // Verify widget was set
    test.AssertWidgetSet(t, harness, "my-widget")
    test.AssertWidgetText(t, harness, "my-widget", "Expected Text")
    test.AssertWidgetTextContains(t, harness, "my-widget", "partial")
    
    // Check widget properties directly
    widget, ok := harness.Context().GetWidget("my-widget")
    if ok {
        t.Logf("Border color: %s", widget.Style.BorderColor)
    }
}
\`\`\`

### Testing Input Handling

\`\`\`go
func TestInput(t *testing.T) {
    harness := test.New(t)
    harness.LoadFile("my-ext.go")
    
    result, _ := harness.Emit(extensions.InputEvent{
        Text:   "!mycommand",
        Source: "cli",
    })
    
    test.AssertInputHandled(t, result, "handled")
}
\`\`\`

### Testing Headers and Footers

\`\`\`go
func TestHeaderFooter(t *testing.T) {
    harness := test.New(t)
    harness.LoadFile("my-ext.go")
    
    _, _ = harness.Emit(extensions.SessionStartEvent{SessionID: "test"})
    
    test.AssertHeaderSet(t, harness)
    test.AssertFooterSet(t, harness)
    
    // Inspect content
    header := harness.Context().GetHeader()
    if header != nil {
        t.Logf("Header text: %s", header.Content.Text)
    }
}
\`\`\`

### Testing Status Bar

\`\`\`go
func TestStatus(t *testing.T) {
    harness := test.New(t)
    harness.LoadFile("my-ext.go")
    
    _, _ = harness.Emit(extensions.AgentEndEvent{})
    
    test.AssertStatusSet(t, harness, "myext:status")
    test.AssertStatusText(t, harness, "myext:status", "Ready")
}
\`\`\`

### Testing Print Output

\`\`\`go
func TestOutput(t *testing.T) {
    harness := test.New(t)
    harness.LoadFile("my-ext.go")
    
    _, _ = harness.Emit(extensions.ToolCallEvent{ToolName: "test"})
    
    // Exact match
    test.AssertPrinted(t, harness, "exact output")
    
    // Partial match
    test.AssertPrintedContains(t, harness, "partial")
    
    // Styled output
    test.AssertPrintInfo(t, harness, "info message")
    test.AssertPrintError(t, harness, "error message")
}
\`\`\`

### Testing with Prompts

Configure mock prompt results for testing interactive behavior:

\`\`\`go
func TestWithPrompts(t *testing.T) {
    harness := test.New(t)
    harness.LoadFile("my-ext.go")
    
    // Configure what prompts should return
    harness.Context().SetPromptSelectResult(extensions.PromptSelectResult{
        Value:     "option1",
        Index:     0,
        Cancelled: false,
    })
    
    harness.Context().SetPromptConfirmResult(extensions.PromptConfirmResult{
        Value:     true,
        Cancelled: false,
    })
    
    // Now when your extension calls ctx.PromptSelect(), it gets this result
    _, _ = harness.Emit(extensions.SessionStartEvent{SessionID: "test"})
}
\`\`\`

### Testing Complete Session Flow

\`\`\`go
func TestFullSession(t *testing.T) {
    harness := test.New(t)
    harness.LoadFile("my-ext.go")
    
    // Simulate a complete session
    _, _ = harness.Emit(extensions.SessionStartEvent{SessionID: "test"})
    _, _ = harness.Emit(extensions.BeforeAgentStartEvent{})
    _, _ = harness.Emit(extensions.AgentStartEvent{})
    
    // Multiple tool calls
    tools := []string{"Read", "Grep", "Bash"}
    for _, tool := range tools {
        _, _ = harness.Emit(extensions.ToolCallEvent{ToolName: tool})
        _, _ = harness.Emit(extensions.ToolResultEvent{ToolName: tool})
    }
    
    _, _ = harness.Emit(extensions.AgentEndEvent{})
    _, _ = harness.Emit(extensions.SessionShutdownEvent{})
    
    // Verify final state
    test.AssertWidgetTextContains(t, harness, "status", "Complete")
}
\`\`\`

## Available Assertions

The test package provides these assertion helpers:

### Event Results

| Function | Description |
|----------|-------------|
| \`AssertNotBlocked(t, result)\` | Verify tool was not blocked |
| \`AssertBlocked(t, result, reason)\` | Verify tool was blocked with reason |
| \`AssertInputHandled(t, result, action)\` | Verify input was handled |
| \`AssertInputTransformed(t, result, text)\` | Verify input was transformed |

### Context Interactions

| Function | Description |
|----------|-------------|
| \`AssertPrinted(t, harness, text)\` | Verify exact print output |
| \`AssertPrintedContains(t, harness, substring)\` | Verify partial print output |
| \`AssertPrintInfo(t, harness, text)\` | Verify PrintInfo was called |
| \`AssertPrintError(t, harness, text)\` | Verify PrintError was called |
| \`AssertWidgetSet(t, harness, id)\` | Verify widget was set |
| \`AssertWidgetNotSet(t, harness, id)\` | Verify widget was not set |
| \`AssertWidgetText(t, harness, id, text)\` | Verify widget content |
| \`AssertWidgetTextContains(t, harness, id, substring)\` | Verify widget contains text |
| \`AssertHeaderSet(t, harness)\` | Verify header was set |
| \`AssertFooterSet(t, harness)\` | Verify footer was set |
| \`AssertStatusSet(t, harness, key)\` | Verify status was set |
| \`AssertStatusText(t, harness, key, text)\` | Verify status text |

### Registration

| Function | Description |
|----------|-------------|
| \`AssertToolRegistered(t, harness, name)\` | Verify tool registration |
| \`AssertCommandRegistered(t, harness, name)\` | Verify command registration |
| \`AssertHasHandlers(t, harness, eventType)\` | Verify handlers exist |
| \`AssertNoHandlers(t, harness, eventType)\` | Verify no handlers |

### Messaging

| Function | Description |
|----------|-------------|
| \`AssertMessageSent(t, harness, text)\` | Verify SendMessage was called |
| \`AssertCancelAndSend(t, harness, text)\` | Verify CancelAndSend was called |

## Helper Functions

For custom assertions, extract result details:

\`\`\`go
result, _ := harness.Emit(extensions.ToolCallEvent{...})
tcr := test.GetToolCallResult(result)
if tcr != nil {
    t.Logf("Block: %v, Reason: %s", tcr.Block, tcr.Reason)
}

ir := test.GetInputResult(result)
trr := test.GetToolResultResult(result)
\`\`\`

## Advanced Usage

### Accessing the Mock Context

For custom verification:

\`\`\`go
ctx := harness.Context()

// Get all recorded prints
prints := ctx.GetPrints()

// Check options
value := ctx.GetOption("my-option")

// Verify widget properties
widget, ok := ctx.GetWidget("my-widget")
if ok && widget.Style.BorderColor == "#ff0000" {
    t.Log("Widget has red border")
}

// Check status entries
status, ok := ctx.GetStatus("myext:status")
\`\`\`

### Testing Multiple Extensions

Each harness is isolated:

\`\`\`go
harness1 := test.New(t)
harness1.LoadFile("ext1.go")

harness2 := test.New(t)
harness2.LoadFile("ext2.go")

// Events to one don't affect the other
\`\`\`

### Running Tests

Run all tests in your extension directory:

\`\`\`bash
cd examples/extensions
go test -v
\`\`\`

Run with race detector:

\`\`\`bash
go test -race -v
\`\`\`

Run a specific test:

\`\`\`bash
go test -v -run TestMyExtension
\`\`\`

## Best Practices

1. **Test one behavior per test** — Keep tests focused and readable
2. **Use inline source for edge cases** — \`LoadString()\` is great for testing specific scenarios
3. **Use \`LoadFile()\` for integration tests** — Tests the actual extension file
4. **Assert on context calls** — Verify your extension interacts with the context correctly
5. **Test both positive and negative cases** — Verify tools are blocked AND allowed appropriately
6. **Test all event handlers** — Make sure all registered handlers work correctly
7. **Use descriptive test names** — \`TestExtension_BlocksDangerousTools\` is clearer than \`Test1\`

## Limitations

The test harness has these intentional limitations:

- **No TUI rendering** — Widgets are recorded but not rendered visually
- **Prompts return configured values** — Pre-configure prompt results in tests
- **Subagents don't spawn real processes** — \`SpawnSubagent()\` returns nil/empty results
- **LLM completions are mocked** — \`Complete()\` returns empty responses
- **Some context methods are no-ops** — \`Exit()\`, \`SetActiveTools()\`, etc. don't have side effects

These limitations focus testing on extension logic rather than the full Kit runtime.

## Complete Example

See \`examples/extensions/tool-logger_test.go\` for a complete example with 14 tests covering:

- Handler registration
- Tool call and result handling
- Session lifecycle events
- Input commands (\`!time\`, \`!status\`)
- Unknown command handling
- Concurrent operations (race condition check)
- Real file logging verification
`};export{s as default};

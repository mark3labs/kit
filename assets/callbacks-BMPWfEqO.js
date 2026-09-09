var e={frontmatter:{title:`Callbacks`,description:`Monitor tool calls and streaming output with the Kit Go SDK.`,hidden:!1,toc:!0,draft:!1},html:`<h1 id="callbacks"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#callbacks"><span class="icon icon-link"></span></a>Callbacks</h1>
<h2 id="event-based-monitoring"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#event-based-monitoring"><span class="icon icon-link"></span></a>Event-based monitoring</h2>
<p>Subscribe to events for real-time monitoring. Each method returns an unsubscribe function:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">unsub </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">OnToolCall</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#D73A49;--shiki-dark:#F97583">func</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#E36209;--shiki-dark:#FFAB70">event</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ToolCallEvent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) {</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    fmt.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Printf</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"Tool: </span><span style="color:#005CC5;--shiki-dark:#79B8FF">%s</span><span style="color:#032F62;--shiki-dark:#9ECBFF">, Args: </span><span style="color:#005CC5;--shiki-dark:#79B8FF">%s\\n</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, event.ToolName, event.ToolArgs)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">defer</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> unsub</span><span style="color:#24292E;--shiki-dark:#E1E4E8">()</span></span>
<span class="line"></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">unsub2 </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">OnToolResult</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#D73A49;--shiki-dark:#F97583">func</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#E36209;--shiki-dark:#FFAB70">event</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ToolResultEvent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) {</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    fmt.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Printf</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"Result: </span><span style="color:#005CC5;--shiki-dark:#79B8FF">%s</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> (error: </span><span style="color:#005CC5;--shiki-dark:#79B8FF">%v</span><span style="color:#032F62;--shiki-dark:#9ECBFF">)</span><span style="color:#005CC5;--shiki-dark:#79B8FF">\\n</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, event.ToolName, event.IsError)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">defer</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> unsub2</span><span style="color:#24292E;--shiki-dark:#E1E4E8">()</span></span>
<span class="line"></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">unsub3 </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">OnMessageUpdate</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#D73A49;--shiki-dark:#F97583">func</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#E36209;--shiki-dark:#FFAB70">event</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">MessageUpdateEvent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) {</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    fmt.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Print</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(event.Chunk)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">defer</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> unsub3</span><span style="color:#24292E;--shiki-dark:#E1E4E8">()</span></span>
<span class="line"></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">unsub4 </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">OnResponse</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#D73A49;--shiki-dark:#F97583">func</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#E36209;--shiki-dark:#FFAB70">event</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ResponseEvent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) {</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    fmt.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Println</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"Final response received"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">defer</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> unsub4</span><span style="color:#24292E;--shiki-dark:#E1E4E8">()</span></span>
<span class="line"></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">unsub5 </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">OnTurnStart</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#D73A49;--shiki-dark:#F97583">func</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#E36209;--shiki-dark:#FFAB70">event</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">TurnStartEvent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) {</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    fmt.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Println</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"Turn started"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">defer</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> unsub5</span><span style="color:#24292E;--shiki-dark:#E1E4E8">()</span></span>
<span class="line"></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">unsub6 </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">OnTurnEnd</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#D73A49;--shiki-dark:#F97583">func</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#E36209;--shiki-dark:#FFAB70">event</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">TurnEndEvent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) {</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    fmt.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Println</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"Turn ended"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">defer</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> unsub6</span><span style="color:#24292E;--shiki-dark:#E1E4E8">()</span></span></code></pre>
<h2 id="tool-call-argument-streaming"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#tool-call-argument-streaming"><span class="icon icon-link"></span></a>Tool call argument streaming</h2>
<p>For tools with large arguments (e.g., <code>write</code> with a full file body), the <code>ToolCallEvent</code> only fires after the full argument JSON finishes streaming — which can take 5-10+ seconds of "dead air." These three events fire during argument generation so UIs can show activity immediately:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">OnToolCallStart</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#D73A49;--shiki-dark:#F97583">func</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#E36209;--shiki-dark:#FFAB70">event</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ToolCallStartEvent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) {</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // Fires as soon as the LLM begins generating tool arguments.</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // event.ToolCallID, event.ToolName, event.ToolKind</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    fmt.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Printf</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"⏳ </span><span style="color:#005CC5;--shiki-dark:#79B8FF">%s</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> generating arguments...</span><span style="color:#005CC5;--shiki-dark:#79B8FF">\\n</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, event.ToolName)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span>
<span class="line"></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">OnToolCallDelta</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#D73A49;--shiki-dark:#F97583">func</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#E36209;--shiki-dark:#FFAB70">event</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ToolCallDeltaEvent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) {</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // Each streamed JSON fragment of the tool arguments.</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // event.ToolCallID, event.Delta</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // Useful for live-previewing content or showing byte progress.</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span>
<span class="line"></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">OnToolCallEnd</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#D73A49;--shiki-dark:#F97583">func</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#E36209;--shiki-dark:#FFAB70">event</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ToolCallEndEvent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) {</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // Tool argument streaming complete — execution about to begin.</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // event.ToolCallID</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    fmt.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Printf</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"✓ Arguments ready, executing...</span><span style="color:#005CC5;--shiki-dark:#79B8FF">\\n</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span></code></pre>
<p><strong>Full tool lifecycle</strong>: <code>ToolCallStartEvent</code> → <code>ToolCallDeltaEvent</code> (repeated) → <code>ToolCallEndEvent</code> → <code>ToolCallEvent</code> → <code>ToolExecutionStartEvent</code> → <code>ToolOutputEvent</code> (optional) → <code>ToolExecutionEndEvent</code> → <code>ToolResultEvent</code></p>
<h2 id="hook-system"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#hook-system"><span class="icon icon-link"></span></a>Hook system</h2>
<p>Hooks can <strong>modify or cancel</strong> operations. Unlike events (read-only), hooks are read-write interceptors.</p>
<h3 id="beforetoolcall--block-tool-execution"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#beforetoolcall--block-tool-execution"><span class="icon icon-link"></span></a>BeforeToolCall — block tool execution</h3>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">OnBeforeToolCall</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(kit.HookPriorityNormal, </span><span style="color:#D73A49;--shiki-dark:#F97583">func</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#E36209;--shiki-dark:#FFAB70">h</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">BeforeToolCallHook</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) </span><span style="color:#D73A49;--shiki-dark:#F97583">*</span><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">BeforeToolCallResult</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> {</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // h.ToolCallID, h.ToolName, h.ToolArgs</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // The command-execution tool reports itself as "shell" (both kit.NewShellTool</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // and the deprecated kit.NewBashTool register it under that name).</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">    if</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> h.ToolName </span><span style="color:#D73A49;--shiki-dark:#F97583">==</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> "shell"</span><span style="color:#D73A49;--shiki-dark:#F97583"> &amp;&amp;</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> strings.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Contains</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(h.ToolArgs, </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"rm -rf"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) {</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">        return</span><span style="color:#D73A49;--shiki-dark:#F97583"> &amp;</span><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">BeforeToolCallResult</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{Block: </span><span style="color:#005CC5;--shiki-dark:#79B8FF">true</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, Reason: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"dangerous command"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">}</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    }</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">    return</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> nil</span><span style="color:#6A737D;--shiki-dark:#6A737D"> // allow</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span></code></pre>
<p><code>h.ToolArgs</code> is the raw JSON the model produced. A substring match over it is a
convenience guard, not a security boundary — <code>rm -r -f</code>, a shell expansion, or a
script file all get past it. Parse the arguments and enforce an allowlist when
you need a real control.</p>
<h3 id="aftertoolresult--modify-tool-output"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#aftertoolresult--modify-tool-output"><span class="icon icon-link"></span></a>AfterToolResult — modify tool output</h3>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">OnAfterToolResult</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(kit.HookPriorityNormal, </span><span style="color:#D73A49;--shiki-dark:#F97583">func</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#E36209;--shiki-dark:#FFAB70">h</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">AfterToolResultHook</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) </span><span style="color:#D73A49;--shiki-dark:#F97583">*</span><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">AfterToolResultResult</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> {</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // h.ToolCallID, h.ToolName, h.ToolArgs, h.Result, h.IsError</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">    if</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> h.ToolName </span><span style="color:#D73A49;--shiki-dark:#F97583">==</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> "read"</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> {</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">        filtered </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> redactSecrets</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(h.Result)</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">        return</span><span style="color:#D73A49;--shiki-dark:#F97583"> &amp;</span><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">AfterToolResultResult</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{Result: </span><span style="color:#D73A49;--shiki-dark:#F97583">&amp;</span><span style="color:#24292E;--shiki-dark:#E1E4E8">filtered}</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    }</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">    return</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> nil</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span></code></pre>
<h3 id="beforeturn--modify-prompt-inject-messages"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#beforeturn--modify-prompt-inject-messages"><span class="icon icon-link"></span></a>BeforeTurn — modify prompt, inject messages</h3>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">OnBeforeTurn</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(kit.HookPriorityNormal, </span><span style="color:#D73A49;--shiki-dark:#F97583">func</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#E36209;--shiki-dark:#FFAB70">h</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">BeforeTurnHook</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) </span><span style="color:#D73A49;--shiki-dark:#F97583">*</span><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">BeforeTurnResult</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> {</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // h.Prompt</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    newPrompt </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> h.Prompt </span><span style="color:#D73A49;--shiki-dark:#F97583">+</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> "</span><span style="color:#005CC5;--shiki-dark:#79B8FF">\\n</span><span style="color:#032F62;--shiki-dark:#9ECBFF">Always respond in JSON."</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">    return</span><span style="color:#D73A49;--shiki-dark:#F97583"> &amp;</span><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">BeforeTurnResult</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{Prompt: </span><span style="color:#D73A49;--shiki-dark:#F97583">&amp;</span><span style="color:#24292E;--shiki-dark:#E1E4E8">newPrompt}</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // Also available: SystemPrompt *string, InjectText *string</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span></code></pre>
<h3 id="afterturn--observation-only"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#afterturn--observation-only"><span class="icon icon-link"></span></a>AfterTurn — observation only</h3>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">OnAfterTurn</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(kit.HookPriorityNormal, </span><span style="color:#D73A49;--shiki-dark:#F97583">func</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#E36209;--shiki-dark:#FFAB70">h</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">AfterTurnHook</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) {</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // h.Response, h.Error</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    log.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Printf</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"Turn completed: </span><span style="color:#005CC5;--shiki-dark:#79B8FF">%d</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> chars"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#6F42C1;--shiki-dark:#B392F0">len</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(h.Response))</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span></code></pre>
<h3 id="preparestep--intercept-messages-between-steps"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#preparestep--intercept-messages-between-steps"><span class="icon icon-link"></span></a>PrepareStep — intercept messages between steps</h3>
<p>The most powerful hook — fires between steps within a multi-step agent turn, after any steering messages are injected and before messages are sent to the LLM. Can replace the entire context window.</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">OnPrepareStep</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(kit.HookPriorityNormal, </span><span style="color:#D73A49;--shiki-dark:#F97583">func</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#E36209;--shiki-dark:#FFAB70">h</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">PrepareStepHook</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) </span><span style="color:#D73A49;--shiki-dark:#F97583">*</span><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">PrepareStepResult</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> {</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // h.StepNumber — zero-based step index within the turn</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // h.Messages   — current context window (includes any steering)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    </span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // Example: transform tool results with images into user messages</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    modified </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> transformImageToolResults</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(h.Messages)</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">    return</span><span style="color:#D73A49;--shiki-dark:#F97583"> &amp;</span><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">PrepareStepResult</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{Messages: modified}</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // Return nil to pass through unchanged</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span></code></pre>
<p>Use cases: transforming tool results (e.g., image data for vision models), dynamic tool filtering per step, mid-turn context injection, custom stop conditions.</p>
<h3 id="hook-priorities"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#hook-priorities"><span class="icon icon-link"></span></a>Hook priorities</h3>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">kit.HookPriorityHigh   </span><span style="color:#D73A49;--shiki-dark:#F97583">=</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> 0</span><span style="color:#6A737D;--shiki-dark:#6A737D">   // runs first</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">kit.HookPriorityNormal </span><span style="color:#D73A49;--shiki-dark:#F97583">=</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> 50</span><span style="color:#6A737D;--shiki-dark:#6A737D">  // default</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">kit.HookPriorityLow    </span><span style="color:#D73A49;--shiki-dark:#F97583">=</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> 100</span><span style="color:#6A737D;--shiki-dark:#6A737D"> // runs last</span></span></code></pre>
<p>Lower values run first. First non-nil result wins.</p>
<h2 id="all-event-types"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#all-event-types"><span class="icon icon-link"></span></a>All event types</h2>
<table>
<thead>
<tr>
<th>Event</th>
<th>Typed Subscriber</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td><code>TurnStartEvent</code></td>
<td><code>OnTurnStart</code></td>
<td>Agent turn started</td>
</tr>
<tr>
<td><code>TurnEndEvent</code></td>
<td><code>OnTurnEnd</code></td>
<td>Agent turn completed</td>
</tr>
<tr>
<td><code>MessageStartEvent</code></td>
<td><code>OnMessageStart</code></td>
<td>New assistant message begins</td>
</tr>
<tr>
<td><code>MessageUpdateEvent</code></td>
<td><code>OnMessageUpdate</code></td>
<td>Streaming text chunk from LLM</td>
</tr>
<tr>
<td><code>MessageEndEvent</code></td>
<td><code>OnMessageEnd</code></td>
<td>Assistant message complete</td>
</tr>
<tr>
<td><code>ToolCallStartEvent</code></td>
<td><code>OnToolCallStart</code></td>
<td>LLM began generating tool call arguments</td>
</tr>
<tr>
<td><code>ToolCallDeltaEvent</code></td>
<td><code>OnToolCallDelta</code></td>
<td>Streamed JSON fragment of tool call arguments</td>
</tr>
<tr>
<td><code>ToolCallEndEvent</code></td>
<td><code>OnToolCallEnd</code></td>
<td>Tool argument streaming complete</td>
</tr>
<tr>
<td><code>ToolCallEvent</code></td>
<td><code>OnToolCall</code></td>
<td>Tool call fully parsed, about to execute</td>
</tr>
<tr>
<td><code>ToolExecutionStartEvent</code></td>
<td><code>OnToolExecutionStart</code></td>
<td>Tool begins executing</td>
</tr>
<tr>
<td><code>ToolExecutionEndEvent</code></td>
<td><code>OnToolExecutionEnd</code></td>
<td>Tool finishes executing</td>
</tr>
<tr>
<td><code>ToolResultEvent</code></td>
<td><code>OnToolResult</code></td>
<td>Tool execution completed with result</td>
</tr>
<tr>
<td><code>ToolCallContentEvent</code></td>
<td><code>OnToolCallContent</code></td>
<td>Text content alongside tool calls</td>
</tr>
<tr>
<td><code>ToolOutputEvent</code></td>
<td><code>OnToolOutput</code></td>
<td>Streaming output chunk from tool (e.g., bash)</td>
</tr>
<tr>
<td><code>ResponseEvent</code></td>
<td><code>OnResponse</code></td>
<td>Final response received</td>
</tr>
<tr>
<td><code>ReasoningStartEvent</code></td>
<td><code>OnReasoningStart</code></td>
<td>LLM begins reasoning/thinking</td>
</tr>
<tr>
<td><code>ReasoningDeltaEvent</code></td>
<td><code>OnReasoningDelta</code></td>
<td>Streaming reasoning/thinking chunk</td>
</tr>
<tr>
<td><code>ReasoningCompleteEvent</code></td>
<td><code>OnReasoningComplete</code></td>
<td>Reasoning/thinking finished</td>
</tr>
<tr>
<td><code>StepStartEvent</code></td>
<td><code>OnStepStart</code></td>
<td>New LLM call begins within a turn</td>
</tr>
<tr>
<td><code>StepFinishEvent</code></td>
<td><code>OnStepFinish</code></td>
<td>Step completes (with usage, finish reason, tool call info)</td>
</tr>
<tr>
<td><code>StepUsageEvent</code></td>
<td><code>OnStepUsage</code></td>
<td>Per-step token usage</td>
</tr>
<tr>
<td><code>StreamFinishEvent</code></td>
<td><code>OnStreamFinish</code></td>
<td>Per-step stream completes (with usage + finish reason)</td>
</tr>
<tr>
<td><code>TextStartEvent</code></td>
<td><code>OnTextStart</code></td>
<td>LLM begins text content generation</td>
</tr>
<tr>
<td><code>TextEndEvent</code></td>
<td><code>OnTextEnd</code></td>
<td>LLM finishes text content generation</td>
</tr>
<tr>
<td><code>WarningsEvent</code></td>
<td><code>OnWarnings</code></td>
<td>LLM provider returned warnings</td>
</tr>
<tr>
<td><code>SourceEvent</code></td>
<td><code>OnSource</code></td>
<td>LLM referenced a source (e.g., web search)</td>
</tr>
<tr>
<td><code>ErrorEvent</code></td>
<td><code>OnError</code></td>
<td>Agent-level error during streaming</td>
</tr>
<tr>
<td><code>RetryEvent</code></td>
<td><code>OnRetry</code></td>
<td>LLM request retried after transient error</td>
</tr>
<tr>
<td><code>CompactionEvent</code></td>
<td><code>OnCompaction</code></td>
<td>Conversation compacted (fires on success <strong>and</strong> failure — check <code>Err</code>)</td>
</tr>
<tr>
<td><code>SteerConsumedEvent</code></td>
<td><code>OnSteerConsumed</code></td>
<td>Steering messages injected into turn</td>
</tr>
<tr>
<td><code>PasswordPromptEvent</code></td>
<td>—</td>
<td>Sudo command needs password (respond via <code>ResponseCh</code>)</td>
</tr>
</tbody>
</table>
<blockquote>
<p><strong>Note:</strong> <code>OnStreaming</code> is a deprecated alias for <code>OnMessageUpdate</code> and will be removed in a future release.</p>
</blockquote>
<h3 id="compaction-telemetry"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#compaction-telemetry"><span class="icon icon-link"></span></a>Compaction telemetry</h3>
<p><code>CompactionEvent</code> fires after every compaction attempt. On success <code>Err</code> is
<code>nil</code> and the summary/token/file fields are populated; on failure <code>Err</code> is
non-nil and the rest are zero-valued. This lets you wire symmetric
start/end lifecycle telemetry without hand-rolling the failure path:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">OnCompaction</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#D73A49;--shiki-dark:#F97583">func</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#E36209;--shiki-dark:#FFAB70">e</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">CompactionEvent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) {</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">    if</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> e.Err </span><span style="color:#D73A49;--shiki-dark:#F97583">!=</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> nil</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> {</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">        log.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Printf</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"compaction failed: </span><span style="color:#005CC5;--shiki-dark:#79B8FF">%v</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, e.Err)</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">        return</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    }</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    log.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Printf</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"compacted </span><span style="color:#005CC5;--shiki-dark:#79B8FF">%d</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> → </span><span style="color:#005CC5;--shiki-dark:#79B8FF">%d</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> tokens (</span><span style="color:#005CC5;--shiki-dark:#79B8FF">%d</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> messages removed)"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">        e.OriginalTokens, e.CompactedTokens, e.MessagesRemoved)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span></code></pre>
<h2 id="subagent-event-monitoring"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#subagent-event-monitoring"><span class="icon icon-link"></span></a>Subagent event monitoring</h2>
<p>Monitor real-time events from LLM-initiated subagents (when the model uses the <code>subagent</code> tool):</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">OnToolCall</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#D73A49;--shiki-dark:#F97583">func</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#E36209;--shiki-dark:#FFAB70">e</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ToolCallEvent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) {</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">    if</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> e.ToolName </span><span style="color:#D73A49;--shiki-dark:#F97583">==</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> "subagent"</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> {</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">        host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">SubscribeSubagent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(e.ToolCallID, </span><span style="color:#D73A49;--shiki-dark:#F97583">func</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#E36209;--shiki-dark:#FFAB70">event</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Event</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) {</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">            // Receives the same event types as Subscribe(), scoped to the child agent</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">            switch</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> ev </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> event.(</span><span style="color:#D73A49;--shiki-dark:#F97583">type</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) {</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">            case</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">MessageUpdateEvent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">:</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">                fmt.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Print</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(ev.Chunk)</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">            case</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ToolCallEvent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">:</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">                fmt.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Printf</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"Subagent calling: </span><span style="color:#005CC5;--shiki-dark:#79B8FF">%s\\n</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, ev.ToolName)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">            }</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">        })</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    }</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span></code></pre>
<p><code>SubscribeSubagent</code> returns an unsubscribe function. Listeners are also cleaned up automatically when the subagent completes. See <a href="/advanced/subagents">Subagents</a> for more details.</p>`,headings:[{depth:2,text:`Event-based monitoring`,id:`event-based-monitoring`},{depth:2,text:`Tool call argument streaming`,id:`tool-call-argument-streaming`},{depth:2,text:`Hook system`,id:`hook-system`},{depth:3,text:`BeforeToolCall — block tool execution`,id:`beforetoolcall--block-tool-execution`},{depth:3,text:`AfterToolResult — modify tool output`,id:`aftertoolresult--modify-tool-output`},{depth:3,text:`BeforeTurn — modify prompt, inject messages`,id:`beforeturn--modify-prompt-inject-messages`},{depth:3,text:`AfterTurn — observation only`,id:`afterturn--observation-only`},{depth:3,text:`PrepareStep — intercept messages between steps`,id:`preparestep--intercept-messages-between-steps`},{depth:3,text:`Hook priorities`,id:`hook-priorities`},{depth:2,text:`All event types`,id:`all-event-types`},{depth:3,text:`Compaction telemetry`,id:`compaction-telemetry`},{depth:2,text:`Subagent event monitoring`,id:`subagent-event-monitoring`}],raw:`
# Callbacks

## Event-based monitoring

Subscribe to events for real-time monitoring. Each method returns an unsubscribe function:

\`\`\`go
unsub := host.OnToolCall(func(event kit.ToolCallEvent) {
    fmt.Printf("Tool: %s, Args: %s\\n", event.ToolName, event.ToolArgs)
})
defer unsub()

unsub2 := host.OnToolResult(func(event kit.ToolResultEvent) {
    fmt.Printf("Result: %s (error: %v)\\n", event.ToolName, event.IsError)
})
defer unsub2()

unsub3 := host.OnMessageUpdate(func(event kit.MessageUpdateEvent) {
    fmt.Print(event.Chunk)
})
defer unsub3()

unsub4 := host.OnResponse(func(event kit.ResponseEvent) {
    fmt.Println("Final response received")
})
defer unsub4()

unsub5 := host.OnTurnStart(func(event kit.TurnStartEvent) {
    fmt.Println("Turn started")
})
defer unsub5()

unsub6 := host.OnTurnEnd(func(event kit.TurnEndEvent) {
    fmt.Println("Turn ended")
})
defer unsub6()
\`\`\`

## Tool call argument streaming

For tools with large arguments (e.g., \`write\` with a full file body), the \`ToolCallEvent\` only fires after the full argument JSON finishes streaming — which can take 5-10+ seconds of "dead air." These three events fire during argument generation so UIs can show activity immediately:

\`\`\`go
host.OnToolCallStart(func(event kit.ToolCallStartEvent) {
    // Fires as soon as the LLM begins generating tool arguments.
    // event.ToolCallID, event.ToolName, event.ToolKind
    fmt.Printf("⏳ %s generating arguments...\\n", event.ToolName)
})

host.OnToolCallDelta(func(event kit.ToolCallDeltaEvent) {
    // Each streamed JSON fragment of the tool arguments.
    // event.ToolCallID, event.Delta
    // Useful for live-previewing content or showing byte progress.
})

host.OnToolCallEnd(func(event kit.ToolCallEndEvent) {
    // Tool argument streaming complete — execution about to begin.
    // event.ToolCallID
    fmt.Printf("✓ Arguments ready, executing...\\n")
})
\`\`\`

**Full tool lifecycle**: \`ToolCallStartEvent\` → \`ToolCallDeltaEvent\` (repeated) → \`ToolCallEndEvent\` → \`ToolCallEvent\` → \`ToolExecutionStartEvent\` → \`ToolOutputEvent\` (optional) → \`ToolExecutionEndEvent\` → \`ToolResultEvent\`

## Hook system

Hooks can **modify or cancel** operations. Unlike events (read-only), hooks are read-write interceptors.

### BeforeToolCall — block tool execution

\`\`\`go
host.OnBeforeToolCall(kit.HookPriorityNormal, func(h kit.BeforeToolCallHook) *kit.BeforeToolCallResult {
    // h.ToolCallID, h.ToolName, h.ToolArgs
    // The command-execution tool reports itself as "shell" (both kit.NewShellTool
    // and the deprecated kit.NewBashTool register it under that name).
    if h.ToolName == "shell" && strings.Contains(h.ToolArgs, "rm -rf") {
        return &kit.BeforeToolCallResult{Block: true, Reason: "dangerous command"}
    }
    return nil // allow
})
\`\`\`

\`h.ToolArgs\` is the raw JSON the model produced. A substring match over it is a
convenience guard, not a security boundary — \`rm -r -f\`, a shell expansion, or a
script file all get past it. Parse the arguments and enforce an allowlist when
you need a real control.

### AfterToolResult — modify tool output

\`\`\`go
host.OnAfterToolResult(kit.HookPriorityNormal, func(h kit.AfterToolResultHook) *kit.AfterToolResultResult {
    // h.ToolCallID, h.ToolName, h.ToolArgs, h.Result, h.IsError
    if h.ToolName == "read" {
        filtered := redactSecrets(h.Result)
        return &kit.AfterToolResultResult{Result: &filtered}
    }
    return nil
})
\`\`\`

### BeforeTurn — modify prompt, inject messages

\`\`\`go
host.OnBeforeTurn(kit.HookPriorityNormal, func(h kit.BeforeTurnHook) *kit.BeforeTurnResult {
    // h.Prompt
    newPrompt := h.Prompt + "\\nAlways respond in JSON."
    return &kit.BeforeTurnResult{Prompt: &newPrompt}
    // Also available: SystemPrompt *string, InjectText *string
})
\`\`\`

### AfterTurn — observation only

\`\`\`go
host.OnAfterTurn(kit.HookPriorityNormal, func(h kit.AfterTurnHook) {
    // h.Response, h.Error
    log.Printf("Turn completed: %d chars", len(h.Response))
})
\`\`\`

### PrepareStep — intercept messages between steps

The most powerful hook — fires between steps within a multi-step agent turn, after any steering messages are injected and before messages are sent to the LLM. Can replace the entire context window.

\`\`\`go
host.OnPrepareStep(kit.HookPriorityNormal, func(h kit.PrepareStepHook) *kit.PrepareStepResult {
    // h.StepNumber — zero-based step index within the turn
    // h.Messages   — current context window (includes any steering)
    
    // Example: transform tool results with images into user messages
    modified := transformImageToolResults(h.Messages)
    return &kit.PrepareStepResult{Messages: modified}
    // Return nil to pass through unchanged
})
\`\`\`

Use cases: transforming tool results (e.g., image data for vision models), dynamic tool filtering per step, mid-turn context injection, custom stop conditions.

### Hook priorities

\`\`\`go
kit.HookPriorityHigh   = 0   // runs first
kit.HookPriorityNormal = 50  // default
kit.HookPriorityLow    = 100 // runs last
\`\`\`

Lower values run first. First non-nil result wins.

## All event types

| Event | Typed Subscriber | Description |
|-------|-----------------|-------------|
| \`TurnStartEvent\` | \`OnTurnStart\` | Agent turn started |
| \`TurnEndEvent\` | \`OnTurnEnd\` | Agent turn completed |
| \`MessageStartEvent\` | \`OnMessageStart\` | New assistant message begins |
| \`MessageUpdateEvent\` | \`OnMessageUpdate\` | Streaming text chunk from LLM |
| \`MessageEndEvent\` | \`OnMessageEnd\` | Assistant message complete |
| \`ToolCallStartEvent\` | \`OnToolCallStart\` | LLM began generating tool call arguments |
| \`ToolCallDeltaEvent\` | \`OnToolCallDelta\` | Streamed JSON fragment of tool call arguments |
| \`ToolCallEndEvent\` | \`OnToolCallEnd\` | Tool argument streaming complete |
| \`ToolCallEvent\` | \`OnToolCall\` | Tool call fully parsed, about to execute |
| \`ToolExecutionStartEvent\` | \`OnToolExecutionStart\` | Tool begins executing |
| \`ToolExecutionEndEvent\` | \`OnToolExecutionEnd\` | Tool finishes executing |
| \`ToolResultEvent\` | \`OnToolResult\` | Tool execution completed with result |
| \`ToolCallContentEvent\` | \`OnToolCallContent\` | Text content alongside tool calls |
| \`ToolOutputEvent\` | \`OnToolOutput\` | Streaming output chunk from tool (e.g., bash) |
| \`ResponseEvent\` | \`OnResponse\` | Final response received |
| \`ReasoningStartEvent\` | \`OnReasoningStart\` | LLM begins reasoning/thinking |
| \`ReasoningDeltaEvent\` | \`OnReasoningDelta\` | Streaming reasoning/thinking chunk |
| \`ReasoningCompleteEvent\` | \`OnReasoningComplete\` | Reasoning/thinking finished |
| \`StepStartEvent\` | \`OnStepStart\` | New LLM call begins within a turn |
| \`StepFinishEvent\` | \`OnStepFinish\` | Step completes (with usage, finish reason, tool call info) |
| \`StepUsageEvent\` | \`OnStepUsage\` | Per-step token usage |
| \`StreamFinishEvent\` | \`OnStreamFinish\` | Per-step stream completes (with usage + finish reason) |
| \`TextStartEvent\` | \`OnTextStart\` | LLM begins text content generation |
| \`TextEndEvent\` | \`OnTextEnd\` | LLM finishes text content generation |
| \`WarningsEvent\` | \`OnWarnings\` | LLM provider returned warnings |
| \`SourceEvent\` | \`OnSource\` | LLM referenced a source (e.g., web search) |
| \`ErrorEvent\` | \`OnError\` | Agent-level error during streaming |
| \`RetryEvent\` | \`OnRetry\` | LLM request retried after transient error |
| \`CompactionEvent\` | \`OnCompaction\` | Conversation compacted (fires on success **and** failure — check \`Err\`) |
| \`SteerConsumedEvent\` | \`OnSteerConsumed\` | Steering messages injected into turn |
| \`PasswordPromptEvent\` | — | Sudo command needs password (respond via \`ResponseCh\`) |

> **Note:** \`OnStreaming\` is a deprecated alias for \`OnMessageUpdate\` and will be removed in a future release.

### Compaction telemetry

\`CompactionEvent\` fires after every compaction attempt. On success \`Err\` is
\`nil\` and the summary/token/file fields are populated; on failure \`Err\` is
non-nil and the rest are zero-valued. This lets you wire symmetric
start/end lifecycle telemetry without hand-rolling the failure path:

\`\`\`go
host.OnCompaction(func(e kit.CompactionEvent) {
    if e.Err != nil {
        log.Printf("compaction failed: %v", e.Err)
        return
    }
    log.Printf("compacted %d → %d tokens (%d messages removed)",
        e.OriginalTokens, e.CompactedTokens, e.MessagesRemoved)
})
\`\`\`

## Subagent event monitoring

Monitor real-time events from LLM-initiated subagents (when the model uses the \`subagent\` tool):

\`\`\`go
host.OnToolCall(func(e kit.ToolCallEvent) {
    if e.ToolName == "subagent" {
        host.SubscribeSubagent(e.ToolCallID, func(event kit.Event) {
            // Receives the same event types as Subscribe(), scoped to the child agent
            switch ev := event.(type) {
            case kit.MessageUpdateEvent:
                fmt.Print(ev.Chunk)
            case kit.ToolCallEvent:
                fmt.Printf("Subagent calling: %s\\n", ev.ToolName)
            }
        })
    }
})
\`\`\`

\`SubscribeSubagent\` returns an unsubscribe function. Listeners are also cleaned up automatically when the subagent completes. See [Subagents](/advanced/subagents) for more details.
`};export{e as default};
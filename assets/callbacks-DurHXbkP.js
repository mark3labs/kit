const s={frontmatter:{title:"Callbacks",description:"Monitor tool calls and streaming output with the Kit Go SDK.",hidden:!1,toc:!0,draft:!1},html:`<h1 id="callbacks"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#callbacks"><span class="icon icon-link"></span></a>Callbacks</h1>
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
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">    if</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> h.ToolName </span><span style="color:#D73A49;--shiki-dark:#F97583">==</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> "bash"</span><span style="color:#D73A49;--shiki-dark:#F97583"> &amp;&amp;</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> strings.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Contains</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(h.ToolArgs, </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"rm -rf"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) {</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">        return</span><span style="color:#D73A49;--shiki-dark:#F97583"> &amp;</span><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">BeforeToolCallResult</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{Block: </span><span style="color:#005CC5;--shiki-dark:#79B8FF">true</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, Reason: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"dangerous command"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">}</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    }</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">    return</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> nil</span><span style="color:#6A737D;--shiki-dark:#6A737D"> // allow</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span></code></pre>
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
<td>Conversation compacted</td>
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
<p><code>SubscribeSubagent</code> returns an unsubscribe function. Listeners are also cleaned up automatically when the subagent completes. See <a href="/advanced/subagents">Subagents</a> for more details.</p>`,headings:[{depth:2,text:"Event-based monitoring",id:"event-based-monitoring"},{depth:2,text:"Tool call argument streaming",id:"tool-call-argument-streaming"},{depth:2,text:"Hook system",id:"hook-system"},{depth:3,text:"BeforeToolCall — block tool execution",id:"beforetoolcall--block-tool-execution"},{depth:3,text:"AfterToolResult — modify tool output",id:"aftertoolresult--modify-tool-output"},{depth:3,text:"BeforeTurn — modify prompt, inject messages",id:"beforeturn--modify-prompt-inject-messages"},{depth:3,text:"AfterTurn — observation only",id:"afterturn--observation-only"},{depth:3,text:"PrepareStep — intercept messages between steps",id:"preparestep--intercept-messages-between-steps"},{depth:3,text:"Hook priorities",id:"hook-priorities"},{depth:2,text:"All event types",id:"all-event-types"},{depth:2,text:"Subagent event monitoring",id:"subagent-event-monitoring"}],raw:'\n# Callbacks\n\n## Event-based monitoring\n\nSubscribe to events for real-time monitoring. Each method returns an unsubscribe function:\n\n```go\nunsub := host.OnToolCall(func(event kit.ToolCallEvent) {\n    fmt.Printf("Tool: %s, Args: %s\\n", event.ToolName, event.ToolArgs)\n})\ndefer unsub()\n\nunsub2 := host.OnToolResult(func(event kit.ToolResultEvent) {\n    fmt.Printf("Result: %s (error: %v)\\n", event.ToolName, event.IsError)\n})\ndefer unsub2()\n\nunsub3 := host.OnMessageUpdate(func(event kit.MessageUpdateEvent) {\n    fmt.Print(event.Chunk)\n})\ndefer unsub3()\n\nunsub4 := host.OnResponse(func(event kit.ResponseEvent) {\n    fmt.Println("Final response received")\n})\ndefer unsub4()\n\nunsub5 := host.OnTurnStart(func(event kit.TurnStartEvent) {\n    fmt.Println("Turn started")\n})\ndefer unsub5()\n\nunsub6 := host.OnTurnEnd(func(event kit.TurnEndEvent) {\n    fmt.Println("Turn ended")\n})\ndefer unsub6()\n```\n\n## Tool call argument streaming\n\nFor tools with large arguments (e.g., `write` with a full file body), the `ToolCallEvent` only fires after the full argument JSON finishes streaming — which can take 5-10+ seconds of "dead air." These three events fire during argument generation so UIs can show activity immediately:\n\n```go\nhost.OnToolCallStart(func(event kit.ToolCallStartEvent) {\n    // Fires as soon as the LLM begins generating tool arguments.\n    // event.ToolCallID, event.ToolName, event.ToolKind\n    fmt.Printf("⏳ %s generating arguments...\\n", event.ToolName)\n})\n\nhost.OnToolCallDelta(func(event kit.ToolCallDeltaEvent) {\n    // Each streamed JSON fragment of the tool arguments.\n    // event.ToolCallID, event.Delta\n    // Useful for live-previewing content or showing byte progress.\n})\n\nhost.OnToolCallEnd(func(event kit.ToolCallEndEvent) {\n    // Tool argument streaming complete — execution about to begin.\n    // event.ToolCallID\n    fmt.Printf("✓ Arguments ready, executing...\\n")\n})\n```\n\n**Full tool lifecycle**: `ToolCallStartEvent` → `ToolCallDeltaEvent` (repeated) → `ToolCallEndEvent` → `ToolCallEvent` → `ToolExecutionStartEvent` → `ToolOutputEvent` (optional) → `ToolExecutionEndEvent` → `ToolResultEvent`\n\n## Hook system\n\nHooks can **modify or cancel** operations. Unlike events (read-only), hooks are read-write interceptors.\n\n### BeforeToolCall — block tool execution\n\n```go\nhost.OnBeforeToolCall(kit.HookPriorityNormal, func(h kit.BeforeToolCallHook) *kit.BeforeToolCallResult {\n    // h.ToolCallID, h.ToolName, h.ToolArgs\n    if h.ToolName == "bash" && strings.Contains(h.ToolArgs, "rm -rf") {\n        return &kit.BeforeToolCallResult{Block: true, Reason: "dangerous command"}\n    }\n    return nil // allow\n})\n```\n\n### AfterToolResult — modify tool output\n\n```go\nhost.OnAfterToolResult(kit.HookPriorityNormal, func(h kit.AfterToolResultHook) *kit.AfterToolResultResult {\n    // h.ToolCallID, h.ToolName, h.ToolArgs, h.Result, h.IsError\n    if h.ToolName == "read" {\n        filtered := redactSecrets(h.Result)\n        return &kit.AfterToolResultResult{Result: &filtered}\n    }\n    return nil\n})\n```\n\n### BeforeTurn — modify prompt, inject messages\n\n```go\nhost.OnBeforeTurn(kit.HookPriorityNormal, func(h kit.BeforeTurnHook) *kit.BeforeTurnResult {\n    // h.Prompt\n    newPrompt := h.Prompt + "\\nAlways respond in JSON."\n    return &kit.BeforeTurnResult{Prompt: &newPrompt}\n    // Also available: SystemPrompt *string, InjectText *string\n})\n```\n\n### AfterTurn — observation only\n\n```go\nhost.OnAfterTurn(kit.HookPriorityNormal, func(h kit.AfterTurnHook) {\n    // h.Response, h.Error\n    log.Printf("Turn completed: %d chars", len(h.Response))\n})\n```\n\n### PrepareStep — intercept messages between steps\n\nThe most powerful hook — fires between steps within a multi-step agent turn, after any steering messages are injected and before messages are sent to the LLM. Can replace the entire context window.\n\n```go\nhost.OnPrepareStep(kit.HookPriorityNormal, func(h kit.PrepareStepHook) *kit.PrepareStepResult {\n    // h.StepNumber — zero-based step index within the turn\n    // h.Messages   — current context window (includes any steering)\n    \n    // Example: transform tool results with images into user messages\n    modified := transformImageToolResults(h.Messages)\n    return &kit.PrepareStepResult{Messages: modified}\n    // Return nil to pass through unchanged\n})\n```\n\nUse cases: transforming tool results (e.g., image data for vision models), dynamic tool filtering per step, mid-turn context injection, custom stop conditions.\n\n### Hook priorities\n\n```go\nkit.HookPriorityHigh   = 0   // runs first\nkit.HookPriorityNormal = 50  // default\nkit.HookPriorityLow    = 100 // runs last\n```\n\nLower values run first. First non-nil result wins.\n\n## All event types\n\n| Event | Typed Subscriber | Description |\n|-------|-----------------|-------------|\n| `TurnStartEvent` | `OnTurnStart` | Agent turn started |\n| `TurnEndEvent` | `OnTurnEnd` | Agent turn completed |\n| `MessageStartEvent` | `OnMessageStart` | New assistant message begins |\n| `MessageUpdateEvent` | `OnMessageUpdate` | Streaming text chunk from LLM |\n| `MessageEndEvent` | `OnMessageEnd` | Assistant message complete |\n| `ToolCallStartEvent` | `OnToolCallStart` | LLM began generating tool call arguments |\n| `ToolCallDeltaEvent` | `OnToolCallDelta` | Streamed JSON fragment of tool call arguments |\n| `ToolCallEndEvent` | `OnToolCallEnd` | Tool argument streaming complete |\n| `ToolCallEvent` | `OnToolCall` | Tool call fully parsed, about to execute |\n| `ToolExecutionStartEvent` | `OnToolExecutionStart` | Tool begins executing |\n| `ToolExecutionEndEvent` | `OnToolExecutionEnd` | Tool finishes executing |\n| `ToolResultEvent` | `OnToolResult` | Tool execution completed with result |\n| `ToolCallContentEvent` | `OnToolCallContent` | Text content alongside tool calls |\n| `ToolOutputEvent` | `OnToolOutput` | Streaming output chunk from tool (e.g., bash) |\n| `ResponseEvent` | `OnResponse` | Final response received |\n| `ReasoningStartEvent` | `OnReasoningStart` | LLM begins reasoning/thinking |\n| `ReasoningDeltaEvent` | `OnReasoningDelta` | Streaming reasoning/thinking chunk |\n| `ReasoningCompleteEvent` | `OnReasoningComplete` | Reasoning/thinking finished |\n| `StepStartEvent` | `OnStepStart` | New LLM call begins within a turn |\n| `StepFinishEvent` | `OnStepFinish` | Step completes (with usage, finish reason, tool call info) |\n| `StepUsageEvent` | `OnStepUsage` | Per-step token usage |\n| `StreamFinishEvent` | `OnStreamFinish` | Per-step stream completes (with usage + finish reason) |\n| `TextStartEvent` | `OnTextStart` | LLM begins text content generation |\n| `TextEndEvent` | `OnTextEnd` | LLM finishes text content generation |\n| `WarningsEvent` | `OnWarnings` | LLM provider returned warnings |\n| `SourceEvent` | `OnSource` | LLM referenced a source (e.g., web search) |\n| `ErrorEvent` | `OnError` | Agent-level error during streaming |\n| `RetryEvent` | `OnRetry` | LLM request retried after transient error |\n| `CompactionEvent` | `OnCompaction` | Conversation compacted |\n| `SteerConsumedEvent` | `OnSteerConsumed` | Steering messages injected into turn |\n| `PasswordPromptEvent` | — | Sudo command needs password (respond via `ResponseCh`) |\n\n> **Note:** `OnStreaming` is a deprecated alias for `OnMessageUpdate` and will be removed in a future release.\n\n## Subagent event monitoring\n\nMonitor real-time events from LLM-initiated subagents (when the model uses the `subagent` tool):\n\n```go\nhost.OnToolCall(func(e kit.ToolCallEvent) {\n    if e.ToolName == "subagent" {\n        host.SubscribeSubagent(e.ToolCallID, func(event kit.Event) {\n            // Receives the same event types as Subscribe(), scoped to the child agent\n            switch ev := event.(type) {\n            case kit.MessageUpdateEvent:\n                fmt.Print(ev.Chunk)\n            case kit.ToolCallEvent:\n                fmt.Printf("Subagent calling: %s\\n", ev.ToolName)\n            }\n        })\n    }\n})\n```\n\n`SubscribeSubagent` returns an unsubscribe function. Listeners are also cleaned up automatically when the subagent completes. See [Subagents](/advanced/subagents) for more details.\n'};export{s as default};

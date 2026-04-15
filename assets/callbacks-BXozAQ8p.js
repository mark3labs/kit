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
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">unsub3 </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">OnStreaming</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#D73A49;--shiki-dark:#F97583">func</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#E36209;--shiki-dark:#FFAB70">event</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">MessageUpdateEvent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) {</span></span>
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
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td><code>ToolCallEvent</code></td>
<td>Tool call parsed and about to execute</td>
</tr>
<tr>
<td><code>ToolResultEvent</code></td>
<td>Tool execution completed with result</td>
</tr>
<tr>
<td><code>ToolOutputEvent</code></td>
<td>Streaming output chunk from tool (e.g., bash stdout/stderr)</td>
</tr>
<tr>
<td><code>MessageUpdateEvent</code></td>
<td>Streaming text chunk from LLM</td>
</tr>
<tr>
<td><code>ResponseEvent</code></td>
<td>Final response received</td>
</tr>
<tr>
<td><code>TurnStartEvent</code></td>
<td>Agent turn started</td>
</tr>
<tr>
<td><code>TurnEndEvent</code></td>
<td>Agent turn completed</td>
</tr>
<tr>
<td><code>PasswordPromptEvent</code></td>
<td>Sudo command needs password (respond via <code>ResponseCh</code>)</td>
</tr>
</tbody>
</table>
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
<p><code>SubscribeSubagent</code> returns an unsubscribe function. Listeners are also cleaned up automatically when the subagent completes. See <a href="/advanced/subagents">Subagents</a> for more details.</p>`,headings:[{depth:2,text:"Event-based monitoring",id:"event-based-monitoring"},{depth:2,text:"Hook system",id:"hook-system"},{depth:3,text:"BeforeToolCall — block tool execution",id:"beforetoolcall--block-tool-execution"},{depth:3,text:"AfterToolResult — modify tool output",id:"aftertoolresult--modify-tool-output"},{depth:3,text:"BeforeTurn — modify prompt, inject messages",id:"beforeturn--modify-prompt-inject-messages"},{depth:3,text:"AfterTurn — observation only",id:"afterturn--observation-only"},{depth:3,text:"Hook priorities",id:"hook-priorities"},{depth:2,text:"All event types",id:"all-event-types"},{depth:2,text:"Subagent event monitoring",id:"subagent-event-monitoring"}],raw:`
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

unsub3 := host.OnStreaming(func(event kit.MessageUpdateEvent) {
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

## Hook system

Hooks can **modify or cancel** operations. Unlike events (read-only), hooks are read-write interceptors.

### BeforeToolCall — block tool execution

\`\`\`go
host.OnBeforeToolCall(kit.HookPriorityNormal, func(h kit.BeforeToolCallHook) *kit.BeforeToolCallResult {
    // h.ToolCallID, h.ToolName, h.ToolArgs
    if h.ToolName == "bash" && strings.Contains(h.ToolArgs, "rm -rf") {
        return &kit.BeforeToolCallResult{Block: true, Reason: "dangerous command"}
    }
    return nil // allow
})
\`\`\`

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

### Hook priorities

\`\`\`go
kit.HookPriorityHigh   = 0   // runs first
kit.HookPriorityNormal = 50  // default
kit.HookPriorityLow    = 100 // runs last
\`\`\`

Lower values run first. First non-nil result wins.

## All event types

| Event | Description |
|-------|-------------|
| \`ToolCallEvent\` | Tool call parsed and about to execute |
| \`ToolResultEvent\` | Tool execution completed with result |
| \`ToolOutputEvent\` | Streaming output chunk from tool (e.g., bash stdout/stderr) |
| \`MessageUpdateEvent\` | Streaming text chunk from LLM |
| \`ResponseEvent\` | Final response received |
| \`TurnStartEvent\` | Agent turn started |
| \`TurnEndEvent\` | Agent turn completed |
| \`PasswordPromptEvent\` | Sudo command needs password (respond via \`ResponseCh\`) |

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
`};export{s as default};

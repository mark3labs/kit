var e={frontmatter:{title:`SDK Sessions`,description:`Session management in the Kit Go SDK.`,hidden:!1,toc:!0,ogImage:`/og-image.png`,draft:!1},html:`<h1 id="sdk-sessions"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#sdk-sessions"><span class="icon icon-link"></span></a>SDK Sessions</h1>
<h2 id="automatic-persistence"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#automatic-persistence"><span class="icon icon-link"></span></a>Automatic persistence</h2>
<p>By default, Kit automatically persists sessions to JSONL files. Multi-turn conversations retain context across calls:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Prompt</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(ctx, </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"My name is Alice"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">response, _ </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Prompt</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(ctx, </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"What's my name?"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// response: "Your name is Alice"</span></span></code></pre>
<h2 id="accessing-session-info"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#accessing-session-info"><span class="icon icon-link"></span></a>Accessing session info</h2>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// Get the current session file path</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">path </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">GetSessionPath</span><span style="color:#24292E;--shiki-dark:#E1E4E8">()</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// Get the session ID</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">id </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">GetSessionID</span><span style="color:#24292E;--shiki-dark:#E1E4E8">()</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// Get the current model string</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">model </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">GetModelString</span><span style="color:#24292E;--shiki-dark:#E1E4E8">()</span></span></code></pre>
<h2 id="configuring-sessions-via-options"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#configuring-sessions-via-options"><span class="icon icon-link"></span></a>Configuring sessions via Options</h2>
<p>Session behavior is configured at initialization:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// Open a specific session file</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">host, _ </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> kit.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">New</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(ctx, </span><span style="color:#D73A49;--shiki-dark:#F97583">&amp;</span><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Options</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    SessionPath: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"./my-session.jsonl"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// Resume the most recent session for the current directory</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">host, _ </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> kit.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">New</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(ctx, </span><span style="color:#D73A49;--shiki-dark:#F97583">&amp;</span><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Options</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    Continue: </span><span style="color:#005CC5;--shiki-dark:#79B8FF">true</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// Ephemeral mode (no file persistence)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">host, _ </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> kit.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">New</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(ctx, </span><span style="color:#D73A49;--shiki-dark:#F97583">&amp;</span><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Options</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    NoSession: </span><span style="color:#005CC5;--shiki-dark:#79B8FF">true</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// Project identity for session discovery, not an output-storage root</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">host, _ </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> kit.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">New</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(ctx, </span><span style="color:#D73A49;--shiki-dark:#F97583">&amp;</span><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Options</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    SessionDir: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"/home/user/project"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span></code></pre>
<h2 id="clearing-history"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#clearing-history"><span class="icon icon-link"></span></a>Clearing history</h2>
<p>Clear the in-memory conversation history (does not delete the session file):</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ClearSession</span><span style="color:#24292E;--shiki-dark:#E1E4E8">()</span></span></code></pre>
<h2 id="tree-based-sessions"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#tree-based-sessions"><span class="icon icon-link"></span></a>Tree-based sessions</h2>
<p>Kit's session model is tree-based, supporting branching. You can branch from any entry to explore alternate conversation paths:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// Access the tree session manager</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">ts </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">GetTreeSession</span><span style="color:#24292E;--shiki-dark:#E1E4E8">() </span><span style="color:#6A737D;--shiki-dark:#6A737D">// nil with a custom manager or no session</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// Branch from a specific entry</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">err </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Branch</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"entry-id-123"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span></code></pre>
<h2 id="listing-and-managing-sessions"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#listing-and-managing-sessions"><span class="icon icon-link"></span></a>Listing and managing sessions</h2>
<p>Package-level functions for session discovery:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// List sessions for a specific directory</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">sessions, err </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> kit.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ListSessions</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"/home/user/project"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">if</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> err </span><span style="color:#D73A49;--shiki-dark:#F97583">!=</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> nil</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> { </span><span style="color:#D73A49;--shiki-dark:#F97583">return</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> err }</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// List all sessions across all directories</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">all, err </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> kit.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ListAllSessions</span><span style="color:#24292E;--shiki-dark:#E1E4E8">()</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">if</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> err </span><span style="color:#D73A49;--shiki-dark:#F97583">!=</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> nil</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> { </span><span style="color:#D73A49;--shiki-dark:#F97583">return</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> err }</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// Delete a session file</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">if</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> err </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> kit.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">DeleteSession</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"/path/to/session.jsonl"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">); err </span><span style="color:#D73A49;--shiki-dark:#F97583">!=</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> nil</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> { </span><span style="color:#D73A49;--shiki-dark:#F97583">return</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> err }</span></span></code></pre>
<h2 id="custom-session-manager"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#custom-session-manager"><span class="icon icon-link"></span></a>Custom session manager</h2>
<p>For advanced use cases (databases, cloud storage, multi-user apps), implement the <code>SessionManager</code> interface to replace the default JSONL file backend:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">host, _ </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> kit.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">New</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(ctx, </span><span style="color:#D73A49;--shiki-dark:#F97583">&amp;</span><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Options</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    SessionManager: myCustomSession,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span></code></pre>
<p><code>SessionManager</code> implementations must be safe for concurrent reads and writes during generation. Its method set is frozen for v0.x; new capabilities use optional interfaces. <code>GetSessionManager</code> accesses custom backends; <code>GetTreeSession</code> is only for the default file-backed tree and can return nil. <code>SessionDir</code> identifies the project bucket used by discovery and Continue, not a storage-root override.</p>
<p>The interface requires methods for message storage, branching, compaction, branch summaries, extension data, and lifecycle management. See the <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#SessionManager"><code>SessionManager</code> interface definition</a> for the complete method set.</p>
<p>The <code>AppendBranchSummary(fromID, summary)</code> method backs <code>host.CollapseBranch</code>,
which collapses a branch range into a single summary entry. Custom managers
that don't track branch summaries can return <code>kit.ErrBranchSummaryNotSupported</code>
from that method; <code>host.CollapseBranch</code> then surfaces the same sentinel so
callers can detect it with <code>errors.Is</code>.</p>
<p>When a Kit has no session manager at all, <code>host.Branch</code>, <code>host.NavigateTo</code>,
<code>host.SummarizeBranch</code>, <code>host.CollapseBranch</code>, <code>host.SetSessionName</code> and the
extension session API return <code>kit.ErrNoSession</code>, which is also detectable with
<code>errors.Is</code>.</p>
<p>When using a custom <code>SessionManager</code>, the <code>SessionPath</code>, <code>Continue</code>, and <code>NoSession</code> options are ignored — your manager handles its own storage and session selection.</p>
<h2 id="model-request-retries"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#model-request-retries"><span class="icon icon-link"></span></a>Model request retries</h2>
<p>Kit retries transient model request failures, including rate limits and
retryable provider or transport errors. Each model request has its own budget.
The default policy permits four total attempts, with exponential backoff and
jitter. The initial delay is one second, the maximum delay is 30 seconds, and
retry scheduling is limited to two minutes. Use a context deadline to limit the
duration of active requests.</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">k, err </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> kit.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">NewAgent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(ctx,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    kit.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">WithModel</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"openai/gpt-4.1"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">),</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    kit.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">WithRetryPolicy</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">RetryPolicy</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">        MaxAttempts:  </span><span style="color:#005CC5;--shiki-dark:#79B8FF">3</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#6A737D;--shiki-dark:#6A737D">// Includes the initial request.</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">        InitialDelay: time.Second,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">        MaxDelay:     </span><span style="color:#005CC5;--shiki-dark:#79B8FF">10</span><span style="color:#D73A49;--shiki-dark:#F97583"> *</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> time.Second,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">        MaxElapsed:   time.Minute,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    }),</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span></code></pre>
<p>You can also set <code>Options.RetryPolicy</code> when you use <code>kit.New</code>. Nil uses the
built-in defaults. Zero policy fields use their default values. Negative
fields also use defaults, except <code>MaxResumes</code> (see below).
Set <code>MaxAttempts: 1</code> to disable retries. Server <code>Retry-After</code> and
<code>retry-after-ms</code> hints are respected. A server delay above <code>MaxDelay</code> stops
retries instead of sending a request before the server permits it.</p>
<p><code>OnRetry</code> receives a <code>RetryEvent</code> before the scheduled wait. <code>Attempt</code> starts at
1 for the first retry and resets for each model request. <code>Delay</code> is the scheduled
wait. Cancellation can stop the wait, so an event does not guarantee that the
next request will start. Extension retry events also include the delay.</p>
<p>For safety, streaming retries are permitted only before content is delivered.
Warnings and start parts (text, reasoning, tool input) are held until content
arrives, so they do not prevent a retry. Keepalive parts are forwarded
immediately and can reach consumers from an attempt that later fails. A failure after content
was delivered is handled by step resume (below). The TUI resets transient
stream state on a retry event and keeps completed messages. Retries do not
repeat completed steps or tool execution. The underlying agent library's
retries are disabled on these agent calls to prevent a second retry layer.</p>
<h3 id="step-resume"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#step-resume"><span class="icon icon-link"></span></a>Step resume</h3>
<p>When a streaming step fails with a transient error after it streamed output,
or after its request retries are used up, Kit restarts that step from the last
completed step. This is like resuming a failed run from its saved conversation.
The default policy permits five resumes for each turn, about 5, 10, 20, 40 and
80 seconds apart (<code>MaxResumes</code>, <code>ResumeInitialDelay</code>, <code>ResumeMaxDelay</code>). Set
<code>MaxResumes: -1</code> to disable resumes.</p>
<p>Safety rules:</p>
<ul>
<li>Local tools run only after a step stream ends without error, so the failed
step ran no tools. Completed steps and tool results are never repeated.</li>
<li>The failed step's partial output is excluded from normal model history and completed-step storage. A terminal failure can still save it as an <code>incomplete_output</code> transcript record (see below).</li>
<li>Only transient errors are resumed. Auth, validation, cancellation, deadline
and tool errors are not. A failed session write stops resumes.</li>
<li>The <code>MaxSteps</code> limit applies to the whole turn, also across resumes.</li>
<li>A wait that would pass the context deadline is not started.</li>
</ul>
<p><code>OnStepResume</code> receives a <code>StepResumeEvent</code> before the wait. Remove the output
of the failed step (message, reasoning and tool input deltas since the last
<code>StepStartEvent</code>) from your display. <code>TurnResult.Stream</code> already excludes it. A
<code>StepStartEvent</code> with the same <code>StepNumber</code> follows when the step restarts.
Extension handlers can use <code>OnStepResume</code> too.</p>
<p>A resume sends a new provider request, which uses tokens again for the
conversation so far. Provider-side tools (for example web search) in the failed
step can run again on the provider.</p>
<h3 id="partial-results-and-session-write-errors"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#partial-results-and-session-write-errors"><span class="icon icon-link"></span></a>Partial results and session write errors</h3>
<p><code>PromptResult</code> and the other turn-result methods can return both a non-nil
result and an error. Check both values. <code>Incomplete</code> marks a failed turn.
<code>PartialText</code> contains text from its unfinished step. <code>AttemptID</code> identifies
that generation. <code>Messages</code> contains only normal history and completed steps;
it does not contain <code>PartialText</code>. <code>Stream</code> still contains the observed events.
The string-returning prompt methods keep their existing error behavior.</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">result, err </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">PromptResult</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(ctx, </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"Explain the change."</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">if</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> err </span><span style="color:#D73A49;--shiki-dark:#F97583">!=</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> nil</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> {</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">    if</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> result </span><span style="color:#D73A49;--shiki-dark:#F97583">!=</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> nil</span><span style="color:#D73A49;--shiki-dark:#F97583"> &amp;&amp;</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> result.PartialText </span><span style="color:#D73A49;--shiki-dark:#F97583">!=</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> ""</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> {</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">        fmt.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Printf</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"Incomplete output (</span><span style="color:#005CC5;--shiki-dark:#79B8FF">%s</span><span style="color:#032F62;--shiki-dark:#9ECBFF">): </span><span style="color:#005CC5;--shiki-dark:#79B8FF">%s\\n</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, result.AttemptID, result.PartialText)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    }</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">    return</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> err</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">}</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">fmt.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Println</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(result.Response)</span></span></code></pre>
<p>The default session backend saves unfinished text as an <code>incomplete_output</code>
JSONL tree entry with <code>attempt_id</code>, <code>text</code>, and <code>incomplete: true</code>. This entry
is available in the session tree, but <code>GetMessages</code> and <code>BuildContext</code> exclude
it from model history. The UI keeps the text and marks it as incomplete, also
when the session is loaded again. This records received text after an orderly
error or cancellation; it does not save every token before a process crash.</p>
<p><code>SessionManager</code> has no new required methods. Custom backends can implement
<code>IncompleteOutputAppender</code> to save these transcript records. Backends without
this capability still return partial text through <code>TurnResult</code>.</p>
<p>Session write errors now return to the caller. A failed prompt write prevents
generation. A failed step write stops the turn and does not count as a confirmed
write. Kit does not retry a failed write in the same turn. With the legacy
per-message fallback, earlier successful writes remain confirmed; implement
<code>StepAppender</code> for atomic storage of tool call/result pairs.</p>
<p>Kit gives optional context-aware writes a detached five-second deadline and
keeps the turn's context values. Backends must honor that deadline. Legacy
<code>AppendMessage</code> has no context, and local filesystem calls cannot be interrupted
safely; these writes remain synchronous. Kit does not start a background write
that could change the session after the caller starts another turn.</p>
<p>See the <a href="https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#StepAppender">StepAppender contract</a> for atomicity and cancellation requirements.</p>`,headings:[{depth:2,text:`Automatic persistence`,id:`automatic-persistence`},{depth:2,text:`Accessing session info`,id:`accessing-session-info`},{depth:2,text:`Configuring sessions via Options`,id:`configuring-sessions-via-options`},{depth:2,text:`Clearing history`,id:`clearing-history`},{depth:2,text:`Tree-based sessions`,id:`tree-based-sessions`},{depth:2,text:`Listing and managing sessions`,id:`listing-and-managing-sessions`},{depth:2,text:`Custom session manager`,id:`custom-session-manager`},{depth:2,text:`Model request retries`,id:`model-request-retries`},{depth:3,text:`Step resume`,id:`step-resume`},{depth:3,text:`Partial results and session write errors`,id:`partial-results-and-session-write-errors`}],raw:`
# SDK Sessions

## Automatic persistence

By default, Kit automatically persists sessions to JSONL files. Multi-turn conversations retain context across calls:

\`\`\`go
host.Prompt(ctx, "My name is Alice")
response, _ := host.Prompt(ctx, "What's my name?")
// response: "Your name is Alice"
\`\`\`

## Accessing session info

\`\`\`go
// Get the current session file path
path := host.GetSessionPath()

// Get the session ID
id := host.GetSessionID()

// Get the current model string
model := host.GetModelString()
\`\`\`

## Configuring sessions via Options

Session behavior is configured at initialization:

\`\`\`go
// Open a specific session file
host, _ := kit.New(ctx, &kit.Options{
    SessionPath: "./my-session.jsonl",
})

// Resume the most recent session for the current directory
host, _ := kit.New(ctx, &kit.Options{
    Continue: true,
})

// Ephemeral mode (no file persistence)
host, _ := kit.New(ctx, &kit.Options{
    NoSession: true,
})

// Project identity for session discovery, not an output-storage root
host, _ := kit.New(ctx, &kit.Options{
    SessionDir: "/home/user/project",
})
\`\`\`

## Clearing history

Clear the in-memory conversation history (does not delete the session file):

\`\`\`go
host.ClearSession()
\`\`\`

## Tree-based sessions

Kit's session model is tree-based, supporting branching. You can branch from any entry to explore alternate conversation paths:

\`\`\`go
// Access the tree session manager
ts := host.GetTreeSession() // nil with a custom manager or no session

// Branch from a specific entry
err := host.Branch("entry-id-123")
\`\`\`

## Listing and managing sessions

Package-level functions for session discovery:

\`\`\`go
// List sessions for a specific directory
sessions, err := kit.ListSessions("/home/user/project")
if err != nil { return err }

// List all sessions across all directories
all, err := kit.ListAllSessions()
if err != nil { return err }

// Delete a session file
if err := kit.DeleteSession("/path/to/session.jsonl"); err != nil { return err }
\`\`\`

## Custom session manager

For advanced use cases (databases, cloud storage, multi-user apps), implement the \`SessionManager\` interface to replace the default JSONL file backend:

\`\`\`go
host, _ := kit.New(ctx, &kit.Options{
    SessionManager: myCustomSession,
})
\`\`\`

\`SessionManager\` implementations must be safe for concurrent reads and writes during generation. Its method set is frozen for v0.x; new capabilities use optional interfaces. \`GetSessionManager\` accesses custom backends; \`GetTreeSession\` is only for the default file-backed tree and can return nil. \`SessionDir\` identifies the project bucket used by discovery and Continue, not a storage-root override.

The interface requires methods for message storage, branching, compaction, branch summaries, extension data, and lifecycle management. See the [\`SessionManager\` interface definition](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#SessionManager) for the complete method set.

The \`AppendBranchSummary(fromID, summary)\` method backs \`host.CollapseBranch\`,
which collapses a branch range into a single summary entry. Custom managers
that don't track branch summaries can return \`kit.ErrBranchSummaryNotSupported\`
from that method; \`host.CollapseBranch\` then surfaces the same sentinel so
callers can detect it with \`errors.Is\`.

When a Kit has no session manager at all, \`host.Branch\`, \`host.NavigateTo\`,
\`host.SummarizeBranch\`, \`host.CollapseBranch\`, \`host.SetSessionName\` and the
extension session API return \`kit.ErrNoSession\`, which is also detectable with
\`errors.Is\`.

When using a custom \`SessionManager\`, the \`SessionPath\`, \`Continue\`, and \`NoSession\` options are ignored — your manager handles its own storage and session selection.

## Model request retries

Kit retries transient model request failures, including rate limits and
retryable provider or transport errors. Each model request has its own budget.
The default policy permits four total attempts, with exponential backoff and
jitter. The initial delay is one second, the maximum delay is 30 seconds, and
retry scheduling is limited to two minutes. Use a context deadline to limit the
duration of active requests.

\`\`\`go
k, err := kit.NewAgent(ctx,
    kit.WithModel("openai/gpt-4.1"),
    kit.WithRetryPolicy(kit.RetryPolicy{
        MaxAttempts:  3, // Includes the initial request.
        InitialDelay: time.Second,
        MaxDelay:     10 * time.Second,
        MaxElapsed:   time.Minute,
    }),
)
\`\`\`

You can also set \`Options.RetryPolicy\` when you use \`kit.New\`. Nil uses the
built-in defaults. Zero policy fields use their default values. Negative
fields also use defaults, except \`MaxResumes\` (see below).
Set \`MaxAttempts: 1\` to disable retries. Server \`Retry-After\` and
\`retry-after-ms\` hints are respected. A server delay above \`MaxDelay\` stops
retries instead of sending a request before the server permits it.

\`OnRetry\` receives a \`RetryEvent\` before the scheduled wait. \`Attempt\` starts at
1 for the first retry and resets for each model request. \`Delay\` is the scheduled
wait. Cancellation can stop the wait, so an event does not guarantee that the
next request will start. Extension retry events also include the delay.

For safety, streaming retries are permitted only before content is delivered.
Warnings and start parts (text, reasoning, tool input) are held until content
arrives, so they do not prevent a retry. Keepalive parts are forwarded
immediately and can reach consumers from an attempt that later fails. A failure after content
was delivered is handled by step resume (below). The TUI resets transient
stream state on a retry event and keeps completed messages. Retries do not
repeat completed steps or tool execution. The underlying agent library's
retries are disabled on these agent calls to prevent a second retry layer.

### Step resume

When a streaming step fails with a transient error after it streamed output,
or after its request retries are used up, Kit restarts that step from the last
completed step. This is like resuming a failed run from its saved conversation.
The default policy permits five resumes for each turn, about 5, 10, 20, 40 and
80 seconds apart (\`MaxResumes\`, \`ResumeInitialDelay\`, \`ResumeMaxDelay\`). Set
\`MaxResumes: -1\` to disable resumes.

Safety rules:

- Local tools run only after a step stream ends without error, so the failed
  step ran no tools. Completed steps and tool results are never repeated.
- The failed step's partial output is excluded from normal model history and completed-step storage. A terminal failure can still save it as an \`incomplete_output\` transcript record (see below).
- Only transient errors are resumed. Auth, validation, cancellation, deadline
  and tool errors are not. A failed session write stops resumes.
- The \`MaxSteps\` limit applies to the whole turn, also across resumes.
- A wait that would pass the context deadline is not started.

\`OnStepResume\` receives a \`StepResumeEvent\` before the wait. Remove the output
of the failed step (message, reasoning and tool input deltas since the last
\`StepStartEvent\`) from your display. \`TurnResult.Stream\` already excludes it. A
\`StepStartEvent\` with the same \`StepNumber\` follows when the step restarts.
Extension handlers can use \`OnStepResume\` too.

A resume sends a new provider request, which uses tokens again for the
conversation so far. Provider-side tools (for example web search) in the failed
step can run again on the provider.

### Partial results and session write errors

\`PromptResult\` and the other turn-result methods can return both a non-nil
result and an error. Check both values. \`Incomplete\` marks a failed turn.
\`PartialText\` contains text from its unfinished step. \`AttemptID\` identifies
that generation. \`Messages\` contains only normal history and completed steps;
it does not contain \`PartialText\`. \`Stream\` still contains the observed events.
The string-returning prompt methods keep their existing error behavior.

\`\`\`go
result, err := host.PromptResult(ctx, "Explain the change.")
if err != nil {
    if result != nil && result.PartialText != "" {
        fmt.Printf("Incomplete output (%s): %s\\n", result.AttemptID, result.PartialText)
    }
    return err
}
fmt.Println(result.Response)
\`\`\`

The default session backend saves unfinished text as an \`incomplete_output\`
JSONL tree entry with \`attempt_id\`, \`text\`, and \`incomplete: true\`. This entry
is available in the session tree, but \`GetMessages\` and \`BuildContext\` exclude
it from model history. The UI keeps the text and marks it as incomplete, also
when the session is loaded again. This records received text after an orderly
error or cancellation; it does not save every token before a process crash.

\`SessionManager\` has no new required methods. Custom backends can implement
\`IncompleteOutputAppender\` to save these transcript records. Backends without
this capability still return partial text through \`TurnResult\`.

Session write errors now return to the caller. A failed prompt write prevents
generation. A failed step write stops the turn and does not count as a confirmed
write. Kit does not retry a failed write in the same turn. With the legacy
per-message fallback, earlier successful writes remain confirmed; implement
\`StepAppender\` for atomic storage of tool call/result pairs.

Kit gives optional context-aware writes a detached five-second deadline and
keeps the turn's context values. Backends must honor that deadline. Legacy
\`AppendMessage\` has no context, and local filesystem calls cannot be interrupted
safely; these writes remain synchronous. Kit does not start a background write
that could change the session after the caller starts another turn.

See the [StepAppender contract](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#StepAppender) for atomicity and cancellation requirements.
`};export{e as default};
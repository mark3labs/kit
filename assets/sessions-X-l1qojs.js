const s={frontmatter:{title:"SDK Sessions",description:"Session management in the Kit Go SDK.",hidden:!1,toc:!0,draft:!1},html:`<h1 id="sdk-sessions"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#sdk-sessions"><span class="icon icon-link"></span></a>SDK Sessions</h1>
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
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// Custom session directory</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">host, _ </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> kit.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">New</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(ctx, </span><span style="color:#D73A49;--shiki-dark:#F97583">&amp;</span><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Options</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    SessionDir: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"/custom/sessions/"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">,</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span></code></pre>
<h2 id="clearing-history"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#clearing-history"><span class="icon icon-link"></span></a>Clearing history</h2>
<p>Clear the in-memory conversation history (does not delete the session file):</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ClearSession</span><span style="color:#24292E;--shiki-dark:#E1E4E8">()</span></span></code></pre>
<h2 id="tree-based-sessions"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#tree-based-sessions"><span class="icon icon-link"></span></a>Tree-based sessions</h2>
<p>Kit's session model is tree-based, supporting branching. You can branch from any entry to explore alternate conversation paths:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// Access the tree session manager</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">ts </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">GetTreeSession</span><span style="color:#24292E;--shiki-dark:#E1E4E8">()</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// Branch from a specific entry</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">err </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> host.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Branch</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"entry-id-123"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span></code></pre>
<h2 id="listing-and-managing-sessions"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#listing-and-managing-sessions"><span class="icon icon-link"></span></a>Listing and managing sessions</h2>
<p>Package-level functions for session discovery:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// List sessions for a specific directory</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">sessions </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> kit.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ListSessions</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"/home/user/project"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// List all sessions across all directories</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">all </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> kit.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ListAllSessions</span><span style="color:#24292E;--shiki-dark:#E1E4E8">()</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">// Delete a session file</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">kit.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">DeleteSession</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"/path/to/session.jsonl"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span></code></pre>`,headings:[{depth:2,text:"Automatic persistence",id:"automatic-persistence"},{depth:2,text:"Accessing session info",id:"accessing-session-info"},{depth:2,text:"Configuring sessions via Options",id:"configuring-sessions-via-options"},{depth:2,text:"Clearing history",id:"clearing-history"},{depth:2,text:"Tree-based sessions",id:"tree-based-sessions"},{depth:2,text:"Listing and managing sessions",id:"listing-and-managing-sessions"}],raw:`
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

// Custom session directory
host, _ := kit.New(ctx, &kit.Options{
    SessionDir: "/custom/sessions/",
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
ts := host.GetTreeSession()

// Branch from a specific entry
err := host.Branch("entry-id-123")
\`\`\`

## Listing and managing sessions

Package-level functions for session discovery:

\`\`\`go
// List sessions for a specific directory
sessions := kit.ListSessions("/home/user/project")

// List all sessions across all directories
all := kit.ListAllSessions()

// Delete a session file
kit.DeleteSession("/path/to/session.jsonl")
\`\`\`
`};export{s as default};

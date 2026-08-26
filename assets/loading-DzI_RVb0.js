var e={frontmatter:{title:`Loading Extensions`,description:`How Kit discovers and loads extensions.`,hidden:!1,toc:!0,draft:!1},html:`<h1 id="loading-extensions"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#loading-extensions"><span class="icon icon-link"></span></a>Loading Extensions</h1>
<h2 id="auto-discovery"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#auto-discovery"><span class="icon icon-link"></span></a>Auto-discovery</h2>
<p>Kit automatically discovers and loads extensions from these paths, in order:</p>
<table>
<thead>
<tr>
<th>Path</th>
<th>Scope</th>
</tr>
</thead>
<tbody>
<tr>
<td><code>/usr/share/kit/extensions/*.go</code></td>
<td>System-wide single files</td>
</tr>
<tr>
<td><code>/usr/share/kit/extensions/*/main.go</code></td>
<td>System-wide subdirectory extensions</td>
</tr>
<tr>
<td><code>~/.config/kit/extensions/*.go</code></td>
<td>User single files</td>
</tr>
<tr>
<td><code>~/.config/kit/extensions/*/main.go</code></td>
<td>User subdirectory extensions</td>
</tr>
<tr>
<td><code>~/.local/share/kit/git/</code></td>
<td>Global git-installed packages</td>
</tr>
<tr>
<td><code>.kit/extensions/*.go</code></td>
<td>Project-local single files</td>
</tr>
<tr>
<td><code>.kit/extensions/*/main.go</code></td>
<td>Project-local subdirectory extensions</td>
</tr>
<tr>
<td><code>.kit/git/</code></td>
<td>Project-local git-installed packages</td>
</tr>
</tbody>
</table>
<p>Kit loads every extension it finds. The order in the table is the order in
which extensions load and in which their event handlers run, thus a
project-local extension runs after a user one, and a user extension runs
after a system-wide one.</p>
<h2 id="system-wide-extensions"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#system-wide-extensions"><span class="icon icon-link"></span></a>System-wide extensions</h2>
<p>The system-wide directory holds extensions that come with a packaged
install of Kit (rpm, deb, Homebrew, and so on) and are shared by every
user of the machine. It defaults to <code>/usr/share/kit/extensions</code>.</p>
<p>Set <code>KIT_SYSTEM_EXTENSIONS_DIR</code> to use different directories. Give more
than one directory with the platform list separator (<code>:</code> on Unix, <code>;</code> on
Windows):</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D"># Unix</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">KIT_SYSTEM_EXTENSIONS_DIR</span><span style="color:#D73A49;--shiki-dark:#F97583">=</span><span style="color:#032F62;--shiki-dark:#9ECBFF">/opt/kit/extensions:/srv/kit/extensions</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> kit</span></span></code></pre>
<pre class="tome-code" data-lang="powershell"><code># Windows (PowerShell)
$env:KIT_SYSTEM_EXTENSIONS_DIR = "C:\\ProgramData\\kit\\extensions;D:\\kit\\extensions"; kit
</code></pre>
<p>Set the variable to an empty value to turn off system-wide discovery:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">KIT_SYSTEM_EXTENSIONS_DIR</span><span style="color:#D73A49;--shiki-dark:#F97583">=</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> kit</span></span></code></pre>
<p>Packagers can change the compiled-in default at build time:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">go</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> build</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> -ldflags</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> \\</span></span>
<span class="line"><span style="color:#032F62;--shiki-dark:#9ECBFF">  "-X github.com/mark3labs/kit/internal/extensions.SystemExtensionsDir=/opt/kit/extensions"</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> \\</span></span>
<span class="line"><span style="color:#032F62;--shiki-dark:#9ECBFF">  ./cmd/kit</span></span></code></pre>
<h2 id="explicit-loading"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#explicit-loading"><span class="icon icon-link"></span></a>Explicit loading</h2>
<p>Load extensions by path using the <code>-e</code> flag:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> -e</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> path/to/extension.go</span></span></code></pre>
<p>Load multiple extensions:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> -e</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> ext1.go</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> -e</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> ext2.go</span></span></code></pre>
<h2 id="disabling-extensions"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#disabling-extensions"><span class="icon icon-link"></span></a>Disabling extensions</h2>
<p>Disable all auto-discovered extensions:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> --no-extensions</span></span></code></pre>
<p>You can combine <code>--no-extensions</code> with <code>-e</code> to load only specific extensions:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> --no-extensions</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> -e</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> my-extension.go</span></span></code></pre>
<h2 id="installing-from-git"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#installing-from-git"><span class="icon icon-link"></span></a>Installing from git</h2>
<p>Install extensions from git repositories using <code>kit install</code>:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D"># Install globally (to ~/.local/share/kit/git/)</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> install</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> https://github.com/user/my-kit-extension.git</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D"># Install project-locally (to .kit/git/)</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> install</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> -l</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> https://github.com/user/my-kit-extension.git</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D"># Update an installed package</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> install</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> -u</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> https://github.com/user/my-kit-extension.git</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D"># Remove</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> install</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> --uninstall</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> my-kit-extension</span></span></code></pre>
<h2 id="extension-structure"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#extension-structure"><span class="icon icon-link"></span></a>Extension structure</h2>
<h3 id="single-file-extensions"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#single-file-extensions"><span class="icon icon-link"></span></a>Single-file extensions</h3>
<p>A single <code>.go</code> file with an <code>Init</code> function:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">//go:build ignore</span></span>
<span class="line"></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">package</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> main</span></span>
<span class="line"></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">import</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> "</span><span style="color:#6F42C1;--shiki-dark:#B392F0">kit/ext</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"</span></span>
<span class="line"></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">func</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> Init</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#E36209;--shiki-dark:#FFAB70">api</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">API</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) {</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">    // register handlers, tools, commands, etc.</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">}</span></span></code></pre>
<p>The <code>//go:build ignore</code> directive prevents the Go toolchain from trying to compile the file as part of a normal build.</p>
<h3 id="subdirectory-extensions"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#subdirectory-extensions"><span class="icon icon-link"></span></a>Subdirectory extensions</h3>
<p>For more complex extensions, create a directory with a <code>main.go</code> entry point:</p>
<pre><code>.kit/extensions/my-extension/
├── main.go      # Must contain Init(api ext.API)
├── helpers.go   # Additional source files
└── config.go
</code></pre>
<h3 id="package-level-state"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#package-level-state"><span class="icon icon-link"></span></a>Package-level state</h3>
<p>Yaegi supports package-level variables captured in closures. This is the standard way to maintain state across event callbacks:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">package</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> main</span></span>
<span class="line"></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">import</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> "</span><span style="color:#6F42C1;--shiki-dark:#B392F0">kit/ext</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"</span></span>
<span class="line"></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">var</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> callCount </span><span style="color:#D73A49;--shiki-dark:#F97583">int</span></span>
<span class="line"></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">func</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> Init</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#E36209;--shiki-dark:#FFAB70">api</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">API</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) {</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    api.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">OnToolCall</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#D73A49;--shiki-dark:#F97583">func</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#E36209;--shiki-dark:#FFAB70">_</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ToolCallEvent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#E36209;--shiki-dark:#FFAB70">ctx</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Context</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) {</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">        callCount</span><span style="color:#D73A49;--shiki-dark:#F97583">++</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">        ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">SetFooter</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">HeaderFooterConfig</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">            Content: </span><span style="color:#6F42C1;--shiki-dark:#B392F0">ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">WidgetContent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">                Text: fmt.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Sprintf</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"Tools called: </span><span style="color:#005CC5;--shiki-dark:#79B8FF">%d</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, callCount),</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">            },</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">        })</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    })</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">}</span></span></code></pre>
<h3 id="standard-library-access"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#standard-library-access"><span class="icon icon-link"></span></a>Standard library access</h3>
<p>Extensions can import the full Go standard library, plus <code>os/exec</code> for spawning
subprocesses. Environment variables are also readable: <code>os.Getenv</code>,
<code>os.LookupEnv</code>, and <code>os.Environ</code> return Kit's process environment, so extensions
can pick up CI-provided variables (for example <code>GITHUB_EVENT_PATH</code> or a provider
API key) and any vars the user exported before launching Kit.</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">package</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> main</span></span>
<span class="line"></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">import</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> (</span></span>
<span class="line"><span style="color:#032F62;--shiki-dark:#9ECBFF">    "</span><span style="color:#6F42C1;--shiki-dark:#B392F0">os</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"</span></span>
<span class="line"><span style="color:#032F62;--shiki-dark:#9ECBFF">    "</span><span style="color:#6F42C1;--shiki-dark:#B392F0">kit/ext</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span>
<span class="line"></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">func</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> Init</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#E36209;--shiki-dark:#FFAB70">api</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">API</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) {</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    api.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">OnSessionStart</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#D73A49;--shiki-dark:#F97583">func</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#E36209;--shiki-dark:#FFAB70">_</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">SessionStartEvent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#E36209;--shiki-dark:#FFAB70">ctx</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Context</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) {</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">        if</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> eventPath </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> os.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Getenv</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"GITHUB_EVENT_PATH"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">); eventPath </span><span style="color:#D73A49;--shiki-dark:#F97583">!=</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> ""</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> {</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">            ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">PrintInfo</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"Running in GitHub Actions: "</span><span style="color:#D73A49;--shiki-dark:#F97583"> +</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> eventPath)</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">        }</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    })</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">}</span></span></code></pre>
<p>Environment access is read-only from the host's perspective: the environment is
snapshotted when the extension loads, and calls to <code>os.Setenv</code> mutate only the
extension's sandboxed copy — they never change Kit's process environment or the
host. This keeps extensions from leaking state into Kit or other extensions
while still letting them read the configuration they need.</p>
<h3 id="failure-isolation"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#failure-isolation"><span class="icon icon-link"></span></a>Failure isolation</h3>
<p>Each extension is loaded into its own interpreter, and a failure in one never
stops the others. A syntax error, a bad <code>Init</code> signature, a panic while the
source is evaluated, or a panic inside <code>Init</code> is reported as a load failure for
that file only — Kit continues starting up with the remaining extensions.</p>
<p>At runtime the same applies to widget rendering: a panic inside a
<a href="/extensions/capabilities#custom-rendering"><code>Render</code> callback</a> hides that widget
and logs the error rather than taking down the TUI.</p>
<p>This is isolation, not sandboxing. Extensions run in-process with <code>os/exec</code>
access, so only load a <code>.go</code> file you would be willing to run directly.</p>`,headings:[{depth:2,text:`Auto-discovery`,id:`auto-discovery`},{depth:2,text:`System-wide extensions`,id:`system-wide-extensions`},{depth:2,text:`Explicit loading`,id:`explicit-loading`},{depth:2,text:`Disabling extensions`,id:`disabling-extensions`},{depth:2,text:`Installing from git`,id:`installing-from-git`},{depth:2,text:`Extension structure`,id:`extension-structure`},{depth:3,text:`Single-file extensions`,id:`single-file-extensions`},{depth:3,text:`Subdirectory extensions`,id:`subdirectory-extensions`},{depth:3,text:`Package-level state`,id:`package-level-state`},{depth:3,text:`Standard library access`,id:`standard-library-access`},{depth:3,text:`Failure isolation`,id:`failure-isolation`}],raw:`
# Loading Extensions

## Auto-discovery

Kit automatically discovers and loads extensions from these paths, in order:

| Path | Scope |
|------|-------|
| \`/usr/share/kit/extensions/*.go\` | System-wide single files |
| \`/usr/share/kit/extensions/*/main.go\` | System-wide subdirectory extensions |
| \`~/.config/kit/extensions/*.go\` | User single files |
| \`~/.config/kit/extensions/*/main.go\` | User subdirectory extensions |
| \`~/.local/share/kit/git/\` | Global git-installed packages |
| \`.kit/extensions/*.go\` | Project-local single files |
| \`.kit/extensions/*/main.go\` | Project-local subdirectory extensions |
| \`.kit/git/\` | Project-local git-installed packages |

Kit loads every extension it finds. The order in the table is the order in
which extensions load and in which their event handlers run, thus a
project-local extension runs after a user one, and a user extension runs
after a system-wide one.

## System-wide extensions

The system-wide directory holds extensions that come with a packaged
install of Kit (rpm, deb, Homebrew, and so on) and are shared by every
user of the machine. It defaults to \`/usr/share/kit/extensions\`.

Set \`KIT_SYSTEM_EXTENSIONS_DIR\` to use different directories. Give more
than one directory with the platform list separator (\`:\` on Unix, \`;\` on
Windows):

\`\`\`bash
# Unix
KIT_SYSTEM_EXTENSIONS_DIR=/opt/kit/extensions:/srv/kit/extensions kit
\`\`\`

\`\`\`powershell
# Windows (PowerShell)
$env:KIT_SYSTEM_EXTENSIONS_DIR = "C:\\ProgramData\\kit\\extensions;D:\\kit\\extensions"; kit
\`\`\`

Set the variable to an empty value to turn off system-wide discovery:

\`\`\`bash
KIT_SYSTEM_EXTENSIONS_DIR= kit
\`\`\`

Packagers can change the compiled-in default at build time:

\`\`\`bash
go build -ldflags \\
  "-X github.com/mark3labs/kit/internal/extensions.SystemExtensionsDir=/opt/kit/extensions" \\
  ./cmd/kit
\`\`\`

## Explicit loading

Load extensions by path using the \`-e\` flag:

\`\`\`bash
kit -e path/to/extension.go
\`\`\`

Load multiple extensions:

\`\`\`bash
kit -e ext1.go -e ext2.go
\`\`\`

## Disabling extensions

Disable all auto-discovered extensions:

\`\`\`bash
kit --no-extensions
\`\`\`

You can combine \`--no-extensions\` with \`-e\` to load only specific extensions:

\`\`\`bash
kit --no-extensions -e my-extension.go
\`\`\`

## Installing from git

Install extensions from git repositories using \`kit install\`:

\`\`\`bash
# Install globally (to ~/.local/share/kit/git/)
kit install https://github.com/user/my-kit-extension.git

# Install project-locally (to .kit/git/)
kit install -l https://github.com/user/my-kit-extension.git

# Update an installed package
kit install -u https://github.com/user/my-kit-extension.git

# Remove
kit install --uninstall my-kit-extension
\`\`\`

## Extension structure

### Single-file extensions

A single \`.go\` file with an \`Init\` function:

\`\`\`go
//go:build ignore

package main

import "kit/ext"

func Init(api ext.API) {
    // register handlers, tools, commands, etc.
}
\`\`\`

The \`//go:build ignore\` directive prevents the Go toolchain from trying to compile the file as part of a normal build.

### Subdirectory extensions

For more complex extensions, create a directory with a \`main.go\` entry point:

\`\`\`
.kit/extensions/my-extension/
├── main.go      # Must contain Init(api ext.API)
├── helpers.go   # Additional source files
└── config.go
\`\`\`

### Package-level state

Yaegi supports package-level variables captured in closures. This is the standard way to maintain state across event callbacks:

\`\`\`go
package main

import "kit/ext"

var callCount int

func Init(api ext.API) {
    api.OnToolCall(func(_ ext.ToolCallEvent, ctx ext.Context) {
        callCount++
        ctx.SetFooter(ext.HeaderFooterConfig{
            Content: ext.WidgetContent{
                Text: fmt.Sprintf("Tools called: %d", callCount),
            },
        })
    })
}
\`\`\`

### Standard library access

Extensions can import the full Go standard library, plus \`os/exec\` for spawning
subprocesses. Environment variables are also readable: \`os.Getenv\`,
\`os.LookupEnv\`, and \`os.Environ\` return Kit's process environment, so extensions
can pick up CI-provided variables (for example \`GITHUB_EVENT_PATH\` or a provider
API key) and any vars the user exported before launching Kit.

\`\`\`go
package main

import (
    "os"
    "kit/ext"
)

func Init(api ext.API) {
    api.OnSessionStart(func(_ ext.SessionStartEvent, ctx ext.Context) {
        if eventPath := os.Getenv("GITHUB_EVENT_PATH"); eventPath != "" {
            ctx.PrintInfo("Running in GitHub Actions: " + eventPath)
        }
    })
}
\`\`\`

Environment access is read-only from the host's perspective: the environment is
snapshotted when the extension loads, and calls to \`os.Setenv\` mutate only the
extension's sandboxed copy — they never change Kit's process environment or the
host. This keeps extensions from leaking state into Kit or other extensions
while still letting them read the configuration they need.

### Failure isolation

Each extension is loaded into its own interpreter, and a failure in one never
stops the others. A syntax error, a bad \`Init\` signature, a panic while the
source is evaluated, or a panic inside \`Init\` is reported as a load failure for
that file only — Kit continues starting up with the remaining extensions.

At runtime the same applies to widget rendering: a panic inside a
[\`Render\` callback](/extensions/capabilities#custom-rendering) hides that widget
and logs the error rather than taking down the TUI.

This is isolation, not sandboxing. Extensions run in-process with \`os/exec\`
access, so only load a \`.go\` file you would be willing to run directly.
`};export{e as default};
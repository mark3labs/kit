const s={frontmatter:{title:"Loading Extensions",description:"How Kit discovers and loads extensions.",hidden:!1,toc:!0,draft:!1},html:`<h1 id="loading-extensions"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#loading-extensions"><span class="icon icon-link"></span></a>Loading Extensions</h1>
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
<td><code>~/.config/kit/extensions/*.go</code></td>
<td>Global single files</td>
</tr>
<tr>
<td><code>~/.config/kit/extensions/*/main.go</code></td>
<td>Global subdirectory extensions</td>
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
<td><code>~/.local/share/kit/git/</code></td>
<td>Global git-installed packages</td>
</tr>
<tr>
<td><code>.kit/git/</code></td>
<td>Project-local git-installed packages</td>
</tr>
</tbody>
</table>
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
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">}</span></span></code></pre>`,headings:[{depth:2,text:"Auto-discovery",id:"auto-discovery"},{depth:2,text:"Explicit loading",id:"explicit-loading"},{depth:2,text:"Disabling extensions",id:"disabling-extensions"},{depth:2,text:"Installing from git",id:"installing-from-git"},{depth:2,text:"Extension structure",id:"extension-structure"},{depth:3,text:"Single-file extensions",id:"single-file-extensions"},{depth:3,text:"Subdirectory extensions",id:"subdirectory-extensions"},{depth:3,text:"Package-level state",id:"package-level-state"}],raw:`
# Loading Extensions

## Auto-discovery

Kit automatically discovers and loads extensions from these paths, in order:

| Path | Scope |
|------|-------|
| \`~/.config/kit/extensions/*.go\` | Global single files |
| \`~/.config/kit/extensions/*/main.go\` | Global subdirectory extensions |
| \`.kit/extensions/*.go\` | Project-local single files |
| \`.kit/extensions/*/main.go\` | Project-local subdirectory extensions |
| \`~/.local/share/kit/git/\` | Global git-installed packages |
| \`.kit/git/\` | Project-local git-installed packages |

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
`};export{s as default};

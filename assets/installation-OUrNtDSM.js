var e={frontmatter:{title:`Installation`,description:`Install Kit using the install script, npm, bun, pnpm, Go, or build from source.`,hidden:!1,toc:!0,draft:!1},html:`<h1 id="installation"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#installation"><span class="icon icon-link"></span></a>Installation</h1>
<h2 id="using-the-install-script-macos--linux"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#using-the-install-script-macos--linux"><span class="icon icon-link"></span></a>Using the install script (macOS / Linux)</h2>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">curl</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> -fsSL</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> https://raw.githubusercontent.com/mark3labs/kit/master/install.sh</span><span style="color:#D73A49;--shiki-dark:#F97583"> |</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> bash</span></span></code></pre>
<p>The script:</p>
<ul>
<li>Detects your OS (<code>linux</code>, <code>darwin</code>) and architecture (<code>amd64</code>, <code>arm64</code>)</li>
<li>Downloads the matching binary from the latest <a href="https://github.com/mark3labs/kit/releases">GitHub release</a></li>
<li>Verifies the SHA-256 checksum against <code>checksums.txt</code> and stops if it does not match</li>
<li>Installs <code>kit</code> into the first writable directory of <code>~/.local/bin</code>, <code>~/bin</code>, or <code>/usr/local/bin</code> (it creates <code>~/.local/bin</code> if none exist)</li>
<li>Tells you if the install directory is not on your <code>$PATH</code></li>
</ul>
<p>To pass options, use <code>bash -s --</code>:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">curl</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> -fsSL</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> https://raw.githubusercontent.com/mark3labs/kit/master/install.sh</span><span style="color:#D73A49;--shiki-dark:#F97583"> |</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> bash</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> -s</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> --</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> --version</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> v0.111.0</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> --bin-dir</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> /usr/local/bin</span></span></code></pre>
<table>
<thead>
<tr>
<th>Option</th>
<th>Environment variable</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td><code>--bin-dir &lt;dir&gt;</code></td>
<td><code>KIT_BIN_DIR</code></td>
<td>Directory to install <code>kit</code> into</td>
</tr>
<tr>
<td><code>--version &lt;tag&gt;</code></td>
<td><code>KIT_VERSION</code></td>
<td>Release tag to install (default: latest)</td>
</tr>
<tr>
<td><code>--no-provider-check</code></td>
<td></td>
<td>Skip the LLM provider API key reminder</td>
</tr>
<tr>
<td><code>-h</code>, <code>--help</code></td>
<td></td>
<td>Show help</td>
</tr>
</tbody>
</table>
<p>On Windows, use npm / bun / pnpm below, or download the <code>.zip</code> from the <a href="https://github.com/mark3labs/kit/releases">releases page</a>.</p>
<h2 id="using-npm--bun--pnpm"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#using-npm--bun--pnpm"><span class="icon icon-link"></span></a>Using npm / bun / pnpm</h2>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">npm</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> install</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> -g</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> @mark3labs/kit</span></span></code></pre>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">bun</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> install</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> -g</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> @mark3labs/kit</span></span></code></pre>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">pnpm</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> install</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> -g</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> @mark3labs/kit</span></span></code></pre>
<h2 id="using-go"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#using-go"><span class="icon icon-link"></span></a>Using Go</h2>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">go</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> install</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> github.com/mark3labs/kit/cmd/kit@latest</span></span></code></pre>
<h2 id="building-from-source"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#building-from-source"><span class="icon icon-link"></span></a>Building from source</h2>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">git</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> clone</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> https://github.com/mark3labs/kit.git</span></span>
<span class="line"><span style="color:#005CC5;--shiki-dark:#79B8FF">cd</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> kit</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">go</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> build</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> -o</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> ./cmd/kit</span></span></code></pre>
<h2 id="verifying-the-installation"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#verifying-the-installation"><span class="icon icon-link"></span></a>Verifying the installation</h2>
<p>After installing, verify Kit is available:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> --help</span></span></code></pre>
<h2 id="setting-up-a-provider"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#setting-up-a-provider"><span class="icon icon-link"></span></a>Setting up a provider</h2>
<p>Kit needs at least one LLM provider configured. Set an API key for your preferred provider:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D"># Anthropic (default provider)</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">export</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> ANTHROPIC_API_KEY</span><span style="color:#D73A49;--shiki-dark:#F97583">=</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"sk-..."</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D"># OpenAI</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">export</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> OPENAI_API_KEY</span><span style="color:#D73A49;--shiki-dark:#F97583">=</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"sk-..."</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D"># Google Gemini</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">export</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> GOOGLE_API_KEY</span><span style="color:#D73A49;--shiki-dark:#F97583">=</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"..."</span></span></code></pre>
<p>For OAuth-enabled providers like Anthropic, you can also authenticate interactively:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> auth</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> login</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> anthropic</span></span></code></pre>
<p>See <a href="/providers">Providers</a> for the full list of supported providers and their configuration.</p>`,headings:[{depth:2,text:`Using the install script (macOS / Linux)`,id:`using-the-install-script-macos--linux`},{depth:2,text:`Using npm / bun / pnpm`,id:`using-npm--bun--pnpm`},{depth:2,text:`Using Go`,id:`using-go`},{depth:2,text:`Building from source`,id:`building-from-source`},{depth:2,text:`Verifying the installation`,id:`verifying-the-installation`},{depth:2,text:`Setting up a provider`,id:`setting-up-a-provider`}],raw:'\n# Installation\n\n## Using the install script (macOS / Linux)\n\n```bash\ncurl -fsSL https://raw.githubusercontent.com/mark3labs/kit/master/install.sh | bash\n```\n\nThe script:\n\n- Detects your OS (`linux`, `darwin`) and architecture (`amd64`, `arm64`)\n- Downloads the matching binary from the latest [GitHub release](https://github.com/mark3labs/kit/releases)\n- Verifies the SHA-256 checksum against `checksums.txt` and stops if it does not match\n- Installs `kit` into the first writable directory of `~/.local/bin`, `~/bin`, or `/usr/local/bin` (it creates `~/.local/bin` if none exist)\n- Tells you if the install directory is not on your `$PATH`\n\nTo pass options, use `bash -s --`:\n\n```bash\ncurl -fsSL https://raw.githubusercontent.com/mark3labs/kit/master/install.sh | bash -s -- --version v0.111.0 --bin-dir /usr/local/bin\n```\n\n| Option | Environment variable | Description |\n|--------|----------------------|-------------|\n| `--bin-dir <dir>` | `KIT_BIN_DIR` | Directory to install `kit` into |\n| `--version <tag>` | `KIT_VERSION` | Release tag to install (default: latest) |\n| `--no-provider-check` | | Skip the LLM provider API key reminder |\n| `-h`, `--help` | | Show help |\n\nOn Windows, use npm / bun / pnpm below, or download the `.zip` from the [releases page](https://github.com/mark3labs/kit/releases).\n\n## Using npm / bun / pnpm\n\n```bash\nnpm install -g @mark3labs/kit\n```\n\n```bash\nbun install -g @mark3labs/kit\n```\n\n```bash\npnpm install -g @mark3labs/kit\n```\n\n## Using Go\n\n```bash\ngo install github.com/mark3labs/kit/cmd/kit@latest\n```\n\n## Building from source\n\n```bash\ngit clone https://github.com/mark3labs/kit.git\ncd kit\ngo build -o kit ./cmd/kit\n```\n\n## Verifying the installation\n\nAfter installing, verify Kit is available:\n\n```bash\nkit --help\n```\n\n## Setting up a provider\n\nKit needs at least one LLM provider configured. Set an API key for your preferred provider:\n\n```bash\n# Anthropic (default provider)\nexport ANTHROPIC_API_KEY="sk-..."\n\n# OpenAI\nexport OPENAI_API_KEY="sk-..."\n\n# Google Gemini\nexport GOOGLE_API_KEY="..."\n```\n\nFor OAuth-enabled providers like Anthropic, you can also authenticate interactively:\n\n```bash\nkit auth login anthropic\n```\n\nSee [Providers](/providers) for the full list of supported providers and their configuration.\n'};export{e as default};
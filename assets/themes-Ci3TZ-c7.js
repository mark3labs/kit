const s={frontmatter:{title:"Themes",description:"Customize Kit's appearance with built-in themes, custom theme files, and the extension theme API.",hidden:!1,toc:!0,draft:!1},html:`<h1 id="themes"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#themes"><span class="icon icon-link"></span></a>Themes</h1>
<p>Kit ships with 22 built-in color themes and supports custom themes via YAML/JSON files or the extension API. Themes control all UI colors: input borders, popups, system messages, markdown rendering, syntax highlighting, and diff displays.</p>
<h2 id="quick-start"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#quick-start"><span class="icon icon-link"></span></a>Quick start</h2>
<p>Switch themes at runtime with the <code>/theme</code> command:</p>
<pre><code>/theme dracula
/theme catppuccin
/theme kitt
</code></pre>
<p>Run <code>/theme</code> with no arguments to list all available themes.</p>
<p><strong>Theme selections are automatically saved</strong> to <code>~/.config/kit/preferences.yml</code> and restored on next launch. You don't need to add anything to your config file — just <code>/theme &lt;name&gt;</code> and it sticks.</p>
<h2 id="built-in-themes"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#built-in-themes"><span class="icon icon-link"></span></a>Built-in themes</h2>
<table>
<thead>
<tr>
<th>Theme</th>
<th>Style</th>
</tr>
</thead>
<tbody>
<tr>
<td><code>kitt</code></td>
<td>KITT-inspired reds and ambers (default)</td>
</tr>
<tr>
<td><code>catppuccin</code></td>
<td>Soothing pastels (Mocha/Latte)</td>
</tr>
<tr>
<td><code>dracula</code></td>
<td>Purple and cyan dark theme</td>
</tr>
<tr>
<td><code>tokyonight</code></td>
<td>Cool blues with warm accents</td>
</tr>
<tr>
<td><code>nord</code></td>
<td>Arctic, north-bluish palette</td>
</tr>
<tr>
<td><code>gruvbox</code></td>
<td>Retro groove colors</td>
</tr>
<tr>
<td><code>monokai</code></td>
<td>Classic syntax theme</td>
</tr>
<tr>
<td><code>solarized</code></td>
<td>Precision colors for machines and people</td>
</tr>
<tr>
<td><code>github</code></td>
<td>GitHub's light and dark palettes</td>
</tr>
<tr>
<td><code>one-dark</code></td>
<td>Atom One Dark</td>
</tr>
<tr>
<td><code>rose-pine</code></td>
<td>Soho vibes with muted tones</td>
</tr>
<tr>
<td><code>ayu</code></td>
<td>Simple with bright colors</td>
</tr>
<tr>
<td><code>material</code></td>
<td>Material Design palette</td>
</tr>
<tr>
<td><code>everforest</code></td>
<td>Green-focused comfortable theme</td>
</tr>
<tr>
<td><code>kanagawa</code></td>
<td>Dark theme inspired by Katsushika Hokusai</td>
</tr>
<tr>
<td><code>amoled</code></td>
<td>Pure black background, vivid accents</td>
</tr>
<tr>
<td><code>synthwave</code></td>
<td>Retro neon glows</td>
</tr>
<tr>
<td><code>vesper</code></td>
<td>Warm minimalist dark theme</td>
</tr>
<tr>
<td><code>flexoki</code></td>
<td>Inky reading palette</td>
</tr>
<tr>
<td><code>matrix</code></td>
<td>Green-on-black terminal aesthetic</td>
</tr>
<tr>
<td><code>vercel</code></td>
<td>Clean monochrome with blue accents</td>
</tr>
<tr>
<td><code>zenburn</code></td>
<td>Low-contrast, warm dark theme</td>
</tr>
</tbody>
</table>
<p>All themes support both light and dark terminal modes via adaptive colors.</p>
<h2 id="custom-theme-files"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#custom-theme-files"><span class="icon icon-link"></span></a>Custom theme files</h2>
<p>Create a <code>.yml</code>, <code>.yaml</code>, or <code>.json</code> file with color definitions. Kit discovers themes from two directories:</p>
<table>
<thead>
<tr>
<th>Location</th>
<th>Scope</th>
<th>Precedence</th>
</tr>
</thead>
<tbody>
<tr>
<td><code>~/.config/kit/themes/</code></td>
<td>User (global)</td>
<td>Overrides built-ins</td>
</tr>
<tr>
<td><code>.kit/themes/</code></td>
<td>Project-local</td>
<td>Overrides user and built-ins</td>
</tr>
</tbody>
</table>
<h3 id="theme-file-format"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#theme-file-format"><span class="icon icon-link"></span></a>Theme file format</h3>
<p>A theme file defines adaptive color pairs with <code>light</code> and <code>dark</code> hex values. Any field left empty inherits from the default KITT theme.</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D"># ~/.config/kit/themes/my-theme.yml</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D"># Core semantic colors</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">primary</span><span style="color:#24292E;--shiki-dark:#E1E4E8">:</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">  light</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#8839ef"</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">  dark</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#cba6f7"</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">secondary</span><span style="color:#24292E;--shiki-dark:#E1E4E8">:</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">  light</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#04a5e5"</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">  dark</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#89dceb"</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">success</span><span style="color:#24292E;--shiki-dark:#E1E4E8">:</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">  light</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#40a02b"</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">  dark</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#a6e3a1"</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">warning</span><span style="color:#24292E;--shiki-dark:#E1E4E8">:</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">  light</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#df8e1d"</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">  dark</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#f9e2af"</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">error</span><span style="color:#24292E;--shiki-dark:#E1E4E8">:</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">  light</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#d20f39"</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">  dark</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#f38ba8"</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">info</span><span style="color:#24292E;--shiki-dark:#E1E4E8">:</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">  light</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#1e66f5"</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">  dark</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#89b4fa"</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D"># Text and chrome</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">text</span><span style="color:#24292E;--shiki-dark:#E1E4E8">:</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">  light</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#4c4f69"</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">  dark</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#cdd6f4"</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">muted</span><span style="color:#24292E;--shiki-dark:#E1E4E8">:</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">  light</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#6c6f85"</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">  dark</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#a6adc8"</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">very-muted</span><span style="color:#24292E;--shiki-dark:#E1E4E8">:</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">  light</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#9ca0b0"</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">  dark</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#6c7086"</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">background</span><span style="color:#24292E;--shiki-dark:#E1E4E8">:</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">  light</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#eff1f5"</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">  dark</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#1e1e2e"</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">border</span><span style="color:#24292E;--shiki-dark:#E1E4E8">:</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">  light</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#acb0be"</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">  dark</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#585b70"</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">muted-border</span><span style="color:#24292E;--shiki-dark:#E1E4E8">:</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">  light</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#ccd0da"</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">  dark</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#313244"</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D"># Semantic roles</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">system</span><span style="color:#24292E;--shiki-dark:#E1E4E8">:</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">  light</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#179299"</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">  dark</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#94e2d5"</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">tool</span><span style="color:#24292E;--shiki-dark:#E1E4E8">:</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">  light</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#fe640b"</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">  dark</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#fab387"</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">accent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">:</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">  light</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#ea76cb"</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">  dark</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#f5c2e7"</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">highlight</span><span style="color:#24292E;--shiki-dark:#E1E4E8">:</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">  light</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#e6e9ef"</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">  dark</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#181825"</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D"># Diff backgrounds</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">diff-insert-bg</span><span style="color:#24292E;--shiki-dark:#E1E4E8">:</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">  light</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#d5f0d5"</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">  dark</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#1a3a2a"</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">diff-delete-bg</span><span style="color:#24292E;--shiki-dark:#E1E4E8">:</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">  light</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#f5d5d5"</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">  dark</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#3a1a2a"</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">diff-equal-bg</span><span style="color:#24292E;--shiki-dark:#E1E4E8">:</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">  light</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#eceef3"</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">  dark</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#232336"</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">diff-missing-bg</span><span style="color:#24292E;--shiki-dark:#E1E4E8">:</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">  light</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#e4e6eb"</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">  dark</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#1a1a2e"</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D"># Code block backgrounds</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">code-bg</span><span style="color:#24292E;--shiki-dark:#E1E4E8">:</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">  light</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#eceef3"</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">  dark</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#232336"</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">gutter-bg</span><span style="color:#24292E;--shiki-dark:#E1E4E8">:</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">  light</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#e4e6eb"</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">  dark</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#1a1a2e"</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">write-bg</span><span style="color:#24292E;--shiki-dark:#E1E4E8">:</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">  light</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#d5f0d5"</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">  dark</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#1a3a2a"</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D"># Markdown and syntax highlighting</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">markdown</span><span style="color:#24292E;--shiki-dark:#E1E4E8">:</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">  heading</span><span style="color:#24292E;--shiki-dark:#E1E4E8">:</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">    light</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#1e66f5"</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">    dark</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#89b4fa"</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">  link</span><span style="color:#24292E;--shiki-dark:#E1E4E8">:</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">    light</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#1e66f5"</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">    dark</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#89b4fa"</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">  keyword</span><span style="color:#24292E;--shiki-dark:#E1E4E8">:</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">    light</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#8839ef"</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">    dark</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#cba6f7"</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">  string</span><span style="color:#24292E;--shiki-dark:#E1E4E8">:</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">    light</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#40a02b"</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">    dark</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#a6e3a1"</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">  number</span><span style="color:#24292E;--shiki-dark:#E1E4E8">:</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">    light</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#fe640b"</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">    dark</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#fab387"</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">  comment</span><span style="color:#24292E;--shiki-dark:#E1E4E8">:</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">    light</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#9ca0b0"</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">    dark</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#6c7086"</span></span></code></pre>
<h3 id="partial-themes"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#partial-themes"><span class="icon icon-link"></span></a>Partial themes</h3>
<p>You only need to define the colors you want to change. Unspecified fields fall back to the default theme:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D"># Just override the primary and accent colors</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">primary</span><span style="color:#24292E;--shiki-dark:#E1E4E8">:</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">  dark</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#FF00FF"</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">accent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">:</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">  dark</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#00FFFF"</span></span></code></pre>
<h3 id="distributing-themes"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#distributing-themes"><span class="icon icon-link"></span></a>Distributing themes</h3>
<ul>
<li><strong>Personal</strong>: Drop a file in <code>~/.config/kit/themes/</code></li>
<li><strong>Team/project</strong>: Drop a file in <code>.kit/themes/</code> and commit it to version control</li>
<li><strong>Override built-in</strong>: Name your file the same as a built-in (e.g., <code>dracula.yml</code>) and it takes precedence</li>
</ul>
<h2 id="config-file-theme"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#config-file-theme"><span class="icon icon-link"></span></a>Config file theme</h2>
<p>You can also set theme colors directly in <code>.kit.yml</code>:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">theme</span><span style="color:#24292E;--shiki-dark:#E1E4E8">:</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">  primary</span><span style="color:#24292E;--shiki-dark:#E1E4E8">:</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">    light</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#8839ef"</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">    dark</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#cba6f7"</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">  error</span><span style="color:#24292E;--shiki-dark:#E1E4E8">:</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">    dark</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#FF0000"</span></span></code></pre>
<p>Or reference an external theme file:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">theme</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"./themes/my-custom-theme.yml"</span></span></code></pre>
<h2 id="extension-theme-api"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#extension-theme-api"><span class="icon icon-link"></span></a>Extension theme API</h2>
<p>Extensions can register and switch themes programmatically at runtime.</p>
<h3 id="registering-a-theme"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#registering-a-theme"><span class="icon icon-link"></span></a>Registering a theme</h3>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">api.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">OnSessionStart</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#D73A49;--shiki-dark:#F97583">func</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#E36209;--shiki-dark:#FFAB70">_</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">SessionStartEvent</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#E36209;--shiki-dark:#FFAB70">ctx</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">Context</span><span style="color:#24292E;--shiki-dark:#E1E4E8">) {</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">RegisterTheme</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"neon"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#6F42C1;--shiki-dark:#B392F0">ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ThemeColorConfig</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">        Primary:    </span><span style="color:#6F42C1;--shiki-dark:#B392F0">ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ThemeColor</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{Light: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#CC00FF"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, Dark: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#FF00FF"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">},</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">        Secondary:  </span><span style="color:#6F42C1;--shiki-dark:#B392F0">ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ThemeColor</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{Light: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#0088CC"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, Dark: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#00FFFF"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">},</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">        Success:    </span><span style="color:#6F42C1;--shiki-dark:#B392F0">ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ThemeColor</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{Light: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#00CC44"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, Dark: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#00FF66"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">},</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">        Warning:    </span><span style="color:#6F42C1;--shiki-dark:#B392F0">ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ThemeColor</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{Light: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#CCAA00"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, Dark: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#FFFF00"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">},</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">        Error:      </span><span style="color:#6F42C1;--shiki-dark:#B392F0">ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ThemeColor</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{Light: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#CC0033"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, Dark: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#FF0055"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">},</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">        Info:       </span><span style="color:#6F42C1;--shiki-dark:#B392F0">ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ThemeColor</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{Light: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#0088CC"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, Dark: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#00CCFF"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">},</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">        Text:       </span><span style="color:#6F42C1;--shiki-dark:#B392F0">ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ThemeColor</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{Light: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#111111"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, Dark: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#F0F0F0"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">},</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">        Background: </span><span style="color:#6F42C1;--shiki-dark:#B392F0">ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ThemeColor</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{Light: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#F0F0F0"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, Dark: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#0A0A14"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">},</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">        MdKeyword:  </span><span style="color:#6F42C1;--shiki-dark:#B392F0">ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ThemeColor</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{Light: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#CC00FF"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, Dark: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#FF00FF"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">},</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">        MdString:   </span><span style="color:#6F42C1;--shiki-dark:#B392F0">ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ThemeColor</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{Light: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#00CC44"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, Dark: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#00FF66"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">},</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">        MdComment:  </span><span style="color:#6F42C1;--shiki-dark:#B392F0">ext</span><span style="color:#24292E;--shiki-dark:#E1E4E8">.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ThemeColor</span><span style="color:#24292E;--shiki-dark:#E1E4E8">{Light: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#888888"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, Dark: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"#555555"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">},</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">    })</span></span>
<span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">})</span></span></code></pre>
<h3 id="switching-themes"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#switching-themes"><span class="icon icon-link"></span></a>Switching themes</h3>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">err </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">SetTheme</span><span style="color:#24292E;--shiki-dark:#E1E4E8">(</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"dracula"</span><span style="color:#24292E;--shiki-dark:#E1E4E8">)</span></span></code></pre>
<h3 id="listing-available-themes"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#listing-available-themes"><span class="icon icon-link"></span></a>Listing available themes</h3>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#24292E;--shiki-dark:#E1E4E8">names </span><span style="color:#D73A49;--shiki-dark:#F97583">:=</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> ctx.</span><span style="color:#6F42C1;--shiki-dark:#B392F0">ListThemes</span><span style="color:#24292E;--shiki-dark:#E1E4E8">()</span></span></code></pre>
<h3 id="themecolorconfig-fields"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#themecolorconfig-fields"><span class="icon icon-link"></span></a>ThemeColorConfig fields</h3>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td><code>Primary</code></td>
<td>Main brand/accent color</td>
</tr>
<tr>
<td><code>Secondary</code></td>
<td>Secondary accent</td>
</tr>
<tr>
<td><code>Success</code></td>
<td>Success states</td>
</tr>
<tr>
<td><code>Warning</code></td>
<td>Warning states</td>
</tr>
<tr>
<td><code>Error</code></td>
<td>Error/critical states</td>
</tr>
<tr>
<td><code>Info</code></td>
<td>Informational states</td>
</tr>
<tr>
<td><code>Text</code></td>
<td>Primary text</td>
</tr>
<tr>
<td><code>Muted</code></td>
<td>Dimmed text</td>
</tr>
<tr>
<td><code>VeryMuted</code></td>
<td>Very dimmed text</td>
</tr>
<tr>
<td><code>Background</code></td>
<td>Base background</td>
</tr>
<tr>
<td><code>Border</code></td>
<td>Panel borders</td>
</tr>
<tr>
<td><code>MutedBorder</code></td>
<td>Subtle dividers</td>
</tr>
<tr>
<td><code>System</code></td>
<td>System messages</td>
</tr>
<tr>
<td><code>Tool</code></td>
<td>Tool-related elements</td>
</tr>
<tr>
<td><code>Accent</code></td>
<td>Secondary highlight</td>
</tr>
<tr>
<td><code>Highlight</code></td>
<td>Highlighted regions</td>
</tr>
<tr>
<td><code>MdHeading</code></td>
<td>Markdown headings</td>
</tr>
<tr>
<td><code>MdLink</code></td>
<td>Markdown links</td>
</tr>
<tr>
<td><code>MdKeyword</code></td>
<td>Syntax: keywords</td>
</tr>
<tr>
<td><code>MdString</code></td>
<td>Syntax: strings</td>
</tr>
<tr>
<td><code>MdNumber</code></td>
<td>Syntax: numbers</td>
</tr>
<tr>
<td><code>MdComment</code></td>
<td>Syntax: comments</td>
</tr>
</tbody>
</table>
<p>Each field is an <code>ext.ThemeColor</code> with <code>Light</code> and <code>Dark</code> hex strings. Empty fields inherit from the default theme.</p>
<h2 id="precedence-order"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#precedence-order"><span class="icon icon-link"></span></a>Precedence order</h2>
<p>When multiple sources define the same theme name, later sources win:</p>
<ol>
<li>Built-in presets (lowest)</li>
<li>User themes (<code>~/.config/kit/themes/</code>)</li>
<li>Project-local themes (<code>.kit/themes/</code>)</li>
<li>Extension-registered themes (highest)</li>
</ol>
<h3 id="startup-theme-resolution"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#startup-theme-resolution"><span class="icon icon-link"></span></a>Startup theme resolution</h3>
<p>At startup, Kit determines which theme to apply:</p>
<ol>
<li><strong><code>.kit.yml</code> <code>theme:</code> key</strong> — explicit config always wins (highest priority)</li>
<li><strong><code>~/.config/kit/preferences.yml</code></strong> — persisted <code>/theme</code> selection</li>
<li><strong>Default <code>kitt</code> theme</strong> — fallback</li>
</ol>
<p>The preferences file is updated automatically whenever you use <code>/theme</code> or <code>ctx.SetTheme()</code>. It is separate from <code>.kit.yml</code> so it never clobbers your config comments or formatting.</p>
<p>Theme changes via <code>/theme</code> or <code>ctx.SetTheme()</code> take effect immediately on all UI elements, including previously rendered messages.</p>`,headings:[{depth:2,text:"Quick start",id:"quick-start"},{depth:2,text:"Built-in themes",id:"built-in-themes"},{depth:2,text:"Custom theme files",id:"custom-theme-files"},{depth:3,text:"Theme file format",id:"theme-file-format"},{depth:3,text:"Partial themes",id:"partial-themes"},{depth:3,text:"Distributing themes",id:"distributing-themes"},{depth:2,text:"Config file theme",id:"config-file-theme"},{depth:2,text:"Extension theme API",id:"extension-theme-api"},{depth:3,text:"Registering a theme",id:"registering-a-theme"},{depth:3,text:"Switching themes",id:"switching-themes"},{depth:3,text:"Listing available themes",id:"listing-available-themes"},{depth:3,text:"ThemeColorConfig fields",id:"themecolorconfig-fields"},{depth:2,text:"Precedence order",id:"precedence-order"},{depth:3,text:"Startup theme resolution",id:"startup-theme-resolution"}],raw:`
# Themes

Kit ships with 22 built-in color themes and supports custom themes via YAML/JSON files or the extension API. Themes control all UI colors: input borders, popups, system messages, markdown rendering, syntax highlighting, and diff displays.

## Quick start

Switch themes at runtime with the \`/theme\` command:

\`\`\`
/theme dracula
/theme catppuccin
/theme kitt
\`\`\`

Run \`/theme\` with no arguments to list all available themes.

**Theme selections are automatically saved** to \`~/.config/kit/preferences.yml\` and restored on next launch. You don't need to add anything to your config file — just \`/theme <name>\` and it sticks.

## Built-in themes

| Theme | Style |
|-------|-------|
| \`kitt\` | KITT-inspired reds and ambers (default) |
| \`catppuccin\` | Soothing pastels (Mocha/Latte) |
| \`dracula\` | Purple and cyan dark theme |
| \`tokyonight\` | Cool blues with warm accents |
| \`nord\` | Arctic, north-bluish palette |
| \`gruvbox\` | Retro groove colors |
| \`monokai\` | Classic syntax theme |
| \`solarized\` | Precision colors for machines and people |
| \`github\` | GitHub's light and dark palettes |
| \`one-dark\` | Atom One Dark |
| \`rose-pine\` | Soho vibes with muted tones |
| \`ayu\` | Simple with bright colors |
| \`material\` | Material Design palette |
| \`everforest\` | Green-focused comfortable theme |
| \`kanagawa\` | Dark theme inspired by Katsushika Hokusai |
| \`amoled\` | Pure black background, vivid accents |
| \`synthwave\` | Retro neon glows |
| \`vesper\` | Warm minimalist dark theme |
| \`flexoki\` | Inky reading palette |
| \`matrix\` | Green-on-black terminal aesthetic |
| \`vercel\` | Clean monochrome with blue accents |
| \`zenburn\` | Low-contrast, warm dark theme |

All themes support both light and dark terminal modes via adaptive colors.

## Custom theme files

Create a \`.yml\`, \`.yaml\`, or \`.json\` file with color definitions. Kit discovers themes from two directories:

| Location | Scope | Precedence |
|----------|-------|------------|
| \`~/.config/kit/themes/\` | User (global) | Overrides built-ins |
| \`.kit/themes/\` | Project-local | Overrides user and built-ins |

### Theme file format

A theme file defines adaptive color pairs with \`light\` and \`dark\` hex values. Any field left empty inherits from the default KITT theme.

\`\`\`yaml
# ~/.config/kit/themes/my-theme.yml

# Core semantic colors
primary:
  light: "#8839ef"
  dark: "#cba6f7"
secondary:
  light: "#04a5e5"
  dark: "#89dceb"
success:
  light: "#40a02b"
  dark: "#a6e3a1"
warning:
  light: "#df8e1d"
  dark: "#f9e2af"
error:
  light: "#d20f39"
  dark: "#f38ba8"
info:
  light: "#1e66f5"
  dark: "#89b4fa"

# Text and chrome
text:
  light: "#4c4f69"
  dark: "#cdd6f4"
muted:
  light: "#6c6f85"
  dark: "#a6adc8"
very-muted:
  light: "#9ca0b0"
  dark: "#6c7086"
background:
  light: "#eff1f5"
  dark: "#1e1e2e"
border:
  light: "#acb0be"
  dark: "#585b70"
muted-border:
  light: "#ccd0da"
  dark: "#313244"

# Semantic roles
system:
  light: "#179299"
  dark: "#94e2d5"
tool:
  light: "#fe640b"
  dark: "#fab387"
accent:
  light: "#ea76cb"
  dark: "#f5c2e7"
highlight:
  light: "#e6e9ef"
  dark: "#181825"

# Diff backgrounds
diff-insert-bg:
  light: "#d5f0d5"
  dark: "#1a3a2a"
diff-delete-bg:
  light: "#f5d5d5"
  dark: "#3a1a2a"
diff-equal-bg:
  light: "#eceef3"
  dark: "#232336"
diff-missing-bg:
  light: "#e4e6eb"
  dark: "#1a1a2e"

# Code block backgrounds
code-bg:
  light: "#eceef3"
  dark: "#232336"
gutter-bg:
  light: "#e4e6eb"
  dark: "#1a1a2e"
write-bg:
  light: "#d5f0d5"
  dark: "#1a3a2a"

# Markdown and syntax highlighting
markdown:
  heading:
    light: "#1e66f5"
    dark: "#89b4fa"
  link:
    light: "#1e66f5"
    dark: "#89b4fa"
  keyword:
    light: "#8839ef"
    dark: "#cba6f7"
  string:
    light: "#40a02b"
    dark: "#a6e3a1"
  number:
    light: "#fe640b"
    dark: "#fab387"
  comment:
    light: "#9ca0b0"
    dark: "#6c7086"
\`\`\`

### Partial themes

You only need to define the colors you want to change. Unspecified fields fall back to the default theme:

\`\`\`yaml
# Just override the primary and accent colors
primary:
  dark: "#FF00FF"
accent:
  dark: "#00FFFF"
\`\`\`

### Distributing themes

- **Personal**: Drop a file in \`~/.config/kit/themes/\`
- **Team/project**: Drop a file in \`.kit/themes/\` and commit it to version control
- **Override built-in**: Name your file the same as a built-in (e.g., \`dracula.yml\`) and it takes precedence

## Config file theme

You can also set theme colors directly in \`.kit.yml\`:

\`\`\`yaml
theme:
  primary:
    light: "#8839ef"
    dark: "#cba6f7"
  error:
    dark: "#FF0000"
\`\`\`

Or reference an external theme file:

\`\`\`yaml
theme: "./themes/my-custom-theme.yml"
\`\`\`

## Extension theme API

Extensions can register and switch themes programmatically at runtime.

### Registering a theme

\`\`\`go
api.OnSessionStart(func(_ ext.SessionStartEvent, ctx ext.Context) {
    ctx.RegisterTheme("neon", ext.ThemeColorConfig{
        Primary:    ext.ThemeColor{Light: "#CC00FF", Dark: "#FF00FF"},
        Secondary:  ext.ThemeColor{Light: "#0088CC", Dark: "#00FFFF"},
        Success:    ext.ThemeColor{Light: "#00CC44", Dark: "#00FF66"},
        Warning:    ext.ThemeColor{Light: "#CCAA00", Dark: "#FFFF00"},
        Error:      ext.ThemeColor{Light: "#CC0033", Dark: "#FF0055"},
        Info:       ext.ThemeColor{Light: "#0088CC", Dark: "#00CCFF"},
        Text:       ext.ThemeColor{Light: "#111111", Dark: "#F0F0F0"},
        Background: ext.ThemeColor{Light: "#F0F0F0", Dark: "#0A0A14"},
        MdKeyword:  ext.ThemeColor{Light: "#CC00FF", Dark: "#FF00FF"},
        MdString:   ext.ThemeColor{Light: "#00CC44", Dark: "#00FF66"},
        MdComment:  ext.ThemeColor{Light: "#888888", Dark: "#555555"},
    })
})
\`\`\`

### Switching themes

\`\`\`go
err := ctx.SetTheme("dracula")
\`\`\`

### Listing available themes

\`\`\`go
names := ctx.ListThemes()
\`\`\`

### ThemeColorConfig fields

| Field | Description |
|-------|-------------|
| \`Primary\` | Main brand/accent color |
| \`Secondary\` | Secondary accent |
| \`Success\` | Success states |
| \`Warning\` | Warning states |
| \`Error\` | Error/critical states |
| \`Info\` | Informational states |
| \`Text\` | Primary text |
| \`Muted\` | Dimmed text |
| \`VeryMuted\` | Very dimmed text |
| \`Background\` | Base background |
| \`Border\` | Panel borders |
| \`MutedBorder\` | Subtle dividers |
| \`System\` | System messages |
| \`Tool\` | Tool-related elements |
| \`Accent\` | Secondary highlight |
| \`Highlight\` | Highlighted regions |
| \`MdHeading\` | Markdown headings |
| \`MdLink\` | Markdown links |
| \`MdKeyword\` | Syntax: keywords |
| \`MdString\` | Syntax: strings |
| \`MdNumber\` | Syntax: numbers |
| \`MdComment\` | Syntax: comments |

Each field is an \`ext.ThemeColor\` with \`Light\` and \`Dark\` hex strings. Empty fields inherit from the default theme.

## Precedence order

When multiple sources define the same theme name, later sources win:

1. Built-in presets (lowest)
2. User themes (\`~/.config/kit/themes/\`)
3. Project-local themes (\`.kit/themes/\`)
4. Extension-registered themes (highest)

### Startup theme resolution

At startup, Kit determines which theme to apply:

1. **\`.kit.yml\` \`theme:\` key** — explicit config always wins (highest priority)
2. **\`~/.config/kit/preferences.yml\`** — persisted \`/theme\` selection
3. **Default \`kitt\` theme** — fallback

The preferences file is updated automatically whenever you use \`/theme\` or \`ctx.SetTheme()\`. It is separate from \`.kit.yml\` so it never clobbers your config comments or formatting.

Theme changes via \`/theme\` or \`ctx.SetTheme()\` take effect immediately on all UI elements, including previously rendered messages.
`};export{s as default};

const s={frontmatter:{title:"Testing with tmux",description:"Test Kit's TUI non-interactively using tmux.",hidden:!1,toc:!0,draft:!1},html:`<h1 id="testing-with-tmux"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#testing-with-tmux"><span class="icon icon-link"></span></a>Testing with tmux</h1>
<p>Kit's interactive TUI can be tested non-interactively using tmux. This is useful for automated testing, CI pipelines, and extension development.</p>
<h2 id="basic-pattern"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#basic-pattern"><span class="icon icon-link"></span></a>Basic pattern</h2>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D"># Start Kit in a detached tmux session</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">tmux</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> new-session</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> -d</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> -s</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> kittest</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> -x</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> 120</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> -y</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> 40</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> \\</span></span>
<span class="line"><span style="color:#032F62;--shiki-dark:#9ECBFF">  "output/kit -e ext.go --no-session 2&gt;kit_stderr.log"</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D"># Wait for startup</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">sleep</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> 3</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D"># Capture the current screen</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">tmux</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> capture-pane</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> -t</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> kittest</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> -p</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D"># Send input</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">tmux</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> send-keys</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> -t</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> kittest</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> '/command'</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> Enter</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D"># Wait for response</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">sleep</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> 2</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D"># Capture updated screen</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">tmux</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> capture-pane</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> -t</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> kittest</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> -p</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D"># Cleanup</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">tmux</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> kill-session</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> -t</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> kittest</span></span></code></pre>
<h2 id="testing-extensions"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#testing-extensions"><span class="icon icon-link"></span></a>Testing extensions</h2>
<p>When testing extensions, the pattern is:</p>
<ol>
<li>Build Kit with your changes</li>
<li>Start Kit in tmux with the extension loaded</li>
<li>Send slash commands or prompts</li>
<li>Capture and verify the screen output</li>
<li>Check stderr logs for errors</li>
</ol>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D"># Build first</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">go</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> build</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> -o</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> output/kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> ./cmd/kit</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D"># Start with extension</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">tmux</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> new-session</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> -d</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> -s</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> kittest</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> -x</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> 120</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> -y</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> 40</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> \\</span></span>
<span class="line"><span style="color:#032F62;--shiki-dark:#9ECBFF">  "output/kit -e examples/extensions/widget-status.go --no-session 2&gt;kit_stderr.log"</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">sleep</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> 3</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D"># Verify widget appears in screen</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">tmux</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> capture-pane</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> -t</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> kittest</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> -p</span><span style="color:#D73A49;--shiki-dark:#F97583"> |</span><span style="color:#6F42C1;--shiki-dark:#B392F0"> grep</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> "Status"</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D"># Send a slash command</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">tmux</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> send-keys</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> -t</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> kittest</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> '/stats'</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> Enter</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">sleep</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> 1</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">tmux</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> capture-pane</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> -t</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> kittest</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> -p</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D"># Cleanup</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">tmux</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> kill-session</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> -t</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> kittest</span></span></code></pre>
<h2 id="tips"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#tips"><span class="icon icon-link"></span></a>Tips</h2>
<ul>
<li>Use <code>-x</code> and <code>-y</code> to set consistent terminal dimensions</li>
<li>Redirect stderr to a log file (<code>2&gt;kit.log</code>) for debugging</li>
<li>Use <code>--no-session</code> to avoid creating session files during tests</li>
<li>Add sufficient <code>sleep</code> between commands for the TUI to render</li>
<li>Use <code>grep</code> on captured pane output to verify specific content</li>
</ul>`,headings:[{depth:2,text:"Basic pattern",id:"basic-pattern"},{depth:2,text:"Testing extensions",id:"testing-extensions"},{depth:2,text:"Tips",id:"tips"}],raw:`
# Testing with tmux

Kit's interactive TUI can be tested non-interactively using tmux. This is useful for automated testing, CI pipelines, and extension development.

## Basic pattern

\`\`\`bash
# Start Kit in a detached tmux session
tmux new-session -d -s kittest -x 120 -y 40 \\
  "output/kit -e ext.go --no-session 2>kit_stderr.log"

# Wait for startup
sleep 3

# Capture the current screen
tmux capture-pane -t kittest -p

# Send input
tmux send-keys -t kittest '/command' Enter

# Wait for response
sleep 2

# Capture updated screen
tmux capture-pane -t kittest -p

# Cleanup
tmux kill-session -t kittest
\`\`\`

## Testing extensions

When testing extensions, the pattern is:

1. Build Kit with your changes
2. Start Kit in tmux with the extension loaded
3. Send slash commands or prompts
4. Capture and verify the screen output
5. Check stderr logs for errors

\`\`\`bash
# Build first
go build -o output/kit ./cmd/kit

# Start with extension
tmux new-session -d -s kittest -x 120 -y 40 \\
  "output/kit -e examples/extensions/widget-status.go --no-session 2>kit_stderr.log"

sleep 3

# Verify widget appears in screen
tmux capture-pane -t kittest -p | grep "Status"

# Send a slash command
tmux send-keys -t kittest '/stats' Enter
sleep 1
tmux capture-pane -t kittest -p

# Cleanup
tmux kill-session -t kittest
\`\`\`

## Tips

- Use \`-x\` and \`-y\` to set consistent terminal dimensions
- Redirect stderr to a log file (\`2>kit.log\`) for debugging
- Use \`--no-session\` to avoid creating session files during tests
- Add sufficient \`sleep\` between commands for the TUI to render
- Use \`grep\` on captured pane output to verify specific content
`};export{s as default};

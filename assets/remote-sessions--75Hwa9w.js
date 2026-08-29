var e={frontmatter:{title:`Remote Sessions`,description:`Run Kit on one machine and drive it from another over an end-to-end encrypted iroh connection.`,hidden:!1,toc:!0,draft:!1},html:`<h1 id="remote-sessions"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#remote-sessions"><span class="icon icon-link"></span></a>Remote Sessions</h1>
<p>Kit can run as a daemon on one machine and be driven from another. All work —
the agent, tools, extensions, sessions — happens on the daemon host; your
local terminal just renders it. The transport is <a href="https://iroh.computer">iroh</a>:
a direct, end-to-end encrypted QUIC connection that holes through NATs and
falls back to relays.</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D"># On the machine that does the work:</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> daemon</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">#   Pairing code: A1B2-C3D4</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D"># On the machine you are sitting at:</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> --remote</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> A1B2C3D4</span></span></code></pre>
<p>On connection the remote peer picks a working directory (the picker starts in
the daemon user's home directory), and the session TUI starts there. The
session behaves exactly like a local one: extensions, widgets, tool
rendering, and session persistence all run on the daemon host.</p>
<h2 id="requirements"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#requirements"><span class="icon icon-link"></span></a>Requirements</h2>
<ul>
<li>Both machines run a recent <code>kit</code> build (the transport sidecar is embedded
in release binaries; source builds need <code>task tunnel</code> once).</li>
<li>Outbound internet access for iroh discovery and, when a direct path cannot
be punched, the n0 relay fleet.</li>
</ul>
<h2 id="commands"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#commands"><span class="icon icon-link"></span></a>Commands</h2>
<table>
<thead>
<tr>
<th>Command</th>
<th>Purpose</th>
</tr>
</thead>
<tbody>
<tr>
<td><code>kit daemon</code></td>
<td>Start the daemon and print the pairing code</td>
</tr>
<tr>
<td><code>kit daemon status</code></td>
<td>Show code, endpoint, uptime and active sessions of a running daemon</td>
</tr>
<tr>
<td><code>kit daemon service install</code></td>
<td>Install and start a systemd user service</td>
</tr>
<tr>
<td><code>kit daemon service remove</code></td>
<td>Stop and uninstall the service</td>
</tr>
<tr>
<td><code>kit --remote CODE</code></td>
<td>Attach this terminal to a daemon session</td>
</tr>
</tbody>
</table>
<p>Useful daemon flags: <code>--code ABCD2345</code> pins a specific pairing code
(hidden, mainly for tests).</p>
<h2 id="multiple-sessions"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#multiple-sessions"><span class="icon icon-link"></span></a>Multiple sessions</h2>
<p>Each verified client gets its own session with its own working directory
choice. Exiting a session (<code>/quit</code>) closes only that client's connection;
detaching with <code>Ctrl-]</code> keeps the session running until it is reaped by
its own timeout. One pairing code stays valid for the whole daemon run, so
teammates (or your other machines) can attach while you are working.</p>
<h2 id="systemd"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#systemd"><span class="icon icon-link"></span></a>systemd</h2>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> daemon</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> service</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> install</span><span style="color:#6A737D;--shiki-dark:#6A737D">   # writes ~/.config/systemd/user/kit.service, enables + starts it</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> daemon</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> status</span><span style="color:#6A737D;--shiki-dark:#6A737D">            # shows the live pairing code</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">systemctl</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> --user</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> status</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> kit</span><span style="color:#6A737D;--shiki-dark:#6A737D">  # manage the service directly</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> daemon</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> service</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> remove</span><span style="color:#6A737D;--shiki-dark:#6A737D">    # stop and uninstall</span></span></code></pre>
<p><code>install</code> captures provider credentials from your current shell
(<code>*_API_KEY</code>, <code>*_TOKEN</code>, <code>PROVIDER_*</code>, and similar) into
<code>~/.config/kit/daemon.env</code>, which the unit loads via <code>EnvironmentFile</code>. Edit
that file and run <code>systemctl --user restart kit</code> when keys change.</p>
<h2 id="security-model"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#security-model"><span class="icon icon-link"></span></a>Security model</h2>
<ul>
<li>The pairing code is 8 characters from a 32-symbol alphabet (~40 bits of
entropy). It stays valid for the daemon's lifetime — treat it like a
password.</li>
<li>The daemon's endpoint identity is derived from the code: without it, a
peer cannot even find the endpoint. Connections are additionally
authenticated with a mutual HMAC handshake; failed attempts back off
exponentially.</li>
<li>Session slots are capped, and polite rejections of over-cap peers are
budgeted so connection floods cannot pin daemon resources.</li>
<li>Only one daemon may run per user (enforced with a <code>flock</code>); state lives in
<code>~/.cache/kit/daemon/</code>.</li>
</ul>
<h2 id="troubleshooting"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#troubleshooting"><span class="icon icon-link"></span></a>Troubleshooting</h2>
<table>
<thead>
<tr>
<th>Symptom</th>
<th>Cause and fix</th>
</tr>
</thead>
<tbody>
<tr>
<td><code>another instance is already running</code></td>
<td>A daemon (or service) already holds the lock — see <code>kit daemon status</code></td>
</tr>
<tr>
<td><code>no daemon is live for this pairing code</code></td>
<td>The daemon restarted or stopped; get the current code from <code>kit daemon status</code></td>
</tr>
<tr>
<td><code>could not reach the daemon</code></td>
<td>Network or relay issue; check connectivity on both sides</td>
</tr>
<tr>
<td>Session dies with <code>API key not provided</code></td>
<td>The daemon environment is missing provider keys — for the systemd service, edit <code>~/.config/kit/daemon.env</code> and restart</td>
</tr>
<tr>
<td>Detached by accident</td>
<td>Just reconnect with the same code; the daemon is still running and sessions persist on the daemon host</td>
</tr>
</tbody>
</table>`,headings:[{depth:2,text:`Requirements`,id:`requirements`},{depth:2,text:`Commands`,id:`commands`},{depth:2,text:`Multiple sessions`,id:`multiple-sessions`},{depth:2,text:`systemd`,id:`systemd`},{depth:2,text:`Security model`,id:`security-model`},{depth:2,text:`Troubleshooting`,id:`troubleshooting`}],raw:`
# Remote Sessions

Kit can run as a daemon on one machine and be driven from another. All work —
the agent, tools, extensions, sessions — happens on the daemon host; your
local terminal just renders it. The transport is [iroh](https://iroh.computer):
a direct, end-to-end encrypted QUIC connection that holes through NATs and
falls back to relays.

\`\`\`bash
# On the machine that does the work:
kit daemon
#   Pairing code: A1B2-C3D4

# On the machine you are sitting at:
kit --remote A1B2C3D4
\`\`\`

On connection the remote peer picks a working directory (the picker starts in
the daemon user's home directory), and the session TUI starts there. The
session behaves exactly like a local one: extensions, widgets, tool
rendering, and session persistence all run on the daemon host.

## Requirements

- Both machines run a recent \`kit\` build (the transport sidecar is embedded
  in release binaries; source builds need \`task tunnel\` once).
- Outbound internet access for iroh discovery and, when a direct path cannot
  be punched, the n0 relay fleet.

## Commands

| Command | Purpose |
|---------|---------|
| \`kit daemon\` | Start the daemon and print the pairing code |
| \`kit daemon status\` | Show code, endpoint, uptime and active sessions of a running daemon |
| \`kit daemon service install\` | Install and start a systemd user service |
| \`kit daemon service remove\` | Stop and uninstall the service |
| \`kit --remote CODE\` | Attach this terminal to a daemon session |

Useful daemon flags: \`--code ABCD2345\` pins a specific pairing code
(hidden, mainly for tests).

## Multiple sessions

Each verified client gets its own session with its own working directory
choice. Exiting a session (\`/quit\`) closes only that client's connection;
detaching with \`Ctrl-]\` keeps the session running until it is reaped by
its own timeout. One pairing code stays valid for the whole daemon run, so
teammates (or your other machines) can attach while you are working.

## systemd

\`\`\`bash
kit daemon service install   # writes ~/.config/systemd/user/kit.service, enables + starts it
kit daemon status            # shows the live pairing code
systemctl --user status kit  # manage the service directly
kit daemon service remove    # stop and uninstall
\`\`\`

\`install\` captures provider credentials from your current shell
(\`*_API_KEY\`, \`*_TOKEN\`, \`PROVIDER_*\`, and similar) into
\`~/.config/kit/daemon.env\`, which the unit loads via \`EnvironmentFile\`. Edit
that file and run \`systemctl --user restart kit\` when keys change.

## Security model

- The pairing code is 8 characters from a 32-symbol alphabet (~40 bits of
  entropy). It stays valid for the daemon's lifetime — treat it like a
  password.
- The daemon's endpoint identity is derived from the code: without it, a
  peer cannot even find the endpoint. Connections are additionally
  authenticated with a mutual HMAC handshake; failed attempts back off
  exponentially.
- Session slots are capped, and polite rejections of over-cap peers are
  budgeted so connection floods cannot pin daemon resources.
- Only one daemon may run per user (enforced with a \`flock\`); state lives in
  \`~/.cache/kit/daemon/\`.

## Troubleshooting

| Symptom | Cause and fix |
|---------|---------------|
| \`another instance is already running\` | A daemon (or service) already holds the lock — see \`kit daemon status\` |
| \`no daemon is live for this pairing code\` | The daemon restarted or stopped; get the current code from \`kit daemon status\` |
| \`could not reach the daemon\` | Network or relay issue; check connectivity on both sides |
| Session dies with \`API key not provided\` | The daemon environment is missing provider keys — for the systemd service, edit \`~/.config/kit/daemon.env\` and restart |
| Detached by accident | Just reconnect with the same code; the daemon is still running and sessions persist on the daemon host |
`};export{e as default};
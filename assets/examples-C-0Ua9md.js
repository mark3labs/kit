const t={frontmatter:{title:"Examples",description:"Catalog of example extensions included with Kit.",hidden:!1,toc:!0,draft:!1},html:`<h1 id="extension-examples"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#extension-examples"><span class="icon icon-link"></span></a>Extension Examples</h1>
<p>Kit ships with a rich set of example extensions in the <code>examples/extensions/</code> directory. These serve as both documentation and starting points for your own extensions.</p>
<h2 id="ui-and-display"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#ui-and-display"><span class="icon icon-link"></span></a>UI and display</h2>
<table>
<thead>
<tr>
<th>Extension</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td><code>minimal.go</code></td>
<td>Clean UI with custom footer</td>
</tr>
<tr>
<td><code>branded-output.go</code></td>
<td>Branded output rendering</td>
</tr>
<tr>
<td><code>header-footer-demo.go</code></td>
<td>Custom headers and footers</td>
</tr>
<tr>
<td><code>widget-status.go</code></td>
<td>Persistent status widgets</td>
</tr>
<tr>
<td><code>overlay-demo.go</code></td>
<td>Modal dialogs</td>
</tr>
<tr>
<td><code>tool-renderer-demo.go</code></td>
<td>Custom tool call rendering</td>
</tr>
<tr>
<td><code>custom-editor-demo.go</code></td>
<td>Vim-like modal editor</td>
</tr>
<tr>
<td><code>pirate.go</code></td>
<td>Pirate-themed personality</td>
</tr>
</tbody>
</table>
<h2 id="workflow-and-automation"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#workflow-and-automation"><span class="icon icon-link"></span></a>Workflow and automation</h2>
<table>
<thead>
<tr>
<th>Extension</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td><code>auto-commit.go</code></td>
<td>Auto-commit changes on shutdown</td>
</tr>
<tr>
<td><code>plan-mode.go</code></td>
<td>Read-only planning mode</td>
</tr>
<tr>
<td><code>permission-gate.go</code></td>
<td>Permission gating for destructive tools</td>
</tr>
<tr>
<td><code>confirm-destructive.go</code></td>
<td>Confirm destructive operations</td>
</tr>
<tr>
<td><code>protected-paths.go</code></td>
<td>Path protection for sensitive files</td>
</tr>
<tr>
<td><code>project-rules.go</code></td>
<td>Project-specific rules injection</td>
</tr>
<tr>
<td><code>compact-notify.go</code></td>
<td>Notification on conversation compaction</td>
</tr>
</tbody>
</table>
<h2 id="interactive-features"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#interactive-features"><span class="icon icon-link"></span></a>Interactive features</h2>
<table>
<thead>
<tr>
<th>Extension</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td><code>prompt-demo.go</code></td>
<td>Interactive prompts (select/confirm/input)</td>
</tr>
<tr>
<td><code>bookmark.go</code></td>
<td>Bookmark conversations</td>
</tr>
<tr>
<td><code>inline-bash.go</code></td>
<td>Inline bash execution</td>
</tr>
<tr>
<td><code>interactive-shell.go</code></td>
<td>Interactive shell integration</td>
</tr>
<tr>
<td><code>notify.go</code></td>
<td>Desktop notifications</td>
</tr>
</tbody>
</table>
<h2 id="agent-and-context"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#agent-and-context"><span class="icon icon-link"></span></a>Agent and context</h2>
<table>
<thead>
<tr>
<th>Extension</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td><code>tool-logger.go</code></td>
<td>Log all tool calls</td>
</tr>
<tr>
<td><code>context-inject.go</code></td>
<td>Inject context into conversations</td>
</tr>
<tr>
<td><code>summarize.go</code></td>
<td>Conversation summarization</td>
</tr>
<tr>
<td><code>lsp-diagnostics.go</code></td>
<td>LSP diagnostic integration</td>
</tr>
</tbody>
</table>
<h2 id="multi-agent"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#multi-agent"><span class="icon icon-link"></span></a>Multi-agent</h2>
<table>
<thead>
<tr>
<th>Extension</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td><code>kit-kit.go</code></td>
<td>Kit-in-Kit sub-agent spawning</td>
</tr>
<tr>
<td><code>subagent-widget.go</code></td>
<td>Multi-agent orchestration with status widget</td>
</tr>
<tr>
<td><code>subagent-test.go</code></td>
<td>Subagent testing utilities</td>
</tr>
</tbody>
</table>
<h2 id="development"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#development"><span class="icon icon-link"></span></a>Development</h2>
<table>
<thead>
<tr>
<th>Extension</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td><code>dev-reload.go</code></td>
<td>Development live-reload</td>
</tr>
</tbody>
</table>
<h2 id="subdirectory-extensions"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#subdirectory-extensions"><span class="icon icon-link"></span></a>Subdirectory extensions</h2>
<table>
<thead>
<tr>
<th>Directory</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td><code>kit-kit-agents/</code></td>
<td>Multi-agent orchestration example</td>
</tr>
<tr>
<td><code>kit-telegram/</code></td>
<td>Telegram bot integration</td>
</tr>
<tr>
<td><code>status-tools/</code></td>
<td>Status bar tool examples</td>
</tr>
</tbody>
</table>`,headings:[{depth:2,text:"UI and display",id:"ui-and-display"},{depth:2,text:"Workflow and automation",id:"workflow-and-automation"},{depth:2,text:"Interactive features",id:"interactive-features"},{depth:2,text:"Agent and context",id:"agent-and-context"},{depth:2,text:"Multi-agent",id:"multi-agent"},{depth:2,text:"Development",id:"development"},{depth:2,text:"Subdirectory extensions",id:"subdirectory-extensions"}],raw:`
# Extension Examples

Kit ships with a rich set of example extensions in the \`examples/extensions/\` directory. These serve as both documentation and starting points for your own extensions.

## UI and display

| Extension | Description |
|-----------|-------------|
| \`minimal.go\` | Clean UI with custom footer |
| \`branded-output.go\` | Branded output rendering |
| \`header-footer-demo.go\` | Custom headers and footers |
| \`widget-status.go\` | Persistent status widgets |
| \`overlay-demo.go\` | Modal dialogs |
| \`tool-renderer-demo.go\` | Custom tool call rendering |
| \`custom-editor-demo.go\` | Vim-like modal editor |
| \`pirate.go\` | Pirate-themed personality |

## Workflow and automation

| Extension | Description |
|-----------|-------------|
| \`auto-commit.go\` | Auto-commit changes on shutdown |
| \`plan-mode.go\` | Read-only planning mode |
| \`permission-gate.go\` | Permission gating for destructive tools |
| \`confirm-destructive.go\` | Confirm destructive operations |
| \`protected-paths.go\` | Path protection for sensitive files |
| \`project-rules.go\` | Project-specific rules injection |
| \`compact-notify.go\` | Notification on conversation compaction |

## Interactive features

| Extension | Description |
|-----------|-------------|
| \`prompt-demo.go\` | Interactive prompts (select/confirm/input) |
| \`bookmark.go\` | Bookmark conversations |
| \`inline-bash.go\` | Inline bash execution |
| \`interactive-shell.go\` | Interactive shell integration |
| \`notify.go\` | Desktop notifications |

## Agent and context

| Extension | Description |
|-----------|-------------|
| \`tool-logger.go\` | Log all tool calls |
| \`context-inject.go\` | Inject context into conversations |
| \`summarize.go\` | Conversation summarization |
| \`lsp-diagnostics.go\` | LSP diagnostic integration |

## Multi-agent

| Extension | Description |
|-----------|-------------|
| \`kit-kit.go\` | Kit-in-Kit sub-agent spawning |
| \`subagent-widget.go\` | Multi-agent orchestration with status widget |
| \`subagent-test.go\` | Subagent testing utilities |

## Development

| Extension | Description |
|-----------|-------------|
| \`dev-reload.go\` | Development live-reload |

## Subdirectory extensions

| Directory | Description |
|-----------|-------------|
| \`kit-kit-agents/\` | Multi-agent orchestration example |
| \`kit-telegram/\` | Telegram bot integration |
| \`status-tools/\` | Status bar tool examples |
`};export{t as default};

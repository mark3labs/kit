const e={frontmatter:{title:"Development",description:"Build, test, and contribute to Kit.",hidden:!1,toc:!0,draft:!1},html:`<h1 id="development"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#development"><span class="icon icon-link"></span></a>Development</h1>
<h2 id="build-and-test"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#build-and-test"><span class="icon icon-link"></span></a>Build and test</h2>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D"># Build</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">go</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> build</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> -o</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> output/kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> ./cmd/kit</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D"># Run all tests</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">go</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> test</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> -race</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> ./...</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D"># Run a specific test</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">go</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> test</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> -race</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> ./cmd</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> -run</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> TestScriptExecution</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D"># Lint</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">go</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> vet</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> ./...</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D"># Format</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">go</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> fmt</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> ./...</span></span></code></pre>
<h2 id="project-structure"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#project-structure"><span class="icon icon-link"></span></a>Project structure</h2>
<pre><code>cmd/kit/             - CLI entry point (main.go)
cmd/                 - CLI command implementations (root, auth, models, etc.)
pkg/kit/             - Go SDK for embedding Kit
internal/app/        - Application orchestrator (agent loop, message store, queue)
internal/agent/      - Agent execution and tool dispatch
internal/auth/       - OAuth authentication and credential storage
internal/acpserver/  - ACP (Agent Client Protocol) server
internal/clipboard/  - Cross-platform clipboard operations
internal/compaction/ - Conversation compaction and summarization
internal/config/     - Configuration management
internal/core/       - Built-in tools (bash, read, write, edit, grep, find, ls)
internal/extensions/ - Yaegi extension system
internal/kitsetup/   - Initial setup wizard
internal/message/    - Message content types and structured content blocks
internal/models/     - Provider and model management
internal/session/    - Session persistence (tree-based JSONL)
internal/skills/     - Skill loading and system prompt composition
internal/tools/      - MCP tool integration
internal/ui/         - Bubble Tea TUI components
examples/extensions/ - Example extension files
npm/                 - NPM package wrapper for distribution
</code></pre>
<h2 id="architecture-overview"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#architecture-overview"><span class="icon icon-link"></span></a>Architecture overview</h2>
<p>Kit is built around a few key architectural patterns:</p>
<h3 id="multi-provider-llm-support"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#multi-provider-llm-support"><span class="icon icon-link"></span></a>Multi-provider LLM support</h3>
<p>The <code>llm.Provider</code> interface abstracts different LLM providers. Each provider implements message formatting, tool calling, and streaming for its specific API.</p>
<h3 id="mcp-client-server-model"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#mcp-client-server-model"><span class="icon icon-link"></span></a>MCP client-server model</h3>
<p>External tools are integrated via the Model Context Protocol (MCP). Kit acts as an MCP client, connecting to MCP servers configured in <code>.kit.yml</code>.</p>
<h3 id="extension-system"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#extension-system"><span class="icon icon-link"></span></a>Extension system</h3>
<p>Extensions are Go source files interpreted at runtime by Yaegi. The <code>internal/extensions/</code> package manages loading, symbol export, and lifecycle dispatch. See the <a href="/extensions/overview">Extension System</a> docs for details.</p>
<h3 id="tui-architecture"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#tui-architecture"><span class="icon icon-link"></span></a>TUI architecture</h3>
<p>The interactive terminal UI is built with <a href="https://github.com/charmbracelet/bubbletea">Bubble Tea v2</a>, using a parent-child model where <code>AppModel</code> manages child components (<code>InputComponent</code>, <code>StreamComponent</code>, etc.).</p>
<h3 id="decoupling-pattern"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#decoupling-pattern"><span class="icon icon-link"></span></a>Decoupling pattern</h3>
<p><code>cmd/root.go</code> contains converter functions (e.g., <code>widgetProviderForUI()</code>) that bridge <code>internal/extensions/</code> types to <code>internal/ui/</code> types. The UI never imports the extensions package directly.</p>
<h2 id="contributing"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#contributing"><span class="icon icon-link"></span></a>Contributing</h2>
<p>Contributions are welcome! Please see the <a href="https://github.com/mark3labs/kit/blob/master/contribute/contribute.md">contribution guide</a> for guidelines.</p>
<h2 id="community"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#community"><span class="icon icon-link"></span></a>Community</h2>
<ul>
<li><a href="https://discord.gg/RqSS2NQVsY">Discord</a></li>
<li><a href="https://github.com/mark3labs/kit/issues">GitHub Issues</a></li>
</ul>`,headings:[{depth:2,text:"Build and test",id:"build-and-test"},{depth:2,text:"Project structure",id:"project-structure"},{depth:2,text:"Architecture overview",id:"architecture-overview"},{depth:3,text:"Multi-provider LLM support",id:"multi-provider-llm-support"},{depth:3,text:"MCP client-server model",id:"mcp-client-server-model"},{depth:3,text:"Extension system",id:"extension-system"},{depth:3,text:"TUI architecture",id:"tui-architecture"},{depth:3,text:"Decoupling pattern",id:"decoupling-pattern"},{depth:2,text:"Contributing",id:"contributing"},{depth:2,text:"Community",id:"community"}],raw:`
# Development

## Build and test

\`\`\`bash
# Build
go build -o output/kit ./cmd/kit

# Run all tests
go test -race ./...

# Run a specific test
go test -race ./cmd -run TestScriptExecution

# Lint
go vet ./...

# Format
go fmt ./...
\`\`\`

## Project structure

\`\`\`
cmd/kit/             - CLI entry point (main.go)
cmd/                 - CLI command implementations (root, auth, models, etc.)
pkg/kit/             - Go SDK for embedding Kit
internal/app/        - Application orchestrator (agent loop, message store, queue)
internal/agent/      - Agent execution and tool dispatch
internal/auth/       - OAuth authentication and credential storage
internal/acpserver/  - ACP (Agent Client Protocol) server
internal/clipboard/  - Cross-platform clipboard operations
internal/compaction/ - Conversation compaction and summarization
internal/config/     - Configuration management
internal/core/       - Built-in tools (bash, read, write, edit, grep, find, ls)
internal/extensions/ - Yaegi extension system
internal/kitsetup/   - Initial setup wizard
internal/message/    - Message content types and structured content blocks
internal/models/     - Provider and model management
internal/session/    - Session persistence (tree-based JSONL)
internal/skills/     - Skill loading and system prompt composition
internal/tools/      - MCP tool integration
internal/ui/         - Bubble Tea TUI components
examples/extensions/ - Example extension files
npm/                 - NPM package wrapper for distribution
\`\`\`

## Architecture overview

Kit is built around a few key architectural patterns:

### Multi-provider LLM support

The \`llm.Provider\` interface abstracts different LLM providers. Each provider implements message formatting, tool calling, and streaming for its specific API.

### MCP client-server model

External tools are integrated via the Model Context Protocol (MCP). Kit acts as an MCP client, connecting to MCP servers configured in \`.kit.yml\`.

### Extension system

Extensions are Go source files interpreted at runtime by Yaegi. The \`internal/extensions/\` package manages loading, symbol export, and lifecycle dispatch. See the [Extension System](/extensions/overview) docs for details.

### TUI architecture

The interactive terminal UI is built with [Bubble Tea v2](https://github.com/charmbracelet/bubbletea), using a parent-child model where \`AppModel\` manages child components (\`InputComponent\`, \`StreamComponent\`, etc.).

### Decoupling pattern

\`cmd/root.go\` contains converter functions (e.g., \`widgetProviderForUI()\`) that bridge \`internal/extensions/\` types to \`internal/ui/\` types. The UI never imports the extensions package directly.

## Contributing

Contributions are welcome! Please see the [contribution guide](https://github.com/mark3labs/kit/blob/master/contribute/contribute.md) for guidelines.

## Community

- [Discord](https://discord.gg/RqSS2NQVsY)
- [GitHub Issues](https://github.com/mark3labs/kit/issues)
`};export{e as default};

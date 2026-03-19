const s={frontmatter:{title:"Providers",description:"Supported LLM providers and model configuration.",hidden:!1,toc:!0,draft:!1},html:`<h1 id="providers"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#providers"><span class="icon icon-link"></span></a>Providers</h1>
<p>Kit supports a wide range of LLM providers through a unified <code>provider/model</code> string format.</p>
<h2 id="supported-providers"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#supported-providers"><span class="icon icon-link"></span></a>Supported providers</h2>
<table>
<thead>
<tr>
<th>Provider</th>
<th>Prefix</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td><strong>Anthropic</strong></td>
<td><code>anthropic/</code></td>
<td>Claude models (native, prompt caching, OAuth)</td>
</tr>
<tr>
<td><strong>OpenAI</strong></td>
<td><code>openai/</code></td>
<td>GPT models</td>
</tr>
<tr>
<td><strong>Google</strong></td>
<td><code>google/</code> or <code>gemini/</code></td>
<td>Gemini models</td>
</tr>
<tr>
<td><strong>Ollama</strong></td>
<td><code>ollama/</code></td>
<td>Local models</td>
</tr>
<tr>
<td><strong>Azure OpenAI</strong></td>
<td><code>azure/</code></td>
<td>Azure-hosted OpenAI</td>
</tr>
<tr>
<td><strong>AWS Bedrock</strong></td>
<td><code>bedrock/</code></td>
<td>Bedrock models</td>
</tr>
<tr>
<td><strong>Google Vertex</strong></td>
<td><code>google-vertex-anthropic/</code></td>
<td>Claude on Vertex AI</td>
</tr>
<tr>
<td><strong>OpenRouter</strong></td>
<td><code>openrouter/</code></td>
<td>Multi-provider router</td>
</tr>
<tr>
<td><strong>Vercel AI</strong></td>
<td><code>vercel/</code></td>
<td>Vercel AI SDK models</td>
</tr>
<tr>
<td><strong>Auto-routed</strong></td>
<td>any</td>
<td>Any provider from the models.dev database</td>
</tr>
</tbody>
</table>
<h2 id="model-string-format"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#model-string-format"><span class="icon icon-link"></span></a>Model string format</h2>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">provider/model</span><span style="color:#6A737D;--shiki-dark:#6A737D">            # Standard format</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">anthropic/claude-sonnet-4-5-20250929</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">openai/gpt-4o</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">ollama/llama3</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">google/gemini-2.0-flash-exp</span></span></code></pre>
<h2 id="model-aliases"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#model-aliases"><span class="icon icon-link"></span></a>Model aliases</h2>
<p>Kit provides aliases for commonly used models:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">claude-opus-latest</span><span style="color:#032F62;--shiki-dark:#9ECBFF">        →</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> claude-opus-4-20250514</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">claude-sonnet-latest</span><span style="color:#032F62;--shiki-dark:#9ECBFF">      →</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> claude-sonnet-4-5-20250929</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">claude-4-opus-latest</span><span style="color:#032F62;--shiki-dark:#9ECBFF">      →</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> claude-opus-4-20250514</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">claude-4-sonnet-latest</span><span style="color:#032F62;--shiki-dark:#9ECBFF">    →</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> claude-sonnet-4-5-20250929</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">claude-3-7-sonnet-latest</span><span style="color:#032F62;--shiki-dark:#9ECBFF">  →</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> claude-3-7-sonnet-20250219</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">claude-3-5-sonnet-latest</span><span style="color:#032F62;--shiki-dark:#9ECBFF">  →</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> claude-3-5-sonnet-20241022</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">claude-3-5-haiku-latest</span><span style="color:#032F62;--shiki-dark:#9ECBFF">   →</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> claude-3-5-haiku-20241022</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">claude-3-opus-latest</span><span style="color:#032F62;--shiki-dark:#9ECBFF">      →</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> claude-3-opus-20240229</span></span></code></pre>
<h2 id="specifying-a-model"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#specifying-a-model"><span class="icon icon-link"></span></a>Specifying a model</h2>
<p>Via CLI flag:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> --model</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> openai/gpt-4o</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> -m</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> ollama/llama3</span></span></code></pre>
<p>Via config file:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">model</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">anthropic/claude-sonnet-4-5-20250929</span></span></code></pre>
<p>Via environment variable:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">export</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> KIT_MODEL</span><span style="color:#D73A49;--shiki-dark:#F97583">=</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"google/gemini-2.0-flash-exp"</span></span></code></pre>
<h2 id="authentication"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#authentication"><span class="icon icon-link"></span></a>Authentication</h2>
<h3 id="api-keys"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#api-keys"><span class="icon icon-link"></span></a>API keys</h3>
<p>Set the appropriate environment variable for your provider:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">export</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> ANTHROPIC_API_KEY</span><span style="color:#D73A49;--shiki-dark:#F97583">=</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"sk-..."</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">export</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> OPENAI_API_KEY</span><span style="color:#D73A49;--shiki-dark:#F97583">=</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"sk-..."</span></span>
<span class="line"><span style="color:#D73A49;--shiki-dark:#F97583">export</span><span style="color:#24292E;--shiki-dark:#E1E4E8"> GOOGLE_API_KEY</span><span style="color:#D73A49;--shiki-dark:#F97583">=</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"..."</span></span></code></pre>
<p>Or pass it directly:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> --provider-api-key</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> "sk-..."</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> --model</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> openai/gpt-4o</span></span></code></pre>
<h3 id="oauth"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#oauth"><span class="icon icon-link"></span></a>OAuth</h3>
<p>For providers that support OAuth (e.g., Anthropic):</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> auth</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> login</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> anthropic</span><span style="color:#6A737D;--shiki-dark:#6A737D">     # Start OAuth flow</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> auth</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> status</span><span style="color:#6A737D;--shiki-dark:#6A737D">              # Check authentication status</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> auth</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> logout</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> anthropic</span><span style="color:#6A737D;--shiki-dark:#6A737D">    # Remove credentials</span></span></code></pre>
<h3 id="custom-provider-url"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#custom-provider-url"><span class="icon icon-link"></span></a>Custom provider URL</h3>
<p>For self-hosted or proxy endpoints:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> --provider-url</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> "https://my-proxy.example.com/v1"</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> --model</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> openai/gpt-4o</span></span></code></pre>
<h2 id="model-database"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#model-database"><span class="icon icon-link"></span></a>Model database</h2>
<p>Kit ships with a local model database that maps provider names to API configurations. You can manage it with:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> models</span><span style="color:#6A737D;--shiki-dark:#6A737D">                   # List available models</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> models</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> openai</span><span style="color:#6A737D;--shiki-dark:#6A737D">            # Filter by provider</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> models</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> --all</span><span style="color:#6A737D;--shiki-dark:#6A737D">             # Show all providers</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> update-models</span><span style="color:#6A737D;--shiki-dark:#6A737D">            # Update from models.dev</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> update-models</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> embedded</span><span style="color:#6A737D;--shiki-dark:#6A737D">   # Reset to bundled database</span></span></code></pre>`,headings:[{depth:2,text:"Supported providers",id:"supported-providers"},{depth:2,text:"Model string format",id:"model-string-format"},{depth:2,text:"Model aliases",id:"model-aliases"},{depth:2,text:"Specifying a model",id:"specifying-a-model"},{depth:2,text:"Authentication",id:"authentication"},{depth:3,text:"API keys",id:"api-keys"},{depth:3,text:"OAuth",id:"oauth"},{depth:3,text:"Custom provider URL",id:"custom-provider-url"},{depth:2,text:"Model database",id:"model-database"}],raw:`
# Providers

Kit supports a wide range of LLM providers through a unified \`provider/model\` string format.

## Supported providers

| Provider | Prefix | Description |
|----------|--------|-------------|
| **Anthropic** | \`anthropic/\` | Claude models (native, prompt caching, OAuth) |
| **OpenAI** | \`openai/\` | GPT models |
| **Google** | \`google/\` or \`gemini/\` | Gemini models |
| **Ollama** | \`ollama/\` | Local models |
| **Azure OpenAI** | \`azure/\` | Azure-hosted OpenAI |
| **AWS Bedrock** | \`bedrock/\` | Bedrock models |
| **Google Vertex** | \`google-vertex-anthropic/\` | Claude on Vertex AI |
| **OpenRouter** | \`openrouter/\` | Multi-provider router |
| **Vercel AI** | \`vercel/\` | Vercel AI SDK models |
| **Auto-routed** | any | Any provider from the models.dev database |

## Model string format

\`\`\`bash
provider/model            # Standard format
anthropic/claude-sonnet-4-5-20250929
openai/gpt-4o
ollama/llama3
google/gemini-2.0-flash-exp
\`\`\`

## Model aliases

Kit provides aliases for commonly used models:

\`\`\`bash
claude-opus-latest        → claude-opus-4-20250514
claude-sonnet-latest      → claude-sonnet-4-5-20250929
claude-4-opus-latest      → claude-opus-4-20250514
claude-4-sonnet-latest    → claude-sonnet-4-5-20250929
claude-3-7-sonnet-latest  → claude-3-7-sonnet-20250219
claude-3-5-sonnet-latest  → claude-3-5-sonnet-20241022
claude-3-5-haiku-latest   → claude-3-5-haiku-20241022
claude-3-opus-latest      → claude-3-opus-20240229
\`\`\`

## Specifying a model

Via CLI flag:

\`\`\`bash
kit --model openai/gpt-4o
kit -m ollama/llama3
\`\`\`

Via config file:

\`\`\`yaml
model: anthropic/claude-sonnet-4-5-20250929
\`\`\`

Via environment variable:

\`\`\`bash
export KIT_MODEL="google/gemini-2.0-flash-exp"
\`\`\`

## Authentication

### API keys

Set the appropriate environment variable for your provider:

\`\`\`bash
export ANTHROPIC_API_KEY="sk-..."
export OPENAI_API_KEY="sk-..."
export GOOGLE_API_KEY="..."
\`\`\`

Or pass it directly:

\`\`\`bash
kit --provider-api-key "sk-..." --model openai/gpt-4o
\`\`\`

### OAuth

For providers that support OAuth (e.g., Anthropic):

\`\`\`bash
kit auth login anthropic     # Start OAuth flow
kit auth status              # Check authentication status
kit auth logout anthropic    # Remove credentials
\`\`\`

### Custom provider URL

For self-hosted or proxy endpoints:

\`\`\`bash
kit --provider-url "https://my-proxy.example.com/v1" --model openai/gpt-4o
\`\`\`

## Model database

Kit ships with a local model database that maps provider names to API configurations. You can manage it with:

\`\`\`bash
kit models                   # List available models
kit models openai            # Filter by provider
kit models --all             # Show all providers
kit update-models            # Update from models.dev
kit update-models embedded   # Reset to bundled database
\`\`\`
`};export{s as default};

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
<td><strong>Custom</strong></td>
<td><code>custom/</code></td>
<td>Any OpenAI-compatible endpoint</td>
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
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">anthropic/claude-sonnet-latest</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">openai/gpt-4o</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">ollama/llama3</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">google/gemini-2.5-flash</span></span></code></pre>
<h2 id="model-aliases"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#model-aliases"><span class="icon icon-link"></span></a>Model aliases</h2>
<p>Kit provides aliases for commonly used models:</p>
<h3 id="anthropic-claude"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#anthropic-claude"><span class="icon icon-link"></span></a>Anthropic Claude</h3>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">claude-opus-latest</span><span style="color:#032F62;--shiki-dark:#9ECBFF">        →</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> claude-opus-4-6</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">claude-sonnet-latest</span><span style="color:#032F62;--shiki-dark:#9ECBFF">      →</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> claude-sonnet-4-6</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">claude-haiku-latest</span><span style="color:#032F62;--shiki-dark:#9ECBFF">       →</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> claude-haiku-4-5</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">claude-4-opus-latest</span><span style="color:#032F62;--shiki-dark:#9ECBFF">      →</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> claude-opus-4-6</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">claude-4-sonnet-latest</span><span style="color:#032F62;--shiki-dark:#9ECBFF">    →</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> claude-sonnet-4-6</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">claude-4-haiku-latest</span><span style="color:#032F62;--shiki-dark:#9ECBFF">     →</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> claude-haiku-4-5</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">claude-3-7-sonnet-latest</span><span style="color:#032F62;--shiki-dark:#9ECBFF">  →</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> claude-3-7-sonnet-20250219</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">claude-3-5-sonnet-latest</span><span style="color:#032F62;--shiki-dark:#9ECBFF">  →</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> claude-3-5-sonnet-20241022</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">claude-3-5-haiku-latest</span><span style="color:#032F62;--shiki-dark:#9ECBFF">   →</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> claude-3-5-haiku-20241022</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">claude-3-opus-latest</span><span style="color:#032F62;--shiki-dark:#9ECBFF">      →</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> claude-3-opus-20240229</span></span></code></pre>
<h3 id="openai-gpt"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#openai-gpt"><span class="icon icon-link"></span></a>OpenAI GPT</h3>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">o1-latest</span><span style="color:#032F62;--shiki-dark:#9ECBFF">                 →</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> o1</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">o3-latest</span><span style="color:#032F62;--shiki-dark:#9ECBFF">                 →</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> o3</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">o4-latest</span><span style="color:#032F62;--shiki-dark:#9ECBFF">                 →</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> o4-mini</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">gpt-5-latest</span><span style="color:#032F62;--shiki-dark:#9ECBFF">              →</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> gpt-5.4</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">gpt-5-chat-latest</span><span style="color:#032F62;--shiki-dark:#9ECBFF">         →</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> gpt-5.4</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">gpt-4-latest</span><span style="color:#032F62;--shiki-dark:#9ECBFF">              →</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> gpt-4o</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">gpt-4</span><span style="color:#032F62;--shiki-dark:#9ECBFF">                     →</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> gpt-4o</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">gpt-3.5-latest</span><span style="color:#032F62;--shiki-dark:#9ECBFF">            →</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> gpt-3.5-turbo</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">gpt-3.5</span><span style="color:#032F62;--shiki-dark:#9ECBFF">                   →</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> gpt-3.5-turbo</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">codex-latest</span><span style="color:#032F62;--shiki-dark:#9ECBFF">              →</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> codex-mini-latest</span></span></code></pre>
<h3 id="google-gemini"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#google-gemini"><span class="icon icon-link"></span></a>Google Gemini</h3>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">gemini-pro-latest</span><span style="color:#032F62;--shiki-dark:#9ECBFF">         →</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> gemini-2.5-pro</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">gemini-flash-latest</span><span style="color:#032F62;--shiki-dark:#9ECBFF">       →</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> gemini-2.5-flash</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">gemini-flash</span><span style="color:#032F62;--shiki-dark:#9ECBFF">              →</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> gemini-2.5-flash</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">gemini-pro</span><span style="color:#032F62;--shiki-dark:#9ECBFF">                →</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> gemini-2.5-pro</span></span></code></pre>
<h2 id="specifying-a-model"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#specifying-a-model"><span class="icon icon-link"></span></a>Specifying a model</h2>
<p>Via CLI flag:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> --model</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> openai/gpt-4o</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> -m</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> ollama/llama3</span></span></code></pre>
<p>Via config file:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">model</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">anthropic/claude-sonnet-latest</span></span></code></pre>
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
<p>When <code>--provider-url</code> is provided without <code>--model</code>, Kit automatically defaults to <code>custom/custom</code>:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> --provider-url</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> "http://localhost:8080/v1"</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> "Hello"</span></span></code></pre>
<p>The <code>custom/custom</code> model has zero cost, 262K context window, and supports reasoning. It routes through the <code>openaicompat</code> provider and accepts any OpenAI-compatible API endpoint.</p>
<p>Optionally set <code>CUSTOM_API_KEY</code> environment variable or use <code>--provider-api-key</code> for endpoints requiring authentication.</p>
<h2 id="auto-routed-providers"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#auto-routed-providers"><span class="icon icon-link"></span></a>Auto-routed providers</h2>
<p>Any provider in the <a href="https://models.dev">models.dev</a> database can be used with the
standard <code>provider/model</code> format, even without a dedicated native integration. Kit
auto-routes the request through the matching <strong>wire protocol</strong> — the actual API
shape the provider speaks — rather than requiring a per-provider code path:</p>
<table>
<thead>
<tr>
<th>Wire protocol</th>
<th>npm package (models.dev)</th>
<th>Transport used</th>
</tr>
</thead>
<tbody>
<tr>
<td>OpenAI (Responses API)</td>
<td><code>@ai-sdk/openai</code></td>
<td>OpenAI</td>
</tr>
<tr>
<td>OpenAI (chat completions)</td>
<td><code>@ai-sdk/openai-compatible</code></td>
<td>OpenAI-compatible</td>
</tr>
<tr>
<td>Anthropic</td>
<td><code>@ai-sdk/anthropic</code></td>
<td>Anthropic</td>
</tr>
<tr>
<td>Google Gemini</td>
<td><code>@ai-sdk/google</code></td>
<td>Google</td>
</tr>
</tbody>
</table>
<p>The provider's <code>api</code> URL from the database is used as the base URL. A provider
whose npm package isn't recognized but that has an <code>api</code> URL falls back to the
OpenAI-compatible wire.</p>
<p>Because routing follows the wire protocol, aggregator/proxy providers work across
<strong>all</strong> of their models — including ones they re-flavor onto a different protocol
via a per-model override. For example, an aggregator that proxies Claude, GPT,
<em>and</em> Gemini routes them to the Anthropic, OpenAI, and Google transports
respectively:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> --model</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> opencode/claude-haiku-4-5</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> "Hello"</span><span style="color:#6A737D;--shiki-dark:#6A737D">     # → Anthropic wire</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> --model</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> opencode/gpt-5</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> "Hello"</span><span style="color:#6A737D;--shiki-dark:#6A737D">                # → OpenAI wire</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> --model</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> opencode/gemini-3.5-flash</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> "Hello"</span><span style="color:#6A737D;--shiki-dark:#6A737D">     # → Google wire</span></span></code></pre>
<p>Provide the provider's API key the same way as any other — via its environment
variable (e.g. <code>OPENCODE_API_KEY</code>) or <code>--provider-api-key</code>.</p>
<h2 id="model-database"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#model-database"><span class="icon icon-link"></span></a>Model database</h2>
<p>Kit ships with a local model database that maps provider names to API configurations. You can manage it with:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> models</span><span style="color:#6A737D;--shiki-dark:#6A737D">                   # List available models</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> models</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> openai</span><span style="color:#6A737D;--shiki-dark:#6A737D">            # Filter by provider</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> models</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> --all</span><span style="color:#6A737D;--shiki-dark:#6A737D">             # Show all providers</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> update-models</span><span style="color:#6A737D;--shiki-dark:#6A737D">            # Update from models.dev</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> update-models</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> embedded</span><span style="color:#6A737D;--shiki-dark:#6A737D">   # Reset to bundled database</span></span></code></pre>`,headings:[{depth:2,text:"Supported providers",id:"supported-providers"},{depth:2,text:"Model string format",id:"model-string-format"},{depth:2,text:"Model aliases",id:"model-aliases"},{depth:3,text:"Anthropic Claude",id:"anthropic-claude"},{depth:3,text:"OpenAI GPT",id:"openai-gpt"},{depth:3,text:"Google Gemini",id:"google-gemini"},{depth:2,text:"Specifying a model",id:"specifying-a-model"},{depth:2,text:"Authentication",id:"authentication"},{depth:3,text:"API keys",id:"api-keys"},{depth:3,text:"OAuth",id:"oauth"},{depth:3,text:"Custom provider URL",id:"custom-provider-url"},{depth:2,text:"Auto-routed providers",id:"auto-routed-providers"},{depth:2,text:"Model database",id:"model-database"}],raw:`
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
| **Custom** | \`custom/\` | Any OpenAI-compatible endpoint |
| **Auto-routed** | any | Any provider from the models.dev database |

## Model string format

\`\`\`bash
provider/model            # Standard format
anthropic/claude-sonnet-latest
openai/gpt-4o
ollama/llama3
google/gemini-2.5-flash
\`\`\`

## Model aliases

Kit provides aliases for commonly used models:

### Anthropic Claude

\`\`\`bash
claude-opus-latest        → claude-opus-4-6
claude-sonnet-latest      → claude-sonnet-4-6
claude-haiku-latest       → claude-haiku-4-5
claude-4-opus-latest      → claude-opus-4-6
claude-4-sonnet-latest    → claude-sonnet-4-6
claude-4-haiku-latest     → claude-haiku-4-5
claude-3-7-sonnet-latest  → claude-3-7-sonnet-20250219
claude-3-5-sonnet-latest  → claude-3-5-sonnet-20241022
claude-3-5-haiku-latest   → claude-3-5-haiku-20241022
claude-3-opus-latest      → claude-3-opus-20240229
\`\`\`

### OpenAI GPT

\`\`\`bash
o1-latest                 → o1
o3-latest                 → o3
o4-latest                 → o4-mini
gpt-5-latest              → gpt-5.4
gpt-5-chat-latest         → gpt-5.4
gpt-4-latest              → gpt-4o
gpt-4                     → gpt-4o
gpt-3.5-latest            → gpt-3.5-turbo
gpt-3.5                   → gpt-3.5-turbo
codex-latest              → codex-mini-latest
\`\`\`

### Google Gemini

\`\`\`bash
gemini-pro-latest         → gemini-2.5-pro
gemini-flash-latest       → gemini-2.5-flash
gemini-flash              → gemini-2.5-flash
gemini-pro                → gemini-2.5-pro
\`\`\`

## Specifying a model

Via CLI flag:

\`\`\`bash
kit --model openai/gpt-4o
kit -m ollama/llama3
\`\`\`

Via config file:

\`\`\`yaml
model: anthropic/claude-sonnet-latest
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

When \`--provider-url\` is provided without \`--model\`, Kit automatically defaults to \`custom/custom\`:

\`\`\`bash
kit --provider-url "http://localhost:8080/v1" "Hello"
\`\`\`

The \`custom/custom\` model has zero cost, 262K context window, and supports reasoning. It routes through the \`openaicompat\` provider and accepts any OpenAI-compatible API endpoint.

Optionally set \`CUSTOM_API_KEY\` environment variable or use \`--provider-api-key\` for endpoints requiring authentication.

## Auto-routed providers

Any provider in the [models.dev](https://models.dev) database can be used with the
standard \`provider/model\` format, even without a dedicated native integration. Kit
auto-routes the request through the matching **wire protocol** — the actual API
shape the provider speaks — rather than requiring a per-provider code path:

| Wire protocol | npm package (models.dev) | Transport used |
|---------------|--------------------------|----------------|
| OpenAI (Responses API) | \`@ai-sdk/openai\` | OpenAI |
| OpenAI (chat completions) | \`@ai-sdk/openai-compatible\` | OpenAI-compatible |
| Anthropic | \`@ai-sdk/anthropic\` | Anthropic |
| Google Gemini | \`@ai-sdk/google\` | Google |

The provider's \`api\` URL from the database is used as the base URL. A provider
whose npm package isn't recognized but that has an \`api\` URL falls back to the
OpenAI-compatible wire.

Because routing follows the wire protocol, aggregator/proxy providers work across
**all** of their models — including ones they re-flavor onto a different protocol
via a per-model override. For example, an aggregator that proxies Claude, GPT,
*and* Gemini routes them to the Anthropic, OpenAI, and Google transports
respectively:

\`\`\`bash
kit --model opencode/claude-haiku-4-5 "Hello"     # → Anthropic wire
kit --model opencode/gpt-5 "Hello"                # → OpenAI wire
kit --model opencode/gemini-3.5-flash "Hello"     # → Google wire
\`\`\`

Provide the provider's API key the same way as any other — via its environment
variable (e.g. \`OPENCODE_API_KEY\`) or \`--provider-api-key\`.

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

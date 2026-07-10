const e={frontmatter:{title:"Providers",description:"Supported LLM providers and model configuration.",hidden:!1,toc:!0,draft:!1},html:`<h1 id="providers"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#providers"><span class="icon icon-link"></span></a>Providers</h1>
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
<td><strong>GitHub Copilot</strong></td>
<td><code>copilot/</code></td>
<td>Copilot models through GitHub device login (experimental)</td>
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
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">copilot/gpt-5.5</span></span>
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
<p>For providers that support OAuth:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> auth</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> login</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> anthropic</span><span style="color:#6A737D;--shiki-dark:#6A737D">     # Anthropic OAuth</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> auth</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> login</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> openai</span><span style="color:#6A737D;--shiki-dark:#6A737D">        # ChatGPT/Codex OAuth</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> auth</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> login</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> copilot</span><span style="color:#6A737D;--shiki-dark:#6A737D">       # GitHub Copilot device login (experimental)</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> auth</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> status</span><span style="color:#6A737D;--shiki-dark:#6A737D">              # Check authentication status</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> auth</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> logout</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> copilot</span><span style="color:#6A737D;--shiki-dark:#6A737D">      # Remove credentials</span></span></code></pre>
<p>The experimental <code>copilot/</code> provider requires an active GitHub Copilot subscription
and uses GitHub device login; no OpenAI account or OpenAI API key is required.</p>
<h3 id="custom-provider-url"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#custom-provider-url"><span class="icon icon-link"></span></a>Custom provider URL</h3>
<p>For self-hosted or proxy endpoints:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> --provider-url</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> "https://my-proxy.example.com/v1"</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> --model</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> openai/gpt-4o</span></span></code></pre>
<p>When <code>--provider-url</code> is set with an explicit <code>--model</code>, Kit routes through the
<code>custom</code> (OpenAI-compatible) wire and strips any provider prefix from the model
name. So <code>openai/gpt-4o</code>, <code>google/gemma-4-12b</code>, and bare <code>gpt-4o</code> all resolve
to the same endpoint — Kit treats <code>--provider-url</code> as authoritative about <em>where</em>
to send the request, and the model string as just the upstream model id.</p>
<p>This avoids name collisions when a local server (LM Studio, Ollama, vLLM, ...)
happens to expose a model whose name matches a known cloud provider.</p>
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
<h2 id="provider-overrides"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#provider-overrides"><span class="icon icon-link"></span></a>Provider overrides</h2>
<p>The wire protocol is normally inferred from the model database, but you can
declare it explicitly — either to patch a provider the database routes wrongly,
or to define an entirely new provider (e.g. an internal LLM gateway) that the
database doesn't know about. Add a <code>providers</code> section to your config file:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D"># ~/.kit.yml</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">providers</span><span style="color:#24292E;--shiki-dark:#E1E4E8">:</span></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">  # Patch the wire protocol of a known provider</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">  minimax</span><span style="color:#24292E;--shiki-dark:#E1E4E8">:</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">    wire</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">anthropic</span></span>
<span class="line"></span>
<span class="line"><span style="color:#6A737D;--shiki-dark:#6A737D">  # Declare a brand-new provider</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">  corp-llm</span><span style="color:#24292E;--shiki-dark:#E1E4E8">:</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">    name</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">"Corp LLM Gateway"</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">    wire</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">anthropic</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">    baseUrl</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">https://llm.internal.corp/api</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">    apiKeyEnv</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: [</span><span style="color:#032F62;--shiki-dark:#9ECBFF">CORP_LLM_KEY</span><span style="color:#24292E;--shiki-dark:#E1E4E8">, </span><span style="color:#032F62;--shiki-dark:#9ECBFF">LLM_GATEWAY_KEY</span><span style="color:#24292E;--shiki-dark:#E1E4E8">]</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">    headers</span><span style="color:#24292E;--shiki-dark:#E1E4E8">:</span></span>
<span class="line"><span style="color:#22863A;--shiki-dark:#85E89D">      X-Team</span><span style="color:#24292E;--shiki-dark:#E1E4E8">: </span><span style="color:#032F62;--shiki-dark:#9ECBFF">platform</span></span></code></pre>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> --model</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> corp-llm/claude-sonnet-4-5</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> "Hello"</span></span></code></pre>
<p>All fields are optional; unset fields inherit the database values. Accepted
<code>wire</code> values are <code>openai</code> (Responses API), <code>openai-compat</code> (chat completions),
<code>anthropic</code>, and <code>google</code>. <code>apiKeyEnv</code> lists environment variables tried in
order, and <code>headers</code> adds default HTTP headers to every request. See the
<a href="/configuration#provider-overrides">provider override fields reference</a> for
the full field table.</p>
<p>For one-off use without editing config, pass <code>--provider-wire</code> together with
<code>--provider-url</code>. Unlike <code>--provider-url</code> alone (which always speaks the
OpenAI-compatible wire via <code>custom/</code>), this routes the model's own provider
prefix through the declared wire — and works even for providers not in the
database:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> --model</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> corp-llm/claude-sonnet-4-5</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> \\</span></span>
<span class="line"><span style="color:#005CC5;--shiki-dark:#79B8FF">    --provider-url</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> https://llm.internal.corp/api</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> \\</span></span>
<span class="line"><span style="color:#005CC5;--shiki-dark:#79B8FF">    --provider-wire</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> anthropic</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> \\</span></span>
<span class="line"><span style="color:#005CC5;--shiki-dark:#79B8FF">    --provider-api-key</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> "</span><span style="color:#24292E;--shiki-dark:#E1E4E8">$CORP_LLM_KEY</span><span style="color:#032F62;--shiki-dark:#9ECBFF">"</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> "Hello"</span></span></code></pre>
<h3 id="when-to-use-which"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#when-to-use-which"><span class="icon icon-link"></span></a>When to use which</h3>
<table>
<thead>
<tr>
<th>Mechanism</th>
<th>Wire protocols</th>
<th>Scope</th>
<th>Best for</th>
</tr>
</thead>
<tbody>
<tr>
<td><code>--provider-url</code> alone</td>
<td>OpenAI-compatible only</td>
<td>One-off (routes via <code>custom/</code>)</td>
<td>Quick tests against local/OpenAI-compatible endpoints</td>
</tr>
<tr>
<td><a href="/configuration#custom-models"><code>customModels</code></a></td>
<td>OpenAI-compatible only</td>
<td>Per-model, persistent</td>
<td>Self-hosted models needing cost/limit metadata</td>
</tr>
<tr>
<td><a href="/configuration#provider-overrides"><code>providers</code> overrides</a></td>
<td>All four</td>
<td>Per-provider, persistent</td>
<td>Internal gateways, fixing database routing, non-OpenAI wires</td>
</tr>
<tr>
<td><code>--provider-wire</code> + <code>--provider-url</code></td>
<td>All four</td>
<td>One-off</td>
<td>Ad-hoc proxies on any wire, no config edit</td>
</tr>
</tbody>
</table>
<h2 id="model-database"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#model-database"><span class="icon icon-link"></span></a>Model database</h2>
<p>Kit ships with a local model database that maps provider names to API configurations. You can manage it with:</p>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> models</span><span style="color:#6A737D;--shiki-dark:#6A737D">                   # List available models</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> models</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> openai</span><span style="color:#6A737D;--shiki-dark:#6A737D">            # Filter by provider</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> models</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> --all</span><span style="color:#6A737D;--shiki-dark:#6A737D">             # Show all providers</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> update-models</span><span style="color:#6A737D;--shiki-dark:#6A737D">            # Update from models.dev</span></span>
<span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">kit</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> update-models</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> embedded</span><span style="color:#6A737D;--shiki-dark:#6A737D">   # Reset to bundled database</span></span></code></pre>`,headings:[{depth:2,text:"Supported providers",id:"supported-providers"},{depth:2,text:"Model string format",id:"model-string-format"},{depth:2,text:"Model aliases",id:"model-aliases"},{depth:3,text:"Anthropic Claude",id:"anthropic-claude"},{depth:3,text:"OpenAI GPT",id:"openai-gpt"},{depth:3,text:"Google Gemini",id:"google-gemini"},{depth:2,text:"Specifying a model",id:"specifying-a-model"},{depth:2,text:"Authentication",id:"authentication"},{depth:3,text:"API keys",id:"api-keys"},{depth:3,text:"OAuth",id:"oauth"},{depth:3,text:"Custom provider URL",id:"custom-provider-url"},{depth:2,text:"Auto-routed providers",id:"auto-routed-providers"},{depth:2,text:"Provider overrides",id:"provider-overrides"},{depth:3,text:"When to use which",id:"when-to-use-which"},{depth:2,text:"Model database",id:"model-database"}],raw:`
# Providers

Kit supports a wide range of LLM providers through a unified \`provider/model\` string format.

## Supported providers

| Provider | Prefix | Description |
|----------|--------|-------------|
| **Anthropic** | \`anthropic/\` | Claude models (native, prompt caching, OAuth) |
| **OpenAI** | \`openai/\` | GPT models |
| **GitHub Copilot** | \`copilot/\` | Copilot models through GitHub device login (experimental) |
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
copilot/gpt-5.5
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

For providers that support OAuth:

\`\`\`bash
kit auth login anthropic     # Anthropic OAuth
kit auth login openai        # ChatGPT/Codex OAuth
kit auth login copilot       # GitHub Copilot device login (experimental)
kit auth status              # Check authentication status
kit auth logout copilot      # Remove credentials
\`\`\`

The experimental \`copilot/\` provider requires an active GitHub Copilot subscription
and uses GitHub device login; no OpenAI account or OpenAI API key is required.

### Custom provider URL

For self-hosted or proxy endpoints:

\`\`\`bash
kit --provider-url "https://my-proxy.example.com/v1" --model openai/gpt-4o
\`\`\`

When \`--provider-url\` is set with an explicit \`--model\`, Kit routes through the
\`custom\` (OpenAI-compatible) wire and strips any provider prefix from the model
name. So \`openai/gpt-4o\`, \`google/gemma-4-12b\`, and bare \`gpt-4o\` all resolve
to the same endpoint — Kit treats \`--provider-url\` as authoritative about *where*
to send the request, and the model string as just the upstream model id.

This avoids name collisions when a local server (LM Studio, Ollama, vLLM, ...)
happens to expose a model whose name matches a known cloud provider.

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

## Provider overrides

The wire protocol is normally inferred from the model database, but you can
declare it explicitly — either to patch a provider the database routes wrongly,
or to define an entirely new provider (e.g. an internal LLM gateway) that the
database doesn't know about. Add a \`providers\` section to your config file:

\`\`\`yaml
# ~/.kit.yml
providers:
  # Patch the wire protocol of a known provider
  minimax:
    wire: anthropic

  # Declare a brand-new provider
  corp-llm:
    name: "Corp LLM Gateway"
    wire: anthropic
    baseUrl: https://llm.internal.corp/api
    apiKeyEnv: [CORP_LLM_KEY, LLM_GATEWAY_KEY]
    headers:
      X-Team: platform
\`\`\`

\`\`\`bash
kit --model corp-llm/claude-sonnet-4-5 "Hello"
\`\`\`

All fields are optional; unset fields inherit the database values. Accepted
\`wire\` values are \`openai\` (Responses API), \`openai-compat\` (chat completions),
\`anthropic\`, and \`google\`. \`apiKeyEnv\` lists environment variables tried in
order, and \`headers\` adds default HTTP headers to every request. See the
[provider override fields reference](/configuration#provider-overrides) for
the full field table.

For one-off use without editing config, pass \`--provider-wire\` together with
\`--provider-url\`. Unlike \`--provider-url\` alone (which always speaks the
OpenAI-compatible wire via \`custom/\`), this routes the model's own provider
prefix through the declared wire — and works even for providers not in the
database:

\`\`\`bash
kit --model corp-llm/claude-sonnet-4-5 \\
    --provider-url https://llm.internal.corp/api \\
    --provider-wire anthropic \\
    --provider-api-key "$CORP_LLM_KEY" "Hello"
\`\`\`

### When to use which

| Mechanism | Wire protocols | Scope | Best for |
|-----------|---------------|-------|----------|
| \`--provider-url\` alone | OpenAI-compatible only | One-off (routes via \`custom/\`) | Quick tests against local/OpenAI-compatible endpoints |
| [\`customModels\`](/configuration#custom-models) | OpenAI-compatible only | Per-model, persistent | Self-hosted models needing cost/limit metadata |
| [\`providers\` overrides](/configuration#provider-overrides) | All four | Per-provider, persistent | Internal gateways, fixing database routing, non-OpenAI wires |
| \`--provider-wire\` + \`--provider-url\` | All four | One-off | Ad-hoc proxies on any wire, no config edit |

## Model database

Kit ships with a local model database that maps provider names to API configurations. You can manage it with:

\`\`\`bash
kit models                   # List available models
kit models openai            # Filter by provider
kit models --all             # Show all providers
kit update-models            # Update from models.dev
kit update-models embedded   # Reset to bundled database
\`\`\`
`};export{e as default};

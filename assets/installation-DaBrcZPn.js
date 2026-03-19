const n={frontmatter:{title:"Installation",description:"Install Kit using npm, Go, or build from source.",hidden:!1,toc:!0,draft:!1},html:`<h1 id="installation"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#installation"><span class="icon icon-link"></span></a>Installation</h1>
<h2 id="using-npm-recommended"><a class="heading-anchor" aria-hidden="" tabindex="-1" href="#using-npm-recommended"><span class="icon icon-link"></span></a>Using npm (recommended)</h2>
<pre class="shiki shiki-themes github-light github-dark" style="background-color:#fff;--shiki-dark-bg:#24292e;color:#24292e;--shiki-dark:#e1e4e8" tabindex="0"><code><span class="line"><span style="color:#6F42C1;--shiki-dark:#B392F0">npm</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> install</span><span style="color:#005CC5;--shiki-dark:#79B8FF"> -g</span><span style="color:#032F62;--shiki-dark:#9ECBFF"> @mark3labs/kit</span></span></code></pre>
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
<p>See <a href="/providers">Providers</a> for the full list of supported providers and their configuration.</p>`,headings:[{depth:2,text:"Using npm (recommended)",id:"using-npm-recommended"},{depth:2,text:"Using Go",id:"using-go"},{depth:2,text:"Building from source",id:"building-from-source"},{depth:2,text:"Verifying the installation",id:"verifying-the-installation"},{depth:2,text:"Setting up a provider",id:"setting-up-a-provider"}],raw:`
# Installation

## Using npm (recommended)

\`\`\`bash
npm install -g @mark3labs/kit
\`\`\`

## Using Go

\`\`\`bash
go install github.com/mark3labs/kit/cmd/kit@latest
\`\`\`

## Building from source

\`\`\`bash
git clone https://github.com/mark3labs/kit.git
cd kit
go build -o kit ./cmd/kit
\`\`\`

## Verifying the installation

After installing, verify Kit is available:

\`\`\`bash
kit --help
\`\`\`

## Setting up a provider

Kit needs at least one LLM provider configured. Set an API key for your preferred provider:

\`\`\`bash
# Anthropic (default provider)
export ANTHROPIC_API_KEY="sk-..."

# OpenAI
export OPENAI_API_KEY="sk-..."

# Google Gemini
export GOOGLE_API_KEY="..."
\`\`\`

For OAuth-enabled providers like Anthropic, you can also authenticate interactively:

\`\`\`bash
kit auth login anthropic
\`\`\`

See [Providers](/providers) for the full list of supported providers and their configuration.
`};export{n as default};

---
title: Model Providers
description: Register callback-based model backends through trusted extensions.
ogImage: /og-image.png
---

# Model providers (work in progress)

An extension can declare a model backend with `api.RegisterProvider` during
`Init`. The API uses concrete request and event records. The compiled host
adapter implements the model interface; the interpreter does not implement a
host interface.

## Offline example

```sh
kit --bare -e examples/extensions/fake-provider.go --model fake/echo \
  --no-session --no-core-tools --quiet "hello"
```

The output is `Hello from a fake provider.` No API key or network is needed.
The history-driven `fake-tool-provider.go` fixture calls an exposed host `add`
tool with chunked arguments and answers `5` after its matching result.

Declare helper functions before using their names in callbacks. This avoids
Yaegi's forward-reference bug. See the source examples for complete code.

## Ownership and trust

Extensions are trusted Go code, not a sandbox. Put backend resource allocation
in `Create`, not `Init`. A failed Create owns its partial resources. A successful
Create transfers the returned runtime to Kit. Kit serializes calls on one
runtime; separate runtimes can run concurrently. Extension package globals
remain the author's synchronization responsibility.

Only Create receives transient credentials. Do not print or persist creation
requests, keys, or endpoint credentials. API keys resolve from scoped overrides,
stored credentials, then declared environment variables. Externally managed
authentication can provide a safe status callback, not a key. Status callbacks
receive a five-second cancellation signal; safe results have a 30-second cache.
No-auth providers require no key. Kit does not implement extension OAuth login.

Provider IDs are lowercase ASCII identifiers, beginning with a letter and then
letters, digits, `-`, `_`, or `.`. Model IDs are trimmed and case-sensitive; `/`
is allowed. Definitions cannot override native/catalog or SDK factory providers.
Registration errors invalidate a load even if Init ignores them.

## Requests and tools

Requests contain selected-branch history, ordered concrete parts, parameters,
and schemas of the tools actually exposed by Kit. They contain no executable
tool callbacks. **Kit is the sole executor of tools.** The normal hooks,
permission checks and code-mode exposure rules still apply.

Numeric parameter pointers preserve explicit zero. Limits of zero mean unknown.
Prices are USD per million tokens; `Known: true, Value: 0` means free, while
`Known: false` means unknown. Unlisted model IDs require `AllowUnknownModels`.
Unknown lookup does not add the model to the picker. Unknown models have no
invented limits or prices and no advanced capabilities unless declared.

## Streaming protocol

`Stream` calls synchronous `emit` and returns one `ProviderResult`. There is no
error event. Return `ProviderResult.Error` for failure. Stop after emit returns
false and join all emitters before returning. No concurrent emit calls are
allowed. Observe request `Done` for cancellation.

Text and reasoning use start/delta/end events. Every block ID is nonempty and
unique in its request. Tool start names an exposed tool; tool deltas carry JSON
fragments; tool end carries no second full payload. Arguments must form a JSON
object. Distinct open blocks can interleave.

Usage events are cumulative nondecreasing snapshots. Kit counts the final
snapshot once. Exactly one finish is required, with every block closed. Reasons
are `stop`, `length`, and `tool_calls`. Only `tool_calls` may contain completed
executable calls. Kit withholds executable calls and finish until Stream returns
successfully, so a late error or protocol violation cannot execute a tool.

Retryable transport/rate-limit failures can be retried only before content or
tool events have been exposed. After exposure neither request retry nor step
resume repeats the request. Protocol, auth, cancellation and unsupported input
errors are not retried. Panics are converted to safe errors without panic values.

## Catalog scope

Package-level SDK catalog queries remain global. Kit receiver methods include
that Kit's extension metadata. `kit models` remains database-only by default.

```sh
kit --bare -e examples/extensions/fake-provider.go models --extensions fake
```

`--extensions` executes trusted top-level extension code and Init, but does not
create a model, start MCP, or emit session lifecycle events. `--no-extensions`
disables it; `--bare` restricts discovery to explicit paths.

## Lifecycle and persistence

Runtime Close is idempotent and never runs concurrently with Stream. Shutdown
cancels active requests and has a default 30-second provider wait budget,
shortened by CloseContext's deadline. Arbitrary interpreted callbacks cannot be
killed safely. A timeout is not proof that resources have been released; Kit
retains ownership and closes once the callback returns.

Reload validates a strict candidate before replacing registrations. Existing
runtimes retain their old callbacks; their generation's shutdown waits for those
runtimes to close. Arbitrary side effects in Init cannot be rolled back.

Extension-origin history uses additive persistence fields to preserve exact IDs,
ordered text/reasoning, signature-only reasoning, filenames, and namespaced
version-1 JSON replay data. Do not store credentials in replay payloads. Unknown
replay versions or namespaces are unsupported input, not text fallbacks.

Object generation, catalog modes and decision models are unsupported for
extension providers in this release.

## Remaining integration work

This guide describes the intended contract. The current branch is not ready for
release: transactional publication under concurrent reload, complete replay
round-trips (including tool-result and message-level metadata), auth refresh,
reentrant shutdown rejection, complete capability reporting and acceptance-test
coverage still need validation. Do not treat passing baseline tests as proof of
those guarantees.

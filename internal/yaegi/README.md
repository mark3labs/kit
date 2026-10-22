# Local interpreter patch

This directory contains the interpreter from
[`github.com/traefik/yaegi` v0.16.1](https://github.com/traefik/yaegi/tree/v0.16.1).
The upstream Apache-2.0 license is in `LICENSE`. The standard-library export
tables still come from the pinned upstream module.

Kit needs concurrent callbacks from separate provider runtimes in the same
interpreter. Upstream `_select` captures a mutable `[]reflect.SelectCase` in
its compiled node. Concurrent executions write channel values into that same
slice. This races even when extension code has no shared mutable state.

## Changes from upstream

- `interp/run.go`: copy the compiled select-case template into an
  invocation-local slice before setting channel values and cancellation state.
  The compiled template is immutable after compilation. No lock is held while
  waiting on a channel, so independent calls can proceed concurrently.
- `interp/type.go`: point the private `unsafe2` import at this directory.
- Copy only production interpreter files and the two `unsafe2` build variants.
  Generated standard-library tables remain upstream dependencies.

Do not edit the shared module cache or use an absolute `replace` directive.
Both the production extension loader and the public test harness use this
interpreter. Keep their imports aligned.

## Verification

`internal/extensions/provider_concurrency_probe_test.go` exercises concrete
nested create/stream/close callback shapes. It creates separate runtimes from
one interpreter, invokes them concurrently, checks synchronous emission, and
uses channel barriers to verify that canceling some calls does not block the
others. It does not rely on shared extension variables or sleeps.

Run:

```sh
go test -race ./internal/extensions -run TestProviderConcurrencyProbe -count=20
go test -race ./...
```

The test must fail under the race detector if the slice-copy patch is removed.
This patch does **not** make extension package globals thread-safe or promise
that every interpreter operation is safe concurrently. Provider integration
still needs real interpreted concurrency tests for every callback boundary.

When updating Yaegi, compare this directory with the new upstream version and
re-run the regression tests. Remove this copy when an upstream release fixes
the problem and passes Kit's tests. Do not replace the patch with a mutex held
across callbacks: a blocked stream must not prevent a separate runtime from
observing cancellation or producing output.

package agent

import "context"

// steerChKey is the context key for the steer channel.
type steerChKey struct{}

// steerConsumedKey is the context key for the steer-consumed callback.
type steerConsumedKey struct{}

// ContextWithSteerCh returns a new context with the steer channel attached.
// The agent's PrepareStep function checks this channel between steps and
// injects any pending steer messages as user messages before the next LLM call.
func ContextWithSteerCh(ctx context.Context, ch <-chan string) context.Context {
	return context.WithValue(ctx, steerChKey{}, ch)
}

// ContextWithSteerConsumed returns a new context with a callback that fires
// when steer messages are consumed by PrepareStep. The count argument is the
// number of messages injected in this batch.
func ContextWithSteerConsumed(ctx context.Context, fn func(count int)) context.Context {
	return context.WithValue(ctx, steerConsumedKey{}, fn)
}

// steerChFromContext extracts the steer channel from the context, or nil.
func steerChFromContext(ctx context.Context) <-chan string {
	ch, _ := ctx.Value(steerChKey{}).(<-chan string)
	return ch
}

// steerConsumedFromContext extracts the steer-consumed callback, or nil.
func steerConsumedFromContext(ctx context.Context) func(int) {
	fn, _ := ctx.Value(steerConsumedKey{}).(func(int))
	return fn
}

package extensions

import (
	"context"
	"errors"
	"sync"
)

// ProviderOwner keeps interpreter callbacks alive until all successful runtimes
// are closed. Retirement stops new creates but never invalidates old runtimes.
type ProviderOwner struct {
	mu                sync.Mutex
	active            int
	retired           bool
	shuttingDown      bool
	shutdownScheduled bool
	shutdown          func()
	runtimes          map[*providerAdapter]struct{}
	calls             map[int64]context.CancelFunc
	drained           chan struct{}
}

func (o *ProviderOwner) acquire() bool {
	if o == nil {
		return true
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.retired {
		return false
	}
	if o.active == 0 {
		o.drained = make(chan struct{})
	}
	o.active++
	return true
}
func (o *ProviderOwner) release() {
	if o == nil {
		return
	}
	o.mu.Lock()
	o.active--
	if o.active == 0 && o.drained != nil {
		close(o.drained)
	}
	var shutdown func()
	if o.retired && o.active == 0 {
		shutdown = o.shutdown
		o.shutdown = nil
	}
	o.mu.Unlock()
	if shutdown != nil {
		shutdown()
	}
}

// Retire defers interpreter shutdown until the last runtime has closed.
// It invokes no extension callbacks while holding the owner lock.
func (o *ProviderOwner) Retire(shutdown func()) {
	if o == nil {
		shutdown()
		return
	}
	o.mu.Lock()
	if o.shutdownScheduled {
		o.mu.Unlock()
		return
	}
	o.retired = true
	o.shutdownScheduled = true
	if o.active == 0 {
		o.mu.Unlock()
		shutdown()
		return
	}
	o.shutdown = shutdown
	o.mu.Unlock()
}

func (o *ProviderOwner) track(runtime *providerAdapter) bool {
	if o == nil {
		return true
	}
	o.mu.Lock()
	if o.shuttingDown {
		o.mu.Unlock()
		return false
	}
	if o.runtimes == nil {
		o.runtimes = map[*providerAdapter]struct{}{}
	}
	o.runtimes[runtime] = struct{}{}
	o.mu.Unlock()
	return true
}
func (o *ProviderOwner) untrack(runtime *providerAdapter) {
	if o == nil {
		return
	}
	o.mu.Lock()
	delete(o.runtimes, runtime)
	o.mu.Unlock()
}

// CheckReentrant rejects shutdown or reload from a runtime's active callback.
func (o *ProviderOwner) CheckReentrant() error {
	if o == nil {
		return nil
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if _, ok := o.calls[goroutineID()]; ok {
		return errors.New("cannot retire provider generation from its own callback")
	}
	for runtime := range o.runtimes {
		if runtime.callbackGID.Load() == goroutineID() {
			return errors.New("cannot retire provider generation from its own callback")
		}
	}
	return nil
}

// CloseRuntimes cancels all owned runtimes before waiting for any of them.
// It must not be called synchronously from a provider callback.
func (o *ProviderOwner) CloseRuntimes(ctx context.Context) error {
	if o == nil {
		return nil
	}
	if err := o.CancelOwnedWork(); err != nil {
		return err
	}
	o.mu.Lock()
	drained := o.drained
	runtimes := make([]*providerAdapter, 0, len(o.runtimes))
	for runtime := range o.runtimes {
		runtimes = append(runtimes, runtime)
	}
	o.mu.Unlock()
	for _, runtime := range runtimes {
		runtime.cancel()
	}
	var errs []error
	for _, runtime := range runtimes {
		if err := runtime.closeContext(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	if drained != nil {
		select {
		case <-drained:
		case <-ctx.Done():
			errs = append(errs, ctx.Err())
		}
	}
	return errors.Join(errs...)
}

func (o *ProviderOwner) callback(ctx context.Context) (context.Context, func(), error) {
	ctx, cancel := context.WithCancel(ctx)
	if o == nil {
		return ctx, cancel, nil
	}
	gid := goroutineID()
	o.mu.Lock()
	if o.shuttingDown {
		o.mu.Unlock()
		cancel()
		return nil, nil, errors.New("extension provider generation is shutting down")
	}
	if o.calls == nil {
		o.calls = map[int64]context.CancelFunc{}
	}
	o.calls[gid] = cancel
	o.mu.Unlock()
	return ctx, func() { cancel(); o.mu.Lock(); delete(o.calls, gid); o.mu.Unlock() }, nil
}

// CancelOwnedWork stops all request classes without waiting for callbacks.
func (o *ProviderOwner) CancelOwnedWork() error {
	if o == nil {
		return nil
	}
	o.mu.Lock()
	if _, ok := o.calls[goroutineID()]; ok {
		o.mu.Unlock()
		return errors.New("cannot shut down provider generation from its own callback")
	}
	for runtime := range o.runtimes {
		if runtime.callbackGID.Load() == goroutineID() {
			o.mu.Unlock()
			return errors.New("cannot shut down provider generation from its own callback")
		}
	}
	o.retired = true
	o.shuttingDown = true
	var cancels []context.CancelFunc
	for _, cancel := range o.calls {
		cancels = append(cancels, cancel)
	}
	for runtime := range o.runtimes {
		cancels = append(cancels, runtime.cancel)
	}
	o.mu.Unlock()
	for _, cancel := range cancels {
		cancel()
	}
	return nil
}

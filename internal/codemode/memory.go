package codemode

import (
	"context"
	"runtime"
	"runtime/metrics"
	"time"
)

// The JavaScript engine has no per-runtime heap limit, so code mode uses a
// watchdog instead. It samples the process heap and interrupts the script
// when the heap grows by more than the limit above its level at script
// start. This is approximate: the heap is shared with the rest of the
// process, so it guards against runaway scripts, not against a precise
// byte budget.

const (
	memorySampleInterval = 50 * time.Millisecond
	memoryGCInterval     = time.Second
	heapMetric           = "/memory/classes/heap/objects:bytes"
)

func heapBytes() int64 {
	s := []metrics.Sample{{Name: heapMetric}}
	metrics.Read(s)
	if s[0].Value.Kind() != metrics.KindUint64 {
		return 0
	}
	return int64(s[0].Value.Uint64())
}

// watchMemory calls onExceed once when heap growth exceeds limit. It
// returns when ctx ends. Before it reports, it runs a garbage collection
// (at most once per second) so dead objects do not count.
func watchMemory(ctx context.Context, limit int64, onExceed func()) {
	base := heapBytes()
	ticker := time.NewTicker(memorySampleInterval)
	defer ticker.Stop()
	var lastGC time.Time
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		if heapBytes()-base <= limit {
			continue
		}
		if time.Since(lastGC) >= memoryGCInterval {
			runtime.GC()
			lastGC = time.Now()
			if heapBytes()-base <= limit {
				continue
			}
		}
		onExceed()
		return
	}
}

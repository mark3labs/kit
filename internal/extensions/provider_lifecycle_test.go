package extensions

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"charm.land/fantasy"
)

func TestProviderCloseCancelsDrainsAndClosesOnce(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	returned := make(chan struct{})
	var closed atomic.Int32
	def := ProviderDef{ID: "fake", AllowUnknownModels: true, Create: func(ProviderCreateRequest) ProviderCreateResult {
		return ProviderCreateResult{Runtime: ProviderRuntime{
			Stream: func(req ProviderRequest, _ func(ProviderEvent) bool) ProviderResult {
				close(entered)
				<-req.Done
				<-release
				return ProviderResult{}
			},
			Close: func() error { closed.Add(1); return nil },
		}}
	}}
	model, closeModel, err := NewProviderModel(context.Background(), def, ProviderCreateRequest{Model: "echo"})
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		defer close(returned)
		stream, err := model.Stream(context.Background(), fantasy.Call{})
		if err != nil {
			t.Error(err)
			return
		}
		for range stream {
		}
	}()
	<-entered
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	err = closeModel(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("close=%v", err)
	}
	if closed.Load() != 0 {
		t.Error("Close ran concurrently with Stream")
	}
	close(release)
	<-returned
	if err := closeModel(context.Background()); err != nil {
		t.Error(err)
	}
	if err := closeModel(context.Background()); err != nil {
		t.Error(err)
	}
	if closed.Load() != 1 {
		t.Errorf("close count=%d", closed.Load())
	}
}

func TestProviderOwnerRetirementWaitsForRuntimes(t *testing.T) {
	owner := &ProviderOwner{}
	if !owner.acquire() {
		t.Fatal("acquire")
	}
	shutdown := false
	owner.Retire(func() { shutdown = true })
	if shutdown || owner.acquire() {
		t.Fatal("retired owner accepted work or shut down early")
	}
	owner.release()
	if !shutdown {
		t.Fatal("owner not shut down")
	}
	owner.Retire(func() { t.Error("duplicate shutdown") })
}

func TestProviderReentrantCloseRejected(t *testing.T) {
	var closeModel func(context.Context) error
	def := ProviderDef{ID: "fake", AllowUnknownModels: true, Create: func(ProviderCreateRequest) ProviderCreateResult {
		return ProviderCreateResult{Runtime: ProviderRuntime{Stream: func(_ ProviderRequest, emit func(ProviderEvent) bool) ProviderResult {
			if err := closeModel(context.Background()); err == nil {
				t.Error("reentrant close accepted")
			}
			emit(ProviderEvent{Type: ProviderFinish, FinishReason: ProviderFinishStop})
			return ProviderResult{}
		}}}
	}}
	model, close, err := NewProviderModel(context.Background(), def, ProviderCreateRequest{Model: "echo"})
	if err != nil {
		t.Fatal(err)
	}
	closeModel = close
	if _, err := model.Generate(context.Background(), fantasy.Call{}); err != nil {
		t.Fatal(err)
	}
	if err := closeModel(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestProviderCallbackPanicIsSafe(t *testing.T) {
	runtime := ProviderRuntime{Stream: func(ProviderRequest, func(ProviderEvent) bool) ProviderResult { panic("private credential") }}
	err := runProviderStream(context.Background(), runtime, ProviderRequest{}, func(fantasy.StreamPart) bool { return true })
	if err == nil || err.Error() != "panic in extension provider Stream" {
		t.Fatalf("err=%v", err)
	}
}

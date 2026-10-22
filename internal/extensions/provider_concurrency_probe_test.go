package extensions

import (
	"os"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/mark3labs/kit/internal/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"
)

type probeRequest struct {
	Done    <-chan struct{}
	Entered func(string)
	Release <-chan struct{}
	Text    string
}
type probeResult struct {
	Text   string
	Stream func(probeRequest, func(string) bool) string
	Close  func() error
}
type probeDef struct {
	Create func(probeRequest) probeResult
}

// Exercise the proposed callback shapes before exposing a public contract.
func TestProviderConcurrencyProbe(t *testing.T) {
	i := interp.New(interp.Options{Env: os.Environ()})
	if err := i.Use(stdlib.Symbols); err != nil {
		t.Fatal(err)
	}
	if err := i.Use(interp.Exports{"probe/ext/ext": {
		"Request": reflect.ValueOf((*probeRequest)(nil)),
		"Result":  reflect.ValueOf((*probeResult)(nil)),
		"Def":     reflect.ValueOf((*probeDef)(nil)),
	}}); err != nil {
		t.Fatal(err)
	}
	_, err := i.Eval(`package main
import "probe/ext"
func helper(req ext.Request) ext.Result {
 return ext.Result{Stream: func(req ext.Request, emit func(string) bool) string {
  if req.Entered != nil {
    req.Entered(req.Text)
    select { case <-req.Done: return "canceled"; case <-req.Release: }
  } else {
    select { case <-req.Done: return "canceled"; default: }
  }
  if !emit(req.Text) { return "stopped" }
  return "ok"
 }, Close: func() error { return nil }}
}
func define() ext.Def { return ext.Def{Create: helper} }
`)
	if err != nil {
		t.Fatal(err)
	}
	value, err := i.Eval("define()")
	if err != nil {
		t.Fatal(err)
	}
	def := value.Interface().(probeDef)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for range 8 {
		runtime := def.Create(probeRequest{})
		wg.Go(func() {
			<-start
			for range 100 {
				result := runtime.Stream(probeRequest{Text: "hello"}, func(s string) bool { return s == "hello" })
				if result != "ok" {
					t.Errorf("stream result = %q", result)
					return
				}
				if err := runtime.Close(); err != nil {
					t.Error(err)
				}
			}
		})
	}
	close(start)
	wg.Wait()

	// All streams must enter together; cancel half and release the rest.
	entered := make(chan string, 8)
	results := make(chan string, 8)
	release := make(chan struct{})
	var cancel []chan struct{}
	for range 8 {
		runtime := def.Create(probeRequest{})
		done := make(chan struct{})
		cancel = append(cancel, done)
		wg.Go(func() {
			results <- runtime.Stream(probeRequest{Done: done, Entered: func(s string) { entered <- s }, Release: release, Text: "hello"}, func(string) bool { return true })
		})
	}
	defer func() { close(release); wg.Wait() }()
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	for range 8 {
		select {
		case <-entered:
		case <-deadline.C:
			t.Fatal("separate runtimes did not enter concurrently")
		}
	}
	for _, done := range cancel[:4] {
		close(done)
	}
	for range 4 {
		select {
		case result := <-results:
			if result != "canceled" {
				t.Errorf("result = %q", result)
			}
		case <-deadline.C:
			t.Fatal("cancellation did not release stream")
		}
	}
}

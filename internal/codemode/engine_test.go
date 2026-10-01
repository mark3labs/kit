package codemode

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func testCatalog() *Catalog {
	return NewCatalog([]ToolSpec{
		{
			Name:        "read",
			Description: "Read a file.",
			Parameters:  map[string]any{"path": map[string]any{"type": "string"}},
			Required:    []string{"path"},
		},
		{Name: "echo", Description: "Echo the input back as JSON."},
		{Name: "fail", Description: "Always fails."},
		{Name: "slow", Description: "Sleeps 50ms."},
		{
			Name:        "github__list_issues",
			Description: "List issues of a repository.",
			Parameters: map[string]any{
				"repo":  map[string]any{"type": "string"},
				"state": map[string]any{"type": "string", "enum": []any{"open", "closed"}},
			},
			Required: []string{"repo"},
			Exposure: ExposureCodeMode,
		},
		{Name: "secret", Description: "Hidden.", Exposure: ExposureModelOnly},
		{Name: "linear__search", Description: "Search linear tickets.", Exposure: ExposureDeferred},
	})
}

type fakeInvoker struct {
	mu        sync.Mutex
	calls     []Call
	active    atomic.Int32
	maxActive atomic.Int32
}

func (f *fakeInvoker) invoke(ctx context.Context, c Call) (InvokeResult, error) {
	f.mu.Lock()
	f.calls = append(f.calls, c)
	f.mu.Unlock()
	n := f.active.Add(1)
	defer f.active.Add(-1)
	for {
		m := f.maxActive.Load()
		if n <= m || f.maxActive.CompareAndSwap(m, n) {
			break
		}
	}
	switch c.Name {
	case "read":
		var a struct{ Path string }
		_ = json.Unmarshal([]byte(c.Args), &a)
		return InvokeResult{Text: "content of " + a.Path}, nil
	case "echo":
		return InvokeResult{Text: c.Args}, nil
	case "fail":
		return InvokeResult{Text: "boom happened", IsError: true}, nil
	case "slow":
		select {
		case <-time.After(50 * time.Millisecond):
			return InvokeResult{Text: "slow done"}, nil
		case <-ctx.Done():
			return InvokeResult{}, ctx.Err()
		}
	case "github__list_issues":
		return InvokeResult{Text: `[{"n":1,"state":"open"},{"n":2,"state":"closed"},{"n":3,"state":"open"}]`}, nil
	case "linear__search":
		return InvokeResult{Text: "ticket"}, nil
	}
	return InvokeResult{}, fmt.Errorf("no such tool %s", c.Name)
}

func run(t *testing.T, code string, mod ...func(*Program)) (Result, *fakeInvoker) {
	t.Helper()
	f := &fakeInvoker{}
	p := Program{Code: code, Catalog: testCatalog(), Invoke: f.invoke, Limits: Limits{Timeout: 5 * time.Second}}
	for _, m := range mod {
		m(&p)
	}
	return Run(context.Background(), p), f
}

func mustOK(t *testing.T, r Result) {
	t.Helper()
	if !r.OK {
		t.Fatalf("expected success, got %+v", r.Error)
	}
}

func mustFail(t *testing.T, r Result, kind ErrorKind) {
	t.Helper()
	if r.OK {
		t.Fatalf("expected %s failure, got success with output %q", kind, r.Output)
	}
	if r.Error.Kind != kind {
		t.Fatalf("expected kind %s, got %s: %s", kind, r.Error.Kind, r.Error.Message)
	}
}

func TestReturnValue(t *testing.T) {
	r, _ := run(t, `const x = await tools.read({path: "a.txt"}); return x.toUpperCase()`)
	mustOK(t, r)
	if r.Output != "CONTENT OF A.TXT" {
		t.Fatalf("output = %q", r.Output)
	}
	if len(r.Calls) != 1 || r.Calls[0].Status != CallOK || r.Calls[0].Path != "read" {
		t.Fatalf("calls = %+v", r.Calls)
	}
}

func TestReturnObjectIsJSON(t *testing.T) {
	r, _ := run(t, `return {a: 1, b: [1, 2]}`)
	mustOK(t, r)
	var v map[string]any
	if err := json.Unmarshal([]byte(r.Output), &v); err != nil {
		t.Fatalf("output not JSON: %q", r.Output)
	}
}

func TestNamespacedToolAndFiltering(t *testing.T) {
	r, _ := run(t, `
const issues = JSON.parse(await tools.github.list_issues({repo: "kit"}))
return issues.filter(i => i.state === "open").map(i => i.n)`)
	mustOK(t, r)
	if strings.Join(strings.Fields(r.Output), "") != "[1,3]" {
		t.Fatalf("output = %q", r.Output)
	}
}

func TestParallelCallsRespectConcurrency(t *testing.T) {
	r, f := run(t, `
const res = await Promise.all(Array.from({length: 10}, () => tools.slow()))
return res.length`, func(p *Program) { p.Limits.MaxConcurrency = 3 })
	mustOK(t, r)
	if r.Output != "10" {
		t.Fatalf("output = %q", r.Output)
	}
	if got := f.maxActive.Load(); got > 3 || got < 2 {
		t.Fatalf("max concurrent calls = %d, want 2..3", got)
	}
}

func TestParallelIsFaster(t *testing.T) {
	start := time.Now()
	r, _ := run(t, `await Promise.all([tools.slow(), tools.slow(), tools.slow(), tools.slow()])`)
	mustOK(t, r)
	if el := time.Since(start); el > 180*time.Millisecond {
		t.Fatalf("parallel calls took %s, expected about 50ms", el)
	}
}

func TestToolErrorIsCatchable(t *testing.T) {
	r, _ := run(t, `
try { await tools.fail() } catch (e) { return e.name + "|" + e.tool + "|" + e.output }`)
	mustOK(t, r)
	if r.Output != "ToolError|fail|boom happened" {
		t.Fatalf("output = %q", r.Output)
	}
	if r.Calls[0].Status != CallError {
		t.Fatalf("call status = %s", r.Calls[0].Status)
	}
}

func TestUncaughtToolError(t *testing.T) {
	r, _ := run(t, "const a = 1\nawait tools.fail()")
	mustFail(t, r, ErrToolFailure)
	if !strings.Contains(r.Error.Message, "boom happened") {
		t.Fatalf("message = %q", r.Error.Message)
	}
}

func TestAllSettled(t *testing.T) {
	r, _ := run(t, `
const rs = await Promise.allSettled([tools.read({path: "x"}), tools.fail()])
return rs.map(r => r.status).join(",")`)
	mustOK(t, r)
	if r.Output != "fulfilled,rejected" {
		t.Fatalf("output = %q", r.Output)
	}
}

func TestUnknownToolSuggests(t *testing.T) {
	r, _ := run(t, `await tools.raed({path: "x"})`)
	mustFail(t, r, ErrUnknownTool)
	if !strings.Contains(r.Error.Hint, "tools.read") {
		t.Fatalf("hint = %q", r.Error.Hint)
	}
	r, _ = run(t, `await tools.github.list_issue({repo: "x"})`)
	mustFail(t, r, ErrUnknownTool)
	if !strings.Contains(r.Error.Message, "github.list_issue") {
		t.Fatalf("message = %q", r.Error.Message)
	}
}

func TestModelOnlyToolNotCallable(t *testing.T) {
	r, _ := run(t, `await tools.secret()`)
	mustFail(t, r, ErrUnknownTool)
}

func TestDeferredToolCallable(t *testing.T) {
	r, _ := run(t, `return await tools.linear.search({})`)
	mustOK(t, r)
	if r.Output != "ticket" {
		t.Fatalf("output = %q", r.Output)
	}
}

func TestMissingRequiredArg(t *testing.T) {
	r, f := run(t, `await tools.read({})`)
	mustFail(t, r, ErrInvalidToolInput)
	if len(f.calls) != 0 {
		t.Fatalf("invoker should not run, got %d calls", len(f.calls))
	}
	r, _ = run(t, `await tools.read("a.txt")`)
	mustFail(t, r, ErrInvalidToolInput)
}

func TestToolCallLimit(t *testing.T) {
	r, f := run(t, `for (let i = 0; i < 10; i++) await tools.echo({i})`, func(p *Program) { p.Limits.MaxToolCalls = 3 })
	mustFail(t, r, ErrToolCallLimit)
	if len(f.calls) != 3 {
		t.Fatalf("calls = %d, want 3", len(f.calls))
	}
}

func TestBusyLoopTimeout(t *testing.T) {
	start := time.Now()
	r, _ := run(t, `while (true) {}`, func(p *Program) { p.Limits.Timeout = 100 * time.Millisecond })
	mustFail(t, r, ErrTimeout)
	if time.Since(start) > 2*time.Second {
		t.Fatalf("timeout took too long")
	}
}

func TestAwaitTimeoutCancelsTools(t *testing.T) {
	r, _ := run(t, `await sleep(10000)`, func(p *Program) { p.Limits.Timeout = 100 * time.Millisecond })
	mustFail(t, r, ErrTimeout)

	r, _ = run(t, `await tools.slow(); await tools.slow(); await tools.slow()`, func(p *Program) { p.Limits.Timeout = 80 * time.Millisecond })
	mustFail(t, r, ErrTimeout)
	last := r.Calls[len(r.Calls)-1]
	if last.Status != CallCancelled {
		t.Fatalf("last call status = %s, want cancelled", last.Status)
	}
}

func TestParentCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(50*time.Millisecond, cancel)
	f := &fakeInvoker{}
	r := Run(ctx, Program{Code: `while (true) {}`, Catalog: testCatalog(), Invoke: f.invoke})
	mustFail(t, r, ErrCancelled)
}

func TestParseErrorLine(t *testing.T) {
	r, _ := run(t, "const a = 1\nconst b = ;\n")
	mustFail(t, r, ErrParse)
	if r.Error.Line != 2 {
		t.Fatalf("line = %d, message = %q", r.Error.Line, r.Error.Message)
	}
	if !strings.Contains(r.Error.Message, "Line 2:") {
		t.Fatalf("message line not rewritten: %q", r.Error.Message)
	}
}

func TestRuntimeErrorLine(t *testing.T) {
	r, _ := run(t, "const a = 1\nconst b = null\nreturn b.x")
	mustFail(t, r, ErrExecution)
	if r.Error.Line != 3 {
		t.Fatalf("line = %d (%s)", r.Error.Line, r.Error.Message)
	}
	if !strings.HasPrefix(r.Error.Message, "TypeError") {
		t.Fatalf("message = %q", r.Error.Message)
	}
}

func TestThrowString(t *testing.T) {
	r, _ := run(t, `throw "nope"`)
	mustFail(t, r, ErrExecution)
	if !strings.Contains(r.Error.Message, "nope") {
		t.Fatalf("message = %q", r.Error.Message)
	}
}

func TestUnsettledPromise(t *testing.T) {
	r, _ := run(t, `await new Promise(() => {})`)
	mustFail(t, r, ErrUnsettled)
}

func TestStackOverflow(t *testing.T) {
	r, _ := run(t, `function f() { return f() + 1 }; return f()`)
	mustFail(t, r, ErrExecution)
}

func TestTextAndConsole(t *testing.T) {
	var live []string
	r, _ := run(t, `
console.log("hello", {a: 1})
console.warn("careful")
text("part one")
text({b: 2})
return "done"`, func(p *Program) {
		p.Observer.OnLog = func(s string) { live = append(live, s) }
	})
	mustOK(t, r)
	if len(r.Logs) != 2 || !strings.HasPrefix(r.Logs[0], "hello {") || r.Logs[1] != "[warn] careful" {
		t.Fatalf("logs = %q", r.Logs)
	}
	if len(live) != 2 {
		t.Fatalf("live logs = %q", live)
	}
	if !strings.HasPrefix(r.Output, "part one\n{") || !strings.HasSuffix(r.Output, "\ndone") {
		t.Fatalf("output = %q", r.Output)
	}
}

func TestStorePersistsAcrossRuns(t *testing.T) {
	store := NewStore()
	withStore := func(p *Program) { p.Store = store }
	r, _ := run(t, `store("k", {n: 41}); return load("k").n`, withStore)
	mustOK(t, r)
	r, _ = run(t, `const v = load("k"); v.n++; store("k", v); return load("k").n + "," + String(load("missing"))`, withStore)
	mustOK(t, r)
	if r.Output != "42,undefined" {
		t.Fatalf("output = %q", r.Output)
	}
	r, _ = run(t, `store("k", undefined); return load("k") === undefined`, withStore)
	mustOK(t, r)
	if r.Output != "true" {
		t.Fatalf("output = %q", r.Output)
	}
}

func TestDiscoveryHelpers(t *testing.T) {
	r, _ := run(t, `
const hits = searchTools("linear tickets")
const d = describeTool("github.list_issues")
return [hits[0].name, hits[0].path, d.includes("state?: \"open\" | \"closed\""), ALL_TOOLS.length, call("read", {path: "z"}) instanceof Promise].join("|")`)
	mustOK(t, r)
	// ALL_TOOLS excludes the model-only tool.
	if r.Output != `linear__search|tools.linear.search|true|6|true` {
		t.Fatalf("output = %q", r.Output)
	}
}

func TestCallByName(t *testing.T) {
	r, _ := run(t, `return await call("github__list_issues", {repo: "a"})`)
	mustOK(t, r)
	r, _ = run(t, `return await call("nope", {})`)
	mustFail(t, r, ErrUnknownTool)
}

func TestObserver(t *testing.T) {
	var starts, ends int
	r, _ := run(t, `await Promise.all([tools.read({path: "a"}), tools.echo({})])`, func(p *Program) {
		p.CallIDPrefix = "call_1"
		p.Observer.OnCallStart = func(c Call) {
			starts++
			if !strings.HasPrefix(c.ID, "call_1.") {
				t.Errorf("call id = %q", c.ID)
			}
		}
		p.Observer.OnCallEnd = func(c Call) { ends++ }
	})
	mustOK(t, r)
	if starts != 2 || ends != 2 {
		t.Fatalf("starts=%d ends=%d", starts, ends)
	}
}

func TestObserverOrder(t *testing.T) {
	var events []string
	r, _ := run(t, `await tools.echo({}); console.log("after")`, func(p *Program) {
		p.Observer.OnCallStart = func(c Call) { events = append(events, "start") }
		p.Observer.OnCallEnd = func(c Call) { events = append(events, "end") }
		p.Observer.OnLog = func(s string) { events = append(events, s) }
	})
	mustOK(t, r)
	if strings.Join(events, ",") != "start,end,after" {
		t.Fatalf("events = %v", events)
	}
}

func TestFencedCode(t *testing.T) {
	r, _ := run(t, "```js\nreturn 1 + 1\n```")
	mustOK(t, r)
	if r.Output != "2" {
		t.Fatalf("output = %q", r.Output)
	}
}

func TestNoAmbientAccess(t *testing.T) {
	r, _ := run(t, `return [typeof require, typeof process, typeof fetch, typeof setTimeout].join(",")`)
	mustOK(t, r)
	if r.Output != "undefined,undefined,undefined,undefined" {
		t.Fatalf("output = %q", r.Output)
	}
}

func TestMemoryLimit(t *testing.T) {
	if testing.Short() {
		t.Skip("allocates memory")
	}
	r, _ := run(t, `const a = []; while (true) a.push("x".repeat(1024) + a.length)`, func(p *Program) {
		p.Limits.MemoryLimit = 64 << 20
		p.Limits.Timeout = 20 * time.Second
	})
	mustFail(t, r, ErrMemoryLimit)
}

func TestToolsObjectInspection(t *testing.T) {
	// Promise resolution and JSON probe properties such as "then" and
	// "toJSON"; the unknown-tool guard must not trip on them.
	r, _ := run(t, `const t = await Promise.resolve(tools); return Object.keys(t).sort().join(",") + "|" + JSON.stringify(tools.github)`)
	mustOK(t, r)
	if r.Output != "echo,fail,github,linear,read,slow|{}" {
		t.Fatalf("output = %q", r.Output)
	}
}

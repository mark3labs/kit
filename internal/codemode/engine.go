package codemode

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/dop251/goja"
)

// Default limits. Each one can be overridden in Limits.
const (
	DefaultTimeout        = 30 * time.Second
	DefaultMaxToolCalls   = 50
	DefaultMaxOutputBytes = 100_000
	DefaultMaxConcurrency = 8
	DefaultMemoryLimit    = 256 << 20

	// captureCap bounds everything a script can print (output plus logs),
	// independent of the model-facing MaxOutputBytes. Output beyond
	// MaxOutputBytes but under captureCap is kept so the host can save it
	// to a file.
	captureCap = 4 << 20
	// maxLogLines bounds the number of console lines kept.
	maxLogLines = 2000
	// maxStoreBytes bounds the total size of the persistent store.
	maxStoreBytes = 8 << 20
	// maxCallStack bounds JavaScript recursion depth.
	maxCallStack = 2000
	// scriptName is the file name used in stack traces.
	scriptName = "script.js"
)

// Limits bounds one script run. Zero fields use the defaults above. A
// negative MemoryLimit disables the memory watchdog.
type Limits struct {
	Timeout        time.Duration
	MaxToolCalls   int
	MaxOutputBytes int
	MaxConcurrency int
	MemoryLimit    int64
}

// WithDefaults returns l with every zero field set to its default.
func (l Limits) WithDefaults() Limits {
	if l.Timeout <= 0 {
		l.Timeout = DefaultTimeout
	}
	if l.MaxToolCalls <= 0 {
		l.MaxToolCalls = DefaultMaxToolCalls
	}
	if l.MaxOutputBytes <= 0 {
		l.MaxOutputBytes = DefaultMaxOutputBytes
	}
	if l.MaxConcurrency <= 0 {
		l.MaxConcurrency = DefaultMaxConcurrency
	}
	if l.MemoryLimit == 0 {
		l.MemoryLimit = DefaultMemoryLimit
	}
	return l
}

// CallStatus is the state of one nested tool call.
type CallStatus string

const (
	CallRunning   CallStatus = "running"
	CallOK        CallStatus = "ok"
	CallError     CallStatus = "error"
	CallCancelled CallStatus = "cancelled"
)

// Call records one nested tool call made by a script.
type Call struct {
	// ID is a synthetic tool call ID: "<prefix>.<n>".
	ID string `json:"id"`
	// Name is the full tool name ("github__list_issues").
	Name string `json:"name"`
	// Path is the dotted script path ("github.list_issues").
	Path string `json:"path"`
	// Args is the JSON input.
	Args string `json:"args,omitempty"`
	// Status is the call state.
	Status CallStatus `json:"status"`
	// Duration is the wall time of the call.
	Duration time.Duration `json:"duration_ns"`
	// Error is the error text for failed calls.
	Error string `json:"error,omitempty"`
}

// InvokeResult is the outcome of one nested tool call.
type InvokeResult struct {
	Text    string
	IsError bool
}

// Invoker executes one nested tool call. A returned error means the call
// could not run (not a tool-level failure, which uses IsError).
type Invoker func(ctx context.Context, call Call) (InvokeResult, error)

// Observer receives live progress. All callbacks run on the script
// goroutine; they must not block for long.
type Observer struct {
	OnCallStart func(Call)
	OnCallEnd   func(Call)
	OnLog       func(line string)
}

// Program is one script run request.
type Program struct {
	Code    string
	Catalog *Catalog
	Invoke  Invoker
	Limits  Limits
	// Store persists values across runs. Nil gives each run a fresh store.
	Store    *Store
	Observer Observer
	// CallIDPrefix prefixes nested call IDs. Usually the code mode tool
	// call ID.
	CallIDPrefix string
}

// ErrorKind classifies a failed run.
type ErrorKind string

const (
	ErrParse               ErrorKind = "ParseError"
	ErrUnknownTool         ErrorKind = "UnknownTool"
	ErrInvalidToolInput    ErrorKind = "InvalidToolInput"
	ErrToolFailure         ErrorKind = "ToolFailure"
	ErrToolCallLimit       ErrorKind = "ToolCallLimitExceeded"
	ErrTimeout             ErrorKind = "TimeoutExceeded"
	ErrMemoryLimit         ErrorKind = "MemoryLimitExceeded"
	ErrCancelled           ErrorKind = "Cancelled"
	ErrUnsettled           ErrorKind = "UnsettledPromise"
	ErrExecution           ErrorKind = "ExecutionFailure"
	errNameToolError                 = "ToolError"
	errNameUnknownTool               = "UnknownToolError"
	errNameInvalidToolArgs           = "InvalidToolInputError"
	errNameToolCallLimit             = "ToolCallLimitError"
)

// Diagnostic describes why a run failed.
type Diagnostic struct {
	Kind    ErrorKind `json:"kind"`
	Message string    `json:"message"`
	// Line and Column locate the error in the submitted code (1-based);
	// zero when unknown.
	Line   int    `json:"line,omitempty"`
	Column int    `json:"column,omitempty"`
	Hint   string `json:"hint,omitempty"`
}

// Result is the outcome of a run. Run never panics and never returns a Go
// error; failures are reported in Error.
type Result struct {
	OK bool `json:"ok"`
	// Output is everything the script passed to text() followed by its
	// return value.
	Output string   `json:"output,omitempty"`
	Logs   []string `json:"logs,omitempty"`
	Calls  []Call   `json:"calls,omitempty"`
	// Error is set when OK is false.
	Error *Diagnostic `json:"error,omitempty"`
	// CaptureTruncated reports that output or logs exceeded the capture
	// cap and were cut.
	CaptureTruncated bool          `json:"capture_truncated,omitempty"`
	WallTime         time.Duration `json:"wall_time_ns"`
}

var (
	errTimeout = errors.New("code mode: time limit exceeded")
	errMemory  = errors.New("code mode: memory limit exceeded")
)

// Run executes p and returns its result.
func Run(ctx context.Context, p Program) Result {
	start := time.Now()
	r := newRunner(p)
	res := r.run(ctx)
	res.WallTime = time.Since(start)
	return res
}

type runner struct {
	p   Program
	lim Limits
	vm  *goja.Runtime

	runCtx context.Context
	jobs   chan func()
	done   chan struct{}
	sem    chan struct{}

	pending   int
	callCount int
	calls     []Call
	limitHit  bool
	fatal     error

	output    strings.Builder
	logs      []string
	captured  int
	truncated bool

	stringify goja.Callable
	parse     goja.Callable
}

func newRunner(p Program) *runner {
	lim := p.Limits.WithDefaults()
	if p.Store == nil {
		p.Store = NewStore()
	}
	if p.Catalog == nil {
		p.Catalog = NewCatalog(nil)
	}
	if p.CallIDPrefix == "" {
		p.CallIDPrefix = "codemode"
	}
	return &runner{
		p:    p,
		lim:  lim,
		jobs: make(chan func(), 256),
		done: make(chan struct{}),
		sem:  make(chan struct{}, lim.MaxConcurrency),
	}
}

func (r *runner) run(parent context.Context) Result {
	ctx, cancelCause := context.WithCancelCause(parent)
	defer cancelCause(nil)
	runCtx, cancelTimeout := context.WithTimeoutCause(ctx, r.lim.Timeout, errTimeout)
	defer cancelTimeout()
	r.runCtx = runCtx
	defer close(r.done)

	src := wrapSource(r.p.Code)
	prog, err := goja.Compile(scriptName, src, false)
	if err != nil {
		return r.finish(parseDiagnostic(err))
	}

	r.vm = goja.New()
	r.vm.SetMaxCallStackSize(maxCallStack)
	if err := r.installGlobals(); err != nil {
		return r.finish(&Diagnostic{Kind: ErrExecution, Message: "sandbox setup failed: " + err.Error()})
	}

	// Interrupt a busy script when the run context ends (timeout, memory
	// limit, or cancellation by the caller).
	stopInterrupt := context.AfterFunc(runCtx, func() { r.vm.Interrupt(context.Cause(runCtx)) })
	defer stopInterrupt()

	if r.lim.MemoryLimit > 0 {
		go watchMemory(runCtx, r.lim.MemoryLimit, func() { cancelCause(errMemory) })
	}

	v, err := r.vm.RunProgram(prog)
	if err != nil {
		return r.finish(r.diagFromError(err))
	}
	promise, ok := v.Export().(*goja.Promise)
	if !ok {
		return r.finish(&Diagnostic{Kind: ErrExecution, Message: "internal error: script did not produce a promise"})
	}

	for promise.State() == goja.PromiseStatePending {
		if r.fatal != nil {
			return r.finish(r.diagFromError(r.fatal))
		}
		if r.pending == 0 {
			return r.finish(&Diagnostic{
				Kind:    ErrUnsettled,
				Message: "the script is waiting for a promise that can never settle",
				Hint:    "Await tool calls, sleep() or Promise.all() directly. Do not create a Promise that nothing resolves.",
			})
		}
		select {
		case f := <-r.jobs:
			f()
		case <-runCtx.Done():
			return r.finish(r.interruptDiagnostic())
		}
	}
	if r.fatal != nil {
		return r.finish(r.diagFromError(r.fatal))
	}

	if promise.State() == goja.PromiseStateRejected {
		return r.finish(r.diagFromValue(promise.Result()))
	}
	if rv := promise.Result(); rv != nil && !goja.IsUndefined(rv) {
		r.appendOutput(r.format(rv))
	}
	return r.finish(nil)
}

func (r *runner) finish(diag *Diagnostic) Result {
	for i := range r.calls {
		if r.calls[i].Status == CallRunning {
			r.calls[i].Status = CallCancelled
		}
	}
	res := Result{
		OK:               diag == nil,
		Output:           r.output.String(),
		Logs:             r.logs,
		Calls:            r.calls,
		Error:            diag,
		CaptureTruncated: r.truncated,
	}
	return res
}

// post schedules f on the script goroutine. It drops f when the run has
// ended.
func (r *runner) post(f func()) {
	select {
	case r.jobs <- f:
	case <-r.done:
	}
}

// settle records an uncatchable error returned by a resolve/reject call.
func (r *runner) settle(err error) {
	if err != nil && r.fatal == nil {
		r.fatal = err
	}
}

// ---------------------------------------------------------------------------
// globals
// ---------------------------------------------------------------------------

func (r *runner) installGlobals() error {
	vm := r.vm
	jsonObj := vm.Get("JSON").ToObject(vm)
	var ok bool
	if r.stringify, ok = goja.AssertFunction(jsonObj.Get("stringify")); !ok {
		return errors.New("JSON.stringify missing")
	}
	if r.parse, ok = goja.AssertFunction(jsonObj.Get("parse")); !ok {
		return errors.New("JSON.parse missing")
	}

	tools, err := r.buildToolsTree()
	if err != nil {
		return err
	}
	set := func(name string, v any) {
		if err == nil {
			err = vm.Set(name, v)
		}
	}
	set("tools", tools)

	console := vm.NewObject()
	for _, level := range []string{"log", "info", "debug", "warn", "error"} {
		prefix := ""
		if level == "warn" || level == "error" {
			prefix = "[" + level + "] "
		}
		_ = console.Set(level, func(fc goja.FunctionCall) goja.Value {
			r.appendLog(prefix + r.formatArgs(fc.Arguments))
			return goja.Undefined()
		})
	}
	set("console", console)

	set("text", func(fc goja.FunctionCall) goja.Value {
		r.appendOutput(r.formatArgs(fc.Arguments))
		return goja.Undefined()
	})

	set("call", func(name string, args goja.Value) goja.Value {
		e, ok := r.p.Catalog.Lookup(name)
		if !ok {
			panic(r.unknownToolError(name))
		}
		return r.startCall(e, args)
	})

	set("searchTools", func(query string, opts goja.Value) goja.Value {
		limit, namespace := 10, ""
		if o, ok := opts.(*goja.Object); ok {
			if v := o.Get("limit"); v != nil && !goja.IsUndefined(v) {
				limit = int(v.ToInteger())
			}
			if v := o.Get("namespace"); v != nil && !goja.IsUndefined(v) {
				namespace = v.String()
			}
		}
		return r.toJS(r.p.Catalog.Search(query, namespace, limit))
	})

	set("describeTool", func(name string) string {
		d, ok := r.p.Catalog.Describe(name)
		if !ok {
			panic(r.unknownToolError(name))
		}
		return d
	})

	all := make([]map[string]any, 0, len(r.p.Catalog.Entries()))
	for _, e := range r.p.Catalog.Entries() {
		all = append(all, map[string]any{
			"name":        e.Name,
			"path":        strings.TrimPrefix(e.Path(), "tools."),
			"description": firstLine(e.Description, 200),
		})
	}
	set("ALL_TOOLS", r.toJS(all))

	set("store", func(key string, value goja.Value) goja.Value {
		if value == nil || goja.IsUndefined(value) {
			r.p.Store.Delete(key)
			return goja.Undefined()
		}
		raw, err := r.stringify(goja.Undefined(), value)
		if err != nil {
			panic(err)
		}
		if goja.IsUndefined(raw) {
			panic(vm.NewTypeError("store(): value for %q cannot be serialized to JSON", key))
		}
		if err := r.p.Store.Set(key, raw.String()); err != nil {
			panic(vm.NewTypeError("store(): %s", err.Error()))
		}
		return goja.Undefined()
	})
	set("load", func(key string) goja.Value {
		raw, ok := r.p.Store.Get(key)
		if !ok {
			return goja.Undefined()
		}
		v, err := r.parse(goja.Undefined(), vm.ToValue(raw))
		if err != nil {
			panic(err)
		}
		return v
	})

	set("sleep", func(ms int64) goja.Value {
		p, resolve, _ := vm.NewPromise()
		d := time.Duration(max(ms, 0)) * time.Millisecond
		r.pending++
		go func() {
			t := time.NewTimer(d)
			defer t.Stop()
			select {
			case <-t.C:
			case <-r.runCtx.Done():
				return
			}
			r.post(func() {
				r.pending--
				r.settle(resolve(goja.Undefined()))
			})
		}()
		return vm.ToValue(p)
	})
	return err
}

// passthroughProps are property names that runtime machinery probes on any
// object (promise resolution, JSON, inspection). Reading them on the tools
// tree must not raise "unknown tool".
var passthroughProps = map[string]bool{
	"then": true, "toJSON": true, "constructor": true, "inspect": true,
	"__proto__": true, "length": true, "prototype": true, "valueOf": true,
	"toString": true, "nodeType": true, "$$typeof": true,
}

func (r *runner) buildToolsTree() (goja.Value, error) {
	vm := r.vm
	root := vm.NewObject()
	nsObjects := map[string]*goja.Object{}
	for _, e := range r.p.Catalog.Entries() {
		target := root
		if e.Namespace != "" {
			obj, ok := nsObjects[e.Namespace]
			if !ok {
				obj = vm.NewObject()
				nsObjects[e.Namespace] = obj
			}
			target = obj
		}
		entry := e
		if err := target.Set(e.Method, func(fc goja.FunctionCall) goja.Value {
			return r.startCall(entry, fc.Argument(0))
		}); err != nil {
			return nil, err
		}
	}
	for ns, obj := range nsObjects {
		if err := root.Set(ns, r.guard(obj, ns)); err != nil {
			return nil, err
		}
	}
	return r.guard(root, ""), nil
}

// guard wraps obj in a proxy that raises a helpful UnknownToolError for a
// missing property instead of yielding undefined (which would surface as an
// opaque "is not a function" TypeError).
func (r *runner) guard(obj *goja.Object, namespace string) goja.Value {
	return r.vm.ToValue(r.vm.NewProxy(obj, &goja.ProxyTrapConfig{
		Get: func(target *goja.Object, prop string, _ goja.Value) goja.Value {
			if v := target.Get(prop); v != nil {
				return v
			}
			if passthroughProps[prop] || strings.HasPrefix(prop, "@@") {
				return goja.Undefined()
			}
			name := prop
			if namespace != "" {
				name = namespace + "." + prop
			}
			panic(r.unknownToolError(name))
		},
	}))
}

// ---------------------------------------------------------------------------
// nested calls
// ---------------------------------------------------------------------------

func (r *runner) startCall(e Entry, arg goja.Value) goja.Value {
	vm := r.vm
	path := strings.TrimPrefix(e.Path(), "tools.")
	argsJSON := "{}"
	if arg != nil && !goja.IsUndefined(arg) && !goja.IsNull(arg) {
		obj, isObj := arg.(*goja.Object)
		if !isObj || obj.ClassName() == "Array" || obj.ClassName() == "Function" {
			panic(r.newError(errNameInvalidToolArgs,
				fmt.Sprintf("%s expects one object argument, got %s", path, typeOf(arg)),
				fmt.Sprintf("Call it as await tools.%s({ ... }).", path)))
		}
		raw, err := r.stringify(goja.Undefined(), arg)
		if err != nil {
			panic(err)
		}
		argsJSON = raw.String()
	}
	if missing := missingRequired(e, argsJSON); len(missing) > 0 {
		panic(r.newError(errNameInvalidToolArgs,
			fmt.Sprintf("%s: missing required argument(s): %s", path, strings.Join(missing, ", ")),
			"Signature: "+e.Signature()))
	}
	if r.callCount >= r.lim.MaxToolCalls {
		r.limitHit = true
		panic(r.newError(errNameToolCallLimit,
			fmt.Sprintf("tool call limit reached (%d calls per script)", r.lim.MaxToolCalls),
			"Process fewer items per script, or return partial results and continue in a new script."))
	}
	r.callCount++

	call := Call{
		ID:     fmt.Sprintf("%s.%d", r.p.CallIDPrefix, r.callCount),
		Name:   e.Name,
		Path:   path,
		Args:   argsJSON,
		Status: CallRunning,
	}
	idx := len(r.calls)
	r.calls = append(r.calls, call)
	if r.p.Observer.OnCallStart != nil {
		r.p.Observer.OnCallStart(call)
	}

	p, resolve, reject := vm.NewPromise()
	r.pending++
	go func() {
		started := time.Now()
		res, err := r.invoke(call)
		elapsed := time.Since(started)
		r.post(func() {
			r.pending--
			c := &r.calls[idx]
			c.Duration = elapsed
			switch {
			case err != nil:
				c.Status = CallError
				if r.runCtx.Err() != nil {
					c.Status = CallCancelled
				}
				c.Error = err.Error()
			case res.IsError:
				c.Status = CallError
				c.Error = firstLine(res.Text, 300)
			default:
				c.Status = CallOK
			}
			// Report the end before settling: settling runs the script's
			// continuation, whose logs must come after this call's end.
			if r.p.Observer.OnCallEnd != nil {
				r.p.Observer.OnCallEnd(*c)
			}
			switch {
			case err != nil:
				r.settle(reject(r.toolError(c, err.Error())))
			case res.IsError:
				r.settle(reject(r.toolError(c, res.Text)))
			default:
				r.settle(resolve(res.Text))
			}
		})
	}()
	return vm.ToValue(p)
}

func (r *runner) invoke(call Call) (res InvokeResult, err error) {
	select {
	case r.sem <- struct{}{}:
	case <-r.runCtx.Done():
		return InvokeResult{}, context.Cause(r.runCtx)
	}
	defer func() { <-r.sem }()
	defer func() {
		if rec := recover(); rec != nil {
			err = fmt.Errorf("tool %s panicked: %v", call.Name, rec)
		}
	}()
	if r.p.Invoke == nil {
		return InvokeResult{}, errors.New("no tool invoker configured")
	}
	return r.p.Invoke(r.runCtx, call)
}

func missingRequired(e Entry, argsJSON string) []string {
	if len(e.Required) == 0 {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(argsJSON), &m); err != nil {
		return nil
	}
	var missing []string
	for _, k := range e.Required {
		if v, ok := m[k]; !ok || v == nil {
			missing = append(missing, k)
		}
	}
	return missing
}

// ---------------------------------------------------------------------------
// errors
// ---------------------------------------------------------------------------

func (r *runner) newError(name, msg, hint string) *goja.Object {
	ctor, _ := goja.AssertConstructor(r.vm.Get("Error"))
	obj, err := ctor(nil, r.vm.ToValue(msg))
	if err != nil {
		panic(r.vm.NewTypeError(msg))
	}
	_ = obj.Set("name", name)
	if hint != "" {
		_ = obj.Set("hint", hint)
	}
	return obj
}

func (r *runner) toolError(c *Call, text string) *goja.Object {
	msg := fmt.Sprintf("%s failed: %s", c.Path, firstLine(text, 500))
	obj := r.newError(errNameToolError, msg, "Catch the error with try/catch to handle it, or fix the arguments.")
	_ = obj.Set("tool", c.Path)
	_ = obj.Set("output", text)
	return obj
}

func (r *runner) unknownToolError(name string) *goja.Object {
	hint := "Use searchTools(\"keywords\") to find tools, or ALL_TOOLS to list them."
	parts := strings.Split(name, ".")
	if s := r.p.Catalog.Suggest(parts[len(parts)-1]); len(s) > 0 {
		hint = "Did you mean " + strings.Join(s, ", ") + "? " + hint
	}
	return r.newError(errNameUnknownTool, fmt.Sprintf("unknown tool: %s", name), hint)
}

var stackPosRe = regexp.MustCompile(regexp.QuoteMeta(scriptName) + `:(\d+):(\d+)`)

func (r *runner) diagFromError(err error) *Diagnostic {
	var interrupted *goja.InterruptedError
	var exc *goja.Exception
	var overflow *goja.StackOverflowError
	switch {
	case errors.As(err, &interrupted):
		return r.interruptDiagnostic()
	case errors.As(err, &overflow):
		return &Diagnostic{Kind: ErrExecution, Message: "maximum call stack size exceeded", Hint: "Check for unbounded recursion."}
	case errors.As(err, &exc):
		d := r.diagFromValue(exc.Value())
		if d.Line == 0 {
			if frames := exc.Stack(); len(frames) > 0 {
				pos := frames[0].Position()
				if pos.Line > 1 {
					d.Line, d.Column = pos.Line-1, pos.Column
				}
			}
		}
		return d
	}
	return &Diagnostic{Kind: ErrExecution, Message: err.Error()}
}

func (r *runner) interruptDiagnostic() *Diagnostic {
	cause := context.Cause(r.runCtx)
	switch {
	case errors.Is(cause, errTimeout):
		return &Diagnostic{
			Kind:    ErrTimeout,
			Message: fmt.Sprintf("the script exceeded its %s time limit", r.lim.Timeout),
			Hint:    "Do less work per script. Run independent tool calls in parallel with Promise.all.",
		}
	case errors.Is(cause, errMemory):
		return &Diagnostic{
			Kind:    ErrMemoryLimit,
			Message: fmt.Sprintf("the script exceeded its %d MB memory limit", r.lim.MemoryLimit>>20),
			Hint:    "Keep less data in memory. Filter tool results before you collect them.",
		}
	}
	return &Diagnostic{Kind: ErrCancelled, Message: "the script was cancelled"}
}

func (r *runner) diagFromValue(v goja.Value) *Diagnostic {
	d := &Diagnostic{Kind: ErrExecution}
	obj, ok := v.(*goja.Object)
	if !ok || v == nil {
		if v == nil {
			d.Message = "script failed"
		} else {
			d.Message = "uncaught exception: " + v.String()
		}
		return d
	}
	name := propString(obj, "name")
	d.Message = propString(obj, "message")
	custom := name == errNameToolError || name == errNameUnknownTool ||
		name == errNameInvalidToolArgs || name == errNameToolCallLimit
	if d.Message == "" {
		d.Message = obj.String()
	} else if name != "" && name != "Error" && !custom {
		d.Message = name + ": " + d.Message
	}
	d.Hint = propString(obj, "hint")
	switch name {
	case errNameToolError:
		d.Kind = ErrToolFailure
	case errNameUnknownTool:
		d.Kind = ErrUnknownTool
	case errNameInvalidToolArgs:
		d.Kind = ErrInvalidToolInput
	case errNameToolCallLimit:
		d.Kind = ErrToolCallLimit
	}
	if m := stackPosRe.FindStringSubmatch(propString(obj, "stack")); m != nil {
		line, _ := strconv.Atoi(m[1])
		col, _ := strconv.Atoi(m[2])
		if line > 1 {
			d.Line, d.Column = line-1, col
		}
	}
	return d
}

func propString(o *goja.Object, name string) string {
	v := o.Get(name)
	if v == nil || goja.IsUndefined(v) || goja.IsNull(v) {
		return ""
	}
	return v.String()
}

var syntaxPosRe = regexp.MustCompile(`Line (\d+):(\d+)`)

func parseDiagnostic(err error) *Diagnostic {
	msg := err.Error()
	d := &Diagnostic{Kind: ErrParse, Hint: "Send plain JavaScript (no TypeScript types, no import/export). The code runs inside an async function."}
	if m := syntaxPosRe.FindStringSubmatch(msg); m != nil {
		line, _ := strconv.Atoi(m[1])
		col, _ := strconv.Atoi(m[2])
		if line > 1 {
			d.Line, d.Column = line-1, col
		}
		// Rewrite the wrapped-source line number to the submitted one.
		msg = strings.Replace(msg, m[0], fmt.Sprintf("Line %d:%d", d.Line, d.Column), 1)
	}
	msg = strings.Replace(msg, scriptName+": ", "", 1)
	d.Message = msg
	return d
}

// ---------------------------------------------------------------------------
// output
// ---------------------------------------------------------------------------

func (r *runner) appendOutput(s string) {
	if !r.capture(len(s) + 1) {
		return
	}
	if r.output.Len() > 0 {
		r.output.WriteByte('\n')
	}
	r.output.WriteString(s)
}

func (r *runner) appendLog(s string) {
	if r.p.Observer.OnLog != nil {
		r.p.Observer.OnLog(s)
	}
	if len(r.logs) >= maxLogLines {
		r.truncated = true
		return
	}
	if !r.capture(len(s)) {
		return
	}
	r.logs = append(r.logs, s)
}

func (r *runner) capture(n int) bool {
	if r.captured+n > captureCap {
		r.truncated = true
		return false
	}
	r.captured += n
	return true
}

func (r *runner) formatArgs(args []goja.Value) string {
	parts := make([]string, 0, len(args))
	for _, a := range args {
		parts = append(parts, r.format(a))
	}
	return strings.Join(parts, " ")
}

// format renders a value for output: strings as-is, everything else as
// indented JSON, falling back to String() for values JSON cannot encode.
func (r *runner) format(v goja.Value) string {
	if v == nil || goja.IsUndefined(v) {
		return "undefined"
	}
	if s, ok := v.Export().(string); ok {
		return s
	}
	if _, isFn := goja.AssertFunction(v); isFn {
		return "[function]"
	}
	if raw, err := r.stringify(goja.Undefined(), v, goja.Null(), r.vm.ToValue(2)); err == nil && !goja.IsUndefined(raw) {
		return raw.String()
	}
	return v.String()
}

func (r *runner) toJS(v any) goja.Value {
	raw, err := json.Marshal(v)
	if err != nil {
		return goja.Undefined()
	}
	out, err := r.parse(goja.Undefined(), r.vm.ToValue(string(raw)))
	if err != nil {
		return goja.Undefined()
	}
	return out
}

func typeOf(v goja.Value) string {
	if o, ok := v.(*goja.Object); ok {
		return strings.ToLower(o.ClassName())
	}
	switch v.Export().(type) {
	case string:
		return "string"
	case int64, float64:
		return "number"
	case bool:
		return "boolean"
	}
	return "value"
}

// wrapSource strips Markdown code fences and wraps the code in an async
// function so it can use await and return at top level.
func wrapSource(code string) string {
	trimmed := strings.TrimSpace(code)
	if strings.HasPrefix(trimmed, "```") {
		if nl := strings.IndexByte(trimmed, '\n'); nl >= 0 {
			trimmed = trimmed[nl+1:]
		} else {
			trimmed = ""
		}
		trimmed = strings.TrimSuffix(strings.TrimSpace(trimmed), "```")
		code = trimmed
	}
	return "(async () => {\n" + code + "\n})()"
}

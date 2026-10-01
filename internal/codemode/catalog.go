// Package codemode runs model-written JavaScript programs that orchestrate
// other tools. The model calls one tool with a script; the script calls the
// other tools through a `tools` object, combines and filters their results,
// and returns a small answer. Intermediate tool results never enter the
// model's context.
//
// The package is independent of the LLM framework. The host gives it a
// Catalog of tool specs and an Invoker that executes one nested call; the
// core tool in internal/core adapts the agent's tool set to these types.
package codemode

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// NamespaceSeparator separates an MCP server name from the tool name in a
// prefixed tool name ("github__list_issues"). The catalog exposes such a tool
// as tools.github.list_issues.
const NamespaceSeparator = "__"

// ToolSpec describes one tool the script can call.
type ToolSpec struct {
	// Name is the full tool name as the agent knows it, e.g. "read" or
	// "github__list_issues".
	Name string
	// Description is the tool's model-facing description.
	Description string
	// Parameters is the JSON Schema "properties" map of the tool input.
	Parameters map[string]any
	// Required lists the required parameter names.
	Required []string
	// Exposure controls how the tool appears to the model and the script.
	Exposure Exposure
}

// Entry is a catalog entry: a ToolSpec plus its location in the tools tree.
type Entry struct {
	ToolSpec
	// Namespace is the tree namespace ("" for root-level tools).
	Namespace string
	// Method is the property name inside the namespace.
	Method string
}

// Path returns the JavaScript expression that reaches the tool, e.g.
// `tools.read`, `tools.github.list_issues` or `tools["my-tool"]`.
func (e Entry) Path() string {
	p := "tools"
	if e.Namespace != "" {
		p += accessor(e.Namespace)
	}
	return p + accessor(e.Method)
}

// Signature renders a TypeScript-like signature for the tool.
func (e Entry) Signature() string {
	return fmt.Sprintf("%s(args: %s): Promise<string>", e.Path(), schemaObjectToTS(e.Parameters, e.Required, 0))
}

// ShortSignature renders the signature with parameter names only. It is used
// for tools the model already sees as direct tools, whose full schema is
// already in the request.
func (e Entry) ShortSignature() string {
	names := sortedKeys(e.Parameters)
	req := make(map[string]bool, len(e.Required))
	for _, r := range e.Required {
		req[r] = true
	}
	parts := make([]string, 0, len(names))
	for _, n := range names {
		if req[n] {
			parts = append(parts, n)
		} else {
			parts = append(parts, n+"?")
		}
	}
	return fmt.Sprintf("%s({%s})", e.Path(), strings.Join(parts, ", "))
}

// Catalog is the set of tools a script can see, organized as a tree.
type Catalog struct {
	entries []Entry
	byName  map[string]int
}

// NewCatalog builds a catalog from specs. Tools with ExposureModelOnly are
// dropped: the script cannot call them. Specs keep their input order inside
// a namespace; namespaces are sorted.
func NewCatalog(specs []ToolSpec) *Catalog {
	c := &Catalog{byName: make(map[string]int, len(specs))}
	for _, s := range specs {
		if s.Exposure == ExposureModelOnly {
			continue
		}
		if _, dup := c.byName[s.Name]; dup {
			continue
		}
		if s.Exposure == "" {
			s.Exposure = ExposureDirect
		}
		ns, method := splitName(s.Name)
		c.byName[s.Name] = len(c.entries)
		c.entries = append(c.entries, Entry{ToolSpec: s, Namespace: ns, Method: method})
	}
	// A root tool whose name equals a namespace would be shadowed by the
	// namespace object. Move it to its full name under the root so both stay
	// reachable.
	namespaces := map[string]bool{}
	for _, e := range c.entries {
		if e.Namespace != "" {
			namespaces[e.Namespace] = true
		}
	}
	for i, e := range c.entries {
		if e.Namespace == "" && namespaces[e.Method] {
			c.entries[i].Method = e.Name + "_tool"
		}
	}
	return c
}

// Entries returns all entries.
func (c *Catalog) Entries() []Entry {
	if c == nil {
		return nil
	}
	return c.entries
}

// Lookup finds an entry by full tool name or by its dotted path
// ("github.list_issues").
func (c *Catalog) Lookup(name string) (Entry, bool) {
	if c == nil {
		return Entry{}, false
	}
	if i, ok := c.byName[name]; ok {
		return c.entries[i], true
	}
	name = strings.TrimPrefix(name, "tools.")
	for _, e := range c.entries {
		dotted := e.Method
		if e.Namespace != "" {
			dotted = e.Namespace + "." + e.Method
		}
		if dotted == name {
			return e, true
		}
	}
	return Entry{}, false
}

// Namespaces returns the sorted namespace names ("" is the root).
func (c *Catalog) Namespaces() []string {
	seen := map[string]bool{}
	var out []string
	for _, e := range c.Entries() {
		if !seen[e.Namespace] {
			seen[e.Namespace] = true
			out = append(out, e.Namespace)
		}
	}
	sort.Strings(out)
	return out
}

// SearchHit is one search result.
type SearchHit struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	Description string `json:"description"`
	Signature   string `json:"signature"`
}

// Search returns the entries that best match query. Each query word that
// occurs in the tool name scores 3, in the description 1. An empty query
// matches every tool. namespace, when not empty, limits the search to one
// namespace.
func (c *Catalog) Search(query, namespace string, limit int) []SearchHit {
	if limit <= 0 {
		limit = 10
	}
	words := strings.Fields(strings.ToLower(query))
	type scored struct {
		e     Entry
		score int
		idx   int
	}
	var hits []scored
	for i, e := range c.Entries() {
		if namespace != "" && e.Namespace != namespace {
			continue
		}
		score := 0
		name := strings.ToLower(e.Name)
		desc := strings.ToLower(e.Description)
		for _, w := range words {
			if strings.Contains(name, w) {
				score += 3
			}
			if strings.Contains(desc, w) {
				score++
			}
		}
		if len(words) > 0 && score == 0 {
			continue
		}
		hits = append(hits, scored{e, score, i})
	}
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].score != hits[j].score {
			return hits[i].score > hits[j].score
		}
		return hits[i].idx < hits[j].idx
	})
	if len(hits) > limit {
		hits = hits[:limit]
	}
	out := make([]SearchHit, 0, len(hits))
	for _, h := range hits {
		out = append(out, SearchHit{
			Name:        h.e.Name,
			Path:        h.e.Path(),
			Description: firstLine(h.e.Description, 200),
			Signature:   h.e.Signature(),
		})
	}
	return out
}

// Describe returns the full signature, description and JSON schema of one
// tool.
func (c *Catalog) Describe(name string) (string, bool) {
	e, ok := c.Lookup(name)
	if !ok {
		return "", false
	}
	schema := map[string]any{"type": "object", "properties": e.Parameters}
	if len(e.Required) > 0 {
		schema["required"] = e.Required
	}
	raw, _ := json.MarshalIndent(schema, "", "  ")
	return fmt.Sprintf("%s\n\n%s\n\nInput schema:\n%s", e.Signature(), strings.TrimSpace(e.Description), raw), true
}

// Suggest returns up to three tool paths whose name resembles name. It is
// used for "unknown tool" errors.
func (c *Catalog) Suggest(name string) []string {
	name = strings.ToLower(name)
	type cand struct {
		path string
		d    int
	}
	var cands []cand
	for _, e := range c.Entries() {
		m := strings.ToLower(e.Method)
		d := levenshtein(name, m)
		if strings.Contains(m, name) || strings.Contains(name, m) {
			d = 0
		}
		if d <= max(2, len(name)/3) {
			cands = append(cands, cand{e.Path(), d})
		}
	}
	sort.SliceStable(cands, func(i, j int) bool { return cands[i].d < cands[j].d })
	var out []string
	for i := 0; i < len(cands) && i < 3; i++ {
		out = append(out, cands[i].path)
	}
	return out
}

// RenderCatalog renders the model-facing tool list within budget bytes.
// Namespaces are visited round-robin so one large MCP server cannot crowd
// out the others. Tools with ExposureDeferred are never listed. Direct tools
// get a short signature (the model already has their schema); script-only
// tools get the full signature and a one-line description. The second return
// value is the number of callable tools left out.
func (c *Catalog) RenderCatalog(budget int) (string, int) {
	if budget <= 0 {
		budget = 12000
	}
	groups := map[string][]Entry{}
	omitted := 0
	for _, e := range c.Entries() {
		if e.Exposure == ExposureDeferred {
			omitted++
			continue
		}
		groups[e.Namespace] = append(groups[e.Namespace], e)
	}
	order := c.Namespaces()

	rendered := map[string][]string{}
	used := 0
	for progress := true; progress; {
		progress = false
		for _, ns := range order {
			g := groups[ns]
			if len(g) == 0 {
				continue
			}
			e := g[0]
			groups[ns] = g[1:]
			var line string
			if e.Exposure == ExposureDirect {
				line = e.ShortSignature()
			} else {
				line = e.Signature()
				if d := firstLine(e.Description, 160); d != "" {
					line += "\n  // " + d
				}
			}
			cost := len(line) + 1
			if len(rendered[ns]) == 0 {
				cost += len(namespaceHeader(ns)) + 1
			}
			if used+cost > budget {
				omitted++
				continue
			}
			used += cost
			rendered[ns] = append(rendered[ns], line)
			progress = true
		}
	}
	for _, g := range groups {
		omitted += len(g)
	}

	var b strings.Builder
	for _, ns := range order {
		lines := rendered[ns]
		if len(lines) == 0 {
			continue
		}
		b.WriteString(namespaceHeader(ns))
		b.WriteByte('\n')
		for _, l := range lines {
			b.WriteString(l)
			b.WriteByte('\n')
		}
	}
	return strings.TrimRight(b.String(), "\n"), omitted
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func namespaceHeader(ns string) string {
	if ns == "" {
		return "// built-in tools"
	}
	return "// " + ns
}

var identRe = regexp.MustCompile(`^[A-Za-z_$][A-Za-z0-9_$]*$`)

func accessor(name string) string {
	if identRe.MatchString(name) {
		return "." + name
	}
	raw, _ := json.Marshal(name)
	return "[" + string(raw) + "]"
}

func splitName(name string) (namespace, method string) {
	if i := strings.Index(name, NamespaceSeparator); i > 0 && i+len(NamespaceSeparator) < len(name) {
		return name[:i], name[i+len(NamespaceSeparator):]
	}
	return "", name
}

func firstLine(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = strings.TrimSpace(s[:i])
	}
	if r := []rune(s); len(r) > maxLen {
		s = string(r[:maxLen-1]) + "…"
	}
	return s
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// schemaObjectToTS renders an object schema (properties + required) as a
// TypeScript object type.
func schemaObjectToTS(props map[string]any, required []string, depth int) string {
	if len(props) == 0 {
		return "{}"
	}
	if depth > 2 {
		return "object"
	}
	req := make(map[string]bool, len(required))
	for _, r := range required {
		req[r] = true
	}
	parts := make([]string, 0, len(props))
	for _, k := range sortedKeys(props) {
		opt := "?"
		if req[k] {
			opt = ""
		}
		key := k
		if !identRe.MatchString(k) {
			raw, _ := json.Marshal(k)
			key = string(raw)
		}
		parts = append(parts, fmt.Sprintf("%s%s: %s", key, opt, schemaToTS(props[k], depth+1)))
	}
	return "{ " + strings.Join(parts, "; ") + " }"
}

// schemaToTS renders one JSON Schema node as a TypeScript type.
func schemaToTS(node any, depth int) string {
	s, ok := node.(map[string]any)
	if !ok {
		return "any"
	}
	if enum, ok := s["enum"].([]any); ok && len(enum) > 0 && len(enum) <= 12 {
		vals := make([]string, 0, len(enum))
		for _, v := range enum {
			raw, _ := json.Marshal(v)
			vals = append(vals, string(raw))
		}
		return strings.Join(vals, " | ")
	}
	for _, key := range []string{"anyOf", "oneOf"} {
		if alts, ok := s[key].([]any); ok && len(alts) > 0 {
			vals := make([]string, 0, len(alts))
			for _, a := range alts {
				vals = append(vals, schemaToTS(a, depth+1))
			}
			return strings.Join(vals, " | ")
		}
	}
	typ := s["type"]
	if list, ok := typ.([]any); ok {
		vals := make([]string, 0, len(list))
		for _, t := range list {
			vals = append(vals, schemaToTS(map[string]any{"type": t, "items": s["items"], "properties": s["properties"]}, depth))
		}
		return strings.Join(vals, " | ")
	}
	switch typ {
	case "string":
		return "string"
	case "number", "integer":
		return "number"
	case "boolean":
		return "boolean"
	case "null":
		return "null"
	case "array":
		inner := schemaToTS(s["items"], depth+1)
		if strings.Contains(inner, " | ") {
			inner = "(" + inner + ")"
		}
		return inner + "[]"
	case "object":
		props, _ := s["properties"].(map[string]any)
		if len(props) == 0 {
			return "object"
		}
		return schemaObjectToTS(props, toStrings(s["required"]), depth)
	}
	return "any"
}

func toStrings(v any) []string {
	switch t := v.(type) {
	case []string:
		return t
	case []any:
		out := make([]string, 0, len(t))
		for _, x := range t {
			if s, ok := x.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

func levenshtein(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	prev := make([]int, len(rb)+1)
	cur := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(rb)]
}

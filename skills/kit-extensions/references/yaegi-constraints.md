# Kit Extensions: Critical Yaegi Constraints (Full Version)

> Part of the `kit-extensions` skill. Read `SKILL.md` first for the overview and critical constraints.

## Critical Yaegi Constraints

### No Named Function References in Struct Fields

Yaegi has a bug where named function references assigned to struct fields return zero values across the interpreter boundary. Always use anonymous closure literals:

```go
// WRONG - will silently return zero values:
func myHandler(key, text string) ext.EditorKeyAction {
    return ext.EditorKeyAction{Type: ext.EditorKeyPassthrough}
}
ctx.SetEditor(ext.EditorConfig{HandleKey: myHandler})

// CORRECT - use anonymous closure:
ctx.SetEditor(ext.EditorConfig{
    HandleKey: func(key, text string) ext.EditorKeyAction {
        return ext.EditorKeyAction{Type: ext.EditorKeyPassthrough}
    },
})
```

This applies to ALL struct fields that take function values: `ToolDef.Execute`, `CommandDef.Execute`, `EditorConfig.HandleKey`, `EditorConfig.Render`, `ToolRenderConfig.RenderHeader`, `ToolRenderConfig.RenderBody`, etc.

### No Comma-Separated Case Lists in a Tagless Switch

In `switch { case a, b, c: }` Yaegi evaluates **only the first expression** and
silently ignores the rest. There is no load error and no panic — the branch
just fails to fire for inputs that should have matched. A switch with a tag
(`switch n { case 1, 2, 3: }`) is unaffected.

```go
// WRONG - only `r >= 'a' && r <= 'z'` is ever checked, so digits and
// uppercase letters silently fall through to default:
switch {
case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
    b.WriteRune(r)
default:
    b.WriteRune('-')
}

// CORRECT - join the conditions with || into a single case expression:
switch {
case (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9'):
    b.WriteRune(r)
default:
    b.WriteRune('-')
}
```

An `if`/`else` chain also works. This idiom is common in character-class tests
and rune-width tables, so check any such code you write.

### No Interfaces Across the Boundary

All extension-facing API types are concrete structs, never interfaces. Yaegi crashes on interface wrapper generation.

### Package-Level Variables for State

Yaegi supports package-level variables captured in closures. This is the standard way to maintain state across event callbacks:

```go
package main

import "kit/ext"

var callCount int
var lastTool string

func Init(api ext.API) {
    api.OnToolResult(func(e ext.ToolResultEvent, ctx ext.Context) *ext.ToolResultResult {
        callCount++
        lastTool = e.ToolName
        return nil
    })
}
```


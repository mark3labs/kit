# Autoscroll Fix - Final Summary

## Root Cause

The autoscroll was failing for streaming assistant messages due to a bug in how `GotoBottom()` calculated item heights.

### The Problem

1. **Reasoning blocks** (`StreamingMessageItem` with `role="reasoning"`) are **never cached** because they have live duration counters that update every render
2. The `Height()` method returns `0` when `cachedRender == ""`
3. `GotoBottom()` was calling:
   ```go
   itemHeight := item.Height()  // Returns 0 for reasoning
   if itemHeight == 0 {
       item.Render(s.width)  // Renders but doesn't cache (reasoning)
       itemHeight = item.Height()  // Still returns 0!
   }
   ```
4. This caused incorrect scroll position calculations, especially during reasoning → assistant transitions

## The Solution

Changed `GotoBottom()` and `AtBottom()` to calculate height **directly from the rendered string** instead of relying on the cached height:

```go
// OLD: item.Height() which checks cached render
itemHeight := item.Height()
if itemHeight == 0 {
    item.Render(s.width)
    itemHeight = item.Height()  // Still might be 0!
}

// NEW: Calculate from rendered string directly
rendered := item.Render(s.width)
itemHeight := strings.Count(rendered, "\n") + 1
```

This works for **all** items regardless of whether they cache their render or not.

## Files Changed

### `internal/ui/scrolllist.go`
- **`GotoBottom()`**: Calculate height from rendered string (2 loops)
- **`AtBottom()`**: Calculate height from rendered string (1 loop)

### `internal/ui/model.go`
- **`appendStreamingChunk()`**: For existing messages, call `GotoBottom()` directly (iteratr pattern)
- **`refreshContent()`**: Simplified to only call `SetItems()` (removed redundant `GotoBottom()`)
- **Bash streaming handler**: Removed redundant `GotoBottom()` after `refreshContent()`

## Testing Results

✅ **Test prompt**: "explore this repo"

**Before fix**:
- Autoscroll stopped after reasoning block completed
- Viewport stuck showing end of reasoning ("Thought for 203ms")
- Assistant response streamed off-screen below

**After fix**:
- Autoscroll works throughout reasoning block
- Autoscroll continues during reasoning → assistant transition  
- Viewport stays at bottom showing latest assistant content
- Final position shows end of response (build commands section)

## Behavior Verified

1. ✅ Streaming text auto-scrolls to bottom
2. ✅ Works across reasoning → assistant transition
3. ✅ Manual scroll up (PgUp) disables autoscroll
4. ✅ Scroll to bottom (Alt+End) re-enables autoscroll
5. ✅ Accurate positioning with no offset errors

## Performance Note

The fix calls `Render()` on all items during `GotoBottom()` calculations. This is acceptable because:
- `Render()` is already optimized with caching for non-reasoning items
- `GotoBottom()` is only called during content updates (not every frame)
- Reasoning blocks need to render anyway for live duration updates
- This matches iteratr's approach of ensuring items are rendered before height calculations

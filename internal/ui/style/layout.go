package style

// Layout constants and helpers defining the shared geometry of every block
// rendered into the transcript.
//
// The UI keeps one left-edge contract: a marker (gutter glyph, tool bullet,
// receipt check) occupies column 0, a single space follows, and text starts at
// ContentOffset. Blocks with no marker — assistant prose, the splash — are
// indented to the same column, so the transcript reads as one aligned text
// column with markers hanging in the left margin.
//
// Widths follow from the same contract. A block that starts at ContentOffset
// and leaves RightMargin columns free has ContentWidth available for text;
// content nested one level deeper (alert bodies behind a gutter bar, tool
// output beneath a tool header) has BodyWidth. Renderers must size themselves
// with these helpers rather than subtracting their own magic numbers, because
// a renderer that invents its own budget is a renderer that overflows the
// terminal the first time someone passes it a long line.

const (
	// ContentOffset is the column at which every block's text begins.
	ContentOffset = 2

	// RightMargin is the number of columns deliberately left empty at the
	// right edge. Text that runs flush to the final column reads as though
	// it has been cut off even when it has not.
	RightMargin = 1

	// BlockGap is the number of blank lines that follow every block in the
	// transcript. Spacing is the block's own responsibility: the scroll list
	// concatenates items verbatim and inserts nothing between them.
	BlockGap = 1

	// MinContentWidth is the floor for any computed width. Below this a
	// block cannot render anything legible, so renderers clamp here and
	// accept horizontal overflow rather than producing negative widths or
	// wrapping every word onto its own line.
	MinContentWidth = 20
)

// ContentWidth returns the number of columns available to text that begins at
// ContentOffset within a terminal of the given total width, leaving
// RightMargin columns free at the right edge.
//
// This is the budget for a top-level block: user messages, assistant prose,
// the splash, alerts.
func ContentWidth(termWidth int) int {
	return max(termWidth-ContentOffset-RightMargin, MinContentWidth)
}

// BodyWidth returns the number of columns available to content nested one
// level inside a block that already carries a marker — an alert body sitting
// behind a gutter bar, or tool output rendered beneath a tool header.
//
// Nested content pays for its own indent on top of the block's, so it is one
// ContentOffset narrower than ContentWidth.
func BodyWidth(termWidth int) int {
	return max(ContentWidth(termWidth)-ContentOffset, MinContentWidth)
}

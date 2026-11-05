package style

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/indaco/herald"
	heraldmd "github.com/indaco/herald-md"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	east "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"
)

// herald draws a table with one line per row at its natural width and has no
// width limit. ToMarkdown then wraps every line to the terminal width, which
// cuts the border and row lines of a wide table into broken fragments. The
// functions in this file render a top-level table that is too wide with
// narrower columns and wrapped cells, so that the table fits the width and
// the final wrap leaves it alone.

// tableParser parses markdown with the same extensions as herald-md, so the
// tables it finds are the tables herald-md renders.
var tableParser = goldmark.New(goldmark.WithExtensions(extension.GFM))

// minTableColumnWidth is the smallest content width a column gets when a table
// is made narrower. Below this, wrapped cells are not readable, and the table
// is left to herald.
const minTableColumnWidth = 3

// placeholderToken marks where a fitted table goes in the herald output. It
// has no markdown syntax, so herald renders it as a plain paragraph.
const placeholderToken = "KITFITTEDTABLEPLACEHOLDER"

// sourceTable is one top-level table in the markdown source.
type sourceTable struct {
	start, stop int // byte range of the table lines in the source
	node        *east.Table
}

// fitMarkdownTables renders content like heraldmd.Render, but renders each
// top-level table wider than width with columns that fit width. It returns ok
// false when no table needed a change; the caller then keeps the plain herald
// output.
func fitMarkdownTables(ty *herald.Typography, content []byte, width int) (string, bool) {
	tables := findTopLevelTables(content)
	if len(tables) == 0 {
		return "", false
	}

	// Replace each table that does not fit with a placeholder paragraph,
	// from the end so earlier byte offsets stay valid.
	fitted := make(map[string]string)
	src := content
	for i, tbl := range slices.Backward(tables) {
		out, ok := renderFittedTable(ty, content, tbl.node, width)
		if !ok {
			continue
		}
		token := fmt.Sprintf("%s%dX", placeholderToken, i)
		fitted[token] = out
		var b []byte
		b = append(b, src[:tbl.start]...)
		b = append(b, "\n\n"+token+"\n\n"...)
		b = append(b, src[tbl.stop:]...)
		src = b
	}
	if len(fitted) == 0 {
		return "", false
	}

	rendered := heraldmd.Render(ty, src)
	for token, out := range fitted {
		if !strings.Contains(rendered, token) {
			// The placeholder did not survive as its own paragraph.
			// Do not risk a broken document.
			return "", false
		}
		rendered = strings.Replace(rendered, token, out, 1)
	}
	return rendered, true
}

// findTopLevelTables returns the tables that are direct children of the
// document, with the byte range of their source lines.
func findTopLevelTables(content []byte) []sourceTable {
	if !strings.Contains(string(content), "|") {
		return nil
	}
	doc := tableParser.Parser().Parse(text.NewReader(content))
	var tables []sourceTable
	for child := doc.FirstChild(); child != nil; child = child.NextSibling() {
		tbl, ok := child.(*east.Table)
		if !ok {
			continue
		}
		start, stop := -1, -1
		forEachCell(tbl, func(cell ast.Node) {
			lines := cell.Lines()
			for i := range lines.Len() {
				seg := lines.At(i)
				if start < 0 || seg.Start < start {
					start = seg.Start
				}
				if seg.Stop > stop {
					stop = seg.Stop
				}
			}
		})
		if start < 0 {
			continue
		}
		// Extend the range to whole lines, so the leading and trailing
		// pipes are part of it.
		for start > 0 && content[start-1] != '\n' {
			start--
		}
		for stop < len(content) && content[stop] != '\n' {
			stop++
		}
		tables = append(tables, sourceTable{start: start, stop: stop, node: tbl})
	}
	return tables
}

// forEachCell calls fn for every cell in the header and body rows of tbl.
func forEachCell(tbl *east.Table, fn func(cell ast.Node)) {
	for row := tbl.FirstChild(); row != nil; row = row.NextSibling() {
		for cell := row.FirstChild(); cell != nil; cell = cell.NextSibling() {
			fn(cell)
		}
	}
}

// blockStartPattern matches cell text that markdown would parse as a block
// (a heading, list item, blockquote, fence or thematic break) when the cell
// is rendered by itself.
var blockStartPattern = regexp.MustCompile("^(#{1,6}(\\s|$)|[-+*](\\s|$)|>|\\d{1,9}[.)](\\s|$)|```|~~~|([-*_]\\s*){3,}$)")

// renderCell renders the inline markdown of one table cell, as herald-md
// does for a cell inside a table.
func renderCell(ty *herald.Typography, content []byte, cell ast.Node) string {
	src := strings.TrimSpace(string(cell.Lines().Value(content)))
	if src == "" {
		return ""
	}
	// "\|" is a table escape. Outside the table it would stay as is, and
	// a bare "|" is plain text in a paragraph.
	src = strings.ReplaceAll(src, `\|`, "|")
	if blockStartPattern.MatchString(src) {
		// A no-break space is not indentation, so the line is a plain
		// paragraph. It is also Unicode whitespace, so emphasis at the
		// start of the cell still parses. It is removed after rendering.
		// (A backslash escape does not work: herald-md shows it as is.)
		out := heraldmd.Render(ty, []byte("\u00a0"+src))
		return strings.TrimSpace(strings.Replace(out, "\u00a0", "", 1))
	}
	return strings.TrimSpace(heraldmd.Render(ty, []byte(src)))
}

// renderFittedTable renders tbl so that every line is at most width columns.
// It returns ok false when the table already fits (herald's own output is
// correct) or when width is too small for a readable table.
func renderFittedTable(ty *herald.Typography, content []byte, tbl *east.Table, width int) (string, bool) {
	var rows [][]string
	cols := 0
	for row := tbl.FirstChild(); row != nil; row = row.NextSibling() {
		var cells []string
		for cell := row.FirstChild(); cell != nil; cell = cell.NextSibling() {
			cells = append(cells, renderCell(ty, content, cell))
		}
		cols = max(cols, len(cells))
		rows = append(rows, cells)
	}
	if len(rows) == 0 || cols == 0 {
		return "", false
	}

	th := ty.Theme()
	bs := th.TableBorderSet
	pad := th.TableCellPad
	colSep := bs.ColumnSep
	if colSep == "" {
		colSep = "│"
	}
	bordered := bs.TopLeft != ""

	natural := make([]int, cols)
	for _, row := range rows {
		for c, cell := range row {
			natural[c] = max(natural[c], lipgloss.Width(cell))
		}
	}

	overhead := cols*2*pad + (cols-1)*lipgloss.Width(colSep)
	if bordered {
		overhead += lipgloss.Width(bs.Left) + lipgloss.Width(bs.Right)
	}
	total := overhead
	for _, w := range natural {
		total += w
	}
	if total <= width {
		return "", false
	}
	avail := width - overhead
	if avail < cols*minTableColumnWidth {
		return "", false
	}
	colWidths := shareColumnWidths(natural, avail)

	aligns := make([]herald.Alignment, cols)
	for i, a := range tbl.Alignments {
		if i >= cols {
			break
		}
		switch a {
		case east.AlignCenter:
			aligns[i] = herald.AlignCenter
		case east.AlignRight:
			aligns[i] = herald.AlignRight
		}
	}

	border := th.TableBorder.Render
	hline := func(left, fill, junction, right string) string {
		if fill == "" {
			fill = "─"
		}
		parts := make([]string, cols)
		for c, w := range colWidths {
			parts[c] = strings.Repeat(fill, w+2*pad)
		}
		line := strings.Join(parts, junction)
		if !bordered {
			left, right = "", ""
		}
		return border(left + line + right)
	}

	padStr := strings.Repeat(" ", pad)
	// renderRow writes one table row, which takes as many lines as its
	// tallest wrapped cell.
	renderRow := func(row []string, st lipgloss.Style) []string {
		wrapped := make([][]string, cols)
		height := 1
		for c := range cols {
			cell := ""
			if c < len(row) {
				cell = row[c]
			}
			wrapped[c] = strings.Split(lipgloss.Wrap(cell, colWidths[c], ""), "\n")
			height = max(height, len(wrapped[c]))
		}
		lines := make([]string, height)
		for l := range height {
			cells := make([]string, cols)
			for c := range cols {
				part := ""
				if l < len(wrapped[c]) {
					part = strings.TrimRight(wrapped[c][l], " ")
				}
				cells[c] = padStr + alignTableCell(st.Render(part), lipgloss.Width(part), colWidths[c], aligns[c]) + padStr
			}
			line := strings.Join(cells, border(colSep))
			if bordered {
				line = border(bs.Left) + line + border(bs.Right)
			}
			lines[l] = line
		}
		return lines
	}

	var out []string
	if bordered {
		out = append(out, hline(bs.TopLeft, bs.Top, bs.TopJunction, bs.TopRight))
	}
	out = append(out, renderRow(rows[0], th.TableHeader)...)
	out = append(out, hline(bs.HeaderLeft, bs.Header, bs.HeaderCross, bs.HeaderRight))
	for i, row := range rows[1:] {
		// Wrapped cells make rows more than one line tall. A line between
		// rows shows where one row stops and the next starts.
		if i > 0 && bs.Row != "" {
			out = append(out, hline(bs.LeftJunction, bs.Row, bs.Cross, bs.RightJunction))
		}
		out = append(out, renderRow(row, th.TableCell)...)
	}
	if bordered {
		out = append(out, hline(bs.BottomLeft, bs.Bottom, bs.BottomJunction, bs.BottomRight))
	}
	return strings.Join(out, "\n"), true
}

// shareColumnWidths divides avail columns of content width between the table
// columns. A column that fits in an equal share keeps its natural width; the
// wide columns share what is left in proportion to their natural widths, so a
// long column gets more room than a medium one.
func shareColumnWidths(natural []int, avail int) []int {
	widths := make([]int, len(natural))
	open := make([]bool, len(natural))
	for i := range open {
		open[i] = true
	}
	remaining, count := avail, len(natural)
	for changed := true; changed && count > 0; {
		changed = false
		share := remaining / count
		for i, w := range natural {
			if open[i] && w <= share {
				widths[i] = w
				open[i] = false
				remaining -= w
				count--
				changed = true
			}
		}
	}
	if count == 0 {
		return widths
	}

	sumOpen := 0
	for i, w := range natural {
		if open[i] {
			sumOpen += w
		}
	}
	used := 0
	for i, w := range natural {
		if open[i] {
			widths[i] = max(minTableColumnWidth, remaining*w/sumOpen)
			used += widths[i]
		}
	}
	// Give the columns lost to rounding back to the open columns, and
	// take back what the minimum width added, widest column first.
	for used != remaining {
		best := -1
		for i := range widths {
			if !open[i] || (used > remaining && widths[i] <= minTableColumnWidth) {
				continue
			}
			if best < 0 || widths[i] > widths[best] {
				best = i
			}
		}
		if best < 0 {
			break
		}
		if used < remaining {
			widths[best]++
			used++
		} else {
			widths[best]--
			used--
		}
	}
	return widths
}

// alignTableCell pads rendered (whose visible width is cellWidth) to
// totalWidth with the given alignment.
func alignTableCell(rendered string, cellWidth, totalWidth int, align herald.Alignment) string {
	gap := totalWidth - cellWidth
	if gap <= 0 {
		return rendered
	}
	switch align {
	case herald.AlignRight:
		return strings.Repeat(" ", gap) + rendered
	case herald.AlignCenter:
		left := gap / 2
		return strings.Repeat(" ", left) + rendered + strings.Repeat(" ", gap-left)
	default:
		return rendered + strings.Repeat(" ", gap)
	}
}

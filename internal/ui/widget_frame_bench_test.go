package ui

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

// These benchmarks answer one question: can Kit's widget render path carry a
// real framebuffer at 30fps, or does the lipgloss pass over the returned
// string dominate?
//
// An extension widget's output is not used as-is. It goes through
// renderContentBlock (which re-parses every ANSI sequence to apply padding and
// the gutter) and lipgloss.Height in distributeHeight. For a DOOM-style
// truecolor half-block frame that string is ~190KB, so the per-frame budget at
// 30fps is 33ms for everything.

// frameStyle selects how much ANSI a synthetic frame carries per cell.
type frameStyle int

const (
	// framePlain is the Bad Apple case: ASCII ramp, no escapes at all.
	framePlain frameStyle = iota
	// frameHalfBlockRLE is a realistic renderer that only emits an escape
	// when the color actually changes between adjacent cells.
	frameHalfBlockRLE
	// frameHalfBlockWorst is the pathological case: a fresh fg+bg truecolor
	// pair on every single cell, so run-length coalescing never helps.
	frameHalfBlockWorst
)

// buildFrame produces one synthetic frame of the given shape.
func buildFrame(cols, rows int, style frameStyle, rng *rand.Rand) string {
	var b strings.Builder
	b.Grow(cols * rows * 44)

	ramp := []byte(" .:-=+*#%@")

	for y := range rows {
		if y > 0 {
			b.WriteByte('\n')
		}
		lastFg, lastBg := -1, -1
		for range cols {
			switch style {
			case framePlain:
				b.WriteByte(ramp[rng.Intn(len(ramp))])

			case frameHalfBlockRLE:
				// Blocky image: color changes roughly every 8 cells.
				fg := (rng.Intn(32) / 8) * 8
				bg := (rng.Intn(32) / 8) * 8
				if fg != lastFg || bg != lastBg {
					fmt.Fprintf(&b, "\x1b[38;2;%d;%d;%dm\x1b[48;2;%d;%d;%dm",
						fg*8, fg*4, 255-fg*8, bg*8, bg*4, 255-bg*8)
					lastFg, lastBg = fg, bg
				}
				b.WriteString("\u2580")

			case frameHalfBlockWorst:
				fmt.Fprintf(&b, "\x1b[38;2;%d;%d;%dm\x1b[48;2;%d;%d;%dm\u2580",
					rng.Intn(256), rng.Intn(256), rng.Intn(256),
					rng.Intn(256), rng.Intn(256), rng.Intn(256))
			}
		}
		if style != framePlain {
			b.WriteString("\x1b[0m")
		}
	}
	return b.String()
}

// benchFrames pre-builds a ring of frames so the benchmark measures Kit's
// render path, not the synthetic frame generation.
func benchFrames(cols, rows int, style frameStyle, n int) []string {
	rng := rand.New(rand.NewSource(1))
	out := make([]string, n)
	for i := range out {
		out[i] = buildFrame(cols, rows, style, rng)
	}
	return out
}

// BenchmarkWidgetFramePipeline measures the full per-frame cost a widget pays:
// one lipgloss.Height (the distributeHeight measurement pass) plus one
// renderContentBlock (the paint pass).
func BenchmarkWidgetFramePipeline(b *testing.B) {
	cases := []struct {
		name       string
		cols, rows int
		style      frameStyle
	}{
		{"BadApple_96x37_plain", 96, 37, framePlain},
		{"HalfBlock_96x37_rle", 96, 37, frameHalfBlockRLE},
		{"HalfBlock_96x37_worst", 96, 37, frameHalfBlockWorst},
		{"Doom_120x40_rle", 120, 40, frameHalfBlockRLE},
		{"Doom_120x40_worst", 120, 40, frameHalfBlockWorst},
		{"Doom_160x50_worst", 160, 50, frameHalfBlockWorst},
	}

	for _, tc := range cases {
		frames := benchFrames(tc.cols, tc.rows, tc.style, 16)
		b.Run(tc.name, func(b *testing.B) {
			b.ReportMetric(float64(len(frames[0])), "bytes/frame")
			b.ResetTimer()
			for i := 0; b.Loop(); i++ {
				f := frames[i%len(frames)]
				_ = lipgloss.Height(f)
				_ = renderContentBlock(f, tc.cols+4,
					WithAlign(lipgloss.Left),
					WithNoBorder(),
					WithPaddingTop(0),
					WithPaddingBottom(0),
				)
			}
		})
	}
}

// BenchmarkWidgetFrameParts splits the pipeline so it is clear which half
// costs what.
func BenchmarkWidgetFrameParts(b *testing.B) {
	frames := benchFrames(120, 40, frameHalfBlockWorst, 16)

	b.Run("lipgloss.Height", func(b *testing.B) {
		for i := 0; b.Loop(); i++ {
			_ = lipgloss.Height(frames[i%len(frames)])
		}
	})

	b.Run("renderContentBlock", func(b *testing.B) {
		for i := 0; b.Loop(); i++ {
			_ = renderContentBlock(frames[i%len(frames)], 124,
				WithAlign(lipgloss.Left), WithNoBorder())
		}
	})

	b.Run("passthrough_baseline", func(b *testing.B) {
		for i := 0; b.Loop(); i++ {
			_ = len(frames[i%len(frames)])
		}
	})
}

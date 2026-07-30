package style

import (
	"image/color"
	"regexp"
	"strings"
	"testing"
)

var logoSGR = regexp.MustCompile(`\x1b\[[0-9;:]*m`)

// logoText strips ANSI so the glyphs can be compared independently of colour.
func logoText(lines []string) string {
	return logoSGR.ReplaceAllString(strings.Join(lines, "\n"), "")
}

// The animation must only ever change colour. If a frame altered the glyphs or
// the line lengths, the block's footprint would shift mid-animation and the
// splash would jitter against the content indented beside it.
func TestKitLogoAnimationChangesOnlyColor(t *testing.T) {
	want := logoText(KitLogoLinesAt(80, KitLogoAnimationFrames))
	for f := 0; f <= KitLogoAnimationFrames; f++ {
		if got := logoText(KitLogoLinesAt(80, f)); got != want {
			t.Fatalf("frame %d changed the glyphs:\ngot:\n%s\nwant:\n%s", f, got, want)
		}
	}
}

// Every frame must occupy the same number of rows: ScrollList caches item
// height by ID, and the splash is repainted in place under a stable ID, so a
// frame of a different height would leave the cached height stale and offset
// the scroll maths.
func TestKitLogoAnimationHeightIsStable(t *testing.T) {
	for f := 0; f <= KitLogoAnimationFrames; f++ {
		if got := len(KitLogoLinesAt(80, f)); got != KitLogoHeight {
			t.Fatalf("frame %d has %d rows, want %d", f, got, KitLogoHeight)
		}
	}
}

// The block art is deliberately ragged so it can sit at any left offset;
// rendering through a cell buffer must not pad the short rows out to a
// rectangle.
func TestKitLogoLinesHaveNoTrailingSpace(t *testing.T) {
	for f := 0; f <= KitLogoAnimationFrames; f += 7 {
		for i, line := range KitLogoLinesAt(80, f) {
			plain := logoSGR.ReplaceAllString(line, "")
			if strings.TrimRight(plain, " ") != plain {
				t.Errorf("frame %d row %d has trailing whitespace: %q", f, i, plain)
			}
		}
	}
}

// The animation has to actually animate, and it has to settle. Frames before
// the end differ from the resting logo; frames at or past the end are the
// resting logo exactly.
func TestKitLogoAnimationRunsThenSettles(t *testing.T) {
	rest := strings.Join(KitLogoLines(80), "\n")

	moving := 0
	for f := range KitLogoAnimationFrames {
		if strings.Join(KitLogoLinesAt(80, f), "\n") != rest {
			moving++
		}
	}
	if moving < KitLogoAnimationFrames/2 {
		t.Errorf("only %d of %d frames differ from the resting logo",
			moving, KitLogoAnimationFrames)
	}

	for _, f := range []int{KitLogoAnimationFrames, KitLogoAnimationFrames + 1, 10_000} {
		if got := strings.Join(KitLogoLinesAt(80, f), "\n"); got != rest {
			t.Errorf("frame %d did not settle to the resting logo", f)
		}
	}
	// A negative frame must not panic or highlight anything.
	if got := strings.Join(KitLogoLinesAt(80, -1), "\n"); got != rest {
		t.Error("a negative frame should render the resting logo")
	}
}

// The two phases run in sequence, not together: the shine crosses the whole
// mark, then the scanner bar bounces alone. During the scanner phase nothing
// above the bar may be highlighted.
func TestKitLogoPhasesAreSequential(t *testing.T) {
	// Sweep phase: the highlight must reach rows above the scanner bar.
	sweptWordmark := false
	for f := range kitLogoSweepFrames {
		for y := range kitLogoScannerRow {
			for x := range kitLogoWidth {
				if kitLogoHighlight(x, y, f) > 0 {
					sweptWordmark = true
				}
			}
		}
	}
	if !sweptWordmark {
		t.Error("the sweep never highlighted the wordmark")
	}

	// Scanner phase: only the bar moves.
	litBar := false
	for f := kitLogoSweepFrames; f < KitLogoAnimationFrames; f++ {
		for y := range KitLogoHeight {
			for x := range kitLogoWidth {
				amt := kitLogoHighlight(x, y, f)
				if y != kitLogoScannerRow && amt > 0 {
					t.Fatalf("frame %d highlights row %d during the scanner phase", f, y)
				}
				if y == kitLogoScannerRow && amt > 0 {
					litBar = true
				}
			}
		}
	}
	if !litBar {
		t.Error("the scanner phase never lit the bar")
	}
}

// The scanner makes one full bounce: it starts at the left, reaches the right
// at the midpoint, and returns.
func TestKitLogoScannerBounces(t *testing.T) {
	left := logoScannerIntensity(0, 0)
	right := logoScannerIntensity(kitLogoWidth-1, 0.5)
	back := logoScannerIntensity(0, 1)

	if left <= 0 {
		t.Error("scanner should start lit at the left edge")
	}
	if right <= 0 {
		t.Error("scanner should reach the right edge at the midpoint")
	}
	if back <= 0 {
		t.Error("scanner should return to the left edge")
	}
	// Mid-sweep the left edge is dark — otherwise it is a glow, not a scan.
	if mid := logoScannerIntensity(0, 0.5); mid > 0 {
		t.Errorf("left edge should be dark at the midpoint, got %v", mid)
	}
}

// The resting gradient is evaluated per cell rather than in eight stops, which
// is the point of moving off ApplyGradient — it should hold visibly more
// distinct colours across the 22-column mark.
func TestKitLogoRestingGradientIsSmooth(t *testing.T) {
	perCell := map[string]bool{}
	for x := range kitLogoWidth {
		c := kitLogoRestColor(x, GetTheme().Primary, GetTheme().Accent)
		r, g, b, _ := c.RGBA()
		perCell[string(rune(r>>8))+string(rune(g>>8))+string(rune(b>>8))] = true
	}

	banded := map[string]bool{}
	for _, seg := range logoSGR.FindAllString(ApplyGradient(strings.Repeat("█", kitLogoWidth),
		GetTheme().Primary, GetTheme().Accent), -1) {
		banded[seg] = true
	}

	if len(perCell) <= len(banded) {
		t.Errorf("per-cell gradient has %d colours, 8-stop has %d — expected more",
			len(perCell), len(banded))
	}
}

// Narrow terminals fall back to a plain wordmark, and that fallback must not
// try to animate.
func TestKitLogoNarrowFallback(t *testing.T) {
	for _, w := range []int{0, 1, kitLogoWidth - 1} {
		lines := KitLogoLinesAt(w, 5)
		if len(lines) != 1 {
			t.Fatalf("width %d: got %d lines, want a single-line fallback", w, len(lines))
		}
		if !strings.Contains(logoSGR.ReplaceAllString(lines[0], ""), "KIT") {
			t.Errorf("width %d: fallback should read KIT, got %q", w, lines[0])
		}
		if KitLogoFits(w) {
			t.Errorf("KitLogoFits(%d) should be false", w)
		}
	}
	if !KitLogoFits(kitLogoWidth) {
		t.Errorf("KitLogoFits(%d) should be true", kitLogoWidth)
	}
}

// The shine is derived from the active theme, not hardcoded: switching themes
// must change it.
func TestLogoShineFollowsTheme(t *testing.T) {
	orig := GetTheme()
	t.Cleanup(func() { SetTheme(orig) })

	themes := builtinThemes()
	SetTheme(themes["kitt"])
	kitt := logoShineColor()
	SetTheme(themes["matrix"])
	matrix := logoShineColor()

	if distance(kitt, matrix) == 0 {
		t.Error("the shine colour is identical across two very different themes")
	}
}

// relativeLuminance is the standard sRGB luma weighting, used here only to ask
// whether one colour is lighter than another.
func relativeLuminance(c color.Color) float64 {
	r, g, b, _ := c.RGBA()
	return 0.2126*float64(r>>8) + 0.7152*float64(g>>8) + 0.0722*float64(b>>8)
}

// The shine must always be lighter than the glyphs it crosses, in every theme
// and under either terminal background. This is the property the effect is
// named for: an earlier contrast-pole rule inverted on pale palettes and swept
// something *darker* across the wordmark, which reads as a smudge.
func TestLogoShineIsAlwaysLighterThanWordmark(t *testing.T) {
	origTheme, origDark, origProfile := GetTheme(), isDarkBackground(), terminalColorProfile()
	t.Cleanup(func() { SetTerminalCapabilities(origDark, origProfile); SetTheme(origTheme) })

	for _, dark := range []bool{true, false} {
		SetTerminalCapabilities(dark, origProfile)
		for name, th := range builtinThemes() {
			SetTheme(th)
			shine := relativeLuminance(logoShineColor())
			for _, x := range []int{0, kitLogoWidth / 2, kitLogoWidth - 1} {
				base := relativeLuminance(kitLogoRestColor(x, th.Primary, th.Accent))
				if shine <= base {
					t.Errorf("isDarkBg=%v theme %q: shine (luma %.0f) is not lighter than the wordmark at column %d (luma %.0f)",
						dark, name, shine, x, base)
				}
			}
		}
	}
}

// Whatever theme is active, the shine has to be far enough from the wordmark's
// own colours to register as a highlight crossing it.
func TestLogoShineContrastsWithWordmarkInEveryTheme(t *testing.T) {
	origTheme, origDark, origProfile := GetTheme(), isDarkBackground(), terminalColorProfile()
	t.Cleanup(func() { SetTerminalCapabilities(origDark, origProfile); SetTheme(origTheme) })

	// Floor sits just below the measured worst case (~46, catppuccin on dark
	// and vesper on light). Those two palettes have an unusually pale Primary,
	// which leaves little headroom above it.
	const minDistance = 40

	for _, dark := range []bool{true, false} {
		SetTerminalCapabilities(dark, origProfile)
		for name, th := range builtinThemes() {
			SetTheme(th)
			shine := logoShineColor()
			// Check across the whole gradient, not just its midpoint: the
			// shine has to stay visible at both ends of the wordmark.
			for _, x := range []int{0, kitLogoWidth / 2, kitLogoWidth - 1} {
				base := kitLogoRestColor(x, th.Primary, th.Accent)
				if d := distance(shine, base); d < minDistance {
					t.Errorf("isDarkBg=%v theme %q: shine is only %.0f from the wordmark at column %d (want >= %d)",
						dark, name, d, x, minDistance)
				}
			}
		}
	}
}

// The shine must also stay clear of the terminal background, or the glyphs it
// touches appear to blink out instead of lighting up. This is what stops the
// lift toward white from simply being turned up: on a light background it
// closes on the page fast.
func TestLogoShineDoesNotCollideWithBackground(t *testing.T) {
	origTheme, origDark, origProfile := GetTheme(), isDarkBackground(), terminalColorProfile()
	t.Cleanup(func() { SetTerminalCapabilities(origDark, origProfile); SetTheme(origTheme) })

	// Worst measured case is ~53 (vesper on a light background).
	const minDistance = 45

	for _, dark := range []bool{true, false} {
		SetTerminalCapabilities(dark, origProfile)
		for name, th := range builtinThemes() {
			SetTheme(th)
			if d := distance(logoShineColor(), th.Background); d < minDistance {
				t.Errorf("isDarkBg=%v theme %q: shine is only %.0f from the background (want >= %d)",
					dark, name, d, minDistance)
			}
		}
	}
}

func TestBlendColors(t *testing.T) {
	a := GetTheme().Primary
	b := GetTheme().Accent

	ar, ag, ab, _ := a.RGBA()
	gr, gg, gb, _ := blendColors(a, b, 0).RGBA()
	if ar>>8 != gr>>8 || ag>>8 != gg>>8 || ab>>8 != gb>>8 {
		t.Error("t=0 should return the first colour")
	}
	br, bg, bb, _ := b.RGBA()
	hr, hg, hb, _ := blendColors(a, b, 1).RGBA()
	if br>>8 != hr>>8 || bg>>8 != hg>>8 || bb>>8 != hb>>8 {
		t.Error("t=1 should return the second colour")
	}
	// Out-of-range values clamp rather than extrapolating into nonsense.
	if blendColors(a, b, -5) != a {
		t.Error("negative t should clamp to the first colour")
	}
	if blendColors(a, b, 5) != b {
		t.Error("t past 1 should clamp to the second colour")
	}
}

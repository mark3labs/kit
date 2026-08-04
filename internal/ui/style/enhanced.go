package style

import (
	"fmt"
	"image/color"
	"math"
	"os"
	"strings"
	"sync"

	"charm.land/lipgloss/v2"
	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/charmbracelet/colorprofile"
	uv "github.com/charmbracelet/ultraviolet"
)

// Enhanced styling utilities and theme definitions

// terminalCapabilities captures the per-terminal styling inputs: whether the
// background is dark (drives adaptive colour selection) and the terminal's
// colour depth (drives whether smooth animation is worthwhile).
type terminalCapabilities struct {
	darkBackground bool
	colorProfile   colorprofile.Profile
}

// termCaps holds the resolved terminal capabilities. It is nil until first use
// or until SetTerminalCapabilities is called, so the (blocking, fd-touching)
// probe never runs at package-init time. Guarded by termCapsMu.
var (
	termCapsMu sync.RWMutex
	termCaps   *terminalCapabilities
)

// resolveTerminalCapabilities probes the current process's terminal. It is the
// default source of capabilities when SetTerminalCapabilities has not been
// called, and is kept separate so it never fires as an import-time side effect.
func resolveTerminalCapabilities() terminalCapabilities {
	return terminalCapabilities{
		darkBackground: lipgloss.HasDarkBackground(os.Stdin, os.Stdout),
		colorProfile:   colorprofile.Env(os.Environ()),
	}
}

// currentTerminalCapabilities returns the active capabilities, resolving them
// lazily against the process terminal the first time they are needed. Callers
// that serve a specific client should call SetTerminalCapabilities first so
// this never falls back to whatever fds the process happens to hold.
func currentTerminalCapabilities() terminalCapabilities {
	termCapsMu.RLock()
	c := termCaps
	termCapsMu.RUnlock()
	if c != nil {
		return *c
	}

	termCapsMu.Lock()
	defer termCapsMu.Unlock()
	if termCaps == nil {
		resolved := resolveTerminalCapabilities()
		termCaps = &resolved
	}
	return *termCaps
}

// SetTerminalCapabilities overrides the terminal background and colour profile
// used by every subsequent styling decision in this package.
//
// It exists so a frontend can supply the capabilities of the terminal it is
// actually serving instead of inheriting whatever the process's own fds report.
// This replaces the former package-init probes, which fired once against the
// process stdin/stdout and could not represent more than one terminal.
//
// Call this on the UI goroutine, the same as SetTheme: besides the
// mutex-guarded capability swap, it rebuilds theme-derived caches
// (currentTheme, the generation counter, the typography/style/syntax caches)
// that the render loop reads unsynchronized. It is intended to be called once
// during setup, before the render loop starts, or from within Update.
func SetTerminalCapabilities(darkBackground bool, colorProfile colorprofile.Profile) {
	termCapsMu.Lock()
	termCaps = &terminalCapabilities{
		darkBackground: darkBackground,
		colorProfile:   colorProfile,
	}
	termCapsMu.Unlock()

	// A lazily-derived DefaultTheme baked in the previously-detected
	// background, so drop it and let GetTheme rebuild against the new
	// capabilities. A theme the caller chose explicitly is left as-is.
	if !themeExplicitlySet {
		currentTheme = nil
	}
	invalidateThemeCaches()
}

// isDarkBackground reports whether the active terminal has a dark background,
// resolving capabilities lazily on first use.
func isDarkBackground() bool {
	return currentTerminalCapabilities().darkBackground
}

// ResolveTerminalCapabilities eagerly resolves the terminal background and
// colour profile against the current process terminal, unless they have
// already been resolved or supplied via SetTerminalCapabilities.
//
// The interactive frontend calls this once during startup, before the TUI
// takes over the terminal, so the background-detection OSC query runs at a
// point where it cannot race the event loop's reads of stdin. Headless
// frontends should call SetTerminalCapabilities instead and skip this, to avoid
// probing fds that do not describe their client.
func ResolveTerminalCapabilities() {
	_ = currentTerminalCapabilities()
}

// terminalColorProfile returns the active terminal's colour depth, resolving
// capabilities lazily on first use.
func terminalColorProfile() colorprofile.Profile {
	return currentTerminalCapabilities().colorProfile
}

// SupportsSmoothAnimation reports whether the terminal has the colour depth
// for a gradient animation to read as motion.
//
// Below ANSI256 a sweep collapses onto a handful of available colours and
// strobes between them, which looks like a rendering fault rather than an
// effect. Honouring the profile also means NO_COLOR and non-TTY output opt out
// for free.
func SupportsSmoothAnimation() bool {
	return terminalColorProfile() >= colorprofile.ANSI256
}

// AdaptiveColor picks between a light-mode and dark-mode hex color string
// based on the detected terminal background. This replaces the old
// lipgloss.AdaptiveColor{Light: ..., Dark: ...} pattern from v1.
func AdaptiveColor(light, dark string) color.Color {
	if isDarkBackground() {
		return lipgloss.Color(dark)
	}
	return lipgloss.Color(light)
}

// currentTheme holds the active UI theme. It is nil until first use so that
// DefaultTheme — which calls AdaptiveColor and would therefore probe the
// terminal — is not evaluated at package-init time. Resolved lazily by
// GetTheme, or set explicitly by SetTheme.
//
// Like themeGeneration below, this and themeExplicitlySet are mutated only on
// the UI goroutine (SetTheme / SetTerminalCapabilities) and read from the same
// Update/View cycle, so they carry no lock of their own.
var currentTheme *Theme

// themeExplicitlySet records whether the active theme came from SetTheme (true)
// or was lazily derived from DefaultTheme (false). A derived theme bakes in the
// terminal background detected when it was built, so SetTerminalCapabilities
// must discard and rebuild it; an explicitly chosen theme is left untouched.
var themeExplicitlySet bool

// themeGeneration counts theme changes. It starts at 1 so that a zero-valued
// generation stamp — the state of any freshly allocated cache — never compares
// equal to it and is therefore always treated as stale.
//
// Only accessed from BubbleTea's single-threaded Update/View cycle, like the
// rest of the theme state in this package.
var themeGeneration uint64 = 1

// GetTheme returns the currently active UI theme. The theme controls all color
// and styling decisions throughout the application's interface. On first use it
// lazily derives DefaultTheme against the active terminal capabilities.
func GetTheme() Theme {
	if currentTheme == nil {
		derived := DefaultTheme()
		currentTheme = &derived
	}
	return *currentTheme
}

// ThemeGeneration returns a counter that increments on every theme change.
//
// Anything that memoizes rendered output stamps the cached value with the
// generation it was produced at and discards it once the stamp no longer
// matches. That keeps colors correct across a theme switch without forcing
// eager re-rendering: each cache is rebuilt lazily, at most once per switch,
// and only when something asks for it again.
func ThemeGeneration() uint64 {
	return themeGeneration
}

// SetTheme updates the global UI theme, affecting all subsequent rendering
// operations. This allows runtime theme switching for different visual preferences.
// It also invalidates the markdownTypographyCache so the next call to
// GetMarkdownTypography picks up the new theme.
func SetTheme(theme Theme) {
	currentTheme = &theme
	themeExplicitlySet = true
	invalidateThemeCaches()
}

// activeThemeName records the name last passed to ApplyTheme. It is reported
// by CurrentThemeName so callers outside this package — the extension API's
// ctx.GetTheme, for instance — can name the active theme without keeping
// their own parallel bookkeeping.
var activeThemeName string

// CurrentThemeName returns the name of the active theme, or "" when no theme
// has been applied by name and the lazily derived default is in effect.
func CurrentThemeName() string { return activeThemeName }

// HexOf renders a color as a "#rrggbb" string.
//
// color.Color reports 16-bit premultiplied channels, so each is scaled down to
// 8 bits. A nil color yields "", which callers treat as "unset" rather than
// black — the two are not interchangeable when the value feeds a theme slot.
func HexOf(c color.Color) string {
	if c == nil {
		return ""
	}
	r, g, b, _ := c.RGBA()
	return fmt.Sprintf("#%02x%02x%02x", uint8(r>>8), uint8(g>>8), uint8(b>>8))
}

// invalidateThemeCaches bumps the theme generation and drops every cache whose
// contents depend on the active theme colours, so the next access rebuilds
// them. Shared by SetTheme and SetTerminalCapabilities.
func invalidateThemeCaches() {
	themeGeneration++             // invalidate generation-stamped render caches
	markdownTypographyCache = nil // invalidate cached renderer; colors may have changed
	uiTypographyCache = nil       // invalidate cached block typography; colors may have changed
	styleCache = nil              // invalidate cached styles; colors may have changed
	syntaxStyleCache = nil        // invalidate cached chroma style; colors may have changed
}

// CachedStyles holds pre-built lipgloss styles that are reused across
// render frames. Invalidated by SetTheme, lazily rebuilt on next access.
// Only accessed from BubbleTea's single-threaded Update/View cycle.
type CachedStyles struct {
	// render/blocks.go
	FileTokenAccent lipgloss.Style // Foreground(Accent).Bold(true)
	Muted           lipgloss.Style // Foreground(Muted)
	VeryMuted       lipgloss.Style // Foreground(VeryMuted)
	Accent          lipgloss.Style // Foreground(Accent)
	MarginBottom1   lipgloss.Style // MarginBottom(1)

	// stream.go - spinner phases
	SpinnerBright lipgloss.Style // Foreground(Primary)
	SpinnerMed    lipgloss.Style // Foreground(Muted)
	SpinnerDim    lipgloss.Style // Foreground(VeryMuted)
	SpinnerOff    lipgloss.Style // Foreground(MutedBorder)

	// message_items.go - bash output
	BashHeader lipgloss.Style // Foreground(Muted).Italic(true)
	BashStderr lipgloss.Style // Foreground(Error)

	// render/blocks.go - tool block
	ToolSuccess lipgloss.Style // Foreground(Success)
	ToolError   lipgloss.Style // Foreground(Error)
	ToolInfo    lipgloss.Style // Foreground(Info).Bold(true)
	ToolMuted   lipgloss.Style // Foreground(Muted)

	// common
	ErrorFg  lipgloss.Style // Foreground(Error)
	TextBold lipgloss.Style // Foreground(Text).Bold(true)
}

var styleCache *CachedStyles

// GetCachedStyles returns the pre-built style cache, creating it lazily
// from the current theme. Invalidated by SetTheme.
func GetCachedStyles() *CachedStyles {
	if styleCache != nil {
		return styleCache
	}
	theme := GetTheme()
	styleCache = &CachedStyles{
		FileTokenAccent: lipgloss.NewStyle().Foreground(theme.Accent).Bold(true),
		Muted:           lipgloss.NewStyle().Foreground(theme.Muted),
		VeryMuted:       lipgloss.NewStyle().Foreground(theme.VeryMuted),
		Accent:          lipgloss.NewStyle().Foreground(theme.Accent),
		MarginBottom1:   lipgloss.NewStyle().MarginBottom(1),
		SpinnerBright:   lipgloss.NewStyle().Foreground(theme.Primary),
		SpinnerMed:      lipgloss.NewStyle().Foreground(theme.Muted),
		SpinnerDim:      lipgloss.NewStyle().Foreground(theme.VeryMuted),
		SpinnerOff:      lipgloss.NewStyle().Foreground(theme.MutedBorder),
		BashHeader:      lipgloss.NewStyle().Foreground(theme.Muted).Italic(true),
		BashStderr:      lipgloss.NewStyle().Foreground(theme.Error),
		ToolSuccess:     lipgloss.NewStyle().Foreground(theme.Success),
		ToolError:       lipgloss.NewStyle().Foreground(theme.Error),
		ToolInfo:        lipgloss.NewStyle().Foreground(theme.Info).Bold(true),
		ToolMuted:       lipgloss.NewStyle().Foreground(theme.Muted),
		ErrorFg:         lipgloss.NewStyle().Foreground(theme.Error),
		TextBold:        lipgloss.NewStyle().Foreground(theme.Text).Bold(true),
	}
	return styleCache
}

// MarkdownThemeColors defines colors for markdown rendering and syntax highlighting.
type MarkdownThemeColors struct {
	Text    color.Color
	Muted   color.Color
	Heading color.Color
	Emph    color.Color
	Strong  color.Color
	Link    color.Color
	Code    color.Color
	Error   color.Color
	Keyword color.Color
	String  color.Color
	Number  color.Color
	Comment color.Color
}

// Theme defines a comprehensive color scheme for the application's UI, supporting
// both light and dark terminal modes through adaptive colors. Inspired by the
// Knight Rider KITT aesthetic — scanner reds, amber dashboard glows, and dark
// cockpit tones.
type Theme struct {
	Primary     color.Color
	Secondary   color.Color
	Success     color.Color
	Warning     color.Color
	Error       color.Color
	Info        color.Color
	Text        color.Color
	Muted       color.Color
	VeryMuted   color.Color
	Background  color.Color
	Border      color.Color
	MutedBorder color.Color
	System      color.Color
	Tool        color.Color
	Accent      color.Color
	Highlight   color.Color

	// Diff block backgrounds
	DiffInsertBg  color.Color // Green-tinted bg for added lines
	DiffDeleteBg  color.Color // Red-tinted bg for removed lines
	DiffEqualBg   color.Color // Neutral bg for context lines
	DiffMissingBg color.Color // Empty-cell bg when sides are uneven

	// Code/output block backgrounds
	CodeBg   color.Color // Background for code blocks (Read tool)
	GutterBg color.Color // Line-number gutter background
	WriteBg  color.Color // Green-tinted bg for Write tool content

	// InputBg fills the composer bar. It is a shade off the terminal
	// background — enough to read as a distinct surface, not enough to
	// compete with message content for attention.
	InputBg color.Color

	// Markdown rendering and syntax highlighting colors
	Markdown MarkdownThemeColors
}

// DefaultTheme creates and returns the default KIT theme inspired by the
// Knight Rider KITT aesthetic — scanner reds, amber dashboard glows, and a
// dark cockpit. Everything stays in the warm red/amber/gray family of KITT's
// instrument panel, with two deliberate exceptions.
//
// Success and Error must never be mistaken for Warning or for the brand red,
// because they carry opposite meanings and they appear in the same places (the
// turn receipt, the activity row, tool markers). Success therefore leans
// olive-green and Error leans crimson: both still warm, both unmistakably not
// amber and not scanner red.
func DefaultTheme() Theme {
	return Theme{
		Primary:     AdaptiveColor("#CC1100", "#FF2200"), // KITT scanner red
		Secondary:   AdaptiveColor("#CC6600", "#FF8800"), // Amber dashboard glow
		Success:     AdaptiveColor("#5F7A1F", "#A3BE4C"), // Olive green — distinct from amber
		Warning:     AdaptiveColor("#CC8800", "#FFB800"), // Amber caution light
		Error:       AdaptiveColor("#C21038", "#FF4466"), // Crimson — distinct from scanner red
		Info:        AdaptiveColor("#BB6600", "#DD8833"), // Warm amber readout
		Text:        AdaptiveColor("#1A1A1A", "#E0E0E0"), // Console text
		Muted:       AdaptiveColor("#707070", "#808080"), // Dimmed readout
		VeryMuted:   AdaptiveColor("#A0A0A0", "#505050"), // Inactive element
		Background:  AdaptiveColor("#F0F0F0", "#0D0D0D"), // Cockpit interior
		Border:      AdaptiveColor("#B0B0B0", "#3A3A3A"), // Panel edge
		MutedBorder: AdaptiveColor("#D0D0D0", "#222222"), // Subtle divider
		System:      AdaptiveColor("#CC6600", "#FF8800"), // Amber system status
		Tool:        AdaptiveColor("#CC6600", "#FF8800"), // Amber instrument
		Accent:      AdaptiveColor("#DD2222", "#FF4444"), // Secondary scanner glow
		Highlight:   AdaptiveColor("#FFF0F0", "#1A1010"), // Red-tinted mantle

		// Diff backgrounds
		DiffInsertBg:  AdaptiveColor("#F0E8D0", "#2A2410"), // Warm amber tint (added)
		DiffDeleteBg:  AdaptiveColor("#F5D5D5", "#2E1A1A"), // Red tint (removed)
		DiffEqualBg:   AdaptiveColor("#E8E8E8", "#161616"), // Neutral
		DiffMissingBg: AdaptiveColor("#E0E0E0", "#111111"), // Darker neutral

		// Code & output backgrounds
		CodeBg:   AdaptiveColor("#E8E8E8", "#161616"), // Matches DiffEqualBg
		GutterBg: AdaptiveColor("#E0E0E0", "#111111"), // Slightly darker
		WriteBg:  AdaptiveColor("#F0E8D0", "#2A2410"), // Warm amber tint

		// Composer surface — one step off the terminal background.
		InputBg: AdaptiveColor("#E6E6E6", "#141414"),

		// Markdown & syntax highlighting — all warm tones
		Markdown: MarkdownThemeColors{
			Text:    AdaptiveColor("#1A1A1A", "#E0E0E0"), // Console text
			Muted:   AdaptiveColor("#707070", "#808080"), // Dimmed readout
			Heading: AdaptiveColor("#CC1100", "#FF2200"), // Scanner red, matching Primary
			Emph:    AdaptiveColor("#CC8800", "#FFB800"), // Amber emphasis
			Strong:  AdaptiveColor("#1A1A1A", "#E0E0E0"), // Bright text
			Link:    AdaptiveColor("#CC4400", "#FF7744"), // Warm orange link
			Code:    AdaptiveColor("#333333", "#CCCCCC"), // Inline code
			Error:   AdaptiveColor("#C21038", "#FF4466"), // Crimson, matching Theme.Error
			Keyword: AdaptiveColor("#CC3300", "#FF6644"), // Orange-red keyword
			String:  AdaptiveColor("#BB7700", "#DDAA33"), // Amber string
			Number:  AdaptiveColor("#CC8800", "#FFB800"), // Amber number
			Comment: AdaptiveColor("#909090", "#606060"), // Dark gray comment
		},
	}
}

// IsDarkBackground returns whether the active terminal has a dark background.
// Capabilities are resolved lazily on first use, or supplied ahead of time via
// SetTerminalCapabilities.
func IsDarkBackground() bool {
	return isDarkBackground()
}

// CreateBadge generates a styled badge or label with inverted colors (text on
// colored background) for highlighting important tags, statuses, or categories.
func CreateBadge(text string, c color.Color) string {
	return lipgloss.NewStyle().
		Foreground(AdaptiveColor("#FFFFFF", "#000000")).
		Background(c).
		Padding(0, 1).
		Bold(true).
		Render(text)
}

// interpolateColor blends between two colors based on position (0.0 to 1.0)
// using linear RGB channel interpolation.
func interpolateColor(a, b color.Color, pos float64) color.Color {
	r1, g1, b1, _ := a.RGBA()
	r2, g2, b2, _ := b.RGBA()

	r := uint8(float64(r1>>8)*(1-pos) + float64(r2>>8)*pos)
	g := uint8(float64(g1>>8)*(1-pos) + float64(g2>>8)*pos)
	bl := uint8(float64(b1>>8)*(1-pos) + float64(b2>>8)*pos)

	return lipgloss.Color(fmt.Sprintf("#%02x%02x%02x", r, g, bl))
}

// ApplyGradient applies a color gradient from colorA to colorB across the text.
// Uses ~8 color stops for performance rather than per-character coloring.
func ApplyGradient(text string, colorA, colorB color.Color) string {
	runes := []rune(text)
	if len(runes) == 0 {
		return text
	}

	const maxStops = 8
	segmentSize := max(len(runes)/maxStops, 1)

	var result strings.Builder
	for i := 0; i < len(runes); i += segmentSize {
		end := min(i+segmentSize, len(runes))

		pos := float64(i) / float64(len(runes))
		c := interpolateColor(colorA, colorB, pos)
		style := lipgloss.NewStyle().Foreground(c)
		result.WriteString(style.Render(string(runes[i:end])))
	}

	return result.String()
}

// kitLogoArt is the KIT wordmark in block letters. Lines are left-aligned and
// not padded to a uniform width — trailing space would only be invisible
// gradient — so the widest line (the top row and the scanner bar) sets the
// block's footprint. The block can be placed at any left offset without
// re-centering.
var kitLogoArt = []string{
	"██╗  ██╗ ██╗ ████████╗",
	"██║ ██╔╝ ██║ ╚══██╔══╝",
	"█████╔╝  ██║    ██║",
	"██╔═██╗  ██║    ██║",
	"██║  ██╗ ██║    ██║",
	"╚═╝  ╚═╝ ╚═╝    ╚═╝",
	// KITT's scanner bar, sized to the wordmark's widest row.
	"░░ ▒▒ ▓▓ ████ ▓▓ ▒▒ ░░",
}

// kitLogoWidth is the width of the widest line in kitLogoArt, i.e. the number
// of columns the block needs to render without wrapping. KitLogoLines falls
// back to a plain wordmark below this width.
const kitLogoWidth = 22

// kitLogoScannerRow is the index of the KITT scanner bar within kitLogoArt.
// The startup animation drives that row separately from the wordmark above it.
const kitLogoScannerRow = 6

// KitLogoHeight is the number of rows the wordmark block occupies.
const KitLogoHeight = 7

// KitLogoFits reports whether the block art will render at the given content
// width, or whether KitLogoLines will fall back to a plain "KIT".
func KitLogoFits(contentWidth int) bool {
	return contentWidth >= kitLogoWidth
}

// Startup animation timing, in frames.
//
// The animation runs in two phases: a shine sweeps diagonally across the
// wordmark, then the scanner bar makes one bounce beneath it. Both are short —
// this is a flourish on a splash the user is reading past, not something to
// sit through.
//
// These are counts, not durations: the wordmark does not drive itself, it
// advances on the UI's shared frame clock, so how long each phase lasts is a
// function of the rate that clock runs at. The parenthesised times below
// assume 30fps, which is what the sweep was tuned against and what the clock
// ticks at; a test on the clock's side pins the resulting total duration so
// retuning the clock cannot quietly turn the flourish into something the user
// has to wait out.
const (
	kitLogoSweepFrames   = 20 // ~0.67s at 30fps
	kitLogoScannerFrames = 30 // ~1.0s at 30fps, one full left-right-left bounce
)

// KitLogoAnimationFrames is the total number of frames in the startup
// animation. Frame indices at or beyond this render the resting logo.
const KitLogoAnimationFrames = kitLogoSweepFrames + kitLogoScannerFrames

// How far the shine lifts Primary toward white.
//
// The amount depends on the terminal background because the headroom does. On
// a dark background there is nothing above the wordmark to collide with, so
// the shine can go most of the way to white and read as genuinely hot. On a
// light background that same lift would put the highlight within ~20 of the
// page (measured: vesper at 0.80 lands 16 away) and the glyphs would appear to
// blink out rather than glint, so the lift is held back to keep the mark
// visible against the paper.
//
// Both values are tuned so the shine stays at least ~45 from the wordmark's
// own colours across every shipped theme.
const (
	logoShineLightenDark  = 0.80
	logoShineLightenLight = 0.40
)

// logoShineColor returns the colour a glyph reaches at the centre of the
// shine: the theme's Primary lifted toward white.
//
// A highlight is the mark's own colour catching the light, so it keeps
// Primary's hue and simply burns brighter. Deriving it from Primary also means
// the effect is always a glint and never a shadow — an earlier rule that
// picked whichever of Text or Background contrasted more would invert on pale
// palettes and sweep something dark across the wordmark, which reads as a
// smudge rather than a shine.
//
// The one palette this serves less well is a theme whose Primary is already
// near-white (catppuccin): there is little room left above it, so the glint is
// subtler there. That is a smaller cost than inverting the gesture.
func logoShineColor() color.Color {
	amount := logoShineLightenLight
	if isDarkBackground() {
		amount = logoShineLightenDark
	}
	return blendColors(GetTheme().Primary, color.RGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}, amount)
}

// blendColors mixes two colours in RGB space. t is clamped to [0,1].
//
// This is interpolateColor's arithmetic without the fmt.Sprintf round-trip
// through a hex string: the logo evaluates a colour per cell per frame, so the
// allocation is worth skipping.
func blendColors(a, b color.Color, t float64) color.Color {
	if t <= 0 {
		return a
	}
	if t >= 1 {
		return b
	}
	r1, g1, b1, _ := a.RGBA()
	r2, g2, b2, _ := b.RGBA()
	return color.RGBA{
		R: uint8(float64(r1>>8)*(1-t) + float64(r2>>8)*t),
		G: uint8(float64(g1>>8)*(1-t) + float64(g2>>8)*t),
		B: uint8(float64(b1>>8)*(1-t) + float64(b2>>8)*t),
		A: 0xFF,
	}
}

// kitLogoRestColor is the settled colour of the glyph in column x: the
// Primary→Accent gradient evaluated per cell.
//
// ApplyGradient quantises to eight stops, which is the right trade for a line
// of prose but visibly bands across a 22-column wordmark. Here the gradient is
// continuous.
func kitLogoRestColor(x int, primary, accent color.Color) color.Color {
	return blendColors(primary, accent, float64(x)/float64(kitLogoWidth-1))
}

// logoShineIntensity returns the highlight weight for a travelling diagonal
// band, in [0,1].
//
// The band is skewed so upper rows light slightly before lower ones. A
// straight vertical edge reads as a wipe — as though the logo were being
// redrawn a column at a time — where a diagonal reads as light crossing a
// solid object. Intensity falls off quadratically from the centre, giving a
// tight core with a soft shoulder rather than a hard-edged bar.
func logoShineIntensity(x, y int, t float64) float64 {
	const (
		width = 5.0 // half-width of the band, in cells
		skew  = 0.6 // columns of lead per row
	)
	// Travel from fully off the left edge to fully off the right.
	span := float64(kitLogoWidth) + 2*width + skew*float64(len(kitLogoArt))
	pos := -width + t*span
	xs := float64(x) + float64(len(kitLogoArt)-1-y)*skew
	d := math.Abs(xs - pos)
	if d > width {
		return 0
	}
	n := 1 - d/width
	return n * n
}

// logoScannerIntensity returns the highlight weight for the scanner bar: a
// bright cell bouncing left→right→left with a short trail, matching the
// activity spinner's Knight Rider sweep.
func logoScannerIntensity(x int, t float64) float64 {
	const trail = 4.0
	// One full bounce over the phase: 0→1→0.
	phase := math.Mod(t*2, 2)
	if phase > 1 {
		phase = 2 - phase
	}
	pos := phase * float64(kitLogoWidth-1)
	d := math.Abs(float64(x) - pos)
	if d > trail {
		return 0
	}
	n := 1 - d/trail
	return n * n
}

// KitLogoLines returns the KIT wordmark as gradient-colored lines, sized for a
// block that will be rendered at the given content width.
//
// Fixed-width block art cannot wrap, so below the width it needs the wordmark
// degrades to a plain bold "KIT" rather than spilling across the terminal.
func KitLogoLines(contentWidth int) []string {
	return KitLogoLinesAt(contentWidth, KitLogoAnimationFrames)
}

// KitLogoLinesAt returns the wordmark at the given startup-animation frame.
// Frames at or beyond KitLogoAnimationFrames render the resting logo, so a
// caller that does not animate can simply ask for the final frame — which is
// what KitLogoLines does.
//
// Colour is computed per cell as a function of (x, y, frame) and written into
// an ultraviolet buffer, because the effects vary along both axes and over
// time; that cannot be expressed as a gradient across a run of runes. Each row
// gets its own buffer sized to that row's own length, which keeps the art
// unpadded — a rectangular buffer would emit trailing spaces on the short
// rows, and the block is deliberately ragged so it can sit at any left offset.
func KitLogoLinesAt(contentWidth, frame int) []string {
	theme := GetTheme()
	if contentWidth < kitLogoWidth {
		return []string{lipgloss.NewStyle().Bold(true).Foreground(theme.Primary).Render("KIT")}
	}

	shine := logoShineColor()

	out := make([]string, 0, len(kitLogoArt))
	for y, line := range kitLogoArt {
		runes := []rune(line)
		buf := uv.NewBuffer(len(runes), 1)
		for x, r := range runes {
			if r == ' ' {
				continue
			}
			c := kitLogoRestColor(x, theme.Primary, theme.Accent)
			if amt := kitLogoHighlight(x, y, frame); amt > 0 {
				c = blendColors(c, shine, amt)
			}
			buf.SetCell(x, 0, &uv.Cell{
				Content: string(r),
				Width:   1,
				Style:   uv.Style{Fg: c},
			})
		}
		out = append(out, buf.Render())
	}
	return out
}

// kitLogoHighlight returns how far the cell at (x, y) has been pulled toward
// the shine colour at the given frame, in [0,1].
//
// The two phases run in sequence rather than together: the shine crosses the
// whole mark, and only once it has left does the scanner bar start its bounce.
// Overlapping them put two moving highlights on screen at once, which read as
// competing with each other rather than as one gesture followed by another.
func kitLogoHighlight(x, y, frame int) float64 {
	switch {
	case frame < 0 || frame >= KitLogoAnimationFrames:
		return 0
	case frame < kitLogoSweepFrames:
		t := float64(frame) / float64(kitLogoSweepFrames)
		return logoShineIntensity(x, y, t)
	default:
		if y != kitLogoScannerRow {
			return 0
		}
		t := float64(frame-kitLogoSweepFrames) / float64(kitLogoScannerFrames)
		return logoScannerIntensity(x, t)
	}
}

// KitBanner returns the KIT ASCII art title with KITT scanner lights,
// rendered with a KITT red gradient.
//
// This is the banner used in `--help` output, where the art stands alone and
// is centered by its own leading padding. The TUI splash uses KitLogoLines
// instead, which is unpadded so it can sit inside a gutter-marked block.
func KitBanner() string {
	kittDark := lipgloss.Color("#8B0000")
	kittBright := lipgloss.Color("#FF2200")
	lines := []string{
		"             ██╗  ██╗ ██╗ ████████╗",
		"             ██║ ██╔╝ ██║ ╚══██╔══╝",
		"             █████╔╝  ██║    ██║",
		"             ██╔═██╗  ██║    ██║",
		"             ██║  ██╗ ██║    ██║",
		"             ╚═╝  ╚═╝ ╚═╝    ╚═╝",
		"░░ ░░ ░░ ▒▒ ▒▒ ▓▓ ▓▓ ████ ▓▓ ▓▓ ▒▒ ▒▒ ░░ ░░ ░░",
	}

	var result strings.Builder
	for i, line := range lines {
		if i > 0 {
			result.WriteString("\n")
		}
		result.WriteString(ApplyGradient(line, kittDark, kittBright))
	}
	return result.String()
}

// --------------------------------------------------------------------------
// Gutter
// --------------------------------------------------------------------------

// GutterGlyph is the single character used to mark a block's left edge
// throughout the UI: user messages, alerts, permission prompts and any other
// element that needs to be visually attributed.
//
// One glyph, colored by role, replaces what used to be three competing
// conventions (a thick ┃ for user messages, a light │ for alerts, and bare
// indentation for tool bodies). A half-block reads as a deliberate stripe at
// any font weight, where box-drawing characters vary between terminals and
// invite the eye to look for a matching corner that never comes.
const GutterGlyph = "▌"

// ContentOffset, and the width helpers derived from it, live in layout.go.

// GutterBorder returns a lipgloss border consisting only of a left edge drawn
// with GutterGlyph. Callers enable just the left side; the remaining fields
// exist because lipgloss requires a complete Border value.
func GutterBorder() lipgloss.Border {
	return lipgloss.Border{
		Left:        GutterGlyph,
		Right:       "",
		Top:         "",
		Bottom:      "",
		TopLeft:     "",
		TopRight:    "",
		BottomLeft:  "",
		BottomRight: "",
	}
}

// --------------------------------------------------------------------------
// Splash
// --------------------------------------------------------------------------

// SplashBlock renders the startup splash: content lines indented to the shared
// content column, with no left stripe.
//
//	KIT
//	anthropic · claude-opus-5
//
//	context   ~/project/AGENTS.md
//	skills    btca-cli, kit-extensions
//
// The splash carries no gutter glyph because it is not attributed to anyone —
// it is the application introducing itself, not a message from the user, the
// assistant or a tool. A stripe here would compete with the wordmark it sits
// beside and imply an authorship the block does not have.
//
// The block costs exactly as many rows as it has something to say, so a
// session with no skills or extensions loaded produces a shorter splash.
//
// Like every block in the transcript it carries its own trailing gap; the
// scroll list inserts nothing between items.
func SplashBlock(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	return Indent(strings.Join(lines, "\n"), ContentOffset) + strings.Repeat("\n", BlockGap)
}

// Indent prefixes every non-empty line of s with n spaces. Empty lines are
// left untouched so the indent never leaves trailing whitespace behind.
func Indent(s string, n int) string {
	pad := strings.Repeat(" ", n)
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		if line != "" {
			lines[i] = pad + line
		}
	}
	return strings.Join(lines, "\n")
}

// --------------------------------------------------------------------------
// Syntax highlighting
// --------------------------------------------------------------------------

// syntaxStyleCache holds the chroma style derived from the active theme.
// Building a chroma style allocates a map of token entries, so it must not
// happen inside a per-frame render path. Invalidated by SetTheme.
var syntaxStyleCache *chroma.Style

// hexOf renders a color as a chroma-compatible "#rrggbb" string.
func hexOf(c color.Color) string {
	r, g, b, _ := c.RGBA()
	return fmt.Sprintf("#%02x%02x%02x", uint8(r>>8), uint8(g>>8), uint8(b>>8))
}

// SyntaxStyle returns a chroma style built from the active theme's markdown
// colors.
//
// Syntax highlighting used to be pinned to an external palette, which meant a
// warm red/amber theme rendered code in blues, purples and greens — a second,
// unrelated color scheme living inside the first. Deriving the style from the
// theme keeps code blocks in the same family as everything around them, and
// makes every user theme (and every custom theme file) apply to code as well.
//
// Token backgrounds are left unset so the containing lipgloss style controls
// the fill; setting both produces visible seams at each token boundary.
func SyntaxStyle() *chroma.Style {
	if syntaxStyleCache != nil {
		return syntaxStyleCache
	}

	md := GetTheme().Markdown
	keyword := hexOf(md.Keyword)
	str := hexOf(md.String)
	number := hexOf(md.Number)
	comment := hexOf(md.Comment)
	text := hexOf(md.Text)
	name := hexOf(md.Link)

	builder := chroma.NewStyleBuilder("kit-theme")
	builder.Add(chroma.Text, text)
	builder.Add(chroma.Keyword, keyword+" bold")
	builder.Add(chroma.KeywordType, keyword)
	builder.Add(chroma.NameBuiltin, keyword)
	builder.Add(chroma.NameClass, name+" bold")
	builder.Add(chroma.NameFunction, name)
	builder.Add(chroma.LiteralString, str)
	builder.Add(chroma.LiteralNumber, number)
	builder.Add(chroma.Comment, comment+" italic")
	builder.Add(chroma.CommentPreproc, keyword)
	builder.Add(chroma.Operator, text)
	builder.Add(chroma.Punctuation, text)
	builder.Add(chroma.GenericError, hexOf(GetTheme().Error))

	built, err := builder.Build()
	if err != nil {
		// A malformed entry should degrade to no highlighting rather than
		// take down a render.
		built = styles.Fallback
	}
	syntaxStyleCache = built
	return built
}

package ui

import (
	"fmt"
	"image/color"
	"os"
	"strings"
	"sync"
	"time"

	"charm.land/lipgloss/v2"
)

// spinnerFrames defines available spinner animation styles.
var (
	dotFrames = []string{"⣾ ", "⣽ ", "⣻ ", "⢿ ", "⡿ ", "⣟ ", "⣯ ", "⣷ "}
	dotFPS    = time.Second / 10
)

// knightRiderFrames generates a KITT-style scanning animation where a bright
// red light bounces back and forth across a row of dots with a trailing glow.
func knightRiderFrames() []string {
	const numDots = 8
	const dot = "▪"

	bright := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000"))
	med := lipgloss.NewStyle().Foreground(lipgloss.Color("#990000"))
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("#440000"))
	off := lipgloss.NewStyle().Foreground(lipgloss.Color("#222222"))

	// Scanner bounces: 0→7→0
	positions := make([]int, 0, 2*numDots-2)
	for i := 0; i < numDots; i++ {
		positions = append(positions, i)
	}
	for i := numDots - 2; i > 0; i-- {
		positions = append(positions, i)
	}

	frames := make([]string, len(positions))
	for f, pos := range positions {
		var b strings.Builder
		for i := 0; i < numDots; i++ {
			d := pos - i
			if d < 0 {
				d = -d
			}
			switch {
			case d == 0:
				b.WriteString(bright.Render(dot))
			case d == 1:
				b.WriteString(med.Render(dot))
			case d == 2:
				b.WriteString(dim.Render(dot))
			default:
				b.WriteString(off.Render(dot))
			}
		}
		frames[f] = b.String()
	}
	return frames
}

// Spinner provides an animated loading indicator that displays while
// long-running operations are in progress. It writes directly to stderr
// using a goroutine-based animation loop, avoiding Bubble Tea's terminal
// capability queries that can leak escape sequences (mode 2026 DECRPM).
type Spinner struct {
	message string
	frames  []string
	fps     time.Duration
	color   color.Color // nil when frames are pre-rendered with embedded colors
	done    chan struct{}
	once    sync.Once
}

// NewSpinner creates a new animated spinner with the specified message.
// Uses a KITT-style red scanning animation.
func NewSpinner(message string) *Spinner {
	return &Spinner{
		message: message,
		frames:  knightRiderFrames(),
		fps:     time.Second / 14,
		color:   nil, // frames are pre-rendered
		done:    make(chan struct{}),
	}
}

// NewThemedSpinner creates a new animated spinner with custom color styling.
// This allows for different spinner colors based on the operation type or status.
func NewThemedSpinner(message string, c color.Color) *Spinner {
	return &Spinner{
		message: message,
		frames:  dotFrames,
		fps:     dotFPS,
		color:   c,
		done:    make(chan struct{}),
	}
}

// Start begins the spinner animation in a separate goroutine. The spinner
// will continue animating until Stop is called.
func (s *Spinner) Start() {
	go s.run()
}

// Stop halts the spinner animation and cleans up. This method blocks until
// the animation goroutine has exited and the line is cleared.
func (s *Spinner) Stop() {
	s.once.Do(func() { close(s.done) })
}

// run is the animation loop that renders spinner frames to stderr.
func (s *Spinner) run() {
	theme := GetTheme()

	messageStyle := lipgloss.NewStyle().
		Foreground(theme.Text).
		Italic(true)

	var spinnerStyle lipgloss.Style
	if s.color != nil {
		spinnerStyle = lipgloss.NewStyle().
			Foreground(s.color).
			Bold(true)
	}

	ticker := time.NewTicker(s.fps)
	defer ticker.Stop()

	var frame int
	for {
		select {
		case <-s.done:
			// Clear the spinner line and return.
			fmt.Fprint(os.Stderr, "\r\033[K")
			return
		case <-ticker.C:
			f := s.frames[frame%len(s.frames)]
			rendered := f
			if s.color != nil {
				rendered = spinnerStyle.Render(f)
			}
			fmt.Fprintf(os.Stderr, "\r %s %s",
				rendered,
				messageStyle.Render(s.message))
			frame++
		}
	}
}

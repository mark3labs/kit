package ui

import (
	"fmt"
	"image/color"
	"os"
	"sync"
	"time"

	"charm.land/lipgloss/v2"
)

// spinnerFrames defines available spinner animation styles.
var (
	pointsFrames = []string{"∙∙∙", "●∙∙", "∙●∙", "∙∙●"}
	pointsFPS    = time.Second / 7

	dotFrames = []string{"⣾ ", "⣽ ", "⣻ ", "⢿ ", "⡿ ", "⣟ ", "⣯ ", "⣷ "}
	dotFPS    = time.Second / 10
)

// Spinner provides an animated loading indicator that displays while
// long-running operations are in progress. It writes directly to stderr
// using a goroutine-based animation loop, avoiding Bubble Tea's terminal
// capability queries that can leak escape sequences (mode 2026 DECRPM).
type Spinner struct {
	message string
	frames  []string
	fps     time.Duration
	color   color.Color
	done    chan struct{}
	once    sync.Once
}

// NewSpinner creates a new animated spinner with the specified message.
// The spinner uses the theme's primary color and a points animation style.
func NewSpinner(message string) *Spinner {
	return &Spinner{
		message: message,
		frames:  pointsFrames,
		fps:     pointsFPS,
		color:   GetTheme().Primary,
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

	spinnerStyle := lipgloss.NewStyle().
		Foreground(s.color).
		Bold(true)

	messageStyle := lipgloss.NewStyle().
		Foreground(theme.Text).
		Italic(true)

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
			fmt.Fprintf(os.Stderr, "\r %s %s",
				spinnerStyle.Render(f),
				messageStyle.Render(s.message))
			frame++
		}
	}
}

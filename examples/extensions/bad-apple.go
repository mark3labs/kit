//go:build ignore

// Bad Apple!! — a video player as a Kit extension.
//
// This is a benchmark disguised as a demo. It answers the question "can an
// extension drive real per-frame animation through Kit's widget pipeline?" by
// playing 7,741 frames of 96x37 ASCII at 30fps inside a widget.
//
// Frames come from the Datastar Bad Apple example
// (https://data-star.dev/examples/bad_apple), captured from its SSE endpoint
// and repacked as a gzip file. The pack format is:
//
//	line 0     : "<cols> <rows> <fps> <nframes>"
//	lines 1..n : one frame per line, rows joined by \x1f
//
// Playback is wall-clock driven: the frame index is derived from elapsed time,
// not from a render counter, so the animation runs at the correct speed even
// if Kit renders slower than 30fps (it drops frames instead of slowing down).
//
// NOTE ON YAEGI: every helper is declared ABOVE its first use. A bare
// reference to a function declared later in the file, from inside a closure,
// silently returns zero values.
package main

import (
	"compress/gzip"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	ext "kit/ext"
)

// ---------------------------------------------------------------------------
// State
// ---------------------------------------------------------------------------

var (
	mu sync.RWMutex

	frames  []string // each frame: rows joined by \x1f
	cols    int
	rows    int
	fps     int
	loaded  bool
	loadErr string

	playing   bool
	startedAt time.Time
	pausedAt  int // frame index playback was paused on

	showChrome = true

	// Instrumentation: count actual Render invocations to report the real
	// achieved repaint rate, as distinct from the wall-clock frame index.
	renderCount int
	rateWindow  time.Time
	rateCount   int
	actualFPS   float64
)

// ---------------------------------------------------------------------------
// Helpers (declared before use — see the Yaegi note above)
// ---------------------------------------------------------------------------

func esc(code string) string { return "\033[" + code + "m" }

func dim(s string) string { return esc("2") + s + esc("0") }

func packPath() string {
	if p := os.Getenv("BADAPPLE_PACK"); p != "" {
		return p
	}
	base := os.Getenv("XDG_DATA_HOME")
	if base == "" {
		base = filepath.Join(os.Getenv("HOME"), ".local", "share")
	}
	return filepath.Join(base, "kit", "badapple", "badapple.pack.gz")
}

// loadPack reads and decompresses the frame pack.
//
// The heavy lifting (gzip inflate, one big Split) is done by compiled stdlib
// calls rather than interpreted loops, which is what keeps this tolerable
// under Yaegi — a per-frame loop over 7,741 entries would not be.
func loadPack() {
	path := packPath()

	fh, err := os.Open(path)
	if err != nil {
		loadErr = fmt.Sprintf("cannot open %s: %v", path, err)
		return
	}
	defer fh.Close()

	zr, err := gzip.NewReader(fh)
	if err != nil {
		loadErr = fmt.Sprintf("gzip: %v", err)
		return
	}
	defer zr.Close()

	var sb strings.Builder
	buf := make([]byte, 1<<20)
	for {
		n, rerr := zr.Read(buf)
		if n > 0 {
			sb.Write(buf[:n])
		}
		if rerr != nil {
			break
		}
	}

	lines := strings.Split(sb.String(), "\n")
	if len(lines) < 2 {
		loadErr = "pack is empty"
		return
	}

	head := strings.Fields(lines[0])
	if len(head) < 4 {
		loadErr = "bad pack header: " + lines[0]
		return
	}
	cols, _ = strconv.Atoi(head[0])
	rows, _ = strconv.Atoi(head[1])
	fps, _ = strconv.Atoi(head[2])
	if fps <= 0 {
		fps = 30
	}

	frames = lines[1:]
	loaded = len(frames) > 0
	if !loaded {
		loadErr = "no frames in pack"
	}
}

// currentIndex returns the frame to show right now, looping at the end.
func currentIndex() int {
	if !playing {
		return pausedAt
	}
	elapsed := time.Since(startedAt)
	idx := int(elapsed / (time.Second / time.Duration(fps)))
	if len(frames) == 0 {
		return 0
	}
	return idx % len(frames)
}

// fitFrame crops or pads a frame's rows to the available width, keeping the
// image centred. Kit gives the widget an exact content column; drawing wider
// than that would wrap and tear the picture.
func fitFrame(frame string, width int) string {
	lines := strings.Split(frame, "\x1f")

	if width >= cols {
		pad := strings.Repeat(" ", (width-cols)/2)
		for i, ln := range lines {
			lines[i] = pad + ln
		}
		return strings.Join(lines, "\n")
	}

	// Narrower than the source: take a centred slice.
	off := (cols - width) / 2
	for i, ln := range lines {
		if len(ln) > off+width {
			lines[i] = ln[off : off+width]
		} else if len(ln) > off {
			lines[i] = ln[off:]
		} else {
			lines[i] = ""
		}
	}
	return strings.Join(lines, "\n")
}

// renderFrame is the widget body: it returns the current frame, already sized
// to the column Kit handed us. Kit uses the string verbatim.
func renderFrame(width int) string {
	mu.Lock()
	defer mu.Unlock()

	renderCount++
	rateCount++
	if rateWindow.IsZero() {
		rateWindow = time.Now()
	} else if d := time.Since(rateWindow); d >= time.Second {
		actualFPS = float64(rateCount) / d.Seconds()
		rateCount = 0
		rateWindow = time.Now()
	}

	if !loaded {
		if loadErr != "" {
			return dim("bad apple: " + loadErr)
		}
		return dim("bad apple: loading…")
	}
	if width < 8 {
		return ""
	}

	idx := currentIndex()
	if idx < 0 || idx >= len(frames) {
		idx = 0
	}

	body := fitFrame(frames[idx], width)
	if !showChrome {
		return body
	}

	pct := 0.0
	if len(frames) > 0 {
		pct = float64(idx) / float64(len(frames)) * 100
	}
	state := "playing"
	if !playing {
		state = "paused"
	}

	bar := int(pct / 100 * float64(width-1))
	if bar < 0 {
		bar = 0
	}
	if bar > width-1 {
		bar = width - 1
	}
	track := strings.Repeat("━", bar) + "╸" + strings.Repeat("─", width-1-bar)

	status := fmt.Sprintf("bad apple!!  %s  frame %d/%d  %.1f%%  %dx%d  target %dfps  actual %.1ffps  renders %d",
		state, idx, len(frames), pct, cols, rows, fps, actualFPS, renderCount)

	return body + "\n" + dim(track) + "\n" + dim(status)
}

// ---------------------------------------------------------------------------
// Init
// ---------------------------------------------------------------------------

func Init(api ext.API) {
	// Load off the Init path so a big pack cannot stall startup.
	go func() {
		mu.Lock()
		loadPack()
		if loaded {
			playing = true
			startedAt = time.Now()
		}
		mu.Unlock()
	}()

	api.RegisterCommand(ext.CommandDef{
		Name:        "ba",
		Description: "Bad Apple player: play | pause | restart | seek <pct> | chrome | stop",
		Execute: func(args string, ctx ext.Context) (string, error) {
			cmd := strings.TrimSpace(args)
			field := strings.Fields(cmd)
			verb := ""
			if len(field) > 0 {
				verb = field[0]
			}

			mu.Lock()
			defer mu.Unlock()

			switch verb {
			case "", "play":
				if !loaded {
					ctx.PrintInfo("bad apple: not loaded — " + loadErr)
					return "", nil
				}
				if !playing {
					playing = true
					startedAt = time.Now().Add(-time.Duration(pausedAt) * (time.Second / time.Duration(fps)))
				}
				ctx.PrintInfo("playing")

			case "pause":
				if playing {
					pausedAt = currentIndex()
					playing = false
				}
				ctx.PrintInfo(fmt.Sprintf("paused at frame %d", pausedAt))

			case "restart":
				pausedAt = 0
				playing = true
				startedAt = time.Now()
				ctx.PrintInfo("restarted")

			case "seek":
				if len(field) < 2 {
					ctx.PrintInfo("usage: /ba seek <percent>")
					return "", nil
				}
				pct, err := strconv.ParseFloat(field[1], 64)
				if err != nil {
					ctx.PrintInfo("bad percentage: " + field[1])
					return "", nil
				}
				idx := int(pct / 100 * float64(len(frames)))
				pausedAt = idx
				startedAt = time.Now().Add(-time.Duration(idx) * (time.Second / time.Duration(fps)))
				ctx.PrintInfo(fmt.Sprintf("seek to %.1f%% (frame %d)", pct, idx))

			case "chrome":
				showChrome = !showChrome
				ctx.PrintInfo(fmt.Sprintf("chrome: %v", showChrome))

			case "stop":
				playing = false
				ctx.RemoveWidget("badapple:screen")
				ctx.PrintInfo("stopped — /ba play to bring it back")

			default:
				ctx.PrintInfo("usage: /ba play | pause | restart | seek <pct> | chrome | stop")
			}
			return "", nil
		},
	})

	api.OnSessionStart(func(e ext.SessionStartEvent, ctx ext.Context) {
		ctx.SetWidget(ext.WidgetConfig{
			ID:        "badapple:screen",
			Placement: ext.WidgetAbove,
			Priority:  5,
			Style:     ext.WidgetStyle{NoBorder: true},
			Content: ext.WidgetContent{
				// The whole point: hold the shared clock open at Kit's
				// ceiling so the widget repaints at video rate.
				RefreshHz: 30,
				Render:    func(width int) string { return renderFrame(width) },
			},
		})
	})
}

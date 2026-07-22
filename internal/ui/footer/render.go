package footer

import (
	"fmt"
	"math"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/mark3labs/kit/internal/ui/style"
)

// RenderData encapsulates state for rendering the modular footer.
type RenderData struct {
	Width         int
	ModeTag       string
	ModelName     string
	ThinkingLevel string
	ContextTokens int
	ContextLimit  int
	CacheTokens   int
	HitRatio      float64
	SessionCost   float64
	TxCost        float64
	Savings       float64
	IsOAuth       bool
	TurnDuration  time.Duration
	TotalDuration time.Duration
	SpinnerPrefix string
	StatusEntries []string
	Time          time.Time
}

// Render draws the status bar formatted according to Config and RenderData.
func Render(cfg Config, data RenderData) string {
	if !cfg.initialized {
		cfg = DefaultConfig()
	}
	if !cfg.Enabled {
		return data.SpinnerPrefix
	}

	theme := style.GetTheme()

	// 1. Mode tag
	var elMode string
	if cfg.IsFieldEnabled(FieldMode) {
		modeTag := data.ModeTag
		if modeTag == "" {
			modeTag = "[KIT]"
		}
		elMode = lipgloss.NewStyle().
			Foreground(theme.Secondary).
			Bold(true).
			Render(modeTag)
	}

	// 2. Model short name
	var elModel string
	if cfg.IsFieldEnabled(FieldModel) {
		modelShort := FormatShortModelName(data.ModelName)
		if modelShort == "" {
			modelShort = "kit"
		}
		if data.ThinkingLevel != "" && data.ThinkingLevel != "off" {
			modelShort += fmt.Sprintf(" (effort: %s)", data.ThinkingLevel)
		}
		elModel = lipgloss.NewStyle().
			Foreground(theme.Text).
			Render(modelShort)
	}

	// 3. Context
	var elCtx string
	if cfg.IsFieldEnabled(FieldContext) {
		elCtx = lipgloss.NewStyle().
			Foreground(theme.Muted).
			Render("ctx " + FormatTokenCount(data.ContextTokens))
	}

	// 4. Bar
	ctxLimit := data.ContextLimit
	if ctxLimit <= 0 {
		ctxLimit = 200000
	}
	var pct float64
	if ctxLimit > 0 {
		pct = float64(data.ContextTokens) / float64(ctxLimit) * 100.0
	}

	barColor := theme.Success
	if pct >= 80 {
		barColor = theme.Error
	} else if pct >= 60 {
		barColor = theme.Warning
	}

	var elBar string
	if cfg.IsFieldEnabled(FieldBar) {
		filledBlocks := int(math.Round((pct / 100.0) * 10.0))
		if filledBlocks < 0 {
			filledBlocks = 0
		}
		if filledBlocks > 10 {
			filledBlocks = 10
		}
		barStr := "[" + strings.Repeat("█", filledBlocks) + strings.Repeat("░", 10-filledBlocks) + "]"
		elBar = lipgloss.NewStyle().
			Foreground(barColor).
			Render(fmt.Sprintf("%s %.0f%%", barStr, pct))
	}

	// 5. Cache stats
	var elCache string
	if cfg.IsFieldEnabled(FieldCache) {
		elCache = lipgloss.NewStyle().
			Foreground(theme.Muted).
			Render(fmt.Sprintf("🗃 %s (%.0f%%)", FormatTokenCount(data.CacheTokens), data.HitRatio))
	}

	// 6. Cost breakdown
	var elCostFull, elCostCompact string
	if cfg.IsFieldEnabled(FieldCost) {
		var costMain, costSub string
		if data.IsOAuth {
			costMain = "💰 $0.00"
			costSub = fmt.Sprintf(" (🪙 $0.00000 · 🦕 ~%.0f%%)", data.Savings)
		} else {
			costMain = fmt.Sprintf("💰 $%.5f", data.SessionCost)
			if data.TxCost > 0 || data.SessionCost > 0 {
				costSub = fmt.Sprintf(" (🪙 $%.5f · 🦕 ~%.0f%%)", data.TxCost, data.Savings)
			}
		}
		elCostFull = lipgloss.NewStyle().Foreground(theme.Primary).Bold(true).Render(costMain) +
			lipgloss.NewStyle().Foreground(theme.Muted).Render(costSub)
		elCostCompact = lipgloss.NewStyle().Foreground(theme.Primary).Bold(true).Render(costMain)
	}

	// 7. Clock
	var elTime string
	if cfg.IsFieldEnabled(FieldClock) {
		now := data.Time
		if now.IsZero() {
			now = time.Now()
		}
		timeStr := strings.ToLower(now.Format("3:04pm"))
		elTime = lipgloss.NewStyle().
			Foreground(theme.VeryMuted).
			Render("[" + timeStr + "]")
	}

	// 8. Timer
	var elTimer string
	if cfg.IsFieldEnabled(FieldTimer) {
		elTimer = lipgloss.NewStyle().
			Foreground(theme.Secondary).
			Render(fmt.Sprintf("⏱ %s / %s", FormatDuration(data.TurnDuration), FormatDuration(data.TotalDuration)))
	}

	sep := lipgloss.NewStyle().Foreground(theme.VeryMuted).Render(" │ ")

	// Build parts list in active field order
	var parts []string
	if elMode != "" {
		parts = append(parts, elMode)
	}
	if elModel != "" {
		parts = append(parts, elModel)
	}
	for _, entry := range data.StatusEntries {
		if entry = strings.TrimSpace(entry); entry != "" {
			parts = append(parts, lipgloss.NewStyle().Foreground(theme.Muted).Render(entry))
		}
	}
	if elCtx != "" {
		parts = append(parts, elCtx)
	}
	if elBar != "" {
		parts = append(parts, elBar)
	}
	if elCache != "" {
		parts = append(parts, elCache)
	}
	if elCostFull != "" {
		parts = append(parts, elCostFull)
	}

	if elTime != "" && elTimer != "" {
		parts = append(parts, elTime+" "+elTimer)
	} else if elTime != "" {
		parts = append(parts, elTime)
	} else if elTimer != "" {
		parts = append(parts, elTimer)
	}

	if len(parts) == 0 {
		return ""
	}

	fullLine := data.SpinnerPrefix + strings.Join(parts, sep)
	if lipgloss.Width(fullLine) <= data.Width || data.Width <= 0 {
		return fullLine
	}

	// Cascade 1: Drop clock, keep timer
	if elTime != "" && elTimer != "" {
		for i, p := range parts {
			if p == elTime+" "+elTimer {
				parts[i] = elTimer
				break
			}
		}
		line1 := data.SpinnerPrefix + strings.Join(parts, sep)
		if lipgloss.Width(line1) <= data.Width {
			return line1
		}
	}

	// Cascade 2: Drop cache
	if elCache != "" {
		var newParts []string
		for _, p := range parts {
			if p != elCache {
				newParts = append(newParts, p)
			}
		}
		parts = newParts
		line2 := data.SpinnerPrefix + strings.Join(parts, sep)
		if lipgloss.Width(line2) <= data.Width {
			return line2
		}
	}

	// Cascade 3: Drop ctx
	if elCtx != "" {
		var newParts []string
		for _, p := range parts {
			if p != elCtx {
				newParts = append(newParts, p)
			}
		}
		parts = newParts
		line3 := data.SpinnerPrefix + strings.Join(parts, sep)
		if lipgloss.Width(line3) <= data.Width {
			return line3
		}
	}

	// Cascade 4: Compact cost
	if elCostFull != "" {
		for i, p := range parts {
			if p == elCostFull {
				parts[i] = elCostCompact
				break
			}
		}
		line4 := data.SpinnerPrefix + strings.Join(parts, sep)
		if lipgloss.Width(line4) <= data.Width {
			return line4
		}
	}

	// Cascade 5: Bar pct only
	if elBar != "" {
		elPctOnly := lipgloss.NewStyle().Foreground(barColor).Render(fmt.Sprintf("%.0f%%", pct))
		for i, p := range parts {
			if p == elBar {
				parts[i] = elPctOnly
				break
			}
		}
		line5 := data.SpinnerPrefix + strings.Join(parts, sep)
		if lipgloss.Width(line5) <= data.Width {
			return line5
		}
	}

	line := data.SpinnerPrefix + strings.Join(parts, sep)
	if data.Width <= 0 || lipgloss.Width(line) <= data.Width {
		return line
	}
	return lipgloss.NewStyle().MaxWidth(data.Width).Render(line)
}

// FormatShortModelName formats raw model string into human readable short representation.
func FormatShortModelName(modelName string) string {
	if modelName == "" {
		return ""
	}
	name := modelName
	if idx := strings.LastIndex(name, "/"); idx != -1 {
		name = name[idx+1:]
	}
	lower := strings.ToLower(name)

	if strings.Contains(lower, "sonnet") {
		if strings.Contains(lower, "4-5") || strings.Contains(lower, "4.5") {
			return "sonnet 4.5"
		}
		if strings.Contains(lower, "3-7") || strings.Contains(lower, "3.7") {
			return "sonnet 3.7"
		}
		if strings.Contains(lower, "3-5") || strings.Contains(lower, "3.5") {
			return "sonnet 3.5"
		}
		if strings.Contains(lower, "3") {
			return "sonnet 3"
		}
		return "sonnet"
	}
	if strings.Contains(lower, "haiku") {
		if strings.Contains(lower, "3-5") || strings.Contains(lower, "3.5") {
			return "haiku 3.5"
		}
		return "haiku"
	}
	if strings.Contains(lower, "opus") {
		if strings.Contains(lower, "3-5") || strings.Contains(lower, "3.5") {
			return "opus 3.5"
		}
		if strings.Contains(lower, "3") {
			return "opus 3"
		}
		return "opus"
	}

	res := strings.ReplaceAll(name, "-", " ")
	res = strings.ReplaceAll(res, "_", " ")
	return strings.TrimSpace(res)
}

// FormatTokenCount formats token counts as exact, k or M notation.
func FormatTokenCount(tokens int) string {
	if tokens >= 1000000 {
		return fmt.Sprintf("%.1fM", float64(tokens)/1000000.0)
	} else if tokens >= 1000 {
		return fmt.Sprintf("%.1fk", float64(tokens)/1000.0)
	}
	return fmt.Sprintf("%d", tokens)
}

// FormatDuration formats duration nicely in s or m s.
func FormatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	sec := int(d.Seconds())
	if sec < 0 {
		sec = 0
	}
	if sec < 60 {
		return fmt.Sprintf("%ds", sec)
	}
	m := sec / 60
	s := sec % 60
	return fmt.Sprintf("%dm%ds", m, s)
}

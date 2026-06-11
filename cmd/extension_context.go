package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/viper"
	"golang.org/x/term"

	"github.com/mark3labs/kit/internal/app"
	"github.com/mark3labs/kit/internal/extbridge"
	"github.com/mark3labs/kit/internal/extensions"
	"github.com/mark3labs/kit/internal/models"
	"github.com/mark3labs/kit/internal/ui"
	kit "github.com/mark3labs/kit/pkg/kit"
)

// extensionContextDeps groups the runtime dependencies needed to wire up
// an extensions.Context for the interactive TUI mode.
type extensionContextDeps struct {
	ctx          context.Context
	cwd          string
	modelName    string
	interactive  bool
	kitInstance  *kit.Kit
	appInstance  *app.App
	usageTracker *ui.UsageTracker
}

// buildInteractiveExtensionContext returns an extensions.Context with every
// field except Print / PrintInfo / PrintError populated. Callers must set
// the three print routes appropriately for their phase (startup buffering
// vs. live runtime routing).
//
// The headless half (data access, state, options, tree navigation, skills,
// templates, model resolution, subagents) comes from extbridge.BaseContext;
// this function overlays the TUI-specific fields and overrides SetModel /
// ReloadExtensions with TUI-aware versions.
func buildInteractiveExtensionContext(deps extensionContextDeps) extensions.Context {
	kitInstance := deps.kitInstance
	appInstance := deps.appInstance
	usageTracker := deps.usageTracker

	ec := extbridge.BaseContext(deps.ctx, kitInstance)

	ec.CWD = deps.cwd
	ec.Model = deps.modelName
	ec.Interactive = deps.interactive

	ec.PrintBlock = func(opts extensions.PrintBlockOpts) {
		appInstance.PrintBlockFromExtension(opts)
	}
	ec.SendMessage = func(text string) { appInstance.Run(text) }
	ec.CancelAndSend = func(text string) { appInstance.InterruptAndSend(text) }
	ec.Abort = func() { appInstance.Abort() }
	ec.IsIdle = func() bool { return !appInstance.IsBusy() }
	ec.Compact = func(cfg extensions.CompactConfig) error {
		return appInstance.CompactAsync(cfg.CustomInstructions, cfg.OnComplete, cfg.OnError)
	}
	ec.SendMultimodalMessage = func(text string, files []extensions.FilePart) {
		parts := make([]kit.LLMFilePart, len(files))
		for i, f := range files {
			parts[i] = kit.LLMFilePart{
				Filename:  f.Filename,
				Data:      f.Data,
				MediaType: f.MediaType,
			}
		}
		appInstance.RunWithFiles(text, parts)
	}
	ec.GetSessionUsage = func() extensions.SessionUsage {
		if usageTracker == nil {
			return extensions.SessionUsage{}
		}
		stats := usageTracker.GetSessionStats()
		return extensions.SessionUsage{
			TotalInputTokens:      stats.TotalInputTokens,
			TotalOutputTokens:     stats.TotalOutputTokens,
			TotalCacheReadTokens:  stats.TotalCacheReadTokens,
			TotalCacheWriteTokens: stats.TotalCacheWriteTokens,
			TotalCost:             stats.TotalCost,
			RequestCount:          stats.RequestCount,
		}
	}
	ec.Exit = func() { appInstance.QuitFromExtension() }

	// TUI widgets/chrome — mutate runner state, then notify the TUI.
	// Always use a goroutine for NotifyWidgetUpdate: prog.Send() deadlocks
	// if called synchronously from inside BubbleTea's Update() handler.
	// All call sites use go-routines uniformly.
	ec.SetWidget = func(config extensions.WidgetConfig) {
		kitInstance.Extensions().SetWidget(config)
		go appInstance.NotifyWidgetUpdate()
	}
	ec.RemoveWidget = func(id string) {
		kitInstance.Extensions().RemoveWidget(id)
		go appInstance.NotifyWidgetUpdate()
	}
	ec.SetHeader = func(config extensions.HeaderFooterConfig) {
		kitInstance.Extensions().SetHeader(config)
		go appInstance.NotifyWidgetUpdate()
	}
	ec.RemoveHeader = func() {
		kitInstance.Extensions().RemoveHeader()
		go appInstance.NotifyWidgetUpdate()
	}
	ec.SetFooter = func(config extensions.HeaderFooterConfig) {
		kitInstance.Extensions().SetFooter(config)
		go appInstance.NotifyWidgetUpdate()
	}
	ec.RemoveFooter = func() {
		kitInstance.Extensions().RemoveFooter()
		go appInstance.NotifyWidgetUpdate()
	}
	ec.SetUIVisibility = func(v extensions.UIVisibility) {
		kitInstance.Extensions().SetUIVisibility(v)
		go appInstance.NotifyWidgetUpdate()
	}
	ec.SetEditor = func(config extensions.EditorConfig) {
		kitInstance.Extensions().SetEditor(config)
		go appInstance.NotifyWidgetUpdate()
	}
	ec.ResetEditor = func() {
		kitInstance.Extensions().ResetEditor()
		go appInstance.NotifyWidgetUpdate()
	}
	ec.SetEditorText = func(text string) {
		appInstance.SetEditorTextFromExtension(text)
	}
	ec.SetStatus = func(key string, text string, priority int) {
		kitInstance.Extensions().SetStatus(extensions.StatusBarEntry{
			Key:      key,
			Text:     text,
			Priority: priority,
		})
		go appInstance.NotifyWidgetUpdate()
	}
	ec.RemoveStatus = func(key string) {
		kitInstance.Extensions().RemoveStatus(key)
		go appInstance.NotifyWidgetUpdate()
	}

	// Interactive prompts — channel-based round trips through the TUI.
	ec.PromptSelect = func(config extensions.PromptSelectConfig) extensions.PromptSelectResult {
		ch := make(chan app.PromptResponse, 1)
		appInstance.SendPromptRequest(app.PromptRequestEvent{
			PromptType: "select",
			Message:    config.Message,
			Options:    config.Options,
			ResponseCh: ch,
		})
		resp := <-ch
		if resp.Cancelled {
			return extensions.PromptSelectResult{Cancelled: true}
		}
		return extensions.PromptSelectResult{Value: resp.Value, Index: resp.Index}
	}
	ec.PromptConfirm = func(config extensions.PromptConfirmConfig) extensions.PromptConfirmResult {
		ch := make(chan app.PromptResponse, 1)
		def := "false"
		if config.DefaultValue {
			def = "true"
		}
		appInstance.SendPromptRequest(app.PromptRequestEvent{
			PromptType: "confirm",
			Message:    config.Message,
			Default:    def,
			ResponseCh: ch,
		})
		resp := <-ch
		if resp.Cancelled {
			return extensions.PromptConfirmResult{Cancelled: true}
		}
		return extensions.PromptConfirmResult{Value: resp.Confirmed}
	}
	ec.PromptInput = func(config extensions.PromptInputConfig) extensions.PromptInputResult {
		ch := make(chan app.PromptResponse, 1)
		appInstance.SendPromptRequest(app.PromptRequestEvent{
			PromptType:  "input",
			Message:     config.Message,
			Placeholder: config.Placeholder,
			Default:     config.Default,
			ResponseCh:  ch,
		})
		resp := <-ch
		if resp.Cancelled {
			return extensions.PromptInputResult{Cancelled: true}
		}
		return extensions.PromptInputResult{Value: resp.Value}
	}
	ec.ShowOverlay = func(config extensions.OverlayConfig) extensions.OverlayResult {
		ch := make(chan app.OverlayResponse, 1)
		appInstance.SendOverlayRequest(app.OverlayRequestEvent{
			Title:       config.Title,
			Content:     config.Content.Text,
			Markdown:    config.Content.Markdown,
			BorderColor: config.Style.BorderColor,
			Background:  config.Style.Background,
			Width:       config.Width,
			MaxHeight:   config.MaxHeight,
			Anchor:      string(config.Anchor),
			Actions:     config.Actions,
			ResponseCh:  ch,
		})
		resp := <-ch
		if resp.Cancelled {
			return extensions.OverlayResult{Cancelled: true, Index: -1}
		}
		return extensions.OverlayResult{
			Action: resp.Action,
			Index:  resp.Index,
		}
	}
	ec.SuspendTUI = func(callback func()) error {
		return appInstance.SuspendTUI(callback)
	}

	// TUI-aware model switch: also notifies the TUI status bar and
	// refreshes the usage tracker for correct token counting.
	ec.SetModel = func(modelString string) error {
		// Capture previous model for the ModelChange event.
		previousModel := kitInstance.Extensions().GetContext().Model
		err := kitInstance.SetModel(context.Background(), modelString)
		if err != nil {
			return err
		}
		// Notify TUI so it updates model in status bar.
		p, m, _ := models.ParseModelString(modelString)
		appInstance.NotifyModelChanged(p, m)
		// Update the context's Model field so handlers see it.
		kitInstance.Extensions().UpdateContextModel(modelString)
		// Fire OnModelChange event to extensions.
		kitInstance.Extensions().EmitModelChange(modelString, previousModel, "extension")
		// Update usage tracker with new model info for correct token counting.
		ui.UpdateUsageTrackerForModel(usageTracker, modelString, viper.GetString("provider-api-key"))
		return nil
	}

	ec.RenderMessage = func(rendererName, content string) {
		renderer := kitInstance.Extensions().GetMessageRenderer(rendererName)
		if renderer == nil || renderer.Render == nil {
			appInstance.PrintFromExtension("", content)
			return
		}
		w, _, _ := term.GetSize(int(os.Stdout.Fd()))
		if w == 0 {
			w = 80
		}
		rendered := renderer.Render(content, w)
		appInstance.PrintFromExtension("", rendered)
	}
	ec.ReloadExtensions = func() error {
		err := kitInstance.Extensions().Reload()
		if err != nil {
			return err
		}
		// Notify TUI that widgets/status/commands may have changed.
		go appInstance.NotifyWidgetUpdate()
		return nil
	}

	// Theme management (TUI only).
	ec.RegisterTheme = func(name string, config extensions.ThemeColorConfig) {
		tc := func(c extensions.ThemeColor) [2]string { return [2]string{c.Light, c.Dark} }
		ui.RegisterThemeFromConfig(name,
			tc(config.Primary), tc(config.Secondary),
			tc(config.Success), tc(config.Warning),
			tc(config.Error), tc(config.Info),
			tc(config.Text), tc(config.Muted),
			tc(config.VeryMuted), tc(config.Background),
			tc(config.Border), tc(config.MutedBorder),
			tc(config.System), tc(config.Tool),
			tc(config.Accent), tc(config.Highlight),
			tc(config.MdHeading), tc(config.MdLink),
			tc(config.MdKeyword), tc(config.MdString),
			tc(config.MdNumber), tc(config.MdComment),
		)
	}
	ec.SetTheme = func(name string) error {
		return ui.ApplyTheme(name)
	}
	ec.ListThemes = func() []string {
		return ui.ListThemes()
	}

	// Skill context-injection (drives a new agent turn through the TUI).
	ec.InjectSkillAsContext = func(skillName string) string {
		skills := kitInstance.DiscoverSkillsForExtension()
		for _, s := range skills {
			if s.Name == skillName {
				appInstance.Run(fmt.Sprintf("<skill name=%q>\n%s\n</skill>", s.Name, s.Content))
				return ""
			}
		}
		return fmt.Sprintf("skill not found: %s", skillName)
	}
	ec.InjectRawSkillAsContext = func(path string) string {
		s, err := kitInstance.LoadSkillForExtension(path)
		if err != "" {
			return err
		}
		appInstance.Run(fmt.Sprintf("<skill name=%q>\n%s\n</skill>", s.Name, s.Content))
		return ""
	}

	return ec
}

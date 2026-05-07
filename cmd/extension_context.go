package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"
	"golang.org/x/term"

	"github.com/mark3labs/kit/internal/app"
	"github.com/mark3labs/kit/internal/auth"
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
// This consolidates two near-identical 400-line literal expressions that
// previously appeared inline in runNormalMode.
func buildInteractiveExtensionContext(deps extensionContextDeps) extensions.Context {
	kitInstance := deps.kitInstance
	appInstance := deps.appInstance
	usageTracker := deps.usageTracker
	ctx := deps.ctx

	return extensions.Context{
		CWD:         deps.cwd,
		Model:       deps.modelName,
		Interactive: deps.interactive,
		PrintBlock: func(opts extensions.PrintBlockOpts) {
			appInstance.PrintBlockFromExtension(opts)
		},
		SendMessage:   func(text string) { appInstance.Run(text) },
		CancelAndSend: func(text string) { appInstance.InterruptAndSend(text) },
		Abort:         func() { appInstance.Abort() },
		IsIdle:        func() bool { return !appInstance.IsBusy() },
		Compact: func(cfg extensions.CompactConfig) error {
			return appInstance.CompactAsync(cfg.CustomInstructions, cfg.OnComplete, cfg.OnError)
		},
		SendMultimodalMessage: func(text string, files []extensions.FilePart) {
			parts := make([]kit.LLMFilePart, len(files))
			for i, f := range files {
				parts[i] = kit.LLMFilePart{
					Filename:  f.Filename,
					Data:      f.Data,
					MediaType: f.MediaType,
				}
			}
			appInstance.RunWithFiles(text, parts)
		},
		GetSessionUsage: func() extensions.SessionUsage {
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
		},
		Exit: func() { appInstance.QuitFromExtension() },
		SetWidget: func(config extensions.WidgetConfig) {
			kitInstance.Extensions().SetWidget(config)
			go appInstance.NotifyWidgetUpdate()
		},
		RemoveWidget: func(id string) {
			kitInstance.Extensions().RemoveWidget(id)
			go appInstance.NotifyWidgetUpdate()
		},
		SetHeader: func(config extensions.HeaderFooterConfig) {
			kitInstance.Extensions().SetHeader(config)
			go appInstance.NotifyWidgetUpdate()
		},
		RemoveHeader: func() {
			kitInstance.Extensions().RemoveHeader()
			go appInstance.NotifyWidgetUpdate()
		},
		SetFooter: func(config extensions.HeaderFooterConfig) {
			kitInstance.Extensions().SetFooter(config)
			go appInstance.NotifyWidgetUpdate()
		},
		RemoveFooter: func() {
			kitInstance.Extensions().RemoveFooter()
			go appInstance.NotifyWidgetUpdate()
		},
		PromptSelect: func(config extensions.PromptSelectConfig) extensions.PromptSelectResult {
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
		},
		PromptConfirm: func(config extensions.PromptConfirmConfig) extensions.PromptConfirmResult {
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
		},
		PromptInput: func(config extensions.PromptInputConfig) extensions.PromptInputResult {
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
		},
		SetUIVisibility: func(v extensions.UIVisibility) {
			kitInstance.Extensions().SetUIVisibility(v)
			go appInstance.NotifyWidgetUpdate()
		},
		GetContextStats: func() extensions.ContextStats {
			s := kitInstance.GetContextStats()
			return extensions.ContextStats{
				EstimatedTokens: s.EstimatedTokens,
				ContextLimit:    s.ContextLimit,
				UsagePercent:    s.UsagePercent,
				MessageCount:    s.MessageCount,
			}
		},
		SetEditor: func(config extensions.EditorConfig) {
			kitInstance.Extensions().SetEditor(config)
			// Always use a goroutine for NotifyWidgetUpdate: prog.Send()
			// deadlocks if called synchronously from inside BubbleTea's
			// Update() handler. All call sites use go-routines uniformly.
			go appInstance.NotifyWidgetUpdate()
		},
		ResetEditor: func() {
			kitInstance.Extensions().ResetEditor()
			go appInstance.NotifyWidgetUpdate()
		},
		GetMessages: func() []extensions.SessionMessage {
			return kitInstance.Extensions().GetSessionMessages()
		},
		GetSessionPath: func() string {
			return kitInstance.GetSessionPath()
		},
		AppendEntry: func(entryType string, data string) (string, error) {
			return kitInstance.Extensions().AppendEntry(entryType, data)
		},
		GetEntries: func(entryType string) []extensions.ExtensionEntry {
			return kitInstance.Extensions().GetEntries(entryType)
		},
		SetEditorText: func(text string) {
			appInstance.SetEditorTextFromExtension(text)
		},
		SetStatus: func(key string, text string, priority int) {
			kitInstance.Extensions().SetStatus(extensions.StatusBarEntry{
				Key:      key,
				Text:     text,
				Priority: priority,
			})
			go appInstance.NotifyWidgetUpdate()
		},
		RemoveStatus: func(key string) {
			kitInstance.Extensions().RemoveStatus(key)
			go appInstance.NotifyWidgetUpdate()
		},
		GetOption: func(name string) string {
			return kitInstance.Extensions().GetOption(name)
		},
		SetOption: func(name string, value string) {
			kitInstance.Extensions().SetOption(name, value)
		},
		SetModel: func(modelString string) error {
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
			if usageTracker != nil {
				newProvider, newModel, _ := models.ParseModelString(modelString)
				if newProvider != "unknown" && newModel != "unknown" && newProvider != "ollama" {
					registry := models.GetGlobalRegistry()
					if modelInfo := registry.LookupModel(newProvider, newModel); modelInfo != nil {
						// Check OAuth status for Anthropic models
						isOAuth := false
						if newProvider == "anthropic" {
							_, source, err := auth.GetAnthropicAPIKey(viper.GetString("provider-api-key"))
							if err == nil && strings.HasPrefix(source, "stored OAuth") {
								isOAuth = true
							}
						}
						usageTracker.UpdateModelInfo(modelInfo, newProvider, isOAuth)
					}
				}
			}
			return nil
		},
		GetAvailableModels: func() []extensions.ModelInfoEntry {
			return kitInstance.GetAvailableModels()
		},
		EmitCustomEvent: func(name string, data string) {
			kitInstance.Extensions().EmitCustomEvent(name, data)
		},
		Complete: func(req extensions.CompleteRequest) (extensions.CompleteResponse, error) {
			return kitInstance.ExecuteCompletion(context.Background(), req)
		},
		SuspendTUI: func(callback func()) error {
			return appInstance.SuspendTUI(callback)
		},
		RenderMessage: func(rendererName, content string) {
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
		},
		ReloadExtensions: func() error {
			err := kitInstance.Extensions().Reload()
			if err != nil {
				return err
			}
			// Notify TUI that widgets/status/commands may have changed.
			go appInstance.NotifyWidgetUpdate()
			return nil
		},
		GetAllTools: func() []extensions.ToolInfo {
			return kitInstance.Extensions().GetToolInfos()
		},
		SetActiveTools: func(names []string) {
			kitInstance.Extensions().SetActiveTools(names)
		},
		RegisterTheme: func(name string, config extensions.ThemeColorConfig) {
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
		},
		SetTheme: func(name string) error {
			return ui.ApplyTheme(name)
		},
		ListThemes: func() []string {
			return ui.ListThemes()
		},
		ShowOverlay: func(config extensions.OverlayConfig) extensions.OverlayResult {
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
		},
		SpawnSubagent: func(config extensions.SubagentConfig) (*extensions.SubagentHandle, *extensions.SubagentResult, error) {
			return extbridge.SpawnSubagent(ctx, kitInstance, config)
		},
		// -------------------------------------------------------------------
		// Tree Navigation API
		// -------------------------------------------------------------------
		GetTreeNode: func(entryID string) *extensions.TreeNode {
			node := kitInstance.GetTreeNode(entryID)
			if node == nil {
				return nil
			}
			return &extensions.TreeNode{
				ID:        node.ID,
				ParentID:  node.ParentID,
				Type:      node.Type,
				Role:      node.Role,
				Content:   node.Content,
				Model:     node.Model,
				Provider:  node.Provider,
				Timestamp: node.Timestamp,
				Children:  node.Children,
			}
		},
		GetCurrentBranch: func() []extensions.TreeNode {
			nodes := kitInstance.GetCurrentBranch()
			result := make([]extensions.TreeNode, len(nodes))
			for i, n := range nodes {
				result[i] = extensions.TreeNode{
					ID:        n.ID,
					ParentID:  n.ParentID,
					Type:      n.Type,
					Role:      n.Role,
					Content:   n.Content,
					Model:     n.Model,
					Provider:  n.Provider,
					Timestamp: n.Timestamp,
					Children:  n.Children,
				}
			}
			return result
		},
		GetChildren: func(parentID string) []string {
			return kitInstance.GetChildren(parentID)
		},
		NavigateTo: func(entryID string) extensions.TreeNavigationResult {
			err := kitInstance.NavigateTo(entryID)
			if err != nil {
				return extensions.TreeNavigationResult{Success: false, Error: err.Error()}
			}
			return extensions.TreeNavigationResult{Success: true}
		},
		SummarizeBranch: func(fromID, toID string) string {
			summary, _ := kitInstance.SummarizeBranch(fromID, toID)
			return summary
		},
		CollapseBranch: func(fromID, toID, summary string) extensions.TreeNavigationResult {
			err := kitInstance.CollapseBranch(fromID, toID, summary)
			if err != nil {
				return extensions.TreeNavigationResult{Success: false, Error: err.Error()}
			}
			return extensions.TreeNavigationResult{Success: true}
		},

		// -------------------------------------------------------------------
		// Skill Loading API
		// -------------------------------------------------------------------
		LoadSkill: func(path string) (*extensions.Skill, string) {
			s, err := kitInstance.LoadSkillForExtension(path)
			return s, err
		},
		LoadSkillsFromDir: func(dir string) extensions.SkillLoadResult {
			return kitInstance.LoadSkillsFromDirForExtension(dir)
		},
		DiscoverSkills: func() extensions.SkillLoadResult {
			skills := kitInstance.DiscoverSkillsForExtension()
			return extensions.SkillLoadResult{Skills: skills}
		},
		InjectSkillAsContext: func(skillName string) string {
			skills := kitInstance.DiscoverSkillsForExtension()
			for _, s := range skills {
				if s.Name == skillName {
					appInstance.Run(fmt.Sprintf("<skill name=%q>\n%s\n</skill>", s.Name, s.Content))
					return ""
				}
			}
			return fmt.Sprintf("skill not found: %s", skillName)
		},
		InjectRawSkillAsContext: func(path string) string {
			s, err := kitInstance.LoadSkillForExtension(path)
			if err != "" {
				return err
			}
			appInstance.Run(fmt.Sprintf("<skill name=%q>\n%s\n</skill>", s.Name, s.Content))
			return ""
		},
		GetAvailableSkills: func() []extensions.Skill {
			return kitInstance.DiscoverSkillsForExtension()
		},

		// -------------------------------------------------------------------
		// Template Parsing API
		// -------------------------------------------------------------------
		ParseTemplate: func(name, content string) extensions.PromptTemplate {
			return kit.ParseTemplate(name, content)
		},
		RenderTemplate: func(tpl extensions.PromptTemplate, vars map[string]string) string {
			return kit.RenderTemplate(tpl, vars)
		},
		ParseArguments: func(input string, pattern extensions.ArgumentPattern) extensions.ParseResult {
			return kit.ParseArguments(input, pattern)
		},
		SimpleParseArguments: func(input string, count int) []string {
			return kit.SimpleParseArguments(input, count)
		},
		EvaluateModelConditional: func(condition string) bool {
			return kit.EvaluateModelConditional(kitInstance.Extensions().GetContext().Model, condition)
		},
		RenderWithModelConditionals: func(content string) string {
			return kit.RenderWithModelConditionals(content, kitInstance.Extensions().GetContext().Model)
		},

		// -------------------------------------------------------------------
		// Model Resolution API
		// -------------------------------------------------------------------
		ResolveModelChain: func(preferences []string) extensions.ModelResolutionResult {
			return kit.ResolveModelChain(preferences)
		},
		GetModelCapabilities: func(model string) (extensions.ModelCapabilities, string) {
			return kit.GetModelCapabilities(model)
		},
		CheckModelAvailable: func(model string) bool {
			return kit.CheckModelAvailable(model)
		},
		GetCurrentProvider: func() string {
			return kit.GetCurrentProvider(kitInstance.Extensions().GetContext().Model)
		},
		GetCurrentModelID: func() string {
			return kit.GetCurrentModelID(kitInstance.Extensions().GetContext().Model)
		},
	}
}

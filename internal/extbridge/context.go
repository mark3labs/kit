package extbridge

import (
	"context"

	"github.com/mark3labs/kit/internal/extensions"
	kit "github.com/mark3labs/kit/pkg/kit"
)

// BaseContext returns an extensions.Context populated with the headless,
// TUI-independent delegation fields: data access, state, options,
// model/tool management, completions, subagents, tree navigation, skills,
// template parsing, and model resolution.
//
// Callers overlay their UI-specific fields (print routes, widgets, prompts,
// editor, TUI-aware SetModel/ReloadExtensions, etc.) on the returned value:
// cmd/extension_context.go for the interactive TUI and
// internal/acpserver/session.go for headless ACP mode. Keeping the shared
// half here means a new data-access Context field only has to be wired once.
//
// ctx is used for subagent spawns; pass a long-lived context (not a
// per-request one) so later spawns aren't cancelled prematurely.
func BaseContext(ctx context.Context, kitInstance *kit.Kit) extensions.Context {
	return extensions.Context{
		// -------------------------------------------------------------------
		// Data access
		// -------------------------------------------------------------------
		GetContextStats: func() extensions.ContextStats {
			s := kitInstance.GetContextStats()
			return extensions.ContextStats{
				EstimatedTokens: s.EstimatedTokens,
				ContextLimit:    s.ContextLimit,
				UsagePercent:    s.UsagePercent,
				MessageCount:    s.MessageCount,
			}
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

		// -------------------------------------------------------------------
		// Extension state
		// -------------------------------------------------------------------
		SetState: func(key string, value string) {
			kitInstance.Extensions().SetState(key, value)
		},
		GetState: func(key string) (string, bool) {
			return kitInstance.Extensions().GetState(key)
		},
		DeleteState: func(key string) {
			kitInstance.Extensions().DeleteState(key)
		},
		ListState: func() []string {
			return kitInstance.Extensions().ListState()
		},

		// -------------------------------------------------------------------
		// Options, model, and tool management
		// -------------------------------------------------------------------
		GetOption: func(name string) string {
			return kitInstance.Extensions().GetOption(name)
		},
		SetOption: func(name string, value string) {
			kitInstance.Extensions().SetOption(name, value)
		},
		// Headless model switch. The interactive TUI overrides this with a
		// version that also notifies the TUI and refreshes the usage tracker.
		SetModel: func(modelString string) error {
			previousModel := kitInstance.Extensions().GetContext().Model
			if err := kitInstance.SetModel(context.Background(), modelString); err != nil {
				return err
			}
			kitInstance.Extensions().UpdateContextModel(modelString)
			kitInstance.Extensions().EmitModelChange(modelString, previousModel, "extension")
			return nil
		},
		GetAvailableModels: func() []extensions.ModelInfoEntry {
			return kitInstance.GetAvailableModels()
		},
		EmitCustomEvent: func(name string, data string) {
			kitInstance.Extensions().EmitCustomEvent(name, data)
		},
		GetAllTools: func() []extensions.ToolInfo {
			return kitInstance.Extensions().GetToolInfos()
		},
		SetActiveTools: func(names []string) {
			kitInstance.Extensions().SetActiveTools(names)
		},
		// Headless reload. The interactive TUI overrides this to also
		// refresh widgets/status/commands.
		ReloadExtensions: func() error {
			return kitInstance.Extensions().Reload()
		},

		// -------------------------------------------------------------------
		// LLM completions and subagents
		// -------------------------------------------------------------------
		Complete: func(req extensions.CompleteRequest) (extensions.CompleteResponse, error) {
			return kitInstance.ExecuteCompletion(context.Background(), req)
		},
		SpawnSubagent: func(config extensions.SubagentConfig) (*extensions.SubagentHandle, *extensions.SubagentResult, error) {
			return SpawnSubagent(ctx, kitInstance, config)
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
		// Skill Loading API (context-injection variants are TUI-specific and
		// wired by the interactive overlay)
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

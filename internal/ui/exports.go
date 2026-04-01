package ui

// This file re-exports types from subpackages for backward compatibility.
// External importers can continue using ui.XXX without needing to import
// from subpackages directly.

import (
	"github.com/mark3labs/kit/internal/ui/commands"
	"github.com/mark3labs/kit/internal/ui/core"
	"github.com/mark3labs/kit/internal/ui/fileutil"
	"github.com/mark3labs/kit/internal/ui/prefs"
	"github.com/mark3labs/kit/internal/ui/style"
)

// Re-export from core package
type (
	ImageAttachment       = core.ImageAttachment
	SubmitMsg             = core.SubmitMsg
	CancelTimerExpiredMsg = core.CancelTimerExpiredMsg
	TreeNodeSelectedMsg   = core.TreeNodeSelectedMsg
	TreeCancelledMsg      = core.TreeCancelledMsg
	ShellCommandMsg       = core.ShellCommandMsg
	ShellCommandResultMsg = core.ShellCommandResultMsg
)

// Re-export from commands package
type (
	SlashCommand     = commands.SlashCommand
	ExtensionCommand = commands.ExtensionCommand
)

// Re-export functions from fileutil package
var ProcessFileAttachments = fileutil.ProcessFileAttachments

// Re-export from prefs package
var (
	LoadThemePreference         = prefs.LoadThemePreference
	SaveThemePreference         = prefs.SaveThemePreference
	LoadModelPreference         = prefs.LoadModelPreference
	SaveModelPreference         = prefs.SaveModelPreference
	LoadThinkingLevelPreference = prefs.LoadThinkingLevelPreference
	SaveThinkingLevelPreference = prefs.SaveThinkingLevelPreference
)

// Re-export from style package
type (
	Theme               = style.Theme
	MarkdownThemeColors = style.MarkdownThemeColors
)

var (
	GetTheme                = style.GetTheme
	SetTheme                = style.SetTheme
	DefaultTheme            = style.DefaultTheme
	ApplyTheme              = style.ApplyTheme
	ApplyThemeWithoutSave   = style.ApplyThemeWithoutSave
	ListThemes              = style.ListThemes
	RegisterThemeFromConfig = style.RegisterThemeFromConfig
	KitBanner               = style.KitBanner
	AdaptiveColor           = style.AdaptiveColor
	IsDarkBackground        = style.IsDarkBackground
)

//go:build ignore

package main

import "kit/ext"

// Init registers a "neon" theme and a /neon slash command to apply it.
// Demonstrates how extensions can create and set themes programmatically.
//
// Usage: kit -e examples/extensions/neon-theme.go
func Init(api ext.API) {
	api.OnSessionStart(func(_ ext.SessionStartEvent, ctx ext.Context) {
		// Register a cyberpunk neon theme at startup.
		ctx.RegisterTheme("neon", ext.ThemeColorConfig{
			Primary:    ext.ThemeColor{Light: "#CC00FF", Dark: "#FF00FF"},
			Secondary:  ext.ThemeColor{Light: "#0088CC", Dark: "#00FFFF"},
			Success:    ext.ThemeColor{Light: "#00CC44", Dark: "#00FF66"},
			Warning:    ext.ThemeColor{Light: "#CCAA00", Dark: "#FFFF00"},
			Error:      ext.ThemeColor{Light: "#CC0033", Dark: "#FF0055"},
			Info:       ext.ThemeColor{Light: "#0088CC", Dark: "#00CCFF"},
			Text:       ext.ThemeColor{Light: "#111111", Dark: "#F0F0F0"},
			Background: ext.ThemeColor{Light: "#F0F0F0", Dark: "#0A0A14"},
			MdKeyword:  ext.ThemeColor{Light: "#CC00FF", Dark: "#FF00FF"},
			MdString:   ext.ThemeColor{Light: "#00CC44", Dark: "#00FF66"},
			MdComment:  ext.ThemeColor{Light: "#888888", Dark: "#555555"},
		})

		ctx.PrintInfo("Neon theme registered! Use /theme neon to activate.")
	})

	// Also register a /neon slash command as a shortcut.
	api.RegisterCommand(ext.CommandDef{
		Name:        "neon",
		Description: "Switch to the neon cyberpunk theme",
		Execute: func(args string, ctx ext.Context) (string, error) {
			if err := ctx.SetTheme("neon"); err != nil {
				return "", err
			}
			return "Neon theme activated!", nil
		},
	})
}

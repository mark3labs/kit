package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/mark3labs/kit/internal/extensions"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var extensionsCmd = &cobra.Command{
	Use:   "extensions",
	Short: "Manage KIT extensions",
	Long:  "Commands for listing, validating, and scaffolding KIT extensions",
}

var extensionsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List discovered extensions and their handlers",
	RunE: func(cmd *cobra.Command, args []string) error {
		loaded, err := extensions.LoadExtensions(viper.GetStringSlice("extension"))
		if err != nil {
			return fmt.Errorf("loading extensions: %w", err)
		}

		if len(loaded) == 0 {
			fmt.Println("No extensions found.")
			fmt.Println()
			fmt.Println("Extension search paths:")
			fmt.Println("  ~/.config/kit/extensions/*.go        (global)")
			fmt.Println("  .kit/extensions/*.go                 (project)")
			fmt.Println()
			fmt.Println("Run 'kit extensions init' to create an example extension.")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		_, _ = fmt.Fprintln(w, "EXTENSION\tEVENT\tHANDLERS\tTOOLS\tCOMMANDS")

		for _, ext := range loaded {
			totalHandlers := 0
			for _, handlers := range ext.Handlers {
				totalHandlers += len(handlers)
			}
			first := true
			for event, handlers := range ext.Handlers {
				if first {
					_, _ = fmt.Fprintf(w, "%s\t%s\t%d\t%d\t%d\n",
						ext.Path, event, len(handlers), len(ext.Tools), len(ext.Commands))
					first = false
				} else {
					_, _ = fmt.Fprintf(w, "\t%s\t%d\t\t\n",
						event, len(handlers))
				}
			}
			if first {
				// Extension loaded but registered no handlers
				_, _ = fmt.Fprintf(w, "%s\t(none)\t0\t%d\t%d\n",
					ext.Path, len(ext.Tools), len(ext.Commands))
			}
		}

		return w.Flush()
	},
}

var extensionsValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate all extension files can be loaded",
	RunE: func(cmd *cobra.Command, args []string) error {
		loaded, err := extensions.LoadExtensions(viper.GetStringSlice("extension"))
		if err != nil {
			return fmt.Errorf("validation failed: %w", err)
		}

		fmt.Printf("Loaded %d extension(s) successfully\n", len(loaded))
		for _, ext := range loaded {
			total := 0
			for _, h := range ext.Handlers {
				total += len(h)
			}
			fmt.Printf("  %s (%d handlers, %d tools, %d commands)\n",
				ext.Path, total, len(ext.Tools), len(ext.Commands))
		}
		return nil
	},
}

var extensionsInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Generate an example extension file",
	RunE: func(cmd *cobra.Command, args []string) error {
		dir := ".kit/extensions"
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("creating extensions directory: %w", err)
		}

		example := `package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"kit/ext"
)

// Init is called when the extension is loaded. Register handlers here.
func Init(api ext.API) {
	// Log every tool call to a file.
	api.OnToolCall(func(tc ext.ToolCallEvent, ctx ext.Context) *ext.ToolCallResult {
		f, err := os.OpenFile("/tmp/kit-tool-log.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err == nil {
			defer f.Close()
			fmt.Fprintf(f, "[%s] tool=%s\n", time.Now().Format(time.RFC3339), tc.ToolName)
		}
		return nil // don't block
	})

	// Block dangerous bash commands.
	api.OnToolCall(func(tc ext.ToolCallEvent, ctx ext.Context) *ext.ToolCallResult {
		if tc.ToolName == "bash" && strings.Contains(tc.Input, "rm -rf /") {
			return &ext.ToolCallResult{Block: true, Reason: "Blocked: dangerous command"}
		}
		return nil
	})

	// Handle custom ! commands. Use ctx.Print/PrintInfo/PrintError/PrintBlock
	// instead of fmt.Println — BubbleTea captures stdout in interactive mode.
	//
	//   ctx.Print("text")       — plain text
	//   ctx.PrintInfo("text")   — styled system message block
	//   ctx.PrintError("text")  — styled error block
	//   ctx.PrintBlock(opts)    — custom block with border color and subtitle
	api.OnInput(func(ie ext.InputEvent, ctx ext.Context) *ext.InputResult {
		switch ie.Text {
		case "!time":
			ctx.PrintInfo("Current time: " + time.Now().Format(time.RFC3339))
			return &ext.InputResult{Action: "handled"}

		case "!status":
			ctx.PrintBlock(ext.PrintBlockOpts{
				Text:        "Session active\nModel: " + ctx.Model + "\nCWD: " + ctx.CWD,
				BorderColor: "#a6e3a1",
				Subtitle:    "my-extension",
			})
			return &ext.InputResult{Action: "handled"}
		}
		return nil
	})
}
`

		path := dir + "/example.go"
		if err := os.WriteFile(path, []byte(example), 0644); err != nil {
			return fmt.Errorf("writing example: %w", err)
		}

		fmt.Printf("Created %s with example extension\n", path)
		fmt.Println()
		fmt.Println("The extension will be auto-loaded on the next kit run.")
		fmt.Println("Use --no-extensions to disable all extensions.")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(extensionsCmd)
	extensionsCmd.AddCommand(extensionsListCmd)
	extensionsCmd.AddCommand(extensionsValidateCmd)
	extensionsCmd.AddCommand(extensionsInitCmd)
}

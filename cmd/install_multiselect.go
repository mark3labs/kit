package cmd

import (
	"fmt"
	"os"

	"charm.land/huh/v2"
	"github.com/charmbracelet/log"
	"github.com/mark3labs/kit/internal/extensions"
)

// multiSelectForInstall runs a multi-select prompt for extension selection.
// Returns the selected extension paths, or an error if cancelled.
func multiSelectForInstall(previews []extensions.ExtensionPreview) ([]string, error) {
	if len(previews) == 0 {
		return nil, fmt.Errorf("no extensions to select")
	}

	// Non-interactive: select all
	if !isInteractive() {
		log.Info("Non-interactive mode, selecting all extensions")
		paths := make([]string, len(previews))
		for i, p := range previews {
			paths[i] = p.Path
		}
		return paths, nil
	}

	// Single extension: just return it
	if len(previews) == 1 {
		return []string{previews[0].Path}, nil
	}

	// Build options for huh MultiSelect
	options := make([]huh.Option[string], len(previews))
	for i, p := range previews {
		label := fmt.Sprintf("%s  %s", p.Name, p.Path)
		options[i] = huh.NewOption(label, p.Path).Selected(true)
	}

	var selected []string

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Select extensions to install").
				Options(options...).
				Value(&selected),
		),
	)

	if err := form.Run(); err != nil {
		return nil, fmt.Errorf("selection cancelled")
	}

	if len(selected) == 0 {
		return nil, fmt.Errorf("no extensions selected")
	}

	return selected, nil
}

// isInteractive checks if the terminal is interactive.
func isInteractive() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

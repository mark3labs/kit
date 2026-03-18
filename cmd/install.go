package cmd

import (
	"fmt"
	"os/exec"

	"github.com/charmbracelet/log"
	"github.com/mark3labs/kit/internal/extensions"
	"github.com/spf13/cobra"
)

var (
	installLocalFlag     bool
	installUpdateFlag    bool
	installUninstallFlag bool
	installSelectFlag    bool
	installAllFlag       bool
)

var installCmd = &cobra.Command{
	Use:   "install <git-url>",
	Short: "Install extensions from git repositories",
	Long: `Install extensions from git repositories.

The install command downloads and installs Kit extensions from git repositories.
Extensions are stored in the global extensions directory by default, or in the
project's .kit/git/ directory when using the --local flag.

Supported URL formats:
  - github.com/user/repo (shorthand, defaults to HTTPS)
  - git:github.com/user/repo
  - https://github.com/user/repo
  - ssh://git@github.com/user/repo
  - git@github.com:user/repo

You can pin to a specific version, tag, or commit using @:
  - github.com/user/repo@v1.0.0
  - github.com/user/repo@main
  - github.com/user/repo@abc1234

Selection modes for repos with multiple extensions:
  - Default: install all extensions
  - --select: interactively choose which extensions to install
  - --all: explicitly install all extensions (same as default)

Examples:
  kit install github.com/user/my-extension
  kit install github.com/user/my-extension@v1.0.0
  kit install git:github.com/user/my-extension --local
  kit install https://github.com/user/my-extension --select
  kit install github.com/user/collection --select --local`,
	Args: cobra.ExactArgs(1),
	RunE: runInstall,
}

func init() {
	installCmd.Flags().BoolVarP(&installLocalFlag, "local", "l", false, "Install to project-local .kit/git/ directory")
	installCmd.Flags().BoolVarP(&installUpdateFlag, "update", "u", false, "Update an already-installed package")
	installCmd.Flags().BoolVar(&installUninstallFlag, "uninstall", false, "Remove an installed package")
	installCmd.Flags().BoolVarP(&installSelectFlag, "select", "i", false, "Interactively select which extensions to install")
	installCmd.Flags().BoolVar(&installAllFlag, "all", false, "Install all extensions (default behavior)")

	rootCmd.AddCommand(installCmd)
}

func runInstall(cmd *cobra.Command, args []string) error {
	sourceStr := args[0]

	// Check that git is available
	if _, err := exec.LookPath("git"); err != nil {
		return fmt.Errorf("git is not installed or not in PATH")
	}

	// Parse the source
	source, err := extensions.ParseGitSource(sourceStr)
	if err != nil {
		return fmt.Errorf("invalid source: %w", err)
	}

	// Determine scope
	scope := extensions.ScopeGlobal
	if installLocalFlag {
		scope = extensions.ScopeProject
	}

	installer := extensions.NewInstaller(".")

	// Handle uninstall
	if installUninstallFlag {
		return runUninstall(installer, source, scope)
	}

	// Handle update
	if installUpdateFlag {
		return runUpdate(installer, source, scope)
	}

	// Handle install
	return runInstallPackage(installer, source, scope)
}

func runInstallPackage(installer *extensions.Installer, source *extensions.GitSource, scope extensions.InstallScope) error {
	// Check if already installed
	existingScope, installed := installer.IsInstalled(source)
	if installed {
		return fmt.Errorf("extension already installed (scope: %s). Use --update to update or --uninstall to remove", existingScope)
	}

	// If --select flag is used, show interactive selection
	if installSelectFlag {
		return runInstallWithSelection(installer, source, scope)
	}

	// Install all extensions
	if err := installer.Install(source, scope); err != nil {
		return fmt.Errorf("install failed: %w", err)
	}

	// Show success message
	scopeStr := "globally"
	if scope == extensions.ScopeProject {
		scopeStr = "locally in .kit/git/"
	}

	if source.Pinned {
		fmt.Printf("Installed %s at %s %s\n", source.String(), source.Ref, scopeStr)
	} else {
		fmt.Printf("Installed %s %s\n", source.String(), scopeStr)
	}

	log.Info("extension installed", "source", source.String(), "scope", scope)
	return nil
}

func runInstallWithSelection(installer *extensions.Installer, source *extensions.GitSource, scope extensions.InstallScope) error {
	// Preview extensions in the repo
	previews, tempDir, err := installer.PreviewExtensions(source)
	if err != nil {
		return fmt.Errorf("previewing extensions: %w", err)
	}
	defer extensions.CleanupTempDir(tempDir)

	if len(previews) == 0 {
		return fmt.Errorf("no extensions found in %s", source.String())
	}

	// If only one extension, just install it
	if len(previews) == 1 {
		fmt.Printf("Found 1 extension in %s:\n  - %s (%s)\n\n", source.String(), previews[0].Name, previews[0].Path)
		return runInstallPackage(installer, source, scope)
	}

	// Use multi-select UI for selection
	includePaths, err := multiSelectForInstall(previews)
	if err != nil {
		if err.Error() == "selection cancelled" {
			fmt.Println("Install cancelled.")
			return nil
		}
		return fmt.Errorf("selection failed: %w", err)
	}

	// Install with includes (if empty, installs all)
	if err := installer.InstallWithInclude(source, scope, includePaths); err != nil {
		return fmt.Errorf("install failed: %w", err)
	}

	// Show success message
	scopeStr := "globally"
	if scope == extensions.ScopeProject {
		scopeStr = "locally in .kit/git/"
	}

	if len(includePaths) > 0 {
		fmt.Printf("Installed %d extension(s) from %s %s\n", len(includePaths), source.String(), scopeStr)
		for _, path := range includePaths {
			fmt.Printf("  - %s\n", path)
		}
	} else {
		fmt.Printf("Installed %s %s\n", source.String(), scopeStr)
	}

	log.Info("extension installed with selection", "source", source.String(), "scope", scope, "selected", len(includePaths))
	return nil
}

func runUpdate(installer *extensions.Installer, source *extensions.GitSource, scope extensions.InstallScope) error {
	// Find the installed package
	existingScope, installed := installer.IsInstalled(source)
	if !installed {
		// Try to find with wildcard (no version)
		entry, foundScope, err := extensions.FindInManifest(source.Identity())
		if err != nil || entry == nil {
			return fmt.Errorf("extension not installed: %s", source.Identity())
		}
		// Parse the found entry's source
		foundSource, err := extensions.ParseGitSource(entry.Source)
		if err != nil {
			return fmt.Errorf("failed to parse installed source: %w", err)
		}
		existingScope = foundScope
		source = foundSource
	}

	// Override scope if specified
	if installLocalFlag && scope != existingScope {
		return fmt.Errorf("extension installed in %s scope, cannot update with --local flag", existingScope)
	}
	scope = existingScope

	// Check if pinned
	if source.Pinned {
		fmt.Printf("Skipping %s (pinned at %s)\n", source.Identity(), source.Ref)
		return nil
	}

	// Update
	if err := installer.Update(source, scope); err != nil {
		return fmt.Errorf("update failed: %w", err)
	}

	fmt.Printf("Updated %s\n", source.Identity())
	log.Info("extension updated", "source", source.Identity(), "scope", scope)
	return nil
}

func runUninstall(installer *extensions.Installer, source *extensions.GitSource, scope extensions.InstallScope) error {
	// Find where it's installed (ignore scope flag for uninstall - remove from wherever it exists)
	existingScope, installed := installer.IsInstalled(source)
	if !installed {
		// Try to find in manifests
		entry, foundScope, err := extensions.FindInManifest(source.Identity())
		if err != nil || entry == nil {
			return fmt.Errorf("extension not installed: %s", source.Identity())
		}
		existingScope = foundScope
		// Parse the found entry's source
		foundSource, err := extensions.ParseGitSource(entry.Source)
		if err != nil {
			return fmt.Errorf("failed to parse installed source: %w", err)
		}
		source = foundSource
	}

	// Uninstall from the scope where it's installed
	if err := installer.Uninstall(source, existingScope); err != nil {
		return fmt.Errorf("uninstall failed: %w", err)
	}

	fmt.Printf("Uninstalled %s from %s scope\n", source.Identity(), existingScope)
	log.Info("extension uninstalled", "source", source.Identity(), "scope", existingScope)
	return nil
}

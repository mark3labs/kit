package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/mark3labs/kit/pkg/kit"
)

// skillCmd groups the skill-related subcommands. Running it bare keeps the
// historical behaviour of installing the Kit skills via skills.sh so existing
// scripts and docs do not break.
var skillCmd = &cobra.Command{
	Use:   "skill",
	Short: "Manage agent skills (list, validate, install)",
	Long: `Manage Agent Skills (https://agentskills.io/specification).

Skills are directories with a SKILL.md file that teach the agent a workflow.
Kit discovers them from ~/.agents/skills, ~/.config/kit/skills,
<project>/.agents/skills and <project>/.kit/skills. Activate a skill in the
TUI with /<skill-name> [args], or let the model call the activate_skill tool.

Subcommands:

  kit skill list              List discovered skills with scope and spec warnings
  kit skill validate <dir>    Validate a skill directory against the spec
  kit skill install           Install the Kit skills (extensions + SDK) via skills.sh

Running "kit skill" with no subcommand is the same as "kit skill install".`,
	RunE: runSkillInstall,
}

var skillListCmd = &cobra.Command{
	Use:   "list",
	Short: "List discovered skills with scope and spec warnings",
	Long: `List the skills Kit would load from the current directory, honoring the
--skill, --skills-dir and --no-skills flags. For each skill the scope
(user or project), the path and any agentskills.io spec warnings are shown.
Project-local skills are listed without a trust prompt because nothing is
injected into an agent here.`,
	RunE: runSkillList,
}

var skillValidateCmd = &cobra.Command{
	Use:   "validate <path> [path...]",
	Short: "Validate skill files or directories against the agentskills.io spec",
	Long: `Validate one or more skills. Each path may be a skill directory (containing
SKILL.md), a SKILL.md file, or a directory of skills. Errors (missing name or
description) mean the skill will not load; warnings (name format, length
limits, name/directory mismatch, over-long body) mean it loads but deviates
from the spec. The exit code is non-zero when any error is found.`,
	Args: cobra.MinimumNArgs(1),
	RunE: runSkillValidate,
}

var skillInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install the Kit skills (extensions + SDK) via skills.sh",
	Long: `Install the Kit skills that teach AI agents how to build with Kit. Uses
the skills.sh CLI (npx skills) to install all skills from the Kit repository:

  kit-extensions — creating Kit extensions with full knowledge of the
                   extension API, lifecycle events, widgets, tools,
                   commands, editor interceptors, tool renderers, and
                   Yaegi interpreter constraints.

  kit-sdk        — building AI-powered applications with the Kit Go SDK,
                   including providers, agents, tools, and MCP integration.`,
	RunE: runSkillInstall,
}

func init() {
	rootCmd.AddCommand(skillCmd)
	skillCmd.AddCommand(skillListCmd)
	skillCmd.AddCommand(skillValidateCmd)
	skillCmd.AddCommand(skillInstallCmd)
}

func runSkillInstall(_ *cobra.Command, _ []string) error {
	npx, err := exec.LookPath("npx")
	if err != nil {
		return fmt.Errorf("npx not found in PATH — install Node.js to use this command: %w", err)
	}

	args := []string{
		"skills",
		"add",
		"mark3labs/kit",
	}

	cmd := exec.Command(npx, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("skills install failed: %w", err)
	}

	return nil
}

// discoverSkillsForListing resolves the skill set the same way kit.New does
// (explicit --skill paths, then --skills-dir, then auto-discovery) but never
// prompts for project trust: listing is read-only.
func discoverSkillsForListing() ([]*kit.Skill, error) {
	if noSkillsFlag {
		return nil, nil
	}
	if len(skillsPaths) > 0 {
		var all []*kit.Skill
		for _, p := range skillsPaths {
			ss, err := loadSkillPath(p)
			if err != nil {
				return nil, err
			}
			all = append(all, ss...)
		}
		return kit.CombineSkills(all, nil), nil
	}
	if skillsDir != "" {
		ss, err := kit.LoadSkillsFromDir(skillsDir)
		if err != nil {
			return nil, err
		}
		return kit.CombineSkills(ss, nil), nil
	}
	cwd, _ := os.Getwd()
	var project []*kit.Skill
	if !bareFlag {
		project = kit.LoadProjectSkills(cwd)
	}
	return kit.CombineSkills(kit.LoadUserSkills(), project), nil
}

// loadSkillPath loads a single --skill argument, which may be a file or a
// directory of skills.
func loadSkillPath(p string) ([]*kit.Skill, error) {
	info, err := os.Stat(p)
	if err != nil {
		return nil, fmt.Errorf("skill path %s: %w", p, err)
	}
	if info.IsDir() {
		return kit.LoadSkillsFromDir(p)
	}
	s, err := kit.LoadSkill(p)
	if err != nil {
		return nil, err
	}
	return []*kit.Skill{s}, nil
}

func runSkillList(_ *cobra.Command, _ []string) error {
	list, err := discoverSkillsForListing()
	if err != nil {
		return err
	}
	if len(list) == 0 {
		fmt.Println("No skills found.")
		fmt.Println()
		fmt.Println("Skill search paths:")
		home, _ := os.UserHomeDir()
		cwd, _ := os.Getwd()
		fmt.Printf("  %-40s (user)\n", filepath.Join(home, ".agents", "skills"))
		fmt.Printf("  %-40s (user)\n", filepath.Join(home, ".config", "kit", "skills"))
		fmt.Printf("  %-40s (project)\n", filepath.Join(cwd, ".agents", "skills"))
		fmt.Printf("  %-40s (project)\n", filepath.Join(cwd, ".kit", "skills"))
		fmt.Println()
		fmt.Println("A skill is a directory containing SKILL.md with name and description frontmatter.")
		return nil
	}

	sort.Slice(list, func(i, j int) bool { return list[i].Name < list[j].Name })

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "SKILL\tSCOPE\tMODEL\tWARNINGS\tPATH")
	totalWarnings := 0
	for _, s := range list {
		model := "yes"
		if s.DisableModelInvocation {
			model = "hidden"
		}
		warnings := s.Warnings()
		totalWarnings += len(warnings)
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\n", s.Name, s.Scope(), model, len(warnings), s.Path)
	}
	if err := w.Flush(); err != nil {
		return err
	}

	if totalWarnings > 0 {
		fmt.Println()
		for _, s := range list {
			for _, d := range s.Warnings() {
				fmt.Printf("  %s: %s\n", s.Name, formatDiagnostic(d))
			}
		}
		fmt.Println()
		fmt.Println("Run 'kit skill validate <path>' for details on a single skill.")
	}
	return nil
}

func runSkillValidate(_ *cobra.Command, args []string) error {
	var errCount, warnCount, checked int
	for _, p := range args {
		list, err := collectSkillsForValidation(p)
		if err != nil {
			errCount++
			fmt.Printf("✗ %s\n    error: %v\n", p, err)
			continue
		}
		if len(list) == 0 {
			errCount++
			fmt.Printf("✗ %s\n    error: no SKILL.md found\n", p)
			continue
		}
		for _, s := range list {
			checked++
			diags := s.Validate()
			hasErr := false
			for _, d := range diags {
				if d.Severity == "error" {
					hasErr = true
				}
			}
			marker := "✓"
			if hasErr {
				marker = "✗"
				errCount++
			} else if len(diags) > 0 {
				marker = "!"
			}
			fmt.Printf("%s %s (%s)\n", marker, s.Name, s.Path)
			for _, d := range diags {
				if d.Severity == "warning" {
					warnCount++
				}
				fmt.Printf("    %s\n", formatDiagnostic(d))
			}
		}
	}

	fmt.Printf("\n%d skill(s) checked, %d error(s), %d warning(s)\n", checked, errCount, warnCount)
	if errCount > 0 {
		return fmt.Errorf("%d skill(s) failed validation", errCount)
	}
	return nil
}

// collectSkillsForValidation loads every skill reachable from p: a SKILL.md
// file, a directory containing SKILL.md, or a directory of skill directories.
// Skills are returned unvalidated so the caller can report their diagnostics.
func collectSkillsForValidation(p string) ([]*kit.Skill, error) {
	info, err := os.Stat(p)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		s, err := kit.LoadSkill(p)
		if err != nil {
			return nil, err
		}
		return []*kit.Skill{s}, nil
	}
	// A skill directory: SKILL.md (any case) directly inside.
	entries, err := os.ReadDir(p)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if !e.IsDir() && strings.EqualFold(e.Name(), "SKILL.md") {
			s, err := kit.LoadSkill(filepath.Join(p, e.Name()))
			if err != nil {
				return nil, err
			}
			return []*kit.Skill{s}, nil
		}
	}
	// Otherwise treat it as a directory of skills.
	return kit.LoadSkillsFromDir(p)
}

// formatDiagnostic renders a diagnostic as "severity[field]: message".
func formatDiagnostic(d kit.SkillDiagnostic) string {
	if d.Field != "" {
		return fmt.Sprintf("%s[%s]: %s", d.Severity, d.Field, d.Message)
	}
	return fmt.Sprintf("%s: %s", d.Severity, d.Message)
}

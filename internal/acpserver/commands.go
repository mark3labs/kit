package acpserver

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	acp "github.com/coder/acp-go-sdk"
	"github.com/spf13/viper"

	"github.com/mark3labs/kit/internal/prompts"
)

// commandsDelay is how long announceCommands waits before it sends the
// command list, so that the client has the session/new (or load/resume)
// response, and with it the session ID, before the update arrives.
const commandsDelay = 100 * time.Millisecond

// loadPromptTemplates loads the prompt templates for a session cwd, from the
// same places and with the same settings as the interactive CLI.
func loadPromptTemplates(cwd string) []*prompts.PromptTemplate {
	if viper.GetBool("no-prompt-templates") {
		return nil
	}
	home, _ := os.UserHomeDir()
	tpls, _, err := prompts.LoadAll(prompts.LoadOptions{
		Cwd:             cwd,
		HomeDir:         home,
		ExtraPaths:      viper.GetStringSlice("prompt-template"),
		ConfigPaths:     viper.GetStringSlice("prompts"),
		IncludeDefaults: true,
	})
	if err != nil {
		log.Debug("acp: some prompt templates failed to load", "error", err)
	}
	return tpls
}

// availableCommands lists the slash commands of a session: prompt templates
// and skills. When a template and a skill have the same name, the template
// wins, as in the interactive CLI. Extension commands are not listed: most
// of them need the terminal UI.
func availableCommands(sess *acpSession) []acp.AvailableCommand {
	var cmds []acp.AvailableCommand
	seen := map[string]bool{}

	for _, t := range loadPromptTemplates(sess.cwd) {
		if t.Name == "" || seen[t.Name] {
			continue
		}
		seen[t.Name] = true
		desc := strings.TrimSpace(t.Description)
		if desc == "" {
			desc = "Prompt template"
		}
		cmd := acp.AvailableCommand{Name: t.Name, Description: desc}
		if t.HasArgPlaceholders() {
			hint := "arguments"
			if n := t.RequiredArgs(); n > 0 {
				hint = fmt.Sprintf("%d argument(s)", n)
			}
			cmd.Input = &acp.AvailableCommandInput{Unstructured: &acp.UnstructuredCommandInput{Hint: hint}}
		}
		cmds = append(cmds, cmd)
	}

	for _, s := range sess.kit.GetSkills() {
		if s == nil || s.Name == "" || seen[s.Name] {
			continue
		}
		seen[s.Name] = true
		desc := strings.TrimSpace(s.Description)
		if desc == "" {
			desc = "Skill"
		}
		cmds = append(cmds, acp.AvailableCommand{
			Name:        s.Name,
			Description: "Skill: " + desc,
			Input:       &acp.AvailableCommandInput{Unstructured: &acp.UnstructuredCommandInput{Hint: "optional instructions"}},
		})
	}

	sort.Slice(cmds, func(i, j int) bool { return cmds[i].Name < cmds[j].Name })
	return cmds
}

// announceCommands sends the session's slash commands to the client with an
// available_commands_update, shortly after the current request returns.
func (a *Agent) announceCommands(ctx context.Context, sess *acpSession) {
	cmds := availableCommands(sess)
	if len(cmds) == 0 {
		return
	}
	ctx = context.WithoutCancel(ctx)
	go func() {
		time.Sleep(commandsDelay)
		if err := a.conn.SessionUpdate(ctx, acp.SessionNotification{
			SessionId: sess.id(),
			Update: acp.SessionUpdate{AvailableCommandsUpdate: &acp.SessionAvailableCommandsUpdate{
				AvailableCommands: cmds,
			}},
		}); err != nil {
			log.Debug("acp: available commands update failed", "session", sess.sessionID, "error", err)
		}
	}()
}

// expandPromptTemplate expands a prompt of the form "/name args" when name
// is a prompt template. Other prompts, including skill commands (which Kit
// expands itself), are returned unchanged. A template that needs more
// arguments than given is an invalid parameter.
func expandPromptTemplate(cwd, text string) (string, error) {
	trimmed := strings.TrimSpace(text)
	if !strings.HasPrefix(trimmed, "/") {
		return text, nil
	}
	name, args, _ := strings.Cut(trimmed[1:], " ")
	if i := strings.IndexAny(name, "\n\t"); i >= 0 {
		name, args = name[:i], name[i:]+" "+args
	}
	if name == "" {
		return text, nil
	}
	for _, t := range loadPromptTemplates(cwd) {
		if t.Name != name {
			continue
		}
		if need := t.RequiredArgs(); need > 0 {
			if got := len(prompts.ParseCommandArgs(args)); got < need {
				return "", acp.NewInvalidParams(fmt.Sprintf("/%s needs %d argument(s), got %d", name, need, got))
			}
		}
		return t.Expand(strings.TrimSpace(args)), nil
	}
	return text, nil
}

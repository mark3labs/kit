package cmd

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"

	"github.com/mark3labs/kit/internal/app"
	"github.com/mark3labs/kit/internal/config"
	"github.com/mark3labs/kit/internal/ui"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// scriptCmd represents the script command for executing KIT script files.
// Script files can contain YAML frontmatter configuration followed by a prompt,
// allowing for reproducible AI interactions with custom configurations and
// variable substitution support.
var scriptCmd = &cobra.Command{
	Use:   "script <script-file>",
	Short: "Execute a script file with YAML frontmatter configuration",
	Long: `Execute a script file that contains YAML frontmatter with configuration
and a prompt. The script file can contain MCP server configurations,
model settings, and other options.

Example script file:
---
model: "anthropic/claude-sonnet-4-5-20250929"
max-steps: 10
mcpServers:
  filesystem:
    type: "local"
    command: ["npx", "-y", "@modelcontextprotocol/server-filesystem", "${directory:-/tmp}"]
---
Hello ${name:-World}! List the files in ${directory:-/tmp} and tell me about them.

The script command supports the same flags as the main command,
which will override any settings in the script file.

Variable substitution:
Variables in the script can be substituted using ${variable} syntax.
Variables can have default values using ${variable:-default} syntax.
Pass variables using --args:variable value syntax:

  kit script myscript.sh --args:directory /tmp --args:name "John"

This will replace ${directory} with "/tmp" and ${name} with "John" in the script.
Variables with defaults (${var:-default}) are optional and use the default if not provided.`,
	Args: cobra.ExactArgs(1),
	FParseErrWhitelist: cobra.FParseErrWhitelist{
		UnknownFlags: true, // Allow unknown flags for variable substitution
	},
	PreRun: func(cmd *cobra.Command, args []string) {
		// Override config with frontmatter values from the script file
		scriptFile := args[0]
		variables := parseCustomVariables(cmd)
		overrideConfigWithFrontmatter(scriptFile, variables, cmd)
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		scriptFile := args[0]

		// Parse custom variables from unknown flags
		variables := parseCustomVariables(cmd)

		return runScriptCommand(context.Background(), scriptFile, variables, cmd)
	},
}

func init() {
	rootCmd.AddCommand(scriptCmd)
}

// overrideConfigWithFrontmatter parses the script file and overrides viper config with frontmatter values
// This is the only purpose of this function - to apply frontmatter configuration to viper
func overrideConfigWithFrontmatter(scriptFile string, variables map[string]string, cmd *cobra.Command) {
	// Parse the script file to get frontmatter configuration
	scriptConfig, err := parseScriptFile(scriptFile, variables)
	if err != nil {
		// If we can't parse the script file, just continue with existing config
		// The error will be handled again in runScriptCommand
		return
	}

	// Override viper values with frontmatter values (only if flags weren't explicitly set)
	// Check both local flags and persistent flags since script inherits from root
	flagChanged := func(name string) bool {
		return cmd.Flags().Changed(name) || rootCmd.PersistentFlags().Changed(name)
	}

	if scriptConfig.Model != "" && !flagChanged("model") {
		viper.Set("model", scriptConfig.Model)
	}
	if scriptConfig.MaxSteps != 0 && !flagChanged("max-steps") {
		viper.Set("max-steps", scriptConfig.MaxSteps)
	}
	if scriptConfig.Debug && !flagChanged("debug") {
		viper.Set("debug", scriptConfig.Debug)
	}
	if scriptConfig.Compact && !flagChanged("compact") {
		viper.Set("compact", scriptConfig.Compact)
	}
	if scriptConfig.SystemPrompt != "" && !flagChanged("system-prompt") {
		viper.Set("system-prompt", scriptConfig.SystemPrompt)
	}
	if scriptConfig.ProviderAPIKey != "" && !flagChanged("provider-api-key") {
		viper.Set("provider-api-key", scriptConfig.ProviderAPIKey)
	}
	if scriptConfig.ProviderURL != "" && !flagChanged("provider-url") {
		viper.Set("provider-url", scriptConfig.ProviderURL)
	}
	if scriptConfig.MaxTokens != 0 && !flagChanged("max-tokens") {
		viper.Set("max-tokens", scriptConfig.MaxTokens)
	}
	if scriptConfig.Temperature != nil && !flagChanged("temperature") {
		viper.Set("temperature", *scriptConfig.Temperature)
	}
	if scriptConfig.TopP != nil && !flagChanged("top-p") {
		viper.Set("top-p", *scriptConfig.TopP)
	}
	if scriptConfig.TopK != nil && !flagChanged("top-k") {
		viper.Set("top-k", *scriptConfig.TopK)
	}
	if len(scriptConfig.StopSequences) > 0 && !flagChanged("stop-sequences") {
		viper.Set("stop-sequences", scriptConfig.StopSequences)
	}
	if scriptConfig.NoExit && !flagChanged("no-exit") {
		// Set the global noExitFlag variable if it wasn't explicitly set via command line
		noExitFlag = scriptConfig.NoExit
	}
	if scriptConfig.Stream != nil && !flagChanged("stream") {
		viper.Set("stream", *scriptConfig.Stream)
	}
	if scriptConfig.TLSSkipVerify && !flagChanged("tls-skip-verify") {
		viper.Set("tls-skip-verify", scriptConfig.TLSSkipVerify)
	}
}

// parseCustomVariables extracts custom variables from command line arguments
func parseCustomVariables(_ *cobra.Command) map[string]string {
	variables := make(map[string]string)

	// Get all arguments passed to the command
	args := os.Args[1:] // Skip program name

	// Find the script subcommand position
	scriptPos := -1
	for i, arg := range args {
		if arg == "script" {
			scriptPos = i
			break
		}
	}

	if scriptPos == -1 {
		return variables
	}

	// Parse arguments after the script file
	scriptFileFound := false

	for i := scriptPos + 1; i < len(args); i++ {
		arg := args[i]

		// Skip the script file argument (first non-flag after "script")
		if !scriptFileFound && !strings.HasPrefix(arg, "-") {
			scriptFileFound = true
			continue
		}

		// Parse custom variables with --args: prefix
		if after, ok := strings.CutPrefix(arg, "--args:"); ok {
			varName := after
			if varName == "" {
				continue // Skip malformed --args: without name
			}

			// Check if we have a value
			if i+1 < len(args) {
				varValue := args[i+1]

				// Make sure the next arg isn't a flag
				if !strings.HasPrefix(varValue, "-") {
					variables[varName] = varValue
					i++ // Skip the value
				} else {
					// No value provided, treat as empty string
					variables[varName] = ""
				}
			} else {
				// No value provided, treat as empty string
				variables[varName] = ""
			}
		}
	}

	return variables
}

func runScriptCommand(ctx context.Context, scriptFile string, variables map[string]string, _ *cobra.Command) error {
	// Parse the script file to get MCP servers and prompt
	scriptConfig, err := parseScriptFile(scriptFile, variables)
	if err != nil {
		return fmt.Errorf("failed to parse script file: %v", err)
	}

	// Get MCP config - use script servers if available, otherwise use global viper config
	var mcpConfig *config.Config
	if len(scriptConfig.MCPServers) > 0 {
		// Load base config and merge with script config
		baseConfig, err := config.LoadAndValidateConfig()
		if err != nil {
			return fmt.Errorf("failed to load base config: %v", err)
		}
		mcpConfig = config.MergeConfigs(baseConfig, scriptConfig)
	} else {
		// Use the new config loader
		var err error
		mcpConfig, err = config.LoadAndValidateConfig()
		if err != nil {
			return fmt.Errorf("failed to load MCP config: %v", err)
		}
	}

	// Get final prompt - prioritize command line flag, then script content
	finalPrompt := viper.GetString("prompt")
	if finalPrompt == "" && scriptConfig.Prompt != "" {
		finalPrompt = scriptConfig.Prompt
	}

	// Get final no-exit setting - prioritize command line flag, then script config
	finalNoExit := noExitFlag || scriptConfig.NoExit

	// Validate that --no-exit is only used when there's a prompt
	if finalNoExit && finalPrompt == "" {
		return fmt.Errorf("--no-exit flag can only be used when there's a prompt (either from script content or --prompt flag)")
	}

	// Run the script using the unified agentic loop
	return runScriptMode(ctx, mcpConfig, finalPrompt, finalNoExit)
}

// mergeScriptConfig and setScriptValuesInViper functions removed
// Configuration override is now handled in overrideConfigWithFrontmatter in the PreRun hook

// parseScriptFile parses a script file with YAML frontmatter and returns config
func parseScriptFile(filename string, variables map[string]string) (*config.Config, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)

	// Skip shebang line if present
	if scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "#!") {
			// If it's not a shebang, we need to process this line
			return parseScriptContent(line+"\n"+readRemainingLines(scanner), variables)
		}
	}

	// Read the rest of the file
	content := readRemainingLines(scanner)
	return parseScriptContent(content, variables)
}

// readRemainingLines reads all remaining lines from a scanner
func readRemainingLines(scanner *bufio.Scanner) string {
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return strings.Join(lines, "\n")
}

// parseScriptContent parses the content to extract YAML frontmatter and prompt
func parseScriptContent(content string, variables map[string]string) (*config.Config, error) {
	// STEP 1: Apply environment variable substitution FIRST
	envSubstituter := &config.EnvSubstituter{}
	processedContent, err := envSubstituter.SubstituteEnvVars(content)
	if err != nil {
		return nil, fmt.Errorf("script env substitution failed: %v", err)
	}

	// STEP 2: Validate that all declared script variables are provided
	if err := validateVariables(processedContent, variables); err != nil {
		return nil, err
	}

	// STEP 3: Apply script args substitution
	argsSubstituter := config.NewArgsSubstituter(variables)
	content, err = argsSubstituter.SubstituteArgs(processedContent)
	if err != nil {
		return nil, fmt.Errorf("script args substitution failed: %v", err)
	}

	lines := strings.Split(content, "\n")

	// Find YAML frontmatter between --- delimiters
	var yamlLines []string
	var promptLines []string
	var inFrontmatter bool
	var foundFrontmatter bool
	var frontmatterEnd = -1

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Skip comment lines (lines starting with #)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Check for frontmatter start
		if trimmed == "---" && !inFrontmatter {
			// Start of frontmatter
			inFrontmatter = true
			foundFrontmatter = true
			continue
		}

		// Check for frontmatter end
		if trimmed == "---" && inFrontmatter {
			// End of frontmatter
			inFrontmatter = false
			frontmatterEnd = i + 1
			continue
		}

		// Collect frontmatter lines
		if inFrontmatter {
			yamlLines = append(yamlLines, line)
		}
	}

	// Extract prompt (everything after frontmatter)
	if foundFrontmatter && frontmatterEnd != -1 && frontmatterEnd < len(lines) {
		promptLines = lines[frontmatterEnd:]
	} else if !foundFrontmatter {
		// If no frontmatter found, treat entire content as prompt
		promptLines = lines
		yamlLines = []string{} // Empty YAML
	}

	// Parse YAML frontmatter using Viper for consistency with config file parsing
	var scriptConfig config.Config
	if len(yamlLines) > 0 {
		yamlContent := strings.Join(yamlLines, "\n")

		// Create temporary viper instance for frontmatter parsing
		frontmatterViper := viper.New()
		frontmatterViper.SetConfigType("yaml")

		if err := frontmatterViper.ReadConfig(strings.NewReader(yamlContent)); err != nil {
			return nil, fmt.Errorf("failed to parse YAML frontmatter: %v\nYAML content:\n%s", err, yamlContent)
		}

		if err := frontmatterViper.Unmarshal(&scriptConfig); err != nil {
			return nil, fmt.Errorf("failed to unmarshal frontmatter config: %v", err)
		}

		// Manually extract hyphenated keys that Viper might not handle correctly during unmarshal
		if providerURL := frontmatterViper.GetString("provider-url"); providerURL != "" {
			scriptConfig.ProviderURL = providerURL
		}
		if providerAPIKey := frontmatterViper.GetString("provider-api-key"); providerAPIKey != "" {
			scriptConfig.ProviderAPIKey = providerAPIKey
		}
		if systemPrompt := frontmatterViper.GetString("system-prompt"); systemPrompt != "" {
			scriptConfig.SystemPrompt = systemPrompt
		}
		if maxSteps := frontmatterViper.GetInt("max-steps"); maxSteps != 0 {
			scriptConfig.MaxSteps = maxSteps
		}
		if maxTokens := frontmatterViper.GetInt("max-tokens"); maxTokens != 0 {
			scriptConfig.MaxTokens = maxTokens
		}
		if topP := frontmatterViper.GetFloat64("top-p"); topP != 0 {
			topPFloat32 := float32(topP)
			scriptConfig.TopP = &topPFloat32
		}
		if topK := frontmatterViper.GetInt("top-k"); topK != 0 {
			topKInt32 := int32(topK)
			scriptConfig.TopK = &topKInt32
		}
		if stopSequences := frontmatterViper.GetStringSlice("stop-sequences"); len(stopSequences) > 0 {
			scriptConfig.StopSequences = stopSequences
		}
		if noExit := frontmatterViper.GetBool("no-exit"); noExit {
			scriptConfig.NoExit = noExit
		}
		if tlsSkipVerify := frontmatterViper.GetBool("tls-skip-verify"); tlsSkipVerify {
			scriptConfig.TLSSkipVerify = tlsSkipVerify
		}
	}

	// Set prompt from content after frontmatter
	if len(promptLines) > 0 {
		prompt := strings.Join(promptLines, "\n")
		prompt = strings.TrimSpace(prompt) // Remove leading/trailing whitespace
		if prompt != "" {
			scriptConfig.Prompt = prompt
		}
	}

	return &scriptConfig, nil
}

// Variable represents a script variable with optional default value.
// Variables can be declared in scripts using ${variable} syntax for required variables
// or ${variable:-default} syntax for variables with default values.
type Variable struct {
	Name         string // The name of the variable as it appears in the script
	DefaultValue string // The default value if specified using ${variable:-default} syntax
	HasDefault   bool   // Whether this variable has a default value
}

// findVariables extracts all unique variable names from ${variable} patterns in content
// Maintains backward compatibility by returning just variable names
func findVariables(content string) []string {
	variables := findVariablesWithDefaults(content)
	var names []string
	for _, v := range variables {
		names = append(names, v.Name)
	}
	return names
}

// findVariablesWithDefaults extracts all unique variables with their default values
// Supports both ${variable} and ${variable:-default} syntax
func findVariablesWithDefaults(content string) []Variable {
	// Pattern matches:
	// ${varname} - simple variable
	// ${varname:-default} - variable with default value
	re := regexp.MustCompile(`\$\{([^}:]+)(?::-([^}]*))?\}`)
	matches := re.FindAllStringSubmatch(content, -1)

	seenVars := make(map[string]bool)
	var variables []Variable

	for _, match := range matches {
		if len(match) >= 2 {
			varName := match[1]
			if !seenVars[varName] {
				seenVars[varName] = true

				// Check if the original match contains the :- pattern
				hasDefault := strings.Contains(match[0], ":-")

				variable := Variable{
					Name:       varName,
					HasDefault: hasDefault,
				}

				if hasDefault && len(match) >= 3 {
					variable.DefaultValue = match[2] // Can be empty string
				}

				variables = append(variables, variable)
			}
		}
	}

	return variables
}

// validateVariables checks that all declared variables in the content are provided
// Variables with default values are not required
func validateVariables(content string, variables map[string]string) error {
	declaredVars := findVariablesWithDefaults(content)

	var missingVars []string
	for _, variable := range declaredVars {
		if _, exists := variables[variable.Name]; !exists && !variable.HasDefault {
			missingVars = append(missingVars, variable.Name)
		}
	}

	if len(missingVars) > 0 {
		return fmt.Errorf("missing required variables: %s\nProvide them using --args:variable value syntax", strings.Join(missingVars, ", "))
	}

	return nil
}

// substituteVariables replaces ${variable} and ${variable:-default} patterns with their values
// This function is kept for backward compatibility but now uses the shared ArgsSubstituter
func substituteVariables(content string, variables map[string]string) string {
	substituter := config.NewArgsSubstituter(variables)
	result, err := substituter.SubstituteArgs(content)
	if err != nil {
		// For backward compatibility, if there's an error, return the original content
		// This maintains the existing behavior where missing variables were left as-is
		return content
	}
	return result
}

// runScriptMode executes the script using the unified agentic loop
func runScriptMode(ctx context.Context, mcpConfig *config.Config, prompt string, noExit bool) error {
	// Set up logging
	if debugMode || mcpConfig.Debug {
		log.SetFlags(log.LstdFlags | log.Lshortfile)
	}

	// Create agent using shared setup. Script frontmatter values are already
	// merged into viper by overrideConfigWithFrontmatter (PreRun hook), so
	// BuildProviderConfig inside SetupAgent reads the correct final values.
	agentResult, err := SetupAgent(ctx, AgentSetupOptions{
		MCPConfig: mcpConfig,
	})
	if err != nil {
		return err
	}
	mcpAgent := agentResult.Agent
	defer func() { _ = mcpAgent.Close() }()

	// Collect model/server/tool metadata.
	parsedProvider, modelName, serverNames, toolNames := CollectAgentMetadata(mcpAgent, mcpConfig)

	// Create CLI display layer.
	cli, err := SetupCLIForNonInteractive(mcpAgent)
	if err != nil {
		return fmt.Errorf("failed to setup CLI: %v", err)
	}

	DisplayDebugConfig(cli, mcpAgent, mcpConfig, parsedProvider)

	// Build app options.
	appOpts := BuildAppOptions(mcpAgent, mcpConfig, modelName, serverNames, toolNames)
	if cli != nil {
		if tracker := cli.GetUsageTracker(); tracker != nil {
			appOpts.UsageTracker = tracker
		}
	}

	appInstance := app.New(appOpts, nil)
	defer appInstance.Close()

	if quietFlag {
		// Quiet mode: no intermediate display, just print final response.
		return appInstance.RunOnce(ctx, prompt)
	}

	// Display user message before running the agent.
	if cli != nil {
		cli.DisplayUserMessage(prompt)
	}

	// Build an event handler that routes app events to the CLI.
	eventHandler := ui.NewCLIEventHandler(cli, modelName)
	err = appInstance.RunOnceWithDisplay(ctx, prompt, eventHandler.Handle)
	eventHandler.Cleanup()
	return err
}

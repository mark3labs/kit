package kit

import (
	"regexp"
	"strings"

	"github.com/mark3labs/kit/internal/extensions"
	"github.com/mark3labs/kit/internal/models"
)

// ---------------------------------------------------------------------------
// Template Parsing Bridge for Extensions (Phase 3)
// ---------------------------------------------------------------------------

// varRegex matches {{variable}} placeholders in templates.
var varRegex = regexp.MustCompile(`\{\{\s*([a-zA-Z_][a-zA-Z0-9_]*)\s*\}\}`)

// ParseTemplate extracts {{variables}} from template content.
func ParseTemplate(name, content string) extensions.PromptTemplate {
	matches := varRegex.FindAllStringSubmatch(content, -1)
	vars := make([]string, 0, len(matches))
	seen := make(map[string]bool)
	for _, m := range matches {
		if len(m) > 1 && !seen[m[1]] {
			seen[m[1]] = true
			vars = append(vars, m[1])
		}
	}
	return extensions.PromptTemplate{
		Name:      name,
		Content:   content,
		Variables: vars,
	}
}

// RenderTemplate substitutes variables into template content.
func RenderTemplate(tpl extensions.PromptTemplate, vars map[string]string) string {
	result := tpl.Content
	for name, value := range vars {
		placeholder := "{{" + name + "}}"
		result = strings.ReplaceAll(result, placeholder, value)
		// Also handle with spaces
		placeholderSpaced := "{{ " + name + " }}"
		result = strings.ReplaceAll(result, placeholderSpaced, value)
	}
	return result
}

// ParseArguments parses command-line style arguments.
func ParseArguments(input string, pattern extensions.ArgumentPattern) extensions.ParseResult {
	result := extensions.ParseResult{
		Vars:  make(map[string]string),
		Flags: make(map[string]string),
	}

	fields := parseFields(input)
	if len(fields) == 0 {
		return result
	}

	// First field is the command itself (if present)
	startIdx := 0
	if len(fields) > 0 && !strings.HasPrefix(fields[0], "-") {
		// Check if it's a command name or positional arg
		if len(pattern.Positional) == 0 || !isFlag(fields[0], pattern.Flags) {
			startIdx = 1 // Skip command name
		}
	}

	// Parse flags
	i := startIdx
	for i < len(fields) {
		field := fields[i]

		// Check for flags
		if strings.HasPrefix(field, "--") {
			flagName := field[2:]
			if varName, ok := pattern.Flags["--"+flagName]; ok {
				// Flag with value
				if i+1 < len(fields) && !strings.HasPrefix(fields[i+1], "-") {
					result.Flags["--"+flagName] = fields[i+1]
					result.Vars[varName] = fields[i+1]
					i += 2
					continue
				}
				// Boolean flag
				result.Flags["--"+flagName] = "true"
				result.Vars[varName] = "true"
			}
			i++
			continue
		}

		if strings.HasPrefix(field, "-") && len(field) > 1 {
			flagName := field[1:]
			if varName, ok := pattern.Flags["-"+flagName]; ok {
				// Flag with value
				if i+1 < len(fields) && !strings.HasPrefix(fields[i+1], "-") {
					result.Flags["-"+flagName] = fields[i+1]
					result.Vars[varName] = fields[i+1]
					i += 2
					continue
				}
				// Boolean flag
				result.Flags["-"+flagName] = "true"
				result.Vars[varName] = "true"
			}
			i++
			continue
		}

		i++
	}

	// Collect remaining as positional args and "rest"
	positional := make([]string, 0)
	i = startIdx
	for i < len(fields) {
		field := fields[i]
		if !strings.HasPrefix(field, "-") {
			// Check if this was consumed as a flag value
			consumed := false
			for _, v := range result.Vars {
				if v == field {
					// Might be consumed, check previous field
					if i > 0 {
						prev := fields[i-1]
						if strings.HasPrefix(prev, "-") {
							consumed = true
							break
						}
					}
				}
			}
			if !consumed {
				positional = append(positional, field)
			}
		}
		i++
	}

	// Map positional args
	for i, name := range pattern.Positional {
		if i < len(positional) {
			result.Vars[name] = positional[i]
		}
	}

	// Set rest
	if pattern.Rest != "" && len(positional) > len(pattern.Positional) {
		restStart := len(pattern.Positional)
		if restStart < len(positional) {
			result.Vars[pattern.Rest] = strings.Join(positional[restStart:], " ")
		}
	}

	result.Rest = strings.Join(fields, " ")
	return result
}

// SimpleParseArguments parses $1, $2, $@ style arguments.
// Returns slice where [0]=full input, [1]=$1, [2]=$2, ... [n]=$@
func SimpleParseArguments(input string, count int) []string {
	fields := parseFields(input)
	result := make([]string, 0, count+2)
	result = append(result, input) // [0] = full input

	// [1]..[count] = positional args
	for i := 0; i < count; i++ {
		if i < len(fields) {
			result = append(result, fields[i])
		} else {
			result = append(result, "")
		}
	}

	// [n] = $@ (all remaining)
	if len(fields) > count {
		result = append(result, strings.Join(fields[count:], " "))
	} else {
		result = append(result, "")
	}

	return result
}

// parseFields splits input respecting quoted strings.
func parseFields(input string) []string {
	var fields []string
	var current strings.Builder
	inQuote := false
	quoteChar := rune(0)

	for _, r := range input {
		switch r {
		case '"', '\'':
			if !inQuote {
				inQuote = true
				quoteChar = r
			} else if r == quoteChar {
				inQuote = false
				quoteChar = 0
			} else {
				current.WriteRune(r)
			}
		case ' ', '\t':
			if inQuote {
				current.WriteRune(r)
			} else {
				if current.Len() > 0 {
					fields = append(fields, current.String())
					current.Reset()
				}
			}
		default:
			current.WriteRune(r)
		}
	}

	if current.Len() > 0 {
		fields = append(fields, current.String())
	}

	return fields
}

// isFlag checks if a field is a known flag.
func isFlag(field string, flags map[string]string) bool {
	if strings.HasPrefix(field, "--") {
		return true
	}
	if strings.HasPrefix(field, "-") && len(field) > 1 {
		return true
	}
	return false
}

// EvaluateModelConditional checks if condition matches current model.
// Condition supports wildcards: * matches any, ? matches single char.
func EvaluateModelConditional(currentModel, condition string) bool {
	// Handle comma-separated conditions (OR logic)
	for _, c := range strings.Split(condition, ",") {
		c = strings.TrimSpace(c)
		if matchModelPattern(currentModel, c) {
			return true
		}
	}
	return false
}

// matchModelPattern matches a model against a pattern with wildcards.
func matchModelPattern(model, pattern string) bool {
	// Convert pattern to regexp
	pattern = strings.ReplaceAll(pattern, "*", ".*")
	pattern = strings.ReplaceAll(pattern, "?", ".")
	pattern = "^" + pattern + "$"

	re, err := regexp.Compile(pattern)
	if err != nil {
		// Fallback: exact match
		return model == pattern
	}
	return re.MatchString(model)
}

// RenderWithModelConditionals processes <if-model> blocks in content.
func RenderWithModelConditionals(content, currentModel string) string {
	// Simple regex-based processor for <if-model> blocks
	// Supports: <if-model is="pattern">content</if-model>
	// And: <if-model is="pattern">content<else>other</if-model>

	result := content

	// Pattern for if-model blocks
	ifModelRegex := regexp.MustCompile(`(?s)<if-model\s+is="([^"]+)">(.*?)(?:<else>(.*?))?</if-model>`)

	for {
		match := ifModelRegex.FindStringSubmatchIndex(result)
		if match == nil {
			break
		}

		condition := result[match[2]:match[3]]
		ifContent := result[match[4]:match[5]]
		elseContent := ""
		if match[6] >= 0 && match[7] >= 0 {
			elseContent = result[match[6]:match[7]]
		}

		var replacement string
		if EvaluateModelConditional(currentModel, condition) {
			replacement = ifContent
		} else {
			replacement = elseContent
		}

		result = result[:match[0]] + replacement + result[match[1]:]
	}

	return result
}

// ---------------------------------------------------------------------------
// Model Resolution Bridge for Extensions (Phase 4)
// ---------------------------------------------------------------------------

// ResolveModelChain attempts each model in order until one is available.
func ResolveModelChain(preferences []string) extensions.ModelResolutionResult {
	result := extensions.ModelResolutionResult{
		Attempted: make([]string, 0, len(preferences)),
	}

	registry := models.GetGlobalRegistry()

	for _, pref := range preferences {
		pref = strings.TrimSpace(pref)
		result.Attempted = append(result.Attempted, pref)

		// Parse model string
		provider, modelID, err := models.ParseModelString(pref)
		if err != nil {
			continue
		}

		// Check if provider exists
		if registry.GetProviderInfo(provider) == nil {
			continue
		}

		// Check if model exists in registry
		modelInfo := registry.LookupModel(provider, modelID)
		if modelInfo == nil {
			// Try with just the model as bare name
			continue
		}

		// Found available model
		result.Model = provider + "/" + modelID
		result.Capabilities = extensions.ModelCapabilities{
			Provider:     provider,
			ModelID:      modelID,
			ContextLimit: modelInfo.Limit.Context,
			OutputLimit:  modelInfo.Limit.Output,
			Reasoning:    modelInfo.Reasoning,
			Streaming:    true, // Assume streaming support
		}
		return result
	}

	result.Error = "no models in chain are available"
	return result
}

// GetModelCapabilities returns capabilities for a specific model.
// If model is empty, returns zero capabilities.
func GetModelCapabilities(model string) (extensions.ModelCapabilities, string) {
	if model == "" {
		return extensions.ModelCapabilities{}, "no model specified"
	}

	provider, modelID, err := models.ParseModelString(model)
	if err != nil {
		return extensions.ModelCapabilities{}, err.Error()
	}

	registry := models.GetGlobalRegistry()
	modelInfo := registry.LookupModel(provider, modelID)
	if modelInfo == nil {
		return extensions.ModelCapabilities{}, "model not found in registry"
	}

	return extensions.ModelCapabilities{
		Provider:     provider,
		ModelID:      modelID,
		ContextLimit: modelInfo.Limit.Context,
		OutputLimit:  modelInfo.Limit.Output,
		Reasoning:    modelInfo.Reasoning,
		Streaming:    true,
	}, ""
}

// CheckModelAvailable verifies if a model string is valid and provider exists.
func CheckModelAvailable(model string) bool {
	provider, _, err := models.ParseModelString(model)
	if err != nil {
		return false
	}

	registry := models.GetGlobalRegistry()
	if registry.GetProviderInfo(provider) == nil {
		return false
	}

	// Model doesn't need to be in registry - could be dynamic/Ollama
	return true
}

// GetCurrentProvider extracts provider from model string.
func GetCurrentProvider(model string) string {
	provider, _, _ := models.ParseModelString(model)
	return provider
}

// GetCurrentModelID extracts model ID from model string.
func GetCurrentModelID(model string) string {
	_, modelID, _ := models.ParseModelString(model)
	return modelID
}

// JoinModel combines provider and model ID into a model string.
func JoinModel(provider, modelID string) string {
	if provider == "" {
		return modelID
	}
	return provider + "/" + modelID
}

// MatchModelGlob matches a model against a glob pattern.
// Pattern can contain * (match any) and ? (match single).
func MatchModelGlob(model, pattern string) bool {
	return matchModelPattern(model, pattern)
}

// ExtractProviderFromPath extracts provider from a path-like model string.
func ExtractProviderFromPath(model string) string {
	parts := strings.Split(model, "/")
	if len(parts) >= 2 {
		return parts[0]
	}
	return ""
}

// ExtractModelFromPath extracts model ID from a path-like model string.
func ExtractModelFromPath(model string) string {
	parts := strings.Split(model, "/")
	if len(parts) >= 2 {
		return parts[1]
	}
	return model
}

// IsBareModelID checks if a string is a bare model ID (no provider).
func IsBareModelID(model string) bool {
	return !strings.Contains(model, "/")
}

// AddProviderToModel adds a provider prefix to a bare model ID.
func AddProviderToModel(provider, model string) string {
	if strings.Contains(model, "/") {
		return model // Already has provider
	}
	return provider + "/" + model
}

// RemoveProviderFromModel removes the provider prefix from a model string.
func RemoveProviderFromModel(model string) string {
	parts := strings.SplitN(model, "/", 2)
	if len(parts) == 2 {
		return parts[1]
	}
	return model
}

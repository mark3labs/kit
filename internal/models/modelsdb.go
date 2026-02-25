package models

// ModelsDBProviders is the top-level type for models.dev/api.json data:
// a map of provider ID → provider object.
type ModelsDBProviders = map[string]modelsDBProvider

// modelsDBProvider represents a provider entry from models.dev/api.json.
type modelsDBProvider struct {
	ID     string                   `json:"id"`
	Env    []string                 `json:"env"`
	NPM    string                   `json:"npm"`
	API    string                   `json:"api,omitempty"`
	Name   string                   `json:"name"`
	Doc    string                   `json:"doc,omitempty"`
	Models map[string]modelsDBModel `json:"models"`
}

// modelsDBModel represents a model entry from models.dev/api.json.
type modelsDBModel struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Family      string        `json:"family,omitempty"`
	Attachment  bool          `json:"attachment"`
	Reasoning   bool          `json:"reasoning"`
	ToolCall    bool          `json:"tool_call"`
	Temperature bool          `json:"temperature"`
	Cost        modelsDBCost  `json:"cost"`
	Limit       modelsDBLimit `json:"limit"`
}

// modelsDBCost represents model pricing from models.dev.
type modelsDBCost struct {
	Input      float64  `json:"input"`
	Output     float64  `json:"output"`
	CacheRead  *float64 `json:"cache_read,omitempty"`
	CacheWrite *float64 `json:"cache_write,omitempty"`
}

// modelsDBLimit represents model context/output limits from models.dev.
type modelsDBLimit struct {
	Context int `json:"context"`
	Output  int `json:"output"`
}

// npmToFantasyProvider maps npm package names from models.dev to fantasy
// provider identifiers. Providers not in this map but with an api URL
// can be auto-routed through openaicompat.
var npmToFantasyProvider = map[string]string{
	"@ai-sdk/anthropic":               "anthropic",
	"@ai-sdk/openai":                  "openai",
	"@ai-sdk/google":                  "google",
	"@ai-sdk/google-vertex":           "google-vertex",
	"@ai-sdk/google-vertex/anthropic": "google-vertex-anthropic",
	"@ai-sdk/amazon-bedrock":          "bedrock",
	"@ai-sdk/azure":                   "azure",
	"@openrouter/ai-sdk-provider":     "openrouter",
	"@ai-sdk/vercel":                  "vercel",
	"@ai-sdk/openai-compatible":       "openaicompat",
}

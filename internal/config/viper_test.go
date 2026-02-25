package config

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

func TestViperEnvironmentFieldParsing(t *testing.T) {
	// Test YAML content with environment field
	yamlContent := `
mcpServers:
  test:
    type: local
    command: ["echo", "test"]
    environment:
      KEY1: "value1"
      KEY2: "value2"
      EMPTY_KEY: ""
`

	// Test 1: Direct YAML parsing
	t.Run("DirectYAMLParsing", func(t *testing.T) {
		var yamlData map[string]any
		err := yaml.Unmarshal([]byte(yamlContent), &yamlData)
		if err != nil {
			t.Fatalf("YAML unmarshal error: %v", err)
		}

		servers := yamlData["mcpServers"].(map[string]any)
		testServer := servers["test"].(map[string]any)
		env := testServer["environment"].(map[string]any)

		if env["KEY1"] != "value1" {
			t.Errorf("Expected KEY1=value1, got %v", env["KEY1"])
		}
		if env["EMPTY_KEY"] != "" {
			t.Errorf("Expected EMPTY_KEY='', got %v", env["EMPTY_KEY"])
		}
	})

	// Test 2: Viper parsing with LoadAndValidateConfig
	t.Run("ViperParsing", func(t *testing.T) {
		viper.Reset()
		viper.SetConfigType("yaml")
		err := viper.ReadConfig(strings.NewReader(yamlContent))
		if err != nil {
			t.Fatalf("Viper read error: %v", err)
		}

		// Get all settings to debug
		allSettings := viper.AllSettings()
		jsonBytes, _ := json.MarshalIndent(allSettings, "", "  ")
		t.Logf("Viper AllSettings (before case fix):\n%s", jsonBytes)

		// Use LoadAndValidateConfig which includes the case fix
		config, err := LoadAndValidateConfig()
		if err != nil {
			t.Fatalf("LoadAndValidateConfig error: %v", err)
		}

		testServer, exists := config.MCPServers["test"]
		if !exists {
			t.Fatal("test server not found")
		}

		// Log the environment map after case fix
		t.Logf("Environment map (after case fix): %+v", testServer.Environment)

		// Check environment values - they should be uppercase now
		if testServer.Environment["KEY1"] != "value1" {
			t.Errorf("Expected KEY1=value1, got %s", testServer.Environment["KEY1"])
		}
		if testServer.Environment["KEY2"] != "value2" {
			t.Errorf("Expected KEY2=value2, got %s", testServer.Environment["KEY2"])
		}
		if val, exists := testServer.Environment["EMPTY_KEY"]; !exists {
			t.Error("EMPTY_KEY not found in environment map")
		} else if val != "" {
			t.Errorf("Expected EMPTY_KEY='', got '%s'", val)
		}
	})
	// Test 3: Check case sensitivity
	t.Run("CaseSensitivity", func(t *testing.T) {
		viper.Reset()
		viper.SetConfigType("yaml")
		err := viper.ReadConfig(strings.NewReader(yamlContent))
		if err != nil {
			t.Fatalf("Viper read error: %v", err)
		}

		var config Config
		err = viper.Unmarshal(&config)
		if err != nil {
			t.Fatalf("Unmarshal error: %v", err)
		}

		testServer := config.MCPServers["test"]

		// Check if keys are lowercase
		hasUppercase := false
		for key := range testServer.Environment {
			if key != strings.ToLower(key) {
				hasUppercase = true
				t.Logf("Found uppercase key: %s", key)
			}
		}

		if hasUppercase {
			t.Log("WARNING: Viper preserved uppercase keys in environment map")
		} else {
			t.Log("All keys are lowercase in environment map")
		}
	})
}

package config

import (
	"encoding/json"
	"testing"
)

func TestCodeModeEnabledRoundTrip(t *testing.T) {
	for _, input := range []string{`{}`, `{"enabled":false}`, `{"enabled":true}`} {
		var cfg CodeModeConfig
		if err := json.Unmarshal([]byte(input), &cfg); err != nil {
			t.Fatal(err)
		}
		data, err := json.Marshal(cfg)
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != input {
			t.Fatalf("round trip %s = %s", input, data)
		}
	}
}

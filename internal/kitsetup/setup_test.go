package kitsetup

import (
	"fmt"
	"testing"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// formatInt32Ptr returns the value that p points to, or "nil" if p is nil.
func formatInt32Ptr(p *int32) string {
	if p == nil {
		return "nil"
	}
	return fmt.Sprint(*p)
}

// TestBuildProviderConfigGPUFlags binds the Ollama GPU flags with the CLI
// defaults from cmd/root.go. A flag the user did not give must leave its
// option unset. Otherwise Kit sends main_gpu with the request that loads
// the model (issue #265).
func TestBuildProviderConfigGPUFlags(t *testing.T) {
	for _, tc := range []struct {
		name            string
		args            []string
		numGPU, mainGPU *int32
	}{
		{name: "flags not given"},
		{name: "main-gpu 0 given", args: []string{"--main-gpu=0"}, mainGPU: new(int32(0))},
		{name: "both given", args: []string{"--num-gpu-layers=20", "--main-gpu=1"}, numGPU: new(int32(20)), mainGPU: new(int32(1))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := viper.New()
			fs := pflag.NewFlagSet("kit", pflag.ContinueOnError)
			fs.Int32("num-gpu-layers", -1, "")
			fs.Int32("main-gpu", 0, "")
			for _, name := range []string{"num-gpu-layers", "main-gpu"} {
				if err := v.BindPFlag(name, fs.Lookup(name)); err != nil {
					t.Fatalf("BindPFlag(%s): %v", name, err)
				}
			}
			if err := fs.Parse(tc.args); err != nil {
				t.Fatalf("Parse: %v", err)
			}

			cfg, _, err := BuildProviderConfig(v)
			if err != nil {
				t.Fatalf("BuildProviderConfig: %v", err)
			}
			if got, want := formatInt32Ptr(cfg.NumGPU), formatInt32Ptr(tc.numGPU); got != want {
				t.Errorf("NumGPU = %s, want %s", got, want)
			}
			if got, want := formatInt32Ptr(cfg.MainGPU), formatInt32Ptr(tc.mainGPU); got != want {
				t.Errorf("MainGPU = %s, want %s", got, want)
			}
		})
	}
}

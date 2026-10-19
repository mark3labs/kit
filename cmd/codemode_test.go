package cmd

import (
	"testing"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

func TestCodeModeFlagDefaultAndOverride(t *testing.T) {
	flag := rootCmd.PersistentFlags().Lookup("codemode")
	if flag == nil || flag.DefValue != "true" {
		t.Fatal("codemode flag must default to true")
	}
	for _, args := range [][]string{nil, {"--codemode=false"}, {"--codemode"}} {
		flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
		flags.Bool("codemode", flag.DefValue == "true", flag.Usage)
		v := viper.New()
		if err := v.BindPFlag("codemode.enabled", flags.Lookup("codemode")); err != nil {
			t.Fatal(err)
		}
		if err := flags.Parse(args); err != nil {
			t.Fatal(err)
		}
		want := len(args) == 0 || args[0] != "--codemode=false"
		if got := v.GetBool("codemode.enabled"); got != want {
			t.Fatalf("args %v: enabled = %v", args, got)
		}
		if len(args) == 0 {
			v.Set("codemode.enabled", false)
			if v.GetBool("codemode.enabled") {
				t.Fatal("config false must override flag default")
			}
		}
	}
}

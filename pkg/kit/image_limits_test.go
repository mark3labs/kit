package kit

import (
	"testing"

	"github.com/spf13/viper"
)

// TestResolveImageLimitsDefaults checks that an unconfigured Kit leaves the
// built-in image budget in place, signalled by zero values.
func TestResolveImageLimitsDefaults(t *testing.T) {
	edge, bytes, noResize := resolveImageLimits(&Options{}, viper.New())
	if edge != 0 || bytes != 0 || noResize {
		t.Errorf("got (%d, %d, %v), want (0, 0, false)", edge, bytes, noResize)
	}
}

// TestResolveImageLimitsFromConfig checks the config-file source.
func TestResolveImageLimitsFromConfig(t *testing.T) {
	v := fileStore(t, "image-max-edge: 800\nimage-max-bytes: 200000\nimage-no-resize: true\n")
	edge, bytes, noResize := resolveImageLimits(&Options{}, v)
	if edge != 800 {
		t.Errorf("maxEdge = %d, want 800", edge)
	}
	if bytes != 200000 {
		t.Errorf("maxBytes = %d, want 200000", bytes)
	}
	if !noResize {
		t.Error("noResize = false, want true")
	}
}

// TestResolveImageLimitsOptionWins checks that an explicit option overrides
// the config file.
func TestResolveImageLimitsOptionWins(t *testing.T) {
	v := fileStore(t, "image-max-edge: 800\nimage-max-bytes: 200000\n")
	edge, bytes, _ := resolveImageLimits(&Options{ImageMaxEdge: 256, ImageMaxBytes: 1024}, v)
	if edge != 256 {
		t.Errorf("maxEdge = %d, want 256", edge)
	}
	if bytes != 1024 {
		t.Errorf("maxBytes = %d, want 1024", bytes)
	}
}

// TestResolveImageLimitsFromEnv checks the environment source and its
// precedence over the config file.
func TestResolveImageLimitsFromEnv(t *testing.T) {
	t.Setenv("KIT_IMAGE_MAX_EDGE", "512")
	t.Setenv("KIT_IMAGE_NO_RESIZE", "true")
	v := fileStore(t, "image-max-edge: 800\n")
	edge, _, noResize := resolveImageLimits(&Options{}, v)
	if edge != 512 {
		t.Errorf("maxEdge = %d, want 512", edge)
	}
	if !noResize {
		t.Error("noResize = false, want true")
	}
}

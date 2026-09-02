package kit

import (
	"strings"
	"testing"
)

// A subagent created without an explicit tool set runs its commands through
// the parent's configured shell; a parent on an image without bash must not
// spawn children that cannot execute anything.
func TestSubagentDefaultTools_InheritTheParentShell(t *testing.T) {
	m := &Kit{shell: []string{"busybox", "ash"}}
	for _, tool := range m.subagentDefaultTools() {
		if tool.Info().Name != "shell" {
			continue
		}
		if d := tool.Info().Description; !strings.Contains(d, "busybox ash") {
			t.Errorf("subagent shell tool does not name the parent's shell: %q", d)
		}
		return
	}
	t.Fatal("subagent default tools contain no shell tool")
}

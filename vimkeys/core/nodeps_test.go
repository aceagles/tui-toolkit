package core_test

import (
	"os/exec"
	"strings"
	"testing"
)

// TestNoFrameworkDeps guards vimkeys/core's framework-free promise: a
// future edit that reaches for tview, tcell or bubbletea in this package
// would silently couple the chord state machine back to one UI framework,
// which defeats the reason it was split out of vimkeys in the first place.
// `go list -deps` is the ground truth for what actually got imported
// (transitively, not just what the current file happens to mention), so
// this fails loudly the moment such an import lands instead of waiting for
// someone to notice in review.
func TestNoFrameworkDeps(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps", ".").CombinedOutput()
	if err != nil {
		t.Fatalf("go list -deps .: %v\n%s", err, out)
	}
	for _, forbidden := range []string{
		"github.com/rivo/tview",
		"github.com/gdamore/tcell",
		"charm.land/bubbletea",
		"charm.land/lipgloss",
	} {
		if strings.Contains(string(out), forbidden) {
			t.Fatalf("vimkeys/core must stay framework-free; found %q in its dependency list:\n%s", forbidden, out)
		}
	}
}

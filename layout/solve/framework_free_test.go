package solve

import (
	"os/exec"
	"strings"
	"testing"
)

// TestNoTviewOrTcellDependency locks the framework-free contract described
// in the package doc: this package (and everything it imports) must never
// pull in tview or tcell, so a caller that only wants Solve — such as a
// bubbletea tool — is not forced to pull in a terminal UI framework it does
// not use. If this test fails, something added to this package started
// depending on tview/tcell, directly or transitively; move that code back
// into layout instead.
func TestNoTviewOrTcellDependency(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps", ".").CombinedOutput()
	if err != nil {
		t.Fatalf("go list -deps .: %v\n%s", err, out)
	}
	for _, forbidden := range []string{"github.com/rivo/tview", "github.com/gdamore/tcell/v2"} {
		if strings.Contains(string(out), forbidden) {
			t.Errorf("package solve depends on %s; it must stay framework-free", forbidden)
		}
	}
}

package tmuxnav

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// grid builds panes at the given (col,row) positions.
func grid(coords ...[2]int) []pane {
	ps := make([]pane, len(coords))
	for i, c := range coords {
		ps[i] = pane{p: tview.NewBox(), col: c[0], row: c[1]}
	}
	return ps
}

func TestResolve(t *testing.T) {
	// Layout: readout (0,0) | commands (1,0), log spanning below at (0,1).
	panes := grid([2]int{0, 0}, [2]int{1, 0}, [2]int{0, 1})

	tests := []struct {
		name string
		cur  int
		dir  Direction
		want int
		ok   bool
	}{
		{"right from readout", 0, Right, 1, true},
		{"left from commands", 1, Left, 0, true},
		{"down from readout", 0, Down, 2, true},
		{"down from commands picks nearest cross-axis", 1, Down, 2, true},
		{"up from log picks same column", 2, Up, 0, true},
		{"left edge", 0, Left, 0, false},
		{"right edge from log", 2, Right, 1, true}, // (1,0) is strictly right
		{"up edge", 0, Up, 0, false},
		{"unknown focus adopts first pane", -1, Right, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := resolve(panes, tt.cur, tt.dir)
			if ok != tt.ok || (ok && got != tt.want) {
				t.Fatalf("resolve(cur=%d, dir=%v) = %d,%v; want %d,%v", tt.cur, tt.dir, got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestResolveEmpty(t *testing.T) {
	if _, ok := resolve(nil, 0, Left); ok {
		t.Fatal("resolve on no panes must not report a target")
	}
}

func TestInputCaptureMovesFocusAndForwardsAtEdge(t *testing.T) {
	panes := grid([2]int{0, 0}, [2]int{1, 0})
	var focused tview.Primitive = panes[0].p
	var forwarded []Direction
	n := &Navigator{
		panes:    panes,
		focus:    func() tview.Primitive { return focused },
		setFocus: func(p tview.Primitive) { focused = p },
		forward:  func(dir Direction) { forwarded = append(forwarded, dir) },
	}

	// C-l moves right within the TUI.
	if ev := n.InputCapture(tcell.NewEventKey(tcell.KeyCtrlL, 0, tcell.ModCtrl)); ev != nil {
		t.Fatal("C-l should be consumed")
	}
	if focused != panes[1].p {
		t.Fatal("C-l did not move focus to the right pane")
	}

	// C-l again is the edge: forwarded to tmux, focus unchanged.
	n.InputCapture(tcell.NewEventKey(tcell.KeyCtrlL, 0, tcell.ModCtrl))
	if focused != panes[1].p || len(forwarded) != 1 || forwarded[0] != Right {
		t.Fatalf("edge press: focused=%v forwarded=%v", focused, forwarded)
	}

	// Unrelated keys pass through untouched.
	ev := tcell.NewEventKey(tcell.KeyRune, 'j', tcell.ModNone)
	if got := n.InputCapture(ev); got != ev {
		t.Fatal("non-navigation key must pass through")
	}
}

func TestInputCaptureEdgeOutsideTmux(t *testing.T) {
	panes := grid([2]int{0, 0})
	n := &Navigator{
		panes:    panes,
		focus:    func() tview.Primitive { return panes[0].p },
		setFocus: func(tview.Primitive) {},
		forward:  nil, // outside tmux
	}
	if ev := n.InputCapture(tcell.NewEventKey(tcell.KeyCtrlH, 0, tcell.ModCtrl)); ev != nil {
		t.Fatal("edge press outside tmux should still be consumed")
	}
}

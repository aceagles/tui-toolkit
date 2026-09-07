// Package tmuxnav gives a tview application vim-tmux-navigator semantics:
// C-h/j/k/l moves focus between registered panes, and when there is no pane
// in the pressed direction the keypress is forwarded to `tmux select-pane`,
// so one key grammar walks TUI panes, tmux panes and nvim splits.
//
// The tmux side must recognise the binary as vim-like, or tmux swallows the
// keys before they reach the application: name the binary `<something>tui`
// and widen the is_vim pattern in tmux.conf to match `\S+tui`.
package tmuxnav

import (
	"os"
	"os/exec"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// Direction is a pane-navigation direction.
type Direction int

const (
	Left Direction = iota
	Down
	Up
	Right
)

// tmuxFlag maps a direction to the select-pane flag.
var tmuxFlag = map[Direction]string{Left: "-L", Down: "-D", Up: "-U", Right: "-R"}

type pane struct {
	p        tview.Primitive
	col, row int
}

// Navigator routes C-h/j/k/l between registered panes and forwards
// edge presses to tmux. Wire InputCapture into the application:
//
//	nav := tmuxnav.New(app)
//	nav.Add(left, 0, 0)
//	nav.Add(right, 1, 0)
//	app.SetInputCapture(nav.InputCapture)
type Navigator struct {
	panes []pane

	// focus and setFocus wrap the application; tests inject fakes.
	focus    func() tview.Primitive
	setFocus func(tview.Primitive)
	// forward sends an edge press on to tmux; nil outside tmux.
	forward func(dir Direction)
}

// New builds a Navigator bound to app. Outside tmux ($TMUX unset) edge
// presses are consumed silently instead of forwarded.
func New(app *tview.Application) *Navigator {
	n := &Navigator{
		focus:    app.GetFocus,
		setFocus: func(p tview.Primitive) { app.SetFocus(p) },
	}
	if os.Getenv("TMUX") != "" {
		n.forward = func(dir Direction) {
			// Fire and forget: if tmux is gone there is nothing useful to do.
			_ = exec.Command("tmux", "select-pane", tmuxFlag[dir]).Run()
		}
	}
	return n
}

// Add registers a focusable pane at a grid position. Coordinates are a
// logical grid, not screen cells: a classic two-column layout with a full-
// width log underneath is (0,0) (1,0) (0,1)+(1,1) or simply (0,1).
// Register the primitive that actually takes focus.
func (n *Navigator) Add(p tview.Primitive, col, row int) {
	n.panes = append(n.panes, pane{p: p, col: col, row: row})
}

// InputCapture translates C-h/j/k/l. Everything else passes through.
//
// Note C-h is byte 0x08, which some terminals also send for Backspace;
// most send DEL (0x7f) for the key itself, so text entry keeps working.
func (n *Navigator) InputCapture(ev *tcell.EventKey) *tcell.EventKey {
	var dir Direction
	switch ev.Key() {
	case tcell.KeyCtrlH:
		dir = Left
	case tcell.KeyCtrlJ:
		dir = Down
	case tcell.KeyCtrlK:
		dir = Up
	case tcell.KeyCtrlL:
		dir = Right
	default:
		return ev
	}

	cur := -1
	if f := n.focus(); f != nil {
		for i, p := range n.panes {
			if p.p == f {
				cur = i
				break
			}
		}
	}
	if target, ok := resolve(n.panes, cur, dir); ok {
		n.setFocus(n.panes[target].p)
	} else if n.forward != nil {
		n.forward(dir)
	}
	return nil
}

// resolve picks the pane to focus from cur in dir: the nearest column (or
// row) strictly in that direction, ties broken by cross-axis distance.
// ok is false at an edge, or when cur is unknown and there is no pane to
// adopt. With focus outside the registered set (cur == -1) it falls back
// to the first pane so a stray focus state cannot strand navigation.
func resolve(panes []pane, cur int, dir Direction) (int, bool) {
	if len(panes) == 0 {
		return 0, false
	}
	if cur < 0 || cur >= len(panes) {
		return 0, true
	}
	c := panes[cur]
	best, bestOK := 0, false
	for i, p := range panes {
		if i == cur {
			continue
		}
		var ahead bool
		var mainDist, crossDist int
		switch dir {
		case Left:
			ahead, mainDist, crossDist = p.col < c.col, c.col-p.col, abs(p.row-c.row)
		case Right:
			ahead, mainDist, crossDist = p.col > c.col, p.col-c.col, abs(p.row-c.row)
		case Up:
			ahead, mainDist, crossDist = p.row < c.row, c.row-p.row, abs(p.col-c.col)
		case Down:
			ahead, mainDist, crossDist = p.row > c.row, p.row-c.row, abs(p.col-c.col)
		}
		if !ahead {
			continue
		}
		if !bestOK || less(mainDist, crossDist, panes, best, c, dir) {
			best, bestOK = i, true
		}
	}
	return best, bestOK
}

// less reports whether (mainDist, crossDist) beats the current best.
func less(mainDist, crossDist int, panes []pane, best int, c pane, dir Direction) bool {
	b := panes[best]
	var bMain, bCross int
	switch dir {
	case Left:
		bMain, bCross = c.col-b.col, abs(b.row-c.row)
	case Right:
		bMain, bCross = b.col-c.col, abs(b.row-c.row)
	case Up:
		bMain, bCross = c.row-b.row, abs(b.col-c.col)
	case Down:
		bMain, bCross = b.row-c.row, abs(b.col-c.col)
	}
	if mainDist != bMain {
		return mainDist < bMain
	}
	return crossDist < bCross
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

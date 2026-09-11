// Package layout sizes a workbench TUI's panes from what they need instead
// of hard-coded widths, and gives the operator nvim/tmux keys to resize and
// zoom them.
//
// tview's Flex has fixed or proportional sizes only: fixed items never
// shrink, so proportional panes get squashed to nothing on a narrow
// terminal. Here each pane declares a Spec {Min, Pref, Max, Weight} and a
// Stack re-solves the sizes from its rect on every frame (see Solve). A
// pane that cannot have its Min is hidden, rightmost first, and comes back
// as the terminal widens; the stack counts them for a "+2 hidden" badge.
//
// Layout adds the keys, on top of a Stack tree:
//
//	C-w H/J/K/L   move a border 5 cells and enter resize mode, where
//	              H/J/K/L keep moving it; any other key leaves the mode and
//	              is then handled as normal; Esc leaves silently
//	Alt-h/j/k/l   move a border 1 cell, no mode
//	C-w =         equalise: drop every resize
//	C-w z, C-w o  zoom the focused pane to the whole area; again or Esc
//	              restores the layout and focus
//
// A key names the direction the border moves, as in tmux: the focused
// pane's right (bottom) border when it has a neighbour there, else its left
// (top) border. The border is found in the innermost stack along that axis
// where the focused pane has a neighbour.
//
// Wiring, with header and hints outside the zoomable area:
//
//	body := layout.NewStack(layout.Horizontal)
//	body.Add(list, layout.Spec{Min: 24, Pref: 30, Priority: 1})
//	body.Add(detail, layout.Spec{Min: 40, Weight: 1})
//	area := layout.NewStack(layout.Vertical)
//	area.Add(body, layout.Spec{Min: 5, Weight: 1, Priority: 1})
//	area.Add(logView, layout.Spec{Min: 5, Pref: 9})
//	root := layout.NewStack(layout.Vertical)
//	root.Add(header, layout.Fixed(1))
//	root.Add(area, layout.Spec{Weight: 1})
//
//	lay := layout.New(app, area)
//	lay.SkipWhen = func() bool {
//		_, chord := leader.Pending()
//		return chord || modalOpen() || layout.TextFocus(app)
//	}
//	lay.OnState = func() { rebuildNav(); drawHeader() } // header shows lay.Status()
//	leader.MustRegister("z", "[z]oom pane", lay.ToggleZoom)
//	app.SetInputCapture(vimkeys.Chain(lay.InputCapture, leader.InputCapture, nav.InputCapture))
//
// Put Layout first in the chain, so a key that ends resize mode reaches the
// leader and tmuxnav exactly as if the mode had never been on. Register
// only panes for which lay.Visible is true with tmuxnav, and rebuild on
// OnState and on each stack's SetHiddenFunc.
package layout

import (
	"github.com/aceagles/tui-toolkit/theme"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// DefaultStep is how far C-w H/J/K/L moves a border, the step common nvim
// and tmux resize bindings use.
const DefaultStep = 5

// Direction is the way a border moves.
type Direction int

const (
	Left Direction = iota
	Down
	Up
	Right
)

// Mode is Layout's key state.
type Mode int

const (
	ModeNormal Mode = iota
	ModePrefix      // C-w pressed, waiting for the command key
	ModeResize      // H/J/K/L repeat without C-w
)

// Layout handles the resize and zoom keys for the Stack tree under area,
// the part of the screen a zoomed pane fills.
type Layout struct {
	area *Stack
	mode Mode
	// focus from before zooming, restored after.
	zoomFocus tview.Primitive

	// Step is the C-w H/J/K/L border move in cells. Zero means DefaultStep.
	Step int

	// SkipWhen reports that C-w and the Alt nudges should reach the focused
	// widget instead: tview's InputField deletes a word on C-w. The default
	// is TextFocus. Tools compose it with a modal check and a pending leader
	// chord (see the package doc). It gates entering the keys only; once in
	// resize mode, the next key always leaves it.
	SkipWhen func() bool

	// OnState, when set, is called on the event loop after the mode, zoom or
	// offsets change, so the tool can redraw its header and rebuild tmuxnav.
	OnState func()

	// focus and setFocus wrap the application; tests may inject fakes.
	focus    func() tview.Primitive
	setFocus func(tview.Primitive)
}

// New builds a Layout for app over area.
func New(app *tview.Application, area *Stack) *Layout {
	l := &Layout{
		area:     area,
		focus:    app.GetFocus,
		setFocus: func(p tview.Primitive) { app.SetFocus(p) },
	}
	l.SkipWhen = func() bool { return TextFocus(app) }
	return l
}

// TextFocus reports that a text-entry widget has focus: tview's InputField
// and TextArea, which use C-w and Alt keys for editing.
func TextFocus(app *tview.Application) bool {
	switch app.GetFocus().(type) {
	case *tview.InputField, *tview.TextArea:
		return true
	}
	return false
}

// Mode reports the key state.
func (l *Layout) Mode() Mode { return l.mode }

// InputCapture handles the resize and zoom keys. Everything else passes
// through, including the key that ends resize mode.
func (l *Layout) InputCapture(ev *tcell.EventKey) *tcell.EventKey {
	switch l.mode {
	case ModeResize:
		if dir, ok := resizeKey(ev); ok {
			l.Move(dir, l.step())
			return nil
		}
		l.setMode(ModeNormal)
		if ev.Key() == tcell.KeyEscape {
			return nil
		}
		// Returned, not re-examined: a C-w that ends the mode goes on to
		// the widget and cannot start a new prefix.
		return ev
	case ModePrefix:
		l.setMode(ModeNormal)
		if dir, ok := resizeKey(ev); ok {
			l.Move(dir, l.step())
			l.setMode(ModeResize)
			return nil
		}
		if ev.Key() == tcell.KeyRune {
			switch ev.Rune() {
			case '=':
				l.Equalise()
			case 'z', 'o':
				l.ToggleZoom()
			}
		}
		// Unknown keys after C-w are dropped, as in vim: replaying them
		// could run a tool's action nobody asked for.
		return nil
	}

	if l.SkipWhen != nil && l.SkipWhen() {
		return ev
	}
	switch {
	case ev.Key() == tcell.KeyCtrlW:
		l.setMode(ModePrefix)
		return nil
	case ev.Key() == tcell.KeyEscape && l.Zoomed() != nil:
		l.ToggleZoom()
		return nil
	case ev.Key() == tcell.KeyRune && ev.Modifiers()&tcell.ModAlt != 0 && ev.Modifiers()&tcell.ModCtrl == 0:
		if dir, ok := hjkl[ev.Rune()]; ok {
			l.Move(dir, 1)
			return nil
		}
	}
	return ev
}

var (
	hjkl      = map[rune]Direction{'h': Left, 'j': Down, 'k': Up, 'l': Right}
	shiftHJKL = map[rune]Direction{'H': Left, 'J': Down, 'K': Up, 'L': Right}
)

// resizeKey matches H/J/K/L with no Ctrl or Alt (Shift is implied, and some
// terminals report it).
func resizeKey(ev *tcell.EventKey) (Direction, bool) {
	if ev.Key() != tcell.KeyRune || ev.Modifiers()&(tcell.ModCtrl|tcell.ModAlt) != 0 {
		return 0, false
	}
	dir, ok := shiftHJKL[ev.Rune()]
	return dir, ok
}

func (l *Layout) step() int {
	if l.Step > 0 {
		return l.Step
	}
	return DefaultStep
}

func (l *Layout) setMode(m Mode) {
	if l.mode == m {
		return
	}
	l.mode = m
	l.changed()
}

func (l *Layout) changed() {
	if l.OnState != nil {
		l.OnState()
	}
}

// pathStep is one level of the path from area down to the focused pane.
type pathStep struct {
	s *Stack
	i int // the item holding focus
}

// focusPath walks from area to the innermost stack item that holds focus.
func (l *Layout) focusPath() []pathStep {
	var path []pathStep
	for s := l.area; s != nil; {
		idx := -1
		for i, it := range s.items {
			if it.p != nil && it.p.HasFocus() {
				idx = i
				break
			}
		}
		if idx < 0 {
			break
		}
		path = append(path, pathStep{s, idx})
		s, _ = s.items[idx].p.(*Stack)
	}
	return path
}

// Move moves one of the focused pane's borders cells in dir. It reports
// whether it found a border to move; the sizes change on the next Draw,
// clamped to every pane's Min and Max.
func (l *Layout) Move(dir Direction, cells int) bool {
	axis := Horizontal
	if dir == Up || dir == Down {
		axis = Vertical
	}
	d := cells
	if dir == Left || dir == Up {
		d = -cells
	}
	path := l.focusPath()
	for k := len(path) - 1; k >= 0; k-- {
		st := path[k]
		if st.s.axis != axis {
			continue
		}
		prev, next := -1, -1
		for j := st.i - 1; j >= 0 && prev < 0; j-- {
			if st.s.shownAt(j) {
				prev = j
			}
		}
		for j := st.i + 1; j < len(st.s.items) && next < 0; j++ {
			if st.s.shownAt(j) {
				next = j
			}
		}
		a, b := prev, st.i // the border before the pane
		if next >= 0 {
			a, b = st.i, next // the border after it
		}
		if a < 0 {
			continue // alone along this axis; try the stack outside
		}
		if a >= len(st.s.sizes) || b >= len(st.s.sizes) {
			return false // not drawn yet
		}
		ca := st.s.resizeTo(a, st.s.sizes[a]+d)
		cb := st.s.resizeTo(b, st.s.sizes[b]-d)
		if ca || cb {
			l.changed()
		}
		return true
	}
	return false
}

// Equalise drops every user resize under area.
func (l *Layout) Equalise() {
	l.area.equalise()
	l.changed()
}

// ToggleZoom zooms the pane holding focus to the whole area, or restores
// the layout and the focus from before the zoom. The zoomed pane is the
// innermost stack item holding focus, so a bordered table in a Flex zooms
// with its border.
func (l *Layout) ToggleZoom() {
	if l.Zoomed() != nil {
		l.area.zoom = nil
		if f := l.zoomFocus; f != nil && contains(l.area, f) {
			l.setFocus(f)
		}
		l.zoomFocus = nil
		l.changed()
		return
	}
	path := l.focusPath()
	if len(path) == 0 {
		return
	}
	last := path[len(path)-1]
	l.area.zoom = last.s.items[last.i].p
	l.zoomFocus = l.focus()
	l.changed()
}

// Zoomed returns the zoomed pane, or nil. A zoomed pane removed from the
// tree ends the zoom.
func (l *Layout) Zoomed() tview.Primitive {
	if z := l.area.zoom; z != nil && !contains(l.area, z) {
		l.area.zoom, l.zoomFocus = nil, nil
	}
	return l.area.zoom
}

// Visible reports whether p is on screen as of the last Draw: inside the
// zoomed pane while zoomed, otherwise inside panes every enclosing stack
// showed. Primitives outside area report false. Use it to decide what to
// register with tmuxnav.
func (l *Layout) Visible(p tview.Primitive) bool {
	if z := l.Zoomed(); z != nil {
		return contains(z, p)
	}
	return visibleIn(l.area, p)
}

func visibleIn(s *Stack, p tview.Primitive) bool {
	if s == p {
		return true
	}
	for i, it := range s.items {
		if !contains(it.p, p) {
			continue
		}
		if !s.shownAt(i) {
			return false
		}
		if sub, ok := it.p.(*Stack); ok {
			return visibleIn(sub, p)
		}
		return true
	}
	return false
}

// Status is a short tagged indicator of the mode and zoom for a header,
// in theme colours; "" when there is nothing to show.
func (l *Layout) Status() string {
	p := theme.Current()
	var out string
	switch l.mode {
	case ModePrefix:
		out = p.Tag(theme.RoleAccent) + "[::b]C-w[::-][-]"
	case ModeResize:
		out = p.Tag(theme.RoleWarn) + "[::b]RESIZE[::-] H/J/K/L, Esc[-]"
	}
	if l.Zoomed() != nil {
		if out != "" {
			out += " "
		}
		out += p.Tag(theme.RoleAccent) + "[::b]ZOOM[::-][-]"
	}
	return out
}

package layout

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func key(k tcell.Key) *tcell.EventKey { return tcell.NewEventKey(k, 0, tcell.ModNone) }
func r(ch rune) *tcell.EventKey       { return tcell.NewEventKey(tcell.KeyRune, ch, tcell.ModNone) }
func alt(ch rune) *tcell.EventKey     { return tcell.NewEventKey(tcell.KeyRune, ch, tcell.ModAlt) }

var ctrlW = key(tcell.KeyCtrlW)

// rig is a tool wired the way the package doc says: panes a, b, in, c side
// by side in body, a log below, and Layout as the application input
// capture. Keys that reach a focused Box are recorded in got.
type rig struct {
	t            *testing.T
	app          *tview.Application
	screen       tcell.SimulationScreen
	area, body   *Stack
	a, b, c, log *tview.Box
	in           *tview.InputField
	lay          *Layout
	modal        bool
	got          []string
}

func newRig(t *testing.T) *rig {
	t.Helper()
	g := &rig{t: t, app: tview.NewApplication()}
	pane := func(title string) *tview.Box {
		b := tview.NewBox().SetBorder(true).SetTitle(title)
		b.SetInputCapture(func(ev *tcell.EventKey) *tcell.EventKey {
			g.got = append(g.got, ev.Name())
			return nil
		})
		return b
	}
	g.a, g.b, g.c, g.log = pane("A"), pane("B"), pane("C"), pane("Log")
	g.in = tview.NewInputField()
	g.body = NewStack(Horizontal)
	g.body.Add(g.a, Spec{Min: 10, Pref: 20, Priority: 1})
	g.body.Add(g.b, Spec{Min: 10, Pref: 20})
	g.body.Add(g.in, Spec{Min: 10, Pref: 10})
	g.body.Add(g.c, Spec{Min: 10, Pref: 20})
	g.area = NewStack(Vertical)
	g.area.Add(g.body, Spec{Min: 3, Weight: 1, Priority: 1})
	g.area.Add(g.log, Spec{Min: 3, Pref: 5})

	g.lay = New(g.app, g.area)
	g.lay.SkipWhen = func() bool { return g.modal || TextFocus(g.app) }
	g.app.SetRoot(g.area, true)
	g.app.SetInputCapture(g.lay.InputCapture)
	g.app.SetFocus(g.a)

	g.screen = tcell.NewSimulationScreen("UTF-8")
	if err := g.screen.Init(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(g.screen.Fini)
	g.draw(100, 20)
	return g
}

// draw renders the area at w×h, as tview does on each frame.
func (g *rig) draw(w, h int) {
	g.screen.SetSize(w, h)
	g.area.SetRect(0, 0, w, h)
	g.area.Draw(g.screen)
}

// press delivers keys the way tview's event loop does (the application
// input capture, then the focused primitive), redrawing after each.
func (g *rig) press(evs ...*tcell.EventKey) {
	for _, ev := range evs {
		if c := g.app.GetInputCapture(); c != nil {
			if ev = c(ev); ev == nil {
				g.redraw()
				continue
			}
		}
		if h := g.app.GetFocus().InputHandler(); h != nil {
			h(ev, func(p tview.Primitive) { g.app.SetFocus(p) })
		}
		g.redraw()
	}
}

func (g *rig) redraw() {
	w, h := g.screen.Size()
	g.draw(w, h)
}

func width(p tview.Primitive) int  { _, _, w, _ := p.GetRect(); return w }
func height(p tview.Primitive) int { _, _, _, h := p.GetRect(); return h }

func TestResizeModeRepeats(t *testing.T) {
	g := newRig(t)
	if w := width(g.a); w != 20 {
		t.Fatalf("a starts %d wide, want 20", w)
	}
	g.press(ctrlW, r('L'), r('L'), r('L'))
	if g.lay.Mode() != ModeResize {
		t.Fatalf("mode %v after C-w L L L, want resize", g.lay.Mode())
	}
	if w := width(g.a); w != 35 {
		t.Errorf("a is %d wide after C-w L L L, want 35", w)
	}
	if w := width(g.b); w != 10 {
		t.Errorf("b is %d wide, want its Min 10", w)
	}
	if len(g.got) != 0 {
		t.Errorf("resize keys leaked to the pane: %v", g.got)
	}
	if !strings.Contains(g.lay.Status(), "RESIZE") {
		t.Errorf("status %q does not show resize mode", g.lay.Status())
	}
}

// The key that ends resize mode reaches the pane exactly once and does not
// resize; the next H/J/K/L is an ordinary key again.
func TestResizeModeReplaysOtherKeyOnce(t *testing.T) {
	g := newRig(t)
	g.press(ctrlW, r('L'), r('j'))
	if g.lay.Mode() != ModeNormal {
		t.Fatalf("mode %v after j, want normal", g.lay.Mode())
	}
	if want := []string{"Rune[j]"}; strings.Join(g.got, ",") != strings.Join(want, ",") {
		t.Fatalf("pane got %v, want %v", g.got, want)
	}
	g.press(r('L'))
	if w := width(g.a); w != 25 {
		t.Errorf("L after leaving the mode resized: a is %d wide, want 25", w)
	}
	if len(g.got) != 2 || g.got[1] != "Rune[L]" {
		t.Errorf("L after leaving the mode did not reach the pane: %v", g.got)
	}
}

func TestResizeModeEscIsSilent(t *testing.T) {
	g := newRig(t)
	g.press(ctrlW, r('L'), key(tcell.KeyEscape))
	if g.lay.Mode() != ModeNormal || len(g.got) != 0 {
		t.Fatalf("mode %v, pane got %v; want normal and nothing", g.lay.Mode(), g.got)
	}
}

// A C-w that ends resize mode is replayed to the pane; it does not start a
// new prefix.
func TestReplayedKeyNeverReentersMode(t *testing.T) {
	g := newRig(t)
	g.press(ctrlW, r('L'), ctrlW)
	if g.lay.Mode() != ModeNormal {
		t.Fatalf("mode %v, want normal", g.lay.Mode())
	}
	if len(g.got) != 1 || g.got[0] != "Ctrl-W" {
		t.Fatalf("pane got %v, want the replayed Ctrl-W", g.got)
	}
}

func TestUnknownKeyAfterPrefixIsDropped(t *testing.T) {
	g := newRig(t)
	g.press(ctrlW, r('x'))
	if g.lay.Mode() != ModeNormal || len(g.got) != 0 {
		t.Fatalf("mode %v, pane got %v; want normal and nothing", g.lay.Mode(), g.got)
	}
}

// With a text field focused, C-w is the field's delete-word.
func TestInertWithInputFieldFocused(t *testing.T) {
	g := newRig(t)
	g.app.SetFocus(g.in)
	g.in.SetText("hello world")
	g.press(ctrlW, r('L'))
	if g.lay.Mode() != ModeNormal {
		t.Fatalf("mode %v with an InputField focused, want normal", g.lay.Mode())
	}
	if got := g.in.GetText(); got != "hello L" {
		t.Errorf("field text %q, want %q (C-w deleted a word, L typed)", got, "hello L")
	}
}

func TestInertWithModalOpen(t *testing.T) {
	g := newRig(t)
	g.modal = true
	g.press(ctrlW, r('L'), alt('l'))
	if g.lay.Mode() != ModeNormal || width(g.a) != 20 {
		t.Fatalf("mode %v, a %d wide; want normal and 20", g.lay.Mode(), width(g.a))
	}
	if len(g.got) != 3 {
		t.Errorf("pane got %v, want all three keys", g.got)
	}
}

// A key names the direction the border moves: with a neighbour on the
// right the right border moves, otherwise the left one.
func TestBorderRule(t *testing.T) {
	g := newRig(t)
	g.press(ctrlW, r('H'), key(tcell.KeyEscape))
	if w := width(g.a); w != 15 {
		t.Errorf("C-w H on the left pane: a is %d wide, want 15 (right border moved left)", w)
	}

	g = newRig(t)
	g.app.SetFocus(g.c)
	g.press(ctrlW, r('L'), key(tcell.KeyEscape))
	if w := width(g.c); w != 15 {
		t.Errorf("C-w L on the rightmost pane: c is %d wide, want 15 (left border moved right)", w)
	}
	if w := width(g.in); w != 15 {
		t.Errorf("its left neighbour is %d wide, want 15", w)
	}
}

// J/K from a pane in a horizontal stack move the border of the stack that
// holds it in the vertical one outside.
func TestVerticalBorderFromNestedPane(t *testing.T) {
	g := newRig(t)
	g.press(ctrlW, r('K'), key(tcell.KeyEscape))
	if h := height(g.log); h != 10 {
		t.Errorf("C-w K from a body pane: log is %d rows, want 10", h)
	}
	g.app.SetFocus(g.log)
	g.press(ctrlW, r('J'), r('J'))
	if h := height(g.log); h != 3 {
		t.Errorf("C-w J J on the log: log is %d rows, want its Min 3", h)
	}
}

func TestAltNudgeAndEqualise(t *testing.T) {
	g := newRig(t)
	g.press(alt('l'), alt('l'))
	if g.lay.Mode() != ModeNormal || width(g.a) != 22 {
		t.Fatalf("after Alt-l Alt-l: mode %v, a %d wide; want normal, 22", g.lay.Mode(), width(g.a))
	}
	g.press(ctrlW, r('K'), key(tcell.KeyEscape))
	g.press(ctrlW, r('='))
	if width(g.a) != 20 || height(g.log) != 5 {
		t.Errorf("after C-w =: a %d wide, log %d rows; want 20 and 5", width(g.a), height(g.log))
	}
}

// Growth given by the keys is clamped to the space there is and stays
// clamped when the space comes back.
func TestOffsetsReclampedOnResize(t *testing.T) {
	g := newRig(t)
	g.press(ctrlW, r('L'), r('L'), r('L'), key(tcell.KeyEscape))
	if w := width(g.a); w != 35 {
		t.Fatalf("a %d wide, want 35", w)
	}
	g.draw(60, 20) // the other three Mins leave a 30
	if w := width(g.a); w != 30 {
		t.Fatalf("at 60 wide a is %d, want 30", w)
	}
	g.draw(100, 20)
	if w := width(g.a); w != 30 {
		t.Errorf("back at 100 wide a is %d, want 30 (offset re-clamped)", w)
	}
	g.draw(35, 20) // no growth left at all
	g.draw(100, 20)
	if w := width(g.a); w != 20 {
		t.Errorf("after 35 wide and back a is %d, want its Pref 20", w)
	}
}

func TestMinNeverViolatedAtAnyWidth(t *testing.T) {
	g := newRig(t)
	g.press(ctrlW, r('L'), r('L'), key(tcell.KeyEscape))
	panes := []tview.Primitive{g.a, g.b, g.in, g.c}
	mins := []int{10, 10, 10, 10}
	for w := 1; w <= 300; w++ {
		g.draw(w, 20)
		for i, p := range panes {
			if g.body.Shown(p) && width(p) < mins[i] {
				t.Fatalf("width %d: pane %d drawn %d wide, below Min %d", w, i, width(p), mins[i])
			}
		}
	}
}

func TestZoomTogglesAndRestoresFocus(t *testing.T) {
	g := newRig(t)
	g.app.SetFocus(g.b)
	g.press(ctrlW, r('z'))
	if g.lay.Zoomed() != g.b {
		t.Fatalf("zoomed %v, want b", g.lay.Zoomed())
	}
	if w, h := width(g.b), height(g.b); w != 100 || h != 20 {
		t.Errorf("zoomed b is %dx%d, want the whole 100x20 area", w, h)
	}
	if !strings.Contains(g.lay.Status(), "ZOOM") {
		t.Errorf("status %q does not show the zoom", g.lay.Status())
	}
	if g.lay.Visible(g.a) || !g.lay.Visible(g.b) {
		t.Error("while zoomed only the zoomed pane is visible")
	}
	g.app.SetFocus(g.a) // e.g. a tool moved focus while zoomed
	g.press(ctrlW, r('o'))
	if g.lay.Zoomed() != nil || g.app.GetFocus() != g.b {
		t.Fatalf("C-w o: zoomed %v, focus %v; want no zoom, focus back on b", g.lay.Zoomed(), g.app.GetFocus())
	}
	if w := width(g.b); w != 20 {
		t.Errorf("b is %d wide after the zoom, want 20", w)
	}

	g.press(ctrlW, r('z'), key(tcell.KeyEscape))
	if g.lay.Zoomed() != nil || len(g.got) != 0 {
		t.Errorf("Esc: zoomed %v, pane got %v; want no zoom and nothing", g.lay.Zoomed(), g.got)
	}
	g.press(key(tcell.KeyEscape))
	if len(g.got) != 1 {
		t.Errorf("Esc with no zoom did not reach the pane: %v", g.got)
	}
}

func TestZoomEndsWhenPaneRemoved(t *testing.T) {
	g := newRig(t)
	g.app.SetFocus(g.c)
	g.lay.ToggleZoom()
	g.body.Remove(g.c)
	if g.lay.Zoomed() != nil {
		t.Fatal("zoom outlived its pane")
	}
}

func TestVisibleFollowsOverflow(t *testing.T) {
	g := newRig(t)
	var counts []int
	g.body.SetHiddenFunc(func(n int) { counts = append(counts, n) })
	g.draw(25, 20)
	if g.lay.Visible(g.c) || g.lay.Visible(g.in) || !g.lay.Visible(g.a) {
		t.Errorf("at 25 wide: visible a=%v in=%v c=%v; want a only of those", g.lay.Visible(g.a), g.lay.Visible(g.in), g.lay.Visible(g.c))
	}
	g.draw(25, 20)
	g.draw(100, 20)
	if want := "2,0"; joinInts(counts) != want {
		t.Errorf("hidden func calls %v, want %s (once per change)", counts, want)
	}
	if g.body.Hidden() != 0 || !g.lay.Visible(g.c) {
		t.Error("panes not restored at 100 wide")
	}
}

func joinInts(v []int) string {
	s := make([]string, len(v))
	for i, n := range v {
		s[i] = string(rune('0' + n))
	}
	return strings.Join(s, ",")
}

// Nested stacks report their request upward.
func TestStackSpec(t *testing.T) {
	s := NewStack(Horizontal)
	s.Add(tview.NewBox(), Spec{Min: 10, Pref: 30, Max: 30})
	s.Add(tview.NewBox(), Spec{Min: 12, Pref: 20, Max: 25, Priority: 1})
	got := s.Spec()
	if want := (Spec{Min: 12, Pref: 50, Max: 55}); got != want {
		t.Fatalf("Spec() = %+v, want %+v", got, want)
	}
	s.Add(tview.NewBox(), Spec{Weight: 2})
	if got := s.Spec(); got.Max != 0 || got.Weight != 2 {
		t.Errorf("with an unbounded pane Spec() = %+v, want Max 0 Weight 2", got)
	}
}

// rowText returns screen row y as a string.
func rowText(s tcell.SimulationScreen, y int) string {
	w, _ := s.Size()
	var b strings.Builder
	for x := range w {
		str, _, _ := s.Get(x, y)
		if str == "" {
			str = " "
		}
		b.WriteString(str)
	}
	return b.String()
}

func TestRenderSimulationScreen(t *testing.T) {
	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	defer s.Fini()
	st := NewStack(Horizontal).ShowHiddenBadge(true)
	for _, title := range []string{"A", "B", "C"} {
		st.Add(tview.NewBox().SetBorder(true).SetTitle(title), Spec{Min: 10, Pref: 20, Max: 20})
	}
	render := func(w int) (top, bottom string) {
		s.SetSize(w, 5)
		st.SetRect(0, 0, w, 5)
		st.Draw(s)
		return rowText(s, 0), rowText(s, 4)
	}

	top, bottom := render(50) // A 20, B 20, C its Min 10
	for _, x := range []int{0, 20, 40} {
		if c := []rune(top)[x]; c != '┌' {
			t.Errorf("at 50 wide, no pane corner at x=%d: %q", x, top)
		}
	}
	if strings.Contains(bottom, "hidden") {
		t.Errorf("badge shown with nothing hidden: %q", bottom)
	}

	top, bottom = render(25) // C hidden, A 15, B 10
	if strings.Count(top, "┌") != 2 || []rune(top)[15] != '┌' {
		t.Errorf("at 25 wide want A at 0 and B at 15: %q", top)
	}
	if !strings.Contains(bottom, "+1 hidden") {
		t.Errorf("at 25 wide the badge is missing: %q", bottom)
	}

	top, _ = render(70) // spare 10 columns left empty
	if strings.Count(top, "┌") != 3 || strings.TrimRight(top, " ") != strings.TrimRight(top[:len(top)-10], " ") {
		t.Errorf("at 70 wide want three 20-wide panes and 10 empty columns: %q", top)
	}
}

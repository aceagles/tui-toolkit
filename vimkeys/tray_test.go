package vimkeys

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func key_(k tcell.Key) *tcell.EventKey { return tcell.NewEventKey(k, 0, tcell.ModNone) }

// trayRig is a tool wired the way the package doc says: a Tray around the
// root and the leader as the application input capture.
type trayRig struct {
	app     *tview.Application
	l       *Leader
	tray    *Tray
	view    *tview.TextView
	input   *tview.InputField
	fired   []string
	unbound []string
}

func newTrayRig(t *testing.T) *trayRig {
	t.Helper()
	app := tview.NewApplication()
	r := &trayRig{app: app}
	// A 1ms timeout that would fire mid-test if the tray did not disable it.
	r.l = NewLeader(app, time.Millisecond)
	fire := func(name string) func() { return func() { r.fired = append(r.fired, name) } }
	r.l.MustRegister("r", "refresh", fire("r"))
	r.l.MustRegisterGroup("g", "groups")
	r.l.MustRegister("ga", "add group", fire("ga"))
	r.l.MustRegister("gx", "delete group", fire("gx"))
	r.l.OnUnbound = func(pending string, ev *tcell.EventKey) {
		r.unbound = append(r.unbound, pending+"|"+ev.Name())
	}

	r.view = tview.NewTextView()
	r.input = tview.NewInputField()
	root := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(r.view, 0, 1, true).
		AddItem(r.input, 1, 0, false)
	r.tray = NewTray(r.l, root)
	app.SetRoot(r.tray, true)
	app.SetInputCapture(r.l.InputCapture)
	app.SetFocus(r.view)
	return r
}

// press delivers keys the way tview's event loop does: the application
// input capture first, then whatever it lets through to the focused
// primitive's own handler.
func (r *trayRig) press(evs ...*tcell.EventKey) {
	for _, ev := range evs {
		if c := r.app.GetInputCapture(); c != nil {
			if ev = c(ev); ev == nil {
				continue
			}
		}
		if h := r.app.GetFocus().InputHandler(); h != nil {
			h(ev, func(p tview.Primitive) { r.app.SetFocus(p) })
		}
	}
}

// render draws p on an 80x20 simulation screen and returns its rows.
func render(t *testing.T, p tview.Primitive) []string {
	t.Helper()
	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	defer s.Fini()
	s.SetSize(80, 20)
	p.SetRect(0, 0, 80, 20)
	p.Draw(s)
	s.Show()
	cells, w, h := s.GetContents()
	rows := make([]string, h)
	for y := range h {
		var b strings.Builder
		for x := range w {
			c := cells[y*w+x]
			if len(c.Runes) == 0 {
				b.WriteRune(' ')
				continue
			}
			b.WriteRune(c.Runes[0])
		}
		rows[y] = b.String()
	}
	return rows
}

func screenHas(rows []string, s string) bool { return strings.Contains(strings.Join(rows, "\n"), s) }

func TestTraySpaceOpens(t *testing.T) {
	r := newTrayRig(t)
	if rows := render(t, r.tray); screenHas(rows, "refresh") {
		t.Fatal("tray must not draw while the leader is idle")
	}
	r.press(rune_(' '))
	if keys, open := r.l.Pending(); !open || keys != "" {
		t.Fatalf("Pending() = %q,%v after Space; want \"\",true", keys, open)
	}
	rows := render(t, r.tray)
	for _, want := range []string{"<space>", "r  refresh", "g  +groups"} {
		if !screenHas(rows, want) {
			t.Fatalf("tray missing %q:\n%s", want, strings.Join(rows, "\n"))
		}
	}
}

func TestTrayGroupKeyShowsChildren(t *testing.T) {
	r := newTrayRig(t)
	r.press(rune_(' '), rune_('g'))
	rows := render(t, r.tray)
	for _, want := range []string{"<space> g", "a  add group", "x  delete group"} {
		if !screenHas(rows, want) {
			t.Fatalf("group tray missing %q:\n%s", want, strings.Join(rows, "\n"))
		}
	}
	if screenHas(rows, "refresh") {
		t.Fatal("group tray must replace the top level, not add to it")
	}
	if len(r.fired) != 0 {
		t.Fatalf("a group key must not dispatch; fired %v", r.fired)
	}
}

func TestTrayLeafDispatchesOnceAndCloses(t *testing.T) {
	r := newTrayRig(t)
	r.press(rune_(' '), rune_('g'), rune_('a'))
	if !reflect.DeepEqual(r.fired, []string{"ga"}) {
		t.Fatalf("fired = %v, want [ga]", r.fired)
	}
	if _, open := r.l.Pending(); open {
		t.Fatal("tray must close after a leaf dispatches")
	}
	if screenHas(render(t, r.tray), "add group") {
		t.Fatal("tray still drawn after dispatch")
	}
	r.press(rune_('a'))
	if len(r.fired) != 1 {
		t.Fatalf("a key after dispatch must be a plain key; fired %v", r.fired)
	}
}

func TestTrayEscClosesAtEachLevel(t *testing.T) {
	for _, tc := range []struct {
		name string
		keys []*tcell.EventKey
	}{
		{"top level", []*tcell.EventKey{rune_(' ')}},
		{"in group", []*tcell.EventKey{rune_(' '), rune_('g')}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := newTrayRig(t)
			r.press(tc.keys...)
			r.press(key_(tcell.KeyEscape))
			if _, open := r.l.Pending(); open {
				t.Fatal("Esc must close the tray")
			}
			if len(r.fired) != 0 || len(r.unbound) != 0 {
				t.Fatalf("Esc must neither dispatch nor report: fired %v unbound %v", r.fired, r.unbound)
			}
		})
	}
}

func TestTrayBackspaceGoesUpOneLevel(t *testing.T) {
	r := newTrayRig(t)
	r.press(rune_(' '), rune_('g'), key_(tcell.KeyBackspace2))
	if keys, open := r.l.Pending(); !open || keys != "" {
		t.Fatalf("Backspace in a group: Pending() = %q,%v; want \"\",true", keys, open)
	}
	if !screenHas(render(t, r.tray), "refresh") {
		t.Fatal("Backspace must show the top level again")
	}
	r.press(key_(tcell.KeyBackspace2))
	if _, open := r.l.Pending(); open {
		t.Fatal("Backspace at the top level must close the tray")
	}
}

func TestTrayUnboundKeyClosesAndReports(t *testing.T) {
	r := newTrayRig(t)
	r.press(rune_(' '), rune_('g'), rune_('z'))
	if _, open := r.l.Pending(); open {
		t.Fatal("an unbound key must close the tray")
	}
	r.press(rune_(' '), key_(tcell.KeyEnter))
	want := []string{"g|Rune[z]", "|Enter"}
	if !reflect.DeepEqual(r.unbound, want) {
		t.Fatalf("unbound = %v, want %v", r.unbound, want)
	}
	r.press(rune_('a'))
	if len(r.fired) != 0 {
		t.Fatalf("an unbound key must not dispatch anything; fired %v", r.fired)
	}
}

func TestTrayHasNoTimeout(t *testing.T) {
	r := newTrayRig(t) // leader timeout is 1ms
	r.press(rune_(' '))
	time.Sleep(20 * time.Millisecond)
	r.press(rune_('r'))
	if !reflect.DeepEqual(r.fired, []string{"r"}) {
		t.Fatalf("fired = %v; the tray must wait for a key, not time out", r.fired)
	}
}

func TestTraySpaceTypesIntoFocusedInput(t *testing.T) {
	r := newTrayRig(t)
	r.app.SetFocus(r.input)
	r.press(rune_('a'), rune_(' '), rune_('b'))
	if _, open := r.l.Pending(); open {
		t.Fatal("Space in an InputField must not open the tray")
	}
	if got := r.input.GetText(); got != "a b" {
		t.Fatalf("InputField text = %q, want %q", got, "a b")
	}
}

func TestTraySkipWhenModalOpen(t *testing.T) {
	r := newTrayRig(t)
	modal := true
	r.l.SkipWhen = func() bool { return modal || FocusWantsSpace(r.app) }
	ev := rune_(' ')
	if out := r.l.InputCapture(ev); out != ev {
		t.Fatal("Space must pass through to an open modal")
	}
	modal = false
	r.press(rune_(' '))
	if _, open := r.l.Pending(); !open {
		t.Fatal("Space must open the tray once the modal closes")
	}
}

func TestTrayAndHelpShareRegistry(t *testing.T) {
	r := newTrayRig(t)
	r.l.MustRegister("gm", "move group", func() {})
	help := r.l.HelpText()
	r.press(rune_(' '), rune_('g'))
	tray := render(t, r.tray)
	if !strings.Contains(help, "move group") || !screenHas(tray, "m  move group") {
		t.Fatalf("leaf must appear in help and tray:\nhelp:\n%s\ntray:\n%s", help, strings.Join(tray, "\n"))
	}
}

func TestTrayRendersManyEntriesInColumns(t *testing.T) {
	l := NewLeader(tview.NewApplication(), 0)
	for _, k := range "abcdefghijklmnopqrstuvwx" {
		l.MustRegister(string(k), "leaf "+string(k), func() {})
	}
	tray := NewTray(l, tview.NewBox())
	l.InputCapture(rune_(' '))
	rows := render(t, tray)

	var trayRows int
	for _, row := range rows {
		if strings.Contains(row, "leaf") {
			trayRows++
		}
	}
	if trayRows == 0 || trayRows >= 24 {
		t.Fatalf("24 entries drew on %d rows; want several columns:\n%s", trayRows, strings.Join(rows, "\n"))
	}
	for _, k := range "ax" {
		if !screenHas(rows, string(k)+"  leaf "+string(k)) {
			t.Fatalf("entry %c missing:\n%s", k, strings.Join(rows, "\n"))
		}
	}
	if !strings.Contains(rows[len(rows)-1], "─") {
		t.Fatal("tray must sit on the bottom rows of the root")
	}
}

func TestEntriesUnnamedGroup(t *testing.T) {
	l := NewLeader(tview.NewApplication(), 0)
	l.MustRegister("r", "refresh", func() {})
	l.MustRegister("sl", "show log", func() {})
	want := []Entry{
		{Key: 'r', Keys: "r", Desc: "refresh"},
		{Key: 's', Keys: "s", Group: true},
	}
	if got := l.Entries(""); !reflect.DeepEqual(got, want) {
		t.Fatalf("Entries(\"\") = %+v, want %+v", got, want)
	}
	if got := l.Entries("s"); len(got) != 1 || got[0].Keys != "sl" || got[0].Group {
		t.Fatalf("Entries(\"s\") = %+v, want the sl leaf", got)
	}
}

func TestHelpTextWithGroups(t *testing.T) {
	l := NewLeader(tview.NewApplication(), 0)
	l.MustRegister("ga", "add", func() {})
	l.MustRegister("r", "refresh", func() {})
	l.MustRegisterGroup("g", "groups")
	want := "  <space>g      +groups\n" +
		"  <space>ga     add\n" +
		"  <space>r      refresh\n"
	if got := l.HelpText(); got != want {
		t.Fatalf("HelpText() =\n%s\nwant\n%s", got, want)
	}
}

func TestRegisterGroupConflicts(t *testing.T) {
	l := NewLeader(tview.NewApplication(), 0)
	l.MustRegister("r", "refresh", func() {})
	l.MustRegisterGroup("g", "groups")
	if err := l.RegisterGroup("g", "again"); err == nil {
		t.Fatal("duplicate group must be rejected")
	}
	if err := l.RegisterGroup("r", "shadow"); err == nil {
		t.Fatal("group on a leaf must be rejected")
	}
	if err := l.RegisterGroup("rx", "under a leaf"); err == nil {
		t.Fatal("group below a leaf must be rejected")
	}
	if err := l.Register("g", "leaf on a group", func() {}); err == nil {
		t.Fatal("leaf on a group must be rejected")
	}
	if err := l.RegisterGroup("gs", "nested"); err != nil {
		t.Fatalf("nested group: %v", err)
	}
	if err := l.Register("gsa", "nested leaf", func() {}); err != nil {
		t.Fatalf("leaf in nested group: %v", err)
	}
}

package vimkeys

import (
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func rune_(r rune) *tcell.EventKey { return tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone) }

func TestMotionsTranslation(t *testing.T) {
	m := Motions()
	tests := []struct {
		in   *tcell.EventKey
		want tcell.Key
	}{
		{rune_('j'), tcell.KeyDown},
		{rune_('k'), tcell.KeyUp},
		{rune_('G'), tcell.KeyEnd},
		{tcell.NewEventKey(tcell.KeyCtrlD, 0, tcell.ModCtrl), tcell.KeyPgDn},
		{tcell.NewEventKey(tcell.KeyCtrlU, 0, tcell.ModCtrl), tcell.KeyPgUp},
	}
	for _, tt := range tests {
		out := m(tt.in)
		if out == nil || out.Key() != tt.want {
			t.Fatalf("Motions(%v) = %v, want key %v", tt.in, out, tt.want)
		}
	}
}

func TestMotionsGGChord(t *testing.T) {
	m := Motions()
	if out := m(rune_('g')); out != nil {
		t.Fatal("first g must be swallowed")
	}
	out := m(rune_('g'))
	if out == nil || out.Key() != tcell.KeyHome {
		t.Fatalf("gg = %v, want Home", out)
	}
	// A broken chord resets: g then j scrolls down, and a later g starts fresh.
	m(rune_('g'))
	if out := m(rune_('j')); out == nil || out.Key() != tcell.KeyDown {
		t.Fatal("g then j must still scroll down")
	}
	if out := m(rune_('g')); out != nil {
		t.Fatal("chord state must have reset after the broken gg")
	}
}

func TestMotionsPassThrough(t *testing.T) {
	m := Motions()
	ev := rune_('q')
	if out := m(ev); out != ev {
		t.Fatal("unrelated rune must pass through untouched")
	}
	enter := tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)
	if out := m(enter); out != enter {
		t.Fatal("Enter must pass through untouched")
	}
}

func TestChain(t *testing.T) {
	consume := func(ev *tcell.EventKey) *tcell.EventKey {
		if ev.Rune() == 'x' {
			return nil
		}
		return ev
	}
	c := Chain(consume, Motions())
	if out := c(rune_('x')); out != nil {
		t.Fatal("first capture should consume x")
	}
	if out := c(rune_('j')); out == nil || out.Key() != tcell.KeyDown {
		t.Fatal("second capture should still translate j")
	}
}

func newTestLeader(t *testing.T) (*Leader, *[]string) {
	t.Helper()
	l := NewLeader(tview.NewApplication(), time.Hour) // timeout irrelevant to these tests
	l.SkipWhen = func() bool { return false }
	var fired []string
	l.MustRegister("r", "refresh", func() { fired = append(fired, "r") })
	l.MustRegister("sl", "show log", func() { fired = append(fired, "sl") })
	return l, &fired
}

func TestLeaderDispatch(t *testing.T) {
	l, fired := newTestLeader(t)

	if out := l.InputCapture(rune_(' ')); out != nil {
		t.Fatal("space must be consumed as the leader")
	}
	if out := l.InputCapture(rune_('r')); out != nil {
		t.Fatal("chord rune must be consumed")
	}
	if len(*fired) != 1 || (*fired)[0] != "r" {
		t.Fatalf("fired = %v, want [r]", *fired)
	}

	// Multi-rune chord: space s l.
	l.InputCapture(rune_(' '))
	l.InputCapture(rune_('s'))
	l.InputCapture(rune_('l'))
	if len(*fired) != 2 || (*fired)[1] != "sl" {
		t.Fatalf("fired = %v, want [r sl]", *fired)
	}
}

func TestLeaderUnknownChordAndEscape(t *testing.T) {
	l, fired := newTestLeader(t)

	l.InputCapture(rune_(' '))
	l.InputCapture(rune_('z')) // no such chord: reset
	l.InputCapture(rune_('r'))
	if len(*fired) != 0 {
		t.Fatal("r after an abandoned chord must be a plain key, not a chord")
	}

	l.InputCapture(rune_(' '))
	l.InputCapture(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))
	l.InputCapture(rune_('r'))
	if len(*fired) != 0 {
		t.Fatal("Esc must abandon the chord")
	}
}

func TestLeaderSkipWhenTyping(t *testing.T) {
	l, _ := newTestLeader(t)
	l.SkipWhen = func() bool { return true }
	ev := rune_(' ')
	if out := l.InputCapture(ev); out != ev {
		t.Fatal("space must type a space while an input has focus")
	}
}

func TestRegisterConflicts(t *testing.T) {
	l := NewLeader(tview.NewApplication(), time.Hour)
	l.MustRegister("sl", "show log", func() {})
	if err := l.Register("s", "shadow", func() {}); err == nil {
		t.Fatal("prefix of an existing chord must be rejected")
	}
	if err := l.Register("slx", "shadowed", func() {}); err == nil {
		t.Fatal("extension of an existing chord must be rejected")
	}
	if err := l.Register("", "empty", func() {}); err == nil {
		t.Fatal("empty chord must be rejected")
	}
}

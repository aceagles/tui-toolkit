package bubbletea

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

// keyMsg builds a tea.KeyPressMsg the way bubbletea's own decoder would for
// a plain, unmodified character.
func keyMsg(r rune) tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Code: r, Text: string(r)})
}

func specialMsg(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Code: code})
}

func otherMsg() tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Code: tea.KeyUp})
}

// newTestLeader mirrors vimkeys/tview's rig: the same bindings so a test
// can assert the bubbletea path drives identical transitions.
func newTestLeader(t *testing.T) (*Leader, *[]string) {
	t.Helper()
	l := NewLeader(0)
	var fired []string
	fire := func(name string) func() { return func() { fired = append(fired, name) } }
	l.MustRegister("r", "refresh", fire("r"))
	l.MustRegisterGroup("g", "groups")
	l.MustRegister("ga", "add group", fire("ga"))
	l.MustRegister("gx", "delete group", fire("gx"))
	return l, &fired
}

func TestUpdateDispatchesChord(t *testing.T) {
	l, fired := newTestLeader(t)

	if consumed, _ := l.Update(keyMsg(' ')); !consumed {
		t.Fatal("space must be consumed as the leader")
	}
	if consumed, _ := l.Update(keyMsg('g')); !consumed {
		t.Fatal("group key must be consumed")
	}
	if consumed, _ := l.Update(keyMsg('a')); !consumed {
		t.Fatal("leaf key must be consumed")
	}
	if len(*fired) != 1 || (*fired)[0] != "ga" {
		t.Fatalf("fired = %v, want [ga]", *fired)
	}
	if _, open := l.Pending(); open {
		t.Fatal("chord must close after dispatch")
	}
}

func TestUpdateIgnoresUnrelatedMsg(t *testing.T) {
	l, _ := newTestLeader(t)
	consumed, cmd := l.Update(struct{}{})
	if consumed || cmd != nil {
		t.Fatalf("an unrelated Msg must pass through untouched; consumed=%v cmd=%v", consumed, cmd)
	}
}

func TestUpdateEscAbandons(t *testing.T) {
	l, fired := newTestLeader(t)
	l.Update(keyMsg(' '))
	l.Update(keyMsg('g'))
	if consumed, _ := l.Update(specialMsg(tea.KeyEscape)); !consumed {
		t.Fatal("Esc must be consumed")
	}
	if _, open := l.Pending(); open {
		t.Fatal("Esc must abandon the chord")
	}
	l.Update(keyMsg('a'))
	if len(*fired) != 0 {
		t.Fatalf("a after Esc must be a plain key, not a chord; fired %v", *fired)
	}
}

func TestUpdateBackspaceGoesUpOneLevel(t *testing.T) {
	l, _ := newTestLeader(t)
	l.Update(keyMsg(' '))
	l.Update(keyMsg('g'))
	l.Update(specialMsg(tea.KeyBackspace))
	if keys, open := l.Pending(); !open || keys != "" {
		t.Fatalf("Backspace in a group: Pending() = %q,%v; want \"\",true", keys, open)
	}
	l.Update(specialMsg(tea.KeyBackspace))
	if _, open := l.Pending(); open {
		t.Fatal("Backspace at the top level must close the chord")
	}
}

func TestUpdateUnboundReportsOriginalMsg(t *testing.T) {
	l, fired := newTestLeader(t)
	var gotPending string
	var gotMsg tea.KeyPressMsg
	l.OnUnbound = func(pending string, msg tea.KeyPressMsg) {
		gotPending, gotMsg = pending, msg
	}

	l.Update(keyMsg(' '))
	l.Update(keyMsg('g'))
	unbound := otherMsg()
	l.Update(unbound)

	if gotPending != "g" {
		t.Fatalf("OnUnbound pending = %q, want %q", gotPending, "g")
	}
	if gotMsg != unbound {
		t.Fatalf("OnUnbound msg = %+v, want the original %+v", gotMsg, unbound)
	}
	if _, open := l.Pending(); open {
		t.Fatal("an unbound key must close the chord")
	}
	l.Update(keyMsg('r'))
	if len(*fired) != 0 {
		t.Fatalf("a key after an unbound abandon must be a plain key; fired %v", *fired)
	}
}

func TestUpdateSkipWhenTyping(t *testing.T) {
	l, _ := newTestLeader(t)
	l.SkipWhen = func() bool { return true }
	if consumed, _ := l.Update(keyMsg(' ')); consumed {
		t.Fatal("space must pass through while an input has focus")
	}
}

func TestUpdateSchedulesTimeoutOnlyWhenNotSticky(t *testing.T) {
	l, _ := newTestLeader(t)
	l.timeout = time.Millisecond

	_, cmd := l.Update(keyMsg(' '))
	if cmd == nil {
		t.Fatal("arming a chord without a tray mounted must schedule a timeout Cmd")
	}

	l.Sticky = true
	l.Update(specialMsg(tea.KeyEscape)) // close the first chord
	_, cmd = l.Update(keyMsg(' '))
	if cmd != nil {
		t.Fatal("a sticky (tray-mounted) leader must not schedule a timeout")
	}
}

func TestTimeoutMsgResetsOnlyAtItsOwnGeneration(t *testing.T) {
	l, fired := newTestLeader(t)
	l.timeout = time.Millisecond

	_, cmd := l.Update(keyMsg(' '))
	staleMsg := cmd().(timeoutMsg)

	// A real key arrives before the tick would have: generation moves on.
	l.Update(keyMsg('r'))
	if len(*fired) != 1 {
		t.Fatalf("fired = %v, want [r]", *fired)
	}

	// The stale tick must not clobber whatever is pending now.
	l.Update(keyMsg(' '))
	l.Update(timeoutMsg(staleMsg))
	if _, open := l.Pending(); !open {
		t.Fatal("a stale timeoutMsg must not reset a newer pending chord")
	}

	// A tick at the CURRENT generation does reset.
	l.Update(timeoutMsg{gen: l.Generation()})
	if _, open := l.Pending(); open {
		t.Fatal("a timeoutMsg at the current generation must abandon the chord")
	}
}

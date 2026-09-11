package core

import (
	"reflect"
	"testing"
)

func newTestLeader(t *testing.T) (*Leader, *[]string) {
	t.Helper()
	l := NewLeader()
	var fired []string
	l.MustRegister("r", "refresh", func() { fired = append(fired, "r") })
	l.MustRegister("sl", "show log", func() { fired = append(fired, "sl") })
	return l, &fired
}

func TestLeaderDispatch(t *testing.T) {
	l, fired := newTestLeader(t)

	if res := l.Handle(Event{Kind: KindRune, Rune: ' '}); !res.Consumed {
		t.Fatal("space must be consumed as the leader")
	}
	if res := l.Handle(Event{Kind: KindRune, Rune: 'r'}); !res.Consumed {
		t.Fatal("chord rune must be consumed")
	}
	if len(*fired) != 1 || (*fired)[0] != "r" {
		t.Fatalf("fired = %v, want [r]", *fired)
	}

	// Multi-rune chord: space s l.
	l.Handle(Event{Kind: KindRune, Rune: ' '})
	l.Handle(Event{Kind: KindRune, Rune: 's'})
	l.Handle(Event{Kind: KindRune, Rune: 'l'})
	if len(*fired) != 2 || (*fired)[1] != "sl" {
		t.Fatalf("fired = %v, want [r sl]", *fired)
	}
}

func TestLeaderUnknownChordAndEscape(t *testing.T) {
	l, fired := newTestLeader(t)

	l.Handle(Event{Kind: KindRune, Rune: ' '})
	res := l.Handle(Event{Kind: KindRune, Rune: 'z'}) // no such chord: reset
	if !res.Unbound || res.UnboundPending != "" {
		t.Fatalf("unbound key: Result = %+v, want Unbound with empty UnboundPending", res)
	}
	l.Handle(Event{Kind: KindRune, Rune: 'r'})
	if len(*fired) != 0 {
		t.Fatal("r after an abandoned chord must be a plain key, not a chord")
	}

	l.Handle(Event{Kind: KindRune, Rune: ' '})
	if res := l.Handle(Event{Kind: KindEscape}); res.Unbound {
		t.Fatal("Esc must abandon the chord without reporting it as unbound")
	}
	l.Handle(Event{Kind: KindRune, Rune: 'r'})
	if len(*fired) != 0 {
		t.Fatal("Esc must abandon the chord")
	}
}

func TestLeaderUnboundReportsDeeperPending(t *testing.T) {
	l := NewLeader()
	l.MustRegisterGroup("g", "groups")
	l.MustRegister("ga", "add group", func() {})

	l.Handle(Event{Kind: KindRune, Rune: ' '})
	l.Handle(Event{Kind: KindRune, Rune: 'g'})
	res := l.Handle(Event{Kind: KindOther}) // e.g. an arrow key or ctrl-combo
	if !res.Unbound || res.UnboundPending != "g" {
		t.Fatalf("Result = %+v, want Unbound with UnboundPending %q", res, "g")
	}
	if _, open := l.Pending(); open {
		t.Fatal("an unbound key must close the pending chord")
	}
}

func TestLeaderBackspaceGoesUpOneLevel(t *testing.T) {
	l := NewLeader()
	l.MustRegisterGroup("g", "groups")
	l.MustRegister("ga", "add group", func() {})

	l.Handle(Event{Kind: KindRune, Rune: ' '})
	l.Handle(Event{Kind: KindRune, Rune: 'g'})
	if res := l.Handle(Event{Kind: KindBackspace}); !res.Consumed || res.Unbound {
		t.Fatalf("Backspace in a group: Result = %+v", res)
	}
	if keys, open := l.Pending(); !open || keys != "" {
		t.Fatalf("Backspace in a group: Pending() = %q,%v; want \"\",true", keys, open)
	}
	if res := l.Handle(Event{Kind: KindBackspace}); !res.Consumed {
		t.Fatalf("Backspace at the top level: Result = %+v", res)
	}
	if _, open := l.Pending(); open {
		t.Fatal("Backspace at the top level must close the chord")
	}
}

func TestLeaderSkipWhenTyping(t *testing.T) {
	l, _ := newTestLeader(t)
	l.SkipWhen = func() bool { return true }
	ev := Event{Kind: KindRune, Rune: ' '}
	if res := l.Handle(ev); res.Consumed {
		t.Fatal("space must type a space while an input has focus")
	}
}

func TestRegisterConflicts(t *testing.T) {
	l := NewLeader()
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

func TestRegisterGroupConflicts(t *testing.T) {
	l := NewLeader()
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

func TestEntriesUnnamedGroup(t *testing.T) {
	l := NewLeader()
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
	l := NewLeader()
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

func TestGenerationGuardsStaleTimeout(t *testing.T) {
	l, fired := newTestLeader(t)
	l.Handle(Event{Kind: KindRune, Rune: ' '})
	gen := l.Generation()

	// A key arrives before the stale timer fires: generation moves on.
	l.Handle(Event{Kind: KindRune, Rune: 'r'})
	if len(*fired) != 1 {
		t.Fatalf("fired = %v, want [r]", *fired)
	}
	if l.ResetIfGeneration(gen) {
		t.Fatal("a timer from a superseded generation must not reset live state")
	}

	// A timer that fires against the CURRENT generation does reset.
	l.Handle(Event{Kind: KindRune, Rune: ' '})
	if !l.ResetIfGeneration(l.Generation()) {
		t.Fatal("a timer at the current generation must reset the pending chord")
	}
	if _, open := l.Pending(); open {
		t.Fatal("ResetIfGeneration at the current generation must close the chord")
	}
}

package core

import "testing"

func TestContentClosedWhenIdle(t *testing.T) {
	l := NewLeader()
	if content, open := l.Content(); open || content.Title != "" || len(content.Lines) != 0 {
		t.Fatalf("Content() while idle = %+v, %v; want zero value, false", content, open)
	}
}

func TestContentTopLevel(t *testing.T) {
	l := NewLeader()
	l.MustRegister("r", "refresh", func() {})
	l.MustRegisterGroup("g", "groups")
	l.MustRegister("ga", "add group", func() {})
	l.Handle(Event{Kind: KindRune, Rune: ' '})

	content, open := l.Content()
	if !open {
		t.Fatal("Content() must report open once Space is pressed")
	}
	if content.Title != " <space> " {
		t.Fatalf("Title = %q, want %q", content.Title, " <space> ")
	}
	want := []TrayLine{
		{Key: "r", Desc: "refresh"},
		{Key: "g", Desc: "+groups", Group: true},
	}
	if len(content.Lines) != len(want) {
		t.Fatalf("Lines = %+v, want %+v", content.Lines, want)
	}
	for i := range want {
		if content.Lines[i] != want[i] {
			t.Fatalf("Lines[%d] = %+v, want %+v", i, content.Lines[i], want[i])
		}
	}
}

func TestContentUnnamedGroupFallsBackToEllipsis(t *testing.T) {
	l := NewLeader()
	l.MustRegister("ga", "add", func() {})
	l.MustRegister("gb", "remove", func() {}) // "g" is a group with no RegisterGroup call: unnamed
	l.Handle(Event{Kind: KindRune, Rune: ' '})

	content, _ := l.Content()
	if len(content.Lines) != 1 || content.Lines[0].Desc != "+…" {
		t.Fatalf("Lines = %+v, want a single unnamed-group line \"+…\"", content.Lines)
	}
}

func TestContentEmptyLevelShowsPlaceholder(t *testing.T) {
	l := NewLeader()
	l.MustRegister("r", "refresh", func() {})
	l.Handle(Event{Kind: KindRune, Rune: ' '})
	l.Handle(Event{Kind: KindRune, Rune: 'z'}) // unbound: closes the chord

	// Re-open at the top level: still just "r", nothing under an
	// unregistered prefix. Drive into a nonexistent branch is impossible
	// through Handle (there is none), so exercise Content directly via a
	// leader with no bindings at all instead.
	empty := NewLeader()
	empty.Handle(Event{Kind: KindRune, Rune: ' '})
	content, open := empty.Content()
	if !open {
		t.Fatal("Content() must stay open with no bindings registered")
	}
	if len(content.Lines) != 1 || content.Lines[0].Desc != "(no leader bindings)" {
		t.Fatalf("Lines = %+v, want the no-bindings placeholder", content.Lines)
	}
}

func TestContentTitleBreadcrumb(t *testing.T) {
	l := NewLeader()
	l.MustRegisterGroup("g", "groups")
	l.MustRegister("ga", "add group", func() {})
	l.Handle(Event{Kind: KindRune, Rune: ' '})
	l.Handle(Event{Kind: KindRune, Rune: 'g'})

	content, _ := l.Content()
	if content.Title != " <space> g " {
		t.Fatalf("Title = %q, want %q", content.Title, " <space> g ")
	}
}

func TestKeyLabelSpace(t *testing.T) {
	if got := keyLabel(' '); got != "SPC" {
		t.Fatalf("keyLabel(' ') = %q, want SPC", got)
	}
	if got := keyLabel('r'); got != "r" {
		t.Fatalf("keyLabel('r') = %q, want r", got)
	}
}

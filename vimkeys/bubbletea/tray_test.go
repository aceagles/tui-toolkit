package bubbletea

import (
	"strings"
	"testing"
)

func TestRenderEmptyWhenIdle(t *testing.T) {
	l, _ := newTestLeader(t)
	if got := Render(l, DefaultTrayStyle); got != "" {
		t.Fatalf("Render() while idle = %q, want empty", got)
	}
}

func TestRenderShowsEntriesAndGroups(t *testing.T) {
	l, _ := newTestLeader(t)
	l.Update(keyMsg(' '))
	got := Render(l, DefaultTrayStyle)
	for _, want := range []string{"r", "refresh", "g", "groups", "<space>"} {
		if !strings.Contains(got, want) {
			t.Fatalf("Render() missing %q:\n%s", want, got)
		}
	}
}

func TestRenderMatchesUnderlyingContent(t *testing.T) {
	l, _ := newTestLeader(t)
	l.Update(keyMsg(' '))
	l.Update(keyMsg('g'))
	got := Render(l, DefaultTrayStyle)
	for _, want := range []string{"a", "add group", "x", "delete group"} {
		if !strings.Contains(got, want) {
			t.Fatalf("Render() at the group level missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "refresh") {
		t.Fatal("group-level render must not include the top-level entries")
	}
}

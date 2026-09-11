package bubbletea

import (
	"fmt"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// newWideLeader registers 24 chords, one per letter a-x, each with a
// purely-numeric, fixed-width description so a test can search rendered
// text for a key letter without it also matching inside some other
// entry's description (an English description like "leaf" would fail
// that for the key "a").
func newWideLeader(t *testing.T) *Leader {
	t.Helper()
	l := NewLeader(0)
	for i, k := range "abcdefghijklmnopqrstuvwx" {
		if err := l.Register(string(k), fmt.Sprintf("%019d", i), func() {}); err != nil {
			t.Fatal(err)
		}
	}
	l.Update(keyMsg(' '))
	return l
}

func TestRenderEmptyWhenIdle(t *testing.T) {
	l, _ := newTestLeader(t)
	if got := Render(l, DefaultTrayStyle, 40, 10); got != "" {
		t.Fatalf("Render() while idle = %q, want empty", got)
	}
}

func TestRenderEmptyWhenBudgetTooSmall(t *testing.T) {
	l, _ := newTestLeader(t)
	l.Update(keyMsg(' '))
	if got := Render(l, DefaultTrayStyle, 5, 10); got != "" {
		t.Fatalf("Render() at width 5 = %q, want empty", got)
	}
	if got := Render(l, DefaultTrayStyle, 40, 2); got != "" {
		t.Fatalf("Render() at maxHeight 2 = %q, want empty", got)
	}
}

func TestRenderShowsEntriesAndGroups(t *testing.T) {
	l, _ := newTestLeader(t)
	l.Update(keyMsg(' '))
	got := Render(l, DefaultTrayStyle, 40, 10)
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
	got := Render(l, DefaultTrayStyle, 40, 10)
	for _, want := range []string{"a", "add group", "x", "delete group"} {
		if !strings.Contains(got, want) {
			t.Fatalf("Render() at the group level missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "refresh") {
		t.Fatal("group-level render must not include the top-level entries")
	}
}

// assertLeftToRight checks that each key in order appears in line, each
// one starting no earlier than where the previous one ended - i.e. that
// the keys read left to right across the columns, not in some other
// order.
func assertLeftToRight(t *testing.T, line string, keys ...string) {
	t.Helper()
	pos := 0
	for _, k := range keys {
		idx := strings.Index(line[pos:], k)
		if idx < 0 {
			t.Fatalf("expected %q at or after column %d in:\n%q", k, pos, line)
		}
		pos += idx + len(k)
	}
}

// TestRenderAt104FitsWidthAndRows is the width Adam's pane actually uses,
// with a maxHeight short enough to matter: 24 entries need more rows than
// an unbounded layout would use, so this also exercises the row math at a
// size a real caller would hit, not just a synthetic one.
func TestRenderAt104FitsWidthAndRows(t *testing.T) {
	const width, maxHeight = 104, 8
	l := newWideLeader(t)
	got := Render(l, DefaultTrayStyle, width, maxHeight)
	lines := strings.Split(got, "\n")

	if len(lines) > maxHeight {
		t.Fatalf("Render produced %d lines, want at most maxHeight=%d:\n%s", len(lines), maxHeight, got)
	}
	for i, line := range lines {
		if w := lipgloss.Width(line); w != width {
			t.Fatalf("line %d width = %d, want %d:\n%q", i, w, width, line)
		}
	}

	top := ansi.Strip(lines[0])
	corner := strings.IndexRune(top, '┌')
	title := strings.Index(top, "<space>")
	if corner != 0 {
		t.Fatalf("top border does not open with the top-left corner:\n%q", top)
	}
	if title <= corner {
		t.Fatalf("title must sit inside the top border, after its corner:\n%q", top)
	}

	// 24 entries at this width lay out as 4 columns of 6 rows (KeyWidth=1,
	// DescWidth=19, so colWidth=22; see vimkeys/core.LayoutTray's column
	// formula). Column-major fill means the first content row holds each
	// column's first entry - a, g, m, s, left to right - and the last
	// content row holds each column's last - f, l, r, x.
	if len(lines) < 7 {
		t.Fatalf("expected 6 content rows between the borders, got %d lines total:\n%s", len(lines), got)
	}
	assertLeftToRight(t, ansi.Strip(lines[1]), "a", "g", "m", "s")
	assertLeftToRight(t, ansi.Strip(lines[6]), "f", "l", "r", "x")
}

// TestRenderDropsExcessRowsAndSaysSo exercises the row cap with a
// maxHeight too short for all 24 entries, and checks the bottom border
// names how many did not fit - vimkeys/tview can afford to drop these
// silently because the rest of its tray is still visible on a static
// screen, but a bubbletea View is rebuilt from scratch each render, so
// Render says so instead.
func TestRenderDropsExcessRowsAndSaysSo(t *testing.T) {
	const width, maxHeight = 104, 5 // rows budget = maxHeight-2 = 3
	l := newWideLeader(t)
	got := Render(l, DefaultTrayStyle, width, maxHeight)
	lines := strings.Split(got, "\n")

	if len(lines) != maxHeight {
		t.Fatalf("got %d lines, want exactly maxHeight=%d (3 content rows + 2 borders)", len(lines), maxHeight)
	}
	// cols=4, rows capped at 3 -> 12 of the 24 entries are shown, 12 dropped.
	bottom := ansi.Strip(lines[len(lines)-1])
	if !strings.Contains(bottom, "12 more") {
		t.Fatalf("bottom border does not say what was dropped:\n%q", bottom)
	}
}

// TestRenderIgnoresBoxBorderAndPadding pins that a Box style carrying its
// own lipgloss border and padding (the natural way to write a tray style
// before Render drew its own box) still yields exactly one box: every line
// exactly width cells, no rounded-corner glyphs from the caller's border,
// and the height within maxHeight.
func TestRenderIgnoresBoxBorderAndPadding(t *testing.T) {
	l := newWideLeader(t)
	style := DefaultTrayStyle
	style.Box = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#7aa2f7")).
		Padding(0, 1)

	const width, maxHeight = 80, 12
	out := Render(l, style, width, maxHeight)
	lines := strings.Split(ansi.Strip(out), "\n")
	if len(lines) > maxHeight {
		t.Fatalf("Render() is %d lines, want <= %d:\n%s", len(lines), maxHeight, ansi.Strip(out))
	}
	for i, line := range lines {
		if w := lipgloss.Width(line); w != width {
			t.Errorf("line %d is %d cells, want %d: %q", i, w, width, line)
		}
		if strings.ContainsAny(line, "╭╮╰╯") {
			t.Errorf("line %d carries the caller's rounded border: %q", i, line)
		}
	}
	raw := strings.Split(out, "\n")
	if bottom := raw[len(raw)-1]; !strings.Contains(bottom, "\x1b[") {
		t.Errorf("bottom border %q carries no colour; BorderForeground should colour the glyphs", bottom)
	}
}

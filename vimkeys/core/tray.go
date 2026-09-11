package core

import (
	"strings"
	"unicode/utf8"
)

// TrayLine is one row of a which-key tray: the key label as shown to the
// user (e.g. "r", "SPC" for Space), its description — already carrying the
// "+" group prefix and "…" unnamed-group fallback — and whether it names a
// group rather than a leaf.
type TrayLine struct {
	Key   string
	Desc  string
	Group bool
}

// TrayContent is the framework-free content of a which-key tray for the
// currently pending chord: the lines to show, in order, plus the widths a
// simple monospace renderer would need to align the key and description
// columns, and the breadcrumb title (" <space> g s ").
//
// KeyWidth and DescWidth are measured in runes over the plain (unescaped)
// text. That is enough for a renderer with no markup-escaping concerns of
// its own (the bubbletea adapter uses it directly); a renderer that must
// escape its content first (tview's Print treats "[" as markup) should
// re-measure the escaped strings itself rather than trust these widths.
type TrayContent struct {
	Lines               []TrayLine
	KeyWidth, DescWidth int
	Title               string
}

// Content computes the tray content for the leader's current pending
// chord. open is false when no chord is pending, in which case Content is
// the zero value and a renderer should draw nothing.
func (l *Leader) Content() (content TrayContent, open bool) {
	keys, open := l.Pending()
	if !open {
		return TrayContent{}, false
	}

	var lines []TrayLine
	keyW, descW := 1, 0
	for _, e := range l.Entries(keys) {
		line := TrayLine{Key: keyLabel(e.Key), Desc: e.Desc, Group: e.Group}
		if e.Group {
			if line.Desc == "" {
				line.Desc = "…"
			}
			line.Desc = "+" + line.Desc
		}
		keyW = max(keyW, utf8.RuneCountInString(line.Key))
		descW = max(descW, utf8.RuneCountInString(line.Desc))
		lines = append(lines, line)
	}
	if len(lines) == 0 {
		lines = []TrayLine{{Desc: "(no leader bindings)"}}
		descW = utf8.RuneCountInString(lines[0].Desc)
	}

	return TrayContent{Lines: lines, KeyWidth: keyW, DescWidth: descW, Title: trayTitle(keys)}, true
}

// trayTitle is the breadcrumb: " <space> g s ".
func trayTitle(keys string) string {
	var b strings.Builder
	b.WriteString(" <space>")
	for _, r := range keys {
		b.WriteString(" " + keyLabel(r))
	}
	b.WriteString(" ")
	return b.String()
}

// keyLabel names a key for display; Space would otherwise be invisible.
func keyLabel(r rune) string {
	if r == ' ' {
		return "SPC"
	}
	return string(r)
}

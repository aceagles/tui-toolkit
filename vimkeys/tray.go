package vimkeys

import (
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// trayGap is the blank cells between tray columns.
const trayGap = 3

// Tray wraps an application root and, while a leader chord is pending,
// draws a which-key panel over the bottom of it listing the keys that can
// come next. Mount it in place of the root:
//
//	app.SetRoot(vimkeys.NewTray(leader, root), true)
//
// The tray only draws; keys still go through Leader.InputCapture, so a
// chord typed quickly dispatches exactly as without a tray. Mounting a
// tray turns off the leader timeout: the tray stays until a key or Esc.
//
// Every other Primitive method passes through to the wrapped root, so the
// root keeps its focus, mouse and paste handling.
type Tray struct {
	tview.Primitive
	leader *Leader
	box    *tview.Box
}

// NewTray wraps root so it shows l's pending chord.
func NewTray(l *Leader, root tview.Primitive) *Tray {
	l.sticky = true
	return &Tray{Primitive: root, leader: l, box: tview.NewBox().SetBorder(true)}
}

// Draw draws the root, then the tray over its bottom rows when a chord is
// pending. Rows that do not fit the root's height are dropped.
func (t *Tray) Draw(screen tcell.Screen) {
	t.Primitive.Draw(screen)
	keys, open := t.leader.Pending()
	if !open {
		return
	}
	x, y, w, h := t.GetRect()
	if w < 6 || h < 3 {
		return
	}

	type line struct {
		key, desc string
		group     bool
	}
	var lines []line
	keyW, descW := 1, 0
	for _, e := range t.leader.Entries(keys) {
		l := line{key: tview.Escape(keyLabel(e.Key)), desc: e.Desc, group: e.Group}
		if e.Group {
			if l.desc == "" {
				l.desc = "…"
			}
			l.desc = "+" + l.desc
		}
		l.desc = tview.Escape(l.desc)
		keyW = max(keyW, tview.TaggedStringWidth(l.key))
		descW = max(descW, tview.TaggedStringWidth(l.desc))
		lines = append(lines, l)
	}
	if len(lines) == 0 {
		lines = []line{{desc: "(no leader bindings)"}}
		descW = tview.TaggedStringWidth(lines[0].desc)
	}

	innerX, innerW := x+2, w-4 // border plus one cell of padding each side
	colW := keyW + 2 + descW
	cols := max(1, (innerW+trayGap)/(colW+trayGap))
	rows := min((len(lines)+cols-1)/cols, h-2)
	if cols == 1 {
		colW = innerW // one column: let long descriptions use the width
	}

	top := y + h - rows - 2
	t.box.SetRect(x, top, w, rows+2)
	t.box.SetBackgroundColor(tview.Styles.PrimitiveBackgroundColor).
		SetBorderColor(tview.Styles.BorderColor).
		SetTitleColor(tview.Styles.TitleColor).
		SetTitle(tview.Escape(trayTitle(keys))).
		SetTitleAlign(tview.AlignLeft)
	t.box.Draw(screen)

	for i, l := range lines {
		col, row := i/rows, i%rows
		if col >= cols {
			break
		}
		cx, cy := innerX+col*(colW+trayGap), top+1+row
		avail := min(colW, innerX+innerW-cx)
		tview.Print(screen, l.key, cx, cy, avail, tview.AlignLeft, tview.Styles.SecondaryTextColor)
		descColor := tview.Styles.PrimaryTextColor
		if l.group {
			descColor = tview.Styles.TertiaryTextColor
		}
		tview.Print(screen, l.desc, cx+keyW+2, cy, avail-keyW-2, tview.AlignLeft, descColor)
	}
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

package tview

import (
	"github.com/gdamore/tcell/v2"
	rtview "github.com/rivo/tview"
)

// trayGap is the blank cells between tray columns.
const trayGap = 3

// Tray wraps an application root and, while a leader chord is pending,
// draws a which-key panel over the bottom of it listing the keys that can
// come next. Mount it in place of the root:
//
//	app.SetRoot(tview.NewTray(leader, root), true)
//
// The tray only draws; keys still go through Leader.InputCapture, so a
// chord typed quickly dispatches exactly as without a tray. Mounting a
// tray sets core.Leader.Sticky, which turns off the leader's timeout: the
// tray stays until a key or Esc.
//
// Every other Primitive method passes through to the wrapped root, so the
// root keeps its focus, mouse and paste handling.
//
// Draw itself only lays out and paints core.Leader.Content's lines — the
// line ordering, grouping and "(no leader bindings)" fallback are computed
// once in vimkeys/core, so the bubbletea tray renders the identical content
// with no risk of the two drifting apart.
type Tray struct {
	rtview.Primitive
	leader *Leader
	box    *rtview.Box
}

// NewTray wraps root so it shows l's pending chord.
func NewTray(l *Leader, root rtview.Primitive) *Tray {
	l.Sticky = true
	return &Tray{Primitive: root, leader: l, box: rtview.NewBox().SetBorder(true)}
}

// Draw draws the root, then the tray over its bottom rows when a chord is
// pending. Rows that do not fit the root's height are dropped.
func (t *Tray) Draw(screen tcell.Screen) {
	t.Primitive.Draw(screen)
	content, open := t.leader.Content()
	if !open {
		return
	}
	x, y, w, h := t.GetRect()
	if w < 6 || h < 3 {
		return
	}

	// tview.Print treats "[" as the start of a colour tag, so the content's
	// plain text is escaped before measuring or drawing it; that escaping
	// can change display width (tview.Escape doubles a literal "["), which
	// is why the widths are remeasured here with tview.TaggedStringWidth
	// rather than trusting content.KeyWidth/DescWidth.
	type line struct {
		key, desc string
		group     bool
	}
	lines := make([]line, len(content.Lines))
	keyW, descW := 1, 0
	for i, cl := range content.Lines {
		l := line{key: rtview.Escape(cl.Key), desc: rtview.Escape(cl.Desc), group: cl.Group}
		keyW = max(keyW, rtview.TaggedStringWidth(l.key))
		descW = max(descW, rtview.TaggedStringWidth(l.desc))
		lines[i] = l
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
	t.box.SetBackgroundColor(rtview.Styles.PrimitiveBackgroundColor).
		SetBorderColor(rtview.Styles.BorderColor).
		SetTitleColor(rtview.Styles.TitleColor).
		SetTitle(rtview.Escape(content.Title)).
		SetTitleAlign(rtview.AlignLeft)
	t.box.Draw(screen)

	for i, l := range lines {
		col, row := i/rows, i%rows
		if col >= cols {
			break
		}
		cx, cy := innerX+col*(colW+trayGap), top+1+row
		avail := min(colW, innerX+innerW-cx)
		rtview.Print(screen, l.key, cx, cy, avail, rtview.AlignLeft, rtview.Styles.SecondaryTextColor)
		descColor := rtview.Styles.PrimaryTextColor
		if l.group {
			descColor = rtview.Styles.TertiaryTextColor
		}
		rtview.Print(screen, l.desc, cx+keyW+2, cy, avail-keyW-2, rtview.AlignLeft, descColor)
	}
}

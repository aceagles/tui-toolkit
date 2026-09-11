package tview

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	rtview "github.com/rivo/tview"
)

// oldTrayGap and oldTray are the tray.go Draw algorithm exactly as it
// stood before its column/row math moved into vimkeys/core.LayoutTray —
// kept here only as a golden reference. TestTrayVisualUnchanged renders
// this alongside the real Tray from the same Leader state at several
// widths and heights and requires byte-for-byte identical simulation
// screens, proving the extraction changed no cell of what modbustui draws.
const oldTrayGap = 3

type oldTray struct {
	rtview.Primitive
	leader *Leader
	box    *rtview.Box
}

func newOldTray(l *Leader, root rtview.Primitive) *oldTray {
	return &oldTray{Primitive: root, leader: l, box: rtview.NewBox().SetBorder(true)}
}

func (t *oldTray) Draw(screen tcell.Screen) {
	t.Primitive.Draw(screen)
	content, open := t.leader.Content()
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
	lines := make([]line, len(content.Lines))
	keyW, descW := 1, 0
	for i, cl := range content.Lines {
		l := line{key: rtview.Escape(cl.Key), desc: rtview.Escape(cl.Desc), group: cl.Group}
		keyW = max(keyW, rtview.TaggedStringWidth(l.key))
		descW = max(descW, rtview.TaggedStringWidth(l.desc))
		lines[i] = l
	}

	innerX, innerW := x+2, w-4
	colW := keyW + 2 + descW
	cols := max(1, (innerW+oldTrayGap)/(colW+oldTrayGap))
	rows := min((len(lines)+cols-1)/cols, h-2)
	if cols == 1 {
		colW = innerW
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
		cx, cy := innerX+col*(colW+oldTrayGap), top+1+row
		avail := min(colW, innerX+innerW-cx)
		rtview.Print(screen, l.key, cx, cy, avail, rtview.AlignLeft, rtview.Styles.SecondaryTextColor)
		descColor := rtview.Styles.PrimaryTextColor
		if l.group {
			descColor = rtview.Styles.TertiaryTextColor
		}
		rtview.Print(screen, l.desc, cx+keyW+2, cy, avail-keyW-2, rtview.AlignLeft, descColor)
	}
}

// renderRows draws p on a w x h simulation screen and returns its cell
// contents as one string per row.
func renderRows(t *testing.T, p rtview.Primitive, w, h int) []string {
	t.Helper()
	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	defer s.Fini()
	s.SetSize(w, h)
	p.SetRect(0, 0, w, h)
	p.Draw(s)
	s.Show()
	cells, gotW, gotH := s.GetContents()
	rows := make([]string, gotH)
	for y := range gotH {
		var b strings.Builder
		for x := range gotW {
			c := cells[y*gotW+x]
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

// TestTrayVisualUnchanged proves LayoutTray's extraction into
// vimkeys/core carries no visual change: rendering the same Leader state
// through the real Tray and through oldTray (the pre-extraction Draw,
// preserved above) must produce identical screens. It sweeps widths either
// side of column-count breakpoints and a range of heights, including ones
// narrow enough to force the one-column rule and the row cap.
func TestTrayVisualUnchanged(t *testing.T) {
	for _, tc := range []struct{ w, h int }{
		{22, 10}, {23, 10}, // either side of a column-count breakpoint
		{40, 12},
		{60, 15},
		{80, 20},
		{104, 8}, // Adam's pane width, short enough to hit the row cap
		{104, 24},
	} {
		t.Run(fmt.Sprintf("%dx%d", tc.w, tc.h), func(t *testing.T) {
			app := rtview.NewApplication()
			l := NewLeader(app, 0)
			for _, k := range "abcdefghijklmnopqrstuvwx" {
				k := k
				if err := l.Register(string(k), "leaf description "+string(k), func() {}); err != nil {
					t.Fatal(err)
				}
			}
			l.InputCapture(tcell.NewEventKey(tcell.KeyRune, ' ', tcell.ModNone))

			newTray := NewTray(l, rtview.NewBox())
			oldT := newOldTray(l, rtview.NewBox())

			gotNew := renderRows(t, newTray, tc.w, tc.h)
			gotOld := renderRows(t, oldT, tc.w, tc.h)
			if !reflect.DeepEqual(gotNew, gotOld) {
				t.Fatalf("Tray rendering diverged from the pre-extraction algorithm at %dx%d:\nold:\n%s\nnew:\n%s",
					tc.w, tc.h, strings.Join(gotOld, "\n"), strings.Join(gotNew, "\n"))
			}
		})
	}
}

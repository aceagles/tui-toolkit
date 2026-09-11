// Package confirm is the dialogue that stands between a keypress and a
// live system. It lists exactly what will be sent as labelled lines, and
// Cancel holds focus, so Enter on a dialogue that has just appeared backs
// out. Esc cancels. Nothing is sent unless the tool's OnConfirm runs.
//
//	confirm.Show(app, pages, confirm.Dialog{
//		Title:  "CONFIRM PUT",
//		Method: "PUT /limits/export",
//		Lines:  []confirm.Line{{"dest", "127.0.0.1:8080"}, {"body", `{"kw": 42}`}},
//		OnConfirm: send, OnCancel: logCancel,
//	})
//
// Colours come from theme.Current: the body on RoleBgFloat, the border and
// title in RoleError.
package confirm

import (
	"fmt"
	"strings"

	"github.com/aceagles/tui-toolkit/theme"
	"github.com/rivo/tview"
)

// PageName is the Pages name the dialogue occupies while open.
const PageName = "confirm"

// defaultWidth fits a 64-bit value in hex and decimal with room to spare.
const defaultWidth = 78

// Line is one labelled row. A Value containing newlines continues on the
// following rows, aligned under the value column. Both are shown as plain
// text: colour tags are escaped.
type Line struct {
	Label string
	Value string
}

// Dialog is one confirmation.
type Dialog struct {
	Title   string // box title; " — nothing has been sent yet" is appended
	Warning string // optional first row in RoleError, e.g. "remote target"
	Method  string // what will be sent, e.g. "SET key" or "FC16 Write Multiple Registers"
	Lines   []Line // wire fields, destination, key, typed value…

	ConfirmLabel string // confirm button label; "SEND" when empty
	Width        int    // dialogue width in cells; 78 when zero

	OnConfirm func() // runs after the dialogue closes, only on the confirm button
	OnCancel  func() // runs after the dialogue closes on Cancel or Esc
}

// Show adds the dialogue to pages as PageName and focuses its Cancel
// button. Call it on the event loop.
func Show(app *tview.Application, pages *tview.Pages, d Dialog) {
	p := theme.Current()
	body := Body(d, p)

	closeWith := func(fn func()) {
		pages.RemovePage(PageName)
		if fn != nil {
			fn()
		}
	}

	text := tview.NewTextView().SetDynamicColors(true).SetText(body)
	text.SetTextColor(p.Color(theme.RoleFg)).SetBackgroundColor(p.Color(theme.RoleBgFloat))

	label := d.ConfirmLabel
	if label == "" {
		label = "SEND"
	}
	form := tview.NewForm().SetButtonsAlign(tview.AlignCenter)
	form.SetBackgroundColor(p.Color(theme.RoleBgFloat))
	form.AddButton("Cancel", func() { closeWith(d.OnCancel) })
	form.AddButton(label, func() { closeWith(d.OnConfirm) })
	form.SetCancelFunc(func() { closeWith(d.OnCancel) })
	form.SetFocus(0) // Cancel

	frame := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(text, 0, 1, false).
		AddItem(form, 3, 0, true)
	frame.SetBorder(true).
		SetBorderColor(p.Color(theme.RoleError)).
		SetTitleColor(p.Color(theme.RoleError)).
		SetTitle(" " + tview.Escape(d.Title) + " — nothing has been sent yet ").
		SetBackgroundColor(p.Color(theme.RoleBgFloat))

	width := d.Width
	if width <= 0 {
		width = defaultWidth
	}
	height := strings.Count(body, "\n") + 1 + 6 // body, border, buttons, one spare row
	pages.AddPage(PageName, centred(frame, width, height), true, true)
	app.SetFocus(form)
}

// Body renders the dialogue text in p's colours: the warning, the method,
// then one "label : value" row per line.
func Body(d Dialog, p theme.Palette) string {
	var b strings.Builder
	if d.Warning != "" {
		fmt.Fprintf(&b, "%s[::b]%s[-::-]\n\n", p.Tag(theme.RoleError), tview.Escape(d.Warning))
	}
	if d.Method != "" {
		fmt.Fprintf(&b, "%s[::b]%s[-::-]\n\n", p.Tag(theme.RoleTitle), tview.Escape(d.Method))
	}
	labelW := 0
	for _, l := range d.Lines {
		labelW = max(labelW, tview.TaggedStringWidth(tview.Escape(l.Label)))
	}
	indent := strings.Repeat(" ", 2+labelW+3)
	for _, l := range d.Lines {
		label := tview.Escape(l.Label)
		pad := strings.Repeat(" ", labelW-tview.TaggedStringWidth(label))
		value := strings.ReplaceAll(tview.Escape(l.Value), "\n", "\n"+indent)
		fmt.Fprintf(&b, "  %s%s%s :[-] %s\n", p.Tag(theme.RoleFgMuted), label, pad, value)
	}
	return strings.TrimRight(b.String(), "\n")
}

// centred wraps p in a fixed-size box in the middle of the screen.
func centred(p tview.Primitive, width, height int) tview.Primitive {
	return tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(p, height, 0, true).
			AddItem(nil, 0, 1, false), width, 0, true).
		AddItem(nil, 0, 1, false)
}

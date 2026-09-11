// Package theme colours workbench TUIs with tokyonight, the same palette as
// the editor next to them, and follows tmux's dark/light toggle live.
//
// Tools colour by Role (RoleWarn, RoleBorderFocus, …), never by colour
// name. Apply sets tview.Styles so stock widgets, and anything else that
// reads Styles at draw time, pick the palette up; Palette.Tag gives the
// matching tview colour tag for tagged text. Wiring:
//
//	theme.EnableTrueColor()
//	theme.Apply(theme.Detect())
//	defer theme.Watch(app, 0, func(p theme.Palette) { theme.Restyle(pages, p); redraw() })()
//
// Build widgets after Apply: tview copies Styles into a widget when it is
// constructed, so Restyle can only remap colours that came from a palette.
package theme

import (
	"sync"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

var (
	mu      sync.RWMutex
	current = Night
)

// Current returns the palette last passed to Apply, or Night before the
// first call.
func Current() Palette {
	mu.RLock()
	defer mu.RUnlock()
	return current
}

// Apply makes p the current palette and sets tview.Styles from it. Call it
// before building widgets, and on the event loop once the app is running
// (Watch does this for you).
func Apply(p Palette) {
	tview.Styles = tview.Theme{
		PrimitiveBackgroundColor:    p.Color(RoleBg),
		ContrastBackgroundColor:     p.Color(RoleBgContrast),
		MoreContrastBackgroundColor: p.Color(RoleBgFloat),
		BorderColor:                 p.Color(RoleBorder),
		TitleColor:                  p.Color(RoleTitle),
		GraphicsColor:               p.Color(RoleBorder),
		PrimaryTextColor:            p.Color(RoleFg),
		SecondaryTextColor:          p.Color(RoleAccent),
		TertiaryTextColor:           p.Color(RoleTitle),
		InverseTextColor:            p.Color(RoleBg),
		ContrastSecondaryTextColor:  p.Color(RoleFgMuted),
	}
	mu.Lock()
	current = p
	mu.Unlock()
}

// Restyle recolours an existing primitive tree for p, for use after a live
// palette switch. It walks Flex, Pages (hidden pages too), Frame and Form
// children, and any container with Flex's GetItemCount and GetItem (such
// as layout.Stack); Grid and other custom containers must be passed
// separately.
//
// Box backgrounds and borders are remapped role for role, so a dialogue on
// RoleBgFloat stays on RoleBgFloat and a RoleBorderFocus border stays
// focused; colours from no palette are left alone. Things tview has no
// getter for take p's defaults: titles become RoleTitle, TextView text
// RoleFg, table selection RoleSelection, and form fields and buttons the
// stock look Apply gives new widgets. Text carrying colour tags and table
// cells keep their colours until the tool re-renders them.
//
// Call it on the event loop.
func Restyle(root tview.Primitive, p Palette) {
	walk(root, func(prim tview.Primitive) { restyle(prim, p) })
}

// container is any primitive that lists its children the way tview.Flex
// does, such as layout.Stack.
type container interface {
	GetItemCount() int
	GetItem(int) tview.Primitive
}

func walk(prim tview.Primitive, fn func(tview.Primitive)) {
	if prim == nil {
		return
	}
	fn(prim)
	switch c := prim.(type) {
	case container: // tview.Flex, layout.Stack
		for i := range c.GetItemCount() {
			walk(c.GetItem(i), fn)
		}
	case *tview.Pages:
		for _, name := range c.GetPageNames(false) {
			walk(c.GetPage(name), fn)
		}
	case *tview.Frame:
		walk(c.GetPrimitive(), fn)
	case *tview.Form:
		for i := range c.GetFormItemCount() {
			walk(c.GetFormItem(i), fn)
		}
	}
}

// box is the colour surface every tview widget inherits from *tview.Box.
type box interface {
	GetBackgroundColor() tcell.Color
	SetBackgroundColor(tcell.Color) *tview.Box
	GetBorderColor() tcell.Color
	SetBorderColor(tcell.Color) *tview.Box
	SetTitleColor(tcell.Color) *tview.Box
}

func restyle(prim tview.Primitive, p Palette) {
	if b, ok := prim.(box); ok {
		b.SetBackgroundColor(remap(b.GetBackgroundColor(), p))
		b.SetBorderColor(remap(b.GetBorderColor(), p))
		b.SetTitleColor(p.Color(RoleTitle))
	}
	// The same styles tview's constructors derive from the Styles Apply sets.
	on := func(fg, bg Role) tcell.Style {
		return tcell.StyleDefault.Foreground(p.Color(fg)).Background(p.Color(bg))
	}
	field := on(RoleFg, RoleBgContrast)
	switch w := prim.(type) {
	case *tview.TextView:
		w.SetTextColor(p.Color(RoleFg))
	case *tview.Table:
		w.SetSelectedStyle(p.Style(RoleSelection))
		w.SetBordersColor(p.Color(RoleBorder))
	case *tview.Form:
		w.SetLabelColor(p.Color(RoleAccent)).
			SetFieldStyle(field).
			SetButtonStyle(field).
			SetButtonActivatedStyle(on(RoleBgContrast, RoleFg)).
			SetButtonDisabledStyle(on(RoleFgMuted, RoleBgContrast))
	case *tview.InputField:
		w.SetLabelColor(p.Color(RoleAccent)).
			SetFieldStyle(field).
			SetPlaceholderStyle(on(RoleFgMuted, RoleBgContrast))
	case *tview.DropDown:
		w.SetLabelColor(p.Color(RoleAccent)).
			SetFieldStyle(field).
			SetListStyles(on(RoleFg, RoleBgFloat), p.Style(RoleSelection))
	case *tview.Button:
		w.SetStyle(field).SetActivatedStyle(on(RoleBg, RoleFg))
	}
}

// remap returns the colour in p that has c's role in any known palette, or
// c itself when no palette uses it.
func remap(c tcell.Color, p Palette) tcell.Color {
	for _, from := range palettes {
		for r := range roleCount {
			if from.colors[r] == c {
				return p.colors[r]
			}
		}
	}
	return c
}

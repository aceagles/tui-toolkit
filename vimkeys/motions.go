// Package vimkeys makes tview widgets answer to vim motions and gives an
// application a Space-leader chord dispatcher, so a workbench TUI feels
// like another nvim split.
package vimkeys

import "github.com/gdamore/tcell/v2"

// Motions returns an input-capture translator for a scrollable widget:
//
//	j / k   → Down / Up
//	gg / G  → Home / End
//	C-d/C-u → PageDown / PageUp
//
// Attach per widget (each translator carries its own gg chord state):
//
//	list.SetInputCapture(vimkeys.Motions())
//
// Attach only to read/scroll widgets — on an InputField it would eat the
// letters. Anything it does not translate passes through, so widget keys
// (Enter, arrows, tool action keys) keep working.
func Motions() func(ev *tcell.EventKey) *tcell.EventKey {
	pendingG := false
	return func(ev *tcell.EventKey) *tcell.EventKey {
		key := func(k tcell.Key) *tcell.EventKey { return tcell.NewEventKey(k, 0, tcell.ModNone) }

		if ev.Key() == tcell.KeyRune && ev.Modifiers() == tcell.ModNone {
			switch ev.Rune() {
			case 'g':
				if pendingG {
					pendingG = false
					return key(tcell.KeyHome)
				}
				pendingG = true
				return nil // swallow the first g of a possible gg
			case 'G':
				pendingG = false
				return key(tcell.KeyEnd)
			case 'j':
				pendingG = false
				return key(tcell.KeyDown)
			case 'k':
				pendingG = false
				return key(tcell.KeyUp)
			}
		}
		pendingG = false
		switch ev.Key() {
		case tcell.KeyCtrlD:
			return key(tcell.KeyPgDn)
		case tcell.KeyCtrlU:
			return key(tcell.KeyPgUp)
		}
		return ev
	}
}

// Chain composes input captures left to right, stopping when one consumes
// (returns nil) or transforms the event. Useful for stacking Motions with a
// tool's own bindings on the same widget.
func Chain(caps ...func(*tcell.EventKey) *tcell.EventKey) func(*tcell.EventKey) *tcell.EventKey {
	return func(ev *tcell.EventKey) *tcell.EventKey {
		for _, c := range caps {
			out := c(ev)
			if out != ev {
				return out
			}
		}
		return ev
	}
}

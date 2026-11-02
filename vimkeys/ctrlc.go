package vimkeys

import "github.com/gdamore/tcell/v2"

// CtrlCAsEscape makes C-c behave as it does in vim: it breaks out of
// whatever is being edited, exactly like Esc, and never quits the app.
//
// tview stops the application on C-c unless the application's input capture
// returns something other than the event it was given. This wrapper turns
// C-c into an Esc event and hands that to next, so the leader, tmuxnav,
// dialogues, input fields and forms all see a plain Esc and cancel the same
// way. In normal mode Esc does whatever the focused widget does with it,
// which in a read-only pane is nothing.
//
// Wrap the whole app-level capture, not one link of a Chain: a Chain stops
// at the first link that changes the event, so the Esc would never reach the
// links after it. next may be nil.
//
//	app.SetInputCapture(vimkeys.CtrlCAsEscape(vimkeys.Chain(leader.InputCapture, nav.InputCapture)))
//
// This is the keyboard path only. In raw mode tcell delivers C-c as a key
// event, not SIGINT, so a `kill -INT` from outside still ends the process.
func CtrlCAsEscape(next func(*tcell.EventKey) *tcell.EventKey) func(*tcell.EventKey) *tcell.EventKey {
	return func(ev *tcell.EventKey) *tcell.EventKey {
		if ev.Key() == tcell.KeyCtrlC {
			ev = tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone)
		}
		if next == nil {
			return ev
		}
		return next(ev)
	}
}

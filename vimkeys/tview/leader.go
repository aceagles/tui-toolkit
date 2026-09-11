// Package tview adapts vimkeys/core's chord state machine to tview: an
// input capture that decodes *tcell.EventKey, a timeout timer driven by
// tview's own event loop, and a Tray that draws core's tray content with
// tview.Print.
//
// The package is named tview to match its sibling vimkeys/bubbletea, which
// means files in this package that need the real tview library import it
// under an alias (rtview) — otherwise "tview.Print" inside a file that is
// itself package tview reads as a call to its own package, when it is
// actually the vendored UI library. The alias makes the external call
// visually unambiguous; nothing about it is a compile-time necessity, since
// a package's own clause name never occupies an identifier within its
// files.
package tview

import (
	"time"

	"github.com/gdamore/tcell/v2"
	rtview "github.com/rivo/tview"

	"github.com/aceagles/tui-toolkit/vimkeys/core"
)

// Leader dispatches Space-leader chords at application level:
//
//	l := tview.NewLeader(app, 0)
//	l.Register("r", "[R]efresh now", refresh)
//	l.MustRegisterGroup("s", "show")
//	l.Register("sl", "[S]how [L]og pane", showLog)
//	app.SetInputCapture(l.InputCapture)
//
// Put the leader first in any input-capture chain so a pending chord owns
// every key. The chord semantics — timeout/sticky behaviour, the Register
// conflict rule, groups — live in vimkeys/core.Leader, embedded here; see
// its doc for that reasoning. This type adds only the three tview-specific
// seams: the *tview.Application (for focus introspection and thread-safe
// timer callbacks), the *tcell.EventKey decode, and OnUnbound's tcell event.
type Leader struct {
	*core.Leader
	app     *rtview.Application
	timeout time.Duration
	timer   *time.Timer

	// OnUnbound, when set, is called after a key that matched nothing
	// abandoned the chord. pending is the chord before that key ("" for a
	// key pressed straight after Space); ev is the key. Esc and Backspace
	// are navigation, not unbound keys, and are not reported.
	OnUnbound func(pending string, ev *tcell.EventKey)
}

// NewLeader builds a Leader. A zero timeout means 800ms. The timeout does
// not apply once a Tray is mounted (see core.Leader.Sticky).
func NewLeader(app *rtview.Application, timeout time.Duration) *Leader {
	if timeout == 0 {
		timeout = 800 * time.Millisecond
	}
	l := &Leader{Leader: core.NewLeader(), app: app, timeout: timeout}
	l.SkipWhen = func() bool { return FocusWantsSpace(app) }
	return l
}

// FocusWantsSpace reports that the focused widget uses Space itself: tview's
// InputField and TextArea type it, Checkbox toggles on it. Form fields are
// covered because a Form hands focus to the field.
func FocusWantsSpace(app *rtview.Application) bool {
	switch app.GetFocus().(type) {
	case *rtview.InputField, *rtview.TextArea, *rtview.Checkbox:
		return true
	}
	return false
}

// InputCapture consumes Space and the chord keys that follow. Everything
// else passes through untouched.
func (l *Leader) InputCapture(ev *tcell.EventKey) *tcell.EventKey {
	res := l.Handle(decode(ev))
	if res.Unbound && l.OnUnbound != nil {
		l.OnUnbound(res.UnboundPending, ev)
	}
	if !res.Consumed {
		return ev
	}
	if _, open := l.Pending(); open && !l.Sticky {
		l.scheduleTimeout()
	} else {
		l.cancelTimeout()
	}
	return nil
}

// scheduleTimeout (re)arms the timeout timer. It captures the leader's
// current generation so a timer left over from a chord that has since
// moved on is a no-op instead of resetting fresher state — see
// core.Leader.ResetIfGeneration.
func (l *Leader) scheduleTimeout() {
	l.cancelTimeout()
	gen := l.Generation()
	l.timer = time.AfterFunc(l.timeout, func() {
		l.app.QueueUpdateDraw(func() { l.ResetIfGeneration(gen) })
	})
}

func (l *Leader) cancelTimeout() {
	if l.timer != nil {
		l.timer.Stop()
		l.timer = nil
	}
}

// decode translates a tcell key event into a core.Event.
func decode(ev *tcell.EventKey) core.Event {
	switch ev.Key() {
	case tcell.KeyEscape:
		return core.Event{Kind: core.KindEscape}
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		return core.Event{Kind: core.KindBackspace}
	case tcell.KeyRune:
		if ev.Modifiers() == tcell.ModNone {
			return core.Event{Kind: core.KindRune, Rune: ev.Rune()}
		}
	}
	return core.Event{Kind: core.KindOther}
}

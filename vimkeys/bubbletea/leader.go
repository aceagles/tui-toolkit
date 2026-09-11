// Package bubbletea adapts vimkeys/core's chord state machine to
// bubbletea: translating a tea.KeyPressMsg into a core.Event, and
// rendering core's tray content as a string with lipgloss for composition
// into a View.
//
// The package is named bubbletea to match its sibling vimkeys/tview, which
// means files here that need the real bubbletea library import it under
// the conventional "tea" alias — the same alias bubbletea's own docs use —
// so "tea.KeyPressMsg" inside a file that is itself package bubbletea
// reads unambiguously as the external library, not a call back into this
// package. Nothing about the alias is load-bearing for compilation (a
// package's own clause name is never an identifier inside its files); it
// is there purely so a reader is never left wondering which "bubbletea"
// a call means.
package bubbletea

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/aceagles/tui-toolkit/vimkeys/core"
)

// comboMods are the modifiers that turn a keypress into something other
// than a plain chord character: Shift and the lock states just change
// which character was produced (already reflected in Key.Text), so they do
// not disqualify a rune from chord matching, matching tcell's convention
// that only an actual Ctrl/Alt/Meta combo carries a modifier.
const comboMods = tea.ModCtrl | tea.ModAlt | tea.ModMeta | tea.ModHyper | tea.ModSuper

// Leader dispatches Space-leader chords in a bubbletea program:
//
//	l := bubbletea.NewLeader(0)
//	l.Register("r", "[R]efresh now", refresh)
//	l.MustRegisterGroup("s", "show")
//	l.Register("sl", "[S]how [L]og pane", showLog)
//
//	func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
//	    if consumed, cmd := m.leader.Update(msg); consumed {
//	        return m, cmd
//	    }
//	    ... the model's own key handling ...
//	}
//
// Call Update first in the chain so a pending chord owns every key. The
// chord semantics live in vimkeys/core.Leader, embedded here; see its doc
// for the timeout/sticky reasoning and the Register conflict rule.
//
// Bubbletea has no goroutine safe to touch program state from outside
// Update, so unlike the tview adapter (which parks a real time.Timer on a
// background goroutine and hops back via QueueUpdateDraw), the chord
// timeout here is a tea.Tick command: arming or extending a chord returns
// a Cmd that, after the timeout, delivers a timeoutMsg back through
// Update. core.Leader's Generation guards it — a tick left over from a
// chord the user has since extended, dispatched or abandoned is a no-op.
type Leader struct {
	*core.Leader
	timeout time.Duration

	// OnUnbound, when set, is called after a key that matched nothing
	// abandoned the chord. pending is the chord before that key ("" for a
	// key pressed straight after Space); msg is the key. Esc and Backspace
	// are navigation, not unbound keys, and are not reported.
	OnUnbound func(pending string, msg tea.KeyPressMsg)
}

// NewLeader builds a Leader. A zero timeout means 800ms. The timeout does
// not apply once a tray is mounted (see core.Leader.Sticky).
func NewLeader(timeout time.Duration) *Leader {
	if timeout == 0 {
		timeout = 800 * time.Millisecond
	}
	return &Leader{Leader: core.NewLeader(), timeout: timeout}
}

// timeoutMsg is delivered by a scheduled tea.Tick when a chord's timeout
// elapses. gen pins it to the chord state at schedule time.
type timeoutMsg struct{ gen int }

// Update feeds msg to the leader when it is a tea.KeyPressMsg or a pending
// timeoutMsg. It reports whether the message was consumed — the caller's
// own Update should skip its normal key handling when true — and a Cmd to
// run (nil most of the time; a tea.Tick when a chord's timeout needs
// (re)arming). Any other message type is left untouched: consumed is
// false and cmd is nil.
func (l *Leader) Update(msg tea.Msg) (consumed bool, cmd tea.Cmd) {
	switch msg := msg.(type) {
	case timeoutMsg:
		l.ResetIfGeneration(msg.gen)
		return false, nil
	case tea.KeyPressMsg:
		res := l.Handle(decode(msg))
		if res.Unbound && l.OnUnbound != nil {
			l.OnUnbound(res.UnboundPending, msg)
		}
		if !res.Consumed {
			return false, nil
		}
		if _, open := l.Pending(); open && !l.Sticky {
			cmd = l.scheduleTimeout()
		}
		return true, cmd
	}
	return false, nil
}

// scheduleTimeout returns a Cmd that delivers a timeoutMsg pinned to the
// leader's current generation after the configured timeout.
func (l *Leader) scheduleTimeout() tea.Cmd {
	gen := l.Generation()
	return tea.Tick(l.timeout, func(time.Time) tea.Msg { return timeoutMsg{gen} })
}

// decode translates a bubbletea key press into a core.Event.
func decode(msg tea.KeyPressMsg) core.Event {
	key := msg.Key()
	switch key.Code {
	case tea.KeyEscape:
		return core.Event{Kind: core.KindEscape}
	case tea.KeyBackspace:
		return core.Event{Kind: core.KindBackspace}
	}
	if key.Mod&comboMods == 0 {
		// Text is populated only for keys that represent a printable
		// character (Space included: ultraviolet decodes it as
		// Key{Code: KeySpace, Text: " "}), which is exactly the set the
		// chord state machine can extend or match a binding with.
		if r := []rune(key.Text); len(r) == 1 {
			return core.Event{Kind: core.KindRune, Rune: r[0]}
		}
	}
	return core.Event{Kind: core.KindOther}
}

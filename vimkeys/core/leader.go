// Package core implements the vimkeys chord state machine: a Space-leader
// dispatcher, its which-key style tray content, and nothing else. It has no
// dependency on any terminal UI framework — see TestNoFrameworkDeps in
// nodeps_test.go, which enforces that with `go list -deps` so a future edit
// cannot quietly reach for tview, tcell or bubbletea here.
//
// A framework adapter (vimkeys/tview, vimkeys/bubbletea) translates its own
// key-event type into an Event, feeds it to Leader.Handle, and renders
// Leader.Content in whatever way suits that framework. Package core owns
// the chord semantics so the two adapters cannot drift apart.
//
// # Chords, groups and the tray
//
// Leader dispatches Space-leader chords: press Space, then a sequence of
// keys ("r", "sl", ...), and the matching Binding's Fn runs. A prefix
// shared by two or more chords is a group; RegisterGroup names one for
// display ("s  +show"), but naming is optional — an unnamed group still
// shows as "+…".
//
// Registering "s" and "sf" together is rejected: the shorter chord would
// shadow the longer, which vim resolves with timeoutlen ambiguity nobody
// enjoys debugging in a workbench tool. For the same reason a leaf may not
// sit on or above a named group, and vice versa.
//
// While a chord is pending: Esc abandons it, Backspace goes up one level,
// and any key that matches nothing abandons it and is reported through
// Handle's Result (Unbound, UnboundPending) — an adapter typically forwards
// that to its own OnUnbound callback along with the original framework
// event, since only the adapter still has that event in scope.
//
// # Timeouts belong to the adapter
//
// vim's leader has a timeoutlen: an idle pending chord eventually resets.
// Package core does not run that timer itself — a timer needs a
// framework-appropriate way to deliver its callback back onto the event
// loop (tview needs Application.QueueUpdateDraw; bubbletea needs a Msg
// round-tripped through Update), and core has no event loop of its own to
// offer one. Instead, an adapter that wants a timeout schedules its own
// timer using whatever primitive its framework provides, and calls
// ResetIfGeneration when it fires. Generation is bumped on every state
// change, so a timer scheduled before a newer key arrived is a no-op
// instead of clobbering fresher state — see Sticky for the tray's "no
// timeout while shown" behaviour, which an adapter checks before
// scheduling at all.
package core

import (
	"fmt"
	"strings"
)

// Binding is one leader chord: Keys are the runes after Space ("r", "sf"),
// Desc is the which-key style description shown in help.
type Binding struct {
	Keys string
	Desc string
	Fn   func()
}

// Entry is one line of the leader tree at some prefix: either a leaf that
// dispatches a Binding, or a group whose children sit one key further in.
type Entry struct {
	Key   rune   // the key to press at this level
	Keys  string // the full chord from Space, including Key
	Desc  string // leaf description, or group name ("" for an unnamed group)
	Group bool
}

// EventKind classifies a decoded key for the chord state machine. The
// machine only ever needs to ask three questions of a key — is it Escape,
// is it Backspace, and is it a single unmodified printable rune (which can
// extend or match a chord) — everything else behaves identically: it
// abandons a pending chord and is reported as unbound. KindOther covers
// that everything-else case (arrows, Enter, function keys, any modified
// key), so a framework adapter never has to teach core about a new key
// type.
type EventKind int

// Event kinds recognised by Leader.Handle.
const (
	// KindRune is a single unmodified printable character, Space included
	// (Rune == ' '). Ctrl/Alt/Meta combinations and shifted keys that are
	// not a plain character (e.g. Shift+Tab) are KindOther, not KindRune.
	KindRune EventKind = iota
	KindEscape
	KindBackspace
	// KindOther is any key that is not Escape, Backspace, or a plain
	// printable rune: arrows, Enter, function keys, ctrl/alt combinations.
	KindOther
)

// Event is a single decoded key handed to Leader.Handle. A framework
// adapter translates its own event type into one.
//
// Why a four-way Kind and not, say, a "ctrl+w"-style keystroke string: the
// state machine's entire vocabulary is "Escape", "Backspace", "a plain
// rune", and "anything else". A keystroke string would need re-parsing to
// answer those same four questions and would tie the wire format to one
// framework's naming (tcell renders an unbound Enter as "Enter", bubbletea
// as "enter"); Event keeps that decision in the adapter, where the
// framework's own types are already in scope, and keeps core from needing
// to know either naming convention.
type Event struct {
	Kind EventKind
	Rune rune // meaningful only when Kind == KindRune
}

// Result reports what Handle did with an Event, so an adapter knows how to
// respond.
type Result struct {
	// Consumed reports whether the leader owned the event. False only when
	// the leader was idle and the event was not a bare Space: the adapter
	// must let its framework handle the key normally.
	Consumed bool

	// Unbound reports that a key matched no binding and no navigation
	// action (Esc/Backspace), abandoning the pending chord. UnboundPending
	// is the chord that was pending immediately before this event ("" if
	// the very first key after Space was unbound). An adapter with an
	// OnUnbound-style callback checks this and calls its own, passing the
	// original framework event it still has in scope.
	Unbound        bool
	UnboundPending string
}

// Leader dispatches Space-leader chords. See the package doc for the chord,
// group and timeout model.
type Leader struct {
	bindings []Binding
	groups   []Binding // named prefixes; Fn is unused

	pending string
	armed   bool
	gen     int // bumped on every state change; see ResetIfGeneration

	// Sticky reports that a tray (or equivalent always-visible pending-chord
	// display) is mounted. An adapter checks it before scheduling a timeout
	// timer: with the pending chord always on screen there is nothing for a
	// timeout to protect the user from, so the chord waits for a key
	// instead of resetting like vim's timeoutlen.
	Sticky bool

	// SkipWhen reports that leader handling should stand aside: a widget
	// that wants Space has focus, or a modal is open. Framework-specific
	// focus introspection (e.g. tview's FocusWantsSpace) lives in the
	// adapter, which sets this by default and lets a tool compose onto it:
	//
	//	l.SkipWhen = func() bool { return modalOpen() || tview.FocusWantsSpace(app) }
	SkipWhen func() bool

	// OnState, when set, is called with the pending chord ("" when idle) so
	// the tool can render a pending-leader indicator.
	OnState func(pending string)
}

// NewLeader builds an idle Leader.
func NewLeader() *Leader {
	return &Leader{}
}

// Register adds a chord. Registering "s" and "sf" together is rejected:
// the shorter chord would shadow the longer, which vim resolves with
// timeoutlen ambiguity nobody enjoys debugging in a workbench tool. For
// the same reason a leaf may not sit on or above a named group.
func (l *Leader) Register(keys, desc string, fn func()) error {
	if keys == "" || fn == nil {
		return fmt.Errorf("vimkeys: empty binding")
	}
	for _, b := range l.bindings {
		if strings.HasPrefix(b.Keys, keys) || strings.HasPrefix(keys, b.Keys) {
			return fmt.Errorf("vimkeys: %q conflicts with registered %q", keys, b.Keys)
		}
	}
	for _, g := range l.groups {
		if strings.HasPrefix(g.Keys, keys) {
			return fmt.Errorf("vimkeys: %q conflicts with group %q", keys, g.Keys)
		}
	}
	l.bindings = append(l.bindings, Binding{Keys: keys, Desc: desc, Fn: fn})
	return nil
}

// MustRegister is Register that panics; for the static binding tables a
// workbench tool declares at startup.
func (l *Leader) MustRegister(keys, desc string, fn func()) {
	if err := l.Register(keys, desc, fn); err != nil {
		panic(err)
	}
}

// RegisterGroup names a chord prefix, which-key style: after
// RegisterGroup("g", "git"), the tray and help show `g  +git` and pressing
// g lists the "g…" chords. Naming is optional — a prefix shared by
// registered chords is a group either way — but an unnamed group shows as
// `+…`. Groups may nest ("g", then "gs"); order of registration against
// the leaves does not matter.
func (l *Leader) RegisterGroup(keys, desc string) error {
	if keys == "" {
		return fmt.Errorf("vimkeys: empty group")
	}
	for _, g := range l.groups {
		if g.Keys == keys {
			return fmt.Errorf("vimkeys: group %q already registered", keys)
		}
	}
	for _, b := range l.bindings {
		if strings.HasPrefix(keys, b.Keys) {
			return fmt.Errorf("vimkeys: group %q conflicts with registered %q", keys, b.Keys)
		}
	}
	l.groups = append(l.groups, Binding{Keys: keys, Desc: desc})
	return nil
}

// MustRegisterGroup is RegisterGroup that panics.
func (l *Leader) MustRegisterGroup(keys, desc string) {
	if err := l.RegisterGroup(keys, desc); err != nil {
		panic(err)
	}
}

// Bindings returns the registered chords for a `?` help overlay.
func (l *Leader) Bindings() []Binding {
	out := make([]Binding, len(l.bindings))
	copy(out, l.bindings)
	return out
}

// Entries lists the leader tree one level below prefix ("" for the top
// level), in registration order of the first chord under each key. Groups
// with no chords under them are omitted. The tray content renders this;
// tools that draw their own help can walk it too.
func (l *Leader) Entries(prefix string) []Entry {
	var out []Entry
	seen := map[rune]bool{}
	for _, b := range l.bindings {
		if len(b.Keys) <= len(prefix) || !strings.HasPrefix(b.Keys, prefix) {
			continue
		}
		rest := []rune(b.Keys[len(prefix):])
		key := rest[0]
		if seen[key] {
			continue
		}
		seen[key] = true
		keys := prefix + string(key)
		if len(rest) == 1 {
			out = append(out, Entry{Key: key, Keys: keys, Desc: b.Desc})
			continue
		}
		out = append(out, Entry{Key: key, Keys: keys, Desc: l.groupName(keys), Group: true})
	}
	return out
}

func (l *Leader) groupName(keys string) string {
	for _, g := range l.groups {
		if g.Keys == keys {
			return g.Desc
		}
	}
	return ""
}

// HelpText renders the leader tree as plain text, one line per chord and
// per named group, ready for a help modal. It walks the same tree as the
// tray content, so the two cannot drift.
func (l *Leader) HelpText() string {
	var b strings.Builder
	var walk func(prefix string)
	walk = func(prefix string) {
		for _, e := range l.Entries(prefix) {
			if !e.Group {
				fmt.Fprintf(&b, "  <space>%-6s %s\n", e.Keys, e.Desc)
				continue
			}
			if e.Desc != "" {
				fmt.Fprintf(&b, "  <space>%-6s +%s\n", e.Keys, e.Desc)
			}
			walk(e.Keys)
		}
	}
	walk("")
	return b.String()
}

// Pending reports the chord typed since Space and whether one is in
// flight. open with keys "" means Space alone has been pressed.
func (l *Leader) Pending() (keys string, open bool) {
	return l.pending, l.armed
}

// Generation returns a counter bumped on every pending-chord state change
// (armed, extended, popped, dispatched, or abandoned). An adapter that
// implements a timeout captures Generation when it schedules a timer and
// passes it back to ResetIfGeneration when the timer fires, so a timer left
// over from an abandoned chord cannot reset a chord that has since moved on.
func (l *Leader) Generation() int {
	return l.gen
}

// ResetIfGeneration abandons the pending chord if the leader is still armed
// and at generation gen — i.e. no key has arrived since the caller recorded
// that generation. It reports whether it reset anything, and otherwise
// leaves the leader untouched.
func (l *Leader) ResetIfGeneration(gen int) bool {
	if !l.armed || l.gen != gen {
		return false
	}
	l.reset()
	return true
}

// Handle advances the chord state machine by one decoded key and returns
// what happened. See Result's doc for how an adapter should act on it.
func (l *Leader) Handle(ev Event) Result {
	if !l.armed {
		if ev.Kind == KindRune && ev.Rune == ' ' && (l.SkipWhen == nil || !l.SkipWhen()) {
			l.arm()
			return Result{Consumed: true}
		}
		return Result{}
	}

	// A chord is in flight.
	switch ev.Kind {
	case KindEscape:
		l.reset()
		return Result{Consumed: true}
	case KindBackspace:
		if l.pending == "" {
			l.reset()
			return Result{Consumed: true}
		}
		r := []rune(l.pending)
		l.pending = string(r[:len(r)-1])
		l.bump()
		return Result{Consumed: true}
	}
	if ev.Kind != KindRune {
		return l.unbound() // arrows, Enter, ctrl-anything: abandon the chord
	}
	next := l.pending + string(ev.Rune)
	for _, b := range l.bindings {
		if b.Keys == next {
			fn := b.Fn
			l.reset()
			fn()
			return Result{Consumed: true}
		}
	}
	for _, b := range l.bindings {
		if strings.HasPrefix(b.Keys, next) {
			l.pending = next
			l.bump()
			return Result{Consumed: true} // keep collecting
		}
	}
	return l.unbound() // no such chord
}

// unbound abandons the chord and reports the pending prefix it abandoned.
func (l *Leader) unbound() Result {
	pending := l.pending
	l.reset()
	return Result{Consumed: true, Unbound: true, UnboundPending: pending}
}

func (l *Leader) arm() {
	l.armed, l.pending = true, ""
	l.gen++
	l.notify()
}

func (l *Leader) bump() {
	l.gen++
	l.notify()
}

func (l *Leader) reset() {
	l.armed, l.pending = false, ""
	l.gen++
	l.notify()
}

func (l *Leader) notify() {
	if l.OnState == nil {
		return
	}
	if l.armed {
		l.OnState(" " + l.pending)
	} else {
		l.OnState("")
	}
}

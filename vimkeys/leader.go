package vimkeys

import (
	"fmt"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
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

// Leader dispatches Space-leader chords at application level:
//
//	l := vimkeys.NewLeader(app, 0)
//	l.Register("r", "[R]efresh now", refresh)
//	l.MustRegisterGroup("s", "show")
//	l.Register("sl", "[S]how [L]og pane", showLog)
//	app.SetInputCapture(vimkeys.Chain(l.InputCapture, nav.InputCapture))
//
// Put the leader first in the chain so a pending chord owns every key.
//
// While a text-entry widget has focus, Space types a space instead of
// starting a chord (see SkipWhen). Without a Tray a started chord times
// out back to normal keys, like vim's timeoutlen; with a Tray mounted the
// chord waits for a key, since the tray shows what is pending.
//
// While a chord is pending: Esc abandons it, Backspace goes up one level,
// and any key that matches nothing abandons it and is reported to
// OnUnbound.
type Leader struct {
	app      *tview.Application
	timeout  time.Duration
	sticky   bool // no timeout: a Tray is showing the pending chord
	bindings []Binding
	groups   []Binding // named prefixes; Fn is unused

	pending string
	armed   bool
	timer   *time.Timer

	// SkipWhen reports that leader handling should stand aside: a widget
	// that wants Space has focus, or a modal is open. The default is
	// FocusWantsSpace. Tools with modals compose it:
	//
	//	l.SkipWhen = func() bool { return modalOpen() || vimkeys.FocusWantsSpace(app) }
	SkipWhen func() bool

	// OnState, when set, is called with the pending chord ("" when idle) so
	// the tool can render a pending-leader indicator. Called from the event
	// loop and from the timeout goroutine via QueueUpdateDraw.
	OnState func(pending string)

	// OnUnbound, when set, is called after a key that matched nothing
	// abandoned the chord. pending is the chord before that key ("" for a
	// key pressed straight after Space); ev is the key. Esc and Backspace
	// are navigation, not unbound keys, and are not reported.
	OnUnbound func(pending string, ev *tcell.EventKey)
}

// NewLeader builds a Leader. A zero timeout means 800ms. The timeout does
// not apply once a Tray is mounted.
func NewLeader(app *tview.Application, timeout time.Duration) *Leader {
	if timeout == 0 {
		timeout = 800 * time.Millisecond
	}
	l := &Leader{app: app, timeout: timeout}
	l.SkipWhen = func() bool { return FocusWantsSpace(app) }
	return l
}

// FocusWantsSpace reports that the focused widget uses Space itself: tview's
// InputField and TextArea type it, Checkbox toggles on it. Form fields are
// covered because a Form hands focus to the field.
func FocusWantsSpace(app *tview.Application) bool {
	switch app.GetFocus().(type) {
	case *tview.InputField, *tview.TextArea, *tview.Checkbox:
		return true
	}
	return false
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
// with no chords under them are omitted. The Tray renders this; tools that
// draw their own help can walk it too.
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
// Tray, so the two cannot drift.
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

// InputCapture consumes Space and the chord keys that follow. Everything
// else passes through untouched.
func (l *Leader) InputCapture(ev *tcell.EventKey) *tcell.EventKey {
	if !l.armed {
		if ev.Key() == tcell.KeyRune && ev.Rune() == ' ' && ev.Modifiers() == tcell.ModNone &&
			(l.SkipWhen == nil || !l.SkipWhen()) {
			l.arm()
			return nil
		}
		return ev
	}

	// A chord is in flight.
	switch ev.Key() {
	case tcell.KeyEscape:
		l.reset()
		return nil
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if l.pending == "" {
			l.reset()
			return nil
		}
		r := []rune(l.pending)
		l.pending = string(r[:len(r)-1])
		l.bump()
		l.notify()
		return nil
	}
	if ev.Key() != tcell.KeyRune || ev.Modifiers() != tcell.ModNone {
		l.unbound(ev) // arrows, Enter, ctrl-anything: abandon the chord
		return nil
	}
	next := l.pending + string(ev.Rune())
	for _, b := range l.bindings {
		if b.Keys == next {
			fn := b.Fn
			l.reset()
			fn()
			return nil
		}
	}
	for _, b := range l.bindings {
		if strings.HasPrefix(b.Keys, next) {
			l.pending = next
			l.bump()
			l.notify()
			return nil // keep collecting
		}
	}
	l.unbound(ev) // no such chord
	return nil
}

// unbound abandons the chord and reports the key that broke it.
func (l *Leader) unbound(ev *tcell.EventKey) {
	pending := l.pending
	l.reset()
	if l.OnUnbound != nil {
		l.OnUnbound(pending, ev)
	}
}

func (l *Leader) arm() {
	l.armed, l.pending = true, ""
	if !l.sticky {
		l.timer = time.AfterFunc(l.timeout, func() {
			l.app.QueueUpdateDraw(func() { l.reset() })
		})
	}
	l.notify()
}

func (l *Leader) bump() {
	if l.timer != nil {
		l.timer.Reset(l.timeout)
	}
}

func (l *Leader) reset() {
	if l.timer != nil {
		l.timer.Stop()
		l.timer = nil
	}
	l.armed, l.pending = false, ""
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

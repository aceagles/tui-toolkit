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

// Leader dispatches Space-leader chords at application level:
//
//	l := vimkeys.NewLeader(app, 0)
//	l.Register("r", "[R]efresh now", refresh)
//	l.Register("sl", "[S]how [L]og pane", showLog)
//	app.SetInputCapture(vimkeys.Chain(nav.InputCapture, l.InputCapture))
//
// While a text-entry widget has focus, Space types a space instead of
// starting a chord (see SkipWhen). A started chord times out back to
// normal keys, like vim's timeoutlen.
type Leader struct {
	app      *tview.Application
	timeout  time.Duration
	bindings []Binding

	pending string
	armed   bool
	timer   *time.Timer

	// SkipWhen reports that leader handling should stand aside (typically:
	// a text-entry widget has focus). The default skips tview's InputField
	// and TextArea.
	SkipWhen func() bool

	// OnState, when set, is called with the pending chord ("" when idle) so
	// the tool can render a pending-leader indicator. Called from the event
	// loop and from the timeout goroutine via QueueUpdateDraw.
	OnState func(pending string)
}

// NewLeader builds a Leader. A zero timeout means 800ms.
func NewLeader(app *tview.Application, timeout time.Duration) *Leader {
	if timeout == 0 {
		timeout = 800 * time.Millisecond
	}
	l := &Leader{app: app, timeout: timeout}
	l.SkipWhen = func() bool {
		switch app.GetFocus().(type) {
		case *tview.InputField, *tview.TextArea:
			return true
		}
		return false
	}
	return l
}

// Register adds a chord. Registering "s" and "sf" together is rejected:
// the shorter chord would shadow the longer, which vim resolves with
// timeoutlen ambiguity nobody enjoys debugging in a workbench tool.
func (l *Leader) Register(keys, desc string, fn func()) error {
	if keys == "" || fn == nil {
		return fmt.Errorf("vimkeys: empty binding")
	}
	for _, b := range l.bindings {
		if strings.HasPrefix(b.Keys, keys) || strings.HasPrefix(keys, b.Keys) {
			return fmt.Errorf("vimkeys: %q conflicts with registered %q", keys, b.Keys)
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

// Bindings returns the registered chords for a `?` help overlay.
func (l *Leader) Bindings() []Binding {
	out := make([]Binding, len(l.bindings))
	copy(out, l.bindings)
	return out
}

// HelpText renders the binding table as plain text, one chord per line,
// ready for a help modal.
func (l *Leader) HelpText() string {
	var b strings.Builder
	for _, bind := range l.bindings {
		fmt.Fprintf(&b, "  <space>%-6s %s\n", bind.Keys, bind.Desc)
	}
	return b.String()
}

// InputCapture consumes Space and the chord runes that follow. Everything
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
	if ev.Key() != tcell.KeyRune || ev.Modifiers() != tcell.ModNone {
		l.reset() // Esc, arrows, ctrl-anything: abandon the chord
		return nil
	}
	l.pending += string(ev.Rune())
	for _, b := range l.bindings {
		if b.Keys == l.pending {
			fn := b.Fn
			l.reset()
			fn()
			return nil
		}
	}
	for _, b := range l.bindings {
		if strings.HasPrefix(b.Keys, l.pending) {
			l.bump()
			l.notify()
			return nil // keep collecting
		}
	}
	l.reset() // no such chord
	return nil
}

func (l *Leader) arm() {
	l.armed, l.pending = true, ""
	l.timer = time.AfterFunc(l.timeout, func() {
		l.app.QueueUpdateDraw(func() { l.reset() })
	})
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

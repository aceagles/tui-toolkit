// The Leader/Tray implementation here is a thin compatibility layer: the
// chord state machine and tray content live in vimkeys/core, the tview
// wiring (input capture, timeout timer, drawing) lives in vimkeys/tview,
// and this file just re-exports that adapter under the vimkeys package's
// original names so tools written against the pre-split API — modbustui
// among them — keep compiling with no source changes. New tview tools
// should import vimkeys/tview directly; new bubbletea tools use
// vimkeys/bubbletea instead. See vimkeys/core's package doc for the chord,
// group and timeout reasoning, and vimkeys/tview's for the tview-specific
// seams (FocusWantsSpace, the *tcell.EventKey decode, OnUnbound).
package vimkeys

import (
	"time"

	"github.com/rivo/tview"

	"github.com/aceagles/tui-toolkit/vimkeys/core"
	vimtview "github.com/aceagles/tui-toolkit/vimkeys/tview"
)

// Binding is one leader chord: Keys are the runes after Space ("r", "sf"),
// Desc is the which-key style description shown in help.
type Binding = core.Binding

// Entry is one line of the leader tree at some prefix: either a leaf that
// dispatches a Binding, or a group whose children sit one key further in.
type Entry = core.Entry

// Leader dispatches Space-leader chords at application level. It is an
// alias for vimkeys/tview.Leader; see that package's doc for the full
// behaviour.
type Leader = vimtview.Leader

// Tray wraps an application root and, while a leader chord is pending,
// draws a which-key panel over the bottom of it. It is an alias for
// vimkeys/tview.Tray.
type Tray = vimtview.Tray

// NewLeader builds a Leader. A zero timeout means 800ms. The timeout does
// not apply once a Tray is mounted.
func NewLeader(app *tview.Application, timeout time.Duration) *Leader {
	return vimtview.NewLeader(app, timeout)
}

// NewTray wraps root so it shows l's pending chord.
func NewTray(l *Leader, root tview.Primitive) *Tray {
	return vimtview.NewTray(l, root)
}

// FocusWantsSpace reports that the focused widget uses Space itself: tview's
// InputField and TextArea type it, Checkbox toggles on it. Form fields are
// covered because a Form hands focus to the field.
func FocusWantsSpace(app *tview.Application) bool {
	return vimtview.FocusWantsSpace(app)
}

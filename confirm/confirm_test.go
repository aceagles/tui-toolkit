package confirm

import (
	"strings"
	"testing"
	"time"

	"github.com/aceagles/tui-toolkit/theme"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// rig runs a real application on a simulation screen with the dialogue
// open, and reports which callback fired.
type rig struct {
	app    *tview.Application
	pages  *tview.Pages
	screen tcell.SimulationScreen
	result chan string
}

func open(t *testing.T) *rig {
	t.Helper()
	theme.Apply(theme.Night)
	r := &rig{app: tview.NewApplication(), result: make(chan string, 2)}
	r.pages = tview.NewPages().AddPage("main", tview.NewBox(), true, true)
	r.screen = tcell.NewSimulationScreen("UTF-8")
	if err := r.screen.Init(); err != nil {
		t.Fatal(err)
	}
	r.screen.SetSize(100, 30)
	r.app.SetScreen(r.screen).SetRoot(r.pages, true)
	done := make(chan error, 1)
	go func() { done <- r.app.Run() }()
	t.Cleanup(func() { r.app.Stop(); <-done })

	shown := make(chan struct{})
	r.app.QueueUpdateDraw(func() {
		Show(r.app, r.pages, Dialog{
			Title:     "CONFIRM PUT",
			Method:    "PUT /limits/export",
			Lines:     []Line{{"dest", "127.0.0.1:8080"}, {"body", "42"}},
			OnConfirm: func() { r.result <- "confirm" },
			OnCancel:  func() { r.result <- "cancel" },
		})
		close(shown)
	})
	<-shown
	return r
}

func (r *rig) press(keys ...tcell.Key) {
	for _, k := range keys {
		r.screen.InjectKey(k, 0, tcell.ModNone)
	}
}

func (r *rig) want(t *testing.T, want string) {
	t.Helper()
	select {
	case got := <-r.result:
		if got != want {
			t.Errorf("callback %q, want %q", got, want)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("no callback, want %q", want)
	}
	closed := make(chan bool, 1) // QueueUpdate waits for the update to finish
	r.app.QueueUpdate(func() { closed <- !r.pages.HasPage(PageName) })
	if !<-closed {
		t.Error("dialogue still open after its callback")
	}
}

// Cancel holds focus, so Enter on a fresh dialogue backs out.
func TestEnterOnOpenCancels(t *testing.T) {
	r := open(t)
	r.press(tcell.KeyEnter)
	r.want(t, "cancel")
}

func TestTabEnterConfirms(t *testing.T) {
	r := open(t)
	r.press(tcell.KeyTab, tcell.KeyEnter)
	r.want(t, "confirm")
}

func TestEscCancels(t *testing.T) {
	r := open(t)
	r.press(tcell.KeyTab, tcell.KeyEscape) // Esc cancels even from the confirm button
	r.want(t, "cancel")
}

func TestBody(t *testing.T) {
	p := theme.Night
	got := Body(Dialog{
		Warning: "remote target",
		Method:  "WRITE [x]",
		Lines:   []Line{{"dest", "192.0.2.1:502"}, {"words", "0x0007\n7"}},
	}, p)
	muted := p.Tag(theme.RoleFgMuted)
	want := p.Tag(theme.RoleError) + "[::b]remote target[-::-]\n\n" +
		p.Tag(theme.RoleTitle) + "[::b]WRITE [x[][-::-]\n\n" +
		"  " + muted + "dest  :[-] 192.0.2.1:502\n" +
		"  " + muted + "words :[-] 0x0007\n" +
		"          7"
	if got != want {
		t.Errorf("Body:\n%s\nwant:\n%s", got, want)
	}
	if strings.Contains(Body(Dialog{Lines: []Line{{"a", "b"}}}, p), "\n\n") {
		t.Error("empty warning and method left blank rows")
	}
}

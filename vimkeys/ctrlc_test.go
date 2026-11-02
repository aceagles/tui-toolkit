package vimkeys

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// ctrlCApp runs a real tview app on a simulation screen with the wrapper
// installed, so tview's own C-c handling is part of what is tested.
type ctrlCApp struct {
	app     *tview.Application
	sim     tcell.SimulationScreen
	stopped chan struct{}
}

func startCtrlCApp(t *testing.T, root tview.Primitive, next func(*tcell.EventKey) *tcell.EventKey) *ctrlCApp {
	t.Helper()
	sim := tcell.NewSimulationScreen("UTF-8")
	if err := sim.Init(); err != nil {
		t.Fatal(err)
	}
	app := tview.NewApplication().SetScreen(sim).SetRoot(root, true)
	app.SetInputCapture(CtrlCAsEscape(next))
	a := &ctrlCApp{app: app, sim: sim, stopped: make(chan struct{})}
	go func() {
		_ = app.Run()
		close(a.stopped)
	}()
	time.Sleep(50 * time.Millisecond)
	t.Cleanup(func() { app.Stop() })
	return a
}

func (a *ctrlCApp) running() bool {
	select {
	case <-a.stopped:
		return false
	case <-time.After(150 * time.Millisecond):
		return true
	}
}

func TestCtrlCDoesNotStopApp(t *testing.T) {
	a := startCtrlCApp(t, tview.NewBox(), nil)
	a.sim.InjectKey(tcell.KeyCtrlC, 0, tcell.ModCtrl)
	if !a.running() {
		t.Fatal("C-c in normal mode stopped the app")
	}
}

func TestCtrlCAppWithoutWrapperStops(t *testing.T) {
	// Control: proves the harness would notice tview's default C-c quit.
	sim := tcell.NewSimulationScreen("UTF-8")
	if err := sim.Init(); err != nil {
		t.Fatal(err)
	}
	app := tview.NewApplication().SetScreen(sim).SetRoot(tview.NewBox(), true)
	done := make(chan struct{})
	go func() { _ = app.Run(); close(done) }()
	time.Sleep(50 * time.Millisecond)
	sim.InjectKey(tcell.KeyCtrlC, 0, tcell.ModCtrl)
	select {
	case <-done:
	case <-time.After(time.Second):
		app.Stop()
		t.Fatal("harness: tview did not stop on bare C-c")
	}
}

func TestCtrlCLeavesInputFieldLikeEscape(t *testing.T) {
	for _, key := range []tcell.Key{tcell.KeyEscape, tcell.KeyCtrlC} {
		var got atomic.Int32 // the key passed to the done handler
		in := tview.NewInputField()
		in.SetDoneFunc(func(k tcell.Key) { got.Store(int32(k)) })
		a := startCtrlCApp(t, in, nil)
		a.sim.InjectKey(key, 0, tcell.ModNone)
		if !a.running() {
			t.Fatalf("key %v stopped the app", key)
		}
		if tcell.Key(got.Load()) != tcell.KeyEscape {
			t.Fatalf("key %v: input field done key = %v, want Escape", key, tcell.Key(got.Load()))
		}
	}
}

func TestCtrlCReachesNextAsEscape(t *testing.T) {
	var seen atomic.Int32
	a := startCtrlCApp(t, tview.NewBox(), func(ev *tcell.EventKey) *tcell.EventKey {
		seen.Store(int32(ev.Key()))
		return nil
	})
	a.sim.InjectKey(tcell.KeyCtrlC, 0, tcell.ModCtrl)
	a.running()
	if tcell.Key(seen.Load()) != tcell.KeyEscape {
		t.Fatalf("next saw %v, want Escape", tcell.Key(seen.Load()))
	}
}

func TestCtrlCCancelsPendingLeaderChord(t *testing.T) {
	var fired atomic.Bool
	box := tview.NewBox()
	app := tview.NewApplication()
	l := NewLeader(app, time.Minute)
	l.MustRegister("r", "refresh", func() { fired.Store(true) })
	sim := tcell.NewSimulationScreen("UTF-8")
	if err := sim.Init(); err != nil {
		t.Fatal(err)
	}
	app.SetScreen(sim).SetRoot(box, true).SetInputCapture(CtrlCAsEscape(l.InputCapture))
	done := make(chan struct{})
	go func() { _ = app.Run(); close(done) }()
	t.Cleanup(app.Stop)
	time.Sleep(50 * time.Millisecond)

	sim.InjectKey(tcell.KeyRune, ' ', tcell.ModNone)
	sim.InjectKey(tcell.KeyCtrlC, 0, tcell.ModCtrl)
	sim.InjectKey(tcell.KeyRune, 'r', tcell.ModNone)
	select {
	case <-done:
		t.Fatal("C-c stopped the app")
	case <-time.After(200 * time.Millisecond):
	}
	if fired.Load() {
		t.Fatal("chord completed after C-c; it should have been abandoned")
	}
}

func TestExplicitQuitKeyStillQuits(t *testing.T) {
	var app *tview.Application
	quit := func(ev *tcell.EventKey) *tcell.EventKey {
		if ev.Key() == tcell.KeyRune && ev.Rune() == 'q' {
			app.Stop()
			return nil
		}
		return ev
	}
	a := startCtrlCApp(t, tview.NewBox(), quit)
	app = a.app
	a.sim.InjectKey(tcell.KeyRune, 'q', tcell.ModNone)
	if a.running() {
		t.Fatal("q did not quit")
	}
}

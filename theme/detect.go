package theme

import (
	"os"
	"os/exec"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/rivo/tview"
)

// EnvVar names the environment variable that forces a palette: "night" or
// "day". Any other value is ignored.
const EnvVar = "TUI_THEME"

// TmuxLightOption is the global tmux user option that selects Day. A tmux
// dark/light toggle sets it (to 1) in light mode and unsets it in dark mode.
// Tools with a different toggle can change it before calling Detect.
var TmuxLightOption = "@dotbar-light"

// DefaultWatchInterval is how often Watch polls when given no interval.
const DefaultWatchInterval = 2 * time.Second

// tmuxOutput runs a tmux query and returns its trimmed output; tests
// replace it.
var tmuxOutput = func(args ...string) (string, error) {
	out, err := exec.Command("tmux", args...).Output()
	return strings.TrimSpace(string(out)), err
}

// Detect picks the palette. $TUI_THEME=night|day wins. Otherwise, inside
// tmux ($TMUX set), a set TmuxLightOption other than 0 means Day. Anything
// else, including a failed tmux call, is Night.
func Detect() Palette {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(EnvVar))) {
	case Night.Name:
		return Night
	case Day.Name:
		return Day
	}
	if os.Getenv("TMUX") == "" {
		return Night
	}
	v, err := tmuxOutput("show-options", "-gqv", TmuxLightOption)
	if err != nil || v == "" || v == "0" {
		return Night
	}
	return Day
}

// Watch polls Detect every interval (DefaultWatchInterval when interval is
// zero or less). When the palette changes it calls Apply and then onChange
// on the event loop, and redraws; onChange should Restyle long-lived
// widgets and re-render tagged text. The first poll compares against
// Current. The returned func stops polling and waits for the poller to
// exit; it is safe to call more than once.
func Watch(app *tview.Application, interval time.Duration, onChange func(Palette)) (stop func()) {
	if interval <= 0 {
		interval = DefaultWatchInterval
	}
	done, exited := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(exited)
		tick := time.NewTicker(interval)
		defer tick.Stop()
		last := Current().Name
		for {
			select {
			case <-done:
				return
			case <-tick.C:
			}
			p := Detect()
			if p.Name == last {
				continue
			}
			last = p.Name
			app.QueueUpdateDraw(func() {
				Apply(p)
				if onChange != nil {
					onChange(p)
				}
			})
		}
	}()
	var once sync.Once
	return func() {
		once.Do(func() { close(done) })
		<-exited
	}
}

// EnableTrueColor lets tcell draw the palette in 24-bit colour inside tmux.
// Call it before the application starts its screen.
//
// tcell only emits 24-bit colour when $COLORTERM is truecolor/24bit, when
// $TCELL_TRUECOLOR is set to anything but "disable", or when the terminfo
// entry has RGB capabilities — and its tmux-256color entry has none. So
// under tmux it falls back to the nearest of 256 colours unless told
// otherwise. tmux itself accepts 24-bit colour and reports whether the
// attached terminal does: "RGB" in #{client_termfeatures}, which the
// terminal-overrides ",*256col*:Tc" or terminal-features ",*:RGB" setting
// turns on.
//
// EnableTrueColor sets COLORTERM=truecolor for this process when $TMUX is
// set, neither $COLORTERM nor $TCELL_TRUECOLOR is, and tmux reports RGB for
// the client. It reports whether it set it. It never overrides a user's
// setting and does nothing outside tmux; there, or with no client attached,
// set COLORTERM=truecolor yourself if the terminal supports it.
func EnableTrueColor() bool {
	if os.Getenv("TMUX") == "" || os.Getenv("COLORTERM") != "" || os.Getenv("TCELL_TRUECOLOR") != "" {
		return false
	}
	feats, err := tmuxOutput("display-message", "-p", "#{client_termfeatures}")
	if err != nil || !slices.Contains(strings.Split(feats, ","), "RGB") {
		return false
	}
	return os.Setenv("COLORTERM", "truecolor") == nil
}

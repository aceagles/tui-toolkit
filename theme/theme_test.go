package theme

import (
	"errors"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// Spot checks against folke/tokyonight.nvim extras/lua/tokyonight_night.lua
// and tokyonight_day.lua (commit cdc07ac).
func TestPalettesMatchTokyonight(t *testing.T) {
	for _, c := range []struct {
		p    Palette
		role Role
		hex  string // tokyonight key in the comment
	}{
		{Night, RoleBg, "#1a1b26"},          // bg
		{Night, RoleBgFloat, "#16161e"},     // bg_float
		{Night, RoleFg, "#c0caf5"},          // fg
		{Night, RoleBorderFocus, "#27a1b9"}, // border_highlight
		{Night, RoleSelection, "#283457"},   // bg_visual
		{Night, RoleWarn, "#e0af68"},        // warning
		{Night, RoleError, "#db4b4b"},       // error
		{Night, RoleInfo, "#0db9d7"},        // info
		{Day, RoleBg, "#e1e2e7"},            // bg
		{Day, RoleFg, "#3760bf"},            // fg
		{Day, RoleBgFloat, "#d0d5e3"},       // bg_float
		{Day, RoleBorderFocus, "#4094a3"},   // border_highlight
		{Day, RoleWarn, "#8c6c3e"},          // warning
		{Day, RoleError, "#c64343"},         // error
		{Day, RoleRemote, "#f52a65"},        // red
	} {
		if got := c.p.Hex(c.role); got != c.hex {
			t.Errorf("%s %s = %s, want %s", c.p.Name, c.role, got, c.hex)
		}
	}
}

func TestPalettesComplete(t *testing.T) {
	for _, p := range palettes {
		for r := range roleCount {
			if c := p.Color(r); !c.Valid() || c.Hex() < 0 {
				t.Errorf("%s %s unset", p.Name, r)
			}
		}
	}
}

// Restyle maps colours by value, so two roles sharing a colour in one
// palette must share it in every palette.
func TestRemapIsConsistent(t *testing.T) {
	for _, from := range palettes {
		for _, to := range palettes {
			for a := range roleCount {
				for b := range roleCount {
					if from.Color(a) == from.Color(b) && to.Color(a) != to.Color(b) {
						t.Errorf("%s %s and %s share a colour but differ in %s", from.Name, a, b, to.Name)
					}
				}
			}
		}
	}
}

// fakeTmux replaces the tmux call for one test.
func fakeTmux(t *testing.T, fn func(args ...string) (string, error)) {
	t.Helper()
	orig := tmuxOutput
	tmuxOutput = fn
	t.Cleanup(func() { tmuxOutput = orig })
}

func TestDetect(t *testing.T) {
	light := func(v string, err error) func(...string) (string, error) {
		return func(...string) (string, error) { return v, err }
	}
	for _, c := range []struct {
		name, env, tmuxEnv string
		tmux               func(...string) (string, error)
		want               Palette
	}{
		{"env night beats tmux light", "night", "/tmp/tmux", light("1", nil), Night},
		{"env day beats tmux dark", "day", "/tmp/tmux", light("", nil), Day},
		{"env is case-insensitive", " DAY ", "", light("", nil), Day},
		{"unknown env falls through", "auto", "/tmp/tmux", light("1", nil), Day},
		{"no tmux is night", "", "", light("1", nil), Night},
		{"tmux light option set", "", "/tmp/tmux", light("1", nil), Day},
		{"tmux light option unset", "", "/tmp/tmux", light("", nil), Night},
		{"tmux light option 0", "", "/tmp/tmux", light("0", nil), Night},
		{"tmux call fails", "", "/tmp/tmux", light("1", errors.New("no server")), Night},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv(EnvVar, c.env)
			t.Setenv("TMUX", c.tmuxEnv)
			fakeTmux(t, c.tmux)
			if got := Detect(); got.Name != c.want.Name {
				t.Errorf("Detect() = %s, want %s", got.Name, c.want.Name)
			}
		})
	}
}

func TestDetectQueriesTmuxLightOption(t *testing.T) {
	t.Setenv(EnvVar, "")
	t.Setenv("TMUX", "/tmp/tmux")
	orig := TmuxLightOption
	t.Cleanup(func() { TmuxLightOption = orig })
	TmuxLightOption = "@my-light"
	var got []string
	fakeTmux(t, func(args ...string) (string, error) { got = args; return "", nil })
	Detect()
	if strings.Join(got, " ") != "show-options -gqv @my-light" {
		t.Errorf("tmux args %q", got)
	}
}

// applyNight restores the default palette after a test that changes it.
func applyNight(t *testing.T) {
	t.Helper()
	Apply(Night)
	t.Cleanup(func() { Apply(Night) })
}

func TestApplySetsStyles(t *testing.T) {
	applyNight(t)
	Apply(Day)
	s := tview.Styles
	for name, c := range map[string]struct {
		got  tcell.Color
		want Role
	}{
		"PrimitiveBackgroundColor":   {s.PrimitiveBackgroundColor, RoleBg},
		"ContrastBackgroundColor":    {s.ContrastBackgroundColor, RoleBgContrast},
		"BorderColor":                {s.BorderColor, RoleBorder},
		"TitleColor":                 {s.TitleColor, RoleTitle},
		"PrimaryTextColor":           {s.PrimaryTextColor, RoleFg},
		"SecondaryTextColor":         {s.SecondaryTextColor, RoleAccent},
		"TertiaryTextColor":          {s.TertiaryTextColor, RoleTitle},
		"InverseTextColor":           {s.InverseTextColor, RoleBg},
		"ContrastSecondaryTextColor": {s.ContrastSecondaryTextColor, RoleFgMuted},
	} {
		if c.got != Day.Color(c.want) {
			t.Errorf("%s = %06x, want %s %s", name, c.got.Hex(), c.want, Day.Hex(c.want))
		}
	}
	if Current().Name != Day.Name {
		t.Errorf("Current() = %s after Apply(Day)", Current().Name)
	}
}

func TestTag(t *testing.T) {
	for role, want := range map[Role]string{
		RoleWarn:      "[#e0af68]",
		RoleRemote:    "[#f7768e]",
		RoleBg:        "[:#1a1b26]",
		RoleSelection: "[#c0caf5:#283457]",
		RoleHighlight: "[#15161e:#ff9e64]",
	} {
		tag := Night.Tag(role)
		if tag != want {
			t.Errorf("Tag(%s) = %q, want %q", role, tag, want)
		}
		// tview must read it as a tag, not print it.
		if w := tview.TaggedStringWidth(tag + "x[-:-]"); w != 1 {
			t.Errorf("Tag(%s) %q: tview width %d, want 1", role, tag, w)
		}
	}
}

func TestStyle(t *testing.T) {
	fg, bg, _ := Night.Style(RoleHighlight).Decompose()
	if fg != Night.Color(RoleHighlightFg) || bg != Night.Color(RoleHighlight) {
		t.Errorf("Highlight style fg %06x bg %06x", fg.Hex(), bg.Hex())
	}
	if fg, bg, _ := Night.Style(RoleWarn).Decompose(); fg != Night.Color(RoleWarn) || bg != tcell.ColorDefault {
		t.Errorf("Warn style fg %06x bg %v", fg.Hex(), bg)
	}
}

func TestRestyle(t *testing.T) {
	applyNight(t)
	text := tview.NewTextView()
	focused := tview.NewBox().SetBorder(true).SetBorderColor(Night.Color(RoleBorderFocus))
	custom := tview.NewBox().SetBackgroundColor(tcell.NewHexColor(0x123456))
	dialog := tview.NewForm().AddInputField("x", "", 4, nil, nil)
	dialog.SetBackgroundColor(Night.Color(RoleBgFloat))
	pages := tview.NewPages().
		AddPage("main", tview.NewFlex().AddItem(text, 0, 1, true).AddItem(focused, 0, 1, false).AddItem(custom, 0, 1, false), true, true).
		AddPage("dialog", dialog, true, false) // hidden

	Restyle(pages, Day)

	for name, c := range map[string]struct{ got, want tcell.Color }{
		"text background":      {text.GetBackgroundColor(), Day.Color(RoleBg)},
		"text border":          {text.GetBorderColor(), Day.Color(RoleBorder)},
		"focused border":       {focused.GetBorderColor(), Day.Color(RoleBorderFocus)},
		"hidden dialog bg":     {dialog.GetBackgroundColor(), Day.Color(RoleBgFloat)},
		"non-palette colour":   {custom.GetBackgroundColor(), tcell.NewHexColor(0x123456)},
		"form field (drawn)":   {fieldBg(dialog.GetFormItem(0).(*tview.InputField)), Day.Color(RoleBgContrast)},
		"pages box background": {pages.GetBackgroundColor(), Day.Color(RoleBg)},
	} {
		if c.got != c.want {
			t.Errorf("%s = %06x, want %06x", name, c.got.Hex(), c.want.Hex())
		}
	}
}

// fieldBg is the field background an input field draws with.
func fieldBg(i *tview.InputField) tcell.Color {
	_, bg, _ := i.GetFieldStyle().Decompose()
	return bg
}

// Watch flips the palette, restyles on the event loop and stops cleanly.
func TestWatch(t *testing.T) {
	applyNight(t)
	t.Setenv(EnvVar, "")
	t.Setenv("TMUX", "/tmp/tmux")
	var light atomic.Bool
	fakeTmux(t, func(...string) (string, error) {
		if light.Load() {
			return "1", nil
		}
		return "", nil
	})

	app := tview.NewApplication()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	app.SetScreen(screen).SetRoot(tview.NewBox(), true)
	done := make(chan error, 1)
	go func() { done <- app.Run() }()
	defer func() { app.Stop(); <-done }()

	changed := make(chan string, 4)
	stop := Watch(app, 5*time.Millisecond, func(p Palette) {
		changed <- p.Name + " " + tview.Styles.PrimitiveBackgroundColor.String()
	})
	defer stop()

	light.Store(true)
	select {
	case got := <-changed:
		if want := "day " + Day.Color(RoleBg).String(); got != want {
			t.Errorf("onChange saw %q, want %q", got, want)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Watch did not report the switch to day")
	}
	stop()
	stop()
	light.Store(false)
	time.Sleep(30 * time.Millisecond)
	select {
	case got := <-changed:
		t.Errorf("onChange after stop: %q", got)
	default:
	}
}

func TestEnableTrueColor(t *testing.T) {
	feats := func(v string) func(...string) (string, error) {
		return func(...string) (string, error) { return v, nil }
	}
	for _, c := range []struct {
		name, tmux, colorterm, tcell, feats string
		want                                bool
		wantColorterm                       string
	}{
		{"tmux client has RGB", "/tmp/tmux", "", "", "clipboard,RGB,title", true, "truecolor"},
		{"tmux client lacks RGB", "/tmp/tmux", "", "", "clipboard,title", false, ""},
		{"user COLORTERM wins", "/tmp/tmux", "24bit", "", "RGB", false, "24bit"},
		{"user TCELL_TRUECOLOR wins", "/tmp/tmux", "", "disable", "RGB", false, ""},
		{"outside tmux", "", "", "", "RGB", false, ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv("TMUX", c.tmux)
			t.Setenv("COLORTERM", c.colorterm)
			t.Setenv("TCELL_TRUECOLOR", c.tcell)
			fakeTmux(t, feats(c.feats))
			if got := EnableTrueColor(); got != c.want {
				t.Errorf("EnableTrueColor() = %v, want %v", got, c.want)
			}
			if got := os.Getenv("COLORTERM"); got != c.wantColorterm {
				t.Errorf("COLORTERM = %q, want %q", got, c.wantColorterm)
			}
		})
	}
}

package theme

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
)

// Role is what a colour is for. Tools colour by role, never by name, so a
// palette swap recolours the whole tool.
type Role int

const (
	RoleBg          Role = iota // main background (Normal)
	RoleBgFloat                 // dialogues and popups (NormalFloat)
	RoleBgContrast              // input fields and buttons (CursorLine)
	RoleFg                      // body text (Normal)
	RoleFgMuted                 // labels, timestamps, idle states (Comment)
	RoleBorder                  // unfocused borders (NonText)
	RoleBorderFocus             // the focused pane's border (FloatBorder)
	RoleTitle                   // box titles and headings (Title)
	RoleSelection               // cursor row background (Visual)
	RoleSelectionFg             // cursor row text
	RoleHighlight               // a value that just changed (IncSearch)
	RoleHighlightFg             // text on RoleHighlight
	RoleOK                      // healthy, sent, connected
	RoleWarn                    // needs attention (DiagnosticWarn)
	RoleError                   // failed, destructive (DiagnosticError)
	RoleInfo                    // neutral notices (DiagnosticInfo)
	RoleAccent                  // keys in hints and help (WhichKey)
	RoleRemote                  // a target off this machine: plain text colour
	roleCount
)

var roleNames = [roleCount]string{
	"Bg", "BgFloat", "BgContrast", "Fg", "FgMuted", "Border", "BorderFocus", "Title",
	"Selection", "SelectionFg", "Highlight", "HighlightFg",
	"OK", "Warn", "Error", "Info", "Accent", "Remote",
}

func (r Role) String() string {
	if r < 0 || r >= roleCount {
		return fmt.Sprintf("Role(%d)", int(r))
	}
	return roleNames[r]
}

// isBackground reports roles that colour a background rather than text.
func (r Role) isBackground() bool {
	return r == RoleBg || r == RoleBgFloat || r == RoleBgContrast
}

// pairFg returns the text role that goes with a background-and-text role.
func (r Role) pairFg() (Role, bool) {
	switch r {
	case RoleSelection:
		return RoleSelectionFg, true
	case RoleHighlight:
		return RoleHighlightFg, true
	}
	return 0, false
}

// Palette is a full set of role colours. The zero value is unusable; use
// Night, Day or Detect.
type Palette struct {
	Name   string // "night" or "day"
	colors [roleCount]tcell.Color
}

// Color returns the colour for r. For RoleSelection and RoleHighlight it is
// the background; their text colour is the matching …Fg role.
func (p Palette) Color(r Role) tcell.Color {
	if r < 0 || r >= roleCount {
		return tcell.ColorDefault
	}
	return p.colors[r]
}

// Hex returns r's colour as "#rrggbb".
func (p Palette) Hex(r Role) string { return fmt.Sprintf("#%06x", p.Color(r).Hex()) }

// Style returns r as a tcell style: text colour for text roles, background
// for background roles, and both for RoleSelection and RoleHighlight.
func (p Palette) Style(r Role) tcell.Style {
	s := tcell.StyleDefault
	if fg, ok := r.pairFg(); ok {
		return s.Foreground(p.Color(fg)).Background(p.Color(r))
	}
	if r.isBackground() {
		return s.Background(p.Color(r))
	}
	return s.Foreground(p.Color(r))
}

// Tag returns a tview colour tag for r: "[#e0af68]" for a text role,
// "[:#1a1b26]" for a background role and "[#c0caf5:#283457]" for
// RoleSelection or RoleHighlight. Close it with "[-]" or "[-:-]".
func (p Palette) Tag(r Role) string {
	if fg, ok := r.pairFg(); ok {
		return "[" + p.Hex(fg) + ":" + p.Hex(r) + "]"
	}
	if r.isBackground() {
		return "[:" + p.Hex(r) + "]"
	}
	return "[" + p.Hex(r) + "]"
}

// tokyonight is the slice of a tokyonight.nvim ColorScheme the roles draw
// on, named as in the plugin's extras/lua/tokyonight_<style>.lua.
type tokyonight struct {
	bg, bgFloat, bgHighlight, bgVisual         string
	fg, comment, dark3, black, borderHighlight string
	blue, cyan, green, orange, red             string
	error, warning, info                       string
}

// fromTokyonight maps a tokyonight colour set onto roles the way tokyonight
// maps its own highlight groups (the group is noted on each Role).
func fromTokyonight(name string, c tokyonight) Palette {
	hex := func(s string) tcell.Color { return tcell.GetColor(s) }
	p := Palette{Name: name}
	p.colors = [roleCount]tcell.Color{
		RoleBg:          hex(c.bg),
		RoleBgFloat:     hex(c.bgFloat),
		RoleBgContrast:  hex(c.bgHighlight),
		RoleFg:          hex(c.fg),
		RoleFgMuted:     hex(c.comment),
		RoleBorder:      hex(c.dark3),
		RoleBorderFocus: hex(c.borderHighlight),
		RoleTitle:       hex(c.blue),
		RoleSelection:   hex(c.bgVisual),
		RoleSelectionFg: hex(c.fg),
		RoleHighlight:   hex(c.orange),
		RoleHighlightFg: hex(c.black),
		RoleOK:          hex(c.green),
		RoleWarn:        hex(c.warning),
		RoleError:       hex(c.error),
		RoleInfo:        hex(c.info),
		RoleAccent:      hex(c.cyan),
		RoleRemote:      hex(c.red),
	}
	return p
}

// Night is tokyonight-night, the dark style. Values are the computed
// ColorScheme from folke/tokyonight.nvim with the default config
// (extras/lua/tokyonight_night.lua).
var Night = fromTokyonight("night", tokyonight{
	bg:              "#1a1b26",
	bgFloat:         "#16161e",
	bgHighlight:     "#292e42",
	bgVisual:        "#283457",
	fg:              "#c0caf5",
	comment:         "#565f89",
	dark3:           "#545c7e",
	black:           "#15161e",
	borderHighlight: "#27a1b9",
	blue:            "#7aa2f7",
	cyan:            "#7dcfff",
	green:           "#9ece6a",
	orange:          "#ff9e64",
	red:             "#f7768e",
	error:           "#db4b4b",
	warning:         "#e0af68",
	info:            "#0db9d7",
})

// Day is tokyonight-day, the light style: the night palette inverted by
// tokyonight's own code at the default day_brightness of 0.3
// (extras/lua/tokyonight_day.lua).
var Day = fromTokyonight("day", tokyonight{
	bg:              "#e1e2e7",
	bgFloat:         "#d0d5e3",
	bgHighlight:     "#c4c8da",
	bgVisual:        "#b7c1e3",
	fg:              "#3760bf",
	comment:         "#848cb5",
	dark3:           "#8990b3",
	black:           "#b4b5b9",
	borderHighlight: "#4094a3",
	blue:            "#2e7de9",
	cyan:            "#007197",
	green:           "#587539",
	orange:          "#b15c00",
	red:             "#f52a65",
	error:           "#c64343",
	warning:         "#8c6c3e",
	info:            "#07879d",
})

// palettes is every palette Restyle can map from.
var palettes = []Palette{Night, Day}

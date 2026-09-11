package bubbletea

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// TrayStyle carries the lipgloss styles Render uses for each part of the
// tray. The zero value renders in the terminal's default colours (every
// Style field is lipgloss.NewStyle()'s zero value, which applies no
// colour), so a caller only needs to set what it wants to change.
type TrayStyle struct {
	Title Style
	Key   Style
	Desc  Style
	Group Style // description colour for a group line, applied over Desc
	Box   Style // border/frame around the whole tray
}

// Style is a lipgloss.Style; aliased so callers of this package do not need
// their own lipgloss import just to build a TrayStyle.
type Style = lipgloss.Style

// DefaultTrayStyle is a minimal, colour-light style: a rounded border and a
// bold title, otherwise the terminal's default foreground. Tools that want
// the tokyonight-style colouring vimkeys/tview uses via tview.Styles
// should build their own TrayStyle from their own palette instead — lipgloss
// colours and tview's Styles struct are not the same type.
var DefaultTrayStyle = TrayStyle{
	Title: lipgloss.NewStyle().Bold(true),
	Box:   lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1),
}

// Render draws l's current tray content as a string ready to compose into
// a bubbletea View, using style for colours and the border. It returns ""
// when no chord is pending — a caller can render that unconditionally and
// simply not reserve space for an empty result.
//
// Layout is deliberately simple compared to vimkeys/tview's Tray: one
// entry per line, key and description in two aligned columns. tview packs
// entries into as many columns as the root's width allows because it
// draws into a fixed, already-known terminal rect; a bubbletea View has no
// equivalent fixed rect to consult at render time, so multi-column packing
// is left to a caller that wants it, using content.Lines directly.
func Render(l *Leader, style TrayStyle) string {
	content, open := l.Content()
	if !open {
		return ""
	}

	keyCol := style.Key.Width(content.KeyWidth)
	var rows []string
	for _, line := range content.Lines {
		desc := style.Desc
		if line.Group {
			desc = style.Desc.Inherit(style.Group)
		}
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top,
			keyCol.Render(line.Key), "  ", desc.Render(line.Desc)))
	}

	body := strings.Join(rows, "\n")
	title := style.Title.Render(strings.TrimSpace(content.Title))
	return style.Box.Render(title + "\n" + body)
}

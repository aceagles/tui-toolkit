package bubbletea

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/aceagles/tui-toolkit/vimkeys/core"
)

// TrayStyle carries the lipgloss styles Render uses for each part of the
// tray. The zero value renders in the terminal's default colours (every
// Style field is lipgloss.NewStyle()'s zero value, which applies no
// colour), so a caller only needs to set what it wants to change.
//
// Box's own Border() and Padding() settings, if any, are not applied:
// Render draws the box border itself (see Render's doc for why), so Box
// supplies only the colour — Foreground and/or Background — painted onto
// the border characters and the top-border title.
type TrayStyle struct {
	Title Style
	Key   Style
	Desc  Style
	Group Style // description colour for a group line, applied over Desc
	Box   Style // colour for the border and the title inside it
}

// Style is a lipgloss.Style; aliased so callers of this package do not need
// their own lipgloss import just to build a TrayStyle.
type Style = lipgloss.Style

// DefaultTrayStyle is a minimal, colour-light style: a bold title,
// otherwise the terminal's default foreground for everything, border
// included. Tools that want the tokyonight-style colouring vimkeys/tview
// uses via tview.Styles should build their own TrayStyle from their own
// palette instead — lipgloss colours and tview's Styles struct are not the
// same type.
var DefaultTrayStyle = TrayStyle{
	Title: lipgloss.NewStyle().Bold(true),
}

// boxBorder is the box-drawing rune set Render uses for the tray's frame:
// the same light/normal set (┌─┐│└─┘) vimkeys/tview gets from tcell's
// default Borders, so the two adapters look identical rather than merely
// similar.
var boxBorder = lipgloss.NormalBorder()

// Render draws l's current tray content as a string ready to compose into
// a bubbletea View: a full-width box exactly width cells wide and at most
// maxHeight lines tall, the pending chord's breadcrumb in the top border,
// and entries packed column-major by vimkeys/core.LayoutTray — the same
// function vimkeys/tview's Tray uses, so the two adapters cannot drift
// apart. It returns "" when no chord is pending, or when width/maxHeight
// are too small to draw a box at all.
//
// lipgloss has no API for a title embedded in a border (Style.Border
// draws a uniform edge with no room for text), so Render builds the top
// and bottom border strings itself rather than reaching for
// Style.Border/BorderTitle. Every returned line is exactly width cells,
// measured with lipgloss.Width so ANSI colour codes are not mistaken for
// content; Render never returns more than maxHeight lines.
//
// Entries beyond the row budget do not fit — LayoutTray reports how many
// were dropped, and Render says so in the bottom border ("N more") rather
// than silently truncating the way vimkeys/tview does; tview can afford
// silence because the rest of its tray is still on screen for a static
// TUI, but a bubbletea View is regenerated from scratch each render and a
// caller has no other way to learn something didn't fit.
func Render(l *Leader, style TrayStyle, width, maxHeight int) string {
	content, open := l.Content()
	if !open || width < 6 || maxHeight < 3 {
		return ""
	}

	innerWidth := width - 4 // border plus one cell of padding each side, matching vimkeys/tview
	naturalColWidth := content.KeyWidth + 2 + content.DescWidth
	layout, dropped := core.LayoutTray(len(content.Lines), naturalColWidth, innerWidth, maxHeight-2)

	grid := make([][]string, layout.Rows)
	blank := strings.Repeat(" ", layout.ColWidth)
	for r := range grid {
		grid[r] = make([]string, layout.Cols)
		for c := range grid[r] {
			grid[r][c] = blank
		}
	}
	for i, cell := range layout.Cells {
		grid[cell.Row][cell.Col] = renderEntry(style, content.Lines[i], content.KeyWidth, layout.ColWidth)
	}

	lines := make([]string, 0, layout.Rows+2)
	lines = append(lines, borderLine(style.Box, style.Title, boxBorder.TopRight, content.Title, width))
	gap := strings.Repeat(" ", core.TrayGap)
	for _, row := range grid {
		lines = append(lines, contentRow(style.Box, strings.Join(row, gap), innerWidth))
	}
	lines = append(lines, borderLine(style.Box, style.Box, boxBorder.BottomRight, droppedLabel(dropped), width))
	return strings.Join(lines, "\n")
}

// renderEntry renders one tray line's key and description in style, then
// pads or (if the line's natural width somehow exceeds colWidth — only
// possible when the one-column rule has shrunk colWidth below what a
// terminal too narrow to hold it can spare) clips the result to exactly
// colWidth cells, so callers can join a row of these with a plain gap and
// get a rectangular grid.
func renderEntry(style TrayStyle, line core.TrayLine, keyWidth, colWidth int) string {
	desc := style.Desc
	if line.Group {
		desc = style.Desc.Inherit(style.Group)
	}
	entry := lipgloss.JoinHorizontal(lipgloss.Top,
		style.Key.Width(keyWidth).Render(line.Key), "  ", desc.Render(line.Desc))
	switch pad := colWidth - lipgloss.Width(entry); {
	case pad > 0:
		entry += strings.Repeat(" ", pad)
	case pad < 0:
		entry = lipgloss.NewStyle().MaxWidth(colWidth).Render(entry)
	}
	return entry
}

// contentRow wraps one already-assembled row of tray entries with the
// box's left/right border and one cell of padding on each side, padding
// the row out to innerWidth first so every returned row is the same
// width regardless of how many columns it happened to fill.
func contentRow(box Style, row string, innerWidth int) string {
	if pad := innerWidth - lipgloss.Width(row); pad > 0 {
		row += strings.Repeat(" ", pad)
	}
	return box.Render(boxBorder.Left) + " " + row + " " + box.Render(boxBorder.Right)
}

// borderLine builds one horizontal border of the box — top or bottom —
// exactly width cells wide, with text left-aligned immediately after the
// left corner the way tview's Box draws its title. cornerRight names
// which of boxBorder's two right corners to use ("┐" for the top, "┘" for
// the bottom); the left corner is inferred from it so callers only supply
// the one variable part.
func borderLine(box, text Style, cornerRight, label string, width int) string {
	cornerLeft := boxBorder.TopLeft
	if cornerRight == boxBorder.BottomRight {
		cornerLeft = boxBorder.BottomLeft
	}

	inner := max(width-2, 0)
	if lipgloss.Width(label) > inner {
		label = lipgloss.NewStyle().MaxWidth(max(inner-1, 0)).Render(label) + "…"
		if lipgloss.Width(label) > inner {
			label = lipgloss.NewStyle().MaxWidth(inner).Render(label)
		}
	}
	pad := max(inner-lipgloss.Width(label), 0)

	return box.Render(cornerLeft) + text.Render(label) +
		box.Render(strings.Repeat(boxBorder.Top, pad)) + box.Render(cornerRight)
}

// droppedLabel names how many tray lines the row budget left out, ready
// to sit in the bottom border ("" when nothing was dropped).
func droppedLabel(n int) string {
	if n <= 0 {
		return ""
	}
	return fmt.Sprintf(" %d more ", n)
}

package core

// TrayGap is the blank cells left between tray columns. Both framework
// adapters must use the same value for their layouts to match, so it is
// exported rather than duplicated as a local constant in each one.
const TrayGap = 3

// TrayCell places one line of laid-out tray content at a column and row.
// Lines are laid out column-major: a column fills top to bottom before the
// next column starts, the way vim's which-key panels do.
type TrayCell struct {
	Col, Row int
}

// TrayLayout is the column-major placement of a TrayContent's lines into a
// size budget, as computed by LayoutTray. Cells[i] is where the i-th line
// of that TrayContent sits; a line beyond len(Cells) did not fit the
// budget — see LayoutTray's dropped return.
type TrayLayout struct {
	Cols, Rows, ColWidth int
	Cells                []TrayCell
}

// LayoutTray computes column-major placement for n lines whose natural
// column width — one line's key, gap and description, in display cells —
// is colWidth, within an innerWidth x maxRows budget. This is the layout
// vimkeys/tview's tray has always used internally, pulled out here so
// vimkeys/bubbletea can share it instead of drawing its own: as many
// colWidth+TrayGap columns as innerWidth allows (at least one), enough
// rows to hold n lines split across those columns but capped at maxRows,
// and — since a single column of a wide box wastes the width the box
// already has for descriptions — a layout with exactly one column widens
// ColWidth to innerWidth rather than leaving it at colWidth.
//
// LayoutTray takes colWidth as a number rather than the lines themselves:
// core has no framework width function of its own (see the package doc),
// and the right way to measure text differs by adapter — vimkeys/tview
// must re-measure after escaping "[" for tview.Print, while
// vimkeys/bubbletea can use lipgloss.Width directly on unescaped text.
// Handing over the number keeps that choice in the adapter without
// teaching core about either one.
//
// n lines beyond Rows*Cols do not fit the budget; LayoutTray places as
// many as it can, column-major, and reports how many it left out so a
// caller can say so, the way vimkeys/tview's row cap has always silently
// dropped them.
func LayoutTray(n, colWidth, innerWidth, maxRows int) (layout TrayLayout, dropped int) {
	if n <= 0 || maxRows <= 0 {
		return TrayLayout{}, max(n, 0)
	}

	cols := max(1, (innerWidth+TrayGap)/(colWidth+TrayGap))
	rows := min((n+cols-1)/cols, maxRows)
	if cols == 1 {
		colWidth = innerWidth
	}

	visible := min(n, rows*cols)
	cells := make([]TrayCell, visible)
	for i := range cells {
		cells[i] = TrayCell{Col: i / rows, Row: i % rows}
	}
	return TrayLayout{Cols: cols, Rows: rows, ColWidth: colWidth, Cells: cells}, n - visible
}

package core

import "testing"

// With colWidth=10 and TrayGap=3, a column costs 13 cells and the formula
// is cols = max(1, (innerWidth+3)/13); these are the innerWidth values
// either side of where cols steps from 1 to 2, 2 to 3, and 3 to 4. Testing
// exactly on the breakpoint would pass even with an off-by-one formula, so
// each case below is paired with its neighbour on the other side.
func TestLayoutTrayColumnBreakpoints(t *testing.T) {
	for _, tc := range []struct {
		name       string
		innerWidth int
		wantCols   int
	}{
		{"1 to 2 boundary: just under", 22, 1},
		{"1 to 2 boundary: just over", 23, 2},
		{"2 to 3 boundary: just under", 35, 2},
		{"2 to 3 boundary: just over", 36, 3},
		{"3 to 4 boundary: just under", 48, 3},
		{"3 to 4 boundary: just over", 49, 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			layout, _ := LayoutTray(24, 10, tc.innerWidth, 100)
			if layout.Cols != tc.wantCols {
				t.Fatalf("innerWidth=%d: Cols = %d, want %d", tc.innerWidth, layout.Cols, tc.wantCols)
			}
		})
	}
}

// TestLayoutTrayOneColumnWidensToInner exercises the one-column rule: with
// only one column fitting, ColWidth must widen to the full inner width
// instead of staying at the natural (narrower) column width, so a single
// long description gets to use the whole box.
func TestLayoutTrayOneColumnWidensToInner(t *testing.T) {
	layout, _ := LayoutTray(5, 10, 15, 100)
	if layout.Cols != 1 {
		t.Fatalf("Cols = %d, want 1", layout.Cols)
	}
	if layout.ColWidth != 15 {
		t.Fatalf("ColWidth = %d, want the inner width 15 (one-column rule)", layout.ColWidth)
	}
}

// TestLayoutTrayMultiColumnKeepsNaturalWidth is the one-column rule's
// counterpart: once a second column fits, ColWidth must stay at the
// natural width passed in, not widen to fill the box.
func TestLayoutTrayMultiColumnKeepsNaturalWidth(t *testing.T) {
	layout, _ := LayoutTray(24, 10, 30, 100)
	if layout.Cols < 2 {
		t.Fatalf("Cols = %d, want at least 2 for this fixture", layout.Cols)
	}
	if layout.ColWidth != 10 {
		t.Fatalf("ColWidth = %d, want the natural width 10 unchanged", layout.ColWidth)
	}
}

// TestLayoutTrayMaxRowsCap confirms the row cap drops trailing lines
// rather than growing the box past maxRows, and reports how many were
// dropped.
func TestLayoutTrayMaxRowsCap(t *testing.T) {
	// colWidth=40, innerWidth=40 forces exactly one column, so all 10
	// lines would naturally need 10 rows; capped at 3.
	layout, dropped := LayoutTray(10, 40, 40, 3)
	if layout.Rows != 3 {
		t.Fatalf("Rows = %d, want the maxRows cap of 3", layout.Rows)
	}
	if len(layout.Cells) != 3 {
		t.Fatalf("len(Cells) = %d, want 3 (one column, 3 rows)", len(layout.Cells))
	}
	if dropped != 7 {
		t.Fatalf("dropped = %d, want 7 (10 lines - 3 shown)", dropped)
	}
}

// TestLayoutTrayColumnMajorOrder checks the fill order directly: line 0
// must sit at the top of column 0, and the line that starts column 1 must
// be the one right after column 0 fills up — not the line one row major
// order would place there.
func TestLayoutTrayColumnMajorOrder(t *testing.T) {
	// colWidth=10, innerWidth=30 -> cols=2 (see TestLayoutTrayColumnBreakpoints);
	// 6 lines over 2 columns -> rows=3.
	layout, dropped := LayoutTray(6, 10, 30, 100)
	if layout.Cols != 2 || layout.Rows != 3 {
		t.Fatalf("Cols=%d Rows=%d, want 2x3 for this fixture", layout.Cols, layout.Rows)
	}
	if dropped != 0 {
		t.Fatalf("dropped = %d, want 0", dropped)
	}
	want := []TrayCell{
		{Col: 0, Row: 0}, // line 0: top of column 0
		{Col: 0, Row: 1},
		{Col: 0, Row: 2}, // line 2: bottom of column 0 - column 0 is now full
		{Col: 1, Row: 0}, // line 3: top of column 1, not row 0's second entry
		{Col: 1, Row: 1},
		{Col: 1, Row: 2},
	}
	if len(layout.Cells) != len(want) {
		t.Fatalf("len(Cells) = %d, want %d", len(layout.Cells), len(want))
	}
	for i, cell := range layout.Cells {
		if cell != want[i] {
			t.Fatalf("Cells[%d] = %+v, want %+v", i, cell, want[i])
		}
	}
}

// TestLayoutTrayZeroLines and TestLayoutTrayNoRowBudget cover the two ways
// a layout can legitimately show nothing.
func TestLayoutTrayZeroLines(t *testing.T) {
	layout, dropped := LayoutTray(0, 10, 40, 10)
	if layout.Cols != 0 || layout.Rows != 0 || len(layout.Cells) != 0 {
		t.Fatalf("layout = %+v, want the zero value for n=0", layout)
	}
	if dropped != 0 {
		t.Fatalf("dropped = %d, want 0 for n=0", dropped)
	}
}

func TestLayoutTrayNoRowBudget(t *testing.T) {
	layout, dropped := LayoutTray(5, 10, 40, 0)
	if len(layout.Cells) != 0 {
		t.Fatalf("Cells = %+v, want none with maxRows=0", layout.Cells)
	}
	if dropped != 5 {
		t.Fatalf("dropped = %d, want all 5 lines dropped with maxRows=0", dropped)
	}
}

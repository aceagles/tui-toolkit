package layout

import (
	"fmt"

	"github.com/aceagles/tui-toolkit/theme"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// Axis is the direction a Stack lays its panes out in.
type Axis int

const (
	Horizontal Axis = iota // side by side; Specs are in columns
	Vertical               // top to bottom; Specs are in rows
)

// Item is one pane in a Stack.
type Item struct {
	p      tview.Primitive
	spec   Spec
	specFn func() Spec
}

// SetSpec replaces the pane's static size request.
func (it *Item) SetSpec(s Spec) *Item {
	it.spec = s
	return it
}

// SetSpecFunc makes the pane's size request dynamic: fn is called on every
// frame in place of the static Spec, so a table can report the width of
// what it currently holds. It runs inside Draw with the application lock
// held; it must not call Application methods.
func (it *Item) SetSpecFunc(fn func() Spec) *Item {
	it.specFn = fn
	return it
}

func (it *Item) resolve() Spec {
	if it.specFn != nil {
		return it.specFn().Normalize()
	}
	return it.spec.Normalize()
}

// Stack lays panes out along one axis and re-solves their sizes from its
// own rect on every Draw, so a terminal resize needs no event plumbing.
// Build it like a tview.Flex:
//
//	body := layout.NewStack(layout.Horizontal)
//	body.Add(list, layout.Spec{Min: 24, Pref: 30, Priority: 1})
//	body.Add(detail, layout.Spec{Min: 40, Weight: 1})
//
// A pane that cannot have its Min is not drawn; Hidden counts those.
// Stacks nest: a Stack is a Primitive, and Spec gives a nested stack's
// request to a parent laid out the same way.
//
// User offsets (from Layout's resize keys) are kept per primitive, so a
// tool that rebuilds a stack with Clear and Add in one event-loop turn
// keeps them.
type Stack struct {
	*tview.Box
	axis   Axis
	items  []*Item
	offset map[tview.Primitive]int

	// From the last Draw; Layout's resize keys read these.
	avail int
	sizes []int
	shown []bool
	specs []Spec // resolved, without offsets

	zoom     tview.Primitive // drawn alone over the whole stack; set by Layout
	badge    bool
	onHidden func(hidden int)
}

// NewStack returns an empty stack laid out along axis.
func NewStack(axis Axis) *Stack {
	return &Stack{Box: tview.NewBox(), axis: axis, offset: map[tview.Primitive]int{}}
}

// Add appends a pane and returns its Item for SetSpecFunc.
func (s *Stack) Add(p tview.Primitive, spec Spec) *Item {
	it := &Item{p: p, spec: spec}
	s.items = append(s.items, it)
	s.shown = append(s.shown, true)
	return it
}

// Remove drops every item holding p.
func (s *Stack) Remove(p tview.Primitive) *Stack {
	for i := len(s.items) - 1; i >= 0; i-- {
		if s.items[i].p == p {
			s.items = append(s.items[:i], s.items[i+1:]...)
			if i < len(s.shown) {
				s.shown = append(s.shown[:i], s.shown[i+1:]...)
			}
		}
	}
	return s
}

// Clear drops every item. Offsets stay for panes added back before the
// next Draw.
func (s *Stack) Clear() *Stack {
	s.items, s.shown = nil, nil
	return s
}

// GetItemCount returns the number of panes, shown or not. With GetItem it
// matches tview.Flex, so theme.Restyle walks into a Stack.
func (s *Stack) GetItemCount() int { return len(s.items) }

// GetItem returns the primitive of pane i.
func (s *Stack) GetItem(i int) tview.Primitive { return s.items[i].p }

// ShowHiddenBadge draws "+N hidden" in theme.RoleWarn on the stack's bottom
// row while overflow hides N panes, right-aligned to the last shown pane in
// a Horizontal stack. With boxed panes that row is their bottom border.
func (s *Stack) ShowHiddenBadge(on bool) *Stack {
	s.badge = on
	return s
}

// SetHiddenFunc registers fn to be called from Draw when the set of panes
// overflow hides changes, with the new count. It runs on the event loop
// with the application lock held: it may update the tool's own state
// (rebuild tmuxnav, say), but anything that calls Application methods, such
// as moving focus off a pane that just went, must be queued from a new
// goroutine, since QueueUpdate waits for the update to run:
//
//	go app.QueueUpdateDraw(moveFocusOffHiddenPanes)
func (s *Stack) SetHiddenFunc(fn func(hidden int)) *Stack {
	s.onHidden = fn
	return s
}

// Hidden counts the panes the last Draw hid for want of space.
func (s *Stack) Hidden() int {
	n := 0
	for _, sh := range s.shown {
		if !sh {
			n++
		}
	}
	return n
}

// Shown reports whether the last Draw showed the pane holding p. Panes
// added since then count as shown.
func (s *Stack) Shown(p tview.Primitive) bool {
	for i, it := range s.items {
		if it.p == p {
			return s.shownAt(i)
		}
	}
	return false
}

func (s *Stack) shownAt(i int) bool { return i >= len(s.shown) || s.shown[i] }

// Spec is this stack's request to a parent laid out along the same axis:
// Min is the Min of the pane overflow would hide last, Pref and Max are the
// sums of the panes' (Max 0 if any pane is unbounded), Weight is the
// largest pane Weight. Hand it over with SetSpecFunc:
//
//	parent.Add(tiles, layout.Spec{}).SetSpecFunc(tiles.Spec)
func (s *Stack) Spec() Spec {
	var out Spec
	keep, unbounded := -1, false
	var specs []Spec
	for _, it := range s.items {
		specs = append(specs, it.resolve())
	}
	for i, sp := range specs {
		if sp.Max > 0 {
			sp.Pref = clamp(sp.Pref+s.offset[s.items[i].p], sp.Min, sp.Max)
		} else {
			sp.Pref = max(sp.Pref+s.offset[s.items[i].p], sp.Min)
		}
		out.Pref += sp.Pref
		out.Max += sp.Max
		unbounded = unbounded || sp.Max == 0
		out.Weight = max(out.Weight, sp.Weight)
		if keep < 0 || sp.Priority > specs[keep].Priority {
			keep = i
		}
	}
	if keep >= 0 {
		out.Min = specs[keep].Min
	}
	if unbounded {
		out.Max = 0
	}
	return out
}

// Draw solves the pane sizes for the current rect and draws the shown
// panes, the focused one last.
func (s *Stack) Draw(screen tcell.Screen) {
	s.DrawForSubclass(screen, s)
	x, y, w, h := s.GetInnerRect()

	if s.zoom != nil {
		if !contains(s, s.zoom) {
			s.zoom = nil
		} else {
			s.zoom.SetRect(x, y, w, h)
			s.zoom.Draw(screen)
			return
		}
	}

	s.avail = w
	if s.axis == Vertical {
		s.avail = h
	}
	plan := s.solve()
	s.purgeOffsets()

	changed := len(s.shown) != len(plan.Shown)
	for i := range plan.Shown {
		changed = changed || s.shown[i] != plan.Shown[i]
	}
	s.sizes, s.shown = plan.Sizes, plan.Shown

	pos := x
	if s.axis == Vertical {
		pos = y
	}
	var focused tview.Primitive
	for i, it := range s.items {
		if !plan.Shown[i] || it.p == nil {
			continue
		}
		size := plan.Sizes[i]
		if s.axis == Horizontal {
			it.p.SetRect(pos, y, size, h)
		} else {
			it.p.SetRect(x, pos, w, size)
		}
		pos += size
		if it.p.HasFocus() {
			focused = it.p // drawn last, like tview.Flex, so its cursor wins
		} else {
			it.p.Draw(screen)
		}
	}
	if focused != nil {
		focused.Draw(screen)
	}

	// The badge ends one cell short of the last shown pane, so a boxed
	// pane keeps its corner.
	if n, end := s.Hidden(), pos-x-1; s.badge && n > 0 && end > 0 && h > 0 {
		if s.axis == Vertical {
			end = w - 1
		}
		text := fmt.Sprintf("%s +%d hidden [-]", theme.Current().Tag(theme.RoleWarn), n)
		tview.Print(screen, text, x, y+h-1, end, tview.AlignRight, tcell.ColorDefault)
	}
	if changed && s.onHidden != nil {
		s.onHidden(s.Hidden())
	}
}

// solve resolves every pane's Spec, re-clamps the user offsets against the
// current space, and solves with the offsets applied to Pref.
func (s *Stack) solve() Plan {
	s.specs = s.specs[:0]
	for _, it := range s.items {
		s.specs = append(s.specs, it.resolve())
	}
	// Which panes show depends only on Min and Priority, so this first
	// pass fixes the set the offsets are clamped against.
	shown := Solve(s.avail, s.specs).Shown
	eff := make([]Spec, len(s.specs))
	for i, sp := range s.specs {
		p := s.items[i].p
		if off, ok := s.offset[p]; ok {
			// Less space can take back a pane's growth, but never turn it
			// into shrinkage it will keep once the space returns.
			lo, hi := s.prefRange(i, shown)
			next := clamp(sp.Pref+off, lo, hi) - sp.Pref
			if (off > 0) != (next > 0) {
				next = 0
			}
			s.offset[p] = next
			sp.Pref += next
		}
		eff[i] = sp
	}
	return Solve(s.avail, eff)
}

// prefRange is the range pane i's Pref may take with offsets: from its Min
// to its Max, and never more than the space the other shown panes' Mins
// leave.
func (s *Stack) prefRange(i int, shown []bool) (lo, hi int) {
	others := 0
	for j, sp := range s.specs {
		if j != i && shown[j] {
			others += sp.Min
		}
	}
	lo, hi = s.specs[i].Min, s.avail-others
	if m := s.specs[i].Max; m > 0 {
		hi = min(hi, m)
	}
	return lo, max(hi, lo)
}

// purgeOffsets forgets offsets for panes no longer in the stack.
func (s *Stack) purgeOffsets() {
	for p := range s.offset {
		found := false
		for _, it := range s.items {
			if it.p == p {
				found = true
				break
			}
		}
		if !found {
			delete(s.offset, p)
		}
	}
}

// resizeTo sets pane i's offset so its Pref becomes size, clamped to what
// the pane may take. It reports whether the offset changed.
func (s *Stack) resizeTo(i, size int) bool {
	if i >= len(s.specs) || i >= len(s.shown) {
		return false
	}
	lo, hi := s.prefRange(i, s.shown)
	p := s.items[i].p
	was := s.offset[p]
	s.offset[p] = clamp(size, lo, hi) - s.specs[i].Pref
	if s.offset[p] == 0 {
		delete(s.offset, p)
	}
	return s.offset[p] != was
}

// equalise clears the offsets here and in nested stacks.
func (s *Stack) equalise() {
	clear(s.offset)
	for _, it := range s.items {
		if sub, ok := it.p.(*Stack); ok {
			sub.equalise()
		}
	}
}

// Focus hands focus to the first pane.
func (s *Stack) Focus(delegate func(p tview.Primitive)) {
	for _, it := range s.items {
		if it.p != nil {
			delegate(it.p)
			return
		}
	}
	s.Box.Focus(delegate)
}

// HasFocus reports whether any pane, shown or not, has focus.
func (s *Stack) HasFocus() bool {
	for _, it := range s.items {
		if it.p != nil && it.p.HasFocus() {
			return true
		}
	}
	return s.Box.HasFocus()
}

// InputHandler passes keys to the focused pane.
func (s *Stack) InputHandler() func(*tcell.EventKey, func(tview.Primitive)) {
	return s.WrapInputHandler(func(ev *tcell.EventKey, setFocus func(tview.Primitive)) {
		for _, it := range s.items {
			if it.p != nil && it.p.HasFocus() {
				if h := it.p.InputHandler(); h != nil {
					h(ev, setFocus)
				}
				return
			}
		}
	})
}

// PasteHandler passes pasted text to the focused pane.
func (s *Stack) PasteHandler() func(string, func(tview.Primitive)) {
	return s.WrapPasteHandler(func(text string, setFocus func(tview.Primitive)) {
		for _, it := range s.items {
			if it.p != nil && it.p.HasFocus() {
				if h := it.p.PasteHandler(); h != nil {
					h(text, setFocus)
				}
				return
			}
		}
	})
}

// MouseHandler passes mouse events to the first shown pane that takes them.
func (s *Stack) MouseHandler() func(tview.MouseAction, *tcell.EventMouse, func(tview.Primitive)) (bool, tview.Primitive) {
	return s.WrapMouseHandler(func(action tview.MouseAction, ev *tcell.EventMouse, setFocus func(tview.Primitive)) (bool, tview.Primitive) {
		if !s.InRect(ev.Position()) {
			return false, nil
		}
		if s.zoom != nil {
			return s.zoom.MouseHandler()(action, ev, setFocus)
		}
		for i, it := range s.items {
			if it.p == nil || !s.shownAt(i) {
				continue
			}
			if ok, capture := it.p.MouseHandler()(action, ev, setFocus); ok {
				return ok, capture
			}
		}
		return false, nil
	})
}

// container is what tview.Flex and Stack share for walking children.
type container interface {
	GetItemCount() int
	GetItem(int) tview.Primitive
}

// contains reports whether p is c or sits anywhere inside it.
func contains(c, p tview.Primitive) bool {
	if c == nil || p == nil {
		return false
	}
	if c == p {
		return true
	}
	switch t := c.(type) {
	case container:
		for i := range t.GetItemCount() {
			if contains(t.GetItem(i), p) {
				return true
			}
		}
	case *tview.Frame:
		return contains(t.GetPrimitive(), p)
	}
	return false
}

// clamp limits v to [lo, hi]; lo wins if they cross.
func clamp(v, lo, hi int) int {
	return max(min(v, hi), lo)
}

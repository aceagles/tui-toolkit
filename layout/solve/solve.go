// Package solve is the pure pane-sizing algorithm behind layout: given an
// available extent and a list of Specs, it decides how many cells each pane
// gets and which panes overflow hides. It has zero imports beyond the
// standard library, and must stay that way — a bubbletea tool that only
// wants Solve should never be forced to pull in tview or tcell. See
// TestNoTviewOrTcellDependency in solve_test.go, which fails the build if
// that constraint is ever broken.
//
// Package layout re-exports every name here as a type alias (Spec = Spec,
// Plan = Plan, and so on) so its own callers see no difference; this
// package is where the algorithm and its doc comments actually live.
package solve

// Spec is one pane's size request along its stack's axis, in cells: columns
// in a Horizontal stack, rows in a Vertical one.
//
// Min is never violated: a pane that cannot have its Min is hidden instead.
// Pref is what the pane gets once every shown pane has its Min, if space
// allows. Weight shares out what is left after every Pref, up to Max. Max 0
// means unbounded. Priority decides which panes overflow hides first: lowest
// Priority first, and the rightmost (bottom-most) among equals.
type Spec struct {
	Min, Pref, Max, Weight int
	Priority               int
}

// Fixed is a pane that is always exactly n cells, such as a one-line header.
func Fixed(n int) Spec { return Spec{Min: n, Pref: n, Max: n} }

// Normalize returns s with the fields made consistent: nothing negative,
// Pref within [Min, Max], Max no smaller than Min. Stack calls it directly
// when resolving a pane's Spec; Solve calls it internally on every Spec it
// is given.
func (s Spec) Normalize() Spec {
	s.Min = max(s.Min, 0)
	s.Max = max(s.Max, 0)
	s.Weight = max(s.Weight, 0)
	if s.Max > 0 && s.Max < s.Min {
		s.Max = s.Min
	}
	s.Pref = max(s.Pref, s.Min)
	if s.Max > 0 {
		s.Pref = min(s.Pref, s.Max)
	}
	return s
}

// Plan is Solve's answer: a size for each pane and whether it is shown.
// Hidden panes have size 0.
type Plan struct {
	Sizes []int
	Shown []bool
}

// Hidden counts the panes overflow hid.
func (p Plan) Hidden() int {
	n := 0
	for _, s := range p.Shown {
		if !s {
			n++
		}
	}
	return n
}

// Solve shares avail cells among panes, greedily:
//
//  1. Every pane gets its Min. If the Mins do not fit, panes are hidden
//     (lowest Priority, then rightmost, first) until they do.
//  2. Each shown pane, in order, grows towards its Pref from what is left.
//  3. What is left after that goes out by Weight, capped at Max.
//
// Space nobody takes is left unused; a stack draws nothing there. Solve is
// pure: the same width always gives the same plan, so panes hidden as the
// terminal narrows come back in reverse order as it widens.
func Solve(avail int, specs []Spec) Plan {
	n := len(specs)
	p := Plan{Sizes: make([]int, n), Shown: make([]bool, n)}
	avail = max(avail, 0)
	sp := make([]Spec, n)
	need := 0
	for i, s := range specs {
		sp[i] = s.Normalize()
		p.Shown[i] = true
		need += sp[i].Min
	}

	for need > avail {
		v := victim(sp, p.Shown)
		if v < 0 {
			break // only Min-0 panes left; they cost nothing
		}
		p.Shown[v] = false
		need -= sp[v].Min
	}

	rem := avail - need
	for i := range sp {
		if p.Shown[i] {
			p.Sizes[i] = sp[i].Min
		}
	}
	for i := range sp {
		if !p.Shown[i] || rem == 0 {
			continue
		}
		grow := min(sp[i].Pref-p.Sizes[i], rem)
		p.Sizes[i] += grow
		rem -= grow
	}
	distribute(sp, p, rem)
	return p
}

// victim picks the shown pane with a non-zero Min to hide next: lowest
// Priority, rightmost among equals. -1 when there is none.
func victim(sp []Spec, shown []bool) int {
	v := -1
	for i, s := range sp {
		if !shown[i] || s.Min == 0 {
			continue
		}
		if v < 0 || s.Priority <= sp[v].Priority {
			v = i
		}
	}
	return v
}

// distribute hands rem cells to shown panes by Weight, never past Max.
// Each round gives every open pane its floor share; when the shares all
// round to zero, one cell goes to each open pane in order.
func distribute(sp []Spec, p Plan, rem int) {
	open := func(i int) bool {
		return p.Shown[i] && sp[i].Weight > 0 && (sp[i].Max == 0 || p.Sizes[i] < sp[i].Max)
	}
	room := func(i int) int {
		if sp[i].Max == 0 {
			return rem
		}
		return sp[i].Max - p.Sizes[i]
	}
	for rem > 0 {
		total := 0
		for i := range sp {
			if open(i) {
				total += sp[i].Weight
			}
		}
		if total == 0 {
			return
		}
		given := 0
		shares := make([]int, len(sp))
		for i := range sp {
			if open(i) {
				shares[i] = min(rem*sp[i].Weight/total, room(i))
				given += shares[i]
			}
		}
		if given == 0 {
			for i := range sp {
				if rem > 0 && open(i) {
					p.Sizes[i]++
					rem--
				}
			}
			continue
		}
		for i, s := range shares {
			p.Sizes[i] += s
		}
		rem -= given
	}
}

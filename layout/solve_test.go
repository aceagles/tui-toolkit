package layout

import (
	"math/rand"
	"reflect"
	"testing"
)

func TestSolve(t *testing.T) {
	tests := []struct {
		name  string
		avail int
		specs []Spec
		sizes []int
		shown []bool
	}{
		{"mins first when prefs do not fit", 50,
			[]Spec{{Min: 20, Pref: 40}, {Min: 20, Pref: 40}},
			[]int{30, 20}, []bool{true, true}},
		{"pref before weight", 100,
			[]Spec{{Min: 10, Pref: 30}, {Min: 10, Weight: 1}},
			[]int{30, 70}, []bool{true, true}},
		{"pref in order when short", 45,
			[]Spec{{Min: 10, Pref: 30}, {Min: 10, Pref: 30}},
			[]int{30, 15}, []bool{true, true}},
		{"max caps weight, rest goes to the other", 100,
			[]Spec{{Min: 10, Max: 20, Weight: 1}, {Min: 10, Weight: 1}},
			[]int{20, 80}, []bool{true, true}},
		{"remainder by weight", 40,
			[]Spec{{Weight: 1}, {Weight: 3}},
			[]int{10, 30}, []bool{true, true}},
		{"odd remainder is not lost", 10,
			[]Spec{{Weight: 1}, {Weight: 1}, {Weight: 1}},
			[]int{4, 3, 3}, []bool{true, true, true}},
		{"no weight leaves space unused", 100,
			[]Spec{{Min: 10, Pref: 30, Max: 30}, {Min: 10, Pref: 40, Max: 40}},
			[]int{30, 40}, []bool{true, true}},
		{"all capped leaves space unused", 100,
			[]Spec{{Max: 10, Weight: 1}, {Max: 20, Weight: 5}},
			[]int{10, 20}, []bool{true, true}},
		{"overflow hides rightmost", 50,
			[]Spec{{Min: 20}, {Min: 20}, {Min: 20}},
			[]int{20, 20, 0}, []bool{true, true, false}},
		{"overflow hides lowest priority first", 50,
			[]Spec{{Min: 20}, {Min: 20, Priority: 1}, {Min: 20, Priority: 1}},
			[]int{0, 20, 20}, []bool{false, true, true}},
		{"hidden space goes to the rest", 45,
			[]Spec{{Min: 20, Pref: 25}, {Min: 20, Pref: 25}, {Min: 20}},
			[]int{25, 20, 0}, []bool{true, true, false}},
		{"nothing fits", 5,
			[]Spec{{Min: 10}, {Min: 10}},
			[]int{0, 0}, []bool{false, false}},
		{"min-0 panes are never hidden", 5,
			[]Spec{{Min: 10}, {Weight: 1}},
			[]int{0, 5}, []bool{false, true}},
		{"pref below min is min, max below min is min", 30,
			[]Spec{{Min: 10, Pref: 5, Max: 3}},
			[]int{10}, []bool{true}},
		{"fixed", 10, []Spec{Fixed(3), {Weight: 1}}, []int{3, 7}, []bool{true, true}},
		{"negative avail", -3, []Spec{{Weight: 1}}, []int{0}, []bool{true}},
		{"empty", 10, nil, []int{}, []bool{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Solve(tt.avail, tt.specs)
			if !reflect.DeepEqual(got.Sizes, tt.sizes) || !reflect.DeepEqual(got.Shown, tt.shown) {
				t.Fatalf("Solve(%d) = %v %v; want %v %v", tt.avail, got.Sizes, got.Shown, tt.sizes, tt.shown)
			}
		})
	}
}

// Panes hidden as space shrinks come back in reverse order as it grows.
func TestSolveRestoreOrder(t *testing.T) {
	specs := []Spec{{Min: 24, Pref: 30, Priority: 1}, {Min: 30}, {Min: 30}, {Min: 30}}
	hides := func(avail int) []bool { return Solve(avail, specs).Shown }
	var order []int // pane index in the order it disappears
	prev := hides(200)
	for w := 200; w >= 0; w-- {
		cur := hides(w)
		for i := range cur {
			if prev[i] && !cur[i] {
				order = append(order, i)
			}
		}
		prev = cur
	}
	if want := []int{3, 2, 1, 0}; !reflect.DeepEqual(order, want) {
		t.Fatalf("hide order %v, want %v", order, want)
	}
	var back []int
	prev = hides(0)
	for w := 0; w <= 200; w++ {
		cur := hides(w)
		for i := range cur {
			if !prev[i] && cur[i] {
				back = append(back, i)
			}
		}
		prev = cur
	}
	if want := []int{0, 1, 2, 3}; !reflect.DeepEqual(back, want) {
		t.Fatalf("restore order %v, want %v", back, want)
	}
	if n := Solve(80, specs).Hidden(); n != 2 {
		t.Errorf("Hidden at 80 = %d, want 2", n)
	}
}

// At every width 1..300, for random specs: no shown pane is below its Min
// or above its Max, hidden panes are 0, nothing is over-allocated, and a
// pane hides only when the Mins really do not fit.
func TestSolveProperties(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	for trial := range 200 {
		specs := make([]Spec, 1+rng.Intn(6))
		for i := range specs {
			s := Spec{Min: rng.Intn(50), Weight: rng.Intn(3), Priority: rng.Intn(3)}
			s.Pref = s.Min + rng.Intn(40)
			if rng.Intn(2) == 0 {
				s.Max = s.Pref + rng.Intn(20)
			}
			specs[i] = s
		}
		for w := 1; w <= 300; w++ {
			p := Solve(w, specs)
			total, mins := 0, 0
			for i, s := range specs {
				size := p.Sizes[i]
				total += size
				if !p.Shown[i] {
					if size != 0 {
						t.Fatalf("trial %d w=%d: hidden pane %d has size %d", trial, w, i, size)
					}
					continue
				}
				mins += s.Min
				if size < s.Min {
					t.Fatalf("trial %d w=%d: pane %d size %d below Min %d (%+v)", trial, w, i, size, s.Min, specs)
				}
				if s.Max > 0 && size > s.Max {
					t.Fatalf("trial %d w=%d: pane %d size %d above Max %d", trial, w, i, size, s.Max)
				}
			}
			if total > w {
				t.Fatalf("trial %d w=%d: allocated %d", trial, w, total)
			}
			// The last pane overflow hid (highest Priority, leftmost among
			// the hidden) must not fit back: hiding stops as soon as the
			// Mins fit.
			last := -1
			for i, s := range specs {
				if !p.Shown[i] && (last < 0 || s.Priority > specs[last].Priority) {
					last = i
				}
			}
			if last >= 0 && mins+specs[last].Min <= w {
				t.Fatalf("trial %d w=%d: pane %d hidden though it fits (%+v)", trial, w, last, specs)
			}
		}
	}
}

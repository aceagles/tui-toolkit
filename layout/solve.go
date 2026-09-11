package layout

import "github.com/aceagles/tui-toolkit/layout/solve"

// Spec, Plan, Fixed and Solve alias the layout/solve package, which holds
// the pane-sizing algorithm and its doc comments. They stay here, under
// their original names, so nothing that already imports layout needs to
// change; a tool that wants only the algorithm (no tview or tcell) should
// import layout/solve directly instead.

// Spec is one pane's size request; see solve.Spec.
type Spec = solve.Spec

// Fixed is a pane that is always exactly n cells; see solve.Fixed.
func Fixed(n int) Spec { return solve.Fixed(n) }

// Plan is Solve's answer: a size for each pane and whether it is shown; see
// solve.Plan.
type Plan = solve.Plan

// Solve shares avail cells among panes; see solve.Solve.
func Solve(avail int, specs []Spec) Plan { return solve.Solve(avail, specs) }

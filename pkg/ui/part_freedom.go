package ui

import (
	"errors"
	"fmt"

	"github.com/telesapr/telesapr/pkg/sketch"
)

// showPartFreedom shows, on the screen of link group g that moves in a
// mechanism, the mechanism's size and freedom and how the part can move
// in it, as Sketch.PartFreedoms counts it from the mates: the window's
// side of LinkGroup.Freedom.
func (tw *textWindow) showPartFreedom(g *sketch.Group) {
	ss := tw.ss
	sk := ss.SK
	n := len(sk.Mechanism(g.H))
	parts := "parts"
	if n == 1 {
		parts = "part"
	}
	tw.Printf(false, "   %Ftmechanism%E of %d %s, %d DOF", n, parts, g.Solved.Dof)
	f, err := sk.PartFreedomOf(g.H)
	var text string
	switch {
	case errors.Is(err, sketch.ErrNotSolved):
		text = "not counted: the mechanism is not solved"
	case err != nil:
		text = fmt.Sprintf("not counted: %v", err)
	default:
		text = f.Text(func(v float64) string { return ss.MmToString(v, false) })
	}
	for i, line := range wrapWords("freedom: "+text, maxCols-6) {
		if i == 0 {
			tw.Printf(false, "   %Ftfreedom:%E%s", line[len("freedom:"):])
			continue
		}
		tw.Printf(false, "     %s", line)
	}
}

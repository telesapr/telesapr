// The model of path_parameters.script, made by a script
// (TestPathEquivalenceParameters): a rectangle with the mouse's
// corners at (-20, 20) and (16, -6), its bottom and right sides
// dimensioned by parameters, extruded 80 mm as the interface does by
// default. The bottom side is sides[0] and the right one sides[1].
package main

import . "github.com/spacesolve/spacesolve/pkg/script/api"

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	doc := Current()
	sides := doc.Rect(V(-20, 20, 0), V(16, -6, 0))
	must(doc.SetParameter("k", "4"))
	must(doc.SetParameter("w", "k*10"))
	must(doc.SetParameter("h", "w/2"))
	// Each step regenerates, as each does in the interface; the solver
	// is not a fixed point, so a different sequence of solves can end
	// an ulp away.
	for _, d := range []struct {
		side Entity
		v    float64
		expr string
	}{{sides[0], 36, "w"}, {sides[1], 26, "h"}} {
		c := doc.Length(d.side, d.v)
		doc.Regen()
		must(c.SetExpr(d.expr))
		doc.Regen()
	}
	doc.Extrude(80, "")
}

// A slot and a ring of holes drawn roughly, squared up by the
// constraints the window adds while drawing, made by a script.
//
// Without a window:
//
//	telesapr-cli run examples/scripts/autoconstrain/autoconstrain.go [-]
//
// In the application: Script > Run File, or
//
//	telesapr --script examples/scripts/autoconstrain/autoconstrain.go
//
// Doc.AutoConstrain gives a line, an arc or a circle what the window
// gives one drawn with the mouse: here the slot's bottom, drawn a little
// off level, is made horizontal; the arc from its end, starting square to
// it, tangent to it; and three holes drawn about 3 mm in radius, the
// second and third each the same size as the nearest. The script checks
// the constraints it got and the holes' radii, prints them, and saves
// autoconstrain.tlspr, unless given "-".
package main

import (
	"fmt"
	"math"
	"os"

	"github.com/telesapr/telesapr/pkg/core"
	. "github.com/telesapr/telesapr/pkg/script/api"
)

func main() {
	write := len(os.Args) < 2 || os.Args[1] != "-"
	doc := Current()
	ok := true
	expect := func(what string, e Entity, want core.ConstraintType) {
		c, made := doc.AutoConstrain(e)
		fmt.Printf("%-12s %v\n", what, c)
		if !made || c.Type() != want {
			ok = false
		}
	}

	bottom := doc.Line(V(0, 0, 0), V(40, 0.4, 0))
	expect("bottom", bottom, core.ConstraintHorizontal)
	doc.Regen()
	p := bottom.End().Pos()
	// A half circle from the bottom's end, about a centre square above it.
	end := doc.Arc(p.Plus(V(0, 10, 0)), p, p.Plus(V(0, 20, 0)))
	doc.Coincident(end.Start(), bottom.End())
	expect("end", end, core.ConstraintArcLineTangent)

	holes := []Entity{doc.Circle(V(10, 10, 0), 3)}
	holes = append(holes, doc.Circle(V(20, 10, 0), 3.02))
	expect("second hole", holes[1], core.ConstraintEqualRadius)
	holes = append(holes, doc.Circle(V(30, 10, 0), 2.99))
	expect("third hole", holes[2], core.ConstraintEqualRadius)

	doc.Regen()
	for _, h := range holes {
		if math.Abs(h.Radius()-holes[0].Radius()) > 1e-6 {
			ok = false
		}
	}
	fmt.Printf("holes of radius %.3f mm\n", holes[0].Radius())
	if !ok {
		fmt.Fprintln(os.Stderr, "the constraints are wrong")
		os.Exit(1)
	}
	if write {
		if err := doc.Save("autoconstrain.tlspr"); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
	}
}

// How a group is solved: the options its screen carries, set from a
// script, on a sketch that cannot solve as it stands.
//
// Without a window:
//
//	telesapr-cli run examples/scripts/solve/solve.go
//
// In the application, on the sketch that is open: Script > Run File,
// or
//
//	telesapr --script examples/scripts/solve/solve.go
//
// A rectangle is drawn and held with its two sides, and then with its
// diagonal as well, which is one constraint too many: the group
// reports the surplus and does not solve. Each of the options then
// takes the group through that and back. It writes nothing.
package main

import (
	"fmt"
	"math"
	"os"

	"github.com/telesapr/telesapr/pkg/script/api"
)

func main() {
	doc := api.Current()
	sides := doc.Sketch.Rect(api.V(0, 0, 0), api.V(40, 20, 0))
	doc.Constrain.Horizontal(sides[0])
	doc.Constrain.Horizontal(sides[2])
	doc.Constrain.Vertical(sides[1])
	doc.Constrain.Vertical(sides[3])
	doc.Constrain.Lock(sides[0].Start())
	doc.Constrain.Length(sides[0], 40)
	doc.Constrain.Length(sides[1], 20)
	sketch := doc.Active()
	// The diagonal follows from the two sides, so this is the
	// constraint the solver cannot use.
	doc.Constrain.Distance(sides[0].Start(), sides[1].End(), math.Hypot(40, 20))
	doc.Features.Extrude(4, "plate")
	plate := doc.Active()

	fmt.Printf("the sketch: solved %t, %d to remove\n", sketch.Solved(), len(sketch.Conflicts()))

	// AllowRedundant: the surplus is left in, and the group solves.
	sketch.SetAllowRedundant(true)
	fmt.Printf("allowing the redundant constraint: solved %t\n", sketch.Solved())
	sketch.SetAllowRedundant(false)

	// RelaxConstraints: the constraints the sketch cannot use are
	// dropped instead, so a group with several ways of holding the same
	// thing solves.
	sketch.SetRelaxConstraints(true)
	fmt.Printf("relaxing the constraints: solved %t\n", sketch.Solved())
	sketch.SetRelaxConstraints(false)

	// AllDimsReference: the dimensions measure the sketch without
	// holding it, which is how a drawing is annotated without being
	// driven by its own labels.
	sketch.SetAllDimsReference(true)
	sketch.Entities()[2].MoveTo(api.V(50, 30, 0))
	fmt.Printf("with every dimension a reference one: the corner is at %v\n",
		sketch.Entities()[2].Pos())
	sketch.SetAllDimsReference(false)

	// SuppressDof: the group is left out of the model's count of free
	// parameters, which a large sketch makes quicker to regenerate. The
	// count is not made, and Group.Dof says so with -1.
	plate.SetSuppressDof(true)
	fmt.Printf("left out of the freedom count: %d DOF\n", plate.Dof())
	plate.SetSuppressDof(false)

	// ForceToMesh: the NURBS surfaces are approximated with triangles,
	// which an import that carries no such surface needs.
	plate.SetForceToMesh(true)
	fmt.Printf("forced to a triangle mesh: %t\n", plate.ForceToMesh())

	if sketch.Solved() {
		fmt.Fprintln(os.Stderr, "the sketch still solves with the constraint too many")
	}
}

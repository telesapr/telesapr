// A box with a hinged lid that opens up to 110°, made by a script.
//
// Without a window:
//
//	telesapr-cli run examples/scripts/lid/lid.go [-]
//
// In the application: Script > Run File, or
//
//	telesapr --script examples/scripts/lid/lid.go
//
// An open box 60 x 40 x 25 with walls of 2 and a lid of 3 are saved under
// parts/ and linked: the box is the ground, held where it is; the lid is
// set to move in a mechanism (SetMoves). A driven hinge (HingeDriven)
// puts the lid's back bottom edge on the box's back top edge, the lid
// closed on the box, and makes the parameter angle, the lid's opening in
// degrees; a range (SetRange) keeps it between 0 and 110. A motion study
// of angle from 0 to 150 opens the lid: the steps up to 105 solve, those
// past 110 don't. The script checks that, and that at 90 the lid's front
// top edge stands 40 above the hinge, √(40² + 3²) from it, and saves
// lid.tlspr, unless given "-".
package main

import (
	"fmt"
	"math"
	"os"
	"path/filepath"

	"github.com/telesapr/telesapr/pkg/script/api"
)

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}

// edge is part g's edge along x at y and z of its own, found where its
// link placed it, before anything moves it. The box's and the lid's run
// the same way, both drawn from the same rectangle, from the greatest x
// to the least: a positive angle about that way lifts the lid.
func edge(g api.Group, y, z float64) api.Entity {
	// The parts' own origin is where the link put the box's corner: the
	// least x and y of their points, z as drawn.
	x0, y0, z0 := math.Inf(1), math.Inf(1), 0.0
	for _, e := range g.Entities() {
		if e.Kind() == "point" {
			p := e.Pos()
			x0, y0 = math.Min(x0, p.X), math.Min(y0, p.Y)
		}
	}
	for _, e := range g.Entities() {
		if e.Kind() != "line" {
			continue
		}
		s, f := e.Start().Pos(), e.End().Pos()
		if math.Abs(s.Y-y0-y) < 1e-6 && math.Abs(f.Y-y0-y) < 1e-6 && math.Abs(s.Z-z0-z) < 1e-6 &&
			math.Abs(f.Z-z0-z) < 1e-6 && math.Abs(f.X-s.X) > 1 {
			return e
		}
	}
	fmt.Fprintf(os.Stderr, "%s has no edge along x at y %g, z %g\n", g, y, z)
	os.Exit(2)
	return api.Entity{}
}

func main() {
	write := len(os.Args) < 2 || os.Args[1] != "-"
	must(os.MkdirAll("parts", 0o755))

	b := api.New()
	b.Sketch.Rect(api.V(0, 0, 0), api.V(60, 40, 0))
	box := b.Features.Extrude(25, "box")
	var top api.Entity
	for _, e := range box.Entities() {
		if e.Kind() == "face" && e.FaceNormal().Z > 0.99 && (top.H.V == 0 || e.FacePoint().Z > top.FacePoint().Z) {
			top = e
		}
	}
	b.Features.Shell([]api.Entity{top}, 2, "open")
	must(b.File.Save(filepath.Join("parts", "box.tlspr")))
	// The lid drawn where it lies closed, on the box's rim, so that it
	// starts closed: the hinge then has nothing to move, and its angle
	// 0 is the lid shut.
	l := api.New()
	l.Sketch.Anywhere3D()
	rim := l.Sketch.NewWorkplane(api.V(0, 0, 25), api.V(1, 0, 0), api.V(0, 1, 0))
	l.Features.SketchIn(rim, "outline")
	l.Sketch.Rect(api.V(0, 0, 25), api.V(60, 40, 25))
	l.Features.Extrude(3, "lid")
	must(l.File.Save(filepath.Join("parts", "lid.tlspr")))

	doc := api.Current()
	boxLink, err := doc.Assembly.Link(filepath.Join("parts", "box.tlspr"), "box")
	must(err)
	lid, err := doc.Assembly.Link(filepath.Join("parts", "lid.tlspr"), "lid")
	must(err)
	lid.AsLink().SetMoves(true)

	hinge, back := edge(boxLink, 40, 25), edge(lid, 40, 25)
	front := edge(lid, 0, 28).Start()
	doc.Sketch.Anywhere3D()
	mate := doc.Assembly.HingeDriven(hinge, back)
	mate.SetRange(0, 110)
	fmt.Println("the lid hinged: driven by", mate.Param, "solved", lid.Solved(), "with", lid.Dof(), "degrees of freedom")

	// Steps of 15°, so that none lands on the bound itself, where
	// whether it holds is a matter of the last bits.
	frames, err := doc.Assembly.MotionStudy(mate.Param, 0, 150, 10, "")
	must(err)
	ok := true
	for _, f := range frames {
		fmt.Printf("angle %5.1f: solved %v\n", f.Value, f.Solved)
		ok = ok && f.Solved == (f.Value < 110)
	}
	must(doc.Params.Set(mate.Param, "90"))
	p, h := front.Pos(), hinge.Start().Pos()
	r := math.Hypot(p.Y-h.Y, p.Z-h.Z)
	fmt.Printf("at 90: the front edge %.2f above the hinge, %.4f from it\n", p.Z-h.Z, r)
	switch {
	case !ok:
		fmt.Fprintln(os.Stderr, "the range didn't hold the lid between 0 and 110")
		os.Exit(1)
	case math.Abs(r-40.1123) > 1e-3 || p.Z-h.Z < 39:
		fmt.Fprintf(os.Stderr, "at 90 the front edge is %.4f from the hinge, %.2f above it\n", r, p.Z-h.Z)
		os.Exit(1)
	}
	must(doc.Params.Set(mate.Param, "0"))
	if write {
		must(doc.File.Save("lid.tlspr"))
	}
}

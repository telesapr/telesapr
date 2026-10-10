// Two plates put together by mates, made by a script.
//
// Without a window:
//
//	telesapr-cli run examples/scripts/mates/mates.go [-]
//
// In the application: Script > Run File, or
//
//	telesapr --script examples/scripts/mates/mates.go
//
// A mate is a shortcut to the constraints that put two parts together:
// Concentric puts two circles on one axis, Flush two faces in one
// plane, Hinge one edge on another, free to turn.
// They are made in the active group, which places the second part, as a
// link group does. Here a plate with a hole is saved as parts/plate.tlspr
// and linked twice; the second copy is stacked on the first, its bottom
// flush with the first's top and its hole on the first's. The script
// checks where it ended, and saves mates.tlspr, unless given "-".
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

// ends are the faces of g at its lowest and highest z, an extrusion's
// ends.
func ends(g api.Group) (lo, hi api.Entity) {
	for _, e := range g.Entities() {
		if e.Kind() != "face" || math.Abs(e.FaceNormal().Z) < 0.99 {
			continue
		}
		if lo.H.V == 0 || e.FacePoint().Z < lo.FacePoint().Z {
			lo = e
		}
		if hi.H.V == 0 || e.FacePoint().Z > hi.FacePoint().Z {
			hi = e
		}
	}
	return lo, hi
}

// hole is g's circle at its lowest z, the hole's at the bottom.
func hole(g api.Group) api.Entity {
	var best api.Entity
	for _, e := range g.Entities() {
		if e.Kind() == "circle" && (best.H.V == 0 || e.Center().Pos().Z < best.Center().Pos().Z) {
			best = e
		}
	}
	return best
}

func main() {
	write := len(os.Args) < 2 || os.Args[1] != "-"
	part := api.New()
	part.Sketch.Rect(api.V(0, 0, 0), api.V(40, 20, 0))
	part.Sketch.Circle(api.V(10, 10, 0), 3)
	part.Features.Extrude(5, "")
	must(os.MkdirAll("parts", 0o755))
	path := filepath.Join("parts", "plate.tlspr")
	must(part.File.Save(path))

	doc := api.Current()
	a, err := doc.Assembly.Link(path, "lower")
	must(err)
	b, err := doc.Assembly.Link(path, "upper")
	must(err)

	_, top := ends(a)
	bottom, _ := ends(b)
	doc.Assembly.Flush(top, bottom)
	doc.Assembly.Concentric(hole(a), hole(b))

	ha, hb := hole(a).Center().Pos(), hole(b).Center().Pos()
	_, top = ends(a)
	bottom, _ = ends(b)
	gap := bottom.FacePoint().Minus(top.FacePoint()).Dot(top.FaceNormal())
	fmt.Printf("the holes' centres %.4f apart in the plane, the faces %.4f apart, solved %v\n",
		math.Hypot(ha.X-hb.X, ha.Y-hb.Y), gap, b.Solved())
	if !b.Solved() || math.Abs(gap) > 1e-6 || math.Hypot(ha.X-hb.X, ha.Y-hb.Y) > 1e-6 {
		os.Exit(1)
	}
	if write {
		must(doc.File.Save("mates.tlspr"))
	}
}

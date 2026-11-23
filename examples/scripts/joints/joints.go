// Ball, cylindrical, planar and slot joints between plates, made by a script.
//
// Without a window:
//
//	telesapr-cli run examples/scripts/joints/joints.go [-]
//
// In the application: Script > Run File, or
//
//	telesapr --script examples/scripts/joints/joints.go
//
// Each joint is a mate of its own on the Assembly menu, made of the
// ordinary constraints there are, and each leaves the part it places a
// known freedom: a ball turns every way about a point, a cylindrical
// joint turns about an axis and slides along it, a planar joint slides
// two ways on a face and turns about its normal, and a slot lets a
// point slide along a line while the part turns every way about it.
// Here a plate is saved as parts/plate.tlspr and linked five times: the
// first stays, and each of the others is joined to it by one joint. The
// script prints how each can move, as its group's screen says, checks
// the counts, and saves joints.tlspr, unless given "-".
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

// firstOf is g's first entity of kind.
func firstOf(g api.Group, kind string) api.Entity {
	for _, e := range g.Entities() {
		if e.Kind() == kind {
			return e
		}
	}
	return api.Entity{}
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
	base, err := doc.Assembly.Link(path, "base")
	must(err)
	// Joints are made in 3d.
	doc.Sketch.Anywhere3D()

	joints := []struct {
		name          string
		join          func(g api.Group)
		turns, slides int
	}{
		{"ball", func(g api.Group) { doc.Assembly.Ball(firstOf(base, "point"), firstOf(g, "point")) }, 3, 0},
		{"cylindrical", func(g api.Group) { doc.Assembly.Cylindrical(firstOf(base, "line"), firstOf(g, "line")) }, 1, 1},
		{"planar", func(g api.Group) {
			_, top := ends(base)
			bottom, _ := ends(g)
			doc.Assembly.Planar(top, bottom)
		}, 1, 2},
		{"slot", func(g api.Group) { doc.Assembly.Slot(firstOf(base, "line"), firstOf(g, "point")) }, 3, 1},
	}
	ok := true
	for _, j := range joints {
		g, err := doc.Assembly.Link(path, j.name)
		must(err)
		doc.Sketch.Anywhere3D()
		j.join(g)
		f := g.AsLink().Freedom()
		fmt.Printf("%-12s %s\n", j.name, f.Text)
		if !g.Solved() || f.Turns != j.turns || f.Slides != j.slides || f.Joint != j.name {
			ok = false
		}
	}
	if !ok {
		os.Exit(1)
	}
	if write {
		must(doc.File.Save("joints.tlspr"))
	}
}

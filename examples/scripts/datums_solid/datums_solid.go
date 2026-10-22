// Datums on a solid's faces and edges, which no entity makes, made by a
// script.
//
// Without a window:
//
//	telesapr-cli run examples/scripts/datums_solid/datums_solid.go [-]
//
// In the application: Script > Run File, or
//
//	telesapr --script examples/scripts/datums_solid/datums_solid.go
//
// A datum can name a face or an edge of the solid as well as an entity
// (Edit > Datums, with the face or edge selected): a workplane follows
// it, in a flat face, about the axis of a round face or edge, along a
// straight edge. The script saves a plate whose corner a fillet rounds,
// with the fillet's face named "corner" and the plate's bottom "base",
// as parts/rounded.tlspr where it runs. It links the plate twice, puts
// the second's corner on the first's axis and its base flush with the
// first's, checks that, and saves the assembly as datums_solid.tlspr
// unless given "-".
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

// plate saves a plate of 40 x 30 x 10 whose corner at (40, 30) a fillet
// of 6 rounds, with its datums.
func plate() string {
	p := api.New()
	p.Sketch.Rect(api.V(0, 0, 0), api.V(40, 30, 0))
	b := p.Features.Extrude(10, "plate")
	r := p.Features.FilletSolid([]api.SolidEdge{b.Solid().EdgeAt(api.V(40, 30, 5))}, 6, "fillet")
	s := r.Solid()
	k := 6 * (1 - math.Sqrt(0.5))
	p.Datums.NameFace("corner", s.FaceAt(api.V(40-k, 30-k, 5)))
	p.Datums.NameFace("base", s.FaceAt(api.V(20, 15, 0)))
	path := filepath.Join("parts", "rounded.tlspr")
	must(p.File.Save(path))
	return path
}

// frame is the origin and the unit normal of a datum: a workplane, or
// its normal as a link gives it.
func frame(e api.Entity) (at, n api.Vec) {
	at = e.Center().Pos()
	if e.Kind() == "workplane" {
		e = e.Normal()
	}
	u, v := e.Orientation()
	return at, u.Cross(v).WithMagnitude(1)
}

func main() {
	write := len(os.Args) < 2 || os.Args[1] != "-"
	must(os.MkdirAll("parts", 0o755))
	path := plate()

	doc := api.Current()
	_, err := doc.Assembly.Link(path, "a")
	must(err)
	b, err := doc.Assembly.Link(path, "b")
	must(err)
	b.AsLink().Move(api.V(60, 40, 20))
	fmt.Println("datums:", doc.Datums.Names())

	doc.Assembly.Concentric(doc.Datums.Get("a.corner"), doc.Datums.Get("b.corner"))
	doc.Assembly.Flush(doc.Datums.Get("a.base"), doc.Datums.Get("b.base"))
	ca, na := frame(doc.Datums.Get("a.corner"))
	cb, _ := frame(doc.Datums.Get("b.corner"))
	off := cb.Minus(ca).Cross(na).Magnitude()
	fmt.Printf("the second plate's corner lies %.3g mm off the first's axis\n", off)
	if off > 1e-6 {
		os.Exit(1)
	}
	if write {
		must(doc.File.Save("datums_solid.tlspr"))
	}
}

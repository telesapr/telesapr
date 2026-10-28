// Datums on faces and edges neither flat, round nor straight, made by a
// script.
//
// Without a window:
//
//	telesapr-cli run examples/scripts/datums_freeform/datums_freeform.go [-]
//
// In the application: Script > Run File, or
//
//	telesapr --script examples/scripts/datums_freeform/datums_freeform.go
//
// A datum on a curved face that isn't round about an axis, as a loft's
// or a lens's wall, is the plane touching the face at its middle, its
// normal out of the solid; on a curved edge, the line along the edge at
// its middle. Named with Datums.NameFaceAt or NameEdgeAt, the plane or
// line is taken where the face or edge was picked instead, and follows
// that place as the model changes. The script saves a lens-shaped block,
// its wall named "wall", the same wall picked low down near one end
// "foot" and its top rim "rim", as parts/lens.tlspr where it runs,
// lists the datums' kinds, links the block twice, lays the second's
// wall flush on the first's, checks that, and saves the assembly as
// datums_freeform.tlspr unless given "-". A flush mate of the rims is
// refused: an edge's datum is a line, which lies in no one plane.
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

// lens saves a block 5 high on a lens of two cubics from (0, 0) to
// (20, 0), its wall and rim named.
func lens() string {
	p := api.New()
	p.Features.SketchIn(p.Workplane("XY"), "")
	a := p.Sketch.Cubic(api.V(0, 0, 0), api.V(5, 8, 0), api.V(15, 8, 0), api.V(20, 0, 0))
	b := p.Sketch.Cubic(api.V(20, 0, 0), api.V(15, -8, 0), api.V(5, -8, 0), api.V(0, 0, 0))
	pa, pb := a.Points(), b.Points()
	p.Constrain.Coincident(pa[3], pb[0])
	p.Constrain.Coincident(pb[3], pa[0])
	s := p.Features.Extrude(5, "lens").Solid()
	p.Datums.NameFace("wall", s.FaceAt(api.V(10, 6, 2.5)))
	p.Datums.NameFaceAt("foot", s.FaceAt(api.V(10, 6, 2.5)), api.V(4.53125, 4.5, 1.25))
	p.Datums.NameEdge("rim", s.EdgeAt(api.V(10, 6, 5)))
	for _, d := range p.Datums.List() {
		fmt.Printf("%s: %s\n", d.Name, d.Kind)
	}
	at, _ := frame(p.Datums.Get("foot"))
	fmt.Printf("the foot's plane touches the wall at (%.3g, %.3g, %.3g)\n", at.X, at.Y, at.Z)
	path := filepath.Join("parts", "lens.tlspr")
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
	path := lens()

	doc := api.Current()
	_, err := doc.Assembly.Link(path, "a")
	must(err)
	b, err := doc.Assembly.Link(path, "b")
	must(err)
	b.AsLink().Move(api.V(0, 30, 10))

	doc.Assembly.Flush(doc.Datums.Get("a.wall"), doc.Datums.Get("b.wall"))
	pa, na := frame(doc.Datums.Get("a.wall"))
	pb, nb := frame(doc.Datums.Get("b.wall"))
	off := math.Abs(pb.Minus(pa).Dot(na))
	fmt.Printf("the second wall's plane lies %.3g mm off the first's, at %.3g degrees\n",
		off, math.Acos(math.Min(1, math.Abs(na.Dot(nb))))*180/math.Pi)
	if off > 1e-6 {
		os.Exit(1)
	}
	if write {
		must(doc.File.Save("datums_freeform.tlspr"))
	}
}

// A one-cylinder engine's crank, connecting rod and piston, a
// slider-crank mechanism, made by a script.
//
// Without a window:
//
//	telesapr-cli run examples/scripts/engine/engine.go [-]
//
// In the application: Script > Run File, or
//
//	telesapr --script examples/scripts/engine/engine.go
//
// Four parts are saved under parts/ and linked, each 4 thick, stacked
// up: a frame with the crank's main bearing and a construction line up
// its middle, the cylinder's axis; a crank with its main hole and,
// 12 from it, its crank pin's; a connecting rod of 50 between its holes;
// a piston with its pin's hole and a construction line along it. The
// frame is the ground; the crank, the rod and the piston move in a
// mechanism (SetMoves). Each joint is a pin in its hole: the holes on
// one axis (Concentric, for the crank a driven hinge, HingeDriven,
// which makes the parameter angle) and the part's face flush on the
// one below it (Flush), so that it can only turn. The piston slides up
// the cylinder: its pin's centre on the frame's axis and its own line
// parallel to it, both in the XY workplane, where they hold in
// projection what the stacked parts can't hold in 3d. That leaves the
// mechanism one freedom, which angle takes. A motion study of angle
// round a turn solves at every step; the piston's stroke, read every
// 5°, is twice the crank's throw, 24. The script checks both, and
// saves engine.tlspr, unless given "-".
package main

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"

	"github.com/telesapr/telesapr/pkg/script/api"
)

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}

// part saves a part drawn by draw and extruded 4, as parts/name.tlspr,
// and links it.
func part(doc *api.Doc, name string, draw func(p *api.Doc)) api.Group {
	p := api.New()
	draw(p)
	p.Features.Extrude(4, "")
	path := filepath.Join("parts", name+".tlspr")
	must(p.File.Save(path))
	g, err := doc.Assembly.Link(path, name)
	must(err)
	return g
}

// origin is where the link put part g's own origin: the least x and y
// of its lines, as each part is drawn from its corner.
func origin(g api.Group) api.Vec {
	x0, y0 := math.Inf(1), math.Inf(1)
	for _, e := range g.Entities() {
		if e.Kind() == "line" {
			x0, y0 = math.Min(x0, e.Start().Pos().X), math.Min(y0, e.Start().Pos().Y)
		}
	}
	return api.V(x0, y0, 0)
}

// hole is part g's circle at x, y of its own, in its bottom face.
func hole(g api.Group, x, y float64) api.Entity {
	o := origin(g)
	for _, e := range g.Entities() {
		if e.Kind() != "circle" {
			continue
		}
		c := e.Center().Pos()
		if math.Abs(c.X-o.X-x) < 1e-6 && math.Abs(c.Y-o.Y-y) < 1e-6 && math.Abs(c.Z) < 1e-6 {
			return e
		}
	}
	fmt.Fprintf(os.Stderr, "%s has no hole at %g, %g\n", g, x, y)
	os.Exit(2)
	return api.Entity{}
}

// axisLine is part g's construction line at x of its own, along y.
func axisLine(g api.Group, x float64) api.Entity {
	o := origin(g)
	for _, e := range g.Entities() {
		if e.Kind() != "line" {
			continue
		}
		s, f := e.Start().Pos(), e.End().Pos()
		if math.Abs(s.X-o.X-x) < 1e-6 && math.Abs(f.X-o.X-x) < 1e-6 && math.Abs(s.Z) < 1e-6 &&
			math.Abs(f.Z) < 1e-6 && math.Abs(s.Y-f.Y) > 10 {
			return e
		}
	}
	fmt.Fprintf(os.Stderr, "%s has no line along y at %g\n", g, x)
	os.Exit(2)
	return api.Entity{}
}

// faces are part g's bottom and top faces.
func faces(g api.Group) (bottom, top api.Entity) {
	for _, e := range g.Entities() {
		if e.Kind() != "face" || math.Abs(e.FaceNormal().Z) < 0.99 {
			continue
		}
		if bottom.H.V == 0 || e.FacePoint().Z < bottom.FacePoint().Z {
			bottom = e
		}
		if top.H.V == 0 || e.FacePoint().Z > top.FacePoint().Z {
			top = e
		}
	}
	return bottom, top
}

// turn turns g's part so that its own x points deg round from x.
func turn(g api.Group, deg float64) {
	c, s := math.Cos(deg*math.Pi/180), math.Sin(deg*math.Pi/180)
	g.AsLink().SetOrientation(api.V(c, s, 0), api.V(-s, c, 0))
}

func main() {
	write := len(os.Args) < 2 || os.Args[1] != "-"
	must(os.MkdirAll("parts", 0o755))
	doc := api.Current()
	frame := part(doc, "frame", func(p *api.Doc) {
		p.Sketch.Rect(api.V(0, 0, 0), api.V(30, 120, 0))
		p.Sketch.Circle(api.V(15, 15, 0), 3)
		p.Sketch.Line(api.V(15, 40, 0), api.V(15, 115, 0)).SetConstruction(true)
	})
	crank := part(doc, "crank", func(p *api.Doc) {
		p.Sketch.Rect(api.V(0, 0, 0), api.V(24, 10, 0))
		p.Sketch.Circle(api.V(5, 5, 0), 3)
		p.Sketch.Circle(api.V(17, 5, 0), 3)
	})
	rod := part(doc, "rod", func(p *api.Doc) {
		p.Sketch.Rect(api.V(0, 0, 0), api.V(60, 10, 0))
		p.Sketch.Circle(api.V(5, 5, 0), 3)
		p.Sketch.Circle(api.V(55, 5, 0), 3)
	})
	piston := part(doc, "piston", func(p *api.Doc) {
		p.Sketch.Rect(api.V(0, 0, 0), api.V(16, 20, 0))
		p.Sketch.Circle(api.V(8, 10, 0), 3)
		p.Sketch.Line(api.V(8, 0, 0), api.V(8, 20, 0)).SetConstruction(true)
	})
	for _, g := range []api.Group{crank, rod, piston} {
		g.AsLink().SetMoves(true)
	}

	// What the mates join, found before anything moves.
	bearing, cylinder := hole(frame, 15, 15), axisLine(frame, 15)
	mainHole, crankPin := hole(crank, 5, 5), hole(crank, 17, 5)
	bigEnd, smallEnd := hole(rod, 5, 5), hole(rod, 55, 5)
	pistonPin, pistonAxis := hole(piston, 8, 10), axisLine(piston, 8)
	_, frameTop := faces(frame)
	crankBottom, crankTop := faces(crank)
	rodBottom, rodTop := faces(rod)
	pistonBottom, _ := faces(piston)
	// Roughly where they go: the crank up, the rod up the cylinder.
	turn(crank, 90)
	turn(rod, 80)

	doc.Sketch.Anywhere3D()
	mate := doc.Assembly.HingeDriven(bearing, mainHole)
	doc.Assembly.Flush(frameTop, crankBottom)
	doc.Assembly.Concentric(crankPin, bigEnd)
	doc.Assembly.Flush(crankTop, rodBottom)
	doc.Assembly.Concentric(smallEnd, pistonPin)
	doc.Assembly.Flush(rodTop, pistonBottom)
	doc.Sketch.InWorkplane(doc.Workplane("XY"))
	doc.Constrain.On(pistonPin.Center(), cylinder)
	doc.Constrain.Parallel(pistonAxis, cylinder)
	doc.Sketch.Anywhere3D()
	fmt.Println("the engine:", piston.AsLink().Mechanism(), "driven by", mate.Param, "solved", piston.Solved(),
		"with", piston.Dof(), "degrees of freedom")

	frames, err := doc.Assembly.MotionStudy(mate.Param, 0, 360, 24, "")
	must(err)
	ok := true
	for _, f := range frames {
		ok = ok && f.Solved
	}
	fmt.Printf("a motion study of %d steps round: all solved %v\n", len(frames), ok)

	// The piston's pin's height up the cylinder, from where it is with
	// the crank at 0.
	lo, hi, base := math.Inf(1), math.Inf(-1), pistonPin.Center().Pos().Y
	for deg := 0; deg < 360; deg += 5 {
		must(doc.Params.Set(mate.Param, strconv.Itoa(deg)))
		y := pistonPin.Center().Pos().Y
		lo, hi = math.Min(lo, y), math.Max(hi, y)
		if deg%90 == 0 {
			fmt.Printf("crank at %3d°: the piston's pin %+.2f\n", deg, y-base)
		}
	}
	must(doc.Params.Set(mate.Param, "0"))
	fmt.Printf("stroke %.3f, twice the throw 24\n", hi-lo)
	if !ok || math.Abs(hi-lo-24) > 0.05 {
		os.Exit(1)
	}
	if write {
		must(doc.File.Save("engine.tlspr"))
	}
}

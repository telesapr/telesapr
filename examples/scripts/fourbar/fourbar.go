// A four-bar linkage assembled by its mates from parts that all lie on top of each other, made by a script.
//
// Without a window:
//
//	telesapr-cli run examples/scripts/fourbar/fourbar.go [-]
//
// In the application: Script > Run File, or
//
//	telesapr --script examples/scripts/fourbar/fourbar.go
//
// Four bars, each 10 wide and 4 thick, are saved under parts/ and linked: a
// frame of 60, a crank of 20, a coupler of 50 and a rocker of 40. The frame
// is the ground; the others move in a mechanism (SetMoves). Their pins are
// the upright edges at their ends. No part is put anywhere first: each lies
// where its link put it, the four on top of each other at the origin. The
// hinges alone assemble the linkage, each part placed from the ground out and
// the loop closed by the last: the crank on the frame's first pin on a driven
// hinge (HingeDriven, which makes the parameter angle), the coupler on the
// crank's far pin, the rocker on the coupler's far pin and on the frame's
// second, closing the loop. That last hinge repeats what the others already
// hold, which a mechanism accepts. The script prints that each part is
// solved with no freedom left, where each hinge came to be, and that the bars
// kept their lengths. It prints how each part can move (Freedom): the crank
// and the rocker turn about their bearings, the coupler about an axis that
// moves with the mechanism. A motion study of angle round a turn solves at
// every step, the crank being the shortest bar and the frame the longest, and
// the crank turned by a point (Entity.Turn), as Shift-dragging it turns it,
// writes its turn into angle. It saves fourbar.tlspr and fourbar.glb, the
// crank turning round as a glTF animation, unless given "-".
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

// bar saves a bar of this length, 10 wide and 4 thick, as
// parts/name.tlspr, and links it.
func bar(doc *api.Doc, name string, length float64) api.Group {
	p := api.New()
	p.Sketch.Rect(api.V(0, 0, 0), api.V(length, 10, 0))
	p.Features.Extrude(4, "")
	path := filepath.Join("parts", name+".tlspr")
	must(p.File.Save(path))
	g, err := doc.Assembly.Link(path, name)
	must(err)
	return g
}

// pins are the upright front edges of bar g by their distance along it
// from its origin's, found where its link put it, before anything moves
// it. The bar's start is its least x and y.
func pins(g api.Group) map[float64]api.Entity {
	x0, y0 := math.Inf(1), math.Inf(1)
	for _, e := range g.Entities() {
		if e.Kind() == "line" {
			x0, y0 = math.Min(x0, e.Start().Pos().X), math.Min(y0, e.Start().Pos().Y)
		}
	}
	out := map[float64]api.Entity{}
	for _, e := range g.Entities() {
		if e.Kind() != "line" {
			continue
		}
		s, f := e.Start().Pos(), e.End().Pos()
		if math.Abs(s.X-f.X) < 1e-9 && math.Abs(s.Y-f.Y) < 1e-9 && math.Abs(s.Y-y0) < 1e-9 && s.Z > f.Z {
			out[math.Round(s.X-x0)] = e
		}
	}
	return out
}

func main() {
	write := len(os.Args) < 2 || os.Args[1] != "-"
	must(os.MkdirAll("parts", 0o755))
	doc := api.Current()
	frame := bar(doc, "frame", 60)
	crank, coupler, rocker := bar(doc, "crank", 20), bar(doc, "coupler", 50), bar(doc, "rocker", 40)
	for _, g := range []api.Group{crank, coupler, rocker} {
		g.AsLink().SetMoves(true)
	}

	// What the hinges join, found before anything moves; no part is put
	// anywhere.
	pf, pc, pb, pr := pins(frame), pins(crank), pins(coupler), pins(rocker)

	doc.Sketch.Anywhere3D()
	mate := doc.Assembly.HingeDriven(pf[0], pc[0])
	doc.Assembly.Hinge(pc[20], pb[0])
	doc.Assembly.Hinge(pb[50], pr[0])
	doc.Assembly.Hinge(pf[60], pr[40])
	ok := true
	for _, g := range []api.Group{crank, coupler, rocker} {
		fmt.Printf("the %s: solved %v, %d degrees of freedom\n", g.Name(), g.Solved(), g.Dof())
		ok = ok && g.Solved() && g.Dof() == 0
	}
	fmt.Println("the four-bar:", rocker.AsLink().Mechanism(), "driven by", mate.Param,
		"the last hinge implied:", len(rocker.Implied()) > 0)

	// Where the hinges ended up: each one's two pins on one point, and the
	// bars the lengths they were drawn.
	at := func(e api.Entity) api.Vec { return e.Start().Pos() }
	a, b, c, d := at(pf[0]), at(pc[20]), at(pb[50]), at(pf[60])
	for _, j := range []struct {
		name string
		x, y api.Vec
	}{
		{"A", a, at(pc[0])},
		{"B", b, at(pb[0])},
		{"C", c, at(pr[0])},
		{"D", d, at(pr[40])},
	} {
		gap := j.x.Minus(j.y).Magnitude()
		fmt.Printf("hinge %s at (%.2f, %.2f), its pins %.5f apart\n", j.name, j.x.X, j.x.Y, gap)
		ok = ok && gap < 1e-4
	}
	lengths := []float64{b.Minus(a).Magnitude(), c.Minus(b).Magnitude(), d.Minus(c).Magnitude(), d.Minus(a).Magnitude()}
	fmt.Printf("crank %.3f, coupler %.3f, rocker %.3f, frame %.3f\n", lengths[0], lengths[1], lengths[2], lengths[3])
	for i, want := range []float64{20, 50, 40, 60} {
		ok = ok && math.Abs(lengths[i]-want) < 1e-4
	}

	// How each part can move, counted from its mates at the position they
	// came to.
	for _, g := range []api.Group{crank, coupler, rocker} {
		f := g.AsLink().Freedom()
		fmt.Printf("the %s %s (joint %q, moving %v)\n", g.Name(), f.Text, f.Joint, f.Moving)
	}
	ok = ok && crank.AsLink().Freedom().Joint == "hinge" && rocker.AsLink().Freedom().Joint == "hinge" &&
		coupler.AsLink().Freedom().Moving
	if !ok {
		fmt.Fprintln(os.Stderr, "the four-bar didn't assemble")
		os.Exit(1)
	}

	frames, err := doc.Assembly.MotionStudy(mate.Param, 0, 360, 24, "")
	must(err)
	solved := true
	for _, f := range frames {
		solved = solved && f.Solved
	}
	fmt.Printf("a motion study of %d steps round: all solved %v\n", len(frames), solved)

	// The rocker's swing, read every 10° of the crank.
	lo, hi := math.Inf(1), math.Inf(-1)
	for deg := 0; deg < 360; deg += 10 {
		must(doc.Params.Set(mate.Param, strconv.Itoa(deg)))
		r := at(pr[0]).Minus(at(pr[40]))
		swing := math.Atan2(r.Y, r.X) * 180 / math.Pi
		lo, hi = math.Min(lo, swing), math.Max(hi, swing)
	}
	fmt.Printf("the rocker swings %.1f° (from %.1f° to %.1f°)\n", hi-lo, lo, hi)
	must(doc.Params.Set(mate.Param, "0"))

	// Turned by a point of the crank, as Shift-dragging it turns it, the
	// crank turns its drive with it. The pins run down, so a positive
	// angle is a turn about -z.
	pc[20].Start().Turn(api.V(0, 0, -1), 15)
	got, _ := doc.Params.Get(mate.Param)
	fmt.Printf("the crank turned 15°: %s = %.1f\n", mate.Param, got.Value)
	if !solved || hi-lo < 10 || math.Abs(got.Value-15) > 0.5 {
		fmt.Fprintln(os.Stderr, "the four-bar didn't move as it should")
		os.Exit(1)
	}
	must(doc.Params.Set(mate.Param, "0"))
	if write {
		must(doc.File.Save("fourbar.tlspr"))
		_, err := doc.Assembly.MotionStudy(mate.Param, 0, 360, 36, "fourbar.glb")
		must(err)
	}
}

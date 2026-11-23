// A box with two flaps, front and back, that a motion study opens together, made by a script.
//
// Without a window:
//
//	telesapr-cli run examples/scripts/flaps/flaps.go [-]
//
// In the application: Script > Run File, or
//
//	telesapr --script examples/scripts/flaps/flaps.go
//
// An open box 60 x 40 x 25 with walls of 2 is saved under parts/, and two
// flaps of 60 x 20 x 3, one over the box's front half and one over its
// back half; all are linked. The box is the ground; the flaps move in a
// mechanism (SetMoves). Each flap is on a driven hinge (HingeDriven) of
// its own: the back flap's makes the parameter angle, the front flap's
// angle2, each the flap's opening in degrees. A motion study that drives
// both (MotionStudyOf), angle from 0 to 90 and angle2 from 0 to 60 in six
// steps, opens the back flap to a right angle and the front one to 60°
// while it reads each step's values. The script prints each flap's
// freedom, a hinge, checks that every step solves, that at the last
// step the flaps' free edges stand 20 and 20 sin 60° above the box's rim,
// and that each parameter gets its expression back. It saves flaps.tlspr
// and flaps.glb, both flaps opening as one glTF animation, unless given "-".
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

// corner is where the link put the box's least x and y; every linked
// part of the box's file is put there, so that the parts' own
// coordinates are the points' less this.
func corner(g api.Group) api.Vec {
	x0, y0 := math.Inf(1), math.Inf(1)
	for _, e := range g.Entities() {
		if e.Kind() == "point" {
			p := e.Pos()
			x0, y0 = math.Min(x0, p.X), math.Min(y0, p.Y)
		}
	}
	return api.V(x0, y0, 0)
}

// rimZ is the height of the box's rim, where the flaps' hinges lie.
const rimZ = 25.0

// edge is part g's line along x at the rim's height (rimZ) and at y of the
// file's own, found where the link put it, o being the link's corner. It
// stops the script if there is none.
func edge(g api.Group, o api.Vec, y float64) api.Entity {
	z := rimZ
	for _, e := range g.Entities() {
		if e.Kind() != "line" {
			continue
		}
		s, f := e.Start().Pos(), e.End().Pos()
		if math.Abs(s.Y-o.Y-y) < 1e-6 && math.Abs(f.Y-o.Y-y) < 1e-6 && math.Abs(s.Z-z) < 1e-6 &&
			math.Abs(f.Z-z) < 1e-6 && math.Abs(f.X-s.X) > 1 {
			return e
		}
	}
	fmt.Fprintf(os.Stderr, "%s has no edge along x at y %g, z %g\n", g, y, z)
	os.Exit(2)
	return api.Entity{}
}

// flap saves a flap over the box's rim between y0 and y1, extruded 3,
// as parts/name.tlspr, and links it.
func flap(doc *api.Doc, name string, y0, y1 float64) api.Group {
	p := api.New()
	p.Sketch.Anywhere3D()
	rim := p.Sketch.NewWorkplane(api.V(0, 0, 25), api.V(1, 0, 0), api.V(0, 1, 0))
	p.Features.SketchIn(rim, "outline")
	p.Sketch.Rect(api.V(0, y0, 25), api.V(60, y1, 25))
	p.Features.Extrude(3, name)
	path := filepath.Join("parts", name+".tlspr")
	must(p.File.Save(path))
	g, err := doc.Assembly.Link(path, name)
	must(err)
	g.AsLink().SetMoves(true)
	return g
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

	doc := api.Current()
	boxLink, err := doc.Assembly.Link(filepath.Join("parts", "box.tlspr"), "box")
	must(err)
	back := flap(doc, "back", 20, 40)
	front := flap(doc, "front", 0, 20)

	// Found before anything moves: the flaps' hinge edges run along the
	// box's rim, the back one drawn from the greatest x to the least, as
	// the box's, the front one the other way, so that a positive angle
	// lifts either flap.
	o := corner(boxLink)
	rearHinge, frontHinge := edge(boxLink, o, 40), edge(boxLink, o, 0)
	rearEdge, frontEdge := edge(back, o, 40), edge(front, o, 0)
	rearFree, frontFree := edge(back, o, 20).Start(), edge(front, o, 20).Start()

	doc.Sketch.Anywhere3D()
	rear := doc.Assembly.HingeDriven(rearHinge, rearEdge)
	frontMate := doc.Assembly.HingeDriven(frontHinge, frontEdge)
	fmt.Println("the flaps hinged: driven by", rear.Param, "and", frontMate.Param)
	ok := true
	for _, g := range []api.Group{back, front} {
		f := g.AsLink().Freedom()
		fmt.Printf("the %s flap %s (%s) solved %v\n", g.Name(), f.Joint, f.Text, g.Solved())
		ok = ok && g.Solved() && f.Joint == "hinge" && len(f.DrivenBy) == 1
	}

	// Both parameters over the same six steps: 15° a step for the back
	// flap, 10° for the front.
	frames, err := doc.Assembly.MotionStudyOf([]api.Drive{
		{Param: rear.Param, From: 0, To: 90},
		{Param: frontMate.Param, From: 0, To: 60},
	}, 6, "")
	must(err)
	for _, f := range frames {
		fmt.Printf("%s %5.1f, %s %5.1f: solved %v\n", rear.Param, f.Values[0], frontMate.Param, f.Values[1], f.Solved)
		ok = ok && f.Solved
	}
	// Each parameter has its expression back, 0 as it was.
	for _, name := range []string{rear.Param, frontMate.Param} {
		p, _ := doc.Params.Get(name)
		fmt.Printf("%s after the study: %s\n", name, p.Expr)
		ok = ok && p.Expr == "0"
	}
	last := frames[len(frames)-1]
	ok = ok && len(frames) == 7 && last.Values[0] == 90 && last.Values[1] == 60
	// Each flap's free edge above the rim at the last step.
	must(doc.Params.Set(rear.Param, "90"))
	must(doc.Params.Set(frontMate.Param, "60"))
	rise, frontRise := rearFree.Pos().Z-25, frontFree.Pos().Z-25
	fmt.Printf("at the end: the back flap's free edge %.2f above the rim, the front one's %.2f (20 and %.2f)\n",
		rise, frontRise, 20*math.Sin(math.Pi/3))
	ok = ok && math.Abs(rise-20) < 1e-3 && math.Abs(frontRise-20*math.Sin(math.Pi/3)) < 1e-3
	must(doc.Params.Set(rear.Param, "0"))
	must(doc.Params.Set(frontMate.Param, "0"))
	if !ok {
		fmt.Fprintln(os.Stderr, "the flaps didn't open as they should")
		os.Exit(1)
	}
	if write {
		must(doc.File.Save("flaps.tlspr"))
		// Both flaps opening, as an animation a glTF viewer plays.
		_, err := doc.Assembly.MotionStudyOf([]api.Drive{
			{Param: rear.Param, From: 0, To: 90},
			{Param: frontMate.Param, From: 0, To: 60},
		}, 18, "flaps.glb")
		must(err)
	}
}

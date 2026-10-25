package topo_test

import (
	"errors"
	"fmt"
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/check"
	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/internal/sweep"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

// The 10 x 20 x 30 box open at the top, walls 1 thick: the inside is
// 8 x 18 x 29, and the top a rim round a hole.
func TestShellBoxOpenTop(t *testing.T) {
	t.Parallel()
	sh := box(srf.Tolerance{})
	tp := build(t, sh)
	top := faceAt(t, tp, v(0, 0, 30), v(0, 0, 1))
	out, err := topo.ShellSolid(sh, []int{top}, 1, 1<<30)
	if err != nil {
		t.Fatal(err)
	}
	ot := build(t, out)
	solid(t, out, ot, 6000-8*18*29)
	if len(ot.Faces) != 11 {
		t.Errorf("%d faces, want 11: 6 outside, the top a rim, and 5 inside", len(ot.Faces))
	}
}

// insetArea is the area of a polygon (counter-clockwise, in xy) whose
// edge i is moved in by d[i]: consecutive moved edges meet at its new
// corners, as long as none turns round.
func insetArea(pts []geom.Vector, d []float64) float64 {
	n := len(pts)
	type line struct{ p, dir geom.Vector }
	lines := make([]line, n)
	for i := range pts {
		a, b := pts[i], pts[(i+1)%n]
		dir := b.Minus(a).WithMagnitude(1)
		in := v(-dir.Y, dir.X, 0)
		lines[i] = line{a.Plus(in.ScaledBy(d[i])), dir}
	}
	corners := make([]geom.Vector, n)
	for i := range lines {
		l0, l1 := lines[(i+n-1)%n], lines[i]
		// l0.p + s l0.dir on l1.
		den := l0.dir.X*l1.dir.Y - l0.dir.Y*l1.dir.X
		w := l1.p.Minus(l0.p)
		s := (w.X*l1.dir.Y - w.Y*l1.dir.X) / den
		corners[i] = l0.p.Plus(l0.dir.ScaledBy(s))
	}
	area := 0.0
	for i := range corners {
		p, q := corners[i], corners[(i+1)%n]
		area += (p.X*q.Y - q.X*p.Y) / 2
	}
	return area
}

func regular(n int, r float64) []geom.Vector {
	var pts []geom.Vector
	for k := range n {
		a := 2 * math.Pi * float64(k) / float64(n)
		pts = append(pts, v(r*math.Cos(a), r*math.Sin(a), 0))
	}
	return pts
}

// lShape is an L of arms 10 and 6 wide, 20 long.
func lShape() []geom.Vector {
	return []geom.Vector{v(0, 0, 0), v(20, 0, 0), v(20, 6, 0), v(10, 6, 0), v(10, 20, 0), v(0, 20, 0)}
}

// shellPrism hollows the prism of pts, h high, with walls thick, opening
// its top, its bottom and its side from pts[side] (none for -1), and
// checks the result against the volume from insetArea.
func shellPrism(pts []geom.Vector, h, thick float64, top, bottom bool, side int) error {
	var sh srf.SShell
	sh.MakePrism(pts, v(0, 0, 0), v(0, 0, h), core.NewRGBA(100, 100, 100, 255))
	tp, err := topo.Build(&sh)
	if err != nil {
		return err
	}
	var open []int
	d := make([]float64, len(pts))
	for i := range d {
		d[i] = thick
	}
	z0, z1 := thick, h-thick
	for f := range tp.Faces {
		n, p := tp.Surface(f).NormalAt(0.5, 0.5).WithMagnitude(1), tp.Surface(f).PointAt(0.5, 0.5)
		switch {
		case top && n.Z > 0.5 && math.Abs(p.Z-h) < 1e-9:
			open = append(open, f)
			z1 = h
		case bottom && n.Z < -0.5:
			open = append(open, f)
			z0 = 0
		case side >= 0 && math.Abs(n.Z) < 1e-9:
			a, b := pts[side], pts[(side+1)%len(pts)]
			if math.Abs(p.Minus(a).Dot(n)) < 1e-9 && math.Abs(b.Minus(a).Dot(n)) < 1e-9 {
				open = append(open, f)
				d[side] = 0
			}
		}
	}
	out, err := topo.ShellSolid(&sh, open, thick, 1<<30)
	if err != nil {
		return err
	}
	ot, err := topo.Build(out)
	if err != nil {
		return err
	}
	if probs := ot.Validate(); len(probs) > 0 {
		return fmt.Errorf("topology: %s", probs[0].Msg)
	}
	var m srf.SMesh
	out.TriangulateInto(&m)
	r := check.Solid(out, &m)
	area := insetArea(pts, make([]float64, len(pts)))
	want := area*h - insetArea(pts, d)*(z1-z0)
	switch {
	case !r.Sound():
		return fmt.Errorf("unsound: %s", r.String())
	case math.Abs(r.Volume-want) > 1e-6*want:
		return fmt.Errorf("volume %g, want %g", r.Volume, want)
	case ot.Euler() != topo.MeshEuler(&m):
		return fmt.Errorf("Euler characteristic %d, the mesh's %d", ot.Euler(), topo.MeshEuler(&m))
	}
	return nil
}

func TestShellPrisms(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		pts         []geom.Vector
		top, bottom bool
		side        int
	}{
		{"square open top", regular(4, 10), true, false, -1},
		{"hexagon tube", regular(6, 10), true, true, -1},
		{"triangle open side", regular(3, 10), false, false, 0},
		{"square open top and side", regular(4, 10), true, false, 1},
		{"L open top", lShape(), true, false, -1},
		{"L open inner side", lShape(), false, false, 2},
	}
	for _, c := range cases {
		if err := shellPrism(c.pts, 15, 1, c.top, c.bottom, c.side); err != nil {
			t.Errorf("%s: %v", c.name, err)
		}
	}
}

func TestShellRefusals(t *testing.T) {
	t.Parallel()
	sh := box(srf.Tolerance{})
	tp := build(t, sh)
	top := faceAt(t, tp, v(0, 0, 30), v(0, 0, 1))
	var se *topo.ShellError
	// Walls of 5 close up the 10 wide box.
	if _, err := topo.ShellSolid(sh, []int{top}, 5, 1<<30); !errors.As(err, &se) {
		t.Errorf("walls meeting: %v", err)
	}
	if _, err := topo.ShellSolid(sh, nil, 1, 1<<30); err == nil {
		t.Error("no face taken away")
	}
	if _, err := topo.ShellSolid(sh, []int{top}, -1, 1<<30); err == nil {
		t.Error("a negative thickness")
	}
	// Four faces meet at the apex of a pyramid over a rectangle, off
	// centre, and moved in don't meet in one point.
	base := []geom.Vector{v(0, 0, 0), v(0, 10, 0), v(20, 10, 0), v(20, 0, 0)}
	pyr, faces, err := pyramid(base, v(5, 3, 10), srf.Tolerance{})
	if err != nil {
		t.Fatal(err)
	}
	var pm srf.SMesh
	pyr.TriangulateInto(&pm)
	if pr := check.Solid(pyr, &pm); !pr.Sound() || math.Abs(pr.Volume-2000.0/3) > 1e-9 {
		t.Fatalf("the pyramid: %s", pr.String())
	}
	pt := build(t, pyr)
	got, err := topo.ShellSolid(pyr, []int{faceAt(t, pt, v(0, 0, 0), v(0, 0, 1))}, 0.5, 1<<30)
	if err != nil {
		t.Fatalf("four faces at a vertex: %v", err)
	}
	// The apex is split by a new edge inside (roadmap 15.3): a sound
	// shell, its walls about their area times the thickness.
	var m srf.SMesh
	got.TriangulateInto(&m)
	r := check.Solid(got, &m)
	gt := build(t, got)
	if !r.Sound() || len(gt.Validate()) != 0 || r.Volume <= 0 || r.Volume >= 2000.0/3 {
		t.Errorf("the pyramid's shell: %s", r.String())
	}
	area := 0.0
	for _, f := range faces[1:] {
		area += f[1].Minus(f[0]).Cross(f[2].Minus(f[0])).Magnitude() / 2
	}
	if r.Volume < 0.4*area || r.Volume > 0.5*area {
		t.Errorf("walls of %g mm³ for %g mm² of sides 0.5 thick", r.Volume, area)
	}
}

// pyramid is the pyramid over base, a loop in z = 0 running clockwise
// seen from above, to apex, built at tol, and its faces' corners, the
// base first.
func pyramid(base []geom.Vector, apex geom.Vector, tol srf.Tolerance) (*srf.SShell, [][]geom.Vector, error) {
	pyr := &srf.SShell{Tol: tol}
	faces := [][]geom.Vector{base}
	for i := range base {
		faces = append(faces, []geom.Vector{base[(i+1)%len(base)], base[i], apex})
	}
	for _, f := range faces {
		var l []srf.SBezier
		for i := range f {
			l = append(l, srf.SBezierFrom2(f[i], f[(i+1)%len(f)]))
		}
		// A plane over the face: a parallelogram on two of its sides,
		// tripled about its first corner.
		s := srf.FromPlane(f[0].Minus(f[1].Minus(f[0])).Minus(f[len(f)-1].Minus(f[0])),
			f[len(f)-1].Minus(f[0]).ScaledBy(3), f[1].Minus(f[0]).ScaledBy(3))
		if _, err := topo.AddFace(pyr, s, [][]srf.SBezier{l}); err != nil {
			return nil, nil, err
		}
	}
	// AddFace makes the faces face in; turned out, the pyramid is sound.
	for i := range pyr.Surface {
		pyr.Surface[i].Reverse()
	}
	return pyr, faces, nil
}

// Pyramids over a rectangle with their apex here and there, hollowed
// open at the base: four faces meet at the apex, in one point once
// moved in only when the apex is over the middle (roadmap 15.3). The
// walls are sound, and hold about their area times the thickness, less
// what the faces' corners take.
func TestShellApexSweep(t *testing.T) {
	t.Parallel()
	grid := sweep.Grid{
		Axes: []sweep.Axis{
			sweep.Floats("x", 2, 10, 15, 25),
			sweep.Floats("y", 1, 5, 8),
			sweep.Floats("height", 3, 10, 30),
			sweep.Floats("k", 0.02, 0.05, 0.1),
		},
		MaxFailRate: 0,
		Sample:      30,
		Parallel:    true,
	}
	grid.Run(func(p sweep.Point) error {
		base := []geom.Vector{v(0, 0, 0), v(0, 10, 0), v(20, 10, 0), v(20, 0, 0)}
		h := p.Float("height")
		thick := p.Float("k") * math.Min(h, 5)
		pyr, faces, err := pyramid(base, v(p.Float("x"), p.Float("y"), h), srf.Tolerance{})
		if err != nil {
			return err
		}
		pt, err := topo.Build(pyr)
		if err != nil {
			return err
		}
		bottom := -1
		for f := range pt.Faces {
			if n := pyr.Surface[f].NormalAt(0.5, 0.5); n.Z < 0 && math.Abs(n.X)+math.Abs(n.Y) < 1e-9 {
				bottom = f
			}
		}
		out, err := topo.ShellSolid(pyr, []int{bottom}, thick, 1<<30)
		if err != nil {
			return err
		}
		var m srf.SMesh
		out.TriangulateInto(&m)
		r := check.Solid(out, &m)
		ot, err := topo.Build(out)
		if err != nil {
			return err
		}
		if !r.Sound() || len(ot.Validate()) != 0 {
			return fmt.Errorf("unsound: %s", r.String())
		}
		area := 0.0
		for _, f := range faces[1:] {
			area += f[1].Minus(f[0]).Cross(f[2].Minus(f[0])).Magnitude() / 2
		}
		if r.Volume < 0.5*area*thick || r.Volume > area*thick {
			return fmt.Errorf("walls of %g mm³ for %g mm² of sides %g thick", r.Volume, area, thick)
		}
		return nil
	}).Require(t)
}

// A block with a square hole through it, a Boolean's solid, open at the
// top: its top and bottom have holes, and the hole's walls are walls of
// the shell too.
func TestShellHoledBlock(t *testing.T) {
	t.Parallel()
	var outer, hole, sh srf.SShell
	c := core.NewRGBA(100, 100, 100, 255)
	outer.MakePrism([]geom.Vector{v(0, 0, 0), v(30, 0, 0), v(30, 30, 0), v(0, 30, 0)}, v(0, 0, 0), v(0, 0, 10), c)
	hole.MakePrism([]geom.Vector{v(10, 10, -1), v(20, 10, -1), v(20, 20, -1), v(10, 20, -1)}, v(0, 0, 0), v(0, 0, 12), c)
	sh.MakeFromDifferenceOf(&outer, &hole)
	tp := build(t, &sh)
	out, err := topo.ShellSolid(&sh, []int{faceAt(t, tp, v(0, 0, 10), v(0, 0, 1))}, 1, 1<<30)
	if err != nil {
		t.Fatal(err)
	}
	// The inside is 28 x 28 less the hole's 12 x 12, 9 high.
	solid(t, out, build(t, out), 8000-(28*28-12*12)*9)
}

// Prisms of polygons and an L, each opened at the top, both ends, a
// side, or the top and a side, over thickness and height, against
// insetArea; the rate allowed is 0, set before the phase began.
func TestShellSweep(t *testing.T) {
	t.Parallel()
	shapes := map[string][]geom.Vector{
		"triangle": regular(3, 10), "square": regular(4, 10), "hexagon": regular(6, 10),
		"octagon": regular(8, 10), "L": lShape(),
	}
	grid := sweep.Grid{
		Axes: []sweep.Axis{
			sweep.Strings("shape", "triangle", "square", "hexagon", "octagon", "L"),
			sweep.Strings("open", "top", "both ends", "side", "top and side"),
			sweep.Floats("k", 0.05, 0.2, 0.4),
			sweep.Floats("height", 3, 15, 60),
		},
		MaxFailRate: 0,
		Sample:      30,
		Parallel:    true,
	}
	grid.Run(func(p sweep.Point) error {
		pts := shapes[p.String("shape")]
		h := p.Float("height")
		// The thickness as a part of the narrowest the shape gets: the
		// triangle's inradius is 5, the L's arms are 6 wide.
		thick := p.Float("k") * math.Min(5, h)
		open := p.String("open")
		side := -1
		if open == "side" || open == "top and side" {
			side = 1
		}
		return shellPrism(pts, h, thick, open == "top" || open == "both ends" || open == "top and side",
			open == "both ends", side)
	}).Require(t)
}

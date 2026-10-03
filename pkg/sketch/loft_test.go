package sketch_test

import (
	"fmt"
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/internal/sweep"
	"github.com/telesapr/telesapr/pkg/script/api"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

// twoProfiles makes a square of side a in the XY plane and, in a
// workplane h above, a square of side b, and returns their groups.
func twoProfiles(d *api.Doc, a, b, h float64) (api.Group, api.Group) {
	d.With(func(sk *sketch.Sketch) { sk.ChordTolPct = 0.001 })
	bottom := d.SketchIn(d.Workplane("XY"), "bottom")
	d.Rect(api.V(-a/2, -a/2, 0), api.V(a/2, a/2, 0))
	top := d.SketchAt(above(d, h), api.V(1, 0, 0), api.V(0, 1, 0), "top")
	d.Rect(api.V(-b/2, -b/2, h), api.V(b/2, b/2, h))
	return bottom, top
}

// above is a point at height h on the z axis, in 3d, for a workplane's
// origin.
func above(d *api.Doc, h float64) api.Entity {
	d.Sketch3D("")
	return d.Point(api.V(0, 0, h))
}

func TestLoftFrustum(t *testing.T) {
	d := api.New()
	bottom, top := twoProfiles(d, 10, 4, 6)
	step(d)
	before := saved(t, d)
	l := d.Loft(bottom, top, "")
	if p := l.Problem(); p != "" {
		t.Fatal(p)
	}
	r := l.Check()
	want := 6.0 / 3 * (100 + 16 + 40)
	if !r.Sound() || math.Abs(r.Volume-want) > 1e-9*want || l.Kind() != "loft" {
		t.Fatalf("frustum: %s, want %g", r.String(), want)
	}
	undoRedo(t, d, before)
	// Turned by one, the square twists a quarter turn: its faces are
	// saddles, and its volume, h (A0 + A1) / 3 with no mixed term
	// (twistedVolume), is the mesh's within its error.
	l.SetTurn(1)
	quarter := twistedVolume(6, 100, 0.4, math.Pi/2)
	if r := l.Check(); !r.Sound() || l.Turn() != 1 || math.Abs(r.Volume-quarter) > 0.01*quarter {
		t.Fatalf("a quarter turn: %s, want %g", r.String(), quarter)
	}
	l.SetTurn(4)
	if r := l.Check(); !r.Sound() || math.Abs(r.Volume-want) > 1e-6 {
		t.Fatalf("a whole turn: %s", r.String())
	}
	back := roundTrip(t, d)
	if bl := back.Group(l.Name()); bl.Kind() != "loft" || bl.Turn() != 4 {
		t.Fatalf("reopened: %s turn %d", bl.Kind(), bl.Turn())
	}
	if s := l.Solid(); len(s.Problems()) != 0 || len(s.Faces()) != 6 {
		t.Fatalf("topology: %v, %d faces", s.Problems(), len(s.Faces()))
	}
}

func TestLoftCircleToSquare(t *testing.T) {
	d := api.New()
	d.With(func(sk *sketch.Sketch) { sk.ChordTolPct = 0.001 })
	bottom := d.SketchIn(d.Workplane("XY"), "circle")
	d.Circle(api.V(0, 0, 0), 5)
	top := d.SketchAt(above(d, 10), api.V(1, 0, 0), api.V(0, 1, 0), "square")
	d.Rect(api.V(-3, -3, 10), api.V(3, 3, 10))
	l := d.Loft(bottom, top, "")
	r := l.Check()
	// Between the solids lofting the circle's inscribed square and its
	// circumscribed one to the square.
	inner := 10.0 / 3 * (50 + 36 + math.Sqrt(50*36))
	outer := 10.0 / 3 * (100 + 36 + 60)
	if l.Problem() != "" || !r.Sound() || r.Volume < inner || r.Volume > outer {
		t.Fatalf("circle to square: %s (%s), want between %g and %g", r.String(), l.Problem(), inner, outer)
	}
}

// twistedVolume is the volume of the ruled loft, h high, from a loop of
// area a0 to the same loop scaled by c and turned by theta, each point
// joined to its image. A section at t of the way up has area
// (1-t)² a0 + t² a1 + t(1-t) K, K = 2 c cos(theta) a0 (the mixed area of
// the two loops), so the volume is h (a0/3 + a1/3 + K/6).
func twistedVolume(h, a0, c, theta float64) float64 {
	a1 := c * c * a0
	return h * (a0/3 + a1/3 + 2*c*math.Cos(theta)*a0/6)
}

// A square to a square turned by an eighth, a diamond: the four ways of
// pairing their sides twist by an eighth of a turn, one way and the
// other, and by three eighths, and all are sound.
func TestLoftTwist(t *testing.T) {
	d := api.New()
	d.With(func(sk *sketch.Sketch) { sk.ChordTolPct = 0.001 })
	bottom := d.SketchIn(d.Workplane("XY"), "square")
	d.Rect(api.V(-5, -5, 0), api.V(5, 5, 0))
	top := d.SketchAt(above(d, 10), api.V(1, 0, 0), api.V(0, 1, 0), "diamond")
	d.Polyline(true, api.V(5, 0, 10), api.V(0, 5, 10), api.V(-5, 0, 10), api.V(0, -5, 10))
	l := d.Loft(bottom, top, "")
	c := 1 / math.Sqrt2
	eighth, threeEighths := twistedVolume(10, 100, c, math.Pi/4), twistedVolume(10, 100, c, 3*math.Pi/4)
	near := map[float64]int{}
	for k := 0; k < 4; k++ {
		l.SetTurn(k)
		r := l.Check()
		for _, want := range []float64{eighth, threeEighths} {
			if r.Sound() && math.Abs(r.Volume-want) < 0.01*want {
				near[want]++
			}
		}
	}
	if near[eighth] != 2 || near[threeEighths] != 2 {
		t.Errorf("of the four turns, %d are sound near %g and %d near %g; want 2 and 2",
			near[eighth], eighth, near[threeEighths], threeEighths)
	}
}

// ngon draws a regular polygon of n sides about the z axis at height z,
// its corners r out, the first on the x axis; n of 0 draws a circle.
func ngon(d *api.Doc, n int, r, z float64) {
	if n == 0 {
		d.Circle(api.V(0, 0, z), r)
		return
	}
	var pts []api.Vec
	for k := 0; k < n; k++ {
		a := 2 * math.Pi * float64(k) / float64(n)
		pts = append(pts, api.V(r*math.Cos(a), r*math.Sin(a), z))
	}
	d.Polyline(true, pts...)
}

// Lofts between regular polygons, and circles, scaled and turned by a
// curve either way, against twistedVolume: exactly when the sides are
// flat, within the mesh's error when they are saddles or curved.
// Triangles turned by a curve twist by a third of a turn. Since roadmap
// 13.1 the mesher's grid follows a twist to 32 rows, and they are
// sound; but their volume is allowed 2%, not 1%, since the grid has
// only four columns along a side's straight edges, and on a side twisted
// that far the diagonals of its quads bias the volume (+1.2% and -0.7%
// for the two turns of the largest, which are mirror images).
func TestLoftSweep(t *testing.T) {
	loft := func(p sweep.Point) error {
		n, c, h, turn := int(p.Float("sides")), p.Float("scale"), p.Float("height"), int(p.Float("turn"))
		curves := n
		if n == 0 {
			curves = 4
		}
		d := api.New()
		d.With(func(sk *sketch.Sketch) { sk.ChordTolPct = 0.001 })
		bottom := d.SketchIn(d.Workplane("XY"), "")
		ngon(d, n, 10, 0)
		top := d.SketchAt(above(d, h), api.V(1, 0, 0), api.V(0, 1, 0), "")
		ngon(d, n, 10*c, h)
		l := d.Loft(bottom, top, "")
		l.SetTurn((turn + curves) % curves)
		rep := l.Check()
		a0 := math.Pi * 100
		if n > 0 {
			a0 = float64(n) / 2 * 100 * math.Sin(2*math.Pi/float64(n))
		}
		want := twistedVolume(h, a0, c, 2*math.Pi*float64(turn)/float64(curves))
		allow := 1e-9 * want
		if n == 0 || turn != 0 {
			allow = 0.01 * want
		}
		if n == 3 && turn != 0 {
			allow = 0.02 * want
		}
		switch {
		case l.Problem() != "":
			return fmt.Errorf("problem: %s", l.Problem())
		case !rep.Sound():
			return fmt.Errorf("unsound: %s", rep.String())
		case math.Abs(rep.Volume-want) > allow:
			return fmt.Errorf("volume %g, want %g", rep.Volume, want)
		}
		return nil
	}
	scales, heights := sweep.Floats("scale", 0.3, 1, 1.6), sweep.Floats("height", 1, 10, 50)
	sweep.Grid{
		Axes:        []sweep.Axis{sweep.Floats("sides", 0, 4, 6), scales, heights, sweep.Floats("turn", -1, 0, 1)},
		MaxFailRate: 0,
		Sample:      20,
	}.Run(loft).Require(t)
	sweep.Grid{
		Axes:        []sweep.Axis{sweep.Floats("sides", 3), scales, heights, sweep.Floats("turn", -1, 0, 1)},
		MaxFailRate: 0,
		Sample:      5,
	}.Run(loft).Require(t)
}

// A loft joined to a block below it: from a profile on the block's top,
// inside it, on its bottom, below it and above it; flat sides, saddles
// and a circle's cones. Each Boolean finds the loft's faces where they
// cross or lie on the block's, which it does by the boxes about the
// faces' control points, so the loft's planes must cover their faces;
// and it takes the loft's edges for the shell's own, not for curves of
// a Boolean. Where the loft sits on the block, its volume adds to the
// block's.
func TestLoftOnBlock(t *testing.T) {
	for _, z0 := range []float64{5, 3, 0, -2, 7} {
		for _, kind := range []string{"flat", "twisted", "circle"} {
			d := api.New()
			d.With(func(sk *sketch.Sketch) { sk.ChordTolPct = 0.01 })
			block(d, -10, -10, 10, 10, 5)
			bottom := d.SketchAt(above(d, z0), api.V(1, 0, 0), api.V(0, 1, 0), "")
			if kind == "circle" {
				d.Circle(api.V(0, 0, z0), 8)
			} else {
				d.Rect(api.V(-8, -8, z0), api.V(8, 8, z0))
			}
			top := d.SketchAt(above(d, 15), api.V(1, 0, 0), api.V(0, 1, 0), "")
			d.Rect(api.V(-4, -4, 15), api.V(4, 4, 15))
			l := d.Loft(bottom, top, "")
			turn := 0.0
			if kind == "twisted" {
				l.SetTurn(1)
				turn = math.Pi / 2
			}
			r := l.Check()
			if !r.Sound() || l.Problem() != "" {
				t.Errorf("%s from z %g: %s %s", kind, z0, r.String(), l.Problem())
				continue
			}
			if z0 == 5 && kind != "circle" {
				want := 2000 + twistedVolume(10, 256, 0.5, turn)
				if math.Abs(r.Volume-want) > 0.005*want {
					t.Errorf("%s on the block: volume %g, want %g", kind, r.Volume, want)
				}
			}
		}
	}
}

// A loft through three circles about one axis is a solid of revolution
// (roadmap 20): its volume is that of the surface topo.SmoothLoftSurface
// makes of the same circles, pi R² dZ, the mesh's within its error. It
// is undone and redone, saved and read back with its profiles, and a
// profile deleted leaves it a problem.
func TestSmoothLoftGroup(t *testing.T) {
	d := api.New()
	d.With(func(sk *sketch.Sketch) { sk.ChordTolPct = 0.001 })
	rs, zs := []float64{10, 14, 8}, []float64{0, 10, 25}
	var profiles []api.Group
	var loops [][]srf.SBezier
	for i := range rs {
		if i == 0 {
			profiles = append(profiles, d.SketchIn(d.Workplane("XY"), fmt.Sprintf("p%d", i)))
		} else {
			profiles = append(profiles, d.SketchAt(above(d, zs[i]), api.V(1, 0, 0), api.V(0, 1, 0), fmt.Sprintf("p%d", i)))
		}
		d.Circle(api.V(0, 0, zs[i]), rs[i])
		loops = append(loops, circleLoop(rs[i], zs[i]))
	}
	step(d)
	before := saved(t, d)
	l := d.LoftThrough(profiles, "vase")
	if p := l.Problem(); p != "" {
		t.Fatal(p)
	}
	bs, err := topo.SmoothLoftSurface(loops, 0)
	if err != nil {
		t.Fatal(err)
	}
	// pi R² Z' over v, by the trapezoid rule on a fine grid: R and Z
	// are smooth within the two spans across.
	want := 0.0
	const n = 4000
	for k := 0; k < n; k++ {
		a, b := bs.PointAt(0, float64(k)/n), bs.PointAt(0, float64(k+1)/n)
		ra, rb := math.Hypot(a.X, a.Y), math.Hypot(b.X, b.Y)
		want += math.Pi * (ra*ra + rb*rb) / 2 * (b.Z - a.Z)
	}
	r := l.Check()
	if !r.Sound() || math.Abs(r.Volume-want) > 1e-3*want || l.Kind() != "loft" {
		t.Fatalf("the vase: %s, want %g", r.String(), want)
	}
	if got := l.Profiles(); len(got) != 3 || got[1].Name() != "p1" {
		t.Fatalf("profiles %v", got)
	}
	undoRedo(t, d, before)
	// A file holds the curves' points at the chord tolerance, which it
	// doesn't hold: the default's, for the round trip.
	d.With(func(sk *sketch.Sketch) { sk.ChordTolPct = 0 })
	d.Regen()
	r = l.Check()
	back := roundTrip(t, d)
	if bl := back.Group("vase"); len(bl.Profiles()) != 3 || bl.Profiles()[2].Name() != "p2" {
		t.Fatalf("reopened with profiles %v", bl.Profiles())
	} else if br := bl.Check(); math.Abs(br.Volume-r.Volume) > 1e-9 {
		t.Fatalf("reopened with volume %g, was %g", br.Volume, r.Volume)
	}
	profiles[1].Delete()
	if p := d.Group("vase").Problem(); p == "" {
		t.Error("the loft lost a profile and has no problem")
	}
}

// circleLoop is a circle of radius r about the z axis at height z, as
// four quarter arcs.
func circleLoop(r, z float64) []srf.SBezier {
	var out []srf.SBezier
	for k := 0; k < 4; k++ {
		a0 := float64(k) * math.Pi / 2
		sb := srf.SBezierFrom3(geom.Vector{X: r * math.Cos(a0), Y: r * math.Sin(a0), Z: z},
			geom.Vector{X: r * math.Sqrt2 * math.Cos(a0+math.Pi/4), Y: r * math.Sqrt2 * math.Sin(a0+math.Pi/4), Z: z},
			geom.Vector{X: r * math.Cos(a0+math.Pi/2), Y: r * math.Sin(a0+math.Pi/2), Z: z})
		sb.Weight[1] = math.Sqrt2 / 2
		out = append(out, sb)
	}
	return out
}

// Profiles of unequal counts loft once the longest curves are split
// (roadmap 20.1): a triangle to a square, ruled, and through a hexagon,
// smooth; both sound. A loft refuses a profile twice, and one alone.
func TestLoftUnequalCounts(t *testing.T) {
	profiles := func() (*api.Doc, api.Group, api.Group, api.Group) {
		d := api.New()
		d.With(func(sk *sketch.Sketch) { sk.ChordTolPct = 0.01 })
		tri := d.SketchIn(d.Workplane("XY"), "triangle")
		ngon(d, 3, 10, 0)
		sq := d.SketchAt(above(d, 10), api.V(1, 0, 0), api.V(0, 1, 0), "square")
		ngon(d, 4, 8, 10)
		hex := d.SketchAt(above(d, 20), api.V(1, 0, 0), api.V(0, 1, 0), "hexagon")
		ngon(d, 6, 9, 20)
		return d, tri, sq, hex
	}
	d, tri, sq, _ := profiles()
	ruled := d.Loft(tri, sq, "ruled")
	if r := ruled.Check(); !r.Sound() || ruled.Problem() != "" {
		t.Errorf("triangle to square: %s %s", r.String(), ruled.Problem())
	}
	d, tri, sq, hex := profiles()
	smooth := d.LoftThrough([]api.Group{tri, sq, hex}, "smooth")
	if r := smooth.Check(); !r.Sound() || smooth.Problem() != "" {
		t.Errorf("triangle through square to hexagon: %s %s", r.String(), smooth.Problem())
	}
	for name, ps := range map[string][]api.Group{"twice": {tri, sq, tri}, "alone": {tri}} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("a loft of a profile %s", name)
				}
			}()
			d.LoftThrough(ps, "")
		}()
	}
}

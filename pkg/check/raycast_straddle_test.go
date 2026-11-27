package check_test

import (
	"math"
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/script/api"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// straddleCase is a profile on the plane x = 5, parallel to the y axis
// and 5 mm off it, turned about that axis: the line z = 0 on the plane
// is where the turn runs along it, so a profile across that line moves
// through its plane one way on one side and back on the other.
type straddleCase struct {
	kind            string // lathe, revolve or helix
	degrees, length float64
	// A rectangle from z0 to z1, y from 0 to 6; or, with r > 0, a
	// circle of radius r about (5, 3, zc), drawn in a sketch whose axes
	// are diagonal, so that its extremes in z are inside its arcs.
	z0, z1    float64
	zc, r     float64
	onlyShort bool
}

func (c straddleCase) build(d *api.Doc) {
	d.Edit.SetChordTolerance(0.01)
	d.Features.Sketch3D("")
	axis := d.Sketch.Line(api.V(0, 0, 0), api.V(0, 10, 0))
	axis.SetConstruction(true)
	at := d.Sketch.Point(api.V(5, 0, 0))
	if c.r > 0 {
		h := math.Sqrt(0.5)
		d.Features.SketchAt(at, api.V(0, h, h), api.V(0, -h, h), "")
		d.Sketch.Circle(api.V(5, 3, c.zc), c.r)
	} else {
		d.Features.SketchAt(at, api.V(0, 0, 1), api.V(0, 1, 0), "")
		d.Sketch.Rect(api.V(5, 0, c.z0), api.V(5, 6, c.z1))
	}
	switch c.kind {
	case "lathe":
		d.Features.Lathe(axis, "")
	case "revolve":
		d.Features.Revolve(axis, c.degrees, "")
	case "helix":
		d.Features.Helix(axis, c.degrees, c.length, "")
	}
}

// A profile whose parts move through its plane in opposite directions
// as it turns is refused, with the problem marked on the line between
// them: a rectangle across the plane's line nearest the axis, whether
// lathed, revolved or swept up a helix; a circle crossing it by 0.05 mm
// where its extreme is inside an arc, not at an end; and, from the
// leaning profiles of TestOracleLeaning, a rectangle and a circle on a
// plane tilted 80° about y, lathed and swept round an arc. Made, the
// kernel swept only the profile's sides and caps, which crossed one
// another: the lathe from z -2 to 6 had a mesh of 603 mm³ that passed
// Check Solid, where the ring is 679.
func TestOracleStraddlingProfile(t *testing.T) {
	t.Parallel()
	for name, c := range map[string]straddleCase{
		"lathe even":     {kind: "lathe", z0: -4, z1: 4},
		"lathe lopsided": {kind: "lathe", z0: -2, z1: 6},
		"revolve":        {kind: "revolve", degrees: 90, z0: -2, z1: 6},
		"helix":          {kind: "helix", degrees: 270, length: 20, z0: -2, z1: 6},
		"circle":         {kind: "lathe", zc: 2.95, r: 3},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			d := api.New()
			c.build(d)
			at := refusedAt(t, d)
			if math.Abs(at.X-5) > 1e-6 || math.Abs(at.Z) > 1e-6 {
				t.Errorf("the problem is marked at %v, off the line x = 5, z = 0", at)
			}
		})
	}
	for _, c := range []leanCase{
		{kind: "lathe", shape: "rect", gamma: 80},
		{kind: "arc", shape: "rect", gamma: 80},
		{kind: "lathe", shape: "circle", gamma: 80},
		{kind: "arc", shape: "circle", gamma: 80},
	} {
		t.Run(c.String(), func(t *testing.T) {
			t.Parallel()
			d := api.New()
			c.build(d)
			refusedAt(t, d)
		})
	}
}

// refusedAt is where the active group's problem, which must be that the
// profile straddles, is marked.
func refusedAt(t *testing.T, d *api.Doc) geom.Vector {
	t.Helper()
	sk := d.Host().Sketch()
	sk.GenerateAll()
	g := sk.Group(sk.ActiveGroup())
	if !strings.HasPrefix(g.Problem.Msg, sketch.ErrProfileStraddles.Error()) {
		t.Fatalf("problem %q, want that the profile straddles", g.Problem.Msg)
	}
	if !g.Problem.HasPoint {
		t.Fatal("the problem marks no point")
	}
	if n := len(g.ThisShell.Surface); n != 0 {
		t.Errorf("a refused group made %d surfaces", n)
	}
	return g.Problem.At
}

// Profiles beside the line where the motion runs along their plane are
// made, and agree with the oracle: a rectangle with an edge on it, a
// circle touching it inside one of its arcs, and leaning profiles whose
// edge lies on it, on a plane tilted by acos(4/15) about y. A lathe of
// the rectangle fills the ring from 5 mm out to its corner (5, 6) at
// √61.
func TestOracleTouchingProfile(t *testing.T) {
	t.Parallel()
	for name, c := range map[string]straddleCase{
		"lathe":   {kind: "lathe", z0: 0, z1: 6, onlyShort: true},
		"below":   {kind: "lathe", z0: -6, z1: 0},
		"revolve": {kind: "revolve", degrees: 90, z0: 0, z1: 6},
		"helix":   {kind: "helix", degrees: 270, length: 20, z0: 0, z1: 6},
		"circle":  {kind: "lathe", zc: 3, r: 3, onlyShort: true},
	} {
		if testing.Short() && !c.onlyShort {
			continue
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			d := api.New()
			c.build(d)
			if p := d.Active().Problem(); p != "" {
				t.Fatalf("the kernel refused it: %s", p)
			}
			o := oracleOf(t, d)
			if diffs := o.Diff(meshOf(d), 24, 0.03); len(diffs) != 0 {
				t.Errorf("%d rays disagree with the mesh, as %+v", len(diffs), diffs[0])
			}
			if c.kind == "lathe" && c.r == 0 {
				want := math.Pi * 36 * 6
				if v := meshVolume(d); !near(v, want, 3e-3) {
					t.Errorf("the mesh's volume %g, want %g", v, want)
				}
			}
			// Every point the profile passes through is in the solid.
			w := geom.Vector{Y: -1}
			r := rand.New(rand.NewPCG(1, 2))
			degrees := c.degrees
			if c.kind == "lathe" {
				degrees = 360
			}
			outside := 0
			for range 200 {
				p := geom.Vector{X: 5, Y: 0.01 + 5.98*r.Float64(), Z: c.z0 + 0.01 + (c.z1-c.z0-0.02)*r.Float64()}
				if c.r > 0 {
					a := 2 * math.Pi * r.Float64()
					s := (c.r - 0.01) * math.Sqrt(r.Float64())
					p = geom.Vector{X: 5, Y: 3 + s*math.Cos(a), Z: c.zc + s*math.Sin(a)}
				}
				f := 0.001 + 0.998*r.Float64()
				q := p.RotatedAbout(geom.Vector{}, w, f*degrees*math.Pi/180).Plus(w.ScaledBy(f * c.length))
				if !o.Inside(q) {
					outside++
				}
			}
			if outside != 0 {
				t.Errorf("%d of the points the profile passes through are outside", outside)
			}
		})
	}
	gamma := math.Acos(4.0/15) * 180 / math.Pi
	for _, c := range []leanCase{
		{kind: "lathe", shape: "rect", gamma: gamma},
		{kind: "arc", shape: "rect", gamma: gamma},
	} {
		t.Run(c.String(), func(t *testing.T) {
			t.Parallel()
			d := api.New()
			want := c.build(d)
			if p := d.Active().Problem(); p != "" {
				t.Fatalf("the kernel refused it: %s", p)
			}
			o := oracleOf(t, d)
			if v := meshVolume(d); !near(v, want, 3e-3) {
				t.Errorf("the mesh's volume %g, want %g", v, want)
			}
			if diffs := o.Diff(meshOf(d), 24, 0.03); len(diffs) != 0 {
				t.Errorf("%d rays disagree with the mesh, as %+v", len(diffs), diffs[0])
			}
		})
	}
}

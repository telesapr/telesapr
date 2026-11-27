package check_test

import (
	"fmt"
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/internal/sweep"
	"github.com/telesapr/telesapr/pkg/script/api"
)

// leanCase is a profile about (15, 0, 0) on a plane through that point,
// the sketch's XY tilted by beta degrees about x and then by gamma about
// y, swept about the y axis: or, an "arc", along a quarter circle about
// it.
type leanCase struct {
	kind, shape        string
	beta, gamma        float64
	degrees, length    float64
	twoSided, measured bool
}

func (c leanCase) String() string {
	return fmt.Sprintf("%s %s lean %g,%g turn %g rise %g", c.kind, c.shape, c.beta, c.gamma, c.degrees, c.length)
}

// build draws the case, and gives the volume Pappus and Guldin give it:
// the area times what the profile's centroid, on every shape here
// (15, 0, 0), moves along the plane's normal. That holds where all of
// the profile moves the same way through its plane, as in every case
// here.
func (c leanCase) build(d *api.Doc) float64 {
	d.Edit.SetChordTolerance(0.01)
	d.Features.Sketch3D("")
	var axis api.Entity
	if c.kind != "arc" {
		axis = d.Sketch.Line(api.V(0, 0, 0), api.V(0, 10, 0))
		axis.SetConstruction(true)
	}
	centre := d.Sketch.Point(api.V(15, 0, 0))
	var path []api.Entity
	if c.kind == "arc" {
		d.Features.SketchIn(d.Workplane("ZX"), "path")
		path = []api.Entity{d.Sketch.Arc(api.V(0, 0, 0), api.V(15, 0, 0), api.V(0, 0, -15))}
	}
	sinB, cosB := math.Sincos(c.beta * math.Pi / 180)
	sinG, cosG := math.Sincos(c.gamma * math.Pi / 180)
	u := geom.Vector{X: cosG, Z: -sinG}
	v := geom.Vector{X: sinB * sinG, Y: cosB, Z: sinB * cosG}
	d.Features.SketchAt(centre, api.V(u.X, u.Y, u.Z), api.V(v.X, v.Y, v.Z), "")
	at := func(a, b float64) api.Vec {
		p := geom.Vector{X: 15}.Plus(u.ScaledBy(a)).Plus(v.ScaledBy(b))
		return api.V(p.X, p.Y, p.Z)
	}
	area := 48.0
	switch c.shape {
	case "rect":
		d.Sketch.Rect(at(-4, -3), at(4, 3))
	case "holed":
		d.Sketch.Rect(at(-4, -3), at(4, 3))
		d.Sketch.Circle(at(0, 0), 2)
		area -= 4 * math.Pi
	case "circle":
		d.Sketch.Circle(at(0, 0), 4)
		area = 16 * math.Pi
	case "triangle":
		p := []api.Vec{at(-4, -3), at(5, -1), at(-1, 4)}
		var l []api.Entity
		for i := range p {
			l = append(l, d.Sketch.Line(p[i], p[(i+1)%3]))
		}
		for i := range l {
			d.Constrain.Coincident(l[i].End(), l[(i+1)%3].Start())
		}
		area = 28.5
	}
	degrees, length := c.degrees, c.length
	switch c.kind {
	case "helix":
		d.Features.Helix(axis, degrees, length, "").SetTwoSided(c.twoSided)
	case "revolve":
		d.Features.Revolve(axis, degrees, "").SetTwoSided(c.twoSided)
	case "lathe":
		d.Features.Lathe(axis, "")
		degrees = 360
	case "arc":
		d.Features.Sweep(path, "")
		degrees = 90
	}
	// A line's direction runs from its end to its start.
	w := geom.Vector{Y: -1}
	moved := w.Cross(geom.Vector{X: 15}).ScaledBy(degrees * math.Pi / 180).Plus(w.ScaledBy(length))
	return area * math.Abs(u.Cross(v).Dot(moved))
}

// meshVolume is the volume a closed mesh bounds.
func meshVolume(d *api.Doc) float64 {
	v := 0.0
	for _, tr := range meshOf(d).Triangles {
		v += tr.A.Dot(tr.B.Cross(tr.C)) / 6
	}
	return v
}

// Profiles leaning to the axis of a helix, a revolve, a lathe or a
// sweep's arc, which the oracle builds by solving the motion for the
// angle that brings a point onto the profile (leanSweep): the kernel's
// mesh has Pappus and Guldin's volume, its rays agree with the oracle's
// within 0.03 mm at a chord tolerance of 0.01%, and the oracle's volume,
// where it is taken, is the same.
func TestOracleLeaning(t *testing.T) {
	t.Parallel()
	cases := []leanCase{
		{kind: "helix", shape: "rect", beta: 20, degrees: 270, length: 30, measured: true},
		{kind: "revolve", shape: "triangle", beta: 45, degrees: 200, measured: true},
		{kind: "lathe", shape: "rect", beta: 65, measured: true},
		{kind: "arc", shape: "rect", beta: 20, measured: true},
		{kind: "helix", shape: "rect", beta: 65, degrees: 270, length: 30},
		{kind: "revolve", shape: "rect", gamma: 50, degrees: 250, measured: true},
		{kind: "lathe", shape: "triangle", beta: 30, gamma: 30},
		{kind: "arc", shape: "triangle", beta: 65},
		{kind: "helix", shape: "triangle", gamma: 25, degrees: 720, length: 60, measured: true},
		{kind: "revolve", shape: "rect", beta: 30, gamma: 20, degrees: 220, twoSided: true},
		{kind: "lathe", shape: "rect", beta: -40, gamma: -20, measured: true},
		{kind: "arc", shape: "circle", beta: 30, gamma: 30},
		{kind: "helix", shape: "rect", beta: 30, gamma: 30, degrees: -400, length: 40, twoSided: true, measured: true},
		{kind: "revolve", shape: "circle", gamma: 25, degrees: 200},
		{kind: "helix", shape: "holed", beta: 30, gamma: 20, degrees: 270, length: -25},
		{kind: "helix", shape: "circle", beta: 45, degrees: 270, length: 30},
		{kind: "helix", shape: "rect", gamma: 70, degrees: 250, length: 30},
	}
	for _, c := range sweep.Pick(cases, 12) {
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
			if c.measured && measuresVolume() && !testing.Short() {
				if m := o.Measure(2e-3, 64); !near(m.Volume, want, 5e-3) {
					t.Errorf("volume %g, want %g (spread %g at %d rays)", m.Volume, want, m.Spread(), m.Rays)
				}
			}
		})
	}
}

// A square on a plane tilted to the path, swept up a line, round an arc
// and along another: prisms along the lines and a leaning turn about the
// arc's axis.
func TestOracleLeaningSweep(t *testing.T) {
	t.Parallel()
	d := api.New()
	d.Edit.SetChordTolerance(0.01)
	d.Features.Sketch3D("")
	origin := d.Sketch.Point(api.V(0, 0, 0))
	d.Features.SketchIn(d.Workplane("ZX"), "path")
	up := d.Sketch.Line(api.V(0, 0, 0), api.V(0, 0, 30))
	bend := d.Sketch.Arc(api.V(25, 0, 30), api.V(0, 0, 30), api.V(25, 0, 55))
	along := d.Sketch.Line(api.V(25, 0, 55), api.V(65, 0, 55))
	d.Constrain.Coincident(up.End(), bend.Start())
	d.Constrain.Coincident(bend.End(), along.Start())
	d.Constrain.Tangent(up, bend)
	d.Constrain.Tangent(bend, along)
	sin, cos := math.Sincos(25 * math.Pi / 180)
	d.Features.SketchAt(origin, api.V(1, 0, 0), api.V(0, cos, sin), "square")
	d.Sketch.Rect(api.V(-6, -6*cos, -6*sin), api.V(6, 6*cos, 6*sin))
	g := d.Features.Sweep([]api.Entity{up, bend, along}, "bar")
	if p := g.Problem(); p != "" {
		t.Fatalf("the kernel refused it: %s", p)
	}
	o := oracleOf(t, d)
	if diffs := o.Diff(meshOf(d), 24, 0.03); len(diffs) != 0 {
		t.Errorf("%d rays disagree with the mesh, as %+v", len(diffs), diffs[0])
	}
	if measuresVolume() && !testing.Short() {
		if m, v := o.Measure(1e-3, 128), meshVolume(d); !near(m.Volume, v, 3e-3) {
			t.Errorf("volume %g, the mesh's %g (spread %g at %d rays)", m.Volume, v, m.Spread(), m.Rays)
		}
	}
}

package sketch

import (
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// TestExtrudeSeed starts a new extrusion's solve along its workplane's
// normal, toward the viewer, half the default depth long whatever the
// view's zoom; seen edge on, along the normal's own side; skewed, along
// the view's normal.
func TestExtrudeSeed(t *testing.T) {
	t.Parallel()
	sk := NewSketch()
	sk.NewFile()
	sk.GenerateAll()
	// A group with no sketch takes the fallback depth, 10.
	for _, c := range []struct {
		name    string
		view    geom.Vector
		plane   core.HRequest
		subtype GroupSubtype
		want    geom.Vector
	}{
		{"facing the view", geom.Vector{Z: 7}, HRequestReferenceXY, GroupOneSided, geom.Vector{Z: 5}},
		{"from behind", geom.Vector{Z: -0.01}, HRequestReferenceXY, GroupOneSided, geom.Vector{Z: -5}},
		{"at an angle", geom.Vector{X: 3, Z: 1}, HRequestReferenceXY, GroupOneSided, geom.Vector{Z: 5}},
		{"edge on", geom.Vector{Z: 7}, HRequestReferenceZX, GroupOneSided, geom.Vector{Y: 5}},
		{"edge on, skewed", geom.Vector{Z: 7}, HRequestReferenceZX, GroupOneSkewed, geom.Vector{Z: 5}},
	} {
		g := Group{Type: GroupExtrude, Subtype: c.subtype}
		g.Predef.EntityB = c.plane.Entity(0)
		got := g.extrudeSeed(sk, c.view)
		if c.name == "edge on" {
			// Either way along the normal will do.
			got.Y = math.Abs(got.Y)
		}
		if !got.Equals(c.want) {
			t.Errorf("%s: seed %v, want %v", c.name, got, c.want)
		}
	}
}

// TestDefaultExtrudeDepth takes the shorter side of the profile's
// bounding box in its workplane, no less than half the longer, to a
// round number.
func TestDefaultExtrudeDepth(t *testing.T) {
	t.Parallel()
	sk := NewSketch()
	sk.NewFile()
	sk.GenerateAll()
	contour := func(pts ...geom.Vector) srf.SContour {
		var c srf.SContour
		for _, p := range pts {
			c.Points = append(c.Points, srf.SPoint{P: p})
		}
		return c
	}
	rect := func(lo, hi geom.Vector) srf.SContour {
		return contour(lo, geom.Vector{X: hi.X, Y: lo.Y, Z: lo.Z}, hi, geom.Vector{X: lo.X, Y: hi.Y, Z: hi.Z}, lo)
	}
	// A 10 across circle's polygon, its vertices on the circle.
	var circle []geom.Vector
	for i := range 17 {
		a := 2 * math.Pi * float64(i) / 16
		circle = append(circle, geom.Vector{X: 5 * math.Cos(a), Y: 5 * math.Sin(a)})
	}
	for _, c := range []struct {
		name  string
		plane core.HRequest
		loops []srf.SContour
		opens []srf.SBezierLoop
		want  float64
	}{
		{"40 by 25 rectangle", HRequestReferenceXY,
			[]srf.SContour{rect(geom.Vector{}, geom.Vector{X: 40, Y: 25})}, nil, 20},
		{"off the origin, with a hole", HRequestReferenceXY,
			[]srf.SContour{rect(geom.Vector{X: -100, Y: 7}, geom.Vector{X: -60, Y: 32}),
				rect(geom.Vector{X: -90, Y: 10}, geom.Vector{X: -80, Y: 20})}, nil, 20},
		{"in the zx plane", HRequestReferenceZX,
			[]srf.SContour{rect(geom.Vector{Y: 3}, geom.Vector{X: 25, Y: 3, Z: 40})}, nil, 20},
		{"circle", HRequestReferenceXY, []srf.SContour{contour(circle...)}, nil, 10},
		{"90 by 10 bar", HRequestReferenceXY,
			[]srf.SContour{rect(geom.Vector{}, geom.Vector{X: 90, Y: 10})}, nil, 50},
		{"100 square", HRequestReferenceXY,
			[]srf.SContour{rect(geom.Vector{}, geom.Vector{X: 100, Y: 100})}, nil, 100},
		{"70 by 60 rectangle", HRequestReferenceXY,
			[]srf.SContour{rect(geom.Vector{}, geom.Vector{X: 70, Y: 60})}, nil, 50},
		{"open curves", HRequestReferenceXY, nil,
			[]srf.SBezierLoop{{Curves: []srf.SBezier{srf.SBezierFrom2(geom.Vector{}, geom.Vector{X: 3})}}}, 2},
		{"nothing", HRequestReferenceXY, nil, nil, 10},
	} {
		src := sk.AddGroup(Group{Type: GroupDrawingWorkplane, Scale: 1})
		sk.Group(src).PolyLoops.Contours = c.loops
		sk.Group(src).BezierOpens = c.opens
		g := Group{Type: GroupExtrude, Subtype: GroupOneSided, OpA: src}
		g.Predef.EntityB = c.plane.Entity(0)
		if got := g.defaultExtrudeDepth(sk); got != c.want {
			t.Errorf("%s: depth %g, want %g", c.name, got, c.want)
		}
	}
	if got := (&Group{Type: GroupExtrude, OpA: core.HGroup{V: 0x7777}}).defaultExtrudeDepth(sk); got != 10 {
		t.Errorf("no source group: depth %g, want 10", got)
	}
}

// TestRoundDepth takes a length to the nearest of 1, 2 and 5 times a
// power of ten by ratio.
func TestRoundDepth(t *testing.T) {
	t.Parallel()
	for d, want := range map[float64]float64{
		0: 10, -3: 10, math.NaN(): 10,
		1: 1, 1.4: 1, 1.5: 2, 3.1: 2, 3.2: 5, 4.99: 5, 7: 5, 7.1: 10,
		0.3: 0.2, 140: 100, 150: 200, 2000: 2000,
	} {
		if got := roundDepth(d); math.Abs(got-want) > 1e-12*want {
			t.Errorf("roundDepth(%g) = %g, want %g", d, got, want)
		}
	}
}

package slvs

import (
	"testing"

	"github.com/telesapr/telesapr/pkg/internal/golden"
	"github.com/telesapr/telesapr/pkg/persist/tlspr"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// TestModifyTools runs the tangent arc and split tools on sketches built
// on a fixture, and requires the goldens in testdata/tlspr/golden/modify.
// Until the C++ comparison was frozen, the saves were those C++ wrote
// after the same steps (testdata/edit, from a C++ harness).
func TestModifyTools(t *testing.T) {
	v := func(x, y float64) geom.Vector { return geom.Vector{X: x, Y: y} }
	for _, tc := range []struct {
		name    string
		wantErr bool
		run     func(sk *sketch.Sketch) error
	}{
		{"tangent", false, func(sk *sketch.Sketch) error {
			return tangentCorner(sk, sketch.TangentArcSettings{Radius: 10})
		}},
		{"tangent_modify", false, func(sk *sketch.Sketch) error {
			return tangentCorner(sk, sketch.TangentArcSettings{Radius: 10, Modify: true})
		}},
		{"tangent_manual", false, func(sk *sketch.Sketch) error {
			return tangentCorner(sk, sketch.TangentArcSettings{Radius: 4, Manual: true})
		}},
		{"tangent_arc", false, func(sk *sketch.Sketch) error {
			return tangentLineArc(sk, -15, sketch.TangentArcSettings{Radius: 10})
		}},
		{"tangent_arc_modify", false, func(sk *sketch.Sketch) error {
			return tangentLineArc(sk, -15, sketch.TangentArcSettings{Radius: 10, Modify: true})
		}},
		{"tangent_fail", true, func(sk *sketch.Sketch) error {
			return tangentLineArc(sk, 0, sketch.TangentArcSettings{Radius: 10})
		}},
		{"split_lines", false, func(sk *sketch.Sketch) error {
			a := addRequest(sk, sketch.RequestLineSegment, v(-20, -5), v(30, 10))
			b := addRequest(sk, sketch.RequestLineSegment, v(0, -20), v(4, 25))
			sk.Constrain(core.ConstraintVertical, core.NoEntity, core.NoEntity, b.Entity(0), core.NoEntity, false, false)
			sk.GenerateAllTyped(sketch.GenerateAll)
			return sk.SplitLinesOrCurves(selectEntities(a.Entity(0), b.Entity(0)), geom.Point2d{})
		}},
		{"split_circle_line", false, func(sk *sketch.Sketch) error {
			a := addRequest(sk, sketch.RequestCircle, v(5, 5))
			sk.MustGetEntity(a.Entity(64)).DistanceForceTo(sk, 12)
			b := addRequest(sk, sketch.RequestLineSegment, v(-30, 0), v(30, 8))
			sk.GenerateAllTyped(sketch.GenerateAll)
			return sk.SplitLinesOrCurves(selectEntities(b.Entity(0), a.Entity(0)), sk.ProjectPoint(v(20, 6)))
		}},
		{"split_arc_cubic", false, func(sk *sketch.Sketch) error {
			a := addRequest(sk, sketch.RequestArcOfCircle, v(0, 0), v(20, 0), v(-20, 0))
			b := addRequest(sk, sketch.RequestCubic, v(-10, -5), v(-5, 30), v(5, 30), v(12, 25))
			sk.GenerateAllTyped(sketch.GenerateAll)
			return sk.SplitLinesOrCurves(selectEntities(a.Entity(0), b.Entity(0)), sk.ProjectPoint(v(-10, 15)))
		}},
		{"split_point", false, func(sk *sketch.Sketch) error {
			a := addRequest(sk, sketch.RequestLineSegment, v(-20, -5), v(30, 10))
			p := addRequest(sk, sketch.RequestDatumPoint)
			sk.MustGetEntity(p.Entity(0)).PointForceTo(sk, v(5, 3))
			sk.Constrain(core.ConstraintPtOnLine, p.Entity(0), core.NoEntity, a.Entity(0), core.NoEntity, false, false)
			sk.GenerateAllTyped(sketch.GenerateAll)
			return sk.SplitLinesOrCurves(selectEntities(a.Entity(0), p.Entity(0)), geom.Point2d{})
		}},
		{"split_periodic", false, func(sk *sketch.Sketch) error {
			a := addRequest(sk, sketch.RequestCubicPeriodic, v(0, 0), v(20, 0), v(20, 20))
			b := addRequest(sk, sketch.RequestLineSegment, v(-10, 5), v(40, 8))
			sk.GenerateAllTyped(sketch.GenerateAll)
			return sk.SplitLinesOrCurves(selectEntities(a.Entity(0), b.Entity(0)), sk.ProjectPoint(v(0, 5)))
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			doc, err := LoadFile(fixture("request/line_segment/normal.slvs"))
			if err != nil {
				t.Fatal(err)
			}
			sk := doc.Sketch
			if err := tc.run(sk); (err != nil) != tc.wantErr {
				t.Fatalf("error %v, want error %v", err, tc.wantErr)
			}
			sk.GenerateAllTyped(sketch.GenerateAll)
			// As a save regenerates again.
			sk.GenerateAll()
			golden.Check(t, fixture("tlspr/golden/modify/"+tc.name+".tlspr"), tlspr.Encode(doc.Sketch, tlspr.Options{}))
		})
	}
}

// addRequest adds a request of type typ with its points at pts, as a
// C++ harness does with GraphicsWindow::AddRequest and PointForceTo.
func addRequest(sk *sketch.Sketch, typ sketch.RequestType, pts ...geom.Vector) core.HRequest {
	hr := sk.AddRequestOfType(typ, false)
	for i, p := range pts {
		sk.MustGetEntity(hr.Entity(1+i)).PointForceTo(sk, p)
	}
	return hr
}

func selectEntities(hs ...core.HEntity) []sketch.Selection {
	var sel []sketch.Selection
	for _, h := range hs {
		sel = append(sel, sketch.Selection{Entity: h})
	}
	return sel
}

// tangentCorner rounds the corner where two lines meet.
func tangentCorner(sk *sketch.Sketch, s sketch.TangentArcSettings) error {
	v := func(x, y float64) geom.Vector { return geom.Vector{X: x, Y: y} }
	a := addRequest(sk, sketch.RequestLineSegment, v(0, 0), v(40, 5))
	b := addRequest(sk, sketch.RequestLineSegment, v(0, 0), v(3, 30))
	sk.ConstrainCoincident(a.Entity(1), b.Entity(1))
	sk.GenerateAllTyped(sketch.GenerateAll)
	return sk.MakeTangentArc(a.Entity(1), s)
}

// tangentLineArc rounds the corner where a line, from (-30, y0), meets
// an arc.
func tangentLineArc(sk *sketch.Sketch, y0 float64, s sketch.TangentArcSettings) error {
	v := func(x, y float64) geom.Vector { return geom.Vector{X: x, Y: y} }
	a := addRequest(sk, sketch.RequestLineSegment, v(-30, y0), v(0, 0))
	b := addRequest(sk, sketch.RequestArcOfCircle, v(0, 20), v(0, 0), v(20, 20))
	sk.ConstrainCoincident(a.Entity(2), b.Entity(2))
	sk.GenerateAllTyped(sketch.GenerateAll)
	return sk.MakeTangentArc(a.Entity(2), s)
}

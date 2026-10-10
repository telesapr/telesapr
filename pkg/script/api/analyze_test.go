package api

import (
	"math"
	"testing"
)

// The Analyze menu's measurements, from a script: a 40 by 20 plate with
// a hole of radius 5, extruded 10.
func TestAnalyze(t *testing.T) {
	t.Parallel()
	d := New()
	d.Edit.SetChordTolerance(0.01)
	plate := d.Features.SketchIn(d.Workplane("XY"), "plate")
	sides := d.Sketch.Rect(V(0, 0, 0), V(40, 20, 0))
	hole := d.Sketch.Circle(V(20, 10, 0), 5)
	ext := d.Features.Extrude(10, "")

	if got := d.Analyze.CurveLength(sides...); math.Abs(got-120) > 1e-9 {
		t.Errorf("the rectangle's length is %v, want 120", got)
	}
	if got, want := d.Analyze.CurveLength(hole), 2*math.Pi*5; math.Abs(got-want) > 0.01*want {
		t.Errorf("the circle's length is %v, want about %v", got, want)
	}
	if got, want := plate.Analyze().Area(), 800-math.Pi*25; math.Abs(got-want) > 0.01*want {
		t.Errorf("the plate's area is %v, want about %v", got, want)
	}
	if p := ext.Analyze().CenterOfMass(); p.DistanceTo(V(20, 10, 5)) > 0.05 {
		t.Errorf("the centre of mass is %v, want (20, 10, 5)", p)
	}

	// The extrusion's flat faces: the plate's top and bottom, with the
	// hole, and its four sides.
	faces := ext.Faces()
	if len(faces) != 6 {
		t.Fatalf("the extrusion made %d flat faces, want 6", len(faces))
	}
	if got, want := d.Analyze.FacesArea(faces...), 2*(800-math.Pi*25)+2*(40+20)*10; math.Abs(got-want) > 0.01*want {
		t.Errorf("the flat faces are %v mm², want about %v", got, want)
	}

	failsWith(t, "no curve", func() { d.Analyze.CurveLength(sides[0].Start()) })
	failsWith(t, "no curves", func() { d.Analyze.CurveLength() })
	open := d.Features.SketchIn(d.Workplane("XY"), "open")
	d.Sketch.Line(V(0, 0, 0), V(10, 0, 0))
	failsWith(t, "not a closed region", func() { open.Analyze().Area() })
}

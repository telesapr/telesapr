package api

import (
	"math"
	"strings"
	"testing"
)

// A fillet where a flat face meets a cone off its axis is refused with
// what the edge is, not with how its curve failed to be a circle.
func TestFilletFlatOnConeOffAxis(t *testing.T) {
	d := New()
	d.SketchIn(d.Workplane("ZX"), "")
	axis := d.Line(V(0, 0, -2), V(0, 0, 32))
	axis.SetConstruction(true)
	d.Polyline(true, V(0, 0, 0), V(10, 0, 0), V(4, 0, 30), V(0, 0, 30))
	d.Lathe(axis, "")
	// A band whose side y = 3 cuts the cone along a hyperbola.
	d.SketchIn(d.Workplane("XY"), "")
	d.Rect(V(6, -3, 0), V(20, 3, 0))
	band := d.Extrude(25, "band")
	if p := band.Problem(); p != "" {
		t.Fatal(p)
	}
	edge := band.Solid().EdgeAt(V(math.Sqrt(64-9), 3, 10))
	for _, what := range []string{"a fillet", "a chamfer"} {
		func() {
			defer func() {
				err, _ := recover().(error)
				want := "the edge is where the flat face meets a cone off its axis: " + what + " rounds"
				if err == nil || !strings.Contains(err.Error(), want) {
					t.Errorf("%s of the band's edge on the cone: %v, want it to say %q", what, err, want)
				}
			}()
			if what == "a fillet" {
				d.FilletSolid([]SolidEdge{edge}, 1, "")
			} else {
				d.ChamferSolid([]SolidEdge{edge}, 1, "")
			}
		}()
	}
}

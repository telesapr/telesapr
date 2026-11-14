package api

import (
	"fmt"
	"math"
	"testing"
)

// A plane slanted to a lathe's axis meets each of the lathe's cylinder
// patches in an ellipse arc, which ran on past the patch's ends: a cut
// with a straight side neither along nor across the axis, sketched in a
// plane through the axis and turned about it, left the tube open along
// the patches' seams, the more so past a step in the bore, whose
// cylinder's extension runs through the wall (found modelling a tube
// with windows, 2026-11-14). Each solid is checked by the ray-cast
// oracle too.
func TestObliqueCutThroughLathe(t *testing.T) {
	t.Parallel()
	type cut struct {
		step   bool    // a bore of Ø13.9 up to x 5.5, Ø15 after
		shape  string  // "triangle" or "teardrop"
		x0     float64 // where the shape's tip is along the tube
		phi    float64 // how far the sketch's plane is turned about the axis
		copies int     // turned copies of the cut, 120° apart
	}
	cuts := []cut{
		{true, "triangle", 8.6, 30, 0},
		{true, "triangle", 8.6, 45, 0},
		{false, "triangle", 8.6, 45, 0},
		{true, "triangle", 3, 90, 0},
		{true, "triangle", 44, 0, 0},
		{true, "teardrop", 8.6, 30, 0},
		{true, "teardrop", 1, 15, 0},
		{true, "triangle", 8.6, 90, 3},
	}
	for _, c := range cuts {
		name := fmt.Sprintf("%s-%g-at-%g", c.shape, c.x0, c.phi)
		if !c.step {
			name += "-plain"
		}
		if c.copies > 0 {
			name += fmt.Sprintf("-%d-copies", c.copies)
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if testing.Short() && c.shape == "teardrop" {
				t.Skip("a teardrop's ray-cast check takes two seconds")
			}
			d := New()
			axis := d.Sketch.Line(V(0, 0, 0), V(10, 0, 0))
			axis.Request().SetConstruction(true)
			if c.step {
				d.Sketch.Polyline(true, V(0, 6.95, 0), V(5.5, 6.95, 0), V(5.5, 7.5, 0), V(50, 7.5, 0), V(50, 8.3, 0), V(0, 8.3, 0))
			} else {
				d.Sketch.Polyline(true, V(0, 7.5, 0), V(50, 7.5, 0), V(50, 8.3, 0), V(0, 8.3, 0))
			}
			tube := d.Features.Lathe(axis, "tube")
			p := c.phi * math.Pi / 180
			d.Activate(tube)
			d.Features.SketchAt(d.Origin(), V(1, 0, 0), V(0, math.Cos(p), math.Sin(p)), "outline")
			uv := d.Sketch.UV
			if c.shape == "triangle" {
				d.Sketch.Polyline(true, uv(c.x0, 0), uv(c.x0+3.4, 2), uv(c.x0+3.4, -2))
			} else {
				// An arc and two straight lines tangent to it from the tip.
				const l, r = 4.0, 1.8
				b := math.Acos(r / l)
				cx := c.x0 + l
				t1 := uv(cx+r*math.Cos(math.Pi+b), r*math.Sin(math.Pi+b))
				t2 := uv(cx+r*math.Cos(math.Pi-b), r*math.Sin(math.Pi-b))
				d.Sketch.Arc(uv(cx, 0), t1, t2)
				d.Sketch.Line(uv(c.x0, 0), t1)
				d.Sketch.Line(t2, uv(c.x0, 0))
			}
			last := d.Features.Extrude(10, "cut")
			last.SetCombine(Difference)
			if c.copies > 0 {
				// The copies cut as the cut does, their source.
				last = d.Features.Rotate(c.copies, d.Origin(), axis, 120, "copies")
			}
			d.Edit.Regen()
			if r := last.Analyze().Check(); !r.Sound() || r.Empty() {
				t.Errorf("unsound: %s", r.String())
			}
			if rc := last.Analyze().RayCheck(); len(rc.Disagree) > 0 {
				t.Errorf("%d of %d rays disagree with the oracle: volume %.3f, oracle's %.3f",
					len(rc.Disagree), rc.Rays, rc.MeshVolume, rc.Measure.Volume)
			}
		})
	}
}

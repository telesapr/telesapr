package api

import (
	"fmt"
	"math"
	"runtime"
	"testing"
)

// twoArcPath sketches, in the ZX plane, an arc of radius 15 from the origin
// turning turn1 degrees about a centre on x at sgn·15, then a right-angle
// corner and a second arc of radius 15 turning turn2 degrees, bending the
// same way as the first when sgn2 is 1 and the other way when it is -1; then
// a 2 × 3 rectangle on XY to sweep along them.
func twoArcPath(d *Doc, sgn, sgn2, turn1, turn2 float64) (Entity, Entity) {
	y := V(0, 1, 0)
	d.Features.SketchIn(d.Workplane("ZX"), "path")
	c1 := V(sgn*15, 0, 0)
	s := V(0, 0, 0)
	e1 := c1.Plus(s.Minus(c1).RotatedAroundAxis(y, sgn*turn1*math.Pi/180))
	t1 := e1.Minus(c1).RotatedAroundAxis(y, sgn*math.Pi/2).ScaledBy(1.0 / 15)
	c2 := e1.Plus(t1.ScaledBy(15 * sgn2))
	e2 := c2.Plus(e1.Minus(c2).RotatedAroundAxis(y, sgn2*turn2*math.Pi/180))
	a1 := d.Sketch.Arc(c1, s, e1)
	a2 := d.Sketch.Arc(c2, e1, e2)
	d.Features.SketchIn(d.Workplane("XY"), "profile")
	d.Sketch.Rect(V(-1, -1.5, 0), V(1, 1.5, 0))
	return a1, a2
}

// TestSweepTwoArcsAboutCorner sweeps along two arcs about a right-angle
// corner, bending either way, flat and twisted: each is made sound, is a
// problem on the group or is refused with an error, never a runtime panic
// (two arcs alike about the corner put the cut of a sweep crossing itself
// exactly on the corner, and the stretch after it indexed a piece at -1).
func TestSweepTwoArcsAboutCorner(t *testing.T) {
	t.Parallel()
	for _, sgn := range []float64{1, -1} {
		for _, sgn2 := range []float64{1, -1} {
			for _, turn := range []float64{30, 60, 90} {
				for _, twist := range []float64{0, 90} {
					name := fmt.Sprintf("sgn %v sgn2 %v turn %v twist %v", sgn, sgn2, turn, twist)
					func() {
						defer func() {
							if r := recover(); r != nil {
								if _, ok := r.(runtime.Error); ok {
									t.Errorf("%s: %v", name, r)
								} else if _, ok := r.(error); !ok {
									t.Errorf("%s: panic %v", name, r)
								}
							}
						}()
						d := New()
						a1, a2 := twoArcPath(d, sgn, sgn2, turn, turn)
						g := d.Features.Sweep([]Entity{a1, a2}, "")
						if twist != 0 {
							g.AsSweep().SetTwist(twist)
						}
						c := g.Analyze().Check()
						if p := g.Problem(); p == "" && !c.Sound() {
							t.Errorf("%s: unsound with no problem: %s", name, c.Summary())
						}
					}()
				}
			}
		}
	}
}
